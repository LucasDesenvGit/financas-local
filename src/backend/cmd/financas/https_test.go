package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/config"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
)

// preparar roda o subcomando com o relógio do contrato e devolve o que ele escreveu.
func preparar(t *testing.T, argumentos ...string) (string, error) {
	t.Helper()
	var saida strings.Builder
	err := prepararHTTPS(argumentos, &saida, relogio.Fixo{Instante: inicioDoContrato})
	return saida.String(), err
}

func lerAppsettings(t *testing.T, arquivo string) appsettingsHTTPS {
	t.Helper()
	dados, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	var c appsettingsHTTPS
	if err := json.Unmarshal(dados, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// Regra (#775): o subcomando acha ou cria a senha, gera o .pfx ou reaproveita o que abre com ela, e grava a configuração
// com a mesma senha e o caminho do .pfx. Gerar de novo só quando o .pfx não abre com a senha que a configuração traz.
func TestPrepararHTTPS(t *testing.T) {
	casos := []struct {
		nome       string
		jaPreparou bool                      // roda o subcomando uma vez antes: o .pfx e a configuração existem
		mexer      func(texto string) string // muda a configuração que ficou (nil: deixa como está)
		emOutro    bool                      // a configuração a gravar é outro arquivo; a senha vem da anterior (-config-atual)
		saida      string
		mesmaSenha bool // a senha gravada é a de antes
	}{
		{nome: "o .pfx e a configuração faltam", saida: "gerado\n"},
		{nome: "o .pfx e a senha já estão lá", jaPreparou: true, saida: "reaproveitado\n", mesmaSenha: true},
		{
			nome: "o .pfx existe e a configuração perdeu a senha", jaPreparou: true, saida: "gerado\n",
			mexer: func(string) string { return "{}" },
		},
		{
			nome: "o .pfx existe e a senha da configuração não o abre", jaPreparou: true, saida: "gerado\n",
			mexer: func(string) string { return `{"Kestrel":{"Endpoints":{"Https":{"Certificate":{"Password":"outra"}}}}}` },
		},
		{
			nome: "a configuração anterior não é um JSON", jaPreparou: true, saida: "gerado\n",
			mexer: func(string) string { return "{isto não é json" },
		},
		{
			// O Windows PowerShell 5.1 pode gravar a marca do UTF-8: lida como parte do JSON, ela trocaria o certificado a cada atualização.
			nome: "a configuração anterior começa pela marca do UTF-8", jaPreparou: true, saida: "reaproveitado\n", mesmaSenha: true,
			mexer: func(texto string) string { return "\xef\xbb\xbf" + texto },
		},
		{
			// O 3-publicar-atualizar lê a senha do appsettings já publicado e grava o novo na pasta do preparo.
			nome: "a senha vem de outro arquivo (-config-atual)", jaPreparou: true, emOutro: true, saida: "reaproveitado\n", mesmaSenha: true,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pasta := t.TempDir()
			pfx, anterior := filepath.Join(pasta, "financaslocal.pfx"), filepath.Join(pasta, "appsettings.Production.json")
			senhaAntes := ""
			if c.jaPreparou {
				if saida, err := preparar(t, "-pfx", pfx, "-config", anterior); err != nil || saida != "gerado\n" {
					t.Fatalf("primeira chamada: %q, %v", saida, err)
				}
				dados, err := os.ReadFile(anterior)
				if err != nil {
					t.Fatal(err)
				}
				senhaAntes = lerAppsettings(t, anterior).Kestrel.Endpoints.Https.Certificate.Password
				if c.mexer != nil {
					gravar(t, anterior, c.mexer(string(dados)))
				}
			}
			config, argumentos := anterior, []string{"-pfx", pfx}
			if c.emOutro {
				config = filepath.Join(pasta, "novo.json")
				argumentos = append(argumentos, "-config-atual", anterior)
			}
			argumentos = append(argumentos, "-config", config)

			saida, err := preparar(t, argumentos...)
			if err != nil {
				t.Fatal(err)
			}
			if saida != c.saida {
				t.Errorf("saída = %q, esperada %q", saida, c.saida)
			}
			gravada := lerAppsettings(t, config).Kestrel.Endpoints.Https.Certificate
			if gravada.Password == "" || (gravada.Password == senhaAntes) != c.mesmaSenha {
				t.Errorf("senha gravada %q, de antes %q (a mesma: %v)", gravada.Password, senhaAntes, c.mesmaSenha)
			}
			if gravada.Path != filepath.ToSlash(pfx) {
				t.Errorf("caminho gravado %q, esperado %q", gravada.Path, filepath.ToSlash(pfx))
			}
			if _, err := servidor.CarregarPFX(pfx, gravada.Password); err != nil {
				t.Errorf("o .pfx não abre com a senha gravada: %v", err)
			}
			if _, err := os.Stat(strings.TrimSuffix(pfx, ".pfx") + ".pem"); err != nil {
				t.Errorf("faltou a parte pública (.pem): %v", err)
			}
		})
	}
}

func gravar(t *testing.T, arquivo, texto string) {
	t.Helper()
	if err := os.WriteFile(arquivo, []byte(texto), 0o600); err != nil {
		t.Fatal(err)
	}
}

// O que o subcomando grava é o que o site lê: o internal/plataforma/config acha o certificado, a senha, a URL, o banco
// e as chaves. Sem as opções, a configuração não leva Banco, Seguranca nem Url (o appsettings.json decide).
func TestPrepararHTTPSGravaOQueOSiteLe(t *testing.T) {
	pasta := t.TempDir()
	pfx, banco, chaves := filepath.Join(pasta, "chaves", "a.pfx"), filepath.Join(pasta, "sql", "FinancasLocal.db"), filepath.Join(pasta, "chaves")
	if err := os.Mkdir(chaves, 0o700); err != nil {
		t.Fatal(err)
	}
	arquivo := filepath.Join(pasta, "appsettings.Production.json")

	if _, err := preparar(t, "-pfx", pfx, "-config", arquivo, "-url", "https://127.0.0.1:5444", "-banco", banco, "-chaves", chaves); err != nil {
		t.Fatal(err)
	}
	lida, err := config.Carregar(pasta, "Production")
	if err != nil {
		t.Fatal(err)
	}
	esperado := map[string][2]string{
		"endereço":        {lida.Endereco, "127.0.0.1:5444"},
		"banco":           {lida.BancoCaminho, filepath.Clean(banco)},
		"pasta de chaves": {lida.PastaChaves, filepath.Clean(chaves)},
		"certificado":     {lida.CertificadoCaminho, filepath.Clean(pfx)},
	}
	for nome, par := range esperado {
		if par[0] != par[1] {
			t.Errorf("%s: o site leu %q, esperado %q", nome, par[0], par[1])
		}
	}
	if _, err := servidor.CarregarPFX(lida.CertificadoCaminho, lida.CertificadoSenha); err != nil {
		t.Errorf("o site não abriria o certificado: %v", err)
	}

	if _, err := preparar(t, "-pfx", pfx, "-config", arquivo); err != nil {
		t.Fatal(err)
	}
	dados, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	for _, chave := range []string{`"Banco"`, `"Seguranca"`, `"Url"`} {
		if strings.Contains(string(dados), chave) {
			t.Errorf("a configuração sem as opções traz %s:\n%s", chave, dados)
		}
	}
	if lida, err = config.Carregar(pasta, "Production"); err != nil || lida.Endereco != "127.0.0.1:5443" {
		t.Errorf("endereço padrão = %q (%v)", lida.Endereco, err)
	}
}

func TestPrepararHTTPSRecusa(t *testing.T) {
	pasta := t.TempDir()
	pfx, config := filepath.Join(pasta, "a.pfx"), filepath.Join(pasta, "a.json")
	casos := []struct {
		nome         string
		argumentos   []string
		trechoDoErro string
	}{
		{"sem -pfx", []string{"-config", config}, "informe -pfx e -config"},
		{"sem -config", []string{"-pfx", pfx}, "informe -pfx e -config"},
		{"opção desconhecida (a senha não vai pela linha de comando)", []string{"-pfx", pfx, "-config", config, "-senha", "x"}, "senha"},
		{"pasta do .pfx que não existe", []string{"-pfx", filepath.Join(pasta, "falta", "a.pfx"), "-config", config}, "certificado"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			saida, err := preparar(t, c.argumentos...)
			if err == nil || !strings.Contains(err.Error(), c.trechoDoErro) {
				t.Errorf("erro = %v", err)
			}
			if saida != "" {
				t.Errorf("saída = %q", saida)
			}
		})
	}
	for _, arquivo := range []string{pfx, config} {
		if _, err := os.Stat(arquivo); err == nil {
			t.Errorf("gravou %s mesmo recusando", arquivo)
		}
	}
}
