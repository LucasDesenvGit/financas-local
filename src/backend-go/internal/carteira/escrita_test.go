package carteira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

const rotaOperacoes = "/api/carteira/operacoes"

func corpoDaOperacao(campos map[string]any) string {
	base := map[string]any{"ticker": "PETR4", "tipo": 1, "data": "2026-10-01", "quantidade": 10, "precoUnitario": 20.5, "taxas": 1}
	for k, v := range campos {
		if v == nil {
			delete(base, k)
		} else {
			base[k] = v
		}
	}
	dados, _ := json.Marshal(base)
	return string(dados)
}

func umaLinha(t *testing.T, c cena, consulta string, argumentos ...any) string {
	t.Helper()
	linhas, err := c.db.Query(consulta, argumentos...)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	colunas, err := linhas.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var saida []string
	for linhas.Next() {
		valores := make([]any, len(colunas))
		ponteiros := make([]any, len(colunas))
		for i := range valores {
			ponteiros[i] = &valores[i]
		}
		if err := linhas.Scan(ponteiros...); err != nil {
			t.Fatal(err)
		}
		partes := make([]string, len(valores))
		for i, v := range valores {
			partes[i] = fmt.Sprint(v)
		}
		saida = append(saida, strings.Join(partes, " | "))
	}
	return strings.Join(saida, "\n")
}

// Regra: criar uma operação: as mensagens do controller, na ordem dele (moeda, código, operação, moeda do ativo, câmbio,
// venda), e nada gravado.
func TestCriarOperacaoRecusada(t *testing.T) {
	t.Parallel()
	dolar := []cambio.Moeda{{Codigo: "USD", Nome: "Dólar"}}
	casos := []struct {
		nome     string
		campos   map[string]any
		mensagem string
	}{
		{"moeda inválida", map[string]any{"moeda": "U1"}, "Moeda inválida. Use o código de 3 letras (ex.: USD)."},
		{"código da B3 inválido", map[string]any{"ticker": "PE"}, "Código inválido. Use o código da B3, por exemplo MXRF11 ou PETR4."},
		{"código do exterior inválido", map[string]any{"ticker": "PETR4", "moeda": "USD"}, "Código inválido. Use o código como no Yahoo Finance, por exemplo AAPL, VOO ou SAP.DE (ativos da B3 ficam em reais)."},
		{"tipo de operação zero", map[string]any{"tipo": 0}, "Tipo de operação inválido."},
		{"tipo de operação sem vir", map[string]any{"tipo": nil}, "Tipo de operação inválido."},
		{"tipo de operação 3", map[string]any{"tipo": 3}, "Tipo de operação inválido."},
		{"tipo de ativo zero", map[string]any{"tipoAtivo": 0}, "Tipo de ativo inválido."},
		{"tipo de ativo 6", map[string]any{"tipoAtivo": 6}, "Tipo de ativo inválido."},
		{"data no futuro", map[string]any{"data": "2026-10-31"}, "A data não pode estar no futuro."},
		{"data antiga", map[string]any{"data": "1994-12-31"}, "Data inválida."},
		{"sem data", map[string]any{"data": nil}, "Data inválida."},
		{"moeda sem cotação no Banco Central", map[string]any{"ticker": "SAP.DE", "moeda": "EUR"}, "Não há cotação de câmbio do Banco Central para EUR."},
		{"FII no exterior", map[string]any{"ticker": "AAPL", "moeda": "USD", "tipoAtivo": 1}, "FIIs e BDRs são da B3: lance em reais."},
		{"BDR no exterior", map[string]any{"ticker": "AAPL", "moeda": "USD", "tipoAtivo": 4}, "FIIs e BDRs são da B3: lance em reais."},
		{"sem câmbio", map[string]any{"ticker": "AAPL", "moeda": "USD"}, "Informe o câmbio (R$ por 1 USD): não há cotação do Banco Central para essa data."},
		{"câmbio negativo não conta", map[string]any{"ticker": "AAPL", "moeda": "USD", "tipo": 1}, "Informe o câmbio (R$ por 1 USD): não há cotação do Banco Central para essa data."},
		{"venda num ativo novo", map[string]any{"tipo": 2}, "A quantidade vendida é maior do que a quantidade em carteira nessa data."},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := novaCena(t, cambioFalso{moedas: dolar}, reservasFalsas{}, selicFalsa{})
			r := testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(x.campos))
			r.ConferirStatus(t, http.StatusBadRequest)
			r.ConferirJSON(t, `{"mensagem":"`+x.mensagem+`"}`)
			if linhas := umaLinha(t, c, `SELECT (SELECT COUNT(*) FROM Ativo) + (SELECT COUNT(*) FROM OperacaoAtivo)`); linhas != "0" {
				t.Errorf("gravou: %s", linhas)
			}
		})
	}
}

