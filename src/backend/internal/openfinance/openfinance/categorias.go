package openfinance

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/ajustes/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/transacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/transferencias"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/texto"
)

// categoriaDto é o CategoriaPluggyDto.
type categoriaDto struct {
	PluggyCategoriaId    string  `json:"pluggyCategoriaId"`
	Descricao            string  `json:"descricao"`
	PaiDescricao         *string `json:"paiDescricao"`
	CategoriaId          *int64  `json:"categoriaId"`
	CategoriaNome        *string `json:"categoriaNome"`
	QuantidadeTransacoes int64   `json:"quantidadeTransacoes"`
}

// categorias devolve as categorias da Pluggy, as mais usadas primeiro; depois, pelo pai e pela descrição, na ordem pt-BR.
func (s *Servico) categorias(w http.ResponseWriter, r *http.Request) {
	lista, err := s.lerCategorias(r.Context())
	if err != nil {
		s.erroInterno(w, r, "ler as categorias", err)
		return
	}
	slices.SortStableFunc(lista, func(a, b categoriaDto) int {
		return cmp.Or(cmp.Compare(b.QuantidadeTransacoes, a.QuantidadeTransacoes), compararOpcional(a.PaiDescricao, b.PaiDescricao),
			texto.CompararNaCultura(a.Descricao, b.Descricao))
	})
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

func (s *Servico) lerCategorias(ctx context.Context) ([]categoriaDto, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT p."PluggyCategoriaId", p."Descricao", p."PaiDescricao", p."CategoriaId", c."Nome",
		(SELECT COUNT(*) FROM "Transacao" t WHERE t."PluggyCategoriaId" = p."PluggyCategoriaId")
		FROM "CategoriaPluggy" p LEFT JOIN "Categoria" c ON c."Id" = p."CategoriaId"`)
	if err != nil {
		return nil, fmt.Errorf("ler as categorias da Pluggy: %w", err)
	}
	defer linhas.Close()
	lista := []categoriaDto{}
	for linhas.Next() {
		var c categoriaDto
		if err := linhas.Scan(&c.PluggyCategoriaId, &c.Descricao, &c.PaiDescricao, &c.CategoriaId, &c.CategoriaNome, &c.QuantidadeTransacoes); err != nil {
			return nil, fmt.Errorf("ler categoria da Pluggy: %w", err)
		}
		lista = append(lista, c)
	}
	return lista, linhas.Err()
}

// mapearCategoria liga a categoria da Pluggy a uma categoria sua (null desliga). Com aplicarExistentes, reaplica as regras
// nas transações automáticas dela e das subcategorias, e acerta as transferências (ADR 0031).
func (s *Servico) mapearCategoria(w http.ResponseWriter, r *http.Request) {
	categoriaId := respostas.CampoInteiroOpcional{Propriedade: "CategoriaId"}
	aplicar := respostas.CampoBool{Propriedade: "AplicarExistentes"}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.MapearCategoriaPluggyDto", &categoriaId, &aplicar) {
		return
	}
	ctx := r.Context()
	id := r.PathValue("pluggyCategoriaId")
	var existe int64
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM "CategoriaPluggy" WHERE "PluggyCategoriaId" = ?`, id).Scan(&existe)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
		return
	case err != nil:
		s.erroInterno(w, r, "ler a categoria da Pluggy", err)
		return
	}
	if categoriaId.Valor != nil {
		err := s.db.QueryRowContext(ctx, `SELECT 1 FROM "Categoria" WHERE "Id" = ?`, *categoriaId.Valor).Scan(&existe)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			respostas.Mensagem(w, http.StatusBadRequest, "Categoria inválida.")
			return
		case err != nil:
			s.erroInterno(w, r, "ler a categoria", err)
			return
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE "CategoriaPluggy" SET "CategoriaId" = ? WHERE "PluggyCategoriaId" = ?`, categoriaId.Valor, id); err != nil {
		s.erroInterno(w, r, "gravar a categoria da Pluggy", err)
		return
	}
	alteradas := 0
	if aplicar.Valor {
		if alteradas, err = s.reaplicar(ctx, id); err != nil {
			s.erroInterno(w, r, "reaplicar as regras", err)
			return
		}
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, map[string]int{"transacoesAtualizadas": alteradas})
}

// reaplicar recategoriza as transações automáticas da categoria e das subcategorias e devolve quantas mudaram.
func (s *Servico) reaplicar(ctx context.Context, id string) (int, error) {
	regras, err := categorizacao.Carregar(ctx, s.db)
	if err != nil {
		return 0, err
	}
	alteradas := 0
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		ids, err := comSubcategorias(ctx, tx, id)
		if err != nil {
			return err
		}
		lista, err := transacoes.Listar(ctx, tx, `WHERE NOT "CategoriaManual" AND "PluggyCategoriaId" IN (`+
			strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")+`) ORDER BY "Id"`, ids...)
		if err != nil {
			return err
		}
		for _, t := range lista {
			nova := regras.Resolver(t)
			if mesmoId(nova, t.CategoriaId) {
				continue
			}
			t.CategoriaId = nova
			regras.AplicarTipo(t)
			t.AtualizadoEm = s.agora()
			alteradas++
		}
		return transacoes.GravarTodas(ctx, tx, lista)
	})
	if err != nil {
		return 0, err
	}
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		_, err := transferencias.Reclassificar(ctx, tx, s.agora())
		return err
	})
	return alteradas, err
}

func comSubcategorias(ctx context.Context, tx *sql.Tx, id string) ([]any, error) {
	linhas, err := tx.QueryContext(ctx, `SELECT "PluggyCategoriaId" FROM "CategoriaPluggy" WHERE "PaiPluggyId" = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("ler as subcategorias: %w", err)
	}
	defer linhas.Close()
	ids := []any{id}
	for linhas.Next() {
		var sub string
		if err := linhas.Scan(&sub); err != nil {
			return nil, fmt.Errorf("ler subcategoria: %w", err)
		}
		ids = append(ids, sub)
	}
	return ids, linhas.Err()
}

func mesmoId(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
