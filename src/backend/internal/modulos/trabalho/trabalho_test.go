package trabalho

import (
	"encoding/json"
	"fmt"
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

// Revisão do #701 (2ª): no módulo montado, o 13º lê das férias a 1ª parcela que veio com elas, no valor que elas pagaram
// (a metade de 6000, mesmo com o aumento para 7000 em setembro): a 2ª é o líquido menos 3000.
func TestDecimoComAsFeriasNoModulo(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	cat := modulos.NovoCatalogo(db, r, log)
	cat.Adicionar(Novo(db, r, log))
	mux := http.NewServeMux()
	cat.Montar(mux)
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, mux, metodo, caminho, corpo)
	}
	chamar("PUT", "/api/modulos/trabalho", `{"ligado":true}`).ConferirStatus(t, http.StatusOK)
	var e struct{ Id int64 }
	_ = json.Unmarshal(chamar("POST", "/api/trabalho/empregos", `{"empregador":"E","admissao":"2024-03-11","salarioBruto":6000}`).Corpo, &e)
	base := fmt.Sprintf("/api/trabalho/empregos/%d", e.Id)
	chamar("POST", base+"/salarios", `{"inicio":"2026-09-01","salarioBruto":7000}`).ConferirStatus(t, http.StatusOK)
	chamar("POST", base+"/ferias", `{"periodoInicio":"2025-03-11","inicio":"2026-07-15","dias":30,"adiantarDecimo":true}`).
		ConferirStatus(t, http.StatusOK)
	var d struct {
		Primeira struct {
			Valor     json.Number
			ComFerias bool
		}
		Segunda struct{ Valor json.Number }
	}
	resp := chamar("GET", base+"/decimo-terceiro/2026", "")
	resp.ConferirStatus(t, http.StatusOK)
	_ = json.Unmarshal(resp.Corpo, &d)
	if d.Primeira.Valor.String() != "3000.00" || !d.Primeira.ComFerias || d.Segunda.Valor.String() != "2463.75" {
		t.Fatalf("13º = %s", resp.Corpo)
	}
}
