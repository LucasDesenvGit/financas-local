// Pacote rescisao é a rescisão do módulo Trabalho (#215, ADR 0065): quanto se recebe ao sair do emprego em cada tipo
// de desligamento, com o FGTS (a multa e o saque), e o registro da rescisão real.
//
// É estimativa, não termo de rescisão: convenção coletiva e acertos da empresa mudam o valor.
package rescisao

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Tipos de desligamento.
const (
	SemJustaCausa  int64 = 1
	PedidoDemissao int64 = 2
	Acordo         int64 = 3 // CLT, art. 484-A
	JustaCausa     int64 = 4
	FimExperiencia int64 = 5
)

// Aviso prévio.
const (
	SemAviso        int64 = 0
	AvisoTrabalhado int64 = 1
	AvisoIndenizado int64 = 2 // no pedido de demissão: a pessoa não cumpriu (desconta)
)

// TipoConhecido diz se o tipo existe.
func TipoConhecido(t int64) bool { return t >= SemJustaCausa && t <= FimExperiencia }

var nomesDosTipos = map[int64]string{SemJustaCausa: "Dispensa sem justa causa", PedidoDemissao: "Pedido de demissão",
	Acordo: "Acordo (art. 484-A)", JustaCausa: "Justa causa", FimExperiencia: "Fim do contrato de experiência"}

// NomeDoTipo é o nome do tipo, para a tela.
func NomeDoTipo(t int64) string { return nomesDosTipos[t] }

// AnosCompletos é quantos aniversários de admissão já passaram até a data.
func AnosCompletos(admissao, data tipos.Data) int {
	anos := 0
	for !admissao.SomarAnos(anos + 1).Depois(data) {
		anos++
	}
	return anos
}

// DiasDeAviso é o aviso prévio proporcional: 30 dias mais 3 por ano completo, até 90 (Lei 12.506/2011).
func DiasDeAviso(admissao, saida tipos.Data) int {
	return min(30+3*AnosCompletos(admissao, saida), 90)
}

// AvisoPago são os dias de aviso que a empresa paga e a data projetada da saída (o aviso indenizado conta como tempo
// de serviço, para o 13º e as férias). Na dispensa sem justa causa indenizada, todos os dias; no acordo indenizado, a
// metade (art. 484-A); nos outros casos, nenhum.
func AvisoPago(tipo, aviso int64, admissao, saida tipos.Data) (dias int, projetada tipos.Data) {
	if aviso == AvisoIndenizado {
		switch tipo {
		case SemJustaCausa:
			dias = DiasDeAviso(admissao, saida)
		case Acordo:
			dias = DiasDeAviso(admissao, saida) / 2
		}
	}
	return dias, saida.SomarDias(dias)
}

// AvosFerias são os meses do período aquisitivo em curso até a data (a fração de 15 dias ou mais conta), até 12.
func AvosFerias(inicioPeriodo, ate tipos.Data) int {
	avos, limite := 0, ate.SomarDias(1) // o dia seguinte: os intervalos aqui não incluem o fim
	for i := 0; i < 12 && !inicioPeriodo.SomarMeses(i).Depois(ate); i++ {
		de, fim := inicioPeriodo.SomarMeses(i), inicioPeriodo.SomarMeses(i+1)
		if fim.Depois(limite) {
			fim = limite
		}
		if fim.NumeroDoDia()-de.NumeroDoDia() >= 15 {
			avos++
		}
	}
	return avos
}

// FeriasVencidas são os dias de férias de um período completo que não foram tirados.
type FeriasVencidas struct {
	Inicio  tipos.Data `json:"inicio"`
	Dias    int        `json:"dias"`
	EmDobro bool       `json:"emDobro"` // o prazo para tirar passou: a empresa paga em dobro (CLT, art. 137)
}

// Entrada é o que o cálculo precisa (o serviço junta).
type Entrada struct {
	Tipo, Aviso       int64
	Admissao, Saida   tipos.Data
	SalarioMes        tipos.Decimal // o salário da vigência do mês da saída
	SomaVariaveis     tipos.Decimal // horas extras, adicional noturno e DSR dos 12 meses antes
	Dependentes       int64
	Vencidas          []FeriasVencidas
	AvosFerias        int           // do período em curso, até a data projetada
	Avos13            int           // do ano da saída, até a data projetada
	PrimeiraParcela13 tipos.Decimal // a 1ª parcela do 13º que já veio
	SaldoFgts         tipos.Decimal
	Inss              encargos.TabelaInss
	Irrf              encargos.TabelaIrrf
}

