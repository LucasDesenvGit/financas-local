package orcamentos

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// As constantes do SugestaoOrcamentoService (#39).
const (
	mesesBase           = 6
	minimoMesesComGasto = 2
	faturasParaMedia    = 3
	naturezaEssencial   = 1
	naturezaEstiloVida  = 2
	tipoCartao          = 3
	tipoCategoriaGasto  = 1
)

var (
	fatorPico               = decimal.RequireFromString("2.5")  // mês acima de 2,5× a mediana é pico (IPVA, viagem)
	variacaoTendencia       = decimal.RequireFromString("0.15") // entre os 3 últimos meses e os 3 anteriores
	corteMaximoEstiloDeVida = decimal.RequireFromString("0.5")  // cortar mais que isso não é realista de cumprir
	corteMaximoSemNatureza  = decimal.RequireFromString("0.3")
	partePagaNoCartao       = decimal.RequireFromString("0.5") // "paga no cartão": metade do gasto da base
	meioCentavo             = decimal.RequireFromString("0.005")
	cem                     = decimal.NewFromInt(100)
	dez                     = decimal.NewFromInt(10)
	um                      = decimal.NewFromInt(1)
)

// CartaoUsoLimiteDto é o uso do limite de um cartão: média das últimas faturas fechadas ÷ limite.
type CartaoUsoLimiteDto struct {
	ContaId     int64         `json:"contaId"`
	Nome        string        `json:"nome"`
	Limite      tipos.Decimal `json:"limite"`
	FaturaMedia tipos.Decimal `json:"faturaMedia"`
	Percentual  tipos.Decimal `json:"percentual"`
}

// ItemSugestaoDto é uma categoria na sugestão, na ordem do record ItemSugestaoOrcamentoDto do C#.
type ItemSugestaoDto struct {
	CategoriaId            int64          `json:"categoriaId"`
	Nome                   string         `json:"nome"`
	Cor                    string         `json:"cor"`
	Natureza               *int64         `json:"natureza"`
	Mediana                tipos.Decimal  `json:"mediana"`
	Media                  tipos.Decimal  `json:"media"`
	UltimoMes              tipos.Decimal  `json:"ultimoMes"`
	Tendencia              *string        `json:"tendencia"`
	MesesComGasto          int            `json:"mesesComGasto"`
	PagoNoCartaoPercentual tipos.Decimal  `json:"pagoNoCartaoPercentual"`
	Sugerido               *tipos.Decimal `json:"sugerido"`
	Atual                  *tipos.Decimal `json:"atual"`
	Selecionado            bool           `json:"selecionado"`
	Motivos                []string       `json:"motivos"`
}

// SugestaoDto é o SugestaoOrcamentoDto do C#: o resumo (renda, parcelas, poupança, cartões) e uma linha por
// categoria. Ano e Mes têm o tipo do C# (short e byte).
type SugestaoDto struct {
	Ano                    int16                `json:"ano"`
	Mes                    uint8                `json:"mes"`
	MesesBase              int                  `json:"mesesBase"`
	MesesRenda             int                  `json:"mesesRenda"`
	RendaMedia             tipos.Decimal        `json:"rendaMedia"`
	ParcelasCartao         tipos.Decimal        `json:"parcelasCartao"`
	ParcelasFinanciamentos tipos.Decimal        `json:"parcelasFinanciamentos"`
	ParcelasEmprestimos    tipos.Decimal        `json:"parcelasEmprestimos"`
	PercentualPoupanca     tipos.Decimal        `json:"percentualPoupanca"`
	MetaPoupanca           tipos.Decimal        `json:"metaPoupanca"`
	Disponivel             tipos.Decimal        `json:"disponivel"`
	TotalSugerido          tipos.Decimal        `json:"totalSugerido"`
	FaltaParaCaber         *tipos.Decimal       `json:"faltaParaCaber"`
	UsoLimiteCartoes       *tipos.Decimal       `json:"usoLimiteCartoes"`
	NivelCartao            *string              `json:"nivelCartao"`
	LimiteAtencao          tipos.Decimal        `json:"limiteAtencao"`
	LimiteAlerta           tipos.Decimal        `json:"limiteAlerta"`
	Cartoes                []CartaoUsoLimiteDto `json:"cartoes"`
	Avisos                 []string             `json:"avisos"`
	Itens                  []ItemSugestaoDto    `json:"itens"`
}

