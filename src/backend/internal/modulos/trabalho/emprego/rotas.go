package emprego

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Registrador é onde as rotas entram: o modulos.Rotas do núcleo, que as guarda (desligado, 409).
type Registrador interface {
	HandleFunc(padrao string, h http.HandlerFunc)
}

// Registrar põe as rotas do emprego, todas em /api/trabalho/.
func Registrar(r Registrador, s *Servico) {
	r.HandleFunc("GET /api/trabalho/empregos", s.listarHTTP)
	r.HandleFunc("POST /api/trabalho/empregos", s.criarHTTP)
	r.HandleFunc("PUT /api/trabalho/empregos/{id}", s.atualizarHTTP)
	r.HandleFunc("DELETE /api/trabalho/empregos/{id}", s.excluirHTTP)
	r.HandleFunc("POST /api/trabalho/empregos/{id}/salarios", s.definirSalarioHTTP)
	r.HandleFunc("DELETE /api/trabalho/empregos/{id}/salarios/{salarioId}", s.excluirSalarioHTTP)
	r.HandleFunc("GET /api/trabalho/empregos/{id}/estimativa", s.estimarHTTP)
	r.HandleFunc("GET /api/trabalho/tabelas", s.tabelasHTTP)
}

// Mensagens das respostas que a pessoa vê.
const (
	MensagemJSONInvalido = "Os dados enviados não estão no formato esperado."
	MensagemSemTabela    = "Ainda não há tabela do INSS ou do IRRF para esse mês."
	MensagemUltimo       = "O emprego precisa de pelo menos um salário: mude o valor em vez de apagar."
)

// dadosDto é o corpo do POST e do PUT do emprego.
type dadosDto struct {
	Empregador         string         `json:"empregador"`
	Cnpj               *string        `json:"cnpj"`
	Admissao           *tipos.Data    `json:"admissao"`
	Saida              *tipos.Data    `json:"saida"`
	Regime             *int64         `json:"regime"`
	JornadaMensalHoras *int64         `json:"jornadaMensalHoras"`
	Dependentes        int64          `json:"dependentes"`
	ContaId            *int64         `json:"contaId"`
	SalarioBruto       *tipos.Decimal `json:"salarioBruto"`
}

func (d dadosDto) dados() (Dados, error) {
	if d.Admissao == nil {
		return Dados{}, invalido("Informe a data de admissão.")
	}
	r := Dados{Empregador: d.Empregador, Cnpj: d.Cnpj, Admissao: *d.Admissao, Saida: d.Saida, Regime: RegimeClt,
		JornadaMensalHoras: 220, Dependentes: d.Dependentes, ContaId: d.ContaId, SalarioInicial: d.SalarioBruto}
	if d.Regime != nil {
		r.Regime = *d.Regime
	}
	if d.JornadaMensalHoras != nil {
		r.JornadaMensalHoras = *d.JornadaMensalHoras
	}
	return r, nil
}

type salarioDto struct {
	Inicio       *tipos.Data    `json:"inicio"`
	SalarioBruto *tipos.Decimal `json:"salarioBruto"`
}

// lerJSON lê o corpo (415 e 413 pelo respostas.LerCorpo) num destino; false se já respondeu.
func lerJSON(w http.ResponseWriter, r *http.Request, destino any) bool {
	corpo, ok := respostas.LerCorpo(w, r, "dto")
	if !ok {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(corpo))
	if err := dec.Decode(destino); err != nil {
		respostas.Mensagem(w, http.StatusBadRequest, MensagemJSONInvalido)
		return false
	}
	return true
}

func idDoCaminho(w http.ResponseWriter, r *http.Request, nome string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(nome), 10, 64)
	if err != nil { // id 0 ou negativo segue: o banco não tem, e a resposta é o mesmo 404
		respostas.SemCorpo(w, http.StatusNotFound)
		return 0, false
	}
	return id, true
}

