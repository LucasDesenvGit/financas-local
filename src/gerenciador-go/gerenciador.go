package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/atualizacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/elevacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/icones"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/preferencias"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/situacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

const (
	titulo = "Gerenciador do Finanças Local"
	// largura da página, como a do C# (cartão de 448 e 16 de cada lado); a janela soma a moldura, e a altura
	// acompanha o conteúdo (POST /altura).
	largura = 480
	// intervalo entre as consultas da situação, o Intervalo do FormPrincipal.cs.
	intervalo = 5 * time.Second
	// releases é a página da versão mais recente ("Baixar o instalador .msi", instalação pelos scripts; ADR 0028).
	releases = "https://github.com/LucasDesenvGit/financas-local/releases/latest"
)

// O cofrinho do site (o mesmo icone.ico do gerenciador em C#), na janela e na bandeja.
//
//go:embed icone.png
var icone []byte

// As páginas do site que a janela abre no navegador (Abrir o site, Ver os erros do site, Abrir o manual).
var paginasDoSite = map[string]bool{"": true, "configuracoes?aba=erros": true, "manual": true}

// O que a janela diz enquanto liga, desliga ou reinicia (ServicoAsync do C#).
var enquanto = map[string]string{"ligar": "Ligando o site…", "desligar": "Desligando o site…", "reiniciar": "Reiniciando o site…"}

// gerenciador guarda a janela (destruída ao fechar e criada de novo pela bandeja) e a situação. A página é servida e
// a situação é consultada em outras goroutines, por isso a trava.
type gerenciador struct {
	app      *application.App
	inst     instalacao.Instalacao
	consulta situacao.Consulta

	trava       sync.Mutex
	janela      *application.WebviewWindow
	situacao    tela.Situacao
	exemplo     bool // -mostrar: a situação fica fixa
	ocupado     bool // ligando, desligando ou reiniciando
	consultando bool
	maisOpcoes  bool
	ultimaCopia string // o arquivo da última cópia do banco, para "Abrir a pasta"
	nova        versaoNova
	dev         develop

	bandeja      *application.SystemTray
	itemSituacao *application.MenuItem
}

func novoGerenciador(inst instalacao.Instalacao) *gerenciador {
	return &gerenciador{inst: inst, consulta: situacao.Nova(inst), situacao: tela.Verificando,
		nova: versaoNova{cliente: atualizacao.NovoCliente(versao)}}
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
			// Fechar a janela não encerra quando o gerenciador fica na bandeja (fechou, abaixo).
			DisableQuitOnLastWindowClosed: true,
			// O cache do WebView2 na pasta do Finanças Local do usuário, e não em AppData\Roaming\<exe>.
			WebviewUserDataPath: filepath.Join(os.Getenv("LOCALAPPDATA"), "FinancasLocal", "Gerenciador", "WebView2"),
		},
	})
	g.montarBandeja()
	if comJanela {
		g.abrir()
	}
	go func() {
		g.atualizarSituacao()
		go g.procurarClone()
		// Como o IniciarAsync do C#: depois da primeira situação (que diz a versão do site), procura a versão nova.
		if g.inst.PeloMsi && preferencias.ProcurarAoAbrir() {
			go g.procurar()
		}
		for {
			time.Sleep(intervalo)
			g.atualizarSituacao()
		}
	}()
	if err := g.app.Run(); err != nil {
		log.Fatal(err)
	}
}

func (g *gerenciador) tela() tela.Tela {
	g.trava.Lock()
	defer g.trava.Unlock()
	return tela.Tela{
		Situacao:          g.situacao,
		Ocupado:           g.ocupado,
		Versao:            g.textoDaVersao(),
		PeloMsi:           g.inst.PeloMsi,
		MaisOpcoes:        g.maisOpcoes,
		IniciarComSistema: preferencias.IniciarComSistema(),
		ProcurarAoAbrir:   preferencias.ProcurarAoAbrir(),
		PastaDados:        g.inst.PastaDados,
		Endereco:          g.inst.Site,
		Escuro:            preferencias.Escuro(),

		ReceberDesenvolvimento: receberDesenvolvimento(g.situacao.VersaoSite),
		Procura:                g.nova.procura,
		Develop:                g.developNaTela(),
	}
}

/* ------------------------------------------------------------ situação */

