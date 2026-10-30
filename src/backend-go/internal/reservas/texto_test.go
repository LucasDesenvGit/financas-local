package reservas

import (
	"encoding/json"
	"os"
	"testing"
)

// Regra: o texto das reservas é o do C# (testdata/texto.json, gravado pela sonda das contas): os dólares na descrição
// do Méli Dólar (vírgula decimal, ponto de milhar, os dígitos e os espaços de Unicode do Regex do .NET, o "ſ" que o
// RE2 casaria com o "s", só maior que zero, o limite do decimal), o nome da reserva (aparado, até 100) e o "é Méli
// Dólar".
func TestTextoIgualAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/texto.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Quantidades []struct {
			Descricao  string  `json:"descricao"`
			Quantidade *string `json:"quantidade"`
		} `json:"quantidades"`
		Nomes []struct {
			Descricao *string `json:"descricao"`
			Nome      *string `json:"nome"`
			Movimento bool    `json:"movimento"`
		} `json:"nomes"`
		Meli []struct {
			Descricao string `json:"descricao"`
			Meli      bool   `json:"meli"`
		} `json:"meli"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Quantidades) < 25 || len(g.Nomes) < 10 || len(g.Meli) < 6 {
		t.Fatal("gabarito pequeno: a sonda mudou?")
	}
	texto := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}
	for _, c := range g.Quantidades {
		obtido := "<nil>"
		if q := QuantidadeNaDescricao(c.Descricao); q != nil {
			obtido = q.String()
		}
		if obtido != texto(c.Quantidade) {
			t.Errorf("QuantidadeNaDescricao(%q) = %s, o C# dá %s", c.Descricao, obtido, texto(c.Quantidade))
		}
	}
	for _, c := range g.Nomes {
		var nome *string
		if c.Descricao != nil {
			nome = ExtrairNome(*c.Descricao)
		}
		if texto(nome) != texto(c.Nome) {
			t.Errorf("ExtrairNome(%q) = %s, o C# dá %s", texto(c.Descricao), texto(nome), texto(c.Nome))
		}
		if EhMovimentoDeReserva(c.Descricao) != c.Movimento {
			t.Errorf("EhMovimentoDeReserva(%q) = %v, o C# dá %v", texto(c.Descricao), !c.Movimento, c.Movimento)
		}
	}
	for _, c := range g.Meli {
		if EhMeliDolar(c.Descricao) != c.Meli {
			t.Errorf("EhMeliDolar(%q) = %v, o C# dá %v", c.Descricao, !c.Meli, c.Meli)
		}
	}
}
