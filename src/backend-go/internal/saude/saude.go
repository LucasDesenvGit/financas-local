// Pacote saude é o endpoint de exemplo da convivência (#231): GET /api/go/saude passa pelo caminho todo
// (front-end → .NET → Go → banco) e diz se o Go está de pé, qual a versão dele, quem está logado (lido do banco)
// e o modo do diário do banco (tem de ser "wal" com os dois processos no mesmo arquivo).
package saude

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/versao"
)

// Resposta do GET /api/go/saude.
type Resposta struct {
	Situacao      string `json:"situacao"`      // "ok"
	Versao        string `json:"versao"`        // a do executável do Go (igual à do .NET, se publicado junto)
	Usuario       string `json:"usuario"`       // o login de quem está logado, lido da tabela Usuario
	DiarioDoBanco string `json:"diarioDoBanco"` // PRAGMA journal_mode: "wal"
}

// Handler responde o GET /api/go/saude.
func Handler(db *sql.DB, log *slog.Logger) http.HandlerFunc {
	q := consultas.New(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := sessao.Usuario(r.Context())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		login, err := q.ObterLoginDoUsuario(r.Context(), id)
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
		diario, err := banco.ModoDoDiario(r.Context(), db)
		if err != nil {
			log.Error("ler o modo do diário do banco", "erro", err)
			servidor.ResponderErroInterno(w)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(Resposta{Situacao: "ok", Versao: versao.Atual, Usuario: login, DiarioDoBanco: diario})
	}
}
