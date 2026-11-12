package reservas

import (
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// Regra (#691, ADR 0063): o lançamento do app pago direto de um cofrinho (Transacao.ReservaNome) é uma retirada da
// reserva de mesmo nome (sem diferenciar maiúsculas): baixa o saldo e soma em "retirado". Pendente, de conta manual ou
// de antes da conferência não conta; o nome sozinho, sem lançamento de reserva no extrato, já é uma reserva.
func TestPagosPeloCofrinhoBaixamAReserva(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome                     string
		montar                   func(*testeapoio.Fabrica, testeapoio.Conta)
		reserva, saldo, retirado string
		movimentos               int
	}{
		{nome: "baixa a reserva do mesmo nome, sem diferenciar maiúsculas",
			montar: func(f *testeapoio.Fabrica, c testeapoio.Conta) {
				f.Transacao(c, pagoCom("emergência", "-120.00", 10))
			}, reserva: "Emergência", saldo: "380", retirado: "120", movimentos: 2},
		{nome: "pendente não conta",
			montar: func(f *testeapoio.Fabrica, c testeapoio.Conta) {
				f.Transacao(c, pagoCom("Emergência", "-120.00", 10), func(tr *testeapoio.Transacao) { tr.Status = 2 })
			}, reserva: "Emergência", saldo: "500", retirado: "0", movimentos: 1},
		{nome: "de conta manual não conta",
			montar: func(f *testeapoio.Fabrica, _ testeapoio.Conta) {
				manual := f.Conta(func(c *testeapoio.Conta) { c.Origem = 1 })
				f.Transacao(manual, pagoCom("Emergência", "-120.00", 10))
			}, reserva: "Emergência", saldo: "500", retirado: "0", movimentos: 1},
		{nome: "antes da conferência já está nela",
			montar: func(f *testeapoio.Fabrica, c testeapoio.Conta) {
				f.Transacao(c, pagoCom("Emergência", "-120.00", 10))
				f.ReservaConferencia(c, func(r *testeapoio.ReservaConferencia) {
					r.Nome, r.SaldoConferido, r.DataConferencia = "Emergência", testeapoio.Valor("400.0"), testeapoio.Dia(2026, 10, 20)
				})
			}, reserva: "Emergência", saldo: "400", retirado: "0", movimentos: 2},
		{nome: "depois da conferência baixa a partir dela",
			montar: func(f *testeapoio.Fabrica, c testeapoio.Conta) {
				f.Transacao(c, pagoCom("Emergência", "-120.00", 25))
				f.ReservaConferencia(c, func(r *testeapoio.ReservaConferencia) {
					r.Nome, r.SaldoConferido, r.DataConferencia = "Emergência", testeapoio.Valor("400.0"), testeapoio.Dia(2026, 10, 20)
				})
			}, reserva: "Emergência", saldo: "280", retirado: "120", movimentos: 2},
		{nome: "cofrinho que o extrato não tem vira reserva, sem saldo negativo",
			montar: func(f *testeapoio.Fabrica, c testeapoio.Conta) {
				f.Transacao(c, pagoCom("Contas", "-80.00", 10))
			}, reserva: "Contas", saldo: "0", retirado: "80", movimentos: 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			s, _, fab, conta := bancoDaLista(t, nil)
			fab.Transacao(conta, func(tr *testeapoio.Transacao) {
				tr.Descricao, tr.Valor, tr.Data = "Dinheiro reservado Emergência", testeapoio.Valor("-500.00"), testeapoio.Dia(2026, 10, 1)
			})
			c.montar(fab, conta)
			lista, err := s.Listar(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range lista {
				if r.Nome != c.reserva {
					continue
				}
				if r.Saldo.Decimal().Cmp(testeapoio.Valor(c.saldo).Decimal()) != 0 ||
					r.Retirado.Decimal().Cmp(testeapoio.Valor(c.retirado).Decimal()) != 0 || r.Movimentos != c.movimentos {
					t.Errorf("%s: saldo %s, retirado %s, %d movimentos; esperado %s, %s, %d", r.Nome, r.Saldo, r.Retirado,
						r.Movimentos, c.saldo, c.retirado, c.movimentos)
				}
				return
			}
			t.Errorf("a reserva %q não está na lista: %+v", c.reserva, lista)
		})
	}
}

