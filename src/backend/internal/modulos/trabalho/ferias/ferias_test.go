package ferias

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

var v = testeapoio.Valor

func TestDiasDeDireito(t *testing.T) {
	t.Parallel()
	for faltas, dias := range map[int]int{0: 30, 5: 30, 6: 24, 14: 24, 15: 18, 23: 18, 24: 12, 32: 12, 33: 0, 200: 0} {
		if got := DiasDeDireito(faltas); got != dias {
			t.Errorf("%d faltas: %d dias; quer %d", faltas, got, dias)
		}
	}
}

func TestPeriodos(t *testing.T) {
	t.Parallel()
	hoje := testeapoio.Dia(2026, 11, 13)
	p := Periodos(testeapoio.Dia(2024, 3, 11), nil, hoje)
	if len(p) != 3 {
		t.Fatalf("períodos = %+v", p)
	}
	if p[0].Fim.String() != "2025-03-10" || p[0].FimConcessivo.String() != "2026-03-10" || !p[0].Completo || p[2].Completo {
		t.Fatalf("períodos = %+v", p)
	}
	// Saiu antes do terceiro período começar.
	saida := testeapoio.Dia(2026, 1, 31)
	if p := Periodos(testeapoio.Dia(2024, 3, 11), &saida, hoje); len(p) != 2 {
		t.Fatalf("com saída: %+v", p)
	}
	// O período que começa hoje já conta (em aquisição).
	if p := Periodos(hoje, nil, hoje); len(p) != 1 || p[0].Completo {
		t.Fatalf("começa hoje: %+v", p)
	}
	// Admissão em 29/02 (revisão do #701): o aniversário é 01/03 nos anos comuns e volta a 29/02 nos bissextos.
	p = Periodos(testeapoio.Dia(2024, 2, 29), nil, testeapoio.Dia(2028, 3, 1))
	var got []string
	for _, x := range p {
		got = append(got, x.Inicio.String()+".."+x.Fim.String()+"/"+x.FimConcessivo.String())
	}
	quer := []string{"2024-02-29..2025-02-28/2026-02-28", "2025-03-01..2026-02-28/2027-02-28", "2026-03-01..2027-02-28/2028-02-28",
		"2027-03-01..2028-02-28/2029-02-28", "2028-02-29..2029-02-28/2030-02-28"}
	if fmt.Sprint(got) != fmt.Sprint(quer) {
		t.Fatalf("29/02:\n%v\nquer\n%v", got, quer)
	}
}

func TestDiasEmDobro(t *testing.T) {
	t.Parallel()
	fim := testeapoio.Dia(2026, 3, 10)
	casos := []struct {
		inicio tipos.Data
		dias   int
		quer   int
	}{
		{testeapoio.Dia(2026, 2, 9), 30, 0},  // acaba no último dia do concessivo
		{testeapoio.Dia(2026, 2, 10), 30, 1}, // um dia depois
		{testeapoio.Dia(2026, 3, 1), 20, 10},
		{testeapoio.Dia(2026, 3, 11), 30, 30}, // começa depois: tudo em dobro
		{testeapoio.Dia(2026, 3, 10), 5, 4},
	}
	for _, c := range casos {
		if got := DiasEmDobro(c.inicio, c.dias, fim); got != c.quer {
			t.Errorf("DiasEmDobro(%s, %d) = %d; quer %d", c.inicio, c.dias, got, c.quer)
		}
	}
}

