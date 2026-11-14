package ferias

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

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Mensagens que a pessoa vê.
const (
	MensagemSemFerias    = "Estágio e PJ não têm férias pela CLT."
	MensagemSemTabela    = "Ainda não há tabela do INSS ou do IRRF para esse mês."
	MensagemJSONInvalido = "Os dados enviados não estão no formato esperado."
	MensagemPeriodo      = "Escolha um período do emprego."
	MensagemAntes        = "As férias de um período começam depois que ele termina (12 meses de trabalho)."
)

// AvisoVencendo: o concessivo termina em até 60 dias e ainda há dias para tirar.
const diasDoAviso = 60

// Agendamento é uma fração agendada, com o cálculo.
type Agendamento struct {
	Id             int64      `json:"id"`
	Inicio         tipos.Data `json:"inicio"`
	Dias           int        `json:"dias"`
	DiasVendidos   int        `json:"diasVendidos"`
	AdiantarDecimo bool       `json:"adiantarDecimo"`
	Calculo        *Calculo   `json:"calculo"` // nil sem a tabela do mês
}

// PeriodoComFerias é o período com as faltas, o direito, o que já foi agendado e o que sobra.
type PeriodoComFerias struct {
	Periodo
	Faltas       int           `json:"faltas"`
	DiasDireito  int           `json:"diasDireito"`
	DiasGozados  int           `json:"diasAgendados"`
	DiasVendidos int           `json:"diasVendidos"`
	Saldo        int           `json:"saldo"`
	Vencida      bool          `json:"vencida"`  // o concessivo acabou com saldo: a empresa paga em dobro (CLT, art. 137)
	Vencendo     bool          `json:"vencendo"` // o concessivo acaba em até 60 dias, com saldo
	Agendamentos []Agendamento `json:"agendamentos"`
}

// Servico lê, agenda e calcula as férias.
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

var (
	errSemFerias = errors.New("sem férias")
	errSemTabela = errors.New("sem tabela")
)

type erroDeValidacao string

func (e erroDeValidacao) Error() string { return string(e) }

func (s *Servico) emprego(ctx context.Context, id int64) (emprego.Emprego, error) {
	e, err := s.empregos.Obter(ctx, id)
	if err != nil {
		return e, err
	}
	if e.Regime != emprego.RegimeClt {
		return e, errSemFerias
	}
	return e, nil
}

// Simular calcula uma fração sem gravar (para comparar "tirar 30" com "vender 10").
func (s *Servico) Simular(ctx context.Context, empregoId int64, inicio tipos.Data, dias, vendidos int, adiantar bool) (Calculo, error) {
	e, err := s.emprego(ctx, empregoId)
	if err != nil {
		return Calculo{}, err
	}
	return s.calcular(ctx, e, inicio, dias, vendidos, adiantar)
}

func (s *Servico) calcular(ctx context.Context, e emprego.Emprego, inicio tipos.Data, dias, vendidos int, adiantar bool) (Calculo, error) {
	salario, ok := emprego.SalarioDoMes(e.Salarios, inicio)
	if !ok {
		return Calculo{}, erroDeValidacao("Não há salário no mês do início.")
	}
	mes := emprego.InicioDoMes(inicio)
	soma, _, err := s.holerites.VariaveisEntre(ctx, e.Id, mes.SomarMeses(-12), mes)
	if err != nil {
		return Calculo{}, err
	}
	inss, err := encargos.LerTabelasInss(ctx, s.db)
	if err != nil {
		return Calculo{}, err
	}
	irrf, err := encargos.LerTabelasIrrf(ctx, s.db)
	if err != nil {
		return Calculo{}, err
	}
	tInss, okInss := encargos.Vigente(inss, mes)
	tIrrf, okIrrf := encargos.Vigente(irrf, mes)
	if !okInss || !okIrrf {
		return Calculo{}, errSemTabela
	}
	return Calcular(Entrada{Inicio: inicio, Dias: dias, DiasVendidos: vendidos, SalarioBase: salario.SalarioBruto,
		SomaVariaveis: soma, Dependentes: e.Dependentes, AdiantarDecimo: adiantar, Inss: tInss, Irrf: tIrrf}), nil
}

