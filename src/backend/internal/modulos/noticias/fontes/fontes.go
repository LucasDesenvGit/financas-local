// Pacote fontes busca e lê as fontes do módulo Notícias (#10, ADR 0067): o RSS do G1 e da BBC News Brasil e a API
// pública do TabNews. Só o pedido do feed sai, sem nada do usuário; na volta, só HTTPS, tamanho e tempo limitados, e só
// links https:// do domínio da fonte.
package fontes

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	tempoLimite     = 15 * time.Second
	agenteDoCliente = "FinancasLocal/1.0"
	tamanhoMaximo   = 2 << 20 // 2 MB: um feed tem dezenas de KB
	// MaximoPorFonte é quantas notícias de cada fonte ficam.
	MaximoPorFonte = 20
)

// Item é uma notícia lida.
type Item struct {
	Titulo      string
	Link        string
	PublicadaEm time.Time // zero se a fonte não disse
}

// Fonte é uma fonte de notícias.
type Fonte struct {
	ID       string
	Nome     string
	URL      string
	Ler      func([]byte) ([]Item, error)
	Dominios []string // os hosts aceitos nos links das notícias
}

// Padrao são as três fontes da #10.
func Padrao() []Fonte {
	return []Fonte{
		{ID: "g1", Nome: "G1", URL: "https://g1.globo.com/rss/g1/", Ler: LerRSS, Dominios: []string{"g1.globo.com"}},
		{ID: "bbc", Nome: "BBC News Brasil", URL: "https://feeds.bbci.co.uk/portuguese/rss.xml", Ler: LerRSS,
			Dominios: []string{"www.bbc.com", "www.bbc.co.uk"}},
		{ID: "tabnews", Nome: "TabNews", URL: "https://www.tabnews.com.br/api/v1/contents?strategy=relevant&per_page=20",
			Ler: LerTabNews, Dominios: []string{"www.tabnews.com.br"}},
	}
}

// Destinos são os hosts que as fontes acessam (o Info do módulo).
func Destinos(fontes []Fonte) []string {
	var hosts []string
	for _, f := range fontes {
		if u, err := url.Parse(f.URL); err == nil && !slices.Contains(hosts, u.Host) {
			hosts = append(hosts, u.Host)
		}
	}
	return hosts
}

// Cliente busca as fontes.
type Cliente struct{ http *http.Client }

// NovoCliente: com cliente nil, o padrão (tempo limite e redirecionamento só para o mesmo host).
func NovoCliente(cliente *http.Client) *Cliente {
	if cliente == nil {
		cliente = &http.Client{Timeout: tempoLimite}
	}
	copia := *cliente
	copia.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || req.URL.Scheme != "https" || req.URL.Host != via[0].URL.Host {
			return errors.New("redirecionamento para fora da fonte")
		}
		return nil
	}
	return &Cliente{http: &copia}
}

// Buscar pede o feed da fonte e devolve as notícias com link permitido, das mais novas para as mais antigas, até
// MaximoPorFonte.
func (c *Cliente) Buscar(ctx context.Context, f Fonte) ([]Item, error) {
	if !strings.HasPrefix(f.URL, "https://") {
		return nil, fmt.Errorf("%s: a fonte tem de ser HTTPS", f.Nome)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Nome, err)
	}
	req.Header.Set("User-Agent", agenteDoCliente)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: não respondeu: %w", f.Nome, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: respondeu %d", f.Nome, resp.StatusCode)
	}
	dados, err := io.ReadAll(io.LimitReader(resp.Body, tamanhoMaximo+1))
	if err != nil {
		return nil, fmt.Errorf("%s: ler a resposta: %w", f.Nome, err)
	}
	if len(dados) > tamanhoMaximo {
		return nil, fmt.Errorf("%s: a resposta passou de 2 MB", f.Nome)
	}
	itens, err := f.Ler(dados)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Nome, err)
	}
	var bons []Item
	for _, it := range itens {
		it.Titulo = strings.Join(strings.Fields(it.Titulo), " ")
		if it.Titulo != "" && LinkPermitido(it.Link, f.Dominios) {
			bons = append(bons, it)
		}
	}
	sort.SliceStable(bons, func(i, j int) bool { return bons[i].PublicadaEm.After(bons[j].PublicadaEm) })
	return bons[:min(len(bons), MaximoPorFonte)], nil
}

// LinkPermitido: https:// num dos domínios da fonte, sem usuário e sem porta. Barra javascript:, http:// e outros sites.
func LinkPermitido(link string, dominios []string) bool {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	return slices.Contains(dominios, u.Host)
}

// rss é o formato RSS 2.0 (só o que interessa).
type rss struct {
	Itens []struct {
		Titulo string `xml:"title"`
		Link   string `xml:"link"`
		Data   string `xml:"pubDate"`
	} `xml:"channel>item"`
}

// LerRSS lê um RSS 2.0.
func LerRSS(dados []byte) ([]Item, error) {
	var r rss
	if err := xml.Unmarshal(dados, &r); err != nil {
		return nil, fmt.Errorf("o RSS não abriu: %w", err)
	}
	itens := make([]Item, 0, len(r.Itens))
	for _, i := range r.Itens {
		itens = append(itens, Item{Titulo: i.Titulo, Link: strings.TrimSpace(i.Link), PublicadaEm: lerData(i.Data)})
	}
	return itens, nil
}

// lerData lê a data do RSS (RFC 1123, com o fuso em número ou em nome); sem data legível, zero.
func lerData(texto string) time.Time {
	texto = strings.TrimSpace(texto)
	for _, formato := range []string{time.RFC1123Z, time.RFC1123, "Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST"} {
		if t, err := time.Parse(formato, texto); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// tabnews é um conteúdo da API do TabNews (só o que interessa).
type tabnews struct {
	Titulo  string `json:"title"`
	Slug    string `json:"slug"`
	Autor   string `json:"owner_username"`
	Publico string `json:"published_at"`
}

// LerTabNews lê a lista de conteúdos da API do TabNews; o link é montado do autor e do slug.
func LerTabNews(dados []byte) ([]Item, error) {
	var lista []tabnews
	if err := json.Unmarshal(dados, &lista); err != nil {
		return nil, fmt.Errorf("a resposta não abriu: %w", err)
	}
	itens := make([]Item, 0, len(lista))
	for _, c := range lista {
		if c.Autor == "" || c.Slug == "" {
			continue
		}
		data, _ := time.Parse(time.RFC3339Nano, c.Publico)
		itens = append(itens, Item{Titulo: c.Titulo, PublicadaEm: data.UTC(),
			Link: "https://www.tabnews.com.br/" + url.PathEscape(c.Autor) + "/" + url.PathEscape(c.Slug)})
	}
	return itens, nil
}
