package investimentos

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func dec(s string) tipos.Decimal { return testeapoio.Valor(s) }

func ptr[T any](v T) *T { return &v }

// cambioFalso dá um conversor com o dólar a 5,00 (ou o erro, se houver).
type cambioFalso struct{ err error }

func (c cambioFalso) ObterConversor(context.Context, *tipos.Data) (*cambio.Conversor, error) {
	if c.err != nil {
		return nil, c.err
	}
	return cambio.NovoConversor([]cambio.Guardada{{Moeda: "USD", Compra: dec("5.00"), Venda: dec("5.10")}}, nil), nil
}

func ambiente(t *testing.T, r relogio.Relogio, c conversao) (*sql.DB, *testeapoio.Fabrica, *http.ServeMux, *Servico) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	s := NovoServico(db, c, r, slog.New(slog.DiscardHandler))
	mux := http.NewServeMux()
	Registrar(mux, s)
	return db, testeapoio.NovaFabrica(t, db, r), mux, s
}

func lerJSON[T any](t *testing.T, corpo []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(corpo, &v); err != nil {
		t.Fatalf("%v: %s", err, corpo)
	}
	return v
}

// Regra (#170): a lista vai pelo tipo e pelo maior saldo, com o saldo em reais (a moeda sem cotação fica sem) e o nome
// de exibição; renomear apara o nome, vazio volta ao automático, e o que não existe é 404.
func TestListarERenomear(t *testing.T) {
	t.Parallel()
	_, fab, mux, _ := ambiente(t, testeapoio.HojeEmBrasilia(2026, 10, 14), cambioFalso{})
	conexao := fab.ConexaoBancaria(func(c *testeapoio.ConexaoBancaria) { c.NomeInstituicao = ptr("Banco Fictício") })
	pequeno := fab.Investimento(conexao, func(i *testeapoio.Investimento) { i.Nome, i.Tipo, i.Saldo = "Pequeno", "FIXED_INCOME", dec("10.0") })
	grande := fab.Investimento(conexao, func(i *testeapoio.Investimento) { i.Nome, i.Tipo, i.Saldo = "Grande", "FIXED_INCOME", dec("90.0") })
	dolar := fab.Investimento(conexao, func(i *testeapoio.Investimento) {
		i.Nome, i.Tipo, i.Moeda, i.Saldo = "Dólar", "EQUITY", "USD", dec("2.5")
	})
	libra := fab.Investimento(conexao, func(i *testeapoio.Investimento) {
		i.Nome, i.Tipo, i.Moeda, i.Saldo = "Libra", "MUTUAL_FUND", "GBP", dec("1.0")
	})
	lista := lerJSON[[]InvestimentoDto](t, testeapoio.Chamar(t, mux, "GET", "/api/investimentos", "").Corpo)
	var nomes []string
	for _, i := range lista {
		nomes = append(nomes, i.Nome)
	}
	if strings.Join(nomes, ",") != "Dólar,Grande,Pequeno,Libra" {
		t.Errorf("ordem: %v", nomes)
	}
	if lista[0].Id != dolar.Id || lista[0].SaldoEmReais == nil || lista[0].SaldoEmReais.String() != "12.50" ||
		lista[3].Id != libra.Id || lista[3].SaldoEmReais != nil || lista[1].NomeInstituicao == nil {
		t.Errorf("saldo em reais: %+v", lista)
	}
	id := func(n int64) string { b, _ := json.Marshal(n); return string(b) }
	casos := []struct {
		caminho, corpo string
		status         int
		mensagem       string
	}{
		{"/api/investimentos/" + id(grande.Id) + "/nome", `{"nome":"  Caixinha  "}`, 204, ""},
		{"/api/investimentos/" + id(pequeno.Id) + "/nome", `{"nome":"Outro"}`, 204, ""},
		{"/api/investimentos/" + id(pequeno.Id) + "/nome", `{"nome":"   "}`, 204, ""},
		{"/api/investimentos/999/nome", `{"nome":"X"}`, 404, "Investimento não encontrado."},
		{"/api/investimentos/x/nome", `{"nome":"X"}`, 404, ""},
		{"/api/investimentos/" + id(grande.Id) + "/nome", `{"nome":"` + strings.Repeat("a", 121) + `"}`, 400, ""},
	}
	for _, c := range casos {
		r := testeapoio.Chamar(t, mux, "PUT", c.caminho, c.corpo)
		if r.Status != c.status || (c.mensagem != "" && lerJSON[map[string]string](t, r.Corpo)["mensagem"] != c.mensagem) {
			t.Errorf("%s %s: %d %s", c.caminho, c.corpo, r.Status, r.Corpo)
		}
	}
	lista = lerJSON[[]InvestimentoDto](t, testeapoio.Chamar(t, mux, "GET", "/api/investimentos", "").Corpo)
	if lista[1].NomeExibicao != "Caixinha" || lista[1].NomePersonalizado == nil || lista[2].NomePersonalizado != nil {
		t.Errorf("depois de renomear: %+v %+v", lista[1], lista[2])
	}
}

