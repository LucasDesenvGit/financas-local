// Comando versao calcula o número da versão do Finanças Local (ADR 0070): parte do <VersaoBase> do
// Directory.Build.props e sobe a cada merge da linha principal do develop, pelo tipo do título do PR (feat sobe o do
// meio; o resto, o último). Nenhum PR mexe no número. Quem precisa dele (os scripts de instalação, o gerar-msi.ps1, o
// workflow do .msi, o contrato e o gerenciador) chama este programa, em src/backend:
//
//	go run ./cmd/versao                     # o número do HEAD: 3.24.7
//	go run ./cmd/versao -ref abc123         # o de outro commit
//	go run ./cmd/versao -publicada          # num commit de tag (o merge da release na main): o do commit da release
//	go run ./cmd/versao -changelog          # as seções do CHANGELOG do que ainda não foi publicado (changelog.d/)
//	go run ./cmd/versao -publicar 3.25.0 -data 2026-11-20   # na release/X.Y.Z: junta changelog.d/ numa seção do CHANGELOG.md
//
// Sem o histórico do git (uma cópia baixada em .zip, ou um clone raso), o número é a base com o sufixo -sem-git.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

func main() {
	if err := executar(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "versao:", err)
		os.Exit(1)
	}
}

var textoDeData = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func executar(argumentos []string, saida io.Writer) error {
	opcoes := flag.NewFlagSet("versao", flag.ContinueOnError)
	opcoes.SetOutput(io.Discard)
	pasta := opcoes.String("raiz", ".", "pasta de onde procurar o Directory.Build.props (sobe até achar)")
	ref := opcoes.String("ref", "HEAD", "o commit")
	publicada := opcoes.Bool("publicada", false, "o commit é o de uma tag: vale o commit da release (o segundo pai do merge)")
	changelog := opcoes.Bool("changelog", false, "as seções do CHANGELOG do que ainda não foi publicado")
	publicar := opcoes.String("publicar", "", "X.Y.Z: grava no CHANGELOG.md a seção única da versão, com tudo de changelog.d/, e apaga os arquivos")
	data := opcoes.String("data", "", "AAAA-MM-DD da seção do -publicar")
	if err := opcoes.Parse(argumentos); err != nil {
		return err
	}
	if *publicar != "" {
		if _, err := lerNumero(*publicar); err != nil {
			return fmt.Errorf("-publicar: %w", err)
		}
		if !textoDeData.MatchString(*data) {
			return errors.New("-publicar precisa do -data AAAA-MM-DD")
		}
	}

	raiz, err := acharRaiz(*pasta)
	if err != nil {
		return err
	}
	if !temHistorico(raiz) {
		if *changelog || *publicar != "" {
			return fmt.Errorf("-changelog e -publicar: %w", errSemHistorico)
		}
		return semGit(raiz, saida)
	}
	r := repositorio{raiz}
	hash, err := r.resolver(*ref)
	if err != nil {
		return err
	}
	if *publicada {
		if p := r.segundoPai(hash); p != "" {
			hash = p
		}
	}
	c, err := calcularEm(r, hash)
	if err != nil {
		return err
	}
	switch {
	case *publicar != "":
		entradas, err := c.entradasNaoPublicadas(r, hash)
		if err != nil {
			return err
		}
		texto := secaoPublicada(entradas, *publicar, *data)
		if err := gravarPublicacao(r, texto, entradas); err != nil {
			return err
		}
		_, err = io.WriteString(saida, strings.TrimRight(texto, "\n")+"\n")
		return err
	case *changelog:
		entradas, err := c.entradasNaoPublicadas(r, hash)
		if err != nil {
			return err
		}
		_, err = io.WriteString(saida, strings.TrimRight(secoesPorVersao(entradas), "\n")+"\n")
		return err
	}
	_, err = fmt.Fprintln(saida, c.versao())
	return err
}

// semGit é a versão sem histórico: a base do arquivo, com o sufixo -sem-git.
func semGit(raiz string, saida io.Writer) error {
	props, err := os.ReadFile(filepath.Join(raiz, arquivoDaBase))
	if err != nil {
		return err
	}
	base, err := lerBase(string(props))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(saida, "%s-sem-git\n", base)
	return err
}

// calculo é o que o git deu para um commit: a base, onde ela foi gravada e os commits depois dela.
type calculo struct {
	base     numero
	hashBase string
	commits  []commit
	versoes  []numero // a versão depois de cada commit
}

func calcularEm(r repositorio, hash string) (calculo, error) {
	hashBase, err := r.commitDaBase(hash)
	if err != nil {
		return calculo{}, err
	}
	base, err := r.baseEm(hashBase)
	if err != nil {
		return calculo{}, err
	}
	commits, err := r.commitsDepois(hashBase, hash)
	if err != nil {
		return calculo{}, err
	}
	return calculo{base: base, hashBase: hashBase, commits: commits, versoes: versoesDosCommits(base, commits)}, nil
}

func (c calculo) versao() numero {
	if len(c.versoes) == 0 {
		return c.base
	}
	return c.versoes[len(c.versoes)-1]
}

// entradasNaoPublicadas são os arquivos que ainda estão em changelog.d/, cada um com a versão do merge que o trouxe
// (a da base, se veio antes dela), da mais nova para a mais antiga.
func (c calculo) entradasNaoPublicadas(r repositorio, ref string) ([]entrada, error) {
	arquivos, err := r.arquivosDoChangelog(ref)
	if err != nil {
		return nil, err
	}
	if len(arquivos) == 0 {
		return nil, nil
	}
	autores, err := r.quemAcrescentou(ref)
	if err != nil {
		return nil, err
	}
	dataDaBase, err := r.dataDe(c.hashBase)
	if err != nil {
		return nil, err
	}
	posicao := map[string]int{}
	for i, cm := range c.commits {
		posicao[cm.hash] = i
	}
	entradas := make([]entrada, 0, len(arquivos))
	for _, arquivo := range arquivos {
		conteudo, err := r.conteudoEm(ref, arquivo)
		if err != nil {
			return nil, err
		}
		e := entrada{arquivo: arquivo, versao: c.base, data: dataDaBase, ordem: -1, conteudo: conteudo}
		if i, ok := posicao[autores[arquivo]]; ok {
			e.versao, e.data, e.ordem = c.versoes[i], c.commits[i].data, i
		}
		entradas = append(entradas, e)
	}
	slices.SortFunc(entradas, func(a, b entrada) int {
		if a.ordem != b.ordem {
			return b.ordem - a.ordem
		}
		return strings.Compare(a.arquivo, b.arquivo)
	})
	return entradas, nil
}
