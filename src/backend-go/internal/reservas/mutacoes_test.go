package reservas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// cambioDaLista é o câmbio do Listar: o conversor que o teste montou, sem ir à internet.
type cambioDaLista struct{ conversor *cambio.Conversor }

func (c cambioDaLista) ObterCotacao(context.Context, string, *tipos.Data) (*cambio.Cotacao, error) {
	return nil, nil
}
func (c cambioDaLista) GarantirHistorico(context.Context, string, tipos.Data) error { return nil }
func (c cambioDaLista) ObterConversor(context.Context, *tipos.Data) (*cambio.Conversor, error) {
	return c.conversor, nil
}

// bancoDaLista é um banco vazio, com uma conta do Open Finance, e o serviço com o conversor dado.
func bancoDaLista(t *testing.T, conversor *cambio.Conversor) (*Servico, *sql.DB, *testeapoio.Fabrica, testeapoio.Conta) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0)
	fab := testeapoio.NovaFabrica(t, db, r)
	conta := fab.Conta(func(c *testeapoio.Conta) { c.Origem = 2 })
	return NovoServico(db, cambioDaLista{conversor}, r, slog.New(slog.DiscardHandler)), db, fab, conta
}

func compraDeMeli(descricao, valor string) func(*testeapoio.Transacao) {
	return func(tr *testeapoio.Transacao) {
		tr.Descricao, tr.Valor, tr.Data = descricao, testeapoio.Valor(valor), testeapoio.Dia(2026, 10, 1)
	}
}

// Regra: "pode ter saldo anterior" vale até 7 dias depois do começo do extrato, sem conferência; "conferência antiga",
// com mais de 30 dias.
func TestJanela(t *testing.T) {
	t.Parallel()
	inicio, hoje := testeapoio.Dia(2026, 8, 1), testeapoio.Dia(2026, 10, 30)
	data := func(mes time.Month, dia int) *tipos.Data {
		d := testeapoio.Dia(2026, mes, dia)
		return &d
	}
	casos := []struct {
		nome            string
		primeiro        *tipos.Data
		temInicio       bool
		conferencia     *tipos.Data
		anterior, velha bool
	}{
		{nome: "7 dias depois do começo do extrato", primeiro: data(8, 8), temInicio: true, anterior: true},
		{nome: "8 dias depois", primeiro: data(8, 9), temInicio: true},
		{nome: "sem o começo do extrato", primeiro: data(8, 1)},
		{nome: "sem lançamento", temInicio: true},
		{nome: "com conferência de 30 dias", primeiro: data(8, 1), temInicio: true, conferencia: data(9, 30)},
		{nome: "com conferência de 31 dias", primeiro: data(8, 1), temInicio: true, conferencia: data(9, 29), velha: true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			anterior, velha := janela(c.primeiro, inicio, c.temInicio, c.conferencia, hoje)
			if anterior != c.anterior || velha != c.velha {
				t.Errorf("anterior %v, antiga %v; esperado %v, %v", anterior, velha, c.anterior, c.velha)
			}
		})
	}
}

// Regra: com duas conferências do mesmo nome em caixas diferentes, vale a primeira que a consulta lê (o FirstOrDefault
// do C#): esta consulta junta com a conta e percorre a tabela na ordem em que as linhas foram gravadas, o mesmo plano
// do EF. A da conferência (TestConferirAtualizaAPrimeiraDoMesmoNome) lê pelo índice.
func TestListarUsaAPrimeiraConferenciaDoNome(t *testing.T) {
	t.Parallel()
	s, _, fab, conta := bancoDaLista(t, cambio.NovoConversor(nil, nil))
	fab.Transacao(conta, compraDeMeli("Dinheiro reservado Viagem", "-1.0"))
	fab.ReservaConferencia(conta, func(rc *testeapoio.ReservaConferencia) {
		rc.Nome, rc.SaldoConferido, rc.DataConferencia = "viagem", testeapoio.Valor("50.0"), testeapoio.Dia(2026, 9, 1)
	})
	fab.ReservaConferencia(conta, func(rc *testeapoio.ReservaConferencia) {
		rc.Nome, rc.SaldoConferido, rc.DataConferencia = "VIAGEM", testeapoio.Valor("120.0"), testeapoio.Dia(2026, 9, 1)
	})
	lista, err := s.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != 1 || lista[0].SaldoConferido == nil || lista[0].SaldoConferido.String() != "50.0" {
		t.Errorf("lista %+v, esperada a conferência de 50.0", lista)
	}
}

