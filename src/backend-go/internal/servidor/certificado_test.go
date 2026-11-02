package servidor

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

var agoraDoCertificado = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

const senhaDoCertificado = "senha-de-teste"

func pfxDoTeste(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "financaslocal.pfx")
}

func garantir(t *testing.T, caminho, senha string, agora time.Time) bool {
	t.Helper()
	gerou, err := GarantirCertificado(caminho, senha, agora)
	if err != nil {
		t.Fatal(err)
	}
	return gerou
}

func ler(t *testing.T, caminho string) []byte {
	t.Helper()
	dados, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return dados
}

// Regra: o certificado novo serve 127.0.0.1 e localhost por 1 ano, como o do dotnet dev-certs, e abre pelo mesmo caminho
// que o servidor usa para subir.
func TestGeraOCertificado(t *testing.T) {
	caminho := pfxDoTeste(t)
	if !garantir(t, caminho, senhaDoCertificado, agoraDoCertificado) {
		t.Fatal("não disse que gerou")
	}
	aberto, err := CarregarPFX(caminho, senhaDoCertificado)
	if err != nil {
		t.Fatal(err)
	}
	folha := aberto.Leaf
	if err := folha.VerifyHostname("localhost"); err != nil {
		t.Error(err)
	}
	if err := folha.VerifyHostname("127.0.0.1"); err != nil {
		t.Error(err)
	}
	if !slices.ContainsFunc(folha.IPAddresses, func(ip net.IP) bool { return ip.Equal(net.IPv6loopback) }) {
		t.Errorf("sem o ::1: %v", folha.IPAddresses)
	}
	if !folha.NotAfter.Equal(agoraDoCertificado.Add(365 * 24 * time.Hour).Truncate(time.Second)) {
		t.Errorf("vence em %v", folha.NotAfter)
	}
	if folha.NotBefore.After(agoraDoCertificado) {
		t.Errorf("começa em %v", folha.NotBefore)
	}
	if !slices.Contains(folha.ExtKeyUsage, x509.ExtKeyUsageServerAuth) {
		t.Errorf("uso = %v", folha.ExtKeyUsage)
	}
}

// Regra: o .pem é só a parte pública: é o que quem instala importa no navegador, e nele não há chave.
func TestPemSoComAParteDublica(t *testing.T) {
	caminho := pfxDoTeste(t)
	garantir(t, caminho, senhaDoCertificado, agoraDoCertificado)
	texto := ler(t, strings.TrimSuffix(caminho, ".pfx")+".pem")
	bloco, resto := pem.Decode(texto)
	if bloco == nil || bloco.Type != "CERTIFICATE" || len(resto) != 0 {
		t.Fatalf("pem = %s", texto)
	}
	aberto, err := CarregarPFX(caminho, senhaDoCertificado)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bloco.Bytes, aberto.Leaf.Raw) {
		t.Error("o .pem não é o certificado do .pfx")
	}
}

func TestPermissaoDosArquivos(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no Windows, quem protege a pasta é a permissão dela")
	}
	caminho := pfxDoTeste(t)
	garantir(t, caminho, senhaDoCertificado, agoraDoCertificado)
	for arquivo, esperada := range map[string]os.FileMode{caminho: 0o600, strings.TrimSuffix(caminho, ".pfx") + ".pem": 0o644} {
		info, err := os.Stat(arquivo)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != esperada {
			t.Errorf("%s = %o", arquivo, info.Mode().Perm())
		}
	}
}

// Regra (a mesma dos scripts): o que abre com a senha e vale por mais de 30 dias fica, para o navegador continuar confiando.
func TestReaproveitaOuTroca(t *testing.T) {
	const dia = 24 * time.Hour
	vencimento := agoraDoCertificado.Add(365 * dia).Truncate(time.Second)
	casos := []struct {
		nome  string
		senha string
		agora time.Time
		gera  bool
	}{
		{"recém-gerado", senhaDoCertificado, agoraDoCertificado, false},
		{"faltando 30 dias e 1 segundo", senhaDoCertificado, vencimento.Add(-30*dia - time.Second), false},
		{"faltando exatamente 30 dias", senhaDoCertificado, vencimento.Add(-30 * dia), true},
		{"vencido", senhaDoCertificado, vencimento.Add(dia), true},
		{"outra senha", "outra", agoraDoCertificado, true},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			caminho := pfxDoTeste(t)
			garantir(t, caminho, senhaDoCertificado, agoraDoCertificado)
			antes := ler(t, caminho)
			if gerou := garantir(t, caminho, caso.senha, caso.agora); gerou != caso.gera {
				t.Fatalf("gerou = %v", gerou)
			}
			if igual := bytes.Equal(antes, ler(t, caminho)); igual == caso.gera {
				t.Errorf("o arquivo ficou igual = %v", igual)
			}
			if _, err := CarregarPFX(caminho, caso.senha); err != nil {
				t.Errorf("não abre com a senha: %v", err)
			}
		})
	}
}

// Regra: arquivo estragado não trava a instalação: gera outro.
func TestArquivoEstragado(t *testing.T) {
	caminho := pfxDoTeste(t)
	if err := os.WriteFile(caminho, []byte("não é um pfx"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !garantir(t, caminho, senhaDoCertificado, agoraDoCertificado) {
		t.Fatal("não gerou")
	}
}

// Regra: reaproveitar também refaz o .pem, que a instalação antiga não tinha ou que alguém apagou.
func TestReaproveitarRefazOPem(t *testing.T) {
	caminho := pfxDoTeste(t)
	garantir(t, caminho, senhaDoCertificado, agoraDoCertificado)
	publica := strings.TrimSuffix(caminho, ".pfx") + ".pem"
	antes := ler(t, publica)
	if err := os.Remove(publica); err != nil {
		t.Fatal(err)
	}
	if garantir(t, caminho, senhaDoCertificado, agoraDoCertificado) {
		t.Fatal("gerou de novo")
	}
	if !bytes.Equal(antes, ler(t, publica)) {
		t.Error("o .pem mudou")
	}
}

// Regra: não sobra arquivo pela metade, e a pasta que não existe é erro: quem a cria são os scripts.
func TestSemArquivoTemporarioEPastaQueNaoExiste(t *testing.T) {
	caminho := pfxDoTeste(t)
	garantir(t, caminho, senhaDoCertificado, agoraDoCertificado)
	sobras, err := filepath.Glob(filepath.Join(filepath.Dir(caminho), "*.novo"))
	if err != nil || len(sobras) != 0 {
		t.Errorf("sobrou %v, %v", sobras, err)
	}
	if _, err := GarantirCertificado(filepath.Join(t.TempDir(), "nao-existe", "a.pfx"), "senha", agoraDoCertificado); err == nil {
		t.Error("pasta que não existe sem erro")
	}
}
