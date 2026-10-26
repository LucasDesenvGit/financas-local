package respostas

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// LeitorJSON lê o corpo token a token, na ordem do texto, como o Utf8JsonReader do System.Text.Json: a primeira
// coisa errada é a que vira o erro, e a posição do erro é a do fim do token lido (o BytePositionInLine do .NET).
type LeitorJSON struct {
	corpo []byte
	dec   *json.Decoder
}

func NovoLeitorJSON(corpo []byte) *LeitorJSON {
	dec := json.NewDecoder(bytes.NewReader(corpo))
	dec.UseNumber()
	return &LeitorJSON{corpo: corpo, dec: dec}
}

// Token é o próximo token (json.Delim, string, json.Number, bool ou nil); o erro já vem no formato do .NET.
func (l *LeitorJSON) Token() (json.Token, *ErroJSON) {
	t, err := l.dec.Token()
	if err != nil {
		return nil, ErroDeSintaxe(l.corpo)
	}
	return t, nil
}

// Mais diz se o objeto ou a lista aberta ainda tem itens.
func (l *LeitorJSON) Mais() bool { return l.dec.More() }

// Pular descarta o valor que começa no token já lido (com o que tiver dentro), conferindo a sintaxe.
func (l *LeitorJSON) Pular(inicio json.Token) *ErroJSON {
	profundidade := 0
	t := inicio
	for {
		switch t {
		case json.Delim('{'), json.Delim('['):
			profundidade++
		case json.Delim('}'), json.Delim(']'):
			profundidade--
		}
		if profundidade == 0 {
			return nil
		}
		var erro *ErroJSON
		if t, erro = l.Token(); erro != nil {
			return erro
		}
	}
}

// Fim confere que não há nada depois do valor (o leitor do .NET recusa).
func (l *LeitorJSON) Fim() *ErroJSON {
	if _, err := l.dec.Token(); !errors.Is(err, io.EOF) {
		return ErroDeSintaxe(l.corpo)
	}
	return nil
}

// Conversao é o "The JSON value could not be converted to ..." com a posição logo depois do token lido.
func (l *LeitorJSON) Conversao(caminho, tipo string) *ErroJSON {
	return &ErroJSON{Caminho: caminho, Mensagem: fmt.Sprintf("The JSON value could not be converted to %s. Path: %s | %s.",
		tipo, caminho, Posicao(l.corpo, int(l.dec.InputOffset())))}
}

// Campo é uma propriedade de um DTO lido por LerObjeto.
type Campo interface {
	propriedade() string
	// ler guarda o valor do token; false se o token não é do tipo do campo.
	ler(t json.Token) bool
	// validar acrescenta as mensagens dos atributos de validação ([Required], [StringLength]).
	validar(erros map[string][]string)
}

// CampoTexto é um parâmetro string de um DTO com [StringLength(maximo, MinimumLength = minimo)] e, se não for
// Opcional, [Required].
type CampoTexto struct {
	Propriedade string // o nome da propriedade no C#, que é a chave em "errors" ("Login", "SenhaAtual")
	Minimo      int
	Maximo      int
	Opcional    bool // sem [Required] (string? no C#)
	Valor       *string
}

func (c *CampoTexto) propriedade() string { return c.Propriedade }

func (c *CampoTexto) ler(t json.Token) bool {
	switch v := t.(type) {
	case nil:
		c.Valor = nil
	case string:
		c.Valor = &v
	default:
		return false
	}
	return true
}

func (c *CampoTexto) validar(erros map[string][]string) {
	var lista []string
	if !c.Opcional && (c.Valor == nil || strings.TrimSpace(*c.Valor) == "") {
		// [Required] recusa nulo, vazio e só espaços (AllowEmptyStrings = false).
		lista = append(lista, Obrigatorio(c.Propriedade))
	}
	if c.Valor != nil {
		// [StringLength] conta caracteres UTF-16, como string.Length do C#; nulo passa (quem recusa é o Required).
		if n := TamanhoUtf16(*c.Valor); n > c.Maximo || n < c.Minimo {
			if c.Minimo > 0 {
				lista = append(lista, fmt.Sprintf("The field %s must be a string with a minimum length of %d and a maximum length of %d.",
					c.Propriedade, c.Minimo, c.Maximo))
			} else {
				lista = append(lista, fmt.Sprintf("The field %s must be a string with a maximum length of %d.", c.Propriedade, c.Maximo))
			}
		}
	}
	if len(lista) > 0 {
		erros[c.Propriedade] = lista
	}
}