// linha é a categoria principal na conta da sugestão (subcategorias somam nela).
type linha struct {
	categoriaId                             int64
	nome, cor                               string
	natureza                                *int64
	meses                                   []decimal.Decimal
	mediana, media, ultimoMes, pagoNoCartao decimal.Decimal
	emergencias                             decimal.Decimal
	tendencia                               *string
	mesesComGasto                           int
	valor, atual                            *decimal.Decimal
	motivos                                 []string
}

func d(v tipos.Decimal) decimal.Decimal { return v.Decimal() }
func t(v decimal.Decimal) tipos.Decimal { return tipos.DeDecimal(v) }

// div é a divisão do decimal do C# (com a escala dele).
func div(a, b decimal.Decimal) decimal.Decimal {
	q, err := tipos.Dividir(t(a), t(b))
	if err != nil {
		panic(err) // os divisores aqui nunca são zero: cada chamada confere antes
	}
	return d(q)
}

func arredondar(v decimal.Decimal, casas int32) decimal.Decimal {
	return d(tipos.Arredondar(t(v), casas))
}

// Sugerir é o SugerirAsync (#39), por categoria principal:
//  1. base = mediana mensal dos últimos 6 meses completos, sem emergências e sem compras parceladas; com gasto em
//     menos de 2 meses, não sugere;
//  2. folga: essenciais + FolgaEssenciais%;
//  3. cartão: com uso do limite acima da atenção, o estilo de vida pago no cartão perde CorteCartao%;
//  4. encaixe: se o total passar de renda − parcelas − meta de poupança, corta o estilo de vida e depois o sem
//     classificação, proporcionalmente; essencial nunca; quem já tem orçamento entra com o valor atual;
//  5. arredonda para múltiplos de R$ 10.
func (s *Servico) Sugerir(ctx context.Context, ano, mes int) (SugestaoDto, error) {
	var p [5]int
	for i, chave := range []string{"Orcamento.FolgaEssenciais", "Orcamento.MetaPoupanca", "Orcamento.CartaoAtencao",
		"Orcamento.CartaoAlerta", "Orcamento.CorteCartao"} {
		v, err := s.parametros.ObterInteiro(ctx, chave)
		if err != nil {
			return SugestaoDto{}, err
		}
		p[i] = v
	}
	folga, percentualPoupanca := div(decimal.NewFromInt(int64(p[0])), cem), p[1]
	atencao, alerta, corteCartao := p[2], p[3], div(decimal.NewFromInt(int64(p[4])), cem)
	avisos := []string{}

	// Renda, parcelas e poupança.
	rendaT, parcelasCartaoT, mesesRenda, err := s.relatorios.RendaEParcelas(ctx)
	if err != nil {
		return SugestaoDto{}, err
	}
	credito, err := s.emprestimos.Totais(ctx)
	if err != nil {
		return SugestaoDto{}, err
	}
	renda, parcelasCartao := d(rendaT), d(parcelasCartaoT)
	parcelas := parcelasCartao.Add(d(credito.ParcelaMensalFinanciamentos)).Add(d(credito.ParcelaMensalEmprestimos))
	metaPoupanca := div(renda.Mul(decimal.NewFromInt(int64(percentualPoupanca))), cem)
	disponivel := renda.Sub(parcelas).Sub(metaPoupanca)

	// Cartões: uso do limite.
	cartoes, err := s.usoDosCartoes(ctx)
	if err != nil {
		return SugestaoDto{}, err
	}
	limiteTotal, faturas := decimal.Zero, decimal.Zero
	for _, c := range cartoes {
		limiteTotal, faturas = limiteTotal.Add(d(c.Limite)), faturas.Add(d(c.FaturaMedia))
	}
	var usoGeral *decimal.Decimal
	var nivelCartao *string
	if limiteTotal.IsPositive() {
		u := arredondar(div(faturas, limiteTotal).Mul(cem), 1)
		usoGeral = &u
		nivel := "ok"
		if !u.LessThan(decimal.NewFromInt(int64(alerta))) {
			nivel = "alerta"
		} else if !u.LessThan(decimal.NewFromInt(int64(atencao))) {
			nivel = "atencao"
		}
		nivelCartao = &nivel
	}

	// Histórico por categoria principal.
	inicio, fim, meses, err := s.relatorios.MesesCompletos(ctx, mesesBase)
	if err != nil {
		return SugestaoDto{}, err
	}
	linhas, raizDe, err := s.lerCategorias(ctx, ano, mes, meses)
	if err != nil {
		return SugestaoDto{}, err
	}
	cartoesIds, err := s.idsDosCartoes(ctx)
	if err != nil {
		return SugestaoDto{}, err
	}
	porId := map[int64]*linha{}
	for _, l := range linhas {
		porId[l.categoriaId] = l
	}
	porCartao := map[int64]decimal.Decimal{}
	if meses > 0 {
		despesas, err := s.relatorios.Relevantes(ctx, inicio, fim)
		if err != nil {
			return SugestaoDto{}, err
		}
		for _, x := range despesas {
			if !d(x.Valor).IsNegative() || x.Tipo != transacoes.Despesa || x.CategoriaId == nil ||
				(x.ParcelaTotal != nil && *x.ParcelaTotal > 1) {
				continue
			}
			raiz, ok := raizDe[*x.CategoriaId]
			l := porId[raiz]
			if !ok || l == nil {
				continue
			}
			valor := d(x.Valor).Neg()
			if x.Emergencia {
				l.emergencias = l.emergencias.Add(valor)
				continue
			}
			indice := (x.Data.Ano()-inicio.Ano())*12 + int(x.Data.Mes()) - int(inicio.Mes())
			if indice < 0 || indice >= meses {
				continue
			}
			l.meses[indice] = l.meses[indice].Add(valor)
			if cartoesIds[x.ContaId] {
				porCartao[raiz] = porCartao[raiz].Add(valor)
			}
		}
	}

	// 1 e 2: base e folga.
	for _, l := range linhas {
		calcularBase(l, meses, inicio, porCartao[l.categoriaId], folga)
	}
	var comValor, aplicaveis []*linha
	fixos := decimal.Zero
	semClassificacao := 0
	for _, l := range linhas {
		if l.atual != nil {
			fixos = fixos.Add(*l.atual)
		}
		if l.valor == nil {
			continue
		}
		comValor = append(comValor, l)
		if l.atual == nil {
			aplicaveis = append(aplicaveis, l)
		}
		if l.natureza == nil {
			semClassificacao++
		}
	}
	totalDoMes := func() decimal.Decimal {
		total := fixos
		for _, l := range aplicaveis {
			total = total.Add(*l.valor)
		}
		return total
	}

	// 3: uso do limite do cartão.
	if nivelCartao != nil && (*nivelCartao == "atencao" || *nivelCartao == "alerta") && corteCartao.IsPositive() {
		for _, l := range comValor {
			if l.natureza != nil && *l.natureza == naturezaEstiloVida && !l.pagoNoCartao.LessThan(partePagaNoCartao) {
				v := l.valor.Mul(um.Sub(corteCartao))
				l.valor = &v
				l.motivos = append(l.motivos, fmt.Sprintf("−%s%%: pago no cartão, com %s%% do limite em uso",
					inteiro(corteCartao.Mul(cem)), inteiro(*usoGeral)))
			}
		}
	}
	if nivelCartao != nil && *nivelCartao == "alerta" {
		avisos = append(avisos, fmt.Sprintf("Seus cartões estão usando %s%% do limite (acima de %d%%). Vale reduzir o que "+
			"passa pelo cartão até a fatura baixar.", inteiro(*usoGeral), alerta))
	} else if nivelCartao != nil && *nivelCartao == "atencao" {
		avisos = append(avisos, fmt.Sprintf("Seus cartões estão usando %s%% do limite (acima de %d%%): a sugestão aperta o "+
			"estilo de vida pago no cartão.", inteiro(*usoGeral), atencao))
	}

	// 4: caber na renda.
	var falta *decimal.Decimal
	if !renda.IsPositive() {
		avisos = append(avisos, "Sem receitas nos últimos meses: a sugestão usa só o seu histórico de gastos, sem teto de renda.")
	} else {
		excesso := totalDoMes().Sub(disponivel)
		excesso = cortar(filtrar(aplicaveis, func(l *linha) bool { return l.natureza != nil && *l.natureza == naturezaEstiloVida }),
			excesso, corteMaximoEstiloDeVida)
		excesso = cortar(filtrar(aplicaveis, func(l *linha) bool { return l.natureza == nil }), excesso, corteMaximoSemNatureza)
		if excesso.GreaterThan(meioCentavo) {
			f := arredondar(excesso, 2)
			falta = &f
		}
	}

	// 5: arredondar para R$ 10. Essencial arredonda para o mais próximo; o resto, para baixo.
	for _, l := range comValor {
		dezenas := div(*l.valor, dez)
		if l.natureza != nil && *l.natureza == naturezaEssencial {
			dezenas = dezenas.Round(0) // o Math.Round com AwayFromZero
		} else {
			dezenas = dezenas.Floor()
		}
		v := dezenas.Mul(dez)
		if dez.GreaterThan(v) {
			v = dez
		}
		l.valor = &v
	}
	if renda.IsPositive() && falta == nil {
		// O arredondamento dos essenciais pode passar alguns reais do disponível: tira R$ 10 do maior estilo de vida.
		cortaveis := filtrar(aplicaveis, func(l *linha) bool { return l.natureza == nil || *l.natureza != naturezaEssencial })
		for totalDoMes().GreaterThan(disponivel) {
			var maior *linha
			for _, l := range cortaveis {
				if l.valor.GreaterThan(dez) && (maior == nil || l.valor.GreaterThan(*maior.valor)) {
					maior = l
				}
			}
			if maior == nil {
				break
			}
			v := maior.valor.Sub(dez)
			maior.valor = &v
		}
	}

	totalSugerido := totalDoMes()
	if falta != nil {
		f := decimal.Max(decimal.Zero, totalSugerido.Sub(arredondar(disponivel, 2)))
		falta = &f
		avisos = append(avisos, fmt.Sprintf("Mesmo cortando estilo de vida e o que está sem classificação, faltam %s para caber "+
			"em renda − parcelas − poupança. Os essenciais não foram cortados: revise os valores ou a meta de poupança (%d%%).",
			reais(f), percentualPoupanca))
	}
	switch {
	case meses == 0:
		avisos = append(avisos, "Ainda não há um mês completo de transações: sem histórico para sugerir.")
	case meses < mesesBase:
		texto := "meses completos"
		if meses == 1 {
			texto = "mês completo"
		}
		avisos = append(avisos, fmt.Sprintf("Só %d %s de histórico: a sugestão fica menos precisa.", meses, texto))
	}
	if semClassificacao > 0 {
		texto := "categorias estão"
		if semClassificacao == 1 {
			texto = "categoria está"
		}
		avisos = append(avisos, fmt.Sprintf("%d %s sem natureza (essencial ou estilo de vida). Classifique em Categorias e "+
			"regras para a sugestão ficar melhor.", semClassificacao, texto))
	}

	return SugestaoDto{
		Ano: int16(ano), Mes: uint8(mes), MesesBase: meses, MesesRenda: mesesRenda,
		RendaMedia: t(arredondar(renda, 2)), ParcelasCartao: t(arredondar(parcelasCartao, 2)),
		ParcelasFinanciamentos: credito.ParcelaMensalFinanciamentos, ParcelasEmprestimos: credito.ParcelaMensalEmprestimos,
		PercentualPoupanca: tipos.DecimalDeInteiro(int64(percentualPoupanca)), MetaPoupanca: t(arredondar(metaPoupanca, 2)),
		Disponivel: t(arredondar(disponivel, 2)), TotalSugerido: t(totalSugerido), FaltaParaCaber: opcional(falta),
		UsoLimiteCartoes: opcional(usoGeral), NivelCartao: nivelCartao,
		LimiteAtencao: tipos.DecimalDeInteiro(int64(atencao)), LimiteAlerta: tipos.DecimalDeInteiro(int64(alerta)),
		Cartoes: cartoes, Avisos: avisos, Itens: itens(linhas),
	}, nil
}

