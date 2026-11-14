// Pacote recebimento liga o crédito do salário na conta (a transação) ao holerite do mês (#212, ADR 0065): "esta
// receita é o salário de outubro: bruto X, descontos Y". A receita dos relatórios continua sendo só a transação; a
// ligação só explica, e nunca soma nada (não conta duas vezes).
package recebimento

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/tipolancamento"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Folga é o quanto o crédito pode ficar longe do líquido para "valor perto do líquido" (5%).
var Folga = decimal.RequireFromString("0.05")

// Motivos da sugestão, em texto (Nielsen 1: por que o site sugere).
const (
	MotivoValor        = "Valor perto do líquido do holerite"
	MotivoParte        = "Menor que o líquido: pode ser o adiantamento ou o saldo"
	MotivoSalario      = "O banco marcou como salário"
	MotivoEmpregador   = "Cita o empregador"
	MotivoContaSalario = "Na conta do salário"
)

// Transacao é um crédito que pode pagar o holerite.
type Transacao struct {
	Id        int64         `json:"id"`
	Data      tipos.Data    `json:"data"`
	Descricao string        `json:"descricao"`
	ContaId   int64         `json:"contaId"`
	Conta     string        `json:"conta"`
	Valor     tipos.Decimal `json:"valor"`
	Livre     tipos.Decimal `json:"livre"` // o que ainda não está ligado a outro holerite
	Salario   bool          `json:"-"`     // o tipo de lançamento é salário (ADR 0025)
}

// Sugestao é um crédito com os motivos e a pontuação.
type Sugestao struct {
	Transacao
	Motivos []string `json:"motivos"`
	Pontos  int      `json:"-"`
}

// Pontuar diz o quanto o crédito parece o salário do holerite e por quê. Pontos 2 ou mais viram sugestão.
func Pontuar(t Transacao, liquido *tipos.Decimal, empregador string, contaDoSalario *int64) (int, []string) {
	pontos, motivos := 0, []string{}
	livre := t.Livre.Decimal()
	if liquido != nil && liquido.Decimal().IsPositive() {
		l := liquido.Decimal()
		if livre.Sub(l).Abs().LessThanOrEqual(l.Mul(Folga)) {
			pontos, motivos = pontos+3, append(motivos, MotivoValor)
		} else if livre.LessThan(l) {
			pontos, motivos = pontos+1, append(motivos, MotivoParte)
		}
	}
	if t.Salario {
		pontos, motivos = pontos+2, append(motivos, MotivoSalario)
	}
	if citaEmpregador(t.Descricao, empregador) {
		pontos, motivos = pontos+2, append(motivos, MotivoEmpregador)
	}
	if contaDoSalario != nil && *contaDoSalario == t.ContaId {
		pontos, motivos = pontos+1, append(motivos, MotivoContaSalario)
	}
	return pontos, motivos
}

// citaEmpregador: alguma palavra do empregador com 4 letras ou mais (sem "Ltda" e "Eireli") aparece inteira na
// descrição, sem diferença de maiúsculas nem de acentos (como o tipo de lançamento, ADR 0025).
func citaEmpregador(descricao, empregador string) bool {
	d := texto.ParaPalavras(descricao)
	for _, p := range strings.Fields(texto.ParaPalavras(empregador)) {
		if len([]rune(p)) >= 4 && p != "LTDA" && p != "EIRELI" && texto.TemPalavra(d, p) {
			return true
		}
	}
	return false
}

// Recebimento é uma ligação: a transação e a parte dela que é deste holerite.
type Recebimento struct {
	Id int64 `json:"id"`
	Transacao
	ValorLigado tipos.Decimal `json:"valorLigado"`
}

// Resposta é o recebimento do holerite: o líquido, o que já foi ligado, a diferença e as sugestões.
type Resposta struct {
	Liquido      *tipos.Decimal `json:"liquido"` // nil: holerite sem cálculo
	Recebido     tipos.Decimal  `json:"recebido"`
	Diferenca    *tipos.Decimal `json:"diferenca"` // recebido - líquido
	Recebimentos []Recebimento  `json:"recebimentos"`
	Sugestoes    []Sugestao     `json:"sugestoes"`
	Outros       []Transacao    `json:"outros"` // os outros créditos livres da janela, para ligar à mão
}

