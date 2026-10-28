package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/atualizacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

func cloneComVersao(t *testing.T, versao, changelog string) string {
	t.Helper()
	raiz := t.TempDir()
	if versao != "" {
		props := "<Project><PropertyGroup><Version>" + versao + "</Version></PropertyGroup></Project>"
		if err := os.WriteFile(filepath.Join(raiz, "Directory.Build.props"), []byte(props), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if changelog != "" {
		if err := os.WriteFile(filepath.Join(raiz, "CHANGELOG.md"), []byte(changelog), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return raiz
}

const changelogDoClone = "# Changelog\n\n## [2.46.0] — 2026-10-28 · d\n### Funcionalidades\n- nova\n\n## [2.45.0] — 2026-10-28 · d\n- outra\n\n## [2.44.1] — 2026-10-28 · d\n- antiga"

// Regra: depois de trazer o develop, "O que mudou desde a <de antes>" com as seções do CHANGELOG.md do clone, e o
// link para o CHANGELOG no GitHub.
func TestNotasDoDevelop(t *testing.T) {
	t.Parallel()
	raiz := cloneComVersao(t, "2.46.0", changelogDoClone)
	antes, _ := atualizacao.LerVersao("2.44.1")
	n := notasDoDevelop(raiz, antes, true)
	if n == nil || n.Titulo != "O que mudou desde a 2.44.1" || !strings.Contains(string(n.Novidades), "nova") ||
		!strings.Contains(string(n.Novidades), "outra") || strings.Contains(string(n.Novidades), "antiga") ||
		n.Pagina != "https://github.com/LucasDesenvGit/financas-local/blob/develop/CHANGELOG.md" || n.TextoPagina != "Ver o CHANGELOG no GitHub" {
		t.Errorf("%+v", n)
	}
	// Sem a versão de antes: só a da versão nova.
	if so := notasDoDevelop(raiz, atualizacao.Versao{}, false); so == nil || so.Titulo != "O que mudou na versão 2.46.0" ||
		strings.Contains(string(so.Novidades), "outra") {
		t.Errorf("sem a de antes: %+v", so)
	}
}

// Regra: sem versão nova, sem CHANGELOG, sem Directory.Build.props ou sem seção, não há "O que mudou".
func TestNotasDoDevelopSemNada(t *testing.T) {
	t.Parallel()
	mesma, _ := atualizacao.LerVersao("2.46.0")
	casos := map[string]string{
		"sem novidade":  cloneComVersao(t, "2.46.0", changelogDoClone),
		"sem changelog": cloneComVersao(t, "2.46.0", ""),
		"sem props":     cloneComVersao(t, "", changelogDoClone),
	}
	for nome, raiz := range casos {
		if n := notasDoDevelop(raiz, mesma, true); n != nil {
			t.Errorf("%s: %+v", nome, n)
		}
	}
}

// Regra: a tela fica sabendo que há "O que mudou" (o botão só aparece com as notas prontas).
func TestDevelopNaTela(t *testing.T) {
	t.Parallel()
	g := &gerenciador{dev: develop{raiz: `C:\projeto`, rodando: true, saida: []string{"a", "b"}, notas: &tela.Notas{}}}
	if d := g.developNaTela(); !d.Disponivel || !d.Rodando || d.Saida != "a\nb" || !d.Notas {
		t.Errorf("%+v", d)
	}
	if d := (&gerenciador{}).developNaTela(); d.Disponivel || d.Notas {
		t.Errorf("sem clone: %+v", d)
	}
}
