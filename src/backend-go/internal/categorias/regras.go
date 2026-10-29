package categorias

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transferencias"
)

// RegraDto, na ordem do record do C#.
type RegraDto struct {
	Id                  int64  `json:"id"`
	Texto               string `json:"texto"`
	TipoCorrespondencia int64  `json:"tipoCorrespondencia"`
	CategoriaId         int64  `json:"categoriaId"`
	CategoriaNome       string `json:"categoriaNome"`
	Prioridade          int64  `json:"prioridade"`
	Ativa               bool   `json:"ativa"`
}

// listarRegras: por prioridade e Id, com o nome da categoria.
func (s *Servico) listarRegras(w http.ResponseWriter, r *http.Request) {
	linhas, err := s.db.QueryContext(r.Context(), `SELECT r."Id", r."Texto", r."TipoCorrespondencia", r."CategoriaId",
		c."Nome", r."Prioridade", r."Ativa" FROM "RegraCategorizacao" AS r INNER JOIN "Categoria" AS c ON r."CategoriaId" = c."Id"
		ORDER BY r."Prioridade", r."Id"`)
	if err != nil {
		s.erroInterno(w, r, "listar regras", err)
		return
	}
	defer linhas.Close()
	lista := []RegraDto{}
	for linhas.Next() {
		var d RegraDto
		if err := linhas.Scan(&d.Id, &d.Texto, &d.TipoCorrespondencia, &d.CategoriaId, &d.CategoriaNome, &d.Prioridade, &d.Ativa); err != nil {
			s.erroInterno(w, r, "ler regra", err)
			return
		}
		lista = append(lista, d)
	}
	if err := linhas.Err(); err != nil {
		s.erroInterno(w, r, "listar regras", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// lerRegra é o model binding do SalvarRegraDto: [Required, StringLength(200, MinimumLength = 2)] Texto e
// [Range(0, 10000)] Prioridade; Ativa vale true se não vier.
func lerRegra(w http.ResponseWriter, r *http.Request) (RegraDto, bool) {
	texto := respostas.CampoTexto{Propriedade: "Texto", Minimo: 2, Maximo: 200}
	tipo := respostas.CampoEnum{Propriedade: "TipoCorrespondencia"}
	categoria := respostas.CampoInteiro{Propriedade: "CategoriaId"}
	prioridade := respostas.CampoInteiro{Propriedade: "Prioridade", Faixa: &respostas.Faixa{Minimo: 0, Maximo: 10000}}
	ativa := respostas.CampoBool{Propriedade: "Ativa", Valor: true}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarRegraDto", &texto, &tipo, &categoria, &prioridade, &ativa) {
		return RegraDto{}, false
	}
	return RegraDto{Texto: strings.TrimSpace(*texto.Valor), TipoCorrespondencia: tipo.Ou(0), CategoriaId: categoria.Valor,
		Prioridade: prioridade.Valor, Ativa: ativa.Valor}, true
}

// nomeDaCategoria devolve o nome, ou "" e false se ela não existe.
func nomeDaCategoria(r *http.Request, tx *sql.Tx, id int64) (string, bool, error) {
	var nome string
	err := tx.QueryRowContext(r.Context(), `SELECT "Nome" FROM "Categoria" WHERE "Id" = ?`, id).Scan(&nome)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return nome, err == nil, err
}

func (s *Servico) criarRegra(w http.ResponseWriter, r *http.Request) {
	d, ok := lerRegra(w, r)
	if !ok {
		return
	}
	var categoriaInvalida bool
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		nome, existe, err := nomeDaCategoria(r, tx, d.CategoriaId)
		if err != nil || !existe {
			categoriaInvalida = !existe
			return err
		}
		d.CategoriaNome = nome
		res, err := tx.ExecContext(r.Context(), `INSERT INTO "RegraCategorizacao" ("Texto", "TipoCorrespondencia", "CategoriaId",
			"Prioridade", "Ativa", "CriadoEm") VALUES (?, ?, ?, ?, ?, ?)`,
			d.Texto, d.TipoCorrespondencia, d.CategoriaId, d.Prioridade, d.Ativa, s.agora())
		if err != nil {
			return err
		}
		d.Id, err = res.LastInsertId()
		return err
	})
	switch {
	case err != nil:
		s.erroInterno(w, r, "criar regra", err)
	case categoriaInvalida:
		respostas.Mensagem(w, http.StatusBadRequest, "Categoria inválida.")
	default:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, d)
	}
}

