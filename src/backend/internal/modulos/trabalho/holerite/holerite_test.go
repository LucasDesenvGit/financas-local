package holerite

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

type mux struct{ *http.ServeMux }

func (m mux) HandleFunc(p string, h http.HandlerFunc) { m.ServeMux.HandleFunc(p, h) }

type ambiente struct {
	t *testing.T
	s *Servico
	h http.Handler
}

func novo(t *testing.T) ambiente {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	empregos := emprego.NovoServico(db, r, log)
	s := NovoServico(db, empregos, r, log)
	m := mux{http.NewServeMux()}
	emprego.Registrar(m, empregos)
	Registrar(m, s)
	return ambiente{t: t, s: s, h: m}
}

func (a ambiente) chamar(metodo, caminho, corpo string) testeapoio.Resposta {
	a.t.Helper()
	return testeapoio.Chamar(a.t, a.h, metodo, caminho, corpo)
}

func decodificar[T any](t *testing.T, r testeapoio.Resposta) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r.Corpo, &v); err != nil {
		t.Fatalf("ler %s: %v", r.Corpo, err)
	}
	return v
}

// empregoDe5500 cadastra o emprego (5500 desde 2024) e devolve o id.
func (a ambiente) empregoDe5500() int64 {
	a.t.Helper()
	r := a.chamar("POST", "/api/trabalho/empregos", `{"empregador":"Empresa Fictícia","admissao":"2024-03-11","salarioBruto":5500}`)
	r.ConferirStatus(a.t, http.StatusCreated)
	return decodificar[emprego.Emprego](a.t, r).Id
}

func (a ambiente) criar(empregoId int64, mes string) Holerite {
	a.t.Helper()
	r := a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", empregoId), `{"mes":"`+mes+`"}`)
	r.ConferirStatus(a.t, http.StatusCreated)
	return decodificar[Holerite](a.t, r)
}

const lancamentosDeOutubro = `{"diasUteis":27,"diasDescanso":4,"lancamentos":[
	{"tipo":1,"quantidade":10,"percentual":50},
	{"tipo":2,"quantidade":20},
	{"tipo":3,"descricao":"PLR","valor":1000},
	{"tipo":10,"valor":400},
	{"tipo":12,"valor":250,"recorrente":true},
	{"tipo":13,"valor":500,"recorrente":true}]}`

