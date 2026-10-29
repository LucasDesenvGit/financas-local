package main

import (
	"context"
	"errors"
	"html"
	"html/template"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/comando"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/copia"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/desenvolvimento"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/elevacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/servico"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// O que muda no Linux (ADR 0033): o serviço é do systemd do próprio usuário, então ligar, desligar, copiar o banco e
// reinstalar rodam no próprio gerenciador, sem senha.

const (
	linux        = true
	avisoAoMudar = "Aguarde alguns segundos."
	ondeVerOErro = "Em “Mais opções”, “Ver o registro do serviço” mostra o motivo."
	reinstalando = "Código atualizado. Reinstalando o site…"
	// esconderAoFechar: no Linux, a janela destruída deixa o WebKitWebProcess dela vivo (Wails v3.0.0-beta.34 com o
	// WebKitGTK 6.0: +58 MB a cada abrir e fechar, medido na #417). Escondida, a mesma janela volta, sem vazar.
	esconderAoFechar = true
	// esperaAoLigar: depois de ligar ou reiniciar, quanto esperar o site responder (EsperaAoLigar do C# no Linux).
	esperaAoLigar = 30 * time.Second
)

// idDoAplicativo é o id do GTK (GApplication) e o app_id da janela no Wayland.
const idDoAplicativo = "br.financaslocal.Gerenciador"

func opcoesDoSistema(o *application.Options) {
	o.Linux = application.LinuxOptions{
		DisableQuitOnLastWindowClosed: true,
		// No GTK 4, o ícone da janela vem do atalho com o nome do id (br.financaslocal.Gerenciador.desktop), que a
		// instalação cria (#277); o id também é o app_id da janela no Wayland. Sem ele, o Wails usa org.wails.<nome>.
		ApplicationID: idDoAplicativo,
	}
}

// prepararBandeja: no padrão StatusNotifierItem, o clique simples é o "ativar" (mostra a janela) e o botão direito
// abre o menu; o Wails não repassa o duplo clique. O nome do ícone (Title, e a dica no KDE) é o rótulo: sem ele, sai
// "Wails". A dica não acompanha a situação, porque o Wails v3.0.0-beta.34 não a implementa no Linux (setTooltip
// vazio); a situação fica na primeira linha do menu e na cor do ponto.
func prepararBandeja(b *application.SystemTray, abrir func()) {
	b.SetLabel("Finanças Local")
	b.OnClick(abrir)
}

// temBandeja: há quem mostre o ícone (o StatusNotifierWatcher do KDE, ou do GNOME com a extensão de AppIndicator).
func temBandeja() bool {
	saida, err := comando.Rodar(context.Background(), 3*time.Second, "busctl", "--user", "call", "org.freedesktop.DBus",
		"/org/freedesktop/DBus", "org.freedesktop.DBus", "NameHasOwner", "s", "org.kde.StatusNotifierWatcher")
	return err == nil && strings.TrimSpace(saida) == "b true"
}

// executarNoServico pelo systemctl --user, com a mensagem dele quando não dá certo.
func executarNoServico(acao string) (elevacao.Resultado, string) {
	if err := servico.Executar(acao); err != nil {
		return elevacao.Falhou, err.Error()
	}
	return elevacao.Feito, ""
}

// copiarBanco: o banco é do próprio usuário (a pasta dados do projeto).
func copiarBanco(caminhoBanco, destino string) elevacao.Resultado {
	if err := copia.Salvar(caminhoBanco, destino); err != nil {
		if !errors.Is(err, copia.ErrSemBanco) {
			// O motivo vai para o terminal; a janela diz o que fazer.
			log.Print(err)
		}
		return elevacao.Falhou
	}
	return elevacao.Feito
}

// reinstalar roda o instalar.sh do clone, como o atualizar.sh faz depois de trazer o código, com a saída ao vivo.
func reinstalar(raiz string, escrever func(string)) elevacao.Resultado {
	if desenvolvimento.Reinstalar(context.Background(), raiz, escrever) != 0 {
		return elevacao.Falhou
	}
	return elevacao.Feito
}

// textoDaReinstalacao: os textos do AtualizarPeloGitAsync do C# no Linux.
func textoDaReinstalacao(r elevacao.Resultado) string {
	if r == elevacao.Feito {
		return "Pronto: o site está na última versão do develop. O gerenciador continua na versão de antes."
	}
	return "A atualização falhou. Veja a caixa abaixo e, se o site não voltar, “Ver o registro do serviço”."
}

// mostrarNaPasta abre a pasta do arquivo no gerenciador de arquivos do sistema.
func mostrarNaPasta(arquivo string) error {
	return exec.Command("xdg-open", filepath.Dir(arquivo)).Start()
}

// comandoParaAtualizar: com a pasta do projeto (dois níveis acima da pasta da API), o comando pronto para copiar.
func comandoParaAtualizar(inst instalacao.Instalacao) string {
	if inst.PeloMsi || inst.PastaPrograma == "" {
		return ""
	}
	raiz := filepath.Clean(filepath.Join(inst.PastaPrograma, "..", ".."))
	if _, err := os.Stat(filepath.Join(raiz, "scripts", "geral", "3-publicar-atualizar.sh")); err != nil {
		return ""
	}
	return tela.ComandoParaAtualizar(raiz)
}

// lerSemLogin: "sim" com o linger ligado, "nao" sem, "" sem dar para saber.
func lerSemLogin() string {
	sobe, sabe := servico.SobeSemLogin()
	switch {
	case !sabe:
		return ""
	case sobe:
		return "sim"
	default:
		return "nao"
	}
}

func manterNoArSemLogin() error { return servico.ManterNoArSemLogin() }

// linhasDoRegistro: as últimas do journald, como a JanelaRegistro do C#.
const linhasDoRegistro = 200

// registroDoServico é o fim do registro do serviço, no diálogo das notas.
func registroDoServico() *tela.Notas {
	texto, err := servico.Registro(linhasDoRegistro)
	if err != nil {
		texto = "Não deu para ler o registro do serviço: " + err.Error()
	} else if strings.TrimSpace(texto) == "" || strings.HasPrefix(strings.TrimSpace(texto), "-- No entries --") {
		texto = "O registro do serviço está vazio: o site ainda não foi ligado desde que o computador iniciou."
	}
	return &tela.Notas{Titulo: "Registro do serviço (as últimas " + strconv.Itoa(linhasDoRegistro) + " linhas)",
		Novidades: template.HTML("<pre>" + html.EscapeString(texto) + "</pre>")}
}

// avisarNaBandeja: a notificação da área de trabalho pelo notify-send, como o Bandeja.cs; sem ele (libnotify), não
// avisa e segue. O ícone é o cofrinho, gravado no cache do usuário, porque o notify-send pede um arquivo; o de alerta
// e o de informação não mudam nada aqui, como no C#.
func avisarNaBandeja(a tela.AvisoNaBandeja) {
	argumentos := []string{"--app-name=Finanças Local"}
	if cache, err := os.UserCacheDir(); err == nil {
		arquivo := filepath.Join(cache, "financas-local", "icone.png")
		if os.MkdirAll(filepath.Dir(arquivo), 0o700) == nil && os.WriteFile(arquivo, icone, 0o600) == nil {
			argumentos = append(argumentos, "--icon="+arquivo)
		}
	}
	if _, err := comando.Rodar(context.Background(), 5*time.Second, "notify-send", append(argumentos, "Finanças Local", a.Texto)...); err != nil {
		log.Print(err)
	}
}
