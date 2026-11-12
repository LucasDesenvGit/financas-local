package cobrancas

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// agoraDoTeste é o carimbo do que a conciliação grava: 14/10/2026 10:00 UTC, antes do carimbo da fábrica (15:00 UTC,
// o meio-dia de Brasília), para a cobrança da fábrica ser "importada depois" da baixa.
var agoraDoTeste = func() tipos.DataHora {
	d, err := tipos.LerDataHora("2026-10-14 10:00:00")
	if err != nil {
		panic(err)
	}
	return d
}()

// cenarioDoCofrinho é um banco com uma conta do Open Finance e uma conta a pagar de 100.0, mensal, em débito
// automático pelo cofrinho "Emergência", que venceu em 10/10 (hoje é 14/10).
func cenarioDoCofrinho(t *testing.T, ajustar ...func(*testeapoio.ContaAgendada, testeapoio.Conta)) (*sql.DB,
	*testeapoio.Fabrica, testeapoio.Conta, testeapoio.ContaAgendada) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 14))
	conta := fab.Conta(func(c *testeapoio.Conta) { c.Origem = 2 })
	categoria := fab.Categoria()
	agendada := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.Descricao, a.ProximoVencimento, a.FormaCobranca = "Conta de luz", testeapoio.Dia(2026, 10, 10), FormaDebitoAutomatico
		a.ContaId, a.CategoriaId, a.ReservaNome = &conta.Id, &categoria.Id, ptr("Emergência")
		for _, f := range ajustar {
			f(a, conta)
		}
	})
	return db, fab, conta, agendada
}

// estadoDoCofrinho é a conta a pagar e os lançamentos pagos pelo cofrinho, num texto para comparar.
func estadoDoCofrinho(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	var proximo string
	var ultimo *string
	var ativa bool
	if err := db.QueryRow(`SELECT "ProximoVencimento", "UltimoPagamentoEm", "Ativa" FROM "ContaAgendada" WHERE "Id" = ?`, id).
		Scan(&proximo, &ultimo, &ativa); err != nil {
		t.Fatal(err)
	}
	partes := []string{"próximo " + proximo}
	if ultimo != nil {
		partes = append(partes, "último "+*ultimo)
	}
	if !ativa {
		partes = append(partes, "encerrada")
	}
	linhas, err := db.Query(`SELECT "Data", "Valor", "ReservaNome", "Descricao", "Tipo", "CategoriaManual" FROM "Transacao"
		WHERE "ReservaNome" IS NOT NULL ORDER BY "Data"`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var data, valor, reserva, descricao string
		var tipo int
		var manual bool
		if err := linhas.Scan(&data, &valor, &reserva, &descricao, &tipo, &manual); err != nil {
			t.Fatal(err)
		}
		if tipo != 1 || !manual || descricao != "Conta de luz" {
			t.Errorf("lançamento %s: tipo %d, categoria manual %v, descrição %q", data, tipo, manual, descricao)
		}
		partes = append(partes, data+" "+valor+" "+reserva)
	}
	return strings.Join(partes, "; ")
}

