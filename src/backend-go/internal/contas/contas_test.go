package contas

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func p[T any](v T) *T { return &v }

func dec(t *testing.T, s string) tipos.Decimal {
	t.Helper()
	d, err := tipos.NovoDecimal(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// O relógio dos testes: 15/10/2026, meio-dia em Brasília (15h em UTC).
func rotasDeTeste(t *testing.T) (*http.ServeMux, *sql.DB, *testeapoio.Fabrica) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 15)
	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, r, slog.New(slog.DiscardHandler)))
	return mux, db, testeapoio.NovaFabrica(t, db, r)
}

// Regra: o saldo é o do CalcularSaldosAsync do C#, escala inclusive: a conta do Open Finance vale o saldo do banco
// ("123.40"); a manual, o inicial mais as efetivadas (a pendente fica de fora), com a soma no formato em que o EF Core a
// devolve do SQLite (-0.25 + -0.75 = -1.0, e não -1.00). O gabarito é o testdata/saldos.json, gravado pela sonda das
// contas com o EF Core de verdade, sobre estas mesmas linhas.
func TestSaldosIgualAoEFCore(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	for _, c := range []struct {
		id                  int64
		origem              int64
		saldoInicial, saldo string
	}{{1, 1, "0.0", "0.0"}, {2, 1, "100.0", "0.0"}, {3, 1, "5000.0", "0.0"}, {4, 1, "1.5", "0.0"}, {5, 2, "0.0", "123.40"},
		{6, 1, "0", "0.0"}} {
		if _, err := db.Exec(`INSERT INTO Conta (Id, Nome, Tipo, Origem, Moeda, SaldoInicial, Saldo, Cor, IncluirNoSaldo,
			Arquivada, CriadoEm, AtualizadoEm) VALUES (?, 'x', 1, ?, 'BRL', ?, ?, '#000000', 1, 0, '2026-01-01 00:00:00',
			'2026-01-01 00:00:00')`, c.id, c.origem, c.saldoInicial, c.saldo); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range []struct {
		conta  int64
		valor  string
		status int64
	}{{1, "-0.25", 1}, {1, "-0.75", 1}, {2, "10.50", 1}, {2, "0.50", 1}, {2, "-999.999", 2}, {4, "-1.5", 1}, {5, "-50.0", 1},
		{6, "-0.1", 1}, {6, "-0.2", 1}, {6, "0.30", 1}} {
		if _, err := db.Exec(`INSERT INTO Transacao (ContaId, Data, Descricao, Valor, Tipo, Status, CategoriaManual,
			IgnorarRelatorios, Emergencia, DescricaoRevisada, DescricaoManual, CriadoEm, AtualizadoEm) VALUES (?, '2026-01-02',
			'x', ?, 1, ?, 0, 0, 0, 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00')`, l.conta, l.valor, l.status); err != nil {
			t.Fatal(err)
		}
	}
	dados, err := os.ReadFile("testdata/saldos.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Saldos []struct {
			Conta int64  `json:"conta"`
			Saldo string `json:"saldo"`
		} `json:"saldos"`
	}
	if err := json.Unmarshal(dados, &g); err != nil || len(g.Saldos) != 6 {
		t.Fatalf("gabarito: %v (%d saldos)", err, len(g.Saldos))
	}
	saldos, err := Saldos(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range g.Saldos {
		if obtido := saldos[c.Conta]; obtido.String() != c.Saldo {
			t.Errorf("conta %d: saldo %s, o EF Core dá %s", c.Conta, obtido, c.Saldo)
		}
	}
}

// Regra: criar devolve 201 com o Location, o saldo inicial na escala que veio e o saldo como o banco guarda, o
// carimbo com "Z" (o DateTime.UtcNow da memória) e, no cartão manual, o limite disponível (limite + saldo), o banco
// detectado pelo nome e a fatura do mês. Lido depois, o carimbo vem sem "Z"; a lista vai por arquivada, tipo e nome.
func TestCriarObterEListar(t *testing.T) {
	mux, _, fab := rotasDeTeste(t)
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, mux, metodo, caminho, corpo)
	}
	conexao := fab.ConexaoBancaria(func(c *testeapoio.ConexaoBancaria) { c.NomeInstituicao = p("Nu Pagamentos S.A.") })
	daConexao := fab.Conta(func(c *testeapoio.Conta) {
		c.Nome, c.Origem, c.ConexaoBancariaId, c.Saldo = "Conta", OpenFinance, &conexao.Id, dec(t, "1500.50")
	})

	corrente := chamar("POST", "/api/contas", `{"nome":"  Corrente  ","tipo":1,"numero":" 12 ","saldoInicial":100.50,
		"limiteCredito":10,"diaFechamento":5,"cor":"#123abc","incluirNoSaldo":true,"arquivada":false}`)
	corrente.ConferirStatus(t, http.StatusCreated)
	idCorrente := daConexao.Id + 1
	if local := corrente.Cabecalho.Get("Location"); local != "/api/contas/"+strconv.FormatInt(idCorrente, 10) {
		t.Errorf("Location = %q", local)
	}
	corrente.ConferirJSON(t, fmt.Sprintf(`{"id":%d,"nome":"Corrente","tipo":1,"origem":1,"numero":"12","moeda":"BRL",
		"saldo":100.5,"saldoInicial":100.50,"limiteCredito":null,"limiteDisponivel":null,"diaFechamento":null,
		"diaVencimento":null,"cor":"#123abc","incluirNoSaldo":true,"arquivada":false,"conexaoBancariaId":null,
		"nomeInstituicao":null,"atualizadoEm":"2026-10-15T15:00:00Z","banco":null,"bancoDefinido":false,
		"contaVinculadaId":null,"faturaMes":null,"limiteAjustadoNoApp":null,"limiteDiferencaAvaliada":null}`, idCorrente))

	idCartao := idCorrente + 1
	faturaAberta := func(valor string) string {
		return fmt.Sprintf(`{"contaId":%d,"cartao":"Cartão Inter","valorMes":%s,"parcelasMes":0,"faturaId":null,
			"vencimento":"2026-11-05","situacao":"aberta","banco":"inter","fechamento":"2026-10-28","proxima":null}`, idCartao, valor)
	}
	chamar("POST", "/api/contas", fmt.Sprintf(`{"nome":"Cartão Inter","tipo":3,"limiteCredito":1000.00,"diaFechamento":28,
		"diaVencimento":5,"cor":"#000000","incluirNoSaldo":true,"contaVinculadaId":%d}`, idCorrente)).
		ConferirJSON(t, fmt.Sprintf(`{"id":%d,"nome":"Cartão Inter","tipo":3,"origem":1,"numero":null,"moeda":"BRL",
		"saldo":0.0,"saldoInicial":0,"limiteCredito":1000.00,"limiteDisponivel":1000.00,"diaFechamento":28,
		"diaVencimento":5,"cor":"#000000","incluirNoSaldo":true,"arquivada":false,"conexaoBancariaId":null,
		"nomeInstituicao":null,"atualizadoEm":"2026-10-15T15:00:00Z","banco":"inter","bancoDefinido":false,
		"contaVinculadaId":%d,"faturaMes":%s,"limiteAjustadoNoApp":null,"limiteDiferencaAvaliada":null}`,
			idCartao, idCorrente, faturaAberta("0")))

	cartao := testeapoio.Conta{Id: idCartao}
	fab.Transacao(cartao, func(x *testeapoio.Transacao) { x.Valor, x.Data = dec(t, "-120.35"), tipos.NovaData(2026, 10, 10) })
	chamar("GET", fmt.Sprintf("/api/contas/%d", idCartao), "").ConferirJSON(t, fmt.Sprintf(`{"id":%d,"nome":"Cartão Inter",
		"tipo":3,"origem":1,"numero":null,"moeda":"BRL","saldo":-120.35,"saldoInicial":0.0,"limiteCredito":1000.0,
		"limiteDisponivel":879.65,"diaFechamento":28,"diaVencimento":5,"cor":"#000000","incluirNoSaldo":true,
		"arquivada":false,"conexaoBancariaId":null,"nomeInstituicao":null,"atualizadoEm":"2026-10-15T15:00:00",
		"banco":"inter","bancoDefinido":false,"contaVinculadaId":%d,"faturaMes":%s,"limiteAjustadoNoApp":null,
		"limiteDiferencaAvaliada":null}`, idCartao, idCorrente, faturaAberta("120.35")))

	arquivada := fab.Conta(func(c *testeapoio.Conta) { c.Nome, c.Arquivada = "Antiga", true })
	ids := func(corpo []byte) []int64 {
		var lista []struct{ Id int64 }
		if err := json.Unmarshal(corpo, &lista); err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for _, c := range lista {
			ids = append(ids, c.Id)
		}
		return ids
	}
	sem := chamar("GET", "/api/contas", "")
	sem.ConferirStatus(t, http.StatusOK)
	if got := fmt.Sprint(ids(sem.Corpo)); got != fmt.Sprint([]int64{daConexao.Id, idCorrente, idCartao}) {
		t.Errorf("lista = %s", got)
	}
	com := chamar("GET", "/api/contas?IncluirArquivadas=+TRUE+", "")
	if got := fmt.Sprint(ids(com.Corpo)); got != fmt.Sprint([]int64{daConexao.Id, idCorrente, idCartao, arquivada.Id}) {
		t.Errorf("lista com as arquivadas = %s", got)
	}
	falsa := chamar("GET", "/api/contas?incluirArquivadas=false", "")
	if got := fmt.Sprint(ids(falsa.Corpo)); got != fmt.Sprint([]int64{daConexao.Id, idCorrente, idCartao}) {
		t.Errorf("lista com incluirArquivadas=false = %s", got)
	}
	chamar("GET", fmt.Sprintf("/api/contas/%d", daConexao.Id), "").ConferirJSON(t, fmt.Sprintf(`{"id":%d,"nome":"Conta",
		"tipo":1,"origem":2,"numero":null,"moeda":"BRL","saldo":1500.5,"saldoInicial":0.0,"limiteCredito":null,
		"limiteDisponivel":null,"diaFechamento":null,"diaVencimento":null,"cor":"#0f766e","incluirNoSaldo":true,
		"arquivada":false,"conexaoBancariaId":%d,"nomeInstituicao":"Nu Pagamentos S.A.",
		"atualizadoEm":"2026-10-15T15:00:00","banco":"nubank","bancoDefinido":false,"contaVinculadaId":null,
		"faturaMes":null,"limiteAjustadoNoApp":null,"limiteDiferencaAvaliada":null}`, daConexao.Id, conexao.Id))
}

