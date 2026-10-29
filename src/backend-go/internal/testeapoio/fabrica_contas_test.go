package testeapoio

import "testing"

// A fatura do cartão (#77) sai no formato do EF Core: datas "2026-10-10", decimal "0.0" e até 28 casas, sem zeros à
// direita, e o carimbo com 7 casas; sem fechamento, sem mínimo e sem pago, NULL.
func TestFabricaDeFaturaGravaNoFormatoDoEfCore(t *testing.T) {
	t.Parallel()
	db := NovoBancoVazio(t)
	fab := NovaFabrica(t, db, RelogioDosTestes())
	cartao := fab.Conta(func(c *Conta) { c.Tipo = 3 })
	completa := fab.Fatura(cartao, Dia(2026, 10, 10), Valor("474.150"), func(f *FaturaCartao) {
		f.DataFechamento, f.ValorMinimo, f.ValorPago, f.PluggyBillId = new(Dia(2026, 10, 3)), new(Valor("71.12")),
			new(Valor("100")), new("bill-1")
	})
	simples := fab.Fatura(cartao, Dia(2026, 11, 10), Valor("0"))
	var vencimento, fechamento, total, minimo, pago, conta, conta2, conta3, bill, atualizado, fechamento2, total2,
		minimo2, pago2 any
	if err := db.QueryRowContext(t.Context(), `SELECT a.DataVencimento, a.DataFechamento, a.ValorTotal, a.ValorMinimo,
		a.ValorPago, a.ContaId, a.PluggyBillId, a.AtualizadoEm, b.DataFechamento, b.ValorTotal, b.ValorMinimo, b.ValorPago,
		b.ContaId, (SELECT COUNT(*) FROM FaturaCartao WHERE ContaId = ?)
		FROM FaturaCartao a JOIN FaturaCartao b ON b.Id = ? WHERE a.Id = ?`, cartao.Id, simples.Id, completa.Id).
		Scan(&vencimento, &fechamento, &total, &minimo, &pago, &conta, &bill, &atualizado, &fechamento2, &total2, &minimo2,
			&pago2, &conta2, &conta3); err != nil {
		t.Fatal(err)
	}
	casos := []struct{ coluna, obtido, esperado any }{
		{"DataVencimento", vencimento, "2026-10-10"},
		{"DataFechamento", fechamento, "2026-10-03"},
		{"ValorTotal", total, "474.15"},
		{"ValorMinimo", minimo, "71.12"},
		{"ValorPago", pago, "100.0"},
		{"ContaId", conta, cartao.Id},
		{"PluggyBillId", bill, "bill-1"},
		{"AtualizadoEm", atualizado, "2026-10-23 02:30:00"},
		{"DataFechamento (sem)", fechamento2, nil},
		{"ValorTotal (zero)", total2, "0.0"},
		{"ValorMinimo (sem)", minimo2, nil},
		{"ValorPago (sem)", pago2, nil},
		{"ContaId (a outra)", conta2, cartao.Id},
		{"faturas do cartão", conta3, int64(2)},
	}
	for _, c := range casos {
		if c.obtido != c.esperado {
			t.Errorf("%s = %#v, esperado %#v", c.coluna, c.obtido, c.esperado)
		}
	}
}