// Regra (#691, ADR 0063): a conta em débito automático pelo cofrinho recebe a baixa sozinha no vencimento: a despesa
// do app na data dele, com a categoria e a reserva, e o vencimento avança. Atrasada, uma baixa por vencimento; na data
// limite, encerra. À mão, de conta manual, ainda não vencida ou cobrada pelo extrato, nada.
func TestBaixaPeloCofrinho(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome    string
		ajustar func(*testeapoio.ContaAgendada, testeapoio.Conta)
		extrato func(*testeapoio.Fabrica, testeapoio.Conta)
		quer    string
		mudaram int
	}{
		{nome: "venceu: baixa no vencimento", mudaram: 1,
			quer: "próximo 2026-11-10; último 2026-10-10; 2026-10-10 -100.0 Emergência"},
		{nome: "vence amanhã: nada", ajustar: func(a *testeapoio.ContaAgendada, _ testeapoio.Conta) {
			a.ProximoVencimento = testeapoio.Dia(2026, 10, 15)
		}, quer: "próximo 2026-10-15"},
		{nome: "vence hoje: baixa", ajustar: func(a *testeapoio.ContaAgendada, _ testeapoio.Conta) {
			a.ProximoVencimento = testeapoio.Dia(2026, 10, 14)
		}, mudaram: 1, quer: "próximo 2026-11-14; último 2026-10-14; 2026-10-14 -100.0 Emergência"},
		{nome: "atrasada dois meses: uma baixa por vencimento", ajustar: func(a *testeapoio.ContaAgendada, _ testeapoio.Conta) {
			a.ProximoVencimento = testeapoio.Dia(2026, 8, 10)
		}, mudaram: 3, quer: "próximo 2026-11-10; último 2026-10-10; 2026-08-10 -100.0 Emergência; " +
			"2026-09-10 -100.0 Emergência; 2026-10-10 -100.0 Emergência"},
		{nome: "último pagamento: encerra", ajustar: func(a *testeapoio.ContaAgendada, _ testeapoio.Conta) {
			limite := testeapoio.Dia(2026, 10, 10)
			a.DataLimite = &limite
		}, mudaram: 1, quer: "próximo 2026-11-10; último 2026-10-10; encerrada; 2026-10-10 -100.0 Emergência"},
		{nome: "à mão: espera o Pagar", ajustar: func(a *testeapoio.ContaAgendada, _ testeapoio.Conta) {
			a.FormaCobranca = FormaAMao
		}, quer: "próximo 2026-10-10"},
		{nome: "sem cofrinho: espera o extrato", ajustar: func(a *testeapoio.ContaAgendada, _ testeapoio.Conta) {
			a.ReservaNome = nil
		}, quer: "próximo 2026-10-10"},
		{nome: "sem conta de pagamento: nada", ajustar: func(a *testeapoio.ContaAgendada, _ testeapoio.Conta) {
			a.ContaId = nil
		}, quer: "próximo 2026-10-10"},
		{nome: "a cobrança já está no extrato: concilia, sem lançamento do app",
			ajustar: func(a *testeapoio.ContaAgendada, _ testeapoio.Conta) { a.TextoCobranca = ptr("ENEL") },
			extrato: func(f *testeapoio.Fabrica, c testeapoio.Conta) {
				f.Transacao(c, func(tr *testeapoio.Transacao) {
					tr.Data, tr.Valor, tr.Descricao = testeapoio.Dia(2026, 10, 11), testeapoio.Valor("-98.0"), "PAGAMENTO ENEL"
				})
			}, mudaram: 1, quer: "próximo 2026-11-10; último 2026-10-11"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			var ajustes []func(*testeapoio.ContaAgendada, testeapoio.Conta)
			if c.ajustar != nil {
				ajustes = append(ajustes, c.ajustar)
			}
			db, fab, conta, agendada := cenarioDoCofrinho(t, ajustes...)
			if c.extrato != nil {
				c.extrato(fab, conta)
			}
			mudaram, err := Conciliar(t.Context(), db, testeapoio.Dia(2026, 10, 14), agoraDoTeste)
			if err != nil {
				t.Fatal(err)
			}
			if obtido := estadoDoCofrinho(t, db, agendada.Id); obtido != c.quer || mudaram != c.mudaram {
				t.Errorf("Conciliar = %d, %q; esperado %d, %q", mudaram, obtido, c.mudaram, c.quer)
			}
		})
	}
}

