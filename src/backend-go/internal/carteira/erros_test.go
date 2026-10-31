package carteira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// recusar faz o banco recusar a operação na tabela: o caminho do erro ao gravar.
func recusar(t *testing.T, c cena, quando, tabela string) {
	t.Helper()
	if _, err := c.db.Exec(`CREATE TRIGGER recusa BEFORE ` + quando + ` ON "` + tabela + `" BEGIN SELECT RAISE(ABORT, 'recusado'); END`); err != nil {
		t.Fatal(err)
	}
}

// Regra: o banco recusar a gravação é o 500 genérico, com o erro no log, em cada rota que grava.
func TestGravacaoRecusadaPeloBanco(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome, quando, tabela, metodo, caminho, corpo string
	}{
		{"o ativo novo", "INSERT", "Ativo", "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"ticker": "WEGE3"})},
		{"a operação nova", "INSERT", "OperacaoAtivo", "POST", rotaOperacoes, corpoDaOperacao(nil)},
		{"a operação alterada", "UPDATE", "OperacaoAtivo", "PUT", rotaOperacoes + "/1", corpoDaOperacao(nil)},
		{"a operação excluída", "DELETE", "OperacaoAtivo", "DELETE", rotaOperacoes + "/1", ""},
		{"o ativo alterado", "UPDATE", "Ativo", "PUT", rotaAtivos + "1", `{"tipo":2}`},
		{"o ativo manual", "UPDATE", "Ativo", "PUT", rotaAtivos + "1", `{"tipo":2,"precoManual":true,"precoAtual":1}`},
		{"o ativo excluído", "DELETE", "Ativo", "DELETE", rotaAtivos + "1", ""},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaSimples(t)
			c.fab.OperacaoAtivo(c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "PETR4" }))
			recusar(t, c, x.quando, x.tabela)
			testeapoio.Chamar(t, c.mux, x.metodo, x.caminho, x.corpo).ConferirStatus(t, http.StatusInternalServerError)
			if !strings.Contains(c.saida.String(), "erro na carteira") {
				t.Errorf("log %q", c.saida.String())
			}
		})
	}
}

// Regra: um valor ilegível no banco é o 500, em cada leitura, e não um número errado na tela.
func TestValorIlegivelNoBanco(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome, comando, metodo, caminho, corpo string
	}{
		{"o preço do ativo", `UPDATE Ativo SET PrecoAtual = 'abc'`, "GET", "/api/carteira", ""},
		{"a operação", `UPDATE OperacaoAtivo SET Taxas = 'abc'`, "GET", "/api/carteira", ""},
		{"o provento", `INSERT INTO ProventoAtivo (AtivoId, DataCom, ValorPorCota) VALUES (1, '2026-01-02', 'abc')`, "GET", "/api/carteira", ""},
		{"o investimento", `INSERT INTO ConexaoBancaria (Id, PluggyItemId, Ativa, CriadoEm) VALUES (1, 'x', 1, '2026-01-01 00:00:00');
			INSERT INTO Investimento (ConexaoBancariaId, PluggyInvestmentId, Nome, Tipo, Moeda, Saldo, AtualizadoEm) VALUES (1, 'x', 'x', 'x', 'BRL', 'abc', '2026-01-01 00:00:00')`,
			"GET", "/api/carteira", ""},
		{"a operação, ao vender", `UPDATE OperacaoAtivo SET Taxas = 'abc'`, "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"tipo": 2})},
		{"a operação, ao alterar", `UPDATE OperacaoAtivo SET Taxas = 'abc'`, "PUT", rotaOperacoes + "/1", corpoDaOperacao(nil)},
		{"a operação, ao excluir", `UPDATE OperacaoAtivo SET Taxas = 'abc'`, "DELETE", rotaOperacoes + "/1", ""},
		{"o preço, ao alterar o ativo", `UPDATE Ativo SET PrecoAtual = 'abc'`, "PUT", rotaAtivos + "1", `{"tipo":2}`},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaSimples(t)
			c.fab.OperacaoAtivo(c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "PETR4" }))
			if _, err := c.db.Exec(x.comando); err != nil {
				t.Fatal(err)
			}
			testeapoio.Chamar(t, c.mux, x.metodo, x.caminho, x.corpo).ConferirStatus(t, http.StatusInternalServerError)
		})
	}
}

