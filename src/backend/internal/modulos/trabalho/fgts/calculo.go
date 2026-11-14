// Pacote fgts é o FGTS do módulo Trabalho (#216, ADR 0066): o saldo estimado mês a mês (os depósitos de 8%,
// corrigidos a 3% ao ano), conferido com o saldo que a pessoa vê no app do FGTS, e os saques. Fora do patrimônio e dos
// relatórios.
package fgts

import (
	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Tipos de lançamento.
const (
	TipoConferencia  int64 = 1 // o saldo visto no app do FGTS
	TipoSaque        int64 = 2
	TipoDistribuicao int64 = 3 // a distribuição de resultados da Caixa
)

// Motivos do saque.
const (
	MotivoRescisao         int64 = 1
	MotivoSaqueAniversario int64 = 2
	MotivoImovel           int64 = 3
	MotivoOutro            int64 = 4
)

// TaxaMensal é 3% ao ano em juros compostos: (1,03)^(1/12) - 1. A TR fica de fora (ADR 0066).
var TaxaMensal = decimal.RequireFromString("0.0024663")

// Lancamento é um lançamento do FGTS.
type Lancamento struct {
	Id     int64         `json:"id"`
	Data   tipos.Data    `json:"data"`
	Tipo   int64         `json:"tipo"`
	Valor  tipos.Decimal `json:"valor"`
	Motivo *int64        `json:"motivo"`
}

// Mes é um mês da estimativa.
type Mes struct {
	Mes          tipos.Data    `json:"mes"`
	Correcao     tipos.Decimal `json:"correcao"`
	Deposito     tipos.Decimal `json:"deposito"`
	Distribuicao tipos.Decimal `json:"distribuicao"`
	Saques       tipos.Decimal `json:"saques"`
	Saldo        tipos.Decimal `json:"saldo"`
	Conferido    bool          `json:"conferido"` // o saldo do mês é o do app
}

// Conferencia é um saldo do app comparado com o que o site estimava.
type Conferencia struct {
	Data      tipos.Data    `json:"data"`
	Informado tipos.Decimal `json:"informado"`
	Estimado  tipos.Decimal `json:"estimado"`
	Diferenca tipos.Decimal `json:"diferenca"` // informado - estimado
}

// Estimativa é o FGTS até o mês.
type Estimativa struct {
	Saldo        tipos.Decimal `json:"saldo"`
	Meses        []Mes         `json:"meses"` // do mais antigo para o mais novo
	Conferencias []Conferencia `json:"conferencias"`
}

func centavos(v decimal.Decimal) tipos.Decimal { return tipos.DeDecimal(v.Round(2)) }

// Estimar monta o saldo mês a mês, de inicio até ate (os dois no dia 1). depositos[mes.String()] é o depósito de cada
// mês. A ordem de cada mês: a correção do saldo de antes, o depósito, a distribuição e os saques do mês; se houve
// conferência no mês, o saldo passa a ser o do app (a última conferência do mês), e o saque ou a distribuição com data
// depois dela entram depois.
func Estimar(inicio, ate tipos.Data, depositos map[string]decimal.Decimal, lancamentos []Lancamento) Estimativa {
	r := Estimativa{Meses: []Mes{}, Conferencias: []Conferencia{}}
	saldo := decimal.Zero
	for mes := inicio; !mes.Depois(ate); mes = mes.SomarMeses(1) {
		fim := mes.SomarMeses(1)
		correcao := saldo.Mul(TaxaMensal).Round(2)
		deposito := depositos[mes.String()].Round(2)
		var conferencia *Lancamento
		for i, l := range lancamentos {
			if l.Tipo == TipoConferencia && !l.Data.Antes(mes) && l.Data.Antes(fim) {
				conferencia = &lancamentos[i] // a última do mês (a lista vem por data)
			}
		}
		// O saque e a distribuição entram no saldo do mês; os depois da conferência, depois dela.
		distribuicao, saques := decimal.Zero, decimal.Zero
		antes, depois := decimal.Zero, decimal.Zero
		for _, l := range lancamentos {
			if l.Data.Antes(mes) || !l.Data.Antes(fim) || l.Tipo == TipoConferencia {
				continue
			}
			valor := l.Valor.Decimal()
			if l.Tipo == TipoSaque {
				saques, valor = saques.Add(valor), valor.Neg()
			} else {
				distribuicao = distribuicao.Add(valor)
			}
			if conferencia != nil && l.Data.Depois(conferencia.Data) {
				depois = depois.Add(valor)
			} else {
				antes = antes.Add(valor)
			}
		}
		saldo = saldo.Add(correcao).Add(deposito).Add(antes)
		m := Mes{Mes: mes, Correcao: tipos.DeDecimal(correcao), Deposito: tipos.DeDecimal(deposito),
			Distribuicao: centavos(distribuicao), Saques: centavos(saques)}
		if conferencia != nil {
			informado := conferencia.Valor.Decimal()
			r.Conferencias = append(r.Conferencias, Conferencia{Data: conferencia.Data, Informado: centavos(informado),
				Estimado: centavos(saldo), Diferenca: centavos(informado.Sub(saldo))})
			saldo, m.Conferido = informado, true
		}
		saldo = saldo.Add(depois)
		m.Saldo = centavos(saldo)
		r.Meses = append(r.Meses, m)
	}
	r.Saldo = centavos(saldo)
	return r
}
