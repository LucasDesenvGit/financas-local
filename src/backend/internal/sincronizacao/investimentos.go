package sincronizacao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/emprestimos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/investimentos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/pluggy"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/tipos"
)

const (
	limiteDaTaxaAnual   = 99999
	limiteDaFracao      = 1000
	casasDaQuantidade   = 8
	casasDaTaxaAnual    = 4
	casasDaFracao       = 8
	casasDaParcela      = 2
	investimentoPadrao  = "Investimento"
	tipoDeInvestimentoO = "OTHER"
)

// semProduto: a instituição não tem o produto, ou o consentimento não o inclui.
func semProduto(err error) bool {
	var e *pluggy.Erro
	return errors.As(err, &e) && (e.Status == http.StatusBadRequest || e.Status == http.StatusForbidden || e.Status == http.StatusNotFound)
}

// comLimite é o v is { } x && Math.Abs(x) < limite ? Math.Round((decimal)x, casas) : null do C#: fora do limite, ou sem
// caber no decimal, fica nil.
func comLimite(n *pluggy.Numero, limite int64, casas int32) *tipos.Decimal {
	if n == nil {
		return nil
	}
	d, err := n.Decimal()
	if err != nil || d.Decimal().Abs().Cmp(tipos.DecimalDeInteiro(limite).Decimal()) >= 0 {
		return nil
	}
	arredondado := tipos.Arredondar(d, casas)
	return &arredondado
}

func (r *rodada) sincronizarInvestimentos(ctx context.Context, conexao int64, itemId string) error {
	lista, err := r.pluggy.ListarInvestimentos(ctx, r.sessao, itemId)
	if semProduto(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := r.gravarInvestimentos(ctx, conexao, lista); err != nil {
		return err
	}
	// A carteira é complementar: uma falha aqui não invalida a sincronização.
	if _, err := r.importarRendaVariavel(ctx, conexao); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		r.avisar("Não foi possível importar as operações de renda variável do Open Finance.", "erro", err)
	}
	return nil
}