// atualizarSituacao consulta uma vez por vez: com o site demorando, o relógio não empilha pedidos.
func (g *gerenciador) atualizarSituacao() {
	g.trava.Lock()
	if g.exemplo || g.ocupado || g.consultando {
		g.trava.Unlock()
		g.mostrarNaBandeja()
		return
	}
	g.consultando = true
	g.trava.Unlock()

	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	nova := g.consulta.Consultar(ctx)
	cancelar()

	g.trava.Lock()
	g.situacao, g.consultando = nova, false
	g.trava.Unlock()
	g.mostrarNaBandeja()
}

// trocarDetalhe mostra o resultado de uma ação até a próxima consulta, como o _detalhe.Text do C#.
func (g *gerenciador) trocarDetalhe(texto string) {
	g.trava.Lock()
	g.situacao.Detalhe = texto
	g.trava.Unlock()
}

// mostrarNaBandeja: o ícone com o ponto da cor, a dica e a primeira linha do menu.
func (g *gerenciador) mostrarNaBandeja() {
	g.trava.Lock()
	s := g.situacao
	g.trava.Unlock()
	if g.bandeja == nil {
		return
	}
	if comPonto, err := icones.ComPonto(icone, icones.CorDoEstado(s.Estado)); err == nil {
		g.bandeja.SetIcon(comPonto)
	}
	g.bandeja.SetTooltip(limitar("Finanças Local: "+s.Titulo, 127))
	g.itemSituacao.SetLabel("Finanças Local: " + s.Curto())
}

/* ------------------------------------------------------------ ações */

func (g *gerenciador) acaoNoServico(acao string) {
	g.trava.Lock()
	if g.ocupado {
		g.trava.Unlock()
		return
	}
	g.ocupado = true
	g.situacao = tela.Situacao{Estado: tela.Mudando, Titulo: enquanto[acao], Detalhe: "O Windows vai pedir permissão de administrador."}
	g.trava.Unlock()
	g.mostrarNaBandeja()

	resultado := elevacao.Executar("--servico", acao)

	g.trava.Lock()
	g.ocupado = false
	g.trava.Unlock()
	g.atualizarSituacao()
	switch resultado {
	case elevacao.Cancelado:
		g.trocarDetalhe("Cancelado: nada mudou.")
	case elevacao.Falhou:
		g.trocarDetalhe("Não deu certo. Veja os eventos do Windows (Visualizador de Eventos › Aplicativo, origem FinancasLocal).")
	}
}

// salvarCopia pergunta onde salvar, copia elevado e devolve o aviso para a janela mostrar (nil: escolha cancelada).
func (g *gerenciador) salvarCopia() *tela.Aviso {
	opcoes := &application.SaveFileDialogOptions{
		Title:     "Salvar uma cópia do banco",
		Filename:  "FinancasLocal-" + time.Now().Format("2006-01-02") + ".db",
		Directory: preferencias.PastaDocumentos(),
		Filters:   []application.FileFilter{{DisplayName: "Banco do Finanças Local (*.db)", Pattern: "*.db"}},
	}
	g.trava.Lock()
	if g.janela != nil {
		opcoes.Window = g.janela
	}
	g.trava.Unlock()
	destino, err := g.app.Dialog.SaveFileWithOptions(opcoes).PromptForSingleSelection()
	if err != nil || destino == "" {
		return nil
	}
	resultado := elevacao.Executar("--backup", destino)
	if resultado == elevacao.Feito {
		g.trava.Lock()
		g.ultimaCopia = destino
		g.trava.Unlock()
	}
	aviso := tela.AvisoDaCopia(resultado == elevacao.Feito, resultado == elevacao.Cancelado, destino)
	return &aviso
}

// abrirPasta abre o Explorer com a última cópia selecionada. O caminho é o que o próprio gerenciador salvou, nunca
// um que venha da página.
func (g *gerenciador) abrirPasta() {
	g.trava.Lock()
	arquivo := g.ultimaCopia
	g.trava.Unlock()
	if arquivo == "" {
		return
	}
	if err := exec.Command("explorer.exe", "/select,"+arquivo).Start(); err != nil {
		log.Print(err)
	}
}

func (g *gerenciador) abrirNoNavegador(endereco string) {
	if err := g.app.Browser.OpenURL(endereco); err != nil {
		log.Print(err)
	}
}

