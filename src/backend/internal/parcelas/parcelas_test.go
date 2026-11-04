package parcelas

import (
	"encoding/json"
	"os"
	"testing"
)

// Regra: Compra é a do ParcelasFuturas do C#: sem o número da parcela ("03/10", "PARC 3 DE 10", "PARCELA."), as 8
// primeiras letras em maiúsculas, com o \d, o \s e o \b do .NET (Unicode) e o char UTF-16. O gabarito é o
// testdata/compra.json, gravado pela sonda das contas (internal/faturas/testdata/sonda-csharp).
func TestCompraIgualAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/compra.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Compras []struct{ Descricao, Compra string } `json:"compras"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Compras) < 70 {
		t.Fatalf("o gabarito tem só %d casos: a sonda mudou?", len(g.Compras))
	}
	for _, c := range g.Compras {
		if obtido := Compra(c.Descricao); obtido != c.Compra {
			t.Errorf("Compra(%q) = %q, o C# dá %q", c.Descricao, obtido, c.Compra)
		}
	}
}
