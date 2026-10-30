package cripto

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Testes que faltavam, achados pelo teste de mutação (#422).

// Regra: a URL base sem a barra no fim ganha a barra, como o BaseAddress do HttpClient (sem ela, o "tickers" trocaria
// o último pedaço do caminho).
func TestURLBaseSemBarraNoFim(t *testing.T) {
	t.Parallel()
	mercado := testeapoio.NovoServicoDeFora(t)
	mercado.Em("GET", "*", testeapoio.RespostaDeFora{Corpo: `[]`})
	c, err := NovoCliente(mercado.URL()+"/api/v4", mercado.Cliente())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.UltimosNegocios(context.Background(), []string{"BTC"}); err != nil {
		t.Fatal(err)
	}
	if pedido := mercado.URIs()[0]; pedido != "/api/v4/tickers?symbols=BTC-BRL" {
		t.Errorf("endereço %s", pedido)
	}
}

// Regra: as bordas da resposta, como o EnsureSuccessStatusCode e o limite de tamanho: status 299 vale, e o corpo com
// exatamente o tamanho máximo também.
func TestRespostaNasBordas(t *testing.T) {
	t.Parallel()
	item := `[{"pair":"BTC-BRL","last":"1","date":1792858672}]`
	casos := []struct {
		nome   string
		status int
		corpo  string
	}{
		{"status 299", 299, item},
		// O espaço vai antes: cortado um byte do fim, o JSON quebra.
		{"corpo com o tamanho máximo exato", http.StatusOK, strings.Repeat(" ", tamanhoMaximo-len(item)) + item},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			cliente, _ := mercadoGravado(t, testeapoio.RespostaDeFora{Status: c.status, Corpo: c.corpo})
			negocios, err := cliente.UltimosNegocios(context.Background(), []string{"BTC"})
			if err != nil || len(negocios) != 1 {
				t.Errorf("%d moedas, erro %v", len(negocios), err)
			}
		})
	}
}

// Regra: o item ignorado (moeda que não é acompanhada, preço que não vale, data zero) não interrompe a leitura dos
// seguintes.
func TestItemIgnoradoAntesDeUmValido(t *testing.T) {
	t.Parallel()
	corpo := `[
		{"pair":"DOGE-BRL","last":"1","date":1792858672},
		{"pair":"BTC-BRL","last":"0","date":1792858672},
		{"pair":"ETH-BRL","last":"1","date":0},
		{"pair":"BTC-BRL","last":"2","date":1792858672},
		{"pair":"SOL-BRL","last":"3","date":1792858672}
	]`
	c, _ := mercadoGravado(t, testeapoio.RespostaDeFora{Corpo: corpo})
	negocios, err := c.UltimosNegocios(context.Background(), Moedas)
	if err != nil {
		t.Fatal(err)
	}
	if len(negocios) != 2 || negocios["BTC"].Preco.String() != "2" || negocios["SOL"].Preco.String() != "3" {
		t.Errorf("lidas %v, esperado BTC 2 e SOL 3 (o ETH com data zero fica de fora)", negocios)
	}
}

