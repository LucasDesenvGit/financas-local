// Package metas são as metas de economia (#79): o MetasController do C# (as 8 rotas de /api/metas), com o progresso
// pelo saldo da conta ou do cofrinho (#89), a ordem de prioridade, os aportes (#318) e o caixa do plano (#90, #149, o
// AssistenteMetasService; a divisão entre as metas é feita na tela, ADR 0035).
package metas

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/contas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/parametros"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/reservas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Servico são as metas.
type Servico struct {
	db         *sql.DB
	relogio    relogio.Relogio
	log        *slog.Logger
	reservas   *reservas.Servico
	parametros *parametros.Servico
	relatorios *relatorios.Servico
}

// NovoServico monta o serviço. As reservas dão o saldo dos cofrinhos.
func NovoServico(db *sql.DB, res *reservas.Servico, par *parametros.Servico, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, relogio: r, log: log, reservas: res, parametros: par, relatorios: relatorios.NovoServico(db, r)}
}

// Registrar põe as rotas de /api/metas no mux.
func Registrar(mux *http.ServeMux, s *Servico) {
	mux.HandleFunc("GET /api/metas", s.listar)
	mux.HandleFunc("GET /api/metas/plano", s.plano)
	mux.HandleFunc("PUT /api/metas/prioridades", s.prioridades)
	mux.HandleFunc("POST /api/metas", s.criar)
	mux.HandleFunc("PUT /api/metas/{id}", s.atualizar)
	mux.HandleFunc("POST /api/metas/{id}/aporte", s.aporte)
	mux.HandleFunc("GET /api/metas/{id}/aportes", s.aportes)
	mux.HandleFunc("DELETE /api/metas/{id}", s.excluir)
}

func (s *Servico) erroInterno(w http.ResponseWriter, r *http.Request, etapa string, err error) {
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		return
	}
	s.log.ErrorContext(r.Context(), "erro nas metas", "origem", "API", "etapa", etapa, "erro", err)
	servidor.ResponderErroInterno(w)
}

// idDaRota: o id que não é número de 32 bits é 404 sem corpo ({id:int}).
func idDaRota(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.PathValue("id"), "+"), 10, 32)
	if err != nil {
		respostas.SemCorpo(w, http.StatusNotFound)
		return 0, false
	}
	return id, true
}

func naoEncontrado(w http.ResponseWriter) {
	respostas.Mensagem(w, http.StatusNotFound, "Registro não encontrado.")
}

// MetaDto é o MetaDto do C#.
type MetaDto struct {
	Id                   int64          `json:"id"`
	Nome                 string         `json:"nome"`
	ValorAlvo            tipos.Decimal  `json:"valorAlvo"`
	ValorAtual           tipos.Decimal  `json:"valorAtual"`
	DataAlvo             *tipos.Data    `json:"dataAlvo"`
	ContaId              *int64         `json:"contaId"`
	ContaNome            *string        `json:"contaNome"`
	Cor                  string         `json:"cor"`
	Concluida            bool           `json:"concluida"`
	Percentual           tipos.Decimal  `json:"percentual"`
	AporteMensalSugerido *tipos.Decimal `json:"aporteMensalSugerido"`
	ValorInicial         tipos.Decimal  `json:"valorInicial"`
	SaldoConta           tipos.Decimal  `json:"saldoConta"`
	ReservaNome          *string        `json:"reservaNome"`
	SaldoReserva         *tipos.Decimal `json:"saldoReserva"`
	ReservaConferir      bool           `json:"reservaConferir"`
	ReservaNaoEncontrada bool           `json:"reservaNaoEncontrada"`
	Prioridade           int64          `json:"prioridade"`
	Icone                *string        `json:"icone"`
	GuardarPorMes        *tipos.Decimal `json:"guardarPorMes"`
}

// MetaAporteDto é um aporte da meta (#318), com a hora de Brasília (#359).
type MetaAporteDto struct {
	Data  tipos.Data    `json:"data"`
	Valor tipos.Decimal `json:"valor"`
	Hora  *string       `json:"hora"`
}

// meta é a linha de Meta, com o nome da conta (o Include do C#).
type meta struct {
	id                    int64
	nome                  string
	valorAlvo, valorAtual tipos.Decimal
	dataAlvo              *tipos.Data
	contaId               *int64
	contaNome             *string
	reservaNome           *string
	prioridade            int64
	cor                   string
	icone                 *string
	guardarPorMes         *tipos.Decimal
	concluida             bool
}

