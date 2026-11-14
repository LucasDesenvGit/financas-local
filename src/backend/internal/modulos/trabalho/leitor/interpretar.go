package leitor

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Rubrica é uma linha da tabela do holerite lida.
type Rubrica struct {
	Codigo     string        `json:"codigo"`
	Descricao  string        `json:"descricao"`
	Referencia string        `json:"referencia"`
	Valor      tipos.Decimal `json:"valor"`
	Provento   bool          `json:"provento"` // o valor estava na coluna de vencimentos (proventos, créditos)
	Trecho     string        `json:"trecho"`   // a linha do PDF de onde saiu
}

// Prova é uma conferência entre o que o PDF imprime e o que o leitor somou.
type Prova struct {
	Descricao  string        `json:"descricao"`
	Impresso   tipos.Decimal `json:"impresso"`
	Encontrado tipos.Decimal `json:"encontrado"`
	Diferenca  tipos.Decimal `json:"diferenca"` // encontrado - impresso
	Bateu      bool          `json:"bateu"`
}

// Lido é o holerite lido do PDF.
type Lido struct {
	Empregador       string         `json:"empregador"`
	Cnpj             *string        `json:"cnpj"` // só os dígitos
	Mes              *tipos.Data    `json:"mes"`  // a competência (dia 1)
	Rubricas         []Rubrica      `json:"rubricas"`
	TotalVencimentos *tipos.Decimal `json:"totalVencimentos"`
	TotalDescontos   *tipos.Decimal `json:"totalDescontos"`
	Liquido          *tipos.Decimal `json:"liquido"`
	SalarioBase      *tipos.Decimal `json:"salarioBase"`
	BaseInss         *tipos.Decimal `json:"baseInss"`
	BaseFgts         *tipos.Decimal `json:"baseFgts"`
	FgtsDoMes        *tipos.Decimal `json:"fgtsDoMes"`
	BaseIrrf         *tipos.Decimal `json:"baseIrrf"`
	Provas           []Prova        `json:"provas"`
}

var (
	reDinheiro   = regexp.MustCompile(`^(?:R\$\s?)?(?:\d{1,3}(?:\.\d{3})+|\d+),\d{2}-?$`)
	reReferencia = regexp.MustCompile(`(?i)^\d+(?:[.,]\d+)?\s?(?:%|d|h|hs|dias|horas)?$|^\d{1,3}:\d{2}$`)
	reCodigo     = regexp.MustCompile(`^\d{1,6}$`)
	reCnpj       = regexp.MustCompile(`\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2}`)
	reRotuloCnpj = regexp.MustCompile(`(?i)[\s\-–:]*\bCNPJ\b[\s:]*`)
	reMesNome    = regexp.MustCompile(`(?i)(janeiro|fevereiro|mar[çc]o|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)\s*(?:de|/|-)?\s*(\d{4})`)
	reMesNumero  = regexp.MustCompile(`(?:^|[^\d/])(0[1-9]|1[0-2])/(\d{4})(?:$|[^\d/])`)
)

var nomesDosMeses = map[string]time.Month{"JANEIRO": 1, "FEVEREIRO": 2, "MARCO": 3, "ABRIL": 4, "MAIO": 5, "JUNHO": 6, "JULHO": 7,
	"AGOSTO": 8, "SETEMBRO": 9, "OUTUBRO": 10, "NOVEMBRO": 11, "DEZEMBRO": 12}

// LerDinheiro lê "1.234,56", "R$ 1.234,56" ou "1.234,56-" (o sinal do desconto) como decimal, sempre positivo: o que
// diz se é provento ou desconto é a coluna.
func LerDinheiro(s string) (tipos.Decimal, bool) {
	s = strings.TrimSpace(s)
	if !reDinheiro.MatchString(s) {
		return tipos.Decimal{}, false
	}
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "R$"), "-"))
	d := decimal.RequireFromString(strings.ReplaceAll(strings.ReplaceAll(s, ".", ""), ",", "."))
	return tipos.DeDecimal(d), true
}

// normal é o texto como as regras comparam: maiúsculas, sem acento, só letras e números (ADR 0030).
func normal(s string) string { return texto.ParaPalavras(s) }

// comeca diz se o texto normalizado começa por algum dos rótulos.
func comeca(s string, rotulos ...string) bool {
	n := normal(s)
	for _, r := range rotulos {
		if strings.HasPrefix(n, r) {
			return true
		}
	}
	return false
}

// colunas são as posições (o centro) dos títulos da tabela.
type colunas struct {
	referencia, vencimentos, descontos Pontos
	temReferencia                      bool
}

