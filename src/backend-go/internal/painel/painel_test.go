package painel

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/emprestimos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// Regra: GET /api/emprestimos devolve o painel dos empréstimos de hoje (em Brasília), com os 12 meses até o atual;
// falha do banco é 500 com a mensagem genérica.
func TestEmprestimos(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, nil, nil, nil, testeapoio.HojeEmBrasilia(2026, 10, 14), slog.New(slog.DiscardHandler)))
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 14))
	fab.Emprestimo(fab.ConexaoBancaria())
	r := testeapoio.Chamar(t, mux, "GET", "/api/emprestimos", "")
	var p emprestimos.PainelEmprestimosDto
	if err := json.Unmarshal(r.Corpo, &p); err != nil || r.Status != http.StatusOK {
		t.Fatalf("%d %s %v", r.Status, r.Corpo, err)
	}
	ultimo := p.PagamentosPorMes[len(p.PagamentosPorMes)-1]
	if len(p.Contratos) != 1 || p.Emprestimos.ContratosAbertos != 1 || ultimo.Ano != 2026 || ultimo.Mes != 10 {
		t.Errorf("painel: %s", r.Corpo)
	}
	_ = db.Close()
	if r := testeapoio.Chamar(t, mux, "GET", "/api/emprestimos", ""); r.Status != http.StatusInternalServerError {
		t.Errorf("banco fechado: %d", r.Status)
	}
}
