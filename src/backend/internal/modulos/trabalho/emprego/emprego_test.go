package emprego

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// mux é o http.ServeMux como Registrador (o site usa o modulos.Rotas, que guarda as rotas).
type mux struct{ *http.ServeMux }

func (m mux) HandleFunc(p string, h http.HandlerFunc) { m.ServeMux.HandleFunc(p, h) }

type ambiente struct {
	t   *testing.T
	s   *Servico
	h   http.Handler
	fab *testeapoio.Fabrica
}

func novo(t *testing.T) ambiente {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 11, 13)
	s := NovoServico(db, r, slog.New(slog.DiscardHandler))
	m := mux{http.NewServeMux()}
	Registrar(m, s)
	return ambiente{t: t, s: s, h: m, fab: testeapoio.NovaFabrica(t, db, r)}
}

func (a ambiente) chamar(metodo, caminho, corpo string) testeapoio.Resposta {
	a.t.Helper()
	return testeapoio.Chamar(a.t, a.h, metodo, caminho, corpo)
}

func decodificar(t *testing.T, r testeapoio.Resposta, destino any) {
	t.Helper()
	if err := json.Unmarshal(r.Corpo, destino); err != nil {
		t.Fatalf("ler a resposta %s: %v", r.Corpo, err)
	}
}

const empregoValido = `{"empregador":"Empresa Fictícia Ltda","cnpj":"12.345.678/0001-90","admissao":"2024-03-11",
	"regime":1,"jornadaMensalHoras":220,"dependentes":0,"salarioBruto":5000.00}`

// criar cria o emprego válido e devolve o id.
func (a ambiente) criar(corpo string) int64 {
	a.t.Helper()
	r := a.chamar("POST", "/api/trabalho/empregos", corpo)
	r.ConferirStatus(a.t, http.StatusCreated)
	var e Emprego
	decodificar(a.t, r, &e)
	return e.Id
}

func TestCriarEmprego(t *testing.T) {
	t.Parallel()
	a := novo(t)
	r := a.chamar("POST", "/api/trabalho/empregos", empregoValido)
	r.ConferirStatus(t, http.StatusCreated)
	// O CNPJ fica só com os dígitos, e o primeiro salário começa no mês da admissão.
	r.ConferirJSON(t, `{"id":1,"empregador":"Empresa Fictícia Ltda","cnpj":"12345678000190","admissao":"2024-03-11",
		"saida":null,"regime":1,"jornadaMensalHoras":220,"dependentes":0,"contaId":null,"contaNome":null,
		"salarios":[{"id":1,"inicio":"2024-03-01","salarioBruto":5000.00}]}`)
}

func TestCriarComPadroesEConta(t *testing.T) {
	t.Parallel()
	a := novo(t)
	conta := a.fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Conta salário" })
	r := a.chamar("POST", "/api/trabalho/empregos", fmt.Sprintf(`{"empregador":"  Outra  ","admissao":"2026-01-05",
		"salarioBruto":3000,"contaId":%d,"cnpj":""}`, conta.Id))
	r.ConferirStatus(t, http.StatusCreated)
	var e Emprego
	decodificar(t, r, &e)
	if e.Empregador != "Outra" || e.Regime != RegimeClt || e.JornadaMensalHoras != 220 || e.Cnpj != nil ||
		e.ContaNome == nil || *e.ContaNome != "Conta salário" {
		t.Fatalf("emprego = %+v", e)
	}
}

