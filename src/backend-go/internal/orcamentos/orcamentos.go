// Package orcamentos são os orçamentos do mês (#79): o OrcamentosController do C# (as 6 rotas de /api/orcamentos) e a
// sugestão por categoria (#39, o SugestaoOrcamentoService). A lista do mês é a base dos relatórios (#447,
// relatorios.OrcamentosDoMes).
package orcamentos

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/emprestimos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/parametros"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Servico são os orçamentos.
type Servico struct {
	db          *sql.DB
	relogio     relogio.Relogio
	log         *slog.Logger
	relatorios  *relatorios.Servico
	emprestimos *emprestimos.Servico
	parametros  *parametros.Servico
}

// NovoServico monta o serviço.
func NovoServico(db *sql.DB, par *parametros.Servico, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, relogio: r, log: log, relatorios: relatorios.NovoServico(db, r),
		emprestimos: emprestimos.NovoServico(db), parametros: par}
}

// Registrar põe as rotas de /api/orcamentos no mux.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/orcamentos", s.listar)
	mux.HandleFunc("PUT /api/orcamentos", s.salvar)
	mux.HandleFunc("DELETE /api/orcamentos/{id}", s.excluir)
	mux.HandleFunc("GET /api/orcamentos/sugestao", s.sugestao)
	mux.HandleFunc("POST /api/orcamentos/aplicar-sugestao", s.aplicarSugestao)
	mux.HandleFunc("POST /api/orcamentos/copiar-mes-anterior", s.copiarMesAnterior)
}

func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		return
	}
	s.log.ErrorContext(r.Context(), "erro nos orçamentos", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}

// ResultadoDto é o ResultadoAplicacaoDto do C# (o nome do campo vem de outra tela: é a contagem do que foi gravado).
type ResultadoDto struct {
	TransacoesAtualizadas int `json:"transacoesAtualizadas"`
}

// periodo lê o ?ano= e o ?mes= (o int? do C#: sem vir, o de hoje em Brasília) e confere o período. Devolve false se
// já respondeu: 400 do model binding (não é número) ou "Período inválido.".
func (s *Servico) periodo(w http.ResponseWriter, r *http.Request) (ano, mes int, ok bool) {
	hoje := relogio.Hoje(s.relogio)
	ano, mes = hoje.Ano(), int(hoje.Mes())
	erros := map[string][]string{}
	for _, p := range []struct {
		nome  string
		valor *int
	}{{"ano", &ano}, {"mes", &mes}} {
		if v, veio := respostas.ParametroDaQuery(r, p.nome); veio {
			if n, ok := respostas.InteiroDaQuery(v); ok {
				*p.valor = int(n)
			} else {
				erros[p.nome] = []string{respostas.MensagemDaQuery(v)}
			}
		}
	}
	if len(erros) > 0 {
		respostas.Validacao(w, erros)
		return 0, 0, false
	}
	if ano < 2000 || ano > 2100 || mes < 1 || mes > 12 {
		respostas.Mensagem(w, http.StatusBadRequest, "Período inválido.")
		return 0, 0, false
	}
	return ano, mes, true
}

// listar é a lista do mês, pelo nome da categoria (a ordem pt-BR do C#; o empate fica na ordem da base).
func (s *Servico) listar(w http.ResponseWriter, r *http.Request) {
	ano, mes, ok := s.periodo(w, r)
	if !ok {
		return
	}
	lista, err := s.relatorios.OrcamentosDoMes(r.Context(), ano, mes)
	if err != nil {
		s.erroInterno(w, r, "listar", err)
		return
	}
	slices.SortStableFunc(lista, func(a, b relatorios.OrcamentoDto) int {
		return texto.CompararNaCultura(a.CategoriaNome, b.CategoriaNome)
	})
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

func (s *Servico) sugestao(w http.ResponseWriter, r *http.Request) {
	ano, mes, ok := s.periodo(w, r)
	if !ok {
		return
	}
	sugestao, err := s.Sugerir(r.Context(), ano, mes)
	if err != nil {
		s.erroInterno(w, r, "sugerir", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, sugestao)
}

func campoAno() respostas.CampoInteiro {
	return respostas.CampoInteiro{Propriedade: "Ano", Faixa: &respostas.Faixa{Minimo: 2000, Maximo: 2100}}
}

func campoMes() respostas.CampoInteiro {
	return respostas.CampoInteiro{Propriedade: "Mes", Faixa: &respostas.Faixa{Minimo: 1, Maximo: 12}}
}

// salvar cria ou altera o orçamento de uma categoria de despesa no mês.
func (s *Servico) salvar(w http.ResponseWriter, r *http.Request) {
	categoria := respostas.CampoInteiro{Propriedade: "CategoriaId"}
	ano, mes := campoAno(), campoMes()
	valor := respostas.CampoDecimal{Propriedade: "Valor", Faixa: &respostas.Faixa{Minimo: 0, Maximo: 999999999}}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarOrcamentoDto", &categoria, &ano, &mes, &valor) {
		return
	}
	ctx := r.Context()
	var tipo int64
	err := s.db.QueryRowContext(ctx, `SELECT "Tipo" FROM "Categoria" WHERE "Id" = ?`, categoria.Valor).Scan(&tipo)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		respostas.Mensagem(w, http.StatusBadRequest, "Categoria inválida.")
		return
	case err != nil:
		s.erroInterno(w, r, "ler a categoria", err)
		return
	case tipo != tipoCategoriaGasto:
		respostas.Mensagem(w, http.StatusBadRequest, "Orçamentos são definidos para categorias de despesa.")
		return
	}
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		return gravar(ctx, tx, categoria.Valor, ano.Valor, mes.Valor, valor.Valor)
	})
	if err != nil {
		s.erroInterno(w, r, "salvar", err)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}

