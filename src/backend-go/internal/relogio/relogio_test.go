package relogio

import (
	"testing"
	"testing/synctest"
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
		"2018-11-04": time.Date(2018, 11, 4, 3, 0, 0, 0, time.UTC), // começo do horário de verão: sem meia-noite, 01:00
		"2019-02-17": time.Date(2019, 2, 17, 3, 0, 0, 0, time.UTC), // fim: 23h de novo no dia 16, e o 17 às 00:00 de UTC-3
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

// relogioDeMentira é um "time.Now" que só anda quando o teste manda.
type relogioDeMentira struct{ t time.Time }

func (r *relogioDeMentira) agora() time.Time { return r.t }

func TestAPartirDe(t *testing.T) {
	inicio := time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC) // o InicioPadrao do contrato
	casos := []struct {
		nome       string
		inicio     time.Time
		passou     time.Duration
		agora      string // Agora(), em RFC 3339
		hoje       string // Hoje(), em Brasília
		agoraLocal string // AgoraLocal()
	}{
		{"na partida marca o início", inicio, 0, "2026-10-15T01:00:00Z", "2026-10-14", "2026-10-14 22:00:00"},
		{"anda com o tempo (não é parado)", inicio, 90 * time.Second, "2026-10-15T01:01:30Z", "2026-10-14", "2026-10-14 22:01:30"},
		{"bloqueio de login de 15 minutos", inicio, 15 * time.Minute, "2026-10-15T01:15:00Z", "2026-10-14", "2026-10-14 22:15:00"},
		{"vira o dia em Brasília 2 h depois", inicio, 2 * time.Hour, "2026-10-15T03:00:00Z", "2026-10-15", "2026-10-15 00:00:00"},
		{"fração de segundo", inicio, 1500 * time.Microsecond, "2026-10-15T01:00:00.0015Z", "2026-10-14", "2026-10-14 22:00:00"},
		{"início em outro fuso: devolve em UTC", time.Date(2026, 10, 14, 22, 0, 0, 0, Brasilia), time.Minute, "2026-10-15T01:01:00Z", "2026-10-14", "2026-10-14 22:01:00"},
		{"horário de verão antigo (UTC-2)", time.Date(2018, 12, 1, 1, 59, 0, 0, time.UTC), 2 * time.Minute, "2018-12-01T02:01:00Z", "2018-12-01", "2018-12-01 00:01:00"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			// O "agora de verdade" fica longe do início, de propósito: a hora do computador não pode vazar.
			real := &relogioDeMentira{t: time.Date(2031, 3, 9, 12, 0, 0, 0, time.UTC)}
			r := novoAPartirDe(c.inicio, real.agora)
			real.t = real.t.Add(c.passou)
			if got := r.Agora().Format(time.RFC3339Nano); got != c.agora {
				t.Errorf("Agora = %s, esperado %s", got, c.agora)
			}
			if r.Agora().Location() != time.UTC {
				t.Error("Agora deveria devolver UTC, como o Sistema")
			}
			if got := Hoje(r).String(); got != c.hoje {
				t.Errorf("Hoje = %s, esperado %s", got, c.hoje)
			}
			if got := AgoraLocal(r).Format("2006-01-02 15:04:05"); got != c.agoraLocal {
				t.Errorf("AgoraLocal = %s, esperado %s", got, c.agoraLocal)
			}
		})
	}
}

// Com o time.Now: começa no início e anda para a frente. Dentro do synctest o time.Now é de mentira e o time.Sleep
// só adianta esse relógio (sem esperar nada de verdade), então o tempo que passou é exatamente o do Sleep (#458).
func TestNovoAPartirDeAnda(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		inicio := time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)
		r := NovoAPartirDe(inicio)
		primeiro := r.Agora()
		time.Sleep(20 * time.Millisecond)
		segundo := r.Agora()
		if !primeiro.Equal(inicio) {
			t.Errorf("primeira leitura %s, esperado o início %s", primeiro, inicio)
		}
		if d := segundo.Sub(primeiro); d != 20*time.Millisecond {
			t.Errorf("andou %s em 20 ms", d)
		}
	})
}

func TestLerInicio(t *testing.T) {
	casos := []struct {
		texto    string
		esperado string // vazio: tem de dar erro
	}{
		{"2026-10-15T01:00:00Z", "2026-10-15T01:00:00Z"},
		{"2026-10-14T22:00:00-03:00", "2026-10-15T01:00:00Z"},
		{"2026-10-15T01:00:00.0000000+00:00", "2026-10-15T01:00:00Z"}, // o ToString("O") do .NET
		{"2026-10-15T01:00:02.5Z", "2026-10-15T01:00:02.5Z"},
		{"2026-10-15T01:00:00", ""},  // sem fuso: dependeria do computador
		{"2026-10-15 01:00:00Z", ""}, // espaço no lugar do T
		{"1792026000", ""},           // segundos desde 1970, como no FAKETIME: não
		{"amanhã", ""},
		{"", ""},
	}
	for _, c := range casos {
		t.Run(c.texto, func(t *testing.T) {
			got, err := LerInicio(c.texto)
			if c.esperado == "" {
				if err == nil {
					t.Fatalf("aceitou %q (%s)", c.texto, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if s := got.Format(time.RFC3339Nano); s != c.esperado || got.Location() != time.UTC {
				t.Errorf("LerInicio(%q) = %s (%s), esperado %s em UTC", c.texto, s, got.Location(), c.esperado)
			}
		})
	}
}
