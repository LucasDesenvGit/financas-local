package celular

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Os arquivos da autoridade, na pasta das chaves (dados/chaves), ao lado da chave da sessão e com a mesma proteção: a
// pasta só abre para a conta do serviço (e os administradores, no Windows).
const (
	ArquivoChaveDaAutoridade = "celular-autoridade.chave" // a chave privada (PKCS #8 em PEM), só o dono lê
	ArquivoDaAutoridade      = "celular-autoridade.crt"   // o certificado público (PEM), o que vai para o celular
)

const (
	// A autoridade vale 10 anos: o celular a instala uma vez. Ela não tem o limite de 398 dias dos certificados de site.
	validadeDaAutoridade = 10 * 365 * 24 * time.Hour
	// O certificado do site vale 397 dias, dentro do limite do iOS e do Chrome (398), e é refeito a cada vez que o site
	// liga o acesso (ao subir também), então não vence com o site rodando.
	validadeDoCertificado = 397 * 24 * time.Hour
	// Começa uma hora antes, para aguentar um relógio do celular um pouco atrasado (como o certificado do computador).
	folgaDoRelogio = time.Hour
)

// redesDeCasa são as faixas em que a autoridade pode assinar (name constraints, RFC 5280 4.2.1.10): só os IPs da rede de
// casa. Quem roubar a chave dela não consegue assinar um certificado para um site da internet em que o celular confie.
var redesDeCasa = []*net.IPNet{
	{IP: net.IPv4(10, 0, 0, 0).To4(), Mask: net.CIDRMask(8, 32)},
	{IP: net.IPv4(172, 16, 0, 0).To4(), Mask: net.CIDRMask(12, 32)},
	{IP: net.IPv4(192, 168, 0, 0).To4(), Mask: net.CIDRMask(16, 32)},
}

// nenhumNome é o único nome de DNS que a autoridade aceita: "invalid" é reservado (RFC 2606) e nunca existe. Sem ele,
// a restrição valeria só para IPs, e um nome qualquer ("seubanco.com.br") passaria.
var nenhumNome = []string{"invalid"}

// Autoridade é a autoridade certificadora local do acesso pelo celular (ADR 0061): assina o certificado do site para o
// IP da rede de casa. Só o certificado público sai do computador.
type Autoridade struct {
	certificado *x509.Certificate
	chave       *ecdsa.PrivateKey
}

// CarregarAutoridade lê a autoridade da pasta das chaves ou, se ainda não existe (ou venceu), cria outra. Arquivo que
// existe e não abre é erro, e não motivo para criar outra em silêncio: o celular confiaria na antiga.
func CarregarAutoridade(pasta string, agora time.Time) (*Autoridade, error) {
	caminhoChave, caminhoCertificado := filepath.Join(pasta, ArquivoChaveDaAutoridade), filepath.Join(pasta, ArquivoDaAutoridade)
	pemChave, errChave := os.ReadFile(caminhoChave)
	pemCertificado, errCertificado := os.ReadFile(caminhoCertificado)
	switch {
	case errChave == nil && errCertificado == nil:
		a, err := lerAutoridade(pemChave, pemCertificado)
		if err != nil {
			return nil, fmt.Errorf("abrir a autoridade do acesso pelo celular em %s (apague os dois arquivos celular-autoridade para criar outra; o celular precisa instalar a nova): %w", pasta, err)
		}
		if agora.Before(a.certificado.NotAfter.Add(-validadeDoCertificado)) {
			return a, nil
		}
		// Faltam menos de 397 dias: o certificado do site passaria da validade dela. Cria outra (o celular instala de novo).
	case errChave != nil && !errors.Is(errChave, fs.ErrNotExist):
		return nil, fmt.Errorf("ler a chave da autoridade %s: %w", caminhoChave, errChave)
	case errCertificado != nil && !errors.Is(errCertificado, fs.ErrNotExist):
		return nil, fmt.Errorf("ler a autoridade %s: %w", caminhoCertificado, errCertificado)
	}
	// Falta um dos dois (ou os dois, ou venceu): uma nova, gravando os dois arquivos.
	a, err := novaAutoridade(agora)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(pasta, 0o700); err != nil {
		return nil, fmt.Errorf("criar a pasta das chaves %s: %w", pasta, err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(a.chave)
	if err != nil {
		return nil, err
	}
	if err := gravarInteiro(caminhoChave, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, err
	}
	if err := gravarInteiro(caminhoCertificado, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: a.certificado.Raw}), 0o644); err != nil {
		return nil, err
	}
	return a, nil
}

