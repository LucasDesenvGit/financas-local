package api_test

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/api"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// #86: até aqui, o api.Montar só era chamado nos testes pelo modo "atrás do .NET" (cmd/financas), que saiu. Regras: os
// assuntos ficam registrados no mesmo mux; caminho sem rota recebe 404 sem corpo (o MapFallback do .NET); e a
// sincronização das rotas é a mesma instância que a tarefa recebe.
func TestMontar(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	montado := api.Montar(api.Dependencias{DB: db, Log: slog.New(slog.DiscardHandler), Relogio: testeapoio.RelogioDosTestes()})
	if montado.Rotas == nil || montado.Sincronizacao == nil {
		t.Fatalf("montado = %+v", montado)
	}

	t.Run("sem rota: 404 sem corpo", func(t *testing.T) {
		for _, caminho := range []string{"/api/nao-existe", "/api", "/qualquer"} {
			r := testeapoio.Chamar(t, montado.Rotas, "GET", caminho, "")
			r.ConferirStatus(t, http.StatusNotFound)
			if len(r.Corpo) != 0 || r.Cabecalho.Get("Content-Length") != "0" {
				t.Errorf("%s: corpo %q, Content-Length %q", caminho, r.Corpo, r.Cabecalho.Get("Content-Length"))
			}
		}
	})
	t.Run("os assuntos respondem", func(t *testing.T) {
		// Uma rota de cada parte do api.go: a resposta não importa aqui, só que a rota existe (não é o 404 sem corpo).
		for _, caminho := range []string{
			"/api/parametros", "/api/cambio/cotacoes", "/api/cripto/cotacoes", "/api/categorias", "/api/contas",
			"/api/transacoes", "/api/orcamentos", "/api/metas", "/api/contas-agendadas", "/api/carteira",
			"/api/reservas", "/api/investimentos", "/api/painel", "/api/open-finance/configuracao",
		} {
			r := testeapoio.Chamar(t, montado.Rotas, "GET", caminho, "")
			if r.Status == http.StatusNotFound && len(r.Corpo) == 0 {
				t.Errorf("%s: sem rota", caminho)
			}
		}
	})
	t.Run("NovasRotas é o mux do Montar", func(t *testing.T) {
		r := testeapoio.Chamar(t, api.NovasRotas(api.Dependencias{DB: db, Log: slog.New(slog.DiscardHandler),
			Relogio: testeapoio.RelogioDosTestes()}), "GET", "/api/nao-existe", "")
		r.ConferirStatus(t, http.StatusNotFound)
	})
}
