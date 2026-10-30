package respostas

import (
	"reflect"
	"strconv"
	"testing"
)

// lerCampos lê o corpo com os campos e devolve o erro de conversão (o caminho) ou os erros de validação.
func lerCampos(t *testing.T, corpo string, campos ...Campo) (string, map[string][]string) {
	t.Helper()
	_, erro := lerObjeto([]byte(corpo), "Dto", doRecord("Dto"), campos)
	if erro != nil {
		return erro.Caminho, nil
	}
	erros := map[string][]string{}
	for _, c := range campos {
		c.validar(erros)
	}
	return "", erros
}

// Regra: int do C# pelo System.Text.Json com as opções web: número inteiro ou o mesmo número em texto; casa decimal,
// expoente, espaço, "+" e null são erro de conversão; fora de 32 bits também. [Range] com a mensagem do .NET.
func TestCampoInteiro(t *testing.T) {
	casos := []struct {
		nome, corpo string
		conversao   string
		valor       int64
		erros       map[string][]string
	}{
		{"número", `{"p":70}`, "", 70, map[string][]string{}},
		{"número em texto", `{"p":"70"}`, "", 70, map[string][]string{}},
		{"ausente fica o padrão", `{}`, "", 5, map[string][]string{}},
		{"no limite de baixo", `{"p":0}`, "", 0, map[string][]string{}},
		{"no limite de cima", `{"p":10000}`, "", 10000, map[string][]string{}},
		{"negativo fora da faixa", `{"p":-1}`, "", -1, map[string][]string{"P": {"The field P must be between 0 and 10000."}}},
		{"acima da faixa", `{"p":10001}`, "", 10001, map[string][]string{"P": {"The field P must be between 0 and 10000."}}},
		{"casa decimal", `{"p":7.0}`, "$.p", 0, nil},
		{"expoente", `{"p":1e2}`, "$.p", 0, nil},
		{"texto com espaço", `{"p":" 7"}`, "$.p", 0, nil},
		{"texto com sinal de mais", `{"p":"+7"}`, "$.p", 0, nil},
		{"null num int", `{"p":null}`, "$.p", 0, nil},
		{"fora de 32 bits", `{"p":2147483648}`, "$.p", 0, nil},
		{"bool", `{"p":true}`, "$.p", 0, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			campo := &CampoInteiro{Propriedade: "P", Faixa: &Faixa{0, 10000}, Valor: 5}
			conversao, erros := lerCampos(t, c.corpo, campo)
			if conversao != c.conversao || (c.conversao == "" && (campo.Valor != c.valor || !reflect.DeepEqual(erros, c.erros))) {
				t.Errorf("conversão %q, valor %d, erros %v; esperado %q, %d, %v", conversao, campo.Valor, erros, c.conversao, c.valor, c.erros)
			}
		})
	}
}

// Regra: int? aceita null e ausente (nil), e o número em texto.
func TestCampoInteiroOpcional(t *testing.T) {
	for corpo, esperado := range map[string]*int64{`{"p":null}`: nil, `{}`: nil, `{"p":"28"}`: p(int64(28)), `{"p":28}`: p(int64(28))} {
		campo := &CampoInteiroOpcional{Propriedade: "P"}
		if conversao, _ := lerCampos(t, corpo, campo); conversao != "" || !mesmo(campo.Valor, esperado) {
			t.Errorf("%s: conversão %q, valor %v", corpo, conversao, campo.Valor)
		}
	}
	if conversao, _ := lerCampos(t, `{"p":"x"}`, &CampoInteiroOpcional{Propriedade: "P"}); conversao != "$.p" {
		t.Errorf("texto que não é número: %q", conversao)
	}
}