func TestHoleriteDoMesPelaApi(t *testing.T) {
	t.Parallel()
	a := novo(t)
	empregoId := a.empregoDe5500()
	h := a.criar(empregoId, "2026-10")
	// Os dias do DSR vêm do calendário de outubro (27 úteis, 4 domingos); sem lançamentos, só o salário: 5500 - INSS
	// 571,51 - IRRF (4892,80 x 27,5% - 908,73 = 436,79, menos a redução de 246,32) 190,47 = 4738,02.
	if h.DiasUteis != 27 || h.DiasDescanso != 4 || h.Situacao != Aberto || h.Calculo.Liquido.String() != "4738.02" {
		t.Fatalf("holerite novo = %+v (líquido %s)", h, h.Calculo.Liquido)
	}
	r := a.chamar("PUT", fmt.Sprintf("/api/trabalho/holerites/%d", h.Id), lancamentosDeOutubro)
	r.ConferirStatus(t, http.StatusOK)
	h = decodificar[Holerite](t, r)
	// O mesmo holerite do TestCalcularHoleriteCompleto: líquido 5053,13.
	if h.Calculo.Liquido.String() != "5053.13" || len(h.Lancamentos) != 6 {
		t.Fatalf("líquido %s, %d lançamentos", h.Calculo.Liquido, len(h.Lancamentos))
	}
	// O adicional noturno sem percentual ficou com 20%; a pensão deduz do IR; o PLR não deduz.
	if l := h.Lancamentos[1]; l.Percentual == nil || !l.Percentual.Decimal().Equal(v("20").Decimal()) {
		t.Errorf("adicional noturno: %+v", l)
	}
	if !h.Lancamentos[5].DeduzIrrf || h.Lancamentos[2].DeduzIrrf {
		t.Errorf("deduções do IR: %+v", h.Lancamentos)
	}

	// Novembro começa com os recorrentes de outubro (plano de saúde e pensão).
	nov := a.criar(empregoId, "2026-11")
	if nov.Mes.String() != "2026-11-01" || len(nov.Lancamentos) != 2 || nov.Lancamentos[0].Tipo != PlanoDeSaude {
		t.Fatalf("novembro = %+v", nov.Lancamentos)
	}

	// A lista, do mais novo para o mais antigo.
	lista := decodificar[[]Resumo](t, a.chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", empregoId), ""))
	if len(lista) != 2 || lista[0].Mes.String() != "2026-11-01" || lista[1].Liquido.String() != "5053.13" {
		t.Fatalf("lista = %+v", lista)
	}
}

func TestFecharCongelaOMes(t *testing.T) {
	t.Parallel()
	a := novo(t)
	empregoId := a.empregoDe5500()
	h := a.criar(empregoId, "2026-10")
	a.chamar("PUT", fmt.Sprintf("/api/trabalho/holerites/%d", h.Id), lancamentosDeOutubro).ConferirStatus(t, http.StatusOK)
	r := a.chamar("POST", fmt.Sprintf("/api/trabalho/holerites/%d/fechar", h.Id), "")
	r.ConferirStatus(t, http.StatusOK)
	fechado := decodificar[Holerite](t, r)
	if fechado.Situacao != Fechado || fechado.FechadoEm == nil {
		t.Fatalf("fechado = %+v", fechado)
	}
	// Um aumento em outubro e a tabela do INSS trocada depois não mudam o mês fechado.
	a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/salarios", empregoId), `{"inicio":"2026-10-01","salarioBruto":9000}`).
		ConferirStatus(t, http.StatusOK)
	if _, err := a.s.db.ExecContext(t.Context(), `UPDATE "InssFaixa" SET "Aliquota" = '0.5' WHERE "Inicio" = '2026-01-01'`); err != nil {
		t.Fatal(err)
	}
	depois := decodificar[Holerite](t, a.chamar("GET", fmt.Sprintf("/api/trabalho/holerites/%d", h.Id), ""))
	if depois.Calculo.Liquido.String() != "5053.13" || depois.Calculo.SalarioContratual.String() != "5500.00" {
		t.Fatalf("o mês fechado mudou: líquido %s, salário %s", depois.Calculo.Liquido, depois.Calculo.SalarioContratual)
	}
	// Fechado não muda, não fecha de novo e não sai.
	caminho := fmt.Sprintf("/api/trabalho/holerites/%d", h.Id)
	for _, c := range []struct{ metodo, caminho, corpo, mensagem string }{
		{"PUT", caminho, lancamentosDeOutubro, MensagemFechado},
		{"POST", caminho + "/fechar", "", MensagemFechado},
		{"DELETE", caminho, "", MensagemFechado},
	} {
		r := a.chamar(c.metodo, c.caminho, c.corpo)
		r.ConferirStatus(t, http.StatusConflict)
		r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, c.mensagem))
	}

	// Reabrir: volta a calcular (com o salário novo) e fica na auditoria.
	r = a.chamar("POST", caminho+"/reabrir", "")
	r.ConferirStatus(t, http.StatusOK)
	reaberto := decodificar[Holerite](t, r)
	if reaberto.Situacao != Aberto || reaberto.FechadoEm != nil || reaberto.Calculo.SalarioContratual.String() != "9000.00" {
		t.Fatalf("reaberto = %+v", reaberto)
	}
	var evento string
	if err := a.s.db.QueryRowContext(t.Context(), `SELECT "Evento" FROM "AuditoriaAcesso"`).Scan(&evento); err != nil || evento != "HoleriteReaberto (2026-10)" {
		t.Fatalf("auditoria = %q (%v)", evento, err)
	}
	r = a.chamar("POST", caminho+"/reabrir", "")
	r.ConferirStatus(t, http.StatusConflict)
	r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, MensagemAberto))
	a.chamar("DELETE", caminho, "").ConferirStatus(t, http.StatusNoContent)
	a.chamar("GET", caminho, "").ConferirStatus(t, http.StatusNotFound)
}

