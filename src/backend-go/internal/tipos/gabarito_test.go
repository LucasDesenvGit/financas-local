package tipos

import (
	"database/sql/driver"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// gabarito é o testdata/efcore.json, gerado pela sonda testdata/sonda-efcore com o mesmo EF Core da API.
type gabarito struct {
	EfCore string `json:"efCore"`
	Casos  []struct {
		Caso    string                     `json:"caso"`
		NoBanco map[string]*string         `json:"noBanco"`
		JSON    map[string]json.RawMessage `json:"json"`
	} `json:"casos"`
}

// Os valores que a sonda gravou, na mesma ordem (Program.cs). A sonda anota o resultado; o valor de origem fica aqui.
var (
	decimaisDaSonda = map[string]string{
		"zero": "0", "inteiro-negativo": "-5000", "uma-casa": "4864.5", "zero-a-direita": "1.50",
		"meio-centavo": "0.125", "oito-casas": "12345678.12345678", "centavo-negativo": "-0.01",
		"cem-sem-casas": "100", "nulo": "7",
	}
	datasHorasDaSonda = []time.Time{
		time.Date(2026, 10, 15, 22, 55, 31, 348147100, time.UTC),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 15, 12, 0, 0, 500_000_000, time.UTC),
		time.Date(2026, 2, 28, 23, 59, 59, 100_000_000, time.UTC),
	}
)

func lerGabarito(t *testing.T) gabarito {
	t.Helper()
	dados, err := os.ReadFile("testdata/efcore.json")
	if err != nil {
		t.Fatal(err)
	}
	var g gabarito
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Casos) != len(decimaisDaSonda) {
		t.Fatalf("o gabarito tem %d casos e o teste conhece %d: a sonda mudou?", len(g.Casos), len(decimaisDaSonda))
	}
	return g
}

func TestDecimalIgualAoEfCore(t *testing.T) {
	for _, c := range lerGabarito(t).Casos {
		t.Run(c.Caso, func(t *testing.T) {
			origem, err := NovoDecimal(decimaisDaSonda[c.Caso])
			if err != nil {
				t.Fatal(err)
			}
			if got, want := origem.TextoNoBanco(), *c.NoBanco["Valor"]; got != want {
				t.Errorf("gravado no banco = %q, o EF Core grava %q", got, want)
			}

			var lido Decimal
			if err := lido.Scan(*c.NoBanco["Valor"]); err != nil {
				t.Fatal(err)
			}
			if got, want := mustJSON(t, lido), string(c.JSON["valor"]); got != want {
				t.Errorf("JSON depois de ler = %s, o ASP.NET escreve %s", got, want)
			}

			// HasPrecision(18, 2) não arredonda no SQLite: o EF Core grava o valor inteiro.
			if got, want := origem.TextoNoBanco(), *c.NoBanco["Preciso"]; c.Caso != "nulo" && got != want {
				t.Errorf("coluna com HasPrecision: gravado = %q, o EF Core grava %q", got, want)
			}
		})
	}
}

func TestDecimalOpcionalIgualAoEfCore(t *testing.T) {
	for _, c := range lerGabarito(t).Casos {
		var opcional *Decimal
		if texto := c.NoBanco["Opcional"]; texto != nil {
			opcional = new(Decimal)
			if err := opcional.Scan(*texto); err != nil {
				t.Fatal(err)
			}
		}
		if got, want := mustJSON(t, opcional), string(c.JSON["opcional"]); got != want {
			t.Errorf("%s: JSON do opcional = %s, o ASP.NET escreve %s", c.Caso, got, want)
		}
	}
}

func TestDataHoraIgualAoEfCore(t *testing.T) {
	for i, c := range lerGabarito(t).Casos {
		t.Run(c.Caso, func(t *testing.T) {
			origem := NovaDataHora(datasHorasDaSonda[i%len(datasHorasDaSonda)])
			if got, want := mustValor(t, origem), *c.NoBanco["Momento"]; got != want {
				t.Errorf("gravado no banco = %q, o EF Core grava %q", got, want)
			}
			var lida DataHora
			if err := lida.Scan(*c.NoBanco["Momento"]); err != nil {
				t.Fatal(err)
			}
			if got, want := mustJSON(t, lida), string(c.JSON["momento"]); got != want {
				t.Errorf("JSON depois de ler = %s, o ASP.NET escreve %s", got, want)
			}
		})
	}
}

func TestDataEGuidIgualAoEfCore(t *testing.T) {
	for i, c := range lerGabarito(t).Casos {
		dia := NovaData(2026, time.October, 1+i)
		if got, want := mustValor(t, dia), *c.NoBanco["Dia"]; got != want {
			t.Errorf("%s: data gravada = %q, o EF Core grava %q", c.Caso, got, want)
		}
		if got, want := mustJSON(t, dia), string(c.JSON["dia"]); got != want {
			t.Errorf("%s: JSON da data = %s, o ASP.NET escreve %s", c.Caso, got, want)
		}

		var g Guid
		if err := g.Scan(*c.NoBanco["Identificador"]); err != nil {
			t.Fatal(err)
		}
		if got, want := mustValor(t, g), *c.NoBanco["Identificador"]; got != want {
			t.Errorf("%s: guid gravado = %q, o EF Core grava %q", c.Caso, got, want)
		}
		if got, want := mustJSON(t, g), string(c.JSON["identificador"]); got != want {
			t.Errorf("%s: JSON do guid = %s, o ASP.NET escreve %s", c.Caso, got, want)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	dados, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(dados)
}

func mustValor(t *testing.T, v driver.Valuer) string {
	t.Helper()
	valor, err := v.Value()
	if err != nil {
		t.Fatal(err)
	}
	return valor.(string)
}