func opcional(v *decimal.Decimal) *tipos.Decimal {
	if v == nil {
		return nil
	}
	r := t(*v)
	return &r
}

func filtrar(lista []*linha, f func(*linha) bool) []*linha {
	var r []*linha
	for _, l := range lista {
		if f(l) {
			r = append(r, l)
		}
	}
	return r
}

// calcularBase são os passos 1 e 2 de uma linha: totais, mediana, média, tendência, motivos e o valor com a folga.
func calcularBase(l *linha, meses int, inicio tipos.Data, noCartao, folga decimal.Decimal) {
	total := decimal.Zero
	for _, v := range l.meses {
		total = total.Add(v)
		if v.IsPositive() {
			l.mesesComGasto++
		}
	}
	if meses > 0 {
		l.media = div(total, decimal.NewFromInt(int64(meses)))
		l.ultimoMes = l.meses[meses-1]
	}
	l.mediana = mediana(l.meses)
	if total.IsPositive() {
		l.pagoNoCartao = div(noCartao, total)
	}
	// Tendência só faz sentido com gasto recorrente (senão um único mês vira "queda").
	if meses >= 6 && l.mesesComGasto >= minimoMesesComGasto {
		recente, anterior := media3(l.meses[meses-3:]), media3(l.meses[:3])
		tendencia := "estavel"
		switch {
		case !anterior.IsPositive():
			if recente.IsPositive() {
				tendencia = "alta"
			}
		case recente.GreaterThan(anterior.Mul(um.Add(variacaoTendencia))):
			tendencia = "alta"
		case recente.LessThan(anterior.Mul(um.Sub(variacaoTendencia))):
			tendencia = "queda"
		}
		l.tendencia = &tendencia
	}
	if l.mesesComGasto < minimoMesesComGasto {
		if l.mesesComGasto == 1 {
			l.motivos = append(l.motivos, fmt.Sprintf("gasto em só 1 dos últimos %d meses (%s): sem dados suficientes",
				meses, nomeMes(inicio, primeiroPositivo(l.meses))))
		}
		return
	}
	var base decimal.Decimal
	if l.mediana.IsPositive() {
		base = l.mediana
		l.motivos = append(l.motivos, fmt.Sprintf("mediana de %d meses", meses))
	} else {
		base = l.media
		l.motivos = append(l.motivos, fmt.Sprintf("gasto irregular (%d de %d meses): usa a média mensal", l.mesesComGasto, meses))
	}
	maior, indiceMaior := l.meses[0], 0
	for i, v := range l.meses {
		if v.GreaterThan(maior) {
			maior, indiceMaior = v, i
		}
	}
	if l.mediana.IsPositive() && maior.GreaterThan(l.mediana.Mul(fatorPico)) {
		l.motivos = append(l.motivos, fmt.Sprintf("pico em %s (%s) fora da base", nomeMes(inicio, indiceMaior), reais(maior)))
	}
	if l.tendencia != nil && *l.tendencia == "alta" {
		l.motivos = append(l.motivos, "tendência de alta nos últimos 3 meses (não aumenta o orçamento)")
	}
	if l.emergencias.IsPositive() {
		l.motivos = append(l.motivos, fmt.Sprintf("sem as emergências (%s)", reais(l.emergencias)))
	}
	if l.natureza != nil && *l.natureza == naturezaEssencial && folga.IsPositive() {
		base = base.Mul(um.Add(folga))
		l.motivos = append(l.motivos, fmt.Sprintf("+%s%% de folga (essencial)", inteiro(folga.Mul(cem))))
	}
	l.valor = &base
}

