package parcelas

import (
	"slices"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

func ptr(n int64) *int64 { return &n }

// l monta um lançamento no cartão: id, conta, data (dias depois de 01/01/2026), valor, parcela "n/total" (0 = sem) e texto.
func l(id, conta int64, dia int, valor string, numero, total int64, descricao string) Lancamento {
	x := Lancamento{Id: id, ContaId: conta, Data: testeapoio.Dia(2026, 1, 1).SomarDias(dia), Valor: testeapoio.Valor(valor), Descricao: descricao}
	if total > 0 {
		x.Numero, x.Total = ptr(numero), ptr(total)
	}
	return x
}

func ids(m map[int64]bool) []int64 {
	var r []int64
	for id := range m {
		r = append(r, id)
	}
	slices.Sort(r)
	return r
}

// Regra: ComprasEstornadas (#257, ADR 0036), o ParcelasFuturas.ComprasEstornadas do C#. O estorno do valor total de uma
// compra parcelada (parcela × número de parcelas, com a folga de 10 centavos por parcela mais 5 centavos) casa com a 1ª parcela da
// mesma compra (mesma conta, as 8 primeiras letras da descrição, até 400 dias antes); saem o estorno, a 1ª e as outras
// parcelas já lançadas (valor em reais inteiros ±1, até 35 dias depois do estorno).
func TestComprasEstornadas(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome        string
		lancamentos []Lancamento
		esperado    []int64
	}{
		{"nada", nil, nil},
		{"estorno do total casa a 1ª, a 2ª e a 3ª", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 30, "-100", 2, 3, "LOJA EXEMPLO 02/03"),
			l(3, 1, 60, "-100", 3, 3, "LOJA EXEMPLO 03/03"), l(4, 1, 70, "300", 0, 0, "ESTORNO DE LOJA EXEMPLO"),
		}, []int64{1, 2, 3, 4}},
		{"estorno de uma parcela só não casa", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 5, "100", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"estorno sem compra parcelada não casa", []Lancamento{
			l(1, 1, 0, "-300", 0, 0, "LOJA EXEMPLO"), l(4, 1, 5, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"a folga é de 10 centavos por parcela mais 5 centavos: 300,35 casa", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 5, "300.35", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"a folga é de 10 centavos por parcela mais 5 centavos: 300,36 não casa", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 5, "300.36", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"a folga vale para menos: 299,65 casa", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 5, "299.65", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"a folga vale para menos: 299,64 não casa", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 5, "299.64", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"a folga cresce com o número de parcelas: 10 × 100 aceita 1001,05", []Lancamento{
			l(1, 1, 0, "-100", 1, 10, "LOJA EXEMPLO 01/10"), l(4, 1, 5, "1001.05", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"a folga cresce com o número de parcelas: 10 × 100 não aceita 1001,06", []Lancamento{
			l(1, 1, 0, "-100", 1, 10, "LOJA EXEMPLO 01/10"), l(4, 1, 5, "1001.06", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"a folga é em reais, não em porcentagem do total: 2 × 1000 não aceita 2001", []Lancamento{
			l(1, 1, 0, "-1000", 1, 2, "LOJA EXEMPLO 01/02"), l(4, 1, 5, "2001", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"no 400º dia o estorno ainda casa", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 400, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"no 401º dia não casa mais", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 401, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"estorno no mesmo dia da compra casa", []Lancamento{
			l(1, 1, 7, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 7, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"estorno antes da compra não casa", []Lancamento{
			l(1, 1, 7, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 6, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"outra conta não casa", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 2, 5, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"outra loja não casa", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 5, "300", 0, 0, "ESTORNO OUTRA LOJA"),
		}, nil},
		{"a parcela 2 só entra até 35 dias depois do estorno", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 40, "-100", 2, 3, "LOJA EXEMPLO 02/03"),
			l(3, 1, 41, "-100", 3, 3, "LOJA EXEMPLO 03/03"), l(4, 1, 5, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 2, 4}},
		{"a parcela não pode ser anterior à 1ª", []Lancamento{
			l(1, 1, 10, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 5, "-100", 2, 3, "LOJA EXEMPLO 02/03"),
			l(4, 1, 20, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"o valor da outra parcela pode diferir 1 real, não 2", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 30, "-99.4", 2, 3, "LOJA EXEMPLO 02/03"),
			l(3, 1, 60, "-102", 3, 3, "LOJA EXEMPLO 03/03"), l(4, 1, 70, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 2, 4}},
		{"a parcela de outro total de parcelas não entra", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 30, "-100", 2, 4, "LOJA EXEMPLO 02/04"),
			l(4, 1, 70, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"a parcela de outra conta não entra", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 2, 30, "-100", 2, 3, "LOJA EXEMPLO 02/03"),
			l(4, 1, 70, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"a parcela de outra loja não entra", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 30, "-100", 2, 3, "OUTRA LOJA 02/03"),
			l(4, 1, 70, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"duas compras iguais: cada estorno fica com a mais antiga ainda não casada", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 10, "-100", 1, 3, "LOJA EXEMPLO 01/03"),
			l(4, 1, 20, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"dois estornos para duas compras iguais casam as duas", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 10, "-100", 1, 3, "LOJA EXEMPLO 01/03"),
			l(4, 1, 20, "300", 0, 0, "ESTORNO LOJA EXEMPLO"), l(5, 1, 21, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 2, 4, 5}},
		{"o estorno mais antigo escolhe primeiro, com o id desempatando o dia", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 10, "-100", 1, 3, "LOJA EXEMPLO 01/03"),
			l(6, 1, 20, "300", 0, 0, "ESTORNO LOJA EXEMPLO"), l(5, 1, 20, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 2, 5, 6}},
		{"o prefixo de estorno tem quatro formas e o texto vem sem espaço nas pontas", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 5, "300", 0, 0, "  estorno da LOJA EXEMPLO "),
		}, []int64{1, 4}},
		{"estorno sem o prefixo, com o mesmo texto da compra", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 5, "300", 0, 0, "LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"parcelas sem número ou total, ou fora de 1..total, são ignoradas", []Lancamento{
			l(1, 1, 0, "-100", 4, 3, "LOJA EXEMPLO 04/03"), l(2, 1, 0, "-100", 0, 3, "LOJA EXEMPLO"),
			l(3, 1, 0, "-100", 1, 1, "LOJA EXEMPLO"), l(4, 1, 5, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"número nulo com total (dado quebrado) não derruba", []Lancamento{
			{Id: 1, ContaId: 1, Data: testeapoio.Dia(2026, 1, 1), Valor: testeapoio.Valor("-100"), Total: ptr(3), Descricao: "LOJA EXEMPLO"},
			{Id: 4, ContaId: 1, Data: testeapoio.Dia(2026, 1, 6), Valor: testeapoio.Valor("300"), Descricao: "ESTORNO LOJA EXEMPLO"},
		}, nil},
		{"valor positivo parcelado é estorno, não parcela", []Lancamento{
			l(1, 1, 0, "100", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 5, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"valor zero não é nada", []Lancamento{
			l(1, 1, 0, "0", 1, 3, "LOJA EXEMPLO 01/03"), l(4, 1, 5, "0", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"compra de uma parcela só (1/1) não é compra parcelada", []Lancamento{
			l(1, 1, 0, "-100", 1, 1, "LOJA EXEMPLO"), l(4, 1, 5, "100", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, nil},
		{"a parcela 2 do mesmo dia da 1ª também entra", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 0, "-100", 2, 3, "LOJA EXEMPLO 02/03"),
			l(4, 1, 5, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 2, 4}},
		{"estorno sem compra para casar não impede o seguinte", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"),
			l(3, 1, 3, "300", 0, 0, "ESTORNO OUTRA LOJA"), l(4, 1, 5, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		// Duas compras da mesma loja com valores perto (100,00 e 100,20 por parcela): o estorno de 300,30 serve às duas, o de
		// 300,00 só à primeira e o de 300,60 só à segunda. Quem escolhe primeiro decide o resultado.
		{"estornos no mesmo dia: o de menor id escolhe primeiro (300,30 leva a mais antiga e deixa o de 300,00 sem compra)", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 1, "-100.2", 1, 3, "LOJA EXEMPLO 01/03"),
			l(4, 1, 20, "300.3", 0, 0, "ESTORNO LOJA EXEMPLO"), l(5, 1, 20, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"estornos no mesmo dia: o de menor id escolhe primeiro (300,00 leva a mais antiga e o de 300,30 fica com a outra)", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 1, "-100.2", 1, 3, "LOJA EXEMPLO 01/03"),
			l(4, 1, 20, "300", 0, 0, "ESTORNO LOJA EXEMPLO"), l(5, 1, 20, "300.3", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 2, 4, 5}},
		{"estornos listados fora de ordem: o de menor id ainda escolhe primeiro", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 1, "-100.2", 1, 3, "LOJA EXEMPLO 01/03"),
			l(5, 1, 20, "300", 0, 0, "ESTORNO LOJA EXEMPLO"), l(4, 1, 20, "300.3", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
		{"parcelas do mesmo dia listadas fora de ordem: o estorno escolhe a de menor id", []Lancamento{
			l(3, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 0, "-100.2", 1, 3, "LOJA EXEMPLO 01/03"),
			l(4, 1, 20, "300.3", 0, 0, "ESTORNO LOJA EXEMPLO"), l(5, 1, 21, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{2, 3, 4, 5}},
		{"o estorno de data mais antiga escolhe primeiro, mesmo com id maior", []Lancamento{
			l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(2, 1, 1, "-100.2", 1, 3, "LOJA EXEMPLO 01/03"),
			l(5, 1, 19, "300.3", 0, 0, "ESTORNO LOJA EXEMPLO"), l(4, 1, 20, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 5}},
		{"o estorno escolhe a parcela de data mais antiga, não a de menor id", []Lancamento{
			l(2, 1, 10, "-100", 1, 3, "LOJA EXEMPLO 01/03"), l(3, 1, 0, "-100.2", 1, 3, "LOJA EXEMPLO 01/03"),
			l(4, 1, 20, "300.3", 0, 0, "ESTORNO LOJA EXEMPLO"), l(5, 1, 21, "300.6", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{3, 4}},
		{"parcelas do mesmo dia: o estorno escolhe a de menor id", []Lancamento{
			l(2, 1, 0, "-100.2", 1, 3, "LOJA EXEMPLO 01/03"), l(3, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"),
			l(4, 1, 20, "300.3", 0, 0, "ESTORNO LOJA EXEMPLO"), l(5, 1, 21, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{2, 3, 4, 5}},
		{"a 1ª parcela é arredondada em reais pelo método bancário: 100,50 e 99,50 batem", []Lancamento{
			l(1, 1, 0, "-100.50", 1, 2, "LOJA EXEMPLO 01/02"), l(2, 1, 30, "-99.50", 2, 2, "LOJA EXEMPLO 02/02"),
			l(4, 1, 40, "201", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 2, 4}},
		{"100,50 arredonda para 100 e 101,50 para 102: diferença 2, não entra", []Lancamento{
			l(1, 1, 0, "-100.50", 1, 2, "LOJA EXEMPLO 01/02"), l(2, 1, 30, "-101.50", 2, 2, "LOJA EXEMPLO 02/02"),
			l(4, 1, 40, "201", 0, 0, "ESTORNO LOJA EXEMPLO"),
		}, []int64{1, 4}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			if obtido := ids(ComprasEstornadas(c.lancamentos)); !slices.Equal(obtido, c.esperado) {
				t.Errorf("ficam fora %v, esperado %v", obtido, c.esperado)
			}
		})
	}
}

// Regra: a parcela que um estorno casou não casa de novo com outro estorno.
func TestEstornoNaoCasaDuasVezesAMesmaCompra(t *testing.T) {
	t.Parallel()
	fora := ComprasEstornadas([]Lancamento{
		l(1, 1, 0, "-100", 1, 3, "LOJA EXEMPLO 01/03"),
		l(4, 1, 20, "300", 0, 0, "ESTORNO LOJA EXEMPLO"), l(5, 1, 21, "300", 0, 0, "ESTORNO LOJA EXEMPLO"),
	})
	if !fora[1] || !fora[4] || fora[5] {
		t.Errorf("ficam fora %v: o 5 não tem compra para casar", ids(fora))
	}
}

// Regra: o prefixo de estorno sai do texto como o StartsWith(OrdinalIgnoreCase) do C#: sem caixa, só no começo, o
// primeiro da lista que casar, e o resto do texto vai como está (até com acento).
func TestSemEstorno(t *testing.T) {
	t.Parallel()
	casos := []struct{ entrada, esperado string }{
		{"ESTORNO DE LOJA", "LOJA"}, {"Estorno da Loja", "Loja"}, {"estorno do mercado", "mercado"}, {"Estorno loja", "loja"},
		{"  Estorno de loja  ", "loja"}, {"LOJA ESTORNO DE", "LOJA ESTORNO DE"}, {"ESTORNOS DE LOJA", "ESTORNOS DE LOJA"},
		{"ESTORNO DE ÁGUA", "ÁGUA"}, {"estorno de estorno de loja", "estorno de loja"}, {"ESTORNO DE", "DE"},
		{"ESTORNO ", "ESTORNO"}, {"", ""}, {"EStorno de ação", "ação"}, {"ESTORNO DE LOJA", "ESTORNO DE LOJA"},
	}
	for _, c := range casos {
		if obtido := semEstorno(c.entrada); obtido != c.esperado {
			t.Errorf("semEstorno(%q) = %q, esperado %q", c.entrada, obtido, c.esperado)
		}
	}
}
