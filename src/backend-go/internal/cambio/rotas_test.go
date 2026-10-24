package cambio

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// rotasDeTeste monta as rotas com o banco de teste, a PTAX falsa e a lista de moedas guardada (USD e EUR, de 2
// dias atrás: não vence, e a PTAX não é consultada para a lista).
func rotasDeTeste(t *testing.T, fonte *fonteFalsa) (*http.ServeMux, *testeapoio.Fabrica) {
	t.Helper()
	agora := testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0)
	s, _, fab := novoServicoDeTeste(t, agora, fonte)
	fab.Configuracao("Cambio.Moedas", `[{"Codigo":"EUR","Nome":"Euro"},{"Codigo":"USD","Nome":"Dolar"}]`)
	fab.Configuracao("Cambio.MoedasConsultadasEm", "2026-10-12T12:00:00.0000000Z")
	fab.CotacaoMoeda(func(c *testeapoio.CotacaoMoeda) { c.Data = dia(14) })
	mux := http.NewServeMux()
	log, _ := logDoTeste()
	RegistrarCom(mux, s, log)
	return mux, fab
}

func TestRotasDoCambio(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome, caminho string
		status        int
		corpo         string
	}{
		{"cotação do dia", "/api/cambio/cotacao?moeda=USD", 200,
			`{"moeda":"USD","dataSolicitada":"2026-10-14","data":"2026-10-14","compra":5.4123,"venda":5.4129,"fonte":"PTAX/Banco Central"}`},
		{"o nome do parâmetro não diferencia maiúsculas, e o código é normalizado", "/api/cambio/cotacao?MOEDA=%20usd&Data=2026-10-14", 200,
			`{"moeda":"USD","dataSolicitada":"2026-10-14","data":"2026-10-14","compra":5.4123,"venda":5.4129,"fonte":"PTAX/Banco Central"}`},
		{"real, sem olhar a lista do Banco Central", "/api/cambio/cotacao?moeda=BRL&data=2026-10-10", 200,
			`{"moeda":"BRL","dataSolicitada":"2026-10-10","data":"2026-10-10","compra":1,"venda":1,"fonte":"Fixa"}`},
		{"data no formato americano, como o model binding aceita", "/api/cambio/cotacao?moeda=USD&data=10/14/2026", 200,
			`{"moeda":"USD","dataSolicitada":"2026-10-14","data":"2026-10-14","compra":5.4123,"venda":5.4129,"fonte":"PTAX/Banco Central"}`},
		{"data vazia vale hoje", "/api/cambio/cotacao?moeda=USD&data=", 200,
			`{"moeda":"USD","dataSolicitada":"2026-10-14","data":"2026-10-14","compra":5.4123,"venda":5.4129,"fonte":"PTAX/Banco Central"}`},
		{"sem moeda", "/api/cambio/cotacao", 400, `{"mensagem":"Informe a moeda com o código de 3 letras (ex.: USD, EUR)."}`},
		{"moeda com 4 letras", "/api/cambio/cotacao?moeda=USDT", 400, `{"mensagem":"Informe a moeda com o código de 3 letras (ex.: USD, EUR)."}`},
		{"moeda que o Banco Central não publica", "/api/cambio/cotacao?moeda=XYZ", 400, `{"mensagem":"O Banco Central não publica cotação para XYZ."}`},
		{"sem cotação em 7 dias", "/api/cambio/cotacao?moeda=EUR&data=2026-10-13", 503,
			`{"mensagem":"Cotação de EUR indisponível no momento. Tente novamente mais tarde."}`},
		{"moedas", "/api/cambio/moedas", 200, `[{"codigo":"EUR","nome":"Euro"},{"codigo":"USD","nome":"Dolar"}]`},
		{"cotações guardadas", "/api/cambio/cotacoes", 200,
			`{"atualizadoEm":null,"cotacoes":[{"moeda":"USD","dataSolicitada":"2026-10-14","data":"2026-10-14","compra":5.4123,"venda":5.4129,"fonte":"PTAX/Banco Central"}]}`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			mux, _ := rotasDeTeste(t, &fonteFalsa{falham: map[string]bool{"EUR": true}})
			resposta := testeapoio.Chamar(t, mux, "GET", c.caminho, "")
			resposta.ConferirStatus(t, c.status)
			resposta.ConferirJSON(t, c.corpo)
			if tipo := resposta.Cabecalho.Get("Content-Type"); tipo != "application/json; charset=utf-8" {
				t.Errorf("Content-Type %q", tipo)
			}
		})
	}
}

func TestDataInvalidaEhO400DoModelBinding(t *testing.T) {
	t.Parallel()
	mux, _ := rotasDeTeste(t, &fonteFalsa{})
	// A data é conferida antes da moeda (o model binding roda antes da ação): mesmo sem moeda, o erro é da data.
	resposta := testeapoio.Chamar(t, mux, "GET", "/api/cambio/cotacao?data=abc", "")
	resposta.ConferirStatus(t, http.StatusBadRequest)
	var corpo map[string]any
	if err := json.Unmarshal(resposta.Corpo, &corpo); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^00-[0-9a-f]{32}-[0-9a-f]{16}-00$`).MatchString(corpo["traceId"].(string)) {
		t.Errorf("traceId %v fora do formato do Activity.Id do .NET", corpo["traceId"])
	}
	delete(corpo, "traceId")
	semTrace, _ := json.Marshal(corpo)
	testeapoio.ConferirJSON(t, semTrace, `{"type":"https://tools.ietf.org/html/rfc9110#section-15.5.1",`+
		`"title":"One or more validation errors occurred.","status":400,"errors":{"data":["The value 'abc' is not valid for data."]}}`)
}

func TestRotasDoCambioSemLogin(t *testing.T) {
	t.Parallel()
	mux, _ := rotasDeTeste(t, &fonteFalsa{})
	for _, caminho := range []string{"/api/cambio/moedas", "/api/cambio/cotacoes", "/api/cambio/cotacao?moeda=USD"} {
		resposta := testeapoio.ChamarComo(t, mux, 0, "GET", caminho, "")
		resposta.ConferirStatus(t, http.StatusUnauthorized)
	}
}
