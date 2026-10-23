// Comando financas é a API do Finanças Local em Go (épico #70, ADR 0049). Roda de dois jeitos:
//
//   - Atrás do .NET (-atras-do-dotnet, #231): é assim que roda no site. O .NET o inicia como processo filho,
//     manda o segredo interno pela entrada padrão e repassa a ele os prefixos de Go:Prefixos. Escuta em
//     127.0.0.1, numa porta livre, e escreve {"porta":N} na saída padrão quando já aceita conexões. Para quando a
//     entrada padrão fecha (o .NET saiu, mesmo à força) ou com SIGINT/SIGTERM.
//
//   - Sozinho (o esqueleto da #73): serve o Angular com os mesmos cabeçalhos, cache e erros do .NET, e toda rota
//     /api responde 404, porque ainda não há login no Go (#75). Em desenvolvimento, ao lado do .NET (5443):
//
//     go run ./cmd/financas -endereco 127.0.0.1:5444 -wwwroot ../frontend/dist/financas-web/browser -certificado-temporario
package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/api"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/config"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/versao"
)

func main() {
	if err := rodar(); err != nil {
		slog.Error("o Go parou com erro", "erro", err)
		os.Exit(1)
	}
}

func rodar() error {
	executavel, err := os.Executable()
	if err != nil {
		return err
	}
	var (
		pastaBase   = flag.String("pasta", filepath.Dir(executavel), "pasta dos appsettings*.json (a do executável, como no .NET)")
		endereco    = flag.String("endereco", "", "host:porta, no lugar do Kestrel:Endpoints:Https:Url (ex.: 127.0.0.1:5444)")
		wwwroot     = flag.String("wwwroot", "", "pasta do build do Angular (padrão: <pasta>/wwwroot)")
		temporario  = flag.Bool("certificado-temporario", false, "usar um certificado só em memória, sem o .pfx (desenvolvimento)")
		atras       = flag.Bool("atras-do-dotnet", false, "rodar como processo filho do .NET (#231): segredo pela entrada padrão, porta pela saída padrão")
		caminhoDB   = flag.String("banco", "", "caminho do banco, no lugar do Banco:Caminho (o .NET passa o dele)")
		aplicar     = flag.Bool("aplicar-esquema", false, "sozinho: criar e atualizar o banco antes de atender, como o .NET faz (até a #85 quem faz isso é o .NET)")
		ambienteEnv = os.Getenv("DOTNET_ENVIRONMENT")
	)
	flag.Parse()
	if ambienteEnv == "" {
		ambienteEnv = os.Getenv("ASPNETCORE_ENVIRONMENT")
	}

	// Atrás do .NET, o log sai em JSON na saída de erro: o .NET lê cada linha e a registra com a categoria
	// Financas.Go (os avisos e erros vão para Configurações › Erros, ADR 0022).
	var log *slog.Logger
	if *atras {
		log = slog.New(slog.NewJSONHandler(os.Stderr, nil))
	} else {
		log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	slog.SetDefault(log)

	cfg, err := config.Carregar(*pastaBase, ambienteEnv)
	if err != nil {
		return err
	}
	if *caminhoDB != "" {
		cfg.BancoCaminho = *caminhoDB
	}

	// SIGINT (Ctrl+C) e SIGTERM (systemd) param com elegância. O serviço do Windows entra na #84.
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	if *atras {
		return rodarAtrasDoDotnet(ctx, cfg, log, os.Stdin, os.Stdout)
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

	// Desligado por padrão: até a inversão (#85, ADR 0049), o .NET aplica o esquema ao iniciar. Os dois usam a mesma
	// assinatura (PRAGMA user_version), então ligar aqui também não repete a cópia nem a atualização. Atrás do .NET
	// (acima) a opção não vale: quem aplica é sempre o .NET, antes de iniciar o Go.
	if *aplicar {
		if err := banco.AplicarEsquema(ctx, cfg.BancoCaminho, relogio.Sistema{}, log); err != nil {
			return err
		}
	}
	return servidor.Rodar(ctx, servidor.Opcoes{Config: cfg, PastaSite: pastaSite, Certificado: certificado, Log: log})
}

// rodarAtrasDoDotnet lê o segredo da primeira linha da entrada, abre o banco (que o .NET já criou e passou para
// WAL) e atende até a entrada fechar ou o contexto acabar.
func rodarAtrasDoDotnet(ctx context.Context, cfg config.Config, log *slog.Logger, entrada io.Reader, saida io.Writer) error {
	leitor := bufio.NewReader(entrada)
	linha, err := leitor.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("ler o segredo interno da entrada padrão: %w", err)
	}
	segredo := strings.TrimSpace(linha)
	if len(segredo) < servidor.TamanhoMinimoDoSegredo {
		return errors.New("a entrada padrão não trouxe o segredo interno: o Go atrás do .NET só roda iniciado por ele")
	}

	// A entrada fica aberta enquanto o .NET estiver vivo. Quando ela fecha (o .NET parou, ou caiu e o sistema
	// fechou o lado dele), o Go para junto: não sobra processo órfão, nem se o .NET for morto à força.
	ctx, pararPorFimDaEntrada := context.WithCancel(ctx)
	defer pararPorFimDaEntrada()
	go func() {
		_, _ = io.Copy(io.Discard, leitor)
		log.Info("a entrada padrão fechou (o .NET saiu): parando")
		pararPorFimDaEntrada()
	}()

	// O Go não cria nem muda o esquema (ADR 0049): o banco tem de existir, criado pelo EsquemaBanco do .NET.
	if _, err := os.Stat(cfg.BancoCaminho); err != nil {
		return fmt.Errorf("o banco %s não existe (o .NET cria o banco antes de iniciar o Go): %w", cfg.BancoCaminho, err)
	}
	db, err := banco.Abrir(ctx, cfg.BancoCaminho)
	if err != nil {
		return err
	}
	defer db.Close()
	if diario, err := banco.ModoDoDiario(ctx, db); err != nil {
		return err
	} else if diario != "wal" {
		log.Warn("o banco não está em WAL: com o .NET e o Go no mesmo arquivo, quem lê pode esperar quem grava", "journal_mode", diario)
	}

	log.Info("iniciando atrás do .NET", "versao", versao.Atual)
	return servidor.RodarAtrasDoDotnet(ctx, servidor.OpcoesAtras{
		Segredo: segredo,
		Rotas:   api.NovasRotas(api.Dependencias{DB: db, Log: log}),
		Log:     log,
		AvisarPorta: func(porta int) error {
			return json.NewEncoder(saida).Encode(map[string]int{"porta": porta})
		},
	})
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
