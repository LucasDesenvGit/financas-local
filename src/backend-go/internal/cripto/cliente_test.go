package cripto

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

// testdata/tickers.json foi gravado do Mercado Bitcoin de verdade (api.mercadobitcoin.net), em 24/10/2026, com os 4
// pares: os preços vêm como texto, e "date" em segundos (10 dígitos), não em nanossegundos como diz a especificação.

func mercadoGravado(t *testing.T, responder func(http.ResponseWriter, *http.Request)) (*Cliente, *[]string) {
	t.Helper()
	pedidos := &[]string{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*pedidos = append(*pedidos, r.URL.RequestURI())
		if r.Header.Get("User-Agent") != "FinancasLocal/1.0" || r.Header.Get("Accept") != "application/json" {
			t.Errorf("cabeçalhos diferentes do .NET: %v", r.Header)
		}
		responder(w, r)
	}))
	t.Cleanup(srv.Close)
	c, err := NovoCliente(srv.URL+"/api/v4/", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	return c, pedidos
}

func TestUltimosNegociosGravados(t *testing.T) {
	t.Parallel()
	corpo, err := os.ReadFile("testdata/tickers.json")
	if err != nil {
		t.Fatal(err)
	}
	c, pedidos := mercadoGravado(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(corpo) })
	negocios, err := c.UltimosNegocios(context.Background(), Moedas)
	if err != nil {
		t.Fatal(err)
	}
	if (*pedidos)[0] != "/api/v4/tickers?symbols=BTC-BRL,ETH-BRL,SOL-BRL,USDT-BRL" {
		t.Errorf("endereço %s", (*pedidos)[0])
	}
	if len(negocios) != 4 {
		t.Fatalf("%d moedas, esperado 4: %v", len(negocios), negocios)
	}
	btc := negocios["BTC"]
	if btc.Preco.String() != "400555.00000000" || !btc.DataHora.Equal(time.Unix(1792858672, 0)) {
		t.Errorf("BTC %s em %v", btc.Preco, btc.DataHora)
	}
}

func TestUltimosNegociosIgnoraOQueOdotnetIgnora(t *testing.T) {
	t.Parallel()
	agora := time.Date(2026, 10, 14, 21, 0, 0, 0, time.UTC)
	segundos := agora.Unix()
	corpo := `[
		{"pair":"BTC-BRL","last":"618900.12","date":` + itoa(segundos) + `},
		{"pair":"ETH-BRL","last":12000.5,"date":` + itoa(segundos*1000) + `},
		{"pair":"SOL-BRL","last":"900","date":` + itoa(segundos*1_000_000_000) + `},
		{"pair":"USDT-BRL","last":"0","date":` + itoa(segundos) + `},
		{"pair":"DOGE-BRL","last":"1","date":` + itoa(segundos) + `},
		{"pair":"BTC-USD","last":"1","date":` + itoa(segundos) + `},
		{"pair":"XRP-BRL","last":"1","date":"` + itoa(segundos) + `"}
	]`
	c, _ := mercadoGravado(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(corpo)) })
	negocios, err := c.UltimosNegocios(context.Background(), Moedas)
	if err != nil {
		t.Fatal(err)
	}
	casos := []struct {
		moeda, preco string
		ok           bool
	}{
		{"BTC", "618900.12", true}, // segundos
		{"ETH", "12000.5", true},   // milissegundos, preço como número
		{"SOL", "900", true},       // nanossegundos
		{"USDT", "", false},        // preço zero não vale
	}
	for _, c := range casos {
		n, ok := negocios[c.moeda]
		if ok != c.ok {
			t.Errorf("%s: lida = %v, esperado %v", c.moeda, ok, c.ok)
			continue
		}
		if ok && (n.Preco.String() != c.preco || !n.DataHora.Equal(agora)) {
			t.Errorf("%s: %s em %v, esperado %s em %v", c.moeda, n.Preco, n.DataHora, c.preco, agora)
		}
	}
	if len(negocios) != 3 {
		t.Errorf("leu %d moedas (DOGE, o par em USD e a data em texto ficam de fora): %v", len(negocios), negocios)
	}
}

func TestUltimosNegociosComFalha(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome      string
		responder func(http.ResponseWriter, *http.Request)
		deRede    bool // a HttpRequestException do .NET: a tarefa avisa e tenta de novo
	}{
		{"erro 503", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }, true},
		{"objeto em vez de lista", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"erro":"x"}`)) }, false},
		{"não é JSON", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`<html>`)) }, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			cliente, _ := mercadoGravado(t, c.responder)
			_, err := cliente.UltimosNegocios(context.Background(), Moedas)
			if err == nil {
				t.Fatal("esperado erro")
			}
			if errors.Is(err, ErrRede) != c.deRede {
				t.Errorf("erro de rede: %v, esperado %v (%v)", errors.Is(err, ErrRede), c.deRede, err)
			}
		})
	}
}

func TestUltimosNegociosComTempoEsgotado(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	t.Cleanup(srv.Close)
	cliente := srv.Client()
	cliente.Timeout = 50 * time.Millisecond
	c, err := NovoCliente(srv.URL, cliente)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.UltimosNegocios(context.Background(), Moedas); !errors.Is(err, ErrRede) {
		t.Errorf("tempo esgotado tem de ser erro de rede: %v", err)
	}
}

func TestClienteSoHTTPS(t *testing.T) {
	t.Parallel()
	if _, err := NovoCliente("http://api.mercadobitcoin.net/api/v4/", nil); err == nil {
		t.Error("HTTP sem S aceito")
	}
	c, err := NovoCliente(URLPadrao, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.http.Timeout != 20*time.Second {
		t.Errorf("tempo limite %v, o .NET usa 20 s", c.http.Timeout)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
