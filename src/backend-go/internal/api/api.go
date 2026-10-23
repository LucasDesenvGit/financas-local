// Pacote api junta as rotas de todos os assuntos já em Go num ServeMux só. É o MapControllers do Go.
//
// Rota nova entra aqui com o padrão do Go 1.22 ("GET /api/assunto/{id}"). Ela só recebe requisição quando o
// prefixo dela está na lista Go:Prefixos do .NET (ADR 0049): enquanto não estiver, o .NET continua atendendo.
package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/saude"
)

// Dependencias de todos os assuntos, passadas pelo construtor (sem container de injeção).
type Dependencias struct {
	DB  *sql.DB
	Log *slog.Logger
}

// NovasRotas devolve o ServeMux com as rotas de todos os assuntos. Caminho sem rota recebe 404 sem corpo, como o
// MapFallback("/api/{**caminho}") do .NET.
func NovasRotas(d Dependencias) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusNotFound)
	})

	// #231: o exemplo que passa pelo caminho todo (front-end → .NET → Go → banco).
	mux.Handle("GET /api/go/saude", saude.Handler(d.DB, d.Log))
	return mux
}
