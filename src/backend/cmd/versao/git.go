package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// acharRaiz sobe, a partir de inicio, até a pasta que tem o Directory.Build.props: a raiz do projeto, esteja ela num
// clone ou numa cópia baixada em .zip.
func acharRaiz(inicio string) (string, error) {
	pasta, err := filepath.Abs(inicio)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(pasta, arquivoDaBase)); err == nil {
			return pasta, nil
		}
		pai := filepath.Dir(pasta)
		if pai == pasta {
			return "", fmt.Errorf("%s não encontrado a partir de %s", arquivoDaBase, inicio)
		}
		pasta = pai
	}
}

// repositorio é o clone git da raiz do projeto. O git roda com -C na raiz, para valer de qualquer pasta.
type repositorio struct{ raiz string }

// git roda um comando e devolve a saída, sem a quebra de linha do fim.
func (r repositorio) git(argumentos ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", r.raiz, "-c", "core.quotepath=off"}, argumentos...)...)
	var saida, erro bytes.Buffer
	cmd.Stdout, cmd.Stderr = &saida, &erro
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(argumentos, " "), err, strings.TrimSpace(erro.String()))
	}
	return strings.TrimRight(saida.String(), "\r\n"), nil
}

// temHistorico: a raiz é um clone (tem .git, pasta ou arquivo de worktree), o git existe e o clone não é raso. Sem
// isso o número não se calcula, e a versão sai como a base com o sufixo -sem-git.
func temHistorico(raiz string) bool {
	if _, err := os.Stat(filepath.Join(raiz, ".git")); err != nil {
		return false
	}
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	raso, err := repositorio{raiz}.git("rev-parse", "--is-shallow-repository")
	return err == nil && raso == "false"
}

// resolver devolve o hash do commit de uma referência (HEAD, um hash, HEAD^2...).
func (r repositorio) resolver(ref string) (string, error) {
	hash, err := r.git("rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("referência %q não existe neste repositório: %w", ref, err)
	}
	return hash, nil
}

// segundoPai é o segundo pai do commit, ou "" se ele não é um merge.
func (r repositorio) segundoPai(hash string) string {
	pai, err := r.git("rev-parse", "--verify", "--quiet", hash+"^2")
	if err != nil {
		return ""
	}
	return pai
}

// commitDaBase é o último commit da linha principal (primeiro pai) em que o valor do <VersaoBase> mudou (ADR 0070,
// regra 1). Mexer só no comentário do arquivo não conta: o -G olha só as linhas do elemento.
func (r repositorio) commitDaBase(ref string) (string, error) {
	hash, err := r.git("log", "--first-parent", "-m", "-1", "--format=%H",
		"-G<"+elementoBase+">[^<]*</"+elementoBase+">", ref, "--", arquivoDaBase)
	if err != nil {
		return "", err
	}
	if hash == "" {
		return "", fmt.Errorf("nenhum commit de %s muda o <%s> na história de %s", arquivoDaBase, elementoBase, ref)
	}
	return hash, nil
}

// baseEm lê o <VersaoBase> do arquivo, no commit.
func (r repositorio) baseEm(hash string) (numero, error) {
	props, err := r.git("show", hash+":"+arquivoDaBase)
	if err != nil {
		return numero{}, err
	}
	return lerBase(props)
}

const (
	separadorDeCampo  = "\x1f"
	separadorDeCommit = "\x1e"
)

// commitsDepois lista os commits da linha principal depois de base, até ref, do mais antigo ao mais novo.
func (r repositorio) commitsDepois(base, ref string) ([]commit, error) {
	saida, err := r.git("log", "--first-parent", "--reverse",
		"--format=%H"+separadorDeCampo+"%P"+separadorDeCampo+"%s"+separadorDeCampo+"%cs"+separadorDeCampo+"%b"+separadorDeCommit,
		base+".."+ref)
	if err != nil {
		return nil, err
	}
	var commits []commit
	for registro := range strings.SplitSeq(saida, separadorDeCommit) {
		registro = strings.TrimLeft(registro, "\r\n")
		if registro == "" {
			continue
		}
		campos := strings.SplitN(registro, separadorDeCampo, 5)
		if len(campos) != 5 {
			return nil, fmt.Errorf("linha do git log fora do formato: %q", registro)
		}
		commits = append(commits, commit{hash: campos[0], pais: len(strings.Fields(campos[1])), assunto: campos[2],
			data: campos[3], corpo: strings.TrimSpace(campos[4])})
	}
	return commits, nil
}

// dataDe é a data (AAAA-MM-DD) do commit.
func (r repositorio) dataDe(hash string) (string, error) {
	return r.git("log", "-1", "--format=%cs", hash)
}

// arquivosDoChangelog são os arquivos de entrada de changelog.d/ que existem em ref.
func (r repositorio) arquivosDoChangelog(ref string) ([]string, error) {
	saida, err := r.git("ls-tree", "-r", "--name-only", ref, "--", pastaDoChangelog+"/")
	if err != nil {
		return nil, err
	}
	var arquivos []string
	for linha := range strings.SplitSeq(saida, "\n") {
		if ehEntradaDoChangelog(linha) {
			arquivos = append(arquivos, linha)
		}
	}
	return arquivos, nil
}

// quemAcrescentou diz, para cada arquivo de changelog.d/, o commit da linha principal que o acrescentou por último
// (o merge do PR que o trouxe).
func (r repositorio) quemAcrescentou(ref string) (map[string]string, error) {
	acrescimos, err := r.acrescimos(ref)
	if err != nil {
		return nil, err
	}
	autores := map[string]string{}
	for _, a := range acrescimos {
		for _, arquivo := range a.arquivos {
			if _, jaTem := autores[arquivo]; !jaTem { // do mais novo para o mais antigo: vale o primeiro
				autores[arquivo] = a.hash
			}
		}
	}
	return autores, nil
}

// acrescimo é um commit da linha principal que acrescentou entradas a changelog.d/, com elas.
type acrescimo struct {
	hash     string
	arquivos []string
}

// acrescimos lista, do mais novo para o mais antigo, os commits da linha principal da faixa (um ref ou "a..b") que
// acrescentaram entradas a changelog.d/ (o LEIA-ME.md não conta).
func (r repositorio) acrescimos(faixa string) ([]acrescimo, error) {
	saida, err := r.git("log", "--first-parent", "-m", "--diff-filter=A", "--name-only",
		"--format="+separadorDeCommit+"%H", faixa, "--", pastaDoChangelog+"/")
	if err != nil {
		return nil, err
	}
	var lista []acrescimo
	for registro := range strings.SplitSeq(saida, separadorDeCommit) {
		linhas := strings.Split(strings.TrimSpace(registro), "\n")
		if linhas[0] == "" {
			continue
		}
		a := acrescimo{hash: linhas[0]}
		for _, arquivo := range linhas[1:] {
			if arquivo = strings.TrimSpace(arquivo); ehEntradaDoChangelog(arquivo) {
				a.arquivos = append(a.arquivos, arquivo)
			}
		}
		if len(a.arquivos) > 0 {
			lista = append(lista, a)
		}
	}
	return lista, nil
}

// conteudoEm lê um arquivo no commit.
func (r repositorio) conteudoEm(hash, arquivo string) (string, error) {
	return r.git("show", hash+":"+arquivo)
}

var errSemHistorico = errors.New("precisa do histórico do git (um clone completo)")
