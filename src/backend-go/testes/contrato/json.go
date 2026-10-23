package contrato

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// decodificar lê um JSON para any com UseNumber: número vira json.Number (o texto exato, "10.50"), nunca float64.
// Assim "10.50" e "10.5" são diferentes, como no System.Text.Json, que preserva a escala do decimal.
func decodificar(dados []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(dados))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("sobrou conteúdo depois do JSON")
	}
	return v, nil
}

// paraAny passa um valor Go (struct, map) pelo JSON e volta como any, com os números em json.Number.
func paraAny(v any) (any, error) {
	dados, err := codificar(v)
	if err != nil {
		return nil, err
	}
	return decodificar(dados)
}

// codificar escreve JSON indentado, sem trocar <, > e & por < (os esperados ficam legíveis).
func codificar(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// comparar devolve as diferenças entre esperado e obtido, cada uma com o caminho em que aparece. Os dois vêm de
// decodificar (map[string]any, []any, string, json.Number, bool, nil).
//
// Regras: objeto tem de ter as mesmas chaves (null e ausente são diferentes); a ordem das chaves não importa, a
// dos itens de lista importa; número compara pelo texto ("1.50" ≠ "1.5", "1" ≠ "1.0").
func comparar(caminho string, esperado, obtido any) []string {
	switch e := esperado.(type) {
	case map[string]any:
		o, ok := obtido.(map[string]any)
		if !ok {
			return []string{diferenca(caminho, esperado, obtido)}
		}
		var difs []string
		for _, chave := range chavesOrdenadas(e, o) {
			ve, temE := e[chave]
			vo, temO := o[chave]
			sub := caminho + "/" + chave
			switch {
			case !temO:
				difs = append(difs, fmt.Sprintf("%s: esperado %s, obtido ausente", sub, mostrar(ve)))
			case !temE:
				difs = append(difs, fmt.Sprintf("%s: esperado ausente, obtido %s", sub, mostrar(vo)))
			default:
				difs = append(difs, comparar(sub, ve, vo)...)
			}
		}
		return difs
	case []any:
		o, ok := obtido.([]any)
		if !ok {
			return []string{diferenca(caminho, esperado, obtido)}
		}
		var difs []string
		if len(e) != len(o) {
			difs = append(difs, fmt.Sprintf("%s: esperado %d itens, obtido %d", rotulo(caminho), len(e), len(o)))
		}
		for i := range min(len(e), len(o)) {
			difs = append(difs, comparar(caminho+"/"+strconv.Itoa(i), e[i], o[i])...)
		}
		return difs
	case json.Number:
		if o, ok := obtido.(json.Number); !ok || o.String() != e.String() {
			return []string{diferenca(caminho, esperado, obtido)}
		}
	case string:
		if o, ok := obtido.(string); !ok || o != e {
			return []string{diferenca(caminho, esperado, obtido)}
		}
	case bool:
		if o, ok := obtido.(bool); !ok || o != e {
			return []string{diferenca(caminho, esperado, obtido)}
		}
	case nil:
		if obtido != nil {
			return []string{diferenca(caminho, esperado, obtido)}
		}
	default:
		return []string{fmt.Sprintf("%s: tipo inesperado no esperado (%T)", caminho, esperado)}
	}
	return nil
}

func diferenca(caminho string, esperado, obtido any) string {
	return fmt.Sprintf("%s: esperado %s, obtido %s", rotulo(caminho), mostrar(esperado), mostrar(obtido))
}

// rotulo é o caminho para a mensagem: a raiz aparece como "/".
func rotulo(caminho string) string {
	if caminho == "" {
		return "/"
	}
	return caminho
}

// mostrar escreve o valor como JSON curto, dizendo o tipo quando o texto sozinho engana (o número 5 e o texto "5").
func mostrar(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case json.Number:
		return "número " + x.String()
	case string:
		return "texto " + strconv.Quote(x)
	case bool:
		return strconv.FormatBool(x)
	case []any:
		return fmt.Sprintf("lista com %d itens", len(x))
	case map[string]any:
		chaves := make([]string, 0, len(x))
		for k := range x {
			chaves = append(chaves, k)
		}
		sort.Strings(chaves)
		return "objeto {" + strings.Join(chaves, ", ") + "}"
	}
	return fmt.Sprintf("%v", v)
}

func chavesOrdenadas(a, b map[string]any) []string {
	chaves := make([]string, 0, len(a)+len(b))
	for k := range a {
		chaves = append(chaves, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			chaves = append(chaves, k)
		}
	}
	slices.Sort(chaves)
	return chaves
}

// ponteiro segue um JSON Pointer (RFC 6901: "/itens/0/id", com ~1 para "/" e ~0 para "~") e devolve o valor e
// uma função que troca o valor naquele lugar.
func ponteiro(raiz any, p string) (valor any, trocar func(any), err error) {
	if p == "" {
		return raiz, nil, fmt.Errorf("ponteiro vazio")
	}
	if !strings.HasPrefix(p, "/") {
		return nil, nil, fmt.Errorf("ponteiro %q não começa com /", p)
	}
	partes := strings.Split(p[1:], "/")
	atual := raiz
	for i, parte := range partes {
		parte = strings.ReplaceAll(strings.ReplaceAll(parte, "~1", "/"), "~0", "~")
		ultimo := i == len(partes)-1
		switch x := atual.(type) {
		case map[string]any:
			v, ok := x[parte]
			if !ok {
				return nil, nil, fmt.Errorf("%s: não existe %q", p, parte)
			}
			if ultimo {
				return v, func(novo any) { x[parte] = novo }, nil
			}
			atual = v
		case []any:
			n, errN := strconv.Atoi(parte)
			if errN != nil || n < 0 || n >= len(x) {
				return nil, nil, fmt.Errorf("%s: índice %q fora da lista de %d itens", p, parte, len(x))
			}
			if ultimo {
				return x[n], func(novo any) { x[n] = novo }, nil
			}
			atual = x[n]
		default:
			return nil, nil, fmt.Errorf("%s: %q não é objeto nem lista", p, parte)
		}
	}
	return nil, nil, fmt.Errorf("%s: caminho inválido", p)
}

// comoTexto é o valor de uma variável capturada: o texto ou o número como veio.
func comoTexto(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case json.Number:
		return x.String(), nil
	case bool:
		return strconv.FormatBool(x), nil
	}
	return "", fmt.Errorf("só dá para capturar texto, número ou booleano, e o valor é %s", mostrar(v))
}
