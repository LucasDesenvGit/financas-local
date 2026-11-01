package contasagendadas

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cobrancas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sugestoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// As constantes do SugestaoContaAgendadaService (#62, #158, #205, #241).
const (
	diasHistorico       = 210 // janela das mensais (~7 meses)
	diasHistoricoAnual  = 400 // janela das anuais (um pouco mais de um ano, para ver duas cobranças)
	minimoDiasAnual     = 330 // entre duas cobranças de uma assinatura anual
	maximoDiasAnual     = 400
	minimoMeses         = 3  // meses distintos com pagamento para ser recorrente
	diasSemPagamento    = 45 // último pagamento há mais que isso: a conta provavelmente acabou
	maximoSugestoes     = 30
	folgaDiasParecida   = 3 // #205: dia de vencimento a até tantos dias do dia habitual
	maximoDeLancamentos = 20000
	tamanhoDaDescricao  = 150
	tamanhoDoTermo      = 100

	tipoContaCartao       = 3
	tipoContaInvestimento = 4
	tipoDespesa           = 1
)

var (
	variacaoCartao     = decimalFixo("0.05") // no cartão, assinatura tem valor fixo (±5%)
	variacaoValorFixo  = decimalFixo("0.15") // acima disso, "valor variável" (luz, gás)
	dois               = tipos.DecimalDeInteiro(2)
	formaDebitoAutomat = int64(2)
	formaCartao        = int64(relatorios.FormaCartaoCredito)
)

// SugestaoDto é o SugestaoContaAgendadaDto do C#, na ordem do record: um pagamento recorrente do extrato que ainda não
// está em Contas a pagar. ProximoVencimento nunca é passado (#62); VencimentoSemPagamento é o vencimento esperado que
// já passou sem o pagamento aparecer.
type SugestaoDto struct {
	Chave                  string        `json:"chave"`
	Descricao              string        `json:"descricao"`
	Valor                  tipos.Decimal `json:"valor"`
	ValorVariavel          bool          `json:"valorVariavel"`
	ValorMinimo            tipos.Decimal `json:"valorMinimo"`
	ValorMaximo            tipos.Decimal `json:"valorMaximo"`
	ProximoVencimento      tipos.Data    `json:"proximoVencimento"`
	DiaHabitual            int           `json:"diaHabitual"`
	Meses                  int           `json:"meses"`
	Pagamentos             int           `json:"pagamentos"`
	UltimoPagamento        tipos.Data    `json:"ultimoPagamento"`
	PrimeiroPagamento      tipos.Data    `json:"primeiroPagamento"`
	CategoriaId            *int64        `json:"categoriaId"`
	CategoriaNome          *string       `json:"categoriaNome"`
	ContaId                int64         `json:"contaId"`
	ContaNome              string        `json:"contaNome"`
	TermoBusca             string        `json:"termoBusca"`
	VencimentoSemPagamento *tipos.Data   `json:"vencimentoSemPagamento"`
	FormaCobranca          int64         `json:"formaCobranca"`
	TextoCobranca          *string       `json:"textoCobranca"`
	Recorrencia            int64         `json:"recorrencia"`
	ChaveExtrato           *string       `json:"chaveExtrato"`
	PareceSerId            *int64        `json:"pareceSerId"`
	PareceSerNome          *string       `json:"pareceSerNome"`
}

// lancamento é uma despesa do extrato, como a sugestão precisa dela.
type lancamento struct {
	data              tipos.Data
	descricao         string
	descricaoOriginal *string
	estabelecimento   *string
	valor             tipos.Decimal
	categoriaId       *int64
	categoriaNome     *string
	contaId           int64
	contaNome         string
	contaTipo         int64
	regraDescricaoId  *int64
}

// cadastrada é a conta a pagar como a comparação com as sugestões precisa dela (#205).
type cadastrada struct {
	id                int64
	descricao         string
	textoCobranca     *string
	chaveExtrato      *string
	ativa             bool
	valor             tipos.Decimal
	proximoVencimento tipos.Data
	recorrencia       int64
	contaId           *int64
}

