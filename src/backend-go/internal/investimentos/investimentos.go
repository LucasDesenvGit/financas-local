// Package investimentos são os investimentos do Open Finance (#80): o InvestimentosController do C# (as 3 rotas de
// /api/investimentos: a lista, o histórico mês a mês e o renomear) e o HistoricoInvestimentosService (#173 e),
// ADR 0045), cuja gravação a sincronização do Open Finance (#83) também chama.
package investimentos

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

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/carteira"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// conversao é o que os investimentos usam do câmbio (a interface é de quem usa; o de verdade é o internal/cambio).
type conversao interface {
	ObterConversor(ctx context.Context, historicoDesde *tipos.Data) (*cambio.Conversor, error)
}

// Servico são os investimentos do Open Finance.
type Servico struct {
	db      *sql.DB
	cambio  conversao
	relogio relogio.Relogio
	log     *slog.Logger
}

// NovoServico monta o serviço.
func NovoServico(db *sql.DB, c conversao, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, cambio: c, relogio: r, log: log}
}

// Registrar põe as rotas de /api/investimentos no mux.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/investimentos", s.listar)
	mux.HandleFunc("GET /api/investimentos/historico", s.historico)
	mux.HandleFunc("PUT /api/investimentos/{id}/nome", s.renomear)
}

func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		return
	}
	s.log.ErrorContext(r.Context(), "erro nos investimentos", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}

// InvestimentoDto é o InvestimentoDto do C#.
type InvestimentoDto struct {
	Id                int64          `json:"id"`
	Nome              string         `json:"nome"`
	Tipo              string         `json:"tipo"`
	Subtipo           *string        `json:"subtipo"`
	Codigo            *string        `json:"codigo"`
	Emissor           *string        `json:"emissor"`
	Moeda             string         `json:"moeda"`
	Saldo             tipos.Decimal  `json:"saldo"`
	SaldoEmReais      *tipos.Decimal `json:"saldoEmReais"`
	ValorAplicado     *tipos.Decimal `json:"valorAplicado"`
	Rendimento        *tipos.Decimal `json:"rendimento"`
	TaxaAnual         *tipos.Decimal `json:"taxaAnual"`
	DataVencimento    *tipos.Data    `json:"dataVencimento"`
	Status            *string        `json:"status"`
	NomeInstituicao   *string        `json:"nomeInstituicao"`
	AtualizadoEm      tipos.DataHora `json:"atualizadoEm"`
	NomeExibicao      string         `json:"nomeExibicao"`
	NomePersonalizado *string        `json:"nomePersonalizado"`
}

// Listar são os investimentos pelo tipo (ordem binária, a do ORDER BY do SQLite) e, dentro dele, pelo maior saldo, com o
// saldo em reais e o nome de exibição (#170). O saldo é ordenado em Go (o EF Core usa a collation EF_DECIMAL; dinheiro
// é texto no banco); o empate fica na ordem do Id.
func (s *Servico) Listar(ctx context.Context) ([]InvestimentoDto, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT i."Id", i."Nome", i."Tipo", i."Subtipo", i."Codigo", i."Emissor", i."Moeda",
		i."Saldo", i."ValorAplicado", i."Rendimento", i."TaxaAnual", i."DataVencimento", i."Status", c."NomeInstituicao",
		i."AtualizadoEm", i."NomePersonalizado"
		FROM "Investimento" i JOIN "ConexaoBancaria" c ON c."Id" = i."ConexaoBancariaId" ORDER BY i."Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler os investimentos: %w", err)
	}
	defer linhas.Close()
	lista := []InvestimentoDto{}
	for linhas.Next() {
		var i InvestimentoDto
		if err := linhas.Scan(&i.Id, &i.Nome, &i.Tipo, &i.Subtipo, &i.Codigo, &i.Emissor, &i.Moeda, &i.Saldo,
			&i.ValorAplicado, &i.Rendimento, &i.TaxaAnual, &i.DataVencimento, &i.Status, &i.NomeInstituicao,
			&i.AtualizadoEm, &i.NomePersonalizado); err != nil {
			return nil, fmt.Errorf("ler investimento: %w", err)
		}
		lista = append(lista, i)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os investimentos: %w", err)
	}
	slices.SortStableFunc(lista, func(a, b InvestimentoDto) int {
		if c := strings.Compare(a.Tipo, b.Tipo); c != 0 {
			return c
		}
		return b.Saldo.Decimal().Cmp(a.Saldo.Decimal())
	})
	conversor, err := s.cambio.ObterConversor(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("câmbio dos investimentos: %w", err)
	}
	for k := range lista {
		i := &lista[k]
		i.SaldoEmReais = conversor.ParaReais(i.Saldo, i.Moeda)
		i.NomeExibicao = carteira.NomeDeExibicao(i.Nome, i.Tipo, i.Emissor, i.NomePersonalizado)
	}
	return lista, nil
}

func (s *Servico) listar(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Listar(r.Context())
	if err != nil {
		s.erroInterno(w, r, "listar", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

func (s *Servico) historico(w http.ResponseWriter, r *http.Request) {
	h, err := s.MesAMes(r.Context())
	if err != nil {
		s.erroInterno(w, r, "histórico", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, h)
}

// renomear dá um nome ao investimento ("Caixinha viagem", #170); vazio (ou só espaços) volta ao automático. A
// sincronização não mexe nele. O id que não é número de 32 bits é 404 sem corpo ({id:int}).
func (s *Servico) renomear(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.PathValue("id"), "+"), 10, 32)
	if err != nil {
		respostas.SemCorpo(w, http.StatusNotFound)
		return
	}
	nome := respostas.CampoTexto{Propriedade: "Nome", Maximo: 120, Opcional: true,
		MensagemDoTamanho: "O nome pode ter até 120 caracteres."}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.RenomearInvestimentoDto", &nome) {
		return
	}
	var novo *string
	if nome.Valor != nil && strings.TrimSpace(*nome.Valor) != "" {
		aparado := strings.TrimSpace(*nome.Valor)
		novo = &aparado
	}
	res, err := s.db.ExecContext(r.Context(), `UPDATE "Investimento" SET "NomePersonalizado" = ? WHERE "Id" = ?`, novo, id)
	if err != nil {
		s.erroInterno(w, r, "renomear", err)
		return
	}
	if n, err := res.RowsAffected(); err != nil {
		s.erroInterno(w, r, "renomear", err)
	} else if n == 0 {
		respostas.Mensagem(w, http.StatusNotFound, "Investimento não encontrado.")
	} else {
		respostas.SemCorpo(w, http.StatusNoContent)
	}
}
