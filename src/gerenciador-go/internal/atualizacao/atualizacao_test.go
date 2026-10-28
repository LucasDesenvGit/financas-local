package atualizacao

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Regra: a versão é lida como no VersaoDoSistema do C#, com o "v" da tag e o resto ignorado.
func TestLerVersao(t *testing.T) {
	t.Parallel()
	casos := map[string]string{
		"2.44.1": "2.44.1", "v2.44.1": "2.44.1", " V2.44.1 ": "2.44.1", "2.44.1-dev.7": "2.44.1-dev.7",
		"2.44.1-dev.7+abc": "2.44.1-dev.7", "2.44.1+abc": "2.44.1", "2.44.1-rc.1": "2.44.1",
	}
	for texto, quer := range casos {
		if v, ok := LerVersao(texto); !ok || v.String() != quer {
			t.Errorf("%q: %v %v", texto, v, ok)
		}
	}
	if v, _ := LerVersao("2.0.0-dev.0"); !v.EhDesenvolvimento() {
		t.Error("dev.0 como publicada")
	}
	for _, ruim := range []string{"", "2.44", "versão 2.44.1", "a.b.c", "99999999999999999999.0.0", "1.0.0-dev.99999999999999999999"} {
		if _, ok := LerVersao(ruim); ok {
			t.Errorf("%q lida", ruim)
		}
	}
}

// Regra: a ordem das versões, com a publicada depois das de desenvolvimento do mesmo número.
func TestComparar(t *testing.T) {
	t.Parallel()
	ordem := []string{"1.9.9", "2.0.0-dev.0", "2.0.0-dev.1", "2.0.0-dev.2", "2.0.0-dev.10", "2.0.0", "2.0.1-dev.1", "2.0.1", "2.1.0", "3.0.0"}
	for i, a := range ordem {
		va, _ := LerVersao(a)
		for j, b := range ordem {
			vb, _ := LerVersao(b)
			if quer := comparar(i, j); va.Comparar(vb) != quer {
				t.Errorf("%s × %s: %d, esperado %d", a, b, va.Comparar(vb), quer)
			}
		}
	}
}

// Regra: "mais nova na base" olha só o X.Y.Z.
func TestMaisNovaNaBase(t *testing.T) {
	t.Parallel()
	casos := []struct {
		a, b string
		quer bool
	}{
		{"2.25.1", "2.25.0", true}, {"2.25.0", "2.25.0-dev.9", false}, {"2.25.0-dev.9", "2.25.0", false},
		{"3.0.0", "2.99.99", true}, {"2.24.9", "2.25.0", false}, {"2.25.0", "2.24.9", true}, {"2.26.0", "2.25.9", true},
	}
	for _, c := range casos {
		a, _ := LerVersao(c.a)
		b, _ := LerVersao(c.b)
		if a.MaisNovaNaBase(b) != c.quer {
			t.Errorf("%s > %s", c.a, c.b)
		}
	}
}

// Regra: o hash da linha do arquivo, no formato do sha256sum (com ou sem o "*" do modo binário).
func TestEsperado(t *testing.T) {
	t.Parallel()
	somas := "abc123  outro.msi\r\nDEF456 *FinancasLocal-2.44.1.msi\n\nsó-uma-parte\n"
	if h := Esperado(somas, "FinancasLocal-2.44.1.msi"); h != "DEF456" {
		t.Errorf("%q", h)
	}
	if h := Esperado(somas, "outro.msi"); h != "abc123" {
		t.Errorf("%q", h)
	}
	if h := Esperado(somas, "nao-existe.msi"); h != "" {
		t.Errorf("%q", h)
	}
}

// githubFalso responde à API e aos arquivos de uma versão, com o .msi e as somas dados.
type githubFalso struct {
	status  int // da API
	release string
	msi     []byte
	somas   string
	pedidos []string
	agentes []string
	aceitas []string
}

