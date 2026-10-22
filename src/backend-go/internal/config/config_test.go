package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// absoluto devolve um caminho absoluto no sistema em que o teste roda: "/srv/x" no Linux e "C:/srv/x" no Windows,
// onde "/srv/x" não é absoluto (sem a letra do disco) e seria resolvido a partir da pasta base.
func absoluto(p string) string {
	if runtime.GOOS == "windows" {
		return "C:" + p
	}
	return p
}

func escrever(t *testing.T, pasta, nome, conteudo string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(pasta, nome), []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
}

// O appsettings.json da API e o appsettings.Production.json que o 3-publicar-atualizar.sh gera.
const (
	appsettingsDaAPI = `{
  "Banco": { "Caminho": "dados/sql/FinancasLocal.db" },
  "Seguranca": { "PastaChaves": "dados/chaves", "MinutosSessao": 120 },
  "Kestrel": { "Endpoints": { "Https": { "Url": "https://127.0.0.1:5443" } } },
  "Pluggy": { "UrlBase": "https://api.pluggy.ai/" },
  "AllowedHosts": "localhost;127.0.0.1"
}`
	producaoDosScripts = `{
  "Banco": { "Caminho": "/srv/financas/dados/sql/FinancasLocal.db" },
  "Seguranca": { "PastaChaves": "/srv/financas/dados/chaves" },
  "Kestrel": { "Endpoints": { "Https": { "Certificate": { "Path": "/srv/financas/dados/chaves/financaslocal.pfx", "Password": "segredo" } } } }
}`
)

func TestCarregarComoOsScriptsGeram(t *testing.T) {
	pasta := t.TempDir()
	escrever(t, pasta, "appsettings.json", appsettingsDaAPI)
	escrever(t, pasta, "appsettings.Production.json", strings.ReplaceAll(producaoDosScripts, `"/srv/`, `"`+absoluto("/srv/")))

	c, err := Carregar(pasta, "")
	if err != nil {
		t.Fatal(err)
	}
	esperado := Config{
		PastaBase: pasta, BancoCaminho: filepath.Clean(absoluto("/srv/financas/dados/sql/FinancasLocal.db")),
		PastaChaves: filepath.Clean(absoluto("/srv/financas/dados/chaves")), MinutosSessao: 120,
		PluggyURLBase: "https://api.pluggy.ai/", Endereco: "127.0.0.1:5443",
		CertificadoCaminho: filepath.Clean(absoluto("/srv/financas/dados/chaves/financaslocal.pfx")), CertificadoSenha: "segredo",
	}
	if c.BancoCaminho != esperado.BancoCaminho || c.PastaChaves != esperado.PastaChaves || c.Endereco != esperado.Endereco ||
		c.CertificadoCaminho != esperado.CertificadoCaminho || c.CertificadoSenha != esperado.CertificadoSenha ||
		c.MinutosSessao != 120 || c.PluggyURLBase != esperado.PluggyURLBase {
		t.Errorf("leu %+v", c)
	}
	if len(c.HostsPermitidos) != 2 || c.HostsPermitidos[0] != "localhost" {
		t.Errorf("AllowedHosts = %v", c.HostsPermitidos)
	}
}

func TestChavesSemDiferencaDeMaiusculas(t *testing.T) {
	pasta := t.TempDir()
	escrever(t, pasta, "appsettings.json", appsettingsDaAPI)
	escrever(t, pasta, "appsettings.Production.json", `{ "banco": { "caminho": "`+absoluto("/outro.db")+`" }, "seguranca": { "minutossessao": 30 } }`)
	c, err := Carregar(pasta, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.BancoCaminho != filepath.Clean(absoluto("/outro.db")) || c.MinutosSessao != 30 {
		t.Errorf("o arquivo do ambiente deveria ganhar, sem diferença de maiúsculas: %+v", c)
	}
}

func TestPastaDeDadosDoMsi(t *testing.T) {
	pasta, dados := t.TempDir(), t.TempDir()
	t.Setenv("PASTA_TESTE", dados)
	escrever(t, pasta, "appsettings.json", appsettingsDaAPI)
	escrever(t, pasta, "appsettings.Production.json", `{ "Dados": { "Pasta": "%PASTA_TESTE%" } }`)
	escrever(t, dados, "configuracao.json", `{ "Kestrel": { "Endpoints": { "Https": { "Certificate": { "Path": "`+
		filepath.ToSlash(filepath.Join(dados, "certificado", "financaslocal.pfx"))+`", "Password": "x" } } } } }`)

	c, err := Carregar(pasta, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.BancoCaminho != filepath.Join(dados, "sql", "FinancasLocal.db") || c.PastaChaves != filepath.Join(dados, "chaves") {
		t.Errorf("a pasta de dados deveria definir banco e chaves: %+v", c)
	}
	if c.CertificadoCaminho != filepath.Join(dados, "certificado", "financaslocal.pfx") || c.CertificadoSenha != "x" {
		t.Errorf("o configuracao.json deveria trazer o certificado: %+v", c)
	}
}

func TestCaminhoRelativoAPartirDaPastaBase(t *testing.T) {
	pasta := t.TempDir()
	escrever(t, pasta, "appsettings.json", appsettingsDaAPI)
	c, err := Carregar(pasta, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.BancoCaminho != filepath.Join(pasta, "dados", "sql", "FinancasLocal.db") {
		t.Errorf("BancoCaminho = %s", c.BancoCaminho)
	}
}

func TestSemArquivosUsaOPadrao(t *testing.T) {
	c, err := Carregar(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Endereco != "127.0.0.1:5443" || c.MinutosSessao != 120 {
		t.Errorf("padrão errado: %+v", c)
	}
}

func TestJSONInvalidoDaErro(t *testing.T) {
	pasta := t.TempDir()
	escrever(t, pasta, "appsettings.json", `{ "Banco": `)
	if _, err := Carregar(pasta, ""); err == nil {
		t.Error("appsettings.json quebrado deveria dar erro")
	}
}

func TestConferirSoLocal(t *testing.T) {
	for endereco, local := range map[string]bool{
		"127.0.0.1:5443": true, "localhost:5444": true, "[::1]:5443": true,
		"0.0.0.0:5443": false, "192.168.0.10:5443": false, ":5443": false,
	} {
		if err := ConferirSoLocal(endereco); (err == nil) != local {
			t.Errorf("ConferirSoLocal(%q) = %v", endereco, err)
		}
	}
}
