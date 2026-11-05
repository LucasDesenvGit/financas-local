package respostas

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// TipoDeItem é o tipo dos números de uma lista do C#: o tamanho, se é enum (só número JSON de 0 a 255, como o
// CampoEnum) e o nome nas mensagens.
type TipoDeItem struct {
	bits int
	enum bool
	nome string
}

var (
	ItemLong = TipoDeItem{64, false, "System.Int64"}
	ItemInt  = TipoDeItem{32, false, "System.Int32"}
)

// ItemEnum é o item de uma lista de enum (": byte"), com o nome completo do enum no C#.
func ItemEnum(nome string) TipoDeItem { return TipoDeItem{8, true, nome} }

func (t TipoDeItem) ler(token json.Token) (int64, bool) {
	if !t.enum {
		return inteiroDoTokenEm(token, t.bits)
	}
	e := CampoEnum{}
	if !e.ler(token) {
		return 0, false
	}
	return *e.Valor, true
}

// CampoLista é uma lista de números de um DTO (List<long>, List<int>, List<enum>), com [Required] (se não for
// Opcional) e, se Minimo ou Maximo não forem 0, o [MinLength] e o [MaxLength]. O long e o int vêm em número ou em
// texto (AllowReadingFromString); null fica nil. O valor que não é lista cita a lista; o item que não converte cita o
// tipo do item e o índice ("$.ids[1]", contrato 07-transacoes, "emergência em lote").
type CampoLista struct {
	Propriedade    string
	Item           TipoDeItem
	Opcional       bool
	Minimo, Maximo int
	Valor          []int64
}

func (c *CampoLista) propriedade() string { return c.Propriedade }

func (c *CampoLista) lerValor(l *LeitorJSON, inicio json.Token, caminho string) *ErroJSON {
	if inicio == nil {
		c.Valor = nil
		return nil
	}
	if inicio != json.Delim('[') {
		return l.Conversao(caminho, "System.Collections.Generic.List`1["+c.Item.nome+"]")
	}
	lista := []int64{}
	for i := 0; l.Mais(); i++ {
		t, erro := l.Token()
		if erro != nil {
			return erro
		}
		n, ok := c.Item.ler(t)
		if !ok {
			return l.Conversao(caminho+"["+strconv.Itoa(i)+"]", c.Item.nome)
		}
		lista = append(lista, n)
	}
	if _, erro := l.Token(); erro != nil { // o "]"
		return erro
	}
	c.Valor = lista
	return nil
}

func (c *CampoLista) validar(erros map[string][]string) {
	validarTamanhoDaLista(erros, c.Propriedade, c.Valor == nil, len(c.Valor), c.Opcional, c.Minimo, c.Maximo)
}

// validarTamanhoDaLista é o [Required], o [MinLength] e o [MaxLength] de uma lista.
func validarTamanhoDaLista(erros map[string][]string, propriedade string, nula bool, n int, opcional bool, minimo, maximo int) {
	switch {
	case nula && !opcional:
		erros[propriedade] = []string{Obrigatorio(propriedade)}
	case nula:
	case minimo > 0 && n < minimo:
		erros[propriedade] = []string{fmt.Sprintf("The field %s must be a string or array type with a minimum length of '%d'.",
			propriedade, minimo)}
	case maximo > 0 && n > maximo:
		erros[propriedade] = []string{fmt.Sprintf("The field %s must be a string or array type with a maximum length of '%d'.",
			propriedade, maximo)}
	}
}
