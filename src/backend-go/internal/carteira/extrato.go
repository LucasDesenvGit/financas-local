package carteira

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

const (
	// anosDoExtrato é até onde o extrato é lido.
	anosDoExtrato = 10
	// diasDeDuplicata: a operação de mesmo ativo e tipo até tantos dias do lançamento faz dele uma possível duplicata.
	diasDeDuplicata = 3
	// origemExtrato é o OrigemOperacao.Extrato do C#.
	origemExtrato = 3
	// maiorQuantidadeNoTexto: número maior que isso, no texto, não é quantidade.
	maiorQuantidadeNoTexto = 10000000
)

var (
	// folgaDoValor: a diferença aceita entre quantidade × preço e o valor do lançamento (as taxas).
	folgaDoValor = mustDecimal("0.03")
	// centavo: o preço igual ao valor do lançamento é o próprio valor, e não um preço.
	centavo = mustDecimal("0.01")
	// 30% em volta do fechamento do dia: o preço plausível.
	fatorMinimo, fatorMaximo = mustDecimal("0.7"), mustDecimal("1.3")

	regexTicker = regexp.MustCompile(`\b([A-Z]{4}(?:11|12|13|31|32|33|34|35|39|3|4|5|6|7|8))F?\b`)
	regexData   = regexp.MustCompile(`\b\d{1,2}/\d{1,2}(/\d{2,4})?\b`)
	regexMilhar = regexp.MustCompile(`^\d{1,3}(\.\d{3})+$`)
	// As alternativas do número, na ordem do C#. O RE2 não tem o "(?<![\d,.])...(?![\d,.])": quem confere as pontas é
	// numerosDoTexto.
	formasDeNumero = []*regexp.Regexp{
		regexp.MustCompile(`^\d{1,3}(?:\.\d{3})+(?:,\d+)?`), regexp.MustCompile(`^\d+,\d+`), regexp.MustCompile(`^\d+\.\d+`), regexp.MustCompile(`^\d+`),
	}

	termosDeProvento = []string{"REND", "RENDIMENTO", "RENDIMENTOS", "DIVID", "DIVIDENDO", "DIVIDENDOS", "PROVENTO", "PROVENTOS", "JCP", "JUROS S", "JRS CAP",
		"JUROS SOBRE CAPITAL", "AMORTIZ", "AMORTIZACAO", "AMORTIZAÇÃO"}
	termosDeInvestimento = []string{"COMPRA", "COMPRAS", "C/", "BUY", "APLICACAO", "APLICAÇÃO", "LIQUIDACAO", "LIQUIDAÇÃO", "NOTA", "CORRETAGEM", "B3", "BOVESPA",
		"ACOES", "AÇÕES", "FII", "CORRETORA", "INVEST"}
	termosDeVenda = []string{"VENDA", "VENDAS", "V/", "SELL"}
	// O filtro grosso do provento recebido: a descrição tem de ter um destes.
	termosDaDescricao = []string{"REND", "DIVID", "PROVENT", "JCP", "JUROS", "JRS", "AMORT"}
	termosDoOriginal  = []string{"REND", "DIVID", "PROVENT", "JCP", "JUROS", "AMORT"}
)

// ehPalavra é o \w do .NET, que é de Unicode.
func ehPalavra(r rune) bool {
	return unicode.IsLetter(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Nd, r) || unicode.Is(unicode.Pc, r)
}

// emAscii troca o que não é ASCII pelo que tem o mesmo efeito no \b, que no RE2 só conhece ASCII: palavra vira "_", e o
// resto, espaço.
func emAscii(t string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r < utf8.RuneSelf:
			return r
		case ehPalavra(r):
			return '_'
		}
		return ' '
	}, t)
}

// fronteiraAntes é o \b antes de um termo que começa com letra ou dígito.
func fronteiraAntes(t string, i int) bool {
	anterior, _ := utf8.DecodeLastRuneInString(t[:i]) // no começo do texto, RuneError, que não é palavra
	return !ehPalavra(anterior)
}