func TestValidarFracoes(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome                                   string
		existentes                             []int
		vendidosAntes, direito, dias, vendidos int
		mensagem                               string
	}{
		{"30 dias de uma vez", nil, 0, 30, 30, 0, ""},
		{"20 e vende 10", nil, 0, 30, 20, 10, ""},
		{"14 agora, o resto depois", nil, 0, 30, 14, 0, ""},
		{"10 agora, ainda cabe uma de 14", nil, 0, 30, 10, 0, ""},
		{"a 3ª fração com uma de 14 antes", []int{14, 5}, 0, 30, 11, 0, ""},
		{"menos de 5 dias", nil, 0, 30, 4, 0, MensagemFracoes},
		{"a 4ª fração", []int{14, 5, 5}, 0, 30, 5, 0, MensagemFracoes},
		{"sem nenhuma de 14 e sem espaço para ela", []int{10, 5}, 0, 30, 10, 0, MensagemFracoes},
		{"sem 14 e a sobra não cabe uma de 14", []int{10}, 0, 30, 10, 0, MensagemFracoes},
		{"vender mais de 1/3", nil, 0, 30, 19, 11, "Dá para vender até 1/3 dos dias de direito."},
		{"vender de novo passa de 1/3", []int{20}, 10, 30, 0 + 5, 1, "Dá para vender até 1/3 dos dias de direito."},
		{"vender negativo", nil, 0, 30, 20, -1, "Dá para vender até 1/3 dos dias de direito."},
		{"passa do direito", []int{20}, 0, 30, 11, 0, "Os dias passam do que sobra de férias neste período."},
		{"com 24 de direito, 16 e vende 8", nil, 0, 24, 16, 8, ""},
		{"duas frações antes somam", []int{14, 5}, 0, 30, 12, 0, "Os dias passam do que sobra de férias neste período."},
		{"10 e vende 10: não sobra uma de 14", nil, 0, 30, 10, 10, MensagemFracoes},
		{"10 e vende 6: sobram 14", nil, 0, 30, 10, 6, ""},
		{"vendeu antes e não sobra uma de 14", []int{10}, 10, 30, 5, 0, MensagemFracoes},
		{"a 3ª fração sem nenhuma de 14", []int{5, 5}, 0, 30, 5, 0, MensagemFracoes},
		{"sem direito", nil, 0, 0, 14, 0, "Com mais de 32 faltas no período, não há férias (CLT, art. 130)."},
		// Revisão do #701: os 14 dias só valem quando há fracionamento.
		{"12 de direito num bloco só", nil, 0, 12, 12, 0, ""},
		{"18 de direito, vende 6 e tira 12 de uma vez", nil, 0, 18, 12, 6, ""},
		{"12 de direito em duas frações", nil, 0, 12, 6, 0, MensagemFracoes},
		{"a 2ª fração que fecha o direito sem nenhuma de 14", []int{6}, 0, 12, 6, 0, MensagemFracoes},
		// Revisão do #701 (2ª): dias e vendidos enormes estouravam a soma e passavam.
		{"dias enormes com uma fração antes", []int{12}, 0, 30, math.MaxInt - 5, 0, "Os dias passam do que sobra de férias neste período."},
		{"dias no direito, sem fração antes", nil, 0, 30, 31, 0, "Os dias passam do que sobra de férias neste período."},
		{"vendidos enormes com venda antes", []int{14}, 5, 30, 5, math.MaxInt - 2, "Dá para vender até 1/3 dos dias de direito."},
		{"vende o direito inteiro", nil, 0, 9, 5, 10, "Dá para vender até 1/3 dos dias de direito."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			if got := ValidarFracoes(c.existentes, c.vendidosAntes, c.direito, c.dias, c.vendidos); got != c.mensagem {
				t.Fatalf("mensagem = %q; quer %q", got, c.mensagem)
			}
		})
	}
}

func tabelas(t *testing.T) (encargos.TabelaInss, encargos.TabelaIrrf) {
	t.Helper()
	db := testeapoio.NovoBanco(t)
	inss, _ := encargos.LerTabelasInss(t.Context(), db)
	irrf, _ := encargos.LerTabelasIrrf(t.Context(), db)
	ti, _ := encargos.Vigente(inss, testeapoio.Dia(2026, 12, 1))
	tr, _ := encargos.Vigente(irrf, testeapoio.Dia(2026, 12, 1))
	return ti, tr
}

