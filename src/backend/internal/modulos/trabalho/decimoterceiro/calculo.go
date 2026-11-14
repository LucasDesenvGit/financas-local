// Pacote decimoterceiro é o 13º salário do módulo Trabalho (#214, ADR 0065): quanto, quando e quanto já foi
// adiantado. Proporcional aos meses do ano (15 dias ou mais contam o mês), com a média das horas extras dos holerites.
// A 1ª parcela (até 30/11, ou nas férias) é metade, sem desconto; a 2ª (até 20/12) leva o INSS e o IRRF sobre o total,
// menos a 1ª.
//
// É estimativa, não holerite. Não é receita: a receita é a transação de cada parcela (#212).
package decimoterceiro

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Avos são os meses do ano que contam para o 13º: o mês em que a pessoa trabalhou 15 dias ou mais (de calendário).
func Avos(ano int, admissao tipos.Data, saida *tipos.Data) int {
	avos := 0
	for mes := 1; mes <= 12; mes++ {
		inicio := tipos.NovaData(ano, time.Month(mes), 1)
		fim := inicio.SomarMeses(1).SomarDias(-1)
		de, ate := inicio, fim
		if admissao.Depois(de) {
			de = admissao
		}
		if saida != nil && saida.Antes(ate) {
			ate = *saida
		}
		if ate.Antes(de) {
			continue
		}
		if ate.NumeroDoDia()-de.NumeroDoDia()+1 >= 15 {
			avos++
		}
	}
	return avos
}

// Entrada é o que o cálculo precisa.
type Entrada struct {
	Ano           int
	Avos          int
	SalarioBase   tipos.Decimal // o de dezembro, ou o do mês da saída
	SomaVariaveis tipos.Decimal // horas extras, adicional noturno e DSR dos holerites do ano
	Dependentes   int64
	PrimeiraEm    *tipos.Data    // informada: quando veio a 1ª parcela
	PrimeiraValor *tipos.Decimal // informada: quanto veio
	Inss          encargos.TabelaInss
	Irrf          encargos.TabelaIrrf
}

// Parcela é uma parcela do 13º.
type Parcela struct {
	Valor     tipos.Decimal `json:"valor"`
	Data      tipos.Data    `json:"data"`
	Informada bool          `json:"informada"` // a pessoa disse quanto e quando veio
}

// Calculo é o 13º do ano.
type Calculo struct {
	Ano            int           `json:"ano"`
	Avos           int           `json:"avos"`
	SalarioBase    tipos.Decimal `json:"salarioBase"`
	MediaVariaveis tipos.Decimal `json:"mediaVariaveis"`
	Total          tipos.Decimal `json:"total"`
	Primeira       Parcela       `json:"primeira"`
	Inss           encargos.Inss `json:"inss"`
	Irrf           encargos.Irrf `json:"irrf"`
	Segunda        Parcela       `json:"segunda"` // o líquido da 2ª
	Liquido        tipos.Decimal `json:"liquido"` // 1ª + 2ª
}

var doze = decimal.NewFromInt(12)

func centavos(v decimal.Decimal) tipos.Decimal { return tipos.DeDecimal(v.Round(2)) }

// Calcular monta o 13º do ano.
func Calcular(e Entrada) Calculo {
	avos := decimal.NewFromInt(int64(e.Avos))
	media := decimal.Zero
	if e.Avos > 0 {
		media = e.SomaVariaveis.Decimal().Div(avos).Round(2)
	}
	total := e.SalarioBase.Decimal().Add(media).Mul(avos).Div(doze).Round(2)
	c := Calculo{Ano: e.Ano, Avos: e.Avos, SalarioBase: e.SalarioBase, MediaVariaveis: tipos.DeDecimal(media), Total: tipos.DeDecimal(total)}

	c.Primeira = Parcela{Valor: centavos(total.Div(decimal.NewFromInt(2))), Data: tipos.NovaData(e.Ano, 11, 30)}
	if e.PrimeiraValor != nil {
		c.Primeira.Valor, c.Primeira.Informada = centavos(e.PrimeiraValor.Decimal()), true
	}
	if e.PrimeiraEm != nil {
		c.Primeira.Data, c.Primeira.Informada = *e.PrimeiraEm, true
	}

	// O IR do 13º é exclusivo na fonte, separado do salário de dezembro: base = total - INSS - dependentes, sem o
	// desconto simplificado (inferido, ADR 0065). A redução da Lei 15.270/2025 também vale para ele (art. 3º-A, § 3º).
	semSimplificado := e.Irrf
	semSimplificado.DescontoSimplificado = tipos.DeDecimal(decimal.Zero)
	c.Inss = encargos.CalcularInss(e.Inss, c.Total)
	c.Irrf = encargos.CalcularIrrf(semSimplificado, c.Total, c.Inss.Valor, int(e.Dependentes), tipos.DeDecimal(decimal.Zero))
	segunda := total.Sub(c.Inss.Valor.Decimal()).Sub(c.Irrf.Valor.Decimal()).Sub(c.Primeira.Valor.Decimal())
	c.Segunda = Parcela{Valor: centavos(segunda), Data: tipos.NovaData(e.Ano, 12, 20)}
	c.Liquido = centavos(c.Primeira.Valor.Decimal().Add(segunda))
	return c
}
