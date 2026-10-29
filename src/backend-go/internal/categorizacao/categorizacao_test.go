package categorizacao

import (
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// cenario monta categorias, regras e o mapa da Pluggy, e devolve as regras carregadas e os ids.
type cenario struct {
	rs                                                          *Regras
	mercado, transporte, lazer, transferencias, pagamentoFatura int64
	contaCorrente, cartao                                       int64
}

func novoCenario(t *testing.T) cenario {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	categoria := func(nome string, tipo int64) int64 {
		return fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = nome, tipo }).Id
	}
	categoria("PAGAMENTO DE FATURA", CategoriaDespesa) // mesmo nome normalizado, mas de despesa: não é a da fatura
	c := cenario{
		mercado: categoria("Mercado", CategoriaDespesa), transporte: categoria("Transporte", CategoriaDespesa),
		lazer: categoria("Lazer", CategoriaDespesa), transferencias: categoria("Transferências", CategoriaTransferencia),
		pagamentoFatura: categoria("Pagamento de fatura", CategoriaTransferencia),
	}
	categoria("Pagamento de Fatura", CategoriaTransferencia) // a segunda do tipo certo: vale a primeira, pelo Id
	regra := func(texto string, tipo, categoria, prioridade int64, ativa bool) {
		fab.RegraCategorizacao(testeapoio.Categoria{Id: categoria}, func(r *testeapoio.RegraCategorizacao) {
			r.Texto, r.TipoCorrespondencia, r.Prioridade, r.Ativa = texto, tipo, prioridade, ativa
		})
	}
	regra("supermercado", Contem, c.mercado, 100, true)
	regra("UBER", ComecaCom, c.transporte, 90, true)
	regra("PADARIA", Igual, c.mercado, 50, false) // desligada
	regra("CINEMA", Contem, c.lazer, 10, true)
	regra("MERCADO", Contem, c.lazer, 200, true) // perde para "supermercado" (prioridade 100)
	regra("XP INVEST", Contem, c.transferencias, 100, true)
	for _, m := range []struct {
		id, pai   string
		categoria *int64
	}{
		{"05000000", "", &c.mercado},
		{"19000000", "", &c.transporte},
		{"19050000", "19000000", nil}, // sem mapeamento: herda o do pai
		{"99999999", "", nil},
	} {
		var pai *string
		if m.pai != "" {
			pai = &m.pai
		}
		if _, err := db.Exec(`INSERT INTO CategoriaPluggy (PluggyCategoriaId, Descricao, PaiPluggyId, CategoriaId) VALUES (?, ?, ?, ?)`,
			m.id, "x", pai, m.categoria); err != nil {
			t.Fatal(err)
		}
	}
	c.contaCorrente = fab.Conta().Id
	c.cartao = fab.Conta(func(x *testeapoio.Conta) { x.Tipo = 3 }).Id
	rs, err := Carregar(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	c.rs = rs
	return c
}

func p[T any](v T) *T { return &v }

// Regra: prioridade das regras ativas (menor número primeiro), contém/começa com/igual sem acento e sem diferença de
// maiúsculas, o nome de agora (#250) e o mapa da Pluggy (a subcategoria herda o do pai).
func TestResolver(t *testing.T) {
	c := novoCenario(t)
	casos := []struct {
		nome      string
		original  *string
		descricao string
		pluggy    *string
		esperado  *int64
	}{
		{"contém, com acento e minúsculas", nil, "Supermercado São João", nil, &c.mercado},
		{"a regra de prioridade menor vem primeiro", nil, "CINEMA DO SUPERMERCADO", nil, &c.lazer},
		{"a de prioridade maior perde", nil, "SUPERMERCADO", nil, &c.mercado},
		{"começa com", nil, "Uber *trip", nil, &c.transporte},
		{"começa com não acha no meio", nil, "PAGTO UBER", nil, nil},
		{"regra desligada não vale", nil, "PADARIA", nil, nil},
		{"o nome de agora também vale (#250)", p("PIX 123"), "Supermercado do bairro", nil, &c.mercado},
		{"mapa da Pluggy sem regra", nil, "LOJA", p("05000000"), &c.mercado},
		{"subcategoria da Pluggy herda o do pai", nil, "LOJA", p("19050000"), &c.transporte},
		{"mapa da Pluggy sem categoria", nil, "LOJA", p("99999999"), nil},
		{"categoria da Pluggy que não está no mapa", nil, "LOJA", p("12345678"), nil},
		{"regra vence o mapa da Pluggy", nil, "SUPERMERCADO", p("19000000"), &c.mercado},
		{"lançamento manual, sem descrição do banco", nil, "Uber manual", nil, &c.transporte},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			tr := &transacoes.Transacao{DescricaoOriginal: caso.original, Descricao: caso.descricao, PluggyCategoriaId: caso.pluggy}
			if caso.original == nil && caso.nome != "lançamento manual, sem descrição do banco" {
				tr.DescricaoOriginal = &caso.descricao
			}
			if got := c.rs.Resolver(tr); !mesmo(got, caso.esperado) {
				t.Errorf("Resolver = %v, esperado %v", valor(got), valor(caso.esperado))
			}
		})
	}
}

