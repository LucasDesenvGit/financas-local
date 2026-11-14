package fontes

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// fonteDeTeste aponta a fonte para o serviço de fora dos testes.
func fonteDeTeste(srv *testeapoio.ServicoDeFora, caminho string, ler func([]byte) ([]Item, error), dominios ...string) Fonte {
	return Fonte{ID: "t", Nome: "Teste", URL: srv.URL() + caminho, Ler: ler, Dominios: dominios}
}

func TestBuscarRSS(t *testing.T) {
	t.Parallel()
	srv := testeapoio.NovoServicoDeFora(t)
	srv.Exigir("User-Agent", "FinancasLocal/1.0")
	srv.Em("GET", "/rss/g1/", testeapoio.RespostaDeFora{Arquivo: "g1.xml", TipoConteudo: "application/rss+xml"})
	itens, err := NovoCliente(srv.Cliente()).Buscar(t.Context(), fonteDeTeste(srv, "/rss/g1/", LerRSS, "g1.globo.com"))
	if err != nil {
		t.Fatal(err)
	}
	// O javascript:, o outro site e o sem título ficam de fora; os espaços somem; a mais nova primeiro; a sem data no fim.
	var titulos []string
	for _, i := range itens {
		titulos = append(titulos, i.Titulo)
	}
	if got := strings.Join(titulos, " | "); got != "Notícia fictícia de hoje com espaços | Notícia fictícia de ontem | Sem data" {
		t.Fatalf("títulos = %s", got)
	}
	if !itens[0].PublicadaEm.Equal(time.Date(2026, 11, 14, 12, 30, 0, 0, time.UTC)) || !itens[2].PublicadaEm.IsZero() {
		t.Fatalf("datas = %v, %v", itens[0].PublicadaEm, itens[2].PublicadaEm)
	}
	if len(srv.Pedidos()) != 1 {
		t.Fatalf("pedidos = %d", len(srv.Pedidos()))
	}
}

func TestBuscarTabNews(t *testing.T) {
	t.Parallel()
	srv := testeapoio.NovoServicoDeFora(t)
	srv.Em("GET", "/api/v1/contents", testeapoio.RespostaDeFora{Arquivo: "tabnews.json"})
	itens, err := NovoCliente(srv.Cliente()).Buscar(t.Context(), fonteDeTeste(srv, "/api/v1/contents?strategy=relevant", LerTabNews, "www.tabnews.com.br"))
	if err != nil {
		t.Fatal(err)
	}
	// O link é montado do autor e do slug, escapados; sem autor fica de fora.
	if len(itens) != 2 || itens[0].Link != "https://www.tabnews.com.br/autorficticio/artigo-ficticio-sobre-go" ||
		itens[1].Link != "https://www.tabnews.com.br/outra%2Fpessoa/outro%20artigo" {
		t.Fatalf("itens = %+v", itens)
	}
}

