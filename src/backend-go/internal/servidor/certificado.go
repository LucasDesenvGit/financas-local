package servidor

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// CarregarPFX lê o certificado .pfx que os scripts e o .msi já geram (dotnet dev-certs no Linux,
// Export-PfxCertificate no Windows). A biblioteca padrão do Go não lê PKCS#12, e o golang.org/x/crypto/pkcs12 só
// lê o formato antigo (3DES): o .pfx do .NET de hoje vem cifrado com AES, que o go-pkcs12 lê (ADR 0049).
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
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serie, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return tls.Certificate{}, err
	}
	agora := time.Now()
	modelo := &x509.Certificate{
		SerialNumber: serie,
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    agora.Add(-time.Hour),
		NotAfter:     agora.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, modelo, &chave.PublicKey, chave)
	if err != nil {
		return tls.Certificate{}, err
	}
	folha, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: chave, Leaf: folha}, nil
}
