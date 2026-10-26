package cambio

import (
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func TestConversor(t *testing.T) {
	t.Parallel()
	v := testeapoio.Valor
	ultimas := []Guardada{
		{Moeda: "USD", Data: dia(14), Compra: v("5.4123"), Venda: v("5.4129")},
		{Moeda: "EUR", Data: dia(14), Compra: v("6.2981"), Venda: v("6.2995")},
		{Moeda: "CHF", Data: dia(14), Compra: v("1.005"), Venda: v("1.006")},
		{Moeda: "DKK", Data: dia(14), Compra: v("1.015"), Venda: v("1.016")},
	}
	historico := []Guardada{
		{Moeda: "USD", Data: dia(9), Compra: v("5.3871")},
		{Moeda: "USD", Data: dia(13), Compra: v("5.4012")},
		{Moeda: "USD", Data: dia(1), Compra: v("5.30")},
	}
	casos := []struct {
		nome     string
		conta    func(c *Conversor) *tipos.Decimal
		esperado string // texto com a escala, ou "nil"
	}{
		{"real vale 1", func(c *Conversor) *tipos.Decimal { return c.Taxa("BRL") }, "1"},
		{"código inválido conta como real", func(c *Conversor) *tipos.Decimal { return c.Taxa("dólar") }, "1"},
		{"taxa de compra, não a de venda", func(c *Conversor) *tipos.Decimal { return c.Taxa(" usd") }, "5.4123"},
		{"sem cotação: nil", func(c *Conversor) *tipos.Decimal { return c.Taxa("GBP") }, "nil"},
		{"arredonda para duas casas", func(c *Conversor) *tipos.Decimal { return c.ParaReais(v("10.0"), "USD") }, "54.12"},
		{"duas casas mesmo com mais na taxa", func(c *Conversor) *tipos.Decimal { return c.ParaReais(v("1"), "EUR") }, "6.30"},
		{"meio centavo vai para o par, para baixo", func(c *Conversor) *tipos.Decimal { return c.ParaReais(v("1"), "CHF") }, "1.00"},
		{"meio centavo vai para o par, para cima", func(c *Conversor) *tipos.Decimal { return c.ParaReais(v("1"), "DKK") }, "1.02"},
		{"negativo", func(c *Conversor) *tipos.Decimal { return c.ParaReais(v("-100"), "USD") }, "-541.23"},
		{"zero", func(c *Conversor) *tipos.Decimal { return c.ParaReais(v("0"), "USD") }, "0.00"},
		{"pela data: o dia útil até a data", func(c *Conversor) *tipos.Decimal { return c.ParaReaisNaData(v("100"), "USD", dia(12)) }, "538.71"},
		{"pela data: a do próprio dia", func(c *Conversor) *tipos.Decimal { return c.ParaReaisNaData(v("100"), "USD", dia(13)) }, "540.12"},
		{"pela data: mais de 7 dias antes não vale, vai a última", func(c *Conversor) *tipos.Decimal {
			return c.ParaReaisNaData(v("100"), "USD", dia(30))
		}, "541.23"},
		{"pela data: antes do histórico, a última", func(c *Conversor) *tipos.Decimal {
			return c.ParaReaisNaData(v("100"), "USD", tipos.NovaData(2026, 9, 1))
		}, "541.23"},
		{"pela data em real: o valor como está, sem arredondar", func(c *Conversor) *tipos.Decimal {
			return c.ParaReaisNaData(v("10.555"), "BRL", dia(1))
		}, "10.555"},
		{"pela data, moeda sem histórico: a última", func(c *Conversor) *tipos.Decimal { return c.ParaReaisNaData(v("1"), "EUR", dia(2)) }, "6.30"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			conv := NovoConversor(ultimas, historico)
			obtido := "nil"
			if r := c.conta(conv); r != nil {
				obtido = r.String()
			}
			if obtido != c.esperado {
				t.Errorf("obtido %s, esperado %s", obtido, c.esperado)
			}
		})
	}
}

func TestConversorAnotaAsMoedasSemCotacaoEmOrdem(t *testing.T) {
	t.Parallel()
	c := NovoConversor(nil, nil)
	for _, m := range []string{"JPY", "chf", "BRL", "AUD", "JPY"} {
		c.Taxa(m)
	}
	if obtido := strings.Join(c.MoedasSemCotacao(), ","); obtido != "AUD,CHF,JPY" {
		t.Errorf("sem cotação: %s (sem repetir, em ordem, sem o real)", obtido)
	}
}

func TestVencida(t *testing.T) {
	t.Parallel()
	// Horas em Brasília (UTC-3) para ler fácil; o Vencida recebe e compara em UTC.
	brasilia := func(dia, hora, minuto int) time.Time {
		return time.Date(2026, 10, dia, hora+3, minuto, 0, 0, time.UTC)
	}
	casos := []struct {
		nome    string
		ultima  *time.Time
		agora   time.Time
		vencida bool
	}{
		{"nunca rodou", nil, brasilia(14, 9, 0), true},
		{"rodou ontem", new(brasilia(13, 18, 0)), brasilia(14, 9, 0), true},
		{"rodou hoje cedo, ainda antes do fechamento", new(brasilia(14, 8, 0)), brasilia(14, 13, 29), false},
		{"às 13h30 em ponto, com a última antes", new(brasilia(14, 8, 0)), brasilia(14, 13, 30), true},
		{"depois do fechamento, com a última depois", new(brasilia(14, 13, 45)), brasilia(14, 16, 0), false},
		// 22h em Brasília já é o dia seguinte em UTC: quem comparar o dia em UTC acha que "não rodou hoje".
		{"22h de hoje em Brasília é amanhã em UTC", new(brasilia(14, 14, 0)), brasilia(14, 22, 0), false},
		{"virou o dia em Brasília", new(brasilia(14, 22, 0)), brasilia(15, 0, 10), true},
		{"virada do mês", new(time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC)), time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC), true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if obtido := Vencida(c.ultima, c.agora); obtido != c.vencida {
				t.Errorf("Vencida = %v, esperado %v", obtido, c.vencida)
			}
		})
	}
}
