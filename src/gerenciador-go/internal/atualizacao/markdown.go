package atualizacao

import (
	"html"
	"html/template"
	"net/url"
	"regexp"
	"strings"
)

// O Markdown das notas em HTML, só com o que o CHANGELOG e as notas usam (o RenderizadorNotas.cs): títulos (# a ####),
// listas com "-" ou "*" (com recuo por nível), **negrito**, `código` e [links](url). Todo texto é escapado; só links
// http e https viram <a>; comentários HTML (os marcadores dos blocos) não aparecem. Sem biblioteca nova.
var (
	trechosMD = regexp.MustCompile(`\*\*(.+?)\*\*|` + "`([^`]+)`" + `|\[([^\]]+)\]\(([^)\s]+)\)`)
	itemMD    = regexp.MustCompile(`^(\s*)[-*] (.*)$`)
	tituloMD  = regexp.MustCompile(`^(#{1,4}) (.*)$`)
)

// HTMLDasNotas escreve as notas em HTML seguro para pôr na página.
func HTMLDasNotas(markdown string) template.HTML {
	var b strings.Builder
	niveis := 0 // listas abertas
	fecharListas := func(ate int) {
		for ; niveis > ate; niveis-- {
			b.WriteString("</li></ul>")
		}
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
				for ; niveis < nivel; niveis++ {
					b.WriteString("<ul><li>")
				}
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
			b.WriteString("<h" + n + ">" + trechosHTML(m[2]) + "</h" + n + ">")
			continue
		}
		b.WriteString("<p>" + trechosHTML(strings.TrimSpace(linha)) + "</p>")
	}
	fecharListas(0)
	return template.HTML(b.String()) //nolint:gosec // todo texto passou pelo html.EscapeString
}

// trechosHTML: uma linha com **negrito**, `código` e [links](url) no meio do texto comum.
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
