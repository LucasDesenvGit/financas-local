// Pacote holerite é o holerite do mês do módulo Trabalho (#211, ADR 0065): o salário do mês (proporcional na admissão
// e na saída), as horas extras com o DSR, o adicional noturno, os outros proventos, o INSS, o IRRF, os descontos e o
// líquido. Aberto, é calculado na hora; fechado, fica congelado com o resultado guardado.
//
// É estimativa, não holerite: convenção coletiva, benefícios e arredondamentos da empresa mudam o valor. Não é receita:
// nada daqui entra nos relatórios.
package holerite

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Tipos de lançamento: até 9 são proventos; de 10 em diante, descontos.
const (
	HoraExtra         int64 = 1
	AdicionalNoturno  int64 = 2
	OutroProvento     int64 = 3
	ValeTransporte    int64 = 10
	ValeAlimentacao   int64 = 11
	PlanoDeSaude      int64 = 12
	PensaoAlimenticia int64 = 13
	OutroDesconto     int64 = 14
)

// TipoDSR é a linha calculada do DSR sobre as horas extras e o adicional noturno (não é um lançamento da pessoa).
const TipoDSR int64 = 4

// TipoSalario é a linha do salário do mês.
const TipoSalario int64 = 0

// TipoInss e TipoIrrf são as linhas dos descontos calculados.
const (
	TipoInss int64 = 20
	TipoIrrf int64 = 21
)

var nomes = map[int64]string{
	HoraExtra: "Horas extras", AdicionalNoturno: "Adicional noturno", OutroProvento: "Outro provento",
	ValeTransporte: "Vale-transporte", ValeAlimentacao: "VR/VA", PlanoDeSaude: "Plano de saúde",
	PensaoAlimenticia: "Pensão alimentícia", OutroDesconto: "Outro desconto",
}

// TipoConhecido diz se o tipo é um dos lançamentos que a pessoa pode pôr.
func TipoConhecido(t int64) bool { _, ok := nomes[t]; return ok }

// EhPorHora: hora extra e adicional noturno vão por quantidade de horas e percentual; os outros, por valor.
func EhPorHora(t int64) bool { return t == HoraExtra || t == AdicionalNoturno }

// Lancamento é uma linha que a pessoa pôs no holerite.
type Lancamento struct {
	Tipo       int64          `json:"tipo"`
	Descricao  string         `json:"descricao"`
	Quantidade *tipos.Decimal `json:"quantidade"` // horas (hora extra, adicional noturno)
	Percentual *tipos.Decimal `json:"percentual"` // 50 = 50% (hora extra), 20 (adicional noturno)
	Valor      *tipos.Decimal `json:"valor"`      // os outros; no vale-transporte, o custo do mês
	IncideInss bool           `json:"incideInss"`
	IncideIrrf bool           `json:"incideIrrf"`
	DeduzIrrf  bool           `json:"deduzIrrf"`
	Recorrente bool           `json:"recorrente"`
}

// Entrada é o que o cálculo precisa do mês.
type Entrada struct {
	Mes               tipos.Data
	Admissao          tipos.Data
	Saida             *tipos.Data
	SalarioContratual tipos.Decimal
	JornadaHoras      int64
	Dependentes       int64
	SemEncargos       bool // estágio ou PJ
	DiasUteis         int64
	DiasDescanso      int64
	Lancamentos       []Lancamento
	Inss              encargos.TabelaInss
	Irrf              encargos.TabelaIrrf
}

// Linha é uma linha do holerite calculado: o que é, a referência ("10 h a 50%") e o valor.
type Linha struct {
	Tipo       int64         `json:"tipo"`
	Descricao  string        `json:"descricao"`
	Referencia string        `json:"referencia"`
	Valor      tipos.Decimal `json:"valor"`
}

// Calculo é o holerite calculado. Fechado, é isto que fica guardado.
type Calculo struct {
	Mes               tipos.Data     `json:"mes"`
	SalarioContratual tipos.Decimal  `json:"salarioContratual"`
	DiasTrabalhados   int            `json:"diasTrabalhados"` // de 30 (o mês comercial)
	ValorHora         tipos.Decimal  `json:"valorHora"`
	Proventos         []Linha        `json:"proventos"`
	Descontos         []Linha        `json:"descontos"`
	TotalProventos    tipos.Decimal  `json:"totalProventos"`
	TotalDescontos    tipos.Decimal  `json:"totalDescontos"`
	Liquido           tipos.Decimal  `json:"liquido"`
	BaseInss          tipos.Decimal  `json:"baseInss"`
	Inss              *encargos.Inss `json:"inss"`
	Irrf              *encargos.Irrf `json:"irrf"`
	FgtsDoMes         tipos.Decimal  `json:"fgtsDoMes"` // 8% da base, depositado pela empresa: não sai do salário
	SemEncargos       bool           `json:"semEncargos"`
}

