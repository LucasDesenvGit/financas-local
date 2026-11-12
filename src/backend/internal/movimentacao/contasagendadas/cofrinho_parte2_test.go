package contasagendadas

import (
	"fmt"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// sincronizadaEm é o carimbo (UTC) da última sincronização da conexão.
func sincronizadaEm(dia, hora int) *tipos.DataHora {
	d := tipos.NovaDataHora(time.Date(2026, 10, dia, hora, 0, 0, 0, time.UTC))
	return &d
}

// Regra (#691): o "não veio" pergunta "foi paga com um cofrinho?" quando a conta é do Open Finance, em débito
// automático, sem "Pago com", e a conexão sincronizou depois do dia do vencimento (05/10, em Brasília).
func TestAvisoNaoVeioPodeSerCofrinho(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome    string
		origem  int64
		forma   int64
		reserva *string
		sync    *tipos.DataHora
		quer    bool
	}{
		{"sincronizou no dia seguinte", 2, 2, nil, sincronizadaEm(6, 3), true},
		{"sincronizou no fim do dia do vencimento", 2, 2, nil, sincronizadaEm(6, 2), false}, // 23h59 de 05/10 em Brasília
		{"nunca sincronizou", 2, 2, nil, nil, false},
		{"já tem o Pago com", 2, 2, ptr("Emergência"), sincronizadaEm(6, 3), false},
		{"conta manual", 1, 2, nil, sincronizadaEm(6, 3), false},
		{"no cartão", 2, 3, nil, sincronizadaEm(6, 3), false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db, fab, _ := ambiente(t)
			conexao := fab.ConexaoBancaria(func(b *testeapoio.ConexaoBancaria) { b.UltimaSincronizacaoEm = c.sync })
			conta := fab.Conta(func(x *testeapoio.Conta) { x.Origem, x.ConexaoBancariaId = c.origem, &conexao.Id })
			fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
				a.Descricao, a.ProximoVencimento, a.ContaId, a.FormaCobranca = "Luz", dia(5), &conta.Id, c.forma
				a.TextoCobranca, a.ReservaNome = ptr("ENEL"), c.reserva
			})
			s := NovoServico(db, testeapoio.HojeEmBrasilia(2026, 10, 14), nil)
			avisos, err := s.Avisos(t.Context(), dia(14))
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range avisos {
				if a.Tipo != "nao-veio" {
					continue
				}
				if a.PodeSerCofrinho != c.quer {
					t.Errorf("PodeSerCofrinho = %v, esperado %v (%s)", a.PodeSerCofrinho, c.quer, a.Texto)
				}
				if quer := "A cobrança de Luz esperada em 05/10 não chegou pelo extrato, e a conta já sincronizou depois. " +
					"Foi paga com um cofrinho?"; c.quer && a.Texto != quer {
					t.Errorf("Texto = %q", a.Texto)
				}
				return
			}
			t.Errorf("sem o aviso não veio: %+v", avisos)
		})
	}
}