func TestCriarRecusaDadosErrados(t *testing.T) {
	t.Parallel()
	a := novo(t)
	casos := []struct {
		nome, corpo, mensagem string
	}{
		{"sem empregador", `{"empregador":" ","admissao":"2026-01-01","salarioBruto":1}`, "Informe o empregador."},
		{"empregador comprido", `{"empregador":"` + strings.Repeat("x", 151) + `","admissao":"2026-01-01","salarioBruto":1}`,
			"O nome do empregador vai até 150 caracteres."},
		{"sem admissão", `{"empregador":"E","salarioBruto":1}`, "Informe a data de admissão."},
		{"regime desconhecido", `{"empregador":"E","admissao":"2026-01-01","salarioBruto":1,"regime":4}`, "Escolha o regime: CLT, estágio ou PJ."},
		{"regime zero", `{"empregador":"E","admissao":"2026-01-01","salarioBruto":1,"regime":0}`, "Escolha o regime: CLT, estágio ou PJ."},
		{"jornada zero", `{"empregador":"E","admissao":"2026-01-01","salarioBruto":1,"jornadaMensalHoras":0}`,
			"A jornada mensal vai de 1 a 300 horas (a da CLT é 220)."},
		{"jornada demais", `{"empregador":"E","admissao":"2026-01-01","salarioBruto":1,"jornadaMensalHoras":301}`,
			"A jornada mensal vai de 1 a 300 horas (a da CLT é 220)."},
		{"dependentes negativos", `{"empregador":"E","admissao":"2026-01-01","salarioBruto":1,"dependentes":-1}`, "Os dependentes vão de 0 a 20."},
		{"dependentes demais", `{"empregador":"E","admissao":"2026-01-01","salarioBruto":1,"dependentes":21}`, "Os dependentes vão de 0 a 20."},
		{"saída antes da admissão", `{"empregador":"E","admissao":"2026-01-02","saida":"2026-01-01","salarioBruto":1}`,
			"A saída não pode ser antes da admissão."},
		{"CNPJ curto", `{"empregador":"E","admissao":"2026-01-01","salarioBruto":1,"cnpj":"1234567800019"}`, "O CNPJ tem 14 dígitos."},
		{"sem salário", `{"empregador":"E","admissao":"2026-01-01"}`, "Informe o salário bruto."},
		{"salário zero", `{"empregador":"E","admissao":"2026-01-01","salarioBruto":0}`, "Informe o salário bruto."},
		{"salário absurdo", `{"empregador":"E","admissao":"2026-01-01","salarioBruto":10000000.01}`, "O salário bruto vai até R$ 10.000.000,00."},
		{"conta que não existe", `{"empregador":"E","admissao":"2026-01-01","salarioBruto":1,"contaId":99}`, "A conta escolhida não existe mais."},
		{"JSON quebrado", `{"empregador":`, MensagemJSONInvalido},
		{"data em outro formato", `{"empregador":"E","admissao":"01/01/2026","salarioBruto":1}`, MensagemJSONInvalido},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := a.chamar("POST", "/api/trabalho/empregos", c.corpo)
			r.ConferirStatus(t, http.StatusBadRequest)
			r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, c.mensagem))
		})
	}
	a.chamar("GET", "/api/trabalho/empregos", "").ConferirJSON(t, `[]`)
}

func TestAumentoNaoMudaOPassado(t *testing.T) {
	t.Parallel()
	a := novo(t)
	id := a.criar(empregoValido)
	// Aumento no meio do ano: o dia vira 1.
	a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/salarios", id), `{"inicio":"2026-06-20","salarioBruto":6000}`).
		ConferirStatus(t, http.StatusOK)
	e, err := a.s.Obter(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	casos := []struct{ mes, salario string }{
		{"2024-03-01", "5000.00"}, {"2026-05-31", "5000.00"}, {"2026-06-01", "6000.00"}, {"2030-01-01", "6000.00"},
	}
	for _, c := range casos {
		mes, _ := tipos.LerData(c.mes)
		v, ok := SalarioDoMes(e.Salarios, mes)
		if !ok || v.SalarioBruto.String() != c.salario {
			t.Errorf("salário de %s = %s; quer %s", c.mes, v.SalarioBruto, c.salario)
		}
	}
	if _, ok := SalarioDoMes(e.Salarios, testeapoio.Dia(2024, 2, 1)); ok {
		t.Error("antes da admissão não há salário")
	}
	// O mesmo mês troca o valor, sem vigência nova.
	a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/salarios", id), `{"inicio":"2026-06-01","salarioBruto":6100}`).
		ConferirStatus(t, http.StatusOK)
	e, _ = a.s.Obter(t.Context(), id)
	if len(e.Salarios) != 2 || e.Salarios[0].SalarioBruto.String() != "6100.00" || e.Salarios[0].Inicio.String() != "2026-06-01" {
		t.Fatalf("salários = %+v", e.Salarios)
	}
}

func TestSalarioForaDoEmprego(t *testing.T) {
	t.Parallel()
	a := novo(t)
	id := a.criar(`{"empregador":"E","admissao":"2025-02-10","saida":"2025-11-30","salarioBruto":3000}`)
	caminho := fmt.Sprintf("/api/trabalho/empregos/%d/salarios", id)
	casos := []struct {
		nome, corpo string
		status      int
		mensagem    string
	}{
		{"antes da admissão", `{"inicio":"2025-01-01","salarioBruto":3100}`, 400,
			"O salário começa no mês da admissão (fevereiro de 2025) ou depois."},
		{"depois da saída", `{"inicio":"2025-12-01","salarioBruto":3100}`, 400,
			"O salário não pode começar depois da saída (novembro de 2025)."},
		{"sem o mês", `{"salarioBruto":3100}`, 400, "Informe o mês e o salário bruto."},
		{"salário negativo", `{"inicio":"2025-05-01","salarioBruto":-1}`, 400, "O salário bruto tem de ser maior que zero."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := a.chamar("POST", caminho, c.corpo)
			r.ConferirStatus(t, c.status)
			r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, c.mensagem))
		})
	}
	a.chamar("POST", "/api/trabalho/empregos/99/salarios", `{"inicio":"2025-05-01","salarioBruto":1}`).ConferirStatus(t, http.StatusNotFound)
	// O mês da admissão e o da saída valem.
	a.chamar("POST", caminho, `{"inicio":"2025-11-15","salarioBruto":3200}`).ConferirStatus(t, http.StatusOK)
}

