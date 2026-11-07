package tipos

import (
	"strings"
	"testing"
)

// Regra: NaEscalaDoBanco é o decimal depois da ida e volta pelo banco do EF Core (#447): pelo menos uma casa, sem zero
// à direita. É a escala do ef_sum do SQLite, que devolve a soma como texto "0.0###...".
func TestNaEscalaDoBanco(t *testing.T) {
	casos := []struct {
		nome, entrada, esperado string
	}{
		{"soma exata com zero à direita", "21.00", "21.0"},
		{"zero à direita em várias casas", "1.5000", "1.5"},
		{"inteiro ganha uma casa", "100", "100.0"},
		{"zero sem casas", "0", "0.0"},
		{"zero com casas", "0.000", "0.0"},
		{"já na escala do banco", "5000.0", "5000.0"},
		{"duas casas que ficam", "10.25", "10.25"},
		{"negativo", "-0.250", "-0.25"},
		{"negativo inteiro", "-12", "-12.0"},
		{"zero no meio fica", "10.05", "10.05"},
		{"zero no meio e à direita", "10.0500", "10.05"},
		{"valor que acaba em zero antes da vírgula", "1000.00", "1000.0"},
		{"oito casas", "0.12345678", "0.12345678"},
		{"28 casas", "0." + strings.Repeat("0", 27) + "1", "0." + strings.Repeat("0", 27) + "1"},
		{"28 casas com zeros no fim do que sobra", "1.2300000000000000000000000000", "1.23"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, err := NovoDecimal(c.entrada)
			if err != nil {
				t.Fatal(err)
			}
			obtido := d.NaEscalaDoBanco()
			if obtido.String() != c.esperado {
				t.Errorf("NaEscalaDoBanco(%s) = %s, esperado %s", c.entrada, obtido, c.esperado)
			}
			// É o que o banco devolve: o texto gravado, lido de volta, tem a mesma escala.
			lido, err := NovoDecimal(d.TextoNoBanco())
			if err != nil {
				t.Fatal(err)
			}
			if lido.String() != obtido.String() {
				t.Errorf("ida e volta pelo banco = %s, NaEscalaDoBanco = %s", lido, obtido)
			}
			if !obtido.Decimal().Equal(d.Decimal()) {
				t.Errorf("o valor mudou: %s virou %s", d, obtido)
			}
		})
	}
}
