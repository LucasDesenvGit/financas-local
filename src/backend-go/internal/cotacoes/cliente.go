// Pacote cotacoes é o CotacaoService do C# (#82): as cotações e os proventos dos ativos. A fonte principal é o Yahoo Finance
// (pública, sem chave, não oficial); a alternativa, a brapi.dev, com token. Ativos no exterior usam o código como no Yahoo,
// sem ".SA", e a brapi só atende a B3. Só o código do ativo sai do computador: nenhuma quantidade, valor ou dado pessoal.
package cotacoes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Os endereços e os limites são os dos AddHttpClient do Program.cs, mais um teto para o tamanho da resposta.
const (
	URLPadraoYahoo  = "https://query1.finance.yahoo.com/"
	URLPadraoBrapi  = "https://brapi.dev/"
	tempoLimite     = 20 * time.Second
	agenteNavegador = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"
	agenteDoCliente = "FinancasLocal/1.0"
	tamanhoMaximo   = 20 << 20 // 20 MB: o histórico mensal do maior ativo tem poucos KB
	// A faixa do histórico mensal cresce com a idade da primeira operação.
	anosParaTudo    = 9
	anosParaDezAnos = 4
	casasDoProvento = 8
)

// errFormato é a resposta que não tem a forma esperada (as exceções do JsonElement no C#).
var errFormato = errors.New("resposta fora do formato")

// Cliente consulta o Yahoo e a brapi. Faz só HTTPS: as URLs base são configuráveis para os testes.
type Cliente struct {
	yahoo, brapi *url.URL
	http         *http.Client
}

func baseHTTPS(endereco string) (*url.URL, error) {
	base, err := url.Parse(endereco)
	if err != nil {
		return nil, fmt.Errorf("URL inválida %q: %w", endereco, err)
	}
	if base.Scheme != "https" {
		return nil, fmt.Errorf("a URL deve usar HTTPS: %q", endereco)
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/" // como o BaseAddress do HttpClient: o caminho relativo vem depois da última barra
	}
	return base, nil
}

// NovoCliente cria o cliente. http nil usa um com o tempo limite de 20 s; nos testes, o do testeapoio.ServicoDeFora.
func NovoCliente(urlYahoo, urlBrapi string, cliente *http.Client) (*Cliente, error) {
	yahoo, err := baseHTTPS(urlYahoo)
	if err != nil {
		return nil, err
	}
	brapi, err := baseHTTPS(urlBrapi)
	if err != nil {
		return nil, err
	}
	if cliente == nil {
		cliente = &http.Client{Timeout: tempoLimite}
	}
	return &Cliente{yahoo: yahoo, brapi: brapi, http: cliente}, nil
}

// ClientePadrao é o cliente de produção.
func ClientePadrao() *Cliente {
	c, err := NovoCliente(URLPadraoYahoo, URLPadraoBrapi, nil)
	if err != nil {
		panic(err) // as URLs são constantes: erro aqui é bug
	}
	return c
}

// obter faz o GET e devolve o corpo, ou nil quando o status não é de sucesso.
func (c *Cliente) obter(ctx context.Context, base *url.URL, caminho, agente, token string) ([]byte, error) {
	relativo, err := url.Parse(caminho)
	if err != nil {
		return nil, fmt.Errorf("montar o endereço da cotação: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.ResolveReference(relativo).String(), nil)
	if err != nil {
		return nil, fmt.Errorf("montar o pedido da cotação: %w", err)
	}
	req.Header.Set("User-Agent", agente)
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token) // no cabeçalho: não fica em log de URL
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("consultar a cotação: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, nil
	}
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, tamanhoMaximo))
	if err != nil {
		return nil, fmt.Errorf("ler a cotação: %w", err)
	}
	return corpo, nil
}

/* ------------------------------ leitura defensiva de JSON ------------------------------ */

// tipoDoJSON é o primeiro caractere do valor: '{', '[', '"', 'n' (null), 't', 'f', ou o começo de um número.
func tipoDoJSON(v json.RawMessage) byte {
	if t := bytes.TrimLeft(v, " \t\r\n"); len(t) > 0 {
		return t[0]
	}
	return 0
}

// campo é o TryGetProperty do C#: só em objeto, senão a exceção vira erro.
func campo(v json.RawMessage, nome string) (json.RawMessage, bool, error) {
	if tipoDoJSON(v) != '{' {
		return nil, false, fmt.Errorf("%w: %s fora de um objeto", errFormato, nome)
	}
	var o map[string]json.RawMessage
	if err := json.Unmarshal(v, &o); err != nil {
		return nil, false, fmt.Errorf("%w: %w", errFormato, err)
	}
	cru, ok := o[nome]
	return cru, ok, nil
}