// fronteiraDepois é o \b depois do termo: um lado é palavra, e o outro, não.
func fronteiraDepois(t string, fim int, termo string) bool {
	ultimo, _ := utf8.DecodeLastRuneInString(termo)
	proximo, _ := utf8.DecodeRuneInString(t[fim:]) // no fim do texto, RuneError, que não é palavra
	return ehPalavra(ultimo) != ehPalavra(proximo)
}

// temTermo procura um dos termos a partir de uma fronteira de palavra (o "\b(A|B)"); comFimDePalavra exige outra no fim.
func temTermo(t string, termos []string, comFimDePalavra bool) bool {
	for _, termo := range termos {
		for i := 0; ; {
			j := strings.Index(t[i:], termo)
			if j < 0 {
				break
			}
			inicio := i + j
			if fronteiraAntes(t, inicio) && (!comFimDePalavra || fronteiraDepois(t, inicio+len(termo), termo)) {
				return true
			}
			i = inicio + 1
		}
	}
	return false
}

// achado é um lançamento com código de ativo, já classificado.
type achado struct {
	id        int64
	data      tipos.Data
	descricao string
	valor     tipos.Decimal
	nomeConta string
	ticker    string
	provento  bool
}

func contemAlgum(t string, termos []string) bool {
	return slices.ContainsFunc(termos, func(termo string) bool { return strings.Contains(t, termo) })
}

// localizar é o LocalizarAsync: os lançamentos dos últimos 10 anos com um código de ativo. Para evitar falso positivo, exige
// contexto de investimento ou um ativo que já está na carteira.
func (s *Servico) localizar(ctx context.Context, somenteProventos bool) ([]achado, error) {
	var categoria sql.Null[int64]
	err := s.db.QueryRowContext(ctx, `SELECT "Id" FROM "Categoria" WHERE "Nome" = 'Investimentos' LIMIT 1`).Scan(&categoria)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("ler a categoria de investimentos: %w", err)
	}
	carteira, err := s.tickersDaCarteira(ctx)
	if err != nil {
		return nil, err
	}
	linhas, err := s.db.QueryContext(ctx, `SELECT t."Id", t."Data", t."Descricao", t."DescricaoOriginal", t."Valor", t."CategoriaId", c."Nome", c."Tipo"
		FROM "Transacao" AS t INNER JOIN "Conta" AS c ON t."ContaId" = c."Id" WHERE t."Data" >= ? AND t."TransferenciaGrupo" IS NULL`,
		relogio.Hoje(s.relogio).SomarAnos(-anosDoExtrato))
	if err != nil {
		return nil, fmt.Errorf("ler o extrato: %w", err)
	}
	defer linhas.Close()
	var lista []achado
	for linhas.Next() {
		var a achado
		var original *string
		var categoriaDoLancamento *int64
		var tipoDaConta int64
		if err := linhas.Scan(&a.id, &a.data, &a.descricao, &original, &a.valor, &categoriaDoLancamento, &a.nomeConta, &tipoDaConta); err != nil {
			return nil, fmt.Errorf("ler lançamento: %w", err)
		}
		if somenteProventos && !contemAlgum(a.descricao, termosDaDescricao) && (original == nil || !contemAlgum(*original, termosDoOriginal)) {
			continue
		}
		textoDoLancamento := a.descricao + " "
		if original != nil {
			textoDoLancamento += *original
		}
		maiusculo := texto.MaiusculasInvariantes(textoDoLancamento)
		m := regexTicker.FindStringSubmatch(emAscii(maiusculo))
		if m == nil {
			continue
		}
		ticker := NormalizarTicker(&m[1])
		if ticker == nil {
			continue
		}
		a.ticker = *ticker
		a.provento = positivo(a.valor) && temTermo(maiusculo, termosDeProvento, false)
		naCategoria := categoria.Valid && categoriaDoLancamento != nil && *categoriaDoLancamento == categoria.V
		if !a.provento && tipoDaConta != tipoContaInvestimento && !naCategoria && !carteira[a.ticker] &&
			!temTermo(maiusculo, termosDeInvestimento, false) && !temTermo(maiusculo, termosDeVenda, true) {
			continue
		}
		if !emBranco(original) {
			a.descricao = *original
		}
		lista = append(lista, a)
	}
	return lista, linhas.Err()
}

