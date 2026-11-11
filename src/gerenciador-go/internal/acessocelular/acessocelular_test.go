package acessocelular

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/elevacao"
	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
)

const chave = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// siteFalso responde a rota com o status e o corpo dados, e guarda o que recebeu.
type siteFalso struct {
	status    int
	corpo     string
	caminho   string
	metodo    string
	cabecalho string
}

func (s *siteFalso) iniciar(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.caminho, s.metodo, s.cabecalho = r.URL.Path, r.Method, r.Header.Get(Cabecalho)
		w.WriteHeader(s.status)
		_, _ = io.WriteString(w, s.corpo)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func pastaComChave(t *testing.T, conteudo string) string {
	t.Helper()
	pasta := t.TempDir()
	if err := os.WriteFile(filepath.Join(pasta, ArquivoChave), []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
	return pasta
}

// Regra (#684): o gerenciador manda o segredo (sem a quebra de linha do arquivo) no POST da rota de ligar ou de
// desligar, e traduz a resposta do site.
func TestMudar(t *testing.T) {
	t.Parallel()
	ligado := `{"ligado":true,"avisos":[]}`
	desligado := `{"ligado":false,"avisos":[]}`
	casos := []struct {
		nome   string
		ligar  bool
		status int
		corpo  string
		quer   Resultado
	}{
		{"ligou", true, 200, ligado, Feito},
		{"desligou", false, 200, desligado, Feito},
		{"ligou com aviso", true, 200, `{"ligado":true,"avisos":["A rede está como Pública."]}`, ComAviso},
		{"desligado com aviso não é aviso", false, 200, `{"ligado":false,"avisos":["x"]}`, Feito},
		{"pediu ligar e ficou desligado", true, 200, desligado, Falhou},
		{"pediu desligar e ficou ligado", false, 200, ligado, Falhou},
		{"sem o campo ligado", true, 200, `{"avisos":[]}`, Falhou},
		{"resposta que não se lê", true, 200, `{`, Falhou},
		{"site sem a rota ou recusou", true, 404, ``, SemPermissao},
		{"sem rede de casa", true, 409, `{"mensagem":"Este computador não está numa rede de casa (Wi-Fi ou cabo): não há endereço para o celular abrir."}`, SemRede},
		{"porta ou firewall", true, 409, `{"mensagem":"Não deu para abrir a porta no firewall do Windows."}`, PortaFechada},
		{"409 que não se lê", true, 409, `{`, PortaFechada},
		{"erro do site", true, 500, ``, Falhou},
		{"400", false, 400, `{"mensagem":"x"}`, Falhou},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			site := &siteFalso{status: c.status, corpo: c.corpo}
			srv := site.iniciar(t)
			if r := Mudar(t.Context(), srv.Client(), srv.URL+"/", pastaComChave(t, chave+"\r\n"), c.ligar); r != c.quer {
				t.Errorf("deu %d, esperado %d", r, c.quer)
			}
			rota := "/api/celular/gerenciador/desligar"
			if c.ligar {
				rota = "/api/celular/gerenciador/ligar"
			}
			if site.metodo != "POST" || site.caminho != rota || site.cabecalho != chave {
				t.Errorf("pediu %s %s com %q", site.metodo, site.caminho, site.cabecalho)
			}
		})
	}
}

// Regra (#684): sem o arquivo do segredo (sem permissão para ler, ou o site ainda não o criou), nem pede ao site; site
// que não responde é Falhou.
func TestMudarSemSegredoOuSemSite(t *testing.T) {
	t.Parallel()
	site := &siteFalso{status: 200, corpo: `{"ligado":true}`}
	srv := site.iniciar(t)
	if r := Mudar(t.Context(), srv.Client(), srv.URL+"/", t.TempDir(), true); r != SemPermissao || site.metodo != "" {
		t.Errorf("sem o arquivo: %d, pediu %q", r, site.metodo)
	}
	fora := httptest.NewTLSServer(http.NotFoundHandler())
	endereco := fora.URL + "/"
	fora.Close()
	if r := Mudar(t.Context(), srv.Client(), endereco, pastaComChave(t, chave), true); r != Falhou {
		t.Errorf("site fora do ar: %d", r)
	}
	if r := Mudar(t.Context(), srv.Client(), "://quebrado/", pastaComChave(t, chave), true); r != Falhou {
		t.Errorf("endereço quebrado: %d", r)
	}
}

// Regra (#684): o código de saída do gerenciador elevado volta como o Resultado; desconhecido é Falhou.
func TestCodigos(t *testing.T) {
	t.Parallel()
	for codigo, quer := range map[uint32]Resultado{0: Feito, 1: Falhou, 2: ComAviso, 3: SemRede, 4: PortaFechada,
		5: SemPermissao, 6: Falhou, 99: Falhou} {
		if r := DoCodigo(codigo); r != quer {
			t.Errorf("código %d: %d, esperado %d", codigo, r, quer)
		}
	}
	casos := []struct {
		r      elevacao.Resultado
		codigo uint32
		quer   Resultado
	}{
		{elevacao.Feito, 0, Feito},
		{elevacao.Cancelado, 0, Cancelado},
		{elevacao.Falhou, 0, Falhou}, // nem rodou
		{elevacao.Falhou, 3, SemRede},
		{elevacao.Falhou, 2, ComAviso},
	}
	for _, c := range casos {
		if r := DoElevado(c.r, c.codigo); r != c.quer {
			t.Errorf("%d com %d: %d, esperado %d", c.r, c.codigo, r, c.quer)
		}
	}
}

// Regra (#684): só o que deu certo fica sem texto; o resto diz o que houve e o que fazer.
func TestTexto(t *testing.T) {
	t.Parallel()
	if Texto(Feito) != "" {
		t.Errorf("feito: %q", Texto(Feito))
	}
	vistos := map[string]bool{}
	for _, r := range []Resultado{Falhou, ComAviso, SemRede, PortaFechada, SemPermissao, Cancelado} {
		texto := Texto(r)
		if texto == "" || vistos[texto] {
			t.Errorf("%d: %q", r, texto)
		}
		vistos[texto] = true
	}
	for _, r := range []Resultado{Falhou, ComAviso, SemPermissao} {
		if !strings.Contains(Texto(r), "“Ver no site”") {
			t.Errorf("%d não diz para onde ir: %q", r, Texto(r))
		}
	}
}

// Regra (#684): pela instalação, o segredo vem da pasta das chaves dela e o pedido vai ao site dela.
func TestNaInstalacao(t *testing.T) {
	t.Parallel()
	site := &siteFalso{status: 404}
	srv := site.iniciar(t)
	// O certificado do servidor de teste não é confiável, mas o cliente da situação aceita no loopback.
	inst := instalacao.Instalacao{Site: srv.URL + "/", PastaChaves: pastaComChave(t, chave)}
	if r := NaInstalacao(inst, false); r != SemPermissao || site.cabecalho != chave || site.caminho != "/api/celular/gerenciador/desligar" {
		t.Errorf("deu %d, pediu %s com %q", r, site.caminho, site.cabecalho)
	}
}
