package tipos

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/shopspring/decimal"
)

// gabaritoDeConversoes é o testdata/conversoes.json, gravado pela sonda das contas (internal/faturas/testdata/sonda-csharp).
type gabaritoDeConversoes struct {
	DoDouble []struct {
		Bits  string `json:"bits"`
		Valor string `json:"valor"`
	} `json:"doDouble"`
	ParaDouble []struct {
		Valor string `json:"valor"`
		Bits  string `json:"bits"`
	} `json:"paraDouble"`
	Numeros []struct {
		Texto string  `json:"texto"`
		Valor *string `json:"valor"`
	} `json:"numeros"`
	Contas []struct {
		A         string  `json:"a"`
		B         string  `json:"b"`
		Produto   *string `json:"produto"`
		Soma      *string `json:"soma"`
		Diferenca *string `json:"diferenca"`
	} `json:"contas"`
	Divisoes []struct {
		A         string `json:"a"`
		B         string `json:"b"`
		Resultado string `json:"resultado"`
	} `json:"divisoes"`
}

func lerGabaritoDeConversoes(t *testing.T) gabaritoDeConversoes {
	t.Helper()
	dados, err := os.ReadFile("testdata/conversoes.json")
	if err != nil {
		t.Fatal(err)
	}
	var g gabaritoDeConversoes
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.DoDouble) < 900 || len(g.ParaDouble) < 200 || len(g.Numeros) < 50 {
		t.Fatal("gabarito pequeno: a sonda mudou?")
	}
	return g
}

// Regra: o (decimal)double do .NET: 15 dígitos, os zeros à direita saem, arredonda para zero abaixo de 10^-28 e não
// cabe acima de 2^96, com NaN e infinito (941 valores do C#).
func TestDecimalDeFloat64IgualAoCSharp(t *testing.T) {
	t.Parallel()
	for _, c := range lerGabaritoDeConversoes(t).DoDouble {
		bits, err := strconv.ParseInt(c.Bits, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		x := math.Float64frombits(uint64(bits))
		d, err := DecimalDeFloat64(x)
		switch {
		case c.Valor == "estouro":
			if !errors.Is(err, ErrEstouro) {
				t.Errorf("(decimal)%v = %s, %v; o C# estoura", x, d, err)
			}
		case err != nil || d.String() != c.Valor:
			t.Errorf("(decimal)%v = %s, %v; o C# dá %s", x, d, err, c.Valor)
		case d.Decimal().Exponent() > 0:
			t.Errorf("(decimal)%v ficou com o expoente positivo: %d", x, d.Decimal().Exponent())
		}
	}
}

// Regra: o (double)decimal do .NET, bit a bit, com o número nos 96 bits e até 28 casas (214 valores do C#).
func TestParaFloat64IgualAoCSharp(t *testing.T) {
	t.Parallel()
	for _, c := range lerGabaritoDeConversoes(t).ParaDouble {
		bits, err := strconv.ParseInt(c.Bits, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		d, err := NovoDecimal(c.Valor)
		if err != nil {
			t.Fatal(err)
		}
		if obtido := d.ParaFloat64(); math.Float64bits(obtido) != uint64(bits) {
			t.Errorf("(double)%s = %v, o C# dá %v", c.Valor, obtido, math.Float64frombits(uint64(bits)))
		}
	}
}

// Regra: o decimal.TryParse com NumberStyles.Number: os espaços do .NET, o sinal nas duas pontas, a vírgula de milhar
// depois de um dígito, os NUL no fim, o arredondamento bancário e o limite do decimal (60 textos do C#).
func TestLerNumeroIgualAoCSharp(t *testing.T) {
	t.Parallel()
	for _, c := range lerGabaritoDeConversoes(t).Numeros {
		d, ok := LerNumero(c.Texto)
		switch {
		case c.Valor == nil:
			if ok {
				t.Errorf("LerNumero(%q) = %s, o C# recusa", c.Texto, d)
			}
		case !ok || d.String() != *c.Valor:
			t.Errorf("LerNumero(%q) = %s, %v; o C# dá %s", c.Texto, d, ok, *c.Valor)
		}
	}
}

// Regra: na divisão, o número sem a vírgula tem de ser menor que 2^96 (2^95 / 5 tem 2^96 na primeira casa), e a escala
// 0 é a última tentativa (o maior decimal dividido por 1).
func TestDividirNoLimiteDoDecimal(t *testing.T) {
	t.Parallel()
	for _, c := range lerGabaritoDeConversoes(t).Divisoes {
		a, err := NovoDecimal(c.A)
		if err != nil {
			t.Fatal(err)
		}
		b, err := NovoDecimal(c.B)
		if err != nil {
			t.Fatal(err)
		}
		if r, err := Dividir(a, b); err != nil || r.String() != c.Resultado {
			t.Errorf("%s / %s = %s, %v; o C# dá %s", c.A, c.B, r, err, c.Resultado)
		}
	}
}

// Regra: um Decimal montado à mão com expoente positivo, de qualquer um dos lados, não entra na conta da divisão.
func TestDividirComExpoentePositivo(t *testing.T) {
	t.Parallel()
	mil, um := Decimal{v: decimal.New(1, 3)}, DecimalDeInteiro(1)
	if _, err := Dividir(mil, um); err == nil {
		t.Error("dividendo aceito")
	}
	if _, err := Dividir(um, mil); err == nil {
		t.Error("divisor aceito")
	}
}

// Regra: as contas do decimal do C#: o resultado que não cabe nos 96 bits perde casas, arredondando pelo método bancário
// a partir do número exato. As que estouram no C# ficam de fora: lá é erro, e estas funções não devolvem erro.
func TestContasIguaisAoCSharp(t *testing.T) {
	t.Parallel()
	g := lerGabaritoDeConversoes(t)
	if len(g.Contas) < 400 {
		t.Fatal("gabarito pequeno: a sonda mudou?")
	}
	conferir := func(nome string, c func(a, b Decimal) Decimal, a, b Decimal, esperado *string) {
		t.Helper()
		if esperado != nil && c(a, b).String() != *esperado {
			t.Errorf("%s(%s, %s) = %s, o C# dá %s", nome, a, b, c(a, b), *esperado)
		}
	}
	for _, c := range g.Contas {
		a, err := NovoDecimal(c.A)
		if err != nil {
			t.Fatal(err)
		}
		b, err := NovoDecimal(c.B)
		if err != nil {
			t.Fatal(err)
		}
		conferir("Multiplicar", Multiplicar, a, b, c.Produto)
		conferir("Somar", Somar, a, b, c.Soma)
		conferir("Subtrair", Subtrair, a, b, c.Diferenca)
	}
}

// Regra: o Math.Min do decimal devolve o segundo no empate, e a escala é a dele.
func TestMenor(t *testing.T) {
	t.Parallel()
	casos := []struct{ a, b, esperado string }{{"1.0", "1.00", "1.00"}, {"1.00", "1.0", "1.0"}, {"2", "3", "2"}, {"3", "2", "2"}, {"-1", "1", "-1"}}
	for _, c := range casos {
		a, b := mustDecimal(t, c.a), mustDecimal(t, c.b)
		if r := Menor(a, b); r.String() != c.esperado {
			t.Errorf("Menor(%s, %s) = %s", c.a, c.b, r)
		}
	}
}

func mustDecimal(t *testing.T, texto string) Decimal {
	t.Helper()
	d, err := NovoDecimal(texto)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
