// O gerenciador do site em Go (ADR 0052, #418): o ícone na bandeja e a janela própria com o Wails v3, igual à do
// gerenciador em C# (src/gerenciador/Financas.Gerenciador). A página é servida por dentro do programa, sem porta
// aberta.
//
// Ainda é o esqueleto: a situação do site, ligar e desligar e a cópia do banco entram na #419; a versão nova, na
// #420. Até lá, a situação fica em "Verificando…" (ou a do -mostrar, para comparar com o C#).
package main

import (
	_ "embed"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/preferencias"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// versao vem do Directory.Build.props na compilação (-ldflags "-X main.versao=...").
var versao = "0.0.0-dev.0"

const (
	titulo   = "Gerenciador do Finanças Local"
	endereco = "https://127.0.0.1:5443/"
	// largura da página, como a do C# (cartão de 448 e 16 de cada lado); a janela soma a moldura, e a altura
	// acompanha o conteúdo (POST /altura).
	largura = 480
)

// O cofrinho do site (o mesmo icone.ico do gerenciador em C#), na janela e na bandeja.
//
//go:embed icone.png
var icone []byte

// As páginas do site que a janela abre no navegador (Abrir o site, Ver os erros do site, Abrir o manual).
var paginasDoSite = map[string]bool{"": true, "configuracoes?aba=erros": true, "manual": true}

// Situações de exemplo do -mostrar, com os textos do Situacao.cs, até a consulta de verdade (#419).
var exemplos = map[string]tela.Situacao{
	"nao-instalado": {Estado: tela.NaoInstalado, Titulo: "O Finanças Local não está instalado neste computador.",
		Detalhe: "Instale pelo arquivo FinancasLocal.msi."},
	"desligado": {Estado: tela.Desligado, Titulo: "O site está desligado.", Detalhe: "Clique em “Ligar” para usar."},
	"ligando":   {Estado: tela.Mudando, Titulo: "O site está ligando…", Detalhe: "Aguarde alguns segundos."},
	"no-ar":     {Estado: tela.NoAr, Titulo: "O site está no ar.", Detalhe: "Versão " + versao + " · " + endereco},
	"sem-resposta": {Estado: tela.SemResposta, Titulo: "O site está ligado, mas não responde.",
		Detalhe: "Acabou de ligar? Espere alguns segundos. Se continuar assim, clique em “Reiniciar”."},
}

func main() {
	// --bandeja, como o C#: começando com o Windows, só o ícone na área de notificação.
	bandeja := flag.Bool("bandeja", false, "começa sem janela, só na área de notificação (início com o Windows)")
	mostrar := flag.String("mostrar", "", "mostra uma situação de exemplo (desligado, no-ar, ligando, sem-resposta, nao-instalado), até a #419")
	maisOpcoes := flag.Bool("mais-opcoes", false, "abre com \"Mais opções\" aberto")
	flag.Parse()

	situacao := tela.Verificando
	if s, ok := exemplos[*mostrar]; ok {
		situacao = s
	} else if *mostrar != "" {
		log.Fatalf("-mostrar=%s: não conheço essa situação", *mostrar)
	}
	g := &gerenciador{situacao: situacao, maisOpcoes: *maisOpcoes}
	g.iniciar(!*bandeja)
}

// gerenciador guarda a janela (que é destruída ao fechar e criada de novo pela bandeja) e o que ela mostra. A página
// é servida em outra goroutine, por isso a trava.
type gerenciador struct {
	app        *application.App
	trava      sync.Mutex
	janela     *application.WebviewWindow
	situacao   tela.Situacao
	maisOpcoes bool
}

func (g *gerenciador) iniciar(comJanela bool) {
	g.app = application.New(application.Options{
		Name:   titulo,
		Assets: application.AssetOptions{Handler: g.rotas()},
		Icon:   icone,
		// Uma cópia por usuário: abrir de novo mostra a janela da que já está aberta (talvez só na bandeja).
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "br.financaslocal.gerenciador",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { g.abrir() },
		},
		Windows: application.WindowsOptions{
			// Fechar a janela não encerra quando o gerenciador fica na bandeja (fechar, abaixo).
			DisableQuitOnLastWindowClosed: true,
			// O cache do WebView2 na pasta do Finanças Local do usuário, e não em AppData\Roaming\<exe>.
			WebviewUserDataPath: filepath.Join(os.Getenv("LOCALAPPDATA"), "FinancasLocal", "Gerenciador", "WebView2"),
		},
	})
	g.montarBandeja()
	if comJanela {
		g.abrir()
	}
	if err := g.app.Run(); err != nil {
		log.Fatal(err)
	}
}

func (g *gerenciador) tela() tela.Tela {
	g.trava.Lock()
	defer g.trava.Unlock()
	return tela.Tela{
		Situacao:          g.situacao,
		Versao:            tela.TextoDaVersao(versao, true, g.situacao.Estado == tela.NoAr),
		PeloMsi:           true,
		MaisOpcoes:        g.maisOpcoes,
		IniciarComSistema: preferencias.IniciarComSistema(),
		ProcurarAoAbrir:   preferencias.ProcurarAoAbrir(),
		PastaDados:        filepath.Join(os.Getenv("ProgramData"), "FinancasLocal", "dados"),
		Endereco:          endereco,
		Escuro:            preferencias.Escuro(),
	}
}

