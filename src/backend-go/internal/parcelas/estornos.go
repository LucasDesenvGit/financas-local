package parcelas

import (
	"github.com/shopspring/decimal"
	"slices"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// DiasEstornoDaCompra é até quantos dias depois da compra um estorno ainda é casado com ela.
const DiasEstornoDaCompra = 400

// Lancamento é um lançamento no cartão para casar estorno com compra parcelada (#257), como o
// ParcelasFuturas.Lancamento do C#. Numero e Total são nulos fora de compra parcelada.
type Lancamento struct {
	Id            int64
	ContaId       int64
	Data          tipos.Data
	Valor         tipos.Decimal
	Numero, Total *int64
	Descricao     string
}

// ComprasEstornadas é o ParcelasFuturas.ComprasEstornadas do C# (#257, ADR 0036): estorno que devolve uma compra
// parcelada inteira (o valor da parcela × o número de parcelas) casa com a 1ª parcela dessa compra; a compra foi
// cancelada. Devolve os ids que ficam fora dos relatórios: o estorno, a 1ª parcela e as outras parcelas dela já
// lançadas (até 35 dias depois do estorno, para as que o banco antecipa).
//
// Com várias compras iguais (mesma conta, valor e descrição), cada estorno fica com a mais antiga ainda não casada.
// Estorno de parte da compra (uma parcela, um item) não casa.
func ComprasEstornadas(lancamentos []Lancamento) map[int64]bool {
	var parcelas, estornos []Lancamento
	for _, l := range lancamentos {
		switch {
		case l.Valor.Decimal().IsPositive():
			estornos = append(estornos, l)
		case l.Valor.Decimal().IsNegative() && parcelada(l):
			parcelas = append(parcelas, l)
		}
	}
	// OrderBy(Data).ThenBy(Id) do LINQ: o Id desempata, então a ordem é total e não importa a de entrada.
	ordenar := func(a, b Lancamento) int {
		if c := a.Data.NumeroDoDia() - b.Data.NumeroDoDia(); c != 0 {
			return c
		}
		return int(a.Id - b.Id)
	}
	slices.SortStableFunc(estornos, ordenar)
	slices.SortStableFunc(parcelas, ordenar)

	fora := map[int64]bool{}
	for _, e := range estornos {
		compra := Compra(semEstorno(e.Descricao))
		primeira := primeiraDe(parcelas, func(p Lancamento) bool {
			total := tipos.DecimalDeInteiro(*p.Total)
			folga := tipos.Somar(tipos.Multiplicar(total, folgaPorParcela), folgaFixa)
			diferenca := tipos.Subtrair(e.Valor, tipos.Multiplicar(valorAbsoluto(p.Valor), total))
			return *p.Numero == 1 && p.ContaId == e.ContaId && !fora[p.Id] && p.Data.NumeroDoDia() <= e.Data.NumeroDoDia() &&
				e.Data.NumeroDoDia()-p.Data.NumeroDoDia() <= DiasEstornoDaCompra && Compra(p.Descricao) == compra &&
				valorAbsoluto(diferenca).Decimal().Cmp(folga.Decimal()) <= 0
		})
		if primeira == nil {
			continue
		}
		fora[e.Id] = true
		fora[primeira.Id] = true
		reais := tipos.Arredondar(valorAbsoluto(primeira.Valor), 0)
		limite := e.Data.SomarDias(35)
		for n := int64(2); n <= *primeira.Total; n++ {
			outra := primeiraDe(parcelas, func(p Lancamento) bool {
				diferenca := tipos.Subtrair(tipos.Arredondar(valorAbsoluto(p.Valor), 0), reais)
				return *p.Numero == n && *p.Total == *primeira.Total && p.ContaId == primeira.ContaId && !fora[p.Id] &&
					p.Data.NumeroDoDia() >= primeira.Data.NumeroDoDia() && p.Data.NumeroDoDia() <= limite.NumeroDoDia() &&
					valorAbsoluto(diferenca).Decimal().Cmp(tipos.DecimalDeInteiro(1).Decimal()) <= 0 && Compra(p.Descricao) == compra
			})
			if outra != nil {
				fora[outra.Id] = true
			}
		}
	}
	return fora
}

// parcelada: Total > 1 e 1 <= Numero <= Total, com a comparação do C# sobre short?, que dá falso para nulo.
func parcelada(l Lancamento) bool {
	return l.Total != nil && l.Numero != nil && *l.Total > 1 && *l.Numero >= 1 && *l.Numero <= *l.Total
}

// primeiraDe é o Where(...).FirstOrDefault() sobre a lista já em ordem de data e id.
func primeiraDe(lista []Lancamento, serve func(Lancamento) bool) *Lancamento {
	for i := range lista {
		if serve(lista[i]) {
			return &lista[i]
		}
	}
	return nil
}

func valorAbsoluto(d tipos.Decimal) tipos.Decimal { return tipos.DeDecimal(d.Decimal().Abs()) }

// folgaPorParcela (0,10) e folgaFixa (0,05): o estorno pode diferir do valor da parcela × o total em até
// total × 0,10 + 0,05 (a 1ª parcela vem arredondada pelo banco).
var (
	folgaPorParcela = tipos.DeDecimal(decimal.New(10, -2))
	folgaFixa       = tipos.DeDecimal(decimal.New(5, -2))
)

// prefixosDeEstorno, nesta ordem: o primeiro que casar é o que sai.
var prefixosDeEstorno = []string{"ESTORNO DE ", "ESTORNO DA ", "ESTORNO DO ", "ESTORNO "}

// semEstorno é o ParcelasFuturas.SemEstorno do C#: a descrição do estorno sem "Estorno de" na frente (alguns bancos
// repetem a da compra assim). O Trim e o StartsWith(OrdinalIgnoreCase) do C# valem aqui por TrimFunc(IsSpace) e pelo
// mesmo ToUpperInvariant do restante do projeto.
func semEstorno(descricao string) string {
	t := strings.TrimFunc(descricao, unicode.IsSpace)
	maiusculas := texto.MaiusculasInvariantes(t)
	for _, prefixo := range prefixosDeEstorno {
		if strings.HasPrefix(maiusculas, prefixo) {
			return string([]rune(t)[len([]rune(prefixo)):]) // o prefixo é ASCII, e cada rune de t virou uma rune em maiusculas
		}
	}
	return t
}
