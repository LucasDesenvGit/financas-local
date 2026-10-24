package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/config"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/saude"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/servidor"
)

const segredo = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

// inicioDoContrato é o InicioPadrao do testes/contrato: em Brasília ainda é 14/10, 22h.
var inicioDoContrato = time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)

// bancoComUsuario cria, como o EsquemaBanco do .NET faria, um banco em WAL com a estrutura e um usuário.
func bancoComUsuario(t *testing.T) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "FinancasLocal.db")
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	estrutura, err := os.ReadFile("../../../../banco/01-estrutura.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, comando := range []string{
		"PRAGMA journal_mode=WAL",
		string(estrutura),
		`INSERT INTO "Usuario" ("Login", "SenhaHash", "CarimboSeguranca", "CriadoEm")
		 VALUES ('verificador', 'x', '6F1C2B9E-7A43-4D2E-9B8F-3C5A1E0D4F27', '2026-10-23 12:00:00')`,
	} {
		if _, err := db.Exec(comando); err != nil {
			t.Fatalf("%s: %v", comando[:min(len(comando), 40)], err)
		}
	}
	return caminho
}

// O caminho todo do lado do Go: segredo pela entrada padrão, porta pela saída padrão, GET /api/go/saude lendo o
// banco, e parada quando a entrada fecha (é o que acontece quando o .NET sai, mesmo morto à força).
func TestAtrasDoDotnetDoComecoAoFim(t *testing.T) {
	caminho := bancoComUsuario(t)
	entradaLeitor, entradaEscritor := io.Pipe()
	saidaLeitor, saidaEscritor := io.Pipe()
	terminou := make(chan error, 1)
	go func() {
		cfg := config.Config{BancoCaminho: caminho}
		terminou <- rodarAtrasDoDotnet(context.Background(), cfg, relogio.NovoAPartirDe(inicioDoContrato), slog.New(slog.DiscardHandler), entradaLeitor, saidaEscritor)
	}()

	if _, err := io.WriteString(entradaEscritor, segredo+"\n"); err != nil {
		t.Fatal(err)
	}
	linhas := make(chan string, 1)
	go func() {
		linha, _ := bufio.NewReader(saidaLeitor).ReadString('\n')
		linhas <- linha
	}()
	var aviso struct{ Porta int }
	select {
	case linha := <-linhas:
		if err := json.Unmarshal([]byte(linha), &aviso); err != nil || aviso.Porta <= 0 {
			t.Fatalf("a primeira linha da saída devia ser {\"porta\":N}, veio %q", linha)
		}
	case err := <-terminou:
		t.Fatalf("parou antes de avisar a porta: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("não avisou a porta em 10 s")
	}

	url := "http://127.0.0.1:" + strconv.Itoa(aviso.Porta) + "/api/go/saude"
	pedido, _ := http.NewRequest(http.MethodGet, url, nil)
	pedido.Header.Set(servidor.CabecalhoSegredo, segredo)
	pedido.Header.Set(servidor.CabecalhoUsuario, "1")
	resposta, err := http.DefaultClient.Do(pedido)
	if err != nil {
		t.Fatal(err)
	}
	var corpo saude.Resposta
	if err := json.NewDecoder(resposta.Body).Decode(&corpo); err != nil {
		t.Fatal(err)
	}
	resposta.Body.Close()
	if resposta.StatusCode != http.StatusOK || corpo.Situacao != "ok" || corpo.Usuario != "verificador" || corpo.DiarioDoBanco != "wal" {
		t.Fatalf("status %d, corpo %+v", resposta.StatusCode, corpo)
	}
	// O relógio de teste chega até a rota: a hora é a do início (mais o que passou), e não a do computador.
	if passou := corpo.Agora.Tempo().Sub(inicioDoContrato); passou < 0 || passou > 10*time.Second || corpo.Hoje.String() != "2026-10-14" {
		t.Fatalf("relógio: agora %s, hoje %s; esperado logo depois de %s, hoje 2026-10-14 (Brasília)", corpo.Agora, corpo.Hoje, inicioDoContrato)
	}

	// Sem o segredo, 401: outro programa do computador não fala direto com o Go.
	semSegredo, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	semSegredo.Body.Close()
	if semSegredo.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sem o segredo: status %d, esperado 401", semSegredo.StatusCode)
	}

	entradaEscritor.Close() // o .NET saiu
	select {
	case err := <-terminou:
		if err != nil {
			t.Fatalf("parou com erro: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("o Go não parou em 15 s depois de a entrada padrão fechar")
	}
}

func TestAtrasDoDotnetSemSegredoNaoSobe(t *testing.T) {
	cfg := config.Config{BancoCaminho: bancoComUsuario(t)}
	err := rodarAtrasDoDotnet(context.Background(), cfg, relogio.Sistema{}, slog.New(slog.DiscardHandler), strings.NewReader(""), io.Discard)
	if err == nil {
		t.Fatal("subiu sem o segredo interno")
	}
}

// O Go não cria banco: um caminho errado viraria um banco vazio, e o site mostraria tudo zerado.
func TestAtrasDoDotnetSemBancoNaoSobe(t *testing.T) {
	entrada, escritor := io.Pipe()
	go func() { _, _ = io.WriteString(escritor, segredo+"\n") }()
	cfg := config.Config{BancoCaminho: filepath.Join(t.TempDir(), "nao-existe.db")}
	err := rodarAtrasDoDotnet(context.Background(), cfg, relogio.Sistema{}, slog.New(slog.DiscardHandler), entrada, io.Discard)
	if err == nil {
		t.Fatal("subiu sem o banco")
	}
	if _, errStat := os.Stat(cfg.BancoCaminho); errStat == nil {
		t.Fatal("criou um banco vazio")
	}
	escritor.Close()
}

// Sem a opção, o relógio do sistema, como antes da #363; com ela, o relógio de teste; texto inválido não sobe.
func TestEscolherRelogio(t *testing.T) {
	casos := []struct {
		nome   string
		inicio string
		tipo   string // "sistema", "a partir de" ou "erro"
	}{
		{"sem a opção", "", "sistema"},
		{"instante em UTC", "2026-10-15T01:00:00Z", "a partir de"},
		{"instante com fuso (o ToString(\"O\") do .NET)", "2026-10-14T22:00:00.0000000-03:00", "a partir de"},
		{"sem fuso", "2026-10-15T01:00:00", "erro"},
		{"texto qualquer", "-banco /tmp/outro.db", "erro"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r, err := escolherRelogio(c.inicio, slog.New(slog.DiscardHandler))
			switch c.tipo {
			case "erro":
				if err == nil {
					t.Fatalf("aceitou %q", c.inicio)
				}
			case "sistema":
				if _, ok := r.(relogio.Sistema); err != nil || !ok {
					t.Fatalf("esperado o relógio do sistema, veio %T (%v)", r, err)
				}
			case "a partir de":
				if err != nil {
					t.Fatal(err)
				}
				if passou := r.Agora().Sub(inicioDoContrato); passou < 0 || passou > 5*time.Second {
					t.Fatalf("o relógio marca %s, esperado logo depois de %s", r.Agora(), inicioDoContrato)
				}
			}
		})
	}
}
