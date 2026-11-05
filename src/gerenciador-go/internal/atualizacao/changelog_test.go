package atualizacao

import (
	"strings"
	"testing"
)

const changelogDeTeste = "# Changelog\r\n\r\nFormato…\r\n\r\n" +
	"## [2.46.0] — 2026-10-28 · desenvolvimento\n\n### Funcionalidades\n- **Develop** novo\n\n" +
	"## [2.45.0] — 2026-10-28 · desenvolvimento\n\n### Funcionalidades\n- **Versão nova** pelo GitHub\n\n" +
	"## [2.44.1] — 2026-10-28 · desenvolvimento\n\n### Correções\n- foco\n\n" +
	"## Notas antigas sem número\n- solta\n\n" +
	"## [2.44.0] — 2026-10-27 · desenvolvimento\n- antiga"

// Regra: de 2.44.1 para 2.46.0, as seções 2.45.0 e 2.46.0, na ordem do arquivo, e nada da 2.44.1 para trás.
func TestSecoesDoChangelog(t *testing.T) {
	t.Parallel()
	antes, _ := LerVersao("2.44.1")
	nova, _ := LerVersao("2.46.0")
	obtido := SecoesDoChangelog(changelogDeTeste, &antes, nova)
	quer := "## [2.46.0] — 2026-10-28 · desenvolvimento\n\n### Funcionalidades\n- **Develop** novo\n\n" +
		"## [2.45.0] — 2026-10-28 · desenvolvimento\n\n### Funcionalidades\n- **Versão nova** pelo GitHub"
	if obtido != quer {
		t.Errorf("obtido:\n%s", obtido)
	}
}

// Regra: a nova limita (o que vier acima dela não entra); sem a de antes, só a da versão nova; a de desenvolvimento
// conta pelo X.Y.Z.
func TestSecoesDoChangelogNasBordas(t *testing.T) {
	t.Parallel()
	nova, _ := LerVersao("2.45.0-dev.7")
	antes, _ := LerVersao("2.44.0")
	obtido := SecoesDoChangelog(changelogDeTeste, &antes, nova)
	if !strings.HasPrefix(obtido, "## [2.45.0]") || !strings.Contains(obtido, "## [2.44.1]") || strings.Contains(obtido, "2.46.0") ||
		strings.Contains(obtido, "solta") || strings.Contains(obtido, "antiga") {
		t.Errorf("de 2.44.0 para 2.45.0:\n%s", obtido)
	}
	if so := SecoesDoChangelog(changelogDeTeste, nil, nova); !strings.HasPrefix(so, "## [2.45.0]") || strings.Contains(so, "2.44.1") ||
		strings.Contains(so, "2.46.0") {
		t.Errorf("sem a de antes:\n%s", so)
	}
	mesma, _ := LerVersao("2.46.0")
	if nada := SecoesDoChangelog(changelogDeTeste, &mesma, mesma); nada != "" {
		t.Errorf("sem novidade: %q", nada)
	}
}

// Regra: a versão vem do <Version> do Directory.Build.props.
func TestVersaoDoProjeto(t *testing.T) {
	t.Parallel()
	if v, ok := VersaoDoProjeto("<Project>\n  <PropertyGroup>\n    <Version> 2.46.0 </Version>\n"); !ok || v.String() != "2.46.0" {
		t.Errorf("%v %v", v, ok)
	}
	for _, ruim := range []string{"<Project/>", "<Version></Version>", "<Version>abc</Version>"} {
		if _, ok := VersaoDoProjeto(ruim); ok {
			t.Errorf("%q lida", ruim)
		}
	}
}
