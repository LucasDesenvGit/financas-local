package holeritepdf

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/leitor"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/desfazer"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
)

// Registrador é onde as rotas entram: o modulos.Rotas do núcleo, que as guarda (desligado, 409).
type Registrador interface {
	HandleFunc(padrao string, h http.HandlerFunc)
}

// Registrar põe as rotas do holerite em PDF, em /api/trabalho/.
func Registrar(r Registrador, s *Servico) {
	r.HandleFunc("POST /api/trabalho/empregos/{id}/holerites/pdf", s.lerHTTP)
	r.HandleFunc("POST /api/trabalho/empregos/{id}/holerites/pdf/confirmar", s.confirmarHTTP)
	r.HandleFunc("POST /api/trabalho/empregos/{id}/holerites/pdf/desfazer/{chave}", s.desfazerHTTP)
}

// Motivos da recusa do PDF, para a tela saber o que fazer (pedir a senha, oferecer a digitação).
const (
	MotivoSenha    = "senha"
	MotivoSemTexto = "semTexto"
)

// Mensagens que a pessoa vê.
const (
	MensagemJSONInvalido = "Os dados enviados não estão no formato esperado."
	MensagemNaoEPDF      = "O arquivo não é um PDF."
	MensagemGrande       = "O PDF passa do limite (700 KB e 5 páginas): mande só o holerite."
	MensagemSenha        = "O PDF tem senha: digite a senha para abrir. Ela é usada só para abrir o arquivo e não fica guardada."
	MensagemSenhaErrada  = "A senha não abriu o PDF. Confira e tente de novo."
	MensagemSemTexto     = "Este PDF é uma imagem, e por enquanto o leitor só lê PDF com texto. Dá para digitar o holerite."
	MensagemIlegivel     = "O PDF não abriu: ele pode estar corrompido. Tente baixar de novo."
	MensagemSemTabela    = "Não achei a tabela do holerite (as colunas de vencimentos e de descontos) neste PDF. Dá para digitar o holerite."
	MensagemFechado      = "O holerite do mês está fechado: reabra antes de gravar o lido."
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

func recusar(w http.ResponseWriter, mensagem, motivo string) {
	respostas.JSON(w, http.StatusBadRequest, respostas.ConteudoJSON, map[string]string{"mensagem": mensagem, "motivo": motivo})
}

// lerHTTP é o POST .../holerites/pdf: o PDF em base64 (o corpo é JSON, com o limite de 1 MB do site) e a senha, se
// tiver. Responde a prévia; nada é gravado. Nem o PDF, nem a senha, nem o texto vão para o log.
func (s *Servico) lerHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r)
	if !ok {
		return
	}
	var dto struct {
		Arquivo []byte `json:"arquivo"` // base64
		Senha   string `json:"senha"`
	}
	if !lerJSON(w, r, &dto) {
		return
	}
	p, err := s.Ler(r.Context(), id, dto.Arquivo, dto.Senha)
	switch {
	case err == nil:
		respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, p)
	case errors.Is(err, ErrNaoEncontrado):
		respostas.SemCorpo(w, http.StatusNotFound)
	case errors.Is(err, leitor.ErrSenha) && dto.Senha == "":
		recusar(w, MensagemSenha, MotivoSenha)
	case errors.Is(err, leitor.ErrSenha):
		recusar(w, MensagemSenhaErrada, MotivoSenha)
	case errors.Is(err, leitor.ErrSemTexto):
		recusar(w, MensagemSemTexto, MotivoSemTexto)
	case errors.Is(err, leitor.ErrSemTabela):
		recusar(w, MensagemSemTabela, MotivoSemTexto)
	case errors.Is(err, leitor.ErrNaoEPDF):
		respostas.Mensagem(w, http.StatusBadRequest, MensagemNaoEPDF)
	case errors.Is(err, leitor.ErrGrande):
		respostas.Mensagem(w, http.StatusBadRequest, MensagemGrande)
	case errors.Is(err, leitor.ErrIlegivel):
		respostas.Mensagem(w, http.StatusBadRequest, MensagemIlegivel)
	default:
		s.log.ErrorContext(r.Context(), "trabalho: ler o holerite em PDF", "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	}
}

// confirmarHTTP é o POST .../holerites/pdf/confirmar: grava o holerite lido (201, com a chave do "Desfazer").
func (s *Servico) confirmarHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r)
	if !ok {
		return
	}
	var dto Confirmacao
	if !lerJSON(w, r, &dto) {
		return
	}
	g, err := s.Confirmar(r.Context(), id, dto)
	var v erroDeValidacao
	switch {
	case err == nil:
		respostas.JSON(w, http.StatusCreated, respostas.ConteudoJSON, g)
	case errors.Is(err, ErrNaoEncontrado) || errors.Is(err, holerite.ErrNaoEncontrado):
		respostas.SemCorpo(w, http.StatusNotFound)
	case errors.As(err, &v) || errors.Is(err, holerite.ErrInvalido):
		respostas.Mensagem(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, holerite.ErrFechado):
		respostas.Mensagem(w, http.StatusConflict, MensagemFechado)
	default:
		s.log.ErrorContext(r.Context(), "trabalho: gravar o holerite lido", "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	}
}

// desfazerHTTP é o POST .../holerites/pdf/desfazer/{chave}: volta ao que era antes de gravar (204).
func (s *Servico) desfazerHTTP(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(w, r)
	if !ok {
		return
	}
	desfeito, err := s.Desfazer(r.Context(), id, r.PathValue("chave"))
	switch {
	case err != nil:
		s.log.ErrorContext(r.Context(), "trabalho: desfazer o holerite lido", "erro", err)
		respostas.SemCorpo(w, http.StatusInternalServerError)
	case !desfeito:
		respostas.Mensagem(w, http.StatusNotFound, desfazer.MensagemVencida)
	default:
		respostas.SemCorpo(w, http.StatusNoContent)
	}
}
