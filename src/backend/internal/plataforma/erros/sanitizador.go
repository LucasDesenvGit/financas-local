package erros

import (
	"regexp"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/respostas"
)

// Removido é o que fica no lugar do que foi tirado.
const Removido = "[removido]"

// As expressões, no RE2 do Go (regexp/syntax). Nenhuma usa lookahead nem lookbehind, então todas
// passam. Cuidados com o RE2, todos para o lado de tirar mais:
//   - \s pega também \v, U+0085 e os espaços Unicode: aqui é a classe espaco, com os mesmos;
//   - \d pega qualquer dígito Unicode: aqui é \p{Nd}, igual;
//   - \b olharia letras Unicode, mas o do RE2 é só ASCII. Em "çtoken=x" uma regex Unicode não vê fronteira e deixa passar; o Go
//     vê e tira. É o único caso em que o resultado muda, e muda para tirar mais.
const (
	espaco   = `\s\v\x{85}\p{Z}`
	digito   = `\p{Nd}`
	naoValor = `[^` + espaco + `&,;"')\]}]` // #396: o valor para no ")", "]" e "}" que fecham o que veio antes
)

var (
	queryString = regexp.MustCompile(`(?i)(https?://[^` + espaco + `?#"'<>]+)\?[^` + espaco + `#"'<>]*`)
	// #396: "Bearer " depois da chave fica no texto, e o que sai é o token (antes saía a palavra e o token ficava).
	chaveValor = regexp.MustCompile(`(?i)\b(access_?token|refresh_?token|token|api_?key|x-api-key|client_?secret|secret|senha|password|passwd|pwd|authorization|cookie)` +
		`([` + espaco + `]*[=:][` + espaco + `]*"?|"[` + espaco + `]*:[` + espaco + `]*")((?:Bearer[` + espaco + `]+)?)(` + naoValor + `+)`)
	bearer = regexp.MustCompile(`(?i)\bBearer[` + espaco + `]+[A-Za-z0-9\-._~+/]+=*`)
	jwt    = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*`)
	cpf    = regexp.MustCompile(`\b` + digito + `{3}\.?` + digito + `{3}\.?` + digito + `{3}-?` + digito + `{2}\b`)
	reais  = regexp.MustCompile(`R\$[` + espaco + `]?-?` + digito + `[` + digito + `.]*(,` + digito + `{1,2})?`)
)

// Limpar é o Sanitizador.Limpar: tira query strings de URLs, pares "chave=valor" de segredos, tokens Bearer e JWT,
// CPF e valores em reais, nessa ordem. É uma limpeza por padrão de texto, não uma garantia: o código continua não
// pondo segredo em log.
// As expressões e a ordem são conferidas pelo contrato 17-erros.
func Limpar(texto string) string {
	if texto == "" {
		return texto
	}
	texto = queryString.ReplaceAllString(texto, "${1}?"+Removido)
	texto = chaveValor.ReplaceAllString(texto, "${1}${2}${3}"+Removido)
	texto = bearer.ReplaceAllString(texto, "Bearer "+Removido)
	texto = jwt.ReplaceAllString(texto, Removido)
	texto = cpf.ReplaceAllString(texto, Removido)
	texto = reais.ReplaceAllString(texto, "R$$ "+Removido) // "$$" é o "$" literal no ReplaceAllString
	return texto
}

// LimparECortar é o LimparECortar: limpa e corta no tamanho da coluna (em caracteres UTF-16),
// com "…" no fim quando corta. nil fica nil.
func LimparECortar(texto *string, maximo int) *string {
	if texto == nil {
		return nil
	}
	limpo := Limpar(*texto)
	if respostas.TamanhoUtf16(limpo) > maximo {
		limpo = respostas.CortarUtf16(limpo, maximo-1) + "…"
	}
	return &limpo
}
