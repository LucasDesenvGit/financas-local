package main

import (
	"context"
	"errors"
	"log"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/atualizacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/preferencias"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// versaoNova guarda a procura e o download da versão nova (ADR 0028 e 0040), com a trava do gerenciador.
type versaoNova struct {
	cliente    atualizacao.Cliente
	procura    tela.Procura
	resultado  string // a frase da última procura
	texto      string // durante e depois do download, substitui o texto do cartão
	disponivel *atualizacao.Disponivel
	cancelar   context.CancelFunc
}

// versaoAtual é a que está rodando (a do site, quando ele responde): é ela que conta para "tem versão nova".
func (g *gerenciador) versaoAtual() atualizacao.Versao {
	g.trava.Lock()
	doSite := g.situacao.VersaoSite
	g.trava.Unlock()
	return versaoQueRoda(doSite)
}

func (g *gerenciador) versaoAtualDoSite() string {
	g.trava.Lock()
	defer g.trava.Unlock()
	return g.situacao.VersaoSite
}

func versaoQueRoda(doSite string) atualizacao.Versao {
	if v, ok := atualizacao.LerVersao(doSite); ok {
		return v
	}
	v, _ := atualizacao.LerVersao(versao)
	return v
}

// receberDesenvolvimento: a escolha do interruptor; sem escolha, o tipo da versão que está rodando.
func receberDesenvolvimento(doSite string) bool {
	return preferencias.ReceberDesenvolvimento(versaoQueRoda(doSite).EhDesenvolvimento())
}

func (g *gerenciador) textoDaVersao() string {
	if g.nova.texto != "" {
		return g.nova.texto
	}
	if g.dev.texto != "" {
		return g.dev.texto
	}
	if g.dev.raiz != "" {
		return tela.TextoDeDesenvolvimento(g.dev.raiz)
	}
	if linux && !g.inst.PeloMsi {
		return tela.TextoPelosScriptsNoLinux(g.comando != "")
	}
	return tela.TextoDaVersao(versao, g.inst.PeloMsi, g.situacao.VersaoSite != "", g.nova.resultado)
}

// procurar procura a versão nova, uma vez por vez.
func (g *gerenciador) procurar() {
	g.trava.Lock()
	if !g.inst.PeloMsi || g.nova.procura.Procurando || g.nova.procura.Baixando {
		g.trava.Unlock()
		return
	}
	g.nova.procura.Procurando = true
	g.nova.texto = ""
	g.nova.resultado = "Procurando versão nova…"
	g.trava.Unlock()

	desenvolvimento := receberDesenvolvimento(g.versaoAtualDoSite())
	ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Second)
	disponivel, err := g.nova.cliente.Procurar(ctx, desenvolvimento)
	cancelar()
	atual := g.versaoAtual()

	g.trava.Lock()
	defer g.trava.Unlock()
	g.nova.procura.Procurando = false
	if err != nil {
		log.Print(err)
		g.nova.resultado = tela.ErroDaProcura
		return
	}
	nova := disponivel != nil && disponivel.Versao.Comparar(atual) > 0
	g.nova.disponivel, g.nova.procura.Nova = nil, ""
	versaoNova := ""
	if nova {
		g.nova.disponivel, g.nova.procura.Nova, versaoNova = disponivel, disponivel.Versao.String(), disponivel.Versao.String()
	}
	g.nova.resultado = tela.ResultadoDaProcura(nova, desenvolvimento, versaoNova)
}

// esquecerProcura: trocou o tipo de versão, a procura anterior não vale mais.
func (g *gerenciador) esquecerProcura() {
	g.trava.Lock()
	defer g.trava.Unlock()
	if g.nova.procura.Baixando {
		return
	}
	g.nova.disponivel, g.nova.procura.Nova, g.nova.resultado, g.nova.texto = nil, "", "", ""
}

// confirmacao é a pergunta antes de baixar (nil sem versão nova).
func (g *gerenciador) confirmacao() *tela.Aviso {
	g.trava.Lock()
	defer g.trava.Unlock()
	if g.nova.disponivel == nil {
		return nil
	}
	a := tela.ConfirmarAtualizacao(g.nova.disponivel.Versao.String())
	return &a
}

// atualizar baixa o .msi, confere o SHA-256 e abre o instalador; o gerenciador fecha para o instalador poder trocá-lo.
func (g *gerenciador) atualizar() {
	g.trava.Lock()
	d := g.nova.disponivel
	if d == nil || g.nova.procura.Baixando {
		g.trava.Unlock()
		return
	}
	ctx, cancelar := context.WithCancel(context.Background())
	g.nova.cancelar = cancelar
	g.nova.procura.Baixando, g.nova.procura.Progresso = true, 0
	g.nova.texto = "Baixando a versão " + d.Versao.String() + "…"
	g.trava.Unlock()

	msi, err := g.nova.cliente.Baixar(ctx, *d, func(p float64) {
		g.trava.Lock()
		g.nova.procura.Progresso = int(p * 100)
		g.trava.Unlock()
	})
	cancelar()

	g.trava.Lock()
	g.nova.procura.Baixando, g.nova.cancelar = false, nil
	switch {
	case errors.Is(err, context.Canceled):
		g.nova.texto = "Download cancelado. Nada mudou."
	case err != nil:
		g.nova.texto = fraseDoErro(err)
	default:
		g.nova.texto = "Abrindo o instalador…"
	}
	g.trava.Unlock()
	if err != nil {
		log.Print(err)
		return
	}
	if err := atualizacao.Instalar(msi); err != nil {
		log.Print(err)
		g.trava.Lock()
		g.nova.texto = "Não deu para abrir o instalador. Ele está em " + msi + "."
		g.trava.Unlock()
		return
	}
	g.app.Quit()
}

func (g *gerenciador) cancelarDownload() {
	g.trava.Lock()
	defer g.trava.Unlock()
	if g.nova.cancelar != nil {
		g.nova.cancelar()
	}
}

// fraseDoErro: os erros da conferência dizem o que houve; os de rede viram uma frase sem jargão.
func fraseDoErro(err error) string {
	if errors.Is(err, atualizacao.ErrSemSomas) || errors.Is(err, atualizacao.ErrHashDiferente) ||
		strings.Contains(err.Error(), "SHA256SUMS") {
		r, n := utf8.DecodeRuneInString(err.Error())
		return string(unicode.ToUpper(r)) + err.Error()[n:] + "."
	}
	return "Não deu para baixar a versão nova agora (sem internet?). Nada foi instalado."
}

// notas é o "O que mudou" da versão nova (nil sem versão nova).
func (g *gerenciador) notas() *tela.Notas {
	g.trava.Lock()
	d, doDevelop := g.nova.disponivel, g.dev.notas
	g.trava.Unlock()
	if doDevelop != nil {
		return doDevelop
	}
	if d == nil {
		return nil
	}
	atual := g.versaoAtual()
	n := atualizacao.SepararNotas(d.Notas, &atual)
	novidades := n.Novidades
	if strings.TrimSpace(novidades) == "" && !n.TemResto() {
		novidades = "Esta versão não tem notas."
	}
	notas := tela.Notas{Titulo: "O que mudou na versão " + d.Versao.String(), Novidades: atualizacao.HTMLDasNotas(novidades),
		Pagina: d.Pagina}
	if n.TemResto() {
		notas.Resto = atualizacao.HTMLDasNotas(n.Resto)
	}
	return &notas
}

// linkDaWeb: só links http e https abrem no navegador (nada de arquivo local ou outro esquema vindo das notas).
func linkDaWeb(endereco string) bool {
	u, err := url.Parse(endereco)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}
