package texto

import (
	"encoding/json"
	"os"
	"testing"
)

// gabarito é o testdata/normalizar.json, gravado pela sonda testdata/sonda-csharp com o código da própria API.
type gabarito struct {
	Dotnet string `json:"dotnet"`
	Casos  []struct {
		Entrada  string `json:"entrada"`
		Regras   string `json:"regras"`
		Palavras string `json:"palavras"`
	} `json:"casos"`
}

func lerGabarito(t *testing.T) gabarito {
	t.Helper()
	dados, err := os.ReadFile("testdata/normalizar.json")
	if err != nil {
		t.Fatal(err)
	}
	var g gabarito
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Casos) < 70 {
		t.Fatalf("o gabarito tem só %d casos: a sonda mudou?", len(g.Casos))
	}
	return g
}

// Regra: as duas normalizações dão o mesmo texto que o C# (acentos, maiúsculas do ToUpperInvariant, espaços,
// caracteres fora do plano básico, hangul decomposto em ParaPalavras), em todos os casos do gabarito.
func TestNormalizarIgualAoCSharp(t *testing.T) {
	for _, c := range lerGabarito(t).Casos {
		t.Run(c.Entrada, func(t *testing.T) {
			if got := ParaRegras(c.Entrada); got != c.Regras {
				t.Errorf("ParaRegras(%q) = %q, o C# dá %q", c.Entrada, got, c.Regras)
			}
			if got := ParaPalavras(c.Entrada); got != c.Palavras {
				t.Errorf("ParaPalavras(%q) = %q, o C# dá %q", c.Entrada, got, c.Palavras)
			}
		})
	}
}

// Regra: palavra inteira, não pedaço ("PAG" não acha "PAGAMENTO"; "PAGAMENTO CARTAO" acha a expressão).
func TestTemPalavra(t *testing.T) {
	casos := []struct {
		nome, texto string
		termos      []string
		esperado    bool
	}{
		{"palavra no meio", "PIX ENVIADO JOAO", []string{"ENVIADO"}, true},
		{"pedaço de palavra não vale", "PAGAMENTO FATURA", []string{"PAG"}, false},
		{"expressão inteira", "PAGTO CARTAO NUBANK", []string{"PAGAMENTO CARTAO", "PAGTO CARTAO"}, true},
		{"no começo e no fim", "TARIFA", []string{"TARIFA"}, true},
		{"nenhum termo", "SUPERMERCADO", []string{"MERCADO", "SUPER"}, false},
		{"texto vazio", "", []string{"PIX"}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := TemPalavra(c.texto, c.termos...); got != c.esperado {
				t.Errorf("TemPalavra(%q, %q) = %v, esperado %v", c.texto, c.termos, got, c.esperado)
			}
		})
	}
}

// Regra: o tamanho é o string.Length do C#, em unidades UTF-16: acento conta 1 e o que está fora do plano básico
// (emoji) conta 2.
func TestTamanhoUtf16(t *testing.T) {
	t.Parallel()
	casos := map[string]int{"": 0, "abc": 3, "ação": 4, "R$ 1.234,56": 11, "😀": 2, "a😀b": 4, "日本": 2}
	for entrada, quer := range casos {
		if obtido := TamanhoUtf16(entrada); obtido != quer {
			t.Errorf("TamanhoUtf16(%q) = %d, quer %d", entrada, obtido, quer)
		}
	}
}