func TestBuscarFalhas(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		resposta testeapoio.RespostaDeFora
		ler      func([]byte) ([]Item, error)
		erro     string
	}{
		{"servidor caiu", testeapoio.ErroDoServidor, LerRSS, "respondeu 500"},
		{"RSS quebrado", testeapoio.RespostaDeFora{Corpo: "<rss><channel><item>", TipoConteudo: "application/xml"}, LerRSS, "o RSS não abriu"},
		{"JSON quebrado", testeapoio.JSONQuebrado, LerTabNews, "a resposta não abriu"},
		{"grande demais", testeapoio.RespostaDeFora{Corpo: strings.Repeat("a", tamanhoMaximo+1), TipoConteudo: "text/plain"}, LerRSS, "passou de 2 MB"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			srv := testeapoio.NovoServicoDeFora(t)
			srv.Em("GET", "/feed", c.resposta)
			_, err := NovoCliente(srv.Cliente()).Buscar(t.Context(), fonteDeTeste(srv, "/feed", c.ler, "g1.globo.com"))
			if err == nil || !strings.Contains(err.Error(), c.erro) || !strings.HasPrefix(err.Error(), "Teste: ") {
				t.Fatalf("erro = %v; quer %q", err, c.erro)
			}
		})
	}
	t.Run("exatamente 2 MB passa", func(t *testing.T) {
		t.Parallel()
		srv := testeapoio.NovoServicoDeFora(t)
		srv.Em("GET", "/feed", testeapoio.RespostaDeFora{Corpo: "[" + strings.Repeat(" ", tamanhoMaximo-2) + "]"})
		if _, err := NovoCliente(srv.Cliente()).Buscar(t.Context(), fonteDeTeste(srv, "/feed", LerTabNews)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("sem HTTPS", func(t *testing.T) {
		t.Parallel()
		_, err := NovoCliente(nil).Buscar(t.Context(), Fonte{Nome: "X", URL: "http://g1.globo.com/rss"})
		if err == nil || !strings.Contains(err.Error(), "tem de ser HTTPS") {
			t.Fatalf("erro = %v", err)
		}
	})
	t.Run("fora do ar", func(t *testing.T) {
		t.Parallel()
		_, err := NovoCliente(nil).Buscar(t.Context(), Fonte{Nome: "X", URL: "https://127.0.0.1:1/rss"})
		if err == nil || !strings.Contains(err.Error(), "não respondeu") {
			t.Fatalf("erro = %v", err)
		}
	})
	t.Run("URL inválida", func(t *testing.T) {
		t.Parallel()
		if _, err := NovoCliente(nil).Buscar(t.Context(), Fonte{Nome: "X", URL: "https://\x7f"}); err == nil {
			t.Fatal("URL inválida passou")
		}
	})
}

func TestRedirecionamento(t *testing.T) {
	t.Parallel()
	srv := testeapoio.NovoServicoDeFora(t)
	srv.Em("GET", "/feed", testeapoio.RespostaDeFora{Status: http.StatusFound, Cabecalhos: map[string]string{"Location": "https://exemplo.invalid/feed"}})
	srv.Em("GET", "/volta", testeapoio.RespostaDeFora{Status: http.StatusFound, Cabecalhos: map[string]string{"Location": "/rss"}})
	srv.Em("GET", "/rss", testeapoio.RespostaDeFora{Arquivo: "g1.xml", TipoConteudo: "application/xml"})
	srv.Em("GET", "/laco", testeapoio.RespostaDeFora{Status: http.StatusFound, Cabecalhos: map[string]string{"Location": "/laco"}})
	// Até 2 redirecionamentos: /dois → /volta → /rss passa; /tres → /dois → /volta → /rss não.
	srv.Em("GET", "/dois", testeapoio.RespostaDeFora{Status: http.StatusFound, Cabecalhos: map[string]string{"Location": "/volta"}})
	srv.Em("GET", "/tres", testeapoio.RespostaDeFora{Status: http.StatusFound, Cabecalhos: map[string]string{"Location": "/dois"}})
	c := NovoCliente(srv.Cliente())
	if _, err := c.Buscar(t.Context(), fonteDeTeste(srv, "/feed", LerRSS, "g1.globo.com")); err == nil {
		t.Fatal("seguiu o redirecionamento para fora")
	}
	if itens, err := c.Buscar(t.Context(), fonteDeTeste(srv, "/volta", LerRSS, "g1.globo.com")); err != nil || len(itens) != 3 {
		t.Fatalf("redirecionamento no mesmo host: %v, %d", err, len(itens))
	}
	if _, err := c.Buscar(t.Context(), fonteDeTeste(srv, "/dois", LerRSS, "g1.globo.com")); err != nil {
		t.Fatalf("2 redirecionamentos: %v", err)
	}
	if _, err := c.Buscar(t.Context(), fonteDeTeste(srv, "/tres", LerRSS, "g1.globo.com")); err == nil {
		t.Fatal("seguiu 3 redirecionamentos")
	}
	if _, err := c.Buscar(t.Context(), fonteDeTeste(srv, "/laco", LerRSS, "g1.globo.com")); err == nil {
		t.Fatal("seguiu o laço de redirecionamentos")
	}
}

func TestLinkPermitido(t *testing.T) {
	t.Parallel()
	dominios := []string{"g1.globo.com"}
	casos := map[string]bool{
		"https://g1.globo.com/a.ghtml":         true,
		"  https://g1.globo.com/a.ghtml  ":     true,
		"http://g1.globo.com/a.ghtml":          false,
		"javascript:alert(1)":                  false,
		"https://g1.globo.com.exemplo.invalid": false,
		"https://usuario@g1.globo.com/a":       false,
		"https://g1.globo.com:8443/a":          false,
		"https://outro.globo.com/a":            false,
		"%zz":                                  false,
	}
	for link, quer := range casos {
		if got := LinkPermitido(link, dominios); got != quer {
			t.Errorf("%q = %v; quer %v", link, got, quer)
		}
	}
}

func TestMaximoPorFonteEPadrao(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString(`<rss><channel>`)
	for i := range 25 {
		b.WriteString(`<item><title>N` + string(rune('a'+i)) + `</title><link>https://g1.globo.com/` + string(rune('a'+i)) + `</link></item>`)
	}
	b.WriteString(`</channel></rss>`)
	srv := testeapoio.NovoServicoDeFora(t)
	srv.Em("GET", "/feed", testeapoio.RespostaDeFora{Corpo: b.String(), TipoConteudo: "application/xml"})
	itens, err := NovoCliente(srv.Cliente()).Buscar(t.Context(), fonteDeTeste(srv, "/feed", LerRSS, "g1.globo.com"))
	if err != nil || len(itens) != MaximoPorFonte || itens[0].Titulo != "Na" {
		t.Fatalf("%d itens (%v)", len(itens), err)
	}
	p := Padrao()
	if got := strings.Join(Destinos(p), ","); got != "g1.globo.com,feeds.bbci.co.uk,www.tabnews.com.br" {
		t.Fatalf("destinos = %s", got)
	}
	if got := Destinos(append(p, Fonte{URL: "https://g1.globo.com/outro"}, Fonte{URL: "%zz"})); len(got) != 3 {
		t.Fatalf("destinos repetidos = %v", got)
	}
}