// Regra: o tipo do ativo novo: o informado; no exterior, ação; na B3, pelo padrão do código.
func TestCriarOperacaoComAtivoNovo(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome   string
		campos map[string]any
		ativo  string
	}{
		{"FII pelo final 11", map[string]any{"ticker": " mxrf11 "}, "MXRF11 | 1 | BRL"},
		{"BDR pelo final 34", map[string]any{"ticker": "AAPL34"}, "AAPL34 | 4 | BRL"},
		{"ação pelo final 3", map[string]any{"ticker": "WEGE3"}, "WEGE3 | 2 | BRL"},
		{"outro pelo final 12", map[string]any{"ticker": "MXRF12"}, "MXRF12 | 5 | BRL"},
		{"o tipo informado", map[string]any{"ticker": "BOVA11", "tipoAtivo": 3}, "BOVA11 | 3 | BRL"},
		{"o fracionário vira o lote", map[string]any{"ticker": "PETR4F"}, "PETR4 | 2 | BRL"},
		{"exterior é ação", map[string]any{"ticker": "aapl", "moeda": "usd", "cambio": 5.25}, "AAPL | 2 | USD"},
		{"o tipo Outro, o último da lista", map[string]any{"ticker": "PETR4", "tipoAtivo": 5}, "PETR4 | 5 | BRL"},
		{"o tipo informado no exterior", map[string]any{"ticker": "VOO", "moeda": "USD", "cambio": 5.25, "tipoAtivo": 3}, "VOO | 3 | USD"},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			var pedidos [][]int64
			c := cenaComCotacoes(t, cambioFalso{}, reservasFalsas{}, selicFalsa{}, cotacoesFalsas{erro: errors.New("sem rede"), pedidos: &pedidos})
			r := testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(x.campos))
			r.ConferirStatus(t, http.StatusOK)
			ticker, _, _ := strings.Cut(x.ativo, " | ")
			r.ConferirJSON(t, fmt.Sprintf(`{"id":1,"ticker":%q,"ativoId":1}`, ticker))
			if a := umaLinha(t, c, `SELECT Ticker, Tipo, Moeda FROM Ativo`); a != x.ativo {
				t.Errorf("ativo %q, esperado %q", a, x.ativo)
			}
			if fmt.Sprint(pedidos) != "[[1]]" {
				t.Errorf("cotações pedidas %v", pedidos)
			}
		})
	}
}

// Regra: a operação é gravada com o câmbio certo: 1 em reais; o informado; sem ele, a PTAX da data, de venda na compra e
// de compra na venda. Observação aparada, ou nula.
func TestCriarOperacaoGrava(t *testing.T) {
	t.Parallel()
	ptax := &cambio.Cotacao{Compra: testeapoio.Valor("5.1"), Venda: testeapoio.Valor("5.2")}
	casos := []struct {
		nome     string
		campos   map[string]any
		esperado string
	}{
		{"em reais, o câmbio informado não vale", map[string]any{"cambio": 3, "observacao": "  Nota  "}, "1 | 2026-10-01 | 10.0 | 20.5 | 1.0 | 1.0 | Nota | 1"},
		{"observação em branco", map[string]any{"observacao": "   "}, "1 | 2026-10-01 | 10.0 | 20.5 | 1.0 | 1.0 | <nil> | 1"},
		{"o câmbio informado", map[string]any{"ticker": "AAPL", "moeda": "USD", "cambio": 5.25}, "1 | 2026-10-01 | 10.0 | 20.5 | 1.0 | 5.25 | <nil> | 1"},
		{"a PTAX de venda na compra", map[string]any{"ticker": "AAPL", "moeda": "USD"}, "1 | 2026-10-01 | 10.0 | 20.5 | 1.0 | 5.2 | <nil> | 1"},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := novaCena(t, cambioFalso{cotacao: ptax}, reservasFalsas{}, selicFalsa{})
			testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(x.campos)).ConferirStatus(t, http.StatusOK)
			if o := umaLinha(t, c, `SELECT Tipo, Data, Quantidade, PrecoUnitario, Taxas, Cambio, Observacao, Origem FROM OperacaoAtivo`); o != x.esperado {
				t.Errorf("operação %q, esperado %q", o, x.esperado)
			}
		})
	}
}

