package parametros

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
)

// Nomes do C# que aparecem nas mensagens do System.Text.Json.
const (
	tipoDto        = "Financas.Api.Dtos.SalvarParametrosDto"
	tipoDicionario = "System.Collections.Generic.Dictionary`2[System.String,System.Int32]"
	tipoInt32      = "System.Int32"
	caminhoValores = "$.valores" // o nome da propriedade no caminho é o do contrato (camelCase), não o que veio
)

// corpoSalvar é o SalvarParametrosDto lido do JSON.
type corpoSalvar struct {
	nulo    bool    // o JSON inteiro era "null": o ASP.NET trata como corpo vazio
	valores []Valor // nil quando "valores" faltou ou veio null ([Required] recusa)
}

// lerSalvar lê o corpo como o System.Text.Json com as opções web do ASP.NET, na ordem do texto: a primeira coisa
// errada é a que vira o erro (como o leitor do .NET, que para no primeiro problema).
//
//   - nomes de propriedade sem diferença de maiúsculas; propriedades a mais são ignoradas; a última repetida vale;
//   - Valores é um Dictionary<string, int>: a chave repetida fica com o último valor, no lugar da primeira;
//   - o valor é número inteiro de 32 bits, ou texto com um (AllowReadingFromString): "12" vale, 1.5 e "1,5" não.
func lerSalvar(corpo []byte) (corpoSalvar, *respostas.ErroJSON) {
	l := respostas.NovoLeitorJSON(corpo)
	primeiro, erro := l.Token()
	if erro != nil {
		return corpoSalvar{}, erro
	}
	if primeiro == nil {
		return corpoSalvar{nulo: true}, l.Fim()
	}
	if primeiro != json.Delim('{') {
		return corpoSalvar{}, l.Conversao("$", tipoDto)
	}

	var resultado corpoSalvar
	for l.Mais() {
		nomeToken, erro := l.Token()
		if erro != nil {
			return corpoSalvar{}, erro
		}
		valor, erro := l.Token()
		if erro != nil {
			return corpoSalvar{}, erro
		}
		if nome, _ := nomeToken.(string); !strings.EqualFold(nome, "valores") {
			if erro := l.Pular(valor); erro != nil {
				return corpoSalvar{}, erro
			}
			continue
		}
		valores, erro := lerValores(l, valor)
		if erro != nil {
			return corpoSalvar{}, erro
		}
		resultado.valores = valores
	}
	if _, erro := l.Token(); erro != nil { // o "}"
		return corpoSalvar{}, erro
	}
	return resultado, l.Fim()
}

// lerValores lê o valor de "valores", que começa no token inicio: null (nil) ou um objeto de chave e inteiro.
func lerValores(l *respostas.LeitorJSON, inicio json.Token) ([]Valor, *respostas.ErroJSON) {
	if inicio == nil {
		return nil, nil
	}
	if inicio != json.Delim('{') {
		return nil, l.Conversao(caminhoValores, tipoDicionario)
	}
	valores := []Valor{}
	posicao := map[string]int{}
	for l.Mais() {
		chaveToken, erro := l.Token()
		if erro != nil {
			return nil, erro
		}
		chave, _ := chaveToken.(string)
		valorToken, erro := l.Token()
		if erro != nil {
			return nil, erro
		}
		n, ok := inteiro(valorToken)
		if !ok {
			return nil, l.Conversao(respostas.CaminhoDaChave(caminhoValores, chave), tipoInt32)
		}
		if i, repetida := posicao[chave]; repetida {
			valores[i].Valor = n
			continue
		}
		posicao[chave] = len(valores)
		valores = append(valores, Valor{Chave: chave, Valor: n})
	}
	if _, erro := l.Token(); erro != nil { // o "}"
		return nil, erro
	}
	return valores, nil
}

// inteiro é o Int32Converter com AllowReadingFromString: número JSON que é inteiro de 32 bits (o TryGetInt32 do
// Utf8JsonReader recusa 1.5, 1e2 e o que passa do limite), ou texto só com o número, com sinal opcional ("+12",
// "012"); sem espaços. null, true, false, objeto e lista não são inteiro.
func inteiro(t json.Token) (int, bool) {
	var texto string
	switch v := t.(type) {
	case json.Number:
		texto = string(v)
	case string:
		texto = v
		if sem, ok := strings.CutPrefix(texto, "+"); ok {
			if strings.HasPrefix(sem, "-") {
				return 0, false // um sinal só
			}
			texto = sem
		}
	default:
		return 0, false
	}
	digitos := strings.TrimPrefix(texto, "-")
	if digitos == "" {
		return 0, false
	}
	for _, c := range digitos {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(texto, 10, 32)
	if err != nil {
		return 0, false
	}
	return int(n), true
}
