package contrato

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/servidor"
)

// Regra (#638): o cliente do contrato confere o certificado do servidor, e confia só no .pem que recebeu. Sem ele, ou com
// outro certificado, a conexão falha, como no navegador.
func TestClienteConfereOCertificado(t *testing.T) {
	alvoDoTeste := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer alvoDoTeste.Close()
	alvo, err := url.Parse(alvoDoTeste.URL)
	if err != nil {
		t.Fatal(err)
	}
	pasta := t.TempDir()
	gravar := func(nome string, dados []byte) string {
		caminho := filepath.Join(pasta, nome)
		if err := os.WriteFile(caminho, dados, 0o600); err != nil {
			t.Fatal(err)
		}
		return caminho
	}
	certo := gravar("servidor.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: alvoDoTeste.Certificate().Raw}))
	// Outro certificado de verdade: os servidores do httptest dividem o mesmo certificado embutido.
	outro, err := servidor.CertificadoTemporario()
	if err != nil {
		t.Fatal(err)
	}
	errado := gravar("outro.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: outro.Leaf.Raw}))

	casos := []struct {
		nome  string
		raiz  string
		passa bool
	}{
		{"o certificado do servidor", certo, true},
		{"outro certificado", errado, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			raizes, err := lerRaiz(c.raiz)
			if err != nil {
				t.Fatal(err)
			}
			s, err := novaSessao(alvo, "", raizes)
			if err != nil {
				t.Fatal(err)
			}
			resposta, err := s.cliente.Get(alvoDoTeste.URL)
			if err == nil {
				resposta.Body.Close()
			}
			if (err == nil) != c.passa {
				t.Errorf("erro = %v", err)
			}
		})
	}
}

func TestLerRaizRecusa(t *testing.T) {
	vazio := filepath.Join(t.TempDir(), "vazio.pem")
	if err := os.WriteFile(vazio, []byte("não é um certificado"), 0o600); err != nil {
		t.Fatal(err)
	}
	casos := []struct{ nome, caminho, trecho string }{
		{"sem -raiz", "", "obrigatório"},
		{"arquivo que não existe", filepath.Join(t.TempDir(), "nao.pem"), "-raiz"},
		{"sem certificado", vazio, "nenhum certificado"},
	}
	for _, c := range casos {
		if _, err := lerRaiz(c.caminho); err == nil || !strings.Contains(err.Error(), c.trecho) {
			t.Errorf("%s: %v", c.nome, err)
		}
	}
}
