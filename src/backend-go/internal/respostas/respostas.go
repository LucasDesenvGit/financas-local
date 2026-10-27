// Pacote respostas tem as respostas que o ASP.NET monta sozinho, antes ou em volta da action, iguais às do .NET
// (o contrato #72 compara): o ProblemDetails do CSRF recusado, o ValidationProblemDetails do [ApiController] com as
// mensagens do System.Text.Json, o 415, a {"mensagem": ...} do BaseController e o 204/401/404 sem corpo. Os textos em
// inglês são os do ASP.NET, de propósito.
package respostas

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode/utf16"
)

// Mensagem é a resposta do Erro(...) do BaseController: {"mensagem": "..."}.
func Mensagem(w http.ResponseWriter, status int, mensagem string) {
	JSON(w, status, ConteudoJSON, map[string]string{"mensagem": mensagem})
}

// SemCorpo responde só o status, com "Content-Length: 0", como o Kestrel (401, 404, 429 do limitador).
func SemCorpo(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(status)
}

// CortarUtf16 corta o texto em n caracteres UTF-16 (o s[..n] do C#), sem partir um caractere no meio.
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

// LerCorpo é a parte comum do model binding de [FromBody]: o tipo tem de ser JSON (415), o corpo não pode passar do
// limite (413) nem vir vazio (400 com "A non-empty request body is required."). Devolve false se já respondeu.
func LerCorpo(w http.ResponseWriter, r *http.Request, parametro string) ([]byte, bool) {
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
		CorpoVazio(w, parametro)
		return nil, false
	}
	return corpo, true
}

// CorpoVazio é o 400 do BodyModelBinder quando não veio corpo, ou veio um que o System.Text.Json lê como nulo (o
// "null"): o InputFormatterResult.NoValue do SystemTextJsonInputFormatter.
func CorpoVazio(w http.ResponseWriter, parametro string) {
	Validacao(w, map[string][]string{"": {"A non-empty request body is required."}, parametro: {Obrigatorio(parametro)}})
}

// CaminhoDaChave acrescenta ao caminho do System.Text.Json ("$.valores") o nome de uma propriedade ou chave de
// dicionário: ".nome", ou "['nome']" quando ele tem um dos caracteres especiais do .NET 10 (JsonReaderHelper,
// SpecialCharacters; o .NET 10 não escapa o nome dentro dos colchetes).
func CaminhoDaChave(caminho, nome string) string {
	if strings.ContainsAny(nome, especiaisNoCaminho) {
		return caminho + "['" + nome + "']"
	}
	return caminho + "." + nome
}

const especiaisNoCaminho = ". '/\"[]()\t\n\r\f\b\\\u0085\u2028\u2029"

// Respostas de erro que o ASP.NET monta sozinho, antes da action, e que o contrato (#72) gravou: o ProblemDetails do
// CSRF recusado, o ValidationProblemDetails do [ApiController] (corpo que não abre, campo que falta) e o 415 de
// corpo que não é JSON. Os textos em inglês são os do ASP.NET, de propósito: o Angular não os mostra, mas o contrato
// compara.
const (
	tipoBadRequest       = "https://tools.ietf.org/html/rfc9110#section-15.5.1"
	tipoUnsupportedMedia = "https://tools.ietf.org/html/rfc9110#section-15.5.16"
	tituloValidacao      = "One or more validation errors occurred."
)

// novoTraceID é o formato do Activity.Id do .NET (W3C traceparent): "00-<32 hex>-<16 hex>-00".
func NovoTraceID() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "00-" + hex.EncodeToString(b[:16]) + "-" + hex.EncodeToString(b[16:]) + "-00"
}

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

// CsrfRecusado é o 400 do AutoValidateAntiforgeryToken: ProblemDetails sem "errors".
func CsrfRecusado(w http.ResponseWriter) {
	JSON(w, http.StatusBadRequest, ConteudoProblem, map[string]any{
		"type": tipoBadRequest, "title": "Bad Request", "status": http.StatusBadRequest, "traceId": NovoTraceID(),
	})
}

// Validacao é o 400 automático do [ApiController]. O .NET grava este com "application/json" (o
// HttpClient do Angular pede application/json), e não com problem+json: o contrato mostra.
func Validacao(w http.ResponseWriter, erros map[string][]string) {
	JSON(w, http.StatusBadRequest, ConteudoJSON, map[string]any{
		"type": tipoBadRequest, "title": tituloValidacao, "status": http.StatusBadRequest, "errors": erros,
		"traceId": NovoTraceID(),
	})
}

func TipoNaoSuportado(w http.ResponseWriter) {
	JSON(w, http.StatusUnsupportedMediaType, ConteudoProblem, map[string]any{
		"type": tipoUnsupportedMedia, "title": "Unsupported Media Type", "status": http.StatusUnsupportedMediaType,
		"traceId": NovoTraceID(),
	})
}

func Obrigatorio(nome string) string { return "The " + nome + " field is required." }

// tamanhoUtf16 é o string.Length do C#: unidades UTF-16 (um emoji conta 2).
func TamanhoUtf16(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// erroDoJSON é o erro do System.Text.Json: a chave em "errors" é o caminho ("$" ou "$.login").
type ErroJSON struct {
	Caminho  string
	Mensagem string
}

// posicao é o "LineNumber: L | BytePositionInLine: B" do .NET para o byte n do corpo (linha a partir de zero).
func Posicao(corpo []byte, n int) string {
	antes := corpo[:n]
	linha := bytes.Count(antes, []byte("\n"))
	inicioLinha := bytes.LastIndexByte(antes, '\n') + 1
	return fmt.Sprintf("LineNumber: %d | BytePositionInLine: %d", linha, n-inicioLinha)
}
