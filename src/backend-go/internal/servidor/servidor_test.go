package servidor

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/config"
)

var logDescartado = slog.New(slog.DiscardHandler)

func handlerDeTeste(t *testing.T) http.Handler {
	t.Helper()
	h, err := NovoHandler(Opcoes{
		Config:    config.Config{HostsPermitidos: []string{"localhost", "127.0.0.1"}},
		PastaSite: "testdata/wwwroot",
		Log:       logDescartado,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func pedir(t *testing.T, h http.Handler, metodo, caminho, host string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(metodo, caminho, nil)
	r.Host = host
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// O Seguranca.cs continua sendo a fonte dos cabeçalhos até o .NET sair (#86): este teste lê o C# e compara.
func TestCabecalhosIguaisAoSegurancaCs(t *testing.T) {
	fonte, err := os.ReadFile("../../../backend/Financas.Api/Seguranca/Seguranca.cs")
	if err != nil {
		t.Fatal(err)
	}
	texto := string(fonte)
	inicio := strings.Index(texto, "private const string Csp =")
	fim := strings.Index(texto[inicio:], ";\n")
	var cspDoCSharp strings.Builder
	for _, m := range regexp.MustCompile(`"([^"]*)"`).FindAllStringSubmatch(texto[inicio:inicio+fim], -1) {
		cspDoCSharp.WriteString(m[1])
	}
	if cspDoCSharp.String() != csp {
		t.Errorf("CSP diferente do Seguranca.cs:\n C#: %s\n Go: %s", cspDoCSharp.String(), csp)
	}

	doCSharp := map[string]string{"Content-Security-Policy": csp}
	for _, m := range regexp.MustCompile(`h\["([A-Za-z-]+)"\] = "([^"]*)";`).FindAllStringSubmatch(texto, -1) {
		if m[1] != "Cache-Control" && m[1] != "Pragma" { // esses dois só em /api, conferidos abaixo
			doCSharp[m[1]] = m[2]
		}
	}
	doGo := map[string]string{}
	for _, c := range cabecalhosDeSeguranca {
		doGo[c[0]] = c[1]
	}
	if len(doCSharp) != len(doGo) {
		t.Errorf("o C# tem %d cabeçalhos e o Go %d:\n C#: %v\n Go: %v", len(doCSharp), len(doGo), doCSharp, doGo)
	}
	for nome, valor := range doCSharp {
		if doGo[nome] != valor {
			t.Errorf("%s: C# %q, Go %q", nome, valor, doGo[nome])
		}
	}

	resposta := pedir(t, handlerDeTeste(t), "GET", "/", "127.0.0.1:5443")
	for nome, valor := range doGo {
		if resposta.Header().Get(nome) != valor {
			t.Errorf("a resposta não tem %s: %q", nome, valor)
		}
	}
	for _, proibido := range []string{"Server", "X-Powered-By", "Strict-Transport-Security"} {
		if resposta.Header().Get(proibido) != "" {
			t.Errorf("a resposta não deveria ter %s (o .NET não manda)", proibido)
		}
	}
}

// Os casos foram tirados do .NET rodando em 21/10/2026 (veja TestIgualAoDotnetRodando).
func TestRespostasComoODotnet(t *testing.T) {
	h := handlerDeTeste(t)
	casos := []struct {
		nome, metodo, caminho string
		status                int
		cache, pragma, tipo   string
		corpoVazio            bool
	}{
		{"raiz serve o index.html", "GET", "/", 200, "no-cache", "", "text/html", false},
		{"index.html", "GET", "/index.html", 200, "no-cache", "", "text/html", false},
		{"arquivo com hash em maiúsculas: cache de um ano", "GET", "/main-ABCD1234.js", 200, "public, max-age=31536000, immutable", "", "text/javascript", false},
		{"css com hash", "GET", "/styles-ABCD1234.css", 200, "public, max-age=31536000, immutable", "", "text/css", false},
		{"hash com minúsculas: no-cache (defeito do .NET mantido)", "GET", "/chunk-1dMxoNi3.js", 200, "no-cache", "", "text/javascript", false},
		{"manifesto", "GET", "/manifest.webmanifest", 200, "no-cache", "", "application/manifest+json", false},
		{"rota do Angular: index.html sem Cache-Control", "GET", "/transacoes", 200, "", "", "text/html", false},
		{"rota do Angular com barra no fim", "GET", "/media/", 200, "", "", "text/html", false},
		{"arquivo que não existe: 401 (política de login do .NET)", "GET", "/nao-existe.js", 401, "", "", "", true},
		{"extensão sem tipo conhecido não é servida", "GET", "/desconhecido.xyz", 401, "", "", "", true},
		{"POST fora de /api: 401", "POST", "/transacoes", 401, "", "", "", true},
		{"rota /api desconhecida: 404 sem corpo e sem cache", "GET", "/api/nao-existe", 404, "no-store", "no-cache", "", true},
		{"/api sem barra", "GET", "/api", 404, "no-store", "no-cache", "", true},
		{"/API em maiúsculas", "GET", "/API/x", 404, "no-store", "no-cache", "", true},
		{"POST em /api", "POST", "/api/x", 404, "no-store", "no-cache", "", true},
		{"caminho com .. é limpo, sem redirecionar (/go.mod: 401)", "GET", "/../go.mod", 401, "", "", "", true},
		{"caminho com .. dentro do site", "GET", "/x/../index.html", 200, "no-cache", "", "text/html", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := pedir(t, h, c.metodo, c.caminho, "127.0.0.1:5443")
			if w.Code != c.status {
				t.Errorf("status %d, esperado %d", w.Code, c.status)
			}
			if got := w.Header().Get("Cache-Control"); got != c.cache {
				t.Errorf("Cache-Control %q, esperado %q", got, c.cache)
			}
			if got := w.Header().Get("Pragma"); got != c.pragma {
				t.Errorf("Pragma %q, esperado %q", got, c.pragma)
			}
			if got := w.Header().Get("Content-Type"); got != c.tipo {
				t.Errorf("Content-Type %q, esperado %q", got, c.tipo)
			}
			if c.corpoVazio && w.Body.Len() != 0 {
				t.Errorf("corpo deveria ser vazio: %q", w.Body.String())
			}
		})
	}
}

func TestHostNaoPermitido(t *testing.T) {
	h := handlerDeTeste(t)
	w := pedir(t, h, "GET", "/", "exemplo.com")
	if w.Code != 400 || w.Body.String() != paginaHostInvalido || w.Header().Get("Content-Type") != "text/html" {
		t.Errorf("status %d, tipo %q, corpo %q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	if len(paginaHostInvalido) != 334 {
		t.Errorf("a página do .NET tem 334 bytes; a do Go tem %d", len(paginaHostInvalido))
	}
	if w.Header().Get("Content-Security-Policy") != "" {
		t.Error("o .NET responde o 400 de host antes dos cabeçalhos de segurança")
	}
	for host, permitido := range map[string]bool{"LOCALHOST:5443": true, "127.0.0.1": true, "[::1]:5443": false, "": false} {
		if hostPermitido(host, []string{"localhost", "127.0.0.1"}) != permitido {
			t.Errorf("hostPermitido(%q) deveria ser %v", host, permitido)
		}
	}
}

func TestPanicoVira500ComAMensagemDoDotnet(t *testing.T) {
	h := comCabecalhosDeSeguranca(recuperarPanico(logDescartado, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("falha de teste")
	})))
	w := pedir(t, h, "GET", "/api/x", "127.0.0.1")
	if w.Code != 500 || w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("status %d, tipo %q", w.Code, w.Header().Get("Content-Type"))
	}
	var corpo struct{ Mensagem string }
	if err := json.Unmarshal(w.Body.Bytes(), &corpo); err != nil {
		t.Fatal(err)
	}
	if corpo.Mensagem != "Ocorreu um erro interno. Ele foi registrado em Configurações › Erros." {
		t.Errorf("mensagem %q", corpo.Mensagem)
	}
	if !strings.Contains(w.Body.String(), `Configura\u00E7\u00F5es \u203A Erros`) {
		t.Errorf("o System.Text.Json escapa o que não é ASCII: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "falha de teste") {
		t.Error("o erro interno não pode aparecer na resposta")
	}
}

func TestCorpoAcimaDe1MB(t *testing.T) {
	var erroLido error
	h := limitarCorpo(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, erroLido = io.ReadAll(r.Body)
	}))
	r := httptest.NewRequest("POST", "/api/x", strings.NewReader(strings.Repeat("x", LimiteDoCorpo+1)))
	h.ServeHTTP(httptest.NewRecorder(), r)
	var excesso *http.MaxBytesError
	if !errors.As(erroLido, &excesso) {
		t.Errorf("ler mais de 1 MB deveria dar http.MaxBytesError, deu %v", erroLido)
	}
}