func pagoCom(reserva, valor string, dia int) func(*testeapoio.Transacao) {
	return func(tr *testeapoio.Transacao) {
		tr.Descricao, tr.Valor, tr.Data, tr.ReservaNome = "Conta de luz", testeapoio.Valor(valor), testeapoio.Dia(2026, 10, dia), &reserva
	}
}

// Regra (#691): ReservaNome em branco ou do Méli Dólar não vira reserva em reais; a falha ao ler os lançamentos pagos
// pelo cofrinho volta como erro.
func TestPagosPeloCofrinhoIgnoradosEErro(t *testing.T) {
	t.Parallel()
	s, db, fab, conta := bancoDaLista(t, nil)
	fab.Transacao(conta, pagoCom("   ", "-10.00", 10))
	fab.Transacao(conta, pagoCom("Méli Dólar", "-10.00", 10))
	fab.Transacao(conta, pagoCom("Viagem", "-10.00", 10))
	lista, err := s.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != 1 || lista[0].Nome != "Viagem" {
		t.Errorf("Listar = %+v, esperado só a reserva Viagem", lista)
	}
	if _, err := db.Exec(`ALTER TABLE "Transacao" RENAME COLUMN "ReservaNome" TO "OutroNome"`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Listar(t.Context()); err == nil {
		t.Error("Listar sem a coluna ReservaNome devia falhar")
	}
}

// Regra (#691): o nome da reserva é o do lançamento mais recente do extrato, em qualquer ordem de leitura, mesmo
// que o pago pelo cofrinho no app seja mais novo; só sem lançamento do extrato o nome vem do app.
func TestNomeDaReservaPeloExtratoMaisRecente(t *testing.T) {
	t.Parallel()
	reservado := func(descricao string, dia int) func(*testeapoio.Transacao) {
		return func(tr *testeapoio.Transacao) {
			tr.Descricao, tr.Valor, tr.Data = descricao, testeapoio.Valor("-100.00"), testeapoio.Dia(2026, 10, dia)
		}
	}
	casos := []struct {
		nome        string
		lancamentos []func(*testeapoio.Transacao)
		quer        string
	}{
		{"o mais novo do extrato lido primeiro", []func(*testeapoio.Transacao){reservado("Dinheiro reservado EMERGÊNCIA", 5),
			reservado("Dinheiro reservado Emergência", 1), pagoCom("emergência", "-20.00", 20)}, "EMERGÊNCIA"},
		{"o mais novo do extrato lido depois", []func(*testeapoio.Transacao){reservado("Dinheiro reservado Emergência", 1),
			reservado("Dinheiro reservado EMERGÊNCIA", 5), pagoCom("emergência", "-20.00", 20)}, "EMERGÊNCIA"},
		{"o do app lido primeiro", []func(*testeapoio.Transacao){pagoCom("emergência", "-20.00", 20),
			reservado("Dinheiro reservado Emergência", 1)}, "Emergência"},
		{"só o do app", []func(*testeapoio.Transacao){pagoCom("emergência", "-20.00", 20),
			pagoCom("EMERGÊNCIA", "-20.00", 25)}, "EMERGÊNCIA"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			s, _, fab, conta := bancoDaLista(t, nil)
			for _, l := range c.lancamentos {
				fab.Transacao(conta, l)
			}
			lista, err := s.Listar(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(lista) != 1 || lista[0].Nome != c.quer {
				t.Errorf("Listar = %+v, esperado a reserva %s", lista, c.quer)
			}
		})
	}
}
