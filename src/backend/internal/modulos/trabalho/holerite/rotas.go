package holerite

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Registrador é onde as rotas entram: o modulos.Rotas do núcleo, que as guarda (desligado, 409).
type Registrador interface {
	HandleFunc(padrao string, h http.HandlerFunc)
}

// Registrar põe as rotas do holerite, em /api/trabalho/.
func Registrar(r Registrador, s *Servico) {
	r.HandleFunc("GET /api/trabalho/empregos/{id}/holerites", s.listarHTTP)
	r.HandleFunc("POST /api/trabalho/empregos/{id}/holerites", s.criarHTTP)
	r.HandleFunc("GET /api/trabalho/holerites/{id}", s.obterHTTP)
	r.HandleFunc("PUT /api/trabalho/holerites/{id}", s.salvarHTTP)
	r.HandleFunc("POST /api/trabalho/holerites/{id}/fechar", s.fecharHTTP)
	r.HandleFunc("POST /api/trabalho/holerites/{id}/reabrir", s.reabrirHTTP)
	r.HandleFunc("DELETE /api/trabalho/holerites/{id}", s.excluirHTTP)
}

// Mensagens que a pessoa vê.
const (
	MensagemJSONInvalido = "Os dados enviados não estão no formato esperado."
	MensagemFechado      = "O holerite está fechado: reabra para mudar."
	MensagemAberto       = "O holerite já está aberto."
	MensagemSemTabela    = "Ainda não há tabela do INSS ou do IRRF para esse mês: não dá para fechar."
)

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

func idDoCaminho(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		respostas.SemCorpo(w, http.StatusNotFound)
		return 0, false
	}
	return id, true
}

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
	case errors.Is(err, ErrFechado):
		respostas.Mensagem(w, http.StatusConflict, MensagemFechado)
	case errors.Is(err, ErrAberto):
		respostas.Mensagem(w, http.StatusConflict, MensagemAberto)
	case errors.Is(err, ErrSemTabela):
		respostas.Mensagem(w, http.StatusConflict, MensagemSemTabela)
	default:
		s.log.ErrorContext(r.Context(), "trabalho: "+etapa, "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	}
}

func (s *Servico) listarHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r)
	if !ok {
		return
	}
	lista, err := s.Listar(r.Context(), id)
	s.responder(w, r, http.StatusOK, lista, err, "listar os holerites")
}

func (s *Servico) criarHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r)
	if !ok {
		return
	}
	var dto struct {
		Mes string `json:"mes"` // "2026-11"
	}
	if !lerJSON(w, r, &dto) {
		return
	}
	mes, err := tipos.LerData(dto.Mes + "-01")
	if err != nil {
		respostas.Mensagem(w, http.StatusBadRequest, "O mês vai no formato AAAA-MM.")
		return
	}
	h, err := s.Criar(r.Context(), id, mes)
	if errors.Is(err, ErrJaExiste) {
		respostas.Mensagem(w, http.StatusConflict, "Já existe o holerite de "+nomeDoMes(mes)+".")
		return
	}
	s.responder(w, r, http.StatusCreated, h, err, "criar o holerite")
}

func (s *Servico) obterHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r)
	if !ok {
		return
	}
	h, err := s.Obter(r.Context(), id)
	s.responder(w, r, http.StatusOK, h, err, "ler o holerite")
}

func (s *Servico) salvarHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r)
	if !ok {
		return
	}
	var dto struct {
		DiasUteis    int64        `json:"diasUteis"`
		DiasDescanso int64        `json:"diasDescanso"`
		Lancamentos  []Lancamento `json:"lancamentos"`
	}
	if !lerJSON(w, r, &dto) {
		return
	}
	h, err := s.Salvar(r.Context(), id, dto.DiasUteis, dto.DiasDescanso, dto.Lancamentos)
	s.responder(w, r, http.StatusOK, h, err, "gravar o holerite")
}

func (s *Servico) fecharHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r)
	if !ok {
		return
	}
	h, err := s.Fechar(r.Context(), id)
	s.responder(w, r, http.StatusOK, h, err, "fechar o holerite")
}

func (s *Servico) reabrirHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r)
	if !ok {
		return
	}
	h, err := s.Reabrir(r.Context(), r, id)
	s.responder(w, r, http.StatusOK, h, err, "reabrir o holerite")
}

func (s *Servico) excluirHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r)
	if !ok {
		return
	}
	s.responder(w, r, http.StatusNoContent, nil, s.Excluir(r.Context(), id), "excluir o holerite")
}
