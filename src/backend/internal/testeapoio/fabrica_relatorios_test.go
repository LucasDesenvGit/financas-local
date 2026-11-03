package testeapoio

import "testing"

// As tabelas da base dos relatórios (#447) saem no formato do EF Core: é o que o .NET lê do mesmo arquivo (ADR 0049).
func TestFabricaDaBaseDosRelatoriosGravaNoFormatoDoEfCore(t *testing.T) {
	t.Parallel()
	db := NovoBancoVazio(t)
	fab := NovaFabrica(t, db, RelogioDosTestes()) // em Brasília ainda é 22/10/2026

	categoria := fab.Categoria()
	conexao := fab.ConexaoBancaria(func(c *ConexaoBancaria) { c.Ativa = false })
	emprestimo := fab.Emprestimo(conexao, func(e *Emprestimo) {
		e.Natureza = 2
		e.ValorParcela = new(Valor("1500.50"))
		e.ParcelasEmAtraso = new(int64(0))
		e.Periodicidade = new("MONTHLY")
		e.DataPrimeiraParcela = new(Dia(2026, 1, 31))
	})
	orcamento := fab.Orcamento(categoria)
	conta := fab.ContaAgendada(func(c *ContaAgendada) {
		c.CategoriaId = &categoria.Id
		c.DataLimite = new(Dia(2027, 1, 31))
		c.Ativa = false
	})

	var ativa, natureza, saldo, parcela, primeira, atraso, orcValor, orcAno, orcMes, contaValor, vencimento, limite, forma,
		criadoEm any
	if err := db.QueryRowContext(t.Context(), `SELECT cb.Ativa, e.Natureza, e.SaldoDevedor, e.ValorParcela,
		e.DataPrimeiraParcela, e.ParcelasEmAtraso, o.Valor, o.Ano, o.Mes, ca.Valor, ca.ProximoVencimento, ca.DataLimite,
		ca.FormaCobranca, ca.CriadoEm
		FROM Emprestimo e JOIN ConexaoBancaria cb ON cb.Id = e.ConexaoBancariaId
		JOIN Orcamento o ON o.Id = ? JOIN ContaAgendada ca ON ca.Id = ? WHERE e.Id = ?`,
		orcamento.Id, conta.Id, emprestimo.Id).
		Scan(&ativa, &natureza, &saldo, &parcela, &primeira, &atraso, &orcValor, &orcAno, &orcMes, &contaValor,
			&vencimento, &limite, &forma, &criadoEm); err != nil {
		t.Fatal(err)
	}
	casos := []struct{ coluna, obtido, esperado any }{
		{"ConexaoBancaria.Ativa (bool)", ativa, int64(0)},
		{"Emprestimo.Natureza (enum)", natureza, int64(2)},
		{"Emprestimo.SaldoDevedor (decimal sem zero à direita)", saldo, "5000.0"},
		{"Emprestimo.ValorParcela (decimal?)", parcela, "1500.5"},
		{"Emprestimo.DataPrimeiraParcela (DateOnly?)", primeira, "2026-01-31"},
		{"Emprestimo.ParcelasEmAtraso (int?)", atraso, int64(0)},
		{"Orcamento.Valor (decimal)", orcValor, "500.0"},
		{"Orcamento.Ano (hoje em Brasília)", orcAno, int64(2026)},
		{"Orcamento.Mes (hoje em Brasília)", orcMes, int64(10)},
		{"ContaAgendada.Valor (decimal)", contaValor, "100.0"},
		{"ContaAgendada.ProximoVencimento (hoje em Brasília, não em UTC)", vencimento, "2026-10-22"},
		{"ContaAgendada.DataLimite (DateOnly?)", limite, "2027-01-31"},
		{"ContaAgendada.FormaCobranca (enum)", forma, int64(1)},
		{"ContaAgendada.CriadoEm (UTC)", criadoEm, "2026-10-23 02:30:00"},
	}
	for _, c := range casos {
		if c.obtido != c.esperado {
			t.Errorf("%s = %v, esperado %v", c.coluna, c.obtido, c.esperado)
		}
	}
}

// Campo opcional que o teste não preenche vai como NULL, e a conexão padrão está ativa.
func TestFabricaDaBaseDosRelatoriosNulosEPadroes(t *testing.T) {
	t.Parallel()
	db := NovoBancoVazio(t)
	fab := NovaFabrica(t, db, RelogioDosTestes())
	conexao := fab.ConexaoBancaria()
	fab.Emprestimo(conexao)
	fab.ContaAgendada()

	var ativa int64
	var parcela, periodicidade, categoria, conta, ultimo, limite any
	if err := db.QueryRowContext(t.Context(), `SELECT cb.Ativa, e.ValorParcela, e.Periodicidade, ca.CategoriaId, ca.ContaId,
		ca.UltimoPagamentoEm, ca.DataLimite FROM ConexaoBancaria cb, Emprestimo e, ContaAgendada ca`).
		Scan(&ativa, &parcela, &periodicidade, &categoria, &conta, &ultimo, &limite); err != nil {
		t.Fatal(err)
	}
	if ativa != 1 || parcela != nil || periodicidade != nil || categoria != nil || conta != nil || ultimo != nil || limite != nil {
		t.Errorf("gravado: ativa %v, parcela %v, periodicidade %v, categoria %v, conta %v, último %v, limite %v; esperado 1 e todos NULL",
			ativa, parcela, periodicidade, categoria, conta, ultimo, limite)
	}
	// Os nomes que o banco exige únicos vêm da fábrica: duas conexões e dois contratos não se chocam.
	fab.Emprestimo(fab.ConexaoBancaria())
}
