package relatorios

import (
	"context"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// MesValoresDto são as receitas, as despesas e os gastos de emergência de um mês.
type MesValoresDto struct {
	Ano        int           `json:"ano"`
	Mes        int           `json:"mes"`
	Receitas   tipos.Decimal `json:"receitas"`
	Despesas   tipos.Decimal `json:"despesas"`
	Emergencia tipos.Decimal `json:"despesasEmergencia"`
}

// Totais são as receitas e as despesas dos relevantes entre inicio e fim, pelo tipo, e não pelo sinal (ADR 0036): o
// estorno no cartão é despesa positiva e abate as despesas. Cada soma é um SumAsync do EF Core (no formato do banco, ou
// 0 sem linhas); as despesas saem positivas.
func (s *Servico) Totais(ctx context.Context, inicio, fim tipos.Data) (receitas, despesas tipos.Decimal, err error) {
	lancamentos, err := s.Relevantes(ctx, inicio, fim)
	if err != nil {
		return tipos.Zero, tipos.Zero, err
	}
	receitas, despesas = totais(lancamentos)
	return receitas, despesas, nil
}

func totais(lancamentos []Lancamento) (receitas, despesas tipos.Decimal) {
	receitas = somaOuZero(valoresDe(lancamentos, func(l Lancamento) bool { return l.Tipo == transacoes.Receita }))
	despesas = negar(somaOuZero(valoresDe(lancamentos, func(l Lancamento) bool { return l.Tipo == transacoes.Despesa })))
	return receitas, despesas
}

// FluxoMensal são as receitas e as despesas de cada mês entre inicio e fim, inclusive os meses sem movimento (com 0).
// O C# agrupa por ano e mês no SQLite (uma soma condicional por grupo): o mês com movimento tem as três somas no formato do
// banco, mesmo a de um tipo que não apareceu nele ("0.0"); o mês sem movimento, 0 sem casas. As despesas saem
// positivas, e a emergência é o valor absoluto das saídas (Valor < 0) marcadas como emergência.
func (s *Servico) FluxoMensal(ctx context.Context, inicio, fim tipos.Data) ([]MesValoresDto, error) {
	lancamentos, err := s.Relevantes(ctx, inicio, fim)
	if err != nil {
		return nil, err
	}
	return fluxoMensal(lancamentos, inicio, fim), nil
}

func fluxoMensal(lancamentos []Lancamento, inicio, fim tipos.Data) []MesValoresDto {
	type somas struct{ receitas, despesas, emergencia []tipos.Decimal }
	porMes := map[int]*somas{}
	for _, l := range lancamentos {
		k := l.Data.Ano()*12 + int(l.Data.Mes())
		g := porMes[k]
		if g == nil {
			g = &somas{}
			porMes[k] = g
		}
		zero := tipos.Zero
		receita, despesa, emergencia := zero, zero, zero
		if l.Tipo == transacoes.Receita {
			receita = l.Valor
		}
		if l.Tipo == transacoes.Despesa {
			despesa = l.Valor
		}
		if l.Valor.Decimal().IsNegative() && l.Emergencia {
			emergencia = l.Valor
		}
		g.receitas = append(g.receitas, receita)
		g.despesas = append(g.despesas, despesa)
		g.emergencia = append(g.emergencia, emergencia)
	}
	lista := []MesValoresDto{}
	for m := tipos.NovaData(inicio.Ano(), inicio.Mes(), 1); m.NumeroDoDia() <= fim.NumeroDoDia(); m = m.SomarMeses(1) {
		item := MesValoresDto{Ano: m.Ano(), Mes: int(m.Mes()), Receitas: tipos.Zero, Despesas: negar(tipos.Zero),
			Emergencia: tipos.Zero}
		if g := porMes[m.Ano()*12+int(m.Mes())]; g != nil {
			item.Receitas = somaOuZero(g.receitas)
			item.Despesas = negar(somaOuZero(g.despesas))
			item.Emergencia = tipos.DeDecimal(somaOuZero(g.emergencia).Decimal().Abs())
		}
		lista = append(lista, item)
	}
	return lista
}
