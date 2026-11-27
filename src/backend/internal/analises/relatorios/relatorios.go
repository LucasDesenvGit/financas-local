// Pacote relatorios (#447, épico #70): as partes que orçamentos, metas e contas a
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
// O dinheiro é somado e comparado em Go, com decimal: no SQLite ele é texto (CLAUDE.md, "Código Go"). A escala da soma, porém,
// segue o formato do banco (somaDoEFCore): o resultado numérico é o mesmo, mas a
// escala é a do texto gravado no banco (pelo menos uma casa, sem zero à direita).
package relatorios

import (
	"context"
	"fmt"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/transacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Servico é a base dos relatórios. Lê o banco que recebe, que pode ser uma transação (*sql.Tx) de quem chama.
type Servico struct {
	db      banco.DBTX
	relogio relogio.Relogio
}

func NovoServico(db banco.DBTX, r relogio.Relogio) *Servico {
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
// Pendente entra: o Status não é conferido. A ordem é a do Id, e nenhuma soma depende dela.
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

// hoje é a data de hoje em Brasília.
func (s *Servico) hoje() tipos.Data { return relogio.Hoje(s.relogio) }

// somaDoEFCore é a soma de decimais no formato do banco: a soma exata dos
// valores, ou "sem linhas" quando não há nenhum (o chamador troca por 0).
//
// A escala segue o texto: o decimal é gravado como texto, no formato "0.0###..." (pelo menos uma
// casa e nenhum zero à direita). A soma 10.25 + 10.75 = 21.00 sai 21.0, e não
// 21.00. Quem aparece no JSON (o gasto de uma categoria) mostra a diferença.
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

// SomaDoSumAsync é a soma de uma consulta (sem GroupBy). O ef_sum do
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

// maiorDeDois devolve o maior: com os dois iguais devolve o primeiro (Max(0m, 0.0) = 0, sem casas).
func maiorDeDois(a, b tipos.Decimal) tipos.Decimal {
	if a.Decimal().Cmp(b.Decimal()) >= 0 {
		return a
	}
	return b
}

// negar: a mesma escala, o sinal trocado.
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
