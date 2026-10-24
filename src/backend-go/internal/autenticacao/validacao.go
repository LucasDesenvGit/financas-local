package autenticacao

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
func novoTraceID() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "00-" + hex.EncodeToString(b[:16]) + "-" + hex.EncodeToString(b[16:]) + "-00"
}

func escreverJSON(w http.ResponseWriter, status int, tipoConteudo string, corpo any) {
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
	conteudoJSON    = "application/json; charset=utf-8"
	conteudoProblem = "application/problem+json; charset=utf-8"
)

// responderCsrfRecusado é o 400 do AutoValidateAntiforgeryToken: ProblemDetails sem "errors".
func responderCsrfRecusado(w http.ResponseWriter) {
	escreverJSON(w, http.StatusBadRequest, conteudoProblem, map[string]any{
		"type": tipoBadRequest, "title": "Bad Request", "status": http.StatusBadRequest, "traceId": novoTraceID(),
	})
}

// responderValidacao é o 400 automático do [ApiController]. O .NET grava este com "application/json" (o
// HttpClient do Angular pede application/json), e não com problem+json: o contrato mostra.
func responderValidacao(w http.ResponseWriter, erros map[string][]string) {
	escreverJSON(w, http.StatusBadRequest, conteudoJSON, map[string]any{
		"type": tipoBadRequest, "title": tituloValidacao, "status": http.StatusBadRequest, "errors": erros,
		"traceId": novoTraceID(),
	})
}

func responderTipoNaoSuportado(w http.ResponseWriter) {
	escreverJSON(w, http.StatusUnsupportedMediaType, conteudoProblem, map[string]any{
		"type": tipoUnsupportedMedia, "title": "Unsupported Media Type", "status": http.StatusUnsupportedMediaType,
		"traceId": novoTraceID(),
	})
}

// campoTexto é um parâmetro string de um DTO com [Required] e [StringLength(maximo, MinimumLength = minimo)].
type campoTexto struct {
	propriedade string // o nome da propriedade no C#, que é a chave em "errors" ("Login", "SenhaAtual")
	minimo      int
	maximo      int
	valor       *string
}

// lerCorpo faz o model binding e a validação do [ApiController] para um DTO só de strings: devolve false se já
// respondeu (415 ou 400). parametro é o nome do parâmetro da action ("dto"), que aparece quando o corpo não abre;
// tipoDto é o nome completo do tipo no C#, que aparece quando o JSON não é um objeto.
func lerCorpo(w http.ResponseWriter, r *http.Request, parametro, tipoDto string, campos ...*campoTexto) bool {
	tipo, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if tipo != "application/json" && !strings.HasSuffix(tipo, "+json") {
		responderTipoNaoSuportado(w)
		return false
	}
	corpo, err := io.ReadAll(r.Body)
	if err != nil {
		var grande *http.MaxBytesError
		if errors.As(err, &grande) {
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return false
		}
		responderValidacao(w, map[string][]string{"": {"A non-empty request body is required."}, parametro: {obrigatorio(parametro)}})
		return false
	}
	if len(bytes.TrimSpace(corpo)) == 0 {
		responderValidacao(w, map[string][]string{"": {"A non-empty request body is required."}, parametro: {obrigatorio(parametro)}})
		return false
	}

	if erroJSON := lerCampos(corpo, tipoDto, campos); erroJSON != nil {
		responderValidacao(w, map[string][]string{erroJSON.caminho: {erroJSON.mensagem}, parametro: {obrigatorio(parametro)}})
		return false
	}

	erros := map[string][]string{}
	for _, c := range campos {
		var lista []string
		if c.valor == nil || strings.TrimSpace(*c.valor) == "" {
			// [Required] recusa nulo, vazio e só espaços (AllowEmptyStrings = false).
			lista = append(lista, obrigatorio(c.propriedade))
		}
		if c.valor != nil {
			// [StringLength] conta caracteres UTF-16, como string.Length do C#; nulo passa (quem recusa é o Required).
			if n := tamanhoUtf16(*c.valor); n > c.maximo || n < c.minimo {
				if c.minimo > 0 {
					lista = append(lista, fmt.Sprintf("The field %s must be a string with a minimum length of %d and a maximum length of %d.",
						c.propriedade, c.minimo, c.maximo))
				} else {
					lista = append(lista, fmt.Sprintf("The field %s must be a string with a maximum length of %d.", c.propriedade, c.maximo))
				}
			}
		}
		if len(lista) > 0 {
			erros[c.propriedade] = lista
		}
	}
	if len(erros) > 0 {
		responderValidacao(w, erros)
		return false
	}
	return true
}

func obrigatorio(nome string) string { return "The " + nome + " field is required." }

