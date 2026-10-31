package conferenciabanco

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/carteira"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/reservas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// cena é o serviço de verdade sobre o cenário da sonda, com o relógio no dia em que ela rodou. A PTAX fica num serviço falso
// sem nenhuma rota: ir à internet é erro do teste.
type cena struct {
	s    *Servico
	exec func(t *testing.T, comando string)
	mux  *http.ServeMux
}

func novaCena(t *testing.T, hoje string) cena {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	dados, err := os.ReadFile("testdata/cenario.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(strings.ReplaceAll(string(dados), "@MES", hoje[:7])); err != nil {
		t.Fatal(err)
	}
	dia, err := tipos.LerData(hoje)
	if err != nil {
		t.Fatal(err)
	}
	r := testeapoio.HojeEmBrasilia(dia.Ano(), dia.Mes(), dia.Dia())
	ptax := testeapoio.NovoServicoDeFora(t)
	cliente, err := cambio.NovoClientePTAX(ptax.URL(), ptax.Cliente())
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	c := cambio.NovoServico(db, cliente, r, log)
	rs := reservas.NovoServico(db, c, r, log)
	s := NovoServico(db, rs, carteira.NovoServico(db, c, rs, nil, nil, nil, r, log), c, r, log)
	mux := http.NewServeMux()
	Registrar(mux, s)
	return cena{s, func(t *testing.T, comando string) {
		t.Helper()
		if _, err := db.Exec(comando); err != nil {
			t.Fatal(err)
		}
	}, mux}
}

const hojeDosTestes = "2026-10-31"

