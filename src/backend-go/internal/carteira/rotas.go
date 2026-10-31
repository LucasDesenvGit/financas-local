package carteira

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Registrar põe as rotas de /api/carteira no mux (o CarteiraController).
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/carteira", s.obter)
	mux.HandleFunc("GET /api/carteira/ativos/{id}/operacoes", s.operacoes)
	mux.HandleFunc("POST /api/carteira/atualizar-cotacoes", s.atualizarCotacoes)
	mux.HandleFunc("POST /api/carteira/operacoes", s.criarOperacao)
	mux.HandleFunc("PUT /api/carteira/operacoes/{id}", s.atualizarOperacao)
	mux.HandleFunc("DELETE /api/carteira/operacoes/{id}", s.excluirOperacao)
	mux.HandleFunc("PUT /api/carteira/ativos/{id}", s.atualizarAtivo)
	mux.HandleFunc("DELETE /api/carteira/ativos/{id}", s.excluirAtivo)
	mux.HandleFunc("GET /api/carteira/extrato/sugestoes", s.sugestoesDoExtrato)
	mux.HandleFunc("POST /api/carteira/extrato/importar", s.importarDoExtrato)
	mux.HandleFunc("GET /api/carteira/configuracao", s.obterConfiguracao)
	mux.HandleFunc("PUT /api/carteira/configuracao", s.salvarConfiguracao)
}

func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		return
	}
	s.log.ErrorContext(r.Context(), "erro na carteira", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}

func (s *Servico) obter(w http.ResponseWriter, r *http.Request) {
	inicio, err := s.InicioDaSerieSelic(r.Context())
	if err != nil {
		s.erroInterno(w, r, "início da Selic", err)
		return
	}
	serie, err := s.selic.ObterSerie(r.Context(), inicio)
	if err != nil {
		s.erroInterno(w, r, "série da Selic", err)
		return
	}
	meta, err := s.selic.MetaAtual(r.Context())
	if err != nil {
		s.erroInterno(w, r, "meta da Selic", err)
		return
	}
	carteira, err := s.Montar(r.Context(), serie, meta)
	if err != nil {
		s.erroInterno(w, r, "montar", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, carteira)
}

// primeiraDataDoAtivo é o começo da série da Selic para as operações do ativo: a primeira operação, ou hoje.
func (s *Servico) primeiraDataDoAtivo(ctx context.Context, id int64) (tipos.Data, error) {
	var primeira sql.Null[tipos.Data]
	if err := s.db.QueryRowContext(ctx, `SELECT MIN("Data") FROM "OperacaoAtivo" WHERE "AtivoId" = ?`, id).Scan(&primeira); err != nil {
		return tipos.Data{}, fmt.Errorf("ler a primeira operação do ativo: %w", err)
	}
	if primeira.Valid {
		return primeira.V, nil
	}
	return relogio.Hoje(s.relogio), nil
}

func (s *Servico) operacoes(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	inicio, err := s.primeiraDataDoAtivo(r.Context(), id)
	if err != nil {
		s.erroInterno(w, r, "primeira operação", err)
		return
	}
	serie, err := s.selic.ObterSerie(r.Context(), inicio)
	if err != nil {
		s.erroInterno(w, r, "série da Selic", err)
		return
	}
	lista, err := s.Operacoes(r.Context(), id, serie)
	switch {
	case err != nil:
		s.erroInterno(w, r, "operações", err)
	case lista == nil:
		registroNaoEncontrado(w)
	default:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
	}
}

// atualizarCotacoes é o POST /api/carteira/atualizar-cotacoes?ativoId=: as cotações e os proventos de todos os ativos, ou
// de um só. O ativoId é int?: sem ele, ou vazio, são todos.
func (s *Servico) atualizarCotacoes(w http.ResponseWriter, r *http.Request) {
	var somente []int64
	if v, ok := respostas.ParametroDaQuery(r, "ativoId"); ok && strings.TrimFunc(v, unicode.IsSpace) != "" {
		id, ok := respostas.InteiroDaQuery(v)
		if !ok {
			respostas.Validacao(w, map[string][]string{"ativoId": {respostas.MensagemDaQuery(v)}})
			return
		}
		somente = []int64{int64(id)}
	}
	resultado, err := s.cotacoes.Atualizar(r.Context(), somente)
	if err != nil {
		s.erroInterno(w, r, "atualizar as cotações", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, resultado)
}
