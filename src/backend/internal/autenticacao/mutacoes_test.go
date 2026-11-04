package autenticacao

// Testes que o teste de mutação (#373) mostrou que faltavam (#399): em cada um, a mutação que ele pega.

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// Regra: a chave de sessão criada tem 32 bytes aleatórios, e a mesma volta na próxima partida.
// Pega: rand.Read com a condição invertida (a chave voltava nil, sem erro).
func TestChaveCriadaTem32Bytes(t *testing.T) {
	pasta := filepath.Join(t.TempDir(), "chaves")
	chave, err := CarregarChave(pasta)
	if err != nil || len(chave) != tamanhoChaveAssinatura || bytes.Equal(chave, make([]byte, tamanhoChaveAssinatura)) {
		t.Fatalf("chave nova: %d bytes, %v", len(chave), err)
	}
	if deNovo, err := CarregarChave(pasta); err != nil || !bytes.Equal(deNovo, chave) {
		t.Errorf("na segunda partida veio outra chave: %v", err)
	}
}

// cookieAssinado assina um conteúdo qualquer com a chave de teste, como o valorSessao faz com o JSON da sessão.
func cookieAssinado(dados []byte) string {
	return b64.EncodeToString(dados) + "." + b64.EncodeToString(assinar(chaveDeTeste, usoSessao, dados))
}

// Regra: mesmo com a assinatura certa, conteúdo que não é a sessão (JSON quebrado, usuário 0 ou negativo) não abre.
// Pega: "|| s.Usuario <= 0" virando "&&" e "<= 0" virando "< 0".
func TestSessaoAssinadaComConteudoInvalido(t *testing.T) {
	agora := time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)
	expira := agora.Add(time.Hour).Unix()
	for nome, dados := range map[string]string{
		"JSON quebrado com usuário": `{"u":1,"x":` + fmt.Sprint(expira),
		"usuário 0":                 fmt.Sprintf(`{"u":0,"x":%d}`, expira),
		"usuário negativo":          fmt.Sprintf(`{"u":-1,"x":%d}`, expira),
	} {
		t.Run(nome, func(t *testing.T) {
			if s, ok := lerSessao(chaveDeTeste, cookieAssinado([]byte(dados)), agora); ok {
				t.Errorf("aceitou: %+v", s)
			}
		})
	}
}

// Regra: CSRF com o segredo do cookie vazio nunca vale, nem com o token que a chave daria para ele.
// Pega: "segredo == "" || cabecalho == """ virando "&&".
func TestCsrfComSegredoVazio(t *testing.T) {
	if csrfValido(chaveDeTeste, "", tokenCsrf(chaveDeTeste, "", 1), 1) {
		t.Error("segredo vazio valeu")
	}
	if csrfValido(chaveDeTeste, "segredo", "", 1) {
		t.Error("cabeçalho vazio valeu")
	}
}

// Regra: com 1024 IPs no mapa, as janelas vencidas saem (o mapa não cresce sem fim).
// Pega: ">= 1024" virando "> 1024" e "< 1024", e a janela vencida com exatamente a duração (">=" virando ">").
func TestLimitadorLimpaAsJanelasVencidas(t *testing.T) {
	l := novoLimitador(5, time.Minute)
	agora := time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)
	l.agora = func() time.Time { return agora }
	for i := range 1024 {
		l.permitir(fmt.Sprintf("10.0.%d.%d", i/256, i%256))
	}
	agora = agora.Add(time.Minute) // exatamente a duração: a janela já venceu (>=)
	l.permitir("10.9.9.9")
	if len(l.janelas) != 1 {
		t.Errorf("%d janelas no mapa, esperado 1 (as 1024 vencidas saem)", len(l.janelas))
	}
	// Abaixo de 1024, não varre: as vencidas ficam até a próxima limpeza.
	l2 := novoLimitador(5, time.Minute)
	agora2 := agora
	l2.agora = func() time.Time { return agora2 }
	l2.permitir("10.0.0.1")
	agora2 = agora2.Add(2 * time.Minute)
	l2.permitir("10.0.0.2")
	if len(l2.janelas) != 2 {
		t.Errorf("%d janelas, esperado 2 (com poucos IPs, não varre)", len(l2.janelas))
	}
}

