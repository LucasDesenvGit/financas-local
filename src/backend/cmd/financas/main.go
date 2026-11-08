// Comando financas é o site do Finanças Local (épico #70, ADR 0049): serve o Angular com os mesmos cabeçalhos,
// cache e erros do .NET de antes (#73) e atende /api com o login do Go (#75): sessão, CSRF e limite de tentativas
// na frente de todas as rotas. Desde a #85 é o site, e desde a #86 o .NET saiu. Instalado, roda como serviço do
// sistema (#84: Serviço do Windows, ou systemd no Linux), com -aplicar-esquema -rotinas-de-partida. Em
// desenvolvimento:
//
//	go run ./cmd/financas -endereco 127.0.0.1:5444 -wwwroot ../frontend/dist/financas-web/browser -certificado-temporario
//
// O subcomando gerar-certificado (#84) cuida do .pfx da instalação: reaproveita o que ainda vale e gera outro quando precisa.
//
// Só para teste: -inicio-do-relogio 2026-10-15T01:00:00Z faz o relógio começar nesse instante e andar dali (#363);
// o contrato usa. Sem a opção, o relógio do sistema.
package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/ajustes/parametros"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/api"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/openfinance/sincronizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/partida"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/cripto"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/auditoria"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/autenticacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/celular"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/config"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/erros"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servicosistema"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tarefas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/versao"
)

func main() {
	var err error
	if len(os.Args) > 1 && os.Args[1] == "gerar-certificado" {
		err = gerarCertificado(os.Args[2:], os.Stdin, os.Stdout, relogio.Sistema{})
	} else {
		err = servicosistema.Rodar(rodar)
	}
	if err != nil {
		slog.Error("o Go parou com erro", "erro", err)
		os.Exit(1)
	}
}

// gerarCertificado é o subcomando que os scripts chamam. A senha vem pela entrada padrão, e não na linha de comando, onde
// qualquer programa do computador a veria. Escreve "gerado" ou "reaproveitado" na saída.
func gerarCertificado(argumentos []string, entrada io.Reader, saida io.Writer, r relogio.Relogio) error {
	opcoes := flag.NewFlagSet("gerar-certificado", flag.ContinueOnError)
	pfx := opcoes.String("pfx", "", "caminho do .pfx; a parte pública vai para o .pem ao lado")
	if err := opcoes.Parse(argumentos); err != nil {
		return err
	}
	if *pfx == "" {
		return errors.New("gerar-certificado: informe -pfx")
	}
	linha, err := bufio.NewReader(entrada).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("gerar-certificado: ler a senha: %w", err)
	}
	// O pipe do PowerShell 5.1 pode começar pela marca do UTF-8 e termina em \r\n: nada disso é da senha.
	senha := strings.TrimRight(strings.TrimPrefix(linha, "\ufeff"), "\r\n")
	if senha == "" {
		return errors.New("gerar-certificado: a senha vem pela entrada padrão")
	}
	gerou, err := servidor.GarantirCertificado(*pfx, senha, r.Agora())
	if err != nil {
		return err
	}
	resultado := "reaproveitado"
	if gerou {
		resultado = "gerado"
	}
	_, err = fmt.Fprintln(saida, resultado)
	return err
}

