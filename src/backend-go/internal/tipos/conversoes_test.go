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