func TestExcluirSalario(t *testing.T) {
	t.Parallel()
	a := novo(t)
	id := a.criar(empregoValido)
	e, _ := a.s.Obter(t.Context(), id)
	primeiro := e.Salarios[0].Id
	// O único salário não sai.
	r := a.chamar("DELETE", fmt.Sprintf("/api/trabalho/empregos/%d/salarios/%d", id, primeiro), "")
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, MensagemUltimo))

	a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/salarios", id), `{"inicio":"2026-01-01","salarioBruto":5500}`).
		ConferirStatus(t, http.StatusOK)
	a.chamar("DELETE", fmt.Sprintf("/api/trabalho/empregos/%d/salarios/%d", id, primeiro), "").ConferirStatus(t, http.StatusOK)
	e, _ = a.s.Obter(t.Context(), id)
	if len(e.Salarios) != 1 || e.Salarios[0].SalarioBruto.String() != "5500.00" {
		t.Fatalf("salários = %+v", e.Salarios)
	}
	a.chamar("DELETE", fmt.Sprintf("/api/trabalho/empregos/%d/salarios/%d", id, primeiro), "").ConferirStatus(t, http.StatusNotFound)
	a.chamar("DELETE", fmt.Sprintf("/api/trabalho/empregos/%d/salarios/x", id), "").ConferirStatus(t, http.StatusNotFound)
	a.chamar("DELETE", fmt.Sprintf("/api/trabalho/empregos/%d/salarios/0", id), "").ConferirStatus(t, http.StatusNotFound)
	a.chamar("DELETE", "/api/trabalho/empregos/0", "").ConferirStatus(t, http.StatusNotFound)
	a.chamar("DELETE", "/api/trabalho/empregos/-1", "").ConferirStatus(t, http.StatusNotFound)
}

func TestAtualizarEmprego(t *testing.T) {
	t.Parallel()
	a := novo(t)
	id := a.criar(empregoValido)
	caminho := fmt.Sprintf("/api/trabalho/empregos/%d", id)
	// Admissão depois do primeiro salário (março de 2024) deixaria um salário fora do emprego.
	r := a.chamar("PUT", caminho, `{"empregador":"E","admissao":"2024-04-01"}`)
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, `{"mensagem":"Há salário antes dessa admissão. Apague o salário antigo ou escolha uma admissão até março de 2024."}`)

	r = a.chamar("PUT", caminho, `{"empregador":"Nova Razão","admissao":"2024-03-31","saida":"2026-10-31","regime":1,"dependentes":2}`)
	r.ConferirStatus(t, http.StatusOK)
	var e Emprego
	decodificar(t, r, &e)
	if e.Empregador != "Nova Razão" || e.Dependentes != 2 || e.Saida == nil || len(e.Salarios) != 1 {
		t.Fatalf("emprego = %+v", e)
	}
	a.chamar("PUT", "/api/trabalho/empregos/99", `{"empregador":"E","admissao":"2024-03-01"}`).ConferirStatus(t, http.StatusNotFound)
	a.chamar("PUT", caminho, `{"empregador":"","admissao":"2024-03-01"}`).ConferirStatus(t, http.StatusBadRequest)
}