/* ------------------------------------------------------------ página */

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
	mux.HandleFunc("GET /estado", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(g.tela().Atualizacao())
	})
	mux.HandleFunc("POST /abrir", func(w http.ResponseWriter, r *http.Request) {
		pagina := r.URL.Query().Get("pagina")
		if !paginasDoSite[pagina] {
			http.Error(w, "página desconhecida", http.StatusBadRequest)
			return
		}
		g.abrirNoNavegador(g.inst.Pagina(pagina))
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /acao/{acao}", func(w http.ResponseWriter, r *http.Request) {
		switch acao := r.PathValue("acao"); acao {
		case "ligar", "desligar", "reiniciar":
			// O aviso do UAC e a espera não seguram a página: a situação nova chega pelo GET /estado.
			go g.acaoNoServico(acao)
		case "copia":
			// Espera o diálogo de salvar e o UAC: a resposta é o aviso que a janela mostra, no desenho dela.
			if aviso := g.salvarCopia(); aviso != nil {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(aviso)
				return
			}
		case "abrir-pasta":
			g.abrirPasta()
		case "baixar-msi":
			g.abrirNoNavegador(releases)
		case "procurar":
			go g.procurar()
		case "atualizar":
			go g.atualizar()
		case "atualizar-develop":
			go g.atualizarDevelop()
		case "cancelar-download":
			g.cancelarDownload()
		default:
			http.Error(w, "ação desconhecida", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /preferencia/{nome}", func(w http.ResponseWriter, r *http.Request) {
		ligado := r.URL.Query().Get("ligado") == "true"
		var err error
		switch r.PathValue("nome") {
		case "iniciar":
			err = preferencias.DefinirIniciarComSistema(ligado)
		case "procurar":
			err = preferencias.DefinirProcurarAoAbrir(ligado)
		case "desenvolvimento":
			if err = preferencias.DefinirReceberDesenvolvimento(ligado); err == nil {
				g.esquecerProcura()
			}
		default:
			http.Error(w, "preferência desconhecida", http.StatusBadRequest)
			return
		}
		if err != nil {
			log.Print(err)
			http.Error(w, "não deu para gravar", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /confirmar-develop", func(w http.ResponseWriter, _ *http.Request) {
		responderJSON(w, g.confirmacaoDoDevelop())
	})
	mux.HandleFunc("GET /confirmar-atualizacao", func(w http.ResponseWriter, _ *http.Request) {
		responderJSON(w, g.confirmacao())
	})
	mux.HandleFunc("GET /notas", func(w http.ResponseWriter, _ *http.Request) { responderJSON(w, g.notas()) })
	mux.HandleFunc("POST /abrir-link", func(w http.ResponseWriter, r *http.Request) {
		endereco := r.URL.Query().Get("url")
		if !linkDaWeb(endereco) {
			http.Error(w, "só links da web", http.StatusBadRequest)
			return
		}
		g.abrirNoNavegador(endereco)
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

/* ------------------------------------------------------------ janela e bandeja */

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
	// A janela nova também vem para a frente (e com o foco), e não só a que já existia.
	var desligar func()
	desligar = j.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
		desligar()
		j.Focus()
	})
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
	g.bandeja = g.app.SystemTray.New()
	// Como o MenuBandeja.cs: a situação no topo, sem ação; depois abrir o site, mostrar a janela e sair.
	menu := g.app.NewMenu()
	g.itemSituacao = menu.Add("Finanças Local: " + tela.Verificando.Curto()).SetEnabled(false)
	menu.AddSeparator()
	menu.Add("Abrir o site").OnClick(func(*application.Context) { g.abrirNoNavegador(g.inst.Pagina("")) })
	menu.Add("Mostrar o gerenciador").OnClick(func(*application.Context) { g.abrir() })
	menu.AddSeparator()
	menu.Add("Sair").OnClick(func(*application.Context) { g.app.Quit() })
	g.bandeja.SetMenu(menu)
	// Duplo clique mostra a janela; o clique simples abre o menu.
	g.bandeja.OnDoubleClick(g.abrir)
	g.mostrarNaBandeja()
}

// limitar corta o texto da dica do ícone, que o Windows limita a 127 caracteres.
func limitar(texto string, maximo int) string {
	r := []rune(texto)
	if len(r) <= maximo {
		return texto
	}
	return string(r[:maximo-1]) + "…"
}

// responderJSON responde o valor em JSON, ou 404 quando não há (nil).
func responderJSON[T any](w http.ResponseWriter, valor *T) {
	if valor == nil {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(valor)
}
