package testeapoio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// anotador é um testing.TB que guarda os erros em vez de falhar: para conferir que o serviço falso reclama.
type anotador struct {
	testing.TB
	mu    sync.Mutex
	erros []string
}

func (a *anotador) Errorf(formato string, args ...any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.erros = append(a.erros, fmt.Sprintf(formato, args...))
}

func (a *anotador) lidos() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.erros...)
}

func pedir(t *testing.T, s *ServicoDeFora, metodo, caminho, corpo string, cabecalhos map[string]string) (int, string, http.Header) {
	t.Helper()
	var leitor io.Reader
	if corpo != "" {
		leitor = strings.NewReader(corpo)
	}
	req, err := http.NewRequestWithContext(t.Context(), metodo, s.URL()+caminho, leitor)
	if err != nil {
		t.Fatal(err)
	}
	for nome, valor := range cabecalhos {
		req.Header.Set(nome, valor)
	}
	resp, err := s.Cliente().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	dados, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(dados), resp.Header
}

// Regra: o serviço falso responde pela rota (método e caminho, sem a query), com o arquivo de testdata/ ou o corpo, as
// respostas em sequência (a última se repete), e guarda os pedidos com a query e o corpo.
func TestServicoDeForaResponde(t *testing.T) {
	s := NovoServicoDeFora(t)
	s.Em("GET", "/v1/itens", RespostaDeFora{Corpo: `{"n":1}`}, RespostaDeFora{Status: 202, Corpo: `{"n":2}`, TipoConteudo: "text/plain",
		Cabecalhos: map[string]string{"X-Pagina": "2"}})
	s.Em("POST", "/auth", RespostaDeFora{Arquivo: "servico.json"})
	casos := []struct {
		metodo, caminho string
		status          int
		corpo, tipo     string
	}{
		{"GET", "/v1/itens?pagina=1", 200, `{"n":1}`, "application/json; charset=utf-8"},
		{"GET", "/v1/itens?pagina=2", 202, `{"n":2}`, "text/plain"},
		{"GET", "/v1/itens", 202, `{"n":2}`, "text/plain"},
		{"POST", "/auth", 200, "{\"accessToken\": \"token-ficticio\"}\n", "application/json; charset=utf-8"},
	}
	for _, c := range casos {
		status, corpo, cabecalhos := pedir(t, s, c.metodo, c.caminho, `{"clientId":"x"}`, nil)
		if status != c.status || corpo != c.corpo || cabecalhos.Get("Content-Type") != c.tipo {
			t.Errorf("%s %s: %d %q %q", c.metodo, c.caminho, status, corpo, cabecalhos.Get("Content-Type"))
		}
	}
	pedidos := s.Pedidos()
	if len(pedidos) != 4 || pedidos[0].URI != "/v1/itens?pagina=1" || pedidos[0].Caminho != "/v1/itens" ||
		pedidos[0].Query.Get("pagina") != "1" || pedidos[3].Corpo != `{"clientId":"x"}` {
		t.Errorf("pedidos: %+v", pedidos)
	}
	if uris := s.URIs(); strings.Join(uris, " ") != "/v1/itens?pagina=1 /v1/itens?pagina=2 /v1/itens /auth" {
		t.Errorf("URIs: %v", uris)
	}
	// O curinga vale para os caminhos sem rota própria, e a rota própria continua valendo.
	s.Em("GET", "*", RespostaDeFora{Corpo: `{"curinga":true}`})
	if _, corpo, _ := pedir(t, s, "GET", "/qualquer(x=1)?y=2", "", nil); corpo != `{"curinga":true}` {
		t.Errorf("curinga: %q", corpo)
	}
	if _, corpo, _ := pedir(t, s, "GET", "/v1/itens", "", nil); corpo != `{"n":2}` {
		t.Errorf("a rota própria antes do curinga: %q", corpo)
	}
}

