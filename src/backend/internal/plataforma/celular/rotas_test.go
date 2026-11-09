package celular

import (
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

func (a *ambiente) rotas() *http.ServeMux {
	mux := http.NewServeMux()
	Registrar(mux, a.s)
	return mux
}

// Contrato das rotas (#328): a situação, ligar e desligar respondem a Situacao; o pedido errado é 400 com o motivo; a
// falha que a pessoa resolve (porta, firewall) é 409 com o que fazer; o resto é o 500 de sempre.
func TestRotas(t *testing.T) {
	a := novoAmbiente(t)
	mux := a.rotas()
	desligado := `{"ligado":false,"ip":null,"endereco":null,"ipIndisponivel":false,"impressaoDigital":null,"avisos":[],
		"enderecos":[{"ip":"192.168.0.10","interface":"Wi-Fi","rede":"192.168.0.0/24"},{"ip":"10.0.0.20","interface":"Ethernet","rede":"10.0.0.0/8"}]}`

	r := testeapoio.Chamar(t, mux, "GET", "/api/celular", "")
	r.ConferirStatus(t, 200)
	r.ConferirJSON(t, desligado)

	t.Run("pedido sem o IP", func(t *testing.T) {
		r := testeapoio.Chamar(t, mux, "POST", "/api/celular/ligar", `{}`)
		r.ConferirStatus(t, 400)
		// O traceId é novo a cada resposta (o formato é conferido no pacote respostas).
		corpo := regexp.MustCompile(`"traceId":"[^"]*"`).ReplaceAll(r.Corpo, []byte(`"traceId":"<traceId>"`))
		testeapoio.ConferirJSON(t, corpo, `{"type":"https://tools.ietf.org/html/rfc9110#section-15.5.1","title":"One or more validation errors occurred.",
			"status":400,"errors":{"Ip":["The Ip field is required."]},"traceId":"<traceId>"}`)
	})
	t.Run("IP que não é deste computador", func(t *testing.T) {
		r := testeapoio.Chamar(t, mux, "POST", "/api/celular/ligar", `{"ip":"8.8.8.8"}`)
		r.ConferirStatus(t, 400)
		r.ConferirJSON(t, `{"mensagem":"`+MensagemEnderecoInvalido+`"}`)
	})
	t.Run("a porta não abre", func(t *testing.T) {
		a.errEscutar = errors.New("em uso")
		defer func() { a.errEscutar = nil }()
		r := testeapoio.Chamar(t, mux, "POST", "/api/celular/ligar", `{"ip":"192.168.0.10"}`)
		r.ConferirStatus(t, 409)
		r.ConferirJSON(t, `{"mensagem":"`+MensagemEscutar+`"}`)
	})
	t.Run("o firewall não abre", func(t *testing.T) {
		a.fw.errAbrir = errors.New("requer elevação")
		defer func() { a.fw.errAbrir = nil }()
		r := testeapoio.Chamar(t, mux, "POST", "/api/celular/ligar", `{"ip":"192.168.0.10"}`)
		r.ConferirStatus(t, 409)
		r.ConferirJSON(t, `{"mensagem":"`+MensagemFirewall+`"}`)
	})
	t.Run("erro interno", func(t *testing.T) {
		a.errMontar = errors.New("sem index.html")
		defer func() { a.errMontar = nil }()
		r := testeapoio.Chamar(t, mux, "POST", "/api/celular/ligar", `{"ip":"192.168.0.10"}`)
		r.ConferirStatus(t, 500)
		r.ConferirJSON(t, `{"mensagem":"Ocorreu um erro interno. Ele foi registrado em Configurações › Erros."}`)
	})

	// Desligado, não há autoridade para baixar.
	testeapoio.Chamar(t, mux, "GET", RotaAutoridade, "").ConferirStatus(t, 404)

	r = testeapoio.Chamar(t, mux, "POST", "/api/celular/ligar", `{"ip":"192.168.0.10"}`)
	r.ConferirStatus(t, 200)
	sit := a.s.Situacao(t.Context())
	r.ConferirJSON(t, `{"ligado":true,"ip":"192.168.0.10","endereco":"https://192.168.0.10:5443","ipIndisponivel":false,
		"impressaoDigital":"`+*sit.ImpressaoDigital+`","avisos":[],
		"enderecos":[{"ip":"192.168.0.10","interface":"Wi-Fi","rede":"192.168.0.0/24"},{"ip":"10.0.0.20","interface":"Ethernet","rede":"10.0.0.0/8"}]}`)

	// A autoridade, sem login (é só o certificado público): o .crt que o celular instala.
	r = testeapoio.ChamarComo(t, mux, 0, "GET", RotaAutoridade, "")
	r.ConferirStatus(t, 200)
	if r.Cabecalho.Get("Content-Type") != "application/x-x509-ca-cert" ||
		r.Cabecalho.Get("Content-Disposition") != `attachment; filename="financas-local-celular.crt"` ||
		r.Cabecalho.Get("Cache-Control") != "no-store" {
		t.Errorf("cabeçalhos %v", r.Cabecalho)
	}
	if c, err := x509.ParseCertificate(r.Corpo); err != nil || !c.IsCA {
		t.Errorf("o corpo não é a autoridade: %v", err)
	}

	t.Run("desligar com o firewall que falha", func(t *testing.T) {
		a.fw.errFechar = errors.New("negado")
		defer func() { a.fw.errFechar = nil }()
		r := testeapoio.Chamar(t, mux, "POST", "/api/celular/desligar", "")
		r.ConferirStatus(t, 409)
		r.ConferirJSON(t, `{"mensagem":"`+MensagemFirewall+`"}`)
	})
	r = testeapoio.Chamar(t, mux, "POST", "/api/celular/desligar", "")
	r.ConferirStatus(t, 200)
	r.ConferirJSON(t, desligado)
}

// Regra (#679): o estado curto, sem login, responde só ao próprio computador (127.0.0.1 ou ::1), com ligado e o
// endereço; de outro aparelho da rede, 404, como se a rota não existisse.
func TestEstadoParaOGerenciador(t *testing.T) {
	a := novoAmbiente(t)
	mux := a.rotas()
	pedir := func(remoto string) *httptest.ResponseRecorder {
		r := httptest.NewRequestWithContext(t.Context(), "GET", RotaEstado, nil)
		r.RemoteAddr = remoto
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	for _, remoto := range []string{"192.168.0.20:50000", "10.0.0.20:443", "sem-porta", "[fe80::1]:1"} {
		if w := pedir(remoto); w.Code != 404 || w.Body.Len() != 0 {
			t.Errorf("de %s: %d %s", remoto, w.Code, w.Body)
		}
	}
	w := pedir("127.0.0.1:50000")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("desligado: %d %v", w.Code, w.Header())
	}
	testeapoio.ConferirJSON(t, w.Body.Bytes(), `{"ligado":false,"endereco":null}`)

	if _, err := a.s.Ligar(t.Context(), a.pedido(), "192.168.0.10"); err != nil {
		t.Fatal(err)
	}
	w = pedir("[::1]:50000")
	if w.Code != 200 {
		t.Fatalf("ligado: %d", w.Code)
	}
	testeapoio.ConferirJSON(t, w.Body.Bytes(), `{"ligado":true,"endereco":"https://192.168.0.10:5443"}`)
}