// Sugerir é o SugerirAsync: os pagamentos que se repetem todo mês (ou todo ano) no extrato e ainda não estão em Contas
// a pagar. Só sugere: quem cadastra é a pessoa. O Open Finance não informa boletos futuros (DDA), então o próximo
// vencimento é estimado pelo dia habitual.
func (s *Servico) Sugerir(ctx context.Context, hoje tipos.Data) ([]SugestaoDto, error) {
	desde, desdeMensal := hoje.SomarDias(-diasHistoricoAnual), hoje.SomarDias(-diasHistorico)
	lancamentos, err := s.lerLancamentos(ctx, desde, hoje)
	if err != nil {
		return nil, err
	}
	regrasPorValor, err := cobrancas.RegrasPorValor(ctx, s.db)
	if err != nil {
		return nil, err
	}
	cadastradas, err := s.lerCadastradas(ctx)
	if err != nil {
		return nil, err
	}
	// #205: a chave gravada ao cadastrar pela sugestão (ou no "é a mesma") vale qualquer que seja o nome escolhido.
	jaCadastradas := map[string]bool{}
	var textosCadastrados []string
	nomesCadastrados := map[string]bool{}
	for _, c := range cadastradas {
		if k := cobrancas.ChaveDe(c.descricao); k != "" {
			jaCadastradas[k] = true
		}
		if c.chaveExtrato != nil && *c.chaveExtrato != "" {
			jaCadastradas[*c.chaveExtrato] = true
		}
		if t := normalizar(c.textoCobranca); t != "" {
			textosCadastrados = append(textosCadastrados, t)
		}
		nomesCadastrados[cobrancas.ChaveDaRegra(c.descricao)] = true
	}

	resultado := []SugestaoDto{}
	for _, g := range agrupar(lancamentos, regrasPorValor) {
		todos := g.itens // mais recente primeiro
		recente := todos[0]
		textoBanco := texto.ParaRegras(original(recente))
		if jaCadastradas[g.chave] {
			continue
		}
		if g.daRegra {
			if nomesCadastrados[g.chave] {
				continue
			}
		} else if jaCadastradas[cobrancas.ChaveDe(recente.descricao)] ||
			slices.ContainsFunc(textosCadastrados, func(t string) bool { return strings.Contains(textoBanco, t) }) {
			continue
		}

		// #158: cada conta ou cartão primeiro (a mesma marca pode aparecer em dois lugares, com valores diferentes);
		// juntas só se nenhuma passar sozinha, o que cobre a assinatura que trocou de cartão no meio do caminho.
		var achadas []SugestaoDto
		if porConta := agruparPorConta(todos); len(porConta) > 1 {
			for _, c := range porConta {
				if sug, ok := avaliar(g.chave+"#"+strconv.FormatInt(c[0].contaId, 10), g.chave, c, hoje, desdeMensal, g.daRegra); ok {
					achadas = append(achadas, sug)
				}
			}
		}
		if len(achadas) == 0 {
			if sug, ok := avaliar(g.chave, g.chave, todos, hoje, desdeMensal, g.daRegra); ok {
				achadas = append(achadas, sug)
			}
		}
		// #241: uma conta a pagar com o texto da cobrança mais geral (ex.: CONDOMINIO, que está nas duas) só é a
		// cadastrada de uma delas, a de valor parecido.
		if g.daRegra {
			achadas = slices.DeleteFunc(achadas, func(sug SugestaoDto) bool {
				return slices.ContainsFunc(cadastradas, func(c cadastrada) bool {
					t := normalizar(c.textoCobranca)
					return t != "" && (strings.Contains(g.chave, t) || strings.Contains(textoBanco, t)) && valorPerto(sug.Valor, c.valor)
				})
			})
		}
		resultado = append(resultado, achadas...)
	}

	resultado = semAsParecidas(resultado, cadastradas)
	slices.SortStableFunc(resultado, func(a, b SugestaoDto) int {
		if a.ProximoVencimento.Antes(b.ProximoVencimento) {
			return -1
		}
		if a.ProximoVencimento.Depois(b.ProximoVencimento) {
			return 1
		}
		return -a.Valor.Decimal().Cmp(b.Valor.Decimal())
	})
	return resultado[:min(len(resultado), maximoSugestoes)], nil
}

