package noticias

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/noticias/fontes"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/modulos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// relogioMovel deixa o teste andar o tempo (o "Atualizar agora" espera 5 minutos).
type relogioMovel struct {
	mu sync.Mutex
	t  time.Time
}

func (r *relogioMovel) Agora() time.Time      { r.mu.Lock(); defer r.mu.Unlock(); return r.t }
func (r *relogioMovel) andar(d time.Duration) { r.mu.Lock(); r.t = r.t.Add(d); r.mu.Unlock() }

type ambiente struct {
	t   *testing.T
	m   *Modulo
	mux *http.ServeMux
	srv *testeapoio.ServicoDeFora
	rel *relogioMovel
}

func novo(t *testing.T) ambiente {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	srv := testeapoio.NovoServicoDeFora(t)
	lista := []fontes.Fonte{
		{ID: "g1", Nome: "G1", URL: srv.URL() + "/g1", Ler: fontes.LerRSS, Dominios: []string{"g1.globo.com"}},
		{ID: "bbc", Nome: "BBC News Brasil", URL: srv.URL() + "/bbc", Ler: fontes.LerRSS, Dominios: []string{"www.bbc.com"}},
		{ID: "tabnews", Nome: "TabNews", URL: srv.URL() + "/tabnews", Ler: fontes.LerTabNews, Dominios: []string{"www.tabnews.com.br"}},
	}
	rel := &relogioMovel{t: time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)}
	log := slog.New(slog.DiscardHandler)
	m := NovoCom(db, fontes.NovoCliente(srv.Cliente()), lista, rel, log)
	cat := modulos.NovoCatalogo(db, rel, log)
	cat.Adicionar(m)
	mux := http.NewServeMux()
	cat.Montar(mux)
	return ambiente{t: t, m: m, mux: mux, srv: srv, rel: rel}
}

const bbcFicticio = `<rss><channel><item><title>BBC fictícia</title><link>https://www.bbc.com/portuguese/articles/ficticio</link>
	<pubDate>Sat, 14 Nov 2026 11:00:00 GMT</pubDate></item></channel></rss>`

func (a ambiente) lista() Lista {
	a.t.Helper()
	l, err := a.m.Listar(a.t.Context())
	if err != nil {
		a.t.Fatal(err)
	}
	return l
}

func TestModuloNasceDesligado(t *testing.T) {
	t.Parallel()
	a := novo(t)
	i := a.m.Info()
	if i.ID != ID || !i.AcessaInternet() || i.LigadoPorPadrao || strings.Join(i.Destinos, ",") != "g1.globo.com,feeds.bbci.co.uk,www.tabnews.com.br" {
		t.Fatalf("info = %+v", i)
	}
	if ts := a.m.Tarefas(); len(ts) != 1 || ts[0].Intervalo != 45*time.Minute {
		t.Fatalf("tarefas = %+v", ts)
	}
	// Desligado, nem a lista nem o "Atualizar agora" passam, e nada sai.
	testeapoio.Chamar(t, a.mux, "GET", "/api/noticias", "").ConferirStatus(t, http.StatusConflict)
	testeapoio.Chamar(t, a.mux, "POST", "/api/noticias/atualizar", "").ConferirStatus(t, http.StatusConflict)
	if n := len(a.srv.Pedidos()); n != 0 {
		t.Fatalf("saíram %d pedidos com o módulo desligado", n)
	}
}

func TestUmaFonteQueFalhaNaoDerrubaAsOutras(t *testing.T) {
	t.Parallel()
	a := novo(t)
	a.srv.Em("GET", "/g1", testeapoio.RespostaDeFora{Arquivo: "../fontes/testdata/g1.xml", TipoConteudo: "application/xml"})
	a.srv.Em("GET", "/bbc", testeapoio.ErroDoServidor, testeapoio.RespostaDeFora{Corpo: bbcFicticio, TipoConteudo: "application/xml"})
	a.srv.Em("GET", "/tabnews", testeapoio.RespostaDeFora{Arquivo: "../fontes/testdata/tabnews.json"}, testeapoio.ErroDoServidor)

	err := a.m.Atualizar(t.Context())
	if err == nil || !strings.Contains(err.Error(), "BBC News Brasil: respondeu 500") {
		t.Fatalf("erro = %v", err)
	}
	l := a.lista()
	// G1 (3) e TabNews (2); a BBC com o erro e sem notícia.
	if len(l.Noticias) != 5 || l.Fontes[1].Erro == nil || l.Fontes[1].AtualizadoEm != nil || l.Fontes[0].Erro != nil || l.Fontes[0].AtualizadoEm == nil {
		t.Fatalf("lista = %+v", l)
	}
	// A mais nova primeiro (G1, 9h30 em Brasília = 12h30 UTC; o TabNews é das 12h UTC), a sem data no fim.
	if l.Noticias[0].Titulo != "Notícia fictícia de hoje com espaços" || l.Noticias[1].FonteNome != "TabNews" || l.Noticias[4].PublicadaEm != nil {
		t.Fatalf("ordem = %+v", l.Noticias)
	}

	// Na próxima, a BBC volta e o TabNews cai: as notícias do TabNews de antes ficam.
	a.rel.andar(45 * time.Minute)
	if err := a.m.Atualizar(t.Context()); err == nil || !strings.Contains(err.Error(), "TabNews") {
		t.Fatalf("erro = %v", err)
	}
	l = a.lista()
	if len(l.Noticias) != 6 || l.Fontes[1].Erro != nil || l.Fontes[2].Erro == nil || l.Fontes[2].AtualizadoEm == nil {
		t.Fatalf("depois = %+v", l)
	}
}

