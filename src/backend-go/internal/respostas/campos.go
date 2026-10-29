package respostas

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Faixa é o [Range(minimo, maximo)] de um campo numérico.
type Faixa struct {
	Minimo, Maximo float64
}

// mensagem é a do RangeAttribute: os limites saem com a cultura do servidor (pt-BR: "0,01"), como o .NET formata o
// double. Conferido no contrato (06-regras, "criar regra de descrição com valor fora da faixa").
func (f Faixa) mensagem(propriedade string) string {
	return fmt.Sprintf("The field %s must be between %s and %s.", propriedade, numeroPtBR(f.Minimo), numeroPtBR(f.Maximo))
}

func numeroPtBR(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', -1, 64), ".", ",", 1)
}

// inteiroDoToken lê um int do C# (32 bits): número JSON inteiro ou, como as opções web do ASP.NET leem
// (AllowReadingFromString), o mesmo número em texto. "7.0", "1e2" e " 7" não são int para o System.Text.Json.
func inteiroDoToken(t json.Token) (int64, bool) {
	var texto string
	switch v := t.(type) {
	case json.Number:
		texto = string(v)
	case string:
		texto = v
	default:
		return 0, false
	}
	n, err := strconv.ParseInt(texto, 10, 32)
	if err != nil || strings.HasPrefix(texto, "+") {
		return 0, false
	}
	return n, true
}

// CampoInteiro é um parâmetro int de um record: sem a propriedade no JSON fica com o Valor que o chamador deixou (o
// padrão do construtor, 0 se não houver), e null é erro de conversão (int não é anulável). Com Faixa, o [Range].
type CampoInteiro struct {
	Propriedade string
	Faixa       *Faixa
	Valor       int64
}

func (c *CampoInteiro) propriedade() string { return c.Propriedade }

func (c *CampoInteiro) ler(t json.Token) bool {
	n, ok := inteiroDoToken(t)
	c.Valor = n
	return ok
}

func (c *CampoInteiro) validar(erros map[string][]string) {
	if c.Faixa != nil && (float64(c.Valor) < c.Faixa.Minimo || float64(c.Valor) > c.Faixa.Maximo) {
		erros[c.Propriedade] = append(erros[c.Propriedade], c.Faixa.mensagem(c.Propriedade))
	}
}

// CampoInteiroOpcional é um parâmetro int? de um record: null ou ausente fica nil.
type CampoInteiroOpcional struct {
	Propriedade string
	Valor       *int64
}

func (c *CampoInteiroOpcional) propriedade() string { return c.Propriedade }

func (c *CampoInteiroOpcional) ler(t json.Token) bool {
	if t == nil {
		c.Valor = nil
		return true
	}
	n, ok := inteiroDoToken(t)
	c.Valor = &n
	return ok
}

func (c *CampoInteiroOpcional) validar(map[string][]string) {}

// CampoEnum é um parâmetro enum de um record, com o byte por baixo (os enums do projeto são ": byte"). O
// System.Text.Json sem o JsonStringEnumConverter lê só número inteiro de 0 a 255, nunca texto ("1" é recusado:
// contrato 05-categorias), e não confere se o valor existe no enum: quem confere é o controller ou o CHECK do banco.
// Com Anulavel (o "Natureza?"), null fica nil.
type CampoEnum struct {
	Propriedade string
	Anulavel    bool
	Valor       *int64
}

func (c *CampoEnum) propriedade() string { return c.Propriedade }

func (c *CampoEnum) ler(t json.Token) bool {
	if t == nil {
		c.Valor = nil
		return c.Anulavel
	}
	numero, ok := t.(json.Number)
	if !ok {
		return false
	}
	n, err := strconv.ParseUint(string(numero), 10, 8)
	if err != nil {
		return false
	}
	v := int64(n)
	c.Valor = &v
	return true
}

func (c *CampoEnum) validar(map[string][]string) {}

// Ou devolve o valor, ou o padrão quando a propriedade não veio (o default(T) do construtor do record).
func (c *CampoEnum) Ou(padrao int64) int64 {
	if c.Valor == nil {
		return padrao
	}
	return *c.Valor
}

// CampoDecimalOpcional é um parâmetro decimal? de um record: número JSON (com a escala que veio) ou número em texto
// (AllowReadingFromString). null ou ausente fica nil. Com Faixa, o [Range] em double, como o RangeAttribute(double,
// double): o valor é convertido para double só para comparar.
type CampoDecimalOpcional struct {
	Propriedade string
	Faixa       *Faixa
	Valor       *tipos.Decimal
}

func (c *CampoDecimalOpcional) propriedade() string { return c.Propriedade }

func (c *CampoDecimalOpcional) ler(t json.Token) bool {
	var texto string
	switch v := t.(type) {
	case nil:
		c.Valor = nil
		return true
	case json.Number:
		texto = string(v)
	case string:
		texto = v
	default:
		return false
	}
	if strings.TrimSpace(texto) != texto {
		return false
	}
	d, err := tipos.NovoDecimal(texto)
	if err != nil {
		return false
	}
	c.Valor = &d
	return true
}

func (c *CampoDecimalOpcional) validar(erros map[string][]string) {
	if c.Faixa == nil || c.Valor == nil {
		return
	}
	v, _ := c.Valor.Decimal().Float64()
	if v < c.Faixa.Minimo || v > c.Faixa.Maximo || math.IsNaN(v) {
		erros[c.Propriedade] = append(erros[c.Propriedade], c.Faixa.mensagem(c.Propriedade))
	}
}

// Padrao é o [RegularExpression] de um CampoTexto: vale só para texto que não é vazio (o RegularExpressionAttribute
// deixa passar nulo e ""), e o texto inteiro tem de casar.
type Padrao struct {
	Expressao string // como está no atributo do C#, que aparece na mensagem
	re        *regexp.Regexp
}

func NovoPadrao(expressao string) *Padrao {
	return &Padrao{Expressao: expressao, re: regexp.MustCompile(`^(?:` + expressao + `)$`)}
}

func (p *Padrao) mensagem(propriedade string) string {
	return fmt.Sprintf("The field %s must match the regular expression '%s'.", propriedade, p.Expressao)
}