// cabecalhoDaTabela acha a linha com "Vencimentos" (ou "Proventos", "Créditos") e "Descontos" (ou "Débitos").
func cabecalhoDaTabela(linhas []Linha) (int, colunas, bool) {
	for i, l := range linhas {
		var c colunas
		achouV, achouD := false, false
		for _, cel := range l.Celulas {
			switch n := normal(cel.Texto); {
			case strings.HasPrefix(n, "VENCIMENTO") || strings.HasPrefix(n, "PROVENTO") || n == "CREDITOS":
				c.vencimentos, achouV = cel.Centro(), true
			case strings.HasPrefix(n, "DESCONTO") || n == "DEBITOS":
				c.descontos, achouD = cel.Centro(), true
			case strings.HasPrefix(n, "REF") || n == "QTDE" || n == "QUANTIDADE":
				c.referencia, c.temReferencia = cel.Centro(), true
			}
		}
		if achouV && achouD {
			return i, c, true
		}
	}
	return 0, colunas{}, false
}

// fimDaTabela: a tabela acaba nos totais ou no líquido.
func fimDaTabela(l Linha) bool {
	return comeca(l.Texto(), "TOTAL", "TOTAIS", "LIQUIDO", "VALOR LIQUIDO")
}

// perto devolve qual das posições está mais perto de x (0 = a primeira).
func perto(x Pontos, posicoes ...Pontos) int {
	melhor, dist := 0, math.Inf(1)
	for i, p := range posicoes {
		if d := math.Abs(x - p); d < dist {
			melhor, dist = i, d
		}
	}
	return melhor
}

// Interpretar lê o holerite das linhas da primeira página que tem a tabela. O PDF com duas vias usa só a primeira.
func Interpretar(paginas [][]Linha) (Lido, error) {
	var linhas []Linha
	inicio, cols, achou := 0, colunas{}, false
	for _, p := range paginas {
		if inicio, cols, achou = cabecalhoDaTabela(p); achou {
			linhas = p
			break
		}
	}
	if !achou {
		return Lido{}, ErrSemTabela
	}
	lido := Lido{Rubricas: []Rubrica{}, Provas: []Prova{}}
	lerCabecalho(&lido, linhas[:inicio])
	fim := len(linhas)
	for i := inicio + 1; i < len(linhas); i++ {
		if fimDaTabela(linhas[i]) || ehRodapeSemTotais(linhas[i]) {
			fim = i
			break
		}
		if r, ok := lerRubrica(linhas[i], cols); ok {
			lido.Rubricas = append(lido.Rubricas, r)
		}
	}
	lerRodape(&lido, ateAProximaVia(linhas[fim:]), cols)
	provar(&lido)
	return lido, nil
}

// ateAProximaVia corta o rodapé onde começa a segunda via (o PDF com as duas vias na mesma página): na próxima linha
// com um CNPJ ou com o título da tabela.
func ateAProximaVia(linhas []Linha) []Linha {
	for i, l := range linhas {
		if _, _, ok := cabecalhoDaTabela([]Linha{l}); ok || reCnpj.MatchString(l.Texto()) {
			return linhas[:i]
		}
	}
	return linhas
}

// lerCabecalho acha o CNPJ, o empregador e a competência antes da tabela.
func lerCabecalho(lido *Lido, linhas []Linha) {
	for i, l := range linhas {
		t := l.Texto()
		if m := reCnpj.FindString(t); m != "" && lido.Cnpj == nil {
			digitos := strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, m)
			lido.Cnpj = &digitos
			lido.Empregador = maisLetras(l)
			if lido.Empregador == "" && i > 0 {
				lido.Empregador = maisLetras(linhas[i-1])
			}
		}
		if lido.Mes == nil {
			lido.Mes = competencia(t)
		}
	}
}

// palavrasDeTitulo: a célula com uma delas é o título do documento, não o empregador.
var palavrasDeTitulo = []string{"RECIBO", "DEMONSTRATIVO", "HOLERITE", "CONTRACHEQUE", "FOLHA", "PAGAMENTO", "COMPETENCIA",
	"REFERENTE", "MENSAL", "PERIODO", "INSCRICAO"}

// rotuloDoEmpregador sai da frente do nome ("Empregador: ACME LTDA"); sem os dois pontos, é parte do nome ("EMPRESA X").
var rotuloDoEmpregador = regexp.MustCompile(`(?i)^\s*(empregador|empresa|raz[ãa]o social)\s*:\s*`)

