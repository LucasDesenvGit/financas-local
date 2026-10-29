package testeapoio

import (
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Tabela das faturas do cartão (#77), num arquivo à parte, como as do câmbio.

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
func (f *Fabrica) Fatura(cartao Conta, vencimento tipos.Data, total tipos.Decimal, ajustes ...func(*FaturaCartao)) FaturaCartao {
	f.t.Helper()
	fa := FaturaCartao{ContaId: cartao.Id, DataVencimento: vencimento, ValorTotal: total, AtualizadoEm: f.agora()}
	for _, ajustar := range ajustes {
		ajustar(&fa)
	}
	fa.Id = f.inserir("FaturaCartao", `INSERT INTO FaturaCartao (ContaId, PluggyBillId, DataVencimento, DataFechamento,
		ValorTotal, ValorMinimo, ValorPago, AtualizadoEm) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		fa.ContaId, fa.PluggyBillId, fa.DataVencimento, fa.DataFechamento, fa.ValorTotal, fa.ValorMinimo, fa.ValorPago,
		fa.AtualizadoEm)
	return fa
}
