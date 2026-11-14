package decimoterceiro

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

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/encargos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Mensagens que a pessoa vê.
const (
	MensagemSemDecimo    = "Estágio e PJ não têm 13º salário pela CLT."
	MensagemSemTabela    = "Ainda não há tabela do INSS ou do IRRF para esse ano."
	MensagemForaDoAno    = "O emprego não estava ativo nesse ano."
	MensagemJSONInvalido = "Os dados enviados não estão no formato esperado."
)

// Resposta é o 13º do ano com o que a tela precisa dizer.
type Resposta struct {
	Calculo
	// Saida: a pessoa saiu no ano; o 13º proporcional entra na rescisão.
	Saida *tipos.Data `json:"saida"`
}

// Servico calcula o 13º e guarda a 1ª parcela informada.
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

// Registrador é onde as rotas entram: o modulos.Rotas do núcleo.
type Registrador interface {
	HandleFunc(padrao string, h http.HandlerFunc)
}

// Registrar põe as rotas do 13º, em /api/trabalho/.
func Registrar(r Registrador, s *Servico) {
	r.HandleFunc("GET /api/trabalho/empregos/{id}/decimo-terceiro/{ano}", s.obterHTTP)
	r.HandleFunc("PUT /api/trabalho/empregos/{id}/decimo-terceiro/{ano}", s.definirHTTP)
}

var (
	errSemDecimo = errors.New("sem 13º")
	errSemTabela = errors.New("sem tabela")
	errForaDoAno = errors.New("fora do ano")
)

// Obter calcula o 13º do emprego no ano.
func (s *Servico) Obter(ctx context.Context, empregoId int64, ano int) (Resposta, error) {
	e, err := s.empregos.Obter(ctx, empregoId)
	if err != nil {
		return Resposta{}, err
	}
	if e.Regime != emprego.RegimeClt {
		return Resposta{}, errSemDecimo
	}
	avos := Avos(ano, e.Admissao, e.Saida)
	if avos == 0 {
		return Resposta{}, errForaDoAno
	}
	// O salário base: o de dezembro, ou o do mês da saída, se saiu no ano.
	mesBase := tipos.NovaData(ano, 12, 1)
	var saida *tipos.Data
	if e.Saida != nil && e.Saida.Ano() == ano {
		mesBase, saida = emprego.InicioDoMes(*e.Saida), e.Saida
	}
	salario, ok := emprego.SalarioDoMes(e.Salarios, mesBase)
	if !ok {
		return Resposta{}, errForaDoAno
	}
	soma, _, err := s.holerites.VariaveisDoAno(ctx, empregoId, ano)
	if err != nil {
		return Resposta{}, err
	}
	inss, err := encargos.LerTabelasInss(ctx, s.db)
	if err != nil {
		return Resposta{}, err
	}
	irrf, err := encargos.LerTabelasIrrf(ctx, s.db)
	if err != nil {
		return Resposta{}, err
	}
	tInss, okInss := encargos.Vigente(inss, mesBase)
	tIrrf, okIrrf := encargos.Vigente(irrf, mesBase)
	if !okInss || !okIrrf {
		return Resposta{}, errSemTabela
	}
	entrada := Entrada{Ano: ano, Avos: avos, SalarioBase: salario.SalarioBruto, SomaVariaveis: soma, Dependentes: e.Dependentes,
		Inss: tInss, Irrf: tIrrf}
	err = s.db.QueryRowContext(ctx, `SELECT "PrimeiraParcelaEm", "PrimeiraParcelaValor" FROM "DecimoTerceiro"
		WHERE "EmpregoId" = ? AND "Ano" = ?`, empregoId, ano).Scan(&entrada.PrimeiraEm, &entrada.PrimeiraValor)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Resposta{}, fmt.Errorf("ler a 1ª parcela: %w", err)
	}
	return Resposta{Calculo: Calcular(entrada), Saida: saida}, nil
}

