package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
)

// inicioDoContrato é o InicioPadrao do testes/contrato: em Brasília ainda é 14/10, 22h.
var inicioDoContrato = time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)

// Sem a opção, o relógio do sistema, como antes da #363; com ela, o relógio de teste; texto inválido não sobe.
func TestNovoRelogio(t *testing.T) {
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
			r, _, err := novoRelogio(c.inicio)
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

// Regra (#84): gerar-certificado lê a senha da entrada padrão, e não da linha de comando, e diz o que fez. O PowerShell manda
// \r\n no fim, e o printf do bash não manda nada. O pipe do PowerShell 5.1 também pode começar pela marca do UTF-8: lida como
// parte da senha, ela trocaria o certificado a cada atualização.
func TestGerarCertificado(t *testing.T) {
	pfx := filepath.Join(t.TempDir(), "financaslocal.pfx")
	casos := []struct{ nome, entrada, saida string }{
		{"gera", "senha\n", "gerado\n"},
		{"reaproveita, com o fim de linha do PowerShell", "senha\r\n", "reaproveitado\n"},
		{"reaproveita, sem fim de linha", "senha", "reaproveitado\n"},
		{"reaproveita, com a marca do UTF-8", "\ufeffsenha\r\n", "reaproveitado\n"},
		{"só a primeira linha é a senha", "senha\noutra\n", "reaproveitado\n"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var saida strings.Builder
			if err := gerarCertificado([]string{"-pfx", pfx}, strings.NewReader(c.entrada), &saida, relogio.Fixo{Instante: inicioDoContrato}); err != nil {
				t.Fatal(err)
			}
			if saida.String() != c.saida {
				t.Errorf("saída = %q, esperada %q", saida.String(), c.saida)
			}
		})
	}
	if _, err := servidor.CarregarPFX(pfx, "senha"); err != nil {
		t.Error(err)
	}
}

func TestGerarCertificadoRecusa(t *testing.T) {
	pfx := filepath.Join(t.TempDir(), "a.pfx")
	casos := []struct {
		nome         string
		argumentos   []string
		entrada      string
		trechoDoErro string
	}{
		{"sem -pfx", nil, "senha\n", "informe -pfx"},
		{"sem senha", []string{"-pfx", pfx}, "", "entrada padrão"},
		{"linha em branco", []string{"-pfx", pfx}, "\r\n", "entrada padrão"},
		{"só a marca do UTF-8", []string{"-pfx", pfx}, "\ufeff\r\n", "entrada padrão"},
		{"opção desconhecida", []string{"-senha", "x"}, "", "senha"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var saida strings.Builder
			err := gerarCertificado(c.argumentos, strings.NewReader(c.entrada), &saida, relogio.Fixo{Instante: inicioDoContrato})
			if err == nil || !strings.Contains(err.Error(), c.trechoDoErro) {
				t.Errorf("erro = %v", err)
			}
			if saida.Len() != 0 {
				t.Errorf("saída = %q", saida.String())
			}
		})
	}
	if _, err := os.Stat(pfx); err == nil {
		t.Error("gravou o .pfx mesmo recusando")
	}
}
