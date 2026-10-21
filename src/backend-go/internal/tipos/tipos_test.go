package tipos

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDecimalCasosLimite(t *testing.T) {
	casos := []struct {
		nome, entrada, json, banco string
	}{
		{"expoente positivo vira inteiro", "1e3", "1000", "1000.0"},
		{"negativo com escala", "-12.340", "-12.340", "-12.34"},
		{"mais de 28 casas arredonda pelo bancário", "0." + strings.Repeat("0", 27) + "25", "0." + strings.Repeat("0", 27) + "2", "0." + strings.Repeat("0", 27) + "2"},
		{"espaços em volta", " 7.5 ", "7.5", "7.5"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, err := NovoDecimal(c.entrada)
			if err != nil {
				t.Fatal(err)
			}
			if got := mustJSON(t, d); got != c.json {
				t.Errorf("JSON = %s, esperado %s", got, c.json)
			}
			if got := d.TextoNoBanco(); got != c.banco {
				t.Errorf("banco = %s, esperado %s", got, c.banco)
			}
		})
	}
}

func TestDecimalJSONDeEntrada(t *testing.T) {
	casos := []struct {
		nome, entrada, esperado string
		erro                    bool
	}{
		{"número", `10.50`, "10.50", false},
		{"número em texto, como o ASP.NET aceita", `"10.50"`, "10.50", false},
		{"null em campo obrigatório", `null`, "", true},
		{"texto que não é número", `"dez"`, "", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var d Decimal
			err := json.Unmarshal([]byte(c.entrada), &d)
			if c.erro {
				if err == nil {
					t.Fatalf("esperava erro, leu %s", d)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if d.String() != c.esperado {
				t.Errorf("leu %s, esperado %s", d, c.esperado)
			}
		})
	}

	var opcional *Decimal
	if err := json.Unmarshal([]byte(`null`), &opcional); err != nil || opcional != nil {
		t.Errorf("null em *Decimal deveria ficar nil, ficou %v (erro %v)", opcional, err)
	}
}

func TestDecimalRecusaReal(t *testing.T) {
	var d Decimal
	if err := d.Scan(12.5); err == nil {
		t.Fatal("dinheiro gravado como REAL deveria dar erro (float64 erra centavos)")
	}
	if err := d.Scan(int64(42)); err != nil || d.String() != "42" {
		t.Errorf("INTEGER deveria ser lido como decimal sem casas: %s, %v", d, err)
	}
}

func TestArredondamentoBancarioComoOCSharp(t *testing.T) {
	// Math.Round(x, 2) do C# arredonda o meio para o par: 0.125 → 0.12, 0.135 → 0.14.
	for entrada, esperado := range map[string]string{"0.125": "0.12", "0.135": "0.14", "-0.125": "-0.12", "2.5": "2"} {
		d, _ := NovoDecimal(entrada)
		casas := int32(2)
		if entrada == "2.5" {
			casas = 0
		}
		if got := DeDecimal(d.Decimal().RoundBank(casas)).String(); got != esperado {
			t.Errorf("RoundBank(%s) = %s, o C# dá %s", entrada, got, esperado)
		}
	}
}

func TestDataHora(t *testing.T) {
	t.Run("ignora o fuso e guarda a hora de parede", func(t *testing.T) {
		brasilia := time.FixedZone("BRT", -3*3600)
		d := NovaDataHora(time.Date(2026, 10, 15, 9, 30, 0, 0, brasilia))
		if d.String() != "2026-10-15 09:30:00" {
			t.Errorf("= %s", d)
		}
	})
	t.Run("corta na precisão do C# (100 ns)", func(t *testing.T) {
		d := NovaDataHora(time.Date(2026, 10, 15, 9, 30, 0, 123456789, time.UTC))
		if d.String() != "2026-10-15 09:30:00.1234567" {
			t.Errorf("= %s", d)
		}
	})
	t.Run("lê o JSON com Z", func(t *testing.T) {
		var d DataHora
		if err := json.Unmarshal([]byte(`"2026-10-15T12:00:00.5Z"`), &d); err != nil || d.String() != "2026-10-15 12:00:00.5" {
			t.Errorf("= %s, %v", d, err)
		}
	})
	t.Run("recusa fuso diferente de Z", func(t *testing.T) {
		if _, err := LerDataHora("2026-10-15T12:00:00-03:00"); err == nil {
			t.Error("esperava erro: o fuso tem de ser convertido antes")
		}
	})
}

func TestData(t *testing.T) {
	d, err := LerData("2024-02-28")
	if err != nil {
		t.Fatal(err)
	}
	if d.SomarDias(1).String() != "2024-02-29" || d.SomarDias(2).String() != "2024-03-01" {
		t.Errorf("ano bissexto errado: %s, %s", d.SomarDias(1), d.SomarDias(2))
	}
	if _, err := LerData("2026-13-01"); err == nil {
		t.Error("mês 13 deveria dar erro")
	}
}

func TestGuid(t *testing.T) {
	g, err := LerGuid("2F00DEFB-672E-45EF-87D0-702995AE8D2E")
	if err != nil {
		t.Fatal(err)
	}
	if g.String() != "2f00defb-672e-45ef-87d0-702995ae8d2e" {
		t.Errorf("= %s", g)
	}
	novo := NovoGuid()
	if novo == GuidVazio || novo.String()[14] != '4' {
		t.Errorf("NovoGuid deveria ser versão 4: %s", novo)
	}
	if _, err := LerGuid("123"); err == nil {
		t.Error("guid curto deveria dar erro")
	}
}