// DefinirPrimeira guarda quando e quanto veio a 1ª parcela (os dois nulos: volta ao padrão, metade até 30/11, porque o
// cálculo só usa o que veio preenchido).
func (s *Servico) DefinirPrimeira(ctx context.Context, empregoId int64, ano int, em *tipos.Data, valor *tipos.Decimal) (Resposta, error) {
	if _, err := s.Obter(ctx, empregoId, ano); err != nil {
		return Resposta{}, err
	}
	if valor != nil && (valor.Decimal().IsNegative() || valor.Decimal().GreaterThan(decimal.NewFromInt(10_000_000))) {
		return Resposta{}, erroDeValidacao("O valor da 1ª parcela vai de R$ 0,00 a R$ 10.000.000,00.")
	}
	if em != nil && em.Ano() != ano {
		return Resposta{}, erroDeValidacao(fmt.Sprintf("A 1ª parcela do 13º de %d é paga em %d.", ano, ano))
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO "DecimoTerceiro" ("EmpregoId", "Ano", "PrimeiraParcelaEm", "PrimeiraParcelaValor", "AtualizadoEm")
		VALUES (?, ?, ?, ?, ?) ON CONFLICT ("EmpregoId", "Ano") DO UPDATE SET "PrimeiraParcelaEm" = excluded."PrimeiraParcelaEm",
		"PrimeiraParcelaValor" = excluded."PrimeiraParcelaValor", "AtualizadoEm" = excluded."AtualizadoEm"`,
		empregoId, ano, em, valor, tipos.NovaDataHora(s.relogio.Agora()))
	if err != nil {
		return Resposta{}, fmt.Errorf("gravar a 1ª parcela: %w", err)
	}
	return s.Obter(ctx, empregoId, ano)
}

type erroDeValidacao string

func (e erroDeValidacao) Error() string { return string(e) }

func caminho(w http.ResponseWriter, r *http.Request) (int64, int, bool) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64) // id que não é número vira 0: o emprego não existe (404)
	ano, err := strconv.Atoi(r.PathValue("ano"))
	if err != nil {
		ano = 0
	}
	if ano < 1900 || ano > 2200 {
		respostas.SemCorpo(w, http.StatusNotFound)
		return 0, 0, false
	}
	return id, ano, true
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
	case errors.Is(err, errSemDecimo):
		respostas.Mensagem(w, http.StatusBadRequest, MensagemSemDecimo)
	case errors.Is(err, errForaDoAno):
		respostas.Mensagem(w, http.StatusBadRequest, MensagemForaDoAno)
	case errors.Is(err, errSemTabela):
		respostas.Mensagem(w, http.StatusConflict, MensagemSemTabela)
	default:
		s.log.ErrorContext(r.Context(), "trabalho: 13º salário", "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	}
}

func (s *Servico) obterHTTP(w http.ResponseWriter, r *http.Request) {
	id, ano, ok := caminho(w, r)
	if !ok {
		return
	}
	c, err := s.Obter(r.Context(), id, ano)
	s.responder(w, r, c, err)
}

func (s *Servico) definirHTTP(w http.ResponseWriter, r *http.Request) {
	id, ano, ok := caminho(w, r)
	if !ok {
		return
	}
	corpo, ok := respostas.LerCorpo(w, r, "dto")
	if !ok {
		return
	}
	var dto struct {
		PrimeiraParcelaEm    *tipos.Data    `json:"primeiraParcelaEm"`
		PrimeiraParcelaValor *tipos.Decimal `json:"primeiraParcelaValor"`
	}
	if err := json.NewDecoder(bytes.NewReader(corpo)).Decode(&dto); err != nil {
		respostas.Mensagem(w, http.StatusBadRequest, MensagemJSONInvalido)
		return
	}
	c, err := s.DefinirPrimeira(r.Context(), id, ano, dto.PrimeiraParcelaEm, dto.PrimeiraParcelaValor)
	s.responder(w, r, c, err)
}