func TestRotasLigado(t *testing.T) {
	t.Parallel()
	a := novo(t)
	a.srv.Em("GET", "/g1", testeapoio.RespostaDeFora{Arquivo: "../fontes/testdata/g1.xml", TipoConteudo: "application/xml"})
	a.srv.Em("GET", "/bbc", testeapoio.RespostaDeFora{Corpo: bbcFicticio, TipoConteudo: "application/xml"})
	a.srv.Em("GET", "/tabnews", testeapoio.RespostaDeFora{Arquivo: "../fontes/testdata/tabnews.json"})
	testeapoio.Chamar(t, a.mux, "PUT", "/api/modulos/noticias", `{"ligado":true}`).ConferirStatus(t, http.StatusOK)

	// Antes de atualizar: as fontes sem data e nenhuma notícia.
	testeapoio.Chamar(t, a.mux, "GET", "/api/noticias", "").ConferirJSON(t, `{"fontes":[
		{"id":"g1","nome":"G1","atualizadoEm":null,"erro":null},
		{"id":"bbc","nome":"BBC News Brasil","atualizadoEm":null,"erro":null},
		{"id":"tabnews","nome":"TabNews","atualizadoEm":null,"erro":null}],"noticias":[]}`)

	r := testeapoio.Chamar(t, a.mux, "POST", "/api/noticias/atualizar", "")
	r.ConferirStatus(t, http.StatusOK)
	var l Lista
	_ = json.Unmarshal(r.Corpo, &l)
	if len(l.Noticias) != 6 || l.Noticias[2].Link != "https://www.bbc.com/portuguese/articles/ficticio" {
		t.Fatalf("depois de atualizar = %+v", l.Noticias)
	}
	// De novo em menos de 5 minutos: 429, sem pedir às fontes.
	pedidos := len(a.srv.Pedidos())
	a.rel.andar(4*time.Minute + 59*time.Second)
	r = testeapoio.Chamar(t, a.mux, "POST", "/api/noticias/atualizar", "")
	r.ConferirStatus(t, http.StatusTooManyRequests)
	r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, MensagemEspera))
	if len(a.srv.Pedidos()) != pedidos {
		t.Fatal("pediu às fontes antes dos 5 minutos")
	}
	a.rel.andar(time.Second)
	testeapoio.Chamar(t, a.mux, "POST", "/api/noticias/atualizar", "").ConferirStatus(t, http.StatusOK)
}

func TestFonteQueSaiuDaListaNaoAparece(t *testing.T) {
	t.Parallel()
	a := novo(t)
	if _, err := a.m.db.ExecContext(t.Context(), `INSERT INTO "Noticia" ("Fonte", "Titulo", "Link", "AtualizadoEm")
		VALUES ('antiga', 'Fonte que saiu', 'https://exemplo.invalid', '2026-11-14 10:00:00')`); err != nil {
		t.Fatal(err)
	}
	if l := a.lista(); len(l.Noticias) != 0 {
		t.Fatalf("a fonte que saiu apareceu: %+v", l.Noticias)
	}
}

func TestNovoComAsFontesDaIssue(t *testing.T) {
	t.Parallel()
	m := Novo(testeapoio.NovoBancoVazio(t), testeapoio.RelogioDosTestes(), slog.New(slog.DiscardHandler))
	if len(m.fontes) != 3 || m.fontes[2].ID != "tabnews" {
		t.Fatalf("fontes = %+v", m.fontes)
	}
}
