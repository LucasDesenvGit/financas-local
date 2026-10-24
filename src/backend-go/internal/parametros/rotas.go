package parametros

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Registrar põe as rotas do ParametrosController no mux. Login e CSRF são de quem está na frente (o .NET, ou a
// autenticação do Go sozinho): aqui só chega quem passou.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/parametros", s.listar)
	mux.HandleFunc("PUT /api/parametros", s.salvar)
	mux.HandleFunc("GET /api/parametros/sistema", s.sistema)
}

func (s *Servico) listar(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Listar(r.Context())
	if err != nil {
		s.erroInterno(w, "listar", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// salvar é o Salvar do controller: valida tudo antes e grava os valores e a auditoria juntos, ou nada.
func (s *Servico) salvar(w http.ResponseWriter, r *http.Request) {
	corpo, ok := respostas.LerCorpo(w, r, "dto")
	if !ok {
		return
	}
	dto, erroJSON := lerSalvar(corpo)
	switch {
	case erroJSON != nil:
		respostas.Validacao(w, map[string][]string{erroJSON.Caminho: {erroJSON.Mensagem}, "dto": {respostas.Obrigatorio("dto")}})
		return
	case dto.nulo:
		respostas.CorpoVazio(w, "dto")
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
	err := banco.EmTransacao(ctx, s.o.DB, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		login, err := s.login(ctx, q)
		if err != nil {
			return err
		}
		// Como no .NET, a auditoria é registrada antes dos valores, e as duas coisas vão juntas no mesmo SaveChanges.
		if err := auditoria.Registrar(ctx, q, r, tipos.NovaDataHora(s.o.Relogio.Agora()), "ParametrosAlterados", login); err != nil {
			return err
		}
		return s.gravar(ctx, tx, dto.valores)
	})
	if err != nil {
		s.erroInterno(w, "salvar", err)
		return
	}
	s.listar(w, r)
}

func (s *Servico) sistema(w http.ResponseWriter, r *http.Request) {
	info, err := s.InfoSistema(r.Context())
	if err != nil {
		s.erroInterno(w, "sistema", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, info)
}

// login é o User.Identity.Name do .NET: o login de quem está na sessão, ou nil.
func (s *Servico) login(ctx context.Context, q interface {
	ObterLoginDoUsuario(context.Context, int64) (string, error)
}) (*string, error) {
	id, ok := sessao.Usuario(ctx)
	if !ok {
		return nil, nil
	}
	login, err := q.ObterLoginDoUsuario(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &login, nil
}

// erroInterno é o UseExceptionHandler: o erro vai para o log, e quem pediu recebe só o 500 de sempre.
func (s *Servico) erroInterno(w http.ResponseWriter, etapa string, err error) {
	s.o.Log.Error("erro nos parâmetros", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}
