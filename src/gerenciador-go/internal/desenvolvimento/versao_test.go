package desenvolvimento

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func escrever(t *testing.T, caminho, conteudo string, modo os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(caminho), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminho, []byte(conteudo), modo); err != nil {
		t.Fatal(err)
	}
}

// Regra (ADR 0070): o Go que calcula a versão é o do projeto (ferramentas/go/bin, que a instalação baixa) e, sem ele, o do PATH.
func TestGo(t *testing.T) {
	nome := "go"
	if runtime.GOOS == "windows" {
		nome = "go.exe"
	}
	t.Run("o do projeto vence o do PATH", func(t *testing.T) {
		raiz := t.TempDir()
		do := filepath.Join(raiz, "ferramentas", "go", "bin", nome)
		escrever(t, do, "", 0o700)
		if obtido, doProjeto := Go(raiz); obtido != do || !doProjeto {
			t.Errorf("obtido %q (do projeto: %v), quer %q", obtido, doProjeto, do)
		}
	})
	t.Run("sem o do projeto, o do PATH", func(t *testing.T) {
		bin := t.TempDir()
		noPath := filepath.Join(bin, nome)
		escrever(t, noPath, "", 0o700)
		t.Setenv("PATH", bin)
		if obtido, doProjeto := Go(t.TempDir()); obtido != noPath || doProjeto {
			t.Errorf("obtido %q (do projeto: %v), quer %q", obtido, doProjeto, noPath)
		}
	})
	t.Run("sem nenhum", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if obtido, _ := Go(t.TempDir()); obtido != "" {
			t.Errorf("obtido %q, quer vazio", obtido)
		}
		if _, err := CalcularVersao(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "o Go não foi encontrado") {
			t.Errorf("erro: %v", err)
		}
	})
}

// cloneComCalculo monta um clone mínimo: src/backend com um cmd/versao de mentira, que imprime os argumentos e o
// GOTOOLCHAIN, e sai com erro se o primeiro argumento for "falha".
func cloneComCalculo(t *testing.T) string {
	t.Helper()
	raiz := t.TempDir()
	backend := filepath.Join(raiz, "src", "backend")
	escrever(t, filepath.Join(backend, "go.mod"), "module versaodeteste\n\ngo 1.21\n", 0o600)
	escrever(t, filepath.Join(backend, "cmd", "versao", "main.go"), `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "falha" {
		fmt.Fprintln(os.Stderr, "sem o histórico")
		os.Exit(1)
	}
	fmt.Println(strings.Join(os.Args[1:], " ") + "|" + os.Getenv("GOTOOLCHAIN"))
}
`, 0o600)
	return raiz
}

// Regra: o gerenciador roda "go run ./cmd/versao" em src/backend do clone, com o Go local (nunca baixa outro) e devolve a
// saída; o erro traz o que o cálculo disse.
func TestCalcularVersao(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("sem o go neste computador")
	}
	raiz := cloneComCalculo(t)
	if obtido, err := CalcularVersao(context.Background(), raiz); err != nil || obtido != "|local" {
		t.Errorf("sem argumento: %q, %v", obtido, err)
	}
	if obtido, err := CalcularVersao(context.Background(), raiz, "-changelog"); err != nil || obtido != "-changelog|local" {
		t.Errorf("-changelog: %q, %v", obtido, err)
	}
	if _, err := CalcularVersao(context.Background(), raiz, "falha"); err == nil || !strings.Contains(err.Error(), "sem o histórico") {
		t.Errorf("erro do cálculo: %v", err)
	}
}

// Regra: com o Go do projeto, o ambiente é o da instalação (módulos e cache em ferramentas/), como nos scripts.
func TestCalcularVersaoComOGoDoProjeto(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("o go de mentira é um script de shell")
	}
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, "src", "backend", "marca"), "", 0o600)
	escrever(t, filepath.Join(raiz, "ferramentas", "go", "bin", "go"),
		"#!/bin/sh\necho \"$@ cache=$GOCACHE caminho=$GOPATH flags=$GOFLAGS cwd=$(pwd)\"\n", 0o700)
	obtido, err := CalcularVersao(context.Background(), raiz, "-ref", "abc")
	if err != nil {
		t.Fatal(err)
	}
	cwd, _ := filepath.EvalSymlinks(filepath.Join(raiz, "src", "backend"))
	for _, quer := range []string{
		"run ./cmd/versao -ref abc",
		"cache=" + filepath.Join(raiz, "ferramentas", "go-cache"),
		"caminho=" + filepath.Join(raiz, "ferramentas", "go-caminho"),
		"flags=-modcacherw",
		"cwd=" + cwd,
	} {
		if !strings.Contains(obtido, quer) {
			t.Errorf("saída %q sem %q", obtido, quer)
		}
	}
}
