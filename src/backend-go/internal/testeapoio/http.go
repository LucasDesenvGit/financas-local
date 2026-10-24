package testeapoio

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/testes/contrato"
)

// UsuarioDosTestes é o Id do usuário logado que Chamar põe no contexto. A sessão do Go só guarda o Id (quem confere
// o login é o .NET, ADR 0049), então não precisa existir uma linha em Usuario.
const UsuarioDosTestes int64 = 1

// Resposta é o que o handler respondeu.
type Resposta struct {
	Status    int
	Cabecalho http.Header
	Corpo     []byte
}

// Chamar chama o handler direto, sem porta aberta, com UsuarioDosTestes logado: como o servidor do Go faz depois
// que o .NET confere a sessão (sessao.ComUsuario). corpo vazio = sem corpo; com corpo, vai como application/json.
//
//	resposta := testeapoio.Chamar(t, mux, "GET", "/api/contas", "")
func Chamar(t testing.TB, h http.Handler, metodo, caminho, corpo string) Resposta {
	t.Helper()
	return ChamarComo(t, h, UsuarioDosTestes, metodo, caminho, corpo)
}

// ChamarComo é o Chamar com outro usuário. Com usuario 0, a requisição vai sem ninguém logado (o caso de quem não
// passou pelo .NET), e sessao.Usuario devolve false.
func ChamarComo(t testing.TB, h http.Handler, usuario int64, metodo, caminho, corpo string) Resposta {
	t.Helper()
	var leitor io.Reader
	if corpo != "" {
		leitor = strings.NewReader(corpo)
	}
	requisicao := httptest.NewRequestWithContext(t.Context(), metodo, caminho, leitor)
	if corpo != "" {
		requisicao.Header.Set("Content-Type", "application/json")
	}
	if usuario != 0 {
		requisicao = requisicao.WithContext(sessao.ComUsuario(requisicao.Context(), usuario))
	}
	gravador := httptest.NewRecorder()
	h.ServeHTTP(gravador, requisicao)
	resultado := gravador.Result()
	defer resultado.Body.Close()
	dados, err := io.ReadAll(resultado.Body)
	if err != nil {
		t.Fatalf("%s %s: ler o corpo da resposta: %v", metodo, caminho, err)
	}
	return Resposta{Status: resultado.StatusCode, Cabecalho: resultado.Header, Corpo: dados}
}

// ConferirStatus falha o teste (e continua) se o status não for o esperado, mostrando o corpo.
func (r Resposta) ConferirStatus(t testing.TB, esperado int) {
	t.Helper()
	if r.Status != esperado {
		t.Errorf("status = %d, esperado %d; corpo: %s", r.Status, esperado, r.Corpo)
	}
}

// ConferirJSON compara o corpo com o JSON esperado pelo comparador do contrato (testes/contrato): números pelo
// texto, sem float64 ("10.50" ≠ "10.5", a escala do decimal do C#), [] ≠ null, null ≠ ausente, a ordem das chaves
// não importa e a dos itens importa. Falha o teste (e continua) com uma linha por diferença.
func (r Resposta) ConferirJSON(t testing.TB, esperado string) {
	t.Helper()
	ConferirJSON(t, r.Corpo, esperado)
}

// ConferirJSON é o Resposta.ConferirJSON para qualquer JSON (um DTO serializado, por exemplo).
func ConferirJSON(t testing.TB, obtido []byte, esperado string) {
	t.Helper()
	diferencas, err := contrato.CompararJSON([]byte(esperado), obtido)
	if err != nil {
		t.Errorf("comparar JSON: %v\nobtido: %s", err, obtido)
		return
	}
	if len(diferencas) > 0 {
		t.Errorf("JSON diferente do esperado:\n%s\nobtido: %s", strings.Join(diferencas, "\n"), obtido)
	}
}