func TestCriarHoleriteRecusa(t *testing.T) {
	t.Parallel()
	a := novo(t)
	empregoId := a.empregoDe5500()
	a.criar(empregoId, "2026-10")
	casos := []struct {
		nome, caminho, corpo string
		status               int
		mensagem             string
	}{
		{"mês repetido", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", empregoId), `{"mes":"2026-10"}`, 409,
			"Já existe o holerite de outubro de 2026."},
		{"antes da admissão", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", empregoId), `{"mes":"2024-02"}`, 400,
			"Escolha um mês em que o emprego estava ativo."},
		{"mês mal escrito", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", empregoId), `{"mes":"10/2026"}`, 400,
			"O mês vai no formato AAAA-MM."},
		{"JSON quebrado", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", empregoId), `{"mes":`, 400, MensagemJSONInvalido},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := a.chamar("POST", c.caminho, c.corpo)
			r.ConferirStatus(t, c.status)
			r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, c.mensagem))
		})
	}
	a.chamar("POST", "/api/trabalho/empregos/99/holerites", `{"mes":"2026-10"}`).ConferirStatus(t, http.StatusNotFound)
	a.chamar("GET", "/api/trabalho/empregos/99/holerites", "").ConferirStatus(t, http.StatusNotFound)
	a.chamar("GET", "/api/trabalho/empregos/x/holerites", "").ConferirStatus(t, http.StatusNotFound)
}

func TestSalvarRecusaLancamentoErrado(t *testing.T) {
	t.Parallel()
	a := novo(t)
	h := a.criar(a.empregoDe5500(), "2026-10")
	caminho := fmt.Sprintf("/api/trabalho/holerites/%d", h.Id)
	corpo := func(dias, lancamentos string) string { return `{` + dias + `,"lancamentos":[` + lancamentos + `]}` }
	dias := `"diasUteis":27,"diasDescanso":4`
	casos := []struct{ nome, corpo, mensagem string }{
		{"tipo desconhecido", corpo(dias, `{"tipo":5,"valor":1}`), "Escolha o tipo de cada lançamento."},
		{"descrição comprida", corpo(dias, `{"tipo":3,"valor":1,"descricao":"`+strings.Repeat("x", 101)+`"}`), "A descrição vai até 100 caracteres."},
		{"hora extra sem horas", corpo(dias, `{"tipo":1,"percentual":50}`), "As horas vão de 0,01 a 300."},
		{"hora extra demais", corpo(dias, `{"tipo":1,"quantidade":301,"percentual":50}`), "As horas vão de 0,01 a 300."},
		{"hora extra sem percentual", corpo(dias, `{"tipo":1,"quantidade":10}`), "O percentual vai de 0,01% a 300%."},
		{"percentual demais", corpo(dias, `{"tipo":2,"quantidade":10,"percentual":301}`), "O percentual vai de 0,01% a 300%."},
		{"provento sem valor", corpo(dias, `{"tipo":3}`), "O valor vai de R$ 0,01 a R$ 10.000.000,00."},
		{"desconto zerado", corpo(dias, `{"tipo":12,"valor":0}`), "O valor vai de R$ 0,01 a R$ 10.000.000,00."},
		{"valor absurdo", corpo(dias, `{"tipo":14,"valor":10000000.01}`), "O valor vai de R$ 0,01 a R$ 10.000.000,00."},
		{"sem dias úteis", corpo(`"diasUteis":0,"diasDescanso":4`, ``), "Os dias úteis e os de descanso somam no máximo 31."},
		{"dias demais", corpo(`"diasUteis":28,"diasDescanso":4`, ``), "Os dias úteis e os de descanso somam no máximo 31."},
		{"descanso negativo", corpo(`"diasUteis":27,"diasDescanso":-1`, ``), "Os dias úteis e os de descanso somam no máximo 31."},
		{"úteis demais", corpo(`"diasUteis":32,"diasDescanso":0`, ``), "Os dias úteis e os de descanso somam no máximo 31."},
		{"descanso demais", corpo(`"diasUteis":1,"diasDescanso":32`, ``), "Os dias úteis e os de descanso somam no máximo 31."},
		{"lançamentos demais", corpo(dias, strings.TrimSuffix(strings.Repeat(`{"tipo":14,"valor":1},`, 101), ",")), "O holerite vai até 100 lançamentos."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := a.chamar("PUT", caminho, c.corpo)
			r.ConferirStatus(t, http.StatusBadRequest)
			r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, c.mensagem))
		})
	}
	// Os limites valem: 1 dia útil, 31 dias no total e 100 lançamentos.
	a.chamar("PUT", caminho, corpo(`"diasUteis":1,"diasDescanso":30`, strings.TrimSuffix(strings.Repeat(`{"tipo":14,"valor":1},`, 100), ","))).
		ConferirStatus(t, http.StatusOK)
	// O VT sem custo vale (desconta os 6%), e as marcas que não valem para o tipo saem.
	r := a.chamar("PUT", caminho, corpo(`"diasUteis":31,"diasDescanso":0`, `{"tipo":10,"incideInss":true},{"tipo":1,"quantidade":1,"percentual":50,"valor":9,"deduzIrrf":true},{"tipo":12,"valor":10,"incideInss":true,"incideIrrf":true},{"tipo":3,"valor":10,"deduzIrrf":true,"quantidade":3}`))
	r.ConferirStatus(t, http.StatusOK)
	h = decodificar[Holerite](t, r)
	vt, he, plano, outro := h.Lancamentos[0], h.Lancamentos[1], h.Lancamentos[2], h.Lancamentos[3]
	if vt.IncideInss || he.Valor != nil || he.DeduzIrrf || !he.IncideInss || plano.IncideInss || plano.IncideIrrf ||
		outro.DeduzIrrf || outro.Quantidade != nil {
		t.Fatalf("marcas normalizadas: %+v", h.Lancamentos)
	}
	a.chamar("PUT", "/api/trabalho/holerites/99", corpo(dias, ``)).ConferirStatus(t, http.StatusNotFound)
	a.chamar("PUT", caminho, `{"diasUteis":`).ConferirStatus(t, http.StatusBadRequest)
	a.chamar("POST", "/api/trabalho/holerites/99/fechar", "").ConferirStatus(t, http.StatusNotFound)
	a.chamar("POST", "/api/trabalho/holerites/99/reabrir", "").ConferirStatus(t, http.StatusNotFound)
	a.chamar("DELETE", "/api/trabalho/holerites/99", "").ConferirStatus(t, http.StatusNotFound)
}