func TestCalcularFerias(t *testing.T) {
	t.Parallel()
	ti, tr := tabelas(t)
	base := Entrada{Inicio: testeapoio.Dia(2026, 12, 1), SalarioBase: v("6000.00"), Inss: ti, Irrf: tr}
	casos := []struct {
		nome   string
		ajuste func(*Entrada)
		quer   string // férias, 1/3, abono, 1/3 do abono, INSS, IRRF, 13º adiantado, líquido
	}{
		// 6000 + 2000 = 8000: INSS 921,51; IRRF 8000 - 921,51 = 7078,49 x 27,5% - 908,73 = 1037,85 (acima de 7350, sem a redução).
		{"30 dias", func(e *Entrada) { e.Dias = 30 }, "6000.00 2000.00 0.00 0.00 921.51 1037.85 0.00 6040.64"},
		// 20 dias: 4000 + 1333,33 = 5333,33 (INSS 548,18; IRRF pelo simplificado: 4726,13 x 27,5% - 908,73 = 390,96, menos a
		// redução de 268,51 = 122,45). O abono (2000 + 666,67) fica sem INSS e sem IR.
		{"20 dias e vende 10", func(e *Entrada) { e.Dias, e.DiasVendidos = 20, 10 }, "4000.00 1333.33 2000.00 666.67 548.18 122.45 0.00 7329.37"},
		// O 13º adiantado: metade da remuneração (3000), sem desconto.
		{"com o 13º junto", func(e *Entrada) { e.Dias, e.DiasVendidos, e.AdiantarDecimo = 20, 10, true },
			"4000.00 1333.33 2000.00 666.67 548.18 122.45 3000.00 10329.37"},
		// Média de 1200 / 12 = 100: remuneração 6100; 15 dias = 3050 + 1016,67 = 4066,67; INSS 121,575 + 115,3656 + 1163,83 x 12%
		// (139,6596) = 376,6002; IRRF isento pela redução (rendimento até 5000).
		{"com a média das horas extras", func(e *Entrada) { e.Dias, e.SomaVariaveis = 15, v("1200.00") },
			"3050.00 1016.67 0.00 0.00 376.60 0.00 0.00 3690.07"},
		// Revisão do #701: 10 dos 30 dias em dobro. Férias 6000 x 40/30 = 8000 + 1/3 = 10666,67. A dobra (2000 + 666,67) fica
		// fora do INSS (Lei 8.212, art. 28, § 9º, "d"): base 8000, INSS 921,51, como os 30 dias simples. O IRRF sobre o todo:
		// 10666,67 - 921,51 = 9745,16 x 27,5% - 908,73 = 1771,19.
		{"com 10 dias em dobro", func(e *Entrada) { e.Dias, e.DiasEmDobro = 30, 10 }, "8000.00 2666.67 0.00 0.00 921.51 1771.19 0.00 7973.97"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			e := base
			c.ajuste(&e)
			r := Calcular(e)
			got := fmt.Sprintf("%s %s %s %s %s %s %s %s", r.Ferias, r.TercoFerias, r.Abono, r.TercoAbono, r.Inss.Valor, r.Irrf.Valor,
				r.AdiantamentoDecimo, r.Liquido)
			if got != c.quer {
				t.Fatalf("férias = %s\n         quer %s", got, c.quer)
			}
		})
	}
	r := Calcular(Entrada{Inicio: testeapoio.Dia(2026, 12, 1), Dias: 30, SalarioBase: v("6000.00"), Inss: ti, Irrf: tr})
	if r.Pagamento.String() != "2026-11-29" || r.Retorno.String() != "2026-12-31" {
		t.Fatalf("pagamento %s, retorno %s", r.Pagamento, r.Retorno)
	}
}

type mux struct{ *http.ServeMux }

func (m mux) HandleFunc(p string, h http.HandlerFunc) { m.ServeMux.HandleFunc(p, h) }