// Regra: as respostas de erro do controller e do model binding: 404 com mensagem, 404 sem corpo quando o id não é
// número ({id:int}), o 400 do ValidationProblemDetails na query e no corpo, e as mensagens de cada validação.
func TestErrosDasRotas(t *testing.T) {
	mux, _, fab := rotasDeTeste(t)
	conexao := fab.ConexaoBancaria()
	daConexao := fab.Conta(func(c *testeapoio.Conta) { c.Origem, c.ConexaoBancariaId = OpenFinance, &conexao.Id })
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Tipo = CartaoCredito })
	corrente := fab.Conta()
	corpo := func(campos string) string {
		return `{"nome":"X","tipo":1,"cor":"#000000","incluirNoSaldo":true` + campos + `}`
	}
	mensagens := []struct{ metodo, caminho, corpo, mensagem string }{
		{"POST", "/api/contas", `{"nome":"X","tipo":4,"saldoInicial":-0.01,"cor":"#000000"}`, "Saldo inicial inválido."},
		{"POST", "/api/contas", corpo(`,"banco":"banco-x"`), "Banco inválido."},
		{"POST", "/api/contas", corpo(`,"banco":""`), "Banco inválido."},
		{"POST", "/api/contas", corpo(fmt.Sprintf(`,"contaVinculadaId":%d`, corrente.Id)), "Só cartão de crédito tem conta vinculada."},
		{"POST", "/api/contas", corpo(`,"tipo":3,"contaVinculadaId":999`), "Conta do cartão inválida."},
		{"POST", "/api/contas", corpo(fmt.Sprintf(`,"tipo":3,"contaVinculadaId":%d`, cartao.Id)),
			"A conta do cartão precisa ser uma conta, não outro cartão."},
		{"PUT", fmt.Sprintf("/api/contas/%d", cartao.Id), corpo(fmt.Sprintf(`,"tipo":3,"contaVinculadaId":%d`, cartao.Id)),
			"O cartão não pode ser a própria conta dele."},
		// Do Open Finance, vale o tipo da conta (corrente), e não o do corpo.
		{"PUT", fmt.Sprintf("/api/contas/%d", daConexao.Id), corpo(fmt.Sprintf(`,"tipo":3,"contaVinculadaId":%d`, corrente.Id)),
			"Só cartão de crédito tem conta vinculada."},
		{"DELETE", fmt.Sprintf("/api/contas/%d", daConexao.Id), "",
			"Contas do Open Finance são gerenciadas pela conexão. Use 'Arquivar' ou remova a conexão."},
		{"PUT", fmt.Sprintf("/api/contas/%d/limite-ajustado", corrente.Id), `{"ajustado":true}`, "Só cartão de crédito tem limite."},
		{"GET", "/api/contas/faturas-do-mes?ano=1999&mes=5", "", "Período inválido."},
		{"GET", "/api/contas/faturas-do-mes?ano=2101&mes=1", "", "Período inválido."},
		{"GET", "/api/contas/faturas-do-mes?ano=2026&mes=0", "", "Período inválido."},
		{"GET", "/api/contas/faturas-do-mes?ano=2026&mes=13", "", "Período inválido."},
		{"GET", "/api/contas/faturas-do-mes", "", "Período inválido."},
	}
	for _, m := range mensagens {
		resp := testeapoio.Chamar(t, mux, m.metodo, m.caminho, m.corpo)
		resp.ConferirStatus(t, http.StatusBadRequest)
		resp.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, m.mensagem))
	}
	for _, c := range []struct{ metodo, caminho, corpo string }{
		{"GET", "/api/contas/999", ""},
		{"PUT", "/api/contas/999", corpo("")},
		{"DELETE", "/api/contas/999", ""},
		{"GET", "/api/contas/999/faturas", ""},
		{"PUT", "/api/contas/999/limite-ajustado", `{"ajustado":null}`},
	} {
		resp := testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo)
		resp.ConferirStatus(t, http.StatusNotFound)
		resp.ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	}
	for _, caminho := range []string{"/api/contas/abc", "/api/contas/2147483648", "/api/contas/abc/faturas"} {
		resp := testeapoio.Chamar(t, mux, "GET", caminho, "")
		resp.ConferirStatus(t, http.StatusNotFound)
		if len(resp.Corpo) != 0 {
			t.Errorf("%s: corpo %s, esperado vazio", caminho, resp.Corpo)
		}
	}
	for _, c := range []struct{ metodo, caminho, corpo string }{
		{"PUT", "/api/contas/abc", corpo("")},
		{"DELETE", "/api/contas/abc", ""},
		{"PUT", "/api/contas/abc/limite-ajustado", `{"ajustado":true}`},
	} {
		testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo).ConferirStatus(t, http.StatusNotFound)
	}
	validacao := func(caminho string, erros string) {
		t.Helper()
		resp := testeapoio.Chamar(t, mux, "GET", caminho, "")
		resp.ConferirStatus(t, http.StatusBadRequest)
		var problema struct{ Errors json.RawMessage }
		if err := json.Unmarshal(resp.Corpo, &problema); err != nil {
			t.Fatal(err)
		}
		testeapoio.ConferirJSON(t, problema.Errors, erros)
	}
	validacao("/api/contas?incluirArquivadas=talvez", `{"incluirArquivadas":["The value 'talvez' is not valid."]}`)
	validacao("/api/contas?incluirArquivadas=", `{"incluirArquivadas":["The value '' is invalid."]}`)
	validacao("/api/contas/faturas-do-mes?ano=x&mes=dez", `{"ano":["The value 'x' is not valid."],"mes":["The value 'dez' is not valid."]}`)
	// O int da query também vem em hexadecimal, como no TypeConverter do ASP.NET.
	testeapoio.Chamar(t, mux, "GET", "/api/contas/faturas-do-mes?ano=0x7EA&mes=10", "").ConferirStatus(t, http.StatusOK)
	// Os limites do período valem; sem o ano, o mês inválido ainda é o 400 do model binding.
	testeapoio.Chamar(t, mux, "GET", "/api/contas/faturas-do-mes?ano=2000&mes=1", "").ConferirStatus(t, http.StatusOK)
	testeapoio.Chamar(t, mux, "GET", "/api/contas/faturas-do-mes?ano=2100&mes=12", "").ConferirStatus(t, http.StatusOK)
	validacao("/api/contas/faturas-do-mes?mes=dez", `{"mes":["The value 'dez' is not valid."]}`)
	// Saldo inicial negativo só é recusado no investimento; investimento com saldo zero passa.
	testeapoio.Chamar(t, mux, "POST", "/api/contas", `{"nome":"Cheque especial","tipo":1,"saldoInicial":-5,"cor":"#000000"}`).
		ConferirStatus(t, http.StatusCreated)
	testeapoio.Chamar(t, mux, "POST", "/api/contas", `{"nome":"Corretora","tipo":4,"saldoInicial":0,"cor":"#000000"}`).
		ConferirStatus(t, http.StatusCreated)

	resp := testeapoio.Chamar(t, mux, "POST", "/api/contas", `{"tipo":1,"numero":"",
		"cor":"vermelho","diaFechamento":32,"diaVencimento":0,"banco":"1234567890123456789012345678901"}`)
	resp.ConferirStatus(t, http.StatusBadRequest)
	var problema struct{ Errors json.RawMessage }
	if err := json.Unmarshal(resp.Corpo, &problema); err != nil {
		t.Fatal(err)
	}
	testeapoio.ConferirJSON(t, problema.Errors, `{
		"Nome":["The Nome field is required."],
		"Cor":["The field Cor must match the regular expression '^#[0-9a-fA-F]{6}$'."],
		"DiaFechamento":["The field DiaFechamento must be between 1 and 31."],
		"DiaVencimento":["The field DiaVencimento must be between 1 and 31."],
		"Banco":["The field Banco must be a string with a maximum length of 30."]}`)
}

