package main

import (
	"context"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/desenvolvimento"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/elevacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// maximoDeLinhas da caixa de saída: o instalador escreve muito; as mais antigas saem.
const maximoDeLinhas = 2000

// develop é o "Atualizar para a última do develop" (ADR 0040), com a trava do gerenciador.
type develop struct {
	raiz    string // a pasta do clone na branch develop, ou "" (não é instalação de desenvolvimento)
	rodando bool
	saida   []string
	texto   string // durante e depois da atualização, substitui o texto do cartão
}

// procurarClone vê, sem travar a janela, se a instalação é de desenvolvimento (o git leva um instante).
func (g *gerenciador) procurarClone() {
	if g.inst.PeloMsi {
		return
	}
	raiz := desenvolvimento.Disponivel(context.Background(), g.inst)
	g.trava.Lock()
	g.dev.raiz = raiz
	g.trava.Unlock()
}

func (g *gerenciador) escrever(linha string) {
	g.trava.Lock()
	defer g.trava.Unlock()
	g.dev.saida = append(g.dev.saida, linha)
	if len(g.dev.saida) > maximoDeLinhas {
		g.dev.saida = g.dev.saida[len(g.dev.saida)-maximoDeLinhas:]
	}
}

func (g *gerenciador) textoDoDevelop(texto string) {
	g.trava.Lock()
	g.dev.texto = texto
	g.trava.Unlock()
}

// confirmacaoDoDevelop é a pergunta antes de atualizar (nil fora da instalação de desenvolvimento).
func (g *gerenciador) confirmacaoDoDevelop() *tela.Aviso {
	g.trava.Lock()
	defer g.trava.Unlock()
	if g.dev.raiz == "" {
		return nil
	}
	a := tela.ConfirmarDevelop()
	return &a
}

// atualizarDevelop traz o develop como o usuário e reinstala como administrador (UAC), num passo à parte, como o
// AtualizarPeloGitAsync do C#.
func (g *gerenciador) atualizarDevelop() {
	g.trava.Lock()
	raiz := g.dev.raiz
	if raiz == "" || g.dev.rodando || g.ocupado {
		g.trava.Unlock()
		return
	}
	g.dev.rodando, g.ocupado, g.dev.saida = true, true, nil
	g.dev.texto = "Buscando o develop no GitHub…"
	g.trava.Unlock()
	defer func() {
		g.trava.Lock()
		g.dev.rodando, g.ocupado = false, false
		g.trava.Unlock()
		g.atualizarSituacao()
	}()

	switch codigo := desenvolvimento.TrazerCodigo(context.Background(), raiz, g.escrever); codigo {
	case desenvolvimento.Atualizado:
	case desenvolvimento.SemNovidade:
		g.textoDoDevelop("Já está na última versão do develop. Nada mudou.")
		return
	case desenvolvimento.Recusado:
		g.textoDoDevelop("Não atualizou, e nada foi mudado: veja o motivo e o que fazer na caixa abaixo.")
		return
	default:
		g.textoDoDevelop("Não deu para buscar o develop. Nada foi mudado; veja a caixa abaixo.")
		return
	}

	g.textoDoDevelop("Código atualizado. Reinstalando o site (permita no aviso do Windows)…")
	ctx, parar := context.WithCancel(context.Background())
	acompanhado := make(chan struct{})
	go func() {
		desenvolvimento.AcompanharLog(ctx, desenvolvimento.Log(raiz), g.escrever, time.Second)
		close(acompanhado)
	}()
	resultado := elevacao.Executar("--instalar-pelos-scripts")
	parar()
	<-acompanhado
	g.textoDoDevelop(textoDaReinstalacao(resultado))
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

// developNaTela é o que a tela mostra do develop; chamado com a trava.
func (g *gerenciador) developNaTela() tela.Develop {
	return tela.Develop{Disponivel: g.dev.raiz != "", Rodando: g.dev.rodando, Saida: strings.Join(g.dev.saida, "\n")}
}
