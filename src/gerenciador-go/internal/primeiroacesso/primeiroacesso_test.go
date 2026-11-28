package primeiroacesso

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
)

const segredo = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// O mesmo valor do TestTokenDeConfiguracaoValorDeReferencia do site (autenticacao): o site precisa aceitar o que o
// gerenciador gera.
func TestTokenValorDeReferencia(t *testing.T) {
	const esperado = "1760000000.23c730ff718f1b04d3deb18b165ad8b68b6d169f5c9e6986e66c034025f9323f"
	if got := Token(segredo, time.Unix(1760000000, 0)); got != esperado {
		t.Errorf("token = %q, esperado %q", got, esperado)
	}
}

// Regra (#68, item 4): "Abrir o site", sem usuário criado, abre o link com o token; com usuário, sem poder ler o
// segredo, ou para outra página, abre o endereço de sempre.
func TestEndereco(t *testing.T) {
	agora := time.Unix(1760000000, 0)
	comChave := func(t *testing.T) string {
		pasta := t.TempDir()
		if err := os.WriteFile(filepath.Join(pasta, ArquivoChave), []byte(segredo+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return pasta
	}
	casos := []struct {
		nome    string
		status  int
		corpo   string
		pasta   func(*testing.T) string
		pagina  string
		esperar func(site string) string
	}{
		{"sem usuário, com o segredo", 200, `{"configurado":false}`, comChave, "",
			func(site string) string { return site + "entrar?configurar=" + Token(segredo, agora) }},
		{"com usuário", 200, `{"configurado":true}`, comChave, "", func(site string) string { return site }},
		{"sem usuário, sem poder ler o segredo", 200, `{"configurado":false}`, func(t *testing.T) string { return t.TempDir() }, "",
			func(site string) string { return site }},
		{"sem usuário, mas outra página", 200, `{"configurado":false}`, comChave, "manual", func(site string) string { return site + "manual" }},
		{"site com erro", 500, `{"configurado":false}`, comChave, "", func(site string) string { return site }},
		{"resposta que não é JSON", 200, `oi`, comChave, "", func(site string) string { return site }},
		{"resposta sem o campo", 200, `{}`, comChave, "", func(site string) string { return site }},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/auth/estado" {
					t.Errorf("caminho inesperado %s", r.URL.Path)
				}
				w.WriteHeader(c.status)
				_, _ = io.WriteString(w, c.corpo)
			}))
			defer srv.Close()
			inst := instalacao.Instalacao{Site: srv.URL + "/", PastaChaves: c.pasta(t)}
			got, aviso := Endereco(context.Background(), srv.Client(), inst, c.pagina, agora, nil)
			if esperado := c.esperar(inst.Site); got != esperado || aviso != "" {
				t.Errorf("Endereco = %q (aviso %q), esperado %q", got, aviso, esperado)
			}
		})
	}
}

// O site fora do ar não trava o "Abrir o site": abre a página de sempre.
func TestEnderecoComSiteForaDoAr(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	site := srv.URL + "/"
	srv.Close()
	inst := instalacao.Instalacao{Site: site, PastaChaves: t.TempDir()}
	if got, _ := Endereco(context.Background(), http.DefaultClient, inst, "", time.Now(), nil); got != site {
		t.Errorf("Endereco = %q, esperado %q", got, site)
	}
}

// Windows: sem poder ler o segredo, "Abrir o site" pede o token à instância elevada, uma vez, e só enquanto não há
// usuário. Recusar o UAC ou dar erro abre a página de sempre, com o aviso.
func TestEnderecoPedeAoElevado(t *testing.T) {
	agora := time.Unix(1760000000, 0)
	tokenBom := Token(segredo, agora)
	casos := []struct {
		nome     string
		corpo    string
		pagina   string
		elevado  func() (string, Resultado)
		chamadas int
		link     bool
		aviso    string
	}{
		{"sem usuário: usa o token do elevado", `{"configurado":false}`, "", func() (string, Resultado) { return tokenBom, Feito }, 1, true, ""},
		{"com usuário: nunca pede o UAC", `{"configurado":true}`, "", func() (string, Resultado) { return tokenBom, Feito }, 0, false, ""},
		{"outra página: não pede o UAC", `{"configurado":false}`, "manual", func() (string, Resultado) { return tokenBom, Feito }, 0, false, ""},
		{"UAC recusado: página de sempre e aviso", `{"configurado":false}`, "", func() (string, Resultado) { return "", Cancelado }, 1, false, AvisoCancelado},
		{"erro no elevado: página de sempre e aviso", `{"configurado":false}`, "", func() (string, Resultado) { return "", Falhou }, 1, false, AvisoFalhou},
		{"elevado diz Feito mas devolve lixo", `{"configurado":false}`, "", func() (string, Resultado) { return "lixo", Feito }, 1, false, AvisoFalhou},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, c.corpo) }))
			defer srv.Close()
			inst := instalacao.Instalacao{Site: srv.URL + "/", PastaChaves: t.TempDir()} // pasta sem o segredo: "não lê"
			chamadas := 0
			got, aviso := Endereco(context.Background(), srv.Client(), inst, c.pagina, agora, func() (string, Resultado) {
				chamadas++
				return c.elevado()
			})
			esperado := inst.Site + c.pagina
			if c.link {
				esperado = inst.Site + "entrar?configurar=" + tokenBom
			}
			if got != esperado || aviso != c.aviso || chamadas != c.chamadas {
				t.Errorf("Endereco = %q, aviso %q, pedidos ao elevado %d; esperado %q, %q, %d", got, aviso, chamadas, esperado, c.aviso, c.chamadas)
			}
		})
	}
}

// Lendo o segredo direto (Linux, ou Windows já elevado), o elevado nem é chamado.
func TestEnderecoComSegredoLegivelNaoPedeAoElevado(t *testing.T) {
	agora := time.Unix(1760000000, 0)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"configurado":false}`) }))
	defer srv.Close()
	pasta := t.TempDir()
	if err := os.WriteFile(filepath.Join(pasta, ArquivoChave), []byte(segredo), 0o600); err != nil {
		t.Fatal(err)
	}
	inst := instalacao.Instalacao{Site: srv.URL + "/", PastaChaves: pasta}
	got, _ := Endereco(context.Background(), srv.Client(), inst, "", agora, func() (string, Resultado) {
		t.Error("pediu ao elevado com o segredo legível")
		return "", Falhou
	})
	if got != inst.Site+"entrar?configurar="+Token(segredo, agora) {
		t.Errorf("Endereco = %q", got)
	}
}