// Regra: a moeda que faltou na resposta ou que já está guardada não impede de gravar as seguintes; e sem falha não
// há aviso no log.
func TestAtualizarSegueDepoisDeUmaQueNaoGrava(t *testing.T) {
	t.Parallel()
	agora := relogio.Fixo{Instante: utc(15, 1, 0)}
	negocios := map[string]Negocio{ // falta o BTC, o primeiro da lista
		"ETH":  {Preco: testeapoio.Valor("11594"), DataHora: utc(14, 21, 0)},
		"SOL":  {Preco: testeapoio.Valor("900.5"), DataHora: utc(14, 21, 0)},
		"USDT": {Preco: testeapoio.Valor("5.4"), DataHora: utc(14, 21, 0)},
	}
	s, _, _, _ := novoServicoDeTeste(t, agora, &fonteFalsa{negocios: negocios})
	if r, err := s.Atualizar(t.Context()); err != nil || r.Novas != 3 {
		t.Fatalf("faltando o BTC: %+v, %v", r, err)
	}

	// Agora vêm as 4: o ETH, o SOL e o USDT já estão guardados, e o BTC, depois deles na conferência, é novo.
	negocios["BTC"] = Negocio{Preco: testeapoio.Valor("618900.12"), DataHora: utc(14, 21, 0)}
	s, db, fab, saida := novoServicoDeTeste(t, agora, &fonteFalsa{negocios: negocios})
	fab.CotacaoCripto(func(c *testeapoio.CotacaoCripto) { c.Moeda, c.DataHora = "BTC", tipos.NovaDataHora(utc(14, 21, 0)) })
	r, err := s.Atualizar(t.Context())
	if err != nil || r.Novas != 3 || len(r.Falhas) != 0 {
		t.Fatalf("com o BTC já guardado: %+v, %v", r, err)
	}
	if n := contar(t, db, `SELECT COUNT(*) FROM CotacaoCripto WHERE Moeda IN ('ETH','SOL','USDT')`); n != 3 {
		t.Errorf("gravou %d das moedas depois do BTC, esperado 3", n)
	}
	if strings.Contains(saida.String(), "não devolveu") {
		t.Errorf("aviso sem moeda faltando:\n%s", saida.String())
	}
}

// Regra: a moeda sem nada nos últimos 2 dias usa a última guardada, mesmo que outra moeda antes dela tenha cotação
// recente.
func TestUltimasComUmaRecenteEOutraAntiga(t *testing.T) {
	t.Parallel()
	s, _, fab, _ := novoServicoDeTeste(t, relogio.Fixo{Instante: utc(15, 1, 0)}, &fonteFalsa{})
	fab.CotacaoCripto(func(c *testeapoio.CotacaoCripto) {
		c.Moeda, c.DataHora, c.Preco = "BTC", tipos.NovaDataHora(utc(14, 21, 0)), testeapoio.Valor("618900.12")
	})
	fab.CotacaoCripto(func(c *testeapoio.CotacaoCripto) {
		c.Moeda, c.DataHora, c.Preco = "ETH", tipos.NovaDataHora(utc(1, 9, 30)), testeapoio.Valor("11594")
	})
	ultimas, err := s.Ultimas(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	obtido, _ := json.Marshal(ultimas.Cotacoes)
	testeapoio.ConferirJSON(t, obtido,
		`[{"moeda":"BTC","preco":618900.12,"dataHora":"2026-10-14T21:00:00Z","variacao24h":null,"fonte":"Mercado Bitcoin"},`+
			`{"moeda":"ETH","preco":11594.0,"dataHora":"2026-10-01T09:30:00Z","variacao24h":null,"fonte":"Mercado Bitcoin"}]`)
}

// Regra: entre as candidatas da janela de 22 a 26 h, vale a mais perto de 24 h antes, mesmo que seja a mais antiga.
func TestVariacaoContraAMaisPertoDe24h(t *testing.T) {
	t.Parallel()
	s, _, fab, _ := novoServicoDeTeste(t, relogio.Fixo{Instante: utc(15, 1, 0)}, &fonteFalsa{})
	for _, c := range []testeapoio.CotacaoCripto{
		{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(13, 20, 0)), Preco: testeapoio.Valor("100")},  // 25 h: 1 h do alvo
		{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(13, 22, 30)), Preco: testeapoio.Valor("200")}, // 22 h e 30: 1 h e 30
		{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(14, 21, 0)), Preco: testeapoio.Valor("150")},
	} {
		fab.CotacaoCripto(func(x *testeapoio.CotacaoCripto) { x.Moeda, x.DataHora, x.Preco = c.Moeda, c.DataHora, c.Preco })
	}
	ultimas, err := s.Ultimas(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	obtido, _ := json.Marshal(ultimas.Cotacoes)
	testeapoio.ConferirJSON(t, obtido,
		`[{"moeda":"BTC","preco":150.0,"dataHora":"2026-10-14T21:00:00Z","variacao24h":50.0,"fonte":"Mercado Bitcoin"}]`)
}