// Regra: enum de byte: só número inteiro de 0 a 255 (texto é recusado); valor que não existe no enum passa;
// null só no anulável.
func TestCampoEnum(t *testing.T) {
	casos := []struct {
		nome, corpo string
		anulavel    bool
		conversao   string
		valor       *int64
	}{
		{"número", `{"p":3}`, false, "", p(int64(3))},
		{"fora da lista passa", `{"p":9}`, false, "", p(int64(9))},
		{"limite do byte", `{"p":255}`, false, "", p(int64(255))},
		{"acima do byte", `{"p":256}`, false, "$.p", nil},
		{"negativo", `{"p":-1}`, false, "$.p", nil},
		{"texto", `{"p":"1"}`, false, "$.p", nil},
		{"null no não anulável", `{"p":null}`, false, "$.p", nil},
		{"null no anulável", `{"p":null}`, true, "", nil},
		{"ausente", `{}`, false, "", nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			campo := &CampoEnum{Propriedade: "P", Anulavel: c.anulavel}
			if conversao, _ := lerCampos(t, c.corpo, campo); conversao != c.conversao || (conversao == "" && !mesmo(campo.Valor, c.valor)) {
				t.Errorf("conversão %q, valor %v; esperado %q, %v", conversao, campo.Valor, c.conversao, c.valor)
			}
		})
	}
	if got := (&CampoEnum{}).Ou(1); got != 1 {
		t.Errorf("Ou sem valor = %d, esperado o padrão", got)
	}
}

// Regra: decimal? com a escala que veio, também em texto; [Range] em double com os limites em pt-BR ("0,01").
func TestCampoDecimalOpcional(t *testing.T) {
	faixa := &Faixa{0.01, 999999999}
	mensagem := "The field P must be between 0,01 and 999999999."
	casos := []struct {
		nome, corpo string
		conversao   string
		valor       string
		erros       map[string][]string
	}{
		{"número com a escala", `{"p":200.50}`, "", "200.50", map[string][]string{}},
		{"em texto", `{"p":"100"}`, "", "100", map[string][]string{}},
		{"null", `{"p":null}`, "", "", map[string][]string{}},
		{"no limite de baixo", `{"p":0.01}`, "", "0.01", map[string][]string{}},
		{"no limite de cima", `{"p":999999999}`, "", "999999999", map[string][]string{}},
		{"zero fora da faixa", `{"p":0}`, "", "0", map[string][]string{"P": {mensagem}}},
		{"acima da faixa", `{"p":1000000000}`, "", "1000000000", map[string][]string{"P": {mensagem}}},
		{"texto que não é número", `{"p":"abc"}`, "$.p", "", nil},
		{"texto com espaço", `{"p":" 1"}`, "$.p", "", nil},
		{"bool", `{"p":false}`, "$.p", "", nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			campo := &CampoDecimalOpcional{Propriedade: "P", Faixa: faixa}
			conversao, erros := lerCampos(t, c.corpo, campo)
			valor := ""
			if campo.Valor != nil {
				valor = campo.Valor.String()
			}
			if conversao != c.conversao || (conversao == "" && (valor != c.valor || !reflect.DeepEqual(erros, c.erros))) {
				t.Errorf("conversão %q, valor %q, erros %v; esperado %q, %q, %v", conversao, valor, erros, c.conversao, c.valor, c.erros)
			}
		})
	}
}

// Regra: [RegularExpression] só no texto que não é vazio, casando o texto inteiro, depois do [Required].
func TestCampoTextoComPadrao(t *testing.T) {
	padrao := NovoPadrao(`^#[0-9a-fA-F]{6}$`)
	casos := []struct {
		corpo string
		erros map[string][]string
	}{
		{`{"c":"#12abEF"}`, map[string][]string{}},
		{`{"c":"red"}`, map[string][]string{"C": {padrao.Mensagem("C")}}},
		{`{"c":""}`, map[string][]string{"C": {"The C field is required."}}},
		{`{"c":"  "}`, map[string][]string{"C": {"The C field is required.", padrao.Mensagem("C")}}},
	}
	for _, c := range casos {
		campo := &CampoTexto{Propriedade: "C", Maximo: 100, Padrao: padrao}
		if _, erros := lerCampos(t, c.corpo, campo); !reflect.DeepEqual(erros, c.erros) {
			t.Errorf("%s: erros %v; esperado %v", c.corpo, erros, c.erros)
		}
	}
}

func p[T any](v T) *T { return &v }

func mesmo(a, b *int64) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

