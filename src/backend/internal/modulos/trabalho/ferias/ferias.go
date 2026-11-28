package ferias

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/modulos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Mensagens que a pessoa vê.
const (
	MensagemSemFerias = "Estágio e PJ não têm férias pela CLT."
	MensagemSemTabela = "Ainda não há tabela do INSS ou do IRRF para esse mês."
	MensagemPeriodo   = "Escolha um período do emprego."
	MensagemAntes     = "As férias de um período começam depois que ele termina (12 meses de trabalho)."
	MensagemSimular   = "Informe o início (AAAA-MM-DD), os dias (1 a 30) e os vendidos (0 a 10), até 30 dias somados."
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

// Simular calcula uma fração sem gravar (para comparar "tirar 30" com "vender 10"). Com o período, os dias depois do
// concessivo dele saem em dobro, como no agendado (revisão do #701).
func (s *Servico) Simular(ctx context.Context, empregoId int64, periodo *tipos.Data, inicio tipos.Data, dias, vendidos int, adiantar bool) (Calculo, error) {
	e, err := s.emprego(ctx, empregoId)
	if err != nil {
		return Calculo{}, err
	}
	emDobro := 0
	if periodo != nil {
		for _, p := range Periodos(e.Admissao, e.Saida, relogio.Hoje(s.relogio)) {
			if p.Inicio.Igual(*periodo) {
				emDobro = DiasEmDobro(inicio, dias, p.FimConcessivo)
			}
		}
	}
	return s.calcular(ctx, e, inicio, dias, vendidos, adiantar, emDobro)
}

// AdiantamentoDoAno é a 1ª parcela do 13º do ano que veio com as férias (revisão do #701): o pagamento (2 dias antes do
// início) e o valor que as férias pagaram (nil sem a tabela do mês). Os dois nil quando nenhuma férias do ano a traz.
// O 13º pede por aqui, e não lê a tabela das férias.
func (s *Servico) AdiantamentoDoAno(ctx context.Context, empregoId int64, ano int) (*tipos.Data, *tipos.Decimal, error) {
	periodos, err := s.Listar(ctx, empregoId)
	if errors.Is(err, errSemFerias) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	for _, p := range periodos {
		for _, a := range p.Agendamentos {
			if !a.AdiantarDecimo || a.Inicio.Ano() != ano {
				continue
			}
			pagamento := a.Inicio.SomarDias(-2)
			if a.Calculo == nil {
				return &pagamento, nil, nil
			}
			return &pagamento, &a.Calculo.AdiantamentoDecimo, nil
		}
	}
	return nil, nil, nil
}

func (s *Servico) calcular(ctx context.Context, e emprego.Emprego, inicio tipos.Data, dias, vendidos int, adiantar bool, emDobro int) (Calculo, error) {
	salario, ok := emprego.SalarioDoMes(e.Salarios, inicio)
	if !ok {
		return Calculo{}, erroDeValidacao("Não há salário no mês do início.")
	}
	mes := emprego.InicioDoMes(inicio)
	soma, _, err := s.holerites.VariaveisEntre(ctx, e.Id, mes.SomarMeses(-12), mes, salario.SalarioBruto) // Súmula 347
	if err != nil {
		return Calculo{}, err
	}
	tInss, tIrrf, err := encargos.TabelasDoMes(ctx, s.db, mes)
	if err != nil {
		return Calculo{}, err
	}
	return Calcular(Entrada{Inicio: inicio, Dias: dias, DiasVendidos: vendidos, SalarioBase: salario.SalarioBruto,
		SomaVariaveis: soma, Dependentes: e.Dependentes, AdiantarDecimo: adiantar, DiasEmDobro: emDobro, Inss: tInss, Irrf: tIrrf}), nil
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
	if err := errors.Join(linhas.Err(), linhas.Close()); err != nil {
		return nil, err
	}
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
	if err := errors.Join(linhas.Err(), linhas.Close()); err != nil {
		return nil, err
	}

	lista := []PeriodoComFerias{}
	for _, p := range Periodos(e.Admissao, e.Saida, hoje) {
		pf := PeriodoComFerias{Periodo: p, Faltas: faltas[p.Inicio.String()], Agendamentos: []Agendamento{}}
		pf.DiasDireito = DiasDeDireito(pf.Faltas)
		for _, a := range porPeriodo[p.Inicio.String()] {
			c, err := s.calcular(ctx, e, a.Inicio, a.Dias, a.DiasVendidos, a.AdiantarDecimo, DiasEmDobro(a.Inicio, a.Dias, p.FimConcessivo))
			switch {
			case err == nil:
				a.Calculo = &c
			case !errors.Is(err, encargos.ErrSemTabela):
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
	if msg := s.conferirAgenda(periodos, inicio, dias, adiantar); msg != "" {
		return nil, erroDeValidacao(msg)
	}
	e, err := s.emprego(ctx, empregoId)
	if err != nil {
		return nil, err
	}
	if e.Saida != nil && inicio.Depois(*e.Saida) {
		return nil, erroDeValidacao("As férias começam depois da saída do emprego: as que sobraram vêm na rescisão.")
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

// conferirAgenda olha a fração nova contra todas as já agendadas do emprego (revisão do #701): não pode cruzar com
// outra; a 1ª parcela do 13º vem com uma férias só por ano, e só se elas são pagas (2 dias antes do início) até 30/11,
// quando a 1ª vence, e no mesmo ano.
func (s *Servico) conferirAgenda(periodos []PeriodoComFerias, inicio tipos.Data, dias int, adiantar bool) string {
	fim := inicio.SomarDias(dias - 1)
	for _, p := range periodos {
		for _, a := range p.Agendamentos {
			fimA := a.Inicio.SomarDias(a.Dias - 1)
			if !inicio.Depois(fimA) && !a.Inicio.Depois(fim) {
				return fmt.Sprintf("Essas férias cruzam com as de %s a %s: escolha outras datas.", dataBR(a.Inicio), dataBR(fimA))
			}
			if adiantar && a.AdiantarDecimo && a.Inicio.Ano() == inicio.Ano() {
				return fmt.Sprintf("A 1ª parcela do 13º de %d já vem com as férias de %s.", inicio.Ano(), dataBR(a.Inicio))
			}
		}
	}
	pagamento := inicio.SomarDias(-2)
	switch {
	case adiantar && pagamento.Ano() != inicio.Ano():
		return "As férias que começam em 1º ou 2 de janeiro são pagas no ano anterior: a 1ª parcela do 13º não vem com elas."
	case adiantar && pagamento.Depois(tipos.NovaData(inicio.Ano(), time.November, 30)):
		return "A 1ª parcela do 13º vence em 30/11, e estas férias são pagas depois (2 dias antes do início): ela não vem com elas."
	}
	return ""
}

func dataBR(d tipos.Data) string { return fmt.Sprintf("%02d/%02d/%d", d.Dia(), d.Mes(), d.Ano()) }

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
	var periodo *PeriodoComFerias
	for i := range periodos {
		if periodos[i].Inicio.Igual(periodoInicio) {
			periodo = &periodos[i]
		}
	}
	if periodo == nil {
		return nil, erroDeValidacao(MensagemPeriodo)
	}
	// As faltas novas diminuem o direito: ele não pode ficar menor que o já agendado, nem a venda passar de 1/3 dele
	// (revisão do #701).
	direito := DiasDeDireito(faltas)
	if agendados := periodo.DiasGozados + periodo.DiasVendidos; direito < agendados {
		return nil, erroDeValidacao(fmt.Sprintf("Com %d faltas, o direito cai para %d dias, menos que os %d já agendados: desmarque "+
			"férias antes.", faltas, direito, agendados))
	}
	if periodo.DiasVendidos > direito/3 {
		return nil, erroDeValidacao(fmt.Sprintf("Com %d faltas, o direito cai para %d dias, e a venda vai até %d (1/3), menos que os %d "+
			"já vendidos: desmarque férias antes.", faltas, direito, direito/3, periodo.DiasVendidos))
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO "FeriasFaltas" ("EmpregoId", "PeriodoInicio", "Faltas") VALUES (?, ?, ?)
		ON CONFLICT ("EmpregoId", "PeriodoInicio") DO UPDATE SET "Faltas" = excluded."Faltas"`, empregoId, periodoInicio, faltas)
	if err != nil {
		return nil, fmt.Errorf("gravar as faltas: %w", err)
	}
	return s.Listar(ctx, empregoId)
}

/* ------------------------------------------------------------ HTTP */

// Registrar põe as rotas das férias, em /api/trabalho/.
func Registrar(r modulos.Registrador, s *Servico) {
	r.HandleFunc("GET /api/trabalho/empregos/{id}/ferias", s.listarHTTP)
	r.HandleFunc("POST /api/trabalho/empregos/{id}/ferias", s.agendarHTTP)
	r.HandleFunc("DELETE /api/trabalho/empregos/{id}/ferias/{agendamento}", s.desmarcarHTTP)
	r.HandleFunc("PUT /api/trabalho/empregos/{id}/ferias/faltas", s.faltasHTTP)
	r.HandleFunc("GET /api/trabalho/empregos/{id}/ferias/simular", s.simularHTTP)
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
	case errors.Is(err, encargos.ErrSemTabela):
		respostas.Mensagem(w, http.StatusConflict, MensagemSemTabela)
	default:
		s.log.ErrorContext(r.Context(), "trabalho: férias", "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	}
}

func (s *Servico) listarHTTP(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Listar(r.Context(), respostas.IdDe(r, "id"))
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
	if !respostas.LerJSON(w, r, &dto) {
		return
	}
	if dto.PeriodoInicio == nil || dto.Inicio == nil {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe o período e o dia do início.")
		return
	}
	lista, err := s.Agendar(r.Context(), respostas.IdDe(r, "id"), *dto.PeriodoInicio, *dto.Inicio, dto.Dias, dto.DiasVendidos, dto.AdiantarDecimo)
	s.responder(w, r, lista, err)
}

func (s *Servico) desmarcarHTTP(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Desmarcar(r.Context(), respostas.IdDe(r, "id"), respostas.IdDe(r, "agendamento"))
	s.responder(w, r, lista, err)
}

func (s *Servico) faltasHTTP(w http.ResponseWriter, r *http.Request) {
	var dto struct {
		PeriodoInicio *tipos.Data `json:"periodoInicio"`
		Faltas        int         `json:"faltas"`
	}
	if !respostas.LerJSON(w, r, &dto) {
		return
	}
	if dto.PeriodoInicio == nil {
		respostas.Mensagem(w, http.StatusBadRequest, MensagemPeriodo)
		return
	}
	lista, err := s.DefinirFaltas(r.Context(), respostas.IdDe(r, "id"), *dto.PeriodoInicio, dto.Faltas)
	s.responder(w, r, lista, err)
}

// simularHTTP: ?inicio=2026-12-01&dias=20&vendidos=10&adiantar=true.
func (s *Servico) simularHTTP(w http.ResponseWriter, r *http.Request) {
	texto, _ := respostas.ParametroDaQuery(r, "inicio")
	inicio, err := tipos.LerData(texto)
	dias, errDias := strconv.Atoi(r.URL.Query().Get("dias"))
	vendidos, errVendidos := strconv.Atoi(cmp.Or(r.URL.Query().Get("vendidos"), "0"))
	if err != nil || errDias != nil || errVendidos != nil || dias < 1 || vendidos < 0 || vendidos > 10 || dias+vendidos > 30 {
		respostas.Mensagem(w, http.StatusBadRequest, MensagemSimular)
		return
	}
	var periodo *tipos.Data
	if p, err := tipos.LerData(r.URL.Query().Get("periodo")); err == nil {
		periodo = &p
	}
	c, err := s.Simular(r.Context(), respostas.IdDe(r, "id"), periodo, inicio, dias, vendidos, r.URL.Query().Get("adiantar") == "true")
	s.responder(w, r, c, err)
}
