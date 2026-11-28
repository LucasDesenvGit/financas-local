package holerite

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

// Regressão (#750, revisão): o lido do PDF não exige tabela nem salário para entrar; por isso reabre sempre e, aberto,
// pode ser excluído. O calculado nas mesmas condições continua recusando o reabrir (TestReabrirRecusaQuandoOCalculoSumiria).
func TestLidoSemTabelaOuSemSalarioReabreEExclui(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ nome, mes, sql string }{
		{"mês sem tabela", "2025-03", ""},
		{"sem salário no mês", "2026-09", `UPDATE "SalarioVigencia" SET "Inicio" = '2026-11-01' WHERE "EmpregoId" = ?`},
	} {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			a := novo(t)
			ctx := context.Background()
			id := a.empregoDe5500()
			h, _, err := a.s.GravarLido(ctx, id, lido(c.mes+"-01"))
			if err != nil {
				t.Fatal(err)
			}
			if c.sql != "" {
				if _, err := a.s.db.ExecContext(ctx, c.sql, id); err != nil {
					t.Fatal(err)
				}
			}
			caminho := fmt.Sprintf("/api/trabalho/holerites/%d", h.Id)
			a.chamar("DELETE", caminho, "").ConferirStatus(t, http.StatusConflict) // fechado: reabra antes
			a.chamar("POST", caminho+"/reabrir", "").ConferirStatus(t, http.StatusOK)
			if depois := decodificar[Holerite](t, a.chamar("GET", caminho, "")); depois.Origem != Calculado || depois.Situacao != Aberto {
				t.Fatalf("reaberto: %+v", depois)
			}
			a.chamar("DELETE", caminho, "").ConferirStatus(t, http.StatusNoContent)
		})
	}
}
