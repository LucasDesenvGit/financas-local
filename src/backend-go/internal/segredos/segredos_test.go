package segredos

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func abrir(t *testing.T, pasta string) *Cofre {
	t.Helper()
	c, err := Abrir(pasta)
	if err != nil {
		t.Fatalf("Abrir: %v", err)
	}
	return c
}

// Regra: o que se cifra volta igual, inclusive vazio e com acento ou emoji.
func TestCifrarEDecifrar(t *testing.T) {
	c := abrir(t, t.TempDir())
	for _, segredo := range []string{"", "a", "client-secret-123", "çãõ 🔑 \x00 fim", strings.Repeat("x", 10_000)} {
		gravado, err := c.Cifrar("Pluggy.ClientSecret", segredo)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(gravado, "v2:") || (len(segredo) > 8 && strings.Contains(gravado, segredo)) {
			t.Errorf("gravado %q não está no formato v2 cifrado", gravado[:min(len(gravado), 40)])
		}
		lido, err := c.Decifrar("Pluggy.ClientSecret", gravado)
		if err != nil || lido != segredo {
			t.Errorf("Decifrar = %q, %v; esperado %q", lido[:min(len(lido), 40)], err, segredo[:min(len(segredo), 40)])
		}
	}
}

// Regra: cada gravação sorteia um nonce novo; o mesmo segredo cifrado duas vezes sai diferente.
func TestNonceDiferenteACadaGravacao(t *testing.T) {
	c := abrir(t, t.TempDir())
	vistos := map[string]bool{}
	for range 100 {
		gravado, err := c.Cifrar("Brapi.Token", "o mesmo")
		if err != nil {
			t.Fatal(err)
		}
		bruto, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(gravado, "v2:"))
		nonce := string(bruto[:c.aead.NonceSize()])
		if vistos[nonce] {
			t.Fatal("nonce repetido")
		}
		vistos[nonce] = true
	}
}

// Regra: valor sem "v2:" (o do Data Protection) é ErrInformarDeNovo; "v2:" mexido, curto, de outra linha ou de outra
// instalação é ErrAdulterado. Nenhum erro traz o valor.
func TestDecifrarRecusa(t *testing.T) {
	c := abrir(t, t.TempDir())
	outro := abrir(t, t.TempDir())
	gravado, _ := c.Cifrar("Pluggy.ClientSecret", "segredo-de-verdade")
	bruto, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(gravado, "v2:"))
	mexido := bytes.Clone(bruto)
	mexido[len(mexido)-1] ^= 1
	deOutro, _ := outro.Cifrar("Pluggy.ClientSecret", "segredo-de-verdade")

	casos := []struct {
		nome, chave, gravado string
		erro                 error
	}{
		{"legado do Data Protection", "Pluggy.ClientSecret", "CfDJ8NZn1tJ0oQ1Gm1gBQ", ErrInformarDeNovo},
		{"vazio", "Pluggy.ClientSecret", "", ErrInformarDeNovo},
		{"prefixo de outra versão", "Pluggy.ClientSecret", "v3:" + strings.TrimPrefix(gravado, "v2:"), ErrInformarDeNovo},
		{"um bit trocado", "Pluggy.ClientSecret", "v2:" + base64.StdEncoding.EncodeToString(mexido), ErrAdulterado},
		{"base64 inválido", "Pluggy.ClientSecret", "v2:@@@", ErrAdulterado},
		{"curto demais", "Pluggy.ClientSecret", "v2:" + base64.StdEncoding.EncodeToString(bruto[:10]), ErrAdulterado},
		{"copiado para outra linha", "Brapi.Token", gravado, ErrAdulterado},
		{"de outra instalação", "Pluggy.ClientSecret", deOutro, ErrAdulterado},
	}
	for _, caso := range casos {
		lido, err := c.Decifrar(caso.chave, caso.gravado)
		if !errors.Is(err, caso.erro) || lido != "" {
			t.Errorf("%s: %q, %v; esperado %v", caso.nome, lido, err, caso.erro)
		}
		if err != nil && strings.Contains(err.Error(), "segredo-de-verdade") {
			t.Errorf("%s: o erro traz o segredo", caso.nome)
		}
	}
}

