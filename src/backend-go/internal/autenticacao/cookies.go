package autenticacao

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Os nomes dos cookies são os do .NET (ConstantesSeguranca, Seguranca/Seguranca.cs): o Angular lê o do CSRF pelo
// nome, e o prefixo __Host- obriga Secure, Path=/ e nenhum Domain.
const (
	CookieSessao           = "__Host-Financas"
	CookieAntifalsificacao = "__Host-Financas-AF" // segredo do CSRF, HttpOnly
	CookieXsrf             = "__Host-XSRF-TOKEN"  // token que o Angular lê e devolve no cabeçalho
	CabecalhoXsrf          = "X-XSRF-TOKEN"
)

// Prefixos de cada uso da mesma chave: uma assinatura de sessão nunca vale como token CSRF, nem o contrário.
const (
	usoSessao = "financas:sessao:v1\x00"
	usoCsrf   = "financas:csrf:v1\x00"
)

// conteudoSessao é o que vai dentro do cookie da sessão: quem, o carimbo de segurança (trocar a senha muda o carimbo
// e derruba as outras sessões) e a janela de validade. Assinado, não cifrado: não há segredo aqui, só o que não pode
// ser forjado.
type conteudoSessao struct {
	Usuario int64      `json:"u"`
	Login   string     `json:"l"`
	Carimbo tipos.Guid `json:"c"`
	Emitido int64      `json:"e"` // Unix, segundos
	Expira  int64      `json:"x"`
}

func assinar(chave []byte, uso string, dados []byte) []byte {
	m := hmac.New(sha256.New, chave)
	m.Write([]byte(uso))
	m.Write(dados)
	return m.Sum(nil)
}

var b64 = base64.RawURLEncoding

// valorSessao monta o valor do cookie: <conteúdo em Base64URL>.<HMAC-SHA256 em Base64URL>.
func valorSessao(chave []byte, s conteudoSessao) string {
	dados, _ := json.Marshal(s) // struct sem nada que falhe ao serializar
	return b64.EncodeToString(dados) + "." + b64.EncodeToString(assinar(chave, usoSessao, dados))
}

// lerSessao confere a assinatura (em tempo constante) e a validade. Qualquer problema é "sem sessão", como o cookie
// do .NET que não abre: quem pediu continua anônimo.
func lerSessao(chave []byte, valor string, agora time.Time) (conteudoSessao, bool) {
	parteDados, parteAssinatura, ok := strings.Cut(valor, ".")
	if !ok {
		return conteudoSessao{}, false
	}
	dados, err1 := b64.DecodeString(parteDados)
	assinatura, err2 := b64.DecodeString(parteAssinatura)
	if err1 != nil || err2 != nil || !hmac.Equal(assinatura, assinar(chave, usoSessao, dados)) {
		return conteudoSessao{}, false
	}
	var s conteudoSessao
	if err := json.Unmarshal(dados, &s); err != nil || s.Usuario <= 0 {
		return conteudoSessao{}, false
	}
	if agora.Unix() >= s.Expira {
		return conteudoSessao{}, false
	}
	return s, true
}

// precisaRenovar é a renovação deslizante do cookie do .NET (SlidingExpiration): passou da metade da janela, sai um
// cookie novo, com a janela inteira de novo.
func (s conteudoSessao) precisaRenovar(agora time.Time) bool {
	return agora.Unix()-s.Emitido > s.Expira-agora.Unix()
}

// cookieBase tem os atributos de todos os cookies do site: Secure, Path=/, SameSite=Strict (o __Host- exige os dois
// primeiros). Sem Expires: some quando o navegador fecha, como o IsPersistent = false do .NET.
func cookieBase(nome, valor string, httpOnly bool) *http.Cookie {
	return &http.Cookie{Name: nome, Value: valor, Path: "/", Secure: true, HttpOnly: httpOnly, SameSite: http.SameSiteStrictMode}
}

func definirSessao(w http.ResponseWriter, chave []byte, s conteudoSessao) {
	http.SetCookie(w, cookieBase(CookieSessao, valorSessao(chave, s), true))
}

// expirarSessao é o SignOutAsync: o cookie volta vazio e com a data de 1970, para o navegador apagar.
func expirarSessao(w http.ResponseWriter) {
	c := cookieBase(CookieSessao, "", true)
	c.Expires = time.Unix(0, 0).UTC()
	http.SetCookie(w, c)
}

// novoSegredoAntifalsificacao é o valor do cookie __Host-Financas-AF: 32 bytes aleatórios.
func novoSegredoAntifalsificacao() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b64.EncodeToString(b), nil
}

// tokenCsrf é o token que vai no cookie legível e volta no cabeçalho X-XSRF-TOKEN: o HMAC do segredo do cookie
// HttpOnly e do usuário. Ligado ao usuário, como o do .NET: o token de quem ainda não entrou não vale depois do
// login, e por isso ele é reemitido no login e na saída.
func tokenCsrf(chave []byte, segredo string, usuario int64) string {
	return b64.EncodeToString(assinar(chave, usoCsrf, []byte(segredo+"\x00"+strconv.FormatInt(usuario, 10))))
}

// csrfValido confere o cabeçalho com o token esperado para o segredo do cookie e o usuário, em tempo constante.
func csrfValido(chave []byte, segredo, cabecalho string, usuario int64) bool {
	if segredo == "" || cabecalho == "" {
		return false
	}
	esperado := tokenCsrf(chave, segredo, usuario)
	return subtle.ConstantTimeCompare([]byte(esperado), []byte(cabecalho)) == 1
}
