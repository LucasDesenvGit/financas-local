package carteira

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Regra (#80, o ComposicaoAsync do C#): a carteira vale a última cotação (sem cotação, o custo; vendida inteira, nada);
// o Open Finance das conexões ativas entra em reais, sem repetir o que já está na carteira pelo código (sem diferença de
// maiúsculas), a ação e o ETF como renda variável e o resto como "demais"; a moeda sem cotação fica de fora e é
// anotada; as reservas entram pelo total delas; o total soma as quatro partes.
func TestComposicao(t *testing.T) {
	t.Parallel()
	conversor := cambio.NovoConversor([]cambio.Guardada{{Moeda: "USD", Compra: testeapoio.Valor("5.00"), Venda: testeapoio.Valor("5.10")}}, nil)
	c := novaCena(t, cambioFalso{conversor: conversor}, reservasFalsas{total: testeapoio.Valor("3.0")}, selicFalsa{})
	preco := testeapoio.Valor("150.0")
	comPreco := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker, a.PrecoAtual = "HGLG11", &preco })
	c.fab.OperacaoAtivo(comPreco, func(o *testeapoio.OperacaoAtivo) {
		o.Quantidade, o.PrecoUnitario = testeapoio.Valor("2"), testeapoio.Valor("140.0")
	})
	semPreco := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "XPTO3" })
	c.fab.OperacaoAtivo(semPreco, func(o *testeapoio.OperacaoAtivo) {
		o.Quantidade, o.PrecoUnitario = testeapoio.Valor("1"), testeapoio.Valor("10.0")
	})
	vendido := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker, a.PrecoAtual = "VEND3", &preco })
	c.fab.OperacaoAtivo(vendido, func(o *testeapoio.OperacaoAtivo) { o.Quantidade = testeapoio.Valor("1") })
	c.fab.OperacaoAtivo(vendido, func(o *testeapoio.OperacaoAtivo) { o.Tipo, o.Quantidade = 2, testeapoio.Valor("1") })

	ativa := c.fab.ConexaoBancaria()
	inativa := c.fab.ConexaoBancaria(func(x *testeapoio.ConexaoBancaria) { x.Ativa = false })
	investimento := func(conexao testeapoio.ConexaoBancaria, tipo, moeda, saldo string, codigo *string) {
		c.fab.Investimento(conexao, func(i *testeapoio.Investimento) {
			i.Tipo, i.Moeda, i.Saldo, i.Codigo = tipo, moeda, testeapoio.Valor(saldo), codigo
		})
	}
	codigo := func(s string) *string { return &s }
	investimento(ativa, "EQUITY", "BRL", "999.0", codigo("hglg11")) // já está na carteira
	investimento(ativa, "EQUITY", "BRL", "5.0", codigo("VEND3"))    // vendido na carteira: entra
	investimento(ativa, "etf", "BRL", "70.0", nil)
	investimento(ativa, "FIXED_INCOME", "BRL", "40.0", nil)
	investimento(ativa, "FIXED_INCOME", "USD", "2.0", nil)
	investimento(ativa, "FIXED_INCOME", "GBP", "7.0", nil)
	investimento(inativa, "FIXED_INCOME", "BRL", "60.0", nil)

	comp, err := c.s.Composicao(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	igual := func(a tipos.Decimal, b string) bool { return a.Decimal().Equal(testeapoio.Valor(b).Decimal()) }
	if !igual(comp.Carteira, "310") || !igual(comp.OpenFinanceRendaVariavel, "75") || !igual(comp.OpenFinanceDemais, "50") ||
		!igual(comp.Reservas, "3") || !igual(comp.Total(), "438") || !igual(comp.RendaVariavel(), "385") ||
		strings.Join(comp.MoedasSemCotacao, ",") != "GBP" {
		t.Errorf("composição: %+v total %s", comp, comp.Total())
	}

	for k, falha := range []cena{
		novaCena(t, cambioFalso{erro: errors.New("sem câmbio")}, reservasFalsas{}, selicFalsa{}),
		novaCena(t, cambioFalso{}, reservasFalsas{erro: errors.New("sem reservas")}, selicFalsa{}),
	} {
		if _, err := falha.s.Composicao(t.Context()); err == nil {
			t.Errorf("cena %d: o erro sumiu", k)
		}
	}
	fechado := cenaSimples(t)
	_ = fechado.db.Close()
	if _, err := fechado.s.Composicao(t.Context()); err == nil {
		t.Error("banco fechado: o erro sumiu")
	}
}
