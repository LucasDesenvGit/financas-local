package servidor

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// Cabeçalhos de segurança iguais aos do CabecalhosSegurancaMiddleware (src/backend/Financas.Api/Seguranca/
// Seguranca.cs). Um teste lê o Seguranca.cs e confere que são os mesmos: ele continua sendo a fonte até a #86.
const csp = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

var cabecalhosDeSeguranca = [][2]string{
	{"Content-Security-Policy", csp},
	{"X-Content-Type-Options", "nosniff"},
	{"X-Frame-Options", "DENY"},
	{"Referrer-Policy", "no-referrer"},
	{"Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()"},
	{"Cross-Origin-Opener-Policy", "same-origin"},
	{"Cross-Origin-Resource-Policy", "same-origin"},
}

// comCabecalhosDeSeguranca põe os cabeçalhos em toda resposta e, em /api, proíbe o cache (dado financeiro).
// O Go não manda cabeçalho Server nem X-Powered-By, então não há o que tirar. Sem HSTS: o .NET também não manda
// em 127.0.0.1 nem em localhost (conferido no site rodando).
func comCabecalhosDeSeguranca(proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		for _, c := range cabecalhosDeSeguranca {
			h.Set(c[0], c[1])
		}
		if ehAPI(r.URL.Path) {
			h.Set("Cache-Control", "no-store")
			h.Set("Pragma", "no-cache")
		}
		proximo.ServeHTTP(w, r)
	})
}

// ehAPI é o StartsWithSegments("/api") do .NET: "/api" ou "/api/…", sem diferença de maiúsculas.
func ehAPI(caminho string) bool {
	return strings.EqualFold(caminho, "/api") || len(caminho) > 4 && strings.EqualFold(caminho[:5], "/api/")
}

// paginaHostInvalido é a resposta do filtro de hosts do ASP.NET (AllowedHosts), copiada do site rodando.
const paginaHostInvalido = "<!DOCTYPE HTML PUBLIC \"-//W3C//DTD HTML 4.01//EN\"\"http://www.w3.org/TR/html4/strict.dtd\">\r\n" +
	"<HTML><HEAD><TITLE>Bad Request</TITLE>\r\n" +
	"<META HTTP-EQUIV=\"Content-Type\" Content=\"text/html; charset=us-ascii\"></ HEAD >\r\n" +
	"<BODY><h2>Bad Request - Invalid Hostname</h2>\r\n" +
	"<hr><p>HTTP Error 400. The request hostname is invalid.</p>\r\n" +
	"</BODY></HTML>"

// soHostsPermitidos é o AllowedHosts: outro nome no cabeçalho Host (ex.: um site de fora apontando um domínio
// para 127.0.0.1, o "DNS rebinding") recebe 400. Vem antes dos cabeçalhos de segurança, como no .NET.
func soHostsPermitidos(permitidos []string, proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostPermitido(r.Host, permitidos) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(paginaHostInvalido))
			return
		}
		proximo.ServeHTTP(w, r)
	})
}

func hostPermitido(host string, permitidos []string) bool {
	if len(permitidos) == 0 {
		return true
	}
	if semPorta, _, err := net.SplitHostPort(host); err == nil {
		host = semPorta
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return false
	}
	for _, p := range permitidos {
		if p == "*" || strings.EqualFold(p, host) {
			return true
		}
	}
	return false
}

// mensagemErroInterno é o corpo do 500, como o System.Text.Json escreve (ele troca o que não é ASCII por \uXXXX).
var mensagemErroInterno = []byte(`{"mensagem":"` +
	EscaparComoDotnet("Ocorreu um erro interno. Ele foi registrado em Configurações › Erros.") + `"}`)

// EscaparComoDotnet troca o que não é ASCII por \uXXXX, como o JavaScriptEncoder.Default do System.Text.Json (o
// JsonSerializer.Serialize do .NET sem opções): o "ó" de "Dólar" vira uma barra, a letra u e o código 00F3, e o que
// está fora do plano básico (um emoji) vira um par de substitutos. Serve também para gravar no banco o mesmo texto que o .NET gravaria (#81).
func EscaparComoDotnet(texto string) string {
	var b strings.Builder
	for _, r := range texto {
		if r < utf8.RuneSelf {
			b.WriteRune(r)
			continue
		}
		if alto, baixo := utf16.EncodeRune(r); alto != utf8.RuneError {
			fmt.Fprintf(&b, `\u%04X\u%04X`, alto, baixo)
			continue
		}
		fmt.Fprintf(&b, `\u%04X`, r)
	}
	return b.String()
}

// recuperarPanico é o UseExceptionHandler: o erro completo vai para o log, e quem pediu recebe só a mensagem.
func recuperarPanico(log *slog.Logger, proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resposta := envolver(w)
		defer func() {
			p := recover()
			if p == nil {
				return
			}
			if p == http.ErrAbortHandler { // o próprio net/http usa esse pânico para abortar a resposta
				panic(p)
			}
			log.Error("erro não tratado", "caminho", r.URL.Path, "erro", fmt.Sprint(p))
			if resposta.status != 0 {
				return // a resposta já começou: não há como trocar por um 500
			}
			ResponderErroInterno(resposta)
		}()
		proximo.ServeHTTP(resposta, r)
	})
}

// ResponderErroInterno responde 500 com a mensagem genérica do .NET. O erro em si vai para o log, nunca para quem
// pediu.
func ResponderErroInterno(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(mensagemErroInterno)
}

// LimiteDoCorpo é o MaxRequestBodySize do Kestrel: 1 MB.
const LimiteDoCorpo = 1 << 20

func limitarCorpo(proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, LimiteDoCorpo)
		proximo.ServeHTTP(w, r)
	})
}

// registrarRequisicoes escreve uma linha por requisição: método, caminho (sem a query, que pode ter filtros com
// dado da pessoa), status e duração.
func registrarRequisicoes(log *slog.Logger, proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		resposta := envolver(w)
		proximo.ServeHTTP(resposta, r)
		log.LogAttrs(r.Context(), slog.LevelInfo, "requisição",
			slog.String("metodo", r.Method), slog.String("caminho", r.URL.Path),
			slog.Int("status", resposta.statusFinal()), slog.Duration("duracao", time.Since(inicio)))
	})
}

// respostaComStatus guarda o status que foi escrito. O Unwrap deixa o http.ResponseController chegar ao
// ResponseWriter de baixo (Flush, prazos).
type respostaComStatus struct {
	http.ResponseWriter
	status int
}

func envolver(w http.ResponseWriter) *respostaComStatus {
	if r, ok := w.(*respostaComStatus); ok {
		return r
	}
	return &respostaComStatus{ResponseWriter: w}
}

func (r *respostaComStatus) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *respostaComStatus) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func (r *respostaComStatus) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *respostaComStatus) statusFinal() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}
