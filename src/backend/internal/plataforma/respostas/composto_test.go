package respostas

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// filtroDeTeste monta os campos de uma classe com um de cada tipo (o FiltroTransacoesDto tem estes).
func filtroDeTeste() (*CampoObjeto, *CampoDataOpcional, *CampoLista, *CampoEnum, *CampoInteiro) {
	data := &CampoDataOpcional{Propriedade: "Inicio"}
	naturezas := &CampoLista{Propriedade: "Naturezas", Item: ItemEnum, Opcional: true, Maximo: 2}
	tipo := &CampoEnum{Propriedade: "Tipo", Anulavel: true}
	pagina := &CampoInteiro{Propriedade: "Pagina", Faixa: &Faixa{Minimo: 1, Maximo: 10}, Valor: 1}
	campos := []Campo{data, naturezas, tipo, pagina, &CampoBool{Propriedade: "Sem"}, &CampoTexto{Propriedade: "Busca", Maximo: 3, Opcional: true},
		&CampoInteiroOpcional{Propriedade: "ContaId"}, &CampoLista{Propriedade: "Ids", Item: ItemLong, Opcional: true}}
	return &CampoObjeto{Propriedade: "Filtro", Campos: campos}, data, naturezas, tipo, pagina
}

// Regra: o erro de tipo num campo cita o caminho dele, sem o "$." ("O valor de filtro.inicio …"); o corpo que não é
// objeto é o JSON inválido; o objeto aninhado null ou ausente não vem.
func TestClasseEObjetoAninhado(t *testing.T) {
	casos := []struct{ corpo, caminho string }{
		{`{"filtro":{"inicio":5}}`, "$.filtro.inicio"},
		// O DateOnly do System.Text.Json não apara: com espaço, não é data (o LerData aparava).
		{`{"filtro":{"inicio":" 2026-10-01"}}`, "$.filtro.inicio"},
		{`{"filtro":{"tipo":"Receita"}}`, "$.filtro.tipo"},
		{`{"filtro":{"pagina":"x"}}`, "$.filtro.pagina"},
		{`{"filtro":{"sem":1}}`, "$.filtro.sem"},
		{`{"filtro":{"busca":1}}`, "$.filtro.busca"},
		{`{"filtro":{"contaId":"x"}}`, "$.filtro.contaId"},
		{`{"filtro":{"naturezas":5}}`, "$.filtro.naturezas"},
		{`{"filtro":{"naturezas":["1"]}}`, "$.filtro.naturezas[0]"},
		{`{"filtro":{"ids":[1,"x"]}}`, "$.filtro.ids[1]"},
		{`{"filtro":5}`, "$.filtro"},
		{`{"valor":"x"}`, "$.valor"},
		{`[]`, "$"},
	}
	for _, c := range casos {
		t.Run(c.corpo, func(t *testing.T) {
			filtro, _, _, _, _ := filtroDeTeste()
			_, erro := lerObjeto([]byte(c.corpo), []Campo{filtro, &CampoLong{Propriedade: "Valor"}})
			if erro == nil || erro.Caminho != c.caminho || erro.Mensagem != mensagemDeConversao(c.caminho) {
				t.Errorf("erro = %+v", erro)
			}
		})
	}

	filtro, data, naturezas, tipo, pagina := filtroDeTeste()
	if _, erro := lerObjeto([]byte(`{"filtro":{"inicio":"2026-10-01","naturezas":[1,2],"tipo":2,"pagina":3}}`), []Campo{filtro}); erro != nil {
		t.Fatal(erro)
	}
	if !filtro.Veio || data.Valor == nil || data.Valor.String() != "2026-10-01" || len(naturezas.Valor) != 2 || *tipo.Valor != 2 || pagina.Valor != 3 {
		t.Errorf("lido: %v %v %v %v %v", filtro.Veio, data.Valor, naturezas.Valor, tipo.Valor, pagina.Valor)
	}
	for _, corpo := range []string{`{"filtro":null}`, `{}`, `{"filtro":{"inicio":null}}`} {
		filtro, data, _, _, _ := filtroDeTeste()
		if _, erro := lerObjeto([]byte(corpo), []Campo{filtro}); erro != nil || data.Valor != nil {
			t.Errorf("%s: %v %v", corpo, erro, data.Valor)
		}
		if (corpo == `{"filtro":{"inicio":null}}`) != filtro.Veio {
			t.Errorf("%s: Veio = %v", corpo, filtro.Veio)
		}
	}
}

