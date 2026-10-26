package cambio

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// relogioAjustavel é um relógio que o teste adianta (os ciclos da tarefa acontecem em horas diferentes).
type relogioAjustavel struct {
	mu sync.Mutex
	t  time.Time
}

func (r *relogioAjustavel) Agora() time.Time { r.mu.Lock(); defer r.mu.Unlock(); return r.t }
func (r *relogioAjustavel) Adiantar(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.t = r.t.Add(d)
}

func TestCicloDaTarefa(t *testing.T) {
	t.Parallel()
	// 14/10/2026, 14h em Brasília (17:00 UTC): já passou do fechamento das 13h30.
	r := &relogioAjustavel{t: time.Date(2026, 10, 14, 17, 0, 0, 0, time.UTC)}
	fonte := &fonteFalsa{moedas: []Moeda{{Codigo: "USD", Nome: "Dolar"}}, falham: map[string]bool{"USD": true}}
	s, db, fab := novoServicoDeTeste(t, r, fonte)
	log, saida := logDoTeste()
	tarefa := NovaTarefa(s, r, log)

	// A última atualização foi às 12h de hoje, antes do fechamento: venceu.
	fab.Configuracao(ChaveUltimaAtualizacao, "2026-10-14T15:00:00.0000000Z")
	if err := tarefa.Ciclo(t.Context()); err != nil {
		t.Fatal(err)
	}
	if consultas := strings.Join(fonte.consultou(), "|"); consultas != "moedas|USD 2026-10-07 2026-10-14" {
		t.Fatalf("primeiro ciclo consultou %q", consultas)
	}
	if !strings.Contains(saida.String(), "Câmbio atualizado parcialmente") || !strings.Contains(saida.String(), "falharam=USD") {
		t.Errorf("sem o aviso da falha:\n%s", saida.String())
	}

	// 15 minutos depois (próximo ciclo): falhou há menos de 30 minutos, não tenta de novo.
	r.Adiantar(15 * time.Minute)
	if err := tarefa.Ciclo(t.Context()); err != nil {
		t.Fatal(err)
	}
	if n := len(fonte.consultou()); n != 2 {
		t.Fatalf("tentou de novo antes de 30 minutos (%d consultas)", n)
	}

	// 31 minutos depois da tentativa, com internet: atualiza e marca.
	r.Adiantar(16 * time.Minute)
	fonte.mu.Lock()
	fonte.falham = nil
	fonte.fechamentos = map[string][]Fechamento{"USD": {{Data: dia(14), Compra: testeapoio.Valor("5.4123"), Venda: testeapoio.Valor("5.4129")}}}
	fonte.mu.Unlock()
	if err := tarefa.Ciclo(t.Context()); err != nil {
		t.Fatal(err)
	}
	if v := linha(t, db, `SELECT Valor FROM Configuracao WHERE Chave = ?`, ChaveUltimaAtualizacao); v[0] != "2026-10-14T17:31:00.0000000Z" {
		t.Errorf("última atualização %v", v)
	}

	// Já atualizado depois do fechamento: o próximo ciclo não faz nada.
	antes := len(fonte.consultou())
	r.Adiantar(15 * time.Minute)
	if err := tarefa.Ciclo(t.Context()); err != nil {
		t.Fatal(err)
	}
	if depois := len(fonte.consultou()); depois != antes {
		t.Errorf("consultou de novo sem vencer (%d → %d)", antes, depois)
	}
}