// Regra: AplicarTipo (ADR 0036): tipo pelo sinal; transferência fora dos relatórios; no cartão, entrada é pagamento
// de fatura ou estorno; estorno na categoria genérica ganha a categoria do texto sem "Estorno de" e sem "NuPay".
func TestAplicarTipo(t *testing.T) {
	c := novoCenario(t)
	grupo := tipos.NovoGuid()
	casos := []struct {
		nome            string
		conta           int64
		valor           string
		descricao       string
		categoria       *int64
		manual          bool
		tipoAntes       int64
		ignorarAntes    bool
		grupo           *tipos.Guid
		tipo            int64
		ignorar         bool
		categoriaDepois *int64
	}{
		{"saída na conta é despesa", c.contaCorrente, "-10", "LOJA", &c.mercado, false, transacoes.Receita, false, nil, transacoes.Despesa, false, &c.mercado},
		{"entrada na conta é receita", c.contaCorrente, "10", "PIX", nil, false, transacoes.Despesa, false, nil, transacoes.Receita, false, nil},
		{"categoria de transferência fica fora dos relatórios", c.contaCorrente, "-10", "TED", &c.transferencias, false, transacoes.Despesa, false, nil, transacoes.Transferencia, true, &c.transferencias},
		{"saindo de transferência volta aos relatórios", c.contaCorrente, "-10", "LOJA", &c.mercado, false, transacoes.Transferencia, true, nil, transacoes.Despesa, false, &c.mercado},
		{"lançada pelo site não muda de tipo", c.contaCorrente, "-10", "LOJA", &c.mercado, false, transacoes.Transferencia, true, &grupo, transacoes.Transferencia, true, &c.mercado},
		{"no cartão, categoria pagamento de fatura", c.cartao, "100", "QUALQUER", &c.pagamentoFatura, false, transacoes.Receita, false, nil, transacoes.Transferencia, true, &c.pagamentoFatura},
		{"no cartão, texto de pagamento ganha a categoria", c.cartao, "100", "PAGAMENTO RECEBIDO", nil, false, transacoes.Receita, false, nil, transacoes.Transferencia, true, &c.pagamentoFatura},
		{"no cartão, texto de pagamento com categoria manual não troca a categoria", c.cartao, "100", "PAGAMENTO RECEBIDO", &c.lazer, true, transacoes.Receita, false, nil, transacoes.Transferencia, true, &c.lazer},
		{"no cartão, entrada de despesa é estorno", c.cartao, "50", "ESTORNO LOJA", &c.lazer, false, transacoes.Receita, false, nil, transacoes.Despesa, false, &c.lazer},
		{"no cartão, estorno na genérica ganha a categoria do texto", c.cartao, "30", "Estorno de Uber - NuPay", &c.transferencias, false, transacoes.Transferencia, true, nil, transacoes.Despesa, false, &c.transporte},
		{"no cartão, estorno na genérica sem categoria pelo texto fica sem", c.cartao, "30", "ESTORNO LOJA X", &c.transferencias, false, transacoes.Transferencia, true, nil, transacoes.Despesa, false, nil},
		{"no cartão, crédito na genérica sem texto de estorno segue a transferência", c.cartao, "30", "PIX RECEBIDO", &c.transferencias, false, transacoes.Receita, false, nil, transacoes.Transferencia, true, &c.transferencias},
		{"no cartão, transferência manual fica transferência", c.cartao, "30", "ESTORNO", &c.transferencias, true, transacoes.Despesa, false, nil, transacoes.Transferencia, true, &c.transferencias},
		{"no cartão, saída é despesa", c.cartao, "-30", "LOJA", nil, false, transacoes.Receita, false, nil, transacoes.Despesa, false, nil},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			v, err := tipos.NovoDecimal(caso.valor)
			if err != nil {
				t.Fatal(err)
			}
			tr := &transacoes.Transacao{ContaId: caso.conta, Valor: v, Descricao: caso.descricao, DescricaoOriginal: &caso.descricao,
				CategoriaId: caso.categoria, CategoriaManual: caso.manual, Tipo: caso.tipoAntes, IgnorarRelatorios: caso.ignorarAntes,
				TransferenciaGrupo: caso.grupo}
			c.rs.AplicarTipo(tr)
			if tr.Tipo != caso.tipo || tr.IgnorarRelatorios != caso.ignorar || !mesmo(tr.CategoriaId, caso.categoriaDepois) {
				t.Errorf("tipo %d, ignorar %v, categoria %v; esperado %d, %v, %v",
					tr.Tipo, tr.IgnorarRelatorios, valor(tr.CategoriaId), caso.tipo, caso.ignorar, valor(caso.categoriaDepois))
			}
		})
	}
}

