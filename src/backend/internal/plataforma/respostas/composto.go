package respostas

import (
	"encoding/json"
	"strconv"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// CampoLong é um long (não anulável) de um record: número JSON inteiro ou o mesmo número em texto; null é erro.
type CampoLong struct {
	Propriedade string
	Valor       int64
}

func (c *CampoLong) propriedade() string { return c.Propriedade }

func (c *CampoLong) ler(t json.Token) bool {
	n, ok := inteiroDoTokenEm(t, 64)
	c.Valor = n
	return ok
}

func (c *CampoLong) validar(map[string][]string) {}

// CampoDataOpcional é uma propriedade DateOnly?: o texto "aaaa-mm-dd" ou null.
type CampoDataOpcional struct {
	Propriedade string
	Valor       *tipos.Data
}

func (c *CampoDataOpcional) propriedade() string { return c.Propriedade }

func (c *CampoDataOpcional) ler(t json.Token) bool {
	if t == nil {
		c.Valor = nil
		return true
	}
	d := CampoData{}
	if !d.ler(t) {
		return false
	}
	c.Valor = &d.Valor
	return true
}

func (c *CampoDataOpcional) validar(map[string][]string) {}

// CampoObjeto é uma propriedade que é outro objeto (o filtro dentro das categorias em lote). Sem ela ou com null, Veio
// fica false. A validação dos campos sai com o nome da propriedade na frente ("Filtro.Busca", contrato 07-transacoes,
// "categorias em lote com o filtro inválido").
type CampoObjeto struct {
	Propriedade string
	Campos      []Campo
	Veio        bool
}

func (c *CampoObjeto) propriedade() string { return c.Propriedade }

func (c *CampoObjeto) lerValor(l *LeitorJSON, inicio json.Token, caminho string) *ErroJSON {
	if inicio == nil {
		c.Veio = false
		return nil
	}
	if inicio != json.Delim('{') {
		return l.Conversao(caminho)
	}
	c.Veio = true
	return lerPropriedades(l, caminho, c.Campos)
}

func (c *CampoObjeto) validar(erros map[string][]string) {
	if !c.Veio {
		return
	}
	internos := map[string][]string{}
	for _, campo := range c.Campos {
		campo.validar(internos)
	}
	for chave, mensagens := range internos {
		erros[c.Propriedade+"."+chave] = mensagens
	}
}

// CampoListaDeObjetos é uma lista de objetos, com o tamanho mínimo e o máximo se Minimo e Maximo não forem 0, e
// obrigatória com Obrigatoria. Novo dá os campos de um item; o erro de tipo num campo dele sai com o caminho do campo
// ("$.itens[0].id"), e a validação dos campos de um item com a chave "Itens[0].Ticker". null fica nil; um item null
// fica nil na lista.
type CampoListaDeObjetos struct {
	Propriedade string
	Minimo      int
	Maximo      int
	Obrigatoria bool
	Novo        func() []Campo
	Itens       [][]Campo
}

func (c *CampoListaDeObjetos) propriedade() string { return c.Propriedade }

func (c *CampoListaDeObjetos) lerValor(l *LeitorJSON, inicio json.Token, caminho string) *ErroJSON {
	if inicio == nil {
		c.Itens = nil
		return nil
	}
	if inicio != json.Delim('[') {
		return l.Conversao(caminho)
	}
	itens := [][]Campo{}
	for i := 0; l.Mais(); i++ {
		t, erro := l.Token()
		if erro != nil {
			return erro
		}
		doItem := caminho + "[" + strconv.Itoa(i) + "]"
		switch t {
		case nil:
			itens = append(itens, nil)
			continue
		case json.Delim('{'):
		default:
			return l.Conversao(doItem)
		}
		campos := c.Novo()
		if erro := lerPropriedades(l, doItem, campos); erro != nil {
			return erro
		}
		itens = append(itens, campos)
	}
	if _, erro := l.Token(); erro != nil { // o "]"
		return erro
	}
	c.Itens = itens
	return nil
}

func (c *CampoListaDeObjetos) validar(erros map[string][]string) {
	validarTamanhoDaLista(erros, c.Propriedade, c.Itens == nil, len(c.Itens), !c.Obrigatoria, c.Minimo, c.Maximo)
	for i, campos := range c.Itens {
		internos := map[string][]string{}
		for _, campo := range campos {
			campo.validar(internos)
		}
		for chave, mensagens := range internos {
			erros[c.Propriedade+"["+strconv.Itoa(i)+"]."+chave] = mensagens
		}
	}
}
