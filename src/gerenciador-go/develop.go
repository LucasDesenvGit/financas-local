package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/atualizacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/desenvolvimento"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// maximoDeLinhas da caixa de saída: o instalador escreve muito; as mais antigas saem.
const maximoDeLinhas = 2000

// develop é o "Atualizar para a última do develop" (ADR 0040), com a trava do gerenciador.
type develop struct {
	raiz    string // a pasta do clone na branch develop, ou "" (não é instalação de desenvolvimento)
	rodando bool
	saida   []string
	texto   string      // durante e depois da atualização, substitui o texto do cartão
	notas   *tela.Notas // o "O que mudou" da última atualização (#432), ou nil
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
	a := tela.ConfirmarDevelop(linux)
	return &a
}

// atualizarDevelop traz o develop como o usuário e reinstala (no Windows, como administrador, pelo UAC, num passo à
// parte), como o AtualizarPeloGitAsync do C#.
func (g *gerenciador) atualizarDevelop() {
	g.trava.Lock()
	raiz := g.dev.raiz
	if raiz == "" || g.dev.rodando || g.ocupado {
		g.trava.Unlock()
		return
	}
	g.dev.rodando, g.ocupado, g.dev.saida, g.dev.notas = true, true, nil, nil
	g.dev.texto = "Buscando o develop no GitHub…"
	g.trava.Unlock()
	antes, temAntes := versaoDoClone(raiz)
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

	notas := notasDoDevelop(raiz, antes, temAntes)
	g.trava.Lock()
	g.dev.notas = notas
	g.trava.Unlock()
	g.textoDoDevelop(reinstalando)
	g.textoDoDevelop(textoDaReinstalacao(reinstalar(raiz, g.escrever)))
}

// developNaTela é o que a tela mostra do develop; chamado com a trava.
func (g *gerenciador) developNaTela() tela.Develop {
	return tela.Develop{Disponivel: g.dev.raiz != "", Rodando: g.dev.rodando, Saida: strings.Join(g.dev.saida, "\n"),
		Notas: g.dev.notas != nil}
}

// versaoDoClone é a versão do Directory.Build.props do clone (antes ou depois de trazer o develop).
func versaoDoClone(raiz string) (atualizacao.Versao, bool) {
	props, err := os.ReadFile(filepath.Join(raiz, "Directory.Build.props"))
	if err != nil {
		return atualizacao.Versao{}, false
	}
	return atualizacao.VersaoDoProjeto(string(props))
}

// notasDoDevelop é o "O que mudou" entre a versão de antes e a que veio (#432): as seções do CHANGELOG.md do próprio
// clone, sem rede. nil sem versão nova ou sem seção.
func notasDoDevelop(raiz string, antes atualizacao.Versao, temAntes bool) *tela.Notas {
	nova, ok := versaoDoClone(raiz)
	changelog, err := os.ReadFile(filepath.Join(raiz, "CHANGELOG.md"))
	if !ok || err != nil {
		return nil
	}
	var de *atualizacao.Versao
	titulo := "O que mudou na versão " + nova.String()
	if temAntes {
		de = &antes
		titulo = "O que mudou desde a " + antes.String()
	}
	secoes := atualizacao.SecoesDoChangelog(string(changelog), de, nova)
	if secoes == "" {
		return nil
	}
	return &tela.Notas{Titulo: titulo, Novidades: atualizacao.HTMLDasNotas(secoes),
		Pagina: "https://github.com/" + atualizacao.Repositorio + "/blob/develop/CHANGELOG.md", TextoPagina: "Ver o CHANGELOG no GitHub"}
}