// Servico liga as transações aos holerites.
type Servico struct {
	db        *sql.DB
	empregos  *emprego.Servico
	holerites *holerite.Servico
	relogio   relogio.Relogio
	log       *slog.Logger
}

func NovoServico(db *sql.DB, empregos *emprego.Servico, holerites *holerite.Servico, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, empregos: empregos, holerites: holerites, relogio: r, log: log}
}

type erroDeValidacao string

func (e erroDeValidacao) Error() string { return string(e) }

const selecaoTransacao = `SELECT t."Id", t."Data", t."Descricao", t."DescricaoOriginal", t."Valor", t."MeioPagamento",
	t."MesmaTitularidade", t."TransferenciaGrupo" IS NOT NULL, c."Tipo", c."Id", c."Nome",
	COALESCE((SELECT GROUP_CONCAT(r."Valor", ';') FROM "HoleriteRecebimento" r WHERE r."TransacaoId" = t."Id" AND r."HoleriteId" <> ?), '')
	FROM "Transacao" t JOIN "Conta" c ON c."Id" = t."ContaId"`

// lerTransacoes lê os créditos (valor positivo, filtrado em Go) da consulta, com o que está livre (fora do holerite dado).
func (s *Servico) lerTransacoes(ctx context.Context, holeriteId int64, filtro string, args ...any) ([]Transacao, error) {
	linhas, err := s.db.QueryContext(ctx, selecaoTransacao+` WHERE `+filtro+` ORDER BY t."Data", t."Id"`,
		append([]any{holeriteId}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("ler as transações: %w", err)
	}
	defer linhas.Close()
	var lista []Transacao
	for linhas.Next() {
		var t Transacao
		var original, meio *string
		var mesma *bool
		var doSistema bool
		var tipoConta int64
		var ligados string
		if err := linhas.Scan(&t.Id, &t.Data, &t.Descricao, &original, &t.Valor, &meio, &mesma, &doSistema, &tipoConta, &t.ContaId,
			&t.Conta, &ligados); err != nil {
			return nil, err
		}
		if !t.Valor.Decimal().IsPositive() {
			continue // só crédito paga o salário (dinheiro se compara em Go, nunca no SQL)
		}
		t.Valor = tipos.DeDecimal(t.Valor.Decimal().Round(2))
		livre := t.Valor.Decimal()
		for _, parte := range strings.Split(ligados, ";") {
			if v, err := decimal.NewFromString(parte); err == nil {
				livre = livre.Sub(v)
			}
		}
		t.Livre = tipos.DeDecimal(decimal.Max(livre, decimal.Zero).Round(2))
		t.Salario = tipolancamento.Classificar(t.Valor, t.Descricao, original, meio, mesma, doSistema, tipoConta) == tipolancamento.Salario
		lista = append(lista, t)
	}
	return lista, linhas.Err()
}

// Obter devolve o recebimento do holerite com as sugestões: os créditos do dia 15 do mês ao dia 15 do mês seguinte,
// com o que está livre, que parecem o salário (2 pontos ou mais), do mais provável para o menos; os outros créditos
// livres da janela vêm em Outros, para ligar à mão.
func (s *Servico) Obter(ctx context.Context, holeriteId int64) (Resposta, error) {
	h, err := s.holerites.Obter(ctx, holeriteId)
	if err != nil {
		return Resposta{}, err
	}
	e, err := s.empregos.Obter(ctx, h.EmpregoId)
	if err != nil {
		return Resposta{}, err
	}
	r := Resposta{Recebimentos: []Recebimento{}, Sugestoes: []Sugestao{}, Outros: []Transacao{}}
	if h.Calculo != nil {
		r.Liquido = &h.Calculo.Liquido
	}
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "TransacaoId", "Valor" FROM "HoleriteRecebimento" WHERE "HoleriteId" = ? ORDER BY "Id"`, holeriteId)
	if err != nil {
		return Resposta{}, fmt.Errorf("ler os recebimentos: %w", err)
	}
	type ligacao struct {
		id, transacao int64
		valor         tipos.Decimal
	}
	var ligacoes []ligacao
	for linhas.Next() {
		var l ligacao
		if err := linhas.Scan(&l.id, &l.transacao, &l.valor); err != nil {
			linhas.Close()
			return Resposta{}, err
		}
		ligacoes = append(ligacoes, l)
	}
	linhas.Close()
	recebido := decimal.Zero
	ligadas := map[int64]bool{}
	for _, l := range ligacoes {
		ts, err := s.lerTransacoes(ctx, holeriteId, `t."Id" = ?`, l.transacao)
		if err != nil {
			return Resposta{}, err
		}
		if len(ts) == 0 {
			continue // a transação virou débito: a ligação fica sem efeito
		}
		r.Recebimentos = append(r.Recebimentos, Recebimento{Id: l.id, Transacao: ts[0], ValorLigado: tipos.DeDecimal(l.valor.Decimal().Round(2))})
		recebido = recebido.Add(l.valor.Decimal())
		ligadas[l.transacao] = true
	}
	r.Recebido = tipos.DeDecimal(recebido.Round(2))
	if r.Liquido != nil {
		d := tipos.DeDecimal(recebido.Sub(r.Liquido.Decimal()).Round(2))
		r.Diferenca = &d
	}

	de := h.Mes.SomarDias(14)
	ate := h.Mes.SomarMeses(1).SomarDias(14)
	candidatas, err := s.lerTransacoes(ctx, holeriteId, `t."Data" >= ? AND t."Data" <= ?`, de, ate)
	if err != nil {
		return Resposta{}, err
	}
	for _, t := range candidatas {
		if ligadas[t.Id] || !t.Livre.Decimal().IsPositive() {
			continue
		}
		pontos, motivos := Pontuar(t, r.Liquido, e.Empregador, e.ContaId)
		if pontos >= 2 {
			r.Sugestoes = append(r.Sugestoes, Sugestao{Transacao: t, Motivos: motivos, Pontos: pontos})
		} else {
			r.Outros = append(r.Outros, t)
		}
	}
	sort.SliceStable(r.Sugestoes, func(i, j int) bool { return r.Sugestoes[i].Pontos > r.Sugestoes[j].Pontos })
	return r, nil
}

// Ligar liga a transação ao holerite. valor nil: a parte livre da transação, até o que falta do líquido. A transação
// sem categoria passa para "Salário", como escolha manual (as regras não mexem mais).
func (s *Servico) Ligar(ctx context.Context, holeriteId, transacaoId int64, valor *tipos.Decimal) (Resposta, error) {
	atual, err := s.Obter(ctx, holeriteId)
	if err != nil {
		return Resposta{}, err
	}
	ts, err := s.lerTransacoes(ctx, holeriteId, `t."Id" = ?`, transacaoId)
	if err != nil {
		return Resposta{}, err
	}
	if len(ts) == 0 {
		return Resposta{}, erroDeValidacao("Escolha um crédito (uma entrada de dinheiro).")
	}
	for _, r := range atual.Recebimentos {
		if r.Id != 0 && r.Transacao.Id == transacaoId {
			return Resposta{}, erroDeValidacao("Este crédito já está ligado ao holerite.")
		}
	}
	livre := ts[0].Livre.Decimal()
	parte := livre
	if valor != nil {
		parte = valor.Decimal()
	} else if atual.Liquido != nil {
		falta := atual.Liquido.Decimal().Sub(atual.Recebido.Decimal())
		if falta.IsPositive() && falta.LessThan(parte) {
			parte = falta
		}
	}
	if !parte.IsPositive() || parte.GreaterThan(livre) {
		return Resposta{}, erroDeValidacao(fmt.Sprintf("A parte ligada vai de R$ 0,01 até o que sobra do crédito (%s).",
			strings.ReplaceAll(livre.StringFixed(2), ".", ",")))
	}
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO "HoleriteRecebimento" ("HoleriteId", "TransacaoId", "Valor", "CriadoEm")
			VALUES (?, ?, ?, ?)`, holeriteId, transacaoId, tipos.DeDecimal(parte.Round(2)), tipos.NovaDataHora(s.relogio.Agora())); err != nil {
			return fmt.Errorf("ligar o crédito: %w", err)
		}
		_, err := tx.ExecContext(ctx, `UPDATE "Transacao" SET "CategoriaId" = (SELECT "Id" FROM "Categoria" WHERE "Nome" = 'Salário'
			AND "Tipo" = 2 ORDER BY "Id" LIMIT 1), "CategoriaManual" = 1 WHERE "Id" = ? AND "CategoriaId" IS NULL
			AND EXISTS (SELECT 1 FROM "Categoria" WHERE "Nome" = 'Salário' AND "Tipo" = 2)`, transacaoId)
		if err != nil {
			return fmt.Errorf("pôr a categoria: %w", err)
		}
		return nil
	})
	if err != nil {
		return Resposta{}, err
	}
	return s.Obter(ctx, holeriteId)
}

