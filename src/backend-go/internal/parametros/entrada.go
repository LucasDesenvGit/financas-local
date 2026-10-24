package parametros

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	l := leitor{corpo: corpo, dec: json.NewDecoder(bytes.NewReader(corpo))}
	l.dec.UseNumber()

	primeiro, err := l.dec.Token()
	if err != nil {
		return corpoSalvar{}, l.sintaxe(err)
	}
	if primeiro == nil {
		return corpoSalvar{nulo: true}, l.fim()
	}
	if primeiro != json.Delim('{') {
		return corpoSalvar{}, l.conversao("$", tipoDto)
	}

	var resultado corpoSalvar
	for l.dec.More() {
		nomeToken, err := l.dec.Token()
		if err != nil {
			return corpoSalvar{}, l.sintaxe(err)
		}
		nome, _ := nomeToken.(string)
		if !strings.EqualFold(nome, "valores") {
			if err := l.pular(); err != nil {
				return corpoSalvar{}, l.sintaxe(err)
			}
			continue
		}
		valores, erroJSON := l.lerValores()
		if erroJSON != nil {
			return corpoSalvar{}, erroJSON
		}
		resultado.valores = valores
	}
	if _, err := l.dec.Token(); err != nil { // o "}"
		return corpoSalvar{}, l.sintaxe(err)
	}
	return resultado, l.fim()
}

type leitor struct {
	corpo []byte
	dec   *json.Decoder
}

// lerValores lê o valor de "valores": null (nil) ou um objeto de chave e inteiro.
func (l *leitor) lerValores() ([]Valor, *respostas.ErroJSON) {
	t, err := l.dec.Token()
	if err != nil {
		return nil, l.sintaxe(err)
	}
	if t == nil {
		return nil, nil
	}
	if t != json.Delim('{') {
		return nil, l.conversao(caminhoValores, tipoDicionario)
	}
	valores := []Valor{}
	posicao := map[string]int{}
	for l.dec.More() {
		chaveToken, err := l.dec.Token()
		if err != nil {
			return nil, l.sintaxe(err)
		}
		chave, _ := chaveToken.(string)
		valorToken, err := l.dec.Token()
		if err != nil {
			return nil, l.sintaxe(err)
		}
		n, ok := inteiro(valorToken)
		if !ok {
			return nil, l.conversao(respostas.CaminhoDaChave(caminhoValores, chave), tipoInt32)
		}
		if i, repetida := posicao[chave]; repetida {
			valores[i].Valor = n
			continue
		}
		posicao[chave] = len(valores)
		valores = append(valores, Valor{Chave: chave, Valor: n})
	}
	if _, err := l.dec.Token(); err != nil { // o "}"
		return nil, l.sintaxe(err)
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

// pular lê e descarta um valor inteiro (com o que tiver dentro), conferindo a sintaxe.
func (l *leitor) pular() error {
	profundidade := 0
	for {
		t, err := l.dec.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('{'), json.Delim('['):
			profundidade++
		case json.Delim('}'), json.Delim(']'):
			profundidade--
		}
		if profundidade == 0 {
			return nil
		}
	}
}

// fim confere que não há nada depois do valor (o leitor do .NET recusa).
func (l *leitor) fim() *respostas.ErroJSON {
	if _, err := l.dec.Token(); !errors.Is(err, io.EOF) {
		return l.sintaxe(err)
	}
	return nil
}

// conversao é o "The JSON value could not be converted to ..." com a posição logo depois do token lido, como o
// BytePositionInLine do Utf8JsonReader.
func (l *leitor) conversao(caminho, tipo string) *respostas.ErroJSON {
	return &respostas.ErroJSON{Caminho: caminho, Mensagem: fmt.Sprintf("The JSON value could not be converted to %s. Path: %s | %s.",
		tipo, caminho, respostas.Posicao(l.corpo, int(l.dec.InputOffset())))}
}

// sintaxe é o erro de JSON que não abre, com as mensagens do pacote respostas (as mesmas do login, #75).
func (l *leitor) sintaxe(err error) *respostas.ErroJSON {
	var qualquer any
	if errUnmarshal := json.Unmarshal(l.corpo, &qualquer); errUnmarshal != nil {
		err = errUnmarshal
	}
	return respostas.ErroDeSintaxe(l.corpo, err)
}