func (r *rodada) gravarInvestimentos(ctx context.Context, conexao int64, lista []pluggy.Investimento) error {
	return banco.EmTransacao(ctx, r.db, func(tx *sql.Tx) error {
		existentes, err := lerChaves(ctx, tx, `SELECT "PluggyInvestmentId", "Id" FROM "Investimento" WHERE "ConexaoBancariaId" = ?`, conexao)
		if err != nil {
			return err
		}
		recebidos := map[string]bool{}
		var atuais []investimentos.Investimento
		for _, pi := range lista {
			if strings.TrimSpace(pi.Id) == "" || recebidos[pi.Id] {
				continue
			}
			recebidos[pi.Id] = true
			atual, err := r.gravarInvestimento(ctx, tx, conexao, pi, existentes)
			if err != nil {
				return err
			}
			atuais = append(atuais, atual)
		}
		// #173 e) (ADR 0045): o saldo de hoje vai para o histórico (rendimento de cada mês da caixinha ou do CDB).
		if err := investimentos.RegistrarHistorico(ctx, tx, atuais, relogio.Hoje(r.relogio), r.relogio.Agora()); err != nil {
			return err
		}
		for chave, id := range existentes {
			if !recebidos[chave] {
				if _, err := tx.ExecContext(ctx, `DELETE FROM "Investimento" WHERE "Id" = ?`, id); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *rodada) gravarInvestimento(ctx context.Context, tx *sql.Tx, conexao int64, pi pluggy.Investimento, existentes map[string]int64) (investimentos.Investimento, error) {
	nome, tipo := investimentoPadrao, tipoDeInvestimentoO
	if pi.Name != nil {
		nome = limitado(*pi.Name, 200)
	}
	if pi.Type != nil {
		tipo = limitado(*pi.Type, 30)
	}
	moeda := normalizarMoeda(pi.CurrencyCode)
	saldo, err := arredondar(pi.Balance)
	if err != nil {
		return investimentos.Investimento{}, err
	}
	aplicado, err := arredondarNulo(pi.AmountOriginal)
	if err != nil {
		return investimentos.Investimento{}, err
	}
	rendimento, err := arredondarNulo(pi.AmountProfit)
	if err != nil {
		return investimentos.Investimento{}, err
	}
	var quantidade *tipos.Decimal
	if pi.Quantity != nil {
		d, err := pi.Quantity.Decimal()
		if err != nil {
			return investimentos.Investimento{}, err
		}
		arredondada := tipos.Arredondar(d, casasDaQuantidade)
		quantidade = &arredondada
	}
	taxa := pi.AnnualRate
	if taxa == nil {
		taxa = pi.FixedAnnualRate
	}
	aplicacao := pi.PurchaseDate
	if aplicacao == nil {
		aplicacao = pi.IssueDate
	}
	id, existe := existentes[pi.Id]
	if !existe {
		res, err := tx.ExecContext(ctx, `INSERT INTO "Investimento" ("ConexaoBancariaId", "PluggyInvestmentId", "Nome", "Tipo", "Moeda", "Saldo", "AtualizadoEm")
			VALUES (?, ?, ?, ?, ?, ?, ?)`, conexao, pi.Id, nome, tipo, moeda, saldo, r.agora())
		if err != nil {
			return investimentos.Investimento{}, err
		}
		if id, err = res.LastInsertId(); err != nil {
			return investimentos.Investimento{}, err
		}
	}
	// A data de aplicação que a corretora deixa de informar fica como está.
	_, err = tx.ExecContext(ctx, `UPDATE "Investimento" SET "Nome" = ?, "Tipo" = ?, "Subtipo" = ?, "Codigo" = ?, "Emissor" = ?, "Moeda" = ?, "Saldo" = ?,
		"ValorAplicado" = ?, "Rendimento" = ?, "Quantidade" = ?, "TaxaAnual" = ?, "DataVencimento" = ?, "DataAplicacao" = COALESCE(?, "DataAplicacao"),
		"Status" = ?, "AtualizadoEm" = ? WHERE "Id" = ?`, nome, tipo, limitar(pi.Subtype, 60), limitar(pi.Code, 60), limitar(pi.Issuer, 200), moeda, saldo, aplicado,
		rendimento, quantidade, comLimite(taxa, limiteDaTaxaAnual, casasDaTaxaAnual), dataDe(pi.DueDate), dataDe(aplicacao), limitar(pi.Status, 30), r.agora(), id)
	if err != nil {
		return investimentos.Investimento{}, err
	}
	return investimentos.Investimento{PluggyInvestmentId: pi.Id, ConexaoBancariaId: conexao, Nome: nome, Tipo: tipo, Moeda: moeda, Saldo: saldo, ValorAplicado: aplicado}, nil
}

// lerChaves lê a consulta de duas colunas (a chave e o id) num mapa.
func lerChaves(ctx context.Context, tx *sql.Tx, consulta string, argumentos ...any) (map[string]int64, error) {
	linhas, err := tx.QueryContext(ctx, consulta, argumentos...)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	chaves := map[string]int64{}
	for linhas.Next() {
		var chave string
		var id int64
		if err := linhas.Scan(&chave, &id); err != nil {
			return nil, err
		}
		chaves[chave] = id
	}
	return chaves, linhas.Err()
}

// sincronizarEmprestimos: contratos de crédito e os pagamentos feitos. Complementar: se a instituição não compartilha
// "operações de crédito", ou a leitura falha, a sincronização segue sem eles.
func (r *rodada) sincronizarEmprestimos(ctx context.Context, conexao int64, itemId string) error {
	lista, err := r.pluggy.ListarEmprestimos(ctx, r.sessao, itemId)
	if semProduto(err) {
		return nil
	}
	var erroPluggy *pluggy.Erro
	if errors.As(err, &erroPluggy) || errors.Is(err, pluggy.ErrResposta) || errors.Is(err, pluggy.ErrRede) {
		r.avisar("Não foi possível obter os empréstimos e financiamentos da conexão.", "conexao", conexao, "erro", err)
		return nil
	}
	if err != nil {
		return err
	}
	return banco.EmTransacao(ctx, r.db, func(tx *sql.Tx) error {
		existentes, err := lerChaves(ctx, tx, `SELECT "PluggyLoanId", "Id" FROM "Emprestimo" WHERE "ConexaoBancariaId" = ?`, conexao)
		if err != nil {
			return err
		}
		recebidos := map[string]bool{}
		for _, pe := range lista {
			if strings.TrimSpace(pe.Id) == "" || recebidos[pe.Id] {
				continue
			}
			recebidos[pe.Id] = true
			if err := r.gravarEmprestimo(ctx, tx, conexao, pe, existentes); err != nil {
				return err
			}
		}
		for chave, id := range existentes {
			if !recebidos[chave] {
				if _, err := tx.ExecContext(ctx, `DELETE FROM "Emprestimo" WHERE "Id" = ?`, id); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// naturezaDoCredito é o ClassificacaoCredito.Natureza: financiamento é o crédito vinculado à compra de um bem.
func naturezaDoCredito(kind, modalidade, produto *string) int64 {
	if contem(kind, "FINANCING") || contem(modalidade, "FINANCIAMENTO") || contem(modalidade, "AQUISICAO_BENS") {
		return emprestimos.NaturezaFinanciamento
	}
	if (modalidade == nil || strings.TrimSpace(*modalidade) == "") && contem(produto, "FINANC") {
		return emprestimos.NaturezaFinanciamento
	}
	return emprestimos.NaturezaEmprestimo
}

func contem(valor *string, trecho string) bool {
	return valor != nil && strings.Contains(texto.MaiusculasInvariantes(*valor), trecho)
}

func inteiroDe(n *pluggy.Inteiro) *int64 {
	if n == nil {
		return nil
	}
	v := int64(*n)
	return &v
}

// pagamento é um PagamentoEmprestimo a gravar.
type pagamento struct {
	data           tipos.Data
	valor          tipos.Decimal
	extraordinario bool
}

func pagamentosDe(pe pluggy.Emprestimo) ([]pagamento, error) {
	lista := []pagamento{}
	if pe.Payments == nil {
		return lista, nil
	}
	for _, p := range pe.Payments.Releases {
		if p.PaidDate == nil || p.PaidAmount == nil {
			continue
		}
		pago, err := p.PaidAmount.Decimal()
		if err != nil {
			return nil, err
		}
		if !pago.Decimal().IsPositive() {
			continue
		}
		lista = append(lista, pagamento{p.PaidDate.Data(), tipos.ArredondarLongeDoZero(pago, 2), p.IsOverParcelPayment != nil && *p.IsOverParcelPayment})
	}
	return lista, nil
}

// valorDaParcela: a última parcela paga ou, sem nenhuma, o saldo dividido pelas parcelas a vencer.
func valorDaParcela(pagos []pagamento, aVencer *int64, saldo tipos.Decimal) (*tipos.Decimal, error) {
	var ultima *pagamento
	for i := range pagos {
		if !pagos[i].extraordinario && (ultima == nil || pagos[i].data.Depois(ultima.data)) {
			ultima = &pagos[i]
		}
	}
	if ultima != nil {
		return &ultima.valor, nil
	}
	if aVencer == nil || *aVencer <= 0 || !saldo.Decimal().IsPositive() {
		return nil, nil
	}
	dividido, err := tipos.Dividir(saldo, tipos.DecimalDeInteiro(*aVencer))
	if err != nil {
		return nil, err
	}
	parcela := tipos.Arredondar(dividido, casasDaParcela)
	return &parcela, nil
}

func (r *rodada) gravarEmprestimo(ctx context.Context, tx *sql.Tx, conexao int64, pe pluggy.Emprestimo, existentes map[string]int64) error {
	natureza := naturezaDoCredito(pe.Kind, pe.Type, pe.ProductName)
	produto := ""
	if pe.ProductName != nil {
		produto = limitado(strings.TrimSpace(*pe.ProductName), 200)
	}
	if produto == "" {
		produto = emprestimos.DescreverModalidade(pe.Type, natureza)
	}
	contratado, err := arredondar(pe.ContractAmount)
	if err != nil {
		return err
	}
	var saldoInformado *pluggy.Numero
	if pe.Payments != nil {
		saldoInformado = pe.Payments.ContractOutstandingBalance
	}
	saldo, err := arredondar(saldoInformado)
	if err != nil {
		return err
	}
	if saldo.Decimal().IsNegative() {
		saldo = tipos.Zero
	}
	var parcelas pluggy.ParcelasEmprestimo
	if pe.Installments != nil {
		parcelas = *pe.Installments
	}
	var taxa pluggy.TaxaEmprestimo
	if len(pe.InterestRates) > 0 {
		taxa = pe.InterestRates[0]
	}
	indexador := taxa.ReferentialRateIndexerSubType
	if indexador == nil {
		indexador = taxa.ReferentialRateIndexerType
	}
	pagos, err := pagamentosDe(pe)
	if err != nil {
		return err
	}
	aVencer := inteiroDe(parcelas.DueInstallments)
	parcela, err := valorDaParcela(pagos, aVencer, saldo)
	if err != nil {
		return err
	}
	id, existe := existentes[pe.Id]
	if !existe {
		res, err := tx.ExecContext(ctx, `INSERT INTO "Emprestimo" ("ConexaoBancariaId", "PluggyLoanId", "Natureza", "Produto", "Moeda", "ValorContratado", "SaldoDevedor",
			"AtualizadoEm") VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, conexao, pe.Id, natureza, produto, normalizarMoeda(pe.CurrencyCode), contratado, saldo, r.agora())
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE "Emprestimo" SET "Natureza" = ?, "Modalidade" = ?, "Produto" = ?, "NumeroContrato" = ?, "Moeda" = ?, "ValorContratado" = ?,
		"SaldoDevedor" = ?, "ValorParcela" = ?, "ParcelasTotal" = ?, "ParcelasPagas" = ?, "ParcelasAVencer" = ?, "ParcelasEmAtraso" = ?, "Periodicidade" = ?,
		"SistemaAmortizacao" = ?, "TaxaJuros" = ?, "PeriodicidadeTaxa" = ?, "Indexador" = ?, "Cet" = ?, "DataContratacao" = ?, "DataPrimeiraParcela" = ?,
		"DataVencimentoFinal" = ?, "AtualizadoEm" = ? WHERE "Id" = ?`,
		natureza, limitar(pe.Type, 80), produto, limitar(pe.ContractNumber, 100), normalizarMoeda(pe.CurrencyCode), contratado, saldo, parcela,
		inteiroDe(parcelas.TotalNumberOfInstallments), inteiroDe(parcelas.PaidInstallments), aVencer, inteiroDe(parcelas.PastDueInstallments),
		limitar(pe.InstallmentPeriodicity, 30), limitar(pe.AmortizationScheduled, 30), comLimite(taxa.PreFixedRate, limiteDaFracao, casasDaFracao),
		limitar(taxa.TaxPeriodicity, 10), limitar(indexador, 60), comLimite(pe.Cet, limiteDaFracao, casasDaFracao), dataDe(pe.ContractDate),
		dataDe(pe.FirstInstallmentDueDate), dataDe(pe.DueDate), r.agora(), id)
	if err != nil {
		return fmt.Errorf("gravar o empréstimo: %w", err)
	}
	// A instituição sempre devolve o histórico completo, então os pagamentos são recarregados.
	if _, err := tx.ExecContext(ctx, `DELETE FROM "PagamentoEmprestimo" WHERE "EmprestimoId" = ?`, id); err != nil {
		return err
	}
	for _, p := range pagos {
		if _, err := tx.ExecContext(ctx, `INSERT INTO "PagamentoEmprestimo" ("EmprestimoId", "Data", "Valor", "Extraordinario") VALUES (?, ?, ?, ?)`, id, p.data, p.valor, p.extraordinario); err != nil {
			return err
		}
	}
	return nil
}
