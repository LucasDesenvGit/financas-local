package relogio

import (
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Fuzz do relógio (#372): as sementes ficam aqui; a entrada que achou bug, em testdata/fuzz/<alvo>/. Para fuzzar
// de verdade: go test ./internal/relogio -run '^$' -fuzz '^FuzzInicioDoDiaEmBrasilia$' -fuzztime 5m

// noIntervalo leva qualquer número para dentro de [de, de+tamanho); o que já está dentro fica como está.
func noIntervalo(n, de, tamanho int64) int64 { return de + (((n-de)%tamanho)+tamanho)%tamanho }

// FuzzInicioDoDiaEmBrasilia: qualquer instante cai entre o começo do dia de Brasília (InicioDoDiaUtc do Hoje) e o
// começo do dia seguinte, e um nanossegundo antes do começo já é o dia anterior. Vale também nos dias em que o
// horário de verão (até 2019) começava ou acabava à meia-noite.
func FuzzInicioDoDiaEmBrasilia(f *testing.F) {
	for _, s := range []string{
		"2026-10-15T02:59:59Z", "2026-10-15T03:00:00Z", // a meia-noite de Brasília, em UTC
		"2018-11-04T02:30:00Z", "2018-11-04T03:00:00Z", // o horário de verão começava: de 23:59:59 para 01:00
		"2019-02-17T01:59:59Z", "2019-02-17T02:30:00Z", "2019-02-17T03:00:00Z", // e acabava: 23h de novo
		"2024-02-29T12:00:00Z", "1970-01-01T00:00:00Z", "1913-12-31T23:00:00Z", // o fuso de Brasília nasceu em 1914
	} {
		instante, err := time.Parse(time.RFC3339, s)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(instante.Unix(), int64(0))
	}
	de := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	ate := time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	f.Fuzz(func(t *testing.T, segundos, nanos int64) {
		instante := time.Unix(noIntervalo(segundos, de, ate-de), noIntervalo(nanos, 0, int64(time.Second))).UTC()
		hoje := Hoje(Fixo{Instante: instante})
		inicio, amanha := InicioDoDiaUtc(hoje), InicioDoDiaUtc(hoje.SomarDias(1))
		if instante.Before(inicio) || !instante.Before(amanha) {
			t.Fatalf("%s é %s em Brasília, mas o dia vai de %s a %s", instante, hoje, inicio, amanha)
		}
		if dia := Hoje(Fixo{Instante: inicio}); dia.String() != hoje.String() {
			t.Fatalf("InicioDoDiaUtc(%s) = %s, que em Brasília é %s", hoje, inicio, dia)
		}
		if antes := Hoje(Fixo{Instante: inicio.Add(-time.Nanosecond)}); antes.String() != hoje.SomarDias(-1).String() {
			t.Fatalf("1 ns antes de InicioDoDiaUtc(%s) = %s ainda é %s em Brasília", hoje, inicio, antes)
		}
	})
}

// diasNoMes é o DateTime.DaysInMonth do C#, feito à parte: o calendário gregoriano, com 29/02 nos anos divisíveis
// por 4, menos os de 100 que não são de 400.
func diasNoMes(ano int, mes time.Month) int {
	switch mes {
	case time.February:
		if ano%4 == 0 && (ano%100 != 0 || ano%400 == 0) {
			return 29
		}
		return 28
	case time.April, time.June, time.September, time.November:
		return 30
	}
	return 31
}

// FuzzPrimeiroEUltimoDiaMes: em qualquer mês do ano 1 ao 9999 (a faixa do DateOnly), o primeiro dia é o 1 e o
// último é o do calendário (28, 29, 30 ou 31), do mesmo mês e ano; o dia seguinte ao último é o 1 do mês seguinte.
func FuzzPrimeiroEUltimoDiaMes(f *testing.F) {
	for _, s := range []struct{ ano, mes int }{
		{2024, 2}, {2026, 2}, {1900, 2}, {2000, 2}, {2026, 4}, {2026, 12}, {2026, 1}, {1, 1}, {9999, 12},
	} {
		f.Add(s.ano, s.mes)
	}
	f.Fuzz(func(t *testing.T, a, m int) {
		ano, mes := int(noIntervalo(int64(a), 1, 9999)), time.Month(noIntervalo(int64(m), 1, 12))
		primeiro, ultimo := PrimeiroDiaMes(ano, mes), UltimoDiaMes(ano, mes)
		if primeiro.String() != tipos.NovaData(ano, mes, 1).String() || primeiro.Dia() != 1 {
			t.Fatalf("PrimeiroDiaMes(%d, %d) = %s", ano, mes, primeiro)
		}
		if ultimo.Ano() != ano || ultimo.Mes() != mes || ultimo.Dia() != diasNoMes(ano, mes) {
			t.Fatalf("UltimoDiaMes(%d, %d) = %s, esperado o dia %d", ano, mes, ultimo, diasNoMes(ano, mes))
		}
		seguinte := ultimo.SomarDias(1)
		if seguinte.Dia() != 1 || seguinte.Mes() != mes%12+1 {
			t.Fatalf("o dia seguinte a %s é %s", ultimo, seguinte)
		}
	})
}