// Regra: no ativo que já existe, a moeda não muda, a cotação não é pedida e a venda confere a carteira na data.
func TestCriarOperacaoEmAtivoExistente(t *testing.T) {
	t.Parallel()
	ptax := &cambio.Cotacao{Compra: testeapoio.Valor("5.1"), Venda: testeapoio.Valor("5.2")}
	var pedidos [][]int64
	c := cenaComCotacoes(t, cambioFalso{cotacao: ptax}, reservasFalsas{}, selicFalsa{}, cotacoesFalsas{pedidos: &pedidos})
	petr := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "PETR4" })
	c.fab.OperacaoAtivo(petr, func(o *testeapoio.OperacaoAtivo) { o.Data = diaDoAno(5, 1) })
	aapl := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker, a.Moeda = "AAPL", "USD" })
	c.fab.OperacaoAtivo(aapl, func(o *testeapoio.OperacaoAtivo) { o.Data = diaDoAno(5, 1) })

	r := testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"moeda": "USD", "ticker": "AAPL", "data": "2026-06-01", "tipo": 2, "quantidade": 4}))
	r.ConferirStatus(t, http.StatusOK)
	if o := umaLinha(t, c, `SELECT Cambio FROM OperacaoAtivo WHERE Tipo = 2`); o != "5.1" {
		t.Errorf("a venda usa a PTAX de compra: %s", o)
	}
	r = testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"ticker": "AAPL"}))
	r.ConferirStatus(t, http.StatusBadRequest)
	r = testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"ticker": "PETR4", "moeda": "BRL", "tipo": 2, "data": "2026-04-30"}))
	r.ConferirJSON(t, `{"mensagem":"A quantidade vendida é maior do que a quantidade em carteira nessa data."}`)
	r = testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"tipo": 2, "quantidade": 10.000001}))
	r.ConferirStatus(t, http.StatusBadRequest)
	testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"tipo": 2, "quantidade": 10})).ConferirStatus(t, http.StatusOK)
	if len(pedidos) != 0 {
		t.Errorf("pediu cotação de ativo que já existia: %v", pedidos)
	}
}

// Regra: a moeda do ativo já lançado não muda: em BRL ou em USD, a mensagem cita a dele.
func TestMoedaDoAtivoNaoMuda(t *testing.T) {
	t.Parallel()
	c := novaCena(t, cambioFalso{}, reservasFalsas{}, selicFalsa{})
	c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker, a.Moeda = "AAPL", "EUR" })
	r := testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"ticker": "AAPL", "moeda": "USD", "cambio": 5}))
	r.ConferirJSON(t, `{"mensagem":"AAPL já está lançado em EUR. A moeda de um ativo não muda: use EUR."}`)
}

// Regra: com a lista de moedas vazia (sem internet), não dá para recusar a moeda.
func TestListaDeMoedasVazia(t *testing.T) {
	t.Parallel()
	c := novaCena(t, cambioFalso{}, reservasFalsas{}, selicFalsa{})
	testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"ticker": "AAPL", "moeda": "USD", "cambio": 5})).ConferirStatus(t, http.StatusOK)
}

// Regra: a venda confere em ordem cronológica: a compra antes da venda no mesmo dia; a operação nova por último; a
// sobra de arredondamento passa.
func TestVendaPossivel(t *testing.T) {
	t.Parallel()
	op := func(id, tipo int64, dia int, quantidade string) Operacao {
		return Operacao{Id: id, Tipo: tipo, Data: diaDoAno(1, dia), Quantidade: testeapoio.Valor(quantidade)}
	}
	casos := []struct {
		nome      string
		operacoes []Operacao
		esperado  bool
	}{
		{"sem operações", nil, true},
		{"compra e venda igual", []Operacao{op(1, 1, 1, "10"), op(2, 2, 2, "10")}, true},
		{"a venda antes da compra", []Operacao{op(1, 2, 1, "10"), op(2, 1, 2, "10")}, false},
		{"no mesmo dia, a compra primeiro", []Operacao{op(9, 2, 1, "10"), op(1, 1, 1, "10")}, true},
		{"a sobra de arredondamento", []Operacao{op(1, 1, 1, "10"), op(2, 2, 2, "10.0000005")}, true},
		{"um pouco a mais", []Operacao{op(1, 1, 1, "10"), op(2, 2, 2, "10.0000006")}, false},
		{"mesmo tipo e dia, a nova no meio", []Operacao{op(0, 2, 2, "10"), op(1, 1, 1, "10"), op(3, 2, 2, "1"), op(2, 1, 2, "1")}, true},
	}
	for _, x := range casos {
		if obtido := vendaPossivel(x.operacoes); obtido != x.esperado {
			t.Errorf("%s: %v", x.nome, obtido)
		}
	}
}

