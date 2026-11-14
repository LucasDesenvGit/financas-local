package fgts

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/decimoterceiro"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/ferias"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Mensagens que a pessoa vê.
const (
	MensagemSemFgts      = "Estágio e PJ não têm FGTS."
	MensagemJSONInvalido = "Os dados enviados não estão no formato esperado."
)

var oitoPct = decimal.RequireFromString("0.08")

// Resposta é o FGTS do emprego: a estimativa e os lançamentos.
type Resposta struct {
	Estimativa
	Ate         tipos.Data   `json:"ate"`
	Lancamentos []Lancamento `json:"lancamentos"`
}

// Servico estima o FGTS e grava os lançamentos.
type Servico struct {
	db        *sql.DB
	empregos  *emprego.Servico
	holerites *holerite.Servico
	decimo    *decimoterceiro.Servico
	ferias    *ferias.Servico
	relogio   relogio.Relogio
	log       *slog.Logger
}

func NovoServico(db *sql.DB, empregos *emprego.Servico, holerites *holerite.Servico, decimo *decimoterceiro.Servico,
	f *ferias.Servico, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, empregos: empregos, holerites: holerites, decimo: decimo, ferias: f, relogio: r, log: log}
}

var errSemFgts = errors.New("sem FGTS")

type erroDeValidacao string

func (e erroDeValidacao) Error() string { return string(e) }

func (s *Servico) emprego(ctx context.Context, id int64) (emprego.Emprego, error) {
	e, err := s.empregos.Obter(ctx, id)
	if err == nil && e.Regime != emprego.RegimeClt {
		err = errSemFgts
	}
	return e, err
}

// depositos são os de cada mês de inicio até ate: 8% da base do INSS do holerite (ou 8% do salário do mês, proporcional,
// sem holerite), mais 8% do 13º em dezembro e 8% das férias (+ 1/3) no mês em que começam.
func (s *Servico) depositos(ctx context.Context, e emprego.Emprego, inicio, ate tipos.Data) (map[string]decimal.Decimal, error) {
	porMes := map[string]decimal.Decimal{}
	resumos, err := s.holerites.Listar(ctx, e.Id)
	if err != nil {
		return nil, err
	}
	doHolerite := map[string]bool{}
	for _, r := range resumos {
		h, err := s.holerites.Obter(ctx, r.Id)
		if err != nil {
			return nil, err
		}
		if h.Calculo != nil {
			porMes[h.Mes.String()] = h.Calculo.FgtsDoMes.Decimal()
			doHolerite[h.Mes.String()] = true
		}
	}
	for mes := inicio; !mes.Depois(ate); mes = mes.SomarMeses(1) {
		if !doHolerite[mes.String()] {
			if salario, ok := emprego.SalarioDoMes(e.Salarios, mes); ok {
				dias := decimal.NewFromInt(int64(holerite.DiasTrabalhados(mes, e.Admissao, e.Saida)))
				porMes[mes.String()] = salario.SalarioBruto.Decimal().Mul(dias).Div(decimal.NewFromInt(30)).Mul(oitoPct)
			}
		}
		// O 13º de dezembro; quem saiu no ano recebe na rescisão. Sem o 13º (sem tabela), fica sem.
		if mes.Mes() == 12 && (e.Saida == nil || e.Saida.Ano() != mes.Ano()) {
			if d, err := s.decimo.Obter(ctx, e.Id, mes.Ano()); err == nil {
				porMes[mes.String()] = porMes[mes.String()].Add(d.Total.Decimal().Mul(oitoPct))
			}
		}
	}
	periodos, err := s.ferias.Listar(ctx, e.Id)
	if err != nil {
		return nil, err
	}
	for _, p := range periodos {
		for _, a := range p.Agendamentos {
			mes := emprego.InicioDoMes(a.Inicio)
			if a.Calculo != nil { // o mês fora do intervalo o Estimar não lê
				base := a.Calculo.Ferias.Decimal().Add(a.Calculo.TercoFerias.Decimal())
				porMes[mes.String()] = porMes[mes.String()].Add(base.Mul(oitoPct))
			}
		}
	}
	return porMes, nil
}

// Obter estima o FGTS do emprego até o mês (o de hoje, ou o da saída).
func (s *Servico) Obter(ctx context.Context, empregoId int64, ate tipos.Data) (Resposta, error) {
	e, err := s.emprego(ctx, empregoId)
	if err != nil {
		return Resposta{}, err
	}
	ate = emprego.InicioDoMes(ate)
	if e.Saida != nil && ate.Depois(emprego.InicioDoMes(*e.Saida)) {
		ate = emprego.InicioDoMes(*e.Saida)
	}
	inicio := emprego.InicioDoMes(e.Admissao)
	lancamentos, err := s.lancamentos(ctx, empregoId)
	if err != nil {
		return Resposta{}, err
	}
	depositos, err := s.depositos(ctx, e, inicio, ate)
	if err != nil {
		return Resposta{}, err
	}
	return Resposta{Estimativa: Estimar(inicio, ate, depositos, lancamentos), Ate: ate, Lancamentos: lancamentos}, nil
}

