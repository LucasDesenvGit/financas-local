package cambio

import (
	"slices"
	"sort"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// diasValidadeHistorico: quantos dias para trás uma cotação histórica ainda vale (fim de semana + feriado).
const diasValidadeHistorico = 7

// Conversor é o ConversorMoeda: converte para reais com as cotações guardadas, sem consultar a internet. Usa a taxa
// de compra da PTAX, a que vale para avaliar um bem em moeda estrangeira. A moeda sem cotação fica anotada em
// MoedasSemCotacao, para quem somou avisar o usuário. Como no .NET, não é para uso em paralelo.
type Conversor struct {
	taxas      map[string]tipos.Decimal
	historico  map[string][]Guardada // por moeda, em ordem de data
	semCotacao map[string]struct{}
}

// NovoConversor monta o conversor com a última cotação de cada moeda e, opcionalmente, o histórico diário.
func NovoConversor(ultimas, historico []Guardada) *Conversor {
	c := &Conversor{taxas: map[string]tipos.Decimal{}, historico: map[string][]Guardada{}, semCotacao: map[string]struct{}{}}
	for _, u := range ultimas {
		c.taxas[u.Moeda] = u.Compra
	}
	for _, h := range historico {
		c.historico[h.Moeda] = append(c.historico[h.Moeda], h)
	}
	for moeda := range c.historico {
		serie := c.historico[moeda]
		slices.SortStableFunc(serie, func(a, b Guardada) int {
			switch {
			case a.Data.Antes(b.Data):
				return -1
			case a.Data.Depois(b.Data):
				return 1
			}
			return 0
		})
	}
	return c
}

// MoedasSemCotacao são as moedas que apareceram numa conversão sem cotação, em ordem (SortedSet ordinal do .NET).
func (c *Conversor) MoedasSemCotacao() []string {
	lista := make([]string, 0, len(c.semCotacao))
	for m := range c.semCotacao {
		lista = append(lista, m)
	}
	slices.Sort(lista)
	return lista
}

// Taxa são os reais por 1 unidade da moeda (última cotação), ou nil sem cotação guardada. Código inválido conta
// como real, como no .NET.
func (c *Conversor) Taxa(moeda string) *tipos.Decimal {
	codigo := normalizarOuReal(moeda)
	if codigo == MoedaBase {
		um := tipos.DecimalDeInteiro(1)
		return &um
	}
	if taxa, ok := c.taxas[codigo]; ok {
		return &taxa
	}
	c.semCotacao[codigo] = struct{}{}
	return nil
}

// ParaReais é o valor em reais pela última cotação (Math.Round(valor * taxa, 2)), ou nil sem cotação.
func (c *Conversor) ParaReais(valor tipos.Decimal, moeda string) *tipos.Decimal {
	taxa := c.Taxa(moeda)
	if taxa == nil {
		return nil
	}
	reais := tipos.Arredondar(tipos.Multiplicar(valor, *taxa), 2)
	return &reais
}

// ParaReaisNaData é o valor em reais pela cotação da data (o último dia útil até 7 dias antes). Sem histórico para
// a data, usa a última cotação guardada. Em real, o valor volta como está (sem arredondar), como no .NET.
func (c *Conversor) ParaReaisNaData(valor tipos.Decimal, moeda string, data tipos.Data) *tipos.Decimal {
	codigo := normalizarOuReal(moeda)
	if codigo == MoedaBase {
		return &valor
	}
	if serie, ok := c.historico[codigo]; ok {
		if taxa := taxaAte(serie, data); taxa != nil {
			reais := tipos.Arredondar(tipos.Multiplicar(valor, *taxa), 2)
			return &reais
		}
	}
	return c.ParaReais(valor, codigo)
}

// taxaAte é a taxa do dia útil mais recente até a data (busca binária), dentro da validade.
func taxaAte(serie []Guardada, data tipos.Data) *tipos.Decimal {
	// O primeiro índice com Data depois da data procurada; o anterior é o achado.
	i := sort.Search(len(serie), func(i int) bool { return serie[i].Data.Depois(data) })
	if i == 0 {
		return nil
	}
	achado := serie[i-1]
	if achado.Data.Antes(data.SomarDias(-diasValidadeHistorico)) {
		return nil
	}
	return &achado.Compra
}

func normalizarOuReal(moeda string) string {
	if codigo, ok := NormalizarMoeda(moeda); ok {
		return codigo
	}
	return MoedaBase
}