func primeiroPositivo(meses []decimal.Decimal) int {
	for i, v := range meses {
		if v.IsPositive() {
			return i
		}
	}
	return -1
}

// media3 é o Average() do C# de três meses (soma ÷ 3, com a divisão do decimal).
func media3(v []decimal.Decimal) decimal.Decimal {
	return div(v[0].Add(v[1]).Add(v[2]), decimal.NewFromInt(3))
}

// cortar corta o grupo proporcionalmente até o excesso ou até corteMaximo do total dele, e devolve o que sobrou.
func cortar(grupo []*linha, excesso, corteMaximo decimal.Decimal) decimal.Decimal {
	if !excesso.IsPositive() || len(grupo) == 0 {
		return excesso
	}
	total := decimal.Zero
	for _, l := range grupo {
		total = total.Add(*l.valor)
	}
	if !total.IsPositive() {
		return excesso
	}
	corte := decimal.Min(excesso, total.Mul(corteMaximo))
	fracao := div(corte, total)
	for _, l := range grupo {
		v := l.valor.Mul(um.Sub(fracao))
		l.valor = &v
		l.motivos = append(l.motivos, fmt.Sprintf("−%s%% para caber na renda", inteiro(fracao.Mul(cem))))
	}
	return excesso.Sub(corte)
}

