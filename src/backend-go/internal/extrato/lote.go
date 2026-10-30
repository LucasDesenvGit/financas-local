package extrato

import (
	"database/sql"
	"net/http"
	"slices"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transferencias"
)

// ResultadoAplicacaoDto é o do C#: quantas transações a ação mudou.
type ResultadoAplicacaoDto struct {
	TransacoesAtualizadas int64 `json:"transacoesAtualizadas"`
}

// lerIds é o model binding de um DTO de lote: Ids ([Required, MinLength(1), MaxLength(1000)] List<long>) e mais os
// campos do DTO. Devolve os ids sem repetição (o Distinct do C#).
func lerIds(w http.ResponseWriter, r *http.Request, tipoDto string, outros ...respostas.Campo) ([]int64, bool) {
	ids := respostas.CampoListaLong{Propriedade: "Ids", Minimo: 1, Maximo: 1000}
	if !respostas.LerObjeto(w, r, "dto", tipoDto, append([]respostas.Campo{&ids}, outros...)...) {
		return nil, false
	}
	unicos := []int64{}
	for _, id := range ids.Valor {
		if !slices.Contains(unicos, id) {
			unicos = append(unicos, id)
		}
	}
	return unicos, true
}

// emLista é o "(?, ?, ...)" dos ids, com os argumentos.
func emLista(ids []int64) (string, []any) {
	argumentos := make([]any, len(ids))
	for i, id := range ids {
		argumentos[i] = id
	}
	return `(` + strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ") + `)`, argumentos
}

// emergenciaEmLote é o PUT /api/transacoes/emergencia-em-lote: marca (só as saídas) ou desmarca (todas) o gasto de
// emergência. A conta é a das linhas que casam, como a do ExecuteUpdate (inclusive as que já estavam assim). O
// "Valor < 0" do C# é conferido aqui, em decimal: dinheiro não se compara no SQL.
func (s *Servico) emergenciaEmLote(w http.ResponseWriter, r *http.Request) {
	emergencia := respostas.CampoBool{Propriedade: "Emergencia"}
	ids, ok := lerIds(w, r, "Financas.Api.Dtos.EmergenciaLoteDto", &emergencia)
	if !ok {
		return
	}
	var alteradas int64
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		if emergencia.Valor {
			em, argumentos := emLista(ids)
			lista, err := transacoes.Listar(r.Context(), tx, `WHERE "Id" IN `+em, argumentos...)
			if err != nil {
				return err
			}
			ids = ids[:0]
			for _, t := range lista {
				if t.Valor.Decimal().IsNegative() {
					ids = append(ids, t.Id)
				}
			}
		}
		em, argumentos := emLista(ids)
		res, err := tx.ExecContext(r.Context(), `UPDATE "Transacao" SET "Emergencia" = ?, "AtualizadoEm" = ? WHERE "Id" IN `+em,
			append([]any{emergencia.Valor, tipos.NovaDataHora(s.relogio.Agora())}, argumentos...)...)
		if err != nil {
			return err
		}
		alteradas, err = res.RowsAffected()
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "marcar a emergência", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, ResultadoAplicacaoDto{alteradas})
}

// revisadaEmLote é o PUT /api/transacoes/revisada-em-lote (#9): a descrição pouco informativa já olhada sai do filtro.
func (s *Servico) revisadaEmLote(w http.ResponseWriter, r *http.Request) {
	revisada := respostas.CampoBool{Propriedade: "Revisada"}
	ids, ok := lerIds(w, r, "Financas.Api.Dtos.RevisadaLoteDto", &revisada)
	if !ok {
		return
	}
	em, argumentos := emLista(ids)
	res, err := s.db.ExecContext(r.Context(), `UPDATE "Transacao" SET "DescricaoRevisada" = ?, "AtualizadoEm" = ? WHERE "Id" IN `+em,
		append([]any{revisada.Valor, tipos.NovaDataHora(s.relogio.Agora())}, argumentos...)...)
	var alteradas int64
	if err == nil {
		alteradas, err = res.RowsAffected()
	}
	if err != nil {
		s.erroInterno(w, r, "marcar como revisada", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, ResultadoAplicacaoDto{alteradas})
}

// categoriaEmLote é o PUT /api/transacoes/categoria-em-lote: a mesma categoria (manual) ou a das regras (sem ela) em
// várias transações, com o tipo acompanhando; depois, as transferências conferidas de novo.
func (s *Servico) categoriaEmLote(w http.ResponseWriter, r *http.Request) {
	categoria := respostas.CampoInteiroOpcional{Propriedade: "CategoriaId"}
	ids, ok := lerIds(w, r, "Financas.Api.Dtos.CategorizarLoteDto", &categoria)
	if !ok {
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
	var alteradas int64
	err := banco.EmTransacao(r.Context(), s.db, func(tx *sql.Tx) error {
		rs, err := categorizacao.Carregar(r.Context(), tx)
		if err != nil {
			return err
		}
		em, argumentos := emLista(ids)
		lista, err := transacoes.Listar(r.Context(), tx, `WHERE "Id" IN `+em, argumentos...)
		if err != nil {
			return err
		}
		agora := tipos.NovaDataHora(s.relogio.Agora())
		for _, t := range lista {
			if categoria.Valor == nil {
				t.CategoriaManual = false
				t.CategoriaId = rs.Resolver(t)
			} else {
				t.CategoriaManual, t.CategoriaId = true, categoria.Valor
			}
			rs.AplicarTipo(t)
			t.AtualizadoEm = agora
		}
		if err := transacoes.GravarTodas(r.Context(), tx, lista); err != nil {
			return err
		}
		alteradas = int64(len(lista))
		_, err = transferencias.Reclassificar(r.Context(), tx, agora)
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "categorizar em lote", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, ResultadoAplicacaoDto{alteradas})
}
