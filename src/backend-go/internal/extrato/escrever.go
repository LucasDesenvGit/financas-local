package extrato

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transferencias"
)

// idDaRota é o {id:long} da rota: o que não é long não casa (404 sem corpo).
func idDaRota(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.PathValue("id"), "+"), 10, 64)
	if err != nil {
		respostas.SemCorpo(w, http.StatusNotFound)
		return 0, false
	}
	return id, true
}

// ler lê uma transação para alterar; nil se não existe.
func ler(ctx context.Context, q consultas.DBTX, id int64) (*transacoes.Transacao, error) {
	lista, err := transacoes.Listar(ctx, q, `WHERE "Id" = ?`, id)
	if err != nil || len(lista) == 0 {
		return nil, err
	}
	return lista[0], nil
}

// responderTransacao devolve o TransacaoDto de id, como o ProjetarAsync.
func (s *Servico) responderTransacao(w http.ResponseWriter, r *http.Request, id int64) {
	lista, err := lerDtos(r.Context(), s.db, `WHERE t."Id" = ?`, id)
	if err != nil || len(lista) == 0 {
		s.erroInterno(w, r, "ler a transação gravada", errors.Join(err, errors.New("transação gravada não encontrada")))
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista[0])
}

// categoriaExiste confere a categoria escolhida (o "Categoria inválida." do C#).
func (s *Servico) categoriaExiste(ctx context.Context, id int64) (bool, error) {
	var existe bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM "Categoria" WHERE "Id" = ?)`, id).Scan(&existe)
	return existe, err
}

// atualizar é o PUT /api/transacoes/{id:long}: altera um lançamento manual, com as regras do lançamento novo. A do
// Open Finance só muda pelo PATCH, e a transferência (com par) se exclui e se lança de novo.
func (s *Servico) atualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	d, ok := lerSalvarTransacao(w, r)
	if !ok {
		return
	}
	t, err := ler(r.Context(), s.db, id)
	switch {
	case err != nil:
		s.erroInterno(w, r, "ler a transação", err)
		return
	case t == nil:
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	case t.PluggyTransactionId != nil:
		respostas.Mensagem(w, http.StatusBadRequest, "Transações do Open Finance só permitem alterar descrição, categoria e observação.")
		return
	case t.TransferenciaGrupo != nil:
		respostas.Mensagem(w, http.StatusBadRequest, "Para alterar uma transferência, exclua-a e lance novamente.")
		return
	}
	mensagem, err := s.validarManual(r.Context(), d)
	if err != nil {
		s.erroInterno(w, r, "validar o lançamento", err)
		return
	}
	if mensagem != "" {
		respostas.Mensagem(w, http.StatusBadRequest, mensagem)
		return
	}
	err = banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		rs, err := categorizacao.Carregar(r.Context(), tx)
		if err != nil {
			return err
		}
		agora := tipos.NovaDataHora(s.relogio.Agora())
		t.ContaId = d.ContaId
		aplicar(t, d, rs, agora)
		if err := transacoes.Gravar(r.Context(), tx, t); err != nil {
			return err
		}
		_, err = transferencias.Reclassificar(r.Context(), tx, agora)
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "alterar o lançamento", err)
		return
	}
	s.responderTransacao(w, r, id)
}

// ajustar é o PATCH /api/transacoes/{id:long}: o que se muda em qualquer transação, inclusive a do Open Finance: o
// nome (trocado no formulário fica escolhido à mão, #123), a categoria (escolhida fica manual; sem ela, as regras
// escolhem de novo), a observação, ignorar nos relatórios (fora das transferências) e a emergência (só saída).
func (s *Servico) ajustar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	descricao := respostas.CampoTexto{Propriedade: "Descricao", Maximo: 300}
	categoria := respostas.CampoInteiroOpcional{Propriedade: "CategoriaId"}
	observacao := respostas.CampoTexto{Propriedade: "Observacao", Maximo: 1000, Opcional: true}
	ignorar := respostas.CampoBool{Propriedade: "IgnorarRelatorios"}
	emergencia := respostas.CampoBool{Propriedade: "Emergencia"}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.AtualizarTransacaoOpenFinanceDto", &descricao, &categoria,
		&observacao, &ignorar, &emergencia) {
		return
	}
	t, err := ler(r.Context(), s.db, id)
	if err != nil {
		s.erroInterno(w, r, "ler a transação", err)
		return
	}
	if t == nil {
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	}
	if categoria.Valor != nil {
		existe, err := s.categoriaExiste(r.Context(), *categoria.Valor)
		if err != nil {
			s.erroInterno(w, r, "conferir a categoria", err)
			return
		}
		if !existe {
			respostas.Mensagem(w, http.StatusBadRequest, "Categoria inválida.")
			return
		}
	}
	err = banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		rs, err := categorizacao.Carregar(r.Context(), tx)
		if err != nil {
			return err
		}
		agora := tipos.NovaDataHora(s.relogio.Agora())
		if d := strings.TrimSpace(*descricao.Valor); d != t.Descricao {
			t.Descricao, t.DescricaoManual, t.RegraDescricaoId = d, true, nil
		}
		t.Observacao = aparadoOuNulo(observacao.Valor)
		if categoria.Valor == nil {
			t.CategoriaManual = false
			t.CategoriaId = rs.Resolver(t)
		} else {
			t.CategoriaManual, t.CategoriaId = true, categoria.Valor
		}
		rs.AplicarTipo(t)
		if t.Tipo != transacoes.Transferencia {
			t.IgnorarRelatorios = ignorar.Valor
		}
		t.Emergencia = emergencia.Valor && t.Valor.Decimal().IsNegative()
		t.AtualizadoEm = agora
		if err := transacoes.Gravar(r.Context(), tx, t); err != nil {
			return err
		}
		_, err = transferencias.Reclassificar(r.Context(), tx, agora)
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "ajustar a transação", err)
		return
	}
	s.responderTransacao(w, r, id)
}

// excluir é o DELETE /api/transacoes/{id:long}: só lançamento manual; a transferência sai com o par (o mesmo grupo).
func (s *Servico) excluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	t, err := ler(r.Context(), s.db, id)
	switch {
	case err != nil:
		s.erroInterno(w, r, "ler a transação", err)
		return
	case t == nil:
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	case t.PluggyTransactionId != nil:
		respostas.Mensagem(w, http.StatusBadRequest,
			"Transações do Open Finance não podem ser excluídas. Marque 'Ignorar nos relatórios' se necessário.")
		return
	}
	err = banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		var err error
		if t.TransferenciaGrupo != nil {
			_, err = tx.ExecContext(r.Context(), `DELETE FROM "Transacao" WHERE "TransferenciaGrupo" = ?`, *t.TransferenciaGrupo)
		} else {
			_, err = tx.ExecContext(r.Context(), `DELETE FROM "Transacao" WHERE "Id" = ?`, id)
		}
		if err != nil {
			return err
		}
		_, err = transferencias.Reclassificar(r.Context(), tx, tipos.NovaDataHora(s.relogio.Agora()))
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "excluir a transação", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}