func TestEtagComoDotnet(t *testing.T) {
	// O index.html que o .NET servia: 802 bytes, modificado em 21/10/2026 12:46:54 UTC, ETag "1dd615a4004f022".
	modificado := time.Date(2026, 10, 21, 12, 46, 54, 999_000_000, time.UTC)
	if got := etagComoDotnet(modificado, 802); got != `"1dd615a4004f022"` {
		t.Errorf("ETag %s, o .NET dá \"1dd615a4004f022\"", got)
	}
}

func TestCarregarPFX(t *testing.T) {
	temporario, err := CertificadoTemporario()
	if err != nil {
		t.Fatal(err)
	}
	chave, certificado := temporario.PrivateKey.(*ecdsa.PrivateKey), temporario.Leaf

	for nome, codificador := range map[string]*pkcs12.Encoder{
		"3DES (o que o dotnet dev-certs e o .msi geram hoje)":      pkcs12.LegacyDES,
		"AES (OpenSSL 3, Export-PfxCertificate com AES256_SHA256)": pkcs12.Modern2023,
	} {
		t.Run(nome, func(t *testing.T) {
			pfx, err := codificador.Encode(chave, certificado, nil, "senha-de-teste")
			if err != nil {
				t.Fatal(err)
			}
			caminho := filepath.Join(t.TempDir(), "teste.pfx")
			if err := os.WriteFile(caminho, pfx, 0o600); err != nil {
				t.Fatal(err)
			}
			lido, err := CarregarPFX(caminho, "senha-de-teste")
			if err != nil {
				t.Fatal(err)
			}
			if lido.Leaf.SerialNumber.Cmp(certificado.SerialNumber) != 0 {
				t.Error("leu outro certificado")
			}
			if _, err := CarregarPFX(caminho, "senha-errada"); err == nil {
				t.Error("senha errada deveria dar erro")
			}
		})
	}
}

