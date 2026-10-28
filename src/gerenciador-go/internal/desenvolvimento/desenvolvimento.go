// Package desenvolvimento é o "Atualizar para a última do develop" (ADR 0040), como o DesenvolvimentoPeloGit e o
// AtualizarPeloGitAsync do C#: só na instalação de desenvolvimento pelos scripts, com a pasta do projeto num clone
// git na branch develop. O git roda como o usuário (atualizar-windows.ps1 -SoCodigo); só a reinstalação pede o UAC.
package desenvolvimento

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
)

// Branch da instalação de desenvolvimento.
const Branch = "develop"

// Códigos de saída do atualizar.sh e do atualizar-windows.ps1.
const (
	Atualizado  = 0
	Recusado    = 2
	SemNovidade = 10
)

// Raiz é a pasta do projeto, se a instalação é pelos scripts e a pasta é um clone com o script de atualizar.
func Raiz(inst instalacao.Instalacao) string {
	if inst.PeloMsi || inst.PastaPrograma == "" {
		return ""
	}
	raiz := filepath.Clean(filepath.Join(inst.PastaPrograma, "..", ".."))
	if _, err := os.Stat(filepath.Join(raiz, ".git")); err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(raiz, "scripts", "geral", "atualizar.sh")); err != nil {
		return ""
	}
	return raiz
}

// BranchAtual é a branch do clone, pelo git ("" sem git ou fora de uma branch).
func BranchAtual(ctx context.Context, raiz string) string {
	ctx, cancelar := context.WithTimeout(ctx, 10*time.Second)
	defer cancelar()
	cmd := exec.CommandContext(ctx, "git", "-C", raiz, "rev-parse", "--abbrev-ref", "HEAD")
	semJanela(cmd)
	saida, err := cmd.Output()
	branch := strings.TrimSpace(string(saida))
	if err != nil || branch == "HEAD" {
		return ""
	}
	return branch
}

// Disponivel é a pasta do projeto, se dá para atualizar pelo develop (clone na branch develop, com git).
func Disponivel(ctx context.Context, inst instalacao.Instalacao) string {
	raiz := Raiz(inst)
	if raiz == "" || BranchAtual(ctx, raiz) != Branch {
		return ""
	}
	return raiz
}

// Log é o log do instalador, que a janela acompanha enquanto ele roda como administrador, sem janela.
func Log(raiz string) string { return filepath.Join(raiz, "instaladores", "instalacao-windows.log") }

var cores = regexp.MustCompile(`\x1B\[[0-9;]*[A-Za-z]`)

// RodarAoVivo roda o comando e entrega cada linha (saída e erro, sem as cores do terminal) enquanto ele roda.
// Devolve o código de saída; -1 se o programa não existe ou foi cancelado.
func RodarAoVivo(ctx context.Context, cmd *exec.Cmd, linha func(string)) int {
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdin = nil
	leitor, escritor := io.Pipe()
	cmd.Stdout, cmd.Stderr = escritor, escritor
	// Morto o programa, um filho dele (o git que o PowerShell abriu) pode segurar a saída aberta: espera no máximo
	// 2 s por ela.
	cmd.WaitDelay = 2 * time.Second
	semJanela(cmd)
	if err := cmd.Start(); err != nil {
		linha("O programa " + filepath.Base(cmd.Path) + " não foi encontrado neste computador.")
		return -1
	}
	lido := make(chan struct{})
	go func() {
		defer close(lido)
		s := bufio.NewScanner(leitor)
		s.Buffer(nil, 1<<20) // linhas de até 1 MB (o buffer começa pequeno e cresce)
		for s.Scan() {
			linha(cores.ReplaceAllString(strings.TrimRight(s.Text(), "\r"), ""))
		}
		_, _ = io.Copy(io.Discard, leitor) // linha comprida demais: descarta o resto, sem travar o programa
	}()
	espera := make(chan error, 1)
	go func() { espera <- cmd.Wait() }()
	var err error
	select {
	case err = <-espera:
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-espera
		_ = escritor.Close()
		<-lido
		return -1
	}
	_ = escritor.Close()
	<-lido // espera as últimas linhas chegarem
	var saida *exec.ExitError
	if errors.As(err, &saida) {
		return saida.ExitCode()
	}
	if err != nil {
		return -1
	}
	return 0
}

// AcompanharLog entrega as linhas novas do log enquanto o instalador roda, de segundo em segundo; quando o ctx
// termina, entrega o que faltou (inclusive a última linha sem quebra) e volta.
func AcompanharLog(ctx context.Context, caminho string, linha func(string), intervalo time.Duration) {
	var lidos int64
	if info, err := os.Stat(caminho); err == nil {
		lidos = info.Size()
	}
	resto := ""
	for {
		fim := ctx.Err() != nil
		lidos, resto = lerNovas(caminho, lidos, resto, linha)
		if fim {
			if resto != "" {
				linha(resto)
			}
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(intervalo):
		}
	}
}

// lerNovas lê o que o log ganhou desde lidos; a linha sem quebra no fim fica para a próxima vez.
func lerNovas(caminho string, lidos int64, resto string, linha func(string)) (int64, string) {
	arquivo, err := os.Open(caminho)
	if err != nil {
		return lidos, resto
	}
	defer arquivo.Close()
	if _, err := arquivo.Seek(lidos, io.SeekStart); err != nil {
		return lidos, resto
	}
	novo, err := io.ReadAll(arquivo)
	if err != nil {
		return lidos, resto
	}
	partes := strings.Split(resto+string(novo), "\n")
	for _, l := range partes[:len(partes)-1] {
		linha(strings.TrimRight(l, "\r"))
	}
	return lidos + int64(len(novo)), partes[len(partes)-1]
}
