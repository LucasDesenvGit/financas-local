package main

import (
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/atualizacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/tela"
)

// O que o `go run ./cmd/versao -changelog` imprime (uma seção por versão, a mais nova primeiro), seguido do CHANGELOG.md
// do clone, que ainda tem as seções de antes da troca (ADR 0070).
const changelogDoClone = "## [2.46.0] — 2026-10-28 · d\n### Funcionalidades\n- nova\n\n## [2.45.0] — 2026-10-28 · d\n- outra\n\n" +
	"# Changelog\n\n## [2.44.1] — 2026-10-28 · d\n- antiga\n\n## [2.44.0] — 2026-10-27\n- mais antiga"

func lerVersao(t *testing.T, texto string) atualizacao.Versao {
	t.Helper()
	v, ok := atualizacao.LerVersao(texto)
	if !ok {
		t.Fatalf("versão %q", texto)
	}
	return v
}

// Regra: depois de trazer o develop, "O que mudou desde a <de antes>" com as seções mais novas que a versão do site
// instalado, e o link para o CHANGELOG no GitHub.
func TestNotasDoDevelop(t *testing.T) {
	t.Parallel()
	n := notasDoDevelop(lerVersao(t, "2.46.0"), true, changelogDoClone, lerVersao(t, "2.44.1"), true)
	if n == nil || n.Titulo != "O que mudou desde a 2.44.1" || !strings.Contains(string(n.Novidades), "nova") ||
		!strings.Contains(string(n.Novidades), "outra") || strings.Contains(string(n.Novidades), "antiga") ||
		n.Pagina != "https://github.com/LucasDesenvGit/financas-local/blob/develop/CHANGELOG.md" || n.TextoPagina != "Ver o CHANGELOG no GitHub" {
		t.Errorf("%+v", n)
	}
	// Sem a versão de antes: só a da versão nova.
	if so := notasDoDevelop(lerVersao(t, "2.46.0"), true, changelogDoClone, atualizacao.Versao{}, false); so == nil ||
		so.Titulo != "O que mudou na versão 2.46.0" || strings.Contains(string(so.Novidades), "outra") {
		t.Errorf("sem a de antes: %+v", so)
	}
	// Quem atualiza de uma versão sem git (a base e o sufixo -sem-git) conta pelo X.Y.Z.
	semGit := notasDoDevelop(lerVersao(t, "2.46.0"), true, changelogDoClone, lerVersao(t, "2.45.0-sem-git"), true)
	if semGit == nil || semGit.Titulo != "O que mudou desde a 2.45.0" || !strings.Contains(string(semGit.Novidades), "nova") ||
		strings.Contains(string(semGit.Novidades), "outra") {
		t.Errorf("de uma sem-git: %+v", semGit)
	}
}

// Regra: do CHANGELOG.md do clone vai só o que começa na primeira versão, para o título e a explicação do formato não
// ficarem grudados na última seção que veio do -changelog.
func TestDesdeAPrimeiraVersao(t *testing.T) {
	t.Parallel()
	casos := map[string]string{
		"com o título": "# Changelog\n\nFormato.\n\n## [2.0.0] — x\n- a\n\n## [1.0.0] — y\n- b\n",
		"sem o título": "## [2.0.0] — x\n- a\n\n## [1.0.0] — y\n- b\n",
		"com CRLF":     "# Changelog\r\n\r\n## [2.0.0] — x\r\n- a\r\n\r\n## [1.0.0] — y\r\n- b\r\n",
	}
	for nome, texto := range casos {
		if obtido := desdeAPrimeiraVersao(texto); obtido != "## [2.0.0] — x\n- a\n\n## [1.0.0] — y\n- b\n" {
			t.Errorf("%s: %q", nome, obtido)
		}
	}
	if obtido := desdeAPrimeiraVersao("# Changelog\n\nainda sem versão\n"); obtido != "" {
		t.Errorf("sem versão nenhuma: %q", obtido)
	}
}

// Regra: sem versão nova, sem novidade ou sem seção, não há "O que mudou".
func TestNotasDoDevelopSemNada(t *testing.T) {
	t.Parallel()
	mesma := lerVersao(t, "2.46.0")
	casos := map[string]*tela.Notas{
		"sem novidade":        notasDoDevelop(mesma, true, changelogDoClone, mesma, true),
		"sem changelog":       notasDoDevelop(mesma, true, "", lerVersao(t, "2.40.0"), true),
		"sem versão nova":     notasDoDevelop(atualizacao.Versao{}, false, changelogDoClone, lerVersao(t, "2.40.0"), true),
		"sem seção da nova":   notasDoDevelop(lerVersao(t, "2.50.0"), true, "# Changelog\n", atualizacao.Versao{}, false),
		"só de antes da nova": notasDoDevelop(lerVersao(t, "2.47.0"), true, changelogDoClone, lerVersao(t, "2.46.0"), true),
	}
	for nome, n := range casos {
		if n != nil {
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