func (g *githubFalso) servir(t *testing.T) (*httptest.Server, Cliente) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.pedidos = append(g.pedidos, r.URL.Path)
		g.agentes = append(g.agentes, r.Header.Get("User-Agent"))
		g.aceitas = append(g.aceitas, r.Header.Get("Accept"))
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"), strings.HasSuffix(r.URL.Path, "/releases/tags/desenvolvimento"):
			if g.status != 0 {
				w.WriteHeader(g.status)
			}
			_, _ = w.Write([]byte(strings.ReplaceAll(g.release, "{base}", "https://"+r.Host)))
		case r.URL.Path == "/arquivos/FinancasLocal-2.45.0.msi":
			w.Header().Set("Content-Length", strconv.Itoa(len(g.msi)))
			_, _ = w.Write(g.msi)
		case r.URL.Path == "/arquivos/cortado.msi":
			// Diz um tamanho e manda menos: a conexão cai no meio do download.
			w.Header().Set("Content-Length", "1000")
			_, _ = w.Write([]byte("pouco"))
		case r.URL.Path == "/arquivos/vazio.msi":
			w.Header().Set("Content-Length", "0")
		case r.URL.Path == "/arquivos/SHA256SUMS.txt":
			_, _ = w.Write([]byte(g.somas))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, Cliente{HTTP: srv.Client(), Base: srv.URL + "/repos/x/y/", Versao: "2.44.1", Pasta: t.TempDir()}
}

const releaseCompleto = `{"tag_name":"v2.45.0","name":"Finanças Local 2.45.0","body":"## O que mudou","html_url":"https://github.com/x/y/releases/tag/v2.45.0",
 "assets":[{"name":"FinancasLocal-2.45.0.MSI","browser_download_url":"{base}/arquivos/FinancasLocal-2.45.0.msi"},
           {"name":"sha256sums.txt","browser_download_url":"{base}/arquivos/SHA256SUMS.txt"}]}`

// Regra: a última publicada vem do releases/latest, com o .msi, as somas, as notas e a página.
func TestProcurarPublicada(t *testing.T) {
	t.Parallel()
	g := &githubFalso{release: releaseCompleto}
	_, c := g.servir(t)
	d, err := c.Procurar(context.Background(), false)
	if err != nil || d == nil {
		t.Fatalf("%v %v", d, err)
	}
	if d.Versao.String() != "2.45.0" || d.Tag != "v2.45.0" || d.Notas != "## O que mudou" || d.NomeMsi != "FinancasLocal-2.45.0.MSI" ||
		!strings.HasSuffix(d.Msi, "/arquivos/FinancasLocal-2.45.0.msi") || !strings.HasSuffix(d.Somas, "/arquivos/SHA256SUMS.txt") ||
		d.Pagina != "https://github.com/x/y/releases/tag/v2.45.0" {
		t.Errorf("%+v", d)
	}
	if g.pedidos[0] != "/repos/x/y/releases/latest" || g.agentes[0] != "FinancasLocal-Gerenciador/2.44.1" ||
		g.aceitas[0] != "application/vnd.github+json" {
		t.Errorf("pedido %v %v %v", g.pedidos, g.agentes, g.aceitas)
	}
	// Status 299 ainda é sucesso.
	g299 := &githubFalso{status: 299, release: releaseCompleto}
	_, c299 := g299.servir(t)
	if d, err := c299.Procurar(context.Background(), false); err != nil || d == nil {
		t.Errorf("299: %v %v", d, err)
	}
}

// Regra: a de desenvolvimento vem da tag fixa "desenvolvimento", com o número pelo título.
func TestProcurarDesenvolvimento(t *testing.T) {
	t.Parallel()
	g := &githubFalso{release: `{"tag_name":"desenvolvimento","name":"Finanças Local 2.45.0-dev.12 (develop)",
		"assets":[{"name":"FinancasLocal-2.45.0-dev.12.msi","browser_download_url":"{base}/x.msi"}]}`}
	_, c := g.servir(t)
	d, err := c.Procurar(context.Background(), true)
	if err != nil || d == nil || d.Versao.String() != "2.45.0-dev.12" || d.Somas != "" ||
		d.Pagina != "https://github.com/LucasDesenvGit/financas-local/releases" {
		t.Fatalf("%+v %v", d, err)
	}
	if g.pedidos[0] != "/repos/x/y/releases/tags/desenvolvimento" {
		t.Errorf("%v", g.pedidos)
	}
}

// Regra: sem versão (404), versão do tipo errado, número que não lê ou sem .msi: nada a oferecer, sem erro.
func TestProcurarSemNada(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome            string
		status          int
		release         string
		desenvolvimento bool
	}{
		{"404", http.StatusNotFound, `{}`, false},
		{"publicada procurando desenvolvimento", 0, `{"tag_name":"desenvolvimento","name":"Finanças Local 2.45.0","assets":[{"name":"a.msi","browser_download_url":"x"}]}`, true},
		{"desenvolvimento procurando publicada", 0, `{"tag_name":"v2.45.0-dev.3","assets":[{"name":"a.msi","browser_download_url":"x"}]}`, false},
		{"sem número", 0, `{"tag_name":"ultima","assets":[{"name":"a.msi","browser_download_url":"x"}]}`, false},
		{"sem .msi", 0, `{"tag_name":"v2.45.0","assets":[{"name":"codigo.zip","browser_download_url":"x"}]}`, false},
	}
	for _, caso := range casos {
		g := &githubFalso{status: caso.status, release: caso.release}
		_, c := g.servir(t)
		if d, err := c.Procurar(context.Background(), caso.desenvolvimento); d != nil || err != nil {
			t.Errorf("%s: %+v %v", caso.nome, d, err)
		}
	}
}