// Regra: o Méli Dólar aparece com os lançamentos ou só com a conferência; sem nenhum dos dois, não aparece.
func TestMeliDolarComSoUmaDasFontes(t *testing.T) {
	t.Parallel()
	dolar := cambio.Guardada{Moeda: "USD", Data: testeapoio.Dia(2026, 10, 1), Compra: testeapoio.Valor("5.0"), Venda: testeapoio.Valor("5.0")}
	casos := []struct {
		nome    string
		preparo func(*testeapoio.Fabrica, testeapoio.Conta)
		linhas  int
	}{
		{"sem nada", func(*testeapoio.Fabrica, testeapoio.Conta) {}, 0},
		{"só o lançamento", func(fab *testeapoio.Fabrica, conta testeapoio.Conta) {
			fab.Transacao(conta, compraDeMeli("Compra de Meli Dolar MUSD 2", "-10.0"))
		}, 1},
		{"só a conferência", func(fab *testeapoio.Fabrica, conta testeapoio.Conta) {
			fab.ReservaConferencia(conta, func(rc *testeapoio.ReservaConferencia) { rc.Nome = NomeMeliDolar })
		}, 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			s, _, fab, conta := bancoDaLista(t, cambio.NovoConversor([]cambio.Guardada{dolar}, nil))
			c.preparo(fab, conta)
			lista, err := s.Listar(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(lista) != c.linhas {
				t.Errorf("%d linhas, esperado %d: %+v", len(lista), c.linhas, lista)
			}
		})
	}
}

// Regra: sem a taxa do dólar da data (nenhuma cotação, ou a compra zero), o lançamento sem a quantidade na descrição
// não soma dólares: o resto da conta não quebra.
func TestMeliDolarSemTaxa(t *testing.T) {
	t.Parallel()
	zero := cambio.Guardada{Moeda: "USD", Data: testeapoio.Dia(2026, 10, 1), Compra: testeapoio.Valor("0.0"), Venda: testeapoio.Valor("0.0")}
	casos := []struct {
		nome    string
		ultimas []cambio.Guardada
		saldo   string
	}{
		{"sem nenhuma cotação: vale o que entrou menos o que saiu", nil, "10.0"},
		{"com a compra zero", []cambio.Guardada{zero}, "0.0"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			s, _, fab, conta := bancoDaLista(t, cambio.NovoConversor(c.ultimas, nil))
			fab.Transacao(conta, compraDeMeli("Compra de Meli Dolar", "-10.0"))
			lista, err := s.Listar(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(lista) != 1 || lista[0].Quantidade == nil || !lista[0].Quantidade.Decimal().IsZero() ||
				lista[0].Saldo.String() != c.saldo {
				t.Errorf("lista %+v, esperada a quantidade 0 e o saldo %s", lista, c.saldo)
			}
		})
	}
}

// Regra: uma conta com a lista detalhada do banco não interrompe as contas seguintes.
func TestSubstituirPeloBancoSegueDepoisDaLista(t *testing.T) {
	t.Parallel()
	banco := []doBanco{
		{conta: 1, nomeConta: "Lista", total: testeapoio.Valor("5"), reservas: []informada{{Nome: "Caixa", Valor: testeapoio.Valor("5")}}},
		{conta: 2, nomeConta: "Total", total: testeapoio.Valor("7")},
	}
	lista := substituirPeloBanco(nil, banco)
	if len(lista) != 2 || lista[0].Nome != "Caixa" || lista[1].Nome != NomeRestanteDoBanco {
		t.Errorf("lista %+v", lista)
	}
}

// Regra: se o DELETE não apagar nada (outra remoção chegou antes), "não havia": o > 0 do ExecuteDeleteAsync.
func TestRemoverConferenciaQueNaoApagou(t *testing.T) {
	t.Parallel()
	s, db, fab, conta := bancoDaLista(t, nil)
	fab.ReservaConferencia(conta)
	if _, err := db.Exec(`CREATE TRIGGER nao_apaga BEFORE DELETE ON ReservaConferencia BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	removeu, err := s.RemoverConferencia(t.Context(), conta.Id, "Viagem")
	if err != nil || removeu {
		t.Errorf("removeu %v, %v; esperado false", removeu, err)
	}
}

// Regra: pedido cancelado não é erro (nem resposta, nem log), venha o cancelamento no erro ou no contexto do pedido.
func TestPedidoCanceladoNaoEhErro(t *testing.T) {
	t.Parallel()
	cancelado, cancelar := context.WithCancel(t.Context())
	cancelar()
	casos := []struct {
		nome string
		ctx  context.Context
		err  error
	}{
		{"o erro é o cancelamento", t.Context(), fmt.Errorf("ler: %w", context.Canceled)},
		{"o pedido foi cancelado", cancelado, errors.New("outro erro")},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			_, s, saida := rotasDeTeste(t)
			gravador := httptest.NewRecorder()
			s.erroInterno(gravador, httptest.NewRequestWithContext(c.ctx, http.MethodGet, "/api/reservas", nil), "listar", c.err)
			if gravador.Body.Len() != 0 || saida.Len() != 0 {
				t.Errorf("corpo %q, log %q", gravador.Body.String(), saida.String())
			}
		})
	}
}