func TestHoleriteSemTabela(t *testing.T) {
	t.Parallel()
	a := novo(t)
	r := a.chamar("POST", "/api/trabalho/empregos", `{"empregador":"Antigo","admissao":"2024-01-01","salarioBruto":3000}`)
	empregoId := decodificar[emprego.Emprego](t, r).Id
	// Março de 2025 não tem tabela do IRRF (o site traz desde maio de 2025): o holerite existe, sem o cálculo.
	h := a.criar(empregoId, "2025-03")
	if h.Calculo != nil || h.Aviso == nil {
		t.Fatalf("sem tabela = %+v", h)
	}
	r = a.chamar("POST", fmt.Sprintf("/api/trabalho/holerites/%d/fechar", h.Id), "")
	r.ConferirStatus(t, http.StatusConflict)
	r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, MensagemSemTabela))
	lista := decodificar[[]Resumo](t, a.chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", empregoId), ""))
	if len(lista) != 1 || lista[0].Liquido != nil {
		t.Fatalf("lista = %+v", lista)
	}
	// PJ não precisa de tabela.
	r = a.chamar("POST", "/api/trabalho/empregos", `{"empregador":"Cliente","admissao":"2024-01-01","salarioBruto":9000,"regime":3}`)
	pj := a.criar(decodificar[emprego.Emprego](t, r).Id, "2024-06")
	if pj.Calculo == nil || pj.Calculo.Liquido.String() != "9000.00" || !pj.Calculo.SemEncargos {
		t.Fatalf("PJ = %+v", pj.Calculo)
	}
}