// Regra: a validação do objeto aninhado sai com o nome dele na frente; sem ele, não valida nada.
func TestValidacaoDoObjetoAninhado(t *testing.T) {
	filtro, _, _, _, _ := filtroDeTeste()
	if _, erro := lerObjeto([]byte(`{"filtro":{"pagina":0,"busca":"abcd","naturezas":[1,2,1]}}`), []Campo{filtro}); erro != nil {
		t.Fatal(erro)
	}
	erros := map[string][]string{}
	filtro.validar(erros)
	esperado := map[string]string{
		"Filtro.Pagina":    "O campo Pagina tem de estar entre 1 e 10.",
		"Filtro.Busca":     "O campo Busca pode ter até 3 caracteres.",
		"Filtro.Naturezas": "O campo Naturezas pode ter até 2 itens.",
	}
	if len(erros) != len(esperado) {
		t.Errorf("erros: %v", erros)
	}
	for chave, mensagem := range esperado {
		if len(erros[chave]) != 1 || erros[chave][0] != mensagem {
			t.Errorf("%s: %v", chave, erros[chave])
		}
	}
	sem, _, _, _, _ := filtroDeTeste()
	nenhum := map[string][]string{}
	sem.validar(nenhum)
	if len(nenhum) != 0 {
		t.Errorf("sem o objeto: %v", nenhum)
	}
}

// Regra: a lista de records: cada item lido como record (o erro cita o record do item), o item null fica nil, o valor
// que não é lista cita a lista, null fica nil, e o [MaxLength].
func TestListaDeObjetos(t *testing.T) {
	nova := func() *CampoListaDeObjetos {
		return &CampoListaDeObjetos{Propriedade: "Itens", Maximo: 2, Novo: func() []Campo {
			return []Campo{&CampoLong{Propriedade: "Id"}, &CampoBool{Propriedade: "Automatica"}}
		}}
	}
	erros := []struct{ corpo, caminho string }{
		{`{"itens":[{"id":"x"}]}`, "$.itens[0].id"},
		{`{"itens":[{"id":1},{"id":null}]}`, "$.itens[1].id"},
		{`{"itens":[5]}`, "$.itens[0]"},
		{`{"itens":5}`, "$.itens"},
	}
	for _, c := range erros {
		_, erro := lerObjeto([]byte(c.corpo), []Campo{nova()})
		if erro == nil || erro.Caminho != c.caminho || erro.Mensagem != mensagemDeConversao(c.caminho) {
			t.Errorf("%s: %+v", c.corpo, erro)
		}
	}
	for _, corpo := range []string{`{"itens":[{"id":1}`, `{"itens":[{"id":1},`, `{"itens":[1`, `{"itens":[[`, `{"itens":[{}`} {
		if _, erro := lerObjeto([]byte(corpo), []Campo{nova()}); erro == nil {
			t.Errorf("%s: sem erro de sintaxe", corpo)
		}
	}
	lista := nova()
	if _, erro := lerObjeto([]byte(`{"itens":[{"ID":"7","automatica":true,"outro":[1]},null]}`), []Campo{lista}); erro != nil {
		t.Fatal(erro)
	}
	if len(lista.Itens) != 2 || lista.Itens[1] != nil || lista.Itens[0][0].(*CampoLong).Valor != 7 || !lista.Itens[0][1].(*CampoBool).Valor {
		t.Errorf("itens: %v", lista.Itens)
	}
	validacao := map[string][]string{}
	lista.validar(validacao)
	if len(validacao) != 0 {
		t.Errorf("2 itens com o máximo 2: %v", validacao)
	}
	if _, erro := lerObjeto([]byte(`{"itens":[{},{},{}]}`), []Campo{lista}); erro != nil {
		t.Fatal(erro)
	}
	lista.validar(validacao)
	if validacao["Itens"][0] != "O campo Itens pode ter até 2 itens." {
		t.Errorf("máximo: %v", validacao)
	}
	nula := nova()
	if _, erro := lerObjeto([]byte(`{"itens":null}`), []Campo{nula}); erro != nil || nula.Itens != nil {
		t.Errorf("null: %v %v", erro, nula.Itens)
	}
	sem := map[string][]string{}
	nula.validar(sem)
	if len(sem) != 0 {
		t.Errorf("a lista opcional nula: %v", sem)
	}
}

