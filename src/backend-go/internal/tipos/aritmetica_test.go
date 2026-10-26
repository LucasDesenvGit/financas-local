package tipos

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// O gabarito é o testdata/csharp.json, gravado pela sonda testdata/sonda-csharp com o .NET da API:
// o resultado de cada conta escrito pelo decimal.ToString() do C#, que mostra a escala.
type gabaritoAritmetica struct {
	Divisoes []struct {
		A, B, Resultado string
	} `json:"divisoes"`
	Produtos []struct {
		A, B, Resultado string
	} `json:"produtos"`
	Arredondamentos []struct {
		X         string `json:"x"`
		Casas     int32  `json:"casas"`
		Resultado string `json:"resultado"`
	} `json:"arredondamentos"`
}

func lerGabaritoAritmetica(t *testing.T) gabaritoAritmetica {
	t.Helper()
	dados, err := os.ReadFile("testdata/csharp.json")
	if err != nil {
		t.Fatal(err)
	}
	var g gabaritoAritmetica
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Divisoes) == 0 || len(g.Produtos) == 0 || len(g.Arredondamentos) == 0 {
		t.Fatal("gabarito vazio: rode a sonda de novo")
	}
	return g
}

func dec(t *testing.T, texto string) Decimal {
	t.Helper()
	d, err := NovoDecimal(texto)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDividirIgualAoCSharp(t *testing.T) {
	t.Parallel()
	for _, c := range lerGabaritoAritmetica(t).Divisoes {
		t.Run(c.A+"/"+c.B, func(t *testing.T) {
			obtido, err := Dividir(dec(t, c.A), dec(t, c.B))
			if err != nil {
				t.Fatal(err)
			}
			if obtido.String() != c.Resultado {
				t.Errorf("%s / %s = %s, o C# dá %s", c.A, c.B, obtido, c.Resultado)
			}
		})
	}
}

func TestMultiplicarIgualAoCSharp(t *testing.T) {
	t.Parallel()
	for _, c := range lerGabaritoAritmetica(t).Produtos {
		t.Run(c.A+"*"+c.B, func(t *testing.T) {
			if obtido := Multiplicar(dec(t, c.A), dec(t, c.B)); obtido.String() != c.Resultado {
				t.Errorf("%s * %s = %s, o C# dá %s", c.A, c.B, obtido, c.Resultado)
			}
		})
	}
}

func TestArredondarIgualAoMathRound(t *testing.T) {
	t.Parallel()
	for _, c := range lerGabaritoAritmetica(t).Arredondamentos {
		t.Run(c.X, func(t *testing.T) {
			if obtido := Arredondar(dec(t, c.X), c.Casas); obtido.String() != c.Resultado {
				t.Errorf("Math.Round(%s, %d) = %s, o C# dá %s", c.X, c.Casas, obtido, c.Resultado)
			}
		})
	}
}

func TestDividirPorZero(t *testing.T) {
	t.Parallel()
	if _, err := Dividir(dec(t, "1"), dec(t, "0.00")); !errors.Is(err, ErrDivisaoPorZero) {
		t.Fatalf("esperado ErrDivisaoPorZero, veio %v", err)
	}
}

func TestSomarESubtrairMantemAMaiorEscala(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome, a, b, esperado string
		op                   func(a, b Decimal) Decimal
	}{
		{"soma", "1.5", "2.25", "3.75", Somar},
		{"subtração mantém as casas", "2.50", "1", "1.50", Subtrair},
		{"soma com negativo", "618900.12", "-612345.67", "6554.45", Somar},
		{"diferença de preço", "612345.67", "618900.12", "-6554.45", Subtrair},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if obtido := c.op(dec(t, c.a), dec(t, c.b)); obtido.String() != c.esperado {
				t.Errorf("obtido %s, esperado %s", obtido, c.esperado)
			}
		})
	}
}
