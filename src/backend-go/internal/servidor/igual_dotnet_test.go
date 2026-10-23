package servidor

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/config"
)

// TestIgualAoDotnetRodando compara, rota por rota, o Go com o .NET rodando: status, cabeçalhos (menos Date) e
// corpo. Teste manual (#73): só roda com as duas variáveis, e os dois servem o mesmo wwwroot.
//
//	FINANCAS_DOTNET_URL=https://127.0.0.1:5443 FINANCAS_WWWROOT=../../publicado/api/wwwroot \
//	  go test ./internal/servidor -run IgualAoDotnet -v
//
// Só faz GET sem login: não lê nem grava dado nenhum.
func TestIgualAoDotnetRodando(t *testing.T) {
	urlDotnet, wwwroot := os.Getenv("FINANCAS_DOTNET_URL"), os.Getenv("FINANCAS_WWWROOT")
	if urlDotnet == "" || wwwroot == "" {
		t.Skip("defina FINANCAS_DOTNET_URL e FINANCAS_WWWROOT")
	}
	h, err := NovoHandler(Opcoes{
		Config:    config.Config{HostsPermitidos: []string{"localhost", "127.0.0.1"}},
		PastaSite: wwwroot,
		Log:       logDescartado,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Um arquivo de cada tipo do wwwroot, com e sem hash em maiúsculas, mais as rotas que não são arquivo.
	caminhos := []string{"/", "/index.html", "/transacoes", "/media/", "/nao-existe.js", "/api", "/api/nao-existe", "/API/x"}
	vistos := map[string]bool{}
	arquivos, _ := os.ReadDir(wwwroot)
	for _, a := range arquivos {
		tipo := filepath.Ext(a.Name())
		hash := regexp.MustCompile(`-[A-Z0-9]{8}\.`).MatchString(a.Name())
		if chave := tipo + map[bool]string{true: "+hash"}[hash]; !a.IsDir() && !vistos[chave] {
			vistos[chave] = true
			caminhos = append(caminhos, "/"+a.Name())
		}
	}

	cliente := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // o certificado local do .NET
	}}
	for _, caminho := range caminhos {
		t.Run(caminho, func(t *testing.T) {
			pedido, _ := http.NewRequest("GET", urlDotnet+caminho, nil)
			doDotnet, err := cliente.Do(pedido)
			if err != nil {
				t.Fatal(err)
			}
			corpoDotnet, _ := io.ReadAll(doDotnet.Body)
			doDotnet.Body.Close()

			r := httptest.NewRequest("GET", caminho, nil)
			r.Host = "127.0.0.1:5443"
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			doGo := w.Result()
			corpoGo, _ := io.ReadAll(doGo.Body)

			if doGo.StatusCode != doDotnet.StatusCode {
				t.Errorf("status: .NET %d, Go %d", doDotnet.StatusCode, doGo.StatusCode)
			}
			nomes := map[string]bool{}
			for nome := range doDotnet.Header {
				nomes[nome] = true
			}
			for nome := range doGo.Header {
				nomes[nome] = true
			}
			delete(nomes, "Date")
			for nome := range nomes {
				if !slices.Equal(doDotnet.Header.Values(nome), doGo.Header.Values(nome)) {
					t.Errorf("%s: .NET %q, Go %q", nome, doDotnet.Header.Values(nome), doGo.Header.Values(nome))
				}
			}
			if string(corpoGo) != string(corpoDotnet) {
				t.Errorf("corpo diferente: .NET %d bytes, Go %d bytes", len(corpoDotnet), len(corpoGo))
			}
		})
	}
}