// CampoBool é um parâmetro bool (não anulável) de um DTO: sem a propriedade no JSON, fica false.
type CampoBool struct {
	Propriedade string
	Valor       bool
}

func (c *CampoBool) propriedade() string { return c.Propriedade }

func (c *CampoBool) ler(t json.Token) bool {
	v, ok := t.(bool)
	c.Valor = v
	return ok
}

func (c *CampoBool) validar(map[string][]string) {}

// LerObjeto faz o model binding e a validação do [ApiController] para um record do C# com construtor (o
// System.Text.Json com as opções web do ASP.NET): devolve false se já respondeu (415, 413 ou 400).
//
//   - parametro é o nome do parâmetro da action ("dto"), que aparece quando o corpo não abre;
//   - tipoDto é o nome completo do tipo no C#: o erro de tipo num campo de um record com construtor cita o tipo do
//     record, e não o do campo (o contrato 17-erros mostra: "could not be converted to ...ErroFrontEndDto");
//   - nomes sem diferença de maiúsculas, propriedades a mais ignoradas, a última repetida vale.
func LerObjeto(w http.ResponseWriter, r *http.Request, parametro, tipoDto string, campos ...Campo) bool {
	corpo, ok := LerCorpo(w, r, parametro)
	if !ok {
		return false
	}
	nulo, erroJSON := lerObjeto(corpo, tipoDto, campos)
	switch {
	case erroJSON != nil:
		Validacao(w, map[string][]string{erroJSON.Caminho: {erroJSON.Mensagem}, parametro: {Obrigatorio(parametro)}})
		return false
	case nulo:
		CorpoVazio(w, parametro)
		return false
	}
	erros := map[string][]string{}
	for _, c := range campos {
		c.validar(erros)
	}
	if len(erros) > 0 {
		Validacao(w, erros)
		return false
	}
	return true
}

// LerTextos é o LerObjeto de um DTO só com strings obrigatórias ou opcionais (CampoTexto).
func LerTextos(w http.ResponseWriter, r *http.Request, parametro, tipoDto string, campos ...*CampoTexto) bool {
	todos := make([]Campo, len(campos))
	for i, c := range campos {
		todos[i] = c
	}
	return LerObjeto(w, r, parametro, tipoDto, todos...)
}

// lerObjeto lê o objeto e devolve se o JSON inteiro era "null" (o ASP.NET trata como corpo vazio).
func lerObjeto(corpo []byte, tipoDto string, campos []Campo) (bool, *ErroJSON) {
	l := NovoLeitorJSON(corpo)
	primeiro, erro := l.Token()
	if erro != nil {
		return false, erro
	}
	if primeiro == nil {
		return true, l.Fim()
	}
	if primeiro != json.Delim('{') {
		return false, l.Conversao("$", tipoDto)
	}
	for l.Mais() {
		nomeToken, erro := l.Token()
		if erro != nil {
			return false, erro
		}
		nome, _ := nomeToken.(string)
		valor, erro := l.Token()
		if erro != nil {
			return false, erro
		}
		campo := campoDe(campos, nome)
		if campo == nil {
			if erro := l.Pular(valor); erro != nil {
				return false, erro
			}
			continue
		}
		if !campo.ler(valor) {
			return false, l.Conversao(CaminhoDaChave("$", nome), tipoDto)
		}
	}
	if _, erro := l.Token(); erro != nil { // o "}"
		return false, erro
	}
	return false, l.Fim()
}

func campoDe(campos []Campo, nome string) Campo {
	for _, c := range campos {
		if strings.EqualFold(nome, c.propriedade()) {
			return c
		}
	}
	return nil
}