func (s *Servico) tickersDaCarteira(ctx context.Context) (map[string]bool, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Ticker" FROM "Ativo"`)
	if err != nil {
		return nil, fmt.Errorf("ler os tickers: %w", err)
	}
	defer linhas.Close()
	tickers := map[string]bool{}
	for linhas.Next() {
		var t string
		if err := linhas.Scan(&t); err != nil {
			return nil, fmt.Errorf("ler ticker: %w", err)
		}
		tickers[t] = true
	}
	return tickers, linhas.Err()
}

// ProventoRecebido é um crédito de provento no extrato.
type ProventoRecebido struct {
	Id     int64
	Ticker string
	Data   tipos.Data
	Valor  tipos.Decimal
}

// ProventosRecebidos é o ProventosRecebidosAsync: os créditos de provento, por ativo, para comparar com o estimado pela
// data-com.
func (s *Servico) ProventosRecebidos(ctx context.Context) ([]ProventoRecebido, error) {
	achados, err := s.localizar(ctx, true)
	if err != nil {
		return nil, err
	}
	var lista []ProventoRecebido
	for _, a := range achados {
		if a.provento {
			lista = append(lista, ProventoRecebido{a.id, a.ticker, a.data, a.valor})
		}
	}
	return lista, nil
}

// SugestaoExtratoDto é o do C#. Origem: "extrato", "quantidade do extrato", "preço do extrato", "cotação do dia" ou nil
// (preencher).
type SugestaoExtratoDto struct {
	TransacaoId       int64          `json:"transacaoId"`
	Data              tipos.Data     `json:"data"`
	Descricao         string         `json:"descricao"`
	Valor             tipos.Decimal  `json:"valor"`
	ContaNome         string         `json:"contaNome"`
	Ticker            string         `json:"ticker"`
	Tipo              int64          `json:"tipo"`
	Quantidade        *tipos.Decimal `json:"quantidade"`
	PrecoUnitario     *tipos.Decimal `json:"precoUnitario"`
	FechamentoNoDia   *tipos.Decimal `json:"fechamentoNoDia"`
	Origem            *string        `json:"origem"`
	JaImportada       bool           `json:"jaImportada"`
	PossivelDuplicata bool           `json:"possivelDuplicata"`
}

// fechamentoEm é o FechamentoEm: o da data; senão, o último pregão até ela (a liquidação costuma vir 2 dias depois); senão,
// o primeiro que houver. A lista vem pela data, sem repetir.
func fechamentoEm(fechamentos []cotacoes.Fechamento, data tipos.Data) *tipos.Decimal {
	if len(fechamentos) == 0 {
		return nil
	}
	valor := fechamentos[0].Valor
	for _, f := range fechamentos {
		if !f.Data.Depois(data) {
			valor = f.Valor
		}
	}
	return &valor
}

// emOrdemDeData deixa os fechamentos pela data, com o último de cada dia (o SortedDictionary do C#).
func emOrdemDeData(serie []cotacoes.Fechamento) []cotacoes.Fechamento {
	ordenada := slices.Clone(serie)
	slices.SortStableFunc(ordenada, func(a, b cotacoes.Fechamento) int { return cmp.Compare(a.Data.NumeroDoDia(), b.Data.NumeroDoDia()) })
	var unicos []cotacoes.Fechamento
	for _, f := range ordenada {
		if n := len(unicos); n > 0 && unicos[n-1].Data.Igual(f.Data) {
			unicos[n-1] = f
		} else {
			unicos = append(unicos, f)
		}
	}
	return unicos
}

func ehSeparadorDeNumero(c byte) bool { return (c >= '0' && c <= '9') || c == ',' || c == '.' }