// hashV3 monta um hash v3 do ASP.NET Identity com a subchave do tamanho dado (o .NET aceita a partir de 16 bytes).
func hashV3(senha string, subchave int) string {
	sal := bytes.Repeat([]byte{7}, 16)
	chave, _ := pbkdf2.Key(sha512.New, senha, sal, 1000, subchave)
	dados := make([]byte, cabecalhoV3, cabecalhoV3+len(sal)+len(chave))
	dados[0] = 0x01
	binary.BigEndian.PutUint32(dados[1:], prfSHA512)
	binary.BigEndian.PutUint32(dados[5:], 1000)
	binary.BigEndian.PutUint32(dados[9:], uint32(len(sal)))
	dados = append(append(dados, sal...), chave...)
	return base64.StdEncoding.EncodeToString(dados)
}

// Regra: subchave de 16 bytes (128 bits, o mínimo do .NET) vale; de 15, não.
// Pega: "len(esperada) < 16" virando "<= 16".
func TestHashV3ComSubchaveDe16Bytes(t *testing.T) {
	if got := VerificarHash(hashV3("Senha-Forte-123", 16), "Senha-Forte-123"); got == SenhaErrada {
		t.Error("subchave de 16 bytes recusada")
	}
	if got := VerificarHash(hashV3("Senha-Forte-123", 15), "Senha-Forte-123"); got != SenhaErrada {
		t.Errorf("subchave de 15 bytes aceita: %v", got)
	}
}

// Regra: configurar e entrar gravam a auditoria junto (o SaveChanges do .NET).
// Pega: o "if err != nil" da inserção do usuário e o da atualização no login, invertidos (a auditoria sumia).
func TestConfigurarEEntrarGravamAuditoria(t *testing.T) {
	a := novoAmbiente(t)
	n := a.navegador()
	n.esperar(t, "POST", "/api/auth/configurar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"autenticado":true`)
	n.esperar(t, "GET", "/api/auth/auditoria", nil, 200, `"evento":"Configuracao"`)
	outro := a.navegador()
	outro.esperar(t, "POST", "/api/auth/entrar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"login":"ana.teste"`)
	outro.esperar(t, "GET", "/api/auth/auditoria", nil, 200, `"evento":"LoginSucesso"`)
}

// Regra: no login, o hash antigo (v2 ou v3 com menos iterações) é refeito no v3 de hoje; o v3 de hoje fica igual.
// Pega: "resultado == SenhaCertaRefazer" invertido.
func TestLoginRefazOHashAntigo(t *testing.T) {
	a := novoAmbiente(t)
	n := a.navegador()
	n.esperar(t, "POST", "/api/auth/configurar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"autenticado":true`)
	hashDoBanco := func() string {
		var h string
		if err := a.auth.o.DB.QueryRowContext(t.Context(), `SELECT "SenhaHash" FROM "Usuario"`).Scan(&h); err != nil {
			t.Fatal(err)
		}
		return h
	}
	v3 := hashDoBanco()
	a.navegador().esperar(t, "POST", "/api/auth/entrar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"login":"ana.teste"`)
	if hashDoBanco() != v3 {
		t.Error("o hash v3 de hoje foi refeito sem precisar")
	}

	v2 := lerGabaritoSenhas(t).Casos[2].Hash // o v2 do .NET para "Senha-Forte-123"
	if _, err := a.auth.o.DB.ExecContext(t.Context(), `UPDATE "Usuario" SET "SenhaHash" = ?`, v2); err != nil {
		t.Fatal(err)
	}
	a.navegador().esperar(t, "POST", "/api/auth/entrar", credenciais{"ana.teste", "Senha-Forte-123"}, 200, `"login":"ana.teste"`)
	novo := hashDoBanco()
	if dados, _ := base64.StdEncoding.DecodeString(novo); novo == v2 || len(dados) == 0 || dados[0] != 0x01 {
		t.Errorf("o hash v2 não foi refeito no v3: %q", novo)
	}
	if VerificarHash(novo, "Senha-Forte-123") != SenhaCerta {
		t.Error("o hash refeito não confere com a senha")
	}
}
