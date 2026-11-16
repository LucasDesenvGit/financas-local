// Pacote encargos calcula o INSS e o IRRF do mês pelas tabelas oficiais de cada vigência (#210, ADR 0065).
//
// É estimativa, não holerite: convenção coletiva, benefícios e arredondamentos da empresa mudam o valor. Por isso o
// cálculo devolve, junto, como chegou em cada número (as faixas do INSS, a dedução usada no IRRF, a redução da Lei
// 15.270/2025). Dinheiro em decimal, nunca float64.
package encargos

import (
	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// FaixaInss é uma faixa progressiva: a alíquota vale de onde a anterior parou até Ate.
type FaixaInss struct {
	Ate      tipos.Decimal `json:"ate"`
	Aliquota tipos.Decimal `json:"aliquota"`
}

// TabelaInss é a tabela de uma vigência. A última faixa é o teto.
type TabelaInss struct {
	Inicio tipos.Data  `json:"inicio"`
	Fonte  string      `json:"fonte"`
	Faixas []FaixaInss `json:"faixas"`
}

// FaixaIrrf é uma faixa da tabela mensal: a alíquota e a parcela a deduzir. Ate nil é a última (sem limite).
type FaixaIrrf struct {
	Ate      *tipos.Decimal `json:"ate"`
	Aliquota tipos.Decimal  `json:"aliquota"`
	Deducao  tipos.Decimal  `json:"deducao"`
}

// Reducao é a redução mensal da Lei 15.270/2025 (art. 3º-A da Lei 9.250/1995), pelo rendimento tributável do mês: até
// IsencaoAte, o imposto zera (redução de até Maxima); até Ate, a redução é Constante - Fator x rendimento.
type Reducao struct {
	IsencaoAte tipos.Decimal `json:"isencaoAte"`
	Maxima     tipos.Decimal `json:"maxima"`
	Ate        tipos.Decimal `json:"ate"`
	Constante  tipos.Decimal `json:"constante"`
	Fator      tipos.Decimal `json:"fator"`
}

// TabelaIrrf é a tabela de uma vigência.
type TabelaIrrf struct {
	Inicio               tipos.Data    `json:"inicio"`
	Fonte                string        `json:"fonte"`
	DeducaoDependente    tipos.Decimal `json:"deducaoDependente"`
	DescontoSimplificado tipos.Decimal `json:"descontoSimplificado"`
	Reducao              *Reducao      `json:"reducao"`
	Faixas               []FaixaIrrf   `json:"faixas"`
}

// ParcelaInss é o que uma faixa do INSS somou: de onde até onde, a alíquota e o valor (ao centavo, só para mostrar).
type ParcelaInss struct {
	De       tipos.Decimal `json:"de"`
	Ate      tipos.Decimal `json:"ate"`
	Aliquota tipos.Decimal `json:"aliquota"`
	Valor    tipos.Decimal `json:"valor"`
}

// Inss é o INSS do mês e como chegou nele.
type Inss struct {
	Base     tipos.Decimal `json:"base"`
	Valor    tipos.Decimal `json:"valor"`
	NoTeto   bool          `json:"noTeto"` // a base passou do teto: o INSS é o máximo
	Parcelas []ParcelaInss `json:"parcelas"`
	Fonte    string        `json:"fonte"`
}

// Dedução do IRRF: as legais (INSS e dependentes) ou o desconto simplificado, o que der menos imposto.
const (
	DeducaoLegal        = "legal"
	DeducaoSimplificada = "simplificada"
)

// Irrf é o IRRF do mês e como chegou nele.
type Irrf struct {
	Rendimento      tipos.Decimal `json:"rendimento"`      // o tributável do mês, antes das deduções
	DeducoesLegais  tipos.Decimal `json:"deducoesLegais"`  // INSS + dependentes
	Simplificado    tipos.Decimal `json:"simplificado"`    // o desconto simplificado da vigência
	DeducaoUsada    string        `json:"deducaoUsada"`    // DeducaoLegal ou DeducaoSimplificada
	Base            tipos.Decimal `json:"base"`            // rendimento - a dedução usada
	Aliquota        tipos.Decimal `json:"aliquota"`        // da faixa da base
	ParcelaADeduzir tipos.Decimal `json:"parcelaADeduzir"` // da faixa da base
	ImpostoDaTabela tipos.Decimal `json:"impostoDaTabela"` // base x alíquota - parcela, nunca negativo
	ReducaoLei15270 tipos.Decimal `json:"reducaoLei15270"` // a redução, nunca maior que o imposto
	Valor           tipos.Decimal `json:"valor"`           // o que fica retido
	Fonte           string        `json:"fonte"`
}

// centavos arredonda ao centavo, com o empate saindo do zero (0,005 → 0,01), e deixa sempre duas casas.
func centavos(v decimal.Decimal) tipos.Decimal { return tipos.DeDecimal(v.Round(2)) }

// CalcularInss é o INSS progressivo: cada faixa cobra a alíquota dela sobre o pedaço da base que cai nela, até o teto.
// Arredonda só o total, ao centavo.
func CalcularInss(t TabelaInss, base tipos.Decimal) Inss {
	b := base.Decimal()
	anterior, total := decimal.Zero, decimal.Zero
	r := Inss{Base: base, Fonte: t.Fonte, Parcelas: []ParcelaInss{}}
	for _, f := range t.Faixas {
		if b.GreaterThan(anterior) { // a faixa tem um pedaço da base
			topo := decimal.Min(b, f.Ate.Decimal())
			parcela := topo.Sub(anterior).Mul(f.Aliquota.Decimal())
			total = total.Add(parcela)
			r.Parcelas = append(r.Parcelas, ParcelaInss{De: centavos(anterior), Ate: centavos(topo), Aliquota: f.Aliquota, Valor: centavos(parcela)})
		}
		anterior = f.Ate.Decimal()
	}
	if n := len(t.Faixas); n > 0 && b.GreaterThan(t.Faixas[n-1].Ate.Decimal()) {
		r.NoTeto = true
	}
	r.Valor = centavos(total)
	return r
}

// CalcularIrrf é o IRRF mensal: a base é o rendimento menos a dedução que der menos imposto (as legais, INSS,
// dependentes e outras deduções da lei, como a pensão alimentícia, ou o desconto simplificado); o imposto é o da faixa
// da base; e, desde 2026, a redução da Lei 15.270/2025 sai do imposto, pelo rendimento tributável do mês.
func CalcularIrrf(t TabelaIrrf, rendimento, inss tipos.Decimal, dependentes int, outras tipos.Decimal) Irrf {
	rend := rendimento.Decimal()
	legais := inss.Decimal().Add(t.DeducaoDependente.Decimal().Mul(decimal.NewFromInt(int64(dependentes)))).Add(outras.Decimal())
	simplificado := t.DescontoSimplificado.Decimal()
	r := Irrf{Rendimento: rendimento, DeducoesLegais: centavos(legais), Simplificado: t.DescontoSimplificado, Fonte: t.Fonte,
		DeducaoUsada: DeducaoLegal}
	deducao := legais
	if simplificado.GreaterThan(legais) {
		deducao, r.DeducaoUsada = simplificado, DeducaoSimplificada
	}
	base := decimal.Max(decimal.Zero, rend.Sub(deducao))
	r.Base = centavos(base)

	aliquota, parcela := decimal.Zero, decimal.Zero
	for _, f := range t.Faixas {
		aliquota, parcela = f.Aliquota.Decimal(), f.Deducao.Decimal()
		if f.Ate == nil || base.LessThanOrEqual(f.Ate.Decimal()) {
			break
		}
	}
	r.Aliquota, r.ParcelaADeduzir = tipos.DeDecimal(aliquota), centavos(parcela)
	imposto := decimal.Max(decimal.Zero, base.Mul(aliquota).Sub(parcela)).Round(2)
	r.ImpostoDaTabela = tipos.DeDecimal(imposto)

	reducao := decimal.Zero
	if red := t.Reducao; red != nil {
		switch {
		case rend.LessThanOrEqual(red.IsencaoAte.Decimal()):
			reducao = red.Maxima.Decimal()
		case rend.LessThanOrEqual(red.Ate.Decimal()):
			reducao = decimal.Max(decimal.Zero, red.Constante.Decimal().Sub(red.Fator.Decimal().Mul(rend))).Round(2)
		}
	}
	reducao = decimal.Min(reducao, imposto)
	r.ReducaoLei15270 = centavos(reducao)
	r.Valor = centavos(imposto.Sub(reducao))
	return r
}

// Vigente é a tabela do mês: a da última vigência que começou até ele (as tabelas em qualquer ordem). false se nenhuma
// tinha começado.
func Vigente[T TabelaInss | TabelaIrrf](tabelas []T, mes tipos.Data) (T, bool) {
	var achada T
	var inicioAchado tipos.Data
	ok := false
	for _, t := range tabelas {
		inicio := inicioDe(t)
		if inicio.Depois(mes) {
			continue
		}
		if !ok || inicio.Depois(inicioAchado) {
			achada, inicioAchado, ok = t, inicio, true
		}
	}
	return achada, ok
}

func inicioDe[T TabelaInss | TabelaIrrf](t T) tipos.Data {
	switch v := any(t).(type) {
	case TabelaInss:
		return v.Inicio
	case TabelaIrrf:
		return v.Inicio
	}
	return tipos.Data{}
}
