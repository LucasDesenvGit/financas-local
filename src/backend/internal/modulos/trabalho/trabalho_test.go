package trabalho

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/modulos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

func TestModuloTrabalhoNoCatalogo(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.RelogioDosTestes()
	log := slog.New(slog.DiscardHandler)
	m := Novo(db, r, log)
	if i := m.Info(); i.ID != ID || i.AcessaInternet() || i.LigadoPorPadrao || len(m.Tarefas()) != 0 {
		t.Fatalf("info = %+v, tarefas = %d", i, len(m.Tarefas()))
	}
	cat := modulos.NovoCatalogo(db, r, log)
	cat.Adicionar(m) // pânico se as rotas saírem de /api/trabalho/
	mux := http.NewServeMux()
	cat.Montar(mux)

	// Nasce desligado: as rotas respondem 409, sem ler nada.
	resp := testeapoio.Chamar(t, mux, "GET", "/api/trabalho/empregos", "")
	resp.ConferirStatus(t, http.StatusConflict)
	resp.ConferirJSON(t, `{"mensagem":"O módulo Trabalho está desligado. Ligue em Configurações › Módulos."}`)

	testeapoio.Chamar(t, mux, "PUT", "/api/modulos/trabalho", `{"ligado":true}`).ConferirStatus(t, http.StatusOK)
	testeapoio.Chamar(t, mux, "GET", "/api/trabalho/empregos", "").ConferirJSON(t, `[]`)
	testeapoio.Chamar(t, mux, "GET", "/api/trabalho/tabelas", "").ConferirStatus(t, http.StatusOK)
}
