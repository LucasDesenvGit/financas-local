package testeapoio

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// RespostaDeFora é o que o serviço falso devolve a um pedido: o corpo vem de um arquivo de testdata/ (Arquivo) ou do próprio
// teste (Corpo). Status 0 é o 200, e TipoConteudo vazio é JSON. Com Travar, o serviço não responde: espera o cliente
// desistir (o tempo esgotado).
type RespostaDeFora struct {
	Status       int
	Arquivo      string
	Corpo        string
	TipoConteudo string
	Cabecalhos   map[string]string
	Travar       bool
}

// Os erros prontos dos serviços de fora (#460).
var (
	TempoEsgotado  = RespostaDeFora{Travar: true}
	NaoAutorizado  = RespostaDeFora{Status: http.StatusUnauthorized, Corpo: `{"message":"Unauthorized"}`}
	MuitosPedidos  = RespostaDeFora{Status: http.StatusTooManyRequests, Corpo: `{"message":"Too Many Requests"}`, Cabecalhos: map[string]string{"Retry-After": "1"}}
	ErroDoServidor = RespostaDeFora{Status: http.StatusInternalServerError, Corpo: `{"message":"Internal Server Error"}`}
	JSONQuebrado   = RespostaDeFora{Corpo: `{"results": [{"id": `}
)

// PaginaVazia é a resposta sem nenhum item, no campo da lista do serviço ("value" na PTAX, "results" na Pluggy).
func PaginaVazia(campo string) RespostaDeFora { return RespostaDeFora{Corpo: `{"` + campo + `":[]}`} }

// Pedido é o que o serviço falso recebeu. Os cabeçalhos de segredo (NovoServicoDeFora) chegam com o valor trocado por
// "<segredo>": o teste confere que vieram sem que o valor apareça numa mensagem de erro.
type Pedido struct {
	Metodo     string
	Caminho    string // sem a query
	URI        string // caminho e query, como o servidor recebeu
	Query      url.Values
	Cabecalhos http.Header
	Corpo      string
}

// ServicoDeFora é um serviço de fora (a PTAX, o Mercado Bitcoin, o Yahoo, a brapi, o BCB, a Pluggy) num servidor HTTPS
// de teste: responde a cada pedido pela rota cadastrada (o método e o caminho, sem a query) e guarda o que recebeu para
// o teste conferir. Pedido sem rota é erro do teste, e o serviço responde 404. Nada sai para a rede (ADR 0051).
//
// As respostas são fictícias, no formato real de cada serviço: nunca a resposta de uma conta de verdade, porque o
// repositório é público.
type ServicoDeFora struct {
	t        testing.TB
	srv      *httptest.Server
	segredos []string

	mu       sync.Mutex
	rotas    map[string][]RespostaDeFora
	exigidos map[string]string
	pedidos  []Pedido
}

// NovoServicoDeFora sobe o serviço falso (fechado no fim do teste). segredos são os cabeçalhos que levam segredo
// (Authorization, X-API-KEY): o valor deles não é guardado.
func NovoServicoDeFora(t testing.TB, segredos ...string) *ServicoDeFora {
	t.Helper()
	s := &ServicoDeFora{t: t, segredos: segredos, rotas: map[string][]RespostaDeFora{}, exigidos: map[string]string{}}
	s.srv = httptest.NewTLSServer(http.HandlerFunc(s.atender))
	t.Cleanup(s.srv.Close)
	return s
}

// URL é o endereço do serviço falso (https://127.0.0.1:porta), para o cliente montar a URL base.
func (s *ServicoDeFora) URL() string { return s.srv.URL }

// Cliente é um http.Client que confia no certificado do serviço falso. Mexer nele (o Timeout) não muda os outros.
func (s *ServicoDeFora) Cliente() *http.Client { return s.srv.Client() }

// Em cadastra as respostas de uma rota ("GET", "/api/v4/tickers"), em sequência: o primeiro pedido recebe a primeira,
// o segundo a segunda, e a última se repete. O caminho "*" vale para todo caminho do método que não tem rota própria
// (o teste confere o endereço pelos Pedidos).
func (s *ServicoDeFora) Em(metodo, caminho string, respostas ...RespostaDeFora) {
	s.t.Helper()
	if len(respostas) == 0 {
		s.t.Fatal("Em sem resposta")
	}
	for _, r := range respostas {
		if r.Arquivo != "" {
			if _, err := os.Stat(filepath.Join("testdata", r.Arquivo)); err != nil {
				s.t.Fatalf("resposta de %s %s: %v", metodo, caminho, err)
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rotas[metodo+" "+caminho] = respostas
}

// Exigir confere um cabeçalho em todo pedido (o User-Agent e o Accept que o .NET manda): diferente é erro do teste.
func (s *ServicoDeFora) Exigir(cabecalho, valor string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exigidos[cabecalho] = valor
}

// Pedidos devolve os pedidos recebidos, na ordem.
func (s *ServicoDeFora) Pedidos() []Pedido {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Pedido(nil), s.pedidos...)
}

// URIs devolve o caminho e a query de cada pedido recebido, na ordem.
func (s *ServicoDeFora) URIs() []string {
	var lista []string
	for _, p := range s.Pedidos() {
		lista = append(lista, p.URI)
	}
	return lista
}

func (s *ServicoDeFora) atender(w http.ResponseWriter, r *http.Request) {
	corpo, _ := io.ReadAll(r.Body)
	cabecalhos := r.Header.Clone()
	for _, segredo := range s.segredos {
		if cabecalhos.Get(segredo) != "" {
			cabecalhos.Set(segredo, "<segredo>")
		}
	}
	s.mu.Lock()
	s.pedidos = append(s.pedidos, Pedido{Metodo: r.Method, Caminho: r.URL.Path, URI: r.URL.RequestURI(), Query: r.URL.Query(),
		Cabecalhos: cabecalhos, Corpo: string(corpo)})
	for nome, valor := range s.exigidos {
		if obtido := r.Header.Get(nome); obtido != valor {
			s.t.Errorf("%s %s: cabeçalho %s %q, esperado %q", r.Method, r.URL.Path, nome, obtido, valor)
		}
	}
	chave := r.Method + " " + r.URL.Path
	respostas, ok := s.rotas[chave]
	if !ok {
		chave = r.Method + " *"
		respostas, ok = s.rotas[chave]
	}
	var resposta RespostaDeFora
	if ok {
		resposta = respostas[0]
		if len(respostas) > 1 {
			s.rotas[chave] = respostas[1:]
		}
	}
	s.mu.Unlock()
	if !ok {
		s.t.Errorf("pedido sem rota cadastrada no serviço falso: %s %s", r.Method, r.URL.RequestURI())
		http.NotFound(w, r)
		return
	}
	if resposta.Travar {
		<-r.Context().Done()
		return
	}
	dados := []byte(resposta.Corpo)
	if resposta.Arquivo != "" {
		var err error
		if dados, err = os.ReadFile(filepath.Join("testdata", resposta.Arquivo)); err != nil {
			s.t.Errorf("resposta de %s: %v", chave, err)
		}
	}
	tipo := resposta.TipoConteudo
	if tipo == "" {
		tipo = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", tipo)
	for nome, valor := range resposta.Cabecalhos {
		w.Header().Set(nome, valor)
	}
	status := resposta.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(dados)
}

// TemSegredo diz se o pedido levou o cabeçalho de segredo (com o valor que for).
func (p Pedido) TemSegredo(cabecalho string) bool {
	return strings.EqualFold(p.Cabecalhos.Get(cabecalho), "<segredo>")
}