const colunasMeta = `m."Id", m."Nome", m."ValorAlvo", m."ValorAtual", m."DataAlvo", m."ContaId", c."Nome", m."ReservaNome",
	m."Prioridade", m."Cor", m."Icone", m."GuardarPorMes", m."Concluida"`

func lerMetas(ctx context.Context, q consultas.DBTX, onde string, argumentos ...any) ([]meta, error) {
	linhas, err := q.QueryContext(ctx, `SELECT `+colunasMeta+` FROM "Meta" AS m LEFT JOIN "Conta" AS c ON m."ContaId" = c."Id" `+
		onde, argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler as metas: %w", err)
	}
	defer linhas.Close()
	lista := []meta{}
	for linhas.Next() {
		var m meta
		if err := linhas.Scan(&m.id, &m.nome, &m.valorAlvo, &m.valorAtual, &m.dataAlvo, &m.contaId, &m.contaNome,
			&m.reservaNome, &m.prioridade, &m.cor, &m.icone, &m.guardarPorMes, &m.concluida); err != nil {
			return nil, fmt.Errorf("ler meta: %w", err)
		}
		lista = append(lista, m)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as metas: %w", err)
	}
	return lista, nil
}

// mesmoNome é o string.Equals(..., OrdinalIgnoreCase) do C#.
func mesmoNome(a, b string) bool {
	return texto.MaiusculasInvariantes(a) == texto.MaiusculasInvariantes(b)
}

// reservaDa é o cofrinho da meta na lista das reservas: a mesma conta e o mesmo nome, sem diferenciar maiúsculas.
func reservaDa(m meta, lista []reservas.ReservaDto) *reservas.ReservaDto {
	for i := range lista {
		if m.contaId != nil && lista[i].ContaId == *m.contaId && mesmoNome(lista[i].Nome, *m.reservaNome) {
			return &lista[i]
		}
	}
	return nil
}

// paraDto é o ParaDto do C#. O valor inicial (ValorAtual gravado) sempre conta. Com cofrinho, soma o saldo dele (nunca
// o da conta junto, para não contar duas vezes); sem cofrinho, o saldo da conta vinculada, se positivo (#89).
func paraDto(m meta, saldos map[int64]tipos.Decimal, lista []reservas.ReservaDto, hoje tipos.Data) (MetaDto, error) {
	var reserva *reservas.ReservaDto
	saldoConta := tipos.Zero
	if m.reservaNome != nil {
		reserva = reservaDa(m, lista)
	} else if m.contaId != nil {
		if saldo, ok := saldos[*m.contaId]; ok {
			saldoConta = tipos.Maior(tipos.Zero, saldo)
		}
	}
	atual := tipos.Somar(m.valorAtual, saldoConta)
	if reserva != nil {
		atual = tipos.Somar(atual, reserva.Saldo)
	}
	percentual := tipos.Zero
	if m.valorAlvo.Decimal().IsPositive() {
		razao, err := tipos.Dividir(atual, m.valorAlvo)
		if err != nil {
			return MetaDto{}, fmt.Errorf("percentual da meta %d: %w", m.id, err)
		}
		percentual = tipos.Menor(tipos.DecimalDeInteiro(100),
			tipos.Arredondar(tipos.Multiplicar(razao, tipos.DecimalDeInteiro(100)), 1))
	}
	var sugestao *tipos.Decimal
	if m.dataAlvo != nil && !m.concluida && atual.Decimal().LessThan(m.valorAlvo.Decimal()) {
		meses := (m.dataAlvo.Ano()-hoje.Ano())*12 + int(m.dataAlvo.Mes()) - int(hoje.Mes())
		v, err := tipos.Dividir(tipos.Subtrair(m.valorAlvo, atual), tipos.DecimalDeInteiro(int64(max(1, meses))))
		if err != nil {
			return MetaDto{}, fmt.Errorf("sugestão da meta %d: %w", m.id, err)
		}
		v = tipos.Arredondar(v, 2)
		sugestao = &v
	}
	dto := MetaDto{
		Id: m.id, Nome: m.nome, ValorAlvo: m.valorAlvo, ValorAtual: atual, DataAlvo: m.dataAlvo, ContaId: m.contaId,
		ContaNome: m.contaNome, Cor: m.cor, Concluida: m.concluida || !atual.Decimal().LessThan(m.valorAlvo.Decimal()),
		Percentual: percentual, AporteMensalSugerido: sugestao, ValorInicial: m.valorAtual, SaldoConta: saldoConta,
		ReservaNome: m.reservaNome, Prioridade: m.prioridade, Icone: m.icone, GuardarPorMes: m.guardarPorMes,
		ReservaNaoEncontrada: m.reservaNome != nil && reserva == nil,
	}
	if reserva != nil {
		nome, saldo := reserva.Nome, reserva.Saldo
		dto.ReservaNome, dto.SaldoReserva = &nome, &saldo
		dto.ReservaConferir = reserva.PodeTerSaldoAnterior || reserva.ConferenciaAntiga
	}
	return dto, nil
}

