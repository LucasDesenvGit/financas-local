package decimoterceiro

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/shopspring/decimal"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

var v = testeapoio.Valor

func data(a int, m time.Month, d int) *tipos.Data { x := testeapoio.Dia(a, m, d); return &x }

func TestAvos(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		admissao tipos.Data
		saida    *tipos.Data
		avos     int
	}{
		{"o ano inteiro", testeapoio.Dia(2020, 1, 1), nil, 12},
		{"admissão no dia 17 de março: 15 dias contam", testeapoio.Dia(2026, 3, 17), nil, 10},
		{"admissão no dia 18 de março: 14 dias não contam", testeapoio.Dia(2026, 3, 18), nil, 9},
		{"fevereiro com 15 dias (14 a 28)", testeapoio.Dia(2026, 2, 14), nil, 11},
		{"saída no dia 20 de agosto", testeapoio.Dia(2020, 1, 1), data(2026, 8, 20), 8},
		{"saída no dia 14 de agosto", testeapoio.Dia(2020, 1, 1), data(2026, 8, 14), 7},
		{"admitido depois do ano", testeapoio.Dia(2027, 1, 1), nil, 0},
		{"saiu antes do ano", testeapoio.Dia(2020, 1, 1), data(2025, 12, 31), 0},
		{"entrou e saiu no mesmo mês, 15 dias", testeapoio.Dia(2026, 5, 1), data(2026, 5, 15), 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			if got := Avos(2026, c.admissao, c.saida); got != c.avos {
				t.Fatalf("avos = %d; quer %d", got, c.avos)
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

func TestCalcularDecimo(t *testing.T) {
	t.Parallel()
	ti, tr := tabelas(t)
	casos := []struct {
		nome                                          string
		entrada                                       Entrada
		total, primeira, inss, irrf, segunda, liquido string
	}{
		// 6000: INSS 641,51; IRRF (só as deduções legais) 5358,49 x 27,5% - 908,73 = 564,85, menos a redução de 179,75.
		{"o ano inteiro", Entrada{Ano: 2026, Avos: 12, SalarioBase: v("6000.00")},
			"6000.00", "3000.00", "641.51", "385.10", "1973.39", "4973.39"},
		// 3000 x 10/12 = 2500; INSS 121,575 + 879 x 9% = 200,685; IRRF isento (2299,31).
		{"proporcional, 10 avos", Entrada{Ano: 2026, Avos: 10, SalarioBase: v("3000.00")},
			"2500.00", "1250.00", "200.69", "0.00", "1049.31", "2299.31"},
		// média 1200 / 12 = 100: 5100; INSS 515,51; IRRF 4584,49 x 22,5% - 675,49 = 356,02, menos 299,58 (978,62 - 0,133145 x 5100).
		{"com a média das horas extras", Entrada{Ano: 2026, Avos: 12, SalarioBase: v("5000.00"), SomaVariaveis: v("1200.00")},
			"5100.00", "2550.00", "515.51", "56.44", "1978.05", "4528.05"},
		// A 1ª adiantada nas férias, com o valor que veio.
		{"1ª parcela informada", Entrada{Ano: 2026, Avos: 12, SalarioBase: v("6000.00"), PrimeiraValor: &[]tipos.Decimal{v("2800")}[0],
			PrimeiraEm: data(2026, 7, 10)}, "6000.00", "2800.00", "641.51", "385.10", "2173.39", "4973.39"},
		{"sem avos", Entrada{Ano: 2026, Avos: 0, SalarioBase: v("6000.00"), SomaVariaveis: v("100")},
			"0.00", "0.00", "0.00", "0.00", "0.00", "0.00"},
		// Revisão do #700: a 1ª informada maior que o líquido (2299,31) não deixa a 2ª negativa; o excedente aparece.
		{"1ª parcela maior que o líquido", Entrada{Ano: 2026, Avos: 10, SalarioBase: v("3000.00"), PrimeiraValor: &[]tipos.Decimal{v("2400")}[0]},
			"2500.00", "2400.00", "200.69", "0.00", "0.00", "2299.31"},
		{"1ª parcela igual ao líquido", Entrada{Ano: 2026, Avos: 10, SalarioBase: v("3000.00"), PrimeiraValor: &[]tipos.Decimal{v("2299.31")}[0]},
			"2500.00", "2299.31", "200.69", "0.00", "0.00", "2299.31"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			e := c.entrada
			e.Inss, e.Irrf = ti, tr
			r := Calcular(e)
			got := []string{r.Total.String(), r.Primeira.Valor.String(), r.Inss.Valor.String(), r.Irrf.Valor.String(),
				r.Segunda.Valor.String(), r.Liquido.String()}
			quer := []string{c.total, c.primeira, c.inss, c.irrf, c.segunda, c.liquido}
			if fmt.Sprint(got) != fmt.Sprint(quer) {
				t.Fatalf("13º = %v; quer %v", got, quer)
			}
		})
	}
	r := Calcular(Entrada{Ano: 2026, Avos: 12, SalarioBase: v("6000.00"), Inss: ti, Irrf: tr})
	if r.Primeira.Data.String() != "2026-11-30" || r.Segunda.Data.String() != "2026-12-20" || r.Primeira.Informada {
		t.Fatalf("datas = %s, %s", r.Primeira.Data, r.Segunda.Data)
	}
	if r.Irrf.DeducaoUsada != encargos.DeducaoLegal {
		t.Fatalf("o 13º não usa o desconto simplificado: %s", r.Irrf.DeducaoUsada)
	}
	// Só a data informada também marca a parcela como informada (o valor fica o da metade).
	r = Calcular(Entrada{Ano: 2026, Avos: 12, SalarioBase: v("6000.00"), PrimeiraEm: data(2026, 6, 1), Inss: ti, Irrf: tr})
	if !r.Primeira.Informada || r.Primeira.ValorInformado || r.Primeira.Valor.String() != "3000.00" || !r.Excedente.Decimal().IsZero() {
		t.Fatalf("1ª = %+v", r.Primeira)
	}
	r = Calcular(Entrada{Ano: 2026, Avos: 10, SalarioBase: v("3000.00"), PrimeiraValor: &[]tipos.Decimal{v("2400")}[0], MesesComHolerite: 4,
		Inss: ti, Irrf: tr})
	if r.Excedente.String() != "100.69" || !r.Primeira.ValorInformado || r.MesesComHolerite != 4 {
		t.Fatalf("excedente = %s, 1ª = %+v, meses = %d", r.Excedente, r.Primeira, r.MesesComHolerite)
	}
}

type mux struct{ *http.ServeMux }

func (m mux) HandleFunc(p string, h http.HandlerFunc) { m.ServeMux.HandleFunc(p, h) }

func TestDecimoPelaApi(t *testing.T) {
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
		r := chamar("POST", "/api/trabalho/empregos", corpo)
		var e emprego.Emprego
		if err := json.Unmarshal(r.Corpo, &e); err != nil {
			t.Fatal(err)
		}
		return e.Id
	}
	ler := func(r testeapoio.Resposta) Resposta {
		var c Resposta
		if err := json.Unmarshal(r.Corpo, &c); err != nil {
			t.Fatalf("%s: %v", r.Corpo, err)
		}
		return c
	}

	id := criar(`{"empregador":"E","admissao":"2024-03-11","salarioBruto":5000}`)
	chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/salarios", id), `{"inicio":"2026-12-01","salarioBruto":6000}`).ConferirStatus(t, 200)
	// Um holerite de outubro com 10 h extras a 50% (5000/220 x 1,5 x 10 = 340,91) e o DSR (340,91 / 27 x 4 = 50,50):
	// 391,41 de variáveis no ano. Pela Súmula 347 do TST, as horas valem pelo salário de agora (6000 em dezembro):
	// 391,41 x 6000 / 5000 = 469,69, média 469,69 / 12 = 39,14.
	r := chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", id), `{"mes":"2026-10"}`)
	var h holerite.Holerite
	_ = json.Unmarshal(r.Corpo, &h)
	chamar("PUT", fmt.Sprintf("/api/trabalho/holerites/%d", h.Id), `{"diasUteis":27,"diasDescanso":4,"lancamentos":[{"tipo":1,"quantidade":10,"percentual":50}]}`).
		ConferirStatus(t, 200)

	caminho := fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/2026", id)
	c := ler(chamar("GET", caminho, ""))
	// O salário base é o de dezembro (6000), mais a média: 6039,14. Só outubro tem holerite: a tela avisa.
	if c.Avos != 12 || c.SalarioBase.String() != "6000.00" || c.MediaVariaveis.String() != "39.14" || c.Total.String() != "6039.14" ||
		c.Saida != nil || c.MesesComHolerite != 1 {
		t.Fatalf("13º de 2026 = %+v", c.Calculo)
	}
	// A 1ª parcela informada: volta no cálculo; nulos voltam ao padrão.
	c = ler(chamar("PUT", caminho, `{"primeiraParcelaEm":"2026-07-10","primeiraParcelaValor":2800}`))
	if !c.Primeira.Informada || c.Primeira.Valor.String() != "2800.00" || c.Primeira.Data.String() != "2026-07-10" {
		t.Fatalf("1ª = %+v", c.Primeira)
	}
	if !c.Primeira.ValorInformado {
		t.Fatalf("1ª com o valor = %+v", c.Primeira)
	}
	// Só a data (revisão do #700): o valor continua a metade calculada, e a resposta diz que ele não foi informado.
	c = ler(chamar("PUT", caminho, `{"primeiraParcelaEm":"2026-07-10","primeiraParcelaValor":null}`))
	if !c.Primeira.Informada || c.Primeira.ValorInformado || c.Primeira.Valor.String() != "3019.57" {
		t.Fatalf("1ª só com a data = %+v", c.Primeira)
	}
	c = ler(chamar("PUT", caminho, `{"primeiraParcelaEm":null,"primeiraParcelaValor":null}`))
	if c.Primeira.Informada {
		t.Fatalf("1ª depois de limpar = %+v", c.Primeira)
	}

	// Saiu no ano: o salário base é o do mês da saída e a resposta diz a saída.
	saiu := criar(`{"empregador":"S","admissao":"2020-01-01","saida":"2026-08-20","salarioBruto":6000}`)
	c = ler(chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/2026", saiu), ""))
	if c.Avos != 8 || c.Total.String() != "4000.00" || c.Saida == nil {
		t.Fatalf("13º de quem saiu = %+v", c)
	}

	pj := criar(`{"empregador":"P","admissao":"2020-01-01","salarioBruto":9000,"regime":3}`)
	casos := []struct {
		nome, metodo, caminho, corpo string
		status                       int
		mensagem                     string
	}{
		{"PJ", "GET", fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/2026", pj), "", 400, MensagemSemDecimo},
		{"ano antes da admissão", "GET", fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/2023", id), "", 400, MensagemForaDoAno},
		{"ano sem tabela", "GET", fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/2024", id), "", 409, MensagemSemTabela},
		{"valor negativo", "PUT", caminho, `{"primeiraParcelaValor":-1}`, 400, "O valor da 1ª parcela vai de R$ 0,00 a R$ 10.000.000,00."},
		{"valor absurdo", "PUT", caminho, `{"primeiraParcelaValor":10000000.01}`, 400, "O valor da 1ª parcela vai de R$ 0,00 a R$ 10.000.000,00."},
		{"data em outro ano", "PUT", caminho, `{"primeiraParcelaEm":"2025-12-01"}`, 400, "A 1ª parcela do 13º de 2026 é paga em 2026."},
		{"1ª maior que o 13º", "PUT", caminho, `{"primeiraParcelaValor":6039.15}`, 400, "A 1ª parcela não passa do 13º do ano inteiro (R$ 6.039,14)."},
		{"JSON quebrado", "PUT", caminho, `{"primeira`, 400, MensagemJSONInvalido},
		{"PUT no PJ", "PUT", fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/2026", pj), `{"primeiraParcelaValor":1}`, 400, MensagemSemDecimo},
	}
	for _, k := range casos {
		t.Run(k.nome, func(t *testing.T) {
			r := chamar(k.metodo, k.caminho, k.corpo)
			r.ConferirStatus(t, k.status)
			r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, k.mensagem))
		})
	}
	for _, p := range []string{"/api/trabalho/empregos/99/decimo-terceiro/2026", "/api/trabalho/empregos/x/decimo-terceiro/2026",
		fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/abc", id), fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/1899", id),
		fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/2201", id)} {
		chamar("GET", p, "").ConferirStatus(t, http.StatusNotFound)
	}
	// Saiu em março de 2025: o mês base tem tabela do INSS (2025), mas não do IRRF (desde maio de 2025).
	marco := criar(`{"empregador":"M","admissao":"2020-01-01","saida":"2025-03-31","salarioBruto":3000}`)
	r = chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/2025", marco), "")
	r.ConferirStatus(t, http.StatusConflict)
	r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, MensagemSemTabela))
	// Os limites do ano valem.
	chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/2200", id), "").ConferirStatus(t, http.StatusOK)
	chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/1900", id), "").ConferirStatus(t, http.StatusBadRequest)
	chamar("PUT", fmt.Sprintf("/api/trabalho/empregos/%d/decimo-terceiro/2026", id), `{"primeiraParcelaValor":0}`).ConferirStatus(t, http.StatusOK)
}

func TestReais(t *testing.T) {
	for v, quer := range map[string]string{"600": "R$ 600,00", "6039.14": "R$ 6.039,14", "0.5": "R$ 0,50", "1234567.8": "R$ 1.234.567,80"} {
		if got := reais(decimal.RequireFromString(v)); got != quer {
			t.Errorf("reais(%s) = %q; quer %q", v, got, quer)
		}
	}
}

// Revisão do #701: a 1ª parcela pedida nas férias aparece no 13º, com a data do pagamento delas; a informada à mão vale
// mais.
func TestPrimeiraParcelaComAsFerias(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBanco(t)
	rel := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	empregos := emprego.NovoServico(db, rel, log)
	s := NovoServico(db, empregos, holerite.NovoServico(db, empregos, rel, log), rel, log)
	e, err := empregos.Criar(t.Context(), emprego.Dados{Empregador: "E", Admissao: testeapoio.Dia(2024, 3, 11), Regime: emprego.RegimeClt,
		JornadaMensalHoras: 220, SalarioInicial: &[]tipos.Decimal{v("6000")}[0]})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"2026-07-15", "2025-07-15"} {
		if _, err := db.ExecContext(t.Context(), `INSERT INTO "FeriasAgendamento" ("EmpregoId", "PeriodoInicio", "Inicio", "Dias", "DiasVendidos",
			"AdiantarDecimo", "CriadoEm") VALUES (?, '2025-03-11', ?, 20, 0, 1, '2026-01-01 00:00:00')`, e.Id, f); err != nil {
			t.Fatal(err)
		}
	}
	r, err := s.Obter(t.Context(), e.Id, 2026)
	if err != nil || !r.Primeira.ComFerias || !r.Primeira.Informada || r.Primeira.ValorInformado || r.Primeira.Data.String() != "2026-07-13" {
		t.Fatalf("1ª com as férias = %+v (%v)", r.Primeira, err)
	}
	r, _ = s.DefinirPrimeira(t.Context(), e.Id, 2026, &[]tipos.Data{testeapoio.Dia(2026, 7, 10)}[0], nil)
	if r.Primeira.ComFerias || r.Primeira.Data.String() != "2026-07-10" {
		t.Fatalf("1ª informada = %+v", r.Primeira)
	}
}
