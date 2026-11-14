package rescisao

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
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/ferias"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/fgts"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Mensagens que a pessoa vê.
const (
	MensagemSemRescisao  = "Estágio e PJ não têm as verbas da rescisão da CLT."
	MensagemSemTabela    = "Ainda não há tabela do INSS ou do IRRF para o mês da saída."
	MensagemJSONInvalido = "Os dados enviados não estão no formato esperado."
	MensagemTipo         = "Escolha o tipo: sem justa causa, pedido de demissão, acordo, justa causa ou fim da experiência."
	MensagemAviso        = "Escolha o aviso: trabalhado ou indenizado."
	MensagemSaida        = "A saída vai da admissão em diante, com o salário do mês cadastrado."
	MensagemJaRegistrada = "A rescisão deste emprego já está registrada: desfaça antes de registrar outra."
)

// Servico simula e registra a rescisão.
type Servico struct {
	db        *sql.DB
	empregos  *emprego.Servico
	holerites *holerite.Servico
	decimo    *decimoterceiro.Servico
	ferias    *ferias.Servico
	fgts      *fgts.Servico
	relogio   relogio.Relogio
	log       *slog.Logger
}

func NovoServico(db *sql.DB, empregos *emprego.Servico, holerites *holerite.Servico, decimo *decimoterceiro.Servico,
	f *ferias.Servico, g *fgts.Servico, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, empregos: empregos, holerites: holerites, decimo: decimo, ferias: f, fgts: g, relogio: r, log: log}
}

var (
	errSemRescisao = errors.New("sem rescisão")
	errSemTabela   = errors.New("sem tabela")
	errRegistrada  = errors.New("já registrada")
)

type erroDeValidacao string

func (e erroDeValidacao) Error() string { return string(e) }

// avisoDoTipo: justa causa e fim de experiência não têm aviso.
func avisoDoTipo(tipo, aviso int64) int64 {
	if tipo == JustaCausa || tipo == FimExperiencia {
		return SemAviso
	}
	return aviso
}