// Regra: no cartão, a descrição do banco e o nome de agora são olhados separadamente (pagamento de fatura por um ou
// pelo outro; o estorno pela descrição do banco), lançamento manual não tem descrição do banco, o "Estorno de" sai uma
// vez só, e estorno que cai numa categoria de transferência fica sem categoria.
func TestAplicarTipoComADescricaoDoBanco(t *testing.T) {
	c := novoCenario(t)
	casos := []struct {
		nome            string
		descricao       string
		original        *string // nil: lançamento manual, sem descrição do banco
		categoria       *int64
		tipo            int64
		categoriaDepois *int64
	}{
		{"só a descrição do banco diz pagamento", "Fatura paga", p("PAGAMENTO RECEBIDO"), nil, transacoes.Transferencia, &c.pagamentoFatura},
		{"só o nome de agora diz pagamento", "PAGAMENTO", p("CREDITO 123"), nil, transacoes.Transferencia, &c.pagamentoFatura},
		{"manual, pelo nome", "PAGAMENTO", nil, nil, transacoes.Transferencia, &c.pagamentoFatura},
		{"estorno pela descrição do banco", "Reembolso", p("Estorno de Uber"), &c.transferencias, transacoes.Despesa, &c.transporte},
		{"tira o prefixo do estorno uma vez só", "Estorno de estorno Uber", p("Estorno de estorno Uber"), &c.transferencias, transacoes.Despesa, nil},
		{"estorno que cai em transferência fica sem categoria", "ESTORNO XP INVEST", p("ESTORNO XP INVEST"), &c.transferencias, transacoes.Despesa, nil},
		{"estorno manual na genérica", "ESTORNO UBER", nil, &c.transferencias, transacoes.Despesa, &c.transporte},
		{"na categoria da fatura, texto de estorno continua pagamento", "ESTORNO LOJA", p("ESTORNO LOJA"), &c.pagamentoFatura, transacoes.Transferencia, &c.pagamentoFatura},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			v, _ := tipos.NovoDecimal("30")
			tr := &transacoes.Transacao{ContaId: c.cartao, Valor: v, Descricao: caso.descricao, DescricaoOriginal: caso.original,
				CategoriaId: caso.categoria, Tipo: transacoes.Receita}
			c.rs.AplicarTipo(tr)
			if tr.Tipo != caso.tipo || !mesmo(tr.CategoriaId, caso.categoriaDepois) {
				t.Errorf("tipo %d, categoria %v; esperado %d, %v", tr.Tipo, valor(tr.CategoriaId), caso.tipo, valor(caso.categoriaDepois))
			}
		})
	}
}

// Regra: TipoPeloSinal (ADR 0036): saída ou cartão é despesa; o resto é receita. Zero fora do cartão é receita.
func TestTipoPeloSinal(t *testing.T) {
	casos := []struct {
		valor    string
		cartao   bool
		esperado int64
	}{
		{"-0.01", false, transacoes.Despesa}, {"0.01", false, transacoes.Receita}, {"0.00", false, transacoes.Receita},
		{"10", true, transacoes.Despesa}, {"-10", true, transacoes.Despesa},
	}
	for _, c := range casos {
		v, _ := tipos.NovoDecimal(c.valor)
		if got := TipoPeloSinal(v, c.cartao); got != c.esperado {
			t.Errorf("TipoPeloSinal(%s, %v) = %d, esperado %d", c.valor, c.cartao, got, c.esperado)
		}
	}
}

func mesmo(a, b *int64) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

func valor(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}