func (g *gerenciador) rotas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := g.tela().Escrever(w); err != nil {
			log.Print(err)
		}
	})
	mux.HandleFunc("GET /estilo.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(tela.Estilo)
	})
	mux.HandleFunc("GET /icone.png", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(icone)
	})
	mux.HandleFunc("POST /abrir", func(w http.ResponseWriter, r *http.Request) {
		pagina := r.URL.Query().Get("pagina")
		if !paginasDoSite[pagina] {
			http.Error(w, "página desconhecida", http.StatusBadRequest)
			return
		}
		g.abrirNoNavegador(pagina)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /acao/{acao}", func(w http.ResponseWriter, r *http.Request) {
		// Ligar, desligar, reiniciar e a cópia do banco entram na #419; procurar e baixar a versão nova, na #420.
		log.Printf("ação %q ainda não está no gerenciador em Go", r.PathValue("acao"))
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /altura", func(w http.ResponseWriter, r *http.Request) {
		// "conteúdo altura-visível largura-visível": a moldura é o tamanho da janela menos a área que a página vê.
		corpo, _ := io.ReadAll(io.LimitReader(r.Body, 32))
		g.ajustarTamanho(strings.Fields(string(corpo)))
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func (g *gerenciador) ajustarTamanho(partes []string) {
	if len(partes) != 3 {
		return
	}
	var n [3]int
	for i, p := range partes {
		v, err := strconv.Atoi(p)
		if err != nil {
			return
		}
		n[i] = v
	}
	g.trava.Lock()
	j := g.janela
	g.trava.Unlock()
	if j == nil {
		return
	}
	conteudo, alturaVisivel, larguraVisivel := n[0], n[1], n[2]
	larguraAtual, alturaAtual := j.Size()
	j.SetSize(largura+larguraAtual-larguraVisivel, min(max(conteudo+alturaAtual-alturaVisivel, 300), 1000))
}

func (g *gerenciador) abrirNoNavegador(pagina string) {
	if err := g.app.Browser.OpenURL(endereco + pagina); err != nil {
		log.Print(err)
	}
}

// abrir mostra a janela, criando outra se a anterior foi fechada.
func (g *gerenciador) abrir() {
	g.trava.Lock()
	defer g.trava.Unlock()
	if g.janela != nil {
		g.janela.Show().Focus()
		return
	}
	fundo := application.NewRGB(0xf5, 0xf4, 0xfa)
	if preferencias.Escuro() {
		fundo = application.NewRGB(0x13, 0x12, 0x1a)
	}
	// Como a janela do C#: tamanho fixo, sem maximizar, no centro da tela. O menu do botão direito e as ferramentas
	// de desenvolvedor saem no build com -tags production; zoom e atalhos do navegador já vêm desligados no Wails.
	j := g.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: titulo, Width: largura, Height: 560, URL: "/",
		DisableResize:              true,
		MaximiseButtonState:        application.ButtonHidden,
		InitialPosition:            application.WindowCentered,
		DefaultContextMenuDisabled: true,
		BackgroundColour:           fundo,
	})
	j.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) { g.fechou(j) })
	g.janela = j
}

// fechou: a janela vai embora com os processos do WebView2 (parado na bandeja, o gerenciador fica em ~50 MB, ADR
// 0052). Com "Iniciar com o Windows", ele continua na bandeja; sem, encerra, como o OnFormClosing do C#.
func (g *gerenciador) fechou(j *application.WebviewWindow) {
	g.trava.Lock()
	if g.janela == j {
		g.janela = nil
	}
	g.trava.Unlock()
	if !preferencias.IniciarComSistema() {
		go g.app.Quit()
	}
}

func (g *gerenciador) montarBandeja() {
	g.trava.Lock()
	s := g.situacao
	g.trava.Unlock()
	bandeja := g.app.SystemTray.New()
	bandeja.SetIcon(icone)
	bandeja.SetTooltip(limitar("Finanças Local: "+s.Titulo, 127))
	// Como o MenuBandeja.cs: a situação no topo, sem ação; depois abrir o site, mostrar a janela e sair.
	menu := g.app.NewMenu()
	menu.Add("Finanças Local: " + s.Curto()).SetEnabled(false)
	menu.AddSeparator()
	menu.Add("Abrir o site").OnClick(func(*application.Context) { g.abrirNoNavegador("") })
	menu.Add("Mostrar o gerenciador").OnClick(func(*application.Context) { g.abrir() })
	menu.AddSeparator()
	menu.Add("Sair").OnClick(func(*application.Context) { g.app.Quit() })
	bandeja.SetMenu(menu)
	// Duplo clique mostra a janela; o clique simples abre o menu.
	bandeja.OnDoubleClick(g.abrir)
}

// limitar corta o texto da dica do ícone, que o Windows limita a 127 caracteres.
func limitar(texto string, maximo int) string {
	r := []rune(texto)
	if len(r) <= maximo {
		return texto
	}
	return string(r[:maximo-1]) + "…"
}