// Regra: o valor dos cabeçalhos de segredo não é guardado; os cabeçalhos exigidos são conferidos em todo pedido; o
// pedido sem rota é erro do teste (404).
func TestServicoDeForaConfere(t *testing.T) {
	a := &anotador{TB: t}
	s := NovoServicoDeFora(a, "X-API-KEY")
	s.Exigir("User-Agent", "FinancasLocal/1.0")
	s.Em("GET", "/contas", RespostaDeFora{Corpo: `[]`})
	pedir(t, s, "GET", "/contas", "", map[string]string{"User-Agent": "FinancasLocal/1.0", "X-API-KEY": "chave-ficticia"})
	pedir(t, s, "GET", "/contas", "", map[string]string{"User-Agent": "outro"})
	status, _, _ := pedir(t, s, "GET", "/naoexiste", "", map[string]string{"User-Agent": "FinancasLocal/1.0"})
	if status != http.StatusNotFound {
		t.Errorf("sem rota: %d", status)
	}
	pedidos := s.Pedidos()
	if !pedidos[0].TemSegredo("X-API-KEY") || pedidos[1].TemSegredo("X-API-KEY") || strings.Contains(fmt.Sprint(pedidos), "chave-ficticia") {
		t.Errorf("segredo: %v", pedidos)
	}
	erros := a.lidos()
	if len(erros) != 2 || !strings.Contains(erros[0], `cabeçalho User-Agent "outro", esperado "FinancasLocal/1.0"`) ||
		!strings.Contains(erros[1], "sem rota cadastrada no serviço falso: GET /naoexiste") {
		t.Errorf("erros: %v", erros)
	}
}

// Regra: os erros prontos: o tempo esgotado (o serviço só responde quando o cliente desiste), 401, 429 (com o
// Retry-After), 500, o JSON quebrado e a página vazia.
func TestServicoDeForaComErros(t *testing.T) {
	s := NovoServicoDeFora(t)
	s.Em("GET", "/lento", TempoEsgotado)
	cliente := s.Cliente()
	cliente.Timeout = 50 * time.Millisecond
	if _, err := cliente.Get(s.URL() + "/lento"); err == nil || !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "Timeout") {
		t.Errorf("tempo esgotado: %v", err)
	}
	casos := []struct {
		resposta RespostaDeFora
		status   int
		corpo    string
	}{
		{NaoAutorizado, 401, `{"message":"Unauthorized"}`},
		{MuitosPedidos, 429, `{"message":"Too Many Requests"}`},
		{ErroDoServidor, 500, `{"message":"Internal Server Error"}`},
		{JSONQuebrado, 200, `{"results": [{"id": `},
		{PaginaVazia("results"), 200, `{"results":[]}`},
	}
	for i, c := range casos {
		caminho := fmt.Sprintf("/erro%d", i)
		s.Em("GET", caminho, c.resposta)
		status, corpo, cabecalhos := pedir(t, s, "GET", caminho, "", nil)
		if status != c.status || corpo != c.corpo {
			t.Errorf("%s: %d %q", caminho, status, corpo)
		}
		if c.status == 429 && cabecalhos.Get("Retry-After") != "1" {
			t.Errorf("429 sem o Retry-After")
		}
	}
}

// Regra: rota sem resposta e arquivo que não existe são erro do teste na hora de cadastrar.
func TestServicoDeForaRecusaCadastroErrado(t *testing.T) {
	for _, cadastrar := range []func(*ServicoDeFora){
		func(s *ServicoDeFora) { s.Em("GET", "/x") },
		func(s *ServicoDeFora) { s.Em("GET", "/x", RespostaDeFora{Arquivo: "naoexiste.json"}) },
	} {
		a := &fatal{anotador: anotador{TB: t}}
		s := NovoServicoDeFora(a)
		func() {
			defer func() { _ = recover() }()
			cadastrar(s)
		}()
		if !a.falhou {
			t.Error("cadastro errado aceito")
		}
	}
}

// fatal é o anotador que também anota o Fatal (e interrompe, como o de verdade).
type fatal struct {
	anotador
	falhou bool
}

func (f *fatal) Fatal(args ...any)                  { f.falhou = true; panic("fatal") }
func (f *fatal) Fatalf(formato string, args ...any) { f.falhou = true; panic("fatal") }
