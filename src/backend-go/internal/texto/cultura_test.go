package texto

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// Regra: a ordem pela cultura é a do C# (o ThenBy de texto com a cultura pt-BR): acento e caixa só desempatam, espaço
// e pontuação antes de dígito e letra, "ß" depois de "ss". O gabarito é o testdata/ordem.json, da sonda das contas
// (internal/faturas/testdata/sonda-csharp).
func TestCompararNaCulturaIgualAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/ordem.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Nomes     []string `json:"nomes"`
		Ordenados []string `json:"ordenados"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Nomes) < 50 || len(g.Nomes) != len(g.Ordenados) {
		t.Fatalf("gabarito com %d nomes e %d ordenados: a sonda mudou?", len(g.Nomes), len(g.Ordenados))
	}
	obtidos := slices.Clone(g.Nomes)
	slices.SortStableFunc(obtidos, CompararNaCultura)
	for i := range obtidos {
		if obtidos[i] != g.Ordenados[i] {
			t.Errorf("posição %d: %q, e o C# põe %q", i, obtidos[i], g.Ordenados[i])
		}
	}
	if CompararNaCultura("Bolsa", "Bolsa") != 0 || CompararNaCultura("árvore", "Bolsa") >= 0 {
		t.Error("igual não deu 0, ou árvore não veio antes de Bolsa")
	}
}
