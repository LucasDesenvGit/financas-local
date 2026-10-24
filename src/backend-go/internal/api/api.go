// Pacote api junta as rotas de todos os assuntos já em Go num ServeMux só. É o MapControllers do Go.
//
// Cada assunto tem o próprio pacote (internal/<assunto>) e exporta Registrar(mux, <o que ele usa>), que põe as
// rotas dele no mux com o padrão do Go 1.22 ("GET /api/assunto/{id}"). Aqui entra uma linha por assunto, e só isso:
// assim, assuntos feitos em paralelo (#75, #77, #81) não mexem nas mesmas linhas. No merge, conflito neste arquivo é
// sempre "fique com as duas linhas" (e o gofmt põe os imports em ordem).
//
// Uma rota só recebe requisição quando o prefixo dela está em Go:Prefixos do .NET (ADR 0049): enquanto não estiver,
// o .NET continua atendendo. O contrato liga o prefixo só na rodada contra o Go (PREFIXOS_GO, testes/contrato).
package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/saude"
)

// Dependencias de todos os assuntos, passadas pelo construtor (sem container de injeção). Cada assunto recebe no
// Registrar só o que usa.
type Dependencias struct {
	DB      *sql.DB
	Log     *slog.Logger
	Relogio relogio.Relogio // relogio.Sistema no site; relogio.APartirDe no contrato (-inicio-do-relogio)
}

// NovasRotas devolve o ServeMux com as rotas de todos os assuntos. Caminho sem rota recebe 404 sem corpo, como o
// MapFallback("/api/{**caminho}") do .NET.
func NovasRotas(d Dependencias) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusNotFound)
	})

	// Uma linha por assunto, na ordem do épico #70.
	saude.Registrar(mux, d.DB, d.Log, d.Relogio) // #231: o exemplo que passa pelo caminho todo (front-end → .NET → Go → banco)
	return mux
}
