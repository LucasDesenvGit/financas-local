package respostas

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// Regra (#710): a validação do corpo responde em português, por campo, sem traceId; o JSON quebrado e o corpo vazio
// têm uma mensagem só, na chave do pedido inteiro.
func TestLerCorpo(t *testing.T) {
	for _, c := range []struct {
		nome, tipo, corpo string
		status            int
		erros             map[string][]string
	}{
		{nome: "válido", corpo: `{"login":"ana.teste","senha":"Senha-Forte-123"}`, status: 0},
		{nome: "nome sem diferença de maiúsculas", corpo: `{"LOGIN":"ana.teste","Senha":"x"}`, status: 0},
		{nome: "propriedades a mais, com objetos e listas dentro, são puladas",
			corpo: `{"extra":{"a":[1,{"b":2}],"c":{}},"login":"ana.teste","lista":[[]],"senha":"x"}`, status: 0},
		{nome: "senha com o tamanho máximo exato", corpo: `{"login":"ana.teste","senha":"` + strings.Repeat("x", 200) + `"}`, status: 0},
		{nome: "login curto e sem senha", corpo: `{"login":"ab"}`, status: 400, erros: map[string][]string{
			"Login": {"O campo Login tem de ter de 3 a 100 caracteres."},
			"Senha": {"O campo Senha é obrigatório."}}},
		{nome: "vazio e só espaços são obrigatórios", corpo: `{"login":" ","senha":""}`, status: 400, erros: map[string][]string{
			"Login": {"O campo Login é obrigatório.", "O campo Login tem de ter de 3 a 100 caracteres."},
			"Senha": {"O campo Senha é obrigatório."}}},
		{nome: "senha longa demais", corpo: `{"login":"ana","senha":"` + strings.Repeat("x", 201) + `"}`, status: 400, erros: map[string][]string{
			"Senha": {"O campo Senha pode ter até 200 caracteres."}}},
		{nome: "JSON que acaba no meio (o do contrato)", corpo: `{"senhaAtual": "Senha-Forte-123", `, status: 400, erros: map[string][]string{
			"$": {"Os dados do pedido não estão num formato válido."}}},
		{nome: "texto depois do JSON", corpo: `{"login":"ana","senha":"x"} {}`, status: 400, erros: map[string][]string{
			"$": {"Os dados do pedido não estão num formato válido."}}},
		{nome: "corpo vazio", corpo: "", status: 400, erros: map[string][]string{"": {"O pedido veio sem dados."}}},
		{nome: "corpo null", corpo: "null", status: 400, erros: map[string][]string{"": {"O pedido veio sem dados."}}},
		{nome: "corpo que não é objeto", corpo: `[1]`, status: 400, erros: map[string][]string{
			"$": {"Os dados do pedido não estão num formato válido."}}},
		{nome: "campo de outro tipo", corpo: `{"login":5,"senha":"x"}`, status: 400, erros: map[string][]string{
			"$.login": {"O valor de login não é do tipo esperado."}}},
		{nome: "não é JSON", tipo: "text/plain", corpo: `{}`, status: 415},
	} {
		t.Run(c.nome, func(t *testing.T) {
			tipo := c.tipo
			if tipo == "" {
				tipo = "application/json"
			}
			r := httptest.NewRequest(http.MethodPost, "/api/auth/entrar", strings.NewReader(c.corpo))
			r.Header.Set("Content-Type", tipo)
			w := httptest.NewRecorder()
			login := &CampoTexto{Propriedade: "Login", Minimo: 3, Maximo: 100}
			senha := &CampoTexto{Propriedade: "Senha", Maximo: 200}
			ok := LerTextos(w, r, login, senha)
			if c.status == 0 {
				if !ok || login.Valor == nil || *login.Valor != "ana.teste" {
					t.Fatalf("recusou: %d %s", w.Code, w.Body)
				}
				return
			}
			if ok || w.Code != c.status {
				t.Fatalf("status %d (%v), esperado %d", w.Code, ok, c.status)
			}
			if tipo := w.Header().Get("Content-Type"); tipo != ConteudoJSON {
				t.Errorf("Content-Type %q, esperado %q", tipo, ConteudoJSON)
			}
			if c.erros == nil {
				return
			}
			var corpo map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &corpo); err != nil {
				t.Fatal(err)
			}
			var erros map[string][]string
			_ = json.Unmarshal(corpo["errors"], &erros)
			if !reflect.DeepEqual(erros, c.erros) {
				t.Errorf("errors:\n  %v\nesperado\n  %v", erros, c.erros)
			}
			if string(corpo["title"]) != `"`+tituloValidacao+`"` {
				t.Errorf("title %s", corpo["title"])
			}
			if _, tem := corpo["traceId"]; tem {
				t.Errorf("veio traceId: %s", w.Body)
			}
		})
	}
}

