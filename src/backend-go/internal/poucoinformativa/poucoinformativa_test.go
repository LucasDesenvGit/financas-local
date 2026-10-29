package poucoinformativa

import (
	"encoding/json"
	"os"
	"testing"
)

// Regra: as letras úteis são as do C# (sem acento, pontuação, números soltos e termos genéricos; o char UTF-16). O
// gabarito é o testdata/letras.json, gravado pela sonda das contas (internal/faturas/testdata/sonda-csharp).
func TestLetrasUteisIgualAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/letras.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Letras []struct {
			Descricao string `json:"descricao"`
			Letras    int    `json:"letras"`
		} `json:"letras"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Letras) < 40 {
		t.Fatalf("o gabarito tem só %d casos: a sonda mudou?", len(g.Letras))
	}
	for _, c := range g.Letras {
		if obtido := LetrasUteis(c.Descricao); obtido != c.Letras {
			t.Errorf("LetrasUteis(%q) = %d, o C# dá %d", c.Descricao, obtido, c.Letras)
		}
	}
}

// Regra: pouco informativa é ter menos letras úteis que o mínimo (o mínimo exato já não é).
func TestEhPoucoInformativa(t *testing.T) {
	casos := []struct {
		descricao string
		minimo    int
		esperado  bool
	}{
		{"PIX ENVIADO", 4, true}, {"PIX JOSE", 4, false}, {"PIX ANA", 4, true}, {"PIX ANA", 3, false}, {"", 0, false},
	}
	for _, c := range casos {
		if obtido := EhPoucoInformativa(c.descricao, c.minimo); obtido != c.esperado {
			t.Errorf("EhPoucoInformativa(%q, %d) = %v, esperado %v", c.descricao, c.minimo, obtido, c.esperado)
		}
	}
}