func (s *Servico) lancamentos(ctx context.Context, empregoId int64) ([]Lancamento, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Data", "Tipo", "Valor", "Motivo" FROM "FgtsLancamento"
		WHERE "EmpregoId" = ? ORDER BY "Data", "Id"`, empregoId)
	if err != nil {
		return nil, fmt.Errorf("ler os lançamentos do FGTS: %w", err)
	}
	defer linhas.Close()
	lista := []Lancamento{}
	for linhas.Next() {
		var l Lancamento
		if err := linhas.Scan(&l.Id, &l.Data, &l.Tipo, &l.Valor, &l.Motivo); err != nil {
			return nil, err
		}
		l.Valor = centavos(l.Valor.Decimal())
		lista = append(lista, l)
	}
	return lista, linhas.Err()
}

// Lancar grava um lançamento: a conferência do saldo (zero vale), um saque (com o motivo) ou a distribuição.
func (s *Servico) Lancar(ctx context.Context, empregoId int64, l Lancamento) (Resposta, error) {
	e, err := s.emprego(ctx, empregoId)
	if err != nil {
		return Resposta{}, err
	}
	hoje := relogio.Hoje(s.relogio)
	v := l.Valor.Decimal()
	switch {
	case l.Tipo < TipoConferencia || l.Tipo > TipoDistribuicao:
		return Resposta{}, erroDeValidacao("Escolha o tipo: saldo conferido, saque ou distribuição de resultados.")
	case l.Data.Antes(e.Admissao) || l.Data.Depois(hoje):
		return Resposta{}, erroDeValidacao("A data vai da admissão até hoje.")
	case v.IsNegative() || (l.Tipo != TipoConferencia && v.IsZero()) || v.GreaterThan(decimal.NewFromInt(100_000_000)):
		return Resposta{}, erroDeValidacao("O valor vai de R$ 0,01 a R$ 100.000.000,00 (o saldo conferido pode ser zero).")
	case l.Tipo == TipoSaque && (l.Motivo == nil || *l.Motivo < MotivoRescisao || *l.Motivo > MotivoOutro):
		return Resposta{}, erroDeValidacao("Escolha o motivo do saque.")
	}
	if l.Tipo != TipoSaque {
		l.Motivo = nil
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO "FgtsLancamento" ("EmpregoId", "Data", "Tipo", "Valor", "Motivo", "CriadoEm")
		VALUES (?, ?, ?, ?, ?, ?)`, empregoId, l.Data, l.Tipo, centavos(v), l.Motivo, tipos.NovaDataHora(s.relogio.Agora()))
	if err != nil {
		return Resposta{}, fmt.Errorf("gravar o lançamento do FGTS: %w", err)
	}
	return s.Obter(ctx, empregoId, hoje)
}

// Excluir apaga um lançamento.
func (s *Servico) Excluir(ctx context.Context, empregoId, id int64) (Resposta, error) {
	if _, err := s.emprego(ctx, empregoId); err != nil {
		return Resposta{}, err
	}
	r, err := s.db.ExecContext(ctx, `DELETE FROM "FgtsLancamento" WHERE "Id" = ? AND "EmpregoId" = ?`, id, empregoId)
	if err != nil {
		return Resposta{}, fmt.Errorf("excluir o lançamento do FGTS: %w", err)
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return Resposta{}, emprego.ErrNaoEncontrado
	}
	return s.Obter(ctx, empregoId, relogio.Hoje(s.relogio))
}

/* ------------------------------------------------------------ HTTP */

// Registrador é onde as rotas entram: o modulos.Rotas do núcleo.
type Registrador interface {
	HandleFunc(padrao string, h http.HandlerFunc)
}

// Registrar põe as rotas do FGTS, em /api/trabalho/.
func Registrar(r Registrador, s *Servico) {
	r.HandleFunc("GET /api/trabalho/empregos/{id}/fgts", s.obterHTTP)
	r.HandleFunc("POST /api/trabalho/empregos/{id}/fgts/lancamentos", s.lancarHTTP)
	r.HandleFunc("DELETE /api/trabalho/empregos/{id}/fgts/lancamentos/{lancamento}", s.excluirHTTP)
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
	case errors.Is(err, emprego.ErrNaoEncontrado):
		respostas.SemCorpo(w, http.StatusNotFound)
	case errors.As(err, &v):
		respostas.Mensagem(w, http.StatusBadRequest, string(v))
	case errors.Is(err, errSemFgts):
		respostas.Mensagem(w, http.StatusBadRequest, MensagemSemFgts)
	default:
		s.log.ErrorContext(r.Context(), "trabalho: FGTS", "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	}
}

func (s *Servico) obterHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := s.Obter(r.Context(), idDe(r, "id"), relogio.Hoje(s.relogio))
	s.responder(w, r, c, err)
}

func (s *Servico) lancarHTTP(w http.ResponseWriter, r *http.Request) {
	corpo, ok := respostas.LerCorpo(w, r, "dto")
	if !ok {
		return
	}
	var dto struct {
		Data   *tipos.Data    `json:"data"`
		Tipo   int64          `json:"tipo"`
		Valor  *tipos.Decimal `json:"valor"`
		Motivo *int64         `json:"motivo"`
	}
	if err := json.NewDecoder(bytes.NewReader(corpo)).Decode(&dto); err != nil {
		respostas.Mensagem(w, http.StatusBadRequest, MensagemJSONInvalido)
		return
	}
	if dto.Data == nil || dto.Valor == nil {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe a data e o valor.")
		return
	}
	c, err := s.Lancar(r.Context(), idDe(r, "id"), Lancamento{Data: *dto.Data, Tipo: dto.Tipo, Valor: *dto.Valor, Motivo: dto.Motivo})
	s.responder(w, r, c, err)
}

func (s *Servico) excluirHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := s.Excluir(r.Context(), idDe(r, "id"), idDe(r, "lancamento"))
	s.responder(w, r, c, err)
}
