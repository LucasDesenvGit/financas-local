// Pacote autenticacao é o AuthController e a segurança do Program.cs em Go (#75): login, sessão em cookie assinado,
// CSRF, limite de tentativas por IP, bloqueio por usuário e auditoria, com o mesmo contrato do .NET (#72).
//
// Só vale com o Go sozinho, sem o .NET na frente: até a inversão (#85, ADR 0049) quem faz o login no site é o .NET,
// e o Go atrás dele recebe o usuário pelo cabeçalho interno (servidor.RodarAtrasDoDotnet).
package autenticacao

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/erros"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/sessao"
)

// Opcoes da autenticação (as do Program.cs e do ParametroService).
type Opcoes struct {
	DB      *sql.DB
	Relogio relogio.Relogio
	Log     *slog.Logger
	Chave   []byte // CarregarChave(Seguranca:PastaChaves)
	Versao  string // a do sistema, no "estado" (EstadoAutenticacaoDto.Versao)

	MinutosSessao int // Seguranca:MinutosSessao (120)
	// Padrões dos parâmetros de login; o valor gravado em Configuracao vale mais (ParametroService).
	MaximoTentativasLogin int // 5
	MinutosBloqueio       int // 15
}

// Autenticacao guarda o estado que vive o processo todo: o limitador por IP e o hash fictício.
type Autenticacao struct {
	o       Opcoes
	q       *consultas.Queries
	limite  *limitador
	duracao time.Duration

	hashFicticioUmaVez sync.Once
	hashFicticio       string
}

// Nova cria a autenticação.
func Nova(o Opcoes) *Autenticacao {
	return &Autenticacao{
		o:       o,
		q:       consultas.New(o.DB),
		limite:  novoLimitador(5, time.Minute),
		duracao: time.Duration(o.MinutosSessao) * time.Minute,
	}
}

// Rotas de api/auth que não exigem login (AllowAnonymous), mas os POST exigem CSRF, como no .NET. A autoridade do
// acesso pelo celular (#328) também: o celular a baixa antes de confiar no site e de entrar, e ela é só o certificado
// público (o pacote celular a serve só com o acesso ligado). E o estado dele, para o gerenciador, que não tem login
// (#679): só ligado e o endereço, e só para quem pede do próprio computador.
var rotasAnonimas = map[string]bool{
	"GET /api/auth/estado":        true,
	"POST /api/auth/configurar":   true,
	"POST /api/auth/entrar":       true,
	"POST /api/auth/sair":         true,
	"GET /api/celular/autoridade": true,
	"GET /api/celular/estado":     true, // #679: só para o 127.0.0.1 (o pacote celular confere)
	// #684: o gerenciador liga e desliga; só do 127.0.0.1 e com o segredo dele (o pacote celular confere).
	"POST /api/celular/gerenciador/ligar":    true,
	"POST /api/celular/gerenciador/desligar": true,
}

// Rotas sem CSRF (#684): as do gerenciador, que não tem sessão nem o cookie do CSRF. O que as protege é o segredo num
// cabeçalho próprio, que um site de fora nem consegue mandar (o navegador pede o preflight do CORS, que o site recusa).
var rotasSemCsrf = map[string]bool{
	"POST /api/celular/gerenciador/ligar":    true,
	"POST /api/celular/gerenciador/desligar": true,
}

// Rotas da política "login" (EnableRateLimiting): 5 por minuto por IP, somadas.
var rotasLimitadas = map[string]bool{
	"POST /api/auth/configurar": true,
	"POST /api/auth/entrar":     true,
}

// requisicao é o que o intermediário descobriu sobre quem pede: a sessão (nil se anônimo) e o segredo do CSRF.
type requisicao struct {
	sessao  *conteudoSessao
	segredo string
}

type chaveRequisicao struct{}

func daRequisicao(ctx context.Context) *requisicao {
	if r, ok := ctx.Value(chaveRequisicao{}).(*requisicao); ok {
		return r
	}
	return &requisicao{}
}