// maisLetras é a célula com mais letras da linha, sem o CNPJ, os rótulos, o título e o mês: o nome do empregador.
func maisLetras(l Linha) string {
	melhor, n := "", 0
	for _, c := range l.Celulas {
		t := reRotuloCnpj.ReplaceAllString(reCnpj.ReplaceAllString(c.Texto, ""), " ")
		t = strings.TrimSpace(rotuloDoEmpregador.ReplaceAllString(strings.TrimSpace(t), ""))
		if competencia(t) != nil || texto.TemPalavra(normal(t), palavrasDeTitulo...) {
			continue
		}
		if letras := len(strings.ReplaceAll(normal(t), " ", "")); letras > n {
			melhor, n = t, letras
		}
	}
	return melhor
}

// competencia acha o mês do holerite: "Outubro/2026", "outubro de 2026" ou "10/2026" (não o "01/10/2026" de uma data).
func competencia(t string) *tipos.Data {
	if m := reMesNome.FindStringSubmatch(t); m != nil {
		ano, _ := strconv.Atoi(m[2])
		d := tipos.NovaData(ano, nomesDosMeses[normal(m[1])], 1)
		return &d
	}
	if m := reMesNumero.FindStringSubmatch(t); m != nil {
		mes, _ := strconv.Atoi(m[1])
		ano, _ := strconv.Atoi(m[2])
		d := tipos.NovaData(ano, time.Month(mes), 1)
		return &d
	}
	return nil
}

// lerRubrica lê uma linha da tabela: o código, a descrição, a referência e o valor, provento ou desconto pela coluna
// mais perto. Com a coluna de referência, o número que cai nela é a referência ("30,00" dias, "12,02" %).
func lerRubrica(l Linha, cols colunas) (Rubrica, bool) {
	r := Rubrica{Trecho: l.Texto()}
	var textos []string
	posicoes := []Pontos{cols.vencimentos, cols.descontos}
	if cols.temReferencia {
		posicoes = append(posicoes, cols.referencia)
	}
	achou := false
	for i, c := range l.Celulas {
		v, ehDinheiro := LerDinheiro(c.Texto)
		switch {
		case i == 0 && reCodigo.MatchString(c.Texto):
			r.Codigo = c.Texto
		case ehDinheiro:
			switch perto(c.Centro(), posicoes...) {
			case 0:
				r.Valor, r.Provento, achou = v, true, true
			case 1:
				r.Valor, r.Provento, achou = v, false, true
			default:
				r.Referencia = c.Texto
			}
		case reReferencia.MatchString(c.Texto):
			r.Referencia = c.Texto
		default:
			textos = append(textos, c.Texto)
		}
	}
	r.Descricao = strings.Join(textos, " ")
	if r.Descricao == "" || !achou || r.Valor.Decimal().IsZero() {
		return Rubrica{}, false
	}
	return r, true
}

// Campos do rodapé e dos totais, e os rótulos de cada um (normalizados).
const (
	semCampo = iota
	campoVencimentos
	campoDescontos
	campoLiquido
	campoSalario
	campoBaseInss
	campoBaseFgts
	campoFgts
	campoBaseIrrf
)

var rotulos = []struct {
	campo   int
	rotulos []string
}{
	{campoVencimentos, []string{"TOTAL DE VENCIMENTOS", "TOTAL VENCIMENTOS", "TOTAL DE PROVENTOS", "TOTAL PROVENTOS", "TOTAL DE CREDITOS", "TOTAL CREDITOS"}},
	{campoDescontos, []string{"TOTAL DE DESCONTOS", "TOTAL DESCONTOS", "TOTAL DE DEBITOS", "TOTAL DEBITOS"}},
	{campoLiquido, []string{"VALOR LIQUIDO", "LIQUIDO A RECEBER", "LIQUIDO"}},
	{campoSalario, []string{"SALARIO BASE", "SAL BASE"}},
	{campoBaseInss, []string{"BASE INSS", "BASE CALC INSS", "BASE DE CALCULO INSS", "BASE CALCULO INSS", "SAL CONTR INSS", "SALARIO CONTRIBUICAO INSS"}},
	{campoBaseFgts, []string{"BASE FGTS", "BASE CALC FGTS", "BASE DE CALCULO FGTS", "BASE CALCULO FGTS"}},
	{campoFgts, []string{"FGTS DO MES", "FGTS MES", "F G T S DO MES", "FGTS"}},
	{campoBaseIrrf, []string{"BASE IRRF", "BASE CALC IRRF", "BASE DE CALCULO IRRF", "BASE CALCULO IRRF"}},
}

// rotuloDoRodape diz qual campo o texto rotula: o rótulo inteiro, ou o começo dele seguido do valor ("Base INSS: 4.130,91").
func rotuloDoRodape(t string) int {
	n := normal(t)
	for _, r := range rotulos {
		for _, rotulo := range r.rotulos {
			if n == rotulo || strings.HasPrefix(n, rotulo+" ") {
				return r.campo
			}
		}
	}
	return semCampo
}

