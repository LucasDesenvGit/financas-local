package primeiroacesso

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
)

// O nome do pipe é aleatório e só o formato certo é aceito pelo elevado: ele não escreve em qualquer pipe que lhe peçam.
func TestNomeDoCanal(t *testing.T) {
	a, err := NovoNomeDoCanal()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NovoNomeDoCanal()
	if a == b || !CanalValido(a) || !strings.HasPrefix(a, PrefixoDoCanal) {
		t.Errorf("nomes %q e %q", a, b)
	}
	for _, ruim := range []string{"", `\\.\pipe\outro`, PrefixoDoCanal, PrefixoDoCanal + "xyz", a + "0", a[:len(a)-1], `\\servidor\pipe\financas-primeiro-acesso-` + a[len(PrefixoDoCanal):], "/tmp/arquivo"} {
		if CanalValido(ruim) {
			t.Errorf("aceitou %q", ruim)
		}
	}
}

// O protocolo do canal: o elevado escreve só o token, numa linha; quem lê só aceita esse formato e no máximo 256 bytes.
func TestProtocoloDoCanal(t *testing.T) {
	token := Token(segredo, time.Unix(1760000000, 0))
	var canal bytes.Buffer
	if err := EscreverToken(&canal, token); err != nil {
		t.Fatal(err)
	}
	if got, err := LerToken(&canal); err != nil || got != token {
		t.Errorf("LerToken = %q, %v", got, err)
	}
	for nome, entrada := range map[string]string{
		"vazio": "", "sem quebra de linha": token, "outro formato": "ola\n", "caminho": "C:\\Windows\\win.ini\n",
		"token com lixo": token + "x\n", "grande demais": strings.Repeat("1", 300) + "\n", "maiúsculas": strings.ToUpper(token) + "\n",
	} {
		if got, err := LerToken(strings.NewReader(entrada)); err == nil {
			t.Errorf("%s: aceitou %q", nome, got)
		}
	}
}

// O elevado lê o segredo da pasta das chaves e devolve só o token (o mesmo que o gerenciador sem elevação geraria).
func TestGerarEEnviar(t *testing.T) {
	agora := time.Unix(1760000000, 0)
	pasta := t.TempDir()
	if err := os.WriteFile(filepath.Join(pasta, ArquivoChave), []byte(segredo+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var canal bytes.Buffer
	if err := GerarEEnviar(instalacao.Instalacao{PastaChaves: pasta}, agora, &canal); err != nil {
		t.Fatal(err)
	}
	if canal.String() != Token(segredo, agora)+"\n" {
		t.Errorf("escreveu %q", canal.String())
	}
	if strings.Contains(canal.String(), segredo) {
		t.Error("o segredo vazou no canal")
	}
	canal.Reset()
	if err := GerarEEnviar(instalacao.Instalacao{PastaChaves: t.TempDir()}, agora, &canal); err == nil || canal.Len() != 0 {
		t.Errorf("sem o segredo: erro %v, escreveu %q", err, canal.String())
	}
}

// Elevado recusa nome de canal fora do formato, sem tentar abrir nada (código 2); com o formato certo mas sem o pipe
// existir, falha (código 1); erro ao gerar também.
func TestElevado(t *testing.T) {
	chamou := false
	executar := func(io.Writer) error { chamou = true; return nil }
	if codigo := Elevado("/tmp/qualquer", executar); codigo != 2 || chamou {
		t.Errorf("canal inválido: código %d, executou %v", codigo, chamou)
	}
	nome, _ := NovoNomeDoCanal()
	if codigo := Elevado(nome, executar); codigo != 1 || chamou {
		t.Errorf("pipe que não existe: código %d, executou %v", codigo, chamou)
	}
}
