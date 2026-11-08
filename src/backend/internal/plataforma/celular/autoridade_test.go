package celular

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

var agoraDosTestes = testeapoio.InstanteDosTestes

func novaAutoridadeDeTeste(t *testing.T) (*Autoridade, string) {
	t.Helper()
	pasta := filepath.Join(t.TempDir(), "chaves")
	a, err := CarregarAutoridade(pasta, agoraDosTestes)
	if err != nil {
		t.Fatal(err)
	}
	return a, pasta
}

// Regra (#328, ADR 0061): a autoridade é criada na primeira vez, com a chave só para o dono, e vale 10 anos. Ela só
// assina certificado de site (sem outra autoridade abaixo) e só para IPs da rede de casa, sem nenhum nome de DNS.
func TestCarregarAutoridadeCria(t *testing.T) {
	a, pasta := novaAutoridadeDeTeste(t)
	c := a.certificado
	if !c.IsCA || !c.BasicConstraintsValid || c.MaxPathLen != 0 || !c.MaxPathLenZero {
		t.Errorf("não é uma autoridade só de site: IsCA %v, MaxPathLen %d", c.IsCA, c.MaxPathLen)
	}
	if c.KeyUsage != x509.KeyUsageCertSign|x509.KeyUsageCRLSign {
		t.Errorf("KeyUsage %v", c.KeyUsage)
	}
	if len(c.ExtKeyUsage) != 1 || c.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Errorf("ExtKeyUsage %v", c.ExtKeyUsage)
	}
	var faixas []string
	for _, f := range c.PermittedIPRanges {
		faixas = append(faixas, f.String())
	}
	if strings.Join(faixas, " ") != "10.0.0.0/8 172.16.0.0/12 192.168.0.0/16" {
		t.Errorf("faixas %v", faixas)
	}
	if strings.Join(c.PermittedDNSDomains, " ") != "invalid" || c.PermittedDNSDomainsCritical {
		t.Errorf("nomes %v, crítica %v", c.PermittedDNSDomains, c.PermittedDNSDomainsCritical)
	}
	if !c.NotBefore.Equal(agoraDosTestes.Add(-time.Hour).Truncate(time.Second)) ||
		!c.NotAfter.Equal(agoraDosTestes.Add(10*365*24*time.Hour).Truncate(time.Second)) {
		t.Errorf("validade %v a %v", c.NotBefore, c.NotAfter)
	}
	if c.Subject.CommonName != "Finanças Local (celular)" {
		t.Errorf("nome %q", c.Subject.CommonName)
	}

	chave, err := os.Stat(filepath.Join(pasta, ArquivoChaveDaAutoridade))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && chave.Mode().Perm() != 0o600 {
		t.Errorf("a chave tem permissão %v, esperado 0600", chave.Mode().Perm())
	}
	publico, err := os.ReadFile(filepath.Join(pasta, ArquivoDaAutoridade))
	if err != nil {
		t.Fatal(err)
	}
	if bloco, _ := pem.Decode(publico); bloco == nil || bloco.Type != "CERTIFICATE" || string(bloco.Bytes) != string(a.DER()) {
		t.Error("o .crt não é o certificado da autoridade em PEM")
	}
	if _, err := os.Stat(filepath.Join(pasta, ArquivoChaveDaAutoridade+".novo")); !os.IsNotExist(err) {
		t.Errorf("sobrou o arquivo temporário: %v", err)
	}
}

