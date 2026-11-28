// Pacote respostas tem as respostas que a API monta sozinha, antes ou em volta da regra: o 400 do CSRF recusado, o 400
// de validação (corpo que não abre, campo que falta, valor fora da faixa), o 415, a {"mensagem": ...} e o 204/401/404
// sem corpo. O formato é o ProblemDetails, com "errors" por campo, que o front-end lê; as mensagens são
// em português, porque o erro.interceptor.ts mostra a primeira de "errors" na tela (#710).
package respostas

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/texto"
)

// Mensagem é a resposta de erro da regra: {"mensagem": "..."}.
func Mensagem(w http.ResponseWriter, status int, mensagem string) {
	JSON(w, status, ConteudoJSON, map[string]string{"mensagem": mensagem})
}

// SemCorpo responde só o status, com "Content-Length: 0" (401, 404, 429 do limitador).
func SemCorpo(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(status)
}

// CortarUtf16 corta o texto em n caracteres UTF-16, sem partir um caractere no meio.
func CortarUtf16(s string, n int) string {
	usados := 0
	for i, r := range s {
		tamanho := 1
		if r > 0xFFFF {
			tamanho = 2
		}
		if usados+tamanho > n {
			return s[:i]
		}
		usados += tamanho
	}
	return s
}

// LerCorpo é a parte comum da leitura do corpo: o tipo tem de ser JSON (415), o corpo não pode passar do limite (413)
// nem vir vazio (400). Devolve false se já respondeu.
func LerCorpo(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	tipo, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if tipo != "application/json" && !strings.HasSuffix(tipo, "+json") {
		TipoNaoSuportado(w)
		return nil, false
	}
	corpo, err := io.ReadAll(r.Body)
	var grande *http.MaxBytesError
	if errors.As(err, &grande) {
		SemCorpo(w, http.StatusRequestEntityTooLarge)
		return nil, false
	}
	if err != nil || len(bytes.TrimSpace(corpo)) == 0 {
		CorpoVazio(w)
		return nil, false
	}
	return corpo, true
}

// MensagemFormatoEsperado é o 400 do corpo que não é o JSON esperado, para as rotas que respondem só {mensagem}.
const MensagemFormatoEsperado = "Os dados enviados não estão no formato esperado."

// LerJSON lê o corpo (LerCorpo: 415, 413, 400) num destino; JSON quebrado ou de outro formato é 400 com
// MensagemFormatoEsperado. Devolve false se já respondeu.
func LerJSON(w http.ResponseWriter, r *http.Request, destino any) bool {
	corpo, ok := LerCorpo(w, r)
	if !ok {
		return false
	}
	if err := json.NewDecoder(bytes.NewReader(corpo)).Decode(destino); err != nil {
		Mensagem(w, http.StatusBadRequest, MensagemFormatoEsperado)
		return false
	}
	return true
}

// IdDe é o id numérico do segmento {nome} do caminho; o que não é número vira 0, que não existe (404).
func IdDe(r *http.Request, nome string) int64 {
	id, _ := strconv.ParseInt(r.PathValue(nome), 10, 64)
	return id
}

// CorpoVazio é o 400 do pedido sem corpo, ou com o corpo "null".
func CorpoVazio(w http.ResponseWriter) {
	Validacao(w, map[string][]string{"": {MensagemCorpoVazio}})
}

// Mensagens das respostas que não dependem de um campo.
const (
	MensagemCorpoVazio   = "O pedido veio sem dados."
	MensagemJSONInvalido = "Os dados do pedido não estão num formato válido."
)

// CaminhoDaChave acrescenta ao caminho do erro ("$.valores") o nome de uma propriedade ou chave de dicionário:
// ".nome", ou "['nome']" quando ele tem um caractere que confundiria o caminho (ponto, aspas, colchetes, espaço).
func CaminhoDaChave(caminho, nome string) string {
	if strings.ContainsAny(nome, especiaisNoCaminho) {
		return caminho + "['" + nome + "']"
	}
	return caminho + "." + nome
}

const especiaisNoCaminho = ". '/\"[]()\t\n\r\f\b\\\u0085  "

// Os "type" são os da RFC 9110, como no ProblemDetails; o "title" diz em português o que houve.
const (
	tipoBadRequest       = "https://tools.ietf.org/html/rfc9110#section-15.5.1"
	tipoUnsupportedMedia = "https://tools.ietf.org/html/rfc9110#section-15.5.16"
	tituloValidacao      = "Há dados inválidos no pedido."
)

func JSON(w http.ResponseWriter, status int, tipoConteudo string, corpo any) {
	dados, err := json.Marshal(corpo)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", tipoConteudo)
	w.WriteHeader(status)
	_, _ = w.Write(dados)
}

const (
	ConteudoJSON    = "application/json; charset=utf-8"
	ConteudoProblem = "application/problem+json; charset=utf-8"
)

// CsrfRecusado é o 400 do token antifalsificação que não veio ou não bate: ProblemDetails sem "errors".
func CsrfRecusado(w http.ResponseWriter) {
	JSON(w, http.StatusBadRequest, ConteudoProblem, map[string]any{
		"type": tipoBadRequest, "title": "Pedido recusado: o token de segurança não veio ou não confere.", "status": http.StatusBadRequest,
	})
}

// Validacao é o 400 de validação, com as mensagens por campo em "errors" (a chave "" é a do pedido inteiro). Sai com
// "application/json", que é o que o HttpClient do Angular pede.
func Validacao(w http.ResponseWriter, erros map[string][]string) {
	JSON(w, http.StatusBadRequest, ConteudoJSON, map[string]any{
		"type": tipoBadRequest, "title": tituloValidacao, "status": http.StatusBadRequest, "errors": erros,
	})
}

// TipoNaoSuportado é o 415 do corpo que não é JSON.
func TipoNaoSuportado(w http.ResponseWriter) {
	JSON(w, http.StatusUnsupportedMediaType, ConteudoJSON, map[string]any{
		"type": tipoUnsupportedMedia, "title": "Os dados do pedido têm de vir em JSON.",
		"status": http.StatusUnsupportedMediaType,
	})
}

// Obrigatorio é a mensagem do campo que não veio, ou veio vazio.
func Obrigatorio(nome string) string { return "O campo " + nome + " é obrigatório." }

// TamanhoUtf16 conta os chars UTF-16 (texto.TamanhoUtf16), para quem já fala HTTP e valida a entrada.
func TamanhoUtf16(s string) int { return texto.TamanhoUtf16(s) }

// ErroJSON é o erro de leitura do corpo: a chave em "errors" é o caminho ("$" ou "$.login").
type ErroJSON struct {
	Caminho  string
	Mensagem string
}
