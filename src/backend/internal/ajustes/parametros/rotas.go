package parametros

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/autenticacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Registrar põe as rotas dos parâmetros no mux. Login e CSRF são de quem está na frente (a
// autenticação do Go sozinho): aqui só chega quem passou.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/parametros", s.listar)
	mux.HandleFunc("PUT /api/parametros", s.salvar)
	mux.HandleFunc("GET /api/parametros/sistema", s.sistema)
}

func (s *Servico) listar(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Listar(r.Context())
	if err != nil {
		s.erroInterno(w, r, "listar", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// salvar é o Salvar do controller: valida tudo antes e grava os valores e a auditoria juntos, ou nada.
func (s *Servico) salvar(w http.ResponseWriter, r *http.Request) {
	corpo, ok := respostas.LerCorpo(w, r)
	if !ok {
		return
	}
	dto, erroJSON := lerSalvar(corpo)
	switch {
	case erroJSON != nil:
		respostas.Validacao(w, map[string][]string{erroJSON.Caminho: {erroJSON.Mensagem}})
		return
	case dto.nulo:
		respostas.CorpoVazio(w)
		return
	case dto.valores == nil:
		respostas.Validacao(w, map[string][]string{"Valores": {respostas.Obrigatorio("Valores")}})
		return
	case len(dto.valores) == 0:
		respostas.Mensagem(w, http.StatusBadRequest, "Nenhum parâmetro informado.")
		return
	}
	if erros := validar(dto.valores); len(erros) > 0 {
		respostas.Mensagem(w, http.StatusBadRequest, strings.Join(erros, " "))
		return
	}

	ctx := r.Context()
	problema, err := s.conferirSenha(ctx, dto)
	if err != nil {
		s.erroInterno(w, r, "conferir a senha", err)
		return
	}
	if problema != "" {
		respostas.Mensagem(w, http.StatusBadRequest, problema)
		return
	}
	err = banco.EmTransacao(ctx, s.o.DB, func(tx *sql.Tx) error {
		login, err := auditoria.Login(ctx, tx)
		if err != nil {
			return err
		}
		// A auditoria é registrada antes dos valores, e as duas coisas vão juntas, numa gravação só.
		if err := auditoria.Registrar(ctx, tx, r, tipos.NovaDataHora(s.o.Relogio.Agora()), "ParametrosAlterados", login); err != nil {
			return err
		}
		return s.gravar(ctx, tx, dto.valores)
	})
	if err != nil {
		s.erroInterno(w, r, "salvar", err)
		return
	}
	s.listar(w, r)
}

// conferirSenha (#68): quem muda um parâmetro de segurança (Definicao.PedeSenha) para um valor diferente do atual
// precisa mandar a senha atual, conferida como no alterar-senha. Devolve a mensagem do 400, ou "" quando pode gravar.
// A senha errada não conta como tentativa de login, como no alterar-senha: a sessão já é de quem entrou, e o bloqueio
// protege o login, que é por onde se adivinha a senha de fora.
func (s *Servico) conferirSenha(ctx context.Context, dto corpoSalvar) (string, error) {
	atuais, err := s.Listar(ctx)
	if err != nil {
		return "", err
	}
	valorAtual := make(map[string]int, len(atuais))
	for _, p := range atuais {
		valorAtual[p.Chave] = p.Valor
	}
	muda := false
	for _, v := range dto.valores {
		if d, _ := definicao(v.Chave); d.PedeSenha() && v.Valor != valorAtual[v.Chave] {
			muda = true
		}
	}
	if !muda {
		return "", nil
	}
	if dto.senhaAtual == "" {
		return "Informe a senha atual para mudar os parâmetros de segurança.", nil
	}
	confere, err := autenticacao.ConfereSenha(ctx, s.o.DB, dto.senhaAtual)
	if err != nil || confere {
		return "", err
	}
	return autenticacao.MensagemSenhaAtualErrada, nil
}

func (s *Servico) sistema(w http.ResponseWriter, r *http.Request) {
	info, err := s.InfoSistema(r.Context())
	if err != nil {
		s.erroInterno(w, r, "sistema", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, info)
}

// erroInterno é o UseExceptionHandler: o erro vai para o log, e quem pediu recebe só o 500 de sempre.
func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	s.o.Log.ErrorContext(r.Context(), "erro nos parâmetros", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}
