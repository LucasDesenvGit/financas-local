package selic

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

// URLPadrao é a API pública do SGS do Banco Central, a mesma do Program.cs, sem chave.
const URLPadrao = "https://api.bcb.gov.br/"

// Limites iguais aos do AddHttpClient<SelicService> do Program.cs, mais um teto para o tamanho da resposta.
const (
	tempoLimite     = 30 * time.Second
	agenteDoCliente = "FinancasLocal/1.0"
	tamanhoMaximo   = 10 << 20 // 10 MB: 9 anos de taxas diárias têm poucas centenas de KB
)

// Cliente consulta o SGS. Faz só HTTPS: a URL base é configurável para os testes (o testeapoio.ServicoDeFora).
type Cliente struct {
	base *url.URL
	http *http.Client
}

// NovoCliente cria o cliente. http nil usa um com o tempo limite de 30 s; nos testes, o do testeapoio.ServicoDeFora (que
// confia no certificado do servidor de teste).
func NovoCliente(urlBase string, cliente *http.Client) (*Cliente, error) {
	base, err := url.Parse(urlBase)
	if err != nil {
		return nil, fmt.Errorf("URL do Banco Central inválida %q: %w", urlBase, err)
	}
	if base.Scheme != "https" {
		return nil, fmt.Errorf("a URL do Banco Central deve usar HTTPS: %q", urlBase)
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/" // como o BaseAddress do HttpClient: o caminho relativo vem depois da última barra
	}
	if cliente == nil {
		cliente = &http.Client{Timeout: tempoLimite}
	}
	return &Cliente{base: base, http: cliente}, nil
}

// ClientePadrao é o cliente de produção.
func ClientePadrao() *Cliente {
	c, err := NovoCliente(URLPadrao, nil)
	if err != nil {
		panic(err) // a URL é uma constante: erro aqui é bug
	}
	return c
}

// errRespostaDoBanco é a resposta que não tem a forma esperada (as exceções do JsonElement no C#).
var errRespostaDoBanco = errors.New("resposta do Banco Central fora do formato")

// obter faz o GET e devolve o status e o corpo.
func (c *Cliente) obter(ctx context.Context, caminho string) (int, []byte, error) {
	relativo, err := url.Parse(caminho)
	if err != nil {
		return 0, nil, fmt.Errorf("montar o endereço do Banco Central: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base.ResolveReference(relativo).String(), nil)
	if err != nil {
		return 0, nil, fmt.Errorf("montar o pedido ao Banco Central: %w", err)
	}
	req.Header.Set("User-Agent", agenteDoCliente)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("consultar o Banco Central: %w", err)
	}
	defer resp.Body.Close()
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, tamanhoMaximo))
	if err != nil {
		return 0, nil, fmt.Errorf("ler a resposta do Banco Central: %w", err)
	}
	return resp.StatusCode, corpo, nil
}

func sucesso(status int) bool { return status >= 200 && status <= 299 }

// textoDoCampo é o GetString do C#: ausente ou null, nil; outro tipo, erro.
func textoDoCampo(item map[string]json.RawMessage, campo string) (*string, error) {
	cru, ok := item[campo]
	if !ok {
		return nil, nil
	}
	var texto *string
	if err := json.Unmarshal(cru, &texto); err != nil {
		return nil, fmt.Errorf("%w: %s não é texto", errRespostaDoBanco, campo)
	}
	return texto, nil
}

// itens lê a lista de objetos da resposta. Item que não é objeto é erro, como o TryGetProperty do C#.
func itens(corpo []byte) ([]map[string]json.RawMessage, error) {
	var lista []map[string]json.RawMessage
	if err := json.Unmarshal(corpo, &lista); err != nil {
		return nil, fmt.Errorf("%w: %w", errRespostaDoBanco, err)
	}
	for _, item := range lista {
		if item == nil {
			return nil, fmt.Errorf("%w: item nulo", errRespostaDoBanco)
		}
	}
	return lista, nil
}

// ehLista diz se o JSON é uma lista; JSON quebrado é erro.
func ehLista(corpo []byte) (bool, error) {
	if !json.Valid(corpo) {
		return false, fmt.Errorf("%w: JSON quebrado", errRespostaDoBanco)
	}
	return bytes.HasPrefix(bytes.TrimLeft(corpo, " \t\r\n"), []byte("[")), nil
}

// lerDataDoBanco é o DateOnly.TryParseExact com "dd/MM/yyyy".
func lerDataDoBanco(texto string) (tipos.Data, bool) {
	t, err := time.Parse("02/01/2006", texto)
	if err != nil || t.Year() < 1 || strings.ContainsAny(texto, "+-") {
		return tipos.Data{}, false
	}
	return tipos.NovaData(t.Year(), t.Month(), t.Day()), true
}

// BuscarTaxas busca a Selic diária entre as datas (inclusive). Período sem dia útil é 404, e não vem nada; resposta que
// não é uma lista, também não. Linha com a data ou a taxa que não dá para ler é pulada.
func (c *Cliente) BuscarTaxas(ctx context.Context, de, ate tipos.Data) ([]Taxa, error) {
	caminho := "dados/serie/bcdata.sgs.11/dados?formato=json&dataInicial=" + diaMesAno(de) + "&dataFinal=" + diaMesAno(ate)
	status, corpo, err := c.obter(ctx, caminho)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	if !sucesso(status) {
		return nil, fmt.Errorf("Banco Central respondeu %d", status)
	}
	lista, err := ehLista(corpo)
	if err != nil || !lista {
		return nil, err
	}
	linhas, err := itens(corpo)
	if err != nil {
		return nil, err
	}
	var taxas []Taxa
	for _, item := range linhas {
		dataTexto, err := textoDoCampo(item, "data")
		if err != nil {
			return nil, err
		}
		valorTexto, err := textoDoCampo(item, "valor")
		if err != nil {
			return nil, err
		}
		if dataTexto == nil || valorTexto == nil {
			continue
		}
		data, ok := lerDataDoBanco(*dataTexto)
		if !ok {
			continue
		}
		if taxa, ok := tipos.LerNumero(*valorTexto); ok {
			taxas = append(taxas, Taxa{Data: data, Taxa: taxa})
		}
	}
	return taxas, nil
}

// BuscarMeta busca a meta Selic mais recente (% ao ano), em texto; nil se não vier. Sem o campo, ou resposta que não é
// uma lista, é erro.
func (c *Cliente) BuscarMeta(ctx context.Context) (*string, error) {
	status, corpo, err := c.obter(ctx, "dados/serie/bcdata.sgs.432/dados/ultimos/1?formato=json")
	if err != nil {
		return nil, err
	}
	if !sucesso(status) {
		return nil, fmt.Errorf("Banco Central respondeu %d", status)
	}
	linhas, err := itens(corpo)
	if err != nil {
		return nil, err
	}
	var valor *string
	for _, item := range linhas {
		if _, ok := item["valor"]; !ok {
			return nil, fmt.Errorf("%w: sem o valor", errRespostaDoBanco)
		}
		if valor, err = textoDoCampo(item, "valor"); err != nil {
			return nil, err
		}
	}
	return valor, nil
}

func diaMesAno(d tipos.Data) string {
	return fmt.Sprintf("%02d/%02d/%04d", d.Dia(), int(d.Mes()), d.Ano())
}
