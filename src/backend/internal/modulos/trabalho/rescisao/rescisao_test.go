package rescisao

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/decimoterceiro"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/ferias"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/fgts"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

var (
	v   = testeapoio.Valor
	dia = testeapoio.Dia
)

func TestAvisoEAvos(t *testing.T) {
	t.Parallel()
	adm := dia(2024, 3, 11)
	for _, c := range []struct {
		saida tipos.Data
		anos  int
		aviso int
	}{{dia(2024, 3, 11), 0, 30}, {dia(2026, 3, 10), 1, 33}, {dia(2026, 3, 11), 2, 36}, {dia(2054, 3, 11), 30, 90}} {
		if a := AnosCompletos(adm, c.saida); a != c.anos {
			t.Errorf("%s: %d anos; quer %d", c.saida, a, c.anos)
		}
		if d := DiasDeAviso(adm, c.saida); d != c.aviso {
			t.Errorf("%s: %d dias de aviso; quer %d", c.saida, d, c.aviso)
		}
	}
	saida := dia(2026, 12, 15)
	for _, c := range []struct {
		tipo, aviso int64
		dias        int
	}{{SemJustaCausa, AvisoIndenizado, 36}, {Acordo, AvisoIndenizado, 18}, {SemJustaCausa, AvisoTrabalhado, 0},
		{PedidoDemissao, AvisoIndenizado, 0}, {JustaCausa, SemAviso, 0}} {
		d, p := AvisoPago(c.tipo, c.aviso, adm, saida)
		if d != c.dias || !p.Igual(saida.SomarDias(c.dias)) {
			t.Errorf("tipo %d, aviso %d: %d dias (%s); quer %d", c.tipo, c.aviso, d, p, c.dias)
		}
	}
	// 11/03 a 15/12: 9 meses inteiros e 5 dias (não conta); até 25/12, 15 dias (conta); 12 no máximo.
	for _, c := range []struct {
		ate  tipos.Data
		avos int
	}{{dia(2026, 12, 15), 9}, {dia(2026, 12, 25), 10}, {dia(2026, 12, 24), 9}, {dia(2027, 1, 20), 10}, {dia(2026, 3, 11), 0},
		{dia(2027, 3, 10), 12}, {dia(2028, 6, 1), 12}, {dia(2026, 3, 25), 1}, {dia(2026, 3, 24), 0}, {dia(2026, 3, 10), 0}} {
		if a := AvosFerias(dia(2026, 3, 11), c.ate); a != c.avos {
			t.Errorf("avos até %s = %d; quer %d", c.ate, a, c.avos)
		}
	}
	for _, c := range []struct {
		saida, adm tipos.Data
		dias       int
	}{{dia(2026, 12, 15), adm, 15}, {dia(2026, 11, 30), adm, 30}, {dia(2026, 2, 28), adm, 30}, {dia(2026, 12, 31), adm, 30},
		{dia(2026, 12, 15), dia(2026, 12, 10), 6}, {dia(2026, 12, 15), dia(2026, 3, 10), 15}, {dia(2026, 12, 15), dia(2025, 12, 10), 15}} {
		if d := diasNoMes(c.saida, c.adm); d != c.dias {
			t.Errorf("saldo de %s: %d dias; quer %d", c.saida, d, c.dias)
		}
	}
	if NomeDoTipo(Acordo) != "Acordo (art. 484-A)" || TipoConhecido(0) || TipoConhecido(6) || !TipoConhecido(FimExperiencia) {
		t.Error("nomes e tipos")
	}
}

func tabelas(t *testing.T) (encargos.TabelaInss, encargos.TabelaIrrf) {
	t.Helper()
	db := testeapoio.NovoBanco(t)
	inss, _ := encargos.LerTabelasInss(t.Context(), db)
	irrf, _ := encargos.LerTabelasIrrf(t.Context(), db)
	ti, _ := encargos.Vigente(inss, dia(2026, 12, 1))
	tr, _ := encargos.Vigente(irrf, dia(2026, 12, 1))
	return ti, tr
}