// Regra: alterar uma operação: não troca o ativo, e a alteração não pode deixar uma venda maior que a carteira.
func TestAtualizarOperacao(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		caminho  string
		campos   map[string]any
		status   int
		mensagem string
		depois   string
	}{
		{"altera", "/1", map[string]any{"data": "2026-05-02", "quantidade": 12, "precoUnitario": 11, "observacao": " Certo "}, 204, "", "12.0 | 11.0 | 1.0 | 1.0 | Certo"},
		{"outro ativo", "/1", map[string]any{"ticker": "WEGE3"}, 400, "Não é possível trocar o ativo de uma operação. Exclua e lance novamente.", ""},
		{"código inválido", "/1", map[string]any{"ticker": "X"}, 400, "Não é possível trocar o ativo de uma operação. Exclua e lance novamente.", ""},
		{"data no futuro", "/1", map[string]any{"data": "2026-11-01"}, 400, "A data não pode estar no futuro.", ""},
		{"a compra ficaria menor que a venda", "/1", map[string]any{"quantidade": 3}, 400, "Com essa alteração, alguma venda ficaria maior do que a quantidade em carteira.", ""},
		{"a compra iria para depois da venda", "/1", map[string]any{"data": "2026-10-02"}, 400, "Com essa alteração, alguma venda ficaria maior do que a quantidade em carteira.", ""},
		{"não existe", "/99", nil, 404, "Registro não encontrado.", ""},
		{"o id não é um int", "/abc", nil, 404, "", ""},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := novaCena(t, cambioFalso{}, reservasFalsas{}, selicFalsa{})
			ativo := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "PETR4" })
			c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "WEGE3" })
			c.fab.OperacaoAtivo(ativo, func(o *testeapoio.OperacaoAtivo) { o.Data = diaDoAno(5, 1) })
			c.fab.OperacaoAtivo(ativo, func(o *testeapoio.OperacaoAtivo) {
				o.Tipo, o.Data, o.Quantidade = 2, diaDoAno(6, 1), testeapoio.Valor("5.0")
			})
			r := testeapoio.Chamar(t, c.mux, "PUT", rotaOperacoes+x.caminho, corpoDaOperacao(x.campos))
			r.ConferirStatus(t, x.status)
			if x.mensagem != "" {
				r.ConferirJSON(t, `{"mensagem":"`+x.mensagem+`"}`)
			}
			if x.depois != "" {
				if o := umaLinha(t, c, `SELECT Quantidade, PrecoUnitario, Taxas, Cambio, Observacao FROM OperacaoAtivo WHERE Id = 1`); o != x.depois {
					t.Errorf("operação %q, esperado %q", o, x.depois)
				}
			}
		})
	}
}

// Regra: no ativo em dólar, a alteração usa o código do exterior e guarda o câmbio; sem ele e sem PTAX, é recusada.
func TestAtualizarOperacaoEmDolar(t *testing.T) {
	t.Parallel()
	c := novaCena(t, cambioFalso{}, reservasFalsas{}, selicFalsa{})
	ativo := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker, a.Moeda = "AAPL", "USD" })
	c.fab.OperacaoAtivo(ativo)
	campos := map[string]any{"ticker": "aapl"}
	r := testeapoio.Chamar(t, c.mux, "PUT", rotaOperacoes+"/1", corpoDaOperacao(campos))
	r.ConferirJSON(t, `{"mensagem":"Informe o câmbio (R$ por 1 USD): não há cotação do Banco Central para essa data."}`)
	campos["cambio"] = 5.5
	testeapoio.Chamar(t, c.mux, "PUT", rotaOperacoes+"/1", corpoDaOperacao(campos)).ConferirStatus(t, http.StatusNoContent)
	if o := umaLinha(t, c, `SELECT Cambio FROM OperacaoAtivo`); o != "5.5" {
		t.Errorf("câmbio %s", o)
	}
}

