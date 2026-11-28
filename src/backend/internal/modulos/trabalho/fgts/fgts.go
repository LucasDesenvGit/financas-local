package fgts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/decimoterceiro"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/ferias"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/modulos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Mensagens que a pessoa vê.
const (
	MensagemSemFgts = "Estágio e PJ não têm FGTS."
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

// depositos são os de cada mês (a competência) de inicio até ate: 8% da base do INSS do holerite (ou 8% do salário do
// mês, proporcional, sem holerite); 8% da 1ª parcela do 13º no mês em que ela foi paga e 8% do resto em dezembro; e 8%
// das férias (+ 1/3) no mês em que começam, menos o salário dos dias de férias no mês de cada um (as férias pagam esses
// dias; revisão do #702). O holerite lido do PDF (#228) já vem com o FGTS dos dias trabalhados: nele não se tira nada.
func (s *Servico) depositos(ctx context.Context, e emprego.Emprego, inicio, ate tipos.Data) (map[string]decimal.Decimal, error) {
	porMes := map[string]decimal.Decimal{}
	resumos, err := s.holerites.Listar(ctx, e.Id)
	if err != nil {
		return nil, err
	}
	doHolerite, lido := map[string]bool{}, map[string]bool{}
	for _, r := range resumos {
		h, err := s.holerites.Obter(ctx, r.Id)
		if err != nil {
			return nil, err
		}
		if h.Calculo != nil {
			porMes[h.Mes.String()] = h.Calculo.FgtsDoMes.Decimal()
			doHolerite[h.Mes.String()] = true
			lido[h.Mes.String()] = h.Origem == holerite.LidoDoPdf
		}
	}
	for mes := inicio; !mes.Depois(ate); mes = mes.SomarMeses(1) {
		if !doHolerite[mes.String()] {
			if salario, ok := emprego.SalarioDoMes(e.Salarios, mes); ok {
				dias := decimal.NewFromInt(int64(holerite.DiasTrabalhados(mes, e.Admissao, e.Saida)))
				porMes[mes.String()] = salario.SalarioBruto.Decimal().Mul(dias).Div(decimal.NewFromInt(30)).Mul(oitoPct)
			}
		}
	}
	// O 13º de cada ano: a 1ª parcela no mês em que foi paga, o resto em dezembro (cada parte só se o mês está no
	// intervalo); quem saiu no ano recebe na rescisão. Sem o 13º (sem tabela, fora do ano), fica sem; outra falha volta
	// (revisão do #702).
	ultimoAno := ate.Ano()
	if e.Saida != nil {
		ultimoAno = min(ultimoAno, e.Saida.Ano()-1)
	}
	for ano := inicio.Ano(); ano <= ultimoAno; ano++ {
		d, err := s.decimo.Obter(ctx, e.Id, ano)
		if decimoterceiro.SemDecimo(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		// O mês fora do intervalo fica no mapa, mas o Estimar só lê os do intervalo.
		primeira := d.Primeira.Valor.Decimal()
		mesDaPrimeira, dezembro := emprego.InicioDoMes(d.Primeira.Data).String(), tipos.NovaData(ano, 12, 1).String()
		porMes[mesDaPrimeira] = porMes[mesDaPrimeira].Add(primeira.Mul(oitoPct))
		porMes[dezembro] = porMes[dezembro].Add(d.Total.Decimal().Sub(primeira).Mul(oitoPct))
	}
	periodos, err := s.ferias.Listar(ctx, e.Id)
	if err != nil {
		return nil, err
	}
	for _, p := range periodos {
		for _, a := range p.Agendamentos {
			if a.Calculo == nil { // sem tabela: a fração fica sem o depósito, como o holerite sem cálculo
				continue
			}
			mes := emprego.InicioDoMes(a.Inicio)
			// A base é a do INSS das férias: férias + 1/3, sem a dobra do art. 137 nem o 1/3 dela. A Lei 8.036 (art. 15,
			// § 6º) tira do FGTS o que a Lei 8.212 (art. 28, § 9º, "d") tira do INSS, e a dobra está lá (#751).
			base := a.Calculo.Inss.Base.Decimal()
			porMes[mes.String()] = porMes[mes.String()].Add(base.Mul(oitoPct))
			for m, dias := range diasDeFeriasPorMes(a.Inicio, a.Dias) {
				if salario, ok := emprego.SalarioDoMes(e.Salarios, m); ok && !lido[m.String()] {
					tirar := salario.SalarioBruto.Decimal().Mul(decimal.NewFromInt(int64(min(dias, 30)))).Div(decimal.NewFromInt(30)).Mul(oitoPct)
					porMes[m.String()] = decimal.Max(decimal.Zero, porMes[m.String()].Sub(tirar))
				}
			}
		}
	}
	return porMes, nil
}

// diasDeFeriasPorMes são quantos dias de férias caem em cada mês (o dia 1 do mês), do início até o último dia.
func diasDeFeriasPorMes(inicio tipos.Data, dias int) map[tipos.Data]int {
	porMes := map[tipos.Data]int{}
	for i := range dias {
		porMes[emprego.InicioDoMes(inicio.SomarDias(i))]++
	}
	return porMes
}

// Obter estima o FGTS do emprego até o mês. Depois da saída não há depósito, mas o saldo segue com a correção, e os
// saques e as conferências de depois dela contam (o saque da rescisão, revisão do #702).
func (s *Servico) Obter(ctx context.Context, empregoId int64, ate tipos.Data) (Resposta, error) {
	e, err := s.emprego(ctx, empregoId)
	if err != nil {
		return Resposta{}, err
	}
	ate = emprego.InicioDoMes(ate)
	ateDepositos := ate
	if e.Saida != nil && ate.Depois(emprego.InicioDoMes(*e.Saida)) {
		ateDepositos = emprego.InicioDoMes(*e.Saida)
	}
	inicio := emprego.InicioDoMes(e.Admissao)
	lancamentos, err := s.lancamentos(ctx, empregoId)
	if err != nil {
		return Resposta{}, err
	}
	depositos, err := s.depositos(ctx, e, inicio, ateDepositos)
	if err != nil {
		return Resposta{}, err
	}
	return Resposta{Estimativa: Estimar(inicio, ate, depositos, lancamentos), Ate: ate, Lancamentos: lancamentos}, nil
}

// NaSaida é o FGTS para a rescisão (#215, revisão do #704).
type NaSaida struct {
	// Saldo é o estimado até o mês antes da saída, como se o emprego terminasse nela: o mês da saída e o 13º do ano
	// vêm no depósito da rescisão, e não contam aqui (antes, contavam duas vezes).
	Saldo tipos.Decimal
	// BaseDaMulta são os depósitos corrigidos, sem descontar os saques (Lei 8.036, art. 18, § 1º): o saldo mais os
	// saques (*inferido*: sem a correção deles).
	BaseDaMulta tipos.Decimal
	// SaqueAniversario: houve saque-aniversário; na dispensa e no acordo, só a multa sai (Lei 13.932/2019).
	SaqueAniversario bool
}

// ParaRescisao é o FGTS na saída em saida.
func (s *Servico) ParaRescisao(ctx context.Context, empregoId int64, saida tipos.Data) (NaSaida, error) {
	e, err := s.emprego(ctx, empregoId)
	if err != nil {
		return NaSaida{}, err
	}
	e.Saida = &saida
	inicio, ate := emprego.InicioDoMes(e.Admissao), emprego.InicioDoMes(saida).SomarMeses(-1)
	lancamentos, err := s.lancamentos(ctx, empregoId)
	if err != nil {
		return NaSaida{}, err
	}
	depositos, err := s.depositos(ctx, e, inicio, ate)
	if err != nil {
		return NaSaida{}, err
	}
	saldo := Estimar(inicio, ate, depositos, lancamentos).Saldo.Decimal()
	mesDaSaida := emprego.InicioDoMes(saida)
	// A conferência do app no mês da saída, até ela, vale mais que a estimativa (#751): antes, o Estimar ia só até o
	// mês anterior e a ignorava. O saldo passa a ser o do app, mais o depósito do mês anterior se ele ainda não aparecia
	// (o do mês da saída vem no depósito da rescisão); os saques até ela já saíram do saldo do app.
	var conferencia *Lancamento
	for i, l := range lancamentos {
		if l.Tipo == TipoConferencia && !l.Data.Antes(mesDaSaida) && !l.Data.Depois(saida) {
			conferencia = &lancamentos[i] // a última (a lista vem por data)
		}
	}
	if conferencia != nil {
		saldo = conferencia.Valor.Decimal()
		if conferencia.Data.Dia() < DiaDoDeposito(ate) && !ate.Antes(inicio) {
			saldo = saldo.Add(depositos[ate.String()].Round(2))
		}
	}
	base := saldo
	r := NaSaida{}
	for _, l := range lancamentos {
		if l.Tipo != TipoSaque || l.Data.Depois(saida) {
			continue
		}
		r.SaqueAniversario = r.SaqueAniversario || (l.Motivo != nil && *l.Motivo == MotivoSaqueAniversario)
		jaSaiu := l.Data.Antes(mesDaSaida) // o Estimar, até o mês antes, já tirou
		if conferencia != nil {
			jaSaiu = !l.Data.Depois(conferencia.Data) // o app já tinha tirado
		}
		if jaSaiu {
			base = base.Add(l.Valor.Decimal()) // já saiu do saldo: volta para a base da multa
		} else {
			saldo = saldo.Sub(l.Valor.Decimal()) // depois: ainda não saiu (revisão do #704)
		}
	}
	r.Saldo, r.BaseDaMulta = tipos.Centavos(decimal.Max(saldo, decimal.Zero)), tipos.Centavos(base)
	return r, nil
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
		l.Valor = tipos.Centavos(l.Valor.Decimal())
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
		VALUES (?, ?, ?, ?, ?, ?)`, empregoId, l.Data, l.Tipo, tipos.Centavos(v), l.Motivo, tipos.NovaDataHora(s.relogio.Agora()))
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

// Registrar põe as rotas do FGTS, em /api/trabalho/.
func Registrar(r modulos.Registrador, s *Servico) {
	r.HandleFunc("GET /api/trabalho/empregos/{id}/fgts", s.obterHTTP)
	r.HandleFunc("POST /api/trabalho/empregos/{id}/fgts/lancamentos", s.lancarHTTP)
	r.HandleFunc("DELETE /api/trabalho/empregos/{id}/fgts/lancamentos/{lancamento}", s.excluirHTTP)
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
	c, err := s.Obter(r.Context(), respostas.IdDe(r, "id"), relogio.Hoje(s.relogio))
	s.responder(w, r, c, err)
}

func (s *Servico) lancarHTTP(w http.ResponseWriter, r *http.Request) {
	var dto struct {
		Data   *tipos.Data    `json:"data"`
		Tipo   int64          `json:"tipo"`
		Valor  *tipos.Decimal `json:"valor"`
		Motivo *int64         `json:"motivo"`
	}
	if !respostas.LerJSON(w, r, &dto) {
		return
	}
	if dto.Data == nil || dto.Valor == nil {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe a data e o valor.")
		return
	}
	c, err := s.Lancar(r.Context(), respostas.IdDe(r, "id"), Lancamento{Data: *dto.Data, Tipo: dto.Tipo, Valor: *dto.Valor, Motivo: dto.Motivo})
	s.responder(w, r, c, err)
}

func (s *Servico) excluirHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := s.Excluir(r.Context(), respostas.IdDe(r, "id"), respostas.IdDe(r, "lancamento"))
	s.responder(w, r, c, err)
}
