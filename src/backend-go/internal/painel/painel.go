// Package painel são as rotas da entrada "painel" de Go:Prefixos (#80): /api/emprestimos (o EmprestimosController do C#),
// /api/relatorios (o RelatoriosController) e, na parte 3 da #80, /api/painel. As regras ficam nos pacotes delas (emprestimos,
// relatorios); aqui ficam só as rotas e as respostas.
package painel

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/carteira"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/emprestimos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// composicao é o que o painel usa da carteira: os investimentos do patrimônio.
type composicao interface {
	Composicao(ctx context.Context) (carteira.Composicao, error)
}

// totalDasReservas é o que o painel usa das reservas.
type totalDasReservas interface {
	Total(ctx context.Context) (tipos.Decimal, error)
}

// conversao é o que o painel usa do câmbio.
type conversao interface {
	ObterConversor(ctx context.Context, historicoDesde *tipos.Data) (*cambio.Conversor, error)
}

// Servico atende as rotas do painel.
type Servico struct {
	db          *sql.DB
	relogio     relogio.Relogio
	log         *slog.Logger
	emprestimos *emprestimos.Servico
	relatorios  *relatorios.Servico
	carteira    composicao
	reservas    totalDasReservas
	cambio      conversao
}

// NovoServico monta o serviço. A carteira, as reservas e o câmbio dão o patrimônio e a reserva da visão anual.
func NovoServico(db *sql.DB, c composicao, res totalDasReservas, cb conversao, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, relogio: r, log: log, emprestimos: emprestimos.NovoServico(db),
		relatorios: relatorios.NovoServico(db, r), carteira: c, reservas: res, cambio: cb}
}

// Registrar põe as rotas no mux.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/emprestimos", s.emprestimosDoPainel)
	mux.HandleFunc("GET /api/relatorios/categorias", s.porCategoria)
	mux.HandleFunc("GET /api/relatorios/fluxo", s.fluxo)
	mux.HandleFunc("GET /api/relatorios/anual", s.anual)
	mux.HandleFunc("GET /api/relatorios/projecao", s.projecao)
	mux.HandleFunc("GET /api/relatorios/maiores-despesas", s.maioresDespesas)
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
