package instalacao

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func pastaCom(t *testing.T, arquivos map[string]string) string {
	t.Helper()
	pasta := t.TempDir()
	for nome, conteudo := range arquivos {
		if err := os.WriteFile(filepath.Join(pasta, nome), []byte(conteudo), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return pasta
}

// Regra: pelo .msi, a pasta de dados vem do Dados:Pasta do appsettings.Production.json, com as variáveis expandidas, e
// o endereço, do Kestrel.
func TestPeloMsi(t *testing.T) {
	dados := t.TempDir()
	t.Setenv("FINANCAS_TESTE", dados)
	pasta := pastaCom(t, map[string]string{
		"appsettings.Production.json": `{
		  // comentário, como o appsettings aceita
		  "Dados": { "Pasta": "%FINANCAS_TESTE%" },
		  "Kestrel": { "Endpoints": { "Https": { "Url": "https://127.0.0.1:5999/", }, }, },
		}`,
		"appsettings.json": `{"Banco": {"Caminho": "dados/sql/FinancasLocal.db"}}`,
	})
	i := Ler(pasta)
	if !i.PeloMsi || i.PastaDados != dados || i.CaminhoBanco != filepath.Join(dados, "sql", "FinancasLocal.db") ||
		i.Site != "https://127.0.0.1:5999/" || i.Pagina("manual") != "https://127.0.0.1:5999/manual" {
		t.Errorf("%+v", i)
	}
}

// Regra: pelos scripts, o banco vem do Banco:Caminho (relativo à pasta da API), e a pasta de dados é a de cima da
// "sql"; sem Kestrel, o endereço padrão.
func TestPelosScripts(t *testing.T) {
	t.Parallel()
	pasta := pastaCom(t, map[string]string{"appsettings.json": `{"Banco": {"Caminho": "dados/SQL/FinancasLocal.db"}}`})
	i := Ler(pasta)
	if i.PeloMsi || i.CaminhoBanco != filepath.Join(pasta, "dados", "SQL", "FinancasLocal.db") ||
		i.PastaDados != filepath.Join(pasta, "dados") || i.Site != "https://127.0.0.1:5443/" {
		t.Errorf("%+v", i)
	}
}

// Regra: o Banco:Caminho do appsettings.Production.json vale antes do geral; absoluto fica como está; fora de uma
// pasta "sql", a pasta de dados é a do banco.
func TestBancoDaProducaoEAbsoluto(t *testing.T) {
	t.Parallel()
	banco := filepath.Join(t.TempDir(), "outro", "banco.db")
	pasta := pastaCom(t, map[string]string{
		"appsettings.Production.json": `{"Banco": {"Caminho": ` + string(mustJSON(banco)) + `}}`,
		"appsettings.json":            `{"Banco": {"Caminho": "dados/sql/FinancasLocal.db"}, "Kestrel": {"Endpoints": {"Https": {"Url": "https://127.0.0.1:6000"}}}}`,
	})
	i := Ler(pasta)
	if i.CaminhoBanco != banco || i.PastaDados != filepath.Dir(banco) || i.Site != "https://127.0.0.1:6000/" {
		t.Errorf("%+v", i)
	}
}

// Regra: sem appsettings (ou com um que não lê), vale o padrão do .msi: %ProgramData%\FinancasLocal\dados.
func TestSemAppsettings(t *testing.T) {
	t.Setenv("ProgramData", filepath.Join(t.TempDir(), "PD"))
	pasta := pastaCom(t, map[string]string{"appsettings.json": `{ quebrado`})
	i := Ler(pasta)
	esperada := filepath.Join(os.Getenv("ProgramData"), "FinancasLocal", "dados")
	if !filepath.IsAbs(`C:\x`) {
		// No Linux, a barra invertida do padrão faz parte do nome; basta conferir o começo.
		esperada = filepath.Join(os.Getenv("ProgramData")) + `\FinancasLocal\dados`
	}
	if i.PeloMsi || i.PastaDados != esperada || i.Site != "https://127.0.0.1:5443/" {
		t.Errorf("%+v, esperada %s", i, esperada)
	}
}

// Regra: a pasta da API sai da linha do serviço, do .msi (Financas.Api.exe) ou dos scripts (dotnet + .dll).
func TestPastaDaLinhaDoServico(t *testing.T) {
	t.Setenv("PF_TESTE", `C:\Arquivos de Programas`)
	casos := map[string]string{
		`"C:\Program Files\FinancasLocal\Financas.Api.exe"`:                                           `C:\Program Files\FinancasLocal`,
		`"C:\Program Files\dotnet\dotnet.exe" "C:\Users\Ana\financas\publicado\api\financas.api.DLL"`: `C:\Users\Ana\financas\publicado\api`,
		`"%PF_TESTE%\FinancasLocal\Financas.Api.exe" --urls x`:                                        `C:\Arquivos de Programas\FinancasLocal`,
		`"C:\outro\programa.exe"`:                                                                     "",
	}
	for linha, esperada := range casos {
		if obtida := PastaDaLinhaDoServico(linha); obtida != esperada {
			t.Errorf("%s: %q", linha, obtida)
		}
	}
}

// Regra: comentários e vírgulas sobrando saem; dentro de texto, nada muda.
func TestLimparJSON(t *testing.T) {
	t.Parallel()
	casos := map[string]string{
		`{"a": "x // y", /* z */ "b": [1, 2, ], }`: `{"a": "x // y",  "b": [1, 2 ] }`,
		`{"a": "aspas \" , }"}`:                    `{"a": "aspas \" , }"}`,
		"{\"a\": 1 // fim\n}":                      "{\"a\": 1 \n}",
		`{"a": 1 /* sem fim`:                       `{"a": 1 `,
		`{"a": "sem fim`:                           `{"a": "sem fim`,
		`[,]`:                                      `[]`,
		`,]`:                                       `]`,
		`, ]`:                                      ` ]`,
		`{"a": /**/ 1}`:                            `{"a":  1}`,
		`{"a": 1}/`:                                `{"a": 1}/`,
		`{"a": 1 // sem quebra de linha`:           `{"a": 1 `,
		`]`:                                        `]`,
		` ]`:                                       ` ]`,
	}
	for entrada, esperada := range casos {
		if obtida := string(limparJSON([]byte(entrada))); obtida != esperada {
			t.Errorf("%s: %q, esperado %q", entrada, obtida, esperada)
		}
	}
}

// Regra: variável que não existe fica como está.
func TestExpandir(t *testing.T) {
	t.Setenv("FINANCAS_A", "um")
	if obtido := expandir(`%FINANCAS_A%\%FINANCAS_NAO_EXISTE%`); obtido != `um\%FINANCAS_NAO_EXISTE%` {
		t.Error(obtido)
	}
}

// Regra: valor que não é texto ou caminho que não existe não vale.
func TestTexto(t *testing.T) {
	t.Parallel()
	raiz := map[string]any{"a": map[string]any{"b": 1.0, "c": "x"}, "d": "y"}
	if _, ok := texto(raiz, "a", "b"); ok {
		t.Error("número como texto")
	}
	if _, ok := texto(raiz, "d", "e"); ok {
		t.Error("texto como objeto")
	}
	if v, ok := texto(raiz, "a", "c"); !ok || v != "x" {
		t.Error("a.c")
	}
}

func mustJSON(s string) []byte {
	b, _ := json.Marshal(s)
	return b
}

// Regra: no Linux, a pasta da API é a WorkingDirectory da unidade; sem ela (ou relativa, ou "~"), a pasta do
// Financas.Api.dll do ExecStart; unidade que não carregou não tem pasta.
func TestPastaDaUnidade(t *testing.T) {
	t.Parallel()
	execStart := "{ path=/home/ana/financas/ferramentas/dotnet/dotnet ; argv[]=/home/ana/financas/ferramentas/dotnet/dotnet " +
		"/home/ana/financas/publicado/api/Financas.Api.dll ; ignore_errors=no }"
	casos := []struct {
		nome     string
		p        map[string]string
		esperada string
	}{
		{"WorkingDirectory", map[string]string{"LoadState": "loaded", "WorkingDirectory": "/srv/api", "ExecStart": execStart}, "/srv/api"},
		{"sem WorkingDirectory", map[string]string{"LoadState": "loaded", "ExecStart": execStart}, "/home/ana/financas/publicado/api"},
		{"WorkingDirectory ~", map[string]string{"LoadState": "loaded", "WorkingDirectory": "~", "ExecStart": execStart}, "/home/ana/financas/publicado/api"},
		{"sem a API", map[string]string{"LoadState": "loaded", "ExecStart": "{ path=/usr/bin/outro ; argv[]=/usr/bin/outro }"}, ""},
		{"não carregou", map[string]string{"LoadState": "not-found", "WorkingDirectory": "/srv/api"}, ""},
	}
	for _, c := range casos {
		if obtida := PastaDaUnidade(c.p); obtida != c.esperada {
			t.Errorf("%s: %q, esperada %q", c.nome, obtida, c.esperada)
		}
	}
}