func lerAutoridade(pemChave, pemCertificado []byte) (*Autoridade, error) {
	blocoChave, _ := pem.Decode(pemChave)
	blocoCertificado, _ := pem.Decode(pemCertificado)
	if blocoChave == nil || blocoCertificado == nil {
		return nil, errors.New("os arquivos não estão em PEM")
	}
	chave, err := x509.ParsePKCS8PrivateKey(blocoChave.Bytes)
	if err != nil {
		return nil, fmt.Errorf("a chave: %w", err)
	}
	ec, ok := chave.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("a chave não é ECDSA")
	}
	certificado, err := x509.ParseCertificate(blocoCertificado.Bytes)
	if err != nil {
		return nil, fmt.Errorf("o certificado: %w", err)
	}
	if !ec.PublicKey.Equal(certificado.PublicKey) {
		return nil, errors.New("a chave não é a do certificado")
	}
	return &Autoridade{certificado: certificado, chave: ec}, nil
}

func novaAutoridade(agora time.Time) (*Autoridade, error) {
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	modelo := &x509.Certificate{
		SerialNumber:          numeroDeSerie(),
		Subject:               pkix.Name{CommonName: "Finanças Local (celular)", Organization: []string{"Finanças Local"}},
		NotBefore:             agora.Add(-folgaDoRelogio),
		NotAfter:              agora.Add(validadeDaAutoridade),
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLenZero:        true, // assina só o certificado do site, nunca outra autoridade
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		PermittedIPRanges:     redesDeCasa,
		PermittedDNSDomains:   nenhumNome,
		// Não crítica, de propósito (*julgamento*): quem entende a restrição (Chrome, Android, o Go) a cumpre, e quem não
		// entende continua aceitando a autoridade. Crítica, um celular que não a entendesse recusaria a autoridade inteira.
		PermittedDNSDomainsCritical: false,
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, modelo, &chave.PublicKey, chave)
	if err != nil {
		return nil, fmt.Errorf("criar a autoridade: %w", err)
	}
	certificado, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &Autoridade{certificado: certificado, chave: chave}, nil
}

// DER é o certificado público da autoridade, como o celular instala (o .crt baixado).
func (a *Autoridade) DER() []byte { return a.certificado.Raw }

// ImpressaoDigital é o SHA-256 do certificado da autoridade, em pares hexadecimais ("AB:CD:..."), como o iOS mostra nos
// detalhes do perfil: dá para conferir que o arquivo que chegou ao celular é o deste computador.
func (a *Autoridade) ImpressaoDigital() string {
	soma := sha256.Sum256(a.certificado.Raw)
	pares := make([]string, len(soma))
	for i, b := range soma {
		pares[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(pares, ":")
}

// CertificadoPara cria o certificado do site para o IP da rede, assinado pela autoridade, com uma chave nova que só vive
// na memória do processo.
func (a *Autoridade) CertificadoPara(ip net.IP, agora time.Time) (tls.Certificate, error) {
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	fim := agora.Add(validadeDoCertificado)
	if fim.After(a.certificado.NotAfter) {
		fim = a.certificado.NotAfter
	}
	modelo := &x509.Certificate{
		SerialNumber: numeroDeSerie(),
		Subject:      pkix.Name{CommonName: ip.String()},
		IPAddresses:  []net.IP{ip},
		NotBefore:    agora.Add(-folgaDoRelogio),
		NotAfter:     fim,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, a.certificado, &chave.PublicKey, a.chave)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("assinar o certificado do site para %s: %w", ip, err)
	}
	folha, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: chave, Leaf: folha}, nil
}

// numeroDeSerie é aleatório, de 127 bits (positivo e com até 20 bytes, como pede a RFC 5280).
func numeroDeSerie() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		panic(err) // o crypto/rand do Go não falha (Go 1.24+)
	}
	return n
}

// gravarInteiro troca o arquivo de uma vez: uma falha no meio deixa o anterior como estava.
func gravarInteiro(caminho string, dados []byte, permissao os.FileMode) error {
	novo := caminho + ".novo"
	if err := os.WriteFile(novo, dados, permissao); err != nil {
		return fmt.Errorf("gravar %s: %w", caminho, err)
	}
	if err := os.Rename(novo, caminho); err != nil {
		_ = os.Remove(novo)
		return fmt.Errorf("gravar %s: %w", caminho, err)
	}
	return nil
}
