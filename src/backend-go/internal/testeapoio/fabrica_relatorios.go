package testeapoio

import (
	"fmt"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Tabelas da base dos relatórios (#447): conexão bancária, empréstimo, orçamento e conta agendada, num arquivo à
// parte para os assuntos feitos em paralelo não mexerem nas mesmas linhas.

// ConexaoBancaria é uma linha de ConexaoBancaria (Entidades/OpenFinance.cs).
type ConexaoBancaria struct {
	Id                           int64
	PluggyItemId                 string // único
	NomeInstituicao              *string
	CorPrimaria                  *string
	Status                       *string
	StatusExecucao               *string
	UltimaAtualizacaoInstituicao *tipos.DataHora
	ConsentimentoExpiraEm        *tipos.DataHora
	UltimaSincronizacaoEm        *tipos.DataHora
	UltimoErro                   *string
	Ativa                        bool
	CriadoEm                     tipos.DataHora
}

// ConexaoBancaria grava uma conexão ativa "Banco de teste N", com o item da Pluggy inventado.
func (f *Fabrica) ConexaoBancaria(ajustes ...func(*ConexaoBancaria)) ConexaoBancaria {
	f.t.Helper()
	n := f.proximo()
	c := ConexaoBancaria{
		PluggyItemId: fmt.Sprintf("item-de-teste-%d", n), NomeInstituicao: new(fmt.Sprintf("Banco de teste %d", n)),
		Ativa: true, CriadoEm: f.agora(),
	}
	for _, ajustar := range ajustes {
		ajustar(&c)
	}
	c.Id = f.inserir("ConexaoBancaria", `INSERT INTO ConexaoBancaria (PluggyItemId, NomeInstituicao, CorPrimaria, Status,
		StatusExecucao, UltimaAtualizacaoInstituicao, ConsentimentoExpiraEm, UltimaSincronizacaoEm, UltimoErro, Ativa,
		CriadoEm) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.PluggyItemId, c.NomeInstituicao, c.CorPrimaria, c.Status, c.StatusExecucao, c.UltimaAtualizacaoInstituicao,
		c.ConsentimentoExpiraEm, c.UltimaSincronizacaoEm, c.UltimoErro, bit(c.Ativa), c.CriadoEm)
	return c
}

// Emprestimo é uma linha de Emprestimo (Entidades/OpenFinance.cs): contrato de empréstimo ou financiamento.
type Emprestimo struct {
	Id                  int64
	ConexaoBancariaId   int64
	PluggyLoanId        string // único
	Natureza            int64  // 1 Empréstimo, 2 Financiamento
	Modalidade          *string
	Produto             string
	NumeroContrato      *string
	Moeda               string
	ValorContratado     tipos.Decimal
	SaldoDevedor        tipos.Decimal
	ValorParcela        *tipos.Decimal
	ParcelasTotal       *int64
	ParcelasPagas       *int64
	ParcelasAVencer     *int64
	ParcelasEmAtraso    *int64
	Periodicidade       *string // MONTHLY, YEARLY, SEMIANNUALLY, QUARTERLY...
	SistemaAmortizacao  *string
	TaxaJuros           *tipos.Decimal
	PeriodicidadeTaxa   *string
	Indexador           *string
	Cet                 *tipos.Decimal
	DataContratacao     *tipos.Data
	DataPrimeiraParcela *tipos.Data
	DataVencimentoFinal *tipos.Data
	AtualizadoEm        tipos.DataHora
}

// Emprestimo grava um empréstimo (Natureza 1) da conexão dada: contratado 10000.0, saldo devedor 5000.0, em reais,
// sem parcela informada.
func (f *Fabrica) Emprestimo(conexao ConexaoBancaria, ajustes ...func(*Emprestimo)) Emprestimo {
	f.t.Helper()
	n := f.proximo()
	e := Emprestimo{
		ConexaoBancariaId: conexao.Id, PluggyLoanId: fmt.Sprintf("contrato-de-teste-%d", n), Natureza: 1,
		Produto: fmt.Sprintf("Empréstimo de teste %d", n), Moeda: "BRL", ValorContratado: Valor("10000.0"),
		SaldoDevedor: Valor("5000.0"), AtualizadoEm: f.agora(),
	}
	for _, ajustar := range ajustes {
		ajustar(&e)
	}
	e.Id = f.inserir("Emprestimo", `INSERT INTO Emprestimo (ConexaoBancariaId, PluggyLoanId, Natureza, Modalidade, Produto,
		NumeroContrato, Moeda, ValorContratado, SaldoDevedor, ValorParcela, ParcelasTotal, ParcelasPagas, ParcelasAVencer,
		ParcelasEmAtraso, Periodicidade, SistemaAmortizacao, TaxaJuros, PeriodicidadeTaxa, Indexador, Cet, DataContratacao,
		DataPrimeiraParcela, DataVencimentoFinal, AtualizadoEm)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ConexaoBancariaId, e.PluggyLoanId, e.Natureza, e.Modalidade, e.Produto, e.NumeroContrato, e.Moeda,
		e.ValorContratado, e.SaldoDevedor, e.ValorParcela, e.ParcelasTotal, e.ParcelasPagas, e.ParcelasAVencer,
		e.ParcelasEmAtraso, e.Periodicidade, e.SistemaAmortizacao, e.TaxaJuros, e.PeriodicidadeTaxa, e.Indexador, e.Cet,
		e.DataContratacao, e.DataPrimeiraParcela, e.DataVencimentoFinal, e.AtualizadoEm)
	return e
}

// Orcamento é uma linha de Orcamento (Entidades/Financeiro.cs): o limite de uma categoria num mês.
type Orcamento struct {
	Id          int64
	CategoriaId int64
	Ano         int64
	Mes         int64
	Valor       tipos.Decimal
}

// Orcamento grava o orçamento de 500.0 da categoria dada no mês de hoje (em Brasília, pelo relógio da fábrica).
func (f *Fabrica) Orcamento(categoria Categoria, ajustes ...func(*Orcamento)) Orcamento {
	f.t.Helper()
	hoje := relogio.Hoje(f.relogio)
	o := Orcamento{CategoriaId: categoria.Id, Ano: int64(hoje.Ano()), Mes: int64(hoje.Mes()), Valor: Valor("500.0")}
	for _, ajustar := range ajustes {
		ajustar(&o)
	}
	o.Id = f.inserir("Orcamento", `INSERT INTO Orcamento (CategoriaId, Ano, Mes, Valor) VALUES (?, ?, ?, ?)`,
		o.CategoriaId, o.Ano, o.Mes, o.Valor)
	return o
}

// ContaAgendada é uma linha de ContaAgendada (Entidades/Financeiro.cs): conta a pagar.
type ContaAgendada struct {
	Id                int64
	Descricao         string
	Valor             tipos.Decimal
	ProximoVencimento tipos.Data
	Recorrencia       int64 // 0 Única, 1 Mensal, 2 Anual
	CategoriaId       *int64
	ContaId           *int64
	Ativa             bool
	UltimoPagamentoEm *tipos.Data
	DataLimite        *tipos.Data
	FormaCobranca     int64 // 1 À mão, 2 Débito automático, 3 Cartão de crédito
	TextoCobranca     *string
	Observacao        *string
	ChaveExtrato      *string
	CriadoEm          tipos.DataHora
}

// ContaAgendada grava uma conta a pagar ativa de 100.0, mensal, paga à mão, que vence hoje (em Brasília, pelo relógio
// da fábrica), sem categoria nem conta.
func (f *Fabrica) ContaAgendada(ajustes ...func(*ContaAgendada)) ContaAgendada {
	f.t.Helper()
	c := ContaAgendada{
		Descricao: fmt.Sprintf("Conta agendada de teste %d", f.proximo()), Valor: Valor("100.0"),
		ProximoVencimento: relogio.Hoje(f.relogio), Recorrencia: 1, Ativa: true, FormaCobranca: 1, CriadoEm: f.agora(),
	}
	for _, ajustar := range ajustes {
		ajustar(&c)
	}
	c.Id = f.inserir("ContaAgendada", `INSERT INTO ContaAgendada (Descricao, Valor, ProximoVencimento, Recorrencia,
		CategoriaId, ContaId, Ativa, UltimoPagamentoEm, DataLimite, FormaCobranca, TextoCobranca, Observacao, ChaveExtrato,
		CriadoEm) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Descricao, c.Valor, c.ProximoVencimento, c.Recorrencia, c.CategoriaId, c.ContaId, bit(c.Ativa),
		c.UltimoPagamentoEm, c.DataLimite, c.FormaCobranca, c.TextoCobranca, c.Observacao, c.ChaveExtrato, c.CriadoEm)
	return c
}
