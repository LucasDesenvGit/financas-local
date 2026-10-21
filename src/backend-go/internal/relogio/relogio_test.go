package relogio

import (
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func TestHojeEmBrasilia(t *testing.T) {
	casos := []struct {
		nome     string
		agora    time.Time
		esperado string
	}{
		{"23h30 em Brasília ainda é o dia anterior em UTC+0", time.Date(2026, 10, 16, 2, 30, 0, 0, time.UTC), "2026-10-15"},
		{"meia-noite em Brasília", time.Date(2026, 10, 16, 3, 0, 0, 0, time.UTC), "2026-10-16"},
		{"horário de verão de 2018 (UTC-2)", time.Date(2018, 12, 1, 1, 59, 0, 0, time.UTC), "2018-11-30"},
		{"virada do ano", time.Date(2027, 1, 1, 2, 59, 0, 0, time.UTC), "2026-12-31"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := Hoje(Fixo{Instante: c.agora}).String(); got != c.esperado {
				t.Errorf("Hoje = %s, esperado %s", got, c.esperado)
			}
		})
	}
}

func TestInicioDoDiaUtc(t *testing.T) {
	casos := map[string]time.Time{
		"2026-10-15": time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC),
		"2018-12-01": time.Date(2018, 12, 1, 2, 0, 0, 0, time.UTC), // horário de verão: UTC-2
	}
	for dia, esperado := range casos {
		d, _ := tipos.LerData(dia)
		if got := InicioDoDiaUtc(d); !got.Equal(esperado) {
			t.Errorf("InicioDoDiaUtc(%s) = %s, esperado %s", dia, got, esperado)
		}
	}
}

func TestUltimoDiaMes(t *testing.T) {
	for _, c := range []struct {
		ano      int
		mes      time.Month
		esperado string
	}{{2024, time.February, "2024-02-29"}, {2026, time.February, "2026-02-28"}, {2026, time.December, "2026-12-31"}} {
		if got := UltimoDiaMes(c.ano, c.mes).String(); got != c.esperado {
			t.Errorf("UltimoDiaMes(%d, %d) = %s, esperado %s", c.ano, c.mes, got, c.esperado)
		}
	}
}

func TestSistemaEmUtc(t *testing.T) {
	if (Sistema{}).Agora().Location() != time.UTC {
		t.Error("Sistema.Agora deveria devolver UTC")
	}
}
