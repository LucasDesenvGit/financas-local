package servidor

// Modo "atrás do .NET" (#231, ADR 0049): o .NET fica na frente, com o login, o CSRF e os segredos, e repassa ao Go
// os prefixos de rota da lista Go:Prefixos. O Go é um processo filho do .NET e escuta só em 127.0.0.1, numa porta
// escolhida pelo sistema na partida, em HTTP: a conversa não sai do computador, e quem a autentica é o segredo
// interno, e não o TLS.

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/sessao"
)

const (
	// CabecalhoSegredo leva o segredo interno, gerado pelo .NET a cada partida do Go.
	CabecalhoSegredo = "X-Financas-Segredo"
	// CabecalhoUsuario leva o Id de quem está logado. O .NET apaga esse cabeçalho quando ele vem de fora.
	CabecalhoUsuario = "X-Financas-Usuario"
	// TamanhoMinimoDoSegredo, em caracteres: o .NET manda 32 bytes aleatórios em hexadecimal (64).
	TamanhoMinimoDoSegredo = 32
	// esperaAoParar é quanto o Go espera as requisições em andamento ao parar. Fica abaixo dos 10 s que o .NET
	// espera antes de encerrar o processo à força.
	esperaAoParar = 8 * time.Second
)

// OpcoesAtras configura o modo atrás do .NET.
type OpcoesAtras struct {
	Segredo string       // o segredo interno, lido da entrada padrão
	Rotas   http.Handler // as rotas dos assuntos (o ServeMux do pacote api)
	Log     *slog.Logger
	// AvisarPorta é chamada com a porta escolhida, quando o Go já aceita conexões: o .NET lê essa linha.
	AvisarPorta func(porta int) error
}

// NovoHandlerAtras monta o pipeline do modo atrás do .NET: erro → segredo e usuário → limite de corpo → rotas.
// Fora de /api, 404: o front-end continua sendo servido pelo .NET.
func NovoHandlerAtras(segredo string, rotas http.Handler, log *slog.Logger) (http.Handler, error) {
	if len(segredo) < TamanhoMinimoDoSegredo {
		return nil, fmt.Errorf("o segredo interno tem de ter pelo menos %d caracteres", TamanhoMinimoDoSegredo)
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = limparCaminho(r.URL.Path)
		if !ehAPI(r.URL.Path) {
			semCorpo(w, http.StatusNotFound)
			return
		}
		rotas.ServeHTTP(w, r)
	})
	return recuperarPanico(log, exigirSegredo([]byte(segredo), log, limitarCorpo(h))), nil
}

// exigirSegredo só deixa passar quem mandou o segredo interno e o usuário logado; o resto recebe 401 sem corpo.
// Assim, outro programa do computador não fala direto com o Go: só o .NET sabe o segredo. A recusa vira um aviso em
// Configurações › Erros (origem "API"), com a mensagem sempre igual, para repetições somarem na mesma linha.
func exigirSegredo(segredo []byte, log *slog.Logger, proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recebido := []byte(r.Header.Get(CabecalhoSegredo))
		// ConstantTimeCompare leva o mesmo tempo seja onde for a primeira diferença: medir o tempo da resposta não
		// ensina o segredo um caractere de cada vez. (Ele devolve 0 na hora se os tamanhos diferem, e o tamanho
		// não é segredo.)
		if subtle.ConstantTimeCompare(recebido, segredo) != 1 {
			log.Warn("requisição recusada: sem o segredo interno (outro programa tentou falar direto com o Go)", "origem", "API")
			semCorpo(w, http.StatusUnauthorized)
			return
		}
		id, err := strconv.ParseInt(r.Header.Get(CabecalhoUsuario), 10, 64)
		if err != nil || id <= 0 {
			semCorpo(w, http.StatusUnauthorized)
			return
		}
		// Daqui para a frente ninguém precisa dos cabeçalhos internos: o segredo não chega aos handlers nem aos logs.
		r.Header.Del(CabecalhoSegredo)
		r.Header.Del(CabecalhoUsuario)
		proximo.ServeHTTP(w, r.WithContext(sessao.ComUsuario(r.Context(), id)))
	})
}

// RodarAtrasDoDotnet escuta em 127.0.0.1, numa porta livre escolhida pelo sistema, avisa a porta e atende até o
// contexto acabar (o .NET fechou a entrada padrão, ou chegou um sinal de parar). Então para com elegância.
func RodarAtrasDoDotnet(ctx context.Context, o OpcoesAtras) error {
	handler, err := NovoHandlerAtras(o.Segredo, o.Rotas, o.Log)
	if err != nil {
		return err
	}
	// Porta 0: o sistema escolhe uma livre. Só 127.0.0.1, nunca outra interface.
	ouvinte, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("escutar em 127.0.0.1: %w", err)
	}
	servidor := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       130 * time.Second,
		ErrorLog:          slog.NewLogLogger(o.Log.Handler(), slog.LevelWarn),
	}

	erros := make(chan error, 1)
	go func() { erros <- servidor.Serve(ouvinte) }()

	porta := ouvinte.Addr().(*net.TCPAddr).Port
	if err := o.AvisarPorta(porta); err != nil {
		_ = servidor.Close()
		return fmt.Errorf("avisar a porta ao .NET: %w", err)
	}
	o.Log.Info("atendendo o .NET", "endereco", "http://"+ouvinte.Addr().String())

	select {
	case err := <-erros:
		return err
	case <-ctx.Done():
	}
	o.Log.Info("parando: esperando as requisições em andamento")
	prazo, cancelar := context.WithTimeout(context.Background(), esperaAoParar)
	defer cancelar()
	if err := servidor.Shutdown(prazo); err != nil {
		return fmt.Errorf("parar o servidor: %w", err)
	}
	if err := <-erros; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