// Os cinco tipos com as mesmas entradas (contas no comentário). Salário 6000, saída em 15/12/2026, 2 anos de casa (36
// dias de aviso), 10/12 de férias, 12/12 de 13º (com 3000 da 1ª parcela já pagos), 30 dias de férias vencidas simples
// e 10000 de FGTS.
func TestCalcularOsCincoTipos(t *testing.T) {
	t.Parallel()
	ti, tr := tabelas(t)
	base := Entrada{Admissao: dia(2024, 3, 11), Saida: dia(2026, 12, 15), SalarioMes: v("6000.00"),
		Vencidas: []FeriasVencidas{{Inicio: dia(2025, 3, 11), Dias: 30}}, AvosFerias: 10, Avos13: 12,
		PrimeiraParcela13: v("3000.00"), SaldoFgts: v("10000.00"), Inss: ti, Irrf: tr}
	casos := []struct {
		nome                       string
		tipo, aviso                int64
		verbas, descontos, liquido string
		deposito, multa, sacavel   string
		seguro                     bool
	}{
		// Saldo 3000 (15 dias) + aviso 7200 (36 dias) + 13º 6000 + férias 5000 + 1666,67 + vencidas 6000 + 2000 = 30866,67.
		// INSS do saldo 248,60; IRRF do saldo 0; INSS do 13º 641,51; IRRF do 13º 385,10; a 1ª parcela 3000 = 4275,21.
		// FGTS: 8% de 16200 = 1296; multa 40% de 11296 = 4518,40; saca 11296 + 4518,40.
		{"sem justa causa", SemJustaCausa, AvisoIndenizado, "30866.67", "4275.21", "26591.46", "1296.00", "4518.40", "15814.40", true},
		// Aviso pela metade (18 dias, 3600); multa 20% de 11008 e saque de 80%.
		{"acordo", Acordo, AvisoIndenizado, "27266.67", "4275.21", "22991.46", "1008.00", "2201.60", "11008.00", false},
		// Não cumpriu o aviso: desconta 30 dias (6000); sem multa e sem saque.
		{"pedido de demissão sem cumprir o aviso", PedidoDemissao, AvisoIndenizado, "23666.67", "10275.21", "13391.46", "720.00", "0.00", "0.00", false},
		{"pedido de demissão cumprindo o aviso", PedidoDemissao, AvisoTrabalhado, "23666.67", "4275.21", "19391.46", "720.00", "0.00", "0.00", false},
		// Só o saldo e as vencidas; INSS do saldo.
		{"justa causa", JustaCausa, SemAviso, "11000.00", "248.60", "10751.40", "240.00", "0.00", "0.00", false},
		// Sem aviso e sem multa, mas saca tudo (10000 + 720).
		{"fim da experiência", FimExperiencia, SemAviso, "23666.67", "4275.21", "19391.46", "720.00", "0.00", "10720.00", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			e := base
			e.Tipo, e.Aviso = c.tipo, c.aviso
			r := Calcular(e)
			got := fmt.Sprintln(r.TotalVerbas, r.TotalDescontos, r.Liquido, r.Fgts.DepositoRescisao, r.Fgts.Multa, r.Fgts.Sacavel, r.SeguroDesemprego)
			quer := fmt.Sprintln(c.verbas, c.descontos, c.liquido, c.deposito, c.multa, c.sacavel, c.seguro)
			if got != quer {
				t.Fatalf("rescisão = %s\n           quer %s\n%+v\n%+v", got, quer, r.Verbas, r.Descontos)
			}
		})
	}
	// As férias vencidas fora do prazo vêm em dobro: 12000 + 4000.
	e := base
	e.Tipo, e.Aviso = JustaCausa, SemAviso
	e.Vencidas = []FeriasVencidas{{Inicio: dia(2024, 3, 11), Dias: 30, EmDobro: true}}
	r := Calcular(e)
	if r.TotalVerbas.String() != "19000.00" || !strings.Contains(r.Verbas[1].Referencia, "em dobro") ||
		r.Verbas[1].Descricao != "Férias vencidas (período de 11/03/2024)" {
		t.Fatalf("em dobro = %s %+v", r.TotalVerbas, r.Verbas)
	}
	// Um ano exato de casa já dá o seguro.
	e = base
	e.Tipo, e.Aviso, e.Admissao = SemJustaCausa, AvisoIndenizado, dia(2025, 12, 15)
	if r := Calcular(e); !r.SeguroDesemprego {
		t.Fatal("um ano de casa dá o seguro")
	}
	// A 1ª parcela que veio nunca passa do 13º; e o seguro pede 12 meses.
	e = base
	e.Tipo, e.Aviso, e.Avos13, e.Admissao = SemJustaCausa, AvisoTrabalhado, 1, dia(2026, 1, 2)
	r = Calcular(e)
	if r.SeguroDesemprego || r.Descontos[len(r.Descontos)-1].Valor.String() != "500.00" || r.DiasDeAviso != 30 {
		t.Fatalf("1ª parcela = %+v, seguro %v", r.Descontos, r.SeguroDesemprego)
	}
}

type mux struct{ *http.ServeMux }

func (m mux) HandleFunc(p string, h http.HandlerFunc) { m.ServeMux.HandleFunc(p, h) }