// Regra (#79): o [StringLength(ErrorMessage = ...)] do record troca a mensagem padrão do tamanho; o limite exato passa.
func TestMensagemDoTamanho(t *testing.T) {
	const mensagem = "O texto da cobrança pode ter até 100 caracteres."
	for _, c := range []struct {
		nome     string
		tamanho  int
		esperado []string
	}{
		{"no limite", 100, nil},
		{"um a mais", 101, []string{mensagem}},
	} {
		t.Run(c.nome, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/contas-agendadas",
				strings.NewReader(`{"textoCobranca":"`+strings.Repeat("A", c.tamanho)+`"}`))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			texto := &CampoTexto{Propriedade: "TextoCobranca", Maximo: 100, Opcional: true, MensagemDoTamanho: mensagem}
			ok := LerTextos(w, r, texto)
			if c.esperado == nil {
				if !ok {
					t.Fatalf("recusou: %d %s", w.Code, w.Body)
				}
				return
			}
			var corpo struct {
				Errors map[string][]string `json:"errors"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &corpo); err != nil {
				t.Fatal(err)
			}
			if ok || !reflect.DeepEqual(corpo.Errors["TextoCobranca"], c.esperado) {
				t.Errorf("%v: %v", ok, corpo.Errors)
			}
		})
	}
}

// Regra (#710): o JSON quebrado em qualquer ponto (antes do objeto, no valor, numa propriedade a mais, no fim de uma
// lista) é o mesmo erro, na chave "$" do pedido inteiro.
func TestJSONQuebradoEmQualquerPonto(t *testing.T) {
	novos := func() []Campo {
		return []Campo{&CampoTexto{Propriedade: "A", Maximo: 10, Opcional: true},
			&CampoLista{Propriedade: "N", Item: ItemInt, Opcional: true},
			&CampoListaDeObjetos{Propriedade: "Itens", Novo: func() []Campo { return []Campo{&CampoLong{Propriedade: "Id"}} }}}
	}
	for _, corpo := range []string{
		`x`,                   // antes do objeto
		`{"a":}`,              // no valor de um campo
		`{"outro":[1,}`,       // dentro de uma propriedade a mais
		`{"n":[1}`,            // a lista de números fechada com "}"
		`{"itens":[{"id":1}}`, // a lista de objetos fechada com "}"
		`{"a":"x"} {}`,        // texto depois do objeto
	} {
		if _, erro := lerObjeto([]byte(corpo), novos()); erro == nil || erro.Caminho != "$" || erro.Mensagem != MensagemJSONInvalido {
			t.Errorf("%s: %+v", corpo, erro)
		}
	}
}

// Regra: o corpo maior que o limite do servidor é o 413, sem corpo.
func TestCorpoGrandeDemais(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"a":"`+strings.Repeat("x", 100)+`"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.Body = http.MaxBytesReader(w, r.Body, 10)
	if _, ok := LerCorpo(w, r); ok || w.Code != http.StatusRequestEntityTooLarge || w.Body.Len() != 0 {
		t.Errorf("%v %d %q", ok, w.Code, w.Body)
	}
}

// Regra: a resposta que não vira JSON é o 500, sem o corpo pela metade.
func TestRespostaQueNaoViraJSON(t *testing.T) {
	w := httptest.NewRecorder()
	JSON(w, http.StatusOK, ConteudoJSON, map[string]any{"canal": make(chan int)})
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "canal") {
		t.Errorf("%d %q", w.Code, w.Body)
	}
}

// Regra (#772): LerJSON preenche o destino; JSON quebrado, de outro formato ou de outro tipo de conteúdo não passa,
// e a rota responde sozinha.
func TestLerJSON(t *testing.T) {
	for _, c := range []struct {
		nome, tipo, corpo string
		status            int
		mensagem          string
	}{
		{"válido", "application/json", `{"n":7}`, 0, ""},
		{"JSON quebrado", "application/json", `{"n":`, 400, MensagemFormatoEsperado},
		{"campo de outro tipo", "application/json", `{"n":"sete"}`, 400, MensagemFormatoEsperado},
		{"outro tipo de conteúdo", "text/plain", `{"n":7}`, 415, ""},
		{"corpo vazio", "application/json", ``, 400, ""},
	} {
		t.Run(c.nome, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(c.corpo))
			r.Header.Set("Content-Type", c.tipo)
			w := httptest.NewRecorder()
			var dto struct{ N int }
			ok := LerJSON(w, r, &dto)
			if c.status == 0 {
				if !ok || dto.N != 7 || w.Body.Len() != 0 {
					t.Fatalf("ok=%v dto=%+v corpo=%q", ok, dto, w.Body.String())
				}
				return
			}
			if ok || w.Code != c.status {
				t.Fatalf("ok=%v status=%d, esperado %d", ok, w.Code, c.status)
			}
			if c.mensagem != "" && !strings.Contains(w.Body.String(), c.mensagem) {
				t.Fatalf("corpo %q sem %q", w.Body.String(), c.mensagem)
			}
		})
	}
}

// Regra (#772): id do caminho que não é número vira 0, que não existe.
func TestIdDe(t *testing.T) {
	for corpo, esperado := range map[string]int64{"12": 12, "-3": -3, "abc": 0, "": 0, "1.5": 0} {
		r := httptest.NewRequest("GET", "/", nil)
		r.SetPathValue("id", corpo)
		if id := IdDe(r, "id"); id != esperado {
			t.Errorf("IdDe(%q) = %d, esperado %d", corpo, id, esperado)
		}
	}
}
