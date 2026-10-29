package relatorios

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// Situações de um orçamento.
const (
	SituacaoOk         = "ok"
	SituacaoAlerta     = "alerta"
	SituacaoEmergencia = "emergencia"
	SituacaoEstourado  = "estourado"
)

// limiteDoAlerta é o percentual do limite a partir do qual o orçamento está em alerta.
var limiteDoAlerta = tipos.DecimalDeInteiro(80)

var cem = tipos.DecimalDeInteiro(100)

// OrcamentoDto é o orçamento do mês contra o gasto, na ordem do record OrcamentoDto do C#. Ano e Mes têm o tipo do
// C# (short e byte).
type OrcamentoDto struct {
	Id              int64         `json:"id"`
	CategoriaId     int64         `json:"categoriaId"`
	CategoriaNome   string        `json:"categoriaNome"`
	CategoriaCor    string        `json:"categoriaCor"`
	Ano             int16         `json:"ano"`
	Mes             uint8         `json:"mes"`
	Valor           tipos.Decimal `json:"valor"`
	Gasto           tipos.Decimal `json:"gasto"`
	Percentual      tipos.Decimal `json:"percentual"`
	GastoEmergencia tipos.Decimal `json:"gastoEmergencia"`
	Situacao        string        `json:"situacao"`
}

// OrcamentosDoMes é o orçamento contra o gasto do mês, por categoria, do maior percentual para o menor (empate: a
// ordem do Id do orçamento). Sem orçamento no mês, a lista vazia.
//
//   - O gasto é o total de despesas do mês da categoria, com as subcategorias somadas nela (TotaisPorCategoria
//     agrupando no pai). Quando o orçamento é da própria subcategoria, vale o gasto direto dela.
//   - O gasto de emergência é o mesmo cálculo só com as transações marcadas como emergência.
//   - Percentual é gasto ÷ limite × 100, com uma casa (arredondamento bancário); com limite zero, 0.
//   - Situação: "ok" até o limite; "alerta" a partir de 80% do limite (do percentual já arredondado); "emergencia" se
//     passou do limite mas, sem os gastos de emergência, ficaria dentro dele; "estourado" nos demais.
//
// As quatro somas do C# eram quatro consultas; aqui as linhas do mês são lidas uma vez e servem às quatro, com o mesmo
// resultado.
func (s *Servico) OrcamentosDoMes(ctx context.Context, ano, mes int) ([]OrcamentoDto, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT o."Id", o."CategoriaId", o."Valor", c."Nome", c."Cor" FROM "Orcamento" o
		JOIN "Categoria" c ON c."Id" = o."CategoriaId" WHERE o."Ano" = ? AND o."Mes" = ? ORDER BY o."Id"`, ano, mes)
	if err != nil {
		return nil, fmt.Errorf("ler os orçamentos de %d/%d: %w", mes, ano, err)
	}
	defer linhas.Close()
	type orcamento struct {
		id, categoriaId int64
		valor           tipos.Decimal
		nome, cor       string
	}
	orcamentos := []orcamento{}
	for linhas.Next() {
		var o orcamento
		if err := linhas.Scan(&o.id, &o.categoriaId, &o.valor, &o.nome, &o.cor); err != nil {
			return nil, fmt.Errorf("ler orçamento: %w", err)
		}
		orcamentos = append(orcamentos, o)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os orçamentos de %d/%d: %w", mes, ano, err)
	}
	if len(orcamentos) == 0 {
		return []OrcamentoDto{}, nil
	}

	lancamentos, err := s.Relevantes(ctx, relogio.PrimeiroDiaMes(ano, time.Month(mes)), relogio.UltimoDiaMes(ano, time.Month(mes)))
	if err != nil {
		return nil, err
	}
	categorias, err := s.lerCategorias(ctx)
	if err != nil {
		return nil, err
	}
	gastos := totaisPorCategoria(lancamentos, categorias, transacoes.Despesa, true, false)
	// Gasto de subcategorias também precisa contar quando o orçamento é da própria subcategoria.
	gastosDiretos := totaisPorCategoria(lancamentos, categorias, transacoes.Despesa, false, false)
	emergencias := totaisPorCategoria(lancamentos, categorias, transacoes.Despesa, true, true)
	emergenciasDiretas := totaisPorCategoria(lancamentos, categorias, transacoes.Despesa, false, true)

	resultado := make([]OrcamentoDto, 0, len(orcamentos))
	for _, o := range orcamentos {
		gasto := primeiroValor(gastos, gastosDiretos, o.categoriaId)
		percentual := tipos.Zero
		if o.valor.Decimal().IsPositive() {
			quociente, err := tipos.Dividir(gasto, o.valor)
			if err != nil {
				return nil, fmt.Errorf("percentual do orçamento %d: %w", o.id, err)
			}
			percentual = tipos.Arredondar(tipos.Multiplicar(quociente, cem), 1)
		}
		emergencia := primeiroValor(emergencias, emergenciasDiretas, o.categoriaId)
		// "emergencia": estourou, mas sem os gastos de emergência ficaria dentro do limite.
		var situacao string
		switch {
		case gasto.Decimal().Cmp(o.valor.Decimal()) <= 0:
			situacao = SituacaoOk
			if percentual.Decimal().Cmp(limiteDoAlerta.Decimal()) >= 0 {
				situacao = SituacaoAlerta
			}
		case tipos.Subtrair(gasto, emergencia).Decimal().Cmp(o.valor.Decimal()) <= 0:
			situacao = SituacaoEmergencia
		default:
			situacao = SituacaoEstourado
		}
		resultado = append(resultado, OrcamentoDto{
			Id: o.id, CategoriaId: o.categoriaId, CategoriaNome: o.nome, CategoriaCor: o.cor, Ano: int16(ano), Mes: uint8(mes),
			Valor: o.valor, Gasto: gasto, Percentual: percentual, GastoEmergencia: emergencia, Situacao: situacao,
		})
	}
	// OrderByDescending do LINQ é estável: empate fica na ordem do Id.
	slices.SortStableFunc(resultado, func(a, b OrcamentoDto) int { return b.Percentual.Decimal().Cmp(a.Percentual.Decimal()) })
	return resultado, nil
}

// primeiroValor é o "a.FirstOrDefault(...)?.Valor ?? b.FirstOrDefault(...)?.Valor ?? 0m" do C#: o total da categoria
// na primeira lista que a tem, senão na segunda, senão o decimal 0 (sem casas). As listas só têm total positivo.
func primeiroValor(primeira, segunda []ValorCategoriaDto, categoriaId int64) tipos.Decimal {
	for _, lista := range [][]ValorCategoriaDto{primeira, segunda} {
		for _, v := range lista {
			if v.CategoriaId != nil && *v.CategoriaId == categoriaId {
				return v.Valor
			}
		}
	}
	return tipos.Zero
}