// Regra (#328): a autoridade que já existe é a mesma (o celular confia nela); só troca quando faltam menos de 397 dias,
// para o certificado do site não passar da validade dela.
func TestCarregarAutoridadeReaproveita(t *testing.T) {
	a, pasta := novaAutoridadeDeTeste(t)
	for nome, c := range map[string]struct {
		depois time.Duration
		mesma  bool
	}{
		"no dia seguinte":   {24 * time.Hour, true},
		"faltando 398 dias": {10*365*24*time.Hour - 398*24*time.Hour, true},
		"faltando 396 dias": {10*365*24*time.Hour - 396*24*time.Hour, false},
		"vencida":           {11 * 365 * 24 * time.Hour, false},
	} {
		t.Run(nome, func(t *testing.T) {
			copia := filepath.Join(t.TempDir(), "chaves")
			copiarPasta(t, pasta, copia)
			b, err := CarregarAutoridade(copia, agoraDosTestes.Add(c.depois))
			if err != nil {
				t.Fatal(err)
			}
			if mesma := string(b.DER()) == string(a.DER()); mesma != c.mesma {
				t.Errorf("mesma autoridade: %v, esperado %v", mesma, c.mesma)
			}
			// A nova fica gravada: a próxima leitura é ela.
			denovo, err := CarregarAutoridade(copia, agoraDosTestes.Add(c.depois))
			if err != nil || string(denovo.DER()) != string(b.DER()) {
				t.Errorf("não gravou a nova: %v", err)
			}
		})
	}
}

