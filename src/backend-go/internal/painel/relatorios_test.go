package painel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/carteira"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func dec(s string) tipos.Decimal { return testeapoio.Valor(s) }

type composicaoFalsa struct {
	c   carteira.Composicao
	err error
}

func (f composicaoFalsa) Composicao(context.Context) (carteira.Composicao, error) { return f.c, f.err }

type reservasFalsas struct {
	total tipos.Decimal
	err   error
}

func (f reservasFalsas) Total(context.Context) (tipos.Decimal, error) { return f.total, f.err }

// cambioFalso tem o dólar a 5,00; sem outra moeda.
type cambioFalso struct{ err error }

func (f cambioFalso) ObterConversor(context.Context, *tipos.Data) (*cambio.Conversor, error) {
	if f.err != nil {
		return nil, f.err
	}
	return cambio.NovoConversor([]cambio.Guardada{{Moeda: "USD", Compra: dec("5.00"), Venda: dec("5.10")}}, nil), nil
}

type cena struct {
	db  *sql.DB
	fab *testeapoio.Fabrica
	mux *http.ServeMux
	s   *Servico
}

func novaCena(t *testing.T, c composicao, r totalDasReservas, cb conversao) cena {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 10, 14)
	s := NovoServico(db, c, r, cb, rel, slog.New(slog.DiscardHandler))
	mux := http.NewServeMux()
	Registrar(mux, s)
	return cena{db, testeapoio.NovaFabrica(t, db, rel), mux, s}
}

func cenaPadrao(t *testing.T) cena {
	return novaCena(t, composicaoFalsa{c: carteira.Composicao{Carteira: dec("100.0"), OpenFinanceRendaVariavel: dec("10.0"),
		OpenFinanceDemais: dec("20.0"), Reservas: dec("5.0")}}, reservasFalsas{total: dec("5.0")}, cambioFalso{})
}

