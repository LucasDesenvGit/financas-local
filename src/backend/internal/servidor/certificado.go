package servidor

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// CarregarPFX lê o certificado .pfx que os scripts e o .msi já geram (dotnet dev-certs no Linux,
// Export-PfxCertificate no Windows). A biblioteca padrão do Go não lê PKCS#12. Os dois geradores de hoje usam o
// formato antigo (3DES), que o golang.org/x/crypto/pkcs12 também lê; mas ele está congelado e não lê o .pfx com
// AES (OpenSSL 3, ou Export-PfxCertificate -CryptoAlgorithmOption AES256_SHA256). O go-pkcs12 lê os dois (ADR 0049).
func CarregarPFX(caminho, senha string) (tls.Certificate, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("ler o certificado: %w", err)
	}
	chave, certificado, cadeia, err := pkcs12.DecodeChain(dados, senha)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("abrir o certificado %s (senha errada ou formato desconhecido): %w", caminho, err)
	}
	resultado := tls.Certificate{Certificate: [][]byte{certificado.Raw}, PrivateKey: chave, Leaf: certificado}
	for _, c := range cadeia {
		resultado.Certificate = append(resultado.Certificate, c.Raw)
	}
	return resultado, nil
}

// CertificadoTemporario cria um certificado só em memória para 127.0.0.1 e localhost, válido por um dia, para
// rodar em desenvolvimento sem o .pfx. O navegador avisa que não confia nele.
func CertificadoTemporario() (tls.Certificate, error) {
	agora := time.Now()
	chave, folha, err := novoCertificado(agora, 24*time.Hour)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{folha.Raw}, PrivateKey: chave, Leaf: folha}, nil
}

// novoCertificado cria a chave e o certificado autoassinado para 127.0.0.1 e localhost. Começa 1 hora antes de agora, para
// aguentar um relógio um pouco atrasado.
func novoCertificado(agora time.Time, validade time.Duration) (*ecdsa.PrivateKey, *x509.Certificate, error) {
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serie, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return nil, nil, err
	}
	modelo := &x509.Certificate{
		SerialNumber: serie,
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    agora.Add(-time.Hour),
		NotAfter:     agora.Add(validade),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, modelo, &chave.PublicKey, chave)
	if err != nil {
		return nil, nil, err
	}
	folha, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return chave, folha, nil
}

// Regra do certificado da instalação, a mesma dos scripts: dura 1 ano, como o do dotnet dev-certs, e é trocado quando
// faltam 30 dias ou menos.
const (
	validadeDoCertificado = 365 * 24 * time.Hour
	folgaDaValidade       = 30 * 24 * time.Hour
)

// GarantirCertificado é o passo do certificado da instalação (#84): reaproveita o .pfx que abre com a senha e ainda vale por
// mais de 30 dias, para não trocar o certificado em que o navegador já confia, e gera outro no caso contrário. Grava ao lado
// a parte pública, sem a chave, no .pem que quem instala importa no navegador. Devolve se gerou.
func GarantirCertificado(caminho, senha string, agora time.Time) (gerou bool, err error) {
	atual, err := CarregarPFX(caminho, senha)
	var folha *x509.Certificate
	if err == nil && atual.Leaf.NotAfter.After(agora.Add(folgaDaValidade)) {
		folha = atual.Leaf
	} else {
		chave, nova, err := novoCertificado(agora, validadeDoCertificado)
		if err != nil {
			return false, err
		}
		// Modern2023: AES e SHA-256. O .NET e o go-pkcs12 leem esse formato, e o Windows também, desde o 10 versão 1809.
		dados, err := pkcs12.Modern2023.Encode(chave, nova, nil, senha)
		if err != nil {
			return false, fmt.Errorf("montar o certificado: %w", err)
		}
		if err := gravarInteiro(caminho, dados, 0o600); err != nil {
			return false, err
		}
		folha, gerou = nova, true
	}
	publica := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: folha.Raw})
	if err := gravarInteiro(strings.TrimSuffix(caminho, ".pfx")+".pem", publica, 0o644); err != nil {
		return false, err
	}
	return gerou, nil
}

// gravarInteiro troca o arquivo de uma vez: uma falha no meio deixa o anterior como estava.
func gravarInteiro(caminho string, dados []byte, permissao os.FileMode) error {
	novo := caminho + ".novo"
	if err := os.WriteFile(novo, dados, permissao); err != nil {
		return fmt.Errorf("gravar o certificado: %w", err)
	}
	if err := os.Rename(novo, caminho); err != nil {
		return fmt.Errorf("gravar o certificado: %w", err)
	}
	return nil
}
