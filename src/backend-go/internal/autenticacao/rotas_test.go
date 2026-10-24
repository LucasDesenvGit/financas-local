package autenticacao

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
)

// relogioDeTeste anda só quando o teste manda (bloqueio de 15 minutos sem esperar 15 minutos).
type relogioDeTeste struct {
	mu       sync.Mutex
	instante time.Time
}

func (r *relogioDeTeste) Agora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.instante
}

func (r *relogioDeTeste) andar(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instante = r.instante.Add(d)
}

// ambiente é o Go sozinho de verdade (esquema real num SQLite temporário, HTTPS) com a autenticação na frente.
type ambiente struct {
	t       *testing.T
	srv     *httptest.Server
	relogio *relogioDeTeste
	auth    *Autenticacao
}

func novoAmbiente(t *testing.T) *ambiente {
	t.Helper()
	ctx := context.Background()
	rel := &relogioDeTeste{instante: time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)}
	log := slog.New(slog.DiscardHandler)
	caminho := filepath.Join(t.TempDir(), "sql", "FinancasLocal.db")
	if err := banco.AplicarEsquema(ctx, caminho, rel, log); err != nil {
		t.Fatal(err)
	}
	db, err := banco.Abrir(ctx, caminho)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	chave, err := CarregarChave(filepath.Join(t.TempDir(), "chaves"))
	if err != nil {
		t.Fatal(err)
	}
	auth := Nova(Opcoes{DB: db, Relogio: rel, Log: log, Chave: chave, Versao: "teste", MinutosSessao: 120,
		MaximoTentativasLogin: 5, MinutosBloqueio: 15})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { semCorpo(w, http.StatusNotFound) })
	auth.Registrar(mux)
	srv := httptest.NewTLSServer(auth.Proteger(mux))
	t.Cleanup(srv.Close)
	return &ambiente{t: t, srv: srv, relogio: rel, auth: auth}
}

// navegador é uma sessão com os próprios cookies, como uma aba do Angular.
type navegador struct {
	a       *ambiente
	cliente *http.Client
}

func (a *ambiente) navegador() *navegador {
	pote, _ := cookiejar.New(nil)
	c := *a.srv.Client() // cópia: o srv.Client() é um só, e cada navegador tem os próprios cookies
	c.Jar = pote
	return &navegador{a: a, cliente: &c}
}

func (n *navegador) cookie(nome string) string {
	u, _ := url.Parse(n.a.srv.URL)
	for _, c := range n.cliente.Jar.Cookies(u) {
		if c.Name == nome {
			return c.Value
		}
	}
	return ""
}

// pedir faz a requisição; nos POST, manda o token CSRF do cookie (pedindo o estado antes, se ainda não há), como o
// HttpClient do Angular. semCsrf manda sem o cabeçalho.
func (n *navegador) pedir(metodo, caminho string, corpo any, semCsrf bool) (int, string) {
	n.a.t.Helper()
	var leitor io.Reader
	if corpo != nil {
		dados, _ := json.Marshal(corpo)
		leitor = strings.NewReader(string(dados))
	}
	r, _ := http.NewRequest(metodo, n.a.srv.URL+caminho, leitor)
	if corpo != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if metodo != http.MethodGet && !semCsrf {
		if n.cookie(CookieXsrf) == "" {
			n.pedir(http.MethodGet, "/api/auth/estado", nil, false)
		}
		r.Header.Set(CabecalhoXsrf, n.cookie(CookieXsrf))
	}
	resp, err := n.cliente.Do(r)
	if err != nil {
		n.a.t.Fatal(err)
	}
	defer resp.Body.Close()
	dados, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(dados)
}

type credenciais struct {
	Login string `json:"login"`
	Senha string `json:"senha"`
}

func (n *navegador) esperar(t *testing.T, metodo, caminho string, corpo any, status int, trecho string) {
	t.Helper()
	got, texto := n.pedir(metodo, caminho, corpo, false)
	if got != status || !strings.Contains(texto, trecho) {
		t.Fatalf("%s %s: %d %q, esperado %d com %q", metodo, caminho, got, texto, status, trecho)
	}
}