// Regra: a lista de números: o enum só em número (0 a 255), o int em 32 bits; a opcional aceita null; a obrigatória
// recusa; o JSON quebrado dentro da lista é erro de sintaxe.
func TestListaDeNumeros(t *testing.T) {
	enum := &CampoLista{Propriedade: "N", Item: ItemEnum}
	if _, erro := lerObjeto([]byte(`{"n":[0,255]}`), []Campo{enum}); erro != nil || len(enum.Valor) != 2 {
		t.Errorf("enum: %v %v", erro, enum.Valor)
	}
	for _, corpo := range []string{`{"n":[256]}`, `{"n":["1"]}`, `{"n":[null]}`} {
		if _, erro := lerObjeto([]byte(corpo), []Campo{&CampoLista{Propriedade: "N", Item: ItemEnum}}); erro == nil ||
			erro.Mensagem != "O valor de n[0] não é do tipo esperado." {
			t.Errorf("%s: %+v", corpo, erro)
		}
	}
	inteiro := &CampoLista{Propriedade: "N", Item: ItemInt}
	if _, erro := lerObjeto([]byte(`{"n":[2147483648]}`), []Campo{inteiro}); erro == nil ||
		erro.Mensagem != "O valor de n[0] não é do tipo esperado." {
		t.Errorf("int fora dos 32 bits: %+v", erro)
	}
	for _, corpo := range []string{`{"n":[1,}`, `{"n":[1`} {
		if _, erro := lerObjeto([]byte(corpo), []Campo{inteiro}); erro == nil {
			t.Errorf("%s: sem erro de sintaxe", corpo)
		}
	}
	opcional := &CampoLista{Propriedade: "N", Item: ItemInt, Opcional: true}
	obrigatoria := &CampoLista{Propriedade: "M", Item: ItemInt}
	if _, erro := lerObjeto([]byte(`{"n":null,"m":null}`), []Campo{opcional, obrigatoria}); erro != nil {
		t.Fatal(erro)
	}
	erros := map[string][]string{}
	opcional.validar(erros)
	obrigatoria.validar(erros)
	if len(erros) != 1 || erros["M"][0] != "O campo M é obrigatório." {
		t.Errorf("validação: %v", erros)
	}
}

// Regra: a classe também trata o corpo "null" como corpo vazio (o 400 do BodyModelBinder).
func TestLerClasseComCorpoNulo(t *testing.T) {
	r := httptest.NewRequest(http.MethodPatch, "/x", strings.NewReader("null"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	if LerObjeto(w, r, &CampoBool{Propriedade: "Automatica"}) || w.Code != http.StatusBadRequest ||
		!strings.Contains(w.Body.String(), "O pedido veio sem dados.") {
		t.Errorf("%d %s", w.Code, w.Body)
	}
}

// mensagemDeConversao é a mensagem esperada do erro de tipo no caminho, escrita à mão: o "$" é o corpo inteiro.
func mensagemDeConversao(caminho string) string {
	if caminho == "$" {
		return MensagemJSONInvalido
	}
	return "O valor de " + strings.TrimPrefix(caminho, "$.") + " não é do tipo esperado."
}

// Regra: os campos sem regra de validação (enum, bool, long, data opcional) nunca acrescentam erro, mesmo sem valor.
func TestCamposSemValidacao(t *testing.T) {
	for _, c := range []Campo{&CampoEnum{Propriedade: "E"}, &CampoBool{Propriedade: "B"}, &CampoBoolOpcional{Propriedade: "O"},
		&CampoLong{Propriedade: "L"}, &CampoDataOpcional{Propriedade: "D"}} {
		erros := map[string][]string{}
		c.validar(erros)
		if len(erros) != 0 {
			t.Errorf("%s: %v", c.propriedade(), erros)
		}
	}
}

// Regra (#710): o tamanho da lista sai em português, no singular com 1 ("pelo menos 1 item") e no plural com mais.
func TestTamanhoDaListaEmPortugues(t *testing.T) {
	casos := []struct {
		corpo    string
		campo    *CampoLista
		esperado string
	}{
		{`{"ids":[]}`, &CampoLista{Propriedade: "Ids", Item: ItemLong, Minimo: 1}, "O campo Ids precisa de pelo menos 1 item."},
		{`{"ids":[1]}`, &CampoLista{Propriedade: "Ids", Item: ItemLong, Minimo: 2}, "O campo Ids precisa de pelo menos 2 itens."},
		{`{"ids":[1,2]}`, &CampoLista{Propriedade: "Ids", Item: ItemLong, Maximo: 1}, "O campo Ids pode ter até 1 item."},
		{`{}`, &CampoLista{Propriedade: "Ids", Item: ItemLong}, "O campo Ids é obrigatório."},
	}
	for _, c := range casos {
		if _, erro := lerObjeto([]byte(c.corpo), []Campo{c.campo}); erro != nil {
			t.Fatalf("%s: %+v", c.corpo, erro)
		}
		erros := map[string][]string{}
		c.campo.validar(erros)
		if len(erros["Ids"]) != 1 || erros["Ids"][0] != c.esperado {
			t.Errorf("%s: %v, esperado %q", c.corpo, erros, c.esperado)
		}
	}
}
