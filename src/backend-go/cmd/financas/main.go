// Comando financas é a API do Finanças Local em Go (épico #70, ADR 0049). Por enquanto é o esqueleto (#73): serve
// o Angular com os mesmos cabeçalhos, cache e erros do .NET, e toda rota /api responde 404.
//
// Em desenvolvimento, ao lado do .NET (que fica na 5443):
//
//	go run ./cmd/financas -endereco 127.0.0.1:5444 -wwwroot ../frontend/dist/financas-web/browser -certificado-temporario
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/config"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := rodar(log); err != nil {
		log.Error("o servidor parou com erro", "erro", err)
		os.Exit(1)
	}
}

func rodar(log *slog.Logger) error {
	executavel, err := os.Executable()
	if err != nil {
		return err
	}
	var (
		pastaBase   = flag.String("pasta", filepath.Dir(executavel), "pasta dos appsettings*.json (a do executável, como no .NET)")
		endereco    = flag.String("endereco", "", "host:porta, no lugar do Kestrel:Endpoints:Https:Url (ex.: 127.0.0.1:5444)")
		wwwroot     = flag.String("wwwroot", "", "pasta do build do Angular (padrão: <pasta>/wwwroot)")
		temporario  = flag.Bool("certificado-temporario", false, "usar um certificado só em memória, sem o .pfx (desenvolvimento)")
		ambienteEnv = os.Getenv("DOTNET_ENVIRONMENT")
	)
	flag.Parse()
	if ambienteEnv == "" {
		ambienteEnv = os.Getenv("ASPNETCORE_ENVIRONMENT")
	}

	cfg, err := config.Carregar(*pastaBase, ambienteEnv)
	if err != nil {
		return err
	}
	if *endereco != "" {
		cfg.Endereco = *endereco
	}
	pastaSite := *wwwroot
	if pastaSite == "" {
		pastaSite = filepath.Join(cfg.PastaBase, "wwwroot")
	}

	certificado, err := carregarCertificado(cfg, *temporario)
	if err != nil {
		return err
	}

	// SIGINT (Ctrl+C) e SIGTERM (systemd) param com elegância. O serviço do Windows entra na #84.
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()
	return servidor.Rodar(ctx, servidor.Opcoes{Config: cfg, PastaSite: pastaSite, Certificado: certificado, Log: log})
}

func carregarCertificado(cfg config.Config, temporario bool) (tls.Certificate, error) {
	if temporario {
		return servidor.CertificadoTemporario()
	}
	if cfg.CertificadoCaminho == "" {
		return tls.Certificate{}, fmt.Errorf("sem certificado: configure Kestrel:Endpoints:Https:Certificate no appsettings ou use -certificado-temporario")
	}
	return servidor.CarregarPFX(cfg.CertificadoCaminho, cfg.CertificadoSenha)
}