func TestLoginBloqueioEDesbloqueio(t *testing.T) {
	a := novoAmbiente(t)
	a.auth.limite = novoLimitador(1000, time.Minute) // aqui o assunto é o bloqueio por usuário, não o limite por IP
	dono := a.navegador()
	dono.esperar(t, "POST", "/api/auth/configurar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"autenticado":true`)

	outro := a.navegador()
	outro.esperar(t, "POST", "/api/auth/entrar", credenciais{"ana.teste", "errada-123"}, 401, "Usuário ou senha inválidos.")
	for i := 2; i <= 5; i++ {
		outro.esperar(t, "POST", "/api/auth/entrar", credenciais{"ana.teste", "errada-123"}, 401, "inválidos")
	}
	// A 5ª errada bloqueou: até a senha certa recebe 429, com os minutos que faltam.
	outro.esperar(t, "POST", "/api/auth/entrar", credenciais{"ana.teste", "Senha-Forte-123"}, 429, "Tente novamente em 15 minuto(s).")
	a.relogio.andar(14*time.Minute + 30*time.Second)
	outro.esperar(t, "POST", "/api/auth/entrar", credenciais{"ana.teste", "Senha-Forte-123"}, 429, "Tente novamente em 1 minuto(s).")
	// O bloqueio não derruba a sessão que já estava aberta.
	dono.esperar(t, "GET", "/api/auth/estado", nil, 200, `"autenticado":true`)

	a.relogio.andar(time.Minute)
	outro.esperar(t, "POST", "/api/auth/entrar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"login":"ana.teste"`)
	outro.esperar(t, "GET", "/api/auth/auditoria", nil, 200, `"evento":"LoginBloqueado"`)
}

func TestLimiteDeCincoPorMinutoPorIP(t *testing.T) {
	a := novoAmbiente(t)
	n := a.navegador()
	for i := 1; i <= 5; i++ {
		if status, _ := n.pedir("POST", "/api/auth/entrar", credenciais{"nao.existe", "Senha-Forte-123"}, false); status != 401 {
			t.Fatalf("%dª: %d", i, status)
		}
	}
	// A 6ª no mesmo minuto: 429 sem corpo, antes de olhar usuário, senha ou CSRF.
	if status, corpo := n.pedir("POST", "/api/auth/entrar", credenciais{"nao.existe", "x"}, true); status != 429 || corpo != "" {
		t.Errorf("6ª: %d %q", status, corpo)
	}
}

func TestPostSemTokenCsrf(t *testing.T) {
	a := novoAmbiente(t)
	n := a.navegador()
	n.pedir("GET", "/api/auth/estado", nil, false)
	status, corpo := n.pedir("POST", "/api/auth/configurar", credenciais{"ana.teste", "Senha-Forte-123"}, true)
	if status != 400 || !strings.Contains(corpo, `"title":"Bad Request"`) {
		t.Errorf("%d %s", status, corpo)
	}
	// O token do anônimo não vale depois do login: o Angular pega o novo no cookie que o login devolve.
	n.esperar(t, "POST", "/api/auth/configurar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"autenticado":true`)
	antigo := tokenCsrf(a.auth.o.Chave, n.cookie(CookieAntifalsificacao), 0)
	r, _ := http.NewRequest("POST", a.srv.URL+"/api/auth/sair", nil)
	r.Header.Set(CabecalhoXsrf, antigo)
	resp, err := n.cliente.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("token de antes do login: %d", resp.StatusCode)
	}
}

func TestTrocaDeSenhaDerrubaAOutraSessao(t *testing.T) {
	a := novoAmbiente(t)
	casa := a.navegador()
	casa.esperar(t, "POST", "/api/auth/configurar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"autenticado":true`)
	celular := a.navegador()
	celular.esperar(t, "POST", "/api/auth/entrar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"autenticado":true`)

	casa.esperar(t, "POST", "/api/auth/alterar-senha", map[string]string{"senhaAtual": "Senha-Forte-123", "novaSenha": "Outra-Senha-456"}, 204, "")
	casa.esperar(t, "GET", "/api/auth/estado", nil, 200, `"autenticado":true`)
	celular.esperar(t, "GET", "/api/auth/estado", nil, 200, `"autenticado":false`)
	if status, _ := celular.pedir("GET", "/api/auth/auditoria", nil, false); status != 401 {
		t.Errorf("sessão derrubada ainda vê a auditoria: %d", status)
	}
	celular.esperar(t, "POST", "/api/auth/entrar", credenciais{"ana.teste", "Senha-Forte-123"}, 401, "inválidos")
	celular.esperar(t, "POST", "/api/auth/entrar", credenciais{"ana.teste", "Outra-Senha-456"}, 200, `"autenticado":true`)
}

func TestSessaoExpiraERenova(t *testing.T) {
	a := novoAmbiente(t)
	n := a.navegador()
	n.esperar(t, "POST", "/api/auth/configurar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"autenticado":true`)
	antes := n.cookie(CookieSessao)
	a.relogio.andar(61 * time.Minute) // passou da metade: a próxima requisição renova a janela
	n.esperar(t, "GET", "/api/auth/estado", nil, 200, `"autenticado":true`)
	if n.cookie(CookieSessao) == antes {
		t.Error("não renovou depois da metade da janela")
	}
	a.relogio.andar(119 * time.Minute)
	n.esperar(t, "GET", "/api/auth/estado", nil, 200, `"autenticado":true`)
	a.relogio.andar(121 * time.Minute)
	n.esperar(t, "GET", "/api/auth/estado", nil, 200, `"autenticado":false`)
}

func TestRotaQueNaoEAnonimaExigeLogin(t *testing.T) {
	a := novoAmbiente(t)
	n := a.navegador()
	for _, c := range []struct{ metodo, caminho string }{
		{"GET", "/api/auth/auditoria"}, {"POST", "/api/auth/alterar-senha"}, {"GET", "/api/contas"}, {"GET", "/api/go/saude"},
	} {
		if status, corpo := n.pedir(c.metodo, c.caminho, nil, true); status != 401 || corpo != "" {
			t.Errorf("%s %s: %d %q", c.metodo, c.caminho, status, corpo)
		}
	}
	// Logado, a rota que não existe é 404, sem pedir CSRF (no .NET é o MapFallback, que não é controller).
	n.esperar(t, "POST", "/api/auth/configurar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"autenticado":true`)
	if status, _ := n.pedir("POST", "/api/nao-existe", map[string]string{}, true); status != 404 {
		t.Errorf("rota inexistente: %d", status)
	}
}
