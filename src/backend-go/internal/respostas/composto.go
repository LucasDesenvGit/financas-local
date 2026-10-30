package respostas

import (
	"encoding/json"
	"strconv"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Os nomes dos tipos no C#, para o erro de conversão numa classe com propriedades (LerClasse, CampoObjeto): o
// System.Text.Json cita o tipo da propriedade. Os anuláveis saem como o Type.ToString() do .NET.
func anulavel(tipo string) string { return "System.Nullable`1[" + tipo + "]" }

func (c *CampoTexto) tipoCSharp() string           { return "System.String" }
func (c *CampoBool) tipoCSharp() string            { return "System.Boolean" }
func (c *CampoInteiro) tipoCSharp() string         { return "System.Int32" }
func (c *CampoInteiroOpcional) tipoCSharp() string { return anulavel("System.Int32") }
func (c *CampoDataOpcional) tipoCSharp() string    { return anulavel("System.DateOnly") }

func (c *CampoEnum) tipoCSharp() string {
	if c.Anulavel {
		return anulavel(c.Tipo)
	}
	return c.Tipo
}

// CampoLong é um long (não anulável) de um record: número JSON inteiro ou o mesmo número em texto; null é erro.
type CampoLong struct {
	Propriedade string
	Valor       int64
}

func (c *CampoLong) propriedade() string { return c.Propriedade }
func (c *CampoLong) tipoCSharp() string  { return "System.Int64" }

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

// CampoObjeto é uma propriedade que é outro DTO, uma classe com propriedades (o FiltroTransacoesDto dentro do
// AlterarCategoriasEmLoteDto). Sem ela ou com null, Veio fica false. O erro de tipo num campo dele cita o tipo do campo,
// e a validação dos campos sai com o nome da propriedade na frente ("Filtro.Busca", contrato 07-transacoes,
// "categorias em lote com o filtro inválido").
type CampoObjeto struct {
	Propriedade string
	Tipo        string // o nome completo da classe no C#
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
		return l.Conversao(caminho, c.Tipo)
	}
	c.Veio = true
	return lerPropriedades(l, caminho, daClasse, c.Campos)
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

// CampoListaDeObjetos é uma lista de records de um DTO (List<AlteracaoCategoriaDto>), com [MaxLength] se Maximo não
// for 0. Novo dá os campos de um item; cada item é lido como um record, e o erro de tipo num campo dele cita o record
// do item e o caminho do campo ("$.itens[0].id"). null fica nil; um item null fica nil na lista (o C# o recebe null).
type CampoListaDeObjetos struct {
	Propriedade string
	TipoItem    string // o nome completo do record do item no C#
	Maximo      int
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
		return l.Conversao(caminho, "System.Collections.Generic.List`1["+c.TipoItem+"]")
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
			return l.Conversao(doItem, c.TipoItem)
		}
		campos := c.Novo()
		if erro := lerPropriedades(l, doItem, doRecord(c.TipoItem), campos); erro != nil {
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
	validarTamanhoDaLista(erros, c.Propriedade, c.Itens == nil, len(c.Itens), true, 0, c.Maximo)
}
