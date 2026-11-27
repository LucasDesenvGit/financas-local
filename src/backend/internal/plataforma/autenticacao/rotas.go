package autenticacao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Mensagens e limites do AuthController e da ConstantesSeguranca.
const (
	tamanhoMinimoSenha    = 8
	mensagemFalhaLogin    = "Usuário ou senha inválidos."
	chaveMaximoTentativas = "Seguranca.MaximoTentativasLogin"
	chaveMinutosBloqueio  = "Seguranca.MinutosBloqueio"
)

// estadoDto é o EstadoAutenticacaoDto.
type estadoDto struct {
	Configurado bool    `json:"configurado"`
	Autenticado bool    `json:"autenticado"`
	Login       *string `json:"login"`
	Versao      string  `json:"versao"`
}

// auditoriaDto é a AuditoriaAcesso a devolve (a entidade, em camelCase).
type auditoriaDto struct {
	Id        int64          `json:"id"`
	DataHora  tipos.DataHora `json:"dataHora"`
	Evento    string         `json:"evento"`
	Login     *string        `json:"login"`
	Ip        *string        `json:"ip"`
	UserAgent *string        `json:"userAgent"`
}

// Registrar põe as 6 rotas de api/auth no mux.
func (a *Autenticacao) Registrar(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/estado", a.estado)
	mux.HandleFunc("POST /api/auth/configurar", a.configurar)
	mux.HandleFunc("POST /api/auth/entrar", a.entrar)
	mux.HandleFunc("POST /api/auth/sair", a.sair)
	mux.HandleFunc("POST /api/auth/alterar-senha", a.alterarSenha)
	mux.HandleFunc("GET /api/auth/auditoria", a.auditoria)
}

// estado diz se já há usuário e se a sessão está ativa, e emite o token CSRF.
func (a *Autenticacao) estado(w http.ResponseWriter, r *http.Request) {
	req := daRequisicao(r.Context())
	configurado, err := existeUsuario(r.Context(), a.o.DB)
	if err != nil {
		a.erroInterno(w, r, "estado", err)
		return
	}
	if err := a.emitirCsrf(w, req); err != nil {
		a.erroInterno(w, r, "emitir o CSRF", err)
		return
	}
	resposta := estadoDto{Configurado: configurado, Versao: a.o.Versao}
	if req.sessao != nil {
		login := req.sessao.Login
		resposta.Autenticado, resposta.Login = true, &login
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, resposta)
}

