package respostas

import (
	"encoding/json"
	"os"
	"testing"
)

// Regra: a mensagem, o caminho e a posição do JSON que não abre são os do System.Text.Json. O gabarito foi gravado
// pelo próprio .NET (testdata/sonda-json, #402): rode a sonda de novo para acrescentar casos.
func TestErroDeSintaxeIgualAoDotnet(t *testing.T) {
	dados, err := os.ReadFile("testdata/erros-json-dotnet.json")
	if err != nil {
		t.Fatal(err)
	}
	var gabarito []struct {
		Corpo, Caminho, Mensagem string
	}
	if err := json.Unmarshal(dados, &gabarito); err != nil {
		t.Fatal(err)
	}
	for _, g := range gabarito {
		if g.Mensagem == "OK" {
			continue
		}
		var qualquer any
		e := ErroDeSintaxe([]byte(g.Corpo), json.Unmarshal([]byte(g.Corpo), &qualquer))
		if e.Caminho != g.Caminho || e.Mensagem != g.Mensagem {
			t.Errorf("%q:\n  Go:   %s | %s\n  .NET: %s | %s", g.Corpo, e.Caminho, e.Mensagem, g.Caminho, g.Mensagem)
		}
	}
}
