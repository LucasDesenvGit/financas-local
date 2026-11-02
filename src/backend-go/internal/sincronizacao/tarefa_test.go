package sincronizacao

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// relogioAjustavel é um relógio que o teste adianta: os ciclos da tarefa acontecem em horas diferentes.
type relogioAjustavel struct{ t time.Time }

func (r *relogioAjustavel) Agora() time.Time { return r.t }

// autenticacoes conta as idas à Pluggy: toda rodada começa por uma.
func (c cenario) autenticacoes() int {
	n := 0
	for _, p := range c.falsa.Pedidos() {
		if p.Caminho == "/auth" {
			n++
		}
	}
	return n
}

// configurar muda uma chave que os dados iniciais já criaram.
func (c cenario) configurar(t *testing.T, chave, valor string) {
	t.Helper()
	if _, err := c.db.Exec(`UPDATE "Configuracao" SET "Valor" = ? WHERE "Chave" = ?`, valor, chave); err != nil {
		t.Fatal(err)
	}
}

// O relógio do novoCenario marca 15/10/2026, meio-dia em Brasília (15h em UTC).
func TestCicloDaTarefa(t *testing.T) {
	semIntervalo := func(ultima string) func(*testing.T, cenario) {
		return func(t *testing.T, c cenario) {
			if _, err := c.db.Exec(`DELETE FROM "Configuracao" WHERE "Chave" = ?`, chaveIntervaloHoras); err != nil {
				t.Fatal(err)
			}
			c.fab.Configuracao(configuracao.UltimaSincronizacaoGeral, ultima)
		}
	}
	intervalo := func(horas int, valor string) func(*testing.T, cenario) {
		return func(t *testing.T, c cenario) {
			c.configurar(t, chaveIntervaloHoras, fmt.Sprint(horas))
			c.fab.Configuracao(configuracao.UltimaSincronizacaoGeral, valor)
		}
	}
	casos := []struct {
		nome       string
		preparar   func(t *testing.T, c cenario)
		sincroniza bool
	}{
		{"nunca sincronizou", func(*testing.T, cenario) {}, true},
		{"ligada de propósito", func(t *testing.T, c cenario) { c.configurar(t, chaveHabilitada, "true") }, true},
		{"desligada", func(t *testing.T, c cenario) { c.configurar(t, chaveHabilitada, "false") }, false},
		{"sem a chave, fica ligada", func(t *testing.T, c cenario) {
			if _, err := c.db.Exec(`DELETE FROM "Configuracao" WHERE "Chave" = ?`, chaveHabilitada); err != nil {
				t.Fatal(err)
			}
		}, true},
		{"uma rodada em andamento", func(_ *testing.T, c cenario) { c.s.emExecucao.Store(true) }, false},
		{"sem conexão ativa", func(t *testing.T, c cenario) {
			if _, err := c.db.Exec(`UPDATE "ConexaoBancaria" SET "Ativa" = 0`); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"sem a chave, 6 horas: 5h59", semIntervalo("2026-10-15T09:01:00Z"), false},
		{"sem a chave, 6 horas: 6 horas", semIntervalo("2026-10-15T09:00:00Z"), true},
		{"hora sem fuso é a de Brasília: 4 horas, e não 7", semIntervalo("2026-10-15T08:00:00"), false},
		{"intervalo de 2 horas: 1h59", intervalo(2, "2026-10-15T13:01:00Z"), false},
		{"intervalo de 2 horas: 2 horas", intervalo(2, "2026-10-15T13:00:00Z"), true},
		{"abaixo do mínimo vale 1 hora: 59 minutos", intervalo(0, "2026-10-15T14:01:00Z"), false},
		{"abaixo do mínimo vale 1 hora: 1 hora", intervalo(0, "2026-10-15T14:00:00Z"), true},
		{"acima do máximo vale 48 horas: 47h59", intervalo(100, "2026-10-13T15:01:00Z"), false},
		{"acima do máximo vale 48 horas: 48 horas", intervalo(100, "2026-10-13T15:00:00Z"), true},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			c.pluggyCompleta(t)
			caso.preparar(t, c)
			if err := NovaTarefa(c.s, slog.New(slog.DiscardHandler)).Ciclo(t.Context()); err != nil {
				t.Fatal(err)
			}
			if n := c.autenticacoes(); (n == 1) != caso.sincroniza {
				t.Errorf("%d rodadas, esperado sincronizar = %v", n, caso.sincroniza)
			}
		})
	}
}

// Regra: a rodada da tarefa é de todas as conexões, conta como automática e deixa no log o que fez.
func TestTarefaSincronizaComoAutomatica(t *testing.T) {
	c := novoCenario(t)
	c.pluggyCompleta(t)
	c.fab.ConexaoBancaria(func(k *testeapoio.ConexaoBancaria) { k.PluggyItemId = "outro-item" })
	c.falsa.Em("GET", "/items/outro-item", arquivo(t, "item-"+item))
	c.falsa.Em("GET", "/v2/transactions", testeapoio.RespostaDeFora{Corpo: `{"results":[{"id":"t1","date":"2026-10-14T12:00:00Z","description":"Mercado","amount":-10,"status":"POSTED","type":"DEBIT"}]}`})
	var log bytes.Buffer
	if err := NovaTarefa(c.s, slog.New(slog.NewTextHandler(&log, nil))).Ciclo(t.Context()); err != nil {
		t.Fatal(err)
	}
	if o := c.valor(t, `SELECT "Origem" FROM "SincronizacaoLog" WHERE "ConexaoBancariaId" IS NOT NULL ORDER BY "Id" LIMIT 1`); o != OrigemAutomatica {
		t.Errorf("origem = %q", o)
	}
	if n := c.valor(t, `SELECT COUNT(DISTINCT "ConexaoBancariaId") FROM "SincronizacaoLog"`); n != "2" {
		t.Errorf("%s conexões sincronizadas", n)
	}
	for _, trecho := range []string{"Sincronização automática.", "origem=\"Open Finance\"", "novas=", "atualizadas="} {
		if !strings.Contains(log.String(), trecho) {
			t.Errorf("o log não tem %q:\n%s", trecho, log.String())
		}
	}
	if strings.Contains(log.String(), "novas=0") {
		t.Errorf("o log não contou a transação nova:\n%s", log.String())
	}
}

