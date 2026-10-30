package testeapoio

import (
	"fmt"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Tabelas da carteira (#82), num arquivo à parte para os assuntos feitos em paralelo não mexerem nas mesmas linhas.

// Ativo é uma linha de Ativo (Entidades/Carteira.cs).
type Ativo struct {
	Id                int64
	Ticker            string
	Nome              *string
	Tipo              int64 // 1 FII, 2 Ação, 3 ETF, 4 BDR, 5 Outro
	Moeda             string
	PrecoAtual        *tipos.Decimal
	PrecoAnterior     *tipos.Decimal
	PrecoAtualizadoEm *tipos.DataHora
	PrecoManual       bool
	FonteCotacao      *string
	Segmento          *string
	CriadoEm          tipos.DataHora
}

// Ativo grava uma ação em reais, sem cotação, com um ticker da B3 que não se repete.
func (f *Fabrica) Ativo(ajustes ...func(*Ativo)) Ativo {
	f.t.Helper()
	a := Ativo{Ticker: fmt.Sprintf("TEST%d", 3+f.proximo()%6), Tipo: 2, Moeda: "BRL", CriadoEm: f.agora()}
	for _, ajustar := range ajustes {
		ajustar(&a)
	}
	a.Id = f.inserir("Ativo", `INSERT INTO Ativo (Ticker, Nome, Tipo, Moeda, PrecoAtual, PrecoAnterior, PrecoAtualizadoEm, PrecoManual,
		FonteCotacao, Segmento, CriadoEm) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, a.Ticker, a.Nome, a.Tipo, a.Moeda, a.PrecoAtual,
		a.PrecoAnterior, a.PrecoAtualizadoEm, bit(a.PrecoManual), a.FonteCotacao, a.Segmento, a.CriadoEm)
	return a
}

// OperacaoAtivo é uma linha de OperacaoAtivo (Entidades/Carteira.cs).
type OperacaoAtivo struct {
	Id                int64
	AtivoId           int64
	Tipo              int64 // 1 Compra, 2 Venda
	Data              tipos.Data
	Quantidade        tipos.Decimal
	PrecoUnitario     tipos.Decimal
	Taxas             tipos.Decimal
	Cambio            tipos.Decimal
	Observacao        *string
	Origem            int64 // 1 Manual, 2 Open Finance, 3 Extrato
	PluggyTransacaoId *string
	TransacaoId       *int64
	CriadoEm          tipos.DataHora
}

// OperacaoAtivo grava a compra manual de 10 cotas a 10.0, hoje em Brasília, sem taxas e em reais.
func (f *Fabrica) OperacaoAtivo(ativo Ativo, ajustes ...func(*OperacaoAtivo)) OperacaoAtivo {
	f.t.Helper()
	o := OperacaoAtivo{AtivoId: ativo.Id, Tipo: 1, Data: relogio.Hoje(f.relogio), Quantidade: Valor("10.0"), PrecoUnitario: Valor("10.0"),
		Cambio: Valor("1.0"), Origem: 1, CriadoEm: f.agora()}
	for _, ajustar := range ajustes {
		ajustar(&o)
	}
	o.Id = f.inserir("OperacaoAtivo", `INSERT INTO OperacaoAtivo (AtivoId, Tipo, Data, Quantidade, PrecoUnitario, Taxas, Cambio, Observacao,
		Origem, PluggyTransacaoId, TransacaoId, CriadoEm) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, o.AtivoId, o.Tipo, o.Data, o.Quantidade,
		o.PrecoUnitario, o.Taxas, o.Cambio, o.Observacao, o.Origem, o.PluggyTransacaoId, o.TransacaoId, o.CriadoEm)
	return o
}

// Investimento é uma linha de Investimento (Entidades/OpenFinance.cs).
type Investimento struct {
	Id                 int64
	ConexaoBancariaId  int64
	PluggyInvestmentId string
	Nome               string
	Tipo               string
	Subtipo            *string
	Codigo             *string
	Emissor            *string
	Moeda              string
	Saldo              tipos.Decimal
	ValorAplicado      *tipos.Decimal
	Quantidade         *tipos.Decimal
	DataAplicacao      *tipos.Data
	NomePersonalizado  *string
	AtualizadoEm       tipos.DataHora
}

// Investimento grava um CDB de 100.0 da conexão.
func (f *Fabrica) Investimento(conexao ConexaoBancaria, ajustes ...func(*Investimento)) Investimento {
	f.t.Helper()
	n := f.proximo()
	i := Investimento{ConexaoBancariaId: conexao.Id, PluggyInvestmentId: fmt.Sprintf("investimento-%d", n), Nome: fmt.Sprintf("Investimento de teste %d", n),
		Tipo: "FIXED_INCOME", Moeda: "BRL", Saldo: Valor("100.0"), AtualizadoEm: f.agora()}
	for _, ajustar := range ajustes {
		ajustar(&i)
	}
	i.Id = f.inserir("Investimento", `INSERT INTO Investimento (ConexaoBancariaId, PluggyInvestmentId, Nome, Tipo, Subtipo, Codigo, Emissor, Moeda,
		Saldo, ValorAplicado, Quantidade, DataAplicacao, NomePersonalizado, AtualizadoEm) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ConexaoBancariaId, i.PluggyInvestmentId, i.Nome, i.Tipo, i.Subtipo, i.Codigo, i.Emissor, i.Moeda, i.Saldo, i.ValorAplicado, i.Quantidade,
		i.DataAplicacao, i.NomePersonalizado, i.AtualizadoEm)
	return i
}
