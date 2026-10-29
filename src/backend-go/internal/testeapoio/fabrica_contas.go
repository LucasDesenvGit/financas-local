package testeapoio

import (
	"fmt"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Tabelas das contas (#77), num arquivo à parte, como as do câmbio.

// Conexao é uma linha de ConexaoBancaria (Entidades/OpenFinance.cs).
type Conexao struct {
	Id                           int64
	PluggyItemId                 string
	NomeInstituicao              *string
	UltimaAtualizacaoInstituicao *tipos.DataHora
	Ativa                        bool
	CriadoEm                     tipos.DataHora
}

// Conexao grava uma conexão ativa do Open Finance, sem nome de instituição.
func (f *Fabrica) Conexao(ajustes ...func(*Conexao)) Conexao {
	f.t.Helper()
	c := Conexao{PluggyItemId: fmt.Sprintf("item-teste-%d", f.proximo()), Ativa: true, CriadoEm: f.agora()}
	for _, ajustar := range ajustes {
		ajustar(&c)
	}
	c.Id = f.inserir("ConexaoBancaria", `INSERT INTO ConexaoBancaria (PluggyItemId, NomeInstituicao,
		UltimaAtualizacaoInstituicao, Ativa, CriadoEm) VALUES (?, ?, ?, ?, ?)`,
		c.PluggyItemId, c.NomeInstituicao, c.UltimaAtualizacaoInstituicao, bit(c.Ativa), c.CriadoEm)
	return c
}

// FaturaCartao é uma linha de FaturaCartao (Entidades/OpenFinance.cs): a fatura que o banco manda.
type FaturaCartao struct {
	Id             int64
	ContaId        int64
	PluggyBillId   *string
	DataVencimento tipos.Data
	DataFechamento *tipos.Data
	ValorTotal     tipos.Decimal
	ValorMinimo    *tipos.Decimal
	ValorPago      *tipos.Decimal
	AtualizadoEm   tipos.DataHora
}

// Fatura grava uma fatura do cartão dado, com o vencimento e o total dados, sem fechamento nem pagamento.
func (f *Fabrica) Fatura(cartao Conta, vencimento tipos.Data, total string, ajustes ...func(*FaturaCartao)) FaturaCartao {
	f.t.Helper()
	valor, err := tipos.NovoDecimal(total)
	if err != nil {
		f.t.Fatal(err)
	}
	fa := FaturaCartao{ContaId: cartao.Id, DataVencimento: vencimento, ValorTotal: valor, AtualizadoEm: f.agora()}
	for _, ajustar := range ajustes {
		ajustar(&fa)
	}
	fa.Id = f.inserir("FaturaCartao", `INSERT INTO FaturaCartao (ContaId, PluggyBillId, DataVencimento, DataFechamento,
		ValorTotal, ValorMinimo, ValorPago, AtualizadoEm) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		fa.ContaId, fa.PluggyBillId, fa.DataVencimento, fa.DataFechamento, fa.ValorTotal, fa.ValorMinimo, fa.ValorPago,
		fa.AtualizadoEm)
	return fa
}
