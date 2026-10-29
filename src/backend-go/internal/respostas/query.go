package respostas

import (
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// par é um nome=valor da query, decodificado, na ordem da URL.
type par struct{ chave, valor string }

func paresDaQuery(r *http.Request) []par {
	var pares []par
	for bruto := range strings.SplitSeq(r.URL.RawQuery, "&") {
		chave, valor, _ := strings.Cut(bruto, "=")
		if c, err := url.QueryUnescape(chave); err == nil {
			chave = c
		}
		if v, err := url.QueryUnescape(valor); err == nil {
			valor = v
		}
		pares = append(pares, par{chave, valor})
	}
	return pares
}

// ParametroDaQuery é o [FromQuery] do ASP.NET: o nome não diferencia maiúsculas e vale o primeiro valor, na ordem em
// que vieram (com "Ano" e "ano" na mesma query, o primeiro dos dois).
func ParametroDaQuery(r *http.Request, nome string) (string, bool) {
	for _, p := range paresDaQuery(r) {
		if strings.EqualFold(p.chave, nome) {
			return p.valor, true
		}
	}
	return "", false
}

// ValoresDaQuery é o model binding de uma lista na query (List<T>): todos os valores do nome, na ordem
// (?ids=1&ids=2) ou, sem nenhum, o formato com índice (?ids[0]=1&ids[1]=2), do 0 até o primeiro índice que falta.
func ValoresDaQuery(r *http.Request, nome string) []string {
	pares := paresDaQuery(r)
	var valores []string
	for _, p := range pares {
		if strings.EqualFold(p.chave, nome) {
			valores = append(valores, p.valor)
		}
	}
	if len(valores) > 0 {
		return valores
	}
	prefixo := strings.ToLower(nome) + "["
	indexados := map[int]string{}
	for _, p := range pares {
		chave := strings.ToLower(p.chave)
		if !strings.HasPrefix(chave, prefixo) || !strings.HasSuffix(chave, "]") {
			continue
		}
		if i, err := strconv.Atoi(chave[len(prefixo) : len(chave)-1]); err == nil {
			if _, repetido := indexados[i]; !repetido {
				indexados[i] = p.valor
			}
		}
	}
	for i := 0; ; i++ {
		v, ok := indexados[i]
		if !ok {
			return valores
		}
		valores = append(valores, v)
	}
}

// MensagemDaQuery é a do 400 automático para o valor da query que não vira o tipo do parâmetro (ou do item de uma
// lista): "The value 'abc' is not valid." (contratos 02-cambio, 04-contas, 06-regras e 07-transacoes). Vazio num tipo
// que não aceita null é MensagemDeValorInvalido.
func MensagemDaQuery(valor string) string {
	if strings.TrimSpace(valor) == "" {
		return MensagemDeValorInvalido(valor)
	}
	return "The value '" + valor + "' is not valid."
}

// MensagemDaPropriedade é a mesma mensagem para uma propriedade de um objeto lido da query ([FromQuery] FiltroDto):
// ela cita o nome da propriedade ("The value 'um' is not valid for Pagina.", contrato 07-transacoes).
func MensagemDaPropriedade(valor, propriedade string) string {
	if strings.TrimSpace(valor) == "" {
		return MensagemDeValorInvalido(valor)
	}
	return "The value '" + valor + "' is not valid for " + propriedade + "."
}

// MensagemDeValorInvalido é a do valor vazio num tipo que não aceita null e a do número que não existe no enum
// ("The value '9' is invalid.", contrato 07-transacoes).
func MensagemDeValorInvalido(valor string) string { return "The value '" + valor + "' is invalid." }

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

// LongDaQuery é o Int64Converter: as mesmas regras do InteiroDaQuery, em 64 bits (0xFFFFFFFFFFFFFFFF é -1).
func LongDaQuery(valor string) (int64, bool) {
	texto := strings.TrimSpace(valor)
	minusculo := strings.ToLower(texto)
	for _, prefixo := range []string{"0x", "&h", "#"} {
		if strings.HasPrefix(minusculo, prefixo) {
			n, err := strconv.ParseUint(texto[len(prefixo):], 16, 64)
			switch {
			case err != nil:
				return 0, false
			case n <= math.MaxInt64:
				return int64(n), true
			}
			return int64(n-math.MaxInt64-1) + math.MinInt64, true // complemento de dois
		}
	}
	n, err := strconv.ParseInt(texto, 10, 64)
	return n, err == nil
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

// EnumDaQuery é o EnumTypeModelBinder do ASP.NET na query: o nome (sem diferença de maiúsculas; no mapa, em
// minúsculas) ou o número, e vários separados por vírgula viram a combinação (o OR dos valores). converte diz se o texto
// vira o enum (senão, MensagemDaQuery); definido, se o valor existe nele (senão, MensagemDeValorInvalido: o binder
// confere o Enum.IsDefined, e "9" num enum de 1 a 3 é 400).
func EnumDaQuery(valor string, nomes map[string]int64) (v int64, converte, definido bool) {
	for p := range strings.SplitSeq(valor, ",") {
		p = strings.TrimSpace(p)
		if n, ok := nomes[strings.ToLower(p)]; ok {
			v |= n
			continue
		}
		n, err := strconv.ParseUint(p, 10, 8) // os enums do projeto são ": byte"
		if err != nil {
			return 0, false, false
		}
		v |= int64(n)
	}
	for _, n := range nomes {
		if n == v {
			return v, true, true
		}
	}
	return v, true, false
}

// DataDaQuery é o DateOnly do model binding, que lê a query na cultura invariante: "2026-10-15" (o que o front-end
// manda) e o formato americano "10/15/2026". Outros formatos que o .NET aceita (nome do mês por extenso) ficam de
// fora: o front-end nunca os manda.
func DataDaQuery(texto string) (tipos.Data, bool) {
	texto = strings.TrimSpace(texto)
	for _, formato := range []string{"2006-01-02", "1/2/2006", "2006/1/2"} {
		if t, err := time.Parse(formato, texto); err == nil {
			return tipos.DataDe(t), true
		}
	}
	return tipos.Data{}, false
}
