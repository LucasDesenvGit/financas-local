package parcelas

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func data(s string) tipos.Data {
	d, err := tipos.LerData(s)
	if err != nil {
		panic(err)
	}
	return d
}

func valor(s string) tipos.Decimal {
	d, err := tipos.NovoDecimal(s)
	if err != nil {
		panic(err)
	}
	return d
}

// Regra (#165, #167): a parcela de data futura entra como é; de cada compra, só a de maior número projeta as que
// faltam (as de antes contavam em dobro), e só as que caem depois de hoje; a mesma compra se reconhece pela conta, o
// total, os reais inteiros (291,19 e 291,17) e as letras da descrição; parcela inválida ou zerada fica de fora.
func TestProjetar(t *testing.T) {
	t.Parallel()
	hoje := data("2026-10-14")
	casos := []struct {
		nome     string
		parcelas []Parcela
		quer     string // "conta data valor banco origem passos", separadas por "; "
	}{
		{"projeta as que faltam", []Parcela{{1, 7, data("2026-10-05"), valor("-100.50"), 3, 5, "LOJA 03/05"}},
			"7 2026-11-05 100.50 false 1 1; 7 2026-12-05 100.50 false 1 2"},
		{"só a última parcela da compra projeta", []Parcela{
			{1, 7, data("2026-09-05"), valor("-291.19"), 1, 3, "MERCADOLIVRE*MERCADOLIVRE 1/3"},
			{2, 7, data("2026-10-05"), valor("-291.17"), 2, 3, "MERCADOLIVRE*MERC 2/3"}},
			"7 2026-11-05 291.17 false 2 1"},
		{"parcela já no banco com data futura", []Parcela{{9, 7, data("2026-11-05"), valor("-50"), 4, 4, "CURSO 4/4"}},
			"7 2026-11-05 50 true 9 0"},
		{"compras diferentes por conta", []Parcela{
			{1, 7, data("2026-10-01"), valor("-10"), 1, 2, "A 1/2"},
			{2, 8, data("2026-10-01"), valor("-10"), 1, 2, "A 1/2"}},
			"7 2026-11-01 10 false 1 1; 8 2026-11-01 10 false 2 1"},
		{"o mês que não cai depois de hoje fica de fora", []Parcela{{1, 7, data("2026-09-14"), valor("-10"), 1, 2, "B"}}, ""},
		{"inválidas", []Parcela{
			{1, 7, data("2026-10-01"), valor("-10"), 1, 1, "à vista"},
			{2, 7, data("2026-10-01"), valor("-10"), 0, 3, "número zero"},
			{3, 7, data("2026-10-01"), valor("-10"), 4, 3, "número acima do total"},
			{4, 7, data("2026-10-01"), valor("0"), 1, 3, "zerada"}}, ""},
		{"compra de uma parcela só (à vista) com data futura também fica de fora", []Parcela{
			{1, 7, data("2026-11-05"), valor("-10"), 1, 1, "à vista"}}, ""},
		{"parcela de hoje já no banco não é futura", []Parcela{{1, 7, data("2026-10-14"), valor("-10"), 4, 4, "CURSO 4/4"}}, ""},
		{"duas parcelas com o mesmo número: vale a primeira da lista (o MaxBy do LINQ)", []Parcela{
			{1, 7, data("2026-10-05"), valor("-10"), 1, 3, "D 1/3"}, {2, 7, data("2026-10-06"), valor("-10"), 1, 3, "D 1/3"}},
			"7 2026-11-05 10 false 1 1; 7 2026-12-05 10 false 1 2"},
		{"fim de mês (AddMonths)", []Parcela{{1, 7, data("2026-08-31"), valor("-1"), 1, 4, "C"}},
			"7 2026-10-31 1 false 1 2; 7 2026-11-30 1 false 1 3"},
	}
	for _, c := range casos {
		var partes []string
		for _, f := range Projetar(c.parcelas, hoje) {
			partes = append(partes, fmt.Sprintf("%d %s %s %t %d %d", f.ContaId, f.Data, f.Valor, f.JaNoBanco, f.OrigemId, f.Passos))
		}
		if obtido := strings.Join(partes, "; "); obtido != c.quer {
			t.Errorf("%s:\n obtido %s\n quer   %s", c.nome, obtido, c.quer)
		}
	}
}
