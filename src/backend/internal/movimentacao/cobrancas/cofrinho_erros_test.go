package cobrancas

import (
	"context"
	"database/sql"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Regra (#691, como o "Pagar"): o próximo vencimento é um mês ou um ano depois; a única não tem próximo e encerra, e a
// que passa da data limite também.
func TestAvancar(t *testing.T) {
	t.Parallel()
	limite := testeapoio.Dia(2026, 11, 10)
	casos := []struct {
		nome        string
		recorrencia int64
		dataLimite  *tipos.Data
		proximo     string
		ativa       bool
	}{
		{"mensal", 1, nil, "2026-11-10", true},
		{"anual", 2, nil, "2027-10-10", true},
		{"única encerra", 0, nil, "2026-10-10", false},
		{"mensal na data limite continua", 1, &limite, "2026-11-10", true},
		{"anual depois da data limite encerra", 2, &limite, "2027-10-10", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			proximo, ativa := Avancar(testeapoio.Dia(2026, 10, 10), c.recorrencia, c.dataLimite)
			if proximo.String() != c.proximo || ativa != c.ativa {
				t.Errorf("Avancar = %s, %v; esperado %s, %v", proximo, ativa, c.proximo, c.ativa)
			}
		})
	}
}

// naoGrava é o gatilho que recusa a operação na tabela, para o teste ver o erro voltar.
func naoGrava(operacao, tabela string) string {
	return `CREATE TRIGGER "nao_` + operacao + `_` + tabela + `" BEFORE ` + operacao + ` ON "` + tabela +
		`" BEGIN SELECT RAISE(ABORT, 'não grava'); END`
}

func quebrarBanco(t *testing.T, db *sql.DB, quebrar string) {
	t.Helper()
	if _, err := db.Exec(quebrar); err != nil {
		t.Fatalf("%s: %v", quebrar, err)
	}
}

// Regra (#691): a falha ao ler ou gravar a baixa pelo cofrinho volta como erro, sem engolir.
func TestBaixaPeloCofrinhoErroDoBanco(t *testing.T) {
	t.Parallel()
	for _, quebrar := range []string{`DROP TABLE "PagamentoCofrinho"`, `DROP TABLE "RegraCategorizacao"`,
		naoGrava("INSERT", "Transacao"), naoGrava("INSERT", "PagamentoCofrinho"), naoGrava("UPDATE", "ContaAgendada")} {
		t.Run(quebrar, func(t *testing.T) {
			t.Parallel()
			db, _, _, _ := cenarioDoCofrinho(t)
			quebrarBanco(t, db, quebrar)
			if _, err := Conciliar(t.Context(), db, testeapoio.Dia(2026, 10, 14), agoraDoTeste); err == nil {
				t.Error("Conciliar devia falhar")
			}
		})
	}
}

// Regra (#691): a falha ao desfazer a duplicada (ler as regras, apagar o lançamento, gravar o último pagamento) volta
// como erro.
func TestBaixaDuplicadaErroDoBanco(t *testing.T) {
	t.Parallel()
	for _, quebrar := range []string{`DROP TABLE "RegraDescricao"`, naoGrava("DELETE", "Transacao"),
		naoGrava("UPDATE", "ContaAgendada")} {
		t.Run(quebrar, func(t *testing.T) {
			t.Parallel()
			db, fab, conta, _ := cenarioDoCofrinho(t, func(a *testeapoio.ContaAgendada, _ testeapoio.Conta) {
				a.TextoCobranca = ptr("ENEL")
			})
			if _, err := Conciliar(t.Context(), db, testeapoio.Dia(2026, 10, 14), agoraDoTeste); err != nil {
				t.Fatal(err)
			}
			fab.Transacao(conta, func(tr *testeapoio.Transacao) {
				tr.Data, tr.Valor, tr.Descricao = testeapoio.Dia(2026, 10, 13), testeapoio.Valor("-98.0"), "PAGAMENTO ENEL"
			})
			quebrarBanco(t, db, quebrar)
			if _, err := Conciliar(t.Context(), db, testeapoio.Dia(2026, 10, 25), agoraDoTeste); err == nil {
				t.Error("Conciliar devia falhar")
			}
		})
	}
}

// Regra (#691): desfazer, ajustar e listar a baixa pelo cofrinho devolvem o erro do banco.
func TestPagamentoPeloCofrinhoErroDoBanco(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome, quebrar string
		chamar        func(*sql.DB, int64) error
	}{
		{"desfazer sem a tabela", `DROP TABLE "PagamentoCofrinho"`, desfazer},
		{"desfazer sem apagar o lançamento", naoGrava("DELETE", "Transacao"), desfazer},
		{"desfazer sem voltar a conta", naoGrava("UPDATE", "ContaAgendada"), desfazer},
		{"ajustar sem a tabela", `DROP TABLE "PagamentoCofrinho"`, ajustar},
		{"ajustar sem gravar o lançamento", naoGrava("UPDATE", "Transacao"), ajustar},
		{"listar sem a tabela", `DROP TABLE "PagamentoCofrinho"`, func(db *sql.DB, _ int64) error {
			_, err := UltimosPeloCofrinho(t.Context(), db)
			return err
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db, _, _, agendada := cenarioDoCofrinho(t)
			if _, err := Conciliar(t.Context(), db, testeapoio.Dia(2026, 10, 14), agoraDoTeste); err != nil {
				t.Fatal(err)
			}
			quebrarBanco(t, db, c.quebrar)
			if err := c.chamar(db, agendada.Id); err == nil {
				t.Error("devia falhar")
			}
		})
	}
}

func desfazer(db *sql.DB, id int64) error {
	return DesfazerPagamento(context.Background(), db, id)
}

func ajustar(db *sql.DB, id int64) error {
	return AjustarPagamento(context.Background(), db, id, testeapoio.Valor("10.0"), agoraDoTeste)
}