// Linha é uma verba ou um desconto da rescisão.
type Linha struct {
	Descricao  string        `json:"descricao"`
	Referencia string        `json:"referencia"`
	Valor      tipos.Decimal `json:"valor"`
}

// Fgts é o FGTS na rescisão.
type Fgts struct {
	Saldo            tipos.Decimal `json:"saldo"`            // o estimado até a saída
	DepositoRescisao tipos.Decimal `json:"depositoRescisao"` // 8% do saldo de salário, do aviso e do 13º
	PercentualMulta  int           `json:"percentualMulta"`  // 40, 20 ou 0
	Multa            tipos.Decimal `json:"multa"`
	PercentualSaque  int           `json:"percentualSaque"` // 100, 80 ou 0
	Sacavel          tipos.Decimal `json:"sacavel"`         // o que dá para sacar: a parte do saldo e a multa
}

// Calculo é a rescisão calculada.
type Calculo struct {
	Tipo             int64         `json:"tipo"`
	NomeDoTipo       string        `json:"nomeDoTipo"`
	Aviso            int64         `json:"aviso"`
	Saida            tipos.Data    `json:"saida"`
	DataProjetada    tipos.Data    `json:"dataProjetada"`
	DiasDeAviso      int           `json:"diasDeAviso"` // o direito, mesmo quando não pago
	Remuneracao      tipos.Decimal `json:"remuneracao"`
	Verbas           []Linha       `json:"verbas"`
	Descontos        []Linha       `json:"descontos"`
	TotalVerbas      tipos.Decimal `json:"totalVerbas"`
	TotalDescontos   tipos.Decimal `json:"totalDescontos"`
	Liquido          tipos.Decimal `json:"liquido"` // o que a empresa paga na conta
	Fgts             Fgts          `json:"fgts"`
	SeguroDesemprego bool          `json:"seguroDesemprego"` // pode ter direito (só a indicação, sem o cálculo)
}

var (
	trinta = decimal.NewFromInt(30)
	doze   = decimal.NewFromInt(12)
	tres   = decimal.NewFromInt(3)
	oito   = decimal.RequireFromString("0.08")
)

func centavos(v decimal.Decimal) tipos.Decimal { return tipos.DeDecimal(v.Round(2)) }

// diasNoMes são os dias do mês da saída que entram no saldo de salário: o dia da saída (o último dia do mês vale 30).
func diasNoMes(saida, admissao tipos.Data) int {
	inicio := 1
	if admissao.Ano() == saida.Ano() && admissao.Mes() == saida.Mes() {
		inicio = admissao.Dia()
	}
	fim := min(saida.Dia(), 30)
	if saida.SomarDias(1).Dia() == 1 {
		fim = 30
	}
	return max(0, fim-inicio+1)
}

