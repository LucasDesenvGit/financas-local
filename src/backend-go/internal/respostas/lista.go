package respostas

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// tipoListaLong é o nome do List<long> nas mensagens do System.Text.Json.
const tipoListaLong = "System.Collections.Generic.List`1[System.Int64]"

// CampoListaLong é um List<long> de um DTO, com [Required] e, se Minimo ou Maximo não forem 0, o [MinLength] e o
// [MaxLength]. Cada item é um long em número ou em texto (AllowReadingFromString); null fica nil. O erro de um item
// cita a lista e o índice ("$.ids[1]").
type CampoListaLong struct {
	Propriedade    string
	Minimo, Maximo int
	Valor          []int64
}

func (c *CampoListaLong) propriedade() string { return c.Propriedade }

// ler só é chamado para o null: a lista em si vem por lerValor.
func (c *CampoListaLong) ler(t json.Token) bool {
	c.Valor = nil
	return t == nil
}

func (c *CampoListaLong) lerValor(l *LeitorJSON, inicio json.Token, caminho string) *ErroJSON {
	if inicio == nil {
		c.Valor = nil
		return nil
	}
	if inicio != json.Delim('[') {
		return l.Conversao(caminho, tipoListaLong)
	}
	lista := []int64{}
	for i := 0; l.Mais(); i++ {
		t, erro := l.Token()
		if erro != nil {
			return erro
		}
		n, ok := inteiroDoTokenEm(t, 64)
		if !ok {
			return l.Conversao(caminho+"["+strconv.Itoa(i)+"]", tipoListaLong)
		}
		lista = append(lista, n)
	}
	if _, erro := l.Token(); erro != nil { // o "]"
		return erro
	}
	c.Valor = lista
	return nil
}

func (c *CampoListaLong) validar(erros map[string][]string) {
	switch {
	case c.Valor == nil:
		erros[c.Propriedade] = []string{Obrigatorio(c.Propriedade)}
	case c.Minimo > 0 && len(c.Valor) < c.Minimo:
		erros[c.Propriedade] = []string{fmt.Sprintf("The field %s must be a string or array type with a minimum length of '%d'.",
			c.Propriedade, c.Minimo)}
	case c.Maximo > 0 && len(c.Valor) > c.Maximo:
		erros[c.Propriedade] = []string{fmt.Sprintf("The field %s must be a string or array type with a maximum length of '%d'.",
			c.Propriedade, c.Maximo)}
	}
}