// Regra: byte? com [Range(1, 31)] (o dia do mês): número de 0 a 255 ou em texto; null fica nil; fora de 0 a 255, com
// casas ou com "+" é erro de conversão; fora da faixa, a mensagem do RangeAttribute.
func TestCampoByteOpcional(t *testing.T) {
	mensagem := "The field P must be between 1 and 31."
	casos := []struct {
		nome, corpo, conversao, valor string
		erros                         map[string][]string
	}{
		{"número", `{"p":31}`, "", "31", map[string][]string{}},
		{"no limite de baixo", `{"p":1}`, "", "1", map[string][]string{}},
		{"em texto", `{"p":"5"}`, "", "5", map[string][]string{}},
		{"null", `{"p":null}`, "", "<nil>", map[string][]string{}},
		{"ausente", `{}`, "", "<nil>", map[string][]string{}},
		{"zero fora da faixa", `{"p":0}`, "", "0", map[string][]string{"P": {mensagem}}},
		{"32 fora da faixa", `{"p":32}`, "", "32", map[string][]string{"P": {mensagem}}},
		{"256 não é byte", `{"p":256}`, "$.p", "", nil},
		{"negativo", `{"p":-1}`, "$.p", "", nil},
		{"com casas", `{"p":5.0}`, "$.p", "", nil},
		{"com sinal", `{"p":"+5"}`, "$.p", "", nil},
		{"bool", `{"p":true}`, "$.p", "", nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			campo := &CampoByteOpcional{Propriedade: "P", Faixa: &Faixa{1, 31}}
			conversao, erros := lerCampos(t, c.corpo, campo)
			valor := "<nil>"
			if campo.Valor != nil {
				valor = strconv.FormatInt(*campo.Valor, 10)
			}
			if conversao != c.conversao || (conversao == "" && (valor != c.valor || !reflect.DeepEqual(erros, c.erros))) {
				t.Errorf("conversão %q, valor %q, erros %v; esperado %q, %q, %v", conversao, valor, erros, c.conversao, c.valor, c.erros)
			}
		})
	}
}

// Regra: bool? aceita true, false e null; texto e número são erro de conversão.
func TestCampoBoolOpcional(t *testing.T) {
	casos := []struct{ corpo, conversao, valor string }{
		{`{"p":true}`, "", "true"},
		{`{"p":false}`, "", "false"},
		{`{"p":null}`, "", "<nil>"},
		{`{}`, "", "<nil>"},
		{`{"p":"true"}`, "$.p", ""},
		{`{"p":1}`, "$.p", ""},
	}
	for _, c := range casos {
		campo := &CampoBoolOpcional{Propriedade: "P"}
		conversao, _ := lerCampos(t, c.corpo, campo)
		valor := "<nil>"
		if campo.Valor != nil {
			valor = strconv.FormatBool(*campo.Valor)
		}
		if conversao != c.conversao || (conversao == "" && valor != c.valor) {
			t.Errorf("%s: conversão %q, valor %q; esperado %q, %q", c.corpo, conversao, valor, c.conversao, c.valor)
		}
	}
}

// Regra: decimal (não anulável): ausente fica zero, a escala que veio fica, null e texto que não é número são erro.
func TestCampoDecimal(t *testing.T) {
	casos := []struct{ corpo, conversao, valor string }{
		{`{"p":1000.50}`, "", "1000.50"},
		{`{"p":"-0.01"}`, "", "-0.01"},
		{`{}`, "", "0"},
		{`{"p":null}`, "$.p", ""},
		{`{"p":"x"}`, "$.p", ""},
		{`{"p":[]}`, "$.p", ""},
	}
	for _, c := range casos {
		campo := &CampoDecimal{Propriedade: "P"}
		conversao, _ := lerCampos(t, c.corpo, campo)
		if conversao != c.conversao || (conversao == "" && campo.Valor.String() != c.valor) {
			t.Errorf("%s: conversão %q, valor %s; esperado %q, %s", c.corpo, conversao, campo.Valor, c.conversao, c.valor)
		}
	}
}

// Regra: o ErrorMessage do atributo vale no lugar da mensagem padrão do RegularExpressionAttribute.
func TestMensagemDoPadrao(t *testing.T) {
	t.Parallel()
	padrao := NovoPadrao(`^(BRL|USD)$`)
	if m := padrao.Mensagem("Moeda"); m != "The field Moeda must match the regular expression '^(BRL|USD)$'." {
		t.Errorf("mensagem padrão: %q", m)
	}
	padrao.MensagemPropria = "Moeda inválida: use BRL ou USD."
	if m := padrao.Mensagem("Moeda"); m != "Moeda inválida: use BRL ou USD." {
		t.Errorf("mensagem própria: %q", m)
	}
}
