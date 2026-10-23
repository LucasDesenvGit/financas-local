package contrato

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AgenteDoContrato é o User-Agent de todas as requisições: ele vai para a AuditoriaAcesso e tem de ser fixo.
const AgenteDoContrato = "contrato-financas-local"

const (
	cookieXsrf    = "__Host-XSRF-TOKEN" // ConstantesSeguranca.CookieXsrf
	cabecalhoXsrf = "X-XSRF-TOKEN"      // ConstantesSeguranca.CabecalhoXsrf
)

// Sessao é um navegador: cookies próprios e, se pedido, outro IP de origem.
type Sessao struct {
	alvo    *url.URL
	origem  string
	cliente *http.Client
}

func novaSessao(alvo *url.URL, origem string) (*Sessao, error) {
	s := &Sessao{alvo: alvo, origem: origem}
	return s, s.limpar()
}

// limpar começa de novo, sem cookies.
func (s *Sessao) limpar() error {
	pote, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	discador := &net.Dialer{Timeout: 10 * time.Second}
	if s.origem != "" {
		ip := net.ParseIP(s.origem)
		if ip == nil {
			return fmt.Errorf("origem %q não é um IP", s.origem)
		}
		discador.LocalAddr = &net.TCPAddr{IP: ip}
	}
	s.cliente = &http.Client{
		Jar:     pote,
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DialContext: discador.DialContext,
			// O servidor sob teste usa um certificado descartável, criado para a execução.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // só 127.0.0.1, certificado de teste
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return nil
}

// tokenCsrf é o valor do cookie que o Angular manda de volta no cabeçalho X-XSRF-TOKEN. Se a sessão ainda não
// tem o cookie, pede GET /api/auth/estado, como o front-end faz ao abrir (não grava nada no banco).
func (s *Sessao) tokenCsrf(ctx context.Context) (string, error) {
	if v := s.cookie(cookieXsrf); v != "" {
		return v, nil
	}
	r, err := s.enviar(ctx, http.MethodGet, "/api/auth/estado", nil, "", "")
	if err != nil {
		return "", fmt.Errorf("pegar o token CSRF: %w", err)
	}
	if r.Status != http.StatusOK {
		return "", fmt.Errorf("pegar o token CSRF: GET /api/auth/estado respondeu %d", r.Status)
	}
	if v := s.cookie(cookieXsrf); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("GET /api/auth/estado não devolveu o cookie %s", cookieXsrf)
}

func (s *Sessao) cookie(nome string) string {
	for _, c := range s.cliente.Jar.Cookies(s.alvo) {
		if c.Name == nome {
			return c.Value
		}
	}
	return ""
}

// Resposta é o que o servidor devolveu, ainda sem normalizar.
type Resposta struct {
	Status  int
	Tipo    string
	Cookies []string
	Corpo   []byte
}

func (s *Sessao) enviar(ctx context.Context, metodo, caminho string, corpo []byte, tipoCorpo, csrf string) (*Resposta, error) {
	endereco := s.alvo.JoinPath() // cópia
	caminhoURL, err := url.Parse(caminho)
	if err != nil {
		return nil, err
	}
	endereco = endereco.ResolveReference(caminhoURL)
	var leitor io.Reader
	if corpo != nil {
		leitor = strings.NewReader(string(corpo))
	}
	pedido, err := http.NewRequestWithContext(ctx, metodo, endereco.String(), leitor)
	if err != nil {
		return nil, err
	}
	pedido.Header.Set("User-Agent", AgenteDoContrato)
	pedido.Header.Set("Accept", "application/json, text/plain, */*") // o que o HttpClient do Angular manda
	if tipoCorpo != "" {
		pedido.Header.Set("Content-Type", tipoCorpo)
	}
	if csrf != "" {
		pedido.Header.Set(cabecalhoXsrf, csrf)
	}
	resp, err := s.cliente.Do(pedido)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	dados, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	r := &Resposta{Status: resp.StatusCode, Corpo: dados}
	if t := resp.Header.Get("Content-Type"); t != "" {
		r.Tipo = tipoNormalizado(t)
	}
	for _, c := range resp.Header.Values("Set-Cookie") {
		r.Cookies = append(r.Cookies, cookie(c))
	}
	sort.Strings(r.Cookies)
	return r, nil
}

// tipoNormalizado escreve o Content-Type num formato só (minúsculas, um espaço depois do ";").
func tipoNormalizado(t string) string {
	tipo, parametros, err := mime.ParseMediaType(t)
	if err != nil {
		return t
	}
	nomes := make([]string, 0, len(parametros))
	for k := range parametros {
		nomes = append(nomes, k)
	}
	sort.Strings(nomes)
	partes := []string{tipo}
	for _, k := range nomes {
		partes = append(partes, k+"="+strings.ToLower(parametros[k]))
	}
	return strings.Join(partes, "; ")
}

// ehJSON diz se o Content-Type é JSON (application/json, application/problem+json).
func ehJSON(tipo string) bool {
	t, _, _ := strings.Cut(tipo, ";")
	return t == "application/json" || strings.HasSuffix(t, "+json")
}
