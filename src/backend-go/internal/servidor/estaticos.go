package servidor

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// arquivoComHash é a mesma expressão do Program.cs: só esses arquivos do build do Angular ganham cache de um ano.
//
// Defeito conhecido, mantido de propósito (a migração não muda comportamento): o Angular de hoje gera nomes com
// minúsculas e "_" (chunk-1dMxoNi3.js), que a expressão não reconhece. Em 21/10/2026, só 3 dos 35 .js/.css
// ganhavam o cache longo. A correção vai nos dois lados, numa issue à parte.
var arquivoComHash = regexp.MustCompile(`-[A-Z0-9]{8}\.(js|css)$`)

// tiposDeConteudo são os tipos que o .NET (FileExtensionContentTypeProvider) manda para as extensões que existem
// no wwwroot, conferidos no site rodando em 21/10/2026. O Go mandaria "; charset=utf-8" em alguns. Extensão nova
// no build do Angular entra aqui com o tipo conferido no .NET; sem tipo, o arquivo não é servido (como no .NET).
var tiposDeConteudo = map[string]string{
	".css":         "text/css",
	".html":        "text/html",
	".js":          "text/javascript",
	".png":         "image/png",
	".svg":         "image/svg+xml",
	".txt":         "text/plain",
	".webmanifest": "application/manifest+json",
}

// arquivosDoSite serve o wwwroot (o Angular) como o UseDefaultFiles + UseStaticFiles + MapFallbackToFile do .NET:
//   - arquivo que existe: com o tipo, o ETag e o Last-Modified do .NET, e o Cache-Control da regra acima;
//   - caminho sem extensão (rota do Angular, ex.: /transacoes): o index.html, sem Cache-Control;
//   - o resto (arquivo que não existe, método que não é GET/HEAD): 401 sem corpo. No .NET é a política de login
//     obrigatório que responde, porque nenhuma rota atende; o esqueleto devolve o mesmo até a autenticação (#75).
type arquivosDoSite struct {
	raiz fs.FS
}

func novosArquivosDoSite(pasta string) (*arquivosDoSite, error) {
	// os.OpenRoot não deixa um caminho escapar da pasta ("../", link simbólico para fora).
	raiz, err := os.OpenRoot(pasta)
	if err != nil {
		return nil, err
	}
	return &arquivosDoSite{raiz: raiz.FS()}, nil
}

func (a *arquivosDoSite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		semCorpo(w, http.StatusUnauthorized)
		return
	}
	nome := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if nome == "" || strings.HasSuffix(r.URL.Path, "/") {
		nome = path.Join(nome, "index.html") // UseDefaultFiles
	}
	if a.servir(w, r, nome, true) {
		return
	}
	if !strings.Contains(path.Base(r.URL.Path), ".") && a.servir(w, r, "index.html", false) { // {*path:nonfile}
		return
	}
	semCorpo(w, http.StatusUnauthorized)
}

// servir manda o arquivo, se ele existe e tem tipo conhecido. comCache diz se põe o Cache-Control (o fallback do
// .NET não põe).
func (a *arquivosDoSite) servir(w http.ResponseWriter, r *http.Request, nome string, comCache bool) bool {
	tipo, conhecido := tiposDeConteudo[strings.ToLower(path.Ext(nome))]
	if !conhecido {
		return false
	}
	arquivo, err := a.raiz.Open(nome)
	if err != nil {
		return false
	}
	defer arquivo.Close()
	info, err := arquivo.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	conteudo, ok := arquivo.(io.ReadSeeker) // o os.Root devolve *os.File
	if !ok {
		return false
	}

	h := w.Header()
	h.Set("Content-Type", tipo)
	h.Set("ETag", etagComoDotnet(info.ModTime(), info.Size()))
	if comCache {
		if arquivoComHash.MatchString(path.Base(nome)) {
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			h.Set("Cache-Control", "no-cache")
		}
	}
	http.ServeContent(w, r, nome, info.ModTime(), conteudo)
	return true
}

// etagComoDotnet é o ETag do StaticFileMiddleware: o horário de modificação (cortado no segundo) em "file time"
// do Windows (intervalos de 100 ns desde 1601), com XOR do tamanho, em hexadecimal. Conferido contra o site.
func etagComoDotnet(modificado time.Time, tamanho int64) string {
	const segundosDe1601Ate1970 = 11_644_473_600
	fileTime := (modificado.Unix() + segundosDe1601Ate1970) * 10_000_000
	return `"` + strconv.FormatInt(fileTime^tamanho, 16) + `"`
}

// errPastaSemIndex avisa cedo que o wwwroot não tem o build do Angular.
var errPastaSemIndex = errors.New("a pasta do site não tem index.html: compile o front-end (npm run build) ou aponte -wwwroot")
