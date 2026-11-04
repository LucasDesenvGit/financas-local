package sugestoes

import (
	"encoding/json"
	"math"
	"os"
	"slices"
	"strconv"
	"testing"
)

// gabarito é o testdata/sugestoes.json, gravado pela sonda das contas (internal/faturas/testdata/sonda-csharp) com o
// SugestaoCategoriaService e a formatação do próprio .NET.
type gabarito struct {
	Percentuais []struct {
		Razao string `json:"razao"`
		Texto string `json:"texto"`
	} `json:"percentuais"`
	Arredondados []struct {
		Total   string `json:"total"`
		Inteiro int    `json:"inteiro"`
	} `json:"arredondados"`
	Palavras []struct {
		Descricao string   `json:"descricao"`
		Palavras  []string `json:"palavras"`
		Chave     string   `json:"chave"`
	} `json:"palavras"`
	TextosRegra []struct {
		Descricoes []string `json:"descricoes"`
		Texto      string   `json:"texto"`
	} `json:"textosRegra"`
}

func lerGabarito(t *testing.T) gabarito {
	t.Helper()
	dados, err := os.ReadFile("testdata/sugestoes.json")
	if err != nil {
		t.Fatal(err)
	}
	var g gabarito
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Percentuais) < 25 || len(g.Palavras) < 15 || len(g.TextosRegra) < 12 {
		t.Fatal("gabarito pequeno: a sonda mudou?")
	}
	return g
}

// Regra: o "P0" do C# em pt-BR: o valor exato do double vezes 100, com o empate exato indo para o par.
func TestPercentualIgualAoCSharp(t *testing.T) {
	for _, c := range lerGabarito(t).Percentuais {
		razao, err := strconv.ParseFloat(c.Razao, 64)
		if err != nil {
			t.Fatal(err)
		}
		if obtido := percentual(razao); obtido != c.Texto {
			t.Errorf("percentual(%s) = %q, o C# dá %q", c.Razao, obtido, c.Texto)
		}
	}
}

// Regra: o "das N parecidas" é o Math.Round do C# (bancário), com 1 no mínimo.
func TestParecidasIgualAoCSharp(t *testing.T) {
	for _, c := range lerGabarito(t).Arredondados {
		total, _ := strconv.ParseFloat(c.Total, 64)
		if obtido := max(1, int(math.RoundToEven(total))); obtido != c.Inteiro {
			t.Errorf("parecidas(%s) = %d, o C# dá %d", c.Total, obtido, c.Inteiro)
		}
	}
}

// Regra: as palavras significativas e a chave dos grupos são as do C#.
func TestPalavrasEChaveIgualAoCSharp(t *testing.T) {
	for _, c := range lerGabarito(t).Palavras {
		if obtido := Palavras(c.Descricao); !slices.Equal(obtido, c.Palavras) {
			t.Errorf("Palavras(%q) = %q, o C# dá %q", c.Descricao, obtido, c.Palavras)
		}
		if obtido := Chave(c.Descricao); obtido != c.Chave {
			t.Errorf("Chave(%q) = %q, o C# dá %q", c.Descricao, obtido, c.Chave)
		}
	}
}

// Regra: o texto sugerido para a regra é o do C# (a palavra que identifica em 80%, a dupla da palavra curta, o começo
// comum que tem palavra que identifica).
func TestTextoRegraIgualAoCSharp(t *testing.T) {
	for _, c := range lerGabarito(t).TextosRegra {
		if obtido := TextoRegra(c.Descricoes); obtido != c.Texto {
			t.Errorf("TextoRegra(%q) = %q, o C# dá %q", c.Descricoes, obtido, c.Texto)
		}
	}
}