// Regra: a chave é criada uma vez e lida nas próximas partidas (o que foi cifrado continua se lendo), só o dono lê o
// arquivo, e no Windows o arquivo não guarda a chave crua (DPAPI).
func TestChaveCriadaEReaberta(t *testing.T) {
	pasta := filepath.Join(t.TempDir(), "chaves")
	c := abrir(t, pasta)
	gravado, _ := c.Cifrar("Pluggy.ClientSecret", "abc")
	if lido, err := abrir(t, pasta).Decifrar("Pluggy.ClientSecret", gravado); err != nil || lido != "abc" {
		t.Fatalf("reaberto: %q, %v", lido, err)
	}
	arquivo, err := os.Stat(filepath.Join(pasta, ArquivoChave))
	if err != nil {
		t.Fatal(err)
	}
	conteudo, _ := os.ReadFile(filepath.Join(pasta, ArquivoChave))
	if runtime.GOOS == "windows" {
		if len(conteudo) <= tamanhoChave {
			t.Errorf("no Windows o arquivo tem o blob do DPAPI, não a chave crua (%d bytes)", len(conteudo))
		}
	} else {
		if arquivo.Mode().Perm() != 0o600 {
			t.Errorf("permissão %v, esperado 0600", arquivo.Mode().Perm())
		}
		if len(conteudo) != tamanhoChave {
			t.Errorf("%d bytes, esperado %d", len(conteudo), tamanhoChave)
		}
	}
}

// Regra: arquivo de chave mexido é erro claro (sem pânico) e não é trocado por uma chave nova.
func TestChaveAdulterada(t *testing.T) {
	for _, nome := range []string{"um byte trocado", "cortado", "vazio", "grande demais"} {
		t.Run(nome, func(t *testing.T) {
			pasta := t.TempDir()
			abrir(t, pasta)
			caminho := filepath.Join(pasta, ArquivoChave)
			conteudo, _ := os.ReadFile(caminho)
			switch nome {
			case "um byte trocado":
				if runtime.GOOS != "windows" {
					t.Skip("fora do Windows a chave é crua: um byte trocado é só outra chave (o Decifrar recusa)")
				}
				conteudo[len(conteudo)/2] ^= 0xFF
			case "cortado":
				conteudo = conteudo[:len(conteudo)-5]
			case "vazio":
				conteudo = nil
			case "grande demais":
				if runtime.GOOS == "windows" {
					// O CryptUnprotectData lê o tamanho do próprio blob e ignora o que sobra no fim: a chave lida é a
					// certa, então o que estava cifrado continua se lendo (conferido em TestBytesAMaisNoWindows).
					t.Skip("no Windows o DPAPI ignora bytes a mais no fim do blob")
				}
				conteudo = append(conteudo, 1, 2, 3)
			}
			if err := os.WriteFile(caminho, conteudo, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Abrir(pasta)
			if err == nil || !strings.Contains(err.Error(), "chave dos segredos") || !strings.Contains(err.Error(), "informe os segredos de novo") {
				t.Errorf("erro %v, esperado um erro que diga o que fazer", err)
			}
			if depois, _ := os.ReadFile(caminho); !bytes.Equal(depois, conteudo) {
				t.Error("o arquivo de chave mexido foi trocado")
			}
		})
	}
}

// Comportamento do DPAPI: bytes a mais no fim do arquivo não mudam a chave lida.
func TestBytesAMaisNoWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("só no Windows")
	}
	pasta := t.TempDir()
	gravado, _ := abrir(t, pasta).Cifrar("Pluggy.ClientSecret", "abc")
	caminho := filepath.Join(pasta, ArquivoChave)
	conteudo, _ := os.ReadFile(caminho)
	if err := os.WriteFile(caminho, append(conteudo, 1, 2, 3), 0o600); err != nil {
		t.Fatal(err)
	}
	if lido, err := abrir(t, pasta).Decifrar("Pluggy.ClientSecret", gravado); err != nil || lido != "abc" {
		t.Errorf("%q, %v", lido, err)
	}
}
