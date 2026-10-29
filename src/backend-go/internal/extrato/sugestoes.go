package extrato

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sugestoes"
)

// sugestoesDeCategoria é o GET /api/transacoes/sugestoes-categoria: os grupos por descrição parecida com a categoria
// sugerida. somenteSemCategoria vale true e dias vale 365 sem os parâmetros; o período vai de 1 a 3660 dias.
func (s *Servico) sugestoesDeCategoria(w http.ResponseWriter, r *http.Request) {
	somente, dias := true, int32(365)
	erros := map[string][]string{}
	if v, ok := respostas.ParametroDaQuery(r, "somenteSemCategoria"); ok {
		if b, ok := respostas.BoolDaQuery(v); ok {
			somente = b
		} else {
			erros["somenteSemCategoria"] = []string{respostas.MensagemDaQuery(v)}
		}
	}
	if v, ok := respostas.ParametroDaQuery(r, "dias"); ok {
		if n, ok := respostas.InteiroDaQuery(v); ok {
			dias = n
		} else {
			erros["dias"] = []string{respostas.MensagemDaQuery(v)}
		}
	}
	if len(erros) > 0 {
		respostas.Validacao(w, erros)
		return
	}
	if dias < 1 || dias > 3660 {
		respostas.Mensagem(w, http.StatusBadRequest, "Período inválido.")
		return
	}
	lista, err := sugestoes.Grupos(r.Context(), s.db, relogio.Hoje(s.relogio), somente, int(dias))
	if err != nil {
		s.erroInterno(w, r, "sugerir as categorias", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// sugestoesDaTransacao é o GET /api/transacoes/{id:long}/sugestoes-categoria: até 3 categorias para a transação.
func (s *Servico) sugestoesDaTransacao(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.PathValue("id"), "+"), 10, 64)
	if err != nil {
		respostas.SemCorpo(w, http.StatusNotFound)
		return
	}
	lista, err := sugestoes.ParaTransacao(r.Context(), s.db, relogio.Hoje(s.relogio), id)
	switch {
	case errors.Is(err, sugestoes.ErrNaoEncontrada):
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
	case err != nil:
		s.erroInterno(w, r, "sugerir as categorias da transação", err)
	default:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
	}
}
