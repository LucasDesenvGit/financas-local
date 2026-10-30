package selic

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func data(t *testing.T, texto string) tipos.Data {
	t.Helper()
	d, err := tipos.LerData(texto)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// Regra: o fator é o da SerieSelic do C# (testdata/serie.json, gravado pela sonda das contas): o produto em double dos
// dias em [de, ate), as datas sem taxa (fim de semana, feriado, antes e depois da série) e os 15 dígitos do (decimal).
// As taxas entram fora de ordem.
func TestFatorIgualAoCSharp(t *testing.T) {
	t.Parallel()
	dados, err := os.ReadFile("testdata/serie.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Taxas []struct {
			Data string `json:"data"`
			Taxa string `json:"taxa"`
		} `json:"taxas"`
		Fatores []struct {
			De    string `json:"de"`
			Ate   string `json:"ate"`
			Fator string `json:"fator"`
		} `json:"fatores"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Taxas) < 100 || len(g.Fatores) < 80 {
		t.Fatal("gabarito pequeno: a sonda mudou?")
	}
	var taxas []Taxa
	for _, x := range g.Taxas {
		taxas = append(taxas, Taxa{Data: data(t, x.Data), Taxa: testeapoio.Valor(x.Taxa)})
	}
	slices.Reverse(taxas)
	s := NovaSerie(taxas)
	if s.Vazia() || s.UltimaData().String() != g.Taxas[len(g.Taxas)-1].Data {
		t.Errorf("vazia %v, última data %s", s.Vazia(), s.UltimaData())
	}
	for _, c := range g.Fatores {
		f, err := s.Fator(data(t, c.De), data(t, c.Ate))
		if err != nil || f.String() != c.Fator {
			t.Errorf("Fator(%s, %s) = %s, %v; o C# dá %s", c.De, c.Ate, f, err, c.Fator)
		}
	}
}

func TestSerieVazia(t *testing.T) {
	t.Parallel()
	s := NovaSerie(nil)
	if !s.Vazia() || s.UltimaData() != nil {
		t.Errorf("vazia %v, última data %v", s.Vazia(), s.UltimaData())
	}
	f, err := s.Fator(testeapoio.Dia(2026, 1, 1), testeapoio.Dia(2026, 2, 1))
	if err != nil || f.String() != "1" {
		t.Errorf("fator %s, %v", f, err)
	}
}

// Regra: o fator que não cabe num decimal é a OverflowException do C#.
func TestFatorForaDoDecimal(t *testing.T) {
	t.Parallel()
	enorme := testeapoio.Valor("79228162514264337593543950335")
	s := NovaSerie([]Taxa{{Data: testeapoio.Dia(2026, 1, 1), Taxa: enorme}, {Data: testeapoio.Dia(2026, 1, 2), Taxa: enorme}})
	if _, err := s.Fator(testeapoio.Dia(2026, 1, 1), testeapoio.Dia(2026, 1, 3)); !errors.Is(err, tipos.ErrEstouro) {
		t.Errorf("erro %v", err)
	}
}