var (
	cem      = decimal.NewFromInt(100)
	trinta   = decimal.NewFromInt(30)
	seisPct  = decimal.RequireFromString("0.06")
	oitoPct  = decimal.RequireFromString("0.08")
	vinteDef = decimal.NewFromInt(20)
)

func centavos(v decimal.Decimal) tipos.Decimal { return tipos.DeDecimal(v.Round(2)) }

func dec(p *tipos.Decimal) decimal.Decimal {
	if p == nil {
		return decimal.Zero
	}
	return p.Decimal()
}

// DiasTrabalhados é quantos dias do mês comercial (30) o emprego cobre: proporcional no mês da admissão e no da saída.
func DiasTrabalhados(mes, admissao tipos.Data, saida *tipos.Data) int {
	fimDoMes := mes.SomarMeses(1).SomarDias(-1)
	if admissao.Depois(fimDoMes) || (saida != nil && saida.Antes(mes)) {
		return 0
	}
	inicio, fim := 1, 30
	if !admissao.Antes(mes) {
		inicio = min(admissao.Dia(), 30)
	}
	if saida != nil && !saida.Depois(fimDoMes) {
		fim = min(saida.Dia(), 30)
		if saida.Dia() == fimDoMes.Dia() { // saiu no último dia: o mês inteiro, mesmo em fevereiro
			fim = 30
		}
	}
	return max(0, fim-inicio+1)
}

// DiasDoDsr são os dias úteis (segunda a sábado) e os de descanso (domingos) do mês, o padrão do DSR. Os feriados a
// pessoa acerta no holerite.
func DiasDoDsr(mes tipos.Data) (uteis, descanso int64) {
	fim := mes.SomarMeses(1)
	for d := mes; d.Antes(fim); d = d.SomarDias(1) {
		if time.Date(d.Ano(), d.Mes(), d.Dia(), 12, 0, 0, 0, time.UTC).Weekday() == time.Sunday {
			descanso++
		} else {
			uteis++
		}
	}
	return uteis, descanso
}

func descricao(l Lancamento) string {
	if l.Descricao != "" {
		return l.Descricao
	}
	return nomes[l.Tipo]
}

// numero é o número com vírgula, como a tela mostra ("10,5").
func numero(v decimal.Decimal) string { return strings.ReplaceAll(v.String(), ".", ",") }

func horas(v decimal.Decimal) string { return numero(v) + " h" }

