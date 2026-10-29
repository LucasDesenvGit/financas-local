package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/desenvolvimento"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/elevacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// O que muda no Windows: ligar, desligar, copiar o banco e reinstalar pedem administrador, e o gerenciador se reabre
// elevado (UAC) só para a ação.

const (
	linux = false
	// avisoAoMudar aparece enquanto liga, desliga ou reinicia.
	avisoAoMudar = "O Windows vai pedir permissão de administrador."
	// ondeVerOErro completa o "Não deu certo" de ligar, desligar e reiniciar.
	ondeVerOErro = "Veja os eventos do Windows (Visualizador de Eventos › Aplicativo, origem FinancasLocal)."
	reinstalando = "Código atualizado. Reinstalando o site (permita no aviso do Windows)…"
	// esconderAoFechar: no Windows, fechar destrói a janela e os processos do WebView2 (parado na bandeja, ~50 MB).
	esconderAoFechar = false
	// esperaAoLigar: o gerenciador em C# do Windows não esperava o site responder depois de ligar.
	esperaAoLigar = 0
)

func opcoesDoSistema(o *application.Options) {
	o.Windows = application.WindowsOptions{
		// Fechar a janela não encerra quando o gerenciador fica na bandeja (fechou, em gerenciador.go).
		DisableQuitOnLastWindowClosed: true,
		// O cache do WebView2 na pasta do Finanças Local do usuário, e não em AppData\Roaming\<exe>.
		WebviewUserDataPath: filepath.Join(os.Getenv("LOCALAPPDATA"), "FinancasLocal", "Gerenciador", "WebView2"),
	}
}

// prepararBandeja: duplo clique mostra a janela; o clique simples abre o menu.
func prepararBandeja(b *application.SystemTray, abrir func()) { b.OnDoubleClick(abrir) }

// executarNoServico pelo UAC; a mensagem do erro fica nos eventos do Windows.
func executarNoServico(acao string) (elevacao.Resultado, string) {
	return elevacao.Executar("--servico", acao), ""
}

// copiarBanco pelo UAC: o banco fica na pasta de dados do serviço.
func copiarBanco(_, destino string) elevacao.Resultado { return elevacao.Executar("--backup", destino) }

// reinstalar roda o instalar-windows.ps1 como administrador, num passo à parte, e acompanha o log dele.
func reinstalar(raiz string, escrever func(string)) elevacao.Resultado {
	ctx, parar := context.WithCancel(context.Background())
	acompanhado := make(chan struct{})
	go func() {
		desenvolvimento.AcompanharLog(ctx, desenvolvimento.Log(raiz), escrever, time.Second)
		close(acompanhado)
	}()
	resultado := elevacao.Executar("--instalar-pelos-scripts")
	parar()
	<-acompanhado
	return resultado
}

// textoDaReinstalacao: os textos do C#. O "Atualizar o gerenciador e reabrir" entra com a #430.
func textoDaReinstalacao(r elevacao.Resultado) string {
	switch r {
	case elevacao.Feito:
		return "Pronto: o site está na última versão do develop. O gerenciador continua na versão de antes."
	case elevacao.Cancelado:
		return "Reinstalação cancelada no aviso do Windows. O código já está atualizado: rode o instalar-windows.cmd para terminar."
	default:
		return "A reinstalação falhou; o site da versão anterior volta sozinho. Veja a caixa abaixo e o log em " +
			"instaladores\\instalacao-windows.log."
	}
}

// mostrarNaPasta abre o Explorer com o arquivo selecionado.
func mostrarNaPasta(arquivo string) error {
	return exec.Command("explorer.exe", "/select,"+arquivo).Start()
}

// O que é só do Linux: o comando para atualizar pelos scripts, o "sem login" (linger) e o registro do serviço.

func temBandeja() bool                                  { return true }
func comandoParaAtualizar(instalacao.Instalacao) string { return "" }
func lerSemLogin() string                               { return "" }
func manterNoArSemLogin() error                         { return errors.ErrUnsupported }
func registroDoServico() *tela.Notas                    { return nil }
