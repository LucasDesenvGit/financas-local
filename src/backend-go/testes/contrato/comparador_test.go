package contrato

import (
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Estes testes rodam sempre (go test ./...), sem servidor: conferem o comparador e a normalização, e que cada caso
// tem o seu esperado gravado.

func deJSON(t *testing.T, s string) any {
	t.Helper()
	v, err := decodificar([]byte(s))
	if err != nil {
		t.Fatalf("%s: %v", s, err)
	}
	return v
}

func TestComparar(t *testing.T) {
	casos := []struct {
		nome, esperado, obtido string
		diferencas             []string // vazio = iguais
	}{
		{"iguais, chaves em outra ordem", `{"a":1,"b":[true,null,"x"]}`, `{"b":[true,null,"x"],"a":1}`, nil},
		{"escala do decimal", `{"valor":10.50}`, `{"valor":10.5}`, []string{"/valor: esperado número 10.50, obtido número 10.5"}},
		{"inteiro e decimal", `{"valor":100.0}`, `{"valor":100}`, []string{"/valor: esperado número 100.0, obtido número 100"}},
		{"número grande sem perder casas", `12345678901234567.12345678`, `12345678901234567.12345679`,
			[]string{"/: esperado número 12345678901234567.12345678, obtido número 12345678901234567.12345679"}},
		{"número e texto", `{"id":5}`, `{"id":"5"}`, []string{`/id: esperado número 5, obtido texto "5"`}},
		{"null e ausente", `{"login":null}`, `{}`, []string{"/login: esperado null, obtido ausente"}},
		{"campo a mais", `{}`, `{"novo":1}`, []string{"/novo: esperado ausente, obtido número 1"}},
		{"lista vazia e null", `{"itens":[]}`, `{"itens":null}`, []string{"/itens: esperado lista com 0 itens, obtido null"}},
		{"ordem dos itens importa", `[1,2]`, `[2,1]`, []string{"/0: esperado número 1, obtido número 2", "/1: esperado número 2, obtido número 1"}},
		{"lista de outro tamanho", `[1,2]`, `[1]`, []string{"/: esperado 2 itens, obtido 1"}},
		{"caminho aninhado", `{"a":[{"b":"x"}]}`, `{"a":[{"b":"y"}]}`, []string{`/a/0/b: esperado texto "x", obtido texto "y"`}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			difs := comparar("", deJSON(t, c.esperado), deJSON(t, c.obtido))
			if strings.Join(difs, "\n") != strings.Join(c.diferencas, "\n") {
				t.Errorf("diferenças:\n%s\nesperava:\n%s", strings.Join(difs, "\n"), strings.Join(c.diferencas, "\n"))
			}
		})
	}
}

func TestNormalizadorTexto(t *testing.T) {
	n := Normalizador{Inicio: time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC), Versao: "2.37.7", PastaDados: "/tmp/x/dados"}
	casos := []struct{ nome, entrada, saida string }{
		{"carimbo do EF Core logo depois do início", "2026-10-15 01:00:03.1234567", "<relógio +0 min, AAAA-MM-DD hh:mm:ss.f>"},
		{"carimbo do EF Core sem fração", "2026-10-15 01:16:00", "<relógio +16 min, AAAA-MM-DD hh:mm:ss.f>"},
		{"JSON com Z", "2026-10-15T01:00:03.5Z", "<relógio +0 min, AAAA-MM-DDThh:mm:ss.fZ>"},
		{"JSON sem fuso", "2026-10-15T01:00:03.5", "<relógio +0 min, AAAA-MM-DDThh:mm:ss.f>"},
		{"com deslocamento de Brasília", "2026-10-14T22:00:03-03:00", "<relógio +0 min, AAAA-MM-DDThh:mm:ss.f-03:00>"},
		{"hora local gravada como se fosse UTC fica crua", "2026-10-14 22:00:03.25", "2026-10-14 22:00:03.25"},
		{"data de negócio antes do início fica crua", "2026-10-14T09:00:00Z", "2026-10-14T09:00:00Z"},
		{"data sem hora fica crua", "2026-10-15", "2026-10-15"},
		{"mais de um dia depois fica crua", "2026-10-16 01:00:00", "2026-10-16 01:00:00"},
		{"GUID do EF Core", "2F00DEFB-672E-45EF-87D0-702995AE8D2E", "<GUID>"},
		{"GUID do JSON", "2f00defb-672e-45ef-87d0-702995ae8d2e", "<guid>"},
		{"pasta dos dados", "/tmp/x/dados/sql/FinancasLocal.db", "<dados>/sql/FinancasLocal.db"},
		{"texto comum", "Usuário ou senha inválidos.", "Usuário ou senha inválidos."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := n.texto(c.entrada); got != c.saida {
				t.Errorf("texto(%q) = %q, esperava %q", c.entrada, got, c.saida)
			}
		})
	}
}

