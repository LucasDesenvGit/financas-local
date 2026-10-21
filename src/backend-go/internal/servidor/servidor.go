// Pacote servidor é o Program.cs do Go, menos os endpoints: HTTPS só local, cabeçalhos de segurança, filtro de
// hosts, tratamento de erro, limite de corpo, log e o front-end (#73). Toda rota /api ainda responde 404.
package servidor

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/config"
)

// Opcoes do servidor.
type Opcoes struct {
	Config      config.Config
	PastaSite   string // o wwwroot com o build do Angular
	Certificado tls.Certificate
	Log         *slog.Logger
}

// NovoHandler monta o pipeline, na ordem do Program.cs: filtro de hosts → cabeçalhos de segurança → erro → rotas.
func NovoHandler(o Opcoes) (http.Handler, error) {
	site, err := novosArquivosDoSite(o.PastaSite)
	if err != nil {
		return nil, fmt.Errorf("abrir a pasta do site %s: %w", o.PastaSite, err)
	}
	if _, err := fs.Stat(site.raiz, "index.html"); err != nil {
		return nil, errPastaSemIndex
	}

	// As rotas de cada assunto entram neste ServeMux (ADR 0049). O "/" é o MapFallback("/api/{**caminho}") do
	// .NET: 404 sem corpo.
	api := http.NewServeMux()
	api.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { semCorpo(w, http.StatusNotFound) })

	// O caminho é limpo aqui ("/a/../b" vira "/b"), como o Kestrel faz em silêncio: o ServeMux responderia com um
	// redirecionamento. Atenção para as próximas etapas: o ServeMux diferencia maiúsculas, e a rota do .NET não.
	rotas := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = limparCaminho(r.URL.Path)
		if ehAPI(r.URL.Path) {
			api.ServeHTTP(w, r)
			return
		}
		site.ServeHTTP(w, r)
	})

	var h http.Handler = rotas
	h = limitarCorpo(h)
	h = recuperarPanico(o.Log, h)
	h = comCabecalhosDeSeguranca(h)
	h = soHostsPermitidos(o.Config.HostsPermitidos, h)
	h = registrarRequisicoes(o.Log, h)
	return h, nil
}

// semCorpo responde só o status, com "Content-Length: 0" explícito, como o Kestrel.
func semCorpo(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(status)
}

// limparCaminho tira "." e ".." e barras repetidas, mas mantém a barra do fim ("/media/" continua pasta).
func limparCaminho(caminho string) string {
	limpo := path.Clean("/" + caminho)
	if strings.HasSuffix(caminho, "/") && limpo != "/" {
		limpo += "/"
	}
	return limpo
}

// Rodar escuta em HTTPS até o contexto acabar (sinal de parar) e então para com elegância: espera as requisições
// em andamento por até 30 s.
func Rodar(ctx context.Context, o Opcoes) error {
	if err := config.ConferirSoLocal(o.Config.Endereco); err != nil {
		return err
	}
	handler, err := NovoHandler(o)
	if err != nil {
		return err
	}
	servidor := &http.Server{
		Addr:    o.Config.Endereco,
		Handler: handler,
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{o.Certificado},
		},
		// Os prazos padrão do Kestrel: 30 s para chegarem os cabeçalhos (contra conexões que nunca terminam de
		// mandá-los) e 130 s de conexão parada.
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       130 * time.Second,
		ErrorLog:          slog.NewLogLogger(o.Log.Handler(), slog.LevelWarn),
	}

	erros := make(chan error, 1)
	go func() {
		o.Log.Info("escutando", "endereco", "https://"+o.Config.Endereco)
		erros <- servidor.ListenAndServeTLS("", "")
	}()

	select {
	case err := <-erros:
		return err
	case <-ctx.Done():
	}
	o.Log.Info("parando: esperando as requisições em andamento")
	prazo, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelar()
	if err := servidor.Shutdown(prazo); err != nil {
		return fmt.Errorf("parar o servidor: %w", err)
	}
	if err := <-erros; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
