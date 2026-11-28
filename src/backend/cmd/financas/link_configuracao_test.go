package main

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/autenticacao"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/celular"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
)

// Regra (#68, item 4): o link do primeiro acesso leva um token que o site aceita, gerado com o segredo da pasta das
// chaves; sem a pasta ou sem o arquivo (o site ainda não subiu), erro, e o subcomando nunca cria o segredo.
func TestLinkDeConfiguracao(t *testing.T) {
	r := relogio.Fixo{Instante: inicioDoContrato}
	pasta := t.TempDir()
	segredo, err := celular.CarregarChaveDoGerenciador(pasta)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("com o segredo", func(t *testing.T) {
		var saida strings.Builder
		if err := linkDeConfiguracao([]string{"-chaves", pasta, "-url", "https://127.0.0.1:5443/"}, &saida, r); err != nil {
			t.Fatal(err)
		}
		link := strings.TrimSpace(saida.String())
		const inicio = "https://127.0.0.1:5443/entrar?configurar="
		if !strings.HasPrefix(link, inicio) {
			t.Fatalf("link = %q", link)
		}
		if !autenticacao.TokenDeConfiguracaoValido(segredo, strings.TrimPrefix(link, inicio), r.Agora()) {
			t.Errorf("o site não aceitaria o token de %q", link)
		}
	})
	t.Run("sem informar a pasta", func(t *testing.T) {
		if err := linkDeConfiguracao(nil, &strings.Builder{}, r); err == nil || !strings.Contains(err.Error(), "-chaves") {
			t.Errorf("erro = %v", err)
		}
	})
	t.Run("sem o arquivo, não cria", func(t *testing.T) {
		vazia := t.TempDir()
		err := linkDeConfiguracao([]string{"-chaves", vazia}, &strings.Builder{}, r)
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("erro = %v, esperado arquivo inexistente", err)
		}
		if _, err := celular.LerChaveDoGerenciador(vazia); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("o subcomando criou o segredo: %v", err)
		}
	})
}
