package testeapoio

import (
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Tabelas das reservas (#82), num arquivo à parte para os assuntos feitos em paralelo não mexerem nas mesmas linhas.

// ReservaConferencia é uma linha de ReservaConferencia (Entidades/OpenFinance.cs): o saldo real de uma reserva,
// conferido no app da instituição numa data.
type ReservaConferencia struct {
	Id              int64
	ContaId         int64
	Nome            string
	SaldoConferido  tipos.Decimal
	DataConferencia tipos.Data
	AtualizadoEm    tipos.DataHora
}

// ReservaConferencia grava a conferência da reserva "Viagem" da conta, conferida hoje (em Brasília, pelo relógio da
// fábrica) com 100.0.
func (f *Fabrica) ReservaConferencia(conta Conta, ajustes ...func(*ReservaConferencia)) ReservaConferencia {
	f.t.Helper()
	c := ReservaConferencia{ContaId: conta.Id, Nome: "Viagem", SaldoConferido: Valor("100.0"),
		DataConferencia: relogio.Hoje(f.relogio), AtualizadoEm: f.agora()}
	for _, ajustar := range ajustes {
		ajustar(&c)
	}
	c.Id = f.inserir("ReservaConferencia", `INSERT INTO ReservaConferencia (ContaId, Nome, SaldoConferido, DataConferencia,
		AtualizadoEm) VALUES (?, ?, ?, ?, ?)`, c.ContaId, c.Nome, c.SaldoConferido, c.DataConferencia, c.AtualizadoEm)
	return c
}
