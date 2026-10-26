package testeapoio

import (
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Tabelas do câmbio e da cripto (#81), num arquivo à parte para os assuntos feitos em paralelo não mexerem nas
// mesmas linhas.

// CotacaoMoeda é uma linha de CotacaoMoeda (Entidades/Cambio.cs): a PTAX de fechamento de um dia.
type CotacaoMoeda struct {
	Moeda        string // código ISO 4217 (USD, EUR)
	Data         tipos.Data
	Compra       tipos.Decimal
	Venda        tipos.Decimal
	AtualizadoEm tipos.DataHora
}

// CotacaoMoeda grava a cotação do dólar de hoje (em Brasília, pelo relógio da fábrica): compra 5.4123, venda 5.4129.
func (f *Fabrica) CotacaoMoeda(ajustes ...func(*CotacaoMoeda)) CotacaoMoeda {
	f.t.Helper()
	c := CotacaoMoeda{Moeda: "USD", Data: relogio.Hoje(f.relogio), Compra: Valor("5.4123"), Venda: Valor("5.4129"), AtualizadoEm: f.agora()}
	for _, ajustar := range ajustes {
		ajustar(&c)
	}
	f.inserir("CotacaoMoeda", `INSERT INTO CotacaoMoeda (Moeda, Data, Compra, Venda, AtualizadoEm) VALUES (?, ?, ?, ?, ?)`,
		c.Moeda, c.Data, c.Compra, c.Venda, c.AtualizadoEm)
	return c
}

// CotacaoCripto é uma linha de CotacaoCripto (Entidades/Cambio.cs): o último negócio no Mercado Bitcoin.
type CotacaoCripto struct {
	Moeda        string         // BTC, ETH, SOL, USDT
	DataHora     tipos.DataHora // a da fonte, em UTC
	Preco        tipos.Decimal
	AtualizadoEm tipos.DataHora
}

// CotacaoCripto grava a cotação do bitcoin agora (UTC, pelo relógio da fábrica): 612345.67.
func (f *Fabrica) CotacaoCripto(ajustes ...func(*CotacaoCripto)) CotacaoCripto {
	f.t.Helper()
	c := CotacaoCripto{Moeda: "BTC", DataHora: f.agora(), Preco: Valor("612345.67"), AtualizadoEm: f.agora()}
	for _, ajustar := range ajustes {
		ajustar(&c)
	}
	f.inserir("CotacaoCripto", `INSERT INTO CotacaoCripto (Moeda, DataHora, Preco, AtualizadoEm) VALUES (?, ?, ?, ?)`,
		c.Moeda, c.DataHora, c.Preco, c.AtualizadoEm)
	return c
}

// Configuracao grava uma configuração que não é segredo (Protegido = 0), como o DefinirAsync do .NET.
func (f *Fabrica) Configuracao(chave, valor string) {
	f.t.Helper()
	f.inserir("Configuracao", `INSERT INTO Configuracao (Chave, Valor, Protegido, AtualizadoEm) VALUES (?, ?, 0, ?)`,
		chave, valor, f.agora())
}