// paraDtos lê os saldos das contas e as reservas só se alguma meta precisa deles, como o C#.
func (s *Servico) paraDtos(ctx context.Context, lista []meta) ([]MetaDto, error) {
	saldos := map[int64]tipos.Decimal{}
	var res []reservas.ReservaDto
	precisaSaldos, precisaReservas := false, false
	for _, m := range lista {
		precisaSaldos = precisaSaldos || m.contaId != nil
		precisaReservas = precisaReservas || m.reservaNome != nil
	}
	var err error
	if precisaSaldos {
		if saldos, err = contas.Saldos(ctx, s.db); err != nil {
			return nil, err
		}
	}
	if precisaReservas {
		if res, err = s.reservas.Listar(ctx); err != nil {
			return nil, err
		}
	}
	hoje := relogio.Hoje(s.relogio)
	dtos := make([]MetaDto, 0, len(lista))
	for _, m := range lista {
		dto, err := paraDto(m, saldos, res, hoje)
		if err != nil {
			return nil, err
		}
		dtos = append(dtos, dto)
	}
	return dtos, nil
}

// listar: concluídas no fim; depois pela prioridade (0, sem ordem, vai para o fim), pela data (sem data no fim) e pelo
// nome. A ordem é a do ORDER BY que o EF Core gera (o nome na ordem binária do SQLite); nada de dinheiro nela.
func (s *Servico) listar(w http.ResponseWriter, r *http.Request) {
	lista, err := lerMetas(r.Context(), s.db, `ORDER BY m."Concluida", CASE WHEN m."Prioridade" = 0 THEN 2147483647
		ELSE m."Prioridade" END, m."DataAlvo" IS NULL, m."DataAlvo", m."Nome"`)
	if err != nil {
		s.erroInterno(w, r, "listar", err)
		return
	}
	dtos, err := s.paraDtos(r.Context(), lista)
	if err != nil {
		s.erroInterno(w, r, "listar", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, dtos)
}

// responderMeta devolve a meta gravada, como o ObterDtoAsync.
func (s *Servico) responderMeta(w http.ResponseWriter, r *http.Request, id int64) {
	lista, err := lerMetas(r.Context(), s.db, `WHERE m."Id" = ?`, id)
	if err == nil && len(lista) == 0 {
		err = fmt.Errorf("meta %d sumiu depois de gravada", id)
	}
	var dtos []MetaDto
	if err == nil {
		dtos, err = s.paraDtos(r.Context(), lista)
	}
	if err != nil {
		s.erroInterno(w, r, "ler a meta gravada", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, dtos[0])
}

// entrada é o SalvarMetaDto, com os atributos de validação do C#.
type entrada struct {
	nome, cor             respostas.CampoTexto
	valorAlvo, valorAtual respostas.CampoDecimal
	dataAlvo              respostas.CampoDataOpcional
	contaId               respostas.CampoInteiroOpcional
	concluida             respostas.CampoBool
	reservaNome, icone    respostas.CampoTexto
	guardarPorMes         respostas.CampoDecimalOpcional
}

var (
	padraoCor   = respostas.NovoPadrao(`^#[0-9a-fA-F]{6}$`)
	padraoIcone = respostas.NovoPadrao(`^[a-z-]+$`)
	faixaValor  = &respostas.Faixa{Minimo: 0.01, Maximo: 999999999}
)

func lerEntrada(w http.ResponseWriter, r *http.Request) (*entrada, bool) {
	e := &entrada{
		nome:          respostas.CampoTexto{Propriedade: "Nome", Maximo: 120},
		valorAlvo:     respostas.CampoDecimal{Propriedade: "ValorAlvo", Faixa: faixaValor},
		valorAtual:    respostas.CampoDecimal{Propriedade: "ValorAtual", Faixa: &respostas.Faixa{Minimo: 0, Maximo: 999999999}},
		dataAlvo:      respostas.CampoDataOpcional{Propriedade: "DataAlvo"},
		contaId:       respostas.CampoInteiroOpcional{Propriedade: "ContaId"},
		cor:           respostas.CampoTexto{Propriedade: "Cor", Maximo: math.MaxInt32, Padrao: padraoCor},
		concluida:     respostas.CampoBool{Propriedade: "Concluida"},
		reservaNome:   respostas.CampoTexto{Propriedade: "ReservaNome", Maximo: 100, Opcional: true},
		icone:         respostas.CampoTexto{Propriedade: "Icone", Maximo: 30, Opcional: true, Padrao: padraoIcone},
		guardarPorMes: respostas.CampoDecimalOpcional{Propriedade: "GuardarPorMes", Faixa: faixaValor},
	}
	ok := respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.SalvarMetaDto", &e.nome, &e.valorAlvo, &e.valorAtual,
		&e.dataAlvo, &e.contaId, &e.cor, &e.concluida, &e.reservaNome, &e.icone, &e.guardarPorMes)
	return e, ok
}

func (e *entrada) conta() *int64 {
	if e.contaId.Valor == nil {
		return nil
	}
	v := int64(*e.contaId.Valor)
	return &v
}

// conferirConta: "Conta inválida." para a conta que não existe. Devolve false se já respondeu.
func (s *Servico) conferirConta(w http.ResponseWriter, r *http.Request, conta *int64) bool {
	if conta == nil {
		return true
	}
	var existe bool
	err := s.db.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM "Conta" WHERE "Id" = ?)`, *conta).Scan(&existe)
	if err != nil {
		s.erroInterno(w, r, "conferir a conta", err)
		return false
	}
	if !existe {
		respostas.Mensagem(w, http.StatusBadRequest, "Conta inválida.")
		return false
	}
	return true
}

// cofrinho é o ValidarCofrinhoAsync do C#: cofrinho só com conta, e precisa existir nela (conta + nome, sem
// diferenciar maiúsculas). Ao editar, manter o cofrinho que já estava gravado é aceito mesmo que ele tenha sumido do
// extrato (o cartão avisa). Devolve o nome como está no extrato, ou a mensagem de erro.
func (s *Servico) cofrinho(ctx context.Context, e *entrada, atual *meta) (nome *string, mensagem string, err error) {
	if e.reservaNome.Valor == nil || strings.TrimSpace(*e.reservaNome.Valor) == "" {
		return nil, "", nil
	}
	pedido := strings.TrimSpace(*e.reservaNome.Valor)
	conta := e.conta()
	if conta == nil {
		return nil, "Escolha a conta do cofrinho.", nil
	}
	lista, err := s.reservas.Listar(ctx)
	if err != nil {
		return nil, "", err
	}
	for _, r := range lista {
		if r.ContaId == *conta && mesmoNome(r.Nome, pedido) {
			return &r.Nome, "", nil
		}
	}
	if atual != nil && atual.contaId != nil && *atual.contaId == *conta && atual.reservaNome != nil &&
		mesmoNome(*atual.reservaNome, pedido) {
		return atual.reservaNome, "", nil
	}
	return nil, "Cofrinho inválido para esta conta.", nil
}

// valores são as colunas que o Aplicar do C# grava, na ordem do INSERT e do UPDATE.
func (e *entrada) valores(cofrinho *string) []any {
	conta := e.conta()
	if conta == nil {
		cofrinho = nil
	}
	return []any{strings.TrimSpace(*e.nome.Valor), e.valorAlvo.Valor, e.valorAtual.Valor, e.dataAlvo.Valor, conta,
		cofrinho, *e.cor.Valor, e.icone.Valor, e.guardarPorMes.Valor, e.concluida.Valor}
}

func (s *Servico) criar(w http.ResponseWriter, r *http.Request) {
	e, ok := lerEntrada(w, r)
	if !ok || !s.conferirConta(w, r, e.conta()) {
		return
	}
	ctx := r.Context()
	cofrinho, mensagem, err := s.cofrinho(ctx, e, nil)
	if err != nil {
		s.erroInterno(w, r, "conferir o cofrinho", err)
		return
	}
	if mensagem != "" {
		respostas.Mensagem(w, http.StatusBadRequest, mensagem)
		return
	}
	var id int64
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		// Meta nova entra no fim da ordem de prioridade.
		var ultima sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT MAX("Prioridade") FROM "Meta"`).Scan(&ultima); err != nil {
			return fmt.Errorf("ler a última prioridade: %w", err)
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO "Meta" ("Nome", "ValorAlvo", "ValorAtual", "DataAlvo", "ContaId",
			"ReservaNome", "Cor", "Icone", "GuardarPorMes", "Concluida", "Prioridade", "CriadoEm")
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			append(e.valores(cofrinho), ultima.Int64+1, tipos.NovaDataHora(s.relogio.Agora()))...)
		if err != nil {
			return fmt.Errorf("criar a meta: %w", err)
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		s.erroInterno(w, r, "criar", err)
		return
	}
	s.responderMeta(w, r, id)
}

