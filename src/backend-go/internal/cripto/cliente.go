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

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// URLPadrao é a API pública v4 do Mercado Bitcoin (ADR 0048), a mesma do Program.cs: sem chave; só os pares fixos
// saem do computador.
const URLPadrao = "https://api.mercadobitcoin.net/api/v4/"

const (
	tempoLimite     = 20 * time.Second
	agenteDoCliente = "FinancasLocal/1.0"
	tamanhoMaximo   = 1 << 20 // 1 MB: a resposta dos 4 pares tem menos de 1 KB
)

// ErrRede é a HttpRequestException do .NET: sem internet, fonte fora do ar ou status de erro. A tarefa a trata como
// aviso (tenta de novo no próximo ciclo); o resto é erro.
var ErrRede = errors.New("falha ao consultar o Mercado Bitcoin")

// Cliente consulta o Mercado Bitcoin. Só HTTPS: a URL base é configurável para os testes.
type Cliente struct {
	base *url.URL
	http *http.Client
}

// NovoCliente cria o cliente. http nil usa um com o tempo limite de 20 s.
func NovoCliente(urlBase string, cliente *http.Client) (*Cliente, error) {
	base, err := url.Parse(urlBase)
	if err != nil {
		return nil, fmt.Errorf("URL do Mercado Bitcoin inválida %q: %w", urlBase, err)
	}
	if base.Scheme != "https" {
		return nil, fmt.Errorf("a URL do Mercado Bitcoin deve usar HTTPS: %q", urlBase)
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

// UltimosNegocios consulta o último negócio das moedas numa chamada só ("tickers?symbols=BTC-BRL,ETH-BRL,...").
// Devolve só as moedas pedidas com preço positivo e data válida, pela moeda.
func (c *Cliente) UltimosNegocios(ctx context.Context, moedas []string) (map[string]Negocio, error) {
	pares := make([]string, len(moedas))
	for i, m := range moedas {
		pares[i] = m + "-BRL"
	}
	relativo, err := url.Parse("tickers?symbols=" + strings.Join(pares, ","))
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
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%w: status %d", ErrRede, resp.StatusCode)
	}
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, tamanhoMaximo+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRede, err)
	}
	if len(corpo) > tamanhoMaximo {
		return nil, errors.New("a resposta do Mercado Bitcoin passou do tamanho máximo")
	}
	var itens []map[string]json.RawMessage
	if !bytes.HasPrefix(bytes.TrimSpace(corpo), []byte("[")) || json.Unmarshal(corpo, &itens) != nil {
		return nil, errors.New("resposta inesperada do Mercado Bitcoin (não é uma lista)")
	}

	acompanhadas := map[string]bool{}
	for _, m := range moedas {
		acompanhadas[m] = true
	}
	lidas := map[string]Negocio{}
	for _, item := range itens {
		par := texto(item, "pair")
		moeda, ok := strings.CutSuffix(par, "-BRL")
		if !ok || !acompanhadas[moeda] {
			continue
		}
		preco, ok := decimalDoItem(item, "last")
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

// decimalDoItem: o Mercado Bitcoin manda os preços como texto ("419184.00000000"); número também vale, como no .NET.
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

// dataHoraDoItem lê "date", o último negócio. A especificação diz nanossegundos, mas a resposta vem em segundos (10
// dígitos); como no .NET, aceita os três pelo tamanho do número.
func dataHoraDoItem(item map[string]json.RawMessage) (time.Time, bool) {
	bruto, ok := item["date"]
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
