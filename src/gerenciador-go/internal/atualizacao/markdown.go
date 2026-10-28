package atualizacao

import (
	"html"
	"html/template"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// O Markdown das notas em HTML, só com o que o CHANGELOG e as notas usam (o RenderizadorNotas.cs): títulos (# a ####),
// listas com "-" ou "*" (com recuo por nível), **negrito**, *itálico*, `código` e [links](url). Todo texto é escapado;
// só links http e https viram <a>; comentários HTML (os marcadores dos blocos) não aparecem. Sem biblioteca nova.
var (
	trechosMD = regexp.MustCompile(`\*\*(.+?)\*\*|` + "`([^`]+)`" + `|\[([^\]]+)\]\(([^)\s]+)\)|\*([^*\s](?:[^*]*[^*\s])?)\*`)
	itemMD    = regexp.MustCompile(`^(\s*)[-*] (.*)$`)
	tituloMD  = regexp.MustCompile(`^(#{1,4}) (.*)$`)
)

// HTMLDasNotas escreve as notas em HTML seguro para pôr na página.
func HTMLDasNotas(markdown string) template.HTML {
	var b strings.Builder
	// niveis: listas abertas. Abrir e fechar vão de uma vez (strings.Repeat), sem laço que conta: um laço que conta
	// para o lado errado escreveria sem parar e acabaria com a memória (foi o que travou o teste de mutação, #433).
	niveis := 0
	fecharListas := func(ate int) {
		b.WriteString(strings.Repeat("</li></ul>", niveis-ate))
		niveis = ate
	}
	emComentario := false
	for bruta := range strings.SplitSeq(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n") {
		linha := strings.TrimRight(bruta, " \t")
		if emComentario {
			emComentario = !strings.Contains(linha, "-->")
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(linha), "<!--") {
			emComentario = !strings.Contains(linha, "-->")
			continue
		}
		if linha == "" {
			continue
		}
		if m := itemMD.FindStringSubmatch(linha); m != nil {
			nivel := len(strings.ReplaceAll(m[1], "\t", "  "))/2 + 1
			switch {
			case nivel > niveis:
				b.WriteString(strings.Repeat("<ul><li>", nivel-niveis))
				niveis = nivel
			default:
				fecharListas(nivel)
				b.WriteString("</li><li>")
			}
			b.WriteString(trechosHTML(m[2]))
			continue
		}
		fecharListas(0)
		if m := tituloMD.FindStringSubmatch(linha); m != nil {
			// Os níveis do Markdown descem um: o título da janela já é o h2.
			n := string(rune('2' + len(m[1])))
			classe, conteudo := titulo(m[2])
			b.WriteString("<h" + n + classe + ">" + conteudo + "</h" + n + ">")
			continue
		}
		b.WriteString("<p>" + trechosHTML(strings.TrimSpace(linha)) + "</p>")
	}
	fecharListas(0)
	return template.HTML(b.String()) //nolint:gosec // todo texto passou pelo html.EscapeString
}

// trechosHTML: uma linha com **negrito**, *itálico*, `código` e [links](url) no meio do texto comum. O itálico não
// leva asterisco dentro (nem negrito): o CHANGELOG não usa.
func trechosHTML(texto string) string {
	var b strings.Builder
	posicao := 0
	for _, m := range trechosMD.FindAllStringSubmatchIndex(texto, -1) {
		b.WriteString(html.EscapeString(texto[posicao:m[0]]))
		switch {
		case m[2] >= 0:
			// Pode ter link dentro do negrito.
			b.WriteString("<strong>" + trechosHTML(texto[m[2]:m[3]]) + "</strong>")
		case m[4] >= 0:
			b.WriteString("<code>" + html.EscapeString(texto[m[4]:m[5]]) + "</code>")
		case m[10] >= 0:
			// Asterisco grudado em letra ou número não é itálico ("MP*MP", "MP*NOME", das descrições do banco).
			if letraOuNumeroAntes(texto, m[0]) || letraOuNumeroDepois(texto, m[1]) {
				b.WriteString(html.EscapeString(texto[m[0]:m[1]]))
			} else {
				b.WriteString("<em>" + trechosHTML(texto[m[10]:m[11]]) + "</em>")
			}
		default:
			rotulo, destino := html.EscapeString(texto[m[6]:m[7]]), texto[m[8]:m[9]]
			// Só links da web abrem no navegador (nada de arquivo local ou outro esquema vindo das notas).
			if u, err := url.Parse(destino); err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
				b.WriteString(`<a href="` + html.EscapeString(u.String()) + `">` + rotulo + "</a>")
			} else {
				b.WriteString(rotulo)
			}
		}
		posicao = m[1]
	}
	b.WriteString(html.EscapeString(texto[posicao:]))
	return b.String()
}

// tituloDeVersaoMD: "[2.46.0] — 2026-10-28 · desenvolvimento" (CHANGELOG) ou "2.26.1 — …" (notas da versão).
var tituloDeVersaoMD = regexp.MustCompile(`^\[?(\d+\.\d+\.\d+(?:-dev\.\d+)?)\]?(\s.*)?$`)

// tiposDeMudanca: os títulos de tipo do CHANGELOG (#294), cada um com a sua cor.
var tiposDeMudanca = map[string]string{
	"Funcionalidades": "funcionalidades", "Correções": "correcoes", "Publicação e documentação": "documentacao",
}

// titulo dá a classe e o conteúdo de um título: o de versão ganha o selo com o número (e a linha que separa da versão
// anterior, no CSS); o de tipo de mudança, a cor do tipo. Os outros ficam como estão.
func titulo(texto string) (classe, conteudo string) {
	if m := tituloDeVersaoMD.FindStringSubmatch(texto); m != nil {
		return ` class="nota-versao"`, `<span class="selo-versao">` + html.EscapeString(m[1]) + `</span>` + trechosHTML(m[2])
	}
	if tipo, ok := tiposDeMudanca[strings.TrimSpace(texto)]; ok {
		return ` class="nota-tipo ` + tipo + `"`, trechosHTML(texto)
	}
	return "", trechosHTML(texto)
}

// letraOuNumeroAntes e letraOuNumeroDepois: o caractere inteiro (o "ç" tem dois bytes) logo antes ou logo depois da
// posição; no começo ou no fim do texto, não há (RuneError, que não é letra).
func letraOuNumeroAntes(texto string, i int) bool {
	r, _ := utf8.DecodeLastRuneInString(texto[:i])
	return letraOuNumero(r)
}

func letraOuNumeroDepois(texto string, i int) bool {
	r, _ := utf8.DecodeRuneInString(texto[i:])
	return letraOuNumero(r)
}

func letraOuNumero(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