// Regra: erro do câmbio ao alterar uma operação em dólar é o 500.
func TestAlterarComErroDoCambio(t *testing.T) {
	t.Parallel()
	c := novaCena(t, cambioFalso{erroCotacao: errors.New("falhou")}, reservasFalsas{}, selicFalsa{})
	c.fab.OperacaoAtivo(c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker, a.Moeda = "AAPL", "USD" }))
	r := testeapoio.Chamar(t, c.mux, "PUT", rotaOperacoes+"/1", corpoDaOperacao(map[string]any{"ticker": "AAPL"}))
	r.ConferirStatus(t, http.StatusInternalServerError)
}

// cotacoesQueCancelam cancelam o pedido no meio da busca: quem pediu desistiu depois de gravar.
type cotacoesQueCancelam struct{ cancelar context.CancelFunc }

func (c cotacoesQueCancelam) Atualizar(context.Context, []int64) (cotacoes.Resultado, error) {
	c.cancelar()
	return cotacoes.Resultado{}, errors.New("cancelado")
}

func (c cotacoesQueCancelam) FechamentosDiarios(context.Context, string, tipos.Data, tipos.Data) ([]cotacoes.Fechamento, error) {
	return nil, nil
}

// Regra: se o pedido foi cancelado na busca da cotação, o que foi gravado fica, e não há resposta nem log de erro.
func TestCanceladoDepoisDeGravar(t *testing.T) {
	t.Parallel()
	casos := []struct{ nome, metodo, caminho, corpo string }{
		{"a operação nova", "POST", rotaOperacoes, corpoDaOperacao(map[string]any{"ticker": "WEGE3"})},
		{"saindo do preço manual", "PUT", rotaAtivos + "1", `{"tipo":2}`},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaSimples(t)
			c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker, a.FonteCotacao = "PETR4", textoDe("Manual") })
			ctx, cancelar := context.WithCancel(t.Context())
			defer cancelar()
			c.s.cotacoes = cotacoesQueCancelam{cancelar}
			gravador := httptest.NewRecorder()
			pedido := httptest.NewRequestWithContext(ctx, x.metodo, x.caminho, strings.NewReader(x.corpo))
			pedido.Header.Set("Content-Type", "application/json")
			c.mux.ServeHTTP(gravador, pedido)
			if gravador.Body.Len() != 0 || c.saida.Len() != 0 {
				t.Errorf("corpo %q, log %q", gravador.Body.String(), c.saida.String())
			}
		})
	}
}

// Regra: a importação só pede a cotação dos ativos novos; a falha dela não impede o que foi gravado, e só o pedido
// cancelado volta como erro.
func TestImportarPedeACotacaoDosNovos(t *testing.T) {
	t.Parallel()
	preparar := func(t *testing.T, a atualizacao) (cena, []ItemDoExtrato) {
		t.Helper()
		c := cenaSimples(t)
		c.s.cotacoes = a
		c.fab.Ativo(func(x *testeapoio.Ativo) { x.Ticker = "PETR4" })
		tr := c.fab.Transacao(c.fab.Conta())
		item := ItemDoExtrato{TransacaoId: tr.Id, Ticker: "PETR4", Tipo: 1, Quantidade: testeapoio.Valor("1"), PrecoUnitario: testeapoio.Valor("1"), Taxas: testeapoio.Valor("0")}
		return c, []ItemDoExtrato{item}
	}
	t.Run("ativo que já existe", func(t *testing.T) {
		t.Parallel()
		var pedidos [][]int64
		c, itens := preparar(t, cotacoesFalsas{pedidos: &pedidos})
		if n, err := c.s.Importar(t.Context(), itens); err != nil || n != 1 || len(pedidos) != 0 {
			t.Errorf("%d, %v, pedidos %v", n, err, pedidos)
		}
	})
	t.Run("ativo novo", func(t *testing.T) {
		t.Parallel()
		var pedidos [][]int64
		c, itens := preparar(t, cotacoesFalsas{erro: errors.New("sem rede"), pedidos: &pedidos})
		itens[0].Ticker = "WEGE3"
		if n, err := c.s.Importar(t.Context(), itens); err != nil || n != 1 || len(pedidos) != 1 {
			t.Errorf("%d, %v, pedidos %v", n, err, pedidos)
		}
	})
	t.Run("cancelado depois de gravar", func(t *testing.T) {
		t.Parallel()
		ctx, cancelar := context.WithCancel(t.Context())
		defer cancelar()
		c, itens := preparar(t, cotacoesQueCancelam{cancelar})
		itens[0].Ticker = "WEGE3"
		if n, err := c.s.Importar(ctx, itens); !errors.Is(err, context.Canceled) || n != 1 {
			t.Errorf("%d, %v", n, err)
		}
	})
}