// Simular calcula a rescisão do emprego com o tipo, o aviso e a data de saída.
func (s *Servico) Simular(ctx context.Context, empregoId, tipo, aviso int64, saida tipos.Data) (Calculo, error) {
	if !TipoConhecido(tipo) {
		return Calculo{}, erroDeValidacao(MensagemTipo)
	}
	aviso = avisoDoTipo(tipo, aviso)
	if aviso < SemAviso || aviso > AvisoIndenizado || (aviso == SemAviso && tipo != JustaCausa && tipo != FimExperiencia) {
		return Calculo{}, erroDeValidacao(MensagemAviso)
	}
	e, err := s.empregos.Obter(ctx, empregoId)
	if err != nil {
		return Calculo{}, err
	}
	if e.Regime != emprego.RegimeClt {
		return Calculo{}, errSemRescisao
	}
	salario, ok := emprego.SalarioDoMes(e.Salarios, saida)
	if saida.Antes(e.Admissao) || !ok {
		return Calculo{}, erroDeValidacao(MensagemSaida)
	}
	mes := emprego.InicioDoMes(saida)
	entrada := Entrada{Tipo: tipo, Aviso: aviso, Admissao: e.Admissao, Saida: saida, SalarioMes: salario.SalarioBruto,
		Dependentes: e.Dependentes, PrimeiraParcela13: tipos.DeDecimal(decimal.Zero)}
	if entrada.SomaVariaveis, _, err = s.holerites.VariaveisEntre(ctx, empregoId, mes.SomarMeses(-12), mes); err != nil {
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
	var okInss, okIrrf bool
	entrada.Inss, okInss = encargos.Vigente(inss, mes)
	entrada.Irrf, okIrrf = encargos.Vigente(irrf, mes)
	if !okInss || !okIrrf {
		return Calculo{}, errSemTabela
	}

	_, projetada := AvisoPago(tipo, aviso, e.Admissao, saida)
	entrada.Avos13 = decimoterceiro.Avos(saida.Ano(), e.Admissao, &projetada)
	if err := s.feriasNaSaida(ctx, &entrada, e, saida, projetada); err != nil {
		return Calculo{}, err
	}
	// A 1ª parcela do 13º que já veio: a informada (ou a de 30/11, saindo em dezembro), se veio até a saída.
	if d, err := s.decimo.Obter(ctx, empregoId, saida.Ano()); err == nil {
		if !d.Primeira.Data.Depois(saida) {
			entrada.PrimeiraParcela13 = d.Primeira.Valor
		}
	}
	g, err := s.fgts.Obter(ctx, empregoId, saida)
	if err != nil {
		return Calculo{}, err
	}
	entrada.SaldoFgts = g.Saldo
	return Calcular(entrada), nil
}

// feriasNaSaida acha as férias vencidas (os períodos completos até a data projetada, com os dias que não foram
// tirados) e os avos do período em curso.
func (s *Servico) feriasNaSaida(ctx context.Context, entrada *Entrada, e emprego.Emprego, saida, projetada tipos.Data) error {
	feitas, err := s.ferias.Listar(ctx, e.Id)
	if err != nil {
		return err
	}
	porInicio := map[string]ferias.PeriodoComFerias{}
	for _, p := range feitas {
		porInicio[p.Inicio.String()] = p
	}
	for _, p := range ferias.Periodos(e.Admissao, nil, projetada) {
		if p.Completo {
			// Os dias que sobram (zero ou menos não viram verba: o Calcular só lança valor positivo).
			feito := porInicio[p.Inicio.String()]
			dias := ferias.DiasDeDireito(feito.Faltas) - feito.DiasGozados - feito.DiasVendidos
			entrada.Vencidas = append(entrada.Vencidas, FeriasVencidas{Inicio: p.Inicio, Dias: dias, EmDobro: p.FimConcessivo.Antes(saida)})
		} else {
			entrada.AvosFerias = AvosFerias(p.Inicio, projetada)
		}
	}
	return nil
}

// Comparacao é a rescisão de cada tipo na mesma data, lado a lado.
type Comparacao struct {
	Tipo    int64         `json:"tipo"`
	Nome    string        `json:"nome"`
	Aviso   int64         `json:"aviso"`
	Liquido tipos.Decimal `json:"liquido"`
	Multa   tipos.Decimal `json:"multa"`
	Sacavel tipos.Decimal `json:"sacavel"`
	Seguro  bool          `json:"seguroDesemprego"`
}

// Comparar simula os cinco tipos na mesma data: a dispensa e o acordo com o aviso indenizado; o pedido de demissão
// com o aviso trabalhado.
func (s *Servico) Comparar(ctx context.Context, empregoId int64, saida tipos.Data) ([]Comparacao, error) {
	avisos := map[int64]int64{SemJustaCausa: AvisoIndenizado, PedidoDemissao: AvisoTrabalhado, Acordo: AvisoIndenizado}
	lista := []Comparacao{}
	for tipo := SemJustaCausa; tipo <= FimExperiencia; tipo++ {
		c, err := s.Simular(ctx, empregoId, tipo, avisos[tipo], saida)
		if err != nil {
			return nil, err
		}
		lista = append(lista, Comparacao{Tipo: tipo, Nome: c.NomeDoTipo, Aviso: c.Aviso, Liquido: c.Liquido, Multa: c.Fgts.Multa,
			Sacavel: c.Fgts.Sacavel, Seguro: c.SeguroDesemprego})
	}
	return lista, nil
}

// Registrar guarda a rescisão real (o cálculo de agora, congelado) e põe a data de saída no emprego.
func (s *Servico) Registrar(ctx context.Context, empregoId, tipo, aviso int64, saida tipos.Data) (Calculo, error) {
	if _, err := s.Registrada(ctx, empregoId); err == nil {
		return Calculo{}, errRegistrada
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Calculo{}, err
	}
	c, err := s.Simular(ctx, empregoId, tipo, aviso, saida)
	if err != nil {
		return Calculo{}, err
	}
	dados, err := json.Marshal(c)
	if err != nil {
		return Calculo{}, err
	}
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE "Emprego" SET "Saida" = ? WHERE "Id" = ?`, saida, empregoId); err != nil {
			return fmt.Errorf("fechar o emprego: %w", err)
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO "Rescisao" ("EmpregoId", "Data", "Tipo", "Aviso", "Calculo", "CriadoEm")
			VALUES (?, ?, ?, ?, ?, ?)`, empregoId, saida, tipo, c.Aviso, string(dados), tipos.NovaDataHora(s.relogio.Agora()))
		if err != nil {
			return fmt.Errorf("guardar a rescisão: %w", err)
		}
		return nil
	})
	if err != nil {
		return Calculo{}, err
	}
	return c, nil
}

// Registrada devolve a rescisão registrada do emprego (sql.ErrNoRows se não há).
func (s *Servico) Registrada(ctx context.Context, empregoId int64) (Calculo, error) {
	var dados string
	if err := s.db.QueryRowContext(ctx, `SELECT "Calculo" FROM "Rescisao" WHERE "EmpregoId" = ?`, empregoId).Scan(&dados); err != nil {
		return Calculo{}, err
	}
	var c Calculo
	if err := json.Unmarshal([]byte(dados), &c); err != nil {
		return Calculo{}, fmt.Errorf("ler a rescisão: %w", err)
	}
	return c, nil
}

// Desfazer apaga o registro e reabre o emprego (tira a data de saída).
func (s *Servico) Desfazer(ctx context.Context, empregoId int64) error {
	return banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, `DELETE FROM "Rescisao" WHERE "EmpregoId" = ?`, empregoId)
		if err != nil {
			return fmt.Errorf("desfazer a rescisão: %w", err)
		}
		if n, _ := r.RowsAffected(); n == 0 {
			return emprego.ErrNaoEncontrado
		}
		if _, err := tx.ExecContext(ctx, `UPDATE "Emprego" SET "Saida" = NULL WHERE "Id" = ?`, empregoId); err != nil {
			return fmt.Errorf("reabrir o emprego: %w", err)
		}
		return nil
	})
}