// Listar devolve os períodos do emprego com as férias agendadas e o que sobra.
func (s *Servico) Listar(ctx context.Context, empregoId int64) ([]PeriodoComFerias, error) {
	e, err := s.emprego(ctx, empregoId)
	if err != nil {
		return nil, err
	}
	hoje := relogio.Hoje(s.relogio)
	faltas := map[string]int{}
	linhas, err := s.db.QueryContext(ctx, `SELECT "PeriodoInicio", "Faltas" FROM "FeriasFaltas" WHERE "EmpregoId" = ?`, empregoId)
	if err != nil {
		return nil, fmt.Errorf("ler as faltas: %w", err)
	}
	for linhas.Next() {
		var inicio tipos.Data
		var n int
		if err := linhas.Scan(&inicio, &n); err != nil {
			linhas.Close()
			return nil, err
		}
		faltas[inicio.String()] = n
	}
	linhas.Close()
	porPeriodo := map[string][]Agendamento{}
	linhas, err = s.db.QueryContext(ctx, `SELECT "Id", "PeriodoInicio", "Inicio", "Dias", "DiasVendidos", "AdiantarDecimo"
		FROM "FeriasAgendamento" WHERE "EmpregoId" = ? ORDER BY "Inicio"`, empregoId)
	if err != nil {
		return nil, fmt.Errorf("ler as férias: %w", err)
	}
	for linhas.Next() {
		var a Agendamento
		var periodo tipos.Data
		if err := linhas.Scan(&a.Id, &periodo, &a.Inicio, &a.Dias, &a.DiasVendidos, &a.AdiantarDecimo); err != nil {
			linhas.Close()
			return nil, err
		}
		porPeriodo[periodo.String()] = append(porPeriodo[periodo.String()], a)
	}
	linhas.Close()

	lista := []PeriodoComFerias{}
	for _, p := range Periodos(e.Admissao, e.Saida, hoje) {
		pf := PeriodoComFerias{Periodo: p, Faltas: faltas[p.Inicio.String()], Agendamentos: []Agendamento{}}
		pf.DiasDireito = DiasDeDireito(pf.Faltas)
		for _, a := range porPeriodo[p.Inicio.String()] {
			c, err := s.calcular(ctx, e, a.Inicio, a.Dias, a.DiasVendidos, a.AdiantarDecimo)
			switch {
			case err == nil:
				a.Calculo = &c
			case !errors.Is(err, errSemTabela):
				return nil, err
			}
			pf.DiasGozados += a.Dias
			pf.DiasVendidos += a.DiasVendidos
			pf.Agendamentos = append(pf.Agendamentos, a)
		}
		pf.Saldo = max(0, pf.DiasDireito-pf.DiasGozados-pf.DiasVendidos)
		if p.Completo && pf.Saldo > 0 {
			pf.Vencida = p.FimConcessivo.Antes(hoje)
			pf.Vencendo = !pf.Vencida && !p.FimConcessivo.Depois(hoje.SomarDias(diasDoAviso))
		}
		lista = append(lista, pf)
	}
	return lista, nil
}