// ehRodapeSemTotais: o holerite sem totais vai da tabela direto às bases ("Base INSS", "FGTS"). A linha com código é
// rubrica, e o "Salário base" pode ser o nome de uma.
func ehRodapeSemTotais(l Linha) bool {
	if reCodigo.MatchString(l.Celulas[0].Texto) {
		return false
	}
	for _, c := range l.Celulas {
		switch rotuloDoRodape(c.Texto) {
		case campoBaseInss, campoBaseFgts, campoFgts, campoBaseIrrf:
			return true
		}
	}
	return false
}

// campo é o lugar do campo no Lido.
func (lido *Lido) campo(c int) **tipos.Decimal {
	return map[int]**tipos.Decimal{
		campoVencimentos: &lido.TotalVencimentos, campoDescontos: &lido.TotalDescontos, campoLiquido: &lido.Liquido,
		campoSalario: &lido.SalarioBase, campoBaseInss: &lido.BaseInss, campoBaseFgts: &lido.BaseFgts,
		campoFgts: &lido.FgtsDoMes, campoBaseIrrf: &lido.BaseIrrf,
	}[c]
}

// lerRodape acha os totais, o líquido e as bases depois da tabela. Cada campo fica com o primeiro valor achado.
func lerRodape(lido *Lido, linhas []Linha, cols colunas) {
	for i, l := range linhas {
		// "Totais" com os dois números, sem rótulo para cada um: o de vencimentos e o de descontos, pela coluna.
		if comeca(l.Texto(), "TOTAIS") || normal(l.Celulas[0].Texto) == "TOTAL" {
			for _, c := range l.Celulas {
				if v, ok := LerDinheiro(c.Texto); ok {
					alvo := lido.campo(campoVencimentos)
					if perto(c.Centro(), cols.vencimentos, cols.descontos) == 1 {
						alvo = lido.campo(campoDescontos)
					}
					if *alvo == nil {
						*alvo = &v
					}
				}
			}
		}
		for j, c := range l.Celulas {
			alvo := lido.campo(rotuloDoRodape(c.Texto))
			if alvo == nil || *alvo != nil {
				continue
			}
			if v, ok := valorDoRotulo(linhas, i, j); ok {
				*alvo = &v
			}
		}
	}
}

// valorDoRotulo: o dinheiro na mesma célula ("Líquido 5.053,13"), à direita na mesma linha (antes do próximo rótulo)
// ou na linha de baixo, embaixo do rótulo.
func valorDoRotulo(linhas []Linha, i, j int) (tipos.Decimal, bool) {
	rotulo := linhas[i].Celulas[j]
	partes := strings.Fields(rotulo.Texto)
	if v, ok := LerDinheiro(partes[len(partes)-1]); ok {
		return v, true
	}
	for _, c := range linhas[i].Celulas[j+1:] {
		if v, ok := LerDinheiro(c.Texto); ok {
			return v, true
		}
		if n := normal(c.Texto); n != "" && n != "R" && !reReferencia.MatchString(c.Texto) {
			break // outro rótulo: o valor deste não está nesta linha (seta, "R$" e a faixa ficam no caminho)
		}
	}
	if i+1 < len(linhas) {
		for _, c := range linhas[i+1].Celulas {
			if v, ok := LerDinheiro(c.Texto); ok && c.X1 >= rotulo.X0-10 && c.X0 <= rotulo.X1+10 {
				return v, true
			}
		}
	}
	return tipos.Decimal{}, false
}

// provar confere a soma das rubricas com os totais impressos e o líquido.
func provar(lido *Lido) {
	proventos, descontos := decimal.Zero, decimal.Zero
	for _, r := range lido.Rubricas {
		if r.Provento {
			proventos = proventos.Add(r.Valor.Decimal())
		} else {
			descontos = descontos.Add(r.Valor.Decimal())
		}
	}
	prova := func(descricao string, impresso *tipos.Decimal, encontrado decimal.Decimal) {
		if impresso == nil {
			return
		}
		d := encontrado.Sub(impresso.Decimal())
		lido.Provas = append(lido.Provas, Prova{Descricao: descricao, Impresso: *impresso, Encontrado: tipos.DeDecimal(encontrado),
			Diferenca: tipos.DeDecimal(d), Bateu: d.IsZero()})
	}
	prova("Total de vencimentos", lido.TotalVencimentos, proventos)
	prova("Total de descontos", lido.TotalDescontos, descontos)
	prova("Líquido", lido.Liquido, proventos.Sub(descontos))
}
