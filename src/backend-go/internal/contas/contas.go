// Pacote contas são as rotas de /api/contas (o ContasController do C#) e o saldo de cada conta (CalcularSaldosAsync
// do ContaService). A fatura do cartão fica no pacote faturas, e o banco de cada conta no pacote bancos.
package contas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/bancos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/faturas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Tipos e origens da conta (Entidades/Enums.cs).
const (
	CartaoCredito = 3
	Investimento  = 4

	Manual      = 1
	OpenFinance = 2
)

const mensagemNaoEncontrado = "Registro não encontrado."

// Servico atende as rotas das contas.
type Servico struct {
	db      *sql.DB
	relogio relogio.Relogio
	log     *slog.Logger
}

// NovoServico monta o serviço com o banco, o relógio e o log.
func NovoServico(db *sql.DB, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, relogio: r, log: log}
}

// Registrar põe as rotas do ContasController no mux. As de texto fixo ("bancos", "faturas-do-mes") ganham do {id}.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/contas", s.listar)
	mux.HandleFunc("GET /api/contas/{id}", s.obter)
	mux.HandleFunc("GET /api/contas/faturas-do-mes", s.faturasDoMes)
	mux.HandleFunc("GET /api/contas/bancos", s.bancos)
	mux.HandleFunc("POST /api/contas", s.criar)
	mux.HandleFunc("PUT /api/contas/{id}", s.atualizar)
	mux.HandleFunc("DELETE /api/contas/{id}", s.excluir)
	mux.HandleFunc("GET /api/contas/{id}/faturas", s.faturasDaConta)
	mux.HandleFunc("PUT /api/contas/{id}/limite-ajustado", s.limiteAjustado)
}

func (s *Servico) agora() tipos.DataHora { return tipos.NovaDataHora(s.relogio.Agora()) }

// idDaRota é o {id:int} do C#: o que não é número de 32 bits não casa com a rota, e a resposta é 404 sem corpo.
func idDaRota(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.PathValue("id"), "+"), 10, 32)
	if err != nil {
		respostas.SemCorpo(w, http.StatusNotFound)
		return 0, false
	}
	return id, true
}

// erroInterno é o 500 do .NET, com o erro no log; requisição cancelada não responde nem registra.
func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		return
	}
	s.log.ErrorContext(r.Context(), "erro nas contas", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}

// conta é uma linha de Conta, com o nome da instituição da conexão, só com o que as rotas usam.
type conta struct {
	Id                      int64
	ConexaoBancariaId       *int64
	Nome                    string
	Tipo                    int64
	Origem                  int64
	Numero                  *string
	Moeda                   string
	SaldoInicial            tipos.Decimal
	Saldo                   tipos.Decimal
	LimiteCredito           *tipos.Decimal
	LimiteDisponivel        *tipos.Decimal
	LimiteAjustadoNoApp     *bool
	LimiteDiferencaAvaliada *tipos.Decimal
	DiaFechamento           *int64
	DiaVencimento           *int64
	Cor                     string
	IncluirNoSaldo          bool
	Arquivada               bool
	Banco                   *string
	ContaVinculadaId        *int64
	AtualizadoEm            tipos.DataHora
	NomeInstituicao         *string
}

const colunasDaConta = `c."Id", c."ConexaoBancariaId", c."Nome", c."Tipo", c."Origem", c."Numero", c."Moeda",
	c."SaldoInicial", c."Saldo", c."LimiteCredito", c."LimiteDisponivel", c."LimiteAjustadoNoApp",
	c."LimiteDiferencaAvaliada", c."DiaFechamento", c."DiaVencimento", c."Cor", c."IncluirNoSaldo", c."Arquivada",
	c."Banco", c."ContaVinculadaId", c."AtualizadoEm", x."NomeInstituicao"`

// lerContas lê as contas que casam com o filtro, com a instituição da conexão.
func lerContas(ctx context.Context, q consultas.DBTX, onde string, argumentos ...any) ([]conta, error) {
	linhas, err := q.QueryContext(ctx, `SELECT `+colunasDaConta+` FROM "Conta" AS c
		LEFT JOIN "ConexaoBancaria" AS x ON c."ConexaoBancariaId" = x."Id" `+onde, argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler as contas: %w", err)
	}
	defer linhas.Close()
	var lista []conta
	for linhas.Next() {
		var c conta
		if err := linhas.Scan(&c.Id, &c.ConexaoBancariaId, &c.Nome, &c.Tipo, &c.Origem, &c.Numero, &c.Moeda,
			&c.SaldoInicial, &c.Saldo, &c.LimiteCredito, &c.LimiteDisponivel, &c.LimiteAjustadoNoApp,
			&c.LimiteDiferencaAvaliada, &c.DiaFechamento, &c.DiaVencimento, &c.Cor, &c.IncluirNoSaldo, &c.Arquivada,
			&c.Banco, &c.ContaVinculadaId, &c.AtualizadoEm, &c.NomeInstituicao); err != nil {
			return nil, fmt.Errorf("ler conta: %w", err)
		}
		lista = append(lista, c)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as contas: %w", err)
	}
	return lista, nil
}

