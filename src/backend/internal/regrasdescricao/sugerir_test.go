package regrasdescricao

import (
	"encoding/json"
	"os"
	"testing"
)

// Regra: o texto sugerido para a regra é o do C# (sem os pedaços com número, com os dígitos e os espaços de Unicode;
// com menos de 3 letras, a descrição inteira). O gabarito testdata/sugerir.json é gravado pela sonda das contas
// (internal/faturas/testdata/sonda-csharp), com o RegraDescricaoService.SugerirTexto do próprio .NET.
func TestSugerirTextoIgualAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/sugerir.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Sugeridos []struct{ Descricao, Texto string } `json:"sugeridos"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Sugeridos) < 20 {
		t.Fatal("gabarito pequeno: a sonda mudou?")
	}
	for _, c := range g.Sugeridos {
		if obtido := SugerirTexto(c.Descricao); obtido != c.Texto {
			t.Errorf("SugerirTexto(%q) = %q, o C# dá %q", c.Descricao, obtido, c.Texto)
		}
	}
}