// Regra: excluir uma operação: não pode sobrar uma venda sem compra.
func TestExcluirOperacao(t *testing.T) {
	t.Parallel()
	c := novaCena(t, cambioFalso{}, reservasFalsas{}, selicFalsa{})
	ativo := c.fab.Ativo()
	c.fab.OperacaoAtivo(ativo, func(o *testeapoio.OperacaoAtivo) { o.Data = diaDoAno(5, 1) })
	c.fab.OperacaoAtivo(ativo, func(o *testeapoio.OperacaoAtivo) { o.Tipo, o.Data = 2, diaDoAno(6, 1) })

	r := testeapoio.Chamar(t, c.mux, "DELETE", rotaOperacoes+"/1", "")
	r.ConferirStatus(t, http.StatusBadRequest)
	r.ConferirJSON(t, `{"mensagem":"Sem essa compra, uma venda posterior ficaria maior do que a quantidade em carteira."}`)
	testeapoio.Chamar(t, c.mux, "DELETE", rotaOperacoes+"/99", "").ConferirStatus(t, http.StatusNotFound)
	testeapoio.Chamar(t, c.mux, "DELETE", rotaOperacoes+"/abc", "").ConferirStatus(t, http.StatusNotFound)
	testeapoio.Chamar(t, c.mux, "DELETE", rotaOperacoes+"/2", "").ConferirStatus(t, http.StatusNoContent)
	testeapoio.Chamar(t, c.mux, "DELETE", rotaOperacoes+"/1", "").ConferirStatus(t, http.StatusNoContent)
	if n := umaLinha(t, c, `SELECT COUNT(*) FROM OperacaoAtivo`); n != "0" {
		t.Errorf("sobraram %s", n)
	}
}

const rotaAtivos = "/api/carteira/ativos/"

// Regra: alterar um ativo: as mensagens do controller e o que é gravado; o preço manual guarda o anterior, e sair dele
// busca a cotação.
func TestAtualizarAtivo(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		moeda    string
		fonte    *string
		corpo    string
		status   int
		mensagem string
		depois   string
		cotacao  bool
	}{
		{nome: "o que é editável", corpo: `{"nome":"  Novo  ","tipo":3,"segmento":" Híbrido ","precoManual":false}`, status: 204, depois: "Novo | 3 | Híbrido | 0 | 7.5 | 7.0 | <nil>"},
		{nome: "em branco fica nulo", corpo: `{"nome":"  ","tipo":2,"segmento":" ","precoManual":false}`, status: 204, depois: "<nil> | 2 | <nil> | 0 | 7.5 | 7.0 | <nil>"},
		{nome: "preço manual", corpo: `{"tipo":2,"precoManual":true,"precoAtual":9.5}`, status: 204, depois: "<nil> | 2 | <nil> | 1 | 9.5 | 7.5 | Manual"},
		{nome: "sair do preço manual", fonte: textoDe("Manual"), corpo: `{"tipo":2,"precoManual":false}`, status: 204, cotacao: true, depois: "<nil> | 2 | <nil> | 0 | 7.5 | 7.0 | Manual"},
		{nome: "com a fonte de sempre, não busca", fonte: textoDe("Yahoo Finance"), corpo: `{"tipo":2,"precoManual":false}`, status: 204, depois: "<nil> | 2 | <nil> | 0 | 7.5 | 7.0 | Yahoo Finance"},
		{nome: "tipo zero", corpo: `{"tipo":0}`, status: 400, mensagem: "Tipo de ativo inválido."},
		{nome: "sem o tipo", corpo: `{}`, status: 400, mensagem: "Tipo de ativo inválido."},
		{nome: "tipo 6", corpo: `{"tipo":6}`, status: 400, mensagem: "Tipo de ativo inválido."},
		{nome: "FII no exterior", moeda: "USD", corpo: `{"tipo":1}`, status: 400, mensagem: "FIIs e BDRs são da B3: um ativo no exterior não pode ter esse tipo."},
		{nome: "BDR no exterior", moeda: "USD", corpo: `{"tipo":4}`, status: 400, mensagem: "FIIs e BDRs são da B3: um ativo no exterior não pode ter esse tipo."},
		{nome: "ETF no exterior", moeda: "USD", corpo: `{"tipo":3}`, status: 204},
		{nome: "FII na B3", corpo: `{"tipo":1}`, status: 204},
		{nome: "manual sem preço", corpo: `{"tipo":2,"precoManual":true}`, status: 400, mensagem: "Informe o preço atual para usar preço manual."},
		{nome: "manual com preço zero", corpo: `{"tipo":2,"precoManual":true,"precoAtual":0}`, status: 400, mensagem: "Informe o preço atual para usar preço manual."},
		{nome: "segmento fora da lista", corpo: `{"tipo":2,"segmento":"Fazendas"}`, status: 400, mensagem: "Segmento inválido."},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			var pedidos [][]int64
			c := cenaComCotacoes(t, cambioFalso{}, reservasFalsas{}, selicFalsa{}, cotacoesFalsas{erro: errors.New("sem rede"), pedidos: &pedidos})
			ativo := c.fab.Ativo(func(a *testeapoio.Ativo) {
				a.Ticker, a.FonteCotacao = "AAPL34", x.fonte
				a.PrecoAtual, a.PrecoAnterior = opcional(testeapoio.Valor("7.5")), opcional(testeapoio.Valor("7.0"))
				if x.moeda != "" {
					a.Moeda = x.moeda
				}
			})
			r := testeapoio.Chamar(t, c.mux, "PUT", fmt.Sprintf("%s%d", rotaAtivos, ativo.Id), x.corpo)
			r.ConferirStatus(t, x.status)
			if x.mensagem != "" {
				r.ConferirJSON(t, `{"mensagem":"`+x.mensagem+`"}`)
			}
			if x.depois != "" {
				if a := umaLinha(t, c, `SELECT Nome, Tipo, Segmento, PrecoManual, PrecoAtual, PrecoAnterior, FonteCotacao FROM Ativo`); a != x.depois {
					t.Errorf("ativo %q, esperado %q", a, x.depois)
				}
			}
			if (len(pedidos) == 1) != x.cotacao {
				t.Errorf("cotações pedidas: %v", pedidos)
			}
		})
	}
}

