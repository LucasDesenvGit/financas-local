package autenticacao

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// gabaritoSenhas são hashes do PasswordHasher do .NET (testdata/sonda-senha): a referência é o .NET, e não o Go.
type gabaritoSenhas struct {
	Casos []struct {
		Senha   string `json:"senha"`
		Hash    string `json:"hash"`
		Formato string `json:"formato"`
	} `json:"casos"`
}

func lerGabaritoSenhas(t *testing.T) gabaritoSenhas {
	t.Helper()
	dados, err := os.ReadFile("testdata/senhas-dotnet.json")
	if err != nil {
		t.Fatal(err)
	}
	var g gabaritoSenhas
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Casos) == 0 {
		t.Fatal("gabarito vazio")
	}
	return g
}

// O Go aceita a senha com o hash que o .NET gravou, recusa a errada, e pede para refazer o hash mais fraco que o de
// hoje (v2, ou v3 com menos iterações), como o .NET.
func TestVerificarHashDoDotnet(t *testing.T) {
	for _, c := range lerGabaritoSenhas(t).Casos {
		esperado := SenhaCerta
		if c.Formato != "v3" { // v2 e v3 com menos iterações: certa, mas o hash é refeito no formato de hoje
			esperado = SenhaCertaRefazer
		}
		t.Run(c.Formato+" "+c.Senha, func(t *testing.T) {
			if got := VerificarHash(c.Hash, c.Senha); got != esperado {
				t.Errorf("senha certa: %v, esperado %v", got, esperado)
			}
			if got := VerificarHash(c.Hash, c.Senha+"x"); got != SenhaErrada {
				t.Errorf("senha errada: %v", got)
			}
			if got := VerificarHash(c.Hash, strings.ToUpper(c.Senha)); c.Senha != strings.ToUpper(c.Senha) && got != SenhaErrada {
				t.Errorf("a senha diferencia maiúsculas: %v", got)
			}
		})
	}
}

// O hash do Go sai no formato que o .NET grava hoje (a sonda confere que o .NET o aceita: testdata/sonda-senha).
func TestGerarHashNoFormatoV3(t *testing.T) {
	hash, err := GerarHash("Senha-Forte-123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "AQAAAAIAAYag") { // 0x01, PRF 2 (SHA-512), 100000 iterações: o prefixo visto no banco
		t.Errorf("prefixo %q", hash[:12])
	}
	dados, _ := base64.StdEncoding.DecodeString(hash)
	if len(dados) != cabecalhoV3+16+32 || binary.BigEndian.Uint32(dados[9:]) != 16 {
		t.Errorf("tamanho %d, sal %d", len(dados), binary.BigEndian.Uint32(dados[9:]))
	}
	outro, _ := GerarHash("Senha-Forte-123")
	if outro == hash {
		t.Error("o sal tem de ser aleatório: dois hashes da mesma senha saíram iguais")
	}
	if VerificarHash(hash, "Senha-Forte-123") != SenhaCerta || VerificarHash(hash, "Senha-Forte-124") != SenhaErrada {
		t.Error("ida e volta")
	}
}

// Hash estragado ou de formato desconhecido é senha errada, sem pânico.
func TestVerificarHashInvalido(t *testing.T) {
	bom, _ := GerarHash("Senha-Forte-123")
	dados, _ := base64.StdEncoding.DecodeString(bom)

	alterado := append([]byte(nil), dados...)
	alterado[len(alterado)-1] ^= 1
	salCurto := append([]byte(nil), dados...)
	binary.BigEndian.PutUint32(salCurto[9:], 8)
	prfDesconhecida := append([]byte(nil), dados...)
	binary.BigEndian.PutUint32(prfDesconhecida[1:], 9)
	formato9 := append([]byte(nil), dados...)
	formato9[0] = 0x09

	for nome, hash := range map[string]string{
		"vazio":                 "",
		"não é Base64":          "@@@",
		"chave alterada":        base64.StdEncoding.EncodeToString(alterado),
		"sal de 8 bytes":        base64.StdEncoding.EncodeToString(salCurto),
		"PRF desconhecida":      base64.StdEncoding.EncodeToString(prfDesconhecida),
		"formato 9":             base64.StdEncoding.EncodeToString(formato9),
		"só o cabeçalho":        base64.StdEncoding.EncodeToString(dados[:cabecalhoV3]),
		"v2 com tamanho errado": base64.StdEncoding.EncodeToString(append([]byte{0x00}, dados[:10]...)),
	} {
		t.Run(nome, func(t *testing.T) {
			if got := VerificarHash(hash, "Senha-Forte-123"); got != SenhaErrada {
				t.Errorf("%v", got)
			}
		})
	}
}

func TestValidarSenha(t *testing.T) {
	for _, c := range []struct{ nome, senha, login, esperado string }{
		{"curta", "1234567", "ana", "A senha deve ter pelo menos 8 caracteres."},
		{"8 caracteres", "12345678", "ana", ""},
		{"emoji conta 2 (UTF-16)", "😀😀😀ab", "ana", ""},
		{"contém o login", "xxANA.TESTExx", "ana.teste", "A senha não pode conter o nome de usuário."},
		{"login em outra caixa", "Ana.Teste-123", "ANA.teste", "A senha não pode conter o nome de usuário."},
	} {
		t.Run(c.nome, func(t *testing.T) {
			if got := validarSenha(c.senha, c.login); got != c.esperado {
				t.Errorf("%q", got)
			}
		})
	}
}