// Regra: erro do GitHub ou resposta que não lê é erro (a tela diz "sem internet?").
func TestProcurarComErro(t *testing.T) {
	t.Parallel()
	for _, caso := range []githubFalso{{status: http.StatusForbidden, release: releaseCompleto}, {status: 299, release: `{`}, {status: 300, release: releaseCompleto}} {
		g := caso
		_, c := g.servir(t)
		if d, err := c.Procurar(context.Background(), false); err == nil {
			t.Errorf("%d: sem erro, %+v", caso.status, d)
		}
	}
	c := Cliente{HTTP: http.DefaultClient, Base: "http://127.0.0.1:1/"}
	if _, err := c.Procurar(context.Background(), false); err == nil {
		t.Error("sem servidor: sem erro")
	}
	if _, err := (Cliente{HTTP: http.DefaultClient, Base: "%zz"}).Procurar(context.Background(), false); err == nil {
		t.Error("endereço inválido: sem erro")
	}
}

func hashDe(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func disponivel(t *testing.T, g *githubFalso) (Cliente, Disponivel) {
	t.Helper()
	_, c := g.servir(t)
	d, err := c.Procurar(context.Background(), false)
	if err != nil || d == nil {
		t.Fatalf("%v %v", d, err)
	}
	return c, *d
}

// Regra: o .msi com o hash publicado é baixado, com o progresso até o fim.
func TestBaixarConfere(t *testing.T) {
	t.Parallel()
	msi := []byte(strings.Repeat("instalador ", 5000))
	g := &githubFalso{release: releaseCompleto, msi: msi, somas: strings.ToUpper(hashDe(msi)) + "  FinancasLocal-2.45.0.MSI\n"}
	c, d := disponivel(t, g)
	var ultimo float64
	caminho, err := c.Baixar(context.Background(), d, func(p float64) { ultimo = p })
	if err != nil {
		t.Fatal(err)
	}
	if gravado, _ := os.ReadFile(caminho); string(gravado) != string(msi) || filepath.Dir(caminho) != c.Pasta {
		t.Errorf("arquivo %s", caminho)
	}
	if ultimo != 1 {
		t.Errorf("progresso %v", ultimo)
	}
	// Os arquivos não pedem o formato da API.
	for i, p := range g.pedidos {
		if strings.HasPrefix(p, "/arquivos/") && g.aceitas[i] != "" {
			t.Errorf("Accept em %s: %q", p, g.aceitas[i])
		}
	}
}

// Regra: arquivo com o hash errado nunca fica no disco nem é instalado.
func TestBaixarHashErrado(t *testing.T) {
	t.Parallel()
	g := &githubFalso{release: releaseCompleto, msi: []byte("trocado no caminho"), somas: hashDe([]byte("o publicado")) + "  FinancasLocal-2.45.0.MSI"}
	c, d := disponivel(t, g)
	if _, err := c.Baixar(context.Background(), d, nil); !errors.Is(err, ErrHashDiferente) {
		t.Errorf("erro %v", err)
	}
	if arquivos, _ := os.ReadDir(c.Pasta); len(arquivos) != 0 {
		t.Errorf("sobrou %v", arquivos)
	}
}

// Regra: sem o SHA256SUMS.txt, ou sem a linha do .msi nele, nada é baixado.
func TestBaixarSemSomas(t *testing.T) {
	t.Parallel()
	g := &githubFalso{release: releaseCompleto, msi: []byte("x"), somas: "abc  outro.msi"}
	c, d := disponivel(t, g)
	if _, err := c.Baixar(context.Background(), d, nil); err == nil || !strings.Contains(err.Error(), "não tem o FinancasLocal-2.45.0.MSI") {
		t.Errorf("sem a linha: %v", err)
	}
	d.Somas = ""
	if _, err := c.Baixar(context.Background(), d, nil); !errors.Is(err, ErrSemSomas) {
		t.Errorf("sem somas: %v", err)
	}
	for _, p := range g.pedidos {
		if strings.HasSuffix(p, ".msi") {
			t.Errorf("baixou o .msi: %v", g.pedidos)
		}
	}
}

// Regra: download cancelado ou que falha não deixa arquivo.
func TestBaixarCanceladoOuFalhou(t *testing.T) {
	t.Parallel()
	msi := []byte(strings.Repeat("x", 1000))
	g := &githubFalso{release: releaseCompleto, msi: msi, somas: hashDe(msi) + "  FinancasLocal-2.45.0.MSI"}
	c, d := disponivel(t, g)
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	if _, err := c.Baixar(ctx, d, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelado: %v", err)
	}
	d.Msi = strings.Replace(d.Msi, "FinancasLocal-2.45.0.msi", "sumiu.msi", 1)
	if _, err := c.Baixar(context.Background(), d, nil); err == nil {
		t.Error("404 no .msi: sem erro")
	}
	if arquivos, _ := os.ReadDir(c.Pasta); len(arquivos) != 0 {
		t.Errorf("sobrou %v", arquivos)
	}
	d.Somas = strings.Replace(d.Somas, "SHA256SUMS.txt", "sumiu.txt", 1)
	if _, err := c.Baixar(context.Background(), d, nil); err == nil {
		t.Error("404 nas somas: sem erro")
	}
}

// Regra: sem conseguir criar a pasta, erro.
func TestBaixarSemPasta(t *testing.T) {
	t.Parallel()
	msi := []byte("x")
	g := &githubFalso{release: releaseCompleto, msi: msi, somas: hashDe(msi) + "  FinancasLocal-2.45.0.MSI"}
	c, d := disponivel(t, g)
	arquivo := filepath.Join(t.TempDir(), "arquivo")
	if err := os.WriteFile(arquivo, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	c.Pasta = filepath.Join(arquivo, "dentro")
	if _, err := c.Baixar(context.Background(), d, nil); err == nil {
		t.Error("sem erro")
	}
}

// Regra: o cliente de verdade usa a API do repositório e a pasta temporária do sistema.
func TestNovoCliente(t *testing.T) {
	t.Parallel()
	c := NovoCliente("2.44.1")
	if c.Base != "https://api.github.com/repos/LucasDesenvGit/financas-local/" || c.Versao != "2.44.1" ||
		c.Pasta != filepath.Join(os.TempDir(), "FinancasLocal") || c.HTTP.Timeout.Minutes() != 15 {
		t.Errorf("%+v", c)
	}
}

// Regra: a conexão que cai no meio do download é erro de download (e não "hash diferente"), sem deixar arquivo.
func TestBaixarCortado(t *testing.T) {
	t.Parallel()
	g := &githubFalso{release: releaseCompleto, somas: hashDe([]byte("pouco")) + "  FinancasLocal-2.45.0.MSI"}
	c, d := disponivel(t, g)
	d.Msi = strings.Replace(d.Msi, "FinancasLocal-2.45.0.msi", "cortado.msi", 1)
	_, err := c.Baixar(context.Background(), d, nil)
	if err == nil || errors.Is(err, ErrHashDiferente) {
		t.Errorf("erro %v", err)
	}
	if arquivos, _ := os.ReadDir(c.Pasta); len(arquivos) != 0 {
		t.Errorf("sobrou %v", arquivos)
	}
}

// Regra: sem tamanho conhecido (zero), o progresso não é avisado (nada de dividir por zero).
func TestBaixarVazioSemProgresso(t *testing.T) {
	t.Parallel()
	g := &githubFalso{release: releaseCompleto, somas: hashDe(nil) + "  FinancasLocal-2.45.0.MSI"}
	c, d := disponivel(t, g)
	d.Msi = strings.Replace(d.Msi, "FinancasLocal-2.45.0.msi", "vazio.msi", 1)
	avisos := 0
	if _, err := c.Baixar(context.Background(), d, func(float64) { avisos++ }); err != nil || avisos != 0 {
		t.Errorf("erro %v, %d avisos", err, avisos)
	}
}

// Regra: o instalador abre o .msi com o log ao lado.
func TestComandoDeInstalar(t *testing.T) {
	t.Parallel()
	msi := filepath.Join("C:", "Temp", "FinancasLocal-2.45.0.msi")
	cmd := comandoDeInstalar(msi)
	quer := []string{"msiexec.exe", "/i", msi, "/l*v", filepath.Join("C:", "Temp", "FinancasLocal-2.45.0.log")}
	if strings.Join(cmd.Args, "|") != strings.Join(quer, "|") {
		t.Errorf("%v", cmd.Args)
	}
}