// decimalFixo é uma constante decimal do C# (0.05m), lida uma vez.
func decimalFixo(t string) tipos.Decimal {
	d, err := tipos.NovoDecimal(t)
	if err != nil {
		panic(err)
	}
	return d
}

func normalizar(t *string) string {
	if t == nil {
		return ""
	}
	return texto.ParaRegras(*t)
}

// original é o DescricaoOriginal ?? Descricao.
func original(l lancamento) string {
	if l.descricaoOriginal != nil {
		return *l.descricaoOriginal
	}
	return l.descricao
}

type grupo struct {
	chave   string
	daRegra bool
	itens   []lancamento
}

// agrupar é o GroupBy((Chave, DaRegra)) do C#, na ordem em que cada grupo aparece pela primeira vez. A compra parcelada
// no cartão que veio sem os campos de parcela ("PARC 3/10") não entra.
func agrupar(lancamentos []lancamento, regrasPorValor map[int64]bool) []grupo {
	var grupos []grupo
	indice := map[string]int{}
	for _, l := range lancamentos {
		if l.contaTipo == tipoContaCartao && parcelaNoTexto(original(l)) {
			continue
		}
		daRegra := l.regraDescricaoId != nil && regrasPorValor[*l.regraDescricaoId]
		var chave string
		switch {
		case daRegra:
			chave = cobrancas.ChaveDaRegra(l.descricao)
		case l.estabelecimento != nil:
			chave = cobrancas.ChaveDe(*l.estabelecimento)
		default:
			chave = cobrancas.ChaveDe(original(l))
		}
		if chave == "" {
			continue
		}
		k := strconv.FormatBool(daRegra) + "|" + chave
		i, ok := indice[k]
		if !ok {
			i = len(grupos)
			indice[k] = i
			grupos = append(grupos, grupo{chave: chave, daRegra: daRegra})
		}
		grupos[i].itens = append(grupos[i].itens, l)
	}
	return grupos
}

// agruparPorConta é o GroupBy(ContaId), na ordem da primeira aparição.
func agruparPorConta(todos []lancamento) [][]lancamento {
	var grupos [][]lancamento
	indice := map[int64]int{}
	for _, l := range todos {
		i, ok := indice[l.contaId]
		if !ok {
			i = len(grupos)
			indice[l.contaId] = i
			grupos = append(grupos, nil)
		}
		grupos[i] = append(grupos[i], l)
	}
	return grupos
}