// configurar cria o único usuário do sistema; só funciona enquanto não houver nenhum.
func (a *Autenticacao) configurar(w http.ResponseWriter, r *http.Request) {
	login := &respostas.CampoTexto{Propriedade: "Login", Minimo: 3, Maximo: 100}
	senha := &respostas.CampoTexto{Propriedade: "Senha", Maximo: 200}
	if !respostas.LerTextos(w, r, login, senha) {
		return
	}
	ctx := r.Context()
	existe, err := existeUsuario(ctx, a.o.DB)
	if err != nil {
		a.erroInterno(w, r, "configurar", err)
		return
	}
	if existe {
		respostas.Mensagem(w, http.StatusConflict, "O sistema já foi configurado.")
		return
	}
	loginLimpo := strings.TrimSpace(*login.Valor)
	if problema := validarSenha(*senha.Valor, loginLimpo); problema != "" {
		respostas.Mensagem(w, http.StatusBadRequest, problema)
		return
	}
	hash, err := GerarHash(*senha.Valor)
	if err != nil {
		a.erroInterno(w, r, "gerar o hash", err)
		return
	}
	agora := a.agoraDb()
	usuario := Usuario{Login: loginLimpo, SenhaHash: hash, CarimboSeguranca: tipos.NovoGuid(), UltimoLoginEm: &agora, CriadoEm: agora}
	err = banco.EmTransacao(ctx, a.o.DB, func(tx *sql.Tx) error {
		id, err := inserirUsuario(ctx, tx, usuario)
		usuario.Id = id
		if err != nil {
			return err
		}
		return a.auditar(ctx, tx, r, "Configuracao", &loginLimpo)
	})
	if err != nil {
		a.erroInterno(w, r, "configurar", err)
		return
	}
	if err := a.iniciarSessao(w, daRequisicao(ctx), usuario); err != nil {
		a.erroInterno(w, r, "iniciar a sessão", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, estadoDto{Configurado: true, Autenticado: true, Login: &usuario.Login, Versao: a.o.Versao})
}

// entrar confere usuário e senha, com o bloqueio por tentativas e a auditoria.
func (a *Autenticacao) entrar(w http.ResponseWriter, r *http.Request) {
	login := &respostas.CampoTexto{Propriedade: "Login", Minimo: 3, Maximo: 100}
	senha := &respostas.CampoTexto{Propriedade: "Senha", Maximo: 200}
	if !respostas.LerTextos(w, r, login, senha) {
		return
	}
	ctx := r.Context()
	loginLimpo := strings.TrimSpace(*login.Valor)
	usuario, err := usuarioPorLogin(ctx, a.o.DB, loginLimpo)
	if errors.Is(err, sql.ErrNoRows) {
		// Confere a senha mesmo assim, contra um hash qualquer: o tempo da resposta não revela se o usuário existe.
		VerificarHash(a.hashDeMentira(), *senha.Valor)
		if err := a.auditar(ctx, a.o.DB, r, "LoginFalha", &loginLimpo); err != nil {
			a.erroInterno(w, r, "auditar", err)
			return
		}
		respostas.Mensagem(w, http.StatusUnauthorized, mensagemFalhaLogin)
		return
	}
	if err != nil {
		a.erroInterno(w, r, "entrar", err)
		return
	}

	agora := a.o.Relogio.Agora()
	if usuario.BloqueadoAte != nil && usuario.BloqueadoAte.Tempo().After(agora) {
		if err := a.auditar(ctx, a.o.DB, r, "LoginBloqueado", &loginLimpo); err != nil {
			a.erroInterno(w, r, "auditar", err)
			return
		}
		minutos := max(1, int(math.Ceil(usuario.BloqueadoAte.Tempo().Sub(agora).Minutes())))
		respostas.Mensagem(w, http.StatusTooManyRequests,
			fmt.Sprintf("Acesso bloqueado temporariamente por excesso de tentativas. Tente novamente em %d minuto(s).", minutos))
		return
	}

	resultado := VerificarHash(usuario.SenhaHash, *senha.Valor)
	if resultado == SenhaErrada {
		usuario.TentativasFalhas++
		maximo, minutosBloqueio, err := a.parametros(ctx)
		if err != nil {
			a.erroInterno(w, r, "ler os parâmetros", err)
			return
		}
		if usuario.TentativasFalhas >= maximo {
			ate := tipos.NovaDataHora(agora.Add(time.Duration(minutosBloqueio) * time.Minute))
			usuario.BloqueadoAte = &ate
			usuario.TentativasFalhas = 0
		}
		if err := a.salvarComAuditoria(ctx, r, usuario, "LoginFalha", &loginLimpo); err != nil {
			a.erroInterno(w, r, "entrar", err)
			return
		}
		respostas.Mensagem(w, http.StatusUnauthorized, mensagemFalhaLogin)
		return
	}

	if resultado == SenhaCertaRefazer {
		if usuario.SenhaHash, err = GerarHash(*senha.Valor); err != nil {
			a.erroInterno(w, r, "refazer o hash", err)
			return
		}
	}
	ultimo := tipos.NovaDataHora(agora)
	usuario.TentativasFalhas, usuario.BloqueadoAte, usuario.UltimoLoginEm = 0, nil, &ultimo
	if err := a.salvarComAuditoria(ctx, r, usuario, "LoginSucesso", &loginLimpo); err != nil {
		a.erroInterno(w, r, "entrar", err)
		return
	}
	if err := a.iniciarSessao(w, daRequisicao(ctx), usuario); err != nil {
		a.erroInterno(w, r, "iniciar a sessão", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, estadoDto{Configurado: true, Autenticado: true, Login: &usuario.Login, Versao: a.o.Versao})
}

// sair encerra a sessão (logada ou não) e reemite o CSRF para quem agora é anônimo.
func (a *Autenticacao) sair(w http.ResponseWriter, r *http.Request) {
	req := daRequisicao(r.Context())
	if req.sessao != nil {
		login := req.sessao.Login
		if err := a.auditar(r.Context(), a.o.DB, r, "Logout", &login); err != nil {
			a.erroInterno(w, r, "auditar", err)
			return
		}
	}
	expirarSessao(w)
	req.sessao = nil
	if err := a.emitirCsrf(w, req); err != nil {
		a.erroInterno(w, r, "emitir o CSRF", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// alterarSenha troca a senha e o carimbo de segurança: as outras sessões caem, e esta continua (sessão nova).
func (a *Autenticacao) alterarSenha(w http.ResponseWriter, r *http.Request) {
	atual := &respostas.CampoTexto{Propriedade: "SenhaAtual", Maximo: 200}
	nova := &respostas.CampoTexto{Propriedade: "NovaSenha", Maximo: 200}
	if !respostas.LerTextos(w, r, atual, nova) {
		return
	}
	ctx := r.Context()
	req := daRequisicao(ctx)
	usuario, confere, err := usuarioComSenha(ctx, a.o.DB, req.usuario(), *atual.Valor)
	if err != nil {
		a.erroInterno(w, r, "alterar a senha", err)
		return
	}
	if !confere {
		respostas.Mensagem(w, http.StatusBadRequest, MensagemSenhaAtualErrada)
		return
	}
	if problema := validarSenha(*nova.Valor, usuario.Login); problema != "" {
		respostas.Mensagem(w, http.StatusBadRequest, problema)
		return
	}
	if usuario.SenhaHash, err = GerarHash(*nova.Valor); err != nil {
		a.erroInterno(w, r, "gerar o hash", err)
		return
	}
	usuario.CarimboSeguranca = tipos.NovoGuid() // encerra as demais sessões
	login := usuario.Login
	if err := a.salvarComAuditoria(ctx, r, usuario, "SenhaAlterada", &login); err != nil {
		a.erroInterno(w, r, "alterar a senha", err)
		return
	}
	if err := a.iniciarSessao(w, req, usuario); err != nil {
		a.erroInterno(w, r, "iniciar a sessão", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MensagemSenhaAtualErrada é o 400 de quem pede a senha atual e recebe outra (alterar a senha, parâmetros de
// segurança).
const MensagemSenhaAtualErrada = "A senha atual não confere."

// ConfereSenha diz se a senha é a do usuário logado na requisição (sessao.Usuario): a conferência da senha atual do
// alterar-senha, para quem pede a senha antes de mudar algo de segurança (#68). Sem usuário logado, não confere. Como
// no alterar-senha, a senha errada não conta como tentativa de login: quem chega aqui já entrou.
func ConfereSenha(ctx context.Context, q banco.DBTX, senha string) (bool, error) {
	id, ok := sessao.Usuario(ctx)
	if !ok {
		return false, nil
	}
	_, confere, err := usuarioComSenha(ctx, q, id, senha)
	return confere, err
}

// usuarioComSenha lê o usuário e diz se a senha é a dele (certa, mesmo que o hash precise ser refeito).
func usuarioComSenha(ctx context.Context, q banco.DBTX, id int64, senha string) (Usuario, bool, error) {
	usuario, err := usuarioPorId(ctx, q, id)
	if err != nil {
		return Usuario{}, false, err
	}
	return usuario, VerificarHash(usuario.SenhaHash, senha) != SenhaErrada, nil
}

// auditoria devolve os 100 registros mais novos.
func (a *Autenticacao) auditoria(w http.ResponseWriter, r *http.Request) {
	registros, err := ultimosAcessos(r.Context(), a.o.DB)
	if err != nil {
		a.erroInterno(w, r, "auditoria", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, registros)
}

// iniciarSessao é o EntrarAsync: cookie de sessão novo e o token CSRF reemitido para o usuário (ele é ligado a quem
// está logado).
func (a *Autenticacao) iniciarSessao(w http.ResponseWriter, req *requisicao, u Usuario) error {
	agora := a.o.Relogio.Agora()
	s := conteudoSessao{Usuario: u.Id, Login: u.Login, Carimbo: u.CarimboSeguranca, Emitido: agora.Unix(), Expira: agora.Add(a.duracao).Unix()}
	definirSessao(w, a.o.Chave, s)
	req.sessao = &s
	return a.emitirCsrf(w, req)
}

// emitirCsrf é o TokenAntiforgery.Emitir: cria o segredo (cookie HttpOnly) se ainda não há, e põe no cookie legível o
// token do usuário atual.
func (a *Autenticacao) emitirCsrf(w http.ResponseWriter, req *requisicao) error {
	if req.segredo == "" {
		segredo, err := novoSegredoAntifalsificacao()
		if err != nil {
			return err
		}
		req.segredo = segredo
		http.SetCookie(w, cookieBase(CookieAntifalsificacao, segredo, true))
	}
	http.SetCookie(w, cookieBase(CookieXsrf, tokenCsrf(a.o.Chave, req.segredo, req.usuario()), false))
	return nil
}

// salvarComAuditoria grava o usuário e o registro de auditoria juntos.
func (a *Autenticacao) salvarComAuditoria(ctx context.Context, r *http.Request, u Usuario, evento string, login *string) error {
	return banco.EmTransacao(ctx, a.o.DB, func(tx *sql.Tx) error {
		if err := atualizarUsuario(ctx, tx, u); err != nil {
			return err
		}
		return a.auditar(ctx, tx, r, evento, login)
	})
}

// auditar grava o evento na auditoria, com a hora do relógio da autenticação.
func (a *Autenticacao) auditar(ctx context.Context, q banco.DBTX, r *http.Request, evento string, login *string) error {
	return auditoria.Registrar(ctx, q, r, a.agoraDb(), evento, login)
}

// parametros são as tentativas antes do bloqueio e os minutos de bloqueio (ParametroService.ObterInteiroAsync): o valor
// gravado em Configuracao, ou o padrão; sempre dentro da faixa do catálogo (3 a 20 e 1 a 1440).
func (a *Autenticacao) parametros(ctx context.Context) (maximo, minutos int, err error) {
	if maximo, err = a.inteiro(ctx, chaveMaximoTentativas, a.o.MaximoTentativasLogin, 3, 20); err != nil {
		return 0, 0, err
	}
	minutos, err = a.inteiro(ctx, chaveMinutosBloqueio, a.o.MinutosBloqueio, 1, 1440)
	return maximo, minutos, err
}

func (a *Autenticacao) inteiro(ctx context.Context, chave string, padrao, minimo, maximo int) (int, error) {
	valor := padrao
	gravado, err := valorDaConfiguracao(ctx, a.o.DB, chave)
	if err != nil {
		return 0, err
	}
	if gravado != nil {
		if n, err := strconv.Atoi(strings.TrimSpace(*gravado)); err == nil {
			valor = n
		}
	}
	return min(max(valor, minimo), maximo), nil
}

// hashDeMentira: o hash de uma senha aleatória, feito uma vez só.
func (a *Autenticacao) hashDeMentira() string {
	a.hashFicticioUmaVez.Do(func() {
		a.hashFicticio, _ = GerarHash(tipos.NovoGuid().String())
	})
	return a.hashFicticio
}

func (a *Autenticacao) agoraDb() tipos.DataHora { return tipos.NovaDataHora(a.o.Relogio.Agora()) }

// validarSenha é o ValidarSenha do AuthController.
func validarSenha(senha, login string) string {
	if respostas.TamanhoUtf16(senha) < tamanhoMinimoSenha {
		return fmt.Sprintf("A senha deve ter pelo menos %d caracteres.", tamanhoMinimoSenha)
	}
	if strings.Contains(strings.ToUpper(senha), strings.ToUpper(login)) {
		return "A senha não pode conter o nome de usuário."
	}
	return ""
}