func TestFeriasPelaApi(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	empregos := emprego.NovoServico(db, rel, log)
	holerites := holerite.NovoServico(db, empregos, rel, log)
	m := mux{http.NewServeMux()}
	emprego.Registrar(m, empregos)
	holerite.Registrar(m, holerites)
	Registrar(m, NovoServico(db, empregos, holerites, rel, log))
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, m, metodo, caminho, corpo)
	}
	criar := func(corpo string) int64 {
		var e emprego.Emprego
		_ = json.Unmarshal(chamar("POST", "/api/trabalho/empregos", corpo).Corpo, &e)
		return e.Id
	}
	periodos := func(r testeapoio.Resposta) []PeriodoComFerias {
		t.Helper()
		r.ConferirStatus(t, http.StatusOK)
		var p []PeriodoComFerias
		if err := json.Unmarshal(r.Corpo, &p); err != nil {
			t.Fatalf("%s: %v", r.Corpo, err)
		}
		return p
	}

	id := criar(`{"empregador":"E","admissao":"2024-03-11","salarioBruto":6000}`)
	base := fmt.Sprintf("/api/trabalho/empregos/%d/ferias", id)
	p := periodos(chamar("GET", base, ""))
	// O primeiro período venceu (o concessivo acabou em 10/03/2026 com 30 dias); o segundo está para tirar; o terceiro, em aquisição.
	if len(p) != 3 || !p[0].Vencida || p[0].Saldo != 30 || p[1].Vencida || p[1].Vencendo || p[2].Completo {
		t.Fatalf("períodos = %+v", p)
	}
	p = periodos(chamar("POST", base, `{"periodoInicio":"2025-03-11","inicio":"2026-11-03","dias":20,"diasVendidos":10,"adiantarDecimo":true}`))
	a := p[1].Agendamentos
	if len(a) != 1 || p[1].Saldo != 0 || a[0].Calculo == nil || a[0].Calculo.Liquido.String() != "10329.37" || a[0].Calculo.DiasEmDobro != 0 {
		t.Fatalf("agendado = %+v", p[1])
	}
	// Revisão do #701: as faltas não deixam o direito menor que o já agendado.
	r := chamar("PUT", base+"/faltas", `{"periodoInicio":"2025-03-11","faltas":10}`)
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, `{"mensagem":"Com 10 faltas, o direito cai para 24 dias, menos que os 30 já agendados: desmarque férias antes."}`)
	// As faltas reduzem o direito.
	p = periodos(chamar("PUT", base+"/faltas", `{"periodoInicio":"2024-03-11","faltas":10}`))
	if p[0].Faltas != 10 || p[0].DiasDireito != 24 || p[0].Saldo != 24 {
		t.Fatalf("faltas = %+v", p[0])
	}
	// Desmarcar devolve o saldo.
	p = periodos(chamar("DELETE", fmt.Sprintf("%s/%d", base, a[0].Id), ""))
	if p[1].Saldo != 30 || len(p[1].Agendamentos) != 0 {
		t.Fatalf("desmarcado = %+v", p[1])
	}

	casos := []struct {
		nome, metodo, caminho, corpo string
		status                       int
		mensagem                     string
	}{
		{"antes do fim do período", "POST", base, `{"periodoInicio":"2025-03-11","inicio":"2026-03-10","dias":30}`, 400, MensagemAntes},
		{"período que não existe", "POST", base, `{"periodoInicio":"2025-03-12","inicio":"2026-12-01","dias":30}`, 400, MensagemPeriodo},
		{"sem o início", "POST", base, `{"periodoInicio":"2025-03-11","dias":30}`, 400, "Informe o período e o dia do início."},
		{"fração curta", "POST", base, `{"periodoInicio":"2025-03-11","inicio":"2026-12-01","dias":4}`, 400, MensagemFracoes},
		{"13º adiantado com o pagamento depois de 30/11", "POST", base,
			`{"periodoInicio":"2025-03-11","inicio":"2026-12-03","dias":30,"adiantarDecimo":true}`, 400,
			"A 1ª parcela do 13º vence em 30/11, e estas férias são pagas depois (2 dias antes do início): ela não vem com elas."},
		{"13º adiantado com o pagamento no ano anterior", "POST", base,
			`{"periodoInicio":"2025-03-11","inicio":"2027-01-02","dias":30,"adiantarDecimo":true}`, 400,
			"As férias que começam em 1º ou 2 de janeiro são pagas no ano anterior: a 1ª parcela do 13º não vem com elas."},
		{"JSON quebrado", "POST", base, `{"dias":`, 400, MensagemJSONInvalido},
		{"sem tabela (dezembro de 2024)", "GET", base + "/simular?inicio=2024-12-01&dias=30", "", 409, MensagemSemTabela},
		{"simular sem os dias", "GET", base + "/simular?inicio=2026-12-01", "", 400, MensagemSimular},
		{"simular vendendo 11", "GET", base + "/simular?inicio=2026-12-01&dias=19&vendidos=11", "", 400, MensagemSimular},
		{"simular 31 dias", "GET", base + "/simular?inicio=2026-12-01&dias=31", "", 400, MensagemSimular},
		{"simular data errada", "GET", base + "/simular?inicio=01/12/2026&dias=30", "", 400, MensagemSimular},
		{"simular vendidos que não é número", "GET", base + "/simular?inicio=2026-12-01&dias=20&vendidos=abc", "", 400, MensagemSimular},
		{"simular dias e vendidos passam de 30", "GET", base + "/simular?inicio=2026-12-01&dias=25&vendidos=6", "", 400, MensagemSimular},
		{"faltas negativas", "PUT", base + "/faltas", `{"periodoInicio":"2024-03-11","faltas":-1}`, 400, "As faltas vão de 0 a 366."},
		{"faltas demais", "PUT", base + "/faltas", `{"periodoInicio":"2024-03-11","faltas":367}`, 400, "As faltas vão de 0 a 366."},
		{"faltas sem período", "PUT", base + "/faltas", `{"faltas":3}`, 400, MensagemPeriodo},
		{"faltas de período que não existe", "PUT", base + "/faltas", `{"periodoInicio":"2020-01-01","faltas":3}`, 400, MensagemPeriodo},
		{"faltas com JSON quebrado", "PUT", base + "/faltas", `{"faltas":`, 400, MensagemJSONInvalido},
		{"antes do salário", "GET", base + "/simular?inicio=2024-01-02&dias=30", "", 400, "Não há salário no mês do início."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := chamar(c.metodo, c.caminho, c.corpo)
			r.ConferirStatus(t, c.status)
			r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, c.mensagem))
		})
	}

	// Simular: 30 dias × vender 10, os limites aceitos.
	var c30, c20 Calculo
	_ = json.Unmarshal(chamar("GET", base+"/simular?inicio=2026-12-01&dias=30", "").Corpo, &c30)
	_ = json.Unmarshal(chamar("GET", base+"/simular?inicio=2026-12-01&dias=20&vendidos=10&adiantar=true", "").Corpo, &c20)
	if c30.Liquido.String() != "6040.64" || c20.Liquido.String() != "10329.37" {
		t.Fatalf("simulações: %s, %s", c30.Liquido, c20.Liquido)
	}
	chamar("GET", base+"/simular?inicio=2026-12-01&dias=1", "").ConferirStatus(t, http.StatusOK)
	chamar("GET", base+"/simular?inicio=2026-12-01&dias=20&vendidos=10", "").ConferirStatus(t, http.StatusOK)
	chamar("PUT", base+"/faltas", `{"periodoInicio":"2024-03-11","faltas":366}`).ConferirStatus(t, http.StatusOK)
	chamar("PUT", base+"/faltas", `{"periodoInicio":"2024-03-11","faltas":0}`).ConferirStatus(t, http.StatusOK)

	// Duas frações no mesmo período somam; o período todo tirado não fica vencido, mesmo com o prazo passado.
	chamar("POST", base, `{"periodoInicio":"2024-03-11","inicio":"2025-04-01","dias":14,"diasVendidos":5,"adiantarDecimo":true}`).
		ConferirStatus(t, http.StatusOK)
	// Revisão do #701: a fração que cruza com outra (até no último dia) não passa, nem a 2ª 1ª parcela do 13º no ano.
	r = chamar("POST", base, `{"periodoInicio":"2024-03-11","inicio":"2025-03-22","dias":11}`)
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, `{"mensagem":"Essas férias cruzam com as de 01/04/2025 a 14/04/2025: escolha outras datas."}`)
	r = chamar("POST", base, `{"periodoInicio":"2024-03-11","inicio":"2025-07-01","dias":11,"adiantarDecimo":true}`)
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, `{"mensagem":"A 1ª parcela do 13º de 2025 já vem com as férias de 01/04/2025."}`)
	// A que acaba na véspera passa, e sem o 13º ela não esbarra no 13º do ano; os vendidos das duas somam.
	p = periodos(chamar("POST", base, `{"periodoInicio":"2024-03-11","inicio":"2025-03-21","dias":10,"diasVendidos":1}`))
	if p[0].DiasGozados != 24 || p[0].DiasVendidos != 6 || p[0].Saldo != 0 || p[0].Vencida || len(p[0].Agendamentos) != 2 {
		t.Fatalf("período tirado = %+v", p[0])
	}
	// A fração depois do fim do concessivo (10/03/2027) sai em dobro (CLT, art. 137): de 11 a 30/03, 20 dias. O 13º
	// adiantado em outro ano passa.
	p = periodos(chamar("POST", base, `{"periodoInicio":"2025-03-11","inicio":"2027-03-01","dias":30,"adiantarDecimo":true}`))
	if c := p[1].Agendamentos[0].Calculo; c == nil || c.DiasEmDobro != 20 || c.Ferias.String() != "10000.00" || c.AdiantamentoDecimo.String() != "3000.00" {
		t.Fatalf("em dobro = %+v", c)
	}
	// Março de 2025 tem tabela do INSS, mas não do IRRF (desde maio de 2025).
	r = chamar("GET", base+"/simular?inicio=2025-03-20&dias=30", "")
	r.ConferirStatus(t, http.StatusConflict)
	r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, MensagemSemTabela))

	// A média: um holerite de outubro de 2026 com 10 h extras a 50% (409,09) e o DSR (60,61): 469,70 / 12 = 39,14.
	var h holerite.Holerite
	_ = json.Unmarshal(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", id), `{"mes":"2026-10"}`).Corpo, &h)
	chamar("PUT", fmt.Sprintf("/api/trabalho/holerites/%d", h.Id), `{"diasUteis":27,"diasDescanso":4,"lancamentos":[{"tipo":1,"quantidade":10,"percentual":50}]}`).
		ConferirStatus(t, http.StatusOK)
	var comMedia Calculo
	_ = json.Unmarshal(chamar("GET", base+"/simular?inicio=2026-12-01&dias=30", "").Corpo, &comMedia)
	if comMedia.MediaVariaveis.String() != "39.14" {
		t.Fatalf("média = %s", comMedia.MediaVariaveis)
	}

	// Férias vencendo: o concessivo do primeiro período acaba em 30/11/2026, a menos de 60 dias.
	vencendo := criar(`{"empregador":"V","admissao":"2024-12-01","salarioBruto":3000}`)
	p = periodos(chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/ferias", vencendo), ""))
	if !p[0].Vencendo || p[0].Vencida {
		t.Fatalf("vencendo = %+v", p[0])
	}
	// PJ e emprego que não existe.
	pj := criar(`{"empregador":"P","admissao":"2020-01-01","salarioBruto":9000,"regime":3}`)
	r = chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/ferias", pj), "")
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, MensagemSemFerias))
	chamar("GET", "/api/trabalho/empregos/99/ferias", "").ConferirStatus(t, http.StatusNotFound)
	// Férias depois da saída não se agendam: as que sobraram vêm na rescisão.
	saiu := criar(`{"empregador":"S","admissao":"2024-01-02","saida":"2026-06-30","salarioBruto":3000}`)
	r = chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/ferias", saiu), `{"periodoInicio":"2025-01-02","inicio":"2026-07-01","dias":30}`)
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, `{"mensagem":"As férias começam depois da saída do emprego: as que sobraram vêm na rescisão."}`)
	chamar("DELETE", base+"/99", "").ConferirStatus(t, http.StatusNotFound)
	chamar("DELETE", base+"/x", "").ConferirStatus(t, http.StatusNotFound)
}

