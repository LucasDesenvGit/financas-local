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
		if temAvo(ano, mes, admissao, saida) {
			avos++
		}
	}
	return avos
}

// MesesComAvo é a faixa [inicio, fim) dos meses do ano que contam avo. A média das variáveis soma só os holerites dela
// (#750, ADR 0065): o mês da admissão ou da saída com menos de 15 dias não conta para o 13º e não pesa na média dele.
// Só esses dois meses podem ficar de fora (os do meio são inteiros), então a faixa é contínua; sem avo, inicio == fim.
func MesesComAvo(ano int, admissao tipos.Data, saida *tipos.Data) (inicio, fim tipos.Data) {
	primeiro, ultimo := 0, 0
	for mes := 1; mes <= 12; mes++ {
		if temAvo(ano, mes, admissao, saida) {
			if primeiro == 0 {
				primeiro = mes
			}
			ultimo = mes
		}
	}
	if primeiro == 0 {
		vazio := tipos.NovaData(ano, 1, 1)
		return vazio, vazio
	}
	return tipos.NovaData(ano, time.Month(primeiro), 1), tipos.NovaData(ano, time.Month(ultimo), 1).SomarMeses(1)
}

// temAvo: no mês, a pessoa trabalhou 15 dias ou mais (de calendário), entre a admissão e a saída.
func temAvo(ano, mes int, admissao tipos.Data, saida *tipos.Data) bool {
	de := tipos.NovaData(ano, time.Month(mes), 1)
	ate := de.SomarMeses(1).SomarDias(-1)
	if admissao.Depois(de) {
		de = admissao
	}
	if saida != nil && saida.Antes(ate) {
		ate = *saida
	}
	return !ate.Antes(de) && ate.NumeroDoDia()-de.NumeroDoDia()+1 >= 15
}

// Entrada é o que o cálculo precisa.
type Entrada struct {
	Ano           int
	Avos          int
	SalarioBase   tipos.Decimal // o de dezembro, ou o do mês da saída
	SomaVariaveis tipos.Decimal // horas extras, adicional noturno e DSR dos holerites do ano, pelo salário-hora de agora
	// MesesComHolerite: quantos meses do ano têm holerite calculado; os outros entram como zero na média.
	MesesComHolerite int
	Dependentes      int64
	PrimeiraEm       *tipos.Data    // informada: quando veio a 1ª parcela
	PrimeiraValor    *tipos.Decimal // informada: quanto veio
	// PrimeiraComFerias: a 1ª veio com as férias do ano (a data é a do pagamento delas).
	PrimeiraComFerias bool
	// ValorDasFerias: o que as férias pagaram de 1ª parcela (revisão do #701); nil, a metade do 13º.
	ValorDasFerias *tipos.Decimal
	Inss           encargos.TabelaInss
	Irrf           encargos.TabelaIrrf
}

// Parcela é uma parcela do 13º.
type Parcela struct {
	Valor     tipos.Decimal `json:"valor"`
	Data      tipos.Data    `json:"data"`
	Informada bool          `json:"informada"` // a pessoa disse quanto e quando veio
	// ValorInformado: o valor veio da pessoa (e não da metade calculada); a tela não grava o calculado como informado.
	ValorInformado bool `json:"valorInformado"`
	// DataInformada: a data veio da pessoa (e não o 30/11 padrão nem a das férias); a tela não grava a padrão como
	// informada ao salvar só o valor (#750).
	DataInformada bool `json:"dataInformada"`
	// ComFerias: a 1ª veio com as férias (a data é a do pagamento delas, e o valor, o que elas pagaram).
	ComFerias bool `json:"comFerias"`
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
	Segunda        Parcela       `json:"segunda"` // o líquido da 2ª, nunca negativo
	// Excedente: a 1ª parcela informada passou do 13º líquido; a 2ª fica zero e a empresa desconta a diferença.
	Excedente tipos.Decimal `json:"excedente"`
	Liquido   tipos.Decimal `json:"liquido"` // o 13º líquido: total − INSS − IRRF
	// MesesComHolerite: os meses do ano com holerite calculado; menos que os avos, a média das horas extras sai menor.
	MesesComHolerite int `json:"mesesComHolerite"`
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
	c := Calculo{Ano: e.Ano, Avos: e.Avos, SalarioBase: e.SalarioBase, MediaVariaveis: tipos.DeDecimal(media), Total: tipos.DeDecimal(total),
		MesesComHolerite: e.MesesComHolerite}

	c.Primeira = Parcela{Valor: centavos(total.Div(decimal.NewFromInt(2))), Data: tipos.NovaData(e.Ano, 11, 30)}
	if e.ValorDasFerias != nil {
		c.Primeira.Valor = centavos(e.ValorDasFerias.Decimal())
	}
	if e.PrimeiraValor != nil {
		c.Primeira.Valor, c.Primeira.Informada, c.Primeira.ValorInformado = centavos(e.PrimeiraValor.Decimal()), true, true
	}
	if e.PrimeiraEm != nil {
		c.Primeira.Data, c.Primeira.Informada, c.Primeira.ComFerias = *e.PrimeiraEm, true, e.PrimeiraComFerias
		c.Primeira.DataInformada = !e.PrimeiraComFerias
	}

	// O IR do 13º é exclusivo na fonte, separado do salário de dezembro: base = total - INSS - dependentes, sem o
	// desconto simplificado (inferido, ADR 0065). A redução da Lei 15.270/2025 também vale para ele (art. 3º-A, § 3º).
	semSimplificado := e.Irrf
	semSimplificado.DescontoSimplificado = tipos.DeDecimal(decimal.Zero)
	c.Inss = encargos.CalcularInss(e.Inss, c.Total)
	c.Irrf = encargos.CalcularIrrf(semSimplificado, c.Total, c.Inss.Valor, int(e.Dependentes), tipos.DeDecimal(decimal.Zero))
	liquido := total.Sub(c.Inss.Valor.Decimal()).Sub(c.Irrf.Valor.Decimal())
	segunda := liquido.Sub(c.Primeira.Valor.Decimal())
	c.Segunda = Parcela{Valor: centavos(decimal.Max(segunda, decimal.Zero)), Data: tipos.NovaData(e.Ano, 12, 20)}
	c.Excedente = centavos(decimal.Max(segunda.Neg(), decimal.Zero))
	c.Liquido = centavos(liquido)
	return c
}