// avaliar é o Avaliar: a sugestão de um grupo de lançamentos (mais recente primeiro), ou false se ele não for conta
// recorrente. chave identifica a sugestão na tela; chaveTexto dá o texto da cobrança. daRegra (#241): o grupo é o
// nome de uma regra com faixa de valor, e o texto da cobrança é ele inteiro.
func avaliar(chave, chaveTexto string, todos []lancamento, hoje, desdeMensal tipos.Data, daRegra bool) (SugestaoDto, bool) {
	recente := todos[0]
	conta := maisFrequente(todos)
	noCartao := conta.contaTipo == tipoContaCartao

	recorrencia := int64(relatorios.RecorrenciaMensal)
	var itens []lancamento
	for _, l := range todos {
		if !l.data.Antes(desdeMensal) {
			itens = append(itens, l)
		}
	}
	if !ehMensal(datas(itens), hoje) {
		// #158: assinatura anual, uma cobrança por ano (o histórico importado precisa ter as duas).
		if !ehAnual(datas(todos)) {
			return SugestaoDto{}, false
		}
		recorrencia = relatorios.RecorrenciaAnual
		itens = todos
	}

	valores := make([]tipos.Decimal, len(itens))
	for i, l := range itens {
		valores[i] = tipos.DeDecimal(l.valor.Decimal().Neg())
	}
	ultimos := valores[:min(3, len(valores))]
	valor := mediana(ultimos)
	minimo, maximo := extremos(valores)
	variavel := valor.Decimal().IsPositive() && maiorQue(tipos.Subtrair(maximo, minimo), valor, variacaoValorFixo)
	// No cartão, só as últimas cobranças contam para o valor fixo: um aumento de preço antigo não tira a assinatura.
	if menor, maior := extremos(ultimos); noCartao && valor.Decimal().IsPositive() && maiorQue(tipos.Subtrair(maior, menor), valor, variacaoCartao) {
		return SugestaoDto{}, false
	}

	dias := make([]tipos.Decimal, len(itens))
	for i, l := range itens {
		dias[i] = tipos.DecimalDeInteiro(int64(l.data.Dia()))
	}
	dia := int(mediana(dias).Decimal().IntPart()) // o (int) do C# corta as casas

	var esperado, proximo tipos.Data
	if recorrencia == relatorios.RecorrenciaAnual {
		esperado = recente.data.SomarAnos(1)
		proximo = esperado
		for proximo.Antes(hoje) {
			proximo = proximo.SomarAnos(1)
		}
	} else {
		// Vencimento esperado no mês seguinte ao último pagamento. Se já passou sem o pagamento aparecer, a sugestão
		// avisa e propõe o próximo que ainda vai vencer: cadastrar não pode criar uma conta já vencida (#62).
		esperado = noDia(inicioDoMes(recente.data).SomarMeses(1), dia)
		proximo = esperado
		for proximo.Antes(hoje) {
			proximo = noDia(inicioDoMes(proximo).SomarMeses(1), dia)
		}
	}
	var semPagamento *tipos.Data
	if esperado.Antes(hoje) {
		semPagamento = &esperado
	}

	categoriaId, categoriaNome := categoriaMaisFrequente(itens)
	descricao := strings.TrimFunc(recente.descricao, unicode.IsSpace)
	var termo string
	if daRegra {
		termo = cortar(descricao, tamanhoDoTermo)
	} else {
		termo = termoBusca(chaveTexto, recente.estabelecimento, descricao)
	}
	forma := int64(cobrancas.FormaAMao)
	if noCartao {
		forma = formaCartao
	} else if slices.ContainsFunc(itens, func(l lancamento) bool { return ehDebitoAutomatico(original(l)) }) {
		forma = formaDebitoAutomat
	}
	// O texto que liga a conta às cobranças: a palavra mais longa da chave (ex.: "NETFLIX").
	textoCobranca := chaveTexto
	if daRegra {
		textoCobranca = cortar(chaveTexto, tamanhoDoTermo)
	} else if p, ok := maisLonga(sugestoes.Palavras(chaveTexto)); ok {
		textoCobranca = p
	}

	meses := map[int]bool{}
	for _, l := range itens {
		meses[l.data.Ano()*12+int(l.data.Mes())-1] = true
	}
	chaveExtrato := chaveTexto
	return SugestaoDto{
		Chave: chave, Descricao: cortar(descricao, tamanhoDaDescricao),
		Valor: tipos.Arredondar(valor, 2), ValorVariavel: variavel,
		ValorMinimo: tipos.Arredondar(minimo, 2), ValorMaximo: tipos.Arredondar(maximo, 2),
		ProximoVencimento: proximo, DiaHabitual: dia, Meses: len(meses), Pagamentos: len(itens),
		UltimoPagamento: recente.data, PrimeiroPagamento: itens[len(itens)-1].data,
		CategoriaId: categoriaId, CategoriaNome: categoriaNome, ContaId: conta.contaId, ContaNome: conta.contaNome,
		TermoBusca: termo, VencimentoSemPagamento: semPagamento, FormaCobranca: forma, TextoCobranca: &textoCobranca,
		Recorrencia: recorrencia, ChaveExtrato: &chaveExtrato,
	}, true
}

