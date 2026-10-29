package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"

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
		// Uma classe só nossa, para o avisarNaBandeja achar a janela do ícone (e não a de outro programa com Wails).
		WndClass: classeDaJanela,
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

// classeDaJanela é a classe das janelas do Wails (a da página e a do ícone da bandeja).
const classeDaJanela = "FinancasLocalGerenciador"

var findWindowEx = windows.NewLazySystemDLL("user32.dll").NewProc("FindWindowExW")

// avisarNaBandeja mostra o balão no ícone da bandeja (no Windows 10 e 11, a notificação no canto), como o
// ShowBalloonTip do C#. O Wails v3.0.0-beta.34 não tem balão: o ícone é dele, da janela só de mensagens (HWND_MESSAGE)
// que ele cria com a nossa classe, e tem o id 1 (a nossa única bandeja). O aviso troca só o balão (NIF_INFO).
func avisarNaBandeja(a tela.AvisoNaBandeja) {
	classe, err := windows.UTF16PtrFromString(classeDaJanela)
	if err != nil {
		return
	}
	var anterior uintptr
	for {
		h, _, _ := findWindowEx.Call(uintptr(w32.HWND_MESSAGE), anterior, uintptr(unsafe.Pointer(classe)), 0)
		if h == 0 {
			log.Print("aviso na bandeja: o ícone não foi achado")
			return
		}
		anterior = h
		if _, pid := w32.GetWindowThreadProcessId(w32.HWND(h)); pid != os.Getpid() {
			continue
		}
		nid := w32.NOTIFYICONDATA{HWnd: w32.HWND(h), UID: 1, UFlags: w32.NIF_INFO, DwInfoFlags: w32.NIIF_INFO}
		nid.CbSize = uint32(unsafe.Sizeof(nid))
		if a.Alerta {
			nid.DwInfoFlags = w32.NIIF_WARNING
		}
		// O último caractere de cada campo fica zero: o texto longo é cortado e continua terminado em zero.
		copiarTexto(nid.SzInfoTitle[:len(nid.SzInfoTitle)-1], "Finanças Local")
		copiarTexto(nid.SzInfo[:len(nid.SzInfo)-1], a.Texto)
		if w32.ShellNotifyIcon(w32.NIM_MODIFY, &nid) {
			return
		}
	}
}

func copiarTexto(campo []uint16, texto string) {
	if t, err := windows.UTF16FromString(texto); err == nil {
		copy(campo, t)
	}
}
