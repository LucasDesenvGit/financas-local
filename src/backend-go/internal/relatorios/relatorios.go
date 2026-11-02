// Pacote relatorios é a base do RelatorioService do .NET (#447, épico #70): as partes que orçamentos, metas e contas a
// pagar (#79) e o painel (#80) usam, num pacote só, para a regra não ser escrita duas vezes. Não tem rota: quem
// responde HTTP é o assunto que chama.
//
// O que está aqui:
//   - Relevantes: as transações que entram nos relatórios (sem transferências entre contas suas, ADR 0031, e sem as
//     marcadas para ignorar, ADR 0036);
//   - TotaisPorCategoria e OrcamentosDoMes: total por categoria, com estorno abatendo a despesa (ADR 0036), e o
//     orçamento contra o gasto do mês;
//   - ContasAgendadas, ProximoVencimentoApos e PagamentosAte: as contas a pagar e a recorrência delas;
//   - JanelaHistorico, MesesCompletos e RendaEParcelas: a janela dos últimos meses completos e a renda dela.
//
// O dinheiro é somado e comparado em Go, com decimal: no SQLite ele é texto (CLAUDE.md, "Código Go"). O EF Core do
// .NET fazia essas somas no SQL, com o ef_sum do Microsoft.Data.Sqlite (decimal exato); o resultado é o mesmo, mas a
// escala da soma é a do texto que o ef_sum devolve (somaDoEFCore).
package relatorios

import (
	"context"
	"fmt"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// Servico é a base dos relatórios. Lê o banco que recebe, que pode ser uma transação (*sql.Tx) de quem chama.
type Servico struct {
	db      consultas.DBTX
	relogio relogio.Relogio
}

func NovoServico(db consultas.DBTX, r relogio.Relogio) *Servico {
	return &Servico{db: db, relogio: r}
}

// Lancamento são as colunas de Transacao que os relatórios leem. Valor negativo é saída.
type Lancamento struct {
	Id           int64
	ContaId      int64
	Data         tipos.Data
	Valor        tipos.Decimal
	Tipo         int64
	CategoriaId  *int64
	ParcelaTotal *int64
	Emergencia   bool
}

// Relevantes são as transações que entram nos relatórios, entre inicio e fim (as duas datas valem): sem as
// transferências (Tipo 3, entre contas suas, ADR 0031) e sem as marcadas para ignorar nos relatórios (ADR 0036).
// Pendente entra: o Status não é conferido. A ordem é a do Id; o C# não ordenava, e nenhuma soma depende dela.
func (s *Servico) Relevantes(ctx context.Context, inicio, fim tipos.Data) ([]Lancamento, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "ContaId", "Data", "Valor", "Tipo", "CategoriaId", "ParcelaTotal",
		"Emergencia" FROM "Transacao" WHERE "Data" >= ? AND "Data" <= ? AND "IgnorarRelatorios" = 0 AND "Tipo" <> ?
		ORDER BY "Id"`, inicio, fim, transacoes.Transferencia)
	if err != nil {
		return nil, fmt.Errorf("ler as transações dos relatórios de %s a %s: %w", inicio, fim, err)
	}
	defer linhas.Close()
	lista := []Lancamento{}
	for linhas.Next() {
		var l Lancamento
		if err := linhas.Scan(&l.Id, &l.ContaId, &l.Data, &l.Valor, &l.Tipo, &l.CategoriaId, &l.ParcelaTotal,
			&l.Emergencia); err != nil {
			return nil, fmt.Errorf("ler transação dos relatórios: %w", err)
		}
		lista = append(lista, l)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as transações dos relatórios: %w", err)
	}
	return lista, nil
}

// hoje é a data de hoje em Brasília (Relogio.Hoje do C#).
func (s *Servico) hoje() tipos.Data { return relogio.Hoje(s.relogio) }

// somaDoEFCore é o Sum de decimal que o EF Core manda ao SQLite (ef_sum, do Microsoft.Data.Sqlite): a soma exata dos
// valores, ou "sem linhas" quando não há nenhum (o SumAsync(t => (decimal?)t.Valor) devolve null, e o C# faz "?? 0m").
//
// A escala segue o texto: o ef_sum devolve o decimal ao EF Core como texto, no formato "0.0###..." (pelo menos uma
// casa e nenhum zero à direita), e o EF Core o lê de volta. A soma 10.25 + 10.75 = 21.00 sai 21.0; no C# puro seria
// 21.00. Quem aparece no JSON (o gasto de uma categoria) mostra a diferença.
// Ref.: https://github.com/dotnet/efcore/blob/main/src/Microsoft.Data.Sqlite.Core/SqliteValueBinder.cs
func somaDoEFCore(valores []tipos.Decimal) (soma tipos.Decimal, temLinhas bool) {
	if len(valores) == 0 {
		return tipos.Zero, false
	}
	total := valores[0]
	for _, v := range valores[1:] {
		total = tipos.Somar(total, v)
	}
	return total.NaEscalaDoBanco(), true
}

// SomaDoSumAsync é o SumAsync(t => (decimal?)t.Valor) ?? 0m do EF Core sobre uma consulta (sem GroupBy): o ef_sum do
// SQLite devolve a soma no formato do banco, e o conjunto vazio também volta como "0.0" (contrato 13-painel: o mês
// futuro e o mês sem emergência; e o total pago do contrato sem pagamento, 15-emprestimos).
func SomaDoSumAsync(valores []tipos.Decimal) tipos.Decimal {
	return somaOuZero(valores).NaEscalaDoBanco()
}

// somaOuZero é o "?? 0m" depois do SumAsync: sem linhas, o decimal 0 sem casas.
func somaOuZero(valores []tipos.Decimal) tipos.Decimal {
	soma, _ := somaDoEFCore(valores)
	return soma
}

// maiorDeDois é o Math.Max(decimal, decimal) do C#: com os dois iguais devolve o primeiro (Max(0m, 0.0) = 0, sem casas).
func maiorDeDois(a, b tipos.Decimal) tipos.Decimal {
	if a.Decimal().Cmp(b.Decimal()) >= 0 {
		return a
	}
	return b
}

// negar é o "-x" do decimal do C#: a mesma escala, o sinal trocado.
func negar(d tipos.Decimal) tipos.Decimal { return tipos.Subtrair(tipos.Zero, d) }

// valoresDe tira o Valor de cada lançamento que passa no filtro.
func valoresDe(lista []Lancamento, passa func(Lancamento) bool) []tipos.Decimal {
	valores := []tipos.Decimal{}
	for _, l := range lista {
		if passa(l) {
			valores = append(valores, l.Valor)
		}
	}
	return valores
}