// maisFrequente é o todos.GroupBy(ContaId).OrderByDescending(Count).First().First(): o lançamento mais recente da conta
// com mais lançamentos (no empate, a que aparece primeiro).
func maisFrequente(todos []lancamento) lancamento {
	grupos := agruparPorConta(todos)
	melhor := grupos[0]
	for _, g := range grupos[1:] {
		if len(g) > len(melhor) {
			melhor = g
		}
	}
	return melhor[0]
}

// categoriaMaisFrequente é a categoria que mais aparece entre os lançamentos com categoria (no empate, a primeira).
func categoriaMaisFrequente(itens []lancamento) (*int64, *string) {
	contagem := map[int64]int{}
	var ordem []lancamento
	for _, l := range itens {
		if l.categoriaId == nil {
			continue
		}
		if contagem[*l.categoriaId] == 0 {
			ordem = append(ordem, l)
		}
		contagem[*l.categoriaId]++
	}
	if len(ordem) == 0 {
		return nil, nil
	}
	melhor := ordem[0]
	for _, l := range ordem[1:] {
		if contagem[*l.categoriaId] > contagem[*melhor.categoriaId] {
			melhor = l
		}
	}
	return melhor.categoriaId, melhor.categoriaNome
}

func datas(lista []lancamento) []tipos.Data {
	r := make([]tipos.Data, len(lista))
	for i, l := range lista {
		r[i] = l.data
	}
	return r
}

// ehMensal: pago recentemente, em pelo menos minimoMeses meses distintos, sem ser gasto frequente (mais de ~1,5 por
// mês) e cobrindo pelo menos 60% dos meses do intervalo. datas: mais recente primeiro. As frações do C# (decimal) viram
// contas inteiras equivalentes: n > m × 1,5 é 2n > 3m; m / intervalo ≥ 0,6 é 10m ≥ 6 × intervalo.
func ehMensal(datas []tipos.Data, hoje tipos.Data) bool {
	if len(datas) == 0 || datas[0].Antes(hoje.SomarDias(-diasSemPagamento)) {
		return false
	}
	meses := map[int]bool{}
	menor, maior := 0, 0
	for i, d := range datas {
		m := d.Ano()*12 + int(d.Mes()) - 1
		meses[m] = true
		if i == 0 || m < menor {
			menor = m
		}
		if i == 0 || m > maior {
			maior = m
		}
	}
	if len(meses) < minimoMeses || 2*len(datas) > 3*len(meses) {
		return false
	}
	return 10*len(meses) >= 6*(maior-menor+1)
}

// ehAnual (#158): 2 ou 3 cobranças, cada uma de 330 a 400 dias depois da anterior (mais recente primeiro).
func ehAnual(datas []tipos.Data) bool {
	if len(datas) < 2 || len(datas) > 3 {
		return false
	}
	for i := range len(datas) - 1 {
		dias := datas[i].NumeroDoDia() - datas[i+1].NumeroDoDia()
		if dias < minimoDiasAnual || dias > maximoDiasAnual {
			return false
		}
	}
	return true
}

var soLetrasENumeros = regexp.MustCompile(`[^A-Z0-9]+`)

// ehDebitoAutomatico (#158): "DEB AUT", "DEBITO AUTOMATICO", "DEB.AUT.": a conta é debitada sozinha.
func ehDebitoAutomatico(descricao string) bool {
	t := " " + soLetrasENumeros.ReplaceAllString(texto.ParaRegras(descricao), " ") + " "
	return strings.Contains(t, " DEB AUT") || strings.Contains(t, " DEBITO AUT")
}

// parcela é o `\b\d{1,2}\s*/\s*\d{1,2}\b|\bPARC(ELA)?\b` do C# (IgnoreCase). No .NET o \d é qualquer dígito Unicode e
// o \s inclui os separadores Unicode (o espaço não separável dos extratos): as classes abaixo são as mesmas. O \b do
// .NET também é Unicode, e o do Go só ASCII: as bordas são conferidas à mão, em temBorda.
var parcela = regexp.MustCompile(`(?i)\p{Nd}{1,2}[\t\n\v\f\r\x{85}\p{Z}]*/[\t\n\v\f\r\x{85}\p{Z}]*\p{Nd}{1,2}|PARC(?:ELA)?`)

