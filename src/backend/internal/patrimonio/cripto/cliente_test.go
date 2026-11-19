package cripto

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// testdata/ticker-24hr.json foi gravado da Binance de verdade (api.binance.com), em 18/11/2026, com os 4 pares
// (ticker de 24 h, type=MINI): os preços vêm como texto, e "closeTime" em milissegundos (#762).

// binanceGravada é a Binance num serviço falso (testeapoio.ServicoDeFora, #460) que dá a mesma resposta a todo pedido
// e confere os cabeçalhos do .NET.
func binanceGravada(t *testing.T, resposta testeapoio.RespostaDeFora) (*Cliente, *testeapoio.ServicoDeFora) {
	t.Helper()
	binance := testeapoio.NovoServicoDeFora(t)
	binance.Exigir("User-Agent", "FinancasLocal/1.0")
	binance.Exigir("Accept", "application/json")
	binance.Em("GET", "*", resposta)
	c, err := NovoCliente(binance.URL()+"/api/v3/", binance.Cliente())
	if err != nil {
		t.Fatal(err)
	}
	return c, binance
}

func TestUltimosNegociosGravados(t *testing.T) {
	t.Parallel()
	c, binance := binanceGravada(t, testeapoio.RespostaDeFora{Arquivo: "ticker-24hr.json"})
	negocios, err := c.UltimosNegocios(context.Background(), Moedas)
	if err != nil {
		t.Fatal(err)
	}
	if binance.URIs()[0] != "/api/v3/ticker/24hr?symbols=%5B%22BTCBRL%22%2C%22ETHBRL%22%2C%22SOLBRL%22%2C%22USDTBRL%22%5D&type=MINI" {
		t.Errorf("endereço %s", binance.URIs()[0])
	}
	if len(negocios) != 4 {
		t.Fatalf("%d moedas, esperado 4: %v", len(negocios), negocios)
	}
	btc := negocios["BTC"]
	if btc.Preco.String() != "372650.00000000" || !btc.DataHora.Equal(time.Unix(1795050536, 0)) {
		t.Errorf("BTC %s em %v", btc.Preco, btc.DataHora)
	}
}

func TestUltimosNegociosIgnoraOQueOdotnetIgnora(t *testing.T) {
	t.Parallel()
	agora := time.Date(2026, 10, 14, 21, 0, 0, 0, time.UTC)
	segundos := agora.Unix()
	corpo := `[
		{"symbol":"BTCBRL","lastPrice":"618900.12","closeTime":` + itoa(segundos) + `},
		{"symbol":"ETHBRL","lastPrice":12000.5,"closeTime":` + itoa(segundos*1000) + `},
		{"symbol":"SOLBRL","lastPrice":"900","closeTime":` + itoa(segundos*1_000_000_000) + `},
		{"symbol":"USDTBRL","lastPrice":"0","closeTime":` + itoa(segundos) + `},
		{"symbol":"DOGEBRL","lastPrice":"1","closeTime":` + itoa(segundos) + `},
		{"symbol":"BTCUSD","lastPrice":"1","closeTime":` + itoa(segundos) + `},
		{"symbol":"XRPBRL","lastPrice":"1","closeTime":"` + itoa(segundos) + `"}
	]`
	c, _ := binanceGravada(t, testeapoio.RespostaDeFora{Corpo: corpo})
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
	// #762: a página do Cloudflare que o Mercado Bitcoin devolve desde 14/11/2026, a todo pedido.
	cloudflare := testeapoio.RespostaDeFora{Status: http.StatusForbidden, TipoConteudo: "text/html; charset=UTF-8",
		Corpo: `<!DOCTYPE html><html><head><title>Attention Required! | Cloudflare</title></head></html>`}
	casos := []struct {
		nome     string
		resposta testeapoio.RespostaDeFora
		deRede   bool // a HttpRequestException do .NET: a tarefa avisa e tenta de novo
		recusa   bool // a fonte recusou (4xx): a tarefa registra o erro uma vez
	}{
		{"erro 503", testeapoio.RespostaDeFora{Status: http.StatusServiceUnavailable}, true, false},
		{"erro 500, a borda de baixo do 5xx", testeapoio.RespostaDeFora{Status: http.StatusInternalServerError}, true, false},
		{"400, a borda de baixo do 4xx", testeapoio.RespostaDeFora{Status: http.StatusBadRequest}, false, true},
		{"IP banido depois do limite (418)", testeapoio.RespostaDeFora{Status: http.StatusTeapot}, false, true},
		{"país bloqueado (451)", testeapoio.RespostaDeFora{Status: http.StatusUnavailableForLegalReasons}, false, true},
		{"muitos pedidos", testeapoio.MuitosPedidos, true, false},
		{"tempo esgotado do lado de lá (408)", testeapoio.RespostaDeFora{Status: http.StatusRequestTimeout}, true, false},
		{"403 do Cloudflare", cloudflare, false, true},
		{"caminho que sumiu (404)", testeapoio.RespostaDeFora{Status: http.StatusNotFound}, false, true},
		{"passou a pedir chave (401)", testeapoio.RespostaDeFora{Status: http.StatusUnauthorized}, false, true},
		{"objeto em vez de lista", testeapoio.RespostaDeFora{Corpo: `{"erro":"x"}`}, false, false},
		{"não é JSON", testeapoio.RespostaDeFora{Corpo: `<html>`, TipoConteudo: "text/html"}, false, false},
		{"JSON quebrado", testeapoio.JSONQuebrado, false, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			cliente, _ := binanceGravada(t, c.resposta)
			_, err := cliente.UltimosNegocios(context.Background(), Moedas)
			if err == nil {
				t.Fatal("esperado erro")
			}
			if errors.Is(err, ErrRede) != c.deRede {
				t.Errorf("erro de rede: %v, esperado %v (%v)", errors.Is(err, ErrRede), c.deRede, err)
			}
			if errors.Is(err, ErrRecusa) != c.recusa {
				t.Errorf("recusa: %v, esperado %v (%v)", errors.Is(err, ErrRecusa), c.recusa, err)
			}
		})
	}
}

func TestUltimosNegociosComTempoEsgotado(t *testing.T) {
	t.Parallel()
	binance := testeapoio.NovoServicoDeFora(t)
	binance.Em("GET", "*", testeapoio.TempoEsgotado)
	cliente := binance.Cliente()
	cliente.Timeout = 50 * time.Millisecond
	c, err := NovoCliente(binance.URL(), cliente)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.UltimosNegocios(context.Background(), Moedas); !errors.Is(err, ErrRede) {
		t.Errorf("tempo esgotado tem de ser erro de rede: %v", err)
	}
}

func TestClienteSoHTTPS(t *testing.T) {
	t.Parallel()
	if _, err := NovoCliente("http://api.binance.com/api/v3/", nil); err == nil {
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