// Regra (#691): o "Pagar" com o cofrinho escolhido na hora lança a despesa por ele e, com "guardar", ele vira o "Pago
// com" da conta. Sem lançar, em conta manual ou no cartão, o cofrinho escolhido é recusado.
func TestPagarComCofrinhoEscolhido(t *testing.T) {
	t.Parallel()
	db, fab, mux := ambiente(t)
	of := fab.Conta(func(c *testeapoio.Conta) { c.Origem = 2 })
	manual := fab.Conta()
	nova := func(conta int64, forma int64) int64 {
		return fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
			a.Descricao, a.ContaId, a.FormaCobranca, a.TextoCobranca = "Luz", &conta, forma, ptr("ENEL")
		}).Id
	}
	casos := []struct {
		nome, corpo string
		id          int64
		status      int
	}{
		{"sem lançar", `{"lancarTransacao":false,"reservaNome":"Emergência"}`, nova(of.Id, 2), 400},
		{"conta manual", `{"lancarTransacao":true,"reservaNome":"Emergência"}`, nova(manual.Id, 2), 400},
		{"no cartão", `{"lancarTransacao":true,"reservaNome":"Emergência"}`, nova(of.Id, 3), 400},
		{"sem guardar", `{"lancarTransacao":true,"reservaNome":"Viagem","data":"2026-10-14"}`, nova(of.Id, 2), 204},
		{"guardando", `{"lancarTransacao":true,"reservaNome":" Emergência ","guardarCofrinho":true,"data":"2026-10-14"}`,
			nova(of.Id, 2), 204},
	}
	for _, c := range casos {
		r := testeapoio.Chamar(t, mux, "POST", fmt.Sprintf("/api/contas-agendadas/%d/pagar", c.id), c.corpo)
		if r.Status != c.status {
			t.Errorf("%s: status %d: %s", c.nome, r.Status, r.Corpo)
			continue
		}
		if c.status != 204 {
			continue
		}
		var guardado *string
		var lancado string
		if err := db.QueryRow(`SELECT a."ReservaNome", t."ReservaNome" FROM "ContaAgendada" AS a
			INNER JOIN "PagamentoCofrinho" AS p ON p."ContaAgendadaId" = a."Id" INNER JOIN "Transacao" AS t ON t."Id" = p."TransacaoId"
			WHERE a."Id" = ?`, c.id).Scan(&guardado, &lancado); err != nil {
			t.Fatalf("%s: %v", c.nome, err)
		}
		querGuardado := c.nome == "guardando"
		if (guardado != nil) != querGuardado || (querGuardado && *guardado != "Emergência") ||
			(lancado != "Viagem" && lancado != "Emergência") {
			t.Errorf("%s: guardado %v, lançado pelo cofrinho %q", c.nome, guardado, lancado)
		}
	}
}

// Regra (#691): a lista oferece lançar pelo cofrinho o último pagamento recente (até 45 dias) de uma conta com "Pago
// com" que ficou sem despesa; não oferece se ela já foi lançada pelo cofrinho, se a cobrança do banco com o texto está
// no extrato no dia ou se o pagamento é antigo.
func TestUltimoSemDespesa(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome    string
		ultimo  tipos.Data
		extrato func(*testeapoio.Transacao)
		quer    string
	}{
		{"pago em 05/10 sem despesa", dia(5), nil, "2026-10-05"},
		{"pago há 45 dias", testeapoio.Dia(2026, 8, 30), nil, "2026-08-30"},
		{"pago há 46 dias", testeapoio.Dia(2026, 8, 29), nil, ""},
		{"já lançado pelo cofrinho", dia(5), func(tr *testeapoio.Transacao) { tr.Descricao, tr.ReservaNome = "Luz", ptr("Emergência") }, ""},
		{"lançado pelo cofrinho com outro nome", dia(5), func(tr *testeapoio.Transacao) {
			tr.Descricao, tr.ReservaNome = "Outra", ptr("Emergência")
		}, "2026-10-05"},
		{"cobrança do banco no dia", dia(5), func(tr *testeapoio.Transacao) { tr.Descricao = "PAGAMENTO ENEL" }, ""},
		{"cobrança do banco com o texto no original", dia(5), func(tr *testeapoio.Transacao) {
			tr.Descricao, tr.DescricaoOriginal = "Luz de casa", ptr("DEB ENEL")
		}, ""},
		{"outra saída do banco no dia", dia(5), func(tr *testeapoio.Transacao) { tr.Descricao = "PIX ENVIADO" }, "2026-10-05"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			_, fab, mux := ambiente(t)
			of := fab.Conta(func(x *testeapoio.Conta) { x.Origem = 2 })
			agendada := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
				a.Descricao, a.ContaId, a.ReservaNome, a.TextoCobranca = "Luz", &of.Id, ptr("Emergência"), ptr("ENEL")
				a.UltimoPagamentoEm, a.ProximoVencimento = &c.ultimo, dia(20)
			})
			if c.extrato != nil {
				fab.Transacao(of, func(tr *testeapoio.Transacao) {
					tr.Data = c.ultimo
					c.extrato(tr)
				})
			}
			type linha struct {
				Id               int64
				UltimoSemDespesa *string
			}
			for _, l := range lerJSON[[]linha](t, testeapoio.Chamar(t, mux, "GET", "/api/contas-agendadas", "").Corpo) {
				if l.Id != agendada.Id {
					continue
				}
				obtido := ""
				if l.UltimoSemDespesa != nil {
					obtido = *l.UltimoSemDespesa
				}
				if obtido != c.quer {
					t.Errorf("UltimoSemDespesa = %q, esperado %q", obtido, c.quer)
				}
			}
		})
	}
}