// lerConta lê uma conta pelo Id; nil quando ela não existe.
func lerConta(ctx context.Context, q consultas.DBTX, id int64) (*conta, error) {
	lista, err := lerContas(ctx, q, `WHERE c."Id" = ?`, id)
	if err != nil || len(lista) == 0 {
		return nil, err
	}
	return &lista[0], nil
}

// noFormatoDoBanco é o decimal como ele volta do SQLite quando o EF Core faz a conta no banco (o Sum é o ef_sum, e o
// resultado volta em texto como ele grava: "0.0" e até 28 casas, sem zeros à direita). -0.25 + -0.75 vira -1.0, e não
// -1.00, e a escala aparece no JSON (gabarito testdata/saldos.json, do próprio CalcularSaldosAsync).
func noFormatoDoBanco(d tipos.Decimal) tipos.Decimal {
	lido, err := tipos.NovoDecimal(d.TextoNoBanco())
	if err != nil {
		return d
	}
	return lido
}

// Saldos é o CalcularSaldosAsync do C#: o saldo de cada conta. A do Open Finance vale o saldo que a instituição
// informa; a manual, o saldo inicial mais as transações efetivadas.
func Saldos(ctx context.Context, q consultas.DBTX) (map[int64]tipos.Decimal, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "Origem", "Saldo", "SaldoInicial" FROM "Conta"`)
	if err != nil {
		return nil, fmt.Errorf("ler os saldos: %w", err)
	}
	type saldoDaConta struct {
		origem              int64
		saldo, saldoInicial tipos.Decimal
	}
	var ordem []int64
	contas := map[int64]saldoDaConta{}
	for linhas.Next() {
		var id int64
		var c saldoDaConta
		if err := linhas.Scan(&id, &c.origem, &c.saldo, &c.saldoInicial); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler saldo: %w", err)
		}
		ordem = append(ordem, id)
		contas[id] = c
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os saldos: %w", err)
	}

	// Dinheiro é texto no banco: a soma é feita aqui, e não com SUM no SQL.
	linhas, err = q.QueryContext(ctx, `SELECT t."ContaId", t."Valor" FROM "Transacao" AS t
		INNER JOIN "Conta" AS c ON t."ContaId" = c."Id" WHERE c."Origem" = 1 AND t."Status" = 1`)
	if err != nil {
		return nil, fmt.Errorf("ler os lançamentos das contas manuais: %w", err)
	}
	defer linhas.Close()
	somas := map[int64]tipos.Decimal{}
	for linhas.Next() {
		var id int64
		var valor tipos.Decimal
		if err := linhas.Scan(&id, &valor); err != nil {
			return nil, fmt.Errorf("ler lançamento da conta manual: %w", err)
		}
		somas[id] = tipos.Somar(somas[id], valor)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os lançamentos das contas manuais: %w", err)
	}

	saldos := make(map[int64]tipos.Decimal, len(ordem))
	for _, id := range ordem {
		c := contas[id]
		switch soma, ok := somas[id]; {
		case c.origem == OpenFinance:
			saldos[id] = c.saldo
		case ok:
			saldos[id] = tipos.Somar(c.saldoInicial, noFormatoDoBanco(soma))
		default:
			saldos[id] = c.saldoInicial
		}
	}
	return saldos, nil
}

// extras é o que a lista mostra além da própria conta: o saldo, o banco que vale e a fatura do mês atual dos cartões.
type extras struct {
	saldos       map[int64]tipos.Decimal
	bancos       map[int64]*string
	faturasDoMes map[int64]faturas.DoMes
}

func (s *Servico) extras(ctx context.Context) (extras, error) {
	hoje := relogio.Hoje(s.relogio)
	lista, err := faturas.DoMesDosCartoes(ctx, s.db, relogio.PrimeiroDiaMes(hoje.Ano(), hoje.Mes()),
		relogio.UltimoDiaMes(hoje.Ano(), hoje.Mes()), hoje)
	if err != nil {
		return extras{}, err
	}
	e := extras{faturasDoMes: make(map[int64]faturas.DoMes, len(lista))}
	for _, f := range lista {
		e.faturasDoMes[f.ContaId] = f
	}
	if e.saldos, err = Saldos(ctx, s.db); err != nil {
		return extras{}, err
	}
	if e.bancos, err = bancos.Carregar(ctx, s.db); err != nil {
		return extras{}, err
	}
	return e, nil
}

// ContaDto, na ordem do record do C#.
type ContaDto struct {
	Id                      int64          `json:"id"`
	Nome                    string         `json:"nome"`
	Tipo                    int64          `json:"tipo"`
	Origem                  int64          `json:"origem"`
	Numero                  *string        `json:"numero"`
	Moeda                   string         `json:"moeda"`
	Saldo                   tipos.Decimal  `json:"saldo"`
	SaldoInicial            tipos.Decimal  `json:"saldoInicial"`
	LimiteCredito           *tipos.Decimal `json:"limiteCredito"`
	LimiteDisponivel        *tipos.Decimal `json:"limiteDisponivel"`
	DiaFechamento           *int64         `json:"diaFechamento"`
	DiaVencimento           *int64         `json:"diaVencimento"`
	Cor                     string         `json:"cor"`
	IncluirNoSaldo          bool           `json:"incluirNoSaldo"`
	Arquivada               bool           `json:"arquivada"`
	ConexaoBancariaId       *int64         `json:"conexaoBancariaId"`
	NomeInstituicao         *string        `json:"nomeInstituicao"`
	AtualizadoEm            json.Marshaler `json:"atualizadoEm"`
	Banco                   *string        `json:"banco"`
	BancoDefinido           bool           `json:"bancoDefinido"`
	ContaVinculadaId        *int64         `json:"contaVinculadaId"`
	FaturaMes               *faturas.DoMes `json:"faturaMes"`
	LimiteAjustadoNoApp     *bool          `json:"limiteAjustadoNoApp"`
	LimiteDiferencaAvaliada *tipos.Decimal `json:"limiteDiferencaAvaliada"`
}

// paraDto é o ParaDto do controller. atualizadoEm é o carimbo como o C# o tem: lido do banco (sem fuso) ou o que ele
// acabou de gravar (DateTime.UtcNow, com "Z").
func paraDto(c conta, ex extras, atualizadoEm json.Marshaler) ContaDto {
	saldo, ok := ex.saldos[c.Id]
	if !ok {
		saldo = c.Saldo
		if c.Origem == Manual {
			saldo = c.SaldoInicial
		}
	}
	// Conta recém-criada ainda não está no cálculo de bancos: detecta na hora.
	banco, ok := ex.bancos[c.Id]
	if !ok {
		switch {
		case c.Banco != nil && *c.Banco == bancos.Outro:
			banco = nil
		case c.Banco != nil:
			banco = c.Banco
		default:
			banco = bancos.Detectar(c.NomeInstituicao, &c.Nome)
		}
	}
	limiteDisponivel := c.LimiteDisponivel
	if c.Origem == Manual && c.Tipo == CartaoCredito && c.LimiteCredito != nil {
		v := tipos.Somar(*c.LimiteCredito, saldo) // saldo de cartão é negativo (dívida)
		limiteDisponivel = &v
	}
	var faturaMes *faturas.DoMes
	if f, ok := ex.faturasDoMes[c.Id]; ok && c.Tipo == CartaoCredito {
		faturaMes = &f
	}
	return ContaDto{Id: c.Id, Nome: c.Nome, Tipo: c.Tipo, Origem: c.Origem, Numero: c.Numero, Moeda: c.Moeda,
		Saldo: saldo, SaldoInicial: c.SaldoInicial, LimiteCredito: c.LimiteCredito, LimiteDisponivel: limiteDisponivel,
		DiaFechamento: c.DiaFechamento, DiaVencimento: c.DiaVencimento, Cor: c.Cor, IncluirNoSaldo: c.IncluirNoSaldo,
		Arquivada: c.Arquivada, ConexaoBancariaId: c.ConexaoBancariaId, NomeInstituicao: c.NomeInstituicao,
		AtualizadoEm: atualizadoEm, Banco: banco, BancoDefinido: c.Banco != nil, ContaVinculadaId: c.ContaVinculadaId,
		FaturaMes: faturaMes, LimiteAjustadoNoApp: c.LimiteAjustadoNoApp, LimiteDiferencaAvaliada: c.LimiteDiferencaAvaliada}
}