// Regra: com a sincronização falhando antes de marcar a última execução (credenciais recusadas), a tarefa não tenta a
// cada 5 minutos: só 1 hora depois da tentativa.
func TestNovaTentativaSoDepoisDeUmaHora(t *testing.T) {
	c := novoCenario(t)
	c.credenciais(t)
	c.fab.ConexaoBancaria()
	c.falsa.Em("POST", "/auth", testeapoio.NaoAutorizado)
	relogio := &relogioAjustavel{c.s.relogio.Agora()}
	c.s.relogio = relogio
	tarefa := NovaTarefa(c.s, slog.New(slog.DiscardHandler))
	passos := []struct {
		adiantar time.Duration
		rodadas  int
	}{{0, 1}, {IntervaloDaTarefa, 1}, {54 * time.Minute, 1}, {time.Minute, 2}, {IntervaloDaTarefa, 2}}
	for i, passo := range passos {
		relogio.t = relogio.t.Add(passo.adiantar)
		if err := tarefa.Ciclo(t.Context()); err != nil {
			t.Fatal(err)
		}
		if n := c.autenticacoes(); n != passo.rodadas {
			t.Fatalf("ciclo %d: %d rodadas, esperado %d", i, n, passo.rodadas)
		}
	}
}

// Regra: o ciclo que não roda não gasta a tentativa: assim que o motivo passa, o ciclo seguinte sincroniza, sem esperar 1 hora.
func TestCicloSemRodadaNaoGastaATentativa(t *testing.T) {
	conexoes := func(t *testing.T, c cenario, ativa int) {
		t.Helper()
		if _, err := c.db.Exec(`UPDATE "ConexaoBancaria" SET "Ativa" = ?`, ativa); err != nil {
			t.Fatal(err)
		}
	}
	casos := []struct {
		nome             string
		impedir, liberar func(t *testing.T, c cenario)
	}{
		{"rodada em andamento",
			func(_ *testing.T, c cenario) { c.s.emExecucao.Store(true) },
			func(_ *testing.T, c cenario) { c.s.emExecucao.Store(false) }},
		{"desligada",
			func(t *testing.T, c cenario) { c.configurar(t, chaveHabilitada, "false") },
			func(t *testing.T, c cenario) { c.configurar(t, chaveHabilitada, "true") }},
		{"sem conexão ativa",
			func(t *testing.T, c cenario) { conexoes(t, c, 0) },
			func(t *testing.T, c cenario) { conexoes(t, c, 1) }},
		{"o intervalo não venceu",
			func(_ *testing.T, c cenario) {
				c.fab.Configuracao(configuracao.UltimaSincronizacaoGeral, "2026-10-15T14:59:00Z")
			},
			func(t *testing.T, c cenario) {
				c.configurar(t, configuracao.UltimaSincronizacaoGeral, "2026-10-14T00:00:00Z")
			}},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			c.pluggyCompleta(t)
			tarefa := NovaTarefa(c.s, slog.New(slog.DiscardHandler))
			caso.impedir(t, c)
			if err := tarefa.Ciclo(t.Context()); err != nil {
				t.Fatal(err)
			}
			if n := c.autenticacoes(); n != 0 {
				t.Fatalf("%d rodadas com o ciclo impedido", n)
			}
			caso.liberar(t, c)
			if err := tarefa.Ciclo(t.Context()); err != nil {
				t.Fatal(err)
			}
			if n := c.autenticacoes(); n != 1 {
				t.Errorf("%d rodadas depois de liberar, esperada 1", n)
			}
		})
	}
}

// Regra: o erro de banco volta para tarefas.Periodica, que o registra. Sem a tabela, a leitura falha.
func TestCicloComErroDoBanco(t *testing.T) {
	casos := []struct{ nome, comando string }{
		{"sem a Configuracao", `DROP TABLE "Configuracao"`},
		{"sem a ConexaoBancaria", `DROP TABLE "ConexaoBancaria"`},
		{"sem o SincronizacaoLog", `DROP TABLE "SincronizacaoLog"`},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := novoCenario(t)
			c.pluggyCompleta(t)
			if _, err := c.db.Exec(caso.comando); err != nil {
				t.Fatal(err)
			}
			if err := NovaTarefa(c.s, slog.New(slog.DiscardHandler)).Ciclo(t.Context()); err == nil {
				t.Error("a tabela sumiu e o ciclo não deu erro")
			}
		})
	}
}

// Regra: o ctx para a tarefa; sem ele, ela nem chega ao primeiro ciclo.
func TestRodarParaComOContexto(t *testing.T) {
	c := novoCenario(t)
	c.pluggyCompleta(t)
	ctx, cancelar := context.WithCancel(t.Context())
	cancelar()
	NovaTarefa(c.s, slog.New(slog.DiscardHandler)).Rodar(ctx)
	if n := len(c.falsa.Pedidos()); n != 0 {
		t.Errorf("%d pedidos à Pluggy", n)
	}
}