func mediana(valores []decimal.Decimal) decimal.Decimal {
	if len(valores) == 0 {
		return decimal.Zero
	}
	ordenados := slices.Clone(valores)
	slices.SortFunc(ordenados, func(a, b decimal.Decimal) int { return a.Cmp(b) })
	meio := len(ordenados) / 2
	if len(ordenados)%2 == 1 {
		return ordenados[meio]
	}
	return div(ordenados[meio-1].Add(ordenados[meio]), decimal.NewFromInt(2))
}

// itens são as linhas da tela: pelo sugerido (as sem sugestão no fim), pela média e pelo nome (a ordem pt-BR do C#),
// só as com sugestão, gasto ou orçamento atual.
func itens(linhas []*linha) []ItemSugestaoDto {
	ordenadas := slices.Clone(linhas)
	menosUm := decimal.NewFromInt(-1)
	valorOu := func(l *linha) decimal.Decimal {
		if l.valor == nil {
			return menosUm
		}
		return *l.valor
	}
	slices.SortStableFunc(ordenadas, func(a, b *linha) int {
		if c := valorOu(b).Cmp(valorOu(a)); c != 0 {
			return c
		}
		if c := b.media.Cmp(a.media); c != 0 {
			return c
		}
		return texto.CompararNaCultura(a.nome, b.nome)
	})
	lista := []ItemSugestaoDto{}
	for _, l := range ordenadas {
		if l.valor == nil && l.mesesComGasto == 0 && l.atual == nil {
			continue
		}
		motivos := l.motivos
		if motivos == nil {
			motivos = []string{}
		}
		lista = append(lista, ItemSugestaoDto{
			CategoriaId: l.categoriaId, Nome: l.nome, Cor: l.cor, Natureza: l.natureza,
			Mediana: t(arredondar(l.mediana, 2)), Media: t(arredondar(l.media, 2)), UltimoMes: t(arredondar(l.ultimoMes, 2)),
			Tendencia: l.tendencia, MesesComGasto: l.mesesComGasto,
			PagoNoCartaoPercentual: t(arredondar(l.pagoNoCartao.Mul(cem), 0)),
			Sugerido:               opcional(l.valor), Atual: opcional(l.atual), Selecionado: l.valor != nil && l.atual == nil,
			Motivos: motivos,
		})
	}
	return lista
}

