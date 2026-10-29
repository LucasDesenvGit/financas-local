package relatorios

import (
	"context"
	"fmt"
	"slices"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// CorSemCategoria é a cor da fatia "Sem categoria".
const CorSemCategoria = "#94a3b8"

// ValorCategoriaDto é um total por categoria, na ordem do record ValorCategoriaDto do C#. Sem categoria, CategoriaId
// é null e o nome é "Sem categoria".
type ValorCategoriaDto struct {
	CategoriaId *int64        `json:"categoriaId"`
	Nome        string        `json:"nome"`
	Cor         string        `json:"cor"`
	Valor       tipos.Decimal `json:"valor"`
}

// categoriaLida são as colunas de Categoria que o total usa.
type categoriaLida struct {
	nome  string
	cor   string
	paiId *int64
}

// lerCategorias lê todas as categorias, por Id (o ToDictionaryAsync do C#).
func (s *Servico) lerCategorias(ctx context.Context) (map[int64]categoriaLida, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Nome", "Cor", "CategoriaPaiId" FROM "Categoria"`)
	if err != nil {
		return nil, fmt.Errorf("ler as categorias dos relatórios: %w", err)
	}
	defer linhas.Close()
	categorias := map[int64]categoriaLida{}
	for linhas.Next() {
		var id int64
		var c categoriaLida
		if err := linhas.Scan(&id, &c.nome, &c.cor, &c.paiId); err != nil {
			return nil, fmt.Errorf("ler categoria dos relatórios: %w", err)
		}
		categorias[id] = c
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as categorias dos relatórios: %w", err)
	}
	return categorias, nil
}

// TotaisPorCategoria é o total por categoria dos relevantes entre inicio e fim, com valores positivos, do maior para o
// menor (empate: a ordem do Id da categoria, e "Sem categoria" por último). tipo é o Tipo da transação (1 Despesa, 2
// Receita).
//
//   - Despesa: o total é o valor negado da soma, líquido do estorno, que é despesa positiva (ADR 0036). Receita: a
//     soma como está.
//   - agruparNoPai soma a subcategoria na categoria pai (um nível só); a categoria que não tem pai, ou cujo pai não
//     existe, fica como está.
//   - somenteEmergencia conta só as transações marcadas como gasto de emergência.
//   - A categoria cujo total não passa de zero fica de fora (só estorno no período: a compra foi antes). O mesmo vale
//     para "Sem categoria", que entra só quando o total é positivo. Transação de categoria que não existe também cai
//     em "Sem categoria".
func (s *Servico) TotaisPorCategoria(ctx context.Context, inicio, fim tipos.Data, tipo int64, agruparNoPai,
	somenteEmergencia bool) ([]ValorCategoriaDto, error) {
	lancamentos, err := s.Relevantes(ctx, inicio, fim)
	if err != nil {
		return nil, err
	}
	categorias, err := s.lerCategorias(ctx)
	if err != nil {
		return nil, err
	}
	return totaisPorCategoria(lancamentos, categorias, tipo, agruparNoPai, somenteEmergencia), nil
}

// totaisPorCategoria é o miolo do TotaisPorCategoriaAsync, sem banco: o OrcamentosDoMes o roda quatro vezes sobre as
// mesmas linhas.
func totaisPorCategoria(lancamentos []Lancamento, categorias map[int64]categoriaLida, tipo int64, agruparNoPai,
	somenteEmergencia bool) []ValorCategoriaDto {
	// O GroupBy do C#: uma soma por CategoriaId (e uma para as sem categoria). O SQLite entrega os grupos em ordem de
	// CategoriaId, e a ordem em que os totais entram no resultado desempata os de mesmo valor.
	porCategoria := map[int64][]tipos.Decimal{}
	semId := []tipos.Decimal{}
	ids := []int64{}
	for _, l := range lancamentos {
		if l.Tipo != tipo || (somenteEmergencia && !l.Emergencia) {
			continue
		}
		if l.CategoriaId == nil {
			semId = append(semId, l.Valor)
			continue
		}
		if _, existe := porCategoria[*l.CategoriaId]; !existe {
			ids = append(ids, *l.CategoriaId)
		}
		porCategoria[*l.CategoriaId] = append(porCategoria[*l.CategoriaId], l.Valor)
	}
	slices.Sort(ids)

	// Despesa é negativa e o estorno, positivo: a soma já sai líquida (ADR 0036).
	liquido := func(soma tipos.Decimal) tipos.Decimal {
		if tipo == transacoes.Despesa {
			return negar(soma)
		}
		return soma
	}

	type total struct {
		nome, cor string
		valor     tipos.Decimal
	}
	resultado := map[int64]total{}
	ordem := []int64{}
	// Sem lançamentos sem categoria, a soma é o decimal 0 sem casas (somaOuZero), e o total de sem categoria fica 0.
	semCategoria := tipos.Somar(tipos.Zero, liquido(somaOuZero(semId)))
	for _, id := range ids {
		valor := liquido(somaOuZero(porCategoria[id]))
		cat, existe := categorias[id]
		if !existe {
			semCategoria = tipos.Somar(semCategoria, valor)
			continue
		}
		if agruparNoPai && cat.paiId != nil {
			if pai, existePai := categorias[*cat.paiId]; existePai {
				id, cat = *cat.paiId, pai
			}
		}
		if atual, jaTem := resultado[id]; jaTem {
			resultado[id] = total{atual.nome, atual.cor, tipos.Somar(atual.valor, valor)}
		} else {
			resultado[id] = total{cat.nome, cat.cor, valor}
			ordem = append(ordem, id)
		}
	}

	// Categoria só com estorno no período (a compra foi antes) fica de fora: não existe gasto negativo no gráfico.
	lista := []ValorCategoriaDto{}
	for _, id := range ordem {
		if t := resultado[id]; t.valor.Decimal().IsPositive() {
			lista = append(lista, ValorCategoriaDto{CategoriaId: new(id), Nome: t.nome, Cor: t.cor, Valor: t.valor})
		}
	}
	if semCategoria.Decimal().IsPositive() {
		lista = append(lista, ValorCategoriaDto{Nome: "Sem categoria", Cor: CorSemCategoria, Valor: semCategoria})
	}
	// OrderByDescending do LINQ é estável: empate fica na ordem em que entrou.
	slices.SortStableFunc(lista, func(a, b ValorCategoriaDto) int { return b.Valor.Decimal().Cmp(a.Valor.Decimal()) })
	return lista
}
