package cambio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// URLPadraoPTAX é a API Olinda do Banco Central (ADR 0007), a mesma do Program.cs: pública, sem chave. Só o
// código da moeda e as datas saem do computador.
const URLPadraoPTAX = "https://olinda.bcb.gov.br/olinda/servico/PTAX/versao/v1/odata/"

// Limites iguais aos do AddHttpClient<CambioService> do Program.cs, mais um teto para o tamanho da resposta.
const (
	tempoLimitePTAX  = 30 * time.Second
	agenteDoCliente  = "FinancasLocal/1.0"
	tamanhoMaximoRes = 10 << 20 // 10 MB: um ano de boletins de uma moeda tem poucos KB
)

// ClientePTAX consulta a PTAX. Faz só HTTPS: a URL base é configurável para os testes (httptest.NewTLSServer).
type ClientePTAX struct {
	base *url.URL
	http *http.Client
}

// NovoClientePTAX cria o cliente. http nil usa um com o tempo limite de 30 s; nos testes, o do httptest.NewTLSServer
// (que confia no certificado do servidor de teste).
func NovoClientePTAX(urlBase string, cliente *http.Client) (*ClientePTAX, error) {
	base, err := url.Parse(urlBase)
	if err != nil {
		return nil, fmt.Errorf("URL da PTAX inválida %q: %w", urlBase, err)
	}
	if base.Scheme != "https" {
		return nil, fmt.Errorf("a URL da PTAX deve usar HTTPS: %q", urlBase)
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/" // como o BaseAddress do HttpClient: o caminho relativo vem depois da última barra
	}
	if cliente == nil {
		cliente = &http.Client{Timeout: tempoLimitePTAX}
	}
	return &ClientePTAX{base: base, http: cliente}, nil
}

// Moeda é o MoedaDto do .NET.
type Moeda struct {
	Codigo string `json:"codigo"`
	Nome   string `json:"nome"`
}

// Fechamento é um boletim de fechamento da PTAX: reais por 1 unidade da moeda.
type Fechamento struct {
	Data   tipos.Data
	Compra tipos.Decimal
	Venda  tipos.Decimal
}

// boletimFechamento é o tipoBoletim que vale: a PTAX publica também Abertura e Intermediário no mesmo dia.
const boletimFechamento = "Fechamento"

// ListarMoedas busca as moedas com PTAX publicada ("Moedas?$format=json"): só as de código válido, em ordem.
func (c *ClientePTAX) ListarMoedas(ctx context.Context) ([]Moeda, error) {
	itens, err := c.buscarValores(ctx, "Moedas?$format=json")
	if err != nil {
		return nil, err
	}
	moedas := []Moeda{}
	for _, item := range itens {
		m := Moeda{Codigo: texto(item, "simbolo"), Nome: texto(item, "nomeFormatado")}
		if _, ok := NormalizarMoeda(m.Codigo); ok {
			moedas = append(moedas, m)
		}
	}
	ordenarMoedas(moedas)
	return moedas, nil
}

// BuscarFechamentos busca os boletins de fechamento da moeda entre as datas (inclusive). Dia sem boletim (fim de
// semana, feriado) simplesmente não vem. O tipoBoletim é conferido de novo, caso a API ignore o $filter.
func (c *ClientePTAX) BuscarFechamentos(ctx context.Context, moeda string, de, ate tipos.Data) ([]Fechamento, error) {
	// O mesmo endereço do CambioService.BaixarAsync (datas em MM-dd-yyyy, entre aspas simples).
	caminho := "CotacaoMoedaPeriodo(moeda=@moeda,dataInicial=@dataInicial,dataFinalCotacao=@dataFinalCotacao)" +
		"?@moeda='" + url.QueryEscape(moeda) + "'&@dataInicial='" + mesDiaAno(de) + "'&@dataFinalCotacao='" + mesDiaAno(ate) + "'" +
		"&$filter=tipoBoletim%20eq%20'" + boletimFechamento + "'&$format=json"
	itens, err := c.buscarValores(ctx, caminho)
	if err != nil {
		return nil, err
	}
	fechamentos := []Fechamento{}
	for _, item := range itens {
		if texto(item, "tipoBoletim") != boletimFechamento {
			continue
		}
		dataHora := texto(item, "dataHoraCotacao")
		if len(dataHora) < 10 {
			continue
		}
		data, err := tipos.LerData(dataHora[:10])
		if err != nil {
			continue
		}
		compra, okCompra := numero(item, "cotacaoCompra")
		venda, okVenda := numero(item, "cotacaoVenda")
		if !okCompra || !okVenda || !compra.Decimal().IsPositive() || !venda.Decimal().IsPositive() {
			continue
		}
		fechamentos = append(fechamentos, Fechamento{Data: data, Compra: compra, Venda: venda})
	}
	return fechamentos, nil
}

// buscarValores faz o GET e devolve a lista "value" do OData (sem "value", lista vazia, como o .NET).
func (c *ClientePTAX) buscarValores(ctx context.Context, caminho string) ([]map[string]json.RawMessage, error) {
	relativo, err := url.Parse(caminho)
	if err != nil {
		return nil, fmt.Errorf("caminho da PTAX inválido: %w", err)
	}
	endereco := c.base.ResolveReference(relativo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endereco.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", agenteDoCliente)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("consultar a PTAX: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("a PTAX respondeu %d", resp.StatusCode) // o EnsureSuccessStatusCode do .NET
	}
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, tamanhoMaximoRes+1))
	if err != nil {
		return nil, fmt.Errorf("ler a resposta da PTAX: %w", err)
	}
	if len(corpo) > tamanhoMaximoRes {
		return nil, errors.New("a resposta da PTAX passou do tamanho máximo")
	}
	var raiz map[string]json.RawMessage
	if err := json.Unmarshal(corpo, &raiz); err != nil {
		return nil, fmt.Errorf("resposta da PTAX não é um objeto JSON: %w", err)
	}
	var itens []map[string]json.RawMessage
	if bruto, ok := raiz["value"]; ok && bytes.HasPrefix(bytes.TrimSpace(bruto), []byte("[")) {
		if err := json.Unmarshal(bruto, &itens); err != nil {
			return nil, fmt.Errorf("lista da PTAX inválida: %w", err)
		}
	}
	return itens, nil
}

// texto é o TryGetProperty(...).GetString() do .NET: só se o valor for texto JSON.
func texto(item map[string]json.RawMessage, nome string) string {
	var s string
	if bruto, ok := item[nome]; !ok || json.Unmarshal(bruto, &s) != nil {
		return ""
	}
	return s
}

// numero é o TryGetDecimal do .NET: só número JSON (não texto), com a escala escrita ("5.01130" fica com 5 casas).
func numero(item map[string]json.RawMessage, nome string) (tipos.Decimal, bool) {
	bruto, ok := item[nome]
	if !ok || bytes.HasPrefix(bytes.TrimSpace(bruto), []byte(`"`)) {
		return tipos.Decimal{}, false // texto com número não vale: o .NET confere ValueKind == Number
	}
	var n json.Number
	dec := json.NewDecoder(bytes.NewReader(bruto))
	dec.UseNumber()
	if err := dec.Decode(&n); err != nil {
		return tipos.Decimal{}, false
	}
	d, err := tipos.NovoDecimal(n.String())
	if err != nil {
		return tipos.Decimal{}, false
	}
	return d, true
}

// mesDiaAno escreve a data como o {de:MM-dd-yyyy} do C#.
func mesDiaAno(d tipos.Data) string {
	return fmt.Sprintf("%02d-%02d-%04d", d.Mes(), d.Dia(), d.Ano())
}