// lerCategorias são as categorias de despesa: as linhas (as principais e ativas, com o orçamento atual do mês) e a
// raiz de cada uma (a principal da subcategoria, se a mãe também é de despesa).
func (s *Servico) lerCategorias(ctx context.Context, ano, mes, meses int) ([]*linha, map[int64]int64, error) {
	linhasDb, err := s.db.QueryContext(ctx, `SELECT "Id", "Nome", "Cor", "CategoriaPaiId", "Natureza", "Ativa" FROM "Categoria"
		WHERE "Tipo" = ? ORDER BY "Id"`, tipoCategoriaGasto)
	if err != nil {
		return nil, nil, fmt.Errorf("ler as categorias de despesa: %w", err)
	}
	type categoria struct {
		id       int64
		nome     string
		cor      string
		pai      *int64
		natureza *int64
		ativa    bool
	}
	var categorias []categoria
	existe := map[int64]bool{}
	for linhasDb.Next() {
		var c categoria
		if err := linhasDb.Scan(&c.id, &c.nome, &c.cor, &c.pai, &c.natureza, &c.ativa); err != nil {
			linhasDb.Close()
			return nil, nil, fmt.Errorf("ler categoria de despesa: %w", err)
		}
		categorias = append(categorias, c)
		existe[c.id] = true
	}
	linhasDb.Close()
	if err := linhasDb.Err(); err != nil {
		return nil, nil, err
	}
	atuais, err := s.valoresDoMes(ctx, ano, mes)
	if err != nil {
		return nil, nil, err
	}
	raizDe := map[int64]int64{}
	var linhas []*linha
	for _, c := range categorias {
		raiz := c.id
		if c.pai != nil && existe[*c.pai] {
			raiz = *c.pai
		}
		raizDe[c.id] = raiz
		if !c.ativa || raiz != c.id {
			continue
		}
		l := &linha{categoriaId: c.id, nome: c.nome, cor: c.cor, natureza: c.natureza, meses: make([]decimal.Decimal, meses)}
		if v, ok := atuais[c.id]; ok {
			l.atual = &v
		}
		linhas = append(linhas, l)
	}
	return linhas, raizDe, nil
}

