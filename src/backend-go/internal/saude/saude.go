// Pacote saude é o endpoint de exemplo da convivência (#231): GET /api/go/saude passa pelo caminho todo
// (front-end → .NET → Go → banco) e diz se o Go está de pé, qual a versão dele, quem está logado (lido do banco),
// o modo do diário do banco (tem de ser "wal" com os dois processos no mesmo arquivo) e a hora do relógio do Go
// (#363: no contrato, ela tem de ser a do relógio de teste, e não a do computador).
package saude

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/versao"
)

// Resposta do GET /api/go/saude.
type Resposta struct {
	Situacao      string         `json:"situacao"`      // "ok"
	Versao        string         `json:"versao"`        // a do executável do Go (igual à do .NET, se publicado junto)
	Usuario       string         `json:"usuario"`       // o login de quem está logado, lido da tabela Usuario
	DiarioDoBanco string         `json:"diarioDoBanco"` // PRAGMA journal_mode: "wal"
	Agora         tipos.DataHora `json:"agora"`         // a hora do relógio do Go, em UTC (como um CriadoEm)
	Hoje          tipos.Data     `json:"hoje"`          // a data de hoje em Brasília
}

// Registrar põe as rotas do assunto no mux (uma linha por assunto no internal/api).
func Registrar(mux *http.ServeMux, db *sql.DB, log *slog.Logger, r relogio.Relogio) {
	mux.Handle("GET /api/go/saude", Handler(db, log, r))
}

// Handler responde o GET /api/go/saude.
func Handler(db *sql.DB, log *slog.Logger, r relogio.Relogio) http.HandlerFunc {
	q := consultas.New(db)
	return func(w http.ResponseWriter, req *http.Request) {
		id, ok := sessao.Usuario(req.Context())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		login, err := q.ObterLoginDoUsuario(req.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			// O .NET já confere o carimbo da sessão a cada requisição; chegar aqui é usuário apagado no meio.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err != nil {
			log.Error("ler o usuário logado", "erro", err)
			servidor.ResponderErroInterno(w)
			return
		}
		diario, err := banco.ModoDoDiario(req.Context(), db)
		if err != nil {
			log.Error("ler o modo do diário do banco", "erro", err)
			servidor.ResponderErroInterno(w)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(Resposta{
			Situacao:      "ok",
			Versao:        versao.Atual,
			Usuario:       login,
			DiarioDoBanco: diario,
			Agora:         tipos.NovaDataHora(r.Agora()),
			Hoje:          relogio.Hoje(r),
		})
	}
}