func (s *Servico) atualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	e, ok := lerEntrada(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	lista, err := lerMetas(ctx, s.db, `WHERE m."Id" = ?`, id)
	if err != nil {
		s.erroInterno(w, r, "ler a meta", err)
		return
	}
	if len(lista) == 0 {
		naoEncontrado(w)
		return
	}
	if !s.conferirConta(w, r, e.conta()) {
		return
	}
	cofrinho, mensagem, err := s.cofrinho(ctx, e, &lista[0])
	if err != nil {
		s.erroInterno(w, r, "conferir o cofrinho", err)
		return
	}
	if mensagem != "" {
		respostas.Mensagem(w, http.StatusBadRequest, mensagem)
		return
	}
	_, err = s.db.ExecContext(ctx, `UPDATE "Meta" SET "Nome" = ?, "ValorAlvo" = ?, "ValorAtual" = ?, "DataAlvo" = ?,
		"ContaId" = ?, "ReservaNome" = ?, "Cor" = ?, "Icone" = ?, "GuardarPorMes" = ?, "Concluida" = ? WHERE "Id" = ?`,
		append(e.valores(cofrinho), id)...)
	if err != nil {
		s.erroInterno(w, r, "atualizar", err)
		return
	}
	s.responderMeta(w, r, id)
}