func (s *Servico) valoresDoMes(ctx context.Context, ano, mes int) (map[int64]decimal.Decimal, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "CategoriaId", "Valor" FROM "Orcamento" WHERE "Ano" = ? AND "Mes" = ?`, ano, mes)
	if err != nil {
		return nil, fmt.Errorf("ler os orçamentos do mês: %w", err)
	}
	defer linhas.Close()
	valores := map[int64]decimal.Decimal{}
	for linhas.Next() {
		var id int64
		var v tipos.Decimal
		if err := linhas.Scan(&id, &v); err != nil {
			return nil, fmt.Errorf("ler orçamento do mês: %w", err)
		}
		valores[id] = d(v)
	}
	return valores, linhas.Err()
}

func (s *Servico) idsDosCartoes(ctx context.Context) (map[int64]bool, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id" FROM "Conta" WHERE "Tipo" = ?`, tipoCartao)
	if err != nil {
		return nil, fmt.Errorf("ler os cartões: %w", err)
	}
	defer linhas.Close()
	ids := map[int64]bool{}
	for linhas.Next() {
		var id int64
		if err := linhas.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, linhas.Err()
}

// usoDosCartoes é o UsoDosCartoesAsync: média das últimas 3 faturas fechadas ÷ limite, dos cartões ativos com limite,
// do maior uso para o menor. O limite é texto no banco: o "maior que zero" é conferido em Go.
func (s *Servico) usoDosCartoes(ctx context.Context) ([]CartaoUsoLimiteDto, error) {
	hoje := relogio.Hoje(s.relogio)
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Nome", "LimiteCredito" FROM "Conta"
		WHERE "Tipo" = ? AND NOT "Arquivada" AND "LimiteCredito" IS NOT NULL ORDER BY "Id"`, tipoCartao)
	if err != nil {
		return nil, fmt.Errorf("ler os cartões com limite: %w", err)
	}
	type cartao struct {
		id     int64
		nome   string
		limite tipos.Decimal
	}
	var cartoes []cartao
	for linhas.Next() {
		var c cartao
		if err := linhas.Scan(&c.id, &c.nome, &c.limite); err != nil {
			linhas.Close()
			return nil, err
		}
		if d(c.limite).IsPositive() {
			cartoes = append(cartoes, c)
		}
	}
	linhas.Close()
	resultado := []CartaoUsoLimiteDto{}
	for _, c := range cartoes {
		faturas, err := s.ultimasFaturas(ctx, c.id, hoje)
		if err != nil {
			return nil, err
		}
		if len(faturas) == 0 {
			continue
		}
		soma := decimal.Zero
		for _, f := range faturas {
			soma = soma.Add(f.Abs())
		}
		media := div(soma, decimal.NewFromInt(int64(len(faturas))))
		resultado = append(resultado, CartaoUsoLimiteDto{ContaId: c.id, Nome: c.nome, Limite: c.limite,
			FaturaMedia: t(arredondar(media, 2)), Percentual: t(arredondar(div(media, d(c.limite)).Mul(cem), 1))})
	}
	slices.SortStableFunc(resultado, func(a, b CartaoUsoLimiteDto) int { return d(b.Percentual).Cmp(d(a.Percentual)) })
	return resultado, nil
}

func (s *Servico) ultimasFaturas(ctx context.Context, conta int64, hoje tipos.Data) ([]decimal.Decimal, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "ValorTotal" FROM "FaturaCartao" WHERE "ContaId" = ? AND "DataVencimento" <= ?
		ORDER BY "DataVencimento" DESC LIMIT ?`, conta, hoje, faturasParaMedia)
	if err != nil {
		return nil, fmt.Errorf("ler as faturas do cartão %d: %w", conta, err)
	}
	defer linhas.Close()
	var valores []decimal.Decimal
	for linhas.Next() {
		var v tipos.Decimal
		if err := linhas.Scan(&v); err != nil {
			return nil, err
		}
		valores = append(valores, d(v))
	}
	return valores, linhas.Err()
}