// Regra: o filtro dos proventos recebidos olha a descrição e o texto original, cada um com os seus termos, e o código do
// ativo pode vir só na descrição.
func TestProventosRecebidosFiltro(t *testing.T) {
	t.Parallel()
	c := cenaSimples(t)
	conta := c.fab.Conta()
	lancar := func(descricao string, original *string, valor string) {
		c.fab.Transacao(conta, func(x *testeapoio.Transacao) {
			x.Descricao, x.DescricaoOriginal, x.Valor = descricao, original, testeapoio.Valor(valor)
		})
	}
	lancar("RENDIMENTO PETR4", nil, "1.0")
	lancar("PETR4", textoDe("RENDIMENTO"), "2.0")
	lancar("JRS CAP WEGE3", nil, "3.0")
	lancar("WEGE3", textoDe("JRS CAP"), "4.0")
	lancar("RENDIMENTO ITSA4", nil, "-5.0")
	proventos, err := c.s.ProventosRecebidos(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var obtido []string
	for _, p := range proventos {
		obtido = append(obtido, p.Ticker+" "+p.Valor.String())
	}
	if strings.Join(obtido, ", ") != "PETR4 1.0, PETR4 2.0, WEGE3 3.0" {
		t.Errorf("proventos %v", obtido)
	}
}

const rotaSugestoes = "/api/carteira/extrato/sugestoes"

// cenaDoExtrato tem um lançamento com sugestão de compra e uma operação de outro lançamento.
func cenaDoExtrato(t *testing.T, a cotacoesFalsas) cena {
	t.Helper()
	c := cenaComCotacoes(t, cambioFalso{}, reservasFalsas{}, selicFalsa{}, a)
	c.fab.Transacao(c.fab.Conta(), func(x *testeapoio.Transacao) {
		x.Data, x.Descricao, x.Valor = diaDoAno(4, 1), "COMPRA PETR4 10 x 20,50", testeapoio.Valor("-205.0")
	})
	c.fab.OperacaoAtivo(c.fab.Ativo(func(x *testeapoio.Ativo) { x.Ticker = "PETR4" }))
	return c
}

// Regra: o banco falhar em qualquer leitura do extrato é o 500, sem sugestão pela metade.
func TestSugestoesComErroDoBanco(t *testing.T) {
	t.Parallel()
	casos := []struct{ nome, comando string }{
		{"a categoria", `ALTER TABLE Categoria RENAME TO Outra`},
		{"os tickers", `ALTER TABLE Ativo RENAME TO Outro`},
		{"o extrato", `ALTER TABLE Transacao RENAME TO Outra`},
		{"o lançamento", `UPDATE Transacao SET Valor = 'abc'`},
		{"as operações", `ALTER TABLE OperacaoAtivo RENAME TO Outra`},
		{"a operação", `UPDATE OperacaoAtivo SET Data = 'abc'`},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaDoExtrato(t, cotacoesFalsas{})
			if _, err := c.db.Exec(x.comando); err != nil {
				t.Fatal(err)
			}
			testeapoio.Chamar(t, c.mux, "GET", rotaSugestoes, "").ConferirStatus(t, http.StatusInternalServerError)
		})
	}
	t.Run("o histórico de cotações", func(t *testing.T) {
		t.Parallel()
		c := cenaDoExtrato(t, cotacoesFalsas{erroFechamentos: errors.New("falhou")})
		testeapoio.Chamar(t, c.mux, "GET", rotaSugestoes, "").ConferirStatus(t, http.StatusInternalServerError)
	})
	t.Run("os proventos", func(t *testing.T) {
		t.Parallel()
		c := cenaDoExtrato(t, cotacoesFalsas{})
		if _, err := c.db.Exec(`ALTER TABLE Categoria RENAME TO Outra`); err != nil {
			t.Fatal(err)
		}
		if _, err := c.s.ProventosRecebidos(t.Context()); err == nil {
			t.Error("sem erro")
		}
	})
}