// aporte soma (ou subtrai, se negativo) um valor ao progresso da meta: para no zero, conclui ao chegar ao alvo e guarda
// o que de fato mudou com a data de hoje (#318).
func (s *Servico) aporte(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	valor := respostas.CampoDecimal{Propriedade: "Valor", Faixa: &respostas.Faixa{Minimo: -999999999, Maximo: 999999999}}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.AporteDto", &valor) {
		return
	}
	ctx := r.Context()
	encontrada := true
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		var antes, alvo tipos.Decimal
		var concluida bool
		err := tx.QueryRowContext(ctx, `SELECT "ValorAtual", "ValorAlvo", "Concluida" FROM "Meta" WHERE "Id" = ?`, id).
			Scan(&antes, &alvo, &concluida)
		if errors.Is(err, sql.ErrNoRows) {
			encontrada = false
			return nil
		}
		if err != nil {
			return fmt.Errorf("ler a meta: %w", err)
		}
		depois := tipos.Maior(tipos.Zero, tipos.Somar(antes, valor.Valor))
		concluida = concluida || !depois.Decimal().LessThan(alvo.Decimal())
		if _, err := tx.ExecContext(ctx, `UPDATE "Meta" SET "ValorAtual" = ?, "Concluida" = ? WHERE "Id" = ?`,
			depois, concluida, id); err != nil {
			return fmt.Errorf("gravar o aporte na meta: %w", err)
		}
		if depois.Decimal().Equal(antes.Decimal()) {
			return nil
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO "MetaAporte" ("MetaId", "Data", "Valor", "CriadoEm") VALUES (?, ?, ?, ?)`,
			id, relogio.Hoje(s.relogio), tipos.Subtrair(depois, antes), tipos.NovaDataHora(s.relogio.Agora()))
		if err != nil {
			return fmt.Errorf("guardar o aporte: %w", err)
		}
		return nil
	})
	if err != nil {
		s.erroInterno(w, r, "aporte", err)
		return
	}
	if !encontrada {
		naoEncontrado(w)
		return
	}
	s.responderMeta(w, r, id)
}

// aportes são os da meta, do mais antigo ao mais novo, com a hora de Brasília (CriadoEm é gravado em UTC).
func (s *Servico) aportes(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var existe bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM "Meta" WHERE "Id" = ?)`, id).Scan(&existe); err != nil {
		s.erroInterno(w, r, "conferir a meta", err)
		return
	}
	if !existe {
		naoEncontrado(w)
		return
	}
	linhas, err := s.db.QueryContext(ctx, `SELECT "Data", "Valor", "CriadoEm" FROM "MetaAporte" WHERE "MetaId" = ?
		ORDER BY "Data", "Id"`, id)
	if err != nil {
		s.erroInterno(w, r, "ler os aportes", err)
		return
	}
	defer linhas.Close()
	lista := []MetaAporteDto{}
	for linhas.Next() {
		var a MetaAporteDto
		var criado tipos.DataHora
		if err := linhas.Scan(&a.Data, &a.Valor, &criado); err != nil {
			s.erroInterno(w, r, "ler aporte", err)
			return
		}
		hora := relogio.ParaHoraLocal(criado.Tempo()).Format("15:04")
		a.Hora = &hora
		lista = append(lista, a)
	}
	if err := linhas.Err(); err != nil {
		s.erroInterno(w, r, "ler os aportes", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, lista)
}

