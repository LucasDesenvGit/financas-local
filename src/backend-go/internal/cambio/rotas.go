package cambio

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sessao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Registrar põe as rotas de /api/cambio no mux (o CambioController), com o cliente da PTAX de verdade.
func Registrar(mux *http.ServeMux, db *sql.DB, log *slog.Logger, r relogio.Relogio) {
	RegistrarCom(mux, NovoServico(db, ClientePadrao(), r, log), log)
}

// RegistrarCom é o Registrar com o serviço pronto (os testes passam um com a PTAX do testeapoio.ServicoDeFora).
func RegistrarCom(mux *http.ServeMux, s *Servico, log *slog.Logger) {
	mux.Handle("GET /api/cambio/moedas", soLogado(func(w http.ResponseWriter, req *http.Request) {
		moedas, err := s.ListarMoedas(req.Context())
		responder(w, log, moedas, err)
	}))
	mux.Handle("GET /api/cambio/cotacoes", soLogado(func(w http.ResponseWriter, req *http.Request) {
		cotacoes, err := s.Ultimas(req.Context())
		responder(w, log, cotacoes, err)
	}))
	mux.Handle("GET /api/cambio/cotacao", soLogado(func(w http.ResponseWriter, req *http.Request) {
		cotacao(w, req, s, log)
	}))
}

// ClientePadrao é o cliente da PTAX de produção (HTTPS, 30 s).
func ClientePadrao() *ClientePTAX {
	c, err := NovoClientePTAX(URLPadraoPTAX, nil)
	if err != nil {
		panic(err) // a URL é uma constante: erro aqui é bug
	}
	return c
}

// cotacao é o GET /api/cambio/cotacao?moeda=USD&data=2026-10-15.
func cotacao(w http.ResponseWriter, req *http.Request, s *Servico, log *slog.Logger) {
	ctx := req.Context()
	// O model binding do ASP.NET roda antes da ação: data inválida é o 400 automático, antes de olhar a moeda.
	var data *tipos.Data
	if bruta, _ := respostas.ParametroDaQuery(req, "data"); bruta != "" {
		d, ok := respostas.DataDaQuery(bruta)
		if !ok {
			// O 400 automático do [ApiController] quando o texto não vira o tipo do parâmetro (DateOnly?).
			respostas.Validacao(w, map[string][]string{"data": {"The value '" + bruta + "' is not valid."}})
			return
		}
		data = &d
	}

	moeda, _ := respostas.ParametroDaQuery(req, "moeda")
	codigo, ok := NormalizarMoeda(moeda)
	if !ok {
		respostas.Mensagem(w, http.StatusBadRequest, "Informe a moeda com o código de 3 letras (ex.: USD, EUR).")
		return
	}
	if codigo != MoedaBase {
		moedas, err := s.ListarMoedas(ctx)
		if err != nil {
			erroInterno(w, log, ctx, err)
			return
		}
		if len(moedas) > 0 && !slices.ContainsFunc(moedas, func(m Moeda) bool { return m.Codigo == codigo }) {
			respostas.Mensagem(w, http.StatusBadRequest, "O Banco Central não publica cotação para "+codigo+".")
			return
		}
	}

	c, err := s.ObterCotacao(ctx, codigo, data)
	if err != nil {
		erroInterno(w, log, ctx, err)
		return
	}
	if c == nil {
		respostas.Mensagem(w, http.StatusServiceUnavailable,
			"Cotação de "+codigo+" indisponível no momento. Tente novamente mais tarde.")
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, c)
}

// soLogado confere o usuário que o .NET passou (ele já conferiu o login; chegar sem é requisição de fora).
func soLogado(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if _, ok := sessao.Usuario(req.Context()); !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		h(w, req)
	})
}

func responder(w http.ResponseWriter, log *slog.Logger, valor any, err error) {
	if err != nil {
		erroInterno(w, log, context.Background(), err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, valor)
}

// erroInterno registra o erro e responde o 500 de sempre. Pedido cancelado (o navegador desistiu) não é erro.
func erroInterno(w http.ResponseWriter, log *slog.Logger, ctx context.Context, err error) {
	if errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return
	}
	log.Error("câmbio", "origem", "Câmbio", "erro", err)
	servidor.ResponderErroInterno(w)
}