// Regra (#80): a query é lida como o model binding do ASP.NET: o que não vira o tipo é 400 com o nome do parâmetro (a
// data inválida, a vazia, o enum que não existe, o booleano e o número que não são); depois vêm as validações de
// cada rota, com as mensagens do C#.
func TestValidacoesDaQuery(t *testing.T) {
	t.Parallel()
	c := cenaPadrao(t)
	casos := []struct {
		caminho, campo, mensagem string
	}{
		{"/api/relatorios/categorias?inicio=ontem&fim=2026-10-14", "inicio", "The value 'ontem' is not valid."},
		{"/api/relatorios/categorias?inicio=&fim=2026-10-14", "inicio", "The value '' is invalid."},
		{"/api/relatorios/categorias?inicio=2026-01-01&fim=2026-01-02&tipo=9", "tipo", "The value '9' is invalid."},
		{"/api/relatorios/categorias?inicio=2026-01-01&fim=2026-01-02&tipo=xyz", "tipo", "The value 'xyz' is not valid."},
		{"/api/relatorios/categorias?inicio=2026-01-01&fim=2026-01-02&agrupar=talvez", "agrupar", "The value 'talvez' is not valid."},
		{"/api/relatorios/fluxo?inicio=2026-01-01&fim=x", "fim", "The value 'x' is not valid."},
		{"/api/relatorios/anual?ano=dois", "ano", "The value 'dois' is not valid."},
		{"/api/relatorios/projecao?meses=seis", "meses", "The value 'seis' is not valid."},
		{"/api/relatorios/maiores-despesas?inicio=1/2/3", "inicio", "The value '1/2/3' is not valid."},
	}
	for _, caso := range casos {
		r := testeapoio.Chamar(t, c.mux, "GET", caso.caminho, "")
		var corpo struct {
			Errors map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(r.Corpo, &corpo)
		if r.Status != 400 || len(corpo.Errors[caso.campo]) != 1 || corpo.Errors[caso.campo][0] != caso.mensagem {
			t.Errorf("%s: %d %s", caso.caminho, r.Status, r.Corpo)
		}
	}
	mensagens := []struct{ caminho, mensagem string }{
		{"/api/relatorios/categorias?inicio=2026-02-01&fim=2026-01-31", "Período inválido."},
		{"/api/relatorios/categorias?inicio=2016-01-01&fim=2026-01-31", "Período inválido."},
		{"/api/relatorios/categorias?inicio=2026-01-01&fim=2026-01-31&tipo=Transferencia", "Tipo inválido."},
		{"/api/relatorios/fluxo?inicio=2026-02-01&fim=2026-01-31", "Período inválido."},
		{"/api/relatorios/fluxo?inicio=2016-01-01&fim=2026-01-31", "Período inválido."},
		{"/api/relatorios/anual?ano=1999", "Ano inválido."},
		{"/api/relatorios/anual?ano=2101", "Ano inválido."},
		{"/api/relatorios/anual?ano=2026&comparar=1999", "Ano inválido."},
		{"/api/relatorios/anual?comparar=2101", "Ano inválido."},
		{"/api/relatorios/projecao?meses=0", "Informe entre 1 e 24 meses."},
		{"/api/relatorios/projecao?meses=25", "Informe entre 1 e 24 meses."},
		{"/api/relatorios/maiores-despesas?inicio=2026-02-01&fim=2026-01-31", "Período inválido."},
	}
	for _, m := range mensagens {
		r := testeapoio.Chamar(t, c.mux, "GET", m.caminho, "")
		var corpo map[string]string
		_ = json.Unmarshal(r.Corpo, &corpo)
		if r.Status != 400 || corpo["mensagem"] != m.mensagem {
			t.Errorf("%s: %d %s", m.caminho, r.Status, r.Corpo)
		}
	}
	// Os limites que passam: 3660 dias e 1 e 24 meses; sem as datas, o DateOnly vazio (01/01/0001) nos dois.
	for _, caminho := range []string{
		"/api/relatorios/categorias?inicio=2016-01-01&fim=2026-01-08&tipo=receita&agrupar=false",
		"/api/relatorios/categorias", "/api/relatorios/fluxo?inicio=2026-01-01&fim=2026-03-31",
		"/api/relatorios/projecao?meses=1", "/api/relatorios/projecao?meses=24", "/api/relatorios/projecao",
		"/api/relatorios/anual?ano=2025&comparar=2000", "/api/relatorios/anual?ano=&comparar=",
		"/api/relatorios/anual?ano=2000&comparar=2100", "/api/relatorios/anual?ano=2100&comparar=2000",
		"/api/relatorios/maiores-despesas?inicio=2026-01-01&fim=2026-01-01",
	} {
		if r := testeapoio.Chamar(t, c.mux, "GET", caminho, ""); r.Status != 200 {
			t.Errorf("%s: %d %s", caminho, r.Status, r.Corpo)
		}
	}
}

// Regra (#80): o patrimônio de hoje são as contas que entram no saldo (o cartão só com a dívida, −max(0, −saldo); a
// arquivada e a fora do saldo não contam) mais os investimentos da carteira menos as dívidas de crédito; a reserva
// líquida, as contas que não são cartão (só o positivo) mais o Open Finance que não é renda variável (em reais; sem
// cotação, de fora; conexão inativa, de fora) mais as reservas.
func TestPatrimonio(t *testing.T) {
	t.Parallel()
	c := cenaPadrao(t)
	saldo := func(v string) func(*testeapoio.Conta) {
		return func(conta *testeapoio.Conta) { conta.SaldoInicial = dec(v) }
	}
	// O cartão gravado antes das contas: a conta depois dele também entra.
	c.fab.Conta(func(conta *testeapoio.Conta) { conta.Tipo, conta.SaldoInicial = tipoContaCartao, dec("-300.0") })
	c.fab.Conta(saldo("1000.0"))
	c.fab.Conta(saldo("-200.0"))
	c.fab.Conta(func(conta *testeapoio.Conta) { conta.Tipo, conta.SaldoInicial = tipoContaCartao, dec("50.0") })
	c.fab.Conta(func(conta *testeapoio.Conta) { conta.SaldoInicial, conta.Arquivada = dec("999.0"), true })
	c.fab.Conta(func(conta *testeapoio.Conta) { conta.SaldoInicial, conta.IncluirNoSaldo = dec("888.0"), false })
	ativa := c.fab.ConexaoBancaria()
	inativa := c.fab.ConexaoBancaria(func(x *testeapoio.ConexaoBancaria) { x.Ativa = false })
	// A ação gravada antes: a renda fixa depois dela também entra na reserva.
	c.fab.Investimento(ativa, func(i *testeapoio.Investimento) { i.Tipo, i.Saldo = "equity", dec("70.0") })
	c.fab.Investimento(ativa, func(i *testeapoio.Investimento) { i.Tipo, i.Saldo = "FIXED_INCOME", dec("40.0") })
	c.fab.Investimento(ativa, func(i *testeapoio.Investimento) { i.Tipo, i.Moeda, i.Saldo = "FIXED_INCOME", "USD", dec("2.0") })
	c.fab.Investimento(ativa, func(i *testeapoio.Investimento) { i.Tipo, i.Moeda, i.Saldo = "FIXED_INCOME", "GBP", dec("7.0") })
	c.fab.Investimento(inativa, func(i *testeapoio.Investimento) { i.Tipo, i.Saldo = "FIXED_INCOME", dec("60.0") })
	c.fab.Emprestimo(ativa, func(e *testeapoio.Emprestimo) { e.Natureza, e.SaldoDevedor = 2, dec("15.0") })
	c.fab.Emprestimo(ativa, func(e *testeapoio.Emprestimo) { e.SaldoDevedor = dec("5.0") })

	p, err := c.s.patrimonio(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// Contas: 1000 − 200 − 300 − 0 = 500; + composição 135; − 15 − 5 = 615. Reserva: 1000 + 0 (positivas) + 40 + 10.00
	// (2 dólares) + 5 (reservas) = 1055.00.
	if p.Atual.String() != "615.0" || p.Reserva.String() != "1055.00" || p.Credito.SaldoFinanciamentos.String() != "15.0" ||
		p.Credito.SaldoEmprestimos.String() != "5.0" {
		t.Errorf("patrimônio %s, reserva %s, crédito %+v", p.Atual, p.Reserva, p.Credito)
	}
	r := testeapoio.Chamar(t, c.mux, "GET", "/api/relatorios/anual", "")
	var anual relatorios.PainelAnualDto
	if err := json.Unmarshal(r.Corpo, &anual); err != nil || r.Status != 200 || anual.Saude.PatrimonioAtual.String() != "615.0" ||
		anual.Ano != 2026 || anual.AnoComparacao != 2025 {
		t.Errorf("anual: %d %s", r.Status, r.Corpo)
	}
}

// Regra: as maiores despesas vêm do período, na ordem do EF Core (contrato 14-relatorios).
func TestMaioresDespesas(t *testing.T) {
	t.Parallel()
	c := cenaPadrao(t)
	conta := c.fab.Conta()
	for _, v := range []string{"-27.45", "-223.5", "-1850.0"} {
		c.fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor = testeapoio.Dia(2026, 9, 8), dec(v) })
	}
	r := testeapoio.Chamar(t, c.mux, "GET", "/api/relatorios/maiores-despesas?inicio=2026-09-01&fim=2026-09-30", "")
	var lista []struct {
		Valor tipos.Decimal `json:"valor"`
	}
	_ = json.Unmarshal(r.Corpo, &lista)
	var valores []string
	for _, l := range lista {
		valores = append(valores, l.Valor.String())
	}
	if strings.Join(valores, " ") != "-1850.0 -27.45 -223.5" {
		t.Errorf("maiores despesas: %s", r.Corpo)
	}
}

