// Pacote selic é o SelicService do C# (#82): a taxa Selic oficial do Banco Central, para comparar rentabilidades. A
// série 11 do SGS é a Selic diária (% ao dia), e a 432, a meta (% ao ano). Os dados ficam guardados no banco local, e só
// datas saem do computador.
package selic

import (
	"cmp"
	"slices"
	"sort"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Taxa é uma linha de TaxaSelicDiaria: a taxa em % ao dia.
type Taxa struct {
	Data tipos.Data
	Taxa tipos.Decimal
}

// Serie é a SerieSelic: os fatores acumulados da Selic diária.
//
// O acumulado é float64 de propósito, a exceção à regra de nunca usar float64 em valor: é um fator, e não dinheiro, e o
// C# o calcula em double. Em decimal, o número daria diferente na 15ª casa e mudaria os centavos da comparação com a
// Selic.
type Serie struct {
	datas     []tipos.Data
	acumulado []float64 // acumulado[i] é o produto de (1 + taxa) de todas as datas anteriores a datas[i]
}

// NovaSerie monta a série com as taxas, em qualquer ordem.
func NovaSerie(taxas []Taxa) *Serie {
	ordenadas := slices.Clone(taxas)
	slices.SortStableFunc(ordenadas, func(a, b Taxa) int { return cmp.Compare(a.Data.NumeroDoDia(), b.Data.NumeroDoDia()) })
	s := &Serie{acumulado: make([]float64, len(ordenadas)+1)}
	s.acumulado[0] = 1
	for i, t := range ordenadas {
		s.datas = append(s.datas, t.Data)
		s.acumulado[i+1] = s.acumulado[i] * (1 + t.Taxa.ParaFloat64()/100)
	}
	return s
}

// Vazia diz se a série não tem nenhuma taxa.
func (s *Serie) Vazia() bool { return len(s.datas) == 0 }

// UltimaData é a data da última taxa, ou nil na série vazia.
func (s *Serie) UltimaData() *tipos.Data {
	if s.Vazia() {
		return nil
	}
	return &s.datas[len(s.datas)-1]
}

// Fator é o fator de correção da Selic, com as taxas dos dias em [de, ate). Como o (decimal) do C#, guarda 15 dígitos;
// fator fora do decimal é o tipos.ErrEstouro.
func (s *Serie) Fator(de, ate tipos.Data) (tipos.Decimal, error) {
	if !ate.Depois(de) || s.Vazia() {
		return tipos.DecimalDeInteiro(1), nil
	}
	return tipos.DecimalDeFloat64(s.acumulado[s.indice(ate)] / s.acumulado[s.indice(de)])
}

// indice é a posição da data, ou a da primeira data depois dela.
func (s *Serie) indice(data tipos.Data) int {
	return sort.Search(len(s.datas), func(i int) bool { return !s.datas[i].Antes(data) })
}