func registrar(t *testing.T, db *sql.DB, id, nome, data, saldo string, aplicado *string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO "HistoricoInvestimento" ("PluggyInvestmentId", "ConexaoBancariaId", "Nome", "Tipo",
		"Data", "Saldo", "ValorAplicado", "AtualizadoEm") VALUES (?, 1, ?, 'FIXED_INCOME', ?, ?, ?, '2026-01-01 12:00:00')`,
		id, nome, data, saldo, aplicado); err != nil {
		t.Fatal(err)
	}
}

// Regra (#173 e), ADR 0045): o rendimento do mês é quanto (saldo − aplicado) cresceu desde o fim do mês anterior (o
// aporte não conta); sem valor aplicado, null; o saldo do mês é o do último dia com registro (o resgatado antes dele
// entra com 0); o mês sem registro fica de fora; o mês em que o histórico começa conta desde o primeiro registro.
func TestMesAMes(t *testing.T) {
	t.Parallel()
	db, fab, mux, s := ambiente(t, testeapoio.HojeEmBrasilia(2026, 10, 14), cambioFalso{})
	vazio, err := s.MesAMes(t.Context())
	if err != nil || vazio.Desde != nil || len(vazio.Meses) != 0 {
		t.Fatalf("sem histórico: %+v %v", vazio, err)
	}
	conexao := fab.ConexaoBancaria()
	fab.Investimento(conexao, func(i *testeapoio.Investimento) {
		i.PluggyInvestmentId, i.Nome, i.NomePersonalizado = "cdb", "CDB antigo", ptr("Caixinha")
	})
	registrar(t, db, "cdb", "CDB antigo", "2026-07-10", "1000.0", ptr("1000.0"))
	registrar(t, db, "cdb", "CDB antigo", "2026-07-31", "1010.0", ptr("1000.0"))
	registrar(t, db, "cdb", "CDB antigo", "2026-09-30", "1530.005", ptr("1500.0")) // aporte de 500 em agosto/setembro
	registrar(t, db, "fundo", "Fundo resgatado", "2026-07-15", "50.0", nil)
	registrar(t, db, "lci", "LCI", "2026-09-15", "300.0", ptr("300.0"))

	h := lerJSON[HistoricoInvestimentosDto](t, testeapoio.Chamar(t, mux, "GET", "/api/investimentos/historico", "").Corpo)
	if h.Desde == nil || h.Desde.String() != "2026-07-10" || len(h.Meses) != 2 {
		t.Fatalf("histórico: %+v", h)
	}
	julho, setembro := h.Meses[0], h.Meses[1]
	if julho.Mes != 7 || julho.Desde == nil || julho.Rendimento == nil || julho.Rendimento.String() != "10.0" ||
		julho.Saldo.String() != "1010.0" || len(julho.PorInvestimento) != 2 {
		t.Errorf("julho: %+v", julho)
	}
	// Julho: o CDB (renomeado: vale o nome atual) com 1010 e o fundo, resgatado antes do último dia, com 0 e sem
	// rendimento (sem valor aplicado).
	if j := julho.PorInvestimento; j[0].Nome != "Caixinha" || j[1].Nome != "Fundo resgatado" || j[1].Rendimento != nil ||
		j[1].Saldo.String() != "0" {
		t.Errorf("julho por investimento: %+v", j)
	}
	// Setembro: o CDB rendeu (1530.005 − 1500) − (1010 − 1000) = 20.005 → 20.00 (bancário); a LCI começou no mês
	// (rendimento 0) e não estava no último dia (30/09), então entra com saldo 0.
	if setembro.Mes != 9 || setembro.Desde != nil || setembro.Rendimento == nil || setembro.Rendimento.String() != "20.00" ||
		setembro.Saldo.String() != "1530.005" {
		t.Errorf("setembro: %+v", setembro)
	}
}

// Regra: a gravação do histórico (para a sincronização, #83) guarda o saldo de hoje de quem entra no histórico (não a
// renda variável nem outra moeda) e atualiza o registro do dia, se já houver.
func TestRegistrarHistorico(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	hoje := testeapoio.Dia(2026, 10, 14)
	agora := time.Date(2026, 10, 14, 15, 0, 0, 0, time.UTC)
	lista := []Investimento{
		{PluggyInvestmentId: "cdb", ConexaoBancariaId: 1, Nome: "CDB", Tipo: "FIXED_INCOME", Moeda: "BRL", Saldo: dec("100.0"),
			ValorAplicado: ptr(dec("90.0"))},
		{PluggyInvestmentId: "acao", ConexaoBancariaId: 1, Nome: "PETR4", Tipo: "EQUITY", Moeda: "BRL", Saldo: dec("50.0")},
		{PluggyInvestmentId: "etf", ConexaoBancariaId: 1, Nome: "IVVB11", Tipo: "ETF", Moeda: "BRL", Saldo: dec("50.0")},
		{PluggyInvestmentId: "usd", ConexaoBancariaId: 1, Nome: "Fundo", Tipo: "MUTUAL_FUND", Moeda: "USD", Saldo: dec("5.0")},
	}
	if err := RegistrarHistorico(t.Context(), db, lista, hoje, agora); err != nil {
		t.Fatal(err)
	}
	lista[0].Saldo = dec("101.5")
	if err := RegistrarHistorico(t.Context(), db, lista[:1], hoje, agora); err != nil {
		t.Fatal(err)
	}
	var n int
	var saldo, aplicado string
	_ = db.QueryRow(`SELECT COUNT(*) FROM "HistoricoInvestimento"`).Scan(&n)
	_ = db.QueryRow(`SELECT "Saldo", "ValorAplicado" FROM "HistoricoInvestimento" WHERE "PluggyInvestmentId" = 'cdb'`).Scan(&saldo, &aplicado)
	if n != 1 || saldo != "101.5" || aplicado != "90.0" {
		t.Errorf("histórico: %d registros, saldo %s, aplicado %s", n, saldo, aplicado)
	}
	if err := RegistrarHistorico(t.Context(), falha{db}, lista[:1], hoje, agora); err == nil {
		t.Error("o erro do banco sumiu")
	}
}

type falha struct{ *sql.DB }

func (falha) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, errors.New("falha")
}

// Regra: falha do banco ou do câmbio é 500 com a mensagem genérica.
func TestErros(t *testing.T) {
	t.Parallel()
	_, fab, mux, _ := ambiente(t, testeapoio.HojeEmBrasilia(2026, 10, 14), cambioFalso{err: errors.New("sem câmbio")})
	fab.Investimento(fab.ConexaoBancaria())
	if r := testeapoio.Chamar(t, mux, "GET", "/api/investimentos", ""); r.Status != http.StatusInternalServerError {
		t.Errorf("sem câmbio: %d", r.Status)
	}
	db, _, mux2, _ := ambiente(t, testeapoio.HojeEmBrasilia(2026, 10, 14), cambioFalso{})
	_ = db.Close()
	for _, c := range []struct{ metodo, caminho, corpo string }{
		{"GET", "/api/investimentos", ""},
		{"GET", "/api/investimentos/historico", ""},
		{"PUT", "/api/investimentos/1/nome", `{"nome":"X"}`},
	} {
		if r := testeapoio.Chamar(t, mux2, c.metodo, c.caminho, c.corpo); r.Status != http.StatusInternalServerError {
			t.Errorf("%s %s: %d", c.metodo, c.caminho, r.Status)
		}
	}
}