// Agendar grava uma fração das férias de um período (o do PeriodoInicio), conferindo as regras da CLT.
func (s *Servico) Agendar(ctx context.Context, empregoId int64, periodoInicio, inicio tipos.Data, dias, vendidos int, adiantar bool) ([]PeriodoComFerias, error) {
	periodos, err := s.Listar(ctx, empregoId)
	if err != nil {
		return nil, err
	}
	var periodo *PeriodoComFerias
	for i := range periodos {
		if periodos[i].Inicio.Igual(periodoInicio) {
			periodo = &periodos[i]
		}
	}
	if periodo == nil {
		return nil, erroDeValidacao(MensagemPeriodo)
	}
	if !inicio.Depois(periodo.Fim) {
		return nil, erroDeValidacao(MensagemAntes)
	}
	existentes := make([]int, 0, len(periodo.Agendamentos))
	for _, a := range periodo.Agendamentos {
		existentes = append(existentes, a.Dias)
	}
	if msg := ValidarFracoes(existentes, periodo.DiasVendidos, periodo.DiasDireito, dias, vendidos); msg != "" {
		return nil, erroDeValidacao(msg)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO "FeriasAgendamento" ("EmpregoId", "PeriodoInicio", "Inicio", "Dias", "DiasVendidos",
		"AdiantarDecimo", "CriadoEm") VALUES (?, ?, ?, ?, ?, ?, ?)`, empregoId, periodoInicio, inicio, dias, vendidos, adiantar,
		tipos.NovaDataHora(s.relogio.Agora()))
	if err != nil {
		return nil, fmt.Errorf("agendar as férias: %w", err)
	}
	return s.Listar(ctx, empregoId)
}

// Desmarcar apaga uma fração agendada.
func (s *Servico) Desmarcar(ctx context.Context, empregoId, id int64) ([]PeriodoComFerias, error) {
	r, err := s.db.ExecContext(ctx, `DELETE FROM "FeriasAgendamento" WHERE "Id" = ? AND "EmpregoId" = ?`, id, empregoId)
	if err != nil {
		return nil, fmt.Errorf("desmarcar as férias: %w", err)
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return nil, emprego.ErrNaoEncontrado
	}
	return s.Listar(ctx, empregoId)
}

// DefinirFaltas grava as faltas sem justificativa de um período (0 apaga).
func (s *Servico) DefinirFaltas(ctx context.Context, empregoId int64, periodoInicio tipos.Data, faltas int) ([]PeriodoComFerias, error) {
	if faltas < 0 || faltas > 366 {
		return nil, erroDeValidacao("As faltas vão de 0 a 366.")
	}
	periodos, err := s.Listar(ctx, empregoId)
	if err != nil {
		return nil, err
	}
	achou := false
	for _, p := range periodos {
		achou = achou || p.Inicio.Igual(periodoInicio)
	}
	if !achou {
		return nil, erroDeValidacao(MensagemPeriodo)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO "FeriasFaltas" ("EmpregoId", "PeriodoInicio", "Faltas") VALUES (?, ?, ?)
		ON CONFLICT ("EmpregoId", "PeriodoInicio") DO UPDATE SET "Faltas" = excluded."Faltas"`, empregoId, periodoInicio, faltas)
	if err != nil {
		return nil, fmt.Errorf("gravar as faltas: %w", err)
	}
	return s.Listar(ctx, empregoId)
}

/* ------------------------------------------------------------ HTTP */

// Registrador é onde as rotas entram: o modulos.Rotas do núcleo.
type Registrador interface {
	HandleFunc(padrao string, h http.HandlerFunc)
}

// Registrar põe as rotas das férias, em /api/trabalho/.
func Registrar(r Registrador, s *Servico) {
	r.HandleFunc("GET /api/trabalho/empregos/{id}/ferias", s.listarHTTP)
	r.HandleFunc("POST /api/trabalho/empregos/{id}/ferias", s.agendarHTTP)
	r.HandleFunc("DELETE /api/trabalho/empregos/{id}/ferias/{agendamento}", s.desmarcarHTTP)
	r.HandleFunc("PUT /api/trabalho/empregos/{id}/ferias/faltas", s.faltasHTTP)
	r.HandleFunc("GET /api/trabalho/empregos/{id}/ferias/simular", s.simularHTTP)
}

func idDe(r *http.Request, nome string) int64 {
	id, _ := strconv.ParseInt(r.PathValue(nome), 10, 64) // id que não é número vira 0: não existe (404)
	return id
}