func copiarPasta(t *testing.T, de, para string) {
	t.Helper()
	if err := os.MkdirAll(para, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, nome := range []string{ArquivoChaveDaAutoridade, ArquivoDaAutoridade} {
		dados, err := os.ReadFile(filepath.Join(de, nome))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(para, nome), dados, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// Regra (#328): arquivo que existe e não abre é erro, e não motivo para criar outra autoridade em silêncio (o celular
// confiaria na antiga). Falta um dos dois: cria as duas de novo.
func TestCarregarAutoridadeArquivosEstragados(t *testing.T) {
	a, pasta := novaAutoridadeDeTeste(t)
	_, outraPasta := novaAutoridadeDeTeste(t)
	chaveDaOutra, err := os.ReadFile(filepath.Join(outraPasta, ArquivoChaveDaAutoridade))
	if err != nil {
		t.Fatal(err)
	}
	chaveEd := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: chaveEd25519EmPKCS8(t)})
	for nome, c := range map[string]struct {
		arquivo, conteudo, erro string
	}{
		"chave que não é PEM":       {ArquivoChaveDaAutoridade, "lixo", "não estão em PEM"},
		"certificado que não é PEM": {ArquivoDaAutoridade, "lixo", "não estão em PEM"},
		"chave que não abre":        {ArquivoChaveDaAutoridade, "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n", "a chave"},
		"certificado que não abre":  {ArquivoDaAutoridade, "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n", "o certificado"},
		"chave de outra autoridade": {ArquivoChaveDaAutoridade, string(chaveDaOutra), "não é a do certificado"},
		"chave que não é ECDSA":     {ArquivoChaveDaAutoridade, string(chaveEd), "não é ECDSA"},
	} {
		t.Run(nome, func(t *testing.T) {
			copia := filepath.Join(t.TempDir(), "chaves")
			copiarPasta(t, pasta, copia)
			if err := os.WriteFile(filepath.Join(copia, c.arquivo), []byte(c.conteudo), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := CarregarAutoridade(copia, agoraDosTestes)
			if err == nil || !strings.Contains(err.Error(), c.erro) || !strings.Contains(err.Error(), "apague os dois arquivos") {
				t.Errorf("erro %v", err)
			}
		})
	}
	for _, falta := range []string{ArquivoChaveDaAutoridade, ArquivoDaAutoridade} {
		t.Run("sem "+falta, func(t *testing.T) {
			copia := filepath.Join(t.TempDir(), "chaves")
			copiarPasta(t, pasta, copia)
			if err := os.Remove(filepath.Join(copia, falta)); err != nil {
				t.Fatal(err)
			}
			b, err := CarregarAutoridade(copia, agoraDosTestes)
			if err != nil {
				t.Fatal(err)
			}
			if string(b.DER()) == string(a.DER()) {
				t.Error("devia criar outra")
			}
		})
	}
	for _, pasta := range []string{ArquivoChaveDaAutoridade, ArquivoDaAutoridade} {
		t.Run(pasta+" que não dá para ler", func(t *testing.T) {
			copia := filepath.Join(t.TempDir(), "chaves")
			if err := os.MkdirAll(filepath.Join(copia, pasta), 0o700); err != nil { // uma pasta no lugar do arquivo
				t.Fatal(err)
			}
			if _, err := CarregarAutoridade(copia, agoraDosTestes); err == nil || !strings.Contains(err.Error(), "ler a") {
				t.Errorf("erro %v", err)
			}
		})
	}
	t.Run("pasta que não dá para criar", func(t *testing.T) {
		arquivo := filepath.Join(t.TempDir(), "um-arquivo")
		if err := os.WriteFile(arquivo, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := CarregarAutoridade(filepath.Join(arquivo, "chaves"), agoraDosTestes); err == nil {
			t.Error("devia falhar")
		}
	})
}

// chaveEd25519EmPKCS8 é uma chave que abre, mas não é ECDSA (a Ed25519 é instantânea de gerar; a RSA demora).
func chaveEd25519EmPKCS8(t *testing.T) []byte {
	t.Helper()
	_, privada, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privada)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

// Regra (#328): o certificado do site é para o IP da rede, assinado pela autoridade, só para site (ServerAuth), e o
// celular que confia na autoridade confia nele. Vale 397 dias (o limite do iOS e do Chrome é 398).
func TestCertificadoPara(t *testing.T) {
	a, _ := novaAutoridadeDeTeste(t)
	ip := net.ParseIP("192.168.0.10").To4()
	c, err := a.CertificadoPara(ip, agoraDosTestes)
	if err != nil {
		t.Fatal(err)
	}
	folha := c.Leaf
	if len(c.Certificate) != 1 || string(c.Certificate[0]) != string(folha.Raw) {
		t.Error("o certificado vai sozinho, sem a autoridade")
	}
	if len(folha.IPAddresses) != 1 || !folha.IPAddresses[0].Equal(ip) || len(folha.DNSNames) != 0 || folha.IsCA {
		t.Errorf("IPs %v, nomes %v, IsCA %v", folha.IPAddresses, folha.DNSNames, folha.IsCA)
	}
	if !folha.NotBefore.Equal(agoraDosTestes.Add(-time.Hour).Truncate(time.Second)) ||
		!folha.NotAfter.Equal(agoraDosTestes.Add(397*24*time.Hour).Truncate(time.Second)) {
		t.Errorf("validade %v a %v", folha.NotBefore, folha.NotAfter)
	}
	if folha.KeyUsage != x509.KeyUsageDigitalSignature {
		t.Errorf("KeyUsage %v", folha.KeyUsage)
	}
	if _, ok := c.PrivateKey.(*ecdsa.PrivateKey); !ok {
		t.Errorf("chave %T", c.PrivateKey)
	}
	raizes := x509.NewCertPool()
	raizes.AddCert(a.certificado)
	if _, err := folha.Verify(x509.VerifyOptions{Roots: raizes, CurrentTime: agoraDosTestes, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		t.Errorf("o celular não confiaria: %v", err)
	}
	if err := folha.VerifyHostname("192.168.0.10"); err != nil {
		t.Error(err)
	}
	if err := folha.VerifyHostname("192.168.0.11"); err == nil {
		t.Error("vale para outro IP")
	}
	// Duas vezes: chave e número de série novos.
	d, err := a.CertificadoPara(ip, agoraDosTestes)
	if err != nil {
		t.Fatal(err)
	}
	if d.Leaf.SerialNumber.Cmp(folha.SerialNumber) == 0 || d.Leaf.PublicKey.(*ecdsa.PublicKey).Equal(folha.PublicKey) {
		t.Error("repetiu a chave ou o número de série")
	}
	t.Run("não passa da validade da autoridade", func(t *testing.T) {
		quase := agoraDosTestes.Add(10*365*24*time.Hour - 30*24*time.Hour)
		c, err := a.CertificadoPara(ip, quase)
		if err != nil {
			t.Fatal(err)
		}
		if !c.Leaf.NotAfter.Equal(a.certificado.NotAfter) {
			t.Errorf("vence em %v, a autoridade em %v", c.Leaf.NotAfter, a.certificado.NotAfter)
		}
	})
}

// Regra (#328, ADR 0061): quem roubar a chave da autoridade não consegue um certificado em que o celular confie para
// um site da internet: a restrição de nomes recusa um IP público e qualquer nome de DNS.
func TestAutoridadeSoAssinaARedeDeCasa(t *testing.T) {
	a, _ := novaAutoridadeDeTeste(t)
	raizes := x509.NewCertPool()
	raizes.AddCert(a.certificado)
	for nome, c := range map[string]struct {
		ips    []net.IP
		nomes  []string
		aceita bool
	}{
		"IP da rede de casa":  {ips: []net.IP{net.ParseIP("10.1.2.3")}, aceita: true},
		"IP público":          {ips: []net.IP{net.ParseIP("200.1.2.3")}},
		"loopback":            {ips: []net.IP{net.ParseIP("127.0.0.1")}},
		"um site da internet": {nomes: []string{"seubanco.com.br"}},
		"localhost":           {nomes: []string{"localhost"}},
		"rede de casa e site": {ips: []net.IP{net.ParseIP("192.168.0.10")}, nomes: []string{"seubanco.com.br"}},
	} {
		t.Run(nome, func(t *testing.T) {
			folha := assinarComAAutoridade(t, a, c.ips, c.nomes)
			_, err := folha.Verify(x509.VerifyOptions{Roots: raizes, CurrentTime: agoraDosTestes})
			if aceita := err == nil; aceita != c.aceita {
				t.Errorf("aceita %v (%v), esperado %v", aceita, err, c.aceita)
			}
		})
	}
}

// assinarComAAutoridade é o que faria quem roubou a chave: assina o que quiser.
func assinarComAAutoridade(t *testing.T, a *Autoridade, ips []net.IP, nomes []string) *x509.Certificate {
	t.Helper()
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	modelo := &x509.Certificate{
		SerialNumber: numeroDeSerie(), Subject: pkix.Name{CommonName: "teste"}, IPAddresses: ips, DNSNames: nomes,
		NotBefore: agoraDosTestes.Add(-time.Hour), NotAfter: agoraDosTestes.Add(24 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, a.certificado, &chave.PublicKey, a.chave)
	if err != nil {
		t.Fatal(err)
	}
	folha, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return folha
}

// A impressão digital é o SHA-256 do certificado, em pares maiúsculos separados por dois-pontos, como o iOS mostra.
func TestImpressaoDigital(t *testing.T) {
	a, _ := novaAutoridadeDeTeste(t)
	soma := sha256.Sum256(a.DER())
	esperado := strings.ToUpper(hex.EncodeToString(soma[:]))
	obtido := a.ImpressaoDigital()
	if len(obtido) != 32*3-1 || strings.ReplaceAll(obtido, ":", "") != esperado || obtido[2] != ':' {
		t.Errorf("%s\nesperado os pares de %s", obtido, esperado)
	}
}

func TestGravarInteiroFalha(t *testing.T) {
	pasta := t.TempDir()
	// O destino é uma pasta com conteúdo: o rename falha, e o temporário não fica.
	destino := filepath.Join(pasta, "destino")
	if err := os.MkdirAll(filepath.Join(destino, "dentro"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := gravarInteiro(destino, []byte("x"), 0o600); err == nil {
		t.Error("devia falhar no rename")
	}
	if _, err := os.Stat(destino + ".novo"); !os.IsNotExist(err) {
		t.Errorf("sobrou o temporário: %v", err)
	}
	if err := gravarInteiro(filepath.Join(pasta, "nao-existe", "arquivo"), []byte("x"), 0o600); err == nil {
		t.Error("devia falhar ao gravar")
	}
}