// tamanhoUtf16 é o string.Length do C#: unidades UTF-16 (um emoji conta 2).
func tamanhoUtf16(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// erroDoJSON é o erro do System.Text.Json: a chave em "errors" é o caminho ("$" ou "$.login").
type erroDoJSON struct {
	caminho  string
	mensagem string
}

// lerCampos lê o objeto JSON como o System.Text.Json com as opções web do ASP.NET: nomes sem diferença de
// maiúsculas, campos a mais ignorados, o último repetido vale. Cada campo é string ou null.
func lerCampos(corpo []byte, tipoDto string, campos []*campoTexto) *erroDoJSON {
	var bruto any
	if err := json.Unmarshal(corpo, &bruto); err != nil {
		return erroDeSintaxe(corpo, err)
	}
	objeto, ok := bruto.(map[string]any)
	if !ok {
		return &erroDoJSON{"$", fmt.Sprintf("The JSON value could not be converted to %s. Path: $ | %s.", tipoDto, posicao(corpo, primeiroNaoBranco(corpo)+1))}
	}
	// A ordem do JSON decide qual repetido vale; json.Unmarshal num map já fica com o último.
	for nome, valor := range objeto {
		for _, c := range campos {
			if !strings.EqualFold(nome, c.propriedade) {
				continue
			}
			switch v := valor.(type) {
			case nil:
				c.valor = nil
			case string:
				texto := v
				c.valor = &texto
			default:
				return &erroDoJSON{"$." + nome, fmt.Sprintf("The JSON value could not be converted to System.String. Path: $.%s | %s.",
					nome, posicao(corpo, len(corpo)))}
			}
		}
	}
	return nil
}

// erroDeSintaxe imita a mensagem do Utf8JsonReader para o JSON que não abre. A do fim inesperado (o caso do
// contrato) sai igual, com a linha (contando do zero) e o byte na linha; as outras, na forma do .NET, mas a
// posição pode não bater byte a byte.
func erroDeSintaxe(corpo []byte, err error) *erroDoJSON {
	fim := bytes.TrimRight(corpo, " \t\r\n")
	var sintaxe *json.SyntaxError
	if errors.Is(err, io.ErrUnexpectedEOF) || (errors.As(err, &sintaxe) && int(sintaxe.Offset) >= len(fim)) {
		mensagem := "Expected start of a property name or value, but instead reached end of data."
		caminho := "$"
		if dentroDeTexto(fim) {
			mensagem = "Expected end of string, but instead reached end of data."
		} else if n := len(fim); n > 0 && fim[n-1] == ':' {
			caminho = "$." + ultimoNome(fim)
		} else if n > 0 && fim[n-1] != ',' && fim[n-1] != '{' && fim[n-1] != '[' {
			mensagem = "Expected depth to be zero at the end of the JSON payload. There is an open JSON object or array that should be closed."
		}
		return &erroDoJSON{caminho, fmt.Sprintf("%s Path: %s | %s.", mensagem, caminho, posicao(corpo, len(corpo)))}
	}
	deslocamento := len(corpo)
	if errors.As(err, &sintaxe) {
		deslocamento = int(sintaxe.Offset) - 1
	}
	if deslocamento < 0 {
		deslocamento = 0
	}
	caractere := "?"
	if deslocamento < len(corpo) {
		caractere = string(corpo[deslocamento])
	}
	return &erroDoJSON{"$", fmt.Sprintf("'%s' is an invalid start of a value. Path: $ | %s.", caractere, posicao(corpo, deslocamento))}
}

// posicao é o "LineNumber: L | BytePositionInLine: B" do .NET para o byte n do corpo (linha a partir de zero).
func posicao(corpo []byte, n int) string {
	if n > len(corpo) {
		n = len(corpo)
	}
	antes := corpo[:n]
	linha := bytes.Count(antes, []byte("\n"))
	inicioLinha := bytes.LastIndexByte(antes, '\n') + 1
	return fmt.Sprintf("LineNumber: %d | BytePositionInLine: %d", linha, n-inicioLinha)
}

func primeiroNaoBranco(corpo []byte) int {
	return len(corpo) - len(bytes.TrimLeft(corpo, " \t\r\n"))
}

// dentroDeTexto diz se o JSON acabou no meio de uma string (aspas abertas, sem contar as escapadas).
func dentroDeTexto(fim []byte) bool {
	dentro, escapado := false, false
	for _, b := range fim {
		switch {
		case escapado:
			escapado = false
		case b == '\\' && dentro:
			escapado = true
		case b == '"':
			dentro = !dentro
		}
	}
	return dentro
}

// ultimoNome é o nome da propriedade antes do ":" do fim ("…, \"login\":" → "login").
func ultimoNome(fim []byte) string {
	s := strings.TrimSpace(strings.TrimSuffix(string(fim), ":"))
	s = strings.TrimSuffix(s, `"`)
	if i := strings.LastIndex(s, `"`); i >= 0 {
		return s[i+1:]
	}
	return ""
}
