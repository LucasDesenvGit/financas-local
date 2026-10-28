package atualizacao

import (
	"strings"
	"testing"
)

func versao(t *testing.T, texto string) *Versao {
	t.Helper()
	v, ok := LerVersao(texto)
	if !ok {
		t.Fatal(texto)
	}
	return &v
}

const notasComSecoes = `## Como instalar
- baixe o .msi

## O que mudou
### 2.45.0 — 2026-10-29 · desenvolvimento
#### Funcionalidades
- nova A
### 2.44.1 — 2026-10-28 · desenvolvimento
- correção B
### 2.44.0 — 2026-10-27 · desenvolvimento
- antiga C`

// Regra: as novidades são as seções mais novas que a instalada, pelo X.Y.Z; o resto fica atrás de "Ver tudo".
func TestSepararPorSecao(t *testing.T) {
	t.Parallel()
	n := SepararNotas(notasComSecoes, versao(t, "2.44.0-dev.3"))
	if n.QuantasVersoesNovas != 2 || !strings.HasPrefix(n.Novidades, "## O que mudou desde a sua versão (2.44.0-dev.3)\n\n### 2.45.0") ||
		!strings.Contains(n.Novidades, "- correção B") || strings.Contains(n.Novidades, "antiga C") {
		t.Errorf("novidades:\n%s", n.Novidades)
	}
	if !n.TemResto() || !strings.Contains(n.Resto, "## Como instalar") || !strings.Contains(n.Resto, "### 2.44.0 — ") ||
		!strings.Contains(n.Resto, "antiga C") ||
		strings.Contains(n.Resto, "nova A") {
		t.Errorf("resto:\n%s", n.Resto)
	}
}

// Regra: sem saber a instalada, só a seção mais nova, com o título sem versão.
func TestSepararSemInstalada(t *testing.T) {
	t.Parallel()
	n := SepararNotas(strings.ReplaceAll(notasComSecoes, "\n", "\r\n"), nil)
	if n.QuantasVersoesNovas != 1 || !strings.HasPrefix(n.Novidades, "## O que mudou\n\n### 2.45.0") || strings.Contains(n.Novidades, "correção B") {
		t.Errorf("%+v", n)
	}
}

const notasComBlocos = `Notas antigas
<!-- por-versao:inicio -->
### 2.10.0-dev.5 · 20/10/2026
- PR cinco
### 2.10.0-dev.4 · 19/10/2026
- PR quatro
<!-- por-versao:fim -->
### 2.10.0-dev.9 · fora dos marcadores`

// Regra: notas antigas sem seção de versão: os blocos de PRs mais novos que a instalada, só dentro dos marcadores.
func TestSepararPorBloco(t *testing.T) {
	t.Parallel()
	n := SepararNotas(notasComBlocos, versao(t, "2.10.0-dev.4"))
	if n.QuantasVersoesNovas != 1 || !strings.Contains(n.Novidades, "PR cinco") || strings.Contains(n.Novidades, "PR quatro") ||
		strings.Contains(n.Novidades, "fora dos marcadores") {
		t.Errorf("%+v", n)
	}
	// Já tem todas: mostra o bloco mais novo, para não ficar vazio.
	todas := SepararNotas(notasComBlocos, versao(t, "2.10.0-dev.5"))
	if todas.QuantasVersoesNovas != 1 || !strings.Contains(todas.Novidades, "PR cinco") {
		t.Errorf("%+v", todas)
	}
	semInstalada := SepararNotas(notasComBlocos, nil)
	if semInstalada.QuantasVersoesNovas != 2 {
		t.Errorf("%+v", semInstalada)
	}
}

// Regra: sem nada para separar, as notas inteiras, sem resto.
func TestSepararSemNada(t *testing.T) {
	t.Parallel()
	n := SepararNotas("  só um texto\n", versao(t, "2.44.0"))
	if n.Novidades != "só um texto" || n.TemResto() || n.QuantasVersoesNovas != 0 {
		t.Errorf("%+v", n)
	}
	// Seção já instalada e nenhum bloco: as notas inteiras.
	if n := SepararNotas("### 2.44.0 — 2026-10-27 · x\n- y", versao(t, "2.44.0")); n.QuantasVersoesNovas != 0 || n.TemResto() {
		t.Errorf("%+v", n)
	}
}

