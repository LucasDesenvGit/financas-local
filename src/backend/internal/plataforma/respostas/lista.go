package respostas

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// TipoDeItem é o tipo dos números de uma lista: o tamanho e se é enum (só número JSON de 0 a 255, como o CampoEnum).
type TipoDeItem struct {
	bits int
	enum bool
}

var (
	ItemLong = TipoDeItem{64, false}
	ItemInt  = TipoDeItem{32, false}
	ItemEnum = TipoDeItem{8, true} // os enums do projeto vão de 0 a 255
)

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

// CampoLista é uma lista de números (long, int ou enum), obrigatória se não for Opcional e, se Minimo ou Maximo não
// forem 0, com o tamanho mínimo e o máximo. O long e o int vêm em número ou em texto; null fica nil. O item que não
// converte sai com o índice ("$.ids[1]", contrato 07-transacoes, "emergência em lote").
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
		return l.Conversao(caminho)
	}
	lista := []int64{}
	for i := 0; l.Mais(); i++ {
		t, erro := l.Token()
		if erro != nil {
			return erro
		}
		n, ok := c.Item.ler(t)
		if !ok {
			return l.Conversao(caminho + "[" + strconv.Itoa(i) + "]")
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

// validarTamanhoDaLista confere se a lista veio (quando é obrigatória) e o tamanho dela.
func validarTamanhoDaLista(erros map[string][]string, propriedade string, nula bool, n int, opcional bool, minimo, maximo int) {
	switch {
	case nula && !opcional:
		erros[propriedade] = []string{Obrigatorio(propriedade)}
	case nula:
	case minimo > 0 && n < minimo:
		erros[propriedade] = []string{fmt.Sprintf("O campo %s precisa de pelo menos %s.", propriedade, itens(minimo))}
	case maximo > 0 && n > maximo:
		erros[propriedade] = []string{fmt.Sprintf("O campo %s pode ter até %s.", propriedade, itens(maximo))}
	}
}

// itens é "1 item" ou "n itens".
func itens(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d itens", n)
}