// numeroEm é a primeira forma de número que casa na posição, sem dígito, vírgula ou ponto colado depois dela: se a forma
// tem um desses logo depois, a próxima é tentada.
func numeroEm(t string, i int) string {
	for _, forma := range formasDeNumero {
		m := forma.FindString(t[i:])
		if m != "" && (i+len(m) == len(t) || !ehSeparadorDeNumero(t[i+len(m)])) {
			return m
		}
	}
	return ""
}

// numerosDoTexto é o RegexNumero: os números soltos do texto, sem dígito, vírgula ou ponto colado nas pontas.
func numerosDoTexto(t string) []string {
	var numeros []string
	for i := 0; i < len(t); {
		m := ""
		if i == 0 || !ehSeparadorDeNumero(t[i-1]) {
			m = numeroEm(t, i)
		}
		if m == "" {
			i++
			continue
		}
		numeros = append(numeros, m)
		i += len(m)
	}
	return numeros
}

// numerosSeparados separa os números do texto em inteiros (quantidades) e decimais (preços).
func numerosSeparados(t string) (inteiros, decimais []tipos.Decimal) {
	for _, bruto := range numerosDoTexto(t) {
		if strings.Contains(bruto, ",") || (strings.Contains(bruto, ".") && !regexMilhar.MatchString(bruto)) {
			normalizado := bruto
			if strings.Contains(bruto, ",") {
				normalizado = strings.ReplaceAll(strings.ReplaceAll(bruto, ".", ""), ",", ".")
			}
			if d, ok := tipos.LerNumero(normalizado); ok && positivo(d) {
				decimais = append(decimais, d)
			}
			continue
		}
		i, ok := tipos.LerNumero(strings.ReplaceAll(bruto, ".", ""))
		if ok && positivo(i) && i.Decimal().LessThan(tipos.DecimalDeInteiro(maiorQuantidadeNoTexto).Decimal()) {
			inteiros = append(inteiros, i)
		}
	}
	return inteiros, decimais
}

// perto diz se quantidade × preço bate com o valor, com a folga das taxas.
func (c *conta) perto(quantidade, preco, valor tipos.Decimal) bool {
	diferenca := tipos.DeDecimal(tipos.Subtrair(tipos.Multiplicar(quantidade, preco), valor).Decimal().Abs())
	return !c.dividir(diferenca, valor).Decimal().GreaterThan(folgaDoValor.Decimal())
}

// interpretacao é o que o texto do lançamento disse.
type interpretacao struct {
	quantidade, preco *tipos.Decimal
	origem            *string
}

func comOrigem(quantidade, preco tipos.Decimal, origem string) interpretacao {
	return interpretacao{&quantidade, &preco, &origem}
}

// interpretar é o Interpretar: descobre a quantidade e o preço, primeiro pelo texto, depois pelo valor e pela cotação do
// dia.
func interpretar(textoDoLancamento, ticker string, valor tipos.Decimal, fechamento *tipos.Decimal, c *conta) interpretacao {
	limpo := regexData.ReplaceAllString(emAscii(strings.ReplaceAll(textoDoLancamento, ticker, " ")), " ")
	inteiros, decimais := numerosSeparados(limpo)
	sem := interpretacao{preco: fechamento}
	if !positivo(valor) {
		return sem
	}
	// 1) Quantidade e preço no texto, conferindo com o valor do lançamento.
	for _, q := range inteiros {
		for _, p := range decimais {
			if c.perto(q, p, valor) {
				return comOrigem(q, p, "extrato")
			}
		}
	}
	// 2) Só a quantidade no texto: preço = valor / quantidade, se for plausível perto da cotação do dia.
	for _, q := range inteiros {
		p := c.dividir(valor, q)
		if fechamento == nil || (!p.Decimal().LessThan(tipos.Multiplicar(*fechamento, fatorMinimo).Decimal()) &&
			!p.Decimal().GreaterThan(tipos.Multiplicar(*fechamento, fatorMaximo).Decimal())) {
			return comOrigem(q, tipos.Arredondar(p, 4), "quantidade do extrato")
		}
	}
	// 3) Só o preço no texto, diferente do próprio valor do lançamento.
	for _, p := range decimais {
		if !tipos.Subtrair(p, valor).Decimal().Abs().GreaterThan(centavo.Decimal()) {
			continue
		}
		q := tipos.Arredondar(c.dividir(valor, p), 0)
		if !q.Decimal().LessThan(um.Decimal()) && c.perto(q, p, valor) {
			return comOrigem(q, p, "preço do extrato")
		}
	}
	// 4) Nada no texto: quantidade estimada pela cotação de fechamento do dia.
	if fechamento != nil && positivo(*fechamento) {
		q := tipos.Arredondar(c.dividir(valor, *fechamento), 0)
		if !q.Decimal().LessThan(um.Decimal()) {
			return comOrigem(q, tipos.Arredondar(c.dividir(valor, q), 4), "cotação do dia")
		}
	}
	return sem
}