// Calcular monta o holerite do mês.
func Calcular(e Entrada) Calculo {
	c := Calculo{Mes: e.Mes, SalarioContratual: e.SalarioContratual, SemEncargos: e.SemEncargos,
		Proventos: []Linha{}, Descontos: []Linha{}}
	contratual := e.SalarioContratual.Decimal()
	c.DiasTrabalhados = DiasTrabalhados(e.Mes, e.Admissao, e.Saida)
	salarioMes := contratual.Mul(decimal.NewFromInt(int64(c.DiasTrabalhados))).Div(trinta).Round(2)
	valorHora := decimal.Zero
	if e.JornadaHoras > 0 {
		valorHora = contratual.Div(decimal.NewFromInt(e.JornadaHoras))
	}
	c.ValorHora = centavos(valorHora)
	ref := "30 dias"
	if c.DiasTrabalhados < 30 {
		ref = fmt.Sprintf("%d de 30 dias", c.DiasTrabalhados)
	}
	c.Proventos = append(c.Proventos, Linha{Tipo: TipoSalario, Descricao: "Salário", Referencia: ref, Valor: centavos(salarioMes)})

	baseInss, rendIrrf, variaveis := salarioMes, salarioMes, decimal.Zero
	proventos := salarioMes
	for _, l := range e.Lancamentos {
		var valor decimal.Decimal
		var referencia string
		switch l.Tipo {
		case HoraExtra:
			pct := dec(l.Percentual)
			valor = dec(l.Quantidade).Mul(valorHora).Mul(cem.Add(pct)).Div(cem).Round(2)
			referencia = horas(dec(l.Quantidade)) + " a " + numero(pct) + "%"
			variaveis = variaveis.Add(valor)
		case AdicionalNoturno:
			pct := dec(l.Percentual)
			if l.Percentual == nil {
				pct = vinteDef
			}
			valor = dec(l.Quantidade).Mul(valorHora).Mul(pct).Div(cem).Round(2)
			referencia = horas(dec(l.Quantidade)) + " a " + numero(pct) + "%"
			variaveis = variaveis.Add(valor)
		case OutroProvento:
			valor = dec(l.Valor).Round(2)
		default:
			continue // os descontos vêm depois
		}
		proventos = proventos.Add(valor)
		c.Proventos = append(c.Proventos, Linha{Tipo: l.Tipo, Descricao: descricao(l), Referencia: referencia, Valor: tipos.DeDecimal(valor)})
		if l.Tipo != OutroProvento || l.IncideInss {
			baseInss = baseInss.Add(valor)
		}
		if l.Tipo != OutroProvento || l.IncideIrrf {
			rendIrrf = rendIrrf.Add(valor)
		}
	}
	if variaveis.IsPositive() && e.DiasUteis > 0 && e.DiasDescanso > 0 {
		dsr := variaveis.Div(decimal.NewFromInt(e.DiasUteis)).Mul(decimal.NewFromInt(e.DiasDescanso)).Round(2)
		proventos = proventos.Add(dsr)
		baseInss, rendIrrf = baseInss.Add(dsr), rendIrrf.Add(dsr)
		c.Proventos = append(c.Proventos, Linha{Tipo: TipoDSR, Descricao: "DSR sobre horas extras e adicional noturno",
			Referencia: fmt.Sprintf("÷ %d dias úteis × %d de descanso", e.DiasUteis, e.DiasDescanso), Valor: tipos.DeDecimal(dsr)})
	}
	c.TotalProventos = centavos(proventos)
	c.BaseInss = centavos(baseInss)

	// Os descontos da pessoa; a pensão (e o que ela marcar) entra nas deduções do IR.
	outros, deducoesIr := decimal.Zero, decimal.Zero
	var linhasDescontos []Linha
	for _, l := range e.Lancamentos {
		if l.Tipo < ValeTransporte {
			continue
		}
		valor := dec(l.Valor).Round(2)
		referencia := ""
		if l.Tipo == ValeTransporte {
			limite := salarioMes.Mul(seisPct).Round(2)
			if l.Valor == nil || valor.GreaterThan(limite) {
				valor, referencia = limite, "6% do salário (o limite)"
			} else {
				referencia = "custo do mês, abaixo dos 6%"
			}
		}
		outros = outros.Add(valor)
		if l.Tipo == PensaoAlimenticia || l.DeduzIrrf {
			deducoesIr = deducoesIr.Add(valor)
		}
		linhasDescontos = append(linhasDescontos, Linha{Tipo: l.Tipo, Descricao: descricao(l), Referencia: referencia, Valor: tipos.DeDecimal(valor)})
	}

	descontos := outros
	if !e.SemEncargos {
		inss := encargos.CalcularInss(e.Inss, tipos.DeDecimal(baseInss))
		irrf := encargos.CalcularIrrf(e.Irrf, tipos.DeDecimal(rendIrrf), inss.Valor, int(e.Dependentes), tipos.DeDecimal(deducoesIr))
		c.Inss, c.Irrf = &inss, &irrf
		descontos = descontos.Add(inss.Valor.Decimal()).Add(irrf.Valor.Decimal())
		c.Descontos = append(c.Descontos, Linha{Tipo: TipoInss, Descricao: "INSS", Valor: inss.Valor},
			Linha{Tipo: TipoIrrf, Descricao: "IRRF", Valor: irrf.Valor})
		c.FgtsDoMes = centavos(baseInss.Mul(oitoPct))
	} else {
		c.FgtsDoMes = centavos(decimal.Zero)
	}
	c.Descontos = append(c.Descontos, linhasDescontos...)
	c.TotalDescontos = centavos(descontos)
	c.Liquido = centavos(proventos.Sub(descontos))
	return c
}
