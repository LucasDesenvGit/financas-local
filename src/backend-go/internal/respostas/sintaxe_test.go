package respostas

import (
	"encoding/json"
	"os"
	"testing"
)

// Regra: a mensagem, o caminho e a posição do JSON que não abre são os do System.Text.Json. O gabarito foi gravado
// pelo próprio .NET (testdata/sonda-json, #402): rode a sonda de novo para acrescentar casos. A leitura é a do
// LerObjeto, com os campos do record da sonda (Credenciais: Login e Senha).
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
		var login, senha CampoTexto
		login, senha = CampoTexto{Propriedade: "Login", Maximo: 100}, CampoTexto{Propriedade: "Senha", Maximo: 100}
		_, e := lerObjeto([]byte(g.Corpo), "Credenciais", doRecord("Credenciais"), []Campo{&login, &senha})
		if e == nil {
			t.Errorf("%q: Go leu sem erro; .NET: %s | %s", g.Corpo, g.Caminho, g.Mensagem)
			continue
		}
		if e.Caminho != g.Caminho || e.Mensagem != g.Mensagem {
			t.Errorf("%q:\n  Go:   %s | %s\n  .NET: %s | %s", g.Corpo, e.Caminho, e.Mensagem, g.Caminho, g.Mensagem)
		}
	}
}
