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