// Revisão do #701 (2ª): a 1ª parcela do 13º vem com as férias pagas (2 dias antes do início) até 30/11 e no mesmo ano.
func TestConferirAgendaPeloPagamento(t *testing.T) {
	t.Parallel()
	s := &Servico{}
	casos := map[string]bool{"2026-12-01": true, "2026-12-02": true, "2026-12-03": false, "2027-01-02": false, "2027-01-03": true}
	for inicio, pode := range casos {
		d, _ := tipos.LerData(inicio)
		if got := s.conferirAgenda(nil, d, 30, true) == ""; got != pode {
			t.Errorf("início em %s: pode = %v, quero %v", inicio, got, pode)
		}
		if s.conferirAgenda(nil, d, 30, false) != "" {
			t.Errorf("início em %s, sem o 13º: recusou", inicio)
		}
	}
}

// Revisão do #701 (2ª): o "Ver a conta" mostra a dobra do período (como o agendado); as faltas novas não deixam a venda
// passar de 1/3 do direito; e o 13º pede às férias a 1ª parcela que veio com elas, no valor que elas pagaram.
func TestSimularFaltasEAdiantamento(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	empregos := emprego.NovoServico(db, rel, log)
	holerites := holerite.NovoServico(db, empregos, rel, log)
	s := NovoServico(db, empregos, holerites, rel, log)
	m := mux{http.NewServeMux()}
	emprego.Registrar(m, empregos)
	Registrar(m, s)
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, m, metodo, caminho, corpo)
	}
	var e emprego.Emprego
	_ = json.Unmarshal(chamar("POST", "/api/trabalho/empregos", `{"empregador":"E","admissao":"2024-03-11","salarioBruto":6000}`).Corpo, &e)
	base := fmt.Sprintf("/api/trabalho/empregos/%d/ferias", e.Id)

	// O 1º período (concessivo até 10/03/2026) simulado em 01/12/2026: os 20 dias em dobro; sem o período, simples.
	var dobro, simples Calculo
	_ = json.Unmarshal(chamar("GET", base+"/simular?periodo=2024-03-11&inicio=2026-12-01&dias=20", "").Corpo, &dobro)
	_ = json.Unmarshal(chamar("GET", base+"/simular?inicio=2026-12-01&dias=20", "").Corpo, &simples)
	if dobro.DiasEmDobro != 20 || simples.DiasEmDobro != 0 || dobro.Ferias.String() != "8000.00" {
		t.Fatalf("simular: dobro %+v, simples %+v", dobro, simples)
	}
	var outro Calculo
	_ = json.Unmarshal(chamar("GET", base+"/simular?periodo=2020-01-01&inicio=2026-12-01&dias=20", "").Corpo, &outro)
	if outro.DiasEmDobro != 0 {
		t.Fatalf("período que não existe: %+v", outro)
	}

	// 5 dias e 10 vendidos no 2º período: com 6 faltas o direito cai para 24, e a venda vai até 8.
	chamar("POST", base, `{"periodoInicio":"2025-03-11","inicio":"2026-07-15","dias":5,"diasVendidos":10,"adiantarDecimo":true}`).
		ConferirStatus(t, http.StatusOK)
	r := chamar("PUT", base+"/faltas", `{"periodoInicio":"2025-03-11","faltas":6}`)
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, `{"mensagem":"Com 6 faltas, o direito cai para 24 dias, e a venda vai até 8 (1/3), menos que os 10 já vendidos: desmarque férias antes."}`)
	chamar("PUT", base+"/faltas", `{"periodoInicio":"2025-03-11","faltas":5}`).ConferirStatus(t, http.StatusOK)
	// Uma fração antes, no mesmo período, sem o 13º: a de julho, com ele, ainda é a que vale.
	chamar("POST", base, `{"periodoInicio":"2025-03-11","inicio":"2026-04-01","dias":14}`).ConferirStatus(t, http.StatusOK)

	// O aumento para 7000 em setembro não muda o que as férias de julho pagaram de 1ª parcela (3000, a metade de 6000).
	chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/salarios", e.Id), `{"inicio":"2026-09-01","salarioBruto":7000}`).
		ConferirStatus(t, http.StatusOK)
	pagamento, valor, err := s.AdiantamentoDoAno(t.Context(), e.Id, 2026)
	if err != nil || pagamento == nil || pagamento.String() != "2026-07-13" || valor == nil || valor.String() != "3000.00" {
		t.Fatalf("adiantamento de 2026: %v %v %v", pagamento, valor, err)
	}
	if p, v, err := s.AdiantamentoDoAno(t.Context(), e.Id, 2025); p != nil || v != nil || err != nil {
		t.Fatalf("2025, sem férias com o 13º: %v %v %v", p, v, err)
	}
	// Sem a tabela do mês (dezembro de 2024), vem só a data.
	chamar("POST", base, `{"periodoInicio":"2024-03-11","inicio":"2024-09-10","dias":30}`) // antes do fim do período: recusado
	pj := emprego.Emprego{}
	_ = json.Unmarshal(chamar("POST", "/api/trabalho/empregos", `{"empregador":"P","admissao":"2020-01-01","salarioBruto":9000,"regime":3}`).Corpo, &pj)
	if p, v, err := s.AdiantamentoDoAno(t.Context(), pj.Id, 2026); p != nil || v != nil || err != nil {
		t.Fatalf("PJ: %v %v %v", p, v, err)
	}
	if _, _, err := s.AdiantamentoDoAno(t.Context(), 999, 2026); err == nil {
		t.Fatal("emprego que não existe: sem erro")
	}
}