// Regra: editar a conta manual muda tudo (o número aparado; limite e dias só no cartão); a do Open Finance só muda
// nome, cor, incluir no saldo, arquivada, banco e a conta do cartão (o tipo, o número e o saldo inicial ficam os
// da instituição). A resposta traz o carimbo com "Z".
func TestEditar(t *testing.T) {
	mux, db, fab := rotasDeTeste(t)
	antigo := tipos.NovaDataHora(time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC))
	manual := fab.Conta(func(c *testeapoio.Conta) { c.Nome, c.SaldoInicial, c.AtualizadoEm = "Antiga", dec(t, "5.0"), antigo })
	vinculada := fab.Conta()
	testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/contas/%d", manual.Id), fmt.Sprintf(`{"nome":" Cartão Novo ",
		"tipo":3,"numero":"  ","saldoInicial":10,"limiteCredito":300.5,"diaFechamento":15,"diaVencimento":25,"cor":"#ABCDEF",
		"incluirNoSaldo":false,"arquivada":true,"banco":"outro","contaVinculadaId":%d}`, vinculada.Id)).
		ConferirJSON(t, fmt.Sprintf(`{"id":%d,"nome":"Cartão Novo","tipo":3,"origem":1,"numero":"","moeda":"BRL",
		"saldo":10.0,"saldoInicial":10,"limiteCredito":300.5,"limiteDisponivel":310.5,"diaFechamento":15,"diaVencimento":25,
		"cor":"#ABCDEF","incluirNoSaldo":false,"arquivada":true,"conexaoBancariaId":null,"nomeInstituicao":null,
		"atualizadoEm":"2026-10-15T15:00:00Z","banco":null,"bancoDefinido":true,"contaVinculadaId":%d,"faturaMes":null,
		"limiteAjustadoNoApp":null,"limiteDiferencaAvaliada":null}`, manual.Id, vinculada.Id))

	var tipoGravado, incluirGravado int64
	var atualizadoGravado string
	if err := db.QueryRow(`SELECT Tipo, IncluirNoSaldo, AtualizadoEm FROM Conta WHERE Id = ?`, manual.Id).
		Scan(&tipoGravado, &incluirGravado, &atualizadoGravado); err != nil {
		t.Fatal(err)
	}
	if tipoGravado != CartaoCredito || incluirGravado != 0 || atualizadoGravado != "2026-10-15 15:00:00" {
		t.Errorf("gravado: tipo %d, incluir no saldo %d, atualizado em %q", tipoGravado, incluirGravado, atualizadoGravado)
	}

	// Muda o que já tinha valor: número, banco, dia, limite e conta do cartão.
	outraVinculada := fab.Conta()
	testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/contas/%d", manual.Id), fmt.Sprintf(`{"nome":"Cartão Novo",
		"tipo":3,"numero":"9","saldoInicial":10,"limiteCredito":400,"diaFechamento":20,"diaVencimento":25,"cor":"#ABCDEF",
		"incluirNoSaldo":false,"arquivada":true,"banco":"nubank","contaVinculadaId":%d}`, outraVinculada.Id)).
		ConferirStatus(t, http.StatusOK)
	var numeroGravado, bancoGravado, limiteGravado string
	var diaGravado, vinculadaGravada int64
	if err := db.QueryRow(`SELECT Numero, Banco, LimiteCredito, DiaFechamento, ContaVinculadaId FROM Conta WHERE Id = ?`,
		manual.Id).Scan(&numeroGravado, &bancoGravado, &limiteGravado, &diaGravado, &vinculadaGravada); err != nil {
		t.Fatal(err)
	}
	if numeroGravado != "9" || bancoGravado != "nubank" || limiteGravado != "400.0" || diaGravado != 20 ||
		vinculadaGravada != outraVinculada.Id {
		t.Errorf("gravado: %q %q %q %d %d", numeroGravado, bancoGravado, limiteGravado, diaGravado, vinculadaGravada)
	}

	// Volta a ser conta corrente: limite, dias e conta do cartão saem.
	testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/contas/%d", manual.Id), `{"nome":"Corrente","tipo":1,
		"limiteCredito":300.5,"diaFechamento":15,"cor":"#ABCDEF","incluirNoSaldo":true}`).ConferirStatus(t, http.StatusOK)
	var limite, numero, dia, contaVinculada *string
	if err := db.QueryRow(`SELECT LimiteCredito, Numero, DiaFechamento, ContaVinculadaId FROM Conta WHERE Id = ?`, manual.Id).
		Scan(&limite, &numero, &dia, &contaVinculada); err != nil || limite != nil || numero != nil || dia != nil || contaVinculada != nil {
		t.Errorf("corrente com limite %v, número %v, dia %v, conta do cartão %v (%v)", limite, numero, dia, contaVinculada, err)
	}

	conexao := fab.ConexaoBancaria(func(c *testeapoio.ConexaoBancaria) { c.NomeInstituicao = p("Banco Inter") })
	daConexao := fab.Conta(func(c *testeapoio.Conta) {
		c.Nome, c.Tipo, c.Origem, c.ConexaoBancariaId, c.Numero = "Cartão", CartaoCredito, OpenFinance, &conexao.Id, p("**** 1")
		c.LimiteCredito, c.DiaFechamento = p(dec(t, "1000.0")), p(int64(3))
	})
	testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/contas/%d", daConexao.Id), fmt.Sprintf(`{"nome":"Meu cartão","tipo":1,
		"numero":"9","saldoInicial":99,"limiteCredito":1,"diaFechamento":20,"cor":"#111111","incluirNoSaldo":true,
		"banco":"itau","contaVinculadaId":%d}`, vinculada.Id)).ConferirStatus(t, http.StatusOK)
	var nome, cor, banco, num, limiteOF string
	var tipo, diaOF, vinc int64
	if err := db.QueryRow(`SELECT Nome, Cor, Banco, Numero, LimiteCredito, Tipo, DiaFechamento, ContaVinculadaId FROM Conta
		WHERE Id = ?`, daConexao.Id).Scan(&nome, &cor, &banco, &num, &limiteOF, &tipo, &diaOF, &vinc); err != nil {
		t.Fatal(err)
	}
	if nome != "Meu cartão" || cor != "#111111" || banco != "itau" || vinc != vinculada.Id || num != "**** 1" ||
		limiteOF != "1000.0" || tipo != CartaoCredito || diaOF != 3 {
		t.Errorf("Open Finance gravado: %q %q %q %q %q %d %d %d", nome, cor, banco, num, limiteOF, tipo, diaOF, vinc)
	}
}

// Regra: gravar manda só as colunas que mudaram (o SaveChanges do EF Core): a mudança de outra pessoa numa coluna que
// esta edição não mudou fica, decimal compara pelo valor (0 e 0.0 são iguais) e, sem mudança nenhuma, não há UPDATE.
func TestGravarSoOQueMudou(t *testing.T) {
	_, db, fab := rotasDeTeste(t)
	criada := fab.Conta(func(c *testeapoio.Conta) { c.SaldoInicial = dec(t, "0.0") })
	antes, err := lerConta(t.Context(), db, criada.Id)
	if err != nil || antes == nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Conta SET Nome = 'outra pessoa', Cor = '#999999' WHERE Id = ?`, criada.Id); err != nil {
		t.Fatal(err)
	}
	depois := *antes
	depois.SaldoInicial = tipos.Zero // o mesmo valor, outra escala
	if err := gravar(t.Context(), db, *antes, depois); err != nil {
		t.Fatal(err)
	}
	depois.Cor = "#000000"
	depois.Banco = p("nubank")
	if err := gravar(t.Context(), db, *antes, depois); err != nil {
		t.Fatal(err)
	}
	var nome, cor, saldoInicial, banco string
	if err := db.QueryRow(`SELECT Nome, Cor, SaldoInicial, Banco FROM Conta WHERE Id = ?`, criada.Id).
		Scan(&nome, &cor, &saldoInicial, &banco); err != nil {
		t.Fatal(err)
	}
	if nome != "outra pessoa" || cor != "#000000" || saldoInicial != "0.0" || banco != "nubank" {
		t.Errorf("gravado: %q %q %q %q", nome, cor, saldoInicial, banco)
	}
}