// prioridades: a primeira da lista é a 1ª; as metas fora da lista vão para o fim, na ordem que tinham (sem ordem, 0,
// no fim; o empate fica na ordem do Id, a da tabela).
func (s *Servico) prioridades(w http.ResponseWriter, r *http.Request) {
	ids := respostas.CampoLista{Propriedade: "Ids", Item: respostas.ItemInt, Minimo: 1, Maximo: 200}
	if !respostas.LerObjeto(w, r, "dto", "Financas.Api.Dtos.PrioridadesMetasDto", &ids) {
		return
	}
	vistos := map[int64]bool{}
	for _, id := range ids.Valor {
		if vistos[id] {
			respostas.Mensagem(w, http.StatusBadRequest, "Meta repetida na ordem.")
			return
		}
		vistos[id] = true
	}
	ctx := r.Context()
	mensagem := ""
	err := banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		lista, err := lerMetas(ctx, tx, `ORDER BY m."Id"`)
		if err != nil {
			return err
		}
		existe := map[int64]bool{}
		for _, m := range lista {
			existe[m.id] = true
		}
		for _, id := range ids.Valor {
			if !existe[id] {
				mensagem = "Meta inválida."
				return nil
			}
		}
		nova := map[int64]int64{}
		proxima := int64(1)
		for _, id := range ids.Valor {
			nova[id] = proxima
			proxima++
		}
		var resto []meta
		for _, m := range lista {
			if !vistos[m.id] {
				resto = append(resto, m)
			}
		}
		ordem := func(m meta) int64 {
			if m.prioridade == 0 {
				return math.MaxInt32
			}
			return m.prioridade
		}
		slices.SortStableFunc(resto, func(a, b meta) int { return cmp.Compare(ordem(a), ordem(b)) })
		for _, m := range resto {
			nova[m.id] = proxima
			proxima++
		}
		for _, m := range lista {
			if nova[m.id] == m.prioridade {
				continue
			}
			if _, err := tx.ExecContext(ctx, `UPDATE "Meta" SET "Prioridade" = ? WHERE "Id" = ?`, nova[m.id], m.id); err != nil {
				return fmt.Errorf("gravar a prioridade da meta %d: %w", m.id, err)
			}
		}
		return nil
	})
	if err != nil {
		s.erroInterno(w, r, "prioridades", err)
		return
	}
	if mensagem != "" {
		respostas.Mensagem(w, http.StatusBadRequest, mensagem)
		return
	}
	respostas.SemCorpo(w, http.StatusNoContent)
}

// excluir apaga a meta; os aportes saem junto pela chave estrangeira (ON DELETE CASCADE), como no ExecuteDelete.
func (s *Servico) excluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaRota(w, r)
	if !ok {
		return
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM "Meta" WHERE "Id" = ?`, id)
	if err != nil {
		s.erroInterno(w, r, "excluir", err)
		return
	}
	if n, err := res.RowsAffected(); err != nil {
		s.erroInterno(w, r, "excluir", err)
	} else if n == 0 {
		naoEncontrado(w)
	} else {
		respostas.SemCorpo(w, http.StatusNoContent)
	}
}
