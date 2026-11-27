package cripto

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

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// URLPadrao é a API pública de dados de mercado da Binance (ADR 0073, no lugar do Mercado Bitcoin da 0048, que bloqueia
// quem não é navegador desde 14/11/2026, #762): sem chave; só os pares fixos saem do computador.
const URLPadrao = "https://api.binance.com/api/v3/"

const (
	tempoLimite     = 20 * time.Second
	agenteDoCliente = "FinancasLocal/1.0"
	tamanhoMaximo   = 1 << 20 // 1 MB: a resposta dos 4 pares tem menos de 1 KB
)

var (
	// ErrRede é a HttpRequestException do .NET: sem internet, fonte fora do ar (5xx), tempo esgotado ou limite de
	// pedidos (408 e 429). A tarefa a trata como aviso (tenta de novo no próximo ciclo); o resto é erro.
	ErrRede = errors.New("falha ao consultar a Binance")
	// ErrRecusa é a fonte que respondeu e recusou o pedido (os outros 4xx): o bloqueio do WAF (403), o país (451), o
	// banimento do IP depois de passar do limite (418), um caminho que mudou. Não é falta de internet e não passa sozinho
	// no próximo ciclo, então a tarefa a registra como erro, uma vez (#762: era o 403 do Mercado Bitcoin).
	ErrRecusa = errors.New("a Binance recusou o pedido")
)

// ehRecusa: um 4xx que não é passageiro (o 408 e o 429 são).
func ehRecusa(status int) bool {
	return status >= 400 && status < 500 && status != http.StatusRequestTimeout && status != http.StatusTooManyRequests
}

// Cliente consulta a Binance. Só HTTPS: a URL base é configurável para os testes.
type Cliente struct {
	base *url.URL
	http *http.Client
}

// NovoCliente cria o cliente. http nil usa um com o tempo limite de 20 s.
func NovoCliente(urlBase string, cliente *http.Client) (*Cliente, error) {
	base, err := url.Parse(urlBase)
	if err != nil {
		return nil, fmt.Errorf("URL da Binance inválida %q: %w", urlBase, err)
	}
	if base.Scheme != "https" {
		return nil, fmt.Errorf("a URL da Binance deve usar HTTPS: %q", urlBase)
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	if cliente == nil {
		cliente = &http.Client{Timeout: tempoLimite}
	}
	return &Cliente{base: base, http: cliente}, nil
}

// Negocio é o último negócio de uma moeda: o preço em reais e a data e hora informada pela fonte (UTC).
type Negocio struct {
	Preco    tipos.Decimal
	DataHora time.Time
}

// UltimosNegocios consulta o último negócio das moedas numa chamada só: o ticker de 24 h, na forma curta
// (`ticker/24hr?symbols=["BTCBRL",...]&type=MINI`, peso 2 de 6000 por minuto). Devolve só as moedas pedidas com preço
// positivo e data válida, pela moeda.
func (c *Cliente) UltimosNegocios(ctx context.Context, moedas []string) (map[string]Negocio, error) {
	pares := make([]string, len(moedas))
	for i, m := range moedas {
		pares[i] = `"` + m + `BRL"`
	}
	consulta := url.Values{}
	consulta.Set("symbols", "["+strings.Join(pares, ",")+"]")
	consulta.Set("type", "MINI")
	relativo, err := url.Parse("ticker/24hr?" + consulta.Encode())
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base.ResolveReference(relativo).String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", agenteDoCliente)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRede, err)
	}
	defer resp.Body.Close()
	if ehRecusa(resp.StatusCode) {
		return nil, fmt.Errorf("%w: status %d", ErrRecusa, resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%w: status %d", ErrRede, resp.StatusCode)
	}
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, tamanhoMaximo+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRede, err)
	}
	if len(corpo) > tamanhoMaximo {
		return nil, errors.New("a resposta da Binance passou do tamanho máximo")
	}
	var itens []map[string]json.RawMessage
	if !bytes.HasPrefix(bytes.TrimSpace(corpo), []byte("[")) || json.Unmarshal(corpo, &itens) != nil {
		return nil, errors.New("resposta inesperada da Binance (não é uma lista)")
	}

	acompanhadas := map[string]bool{}
	for _, m := range moedas {
		acompanhadas[m] = true
	}
	lidas := map[string]Negocio{}
	for _, item := range itens {
		moeda, ok := strings.CutSuffix(texto(item, "symbol"), "BRL")
		if !ok || !acompanhadas[moeda] {
			continue
		}
		preco, ok := decimalDoItem(item, "lastPrice")
		if !ok || !preco.Decimal().IsPositive() {
			continue
		}
		dataHora, ok := dataHoraDoItem(item)
		if !ok {
			continue
		}
		lidas[moeda] = Negocio{Preco: preco, DataHora: dataHora}
	}
	return lidas, nil
}

func texto(item map[string]json.RawMessage, nome string) string {
	var s string
	if bruto, ok := item[nome]; !ok || json.Unmarshal(bruto, &s) != nil {
		return ""
	}
	return s
}

// decimalDoItem: a Binance manda os preços como texto ("372650.00000000"); número também vale, como no .NET.
func decimalDoItem(item map[string]json.RawMessage, nome string) (tipos.Decimal, bool) {
	bruto, ok := item[nome]
	if !ok {
		return tipos.Decimal{}, false
	}
	var textoNumero string
	if bytes.HasPrefix(bytes.TrimSpace(bruto), []byte(`"`)) {
		if json.Unmarshal(bruto, &textoNumero) != nil {
			return tipos.Decimal{}, false
		}
	} else {
		var n json.Number
		dec := json.NewDecoder(bytes.NewReader(bruto))
		dec.UseNumber()
		if dec.Decode(&n) != nil {
			return tipos.Decimal{}, false
		}
		textoNumero = n.String()
	}
	d, err := tipos.NovoDecimal(textoNumero)
	if err != nil {
		return tipos.Decimal{}, false
	}
	return d, true
}

// dataHoraDoItem lê "closeTime", o fim da janela de 24 h, que é o instante do último negócio. A Binance manda em
// milissegundos; segundos e nanossegundos também valem, pelo tamanho do número, como no cliente do Mercado Bitcoin.
func dataHoraDoItem(item map[string]json.RawMessage) (time.Time, bool) {
	bruto, ok := item["closeTime"]
	if !ok {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(bruto)), 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}, false
	}
	segundos := n
	switch {
	case n > 100_000_000_000_000:
		segundos = n / 1_000_000_000
	case n > 100_000_000_000:
		segundos = n / 1000
	}
	return time.Unix(segundos, 0).UTC(), true
}