func lerJSON(w http.ResponseWriter, r *http.Request, destino any) bool {
	corpo, ok := respostas.LerCorpo(w, r, "dto")
	if !ok {
		return false
	}
	if err := json.NewDecoder(bytes.NewReader(corpo)).Decode(destino); err != nil {
		respostas.Mensagem(w, http.StatusBadRequest, MensagemJSONInvalido)
		return false
	}
	return true
}

func (s *Servico) responder(w http.ResponseWriter, r *http.Request, corpo any, err error) {
	var v erroDeValidacao
	switch {
	case err == nil:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, corpo)
	case errors.Is(err, emprego.ErrNaoEncontrado):
		respostas.SemCorpo(w, http.StatusNotFound)
	case errors.As(err, &v):
		respostas.Mensagem(w, http.StatusBadRequest, string(v))
	case errors.Is(err, errSemFerias):
		respostas.Mensagem(w, http.StatusBadRequest, MensagemSemFerias)
	case errors.Is(err, errSemTabela):
		respostas.Mensagem(w, http.StatusConflict, MensagemSemTabela)
	default:
		s.log.ErrorContext(r.Context(), "trabalho: férias", "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	}
}

func (s *Servico) listarHTTP(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Listar(r.Context(), idDe(r, "id"))
	s.responder(w, r, lista, err)
}

type agendarDto struct {
	PeriodoInicio  *tipos.Data `json:"periodoInicio"`
	Inicio         *tipos.Data `json:"inicio"`
	Dias           int         `json:"dias"`
	DiasVendidos   int         `json:"diasVendidos"`
	AdiantarDecimo bool        `json:"adiantarDecimo"`
}

func (s *Servico) agendarHTTP(w http.ResponseWriter, r *http.Request) {
	var dto agendarDto
	if !lerJSON(w, r, &dto) {
		return
	}
	if dto.PeriodoInicio == nil || dto.Inicio == nil {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe o período e o dia do início.")
		return
	}
	lista, err := s.Agendar(r.Context(), idDe(r, "id"), *dto.PeriodoInicio, *dto.Inicio, dto.Dias, dto.DiasVendidos, dto.AdiantarDecimo)
	s.responder(w, r, lista, err)
}

func (s *Servico) desmarcarHTTP(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Desmarcar(r.Context(), idDe(r, "id"), idDe(r, "agendamento"))
	s.responder(w, r, lista, err)
}

func (s *Servico) faltasHTTP(w http.ResponseWriter, r *http.Request) {
	var dto struct {
		PeriodoInicio *tipos.Data `json:"periodoInicio"`
		Faltas        int         `json:"faltas"`
	}
	if !lerJSON(w, r, &dto) {
		return
	}
	if dto.PeriodoInicio == nil {
		respostas.Mensagem(w, http.StatusBadRequest, MensagemPeriodo)
		return
	}
	lista, err := s.DefinirFaltas(r.Context(), idDe(r, "id"), *dto.PeriodoInicio, dto.Faltas)
	s.responder(w, r, lista, err)
}

// simularHTTP: ?inicio=2026-12-01&dias=20&vendidos=10&adiantar=true.
func (s *Servico) simularHTTP(w http.ResponseWriter, r *http.Request) {
	texto, _ := respostas.ParametroDaQuery(r, "inicio")
	inicio, err := tipos.LerData(texto)
	dias, errDias := strconv.Atoi(r.URL.Query().Get("dias"))
	vendidos, _ := strconv.Atoi(r.URL.Query().Get("vendidos"))
	if err != nil || errDias != nil || dias < 1 || dias > 30 || vendidos < 0 || vendidos > 10 {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe o início (AAAA-MM-DD), os dias (1 a 30) e os vendidos (0 a 10).")
		return
	}
	c, err := s.Simular(r.Context(), idDe(r, "id"), inicio, dias, vendidos, r.URL.Query().Get("adiantar") == "true")
	s.responder(w, r, c, err)
}