// Regra: o preço manual grava o carimbo em UTC, e o ativo que não existe, ou o id que não é int, são 404.
func TestAtualizarAtivoCarimboE404(t *testing.T) {
	t.Parallel()
	c := cenaSimples(t)
	ativo := c.fab.Ativo()
	testeapoio.Chamar(t, c.mux, "PUT", fmt.Sprintf("%s%d", rotaAtivos, ativo.Id), `{"tipo":2,"precoManual":true,"precoAtual":1}`).ConferirStatus(t, http.StatusNoContent)
	if em := umaLinha(t, c, `SELECT PrecoAtualizadoEm FROM Ativo`); em != "2026-10-30 18:00:00" {
		t.Errorf("carimbo %s", em)
	}
	r := testeapoio.Chamar(t, c.mux, "PUT", rotaAtivos+"99", `{"tipo":2}`)
	r.ConferirStatus(t, http.StatusNotFound)
	r.ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	testeapoio.Chamar(t, c.mux, "PUT", rotaAtivos+"abc", `{"tipo":2}`).ConferirStatus(t, http.StatusNotFound)
}

// Regra: excluir o ativo leva as operações, os proventos e os fechamentos junto.
func TestExcluirAtivo(t *testing.T) {
	t.Parallel()
	c := cenaSimples(t)
	ativo := c.fab.Ativo()
	c.fab.OperacaoAtivo(ativo)
	if _, err := c.db.Exec(`INSERT INTO ProventoAtivo (AtivoId, DataCom, ValorPorCota) VALUES (?, '2026-01-02', '1.0');
		INSERT INTO PrecoMensalAtivo (AtivoId, Mes, Fechamento) VALUES (1, '2026-01-01', '1.0')`, ativo.Id); err != nil {
		t.Fatal(err)
	}
	testeapoio.Chamar(t, c.mux, "DELETE", fmt.Sprintf("%s%d", rotaAtivos, ativo.Id), "").ConferirStatus(t, http.StatusNoContent)
	if n := umaLinha(t, c, `SELECT (SELECT COUNT(*) FROM Ativo) + (SELECT COUNT(*) FROM OperacaoAtivo) + (SELECT COUNT(*) FROM ProventoAtivo) + (SELECT COUNT(*) FROM PrecoMensalAtivo)`); n != "0" {
		t.Errorf("sobraram %s", n)
	}
	r := testeapoio.Chamar(t, c.mux, "DELETE", fmt.Sprintf("%s%d", rotaAtivos, ativo.Id), "")
	r.ConferirStatus(t, http.StatusNotFound)
	r.ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	testeapoio.Chamar(t, c.mux, "DELETE", rotaAtivos+"abc", "").ConferirStatus(t, http.StatusNotFound)
}