// Regra: excluir a conta manual leva as transações dela (e as faturas, em cascata, sem quebrar a FK das transações
// ligadas a elas) e deixa sem conta vinculada os cartões que pertenciam a ela; as outras contas não mudam.
func TestExcluir(t *testing.T) {
	mux, db, fab := rotasDeTeste(t)
	corrente := fab.Conta()
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Tipo, c.ContaVinculadaId = CartaoCredito, &corrente.Id })
	outro := fab.Conta(func(c *testeapoio.Conta) { c.Tipo = CartaoCredito })
	fatura := fab.Fatura(cartao, tipos.NovaData(2026, 10, 10), dec(t, "100.0"))
	fab.Transacao(cartao, func(x *testeapoio.Transacao) { x.FaturaCartaoId = &fatura.Id })
	fab.Transacao(corrente)
	fab.Transacao(outro)

	testeapoio.Chamar(t, mux, "DELETE", fmt.Sprintf("/api/contas/%d", corrente.Id), "").ConferirStatus(t, http.StatusNoContent)
	var vinculada *int64
	if err := db.QueryRow(`SELECT ContaVinculadaId FROM Conta WHERE Id = ?`, cartao.Id).Scan(&vinculada); err != nil || vinculada != nil {
		t.Errorf("cartão com conta vinculada %v (%v)", vinculada, err)
	}
	testeapoio.Chamar(t, mux, "DELETE", fmt.Sprintf("/api/contas/%d", cartao.Id), "").ConferirStatus(t, http.StatusNoContent)
	var contas, transacoes, faturas int
	if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM Conta), (SELECT COUNT(*) FROM Transacao), (SELECT COUNT(*) FROM FaturaCartao)`).
		Scan(&contas, &transacoes, &faturas); err != nil {
		t.Fatal(err)
	}
	if contas != 1 || transacoes != 1 || faturas != 0 {
		t.Errorf("sobraram %d contas, %d transações e %d faturas; esperado 1, 1 e 0", contas, transacoes, faturas)
	}
}

// Regra: as faturas do cartão: as 24 de vencimento mais novo, com a quantidade de transações ligadas, o pago (o maior
// entre o informado e o lançado no cartão desde o vencimento anterior) e a situação; conta sem faturas, lista vazia.
func TestFaturasDaConta(t *testing.T) {
	mux, _, fab := rotasDeTeste(t)
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Tipo = CartaoCredito })
	var ultima testeapoio.FaturaCartao
	for i := range 26 {
		vencimento := tipos.NovaData(2024, 9, 10).SomarMeses(i)
		ultima = fab.Fatura(cartao, vencimento, dec(t, "100.0"), func(f *testeapoio.FaturaCartao) {
			f.DataFechamento = p(vencimento.SomarDias(-7))
			if i >= 3 && i < 24 { // a mais velha da lista (i = 2) fica sem o pago informado
				f.ValorPago = p(dec(t, "100.0"))
			}
		})
	}
	fab.Transacao(cartao, func(x *testeapoio.Transacao) { x.FaturaCartaoId = &ultima.Id })
	fab.Transacao(cartao, func(x *testeapoio.Transacao) { x.FaturaCartaoId = &ultima.Id })
	// Pagamento lançado no cartão para a mais velha da lista: os pagamentos são lidos desde 2 meses antes dela.
	fab.Transacao(cartao, func(x *testeapoio.Transacao) {
		x.Valor, x.Tipo, x.Data = dec(t, "25.00"), 3, tipos.NovaData(2024, 11, 1)
	})
	// Pagamento lançado no cartão (entrada que virou transferência) para a penúltima, que o banco não informou pago.
	fab.Transacao(cartao, func(x *testeapoio.Transacao) {
		x.Valor, x.Tipo, x.Data = dec(t, "40.00"), 3, tipos.NovaData(2026, 9, 1)
	})
	resp := testeapoio.Chamar(t, mux, "GET", fmt.Sprintf("/api/contas/%d/faturas", cartao.Id), "")
	var lista []struct {
		Id                   int64
		DataVencimento       string
		ValorPago            json.RawMessage
		QuantidadeTransacoes int
		Situacao             string
	}
	if err := json.Unmarshal(resp.Corpo, &lista); err != nil {
		t.Fatal(err)
	}
	if len(lista) != 24 || lista[0].Id != ultima.Id || lista[0].DataVencimento != "2026-10-10" || lista[23].DataVencimento != "2024-11-10" {
		t.Fatalf("%d faturas, da %v à %v", len(lista), lista[0], lista[len(lista)-1])
	}
	if lista[0].QuantidadeTransacoes != 2 || string(lista[0].ValorPago) != "null" || lista[0].Situacao != "fechada" {
		t.Errorf("a última: %+v (%s)", lista[0], lista[0].ValorPago)
	}
	if string(lista[1].ValorPago) != "40.0" || lista[1].Situacao != "fechada" {
		t.Errorf("a penúltima, paga em parte no cartão: %+v (%s)", lista[1], lista[1].ValorPago)
	}
	if string(lista[2].ValorPago) != "100.0" || lista[2].Situacao != "paga" {
		t.Errorf("a antepenúltima, paga: %+v (%s)", lista[2], lista[2].ValorPago)
	}
	if string(lista[23].ValorPago) != "25.0" || lista[23].Situacao != "fechada" {
		t.Errorf("a mais velha, paga em parte no cartão: %+v (%s)", lista[23], lista[23].ValorPago)
	}
	semFaturas := fab.Conta()
	testeapoio.Chamar(t, mux, "GET", fmt.Sprintf("/api/contas/%d/faturas", semFaturas.Id), "").ConferirJSON(t, `[]`)
}

// Regra: a resposta do limite ajustado guarda a diferença de agora (limite − disponível − dívida, com a dívida zero
// quando o saldo é positivo); "não" guarda a diferença também; null desfaz as duas; sem limite disponível (cartão
// manual), a diferença fica null. O 204 não tem corpo.
func TestLimiteAjustado(t *testing.T) {
	mux, db, fab := rotasDeTeste(t)
	devendo := fab.Conta(func(c *testeapoio.Conta) {
		c.Tipo, c.Origem, c.Saldo = CartaoCredito, OpenFinance, dec(t, "-299.9")
		c.LimiteCredito, c.LimiteDisponivel = p(dec(t, "12000.0")), p(dec(t, "11000.1"))
	})
	comCredito := fab.Conta(func(c *testeapoio.Conta) {
		c.Tipo, c.Origem, c.Saldo = CartaoCredito, OpenFinance, dec(t, "50.0")
		c.LimiteCredito, c.LimiteDisponivel = p(dec(t, "1000.0")), p(dec(t, "900.0"))
	})
	manual := fab.Conta(func(c *testeapoio.Conta) { c.Tipo, c.LimiteCredito = CartaoCredito, p(dec(t, "500.0")) })
	estado := func(id int64) string {
		t.Helper()
		var ajustado, diferenca *string
		if err := db.QueryRow(`SELECT LimiteAjustadoNoApp, LimiteDiferencaAvaliada FROM Conta WHERE Id = ?`, id).
			Scan(&ajustado, &diferenca); err != nil {
			t.Fatal(err)
		}
		texto := func(s *string) string {
			if s == nil {
				return "null"
			}
			return *s
		}
		return texto(ajustado) + " " + texto(diferenca)
	}
	responder := func(id int64, corpo, esperado string) {
		t.Helper()
		resp := testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/contas/%d/limite-ajustado", id), corpo)
		resp.ConferirStatus(t, http.StatusNoContent)
		if len(resp.Corpo) != 0 {
			t.Errorf("204 com corpo %s", resp.Corpo)
		}
		if got := estado(id); got != esperado {
			t.Errorf("%s: %s, esperado %s", corpo, got, esperado)
		}
	}
	responder(devendo.Id, `{"ajustado":true}`, "1 700.0")
	responder(devendo.Id, `{"ajustado":false}`, "0 700.0")
	responder(devendo.Id, `{"ajustado":null}`, "null null")
	responder(comCredito.Id, `{"ajustado":true}`, "1 100.0")
	responder(manual.Id, `{"ajustado":true}`, "1 null")
	responder(manual.Id, `{}`, "null null")
}

// Regra: as duas rotas sem regra própria: os bancos são o catálogo, na ordem dele, com o "outro" no fim; a fatura do
// mês escolhido usa o mesmo cálculo das faturas.
func TestBancosEFaturasDoMes(t *testing.T) {
	mux, _, fab := rotasDeTeste(t)
	resp := testeapoio.Chamar(t, mux, "GET", "/api/contas/bancos", "")
	var lista []struct{ Codigo, Nome string }
	if err := json.Unmarshal(resp.Corpo, &lista); err != nil {
		t.Fatal(err)
	}
	if len(lista) != 29 || lista[0].Codigo != "nubank" || lista[1].Nome != "Itaú" || lista[28].Codigo != "outro" {
		t.Errorf("bancos: %d, %v ... %v", len(lista), lista[0], lista[len(lista)-1])
	}
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Nome, c.Tipo, c.DiaFechamento = "Cartão", CartaoCredito, p(int64(10)) })
	testeapoio.Chamar(t, mux, "GET", "/api/contas/faturas-do-mes?ano=2026&mes=3", "").ConferirJSON(t, fmt.Sprintf(`[{
		"contaId":%d,"cartao":"Cartão","valorMes":0,"parcelasMes":0,"faturaId":null,"vencimento":null,"situacao":null,
		"banco":null,"fechamento":"2026-03-10","proxima":null}]`, cartao.Id))
}

// Regra: erro do banco é o 500 genérico, em todas as rotas que leem.
func TestErroDoBanco(t *testing.T) {
	mux, db, _ := rotasDeTeste(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ metodo, caminho, corpo string }{
		{"GET", "/api/contas", ""},
		{"GET", "/api/contas/1", ""},
		{"GET", "/api/contas/faturas-do-mes?ano=2026&mes=10", ""},
		{"POST", "/api/contas", `{"nome":"X","tipo":1,"cor":"#000000"}`},
		{"POST", "/api/contas", `{"nome":"X","tipo":3,"cor":"#000000","contaVinculadaId":1}`},
		{"PUT", "/api/contas/1", `{"nome":"X","tipo":1,"cor":"#000000"}`},
		{"DELETE", "/api/contas/1", ""},
		{"GET", "/api/contas/1/faturas", ""},
		{"PUT", "/api/contas/1/limite-ajustado", `{"ajustado":true}`},
	} {
		testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo).ConferirStatus(t, http.StatusInternalServerError)
	}
}

// Regra: o limite disponível só é recalculado (limite + saldo) no cartão manual; o do Open Finance, cartão ou conta, é
// o que a instituição informa.
func TestLimiteDisponivelDoOpenFinance(t *testing.T) {
	mux, _, fab := rotasDeTeste(t)
	conexao := fab.ConexaoBancaria()
	cartao := fab.Conta(func(c *testeapoio.Conta) {
		c.Tipo, c.Origem, c.ConexaoBancariaId, c.Saldo = CartaoCredito, OpenFinance, &conexao.Id, dec(t, "-300.0")
		c.LimiteCredito, c.LimiteDisponivel = p(dec(t, "1000.0")), p(dec(t, "650.0"))
	})
	corrente := fab.Conta(func(c *testeapoio.Conta) {
		c.Origem, c.ConexaoBancariaId, c.Saldo = OpenFinance, &conexao.Id, dec(t, "100.0")
		c.LimiteCredito, c.LimiteDisponivel = p(dec(t, "500.0")), p(dec(t, "300.0"))
	})
	for id, esperado := range map[int64]string{cartao.Id: "650.0", corrente.Id: "300.0"} {
		resp := testeapoio.Chamar(t, mux, "GET", fmt.Sprintf("/api/contas/%d", id), "")
		var dto struct{ LimiteDisponivel json.RawMessage }
		if err := json.Unmarshal(resp.Corpo, &dto); err != nil {
			t.Fatal(err)
		}
		if string(dto.LimiteDisponivel) != esperado {
			t.Errorf("conta %d: limite disponível %s, esperado %s", id, dto.LimiteDisponivel, esperado)
		}
	}
}