func lista(v json.RawMessage) ([]json.RawMessage, bool) {
	var l []json.RawMessage
	if tipoDoJSON(v) != '[' || json.Unmarshal(v, &l) != nil {
		return nil, false
	}
	return l, true
}

// decimalDoJSON é o TryGetDecimal: um número JSON que cabe num decimal, com o expoente que vier. O ParseFloat olha o
// tamanho antes: 1e999999999 não pode virar um número de um bilhão de dígitos, nem 1e-999999999 uma divisão enorme.
func decimalDoJSON(cru json.RawMessage) *tipos.Decimal {
	texto := string(bytes.TrimSpace(cru))
	if t := tipoDoJSON(cru); t != '-' && (t < '0' || t > '9') {
		return nil
	}
	f, err := strconv.ParseFloat(texto, 64)
	if err != nil {
		return nil
	}
	if f == 0 {
		return &tipos.Zero
	}
	d, err := tipos.NovoDecimal(texto)
	if err != nil {
		return nil
	}
	cabe, ok := tipos.LerNumero(d.String())
	if !ok {
		return nil
	}
	return &cabe
}

// numero é o Numero: o campo numérico do objeto; o que não for, nil.
func numero(v json.RawMessage, nome string) *tipos.Decimal {
	cru, _, _ := campo(v, nome) // fora de um objeto, ou sem o campo, não há número
	return decimalDoJSON(cru)
}

// textoDoCampo é o Texto: o campo de texto do objeto; o que não for, nil.
func textoDoCampo(v json.RawMessage, nome string) *string {
	cru, _, _ := campo(v, nome) // fora de um objeto, ou sem o campo, não há texto
	if tipoDoJSON(cru) != '"' {
		return nil
	}
	var s string
	if json.Unmarshal(cru, &s) != nil {
		return nil
	}
	return &s
}

// inteiroDoJSON é o TryGetInt64: fora de um número, a exceção do C# vira erro; número que não é inteiro, false.
func inteiroDoJSON(cru json.RawMessage) (int64, bool, error) {
	if t := tipoDoJSON(cru); t != '-' && (t < '0' || t > '9') {
		return 0, false, fmt.Errorf("%w: número esperado", errFormato)
	}
	n, err := strconv.ParseInt(string(bytes.TrimSpace(cru)), 10, 64)
	if err != nil {
		return 0, false, nil
	}
	return n, true, nil
}

// resultado é o Resultado: o primeiro item de chart.result, ou nil.
func resultado(corpo []byte) (json.RawMessage, error) {
	if !json.Valid(corpo) {
		return nil, fmt.Errorf("%w: JSON quebrado", errFormato)
	}
	chart, ok, err := campo(corpo, "chart")
	if err != nil || !ok {
		return nil, err
	}
	cru, _, err := campo(chart, "result")
	if err != nil {
		return nil, err
	}
	itens, ok := lista(cru)
	if !ok || len(itens) == 0 {
		return nil, nil
	}
	return itens[0], nil
}

// fechamentosCrus são os itens de indicators.quote[0].close, ou nil se o caminho não existir.
func fechamentosCrus(r json.RawMessage) ([]json.RawMessage, error) {
	indicadores, ok, err := campo(r, "indicators")
	if err != nil || !ok {
		return nil, err
	}
	cotacoes, _, err := campo(indicadores, "quote")
	if err != nil {
		return nil, err
	}
	itens, ok := lista(cotacoes)
	if !ok || len(itens) == 0 {
		return nil, nil
	}
	fechamentos, _, err := campo(itens[0], "close")
	if err != nil {
		return nil, err
	}
	valores, _ := lista(fechamentos)
	return valores, nil
}

// fechamentos é o Fechamentos: os números da série, sem os nulos.
func fechamentos(r json.RawMessage) ([]tipos.Decimal, error) {
	crus, err := fechamentosCrus(r)
	if err != nil {
		return nil, err
	}
	var valores []tipos.Decimal
	for _, f := range crus {
		if d := decimalDoJSON(f); d != nil {
			valores = append(valores, *d)
		}
	}
	return valores, nil
}

// Fechamento é um fechamento numa data (no mensal, o primeiro dia do mês).
type Fechamento struct {
	Data  tipos.Data
	Valor tipos.Decimal
}

func dataDoUnix(segundos int64) tipos.Data {
	return tipos.DataDe(relogio.ParaHoraLocal(time.Unix(segundos, 0)))
}