// parcelaNoTexto (#158): compra parcelada no cartão que veio sem os campos de parcela ("PARC 3/10", "3/10").
func parcelaNoTexto(descricao string) bool {
	for _, m := range parcela.FindAllStringIndex(descricao, -1) {
		// O .NET tenta em cada posição; o Go devolve o casamento mais à esquerda e mais curto possível dentro das
		// alternativas. Aqui basta que algum trecho que casa tenha borda dos dois lados.
		for inicio := m[0]; inicio < m[1]; inicio++ {
			for fim := m[1]; fim > inicio; fim-- {
				trecho := descricao[inicio:fim]
				if parcela.FindString(trecho) == trecho && temBorda(descricao, inicio) && temBorda(descricao, fim) {
					return true
				}
			}
		}
	}
	return false
}

// palavraNET é o \w do .NET: letra, dígito, marca, conectivo ou o "_".
func palavraNET(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Pc, r)
}

// temBorda é o \b na posição i: um lado é caractere de palavra e o outro não.
func temBorda(s string, i int) bool {
	var antes, depois bool
	if i > 0 {
		r := []rune(s[:i])
		antes = palavraNET(r[len(r)-1])
	}
	if i < len(s) {
		for _, r := range s[i:] {
			depois = palavraNET(r)
			break
		}
	}
	return antes != depois
}

// noDia é o dia habitual dentro do mês (dia 31 vira o último dia em meses mais curtos).
func noDia(inicioDoMes tipos.Data, dia int) tipos.Data {
	ultimo := inicioDoMes.SomarMeses(1).SomarDias(-1).Dia()
	return inicioDoMes.SomarDias(min(dia, ultimo) - 1)
}

func inicioDoMes(d tipos.Data) tipos.Data { return tipos.NovaData(d.Ano(), d.Mes(), 1) }

// termoBusca é a palavra para o link "ver pagamentos" em Transações. A busca de lá diferencia maiúsculas e acentos,
// então a palavra é tirada do texto como está gravado (a que contém a mais longa da chave), não da chave normalizada.
func termoBusca(chave string, estabelecimento *string, descricao string) string {
	if alvo, ok := maisLonga(sugestoes.Palavras(chave)); ok {
		for _, t := range []*string{estabelecimento, &descricao} {
			if t == nil {
				continue
			}
			for _, p := range strings.Split(*t, " ") {
				if p != "" && strings.Contains(texto.ParaRegras(p), alvo) {
					return strings.Trim(p, ",.-*/")
				}
			}
		}
	}
	return cortar(descricao, tamanhoDoTermo)
}

// maisLonga é o OrderByDescending(Length).FirstOrDefault(): a primeira das mais longas, em chars UTF-16.
func maisLonga(palavras []string) (string, bool) {
	if len(palavras) == 0 {
		return "", false
	}
	melhor := palavras[0]
	for _, p := range palavras[1:] {
		if texto.TamanhoUtf16(p) > texto.TamanhoUtf16(melhor) {
			melhor = p
		}
	}
	return melhor, true
}

// cortar é o s[..Math.Min(s.Length, n)] do C#, em chars UTF-16.
func cortar(s string, n int) string {
	if texto.TamanhoUtf16(s) <= n {
		return s
	}
	return string(utf16.Decode(utf16.Encode([]rune(s))[:n]))
}

// mediana dos valores: no número par, a média dos dois do meio.
func mediana(valores []tipos.Decimal) tipos.Decimal {
	ordenados := slices.Clone(valores)
	slices.SortStableFunc(ordenados, func(a, b tipos.Decimal) int { return a.Decimal().Cmp(b.Decimal()) })
	meio := len(ordenados) / 2
	if len(ordenados)%2 == 1 {
		return ordenados[meio]
	}
	m, _ := tipos.Dividir(tipos.Somar(ordenados[meio-1], ordenados[meio]), dois)
	return m
}