// Calcular monta a rescisão.
func Calcular(e Entrada) Calculo {
	media := e.SomaVariaveis.Decimal().Div(doze).Round(2)
	remuneracao := e.SalarioMes.Decimal().Add(media)
	diasPagos, projetada := AvisoPago(e.Tipo, e.Aviso, e.Admissao, e.Saida)
	c := Calculo{Tipo: e.Tipo, NomeDoTipo: NomeDoTipo(e.Tipo), Aviso: e.Aviso, Saida: e.Saida, DataProjetada: projetada,
		DiasDeAviso: DiasDeAviso(e.Admissao, e.Saida), Remuneracao: centavos(remuneracao), Verbas: []Linha{}, Descontos: []Linha{}}
	verba := func(descricao, referencia string, v decimal.Decimal) decimal.Decimal {
		v = v.Round(2)
		if v.IsPositive() {
			c.Verbas = append(c.Verbas, Linha{descricao, referencia, tipos.DeDecimal(v)})
		}
		return v
	}
	desconto := func(descricao, referencia string, v decimal.Decimal) decimal.Decimal {
		v = v.Round(2)
		if v.IsPositive() {
			c.Descontos = append(c.Descontos, Linha{descricao, referencia, tipos.DeDecimal(v)})
		}
		return v
	}

	dias := diasNoMes(e.Saida, e.Admissao)
	saldo := verba("Saldo de salário", fmt.Sprintf("%d dias", dias), e.SalarioMes.Decimal().Mul(decimal.NewFromInt(int64(dias))).Div(trinta))
	aviso := verba("Aviso prévio indenizado", fmt.Sprintf("%d dias", diasPagos), remuneracao.Mul(decimal.NewFromInt(int64(diasPagos))).Div(trinta))
	decimo, ferias := decimal.Zero, decimal.Zero
	if e.Tipo != JustaCausa {
		decimo = verba("13º proporcional", fmt.Sprintf("%d/12", e.Avos13), remuneracao.Mul(decimal.NewFromInt(int64(e.Avos13))).Div(doze))
		prop := verba("Férias proporcionais", fmt.Sprintf("%d/12", e.AvosFerias), remuneracao.Mul(decimal.NewFromInt(int64(e.AvosFerias))).Div(doze))
		ferias = ferias.Add(prop).Add(verba("1/3 das férias proporcionais", "", prop.Div(tres)))
	}
	for _, v := range e.Vencidas {
		valor := remuneracao.Mul(decimal.NewFromInt(int64(v.Dias))).Div(trinta)
		referencia := fmt.Sprintf("%d dias", v.Dias)
		if v.EmDobro {
			valor, referencia = valor.Mul(decimal.NewFromInt(2)), referencia+", em dobro"
		}
		pago := verba(fmt.Sprintf("Férias vencidas (período de %02d/%02d/%d)", v.Inicio.Dia(), v.Inicio.Mes(), v.Inicio.Ano()), referencia, valor)
		ferias = ferias.Add(pago).Add(verba("1/3 das férias vencidas", "", pago.Div(tres)))
	}

	// Os descontos: INSS e IRRF do saldo de salário (o aviso indenizado e as férias indenizadas não têm), os do 13º
	// (separados, sem o desconto simplificado), a 1ª parcela do 13º que já veio e o aviso que não foi cumprido.
	descontos := decimal.Zero
	inssSaldo := encargos.CalcularInss(e.Inss, tipos.DeDecimal(saldo))
	irrfSaldo := encargos.CalcularIrrf(e.Irrf, tipos.DeDecimal(saldo), inssSaldo.Valor, int(e.Dependentes), tipos.DeDecimal(decimal.Zero))
	descontos = descontos.Add(desconto("INSS", "sobre o saldo de salário", inssSaldo.Valor.Decimal()))
	descontos = descontos.Add(desconto("IRRF", "sobre o saldo de salário", irrfSaldo.Valor.Decimal()))
	if decimo.IsPositive() {
		tabela := e.Irrf
		tabela.DescontoSimplificado = tipos.DeDecimal(decimal.Zero)
		inss13 := encargos.CalcularInss(e.Inss, tipos.DeDecimal(decimo))
		irrf13 := encargos.CalcularIrrf(tabela, tipos.DeDecimal(decimo), inss13.Valor, int(e.Dependentes), tipos.DeDecimal(decimal.Zero))
		descontos = descontos.Add(desconto("INSS do 13º", "", inss13.Valor.Decimal()))
		descontos = descontos.Add(desconto("IRRF do 13º", "", irrf13.Valor.Decimal()))
		descontos = descontos.Add(desconto("1ª parcela do 13º que já veio", "", decimal.Min(e.PrimeiraParcela13.Decimal(), decimo)))
	}
	if e.Tipo == PedidoDemissao && e.Aviso == AvisoIndenizado {
		descontos = descontos.Add(desconto("Aviso prévio não cumprido", "30 dias", e.SalarioMes.Decimal()))
	}

	total := saldo.Add(aviso).Add(decimo).Add(ferias)
	c.TotalVerbas, c.TotalDescontos = centavos(total), centavos(descontos)
	c.Liquido = centavos(total.Sub(descontos))

	// O FGTS: o depósito da rescisão, a multa e o saque.
	deposito := saldo.Add(aviso).Add(decimo).Mul(oito).Round(2)
	base := e.SaldoFgts.Decimal().Add(deposito)
	multa, saque := 0, 0
	switch e.Tipo {
	case SemJustaCausa:
		multa, saque = 40, 100
	case Acordo:
		multa, saque = 20, 80
	case FimExperiencia:
		saque = 100
	}
	valorMulta := base.Mul(decimal.NewFromInt(int64(multa))).Div(decimal.NewFromInt(100)).Round(2)
	sacavel := base.Mul(decimal.NewFromInt(int64(saque))).Div(decimal.NewFromInt(100)).Add(valorMulta)
	c.Fgts = Fgts{Saldo: centavos(e.SaldoFgts.Decimal()), DepositoRescisao: tipos.DeDecimal(deposito), PercentualMulta: multa,
		Multa: tipos.DeDecimal(valorMulta), PercentualSaque: saque, Sacavel: centavos(sacavel)}
	// O seguro-desemprego: só na dispensa sem justa causa, com 12 meses ou mais de emprego (na 1ª vez que pede).
	c.SeguroDesemprego = e.Tipo == SemJustaCausa && AnosCompletos(e.Admissao, e.Saida) >= 1
	return c
}