// Regra: a conferência é a do ConferenciaBancoService do C# sobre o mesmo cenário (testdata/conferencia.json): cada conta,
// fatura e investimento, com as pistas, os valores, as datas no fuso de Brasília e a ordem.
func TestConferenciaIgualAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/conferencia.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Hoje   []string `json:"hoje"`
		Passos []struct {
			Nome      string          `json:"nome"`
			Resultado json.RawMessage `json:"resultado"`
		} `json:"passos"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Passos) < 2 {
		t.Fatalf("%d passos: a sonda mudou?", len(g.Passos))
	}
	c := novaCena(t, g.Hoje[0])
	for _, p := range g.Passos {
		if sql, ok := strings.CutPrefix(p.Nome, "mudar: "); ok {
			c.exec(t, sql)
			continue
		}
		if !t.Run(p.Nome, func(t *testing.T) {
			d, err := c.s.Montar(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			obtido, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			testeapoio.ConferirJSON(t, obtido, string(p.Resultado))
		}) {
			t.FailNow()
		}
	}
}

// Regra: a rota devolve a conferência, e o banco falhar em qualquer leitura é o 500.
func TestRota(t *testing.T) {
	t.Parallel()
	const rota = "/api/conferencia-banco"
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		r := testeapoio.Chamar(t, novaCena(t, hojeDosTestes).mux, "GET", rota, "")
		r.ConferirStatus(t, http.StatusOK)
		if !strings.Contains(string(r.Corpo), `"chave":"carteira"`) {
			t.Errorf("resposta %s", r.Corpo)
		}
	})
	casos := []struct{ nome, comando string }{
		{"os saldos", `ALTER TABLE Transacao RENAME TO Outra`},
		{"o valor pendente", `UPDATE Transacao SET Valor = 'abc' WHERE Id = 1`},
		{"as contas", `ALTER TABLE ConexaoBancaria RENAME TO Outra`},
		{"as faturas", `ALTER TABLE FaturaCartao RENAME TO Outra`},
		{"os investimentos", `ALTER TABLE Investimento RENAME TO Outro`},
		{"o saldo do investimento", `UPDATE Investimento SET Saldo = 'abc'`},
		{"a carteira", `ALTER TABLE OperacaoAtivo RENAME TO Outra`},
		{"as reservas", `ALTER TABLE ReservaConferencia RENAME TO Outra`},
		{"o câmbio", `ALTER TABLE CotacaoMoeda RENAME TO Outra`},
		{"as contas, na data", `UPDATE Conta SET AtualizadoEm = 'abc' WHERE Id = 1`},
		{"o lançamento do cartão", `UPDATE Transacao SET Data = 'abc' WHERE Id = 18`},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := novaCena(t, hojeDosTestes)
			c.exec(t, x.comando)
			testeapoio.Chamar(t, c.mux, "GET", rota, "").ConferirStatus(t, http.StatusInternalServerError)
		})
	}
}

// Regra: o pedido cancelado não é erro: sem resposta nem registro.
func TestCancelado(t *testing.T) {
	t.Parallel()
	c := novaCena(t, hojeDosTestes)
	ctx, cancelar := context.WithCancel(t.Context())
	cancelar()
	if _, err := c.s.Montar(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("erro %v", err)
	}
}

// Regra: as pistas da reserva: o valor do banco, o restante, a conferência, a estimativa, a lista do banco e a PTAX.
func TestPistasDaReserva(t *testing.T) {
	t.Parallel()
	dia := tipos.NovaData(2026, 9, 2)
	cotacao := testeapoio.Valor("5.42135")
	mil := testeapoio.Valor("1234.5")
	casos := []struct {
		nome    string
		reserva reservas.ReservaDto
		trechos []string
	}{
		{"do banco", reservas.ReservaDto{InformadoPeloBanco: true, RestanteDoBanco: true, Moeda: cambio.MoedaBase}, []string{"Valor que o banco informa pelo Open Finance."}},
		{"restante", reservas.ReservaDto{RestanteDoBanco: true, DataConferencia: &dia, Moeda: cambio.MoedaBase}, []string{"Parte do total reservado"}},
		{"conferida", reservas.ReservaDto{DataConferencia: &dia, Moeda: cambio.MoedaBase}, []string{"Conferido em 02/09/2026 e, depois"}},
		{"estimada", reservas.ReservaDto{Moeda: cambio.MoedaBase}, []string{"Estimado pelo extrato"}},
		{"fora da lista", reservas.ReservaDto{ForaDaListaDoBanco: true, Moeda: cambio.MoedaBase}, []string{"Estimado pelo extrato", "Não está na lista de reservas"}},
		{"em dólar", reservas.ReservaDto{Moeda: "USD", Cotacao: &cotacao}, []string{"Estimado pelo extrato", "Em dólar, pela PTAX de 5,4214 (a cotação"}},
		{"cotação alta", reservas.ReservaDto{Moeda: "USD", Cotacao: &mil}, []string{"Estimado pelo extrato", "PTAX de 1.234,5000 ("}},
		{"em dólar sem cotação", reservas.ReservaDto{Moeda: "USD"}, []string{"Estimado pelo extrato"}},
		{"em real com cotação", reservas.ReservaDto{Moeda: cambio.MoedaBase, Cotacao: &cotacao}, []string{"Estimado pelo extrato"}},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			d := dtoDaReserva(x.reserva)
			if len(d.Pistas) != len(x.trechos) {
				t.Fatalf("pistas %+v", d.Pistas)
			}
			for i, trecho := range x.trechos {
				if !strings.Contains(d.Pistas[i].Texto, trecho) {
					t.Errorf("pista %d: %q", i, d.Pistas[i].Texto)
				}
			}
		})
	}
}

// Regra: a data da pista é a de Brasília: 02:30 UTC ainda é o dia anterior.
func TestDataLocal(t *testing.T) {
	t.Parallel()
	antes, err := tipos.LerDataHora("2026-10-30 02:30:00")
	if err != nil {
		t.Fatal(err)
	}
	if d := dataLocal(antes); d != "29/10/2026" {
		t.Errorf("data %s", d)
	}
	if relogio.Brasilia == nil {
		t.Error("sem o fuso")
	}
}