/* ------------------------------------------------------------ HTTP */

// Registrador é onde as rotas entram: o modulos.Rotas do núcleo.
type Registrador interface {
	HandleFunc(padrao string, h http.HandlerFunc)
}

// RegistrarRotas põe as rotas da rescisão, em /api/trabalho/ (o Registrar é o da rescisão real).
func RegistrarRotas(r Registrador, s *Servico) {
	r.HandleFunc("GET /api/trabalho/empregos/{id}/rescisao/simular", s.simularHTTP)
	r.HandleFunc("GET /api/trabalho/empregos/{id}/rescisao/comparar", s.compararHTTP)
	r.HandleFunc("GET /api/trabalho/empregos/{id}/rescisao", s.registradaHTTP)
	r.HandleFunc("POST /api/trabalho/empregos/{id}/rescisao", s.registrarHTTP)
	r.HandleFunc("DELETE /api/trabalho/empregos/{id}/rescisao", s.desfazerHTTP)
}

func idDe(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64) // id que não é número vira 0: não existe (404)
	return id
}

func (s *Servico) responder(w http.ResponseWriter, r *http.Request, status int, corpo any, err error) {
	var v erroDeValidacao
	switch {
	case err == nil:
		if corpo == nil {
			respostas.SemCorpo(w, status)
			return
		}
		respostas.JSON(w, status, respostas.ConteudoJSON, corpo)
	case errors.Is(err, emprego.ErrNaoEncontrado):
		respostas.SemCorpo(w, http.StatusNotFound)
	case errors.As(err, &v):
		respostas.Mensagem(w, http.StatusBadRequest, string(v))
	case errors.Is(err, errSemRescisao):
		respostas.Mensagem(w, http.StatusBadRequest, MensagemSemRescisao)
	case errors.Is(err, errSemTabela):
		respostas.Mensagem(w, http.StatusConflict, MensagemSemTabela)
	case errors.Is(err, errRegistrada):
		respostas.Mensagem(w, http.StatusConflict, MensagemJaRegistrada)
	default:
		s.log.ErrorContext(r.Context(), "trabalho: rescisão", "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	}
}

// saidaDaQuery lê ?saida=AAAA-MM-DD.
func saidaDaQuery(r *http.Request) (tipos.Data, bool) {
	texto, _ := respostas.ParametroDaQuery(r, "saida")
	d, err := tipos.LerData(texto)
	return d, err == nil
}

// simularHTTP: ?tipo=1&aviso=2&saida=2026-12-15.
func (s *Servico) simularHTTP(w http.ResponseWriter, r *http.Request) {
	saida, ok := saidaDaQuery(r)
	tipo, errTipo := strconv.ParseInt(r.URL.Query().Get("tipo"), 10, 64)
	aviso, errAviso := strconv.ParseInt(r.URL.Query().Get("aviso"), 10, 64)
	if !ok || errTipo != nil {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe o tipo e a data de saída (AAAA-MM-DD).")
		return
	}
	if errAviso != nil {
		aviso = SemAviso
	}
	c, err := s.Simular(r.Context(), idDe(r), tipo, aviso, saida)
	s.responder(w, r, http.StatusOK, c, err)
}

func (s *Servico) compararHTTP(w http.ResponseWriter, r *http.Request) {
	saida, ok := saidaDaQuery(r)
	if !ok {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe a data de saída (AAAA-MM-DD).")
		return
	}
	lista, err := s.Comparar(r.Context(), idDe(r), saida)
	s.responder(w, r, http.StatusOK, lista, err)
}

func (s *Servico) registradaHTTP(w http.ResponseWriter, r *http.Request) {
	if _, err := s.empregos.Obter(r.Context(), idDe(r)); err != nil {
		s.responder(w, r, 0, nil, err)
		return
	}
	c, err := s.Registrada(r.Context(), idDe(r))
	if errors.Is(err, sql.ErrNoRows) {
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, map[string]any{"registrada": nil})
		return
	}
	s.responder(w, r, http.StatusOK, map[string]any{"registrada": c}, err)
}

func (s *Servico) registrarHTTP(w http.ResponseWriter, r *http.Request) {
	corpo, ok := respostas.LerCorpo(w, r, "dto")
	if !ok {
		return
	}
	var dto struct {
		Tipo  int64       `json:"tipo"`
		Aviso int64       `json:"aviso"`
		Saida *tipos.Data `json:"saida"`
	}
	if err := json.NewDecoder(bytes.NewReader(corpo)).Decode(&dto); err != nil || dto.Saida == nil {
		respostas.Mensagem(w, http.StatusBadRequest, MensagemJSONInvalido)
		return
	}
	c, err := s.Registrar(r.Context(), idDe(r), dto.Tipo, dto.Aviso, *dto.Saida)
	s.responder(w, r, http.StatusCreated, c, err)
}

func (s *Servico) desfazerHTTP(w http.ResponseWriter, r *http.Request) {
	s.responder(w, r, http.StatusNoContent, nil, s.Desfazer(r.Context(), idDe(r)))
}