// Regra: o Markdown vira HTML seguro, com títulos um nível abaixo, listas aninhadas, negrito, código e só links web.
func TestHTMLDasNotas(t *testing.T) {
	t.Parallel()
	md := "# Um\n## Dois\n### Três\n#### Quatro\n<!-- comentário -->\n<!-- de\nvárias\nlinhas -->\n\n" +
		"- item **forte [link](https://github.com/x)** e `código`\n  - dentro\n    - mais dentro\n- volta\n\n" +
		"texto <script>alert(1)</script> com [arquivo](file:///c:/x) e [sem host](https:x) e [rel](/a)\r\n* outro item"
	quer := "<h3>Um</h3><h4>Dois</h4><h5>Três</h5><h6>Quatro</h6>" +
		`<ul><li>item <strong>forte <a href="https://github.com/x">link</a></strong> e <code>código</code>` +
		"<ul><li>dentro<ul><li>mais dentro</li></ul></li></ul></li><li>volta</li></ul>" +
		"<p>texto &lt;script&gt;alert(1)&lt;/script&gt; com arquivo e sem host e rel</p>" +
		"<ul><li>outro item</li></ul>"
	if obtido := string(HTMLDasNotas(md)); obtido != quer {
		t.Errorf("obtido:\n%s\nquer:\n%s", obtido, quer)
	}
}

// Regra: o endereço do link também é escapado.
func TestHTMLDasNotasEscapaOLink(t *testing.T) {
	t.Parallel()
	if obtido := string(HTMLDasNotas(`[a](https://x.com/?a="b"&c=<d>)`)); !strings.Contains(obtido, `href="https://x.com/?a=&#34;b&#34;&amp;c=&lt;d&gt;"`) {
		t.Error(obtido)
	}
}

// Regra: a seção de versão depois de um bloco de marcadores também conta.
func TestSecaoDepoisDosMarcadores(t *testing.T) {
	t.Parallel()
	notas := "<!-- por-versao:inicio -->\n### 2.45.0-dev.1 · x\n<!-- por-versao:fim -->\n### 2.45.0 — 2026-10-29 · d\n- nova"
	if n := SepararNotas(notas, versao(t, "2.44.0")); n.QuantasVersoesNovas != 1 || !strings.Contains(n.Novidades, "- nova") {
		t.Errorf("%+v", n)
	}
}

// Regra: link http também vale; endereço que não lê fica só o texto.
func TestHTMLDasNotasLinks(t *testing.T) {
	t.Parallel()
	if obtido := string(HTMLDasNotas("[a](http://x.com/) [b](https://x.com/%zz)")); obtido != `<p><a href="http://x.com/">a</a> b</p>` {
		t.Error(obtido)
	}
}

// Regra: *itálico* vira <em>, mas o asterisco grudado em letra ou número fica como texto (descrições do banco).
func TestHTMLDasNotasItalico(t *testing.T) {
	t.Parallel()
	casos := map[string]string{
		`parece ser *nome* e`:                    `<p>parece ser <em>nome</em> e</p>`,
		`*a*`:                                    `<p><em>a</em></p>`,
		`Pix, Mercado Pago ("MP*MP", "MP*NOME")`: `<p>Pix, Mercado Pago (&#34;MP*MP&#34;, &#34;MP*NOME&#34;)</p>`,
		`2*x* e a * b * c`:                       `<p>2*x* e a * b * c</p>`,
		`*fim*2`:                                 `<p>*fim*2</p>`,
		`é*ção* ou ão*x*`:                        `<p>é*ção* ou ão*x*</p>`,
	}
	for md, quer := range casos {
		if obtido := string(HTMLDasNotas(md)); obtido != quer {
			t.Errorf("%s:\n%s\nquer %s", md, obtido, quer)
		}
	}
}

// Regra: o título de versão ganha o selo com o número; o de tipo de mudança, a classe do tipo; os outros, nada.
func TestHTMLDasNotasTitulos(t *testing.T) {
	t.Parallel()
	md := "## [2.46.0] — 2026-10-28 · desenvolvimento\n### Funcionalidades\n### Correções\n### Publicação e documentação\n" +
		"### 2.26.1-dev.4 · 21/10/2026\n## 2.17.0\n### Destaques\n## [x] não é versão"
	quer := `<h4 class="nota-versao"><span class="selo-versao">2.46.0</span> — 2026-10-28 · desenvolvimento</h4>` +
		`<h5 class="nota-tipo funcionalidades">Funcionalidades</h5><h5 class="nota-tipo correcoes">Correções</h5>` +
		`<h5 class="nota-tipo documentacao">Publicação e documentação</h5>` +
		`<h5 class="nota-versao"><span class="selo-versao">2.26.1-dev.4</span> · 21/10/2026</h5>` +
		`<h4 class="nota-versao"><span class="selo-versao">2.17.0</span></h4>` +
		`<h5>Destaques</h5><h4>[x] não é versão</h4>`
	if obtido := string(HTMLDasNotas(md)); obtido != quer {
		t.Errorf("obtido:\n%s\nquer:\n%s", obtido, quer)
	}
}
