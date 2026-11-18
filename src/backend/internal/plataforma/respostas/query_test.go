package respostas

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
)

// Regra: o int e o bool da query são lidos como o TypeConverter do ASP.NET (sinal, espaços, hexadecimal com 0x, # e
// &h, 32 bits; true/false sem caixa). O gabarito é o testdata/query.json, gravado pela sonda das contas
// (internal/movimentacao/faturas/testdata/sonda-csharp) com o próprio .NET.
func TestQueryIgualAoASPNET(t *testing.T) {
	dados, err := os.ReadFile("testdata/query.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Valores []struct{ Valor, Inteiro, Longo, Booleano string } `json:"valores"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Valores) < 45 {
		t.Fatalf("o gabarito tem só %d casos: a sonda mudou?", len(g.Valores))
	}
	for _, c := range g.Valores {
		inteiro := "erro"
		if n, ok := InteiroDaQuery(c.Valor); ok {
			inteiro = strconv.Itoa(int(n))
		}
		booleano := "erro"
		if b, ok := BoolDaQuery(c.Valor); ok {
			booleano = map[bool]string{true: "True", false: "False"}[b]
		}
		longo := "erro"
		if n, ok := LongDaQuery(c.Valor); ok {
			longo = strconv.FormatInt(n, 10)
		}
		if inteiro != c.Inteiro || longo != c.Longo || booleano != c.Booleano {
			t.Errorf("%q: int %s, long %s e bool %s; o .NET dá %s, %s e %s", c.Valor, inteiro, longo, booleano, c.Inteiro,
				c.Longo, c.Booleano)
		}
	}
}

// Regra: o parâmetro da query não diferencia maiúsculas, vale o primeiro na ordem da URL e é decodificado (%20 e +).
func TestParametroDaQuery(t *testing.T) {
	casos := []struct {
		query, nome, valor string
		achou              bool
	}{
		{"ano=2026&mes=10", "mes", "10", true},
		{"Ano=1&ano=2", "ano", "1", true},
		{"ano=2&Ano=1", "ANO", "2", true},
		{"texto=a+b%20c", "texto", "a b c", true},
		{"te%78to=x", "texto", "x", true},
		{"ano", "ano", "", true},
		{"ano=%zz", "ano", "%zz", true},
		{"mes=1", "ano", "", false},
		{"", "ano", "", false},
	}
	for _, c := range casos {
		r := httptest.NewRequest("GET", "/x?"+c.query, nil)
		if valor, achou := ParametroDaQuery(r, c.nome); valor != c.valor || achou != c.achou {
			t.Errorf("%q, %q: %q %v; esperado %q %v", c.query, c.nome, valor, achou, c.valor, c.achou)
		}
	}
}

// Regra: a mensagem do 400 da query cita o valor que veio, mesmo vazio ou só com espaço.
func TestMensagemDaQuery(t *testing.T) {
	for valor, esperado := range map[string]string{"": "O valor '' não é válido.", " ": "O valor ' ' não é válido.",
		"abc": "O valor 'abc' não é válido."} {
		if m := MensagemDaQuery(valor); m != esperado {
			t.Errorf("MensagemDaQuery(%q) = %q, esperado %q", valor, m, esperado)
		}
	}
}

// Regra: o enum da query é o do EnumTypeModelBinder: o nome sem diferença de maiúsculas, o número (até 255) e a
// combinação por vírgula (o OR); o que converte mas não existe no enum (9, ou 1|2 num enum sem o 3) é "não definido".
func TestEnumDaQuery(t *testing.T) {
	nomes := map[string]int64{"contem": 1, "comecacom": 2, "igual": 3}
	casos := []struct {
		valor              string
		esperado           int64
		converte, definido bool
	}{
		{"Igual", 3, true, true}, {"comecacom", 2, true, true}, {" CONTEM ", 1, true, true}, {"2", 2, true, true},
		{"Contem, ComecaCom", 3, true, true}, {"2,1", 3, true, true}, {"9", 9, true, false}, {"255", 255, true, false},
		{"0", 0, true, false}, {"abc", 0, false, false}, {"256", 0, false, false}, {"-1", 0, false, false},
		{"", 0, false, false}, {"1,x", 0, false, false},
	}
	for _, c := range casos {
		v, converte, definido := EnumDaQuery(c.valor, nomes)
		if (converte && v != c.esperado) || converte != c.converte || definido != c.definido {
			t.Errorf("EnumDaQuery(%q) = %d, %v, %v; esperado %d, %v, %v", c.valor, v, converte, definido, c.esperado,
				c.converte, c.definido)
		}
	}
}