func TestNormalizadorCorpo(t *testing.T) {
	n := Normalizador{Inicio: time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC), Versao: "2.37.7"}
	corpo := deJSON(t, `{"versao":"2.37.7","traceId":"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		"dotNet":".NET 10.0.3","tamanho":123456,"valor":-5000.0,"outraVersao":"2.37.6"}`)
	saida, err := n.corpo(corpo, map[string]string{"/dotNet": "texto", "/tamanho": "numero"})
	if err != nil {
		t.Fatal(err)
	}
	esperado := deJSON(t, `{"versao":"<versao>","traceId":"<traceId>","dotNet":"<texto>","tamanho":"<número>",
		"valor":-5000.0,"outraVersao":"2.37.6"}`)
	if difs := comparar("", esperado, saida); len(difs) > 0 {
		t.Error(strings.Join(difs, "\n"))
	}

	if _, err := n.corpo(deJSON(t, `{"dotNet":""}`), map[string]string{"/dotNet": "texto"}); err == nil {
		t.Error("texto vazio não pode virar <texto>: o campo sumiu e o contrato tem de ver")
	}
	if _, err := n.corpo(deJSON(t, `{}`), map[string]string{"/dotNet": "texto"}); err == nil {
		t.Error("campo pedido que não veio tem de dar erro")
	}
}

func TestHashSenha(t *testing.T) {
	b := []byte{0x01}
	b = binary.BigEndian.AppendUint32(b, 2)      // HMACSHA512
	b = binary.BigEndian.AppendUint32(b, 100000) // iterações
	b = binary.BigEndian.AppendUint32(b, 16)     // tamanho do sal
	b = append(b, make([]byte, 16+32)...)        // sal e chave
	if got, want := hashSenha(base64.StdEncoding.EncodeToString(b)), "<hash v3: prf 2, 100000 iterações, sal de 16 bytes, chave de 32 bytes>"; got != want {
		t.Errorf("hashSenha = %q, esperava %q", got, want)
	}
	for _, cru := range []string{"nao-e-base64!", base64.StdEncoding.EncodeToString([]byte{0x00, 1, 2})} {
		if got := hashSenha(cru); got != cru {
			t.Errorf("fora do formato v3 fica cru: hashSenha(%q) = %q", cru, got)
		}
	}
}

func TestCookie(t *testing.T) {
	// O .NET escreve os atributos em minúsculas; o net/http do Go, com maiúsculas e em outra ordem. Para o
	// navegador é o mesmo cookie.
	dotnet := cookie("__Host-Financas=CfDJ8abc; path=/; secure; samesite=strict; httponly")
	golang := cookie("__Host-Financas=outro-valor; Path=/; HttpOnly; Secure; SameSite=Strict")
	if dotnet != golang || dotnet != "__Host-Financas=<valor>; httponly; path=/; samesite=strict; secure" {
		t.Errorf("cookies diferentes: %q e %q", dotnet, golang)
	}
	if got := cookie("__Host-Financas=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/"); !strings.HasPrefix(got, "__Host-Financas=;") {
		t.Errorf("cookie apagado tem de continuar vazio: %q", got)
	}
}

func TestPonteiroESubstituir(t *testing.T) {
	v := deJSON(t, `{"itens":[{"id":7,"a/b":"x"}]}`)
	if got, _, err := ponteiro(v, "/itens/0/id"); err != nil || got.(interface{ String() string }).String() != "7" {
		t.Errorf("ponteiro /itens/0/id = %v, %v", got, err)
	}
	if got, _, err := ponteiro(v, "/itens/0/a~1b"); err != nil || got != "x" {
		t.Errorf("ponteiro com ~1 = %v, %v", got, err)
	}
	if _, _, err := ponteiro(v, "/itens/1/id"); err == nil {
		t.Error("índice fora da lista tem de dar erro")
	}
	s, err := substituir(`{"chave":"{{chave}}","id":{{id}}}`, map[string]string{"chave": "abc", "id": "7"})
	if err != nil || s != `{"chave":"abc","id":7}` {
		t.Errorf("substituir = %q, %v", s, err)
	}
	if _, err := substituir("/api/x/{{falta}}", nil); err == nil {
		t.Error("variável não capturada tem de dar erro")
	}
}

// TestCadaCasoTemEsperado confere, sem servidor, que os casos são válidos e que esperado/ tem exatamente os mesmos
// casos, na mesma ordem: caso novo sem esperado gravado do .NET não passa.
func TestCadaCasoTemEsperado(t *testing.T) {
	nomes, arquivos, err := lerCasos("casos")
	if err != nil {
		t.Fatal(err)
	}
	if len(nomes) == 0 {
		t.Fatal("nenhum caso em casos/")
	}
	for _, nome := range nomes {
		dados, err := os.ReadFile(filepath.Join("esperado", nome+".json"))
		if err != nil {
			t.Errorf("%s: sem esperado gravado (rodar-dotnet.sh --atualizar)", nome)
			continue
		}
		lista, err := decodificar(dados)
		if err != nil {
			t.Fatalf("esperado/%s.json: %v", nome, err)
		}
		var gravados, roteiro []string
		for _, item := range lista.([]any) {
			gravados = append(gravados, item.(map[string]any)["nome"].(string))
		}
		for _, c := range arquivos[nome].Casos {
			roteiro = append(roteiro, c.Nome)
		}
		if !slices.Equal(gravados, roteiro) {
			t.Errorf("%s: os casos de esperado/ não são os de casos/ (regrave com rodar-dotnet.sh --atualizar):\n  casos:    %q\n  esperado: %q",
				nome, roteiro, gravados)
		}
	}
	extras, _ := filepath.Glob(filepath.Join("esperado", "*.json"))
	for _, e := range extras {
		if !slices.Contains(nomes, strings.TrimSuffix(filepath.Base(e), ".json")) {
			t.Errorf("%s não tem casos/ correspondente", e)
		}
	}
}
