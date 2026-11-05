package saude_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/saude"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/versao"
)

// #86: até aqui, o GET /api/go/saude só era testado pelo modo "atrás do .NET" (cmd/financas), que saiu. Regras: com
// login, devolve quem está logado (lido do banco), o diário do banco e a hora do relógio do Go; sem login, ou com um
// usuário que não existe mais, 401 sem corpo.
func TestSaude(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	resultado, err := db.ExecContext(t.Context(), `INSERT INTO "Usuario" ("Login", "SenhaHash", "CarimboSeguranca", "CriadoEm")
		VALUES ('verificador', 'x', '6F1C2B9E-7A43-4D2E-9B8F-3C5A1E0D4F27', '2026-10-23 12:00:00')`)
	if err != nil {
		t.Fatal(err)
	}
	id, err := resultado.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	saude.Registrar(mux, db, slog.New(slog.DiscardHandler), testeapoio.RelogioDosTestes())

	t.Run("com login", func(t *testing.T) {
		r := testeapoio.ChamarComo(t, mux, id, "GET", "/api/go/saude", "")
		r.ConferirStatus(t, http.StatusOK)
		var corpo saude.Resposta
		if err := json.Unmarshal(r.Corpo, &corpo); err != nil {
			t.Fatal(err)
		}
		if corpo.Situacao != "ok" || corpo.Usuario != "verificador" || corpo.Versao != versao.Atual || corpo.DiarioDoBanco != "wal" {
			t.Errorf("corpo = %+v", corpo)
		}
		relogio := testeapoio.RelogioDosTestes()
		if !corpo.Agora.Tempo().Equal(relogio.Agora()) {
			t.Errorf("agora = %s, esperado o do relógio dos testes (%s)", corpo.Agora, relogio.Agora())
		}
		if got := r.Cabecalho.Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
	})
	t.Run("sem login", func(t *testing.T) {
		r := testeapoio.ChamarComo(t, mux, 0, "GET", "/api/go/saude", "")
		r.ConferirStatus(t, http.StatusUnauthorized)
		if len(r.Corpo) != 0 {
			t.Errorf("corpo = %q", r.Corpo)
		}
	})
	t.Run("usuário que não existe mais", func(t *testing.T) {
		r := testeapoio.ChamarComo(t, mux, id+100, "GET", "/api/go/saude", "")
		r.ConferirStatus(t, http.StatusUnauthorized)
	})
	t.Run("banco fechado: 500 com a mensagem genérica", func(t *testing.T) {
		outro := testeapoio.NovoBanco(t)
		m := http.NewServeMux()
		saude.Registrar(m, outro, slog.New(slog.DiscardHandler), testeapoio.RelogioDosTestes())
		if err := outro.Close(); err != nil {
			t.Fatal(err)
		}
		r := testeapoio.ChamarComo(t, m, id, "GET", "/api/go/saude", "")
		r.ConferirStatus(t, http.StatusInternalServerError)
	})
}