// serieDeFechamentos é o SerieFechamentos: os pares (data, fechamento), alinhando "timestamp" com o fechamento. O que não
// é um valor positivo fica de fora.
func serieDeFechamentos(r json.RawMessage) ([]Fechamento, error) {
	cru, _, err := campo(r, "timestamp")
	if err != nil {
		return nil, err
	}
	datas, ok := lista(cru)
	if !ok {
		return nil, nil
	}
	valores, err := fechamentosCrus(r)
	if err != nil {
		return nil, err
	}
	var serie []Fechamento
	for i := range min(len(datas), len(valores)) {
		segundos, ok, err := inteiroDoJSON(datas[i])
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if d := decimalDoJSON(valores[i]); d != nil && d.Decimal().IsPositive() {
			serie = append(serie, Fechamento{dataDoUnix(segundos), *d})
		}
	}
	return serie, nil
}

// fechamentosMensais é o FechamentosMensais: um fechamento por mês, o último dele, na ordem em que os meses aparecem.
func fechamentosMensais(r json.RawMessage) ([]Fechamento, error) {
	serie, err := serieDeFechamentos(r)
	if err != nil {
		return nil, err
	}
	var meses []Fechamento
	indices := map[string]int{}
	ultimas := make([]tipos.Data, 0, len(serie))
	for _, f := range serie {
		mes := relogio.PrimeiroDiaMes(f.Data.Ano(), f.Data.Mes())
		i, visto := indices[mes.String()]
		if !visto {
			indices[mes.String()] = len(meses)
			meses, ultimas = append(meses, Fechamento{mes, f.Valor}), append(ultimas, f.Data)
		} else if !f.Data.Antes(ultimas[i]) {
			meses[i].Valor, ultimas[i] = f.Valor, f.Data
		}
	}
	return meses, nil
}

// Provento é o que a fonte informa: a data-com e o valor por cota.
type Provento struct {
	DataCom tipos.Data
	Valor   tipos.Decimal
}

// Obtida é a CotacaoObtida do C#. Moeda é a que a fonte informou, quando informa.
type Obtida struct {
	Preco, PrecoAnterior *tipos.Decimal
	Nome, Moeda          *string
	Fonte                string
	Proventos            []Provento
	Mensais              []Fechamento
}

// pregaoAnterior é a data-com a partir da data "ex": o dia útil de antes.
func pregaoAnterior(d tipos.Data) tipos.Data {
	d = d.SomarDias(-1)
	for {
		dia := time.Date(d.Ano(), d.Mes(), d.Dia(), 0, 0, 0, 0, time.UTC).Weekday()
		if dia != time.Saturday && dia != time.Sunday {
			return d
		}
		d = d.SomarDias(-1)
	}
}

// escapar é o Uri.EscapeDataString.
func escapar(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }

func (c *Cliente) graficoYahoo(ctx context.Context, simbolo, consulta string) (json.RawMessage, error) {
	corpo, err := c.obter(ctx, c.yahoo, "v8/finance/chart/"+escapar(simbolo)+"?"+consulta, agenteNavegador, "")
	if err != nil || corpo == nil {
		return nil, err
	}
	return resultado(corpo)
}

// lerPreco lê o preço atual, o nome, a moeda e o fechamento anterior, que é o do penúltimo pregão (o último é o de hoje,
// se houver).
func lerPreco(r json.RawMessage, o *Obtida) error {
	meta, ok, err := campo(r, "meta")
	if err != nil || !ok {
		return err
	}
	o.Preco, o.Moeda = numero(meta, "regularMarketPrice"), textoDoCampo(meta, "currency")
	if o.Nome = textoDoCampo(meta, "longName"); o.Nome == nil {
		o.Nome = textoDoCampo(meta, "shortName")
	}
	valores, err := fechamentos(r)
	if err != nil {
		return err
	}
	if len(valores) >= 2 {
		o.PrecoAnterior = &valores[len(valores)-2]
		return nil
	}
	if o.PrecoAnterior = numero(meta, "previousClose"); o.PrecoAnterior == nil {
		o.PrecoAnterior = numero(meta, "chartPreviousClose")
	}
	return nil
}

// valoresDoObjeto lê os valores do objeto, que já é JSON válido, na ordem do documento (o EnumerateObject): na mesma
// data-com, fica o primeiro.
func valoresDoObjeto(v json.RawMessage) []json.RawMessage {
	d := json.NewDecoder(bytes.NewReader(v))
	_, _ = d.Token() // a chave que abre o objeto
	var valores []json.RawMessage
	for d.More() {
		_, _ = d.Token() // o nome do campo
		var valor json.RawMessage
		_ = d.Decode(&valor)
		valores = append(valores, valor)
	}
	return valores
}

