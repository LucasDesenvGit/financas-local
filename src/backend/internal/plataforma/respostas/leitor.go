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

// LeitorJSON lê o corpo token a token, na ordem do texto: a primeira coisa errada é a que vira o erro.
type LeitorJSON struct {
	dec *json.Decoder
}

func NovoLeitorJSON(corpo []byte) *LeitorJSON {
	dec := json.NewDecoder(bytes.NewReader(corpo))
	dec.UseNumber()
	return &LeitorJSON{dec: dec}
}

// erroDeSintaxe é o erro do corpo que não é JSON válido: uma mensagem só, para o pedido inteiro.
func erroDeSintaxe() *ErroJSON { return &ErroJSON{Caminho: "$", Mensagem: MensagemJSONInvalido} }

// Token é o próximo token (json.Delim, string, json.Number, bool ou nil).
func (l *LeitorJSON) Token() (json.Token, *ErroJSON) {
	t, err := l.dec.Token()
	if err != nil {
		return nil, erroDeSintaxe()
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

// Fim confere que não há nada depois do valor.
func (l *LeitorJSON) Fim() *ErroJSON {
	if _, err := l.dec.Token(); !errors.Is(err, io.EOF) {
		return erroDeSintaxe()
	}
	return nil
}

// Conversao é o erro do valor que não é do tipo do campo (texto num número, lista num objeto). O caminho "$" é o
// pedido inteiro; os outros saem sem o "$." na mensagem ("valor", "itens[0].id").
func (l *LeitorJSON) Conversao(caminho string) *ErroJSON {
	if caminho == "$" {
		return &ErroJSON{Caminho: caminho, Mensagem: MensagemJSONInvalido}
	}
	return &ErroJSON{Caminho: caminho, Mensagem: fmt.Sprintf("O valor de %s não é do tipo esperado.", strings.TrimPrefix(caminho, "$."))}
}

// Campo é uma propriedade de um DTO lido por LerObjeto: um campoSimples (o valor é um token só) ou um campoComposto
// (lista ou objeto).
type Campo interface {
	propriedade() string
	// validar acrescenta as mensagens dos atributos de validação ([Required], [StringLength]).
	validar(erros map[string][]string)
}

// campoSimples é o campo cujo valor é um token só (texto, número, bool ou null).
type campoSimples interface {
	Campo
	// ler guarda o valor do token; false se o token não é do tipo do campo.
	ler(t json.Token) bool
}

// campoComposto é o campo cujo valor é uma lista ou um objeto: ele mesmo lê o valor inteiro, a partir do primeiro
// token.
type campoComposto interface {
	Campo
	lerValor(l *LeitorJSON, inicio json.Token, caminho string) *ErroJSON
}

// CampoTexto é um parâmetro string de um DTO com [StringLength(maximo, MinimumLength = minimo)] e, se não for
// Opcional, [Required].
type CampoTexto struct {
	Propriedade string // o nome do campo, que é a chave em "errors" ("Login", "SenhaAtual")
	Minimo      int
	Maximo      int
	Opcional    bool    // sem [Required] (string? no C#)
	Padrao      *Padrao // [RegularExpression], se houver
	// MensagemDoTamanho é a mensagem própria do tamanho, quando o campo dá uma ("O texto da cobrança pode ter até 100
	// caracteres."); vazio, vale a mensagem padrão.
	MensagemDoTamanho string
	Valor             *string
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
			if c.MensagemDoTamanho != "" {
				lista = append(lista, c.MensagemDoTamanho)
			} else if c.Minimo > 0 {
				lista = append(lista, fmt.Sprintf("O campo %s tem de ter de %d a %d caracteres.", c.Propriedade, c.Minimo, c.Maximo))
			} else {
				lista = append(lista, fmt.Sprintf("O campo %s pode ter até %d caracteres.", c.Propriedade, c.Maximo))
			}
		}
		if c.Padrao != nil && *c.Valor != "" && !c.Padrao.re.MatchString(*c.Valor) {
			lista = append(lista, c.Padrao.Mensagem(c.Propriedade))
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

// LerObjeto lê o corpo JSON nos campos e confere cada um (obrigatório, tamanho, faixa): devolve false se já respondeu
// (415, 413 ou 400). Os nomes não diferenciam maiúsculas, propriedade a mais é ignorada e, repetida, vale a última.
func LerObjeto(w http.ResponseWriter, r *http.Request, campos ...Campo) bool {
	corpo, ok := LerCorpo(w, r)
	if !ok {
		return false
	}
	nulo, erroJSON := lerObjeto(corpo, campos)
	switch {
	case erroJSON != nil:
		Validacao(w, map[string][]string{erroJSON.Caminho: {erroJSON.Mensagem}})
		return false
	case nulo:
		CorpoVazio(w)
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

// LerTextos é o LerObjeto de um corpo só com textos (CampoTexto).
func LerTextos(w http.ResponseWriter, r *http.Request, campos ...*CampoTexto) bool {
	todos := make([]Campo, len(campos))
	for i, c := range campos {
		todos[i] = c
	}
	return LerObjeto(w, r, todos...)
}

// lerObjeto lê o objeto e devolve se o JSON inteiro era "null" (que vale como corpo vazio).
func lerObjeto(corpo []byte, campos []Campo) (bool, *ErroJSON) {
	l := NovoLeitorJSON(corpo)
	primeiro, erro := l.Token()
	if erro != nil {
		return false, erro
	}
	if primeiro == nil {
		return true, l.Fim()
	}
	if primeiro != json.Delim('{') {
		return false, l.Conversao("$")
	}
	if erro := lerPropriedades(l, "$", campos); erro != nil {
		return false, erro
	}
	return false, l.Fim()
}

// lerPropriedades lê os pares nome e valor de um objeto já aberto, até o "}" (que consome). caminho é o do objeto ("$",
// "$.filtro", "$.itens[0]").
func lerPropriedades(l *LeitorJSON, caminho string, campos []Campo) *ErroJSON {
	for l.Mais() {
		nomeToken, erro := l.Token()
		if erro != nil {
			return erro
		}
		nome, _ := nomeToken.(string)
		valor, erro := l.Token()
		if erro != nil {
			return erro
		}
		campo := campoDe(campos, nome)
		if campo == nil {
			if erro := l.Pular(valor); erro != nil {
				return erro
			}
			continue
		}
		switch c := campo.(type) {
		case campoComposto:
			if erro := c.lerValor(l, valor, CaminhoDaChave(caminho, nome)); erro != nil {
				return erro
			}
		case campoSimples:
			if !c.ler(valor) {
				return l.Conversao(CaminhoDaChave(caminho, nome))
			}
		}
	}
	_, erro := l.Token() // o "}"
	return erro
}

func campoDe(campos []Campo, nome string) Campo {
	for _, c := range campos {
		if strings.EqualFold(nome, c.propriedade()) {
			return c
		}
	}
	return nil
}