// gravar altera o orçamento da categoria no mês, ou cria, se ainda não há.
func gravar(ctx context.Context, tx *sql.Tx, categoria, ano, mes int64, valor tipos.Decimal) error {
	res, err := tx.ExecContext(ctx, `UPDATE "Orcamento" SET "Valor" = ? WHERE "CategoriaId" = ? AND "Ano" = ? AND "Mes" = ?`,
		valor, categoria, ano, mes)
	if err != nil {
		return fmt.Errorf("alterar o orçamento: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n > 0 {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO "Orcamento" ("CategoriaId", "Ano", "Mes", "Valor") VALUES (?, ?, ?, ?)`,
		categoria, ano, mes, valor); err != nil {
		return fmt.Errorf("criar o orçamento: %w", err)
	}
	return nil
}

// excluir: o id que não é número de 32 bits é 404 sem corpo ({id:int}); o que não existe, "Registro não encontrado.".
func (s *Servico) excluir(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.PathValue("id"), "+"), 10, 32)
	if err != nil {
		respostas.SemCorpo(w, http.StatusNotFound)
		return
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM "Orcamento" WHERE "Id" = ?`, id)
	if err != nil {
		s.erroInterno(w, r, "excluir", err)
		return
	}
	if n, err := res.RowsAffected(); err != nil {
		s.erroInterno(w, r, "excluir", err)
	} else if n == 0 {
		respostas.Mensagem(w, http.StatusNotFound, "Registro não encontrado.")
	} else {
		respostas.SemCorpo(w, http.StatusNoContent)
	}
}

// aplicarSugestao grava os valores escolhidos na sugestão (cria ou altera o orçamento de cada categoria no mês).
func (s *Servico) aplicarSugestao(w http.ResponseWriter, r *http.Request) {
	ano, mes := campoAno(), campoMes()
	itens := respostas.CampoListaDeObjetos{Propriedade: "Itens", TipoItem: "Financas.Api.Dtos.ItemAplicarSugestaoDto",
		Minimo: 1, Maximo: 500, Obrigatoria: true, Novo: func() []respostas.Campo {
			return []respostas.Campo{&respostas.CampoInteiro{Propriedade: "CategoriaId"},
				&respostas.CampoDecimal{Propriedade: "Valor", Faixa: &respostas.Faixa{Minimo: 0.01, Maximo: 999999999}}}
		}}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.AplicarSugestaoOrcamentoDto", &ano, &mes, &itens) {
		return
	}
	type item struct {
		categoria int64
		valor     tipos.Decimal
	}
	var lista []item
	ids := map[int64]bool{}
	for _, campos := range itens.Itens {
		if campos == nil {
			s.erroInterno(w, r, "aplicar a sugestão", errors.New("item nulo na lista")) // o NullReferenceException do C#
			return
		}
		x := item{campos[0].(*respostas.CampoInteiro).Valor, campos[1].(*respostas.CampoDecimal).Valor}
		lista = append(lista, x)
		ids[x.categoria] = true
	}
	if len(ids) != len(lista) {
		respostas.Mensagem(w, http.StatusBadRequest, "Categoria repetida na sugestão.")
		return
	}
	ctx := r.Context()
	validas := 0
	for id := range ids {
		var tipo int64
		err := s.db.QueryRowContext(ctx, `SELECT "Tipo" FROM "Categoria" WHERE "Id" = ?`, id).Scan(&tipo)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			s.erroInterno(w, r, "ler as categorias", err)
			return
		}
		if err == nil && tipo == tipoCategoriaGasto {
			validas++
		}
	}
	if validas != len(ids) {
		respostas.Mensagem(w, http.StatusBadRequest, "Orçamentos são definidos para categorias de despesa.")
		return
	}
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		for _, x := range lista {
			if err := gravar(ctx, tx, x.categoria, ano.Valor, mes.Valor, x.valor); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.erroInterno(w, r, "aplicar a sugestão", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, ResultadoDto{TransacoesAtualizadas: len(lista)})
}

// copiarMesAnterior copia os orçamentos do mês anterior para o mês informado, sem sobrescrever os que já existem.
func (s *Servico) copiarMesAnterior(w http.ResponseWriter, r *http.Request) {
	ano, mes := campoAno(), campoMes()
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.CopiarOrcamentoDto", &ano, &mes) {
		return
	}
	ctx := r.Context()
	anterior := tipos.NovaData(int(ano.Valor), time.Month(mes.Valor), 1).SomarMeses(-1)
	var copiados int64
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO "Orcamento" ("CategoriaId", "Ano", "Mes", "Valor")
			SELECT o."CategoriaId", ?, ?, o."Valor" FROM "Orcamento" AS o WHERE o."Ano" = ? AND o."Mes" = ?
				AND NOT EXISTS (SELECT 1 FROM "Orcamento" AS e WHERE e."Ano" = ? AND e."Mes" = ? AND e."CategoriaId" = o."CategoriaId")
			ORDER BY o."Id"`, ano.Valor, mes.Valor, anterior.Ano(), int(anterior.Mes()), ano.Valor, mes.Valor)
		if err != nil {
			return fmt.Errorf("copiar os orçamentos: %w", err)
		}
		copiados, err = res.RowsAffected()
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "copiar do mês anterior", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, ResultadoDto{TransacoesAtualizadas: int(copiados)})
}
