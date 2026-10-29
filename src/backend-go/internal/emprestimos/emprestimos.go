// Pacote emprestimos tem os totais do EmprestimoService do .NET (#447, épico #70): o saldo devedor e a parcela mensal
// de financiamentos e de empréstimos, separados por natureza (ADR 0014). O painel (#80), a sugestão de orçamentos
// (#79) e a visão anual usam. O painel de empréstimos (a lista dos contratos, os pagamentos por mês e a
// classificação por modalidade) vem em outra história.
//
// O dinheiro é somado e comparado em Go, com decimal: no SQLite ele é texto (CLAUDE.md, "Código Go").
package emprestimos

import (
	"context"
	"fmt"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Natureza de um contrato (o enum NaturezaCredito do C#).
const (
	NaturezaEmprestimo    = 1
	NaturezaFinanciamento = 2
)

// casasDaParcelaMensal são as casas do Math.Round da parcela mensal total.
const casasDaParcelaMensal = 2

// TotaisCreditoDto são os totais de crédito, na ordem do record TotaisCreditoDto do C#.
type TotaisCreditoDto struct {
	SaldoFinanciamentos         tipos.Decimal `json:"saldoFinanciamentos"`
	SaldoEmprestimos            tipos.Decimal `json:"saldoEmprestimos"`
	ParcelaMensalFinanciamentos tipos.Decimal `json:"parcelaMensalFinanciamentos"`
	ParcelaMensalEmprestimos    tipos.Decimal `json:"parcelaMensalEmprestimos"`
}

// Contrato são as colunas de Emprestimo que os totais usam.
type Contrato struct {
	Id            int64
	Natureza      int64
	SaldoDevedor  tipos.Decimal
	ValorParcela  *tipos.Decimal
	Periodicidade *string
}

// Servico lê os contratos de crédito. Recebe o banco, que pode ser uma transação (*sql.Tx) de quem chama.
type Servico struct {
	db consultas.DBTX
}

func NovoServico(db consultas.DBTX) *Servico { return &Servico{db: db} }

// ContratosAtivos são os contratos em aberto de conexões ativas: o saldo devedor é maior que zero (comparado em Go,
// com decimal) e a conexão bancária está ativa. A ordem é a do Id.
func (s *Servico) ContratosAtivos(ctx context.Context) ([]Contrato, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT e."Id", e."Natureza", e."SaldoDevedor", e."ValorParcela", e."Periodicidade"
		FROM "Emprestimo" e JOIN "ConexaoBancaria" c ON c."Id" = e."ConexaoBancariaId" WHERE c."Ativa" = 1 ORDER BY e."Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler os contratos de crédito: %w", err)
	}
	defer linhas.Close()
	lista := []Contrato{}
	for linhas.Next() {
		var c Contrato
		if err := linhas.Scan(&c.Id, &c.Natureza, &c.SaldoDevedor, &c.ValorParcela, &c.Periodicidade); err != nil {
			return nil, fmt.Errorf("ler contrato de crédito: %w", err)
		}
		if c.SaldoDevedor.Decimal().IsPositive() {
			lista = append(lista, c)
		}
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os contratos de crédito: %w", err)
	}
	return lista, nil
}

// Totais são o saldo devedor e a parcela mensal estimada, separados por natureza. Só entram as conexões ativas e os
// contratos em aberto (saldo devedor maior que zero); contrato quitado ou de conexão desativada fica de fora.
// A parcela mensal de cada natureza é a soma das parcelas mensais dos contratos (ParcelaMensal), arredondada em duas
// casas pelo método bancário; o saldo não é arredondado. Sem contratos, tudo é zero.
func (s *Servico) Totais(ctx context.Context) (TotaisCreditoDto, error) {
	contratos, err := s.ContratosAtivos(ctx)
	if err != nil {
		return TotaisCreditoDto{}, err
	}
	saldo := func(natureza int64) tipos.Decimal {
		soma := tipos.Zero
		for _, c := range contratos {
			if c.Natureza == natureza {
				soma = tipos.Somar(soma, c.SaldoDevedor)
			}
		}
		return soma
	}
	parcela := func(natureza int64) (tipos.Decimal, error) {
		soma := tipos.Zero
		for _, c := range contratos {
			if c.Natureza != natureza {
				continue
			}
			mensal, err := ParcelaMensal(c.ValorParcela, c.Periodicidade)
			if err != nil {
				return tipos.Zero, fmt.Errorf("parcela mensal do contrato %d: %w", c.Id, err)
			}
			soma = tipos.Somar(soma, mensal)
		}
		return tipos.Arredondar(soma, casasDaParcelaMensal), nil
	}
	parcelaFinanciamentos, err := parcela(NaturezaFinanciamento)
	if err != nil {
		return TotaisCreditoDto{}, err
	}
	parcelaEmprestimos, err := parcela(NaturezaEmprestimo)
	if err != nil {
		return TotaisCreditoDto{}, err
	}
	return TotaisCreditoDto{
		SaldoFinanciamentos: saldo(NaturezaFinanciamento), SaldoEmprestimos: saldo(NaturezaEmprestimo),
		ParcelaMensalFinanciamentos: parcelaFinanciamentos, ParcelaMensalEmprestimos: parcelaEmprestimos,
	}, nil
}

// ParcelaMensal converte a parcela para o equivalente mensal: anual (YEARLY, ANUAL) ÷ 12, semestral (SEMIANNUALLY,
// SEMESTRAL) ÷ 6, trimestral (QUARTERLY, TRIMESTRAL) ÷ 3, e qualquer outra periodicidade (MONTHLY, IRREGULAR, vazia)
// como está. A periodicidade não diferencia maiúsculas. Sem parcela, ou com parcela zero ou negativa, é 0. A divisão é a
// do decimal do C#, até 28 casas, sem arredondar.
func ParcelaMensal(parcela *tipos.Decimal, periodicidade *string) (tipos.Decimal, error) {
	if parcela == nil || !parcela.Decimal().IsPositive() {
		return tipos.Zero, nil
	}
	divisor := int64(1)
	if periodicidade != nil {
		switch strings.ToUpper(*periodicidade) {
		case "YEARLY", "ANUAL":
			divisor = 12
		case "SEMIANNUALLY", "SEMESTRAL":
			divisor = 6
		case "QUARTERLY", "TRIMESTRAL":
			divisor = 3
		}
	}
	if divisor == 1 {
		return *parcela, nil
	}
	mensal, err := tipos.Dividir(*parcela, tipos.DecimalDeInteiro(divisor))
	if err != nil {
		return tipos.Zero, fmt.Errorf("dividir a parcela em %d: %w", divisor, err)
	}
	return mensal, nil
}