// Regra: sem lançamento com código de ativo, as sugestões são a lista vazia, e não null.
func TestSugestoesSemLancamento(t *testing.T) {
	t.Parallel()
	c := cenaSimples(t)
	testeapoio.Chamar(t, c.mux, "GET", rotaSugestoes, "").ConferirJSON(t, `[]`)
}

// Regra: o banco falhar na importação é o 500, e nada fica gravado pela metade.
func TestImportarComErroDoBanco(t *testing.T) {
	t.Parallel()
	const corpo = `{"itens":[{"transacaoId":1,"ticker":"WEGE3","tipo":1,"quantidade":1,"precoUnitario":1,"taxas":0}]}`
	casos := []struct {
		nome    string
		quebrar func(t *testing.T, c cena)
	}{
		{"o lançamento", func(t *testing.T, c cena) { executar(t, c, `ALTER TABLE Transacao RENAME TO Outra`) }},
		{"as operações ligadas", func(t *testing.T, c cena) { executar(t, c, `ALTER TABLE OperacaoAtivo RENAME TO Outra`) }},
		{"o ativo", func(t *testing.T, c cena) { executar(t, c, `ALTER TABLE Ativo RENAME TO Outro`) }},
		{"o ativo novo", func(t *testing.T, c cena) { recusar(t, c, "INSERT", "Ativo") }},
		{"a operação nova", func(t *testing.T, c cena) { recusar(t, c, "INSERT", "OperacaoAtivo") }},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaDoExtrato(t, cotacoesFalsas{})
			x.quebrar(t, c)
			testeapoio.Chamar(t, c.mux, "POST", rotaImportar, corpo).ConferirStatus(t, http.StatusInternalServerError)
		})
	}
}

func executar(t *testing.T, c cena, comando string) {
	t.Helper()
	if _, err := c.db.Exec(comando); err != nil {
		t.Fatal(err)
	}
}

// Regra: na série de fechamentos, o dia repetido fica com o último valor.
func TestDiaRepetidoNaSerie(t *testing.T) {
	t.Parallel()
	dia, outro := diaDoAno(4, 2), diaDoAno(4, 1)
	um, dois, tres := testeapoio.Valor("1.0"), testeapoio.Valor("2.0"), testeapoio.Valor("3.0")
	serie := emOrdemDeData([]cotacoes.Fechamento{{Data: dia, Valor: um}, {Data: dia, Valor: dois}, {Data: outro, Valor: tres}})
	if len(serie) != 2 || serie[0].Valor != tres || serie[1].Valor != dois {
		t.Errorf("série %+v", serie)
	}
}

// Regra: para o \\b do RE2, a letra acentuada é parte da palavra, e o resto fora do ASCII separa as palavras.
func TestEmAscii(t *testing.T) {
	t.Parallel()
	if a := emAscii("ação—x"); a != "a__o x" {
		t.Errorf("emAscii = %q", a)
	}
}
