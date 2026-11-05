package respostas

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// A validação do [ApiController]: as mensagens são as do ASP.NET (o contrato 00-auth grava as do .NET).
func TestLerCorpoComoOAspNet(t *testing.T) {
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
			"Login": {"The field Login must be a string with a minimum length of 3 and a maximum length of 100."},
			"Senha": {"The Senha field is required."}}},
		{nome: "vazio e só espaços são obrigatórios", corpo: `{"login":" ","senha":""}`, status: 400, erros: map[string][]string{
			"Login": {"The Login field is required.", "The field Login must be a string with a minimum length of 3 and a maximum length of 100."},
			"Senha": {"The Senha field is required."}}},
		{nome: "senha longa demais", corpo: `{"login":"ana","senha":"` + strings.Repeat("x", 201) + `"}`, status: 400, erros: map[string][]string{
			"Senha": {"The field Senha must be a string with a maximum length of 200."}}},
		{nome: "JSON que acaba no meio (o do contrato)", corpo: `{"senhaAtual": "Senha-Forte-123", `, status: 400, erros: map[string][]string{
			"$":   {"Expected start of a property name or value, but instead reached end of data. Path: $ | LineNumber: 0 | BytePositionInLine: 34."},
			"dto": {"The dto field is required."}}},
		{nome: "corpo vazio", corpo: "", status: 400, erros: map[string][]string{
			"":    {"A non-empty request body is required."},
			"dto": {"The dto field is required."}}},
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
			ok := LerTextos(w, r, "dto", "Financas.Api.Dtos.CredenciaisDto", login, senha)
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
				t.Errorf("Content-Type %q, o .NET manda %q", tipo, ConteudoJSON)
			}
			if c.erros == nil {
				return
			}
			var corpo struct {
				Errors  map[string][]string `json:"errors"`
				Title   string              `json:"title"`
				TraceID string              `json:"traceId"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &corpo); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(corpo.Errors, c.erros) {
				t.Errorf("errors:\n  %v\nesperado\n  %v", corpo.Errors, c.erros)
			}
			if corpo.Title != tituloValidacao || !strings.HasPrefix(corpo.TraceID, "00-") {
				t.Errorf("title %q, traceId %q", corpo.Title, corpo.TraceID)
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
			ok := LerTextos(w, r, "dto", "Financas.Api.Dtos.SalvarContaAgendadaDto", texto)
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
