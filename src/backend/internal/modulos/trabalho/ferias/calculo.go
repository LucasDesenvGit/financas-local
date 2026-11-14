// Pacote ferias são as férias do módulo Trabalho (#213, ADR 0065): os períodos aquisitivos e concessivos, os dias de
// direito, o agendamento em até três frações, a venda de até 1/3 (abono pecuniário) e o cálculo de cada fração.
//
// É estimativa, não holerite. Não é receita: a receita é a transação das férias (#212).
package ferias

import (
	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// DiasDeDireito são os dias de férias pelas faltas sem justificativa no período aquisitivo (CLT, art. 130).
func DiasDeDireito(faltas int) int {
	switch {
	case faltas <= 5:
		return 30
	case faltas <= 14:
		return 24
	case faltas <= 23:
		return 18
	case faltas <= 32:
		return 12
	}
	return 0
}

// Periodo é um período aquisitivo (12 meses de trabalho) e o concessivo, os 12 meses seguintes, em que as férias têm de
// ser tiradas.
type Periodo struct {
	Inicio        tipos.Data `json:"inicio"`
	Fim           tipos.Data `json:"fim"`           // o último dia do aquisitivo
	FimConcessivo tipos.Data `json:"fimConcessivo"` // o último dia para tirar as férias dele
	Completo      bool       `json:"completo"`      // o aquisitivo já terminou (até hoje)
}

// Periodos são os períodos do emprego que já começaram até hoje (e antes da saída), do mais antigo para o mais novo.
func Periodos(admissao tipos.Data, saida *tipos.Data, hoje tipos.Data) []Periodo {
	var lista []Periodo
	for inicio := admissao; !inicio.Depois(hoje) && (saida == nil || !inicio.Depois(*saida)); inicio = inicio.SomarAnos(1) {
		fim := inicio.SomarAnos(1).SomarDias(-1)
		lista = append(lista, Periodo{Inicio: inicio, Fim: fim, FimConcessivo: fim.SomarAnos(1), Completo: fim.Antes(hoje)})
	}
	return lista
}

// MensagemFracoes é a regra das frações (CLT, art. 134, § 1º).
const MensagemFracoes = "As férias vão em até 3 frações: uma de 14 dias ou mais, e as outras de 5 dias ou mais."

// ValidarFracoes confere a fração nova junto com as que já existem no período: até 3; cada uma com 5 dias ou mais; uma
// de 14 ou mais (se ainda não há, o que sobra tem de caber nela); a venda até 1/3 do direito; e o total até o direito.
// Devolve a mensagem para a pessoa, ou "".
func ValidarFracoes(existentes []int, vendidosAntes, direito, dias, vendidos int) string {
	total, maior := 0, 0
	for _, d := range existentes {
		total += d
		maior = max(maior, d)
	}
	switch {
	case direito == 0:
		return "Com mais de 32 faltas no período, não há férias (CLT, art. 130)."
	case dias < 5 || len(existentes) >= 3:
		return MensagemFracoes
	case vendidos < 0 || vendidosAntes+vendidos > direito/3:
		return "Dá para vender até 1/3 dos dias de direito."
	case total+dias+vendidosAntes+vendidos > direito:
		return "Os dias passam do que sobra de férias neste período."
	}
	sobra := direito - total - dias - vendidosAntes - vendidos
	if max(maior, dias) < 14 && (sobra < 14 || len(existentes)+1 == 3) {
		return MensagemFracoes
	}
	return ""
}

// Entrada é o que o cálculo de uma fração precisa.
type Entrada struct {
	Inicio         tipos.Data
	Dias           int
	DiasVendidos   int
	SalarioBase    tipos.Decimal // o do mês do início
	SomaVariaveis  tipos.Decimal // horas extras, adicional noturno e DSR dos 12 meses antes
	Dependentes    int64
	AdiantarDecimo bool
	Inss           encargos.TabelaInss
	Irrf           encargos.TabelaIrrf
}

// Calculo é uma fração das férias calculada.
type Calculo struct {
	Inicio             tipos.Data    `json:"inicio"`
	Retorno            tipos.Data    `json:"retorno"`   // o dia de voltar ao trabalho
	Pagamento          tipos.Data    `json:"pagamento"` // até 2 dias antes do início (CLT, art. 145)
	Dias               int           `json:"dias"`
	DiasVendidos       int           `json:"diasVendidos"`
	Remuneracao        tipos.Decimal `json:"remuneracao"` // salário + média das variáveis, a base do mês
	MediaVariaveis     tipos.Decimal `json:"mediaVariaveis"`
	Ferias             tipos.Decimal `json:"ferias"`
	TercoFerias        tipos.Decimal `json:"tercoFerias"`
	Abono              tipos.Decimal `json:"abono"`
	TercoAbono         tipos.Decimal `json:"tercoAbono"`
	Inss               encargos.Inss `json:"inss"`
	Irrf               encargos.Irrf `json:"irrf"`
	AdiantamentoDecimo tipos.Decimal `json:"adiantamentoDecimo"` // a 1ª parcela do 13º, se vier junto
	Liquido            tipos.Decimal `json:"liquido"`            // o que cai na conta, com o 13º adiantado
}

var (
	trinta = decimal.NewFromInt(30)
	tres   = decimal.NewFromInt(3)
	doze   = decimal.NewFromInt(12)
)

// Calcular faz a conta de uma fração: férias + 1/3 com INSS e IRRF (separados do salário do mês); o abono pecuniário +
// 1/3 sem INSS nem IR (CLT, art. 144); e, se pedido, a 1ª parcela do 13º (metade da remuneração, sem desconto).
func Calcular(e Entrada) Calculo {
	media := e.SomaVariaveis.Decimal().Div(doze).Round(2)
	remuneracao := e.SalarioBase.Decimal().Add(media)
	ferias := remuneracao.Mul(decimal.NewFromInt(int64(e.Dias))).Div(trinta).Round(2)
	terco := ferias.Div(tres).Round(2)
	abono := remuneracao.Mul(decimal.NewFromInt(int64(e.DiasVendidos))).Div(trinta).Round(2)
	tercoAbono := abono.Div(tres).Round(2)
	tributavel := tipos.DeDecimal(ferias.Add(terco))
	c := Calculo{
		Inicio: e.Inicio, Retorno: e.Inicio.SomarDias(e.Dias), Pagamento: e.Inicio.SomarDias(-2), Dias: e.Dias,
		DiasVendidos: e.DiasVendidos, Remuneracao: tipos.DeDecimal(remuneracao.Round(2)), MediaVariaveis: tipos.DeDecimal(media),
		Ferias: tipos.DeDecimal(ferias), TercoFerias: tipos.DeDecimal(terco), Abono: tipos.DeDecimal(abono),
		TercoAbono: tipos.DeDecimal(tercoAbono), AdiantamentoDecimo: tipos.DeDecimal(decimal.Zero.Round(2)),
	}
	c.Inss = encargos.CalcularInss(e.Inss, tributavel)
	c.Irrf = encargos.CalcularIrrf(e.Irrf, tributavel, c.Inss.Valor, int(e.Dependentes), tipos.DeDecimal(decimal.Zero))
	liquido := ferias.Add(terco).Add(abono).Add(tercoAbono).Sub(c.Inss.Valor.Decimal()).Sub(c.Irrf.Valor.Decimal())
	if e.AdiantarDecimo {
		adiantamento := remuneracao.Div(decimal.NewFromInt(2)).Round(2)
		c.AdiantamentoDecimo = tipos.DeDecimal(adiantamento)
		liquido = liquido.Add(adiantamento)
	}
	c.Liquido = tipos.DeDecimal(liquido.Round(2))
	return c
}
