package respostas

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Regra: o int e o bool da query são lidos como o TypeConverter do ASP.NET (sinal, espaços, hexadecimal com 0x, # e
// &h, 32 bits; true/false sem caixa). O gabarito é o testdata/query.json, gravado pela sonda das contas
// (internal/faturas/testdata/sonda-csharp) com o próprio .NET.
func TestQueryIgualAoASPNET(t *testing.T) {
	dados, err := os.ReadFile("testdata/query.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Valores []struct{ Valor, Inteiro, Booleano string } `json:"valores"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Valores) < 35 {
		t.Fatalf("o gabarito tem só %d casos: a sonda mudou?", len(g.Valores))
	}
	for _, c := range g.Valores {
		inteiro := "erro"
		if n, ok := InteiroDaQuery(c.Valor); ok {
			inteiro = strconv.Itoa(int(n))
		}
		booleano := "erro"
		if b, ok := BoolDaQuery(c.Valor); ok {
			booleano = map[bool]string{true: "True", false: "False"}[b]
		}
		if inteiro != c.Inteiro || booleano != c.Booleano {
			t.Errorf("%q: inteiro %s e bool %s; o .NET dá %s e %s", c.Valor, inteiro, booleano, c.Inteiro, c.Booleano)
		}
	}
}

// Regra: o parâmetro da query não diferencia maiúsculas, vale o primeiro na ordem da URL e é decodificado (%20 e +).
func TestParametroDaQuery(t *testing.T) {
	casos := []struct {
		query, nome, valor string
		achou              bool
	}{
		{"ano=2026&mes=10", "mes", "10", true},
		{"Ano=1&ano=2", "ano", "1", true},
		{"ano=2&Ano=1", "ANO", "2", true},
		{"texto=a+b%20c", "texto", "a b c", true},
		{"te%78to=x", "texto", "x", true},
		{"ano", "ano", "", true},
		{"ano=%zz", "ano", "%zz", true},
		{"mes=1", "ano", "", false},
		{"", "ano", "", false},
	}
	for _, c := range casos {
		r := httptest.NewRequest("GET", "/x?"+c.query, nil)
		if valor, achou := ParametroDaQuery(r, c.nome); valor != c.valor || achou != c.achou {
			t.Errorf("%q, %q: %q %v; esperado %q %v", c.query, c.nome, valor, achou, c.valor, c.achou)
		}
	}
}

// Regra: a mensagem do 400 da query: vazio (ou só espaço) é "is invalid", o resto "is not valid".
func TestMensagemDaQuery(t *testing.T) {
	for valor, esperado := range map[string]string{"": "The value '' is invalid.", " ": "The value ' ' is invalid.",
		"abc": "The value 'abc' is not valid."} {
		if m := MensagemDaQuery(valor); m != esperado || !strings.HasPrefix(m, "The value '") {
			t.Errorf("MensagemDaQuery(%q) = %q, esperado %q", valor, m, esperado)
		}
	}
}
