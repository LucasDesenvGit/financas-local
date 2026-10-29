package relatorios

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// MesesDoHistorico é quantos meses a janela do histórico (JanelaHistorico) olha para trás, no máximo.
const MesesDoHistorico = 6

// MesesDaRenda é quantos meses completos a renda média (RendaEParcelas) olha para trás, no máximo.
const MesesDaRenda = 12

// JanelaHistorico é o início e o número de meses completos com dados que entram nas médias: no máximo seis, terminando
// no mês passado, para não diluir a média com meses vazios (a projeção e o assistente de metas usam). É a janela de
// MesesCompletos(MesesDoHistorico) sem o fim, que é sempre o último dia do mês passado.
func (s *Servico) JanelaHistorico(ctx context.Context) (inicio tipos.Data, meses int, err error) {
	inicio, _, meses, err = s.MesesCompletos(ctx, MesesDoHistorico)
	return inicio, meses, err
}

// MesesCompletos são os últimos quantidade meses completos antes do mês atual, a partir do primeiro mês completo com
// transações: quem começou a usar há pouco tem menos meses. meses é 0 quando ainda não há mês completo (inicio fica no
// mês atual ou depois). fim é o último dia do mês passado.
//
// A primeira transação é a de qualquer tipo e status, inclusive as transferências e as marcadas para ignorar: a conta
// é do banco todo, não só dos relevantes. O primeiro mês completo é o próprio mês da primeira transação, se ela é do
// dia 1, ou o mês seguinte.
func (s *Servico) MesesCompletos(ctx context.Context, quantidade int) (inicio, fim tipos.Data, meses int, err error) {
	hoje := s.hoje()
	inicioMesAtual := tipos.NovaData(hoje.Ano(), hoje.Mes(), 1)
	fim = inicioMesAtual.SomarDias(-1)
	inicio = inicioMesAtual.SomarMeses(-quantidade)

	var primeira sql.Null[tipos.Data]
	if err := s.db.QueryRowContext(ctx, `SELECT MIN("Data") FROM "Transacao"`).Scan(&primeira); err != nil {
		return tipos.Data{}, tipos.Data{}, 0, fmt.Errorf("ler a primeira transação: %w", err)
	}
	if primeira.Valid {
		primeiroMesCompleto := tipos.NovaData(primeira.V.Ano(), primeira.V.Mes(), 1)
		if primeira.V.Dia() != 1 {
			primeiroMesCompleto = primeiroMesCompleto.SomarMeses(1)
		}
		if primeiroMesCompleto.Depois(inicio) {
			inicio = primeiroMesCompleto
		}
	}
	if !inicio.Antes(inicioMesAtual) {
		return inicio, fim, 0, nil
	}
	meses = (inicioMesAtual.Ano()-inicio.Ano())*12 + int(inicioMesAtual.Mes()) - int(inicio.Mes())
	return inicio, fim, meses, nil
}

// RendaEParcelas é a renda mensal média dos últimos 12 meses completos e as parcelas de cartão do último mês: a mesma
// regra da Saúde financeira, usada também pela sugestão de orçamentos. Sem mês completo, tudo zero.
//
//   - renda: a soma das receitas relevantes da janela dividida pelo número de meses (sem arredondar);
//   - parcelas: o que se gastou no último mês da janela em despesas parceladas (ParcelaTotal maior que 1), em valor
//     positivo, e nunca menos que zero (estorno de parcela que passa das compras dá 0).
func (s *Servico) RendaEParcelas(ctx context.Context) (renda, parcelasCartao tipos.Decimal, meses int, err error) {
	inicio, fim, meses, err := s.MesesCompletos(ctx, MesesDaRenda)
	if err != nil {
		return tipos.Zero, tipos.Zero, 0, err
	}
	if meses == 0 {
		return tipos.Zero, tipos.Zero, 0, nil
	}
	janela, err := s.Relevantes(ctx, inicio, fim)
	if err != nil {
		return tipos.Zero, tipos.Zero, 0, err
	}
	receitas := somaOuZero(valoresDe(janela, func(l Lancamento) bool { return l.Tipo == transacoes.Receita }))
	renda, err = tipos.Dividir(receitas, tipos.DecimalDeInteiro(int64(meses)))
	if err != nil {
		return tipos.Zero, tipos.Zero, 0, fmt.Errorf("renda média: %w", err)
	}

	ultimoMes, err := s.Relevantes(ctx, tipos.NovaData(fim.Ano(), fim.Mes(), 1), fim)
	if err != nil {
		return tipos.Zero, tipos.Zero, 0, err
	}
	parceladas := somaOuZero(valoresDe(ultimoMes, func(l Lancamento) bool {
		return l.Tipo == transacoes.Despesa && l.ParcelaTotal != nil && *l.ParcelaTotal > 1
	}))
	return renda, maiorDeDois(tipos.Zero, negar(parceladas)), meses, nil
}