func extremos(valores []tipos.Decimal) (menor, maior tipos.Decimal) {
	menor, maior = valores[0], valores[0]
	for _, v := range valores[1:] {
		menor, maior = tipos.Menor(menor, v), tipos.Maior(maior, v)
	}
	return menor, maior
}

// maiorQue é o diferenca / base > limite do C#, com a divisão do decimal (base nunca é zero aqui).
func maiorQue(diferenca, base, limite tipos.Decimal) bool {
	q, err := tipos.Dividir(diferenca, base)
	return err == nil && q.Decimal().GreaterThan(limite.Decimal())
}

// valorPerto (#205): valores a até 15% um do outro.
func valorPerto(a, b tipos.Decimal) bool {
	referencia := tipos.Maior(a, b)
	limite := tipos.Multiplicar(referencia, variacaoValorFixo)
	return referencia.Decimal().IsPositive() && !tipos.Subtrair(a, b).Decimal().Abs().GreaterThan(limite.Decimal())
}

// semAsParecidas (#205): contas a pagar cadastradas à mão (ou antes da ChaveExtrato) com outro nome. Uma conta a pagar
// ativa, sem ligação, com valor parecido, dia de vencimento a até 3 dias e a mesma conta é a sugestão:
//   - certeza (some): ela é a única candidata, tem a mesma conta, não tem texto da cobrança e não serve para outra;
//   - dúvida (fica, com "parece ser"): mais de uma candidata, a mesma conta a pagar serve para mais de uma sugestão, a
//     conta a pagar não diz a conta ou já tem outro texto da cobrança.
func semAsParecidas(lista []SugestaoDto, cadastradas []cadastrada) []SugestaoDto {
	var livres []cadastrada
	for _, c := range cadastradas {
		if c.ativa && c.chaveExtrato == nil {
			livres = append(livres, c)
		}
	}
	if len(livres) == 0 || len(lista) == 0 {
		return lista
	}
	candidatas := make([][]cadastrada, len(lista))
	usos := map[int64]int{}
	for i, s := range lista {
		for _, c := range livres {
			if parecida(s, c) {
				candidatas[i] = append(candidatas[i], c)
				usos[c.id]++
			}
		}
	}
	resultado := []SugestaoDto{}
	for i, s := range lista {
		opcoes := candidatas[i]
		if len(opcoes) == 0 {
			resultado = append(resultado, s)
			continue
		}
		if len(opcoes) == 1 && certeza(s, opcoes[0]) && usos[opcoes[0].id] == 1 {
			continue
		}
		ordenadas := slices.Clone(opcoes)
		slices.SortStableFunc(ordenadas, func(a, b cadastrada) int {
			if ca, cb := certeza(s, a), certeza(s, b); ca != cb {
				if ca {
					return -1
				}
				return 1
			}
			da := tipos.Subtrair(a.valor, s.Valor).Decimal().Abs()
			db := tipos.Subtrair(b.valor, s.Valor).Decimal().Abs()
			if c := da.Cmp(db); c != 0 {
				return c
			}
			return distanciaDias(a, s) - distanciaDias(b, s)
		})
		melhor := ordenadas[0]
		s.PareceSerId, s.PareceSerNome = &melhor.id, &melhor.descricao
		resultado = append(resultado, s)
	}
	return resultado
}

// parecida (#205): valor, dia e conta parecidos (a conta a pagar sem conta serve para qualquer uma, mas só com dúvida).
func parecida(s SugestaoDto, c cadastrada) bool {
	if c.contaId != nil && *c.contaId != s.ContaId {
		return false
	}
	// Anual com anual; a mensal pode ter sido cadastrada como mensal ou única.
	if (s.Recorrencia == relatorios.RecorrenciaAnual) != (c.recorrencia == relatorios.RecorrenciaAnual) {
		return false
	}
	naFaixa := s.ValorVariavel && !c.valor.Decimal().LessThan(s.ValorMinimo.Decimal()) &&
		!c.valor.Decimal().GreaterThan(s.ValorMaximo.Decimal())
	return (valorPerto(s.Valor, c.valor) || naFaixa) && distanciaDias(c, s) <= folgaDiasParecida
}