// Desligar apaga a ligação (a transação e a categoria dela ficam como estão).
func (s *Servico) Desligar(ctx context.Context, holeriteId, recebimentoId int64) (Resposta, error) {
	r, err := s.db.ExecContext(ctx, `DELETE FROM "HoleriteRecebimento" WHERE "Id" = ? AND "HoleriteId" = ?`, recebimentoId, holeriteId)
	if err != nil {
		return Resposta{}, fmt.Errorf("desligar: %w", err)
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return Resposta{}, holerite.ErrNaoEncontrado
	}
	return s.Obter(ctx, holeriteId)
}

/* ------------------------------------------------------------ HTTP */

// Registrador é onde as rotas entram: o modulos.Rotas do núcleo.
type Registrador interface {
	HandleFunc(padrao string, h http.HandlerFunc)
}

// Registrar põe as rotas do recebimento, em /api/trabalho/.
func Registrar(r Registrador, s *Servico) {
	r.HandleFunc("GET /api/trabalho/holerites/{id}/recebimentos", s.obterHTTP)
	r.HandleFunc("POST /api/trabalho/holerites/{id}/recebimentos", s.ligarHTTP)
	r.HandleFunc("DELETE /api/trabalho/holerites/{id}/recebimentos/{recebimento}", s.desligarHTTP)
}

