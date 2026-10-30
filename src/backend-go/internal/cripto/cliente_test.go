package cripto

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// testdata/tickers.json foi gravado do Mercado Bitcoin de verdade (api.mercadobitcoin.net), em 24/10/2026, com os 4
// pares: os preços vêm como texto, e "date" em segundos (10 dígitos), não em nanossegundos como diz a especificação.

// mercadoGravado é o Mercado Bitcoin num serviço falso (testeapoio.ServicoDeFora, #460) que dá a mesma resposta a
// todo pedido e confere os cabeçalhos do .NET.
func mercadoGravado(t *testing.T, resposta testeapoio.RespostaDeFora) (*Cliente, *testeapoio.ServicoDeFora) {
	t.Helper()
	mercado := testeapoio.NovoServicoDeFora(t)
	mercado.Exigir("User-Agent", "FinancasLocal/1.0")
	mercado.Exigir("Accept", "application/json")
	mercado.Em("GET", "*", resposta)
	c, err := NovoCliente(mercado.URL()+"/api/v4/", mercado.Cliente())
	if err != nil {
		t.Fatal(err)
	}
	return c, mercado
}

func TestUltimosNegociosGravados(t *testing.T) {
	t.Parallel()
	c, mercado := mercadoGravado(t, testeapoio.RespostaDeFora{Arquivo: "tickers.json"})
	negocios, err := c.UltimosNegocios(context.Background(), Moedas)
	if err != nil {
		t.Fatal(err)
	}
	if mercado.URIs()[0] != "/api/v4/tickers?symbols=BTC-BRL,ETH-BRL,SOL-BRL,USDT-BRL" {
		t.Errorf("endereço %s", mercado.URIs()[0])
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
	c, _ := mercadoGravado(t, testeapoio.RespostaDeFora{Corpo: corpo})
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
		nome     string
		resposta testeapoio.RespostaDeFora
		deRede   bool // a HttpRequestException do .NET: a tarefa avisa e tenta de novo
	}{
		{"erro 503", testeapoio.RespostaDeFora{Status: http.StatusServiceUnavailable}, true},
		{"muitos pedidos", testeapoio.MuitosPedidos, true},
		{"objeto em vez de lista", testeapoio.RespostaDeFora{Corpo: `{"erro":"x"}`}, false},
		{"não é JSON", testeapoio.RespostaDeFora{Corpo: `<html>`, TipoConteudo: "text/html"}, false},
		{"JSON quebrado", testeapoio.JSONQuebrado, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			cliente, _ := mercadoGravado(t, c.resposta)
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
	mercado := testeapoio.NovoServicoDeFora(t)
	mercado.Em("GET", "*", testeapoio.TempoEsgotado)
	cliente := mercado.Cliente()
	cliente.Timeout = 50 * time.Millisecond
	c, err := NovoCliente(mercado.URL(), cliente)
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
