// Comando financas é a API do Finanças Local em Go (épico #70, ADR 0049). Roda de dois jeitos:
//
//   - Atrás do .NET (-atras-do-dotnet, #231): é assim que roda no site. O .NET o inicia como processo filho,
//     manda o segredo interno pela entrada padrão e repassa a ele os prefixos de Go:Prefixos. Escuta em
//     127.0.0.1, numa porta livre, e escreve {"porta":N} na saída padrão quando já aceita conexões. Para quando a
//     entrada padrão fecha (o .NET saiu, mesmo à força) ou com SIGINT/SIGTERM.
//
//   - Sozinho: serve o Angular com os mesmos cabeçalhos, cache e erros do .NET (#73) e atende /api com o login do
//     próprio Go (#75): sessão, CSRF e limite de tentativas na frente de todas as rotas. É assim que o site vai rodar
//     depois da inversão (#85); até lá, serve para desenvolver e para o contrato. Em desenvolvimento, ao lado do
//     .NET (5443):
//
//     go run ./cmd/financas -endereco 127.0.0.1:5444 -wwwroot ../frontend/dist/financas-web/browser -certificado-temporario
//
// Só para teste, nos dois jeitos: -inicio-do-relogio 2026-10-15T01:00:00Z faz o relógio começar nesse instante e
// andar dali (#363), como a libfaketime faz com o .NET no contrato. Atrás do .NET, quem passa a opção é ele, quando
// Go:InicioDoRelogio está configurado. Sem a opção, o relógio do sistema.
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
	"sync"
	"syscall"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/api"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/autenticacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/config"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cripto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tarefas"
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
		pastaChaves = flag.String("chaves", "", "sozinho: pasta da chave da sessão, no lugar do Seguranca:PastaChaves (padrão: dados/chaves, ao lado do banco)")
		inicioRel   = flag.String("inicio-do-relogio", "", "só para teste (#363): o relógio começa neste instante e anda dali (RFC 3339 com fuso, ex.: 2026-10-15T01:00:00Z); vazio é o relógio do sistema")
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
	rel, err := escolherRelogio(*inicioRel, log)
	if err != nil {
		return err
	}

	// SIGINT (Ctrl+C) e SIGTERM (systemd) param com elegância. O serviço do Windows entra na #84.
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	if *atras {
		return rodarAtrasDoDotnet(ctx, cfg, rel, log, os.Stdin, os.Stdout)
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
		if err := banco.AplicarEsquema(ctx, cfg.BancoCaminho, rel, log); err != nil {
			return err
		}
	}
	// #75: o Go sozinho faz o próprio login. O banco tem de existir (quem o cria, até a #85, é o .NET, ou o
	// -aplicar-esquema acima).
	if _, err := os.Stat(cfg.BancoCaminho); err != nil {
		return fmt.Errorf("o banco %s não existe (crie-o com -aplicar-esquema ou pelo .NET): %w", cfg.BancoCaminho, err)
	}
	db, err := banco.Abrir(ctx, cfg.BancoCaminho)
	if err != nil {
		return err
	}
	defer db.Close()
	if *pastaChaves != "" {
		cfg.PastaChaves = *pastaChaves
	} else if _, err := os.Stat(cfg.PastaChaves); err != nil {
		// Sem a pasta do appsettings (ex.: o contrato, só com -banco): a dados/chaves ao lado da dados/sql do banco.
		cfg.PastaChaves = filepath.Join(filepath.Dir(filepath.Dir(cfg.BancoCaminho)), "chaves")
	}
	chave, err := autenticacao.CarregarChave(cfg.PastaChaves)
	if err != nil {
		return err
	}
	auth := autenticacao.Nova(autenticacao.Opcoes{
		DB: db, Relogio: rel, Log: log, Chave: chave, Versao: versao.Atual, MinutosSessao: cfg.MinutosSessao,
		MaximoTentativasLogin: cfg.MaximoTentativasLogin, MinutosBloqueio: cfg.MinutosBloqueio,
	})
	rotas := api.NovasRotas(api.Dependencias{DB: db, Log: log, Relogio: rel, Config: cfg})
	auth.Registrar(rotas)
	log.Info("iniciando sozinho, com o login do Go", "versao", versao.Atual)
	return servidor.Rodar(ctx, servidor.Opcoes{
		Config: cfg, PastaSite: pastaSite, Certificado: certificado, Log: log, API: auth.Proteger(rotas),
	})
}

// rodarAtrasDoDotnet lê o segredo da primeira linha da entrada, abre o banco (que o .NET já criou e passou para
// WAL) e atende até a entrada fechar ou o contexto acabar.
func rodarAtrasDoDotnet(ctx context.Context, cfg config.Config, rel relogio.Relogio, log *slog.Logger, entrada io.Reader, saida io.Writer) error {
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

	// Tarefas em segundo plano (#81): só as de Go:Tarefas, que o .NET deixa de rodar (ADR 0049). Vazio, nenhuma: o Go
	// em desenvolvimento nunca vai à internet sozinho. Quem para é o ctx; o defer espera cada uma terminar antes de
	// o banco fechar.
	ligadas, err := tarefas.Ligadas(cfg.GoTarefas)
	if err != nil {
		return err
	}
	ctxTarefas, pararTarefas := context.WithCancel(ctx)
	var emAndamento sync.WaitGroup
	defer func() {
		pararTarefas()
		emAndamento.Wait()
	}()
	if ligadas["Cambio"] {
		t := cambio.NovaTarefa(cambio.NovoServico(db, cambio.ClientePadrao(), rel, log), rel, log)
		emAndamento.Go(func() { t.Rodar(ctxTarefas) })
	}
	if ligadas["Cripto"] {
		t := cripto.NovaTarefa(cripto.NovoServico(db, cripto.ClientePadrao(), rel, log), log)
		emAndamento.Go(func() { t.Rodar(ctxTarefas) })
	}
	if ligadas["Sincronizacao"] {
		// A sincronização com a Pluggy ainda não está em Go (#83): ligada aqui, ninguém a rodaria.
		return errors.New("Go:Tarefas tem Sincronizacao, que ainda não está em Go (#83)")
	}

	log.Info("iniciando atrás do .NET", "versao", versao.Atual, "tarefas", cfg.GoTarefas)
	return servidor.RodarAtrasDoDotnet(ctx, servidor.OpcoesAtras{
		Segredo: segredo,
		Rotas:   api.NovasRotas(api.Dependencias{DB: db, Log: log, Relogio: rel, Config: cfg}),
		Log:     log,
		AvisarPorta: func(porta int) error {
			return json.NewEncoder(saida).Encode(map[string]int{"porta": porta})
		},
	})
}

// escolherRelogio devolve o relógio do sistema, ou, com -inicio-do-relogio, um que começa no instante dado e anda
// dali (relogio.APartirDe). É só para teste: o contrato (#72) faz o mesmo com o .NET pela libfaketime, que não age
// sobre o Go. Ligado, fica um aviso no log (vai para Configurações › Erros quando o Go roda atrás do .NET).
func escolherRelogio(inicio string, log *slog.Logger) (relogio.Relogio, error) {
	if inicio == "" {
		return relogio.Sistema{}, nil
	}
	instante, err := relogio.LerInicio(inicio)
	if err != nil {
		return nil, err
	}
	log.Warn("relógio de teste: a hora começa no instante dado, e não na do computador (-inicio-do-relogio)",
		"inicio", instante.Format(time.RFC3339Nano), "origem", "Sistema")
	return relogio.NovoAPartirDe(instante), nil
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