// responder responde o resultado ou o erro: 404, 400 com a mensagem, ou 500 (no log).
func (s *Servico) responder(w http.ResponseWriter, r *http.Request, status int, corpo any, err error, etapa string) {
	var v erroDeValidacao
	switch {
	case err == nil:
		if corpo == nil {
			respostas.SemCorpo(w, status)
			return
		}
		respostas.JSON(w, status, respostas.ConteudoJSON, corpo)
	case errors.Is(err, ErrNaoEncontrado):
		respostas.SemCorpo(w, http.StatusNotFound)
	case errors.As(err, &v):
		respostas.Mensagem(w, http.StatusBadRequest, v.mensagem)
	case errors.Is(err, ErrUltimoSalario):
		respostas.Mensagem(w, http.StatusBadRequest, MensagemUltimo)
	case errors.Is(err, ErrSemTabela):
		respostas.Mensagem(w, http.StatusConflict, MensagemSemTabela)
	default:
		s.log.ErrorContext(r.Context(), "trabalho: "+etapa, "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	}
}

func (s *Servico) listarHTTP(w http.ResponseWriter, r *http.Request) {
	lista, err := s.Listar(r.Context())
	s.responder(w, r, http.StatusOK, lista, err, "listar os empregos")
}

func (s *Servico) criarHTTP(w http.ResponseWriter, r *http.Request) {
	var dto dadosDto
	if !lerJSON(w, r, &dto) {
		return
	}
	d, err := dto.dados()
	if err != nil {
		s.responder(w, r, 0, nil, err, "")
		return
	}
	e, err := s.Criar(r.Context(), d)
	s.responder(w, r, http.StatusCreated, e, err, "criar o emprego")
}

func (s *Servico) atualizarHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r, "id")
	if !ok {
		return
	}
	var dto dadosDto
	if !lerJSON(w, r, &dto) {
		return
	}
	d, err := dto.dados()
	if err != nil {
		s.responder(w, r, 0, nil, err, "")
		return
	}
	e, err := s.Atualizar(r.Context(), id, d)
	s.responder(w, r, http.StatusOK, e, err, "atualizar o emprego")
}

func (s *Servico) excluirHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r, "id")
	if !ok {
		return
	}
	s.responder(w, r, http.StatusNoContent, nil, s.Excluir(r.Context(), id), "excluir o emprego")
}

func (s *Servico) definirSalarioHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r, "id")
	if !ok {
		return
	}
	var dto salarioDto
	if !lerJSON(w, r, &dto) {
		return
	}
	if dto.Inicio == nil || dto.SalarioBruto == nil {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe o mês e o salário bruto.")
		return
	}
	e, err := s.DefinirSalario(r.Context(), id, *dto.Inicio, *dto.SalarioBruto)
	s.responder(w, r, http.StatusOK, e, err, "gravar o salário")
}

func (s *Servico) excluirSalarioHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r, "id")
	if !ok {
		return
	}
	salarioId, ok := idDoCaminho(w, r, "salarioId")
	if !ok {
		return
	}
	e, err := s.ExcluirSalario(r.Context(), id, salarioId)
	s.responder(w, r, http.StatusOK, e, err, "excluir o salário")
}

// estimarHTTP é a estimativa do mês (?mes=2026-11; sem ?mes, o mês de hoje em Brasília).
func (s *Servico) estimarHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r, "id")
	if !ok {
		return
	}
	mes := InicioDoMes(relogio.Hoje(s.relogio))
	if texto, veio := respostas.ParametroDaQuery(r, "mes"); veio {
		d, err := tipos.LerData(texto + "-01")
		if err != nil {
			respostas.Mensagem(w, http.StatusBadRequest, "O mês vai no formato AAAA-MM.")
			return
		}
		mes = d
	}
	e, err := s.Estimar(r.Context(), id, mes)
	s.responder(w, r, http.StatusOK, e, err, "estimar o mês")
}

func (s *Servico) tabelasHTTP(w http.ResponseWriter, r *http.Request) {
	t, err := s.LerTabelas(r.Context())
	s.responder(w, r, http.StatusOK, t, err, "ler as tabelas")
}