// Regra (#691): "Desfazer" apaga o lançamento e volta a conta a pagar como estava; o vencimento desfeito não recebe a
// baixa automática de novo. Depois de a conta mudar, não desfaz; sem baixa, também não.
func TestDesfazerPagamentoPeloCofrinho(t *testing.T) {
	t.Parallel()
	db, _, _, agendada := cenarioDoCofrinho(t, func(a *testeapoio.ContaAgendada, _ testeapoio.Conta) {
		anterior := testeapoio.Dia(2026, 9, 10)
		a.UltimoPagamentoEm = &anterior
	})
	ctx, hoje := t.Context(), testeapoio.Dia(2026, 10, 14)
	if err := DesfazerPagamento(ctx, db, agendada.Id); !errors.Is(err, ErrSemPagamento) {
		t.Errorf("DesfazerPagamento sem baixa = %v, esperado ErrSemPagamento", err)
	}
	if _, err := Conciliar(ctx, db, hoje, agoraDoTeste); err != nil {
		t.Fatal(err)
	}
	if err := DesfazerPagamento(ctx, db, agendada.Id); err != nil {
		t.Fatal(err)
	}
	if obtido, quer := estadoDoCofrinho(t, db, agendada.Id), "próximo 2026-10-10; último 2026-09-10"; obtido != quer {
		t.Errorf("depois de desfazer: %q, esperado %q", obtido, quer)
	}
	if mudaram, err := Conciliar(ctx, db, hoje, agoraDoTeste); err != nil || mudaram != 0 {
		t.Errorf("Conciliar depois de desfazer = %d, %v; esperado nenhuma baixa", mudaram, err)
	}
	if err := DesfazerPagamento(ctx, db, agendada.Id); !errors.Is(err, ErrSemPagamento) {
		t.Errorf("DesfazerPagamento de novo = %v, esperado ErrSemPagamento", err)
	}

	// Paga de novo à mão pelo cofrinho e a conta muda depois: não desfaz mais.
	c := &ContaDoCofrinho{Id: agendada.Id, Descricao: "Conta de luz", Valor: testeapoio.Valor("100.0"),
		ProximoVencimento: testeapoio.Dia(2026, 10, 10), Recorrencia: 1, ContaId: *agendada.ContaId, Ativa: true,
		ReservaNome: "Emergência"}
	if err := PagarPeloCofrinho(ctx, db, c, hoje, false, agoraDoTeste); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE "ContaAgendada" SET "ProximoVencimento" = '2026-12-10' WHERE "Id" = ?`, agendada.Id); err != nil {
		t.Fatal(err)
	}
	if err := DesfazerPagamento(ctx, db, agendada.Id); !errors.Is(err, ErrPagamentoMudou) {
		t.Errorf("DesfazerPagamento com a conta mudada = %v, esperado ErrPagamentoMudou", err)
	}
}

// Regra (#691): a baixa automática some quando a cobrança chega mesmo assim pelo extrato, com o texto, até 10 dias do
// vencimento e importada depois da baixa; o último pagamento vai para a data da cobrança. Cobrança importada antes,
// fora da janela ou sem o texto não desfaz.
func TestBaixaAutomaticaDuplicada(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		cobranca func(*testeapoio.Transacao)
		quer     string
	}{
		{nome: "chegou pelo extrato: vale a do banco", quer: "próximo 2026-11-10; último 2026-10-13"},
		{nome: "importada antes da baixa: fica a baixa", cobranca: func(tr *testeapoio.Transacao) {
			tr.CriadoEm = agoraDoTeste
		}, quer: "próximo 2026-11-10; último 2026-10-10; 2026-10-10 -100.0 Emergência"},
		{nome: "11 dias depois do vencimento: fica a baixa", cobranca: func(tr *testeapoio.Transacao) {
			tr.Data = testeapoio.Dia(2026, 10, 21)
		}, quer: "próximo 2026-11-10; último 2026-10-10; 2026-10-10 -100.0 Emergência"},
		{nome: "outro texto: fica a baixa", cobranca: func(tr *testeapoio.Transacao) {
			tr.Descricao = "PAGAMENTO SABESP"
		}, quer: "próximo 2026-11-10; último 2026-10-10; 2026-10-10 -100.0 Emergência"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db, fab, conta, agendada := cenarioDoCofrinho(t, func(a *testeapoio.ContaAgendada, _ testeapoio.Conta) {
				a.TextoCobranca = ptr("ENEL")
			})
			ctx := t.Context()
			if _, err := Conciliar(ctx, db, testeapoio.Dia(2026, 10, 14), agoraDoTeste); err != nil {
				t.Fatal(err)
			}
			fab.Transacao(conta, func(tr *testeapoio.Transacao) {
				tr.Data, tr.Valor, tr.Descricao = testeapoio.Dia(2026, 10, 13), testeapoio.Valor("-98.0"), "PAGAMENTO ENEL"
				if c.cobranca != nil {
					c.cobranca(tr)
				}
			})
			if _, err := Conciliar(ctx, db, testeapoio.Dia(2026, 10, 25), agoraDoTeste); err != nil {
				t.Fatal(err)
			}
			if obtido := estadoDoCofrinho(t, db, agendada.Id); obtido != c.quer {
				t.Errorf("depois da cobrança: %q, esperado %q", obtido, c.quer)
			}
		})
	}
}

// Regra (#691): "Ajustar valor" muda o lançamento da última baixa (sempre negativo), e a lista mostra a última baixa
// com o valor positivo, de onde saiu e se dá para desfazer.
func TestAjustarEUltimoPeloCofrinho(t *testing.T) {
	t.Parallel()
	db, _, _, agendada := cenarioDoCofrinho(t)
	ctx := t.Context()
	if err := AjustarPagamento(ctx, db, agendada.Id, testeapoio.Valor("87.35"), agoraDoTeste); !errors.Is(err, ErrSemPagamento) {
		t.Errorf("AjustarPagamento sem baixa = %v, esperado ErrSemPagamento", err)
	}
	if _, err := Conciliar(ctx, db, testeapoio.Dia(2026, 10, 14), agoraDoTeste); err != nil {
		t.Fatal(err)
	}
	if err := AjustarPagamento(ctx, db, agendada.Id, testeapoio.Valor("87.35"), agoraDoTeste); err != nil {
		t.Fatal(err)
	}
	if obtido, quer := estadoDoCofrinho(t, db, agendada.Id), "próximo 2026-11-10; último 2026-10-10; 2026-10-10 -87.35 Emergência"; obtido != quer {
		t.Errorf("depois de ajustar: %q, esperado %q", obtido, quer)
	}
	ultimos, err := UltimosPeloCofrinho(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	u, ok := ultimos[agendada.Id]
	if !ok || u.Valor.String() != "87.35" || u.ReservaNome != "Emergência" || !u.Automatico || !u.PodeDesfazer ||
		u.Data.String() != "2026-10-10" {
		t.Errorf("UltimosPeloCofrinho = %+v", ultimos)
	}
}