func TestRescisaoPelaApi(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	empregos := emprego.NovoServico(db, rel, log)
	holerites := holerite.NovoServico(db, empregos, rel, log)
	decimo := decimoterceiro.NovoServico(db, empregos, holerites, rel, log)
	asFerias := ferias.NovoServico(db, empregos, holerites, rel, log)
	oFgts := fgts.NovoServico(db, empregos, holerites, decimo, asFerias, rel, log)
	m := mux{http.NewServeMux()}
	emprego.Registrar(m, empregos)
	holerite.Registrar(m, holerites)
	ferias.Registrar(m, asFerias)
	RegistrarRotas(m, NovoServico(db, empregos, holerites, decimo, asFerias, oFgts, rel, log))
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, m, metodo, caminho, corpo)
	}
	criar := func(corpo string) int64 {
		var e emprego.Emprego
		_ = json.Unmarshal(chamar("POST", "/api/trabalho/empregos", corpo).Corpo, &e)
		return e.Id
	}
	ler := func(r testeapoio.Resposta, status int) Calculo {
		t.Helper()
		r.ConferirStatus(t, status)
		var c Calculo
		if err := json.Unmarshal(r.Corpo, &c); err != nil {
			t.Fatalf("%s: %v", r.Corpo, err)
		}
		return c
	}

	id := criar(`{"empregador":"E","admissao":"2024-03-11","salarioBruto":6000}`)
	base := fmt.Sprintf("/api/trabalho/empregos/%d/rescisao", id)
	c := ler(chamar("GET", base+"/simular?tipo=1&aviso=2&saida=2026-12-15", ""), http.StatusOK)
	// O 1º período venceu em 10/03/2026 sem tirar (em dobro: 12000 + 4000); o 2º, simples (6000 + 2000); o 3º, 10/12 até
	// a data projetada (20/01/2027). A 1ª parcela do 13º (3000, de 30/11) já veio. O resto como no caso "sem justa causa":
	// 30866,67 + 16000 = 46866,67 de verbas, 4275,21 de descontos.
	if c.TotalVerbas.String() != "46866.67" || c.Liquido.String() != "42591.46" || c.DataProjetada.String() != "2027-01-20" ||
		!c.Fgts.Saldo.Decimal().IsPositive() || c.Fgts.PercentualMulta != 40 || !c.SeguroDesemprego {
		t.Fatalf("simulação = %s, %s, %s\n%+v", c.TotalVerbas, c.Liquido, c.DataProjetada, c.Verbas)
	}

	// Saindo antes de 30/11, a 1ª parcela do 13º ainda não veio: não desconta.
	nov := ler(chamar("GET", base+"/simular?tipo=1&aviso=2&saida=2026-11-20", ""), http.StatusOK)
	for _, d := range nov.Descontos {
		if strings.HasPrefix(d.Descricao, "1ª parcela") {
			t.Fatalf("1ª parcela descontada antes de vir: %+v", nov.Descontos)
		}
	}
	r := chamar("GET", base+"/comparar?saida=2026-12-15", "")
	r.ConferirStatus(t, http.StatusOK)
	var lista []Comparacao
	_ = json.Unmarshal(r.Corpo, &lista)
	if len(lista) != 5 || lista[0].Liquido.String() != "42591.46" || lista[1].Aviso != AvisoTrabalhado || lista[3].Aviso != SemAviso {
		t.Fatalf("comparação = %+v", lista)
	}

	// Férias agendadas num período vencido (14 dias e 5 vendidos): sobram 11, em dobro (6000 x 11/30 x 2 = 4400).
	chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/ferias", id), `{"periodoInicio":"2024-03-11","inicio":"2025-04-01","dias":14,"diasVendidos":5}`).
		ConferirStatus(t, http.StatusOK)
	c = ler(chamar("GET", base+"/simular?tipo=4&saida=2026-12-15", ""), http.StatusOK)
	if c.Verbas[1].Valor.String() != "4400.00" || c.Verbas[1].Referencia != "11 dias, em dobro" {
		t.Fatalf("vencidas com férias tiradas = %+v", c.Verbas)
	}
	// A média das horas extras dos 12 meses antes entra na remuneração: outro emprego, com um holerite de outubro (10 h
	// a 50%: 409,09 e o DSR de 60,61 = 469,70 / 12 = 39,14).
	outro := criar(`{"empregador":"O","admissao":"2024-03-11","salarioBruto":6000}`)
	var h holerite.Holerite
	_ = json.Unmarshal(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", outro), `{"mes":"2026-10"}`).Corpo, &h)
	chamar("PUT", fmt.Sprintf("/api/trabalho/holerites/%d", h.Id), `{"diasUteis":27,"diasDescanso":4,"lancamentos":[{"tipo":1,"quantidade":10,"percentual":50}]}`).
		ConferirStatus(t, http.StatusOK)
	if c := ler(chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/rescisao/simular?tipo=4&saida=2026-12-15", outro), ""), http.StatusOK); c.Remuneracao.String() != "6039.14" {
		t.Fatalf("remuneração com a média = %s", c.Remuneracao)
	}

	// Registrar fecha o emprego; o registro fica congelado; não registra duas vezes; desfazer reabre.
	chamar("GET", base, "").ConferirJSON(t, `{"registrada":null}`)
	reg := ler(chamar("POST", base, `{"tipo":3,"aviso":2,"saida":"2026-12-15"}`), http.StatusCreated)
	e, _ := empregos.Obter(t.Context(), id)
	if reg.Tipo != Acordo || e.Saida == nil || e.Saida.String() != "2026-12-15" {
		t.Fatalf("registrada = %+v, saída %v", reg.Tipo, e.Saida)
	}
	var guardada struct {
		Registrada *Calculo `json:"registrada"`
	}
	_ = json.Unmarshal(chamar("GET", base, "").Corpo, &guardada)
	if guardada.Registrada == nil || guardada.Registrada.Liquido.String() != reg.Liquido.String() {
		t.Fatalf("guardada = %+v", guardada.Registrada)
	}
	r = chamar("POST", base, `{"tipo":1,"aviso":2,"saida":"2026-12-15"}`)
	r.ConferirStatus(t, http.StatusConflict)
	r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, MensagemJaRegistrada))
	chamar("DELETE", base, "").ConferirStatus(t, http.StatusNoContent)
	if e, _ := empregos.Obter(t.Context(), id); e.Saida != nil {
		t.Fatalf("desfeito, a saída ficou: %v", e.Saida)
	}
	chamar("DELETE", base, "").ConferirStatus(t, http.StatusNotFound)

	pj := criar(`{"empregador":"P","admissao":"2020-01-01","salarioBruto":9000,"regime":3}`)
	casos := []struct {
		nome, metodo, caminho, corpo string
		status                       int
		mensagem                     string
	}{
		{"tipo desconhecido", "GET", base + "/simular?tipo=9&aviso=2&saida=2026-12-15", "", 400, MensagemTipo},
		{"tipo zero", "GET", base + "/simular?tipo=0&aviso=2&saida=2026-12-15", "", 400, MensagemTipo},
		{"dispensa sem aviso", "GET", base + "/simular?tipo=1&saida=2026-12-15", "", 400, MensagemAviso},
		{"aviso desconhecido", "GET", base + "/simular?tipo=2&aviso=3&saida=2026-12-15", "", 400, MensagemAviso},
		{"aviso negativo", "GET", base + "/simular?tipo=2&aviso=-1&saida=2026-12-15", "", 400, MensagemAviso},
		{"antes da admissão", "GET", base + "/simular?tipo=4&saida=2024-03-10", "", 400, MensagemSaida},
		{"sem tabela (março de 2025)", "GET", base + "/simular?tipo=4&saida=2025-03-20", "", 409, MensagemSemTabela},
		{"sem o tipo", "GET", base + "/simular?saida=2026-12-15", "", 400, "Informe o tipo e a data de saída (AAAA-MM-DD)."},
		{"sem a data", "GET", base + "/simular?tipo=1&aviso=2", "", 400, "Informe o tipo e a data de saída (AAAA-MM-DD)."},
		{"comparar sem a data", "GET", base + "/comparar", "", 400, "Informe a data de saída (AAAA-MM-DD)."},
		{"PJ", "GET", fmt.Sprintf("/api/trabalho/empregos/%d/rescisao/simular?tipo=1&aviso=2&saida=2026-12-15", pj), "", 400, MensagemSemRescisao},
		{"comparar no PJ", "GET", fmt.Sprintf("/api/trabalho/empregos/%d/rescisao/comparar?saida=2026-12-15", pj), "", 400, MensagemSemRescisao},
		{"registrar sem a data", "POST", base, `{"tipo":1,"aviso":2}`, 400, MensagemJSONInvalido},
		{"registrar com JSON quebrado", "POST", base, `{"tipo":`, 400, MensagemJSONInvalido},
		{"registrar tipo errado", "POST", base, `{"tipo":7,"aviso":2,"saida":"2026-12-15"}`, 400, MensagemTipo},
	}
	for _, k := range casos {
		t.Run(k.nome, func(t *testing.T) {
			r := chamar(k.metodo, k.caminho, k.corpo)
			r.ConferirStatus(t, k.status)
			r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, k.mensagem))
		})
	}
	// Justa causa ignora o aviso que vier.
	jc := ler(chamar("GET", base+"/simular?tipo=4&aviso=2&saida=2026-12-15", ""), http.StatusOK)
	if jc.Aviso != SemAviso {
		t.Fatalf("justa causa com aviso %d", jc.Aviso)
	}
	chamar("GET", "/api/trabalho/empregos/99/rescisao", "").ConferirStatus(t, http.StatusNotFound)
	chamar("GET", "/api/trabalho/empregos/99/rescisao/simular?tipo=1&aviso=2&saida=2026-12-15", "").ConferirStatus(t, http.StatusNotFound)
}