// operacaoParaDuplicata é o que a conferência de duplicata olha.
type operacaoParaDuplicata struct {
	ticker string
	data   tipos.Data
	tipo   int64
}

func (s *Servico) operacoesVinculadas(ctx context.Context) (map[int64]bool, []operacaoParaDuplicata, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT a."Ticker", o."Data", o."Tipo", o."TransacaoId" FROM "OperacaoAtivo" AS o
		INNER JOIN "Ativo" AS a ON o."AtivoId" = a."Id"`)
	if err != nil {
		return nil, nil, fmt.Errorf("ler as operações: %w", err)
	}
	defer linhas.Close()
	vinculadas := map[int64]bool{}
	var operacoes []operacaoParaDuplicata
	for linhas.Next() {
		var o operacaoParaDuplicata
		var transacao *int64
		if err := linhas.Scan(&o.ticker, &o.data, &o.tipo, &transacao); err != nil {
			return nil, nil, fmt.Errorf("ler operação: %w", err)
		}
		operacoes = append(operacoes, o)
		if transacao != nil {
			vinculadas[*transacao] = true
		}
	}
	return vinculadas, operacoes, linhas.Err()
}

// Sugestoes é o SugestoesAsync: as compras e vendas encontradas nas descrições do extrato.
func (s *Servico) Sugestoes(ctx context.Context) ([]SugestaoExtratoDto, error) {
	lista := []SugestaoExtratoDto{}
	todos, err := s.localizar(ctx, false)
	if err != nil {
		return nil, err
	}
	var ordemDosTickers []string
	grupos := map[string][]achado{}
	for _, a := range todos {
		if a.provento {
			continue
		}
		if _, visto := grupos[a.ticker]; !visto {
			ordemDosTickers = append(ordemDosTickers, a.ticker)
		}
		grupos[a.ticker] = append(grupos[a.ticker], a)
	}
	if len(ordemDosTickers) == 0 {
		return lista, nil
	}
	vinculadas, operacoes, err := s.operacoesVinculadas(ctx)
	if err != nil {
		return nil, err
	}
	var c conta
	for _, ticker := range ordemDosTickers {
		grupo := grupos[ticker]
		de, ate := grupo[0].data, grupo[0].data
		for _, a := range grupo {
			if a.data.Antes(de) {
				de = a.data
			}
			if a.data.Depois(ate) {
				ate = a.data
			}
		}
		serie, err := s.cotacoes.FechamentosDiarios(ctx, ticker, de, ate)
		if err != nil {
			return nil, err
		}
		fechamentos := emOrdemDeData(serie)
		for _, a := range grupo {
			maiusculo := texto.MaiusculasInvariantes(a.descricao)
			tipo := operacaoCompra
			if temTermo(maiusculo, termosDeVenda, true) || (positivo(a.valor) && !strings.Contains(maiusculo, "COMPRA")) {
				tipo = operacaoVenda
			}
			fechamento := fechamentoEm(fechamentos, a.data)
			i := interpretar(maiusculo, ticker, tipos.DeDecimal(a.valor.Decimal().Abs()), fechamento, &c)
			duplicata := slices.ContainsFunc(operacoes, func(o operacaoParaDuplicata) bool {
				d := o.data.NumeroDoDia() - a.data.NumeroDoDia()
				return o.ticker == ticker && o.tipo == tipo && d >= -diasDeDuplicata && d <= diasDeDuplicata
			})
			lista = append(lista, SugestaoExtratoDto{TransacaoId: a.id, Data: a.data, Descricao: a.descricao, Valor: a.valor, ContaNome: a.nomeConta, Ticker: ticker,
				Tipo: tipo, Quantidade: i.quantidade, PrecoUnitario: i.preco, FechamentoNoDia: fechamento, Origem: i.origem, JaImportada: vinculadas[a.id], PossivelDuplicata: duplicata})
		}
	}
	slices.SortStableFunc(lista, func(a, b SugestaoExtratoDto) int {
		return cmp.Or(cmp.Compare(b.Data.NumeroDoDia(), a.Data.NumeroDoDia()), texto.CompararNaCultura(a.Ticker, b.Ticker))
	})
	return lista, c.err
}

func (s *Servico) sugestoesDoExtrato(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Sugestoes(r.Context())
	if err != nil {
		s.erroInterno(w, r, "sugestões do extrato", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// ItemDoExtrato é o ImportarExtratoItemDto.
type ItemDoExtrato struct {
	TransacaoId                      int64
	Ticker                           string
	Tipo                             int64
	Quantidade, PrecoUnitario, Taxas tipos.Decimal
}

// tipoDoExtrato: na B3, o final 11 entra como FII; o resto, como ação.
func tipoDoExtrato(ticker string) int64 {
	if strings.HasSuffix(ticker, "11") {
		return tipoFii
	}
	return tipoAcao
}

// Importar é o ImportarAsync: uma operação para cada item válido, ligada ao lançamento do extrato, uma por lançamento. O que
// não dá para importar é pulado. Ativo novo busca a cotação depois, e a falha não impede.
func (s *Servico) Importar(ctx context.Context, itens []ItemDoExtrato) (int, error) {
	criadas := 0
	var novos []int64
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		datas := map[int64]tipos.Data{}
		for _, item := range itens {
			var d tipos.Data
			switch err := tx.QueryRowContext(ctx, `SELECT "Data" FROM "Transacao" WHERE "Id" = ?`, item.TransacaoId).Scan(&d); {
			case err == nil:
				datas[item.TransacaoId] = d
			case !errors.Is(err, sql.ErrNoRows):
				return fmt.Errorf("ler o lançamento: %w", err)
			}
		}
		vinculadas := map[int64]bool{}
		linhas, err := tx.QueryContext(ctx, `SELECT "TransacaoId" FROM "OperacaoAtivo" WHERE "TransacaoId" IS NOT NULL`)
		if err != nil {
			return fmt.Errorf("ler as operações ligadas: %w", err)
		}
		defer linhas.Close()
		for linhas.Next() {
			var id int64
			if err := linhas.Scan(&id); err != nil {
				return fmt.Errorf("ler operação ligada: %w", err)
			}
			vinculadas[id] = true
		}
		if err := linhas.Err(); err != nil {
			return fmt.Errorf("ler as operações ligadas: %w", err)
		}
		agora := tipos.NovaDataHora(s.relogio.Agora())
		observacao := "Importada do extrato"
		for _, item := range itens {
			data, existe := datas[item.TransacaoId]
			if !existe || vinculadas[item.TransacaoId] {
				continue
			}
			vinculadas[item.TransacaoId] = true
			ticker := NormalizarTicker(&item.Ticker)
			if ticker == nil || !positivo(item.Quantidade) || item.PrecoUnitario.Decimal().IsNegative() || (item.Tipo != operacaoCompra && item.Tipo != operacaoVenda) {
				continue
			}
			var ativo int64
			err := tx.QueryRowContext(ctx, `SELECT "Id" FROM "Ativo" WHERE "Ticker" = ?`, *ticker).Scan(&ativo)
			if errors.Is(err, sql.ErrNoRows) {
				res, erro := tx.ExecContext(ctx, `INSERT INTO "Ativo" ("Ticker", "Tipo", "Moeda", "PrecoManual", "CriadoEm") VALUES (?, ?, 'BRL', 0, ?)`,
					*ticker, tipoDoExtrato(*ticker), agora)
				if erro != nil {
					return fmt.Errorf("gravar o ativo: %w", erro)
				}
				if ativo, err = res.LastInsertId(); err != nil {
					return fmt.Errorf("ler o id do ativo: %w", err)
				}
				novos = append(novos, ativo)
			} else if err != nil {
				return fmt.Errorf("ler o ativo: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO "OperacaoAtivo" ("AtivoId", "Tipo", "Data", "Quantidade", "PrecoUnitario", "Taxas", "Cambio",
				"Observacao", "Origem", "TransacaoId", "CriadoEm") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, ativo, item.Tipo, data, tipos.Arredondar(item.Quantidade, 6),
				tipos.Arredondar(item.PrecoUnitario, 6), tipos.Arredondar(tipos.Maior(tipos.Zero, item.Taxas), 2), um, observacao, origemExtrato, item.TransacaoId, agora); err != nil {
				return fmt.Errorf("gravar a operação: %w", err)
			}
			criadas++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if len(novos) > 0 {
		return criadas, s.atualizarSemFalhar(ctx, novos...)
	}
	return criadas, nil
}

// resultadoDaAplicacao é o ResultadoAplicacaoDto.
type resultadoDaAplicacao struct {
	TransacoesAtualizadas int `json:"transacoesAtualizadas"`
}

// importarDoExtrato é o POST /api/carteira/extrato/importar. O item null chega ao C# como null, e ele para com o 500.
func (s *Servico) importarDoExtrato(w http.ResponseWriter, r *http.Request) {
	itens := respostas.CampoListaDeObjetos{Propriedade: "Itens", TipoItem: "Financas.Api.Dtos.ImportarExtratoItemDto", Minimo: 1, Maximo: 500, Obrigatoria: true,
		Novo: func() []respostas.Campo {
			return []respostas.Campo{&respostas.CampoLong{Propriedade: "TransacaoId"}, &respostas.CampoTexto{Propriedade: "Ticker", Minimo: 3, Maximo: 12},
				&respostas.CampoEnum{Propriedade: "Tipo"},
				&respostas.CampoDecimal{Propriedade: "Quantidade", Faixa: &respostas.Faixa{Minimo: 0.000001, Maximo: 1000000000}},
				&respostas.CampoDecimal{Propriedade: "PrecoUnitario", Faixa: &respostas.Faixa{Minimo: 0, Maximo: 100000000}},
				&respostas.CampoDecimal{Propriedade: "Taxas", Faixa: &respostas.Faixa{Minimo: 0, Maximo: 10000000}}}
		}}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.ImportarExtratoDto", &itens) {
		return
	}
	lista := make([]ItemDoExtrato, 0, len(itens.Itens))
	for _, campos := range itens.Itens {
		if campos == nil {
			s.erroInterno(w, r, "importar do extrato", errors.New("item nulo na lista"))
			return
		}
		lista = append(lista, ItemDoExtrato{TransacaoId: campos[0].(*respostas.CampoLong).Valor, Ticker: *campos[1].(*respostas.CampoTexto).Valor,
			Tipo: campos[2].(*respostas.CampoEnum).Ou(0), Quantidade: campos[3].(*respostas.CampoDecimal).Valor,
			PrecoUnitario: campos[4].(*respostas.CampoDecimal).Valor, Taxas: campos[5].(*respostas.CampoDecimal).Valor})
	}
	criadas, err := s.Importar(r.Context(), lista)
	if err != nil {
		s.erroInterno(w, r, "importar do extrato", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, resultadoDaAplicacao{criadas})
}