func TestExcluirEmpregoLevaOsSalarios(t *testing.T) {
	t.Parallel()
	a := novo(t)
	id := a.criar(empregoValido)
	a.chamar("DELETE", fmt.Sprintf("/api/trabalho/empregos/%d", id), "").ConferirStatus(t, http.StatusNoContent)
	a.chamar("DELETE", fmt.Sprintf("/api/trabalho/empregos/%d", id), "").ConferirStatus(t, http.StatusNotFound)
	var n int
	if err := a.s.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM "SalarioVigencia"`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("sobraram %d salários (%v)", n, err)
	}
}

func TestListarAtuaisPrimeiro(t *testing.T) {
	t.Parallel()
	a := novo(t)
	a.criar(`{"empregador":"Antigo","admissao":"2020-01-01","saida":"2023-12-31","salarioBruto":2000}`)
	a.criar(`{"empregador":"Atual","admissao":"2024-01-01","salarioBruto":4000}`)
	a.criar(`{"empregador":"Bico","admissao":"2025-01-01","salarioBruto":1000,"regime":3}`)
	lista, err := a.s.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for _, e := range lista {
		nomes = append(nomes, e.Empregador)
	}
	if got := strings.Join(nomes, ", "); got != "Bico, Atual, Antigo" {
		t.Fatalf("ordem = %s", got)
	}
}

func TestEstimativaDoMes(t *testing.T) {
	t.Parallel()
	a := novo(t)
	id := a.criar(empregoValido) // 5000 desde março de 2024
	a.chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/salarios", id), `{"inicio":"2026-06-01","salarioBruto":6000.00}`).
		ConferirStatus(t, http.StatusOK)
	casos := []struct {
		nome, query                string
		bruto, inss, irrf, liquido string
	}{
		// 6000: INSS 641,51; IRRF 564,85 - 179,75 (Lei 15.270) = 385,10 (os casos do pacote encargos).
		{"mês de hoje (novembro de 2026)", "", "6000.00", "641.51", "385.10", "4973.39"},
		// Antes do aumento, o salário de antes: 5000 zera o IRRF pela redução.
		{"maio de 2026, antes do aumento", "?mes=2026-05", "5000.00", "501.51", "0.00", "4498.49"},
		// 2025: tabela do INSS de 2025 e o IRRF sem a redução.
		{"junho de 2025", "?mes=2025-06", "5000.00", "509.60", "312.89", "4177.51"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := a.chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/estimativa%s", id, c.query), "")
			r.ConferirStatus(t, http.StatusOK)
			var e Estimativa
			decodificar(t, r, &e)
			got := []string{e.SalarioBruto.String(), e.Inss.Valor.String(), e.Irrf.Valor.String(), e.Liquido.String()}
			quer := []string{c.bruto, c.inss, c.irrf, c.liquido}
			if strings.Join(got, " ") != strings.Join(quer, " ") || e.SemEncargos {
				t.Fatalf("estimativa = %v; quer %v", got, quer)
			}
		})
	}
	// Antes da tabela do IRRF de maio de 2025, sem estimativa (não há tabela antiga no banco).
	r := a.chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/estimativa?mes=2025-04", id), "")
	r.ConferirStatus(t, http.StatusConflict)
	r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, MensagemSemTabela))
	// Antes da admissão e mês mal escrito.
	r = a.chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/estimativa?mes=2024-02", id), "")
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, `{"mensagem":"Escolha um mês em que o emprego estava ativo."}`)
	a.chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/estimativa?mes=11-2026", id), "").ConferirStatus(t, http.StatusBadRequest)
	a.chamar("GET", "/api/trabalho/empregos/99/estimativa", "").ConferirStatus(t, http.StatusNotFound)
}

func TestEstimativaComDependentesESemEncargos(t *testing.T) {
	t.Parallel()
	a := novo(t)
	clt := a.criar(`{"empregador":"E","admissao":"2026-01-01","salarioBruto":6000,"dependentes":2}`)
	r := a.chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/estimativa?mes=2026-11", clt), "")
	var e Estimativa
	decodificar(t, r, &e)
	if e.Irrf.Valor.String() != "280.83" || e.Irrf.DeducaoUsada != "legal" {
		t.Fatalf("IRRF com 2 dependentes = %s (%s); quer 280.83 pelas deduções legais", e.Irrf.Valor, e.Irrf.DeducaoUsada)
	}
	pj := a.criar(`{"empregador":"Cliente","admissao":"2026-01-01","salarioBruto":9000,"regime":3}`)
	r = a.chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/estimativa?mes=2026-11", pj), "")
	r.ConferirJSON(t, `{"mes":"2026-11-01","salarioBruto":9000.00,"vigenciaDe":"2026-01-01","semEncargos":true,"inss":null,
		"irrf":null,"liquido":9000.00}`)
	// Fora do emprego, depois da saída.
	saiu := a.criar(`{"empregador":"E","admissao":"2026-01-01","saida":"2026-03-15","salarioBruto":3000}`)
	a.chamar("GET", fmt.Sprintf("/api/trabalho/empregos/%d/estimativa?mes=2026-04", saiu), "").ConferirStatus(t, http.StatusBadRequest)
}

func TestTabelas(t *testing.T) {
	t.Parallel()
	a := novo(t)
	r := a.chamar("GET", "/api/trabalho/tabelas", "")
	r.ConferirStatus(t, http.StatusOK)
	var tab Tabelas
	decodificar(t, r, &tab)
	if len(tab.Inss) != 2 || len(tab.Irrf) != 2 || tab.Irrf[1].Reducao == nil || !strings.Contains(tab.Inss[1].Fonte, "13") {
		t.Fatalf("tabelas = %+v", tab)
	}
}

func TestSemTabelasNoBanco(t *testing.T) {
	t.Parallel()
	a := novo(t)
	if _, err := a.s.db.ExecContext(t.Context(), `DELETE FROM "InssVigencia"; DELETE FROM "IrrfVigencia"`); err != nil {
		t.Fatal(err)
	}
	a.chamar("GET", "/api/trabalho/tabelas", "").ConferirJSON(t, `{"inss":[],"irrf":[]}`)
}