// rodar é o programa: acaba quando o ctx acaba (Ctrl+C, systemd ou o gerenciador de serviços do Windows).
func rodar(ctx context.Context) error {
	executavel, err := os.Executable()
	if err != nil {
		return err
	}
	var (
		pastaBase   = flag.String("pasta", filepath.Dir(executavel), "pasta dos appsettings*.json (a do executável, como no .NET)")
		endereco    = flag.String("endereco", "", "host:porta, no lugar do Kestrel:Endpoints:Https:Url (ex.: 127.0.0.1:5444)")
		wwwroot     = flag.String("wwwroot", "", "pasta do build do Angular (padrão: <pasta>/wwwroot)")
		temporario  = flag.Bool("certificado-temporario", false, "usar um certificado só em memória, sem o .pfx (desenvolvimento)")
		caminhoDB   = flag.String("banco", "", "caminho do banco, no lugar do Banco:Caminho")
		aplicar     = flag.Bool("aplicar-esquema", false, "criar e atualizar o banco antes de atender (a linha do serviço instalado sempre liga)")
		partidaSo   = flag.Bool("rotinas-de-partida", false, "rodar as seis rotinas que acertam dados ao subir (#444), depois do esquema; use junto de -aplicar-esquema")
		pastaChaves = flag.String("chaves", "", "pasta da chave da sessão, no lugar do Seguranca:PastaChaves (padrão: dados/chaves, ao lado do banco)")
		inicioRel   = flag.String("inicio-do-relogio", "", "só para teste (#363): o relógio começa neste instante e anda dali (RFC 3339 com fuso, ex.: 2026-10-15T01:00:00Z); vazio é o relógio do sistema")
		ambienteEnv = os.Getenv("DOTNET_ENVIRONMENT")
	)
	flag.Parse()
	if ambienteEnv == "" {
		ambienteEnv = os.Getenv("ASPNETCORE_ENVIRONMENT")
	}

	rel, inicioDeTeste, err := novoRelogio(*inicioRel)
	if err != nil {
		return err
	}
	// #298: quem põe os avisos e erros na tabela (Configurações › Erros) é o próprio Go: o handler do pacote erros os
	// põe na fila, e o gravador os grava.
	// #531: o log do serviço (journald, Log de Eventos) passa pela mesma limpeza da tabela antes de sair do processo.
	texto := erros.ComLimpeza(servicosistema.ComLogDoSistema(slog.NewTextHandler(os.Stderr, nil)))
	filaDeErros := erros.NovaFila()
	log, logSemTabela := slog.New(erros.NovoHandler(texto, filaDeErros, rel)), slog.New(texto)
	slog.SetDefault(log)
	avisarRelogioDeTeste(log, inicioDeTeste)

	cfg, err := config.Carregar(*pastaBase, ambienteEnv)
	if err != nil {
		return err
	}
	if *caminhoDB != "" {
		cfg.BancoCaminho = *caminhoDB
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

	// Desligado por padrão (o contrato e o desenvolvimento usam um banco pronto); a linha do serviço instalado liga.
	// A assinatura (PRAGMA user_version) é a mesma do EsquemaBanco do .NET de antes: não repete a cópia nem a atualização.
	if *aplicar {
		if err := banco.AplicarEsquema(ctx, cfg.BancoCaminho, rel, log); err != nil {
			return err
		}
	}
	// #75: o login é do Go. O banco tem de existir (o -aplicar-esquema acima o cria).
	if _, err := os.Stat(cfg.BancoCaminho); err != nil {
		return fmt.Errorf("o banco %s não existe (crie-o com -aplicar-esquema): %w", cfg.BancoCaminho, err)
	}
	db, err := banco.Abrir(ctx, cfg.BancoCaminho)
	if err != nil {
		return err
	}
	defer db.Close()
	// #444: as rotinas de partida (regras padrão novas, entradas nos cartões, transferências...). Desligadas por padrão;
	// a linha do serviço instalado liga. Erro numa rotina só é registrado, como no .NET de antes.
	if *partidaSo {
		partida.Executar(ctx, partida.Opcoes{DB: db, Relogio: rel, Log: log})
	}
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
	montado := api.Montar(api.Dependencias{DB: db, Log: log, Relogio: rel, Config: cfg})
	rotas := montado.Rotas
	auth.Registrar(rotas)
	// #85: o Go roda todas as tarefas em segundo plano (câmbio, cripto e a sincronização). Param antes de o banco
	// fechar (o defer do db.Close vem antes).
	defer iniciarTarefas(ctx, tarefas.Todas(), db, montado, rel, log)()
	// #298: os erros do sistema. O gravador para depois do servidor e grava o que sobrou na fila antes de o banco
	// fechar (o defer acima).
	erros.Registrar(rotas, erros.NovoServico(db, filaDeErros, rel, log))
	parametrosSite := parametrosDoSite(db, rel, log, cfg)
	gravador := erros.NovoGravador(db, filaDeErros, rel, parametrosSite, logSemTabela)
	ctxGravador, pararGravador := context.WithCancel(context.Background())
	gravadorParou := make(chan struct{})
	go func() {
		defer close(gravadorParou)
		gravador.Rodar(ctxGravador)
	}()
	defer func() {
		pararGravador()
		<-gravadorParou
	}()
	// #530: a auditoria de acesso tem prazo; a limpeza roda ao subir e uma vez por dia, e para antes de o banco fechar.
	ctxLimpeza, pararLimpeza := context.WithCancel(ctx)
	limpezaParou := make(chan struct{})
	go func() {
		defer close(limpezaParou)
		auditoria.NovaLimpeza(db, parametrosSite, rel, log).Rodar(ctxLimpeza)
	}()
	defer func() {
		pararLimpeza()
		<-limpezaParou
	}()
	site := servidor.Opcoes{Config: cfg, PastaSite: pastaSite, Certificado: certificado, Log: log, API: auth.Proteger(rotas)}
	// #328 (ADR 0061): o acesso pelo celular, desligado por padrão. Ligado, escuta também no IP da rede de casa, com o
	// mesmo pipeline do site, só com esse IP nos hosts permitidos e o certificado da autoridade local.
	acessoCelular, err := novoAcessoCelular(db, rel, log, site)
	if err != nil {
		return err
	}
	celular.Registrar(rotas, acessoCelular)
	acessoCelular.Iniciar(ctx)
	defer func() {
		prazo, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelar()
		acessoCelular.Parar(prazo)
	}()
	log.Info("iniciando", "versao", versao.Atual)
	return servidor.Rodar(ctx, site)
}

// novoAcessoCelular monta o acesso pelo celular na porta do site.
func novoAcessoCelular(db *sql.DB, rel relogio.Relogio, log *slog.Logger, site servidor.Opcoes) (*celular.Servico, error) {
	_, textoPorta, err := net.SplitHostPort(site.Config.Endereco)
	if err != nil {
		return nil, fmt.Errorf("o endereço do site %q: %w", site.Config.Endereco, err)
	}
	porta, err := strconv.Atoi(textoPorta)
	if err != nil {
		return nil, fmt.Errorf("a porta do site %q: %w", site.Config.Endereco, err)
	}
	return celular.Novo(celular.Opcoes{
		DB: db, Relogio: rel, Log: log, PastaChaves: site.Config.PastaChaves, Porta: porta, Firewall: celular.FirewallDoSistema(),
		NovoServidor: func(ip string, c tls.Certificate) (*http.Server, error) {
			daRede := site
			daRede.Config.HostsPermitidos = []string{ip}
			daRede.Certificado = c
			return servidor.NovoServidor(daRede)
		},
	}), nil
}

// novoRelogio devolve o relógio do sistema, ou, com -inicio-do-relogio, um que começa no instante dado e anda dali
// (relogio.APartirDe). É só para teste. O relógio vem antes do log, porque o handler que põe o aviso na tabela de
// erros usa o relógio (a hora do aviso é a do relógio de teste).
func novoRelogio(inicio string) (relogio.Relogio, *time.Time, error) {
	if inicio == "" {
		return relogio.Sistema{}, nil, nil
	}
	instante, err := relogio.LerInicio(inicio)
	if err != nil {
		return nil, nil, err
	}
	return relogio.NovoAPartirDe(instante), &instante, nil
}

func avisarRelogioDeTeste(log *slog.Logger, instante *time.Time) {
	if instante == nil {
		return
	}
	log.Warn("relógio de teste: a hora começa no instante dado, e não na do computador (-inicio-do-relogio)",
		"inicio", instante.Format(time.RFC3339Nano), "origem", "Sistema")
}

// parametrosDoSite é o ParametroService que o gravador e a limpeza da auditoria usam para os prazos (os padrões do
// appsettings, como no api.NovasRotas).
func parametrosDoSite(db *sql.DB, rel relogio.Relogio, log *slog.Logger, cfg config.Config) *parametros.Servico {
	return parametros.Novo(parametros.Opcoes{DB: db, Relogio: rel, Log: log, Versao: versao.Atual,
		CaminhoBanco: cfg.BancoCaminho, PastaChaves: cfg.PastaChaves, Padroes: parametros.PadroesDoAppsettings(cfg)})
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

// iniciarTarefas põe para rodar as tarefas em segundo plano ligadas (no site, todas: tarefas.Todas, #85). A
// sincronização usa a instância das rotas (montado), para as duas dividirem a trava de uma rodada por vez. Devolve
// quem para: cancela e espera cada uma terminar, antes de o banco fechar.
func iniciarTarefas(ctx context.Context, ligadas map[string]bool, db *sql.DB, montado api.Montado, rel relogio.Relogio, log *slog.Logger) func() {
	ctxTarefas, pararTarefas := context.WithCancel(ctx)
	var emAndamento sync.WaitGroup
	if ligadas["Cambio"] {
		t := cambio.NovaTarefa(cambio.NovoServico(db, cambio.ClientePadrao(), rel, log), rel, log)
		emAndamento.Go(func() { t.Rodar(ctxTarefas) })
	}
	if ligadas["Cripto"] {
		t := cripto.NovaTarefa(cripto.NovoServico(db, cripto.ClientePadrao(), rel, log), log)
		emAndamento.Go(func() { t.Rodar(ctxTarefas) })
	}
	if ligadas["Sincronizacao"] {
		t := sincronizacao.NovaTarefa(montado.Sincronizacao, log)
		emAndamento.Go(func() { t.Rodar(ctxTarefas) })
	}
	return func() {
		pararTarefas()
		emAndamento.Wait()
	}
}