// inteiro é o {x:0} do C#: o número sem casas, arredondado para longe do zero no meio.
func inteiro(v decimal.Decimal) string { return v.Round(0).String() }

// mesesAbreviados são os do ToString("MMM", pt-BR) do .NET (ICU), já sem o ponto.
var mesesAbreviados = [...]string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}

// nomeMes é o NomeMes: "out/26", o mês do índice a partir do início (índice negativo vale o primeiro).
func nomeMes(inicio tipos.Data, indice int) string {
	m := inicio.SomarMeses(max(0, indice))
	return fmt.Sprintf("%s/%02d", mesesAbreviados[m.Mes()-1], m.Ano()%100)
}

// reais é o ToString("C0", pt-BR) do .NET: "R$ 1.235", com o espaço não separável depois do R$ e o milhar com ponto,
// arredondado para longe do zero no meio.
func reais(v decimal.Decimal) string {
	n := v.Round(0)
	sinal := ""
	if n.IsNegative() {
		sinal, n = "-", n.Neg()
	}
	digitos := n.String()
	var partes []string
	for len(digitos) > 3 {
		partes = append([]string{digitos[len(digitos)-3:]}, partes...)
		digitos = digitos[:len(digitos)-3]
	}
	partes = append([]string{digitos}, partes...)
	return sinal + "R$ " + strings.Join(partes, ".")
}