// Regra: falha do banco, da carteira, das reservas ou do câmbio é 500 com a mensagem genérica.
func TestErrosDosRelatorios(t *testing.T) {
	t.Parallel()
	falha := errors.New("falha de teste")
	cenas := []cena{
		novaCena(t, composicaoFalsa{err: falha}, reservasFalsas{}, cambioFalso{}),
		novaCena(t, composicaoFalsa{}, reservasFalsas{err: falha}, cambioFalso{}),
		novaCena(t, composicaoFalsa{}, reservasFalsas{}, cambioFalso{err: falha}),
	}
	for k, c := range cenas {
		if r := testeapoio.Chamar(t, c.mux, "GET", "/api/relatorios/anual", ""); r.Status != 500 {
			t.Errorf("cena %d: %d", k, r.Status)
		}
	}
	c := cenaPadrao(t)
	_ = c.db.Close()
	for _, caminho := range []string{
		"/api/relatorios/categorias?inicio=2026-01-01&fim=2026-01-31", "/api/relatorios/fluxo?inicio=2026-01-01&fim=2026-01-31",
		"/api/relatorios/anual", "/api/relatorios/projecao", "/api/relatorios/maiores-despesas?inicio=2026-01-01&fim=2026-01-31",
	} {
		if r := testeapoio.Chamar(t, c.mux, "GET", caminho, ""); r.Status != 500 {
			t.Errorf("%s: %d", caminho, r.Status)
		}
	}
}