// Regra: a lista da query: os valores repetidos, na ordem e sem diferença de maiúsculas no nome; sem eles, o formato
// com índice, do 0 até o primeiro que falta (o repetido vale o primeiro); nada, lista vazia.
func TestValoresDaQuery(t *testing.T) {
	casos := []struct{ query, esperado string }{
		{"ids=1&IDS=2&outro=3", "[1 2]"},
		{"ids[0]=5&ids[1]=6&ids[3]=8", "[5 6]"},
		{"Ids[1]=6&ids[0]=5", "[5 6]"},
		{"ids[0]=5&ids[0]=7", "[5]"},
		{"ids=1&ids[0]=9", "[1]"},
		{"ids[1]=6", "[]"},
		{"ids[x]=1&idsx=2", "[]"},
		{"zzzz0]=9&ids[0=8", "[]"},
		{"outro=1&ids[0]=5", "[5]"},
		{"", "[]"},
	}
	for _, c := range casos {
		r := httptest.NewRequest("GET", "/x?"+c.query, nil)
		if obtido := fmt.Sprint(ValoresDaQuery(r, "ids")); obtido != c.esperado {
			t.Errorf("%q: %s, esperado %s", c.query, obtido, c.esperado)
		}
	}
}

// Regra: o caminho do erro acrescenta o nome com ponto; o nome com um caractere que confundiria o caminho vai entre
// colchetes e aspas.
func TestCaminhoDaChave(t *testing.T) {
	casos := []struct{ caminho, nome, esperado string }{
		{"$", "valor", "$.valor"},
		{"$.valores", "Abc", "$.valores.Abc"},
		{"$.valores", "A.b", "$.valores['A.b']"},
		{"$", "com espaço", "$['com espaço']"},
		{"$", "a[0]", "$['a[0]']"},
	}
	for _, c := range casos {
		if obtido := CaminhoDaChave(c.caminho, c.nome); obtido != c.esperado {
			t.Errorf("CaminhoDaChave(%q, %q) = %q, esperado %q", c.caminho, c.nome, obtido, c.esperado)
		}
	}
}

// Regra: as mensagens do 400 de uma propriedade do objeto da query citam o nome dela; o vazio, não.
func TestMensagemDaPropriedade(t *testing.T) {
	if m := MensagemDaPropriedade("um", "Pagina"); m != "O valor 'um' não é válido para Pagina." {
		t.Errorf("%q", m)
	}
	if m := MensagemDaPropriedade("", "Pagina"); m != "O valor '' não é válido." {
		t.Errorf("%q", m)
	}
	if m := MensagemDeValorInvalido("9"); m != "O valor '9' não é válido." {
		t.Errorf("%q", m)
	}
}

// Regra: a data da query é a do DateOnly na cultura invariante: o ISO, o americano (mês/dia/ano) e ano/mês/dia, com
// espaços nas pontas; o resto não vale.
func TestDataDaQuery(t *testing.T) {
	casos := []struct{ valor, esperado string }{
		{"2026-10-15", "2026-10-15"}, {" 2026-10-15 ", "2026-10-15"}, {"10/15/2026", "2026-10-15"}, {"1/2/2026", "2026-01-02"},
		{"2026/10/15", "2026-10-15"}, {"15/10/2026", ""}, {"2026-13-01", ""}, {"", ""}, {"ontem", ""},
	}
	for _, c := range casos {
		d, ok := DataDaQuery(c.valor)
		obtido := ""
		if ok {
			obtido = d.String()
		}
		if obtido != c.esperado {
			t.Errorf("DataDaQuery(%q) = %q, esperado %q", c.valor, obtido, c.esperado)
		}
	}
}
