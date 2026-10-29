package respostas

import (
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ParametroDaQuery é o [FromQuery] do ASP.NET: o nome não diferencia maiúsculas e vale o primeiro valor, na ordem em
// que vieram (com "Ano" e "ano" na mesma query, o primeiro dos dois).
func ParametroDaQuery(r *http.Request, nome string) (string, bool) {
	for par := range strings.SplitSeq(r.URL.RawQuery, "&") {
		chave, valor, _ := strings.Cut(par, "=")
		if c, err := url.QueryUnescape(chave); err == nil {
			chave = c
		}
		if !strings.EqualFold(chave, nome) {
			continue
		}
		if v, err := url.QueryUnescape(valor); err == nil {
			valor = v
		}
		return valor, true
	}
	return "", false
}

// MensagemDaQuery é a do 400 automático para o valor da query que não vira o tipo do parâmetro: "The value 'abc' is
// not valid." (contratos 02-cambio, 04-contas e 06-regras). Vazio num tipo que não aceita null é "The value ” is
// invalid.".
func MensagemDaQuery(valor string) string {
	if strings.TrimSpace(valor) == "" {
		return "The value '" + valor + "' is invalid."
	}
	return "The value '" + valor + "' is not valid."
}

// InteiroDaQuery é o Int32Converter que o model binding usa (na cultura invariante): espaços nas pontas, sinal e,
// com "0x", "#" ou "&h" na frente, hexadecimal ("0x7EA" é 2026). O gabarito é o testdata/query.json.
func InteiroDaQuery(valor string) (int32, bool) {
	texto := strings.TrimSpace(valor)
	minusculo := strings.ToLower(texto)
	for _, prefixo := range []string{"0x", "&h", "#"} {
		if strings.HasPrefix(minusculo, prefixo) {
			n, err := strconv.ParseUint(texto[len(prefixo):], 16, 32)
			switch {
			case err != nil:
				return 0, false
			case n <= math.MaxInt32:
				return int32(n), true
			}
			// Convert.ToInt32(texto, 16) lê os 32 bits em complemento de dois: FFFFFFFF é -1, 80000000 é o menor int.
			return int32(int64(n) - 1<<32), true
		}
	}
	n, err := strconv.ParseInt(texto, 10, 32)
	return int32(n), err == nil
}

// BoolDaQuery é o BooleanConverter: "true" ou "false", sem diferença de maiúsculas e com espaços nas pontas.
func BoolDaQuery(valor string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(valor)) {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}
