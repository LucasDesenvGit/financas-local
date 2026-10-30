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
	naturezas := &CampoLista{Propriedade: "Naturezas", Item: ItemEnum("Ns.Natureza"), Opcional: true, Maximo: 2}
	tipo := &CampoEnum{Propriedade: "Tipo", Anulavel: true, Tipo: "Ns.Tipo"}
	pagina := &CampoInteiro{Propriedade: "Pagina", Faixa: &Faixa{Minimo: 1, Maximo: 10}, Valor: 1}
	campos := []Campo{data, naturezas, tipo, pagina, &CampoBool{Propriedade: "Sem"}, &CampoTexto{Propriedade: "Busca", Maximo: 3, Opcional: true},
		&CampoInteiroOpcional{Propriedade: "ContaId"}, &CampoLista{Propriedade: "Ids", Item: ItemLong, Opcional: true}}
	return &CampoObjeto{Propriedade: "Filtro", Tipo: "Ns.Filtro", Campos: campos}, data, naturezas, tipo, pagina
}

// Regra: na classe com propriedades, o erro de tipo cita o tipo da propriedade (anulável como Nullable`1[...]); o
// objeto aninhado null ou ausente não vem, e o que não é objeto cita a classe dele.
func TestClasseEObjetoAninhado(t *testing.T) {
	casos := []struct{ corpo, caminho, tipo string }{
		{`{"filtro":{"inicio":5}}`, "$.filtro.inicio", "System.Nullable`1[System.DateOnly]"},
		{`{"filtro":{"tipo":"Receita"}}`, "$.filtro.tipo", "System.Nullable`1[Ns.Tipo]"},
		{`{"filtro":{"pagina":"x"}}`, "$.filtro.pagina", "System.Int32"},
		{`{"filtro":{"sem":1}}`, "$.filtro.sem", "System.Boolean"},
		{`{"filtro":{"busca":1}}`, "$.filtro.busca", "System.String"},
		{`{"filtro":{"contaId":"x"}}`, "$.filtro.contaId", "System.Nullable`1[System.Int32]"},
		{`{"filtro":{"naturezas":5}}`, "$.filtro.naturezas", "System.Collections.Generic.List`1[Ns.Natureza]"},
		{`{"filtro":{"naturezas":["1"]}}`, "$.filtro.naturezas[0]", "Ns.Natureza"},
		{`{"filtro":{"ids":[1,"x"]}}`, "$.filtro.ids[1]", "System.Int64"},
		{`{"filtro":5}`, "$.filtro", "Ns.Filtro"},
		{`{"valor":"x"}`, "$.valor", "System.Int64"},
		{`[]`, "$", "Ns.Lote"},
	}
	for _, c := range casos {
		t.Run(c.corpo, func(t *testing.T) {
			filtro, _, _, _, _ := filtroDeTeste()
			_, erro := lerObjeto([]byte(c.corpo), "Ns.Lote", daClasse, []Campo{filtro, &CampoLong{Propriedade: "Valor"}})
			if erro == nil || erro.Caminho != c.caminho || !strings.Contains(erro.Mensagem, "converted to "+c.tipo+". Path: "+c.caminho+" |") {
				t.Errorf("erro = %+v", erro)
			}
		})
	}

	filtro, data, naturezas, tipo, pagina := filtroDeTeste()
	if _, erro := lerObjeto([]byte(`{"filtro":{"inicio":"2026-10-01","naturezas":[1,2],"tipo":2,"pagina":3}}`), "Ns.Lote",
		daClasse, []Campo{filtro}); erro != nil {
		t.Fatal(erro)
	}
	if !filtro.Veio || data.Valor == nil || data.Valor.String() != "2026-10-01" || len(naturezas.Valor) != 2 || *tipo.Valor != 2 || pagina.Valor != 3 {
		t.Errorf("lido: %v %v %v %v %v", filtro.Veio, data.Valor, naturezas.Valor, tipo.Valor, pagina.Valor)
	}
	for _, corpo := range []string{`{"filtro":null}`, `{}`, `{"filtro":{"inicio":null}}`} {
		filtro, data, _, _, _ := filtroDeTeste()
		if _, erro := lerObjeto([]byte(corpo), "Ns.Lote", daClasse, []Campo{filtro}); erro != nil || data.Valor != nil {
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
	if _, erro := lerObjeto([]byte(`{"filtro":{"pagina":0,"busca":"abcd","naturezas":[1,2,1]}}`), "Ns.Lote", daClasse,
		[]Campo{filtro}); erro != nil {
		t.Fatal(erro)
	}
	erros := map[string][]string{}
	filtro.validar(erros)
	esperado := map[string]string{
		"Filtro.Pagina":    "The field Pagina must be between 1 and 10.",
		"Filtro.Busca":     "The field Busca must be a string with a maximum length of 3.",
		"Filtro.Naturezas": "The field Naturezas must be a string or array type with a maximum length of '2'.",
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
		return &CampoListaDeObjetos{Propriedade: "Itens", TipoItem: "Ns.Item", Maximo: 2, Novo: func() []Campo {
			return []Campo{&CampoLong{Propriedade: "Id"}, &CampoBool{Propriedade: "Automatica"}}
		}}
	}
	erros := []struct{ corpo, caminho, tipo string }{
		{`{"itens":[{"id":"x"}]}`, "$.itens[0].id", "Ns.Item"},
		{`{"itens":[{"id":1},{"id":null}]}`, "$.itens[1].id", "Ns.Item"},
		{`{"itens":[5]}`, "$.itens[0]", "Ns.Item"},
		{`{"itens":5}`, "$.itens", "System.Collections.Generic.List`1[Ns.Item]"},
	}
	for _, c := range erros {
		_, erro := lerObjeto([]byte(c.corpo), "Ns.Lote", daClasse, []Campo{nova()})
		if erro == nil || erro.Caminho != c.caminho || !strings.Contains(erro.Mensagem, "converted to "+c.tipo+".") {
			t.Errorf("%s: %+v", c.corpo, erro)
		}
	}
	for _, corpo := range []string{`{"itens":[{"id":1}`, `{"itens":[{"id":1},`, `{"itens":[1`, `{"itens":[[`, `{"itens":[{}`} {
		if _, erro := lerObjeto([]byte(corpo), "Ns.Lote", daClasse, []Campo{nova()}); erro == nil {
			t.Errorf("%s: sem erro de sintaxe", corpo)
		}
	}
	lista := nova()
	if _, erro := lerObjeto([]byte(`{"itens":[{"ID":"7","automatica":true,"outro":[1]},null]}`), "Ns.Lote", daClasse, []Campo{lista}); erro != nil {
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
	if _, erro := lerObjeto([]byte(`{"itens":[{},{},{}]}`), "Ns.Lote", daClasse, []Campo{lista}); erro != nil {
		t.Fatal(erro)
	}
	lista.validar(validacao)
	if validacao["Itens"][0] != "The field Itens must be a string or array type with a maximum length of '2'." {
		t.Errorf("máximo: %v", validacao)
	}
	nula := nova()
	if _, erro := lerObjeto([]byte(`{"itens":null}`), "Ns.Lote", daClasse, []Campo{nula}); erro != nil || nula.Itens != nil {
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
	enum := &CampoLista{Propriedade: "N", Item: ItemEnum("Ns.E")}
	if _, erro := lerObjeto([]byte(`{"n":[0,255]}`), "Ns.Dto", doRecord("Ns.Dto"), []Campo{enum}); erro != nil || len(enum.Valor) != 2 {
		t.Errorf("enum: %v %v", erro, enum.Valor)
	}
	for _, corpo := range []string{`{"n":[256]}`, `{"n":["1"]}`, `{"n":[null]}`} {
		if _, erro := lerObjeto([]byte(corpo), "Ns.Dto", doRecord("Ns.Dto"), []Campo{&CampoLista{Propriedade: "N", Item: ItemEnum("Ns.E")}}); erro == nil ||
			!strings.Contains(erro.Mensagem, "converted to Ns.E.") {
			t.Errorf("%s: %+v", corpo, erro)
		}
	}
	inteiro := &CampoLista{Propriedade: "N", Item: ItemInt}
	if _, erro := lerObjeto([]byte(`{"n":[2147483648]}`), "Ns.Dto", doRecord("Ns.Dto"), []Campo{inteiro}); erro == nil ||
		!strings.Contains(erro.Mensagem, "converted to System.Int32.") {
		t.Errorf("int fora dos 32 bits: %+v", erro)
	}
	for _, corpo := range []string{`{"n":[1,}`, `{"n":[1`} {
		if _, erro := lerObjeto([]byte(corpo), "Ns.Dto", doRecord("Ns.Dto"), []Campo{inteiro}); erro == nil {
			t.Errorf("%s: sem erro de sintaxe", corpo)
		}
	}
	opcional := &CampoLista{Propriedade: "N", Item: ItemInt, Opcional: true}
	obrigatoria := &CampoLista{Propriedade: "M", Item: ItemInt}
	if _, erro := lerObjeto([]byte(`{"n":null,"m":null}`), "Ns.Dto", doRecord("Ns.Dto"), []Campo{opcional, obrigatoria}); erro != nil {
		t.Fatal(erro)
	}
	erros := map[string][]string{}
	opcional.validar(erros)
	obrigatoria.validar(erros)
	if len(erros) != 1 || erros["M"][0] != "The M field is required." {
		t.Errorf("validação: %v", erros)
	}
}

// Regra: a classe também trata o corpo "null" como corpo vazio (o 400 do BodyModelBinder).
func TestLerClasseComCorpoNulo(t *testing.T) {
	r := httptest.NewRequest(http.MethodPatch, "/x", strings.NewReader("null"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	if LerClasse(w, r, "dto", "Ns.Lote", &CampoBool{Propriedade: "Automatica"}) || w.Code != http.StatusBadRequest ||
		!strings.Contains(w.Body.String(), "A non-empty request body is required.") {
		t.Errorf("%d %s", w.Code, w.Body)
	}
}
