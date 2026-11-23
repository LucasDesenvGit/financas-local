package main

import (
	"log/slog"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/config"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
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

// O acesso pelo celular usa a porta do site; endereço sem porta é erro ao montar, e não um site sem o acesso.
func TestNovoAcessoCelular(t *testing.T) {
	db := testeapoio.NovoBanco(t)
	log := slog.New(slog.DiscardHandler)
	for endereco, ok := range map[string]bool{"127.0.0.1:5443": true, "127.0.0.1": false, "127.0.0.1:porta": false} {
		_, err := novoAcessoCelular(db, relogio.Fixo{Instante: testeapoio.InstanteDosTestes}, log,
			servidor.Opcoes{Config: config.Config{Endereco: endereco, PastaChaves: t.TempDir()}, Log: log})
		if (err == nil) != ok {
			t.Errorf("%s: %v", endereco, err)
		}
	}
}