func (r *requisicao) usuario() int64 {
	if r.sessao == nil {
		return 0
	}
	return r.sessao.Usuario
}

// Proteger é o UseRateLimiter, o UseAuthentication e o UseAuthorization do Program.cs, mais o
// AutoValidateAntiforgeryToken, nessa ordem, na frente das rotas de /api:
//  1. "entrar" e "configurar" contam no limite por IP (429 sem corpo, antes de tudo);
//  2. o cookie da sessão é conferido (assinatura, validade e o carimbo no banco); carimbo trocado derruba a sessão;
//  3. rota que não é anônima exige login: 401 sem corpo, e não redirecionamento (OnRedirectToLogin);
//  4. método que muda algo exige o token CSRF do usuário (400 em ProblemDetails). O 404 de rota que não existe não
//     confere (no .NET é o MapFallback, que não é controller).
func (a *Autenticacao) Proteger(rotas *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chaveRota := r.Method + " " + strings.ToLower(r.URL.Path)
		if rotasLimitadas[chaveRota] && !a.limite.permitir(auditoria.IP(r)) {
			respostas.SemCorpo(w, http.StatusTooManyRequests)
			return
		}

		req := &requisicao{}
		if c, err := r.Cookie(CookieAntifalsificacao); err == nil {
			req.segredo = c.Value
		}
		if c, err := r.Cookie(CookieSessao); err == nil && c.Value != "" {
			agora := a.o.Relogio.Agora()
			if s, ok := lerSessao(a.o.Chave, c.Value, agora); ok {
				valida, err := a.q.SessaoValida(r.Context(), consultas.SessaoValidaParams{Id: s.Usuario, CarimboSeguranca: s.Carimbo})
				if err != nil {
					a.erroInterno(w, r, "conferir a sessão", err)
					return
				}
				if !valida {
					// O ValidadorSessao do .NET: a senha mudou (ou o usuário saiu do banco), então a sessão acaba.
					expirarSessao(w)
				} else {
					if s.precisaRenovar(agora) {
						s.Emitido, s.Expira = agora.Unix(), agora.Add(a.duracao).Unix()
						definirSessao(w, a.o.Chave, s)
					}
					req.sessao = &s
				}
			}
		}

		if req.sessao == nil && !rotasAnonimas[chaveRota] {
			respostas.SemCorpo(w, http.StatusUnauthorized)
			return
		}
		if !metodoSeguro(r.Method) && !rotasSemCsrf[chaveRota] {
			if _, padrao := rotas.Handler(r); padrao != "/" && !csrfValido(a.o.Chave, req.segredo, r.Header.Get(CabecalhoXsrf), req.usuario()) {
				respostas.CsrfRecusado(w)
				return
			}
		}

		// A rota e o login vão para o contexto: um erro logado durante a requisição os leva para a tabela de erros
		// (#298), como o HttpContext no .NET.
		var login *string
		if req.sessao != nil {
			login = &req.sessao.Login
		}
		ctx := erros.ComRequisicao(r.Context(), r.Method, r.URL.Path, login)
		ctx = context.WithValue(ctx, chaveRequisicao{}, req)
		if req.sessao != nil {
			ctx = sessao.ComUsuario(ctx, req.sessao.Usuario)
		}
		rotas.ServeHTTP(w, r.WithContext(ctx))
	})
}

// metodoSeguro são os métodos que o antiforgery do ASP.NET não confere.
func metodoSeguro(metodo string) bool {
	switch metodo {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}

// erroInterno é o UseExceptionHandler: o erro vai para o log, e quem pediu recebe só o 500 de sempre.
func (a *Autenticacao) erroInterno(w http.ResponseWriter, r *http.Request, oque string, err error) {
	a.o.Log.ErrorContext(r.Context(), "erro na autenticação", "origem", erros.OrigemAPI, "etapa", oque, "erro", err)
	servidor.ResponderErroInterno(w)
}
