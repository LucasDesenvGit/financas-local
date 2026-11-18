package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/atualizacao"
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
	texto   string      // durante e depois da atualização, substitui o texto do cartão
	notas   *tela.Notas // o "O que mudou" da última atualização (#432), ou nil
	// reabrir: a reinstalação deu certo, e o gerenciador pode se atualizar e reabrir (#430).
	reabrir bool
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
	versaoDoSite := g.situacao.VersaoSite
	g.dev.rodando, g.ocupado, g.dev.saida, g.dev.notas, g.dev.reabrir = true, true, nil, nil, false
	g.dev.texto = "Buscando o develop no GitHub…"
	g.trava.Unlock()
	// A versão de antes é a do site instalado (que o gerenciador já consulta): o clone não guarda mais o número (ADR 0070).
	antes, temAntes := atualizacao.LerVersao(versaoDoSite)
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

	notas := g.notasDaAtualizacao(raiz, antes, temAntes)
	g.trava.Lock()
	g.dev.notas = notas
	g.trava.Unlock()
	g.textoDoDevelop(reinstalando)
	resultado := reinstalar(raiz, g.escrever)
	g.trava.Lock()
	g.dev.reabrir = resultado == elevacao.Feito
	g.trava.Unlock()
	g.textoDoDevelop(textoDaReinstalacao(resultado))
}

// atualizarGerenciador é o "Atualizar o gerenciador e reabrir" (#430): deixa o instalador do gerenciador rodando à
// parte, com o comando que reabre, e fecha. Só depois de a reinstalação pelo develop dar certo.
func (g *gerenciador) atualizarGerenciador() {
	g.trava.Lock()
	raiz, pode := g.dev.raiz, g.dev.reabrir && !g.dev.rodando
	g.trava.Unlock()
	if !pode {
		return
	}
	executavel, err := os.Executable()
	if err != nil {
		g.escrever("Não deu para achar o próprio gerenciador: " + err.Error())
		return
	}
	if err := reabrirDepoisDeInstalar(raiz, executavel); err != nil {
		g.escrever("Não deu para atualizar o gerenciador: " + err.Error())
		return
	}
	g.app.Quit()
}

// developNaTela é o que a tela mostra do develop; chamado com a trava.
func (g *gerenciador) developNaTela() tela.Develop {
	return tela.Develop{Disponivel: g.dev.raiz != "", Rodando: g.dev.rodando, Saida: strings.Join(g.dev.saida, "\n"),
		Notas: g.dev.notas != nil, ReabrirGerenciador: g.dev.reabrir && !g.dev.rodando}
}

// notasDaAtualizacao pede ao clone, depois de trazer o develop, o número novo e as seções do que ainda não foi publicado
// (src/backend/cmd/versao, ADR 0070), e monta o "O que mudou". Sem o Go ou com erro no cálculo, avisa na caixa de saída e
// segue sem as notas: a reinstalação não depende delas.
func (g *gerenciador) notasDaAtualizacao(raiz string, antes atualizacao.Versao, temAntes bool) *tela.Notas {
	ctx := context.Background()
	texto, err := desenvolvimento.CalcularVersao(ctx, raiz)
	if err != nil {
		g.escrever("Não deu para calcular a versão nova, então o \"O que mudou\" fica de fora: " + err.Error())
		return nil
	}
	nova, temNova := atualizacao.LerVersao(texto)
	changelog, err := desenvolvimento.CalcularVersao(ctx, raiz, "-changelog")
	if err != nil {
		g.escrever("Não deu para montar o \"O que mudou\": " + err.Error())
		return nil
	}
	// As seções que o CHANGELOG.md do clone ainda tem de antes da troca (ADR 0070) vêm depois das novas.
	if antigo, err := os.ReadFile(filepath.Join(raiz, "CHANGELOG.md")); err == nil {
		changelog += "\n\n" + desdeAPrimeiraVersao(string(antigo))
	}
	return notasDoDevelop(nova, temNova, changelog, antes, temAntes)
}

// desdeAPrimeiraVersao tira o título e a explicação do formato do topo do CHANGELOG.md, que ficariam grudados na
// última seção do que veio antes dele.
func desdeAPrimeiraVersao(changelog string) string {
	changelog = strings.ReplaceAll(changelog, "\r\n", "\n")
	if strings.HasPrefix(changelog, "## [") {
		return changelog
	}
	if _, resto, ok := strings.Cut(changelog, "\n## ["); ok {
		return "## [" + resto
	}
	return ""
}

// notasDoDevelop é o "O que mudou" entre a versão de antes e a que veio (#432): as seções do changelog (o -changelog
// seguido do CHANGELOG.md do clone), sem rede. nil sem versão nova ou sem seção.
func notasDoDevelop(nova atualizacao.Versao, temNova bool, changelog string, antes atualizacao.Versao, temAntes bool) *tela.Notas {
	if !temNova {
		return nil
	}
	var de *atualizacao.Versao
	titulo := "O que mudou na versão " + nova.String()
	if temAntes {
		de = &antes
		titulo = "O que mudou desde a " + antes.String()
	}
	secoes := atualizacao.SecoesDoChangelog(changelog, de, nova)
	if secoes == "" {
		return nil
	}
	return &tela.Notas{Titulo: titulo, Novidades: atualizacao.HTMLDasNotas(secoes),
		Pagina: "https://github.com/" + atualizacao.Repositorio + "/blob/develop/CHANGELOG.md", TextoPagina: "Ver o CHANGELOG no GitHub"}
}