// Regra: a configuração só diz se há token; o token passa pelo cofre, aparado; em branco, nada muda; com espaço no meio ou
// caractere de controle, é recusado; remover apaga o valor e vale mesmo com um token junto.
func TestConfiguracaoDaCotacao(t *testing.T) {
	t.Parallel()
	const rota = "/api/carteira/configuracao"
	c := cenaSimples(t)
	configurado := func(esperado string) {
		t.Helper()
		testeapoio.Chamar(t, c.mux, "GET", rota, "").ConferirJSON(t, `{"tokenBrapiConfigurado":`+esperado+`}`)
	}
	valor := func() string {
		t.Helper()
		return umaLinha(t, c, `SELECT Valor, Protegido FROM Configuracao WHERE Chave = ?`, cotacoes.ChaveTokenBrapi)
	}
	configurado("false")
	testeapoio.Chamar(t, c.mux, "PUT", rota, `{"tokenBrapi":"   "}`).ConferirStatus(t, http.StatusNoContent)
	if v := valor(); v != "" {
		t.Errorf("em branco gravou: %s", v)
	}
	for _, invalido := range []string{"a b", `a\tb`, `a\u0001b`, `a b`} {
		r := testeapoio.Chamar(t, c.mux, "PUT", rota, `{"tokenBrapi":"`+invalido+`"}`)
		r.ConferirStatus(t, http.StatusBadRequest)
		r.ConferirJSON(t, `{"mensagem":"Token inválido."}`)
	}
	testeapoio.Chamar(t, c.mux, "PUT", rota, `{"tokenBrapi":"  abc  "}`).ConferirStatus(t, http.StatusNoContent)
	if v := valor(); v != "cifrado:abc | 1" {
		t.Errorf("valor %q", v)
	}
	configurado("true")
	testeapoio.Chamar(t, c.mux, "PUT", rota, `{"tokenBrapi":"novo","remover":true}`).ConferirStatus(t, http.StatusNoContent)
	if v := valor(); v != "<nil> | 0" {
		t.Errorf("valor %q", v)
	}
	configurado("false")
	r := testeapoio.Chamar(t, c.mux, "PUT", rota, `{"tokenBrapi":"`+strings.Repeat("x", 201)+`"}`)
	r.ConferirStatus(t, http.StatusBadRequest)
}

// Regra: erro do banco ou do câmbio nas rotas que gravam é o 500 genérico; a falha das cotações depois de gravar, não.
func TestEscritaComErro(t *testing.T) {
	t.Parallel()
	falha := errors.New("falhou")
	t.Run("a cotação da data", func(t *testing.T) {
		t.Parallel()
		c := novaCena(t, cambioFalso{erroCotacao: falha}, reservasFalsas{}, selicFalsa{})
		testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"ticker": "AAPL", "moeda": "USD"})).ConferirStatus(t, http.StatusInternalServerError)
	})
	t.Run("a lista de moedas", func(t *testing.T) {
		t.Parallel()
		c := novaCena(t, cambioFalso{erroMoedas: falha}, reservasFalsas{}, selicFalsa{})
		testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"ticker": "AAPL", "moeda": "USD"})).ConferirStatus(t, http.StatusInternalServerError)
	})
	for _, tabela := range []string{"Ativo", "OperacaoAtivo", "Configuracao"} {
		t.Run("sem a tabela "+tabela, func(t *testing.T) {
			t.Parallel()
			c := cenaSimples(t)
			ativo := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "PETR4" })
			c.fab.OperacaoAtivo(ativo)
			if _, err := c.db.Exec(`PRAGMA foreign_keys = OFF; DROP TABLE "` + tabela + `"`); err != nil {
				t.Fatal(err)
			}
			var chamadas []testeapoio.Resposta
			switch tabela {
			case "Configuracao":
				chamadas = append(chamadas, testeapoio.Chamar(t, c.mux, "GET", "/api/carteira/configuracao", ""),
					testeapoio.Chamar(t, c.mux, "PUT", "/api/carteira/configuracao", `{"tokenBrapi":"abc"}`),
					testeapoio.Chamar(t, c.mux, "PUT", "/api/carteira/configuracao", `{"remover":true}`))
			default:
				chamadas = append(chamadas, testeapoio.Chamar(t, c.mux, "POST", rotaOperacoes, corpoDaOperacao(nil)),
					testeapoio.Chamar(t, c.mux, "PUT", rotaOperacoes+"/1", corpoDaOperacao(nil)),
					testeapoio.Chamar(t, c.mux, "DELETE", rotaOperacoes+"/1", ""))
			}
			for i, r := range chamadas {
				if r.Status != http.StatusInternalServerError {
					t.Errorf("chamada %d: %d %s", i, r.Status, r.Corpo)
				}
			}
		})
	}
}