// lerProventos lê os dividendos. O Yahoo informa a data "ex"; a data-com é o pregão anterior.
func lerProventos(r json.RawMessage, o *Obtida) error {
	eventos, ok, err := campo(r, "events")
	if err != nil || !ok {
		return err
	}
	dividendos, _, err := campo(eventos, "dividends")
	if err != nil || tipoDoJSON(dividendos) != '{' {
		return err
	}
	for _, item := range valoresDoObjeto(dividendos) {
		valor := numero(item, "amount")
		if valor == nil {
			continue
		}
		data, ok, _ := campo(item, "date") // com o valor lido, o item é um objeto
		if !ok {
			continue
		}
		segundos, ok, err := inteiroDoJSON(data)
		if err != nil {
			return err
		}
		if ok {
			o.Proventos = append(o.Proventos, Provento{pregaoAnterior(dataDoUnix(segundos)), tipos.Arredondar(*valor, casasDoProvento)})
		}
	}
	return nil
}

// Yahoo é o YahooAsync: o preço atual e, desde a primeira compra, os proventos e o histórico mensal. simbolo é o código
// como no Yahoo ("MXRF11.SA" na B3, "AAPL" no exterior). Status de erro não é erro: só não vem nada dele. nil se não
// vier nada das duas consultas.
func (c *Cliente) Yahoo(ctx context.Context, simbolo string, primeiraOperacao tipos.Data, anoDeHoje int) (*Obtida, error) {
	o := Obtida{Fonte: "Yahoo Finance"}
	r, err := c.graficoYahoo(ctx, simbolo, "range=5d&interval=1d")
	if err != nil {
		return nil, err
	}
	if r != nil {
		if err := lerPreco(r, &o); err != nil {
			return nil, err
		}
	}
	faixa := "5y"
	if anos := anoDeHoje - primeiraOperacao.Ano(); anos >= anosParaTudo {
		faixa = "max"
	} else if anos >= anosParaDezAnos {
		faixa = "10y"
	}
	r, err = c.graficoYahoo(ctx, simbolo, "range="+faixa+"&interval=1mo&events=div")
	if err != nil {
		return nil, err
	}
	if r != nil {
		if o.Mensais, err = fechamentosMensais(r); err != nil {
			return nil, err
		}
		if err := lerProventos(r, &o); err != nil {
			return nil, err
		}
	}
	if o.Preco == nil && len(o.Proventos) == 0 && len(o.Mensais) == 0 {
		return nil, nil
	}
	return &o, nil
}

// Brapi é o BrapiAsync: o preço pela brapi.dev, com o token; nil se ela não tiver.
func (c *Cliente) Brapi(ctx context.Context, ticker, token string) (*Obtida, error) {
	corpo, err := c.obter(ctx, c.brapi, "api/quote/"+escapar(ticker), agenteDoCliente, token)
	if err != nil || corpo == nil {
		return nil, err
	}
	if !json.Valid(corpo) {
		return nil, fmt.Errorf("%w: JSON quebrado", errFormato)
	}
	cru, _, err := campo(corpo, "results")
	if err != nil {
		return nil, err
	}
	itens, ok := lista(cru)
	if !ok || len(itens) == 0 {
		return nil, nil
	}
	r := itens[0]
	o := Obtida{Preco: numero(r, "regularMarketPrice"), PrecoAnterior: numero(r, "regularMarketPreviousClose"), Fonte: "brapi.dev"}
	if o.Preco == nil {
		return nil, nil
	}
	if o.Nome = textoDoCampo(r, "longName"); o.Nome == nil {
		o.Nome = textoDoCampo(r, "shortName")
	}
	return &o, nil
}

// MensaisDoIndice é o histórico mensal de um índice pelo Yahoo, em 10 anos; nil se a fonte não responder.
func (c *Cliente) MensaisDoIndice(ctx context.Context, simbolo string) ([]Fechamento, error) {
	r, err := c.graficoYahoo(ctx, simbolo, "range=10y&interval=1mo")
	if err != nil || r == nil {
		return nil, err
	}
	return fechamentosMensais(r)
}

// FechamentosDiarios são os fechamentos do ativo da B3 entre os instantes, em segundos.
func (c *Cliente) FechamentosDiarios(ctx context.Context, ticker string, inicio, fim int64) ([]Fechamento, error) {
	r, err := c.graficoYahoo(ctx, ticker+".SA", fmt.Sprintf("period1=%d&period2=%d&interval=1d", inicio, fim))
	if err != nil || r == nil {
		return nil, err
	}
	return serieDeFechamentos(r)
}
