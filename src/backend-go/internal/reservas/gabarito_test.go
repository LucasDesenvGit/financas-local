package reservas

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// cenarioDoGabarito aplica o testdata/cenario.sql num banco novo, com o relógio no "hoje" em que a sonda rodou, e
// monta o serviço com a PTAX num serviço falso sem nenhuma rota: ir à internet é erro do teste.
func cenarioDoGabarito(t *testing.T, hoje string) (*Servico, *sql.DB, relogio.Relogio) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	cenario, err := os.ReadFile("testdata/cenario.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(cenario)); err != nil {
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
	return NovoServico(db, cambio.NovoServico(db, cliente, r, log), r, log), db, r
}

// Regra: a lista das reservas é a do ReservaService do C# sobre o mesmo cenário (testdata/listar.json, gravado pela
// sonda das contas, internal/faturas/testdata/sonda-csharp): a estimativa pelo extrato, a conferência, o Méli Dólar
// (a quantidade da descrição ou pela PTAX da data com 2 casas, a conferência no meio, o "Méli" com acento que o LIKE
// não acha), o reservado informado pelo banco (o total, a lista, a reserva fora dela) e a ordem (o saldo, e o nome na
// ordem da cultura).
func TestListarIgualAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/listar.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Hoje     []string          `json:"hoje"`
		Reservas []json.RawMessage `json:"reservas"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Reservas) < 12 {
		t.Fatal("gabarito pequeno: a sonda mudou?")
	}
	s, _, _ := cenarioDoGabarito(t, g.Hoje[0])
	lista, err := s.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	obtido, err := json.Marshal(lista)
	if err != nil {
		t.Fatal(err)
	}
	esperado, err := json.Marshal(g.Reservas)
	if err != nil {
		t.Fatal(err)
	}
	testeapoio.ConferirJSON(t, obtido, string(esperado))

	total, err := s.Total(t.Context())
	if err != nil || total.String() != "6504.35" {
		t.Errorf("total %s, %v", total, err)
	}
}