// Regra: porta ocupada é erro na hora (com o endereço na mensagem), e não um servidor que nunca atende.
func TestRodarComPortaOcupada(t *testing.T) {
	certificado, err := CertificadoTemporario()
	if err != nil {
		t.Fatal(err)
	}
	ocupada, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ocupada.Close()
	opcoes := Opcoes{PastaSite: "testdata/wwwroot", Certificado: certificado, Log: logDescartado}
	opcoes.Config.Endereco = ocupada.Addr().String()

	err = Rodar(t.Context(), opcoes)
	if err == nil || !strings.Contains(err.Error(), "escutar em "+opcoes.Config.Endereco) {
		t.Errorf("Rodar numa porta ocupada: erro %v, esperado um erro com \"escutar em %s\"", err, opcoes.Config.Endereco)
	}
}

func TestRodarSoLocalEParaComElegancia(t *testing.T) {
	certificado, err := CertificadoTemporario()
	if err != nil {
		t.Fatal(err)
	}
	opcoes := Opcoes{PastaSite: "testdata/wwwroot", Certificado: certificado, Log: logDescartado}

	opcoes.Config.Endereco = "0.0.0.0:5443"
	if err := Rodar(context.Background(), opcoes); err == nil {
		t.Fatal("deveria recusar escutar fora do computador")
	}

	// Porta 0: o sistema escolhe uma livre, e o AoEscutar diz qual foi quando o servidor já aceita conexões.
	opcoes.Config.Endereco = "127.0.0.1:0"
	escutando := make(chan string, 1)
	opcoes.AoEscutar = func(endereco string) { escutando <- endereco }

	ctx, parar := context.WithCancel(context.Background())
	terminou := make(chan error, 1)
	go func() { terminou <- Rodar(ctx, opcoes) }()

	var endereco string
	select {
	case endereco = <-escutando:
	case err := <-terminou:
		t.Fatalf("o servidor parou antes de escutar: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("o servidor não avisou que estava escutando")
	}

	cliente := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // certificado temporário do próprio teste
		ForceAttemptHTTP2: true,                                  // com TLSClientConfig próprio, o cliente não tenta HTTP/2 sozinho
	}}
	resposta, err := cliente.Get("https://" + endereco + "/")
	if err != nil {
		t.Fatal(err)
	}
	resposta.Body.Close()
	if resposta.StatusCode != 200 || resposta.ProtoMajor != 2 {
		t.Errorf("status %d, HTTP/%d (o Kestrel responde em HTTP/2)", resposta.StatusCode, resposta.ProtoMajor)
	}

	parar()
	select {
	case err := <-terminou:
		if err != nil {
			t.Errorf("parar deveria terminar sem erro: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("o servidor não parou")
	}
}