// Regra: a falha da cotação depois de gravar não impede a resposta; só o pedido cancelado volta como erro.
func TestAtualizarSemFalhar(t *testing.T) {
	t.Parallel()
	cancelado, cancelar := context.WithCancel(t.Context())
	cancelar()
	falha := errors.New("sem rede")
	casos := []struct {
		nome     string
		ctx      context.Context
		erro     error
		esperado error
	}{
		{"tudo certo", t.Context(), nil, nil},
		{"a cotação falhou", t.Context(), falha, nil},
		{"cancelado, com a falha", cancelado, falha, context.Canceled},
		{"cancelado, sem falha", cancelado, nil, nil},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaComCotacoes(t, cambioFalso{}, reservasFalsas{}, selicFalsa{}, cotacoesFalsas{erro: x.erro})
			if err := c.s.atualizarSemFalhar(x.ctx, 1); !errors.Is(err, x.esperado) {
				t.Errorf("erro %v, esperado %v", err, x.esperado)
			}
		})
	}
}

const rotaImportar = "/api/carteira/extrato/importar"

// Regra: importar do extrato: a lista é obrigatória, com 1 a 500 itens; os atributos de cada item valem com a chave do
// item; o que passa cria a operação, e o resultado conta as criadas.
func TestRotasDoExtrato(t *testing.T) {
	t.Parallel()
	c := cenaSimples(t)
	conta := c.fab.Conta()
	tr := c.fab.Transacao(conta, func(x *testeapoio.Transacao) {
		x.Data, x.Descricao, x.Valor = diaDoAno(4, 1), "COMPRA PETR4 10 x 20,50", testeapoio.Valor("-205.0")
	})
	lista := testeapoio.Chamar(t, c.mux, "GET", "/api/carteira/extrato/sugestoes", "")
	lista.ConferirStatus(t, http.StatusOK)
	if !strings.Contains(string(lista.Corpo), `"origem":"extrato"`) {
		t.Errorf("sugestões %s", lista.Corpo)
	}

	item := func(ticker string) string {
		return fmt.Sprintf(`{"transacaoId":%d,"ticker":%q,"tipo":1,"quantidade":10,"precoUnitario":20.5,"taxas":0}`, tr.Id, ticker)
	}
	r := testeapoio.Chamar(t, c.mux, "POST", rotaImportar, `{"itens":[`+item("PETR4")+`]}`)
	r.ConferirStatus(t, http.StatusOK)
	r.ConferirJSON(t, `{"transacoesAtualizadas":1}`)
	if o := umaLinha(t, c, `SELECT Origem, TransacaoId, Observacao, Data FROM OperacaoAtivo`); o != fmt.Sprintf("3 | %d | Importada do extrato | 2026-04-01", tr.Id) {
		t.Errorf("operação %q", o)
	}
	testeapoio.Chamar(t, c.mux, "POST", rotaImportar, `{"itens":[`+item("PETR4")+`]}`).ConferirJSON(t, `{"transacoesAtualizadas":0}`)

	erros := []struct{ nome, corpo, chave string }{
		{"sem a lista", `{}`, "Itens"},
		{"lista nula", `{"itens":null}`, "Itens"},
		{"lista vazia", `{"itens":[]}`, "Itens"},
		{"lista grande demais", `{"itens":[` + strings.TrimSuffix(strings.Repeat(item("PETR4")+",", 501), ",") + `]}`, "Itens"},
		{"código curto", `{"itens":[` + item("AB") + `]}`, "Itens[0].Ticker"},
		{"no segundo item", `{"itens":[` + item("PETR4") + `,` + item("AB") + `]}`, "Itens[1].Ticker"},
	}
	for _, x := range erros {
		r := testeapoio.Chamar(t, c.mux, "POST", rotaImportar, x.corpo)
		r.ConferirStatus(t, http.StatusBadRequest)
		var corpo struct {
			Errors map[string][]string `json:"errors"`
		}
		if err := json.Unmarshal(r.Corpo, &corpo); err != nil {
			t.Fatal(err)
		}
		if len(corpo.Errors) != 1 || corpo.Errors[x.chave] == nil {
			t.Errorf("%s: %s", x.nome, r.Corpo)
		}
	}
	testeapoio.Chamar(t, c.mux, "POST", rotaImportar, `{"itens":[null]}`).ConferirStatus(t, http.StatusInternalServerError)
}
