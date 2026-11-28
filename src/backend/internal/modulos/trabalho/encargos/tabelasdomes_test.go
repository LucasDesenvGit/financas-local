package encargos

import (
	"errors"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// Regra (#772): o mês tem as tabelas vigentes do INSS e do IRRF; mês anterior a todas as vigências é ErrSemTabela.
func TestTabelasDoMes(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBanco(t)
	inss, irrf, err := TabelasDoMes(t.Context(), db, testeapoio.Dia(2026, 12, 1))
	if err != nil || len(inss.Faixas) == 0 || len(irrf.Faixas) == 0 {
		t.Fatalf("2026-12: inss=%v irrf=%v err=%v", inss, irrf, err)
	}
	if _, _, err := TabelasDoMes(t.Context(), db, testeapoio.Dia(1990, 1, 1)); !errors.Is(err, ErrSemTabela) {
		t.Fatalf("1990: err = %v, quero ErrSemTabela", err)
	}
}