func idDe(r *http.Request, nome string) int64 {
	id, _ := strconv.ParseInt(r.PathValue(nome), 10, 64) // id que não é número vira 0: não existe (404)
	return id
}

func (s *Servico) responder(w http.ResponseWriter, r *http.Request, corpo Resposta, err error) {
	var v erroDeValidacao
	switch {
	case err == nil:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, corpo)
	case errors.Is(err, holerite.ErrNaoEncontrado), errors.Is(err, emprego.ErrNaoEncontrado):
		respostas.SemCorpo(w, http.StatusNotFound)
	case errors.As(err, &v):
		respostas.Mensagem(w, http.StatusBadRequest, string(v))
	default:
		s.log.ErrorContext(r.Context(), "trabalho: recebimento", "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	}
}

func (s *Servico) obterHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := s.Obter(r.Context(), idDe(r, "id"))
	s.responder(w, r, c, err)
}

func (s *Servico) ligarHTTP(w http.ResponseWriter, r *http.Request) {
	corpo, ok := respostas.LerCorpo(w, r, "dto")
	if !ok {
		return
	}
	var dto struct {
		TransacaoId int64          `json:"transacaoId"`
		Valor       *tipos.Decimal `json:"valor"`
	}
	if err := json.NewDecoder(bytes.NewReader(corpo)).Decode(&dto); err != nil {
		respostas.Mensagem(w, http.StatusBadRequest, "Os dados enviados não estão no formato esperado.")
		return
	}
	c, err := s.Ligar(r.Context(), idDe(r, "id"), dto.TransacaoId, dto.Valor)
	s.responder(w, r, c, err)
}

func (s *Servico) desligarHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := s.Desligar(r.Context(), idDe(r, "id"), idDe(r, "recebimento"))
	s.responder(w, r, c, err)
}