// certeza (#205): a mesma conta informada e nenhum texto da cobrança que a ligue a outra coisa.
func certeza(s SugestaoDto, c cadastrada) bool {
	return c.contaId != nil && *c.contaId == s.ContaId && (c.textoCobranca == nil || strings.TrimFunc(*c.textoCobranca, unicode.IsSpace) == "")
}

// distanciaDias (#205): dias entre o vencimento da conta a pagar e o da sugestão. Mensal: pelo dia do mês, dando a
// volta (dia 30 e dia 1 estão a 2 dias). Anual: pelo dia do ano.
func distanciaDias(c cadastrada, s SugestaoDto) int {
	a, b, ciclo := c.proximoVencimento.Dia(), s.DiaHabitual, 31
	if s.Recorrencia == relatorios.RecorrenciaAnual {
		a, b, ciclo = diaDoAno(c.proximoVencimento), diaDoAno(s.ProximoVencimento), 365
	}
	d := a - b
	if d < 0 {
		d = -d
	}
	return min(d, ciclo-d)
}

func diaDoAno(d tipos.Data) int {
	return d.NumeroDoDia() - tipos.NovaData(d.Ano(), 1, 1).NumeroDoDia() + 1
}

func (s *Servico) lerLancamentos(ctx context.Context, desde, hoje tipos.Data) ([]lancamento, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT t."Data", t."Descricao", t."DescricaoOriginal", t."Estabelecimento",
		t."Valor", t."CategoriaId", c."Nome", t."ContaId", co."Nome", co."Tipo", t."RegraDescricaoId"
		FROM "Transacao" AS t
		INNER JOIN "Conta" AS co ON t."ContaId" = co."Id"
		LEFT JOIN "Categoria" AS c ON t."CategoriaId" = c."Id"
		WHERE t."Data" >= ? AND t."Data" <= ? AND t."Tipo" = ? AND t."TransferenciaGrupo" IS NULL
			AND NOT t."IgnorarRelatorios" AND t."ParcelaTotal" IS NULL AND co."Tipo" <> ? AND NOT co."Arquivada"
		ORDER BY t."Data" DESC`, desde, hoje, tipoDespesa, tipoContaInvestimento)
	if err != nil {
		return nil, fmt.Errorf("ler os lançamentos das sugestões: %w", err)
	}
	defer linhas.Close()
	var lista []lancamento
	for linhas.Next() {
		var l lancamento
		if err := linhas.Scan(&l.data, &l.descricao, &l.descricaoOriginal, &l.estabelecimento, &l.valor, &l.categoriaId,
			&l.categoriaNome, &l.contaId, &l.contaNome, &l.contaTipo, &l.regraDescricaoId); err != nil {
			return nil, fmt.Errorf("ler lançamento das sugestões: %w", err)
		}
		// O valor é texto no banco: o "menor que zero" é conferido aqui, com decimal (no C#, no SQL, antes do Take).
		if l.valor.Decimal().IsNegative() && len(lista) < maximoDeLancamentos {
			lista = append(lista, l)
		}
	}
	return lista, linhas.Err()
}

func (s *Servico) lerCadastradas(ctx context.Context) ([]cadastrada, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Descricao", "TextoCobranca", "ChaveExtrato", "Ativa", "Valor",
		"ProximoVencimento", "Recorrencia", "ContaId" FROM "ContaAgendada" ORDER BY "Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler as contas a pagar: %w", err)
	}
	defer linhas.Close()
	var lista []cadastrada
	for linhas.Next() {
		var c cadastrada
		if err := linhas.Scan(&c.id, &c.descricao, &c.textoCobranca, &c.chaveExtrato, &c.ativa, &c.valor,
			&c.proximoVencimento, &c.recorrencia, &c.contaId); err != nil {
			return nil, fmt.Errorf("ler conta a pagar: %w", err)
		}
		lista = append(lista, c)
	}
	return lista, linhas.Err()
}
