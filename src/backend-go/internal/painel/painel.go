// Package painel são as rotas da entrada "painel" de Go:Prefixos (#80): /api/emprestimos (o EmprestimosController do C#)
// e, nas próximas partes da #80, /api/relatorios e /api/painel. As regras ficam nos pacotes delas (emprestimos,
// relatorios); aqui ficam só as rotas e as respostas.
package painel

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/emprestimos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
)

// Servico atende as rotas do painel.
type Servico struct {
	relogio     relogio.Relogio
	log         *slog.Logger
	emprestimos *emprestimos.Servico
}

// NovoServico monta o serviço.
func NovoServico(db *sql.DB, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{relogio: r, log: log, emprestimos: emprestimos.NovoServico(db)}
}

// Registrar põe as rotas no mux.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/emprestimos", s.emprestimosDoPainel)
}

func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		return
	}
	s.log.ErrorContext(r.Context(), "erro no painel", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}

// emprestimosDoPainel: os contratos de crédito do Open Finance com os totais de financiamentos e de empréstimos
// separados (ADR 0014).
func (s *Servico) emprestimosDoPainel(w http.ResponseWriter, r *http.Request) {
	p, err := s.emprestimos.Painel(r.Context(), relogio.Hoje(s.relogio))
	if err != nil {
		s.erroInterno(w, r, "empréstimos", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, p)
}