func (s *Servico) atualizarRegra(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	d, ok := lerRegra(w, r)
	if !ok {
		return
	}
	d.Id = id
	var naoExiste, categoriaInvalida bool
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		var existe bool
		if err := tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM "RegraCategorizacao" WHERE "Id" = ?)`, id).Scan(&existe); err != nil {
			return err
		}
		if !existe {
			naoExiste = true
			return nil
		}
		nome, existe, err := nomeDaCategoria(r, tx, d.CategoriaId)
		if err != nil || !existe {
			categoriaInvalida = !existe
			return err
		}
		d.CategoriaNome = nome
		_, err = tx.ExecContext(r.Context(), `UPDATE "RegraCategorizacao" SET "Texto" = ?, "TipoCorrespondencia" = ?,
			"CategoriaId" = ?, "Prioridade" = ?, "Ativa" = ? WHERE "Id" = ?`,
			d.Texto, d.TipoCorrespondencia, d.CategoriaId, d.Prioridade, d.Ativa, id)
		return err
	})
	switch {
	case err != nil:
		s.erroInterno(w, r, "editar regra", err)
	case naoExiste:
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
	case categoriaInvalida:
		respostas.Mensagem(w, http.StatusBadRequest, "Categoria inválida.")
	default:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, d)
	}
}

func (s *Servico) excluirRegra(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM "RegraCategorizacao" WHERE "Id" = ?`, id)
	var apagadas int64
	if err == nil {
		apagadas, err = res.RowsAffected()
	}
	switch {
	case err != nil:
		s.erroInterno(w, r, "excluir regra", err)
	case apagadas == 0:
		respostas.Mensagem(w, http.StatusNotFound, mensagemNaoEncontrado)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// loteDoReaplicar é o Take(2000) do C#. Variável só para o teste dos vários lotes.
var loteDoReaplicar = 2000

// reaplicar recategoriza todas as transações de categoria automática com as regras de agora e o mapa da Pluggy, acerta
// o tipo de cada uma e, no fim, as transferências entre contas suas (ADR 0031). Um lote por vez, cada um gravado numa
// transação do banco, como o SaveChanges por lote do C#.
func (s *Servico) reaplicar(w http.ResponseWriter, r *http.Request) {
	rs, err := categorizacao.Carregar(r.Context(), s.db)
	if err != nil {
		s.erroInterno(w, r, "carregar as regras", err)
		return
	}
	alteradas := 0
	var ultimoId int64
	for {
		var lote []*transacoes.Transacao
		err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
			var err error
			lote, err = transacoes.Listar(r.Context(), tx, `WHERE NOT "CategoriaManual" AND "Id" > ? ORDER BY "Id" LIMIT ?`,
				ultimoId, loteDoReaplicar)
			if err != nil {
				return err
			}
			for _, t := range lote {
				nova := rs.Resolver(t)
				if mesmoId(nova, t.CategoriaId) {
					continue
				}
				t.CategoriaId = nova
				rs.AplicarTipo(t)
				t.AtualizadoEm = s.agora()
				alteradas++
			}
			return transacoes.GravarTodas(r.Context(), tx, lote)
		})
		if err != nil {
			s.erroInterno(w, r, "reaplicar as regras", err)
			return
		}
		if len(lote) == 0 {
			break
		}
		ultimoId = lote[len(lote)-1].Id
	}
	err = banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		_, err := transferencias.Reclassificar(r.Context(), tx, s.agora())
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "reclassificar as transferências", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, map[string]int{"transacoesAtualizadas": alteradas})
}

func mesmoId(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
