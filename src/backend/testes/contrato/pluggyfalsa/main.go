// A Pluggy falsa do contrato (#83): responde com os arquivos de dados/pluggy, em HTTPS, para o .NET e o Go sincronizarem
// o mesmo cenário sem internet e sem conta de verdade. Os dados são fictícios.
//
// O arquivo vem do caminho do pedido (a tabela em arquivoDoPedido). Na segunda vez em que a mesma resposta é pedida, vale
// "<nome>.2.json", se existir: é a transação que mudou entre uma sincronização e a outra.
//
// O certificado é gerado aqui, com validade larga, porque o .NET roda com o relógio do contrato; a raiz vai para o arquivo
// de -certificado, que o rodar.sh passa em SSL_CERT_FILE.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var componentePermitido = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func componenteSeguro(s string) (string, bool) {
	if s == "" || strings.Contains(s, "/") || strings.Contains(s, "\\") || strings.Contains(s, "..") {
		return "", false
	}
	if !componentePermitido.MatchString(s) {
		return "", false
	}
	return s, true
}

// arquivoDoPedido devolve o nome do arquivo (sem a extensão) de um pedido.
func arquivoDoPedido(r *http.Request) string {
	caminho := strings.Trim(r.URL.Path, "/")
	partes := strings.Split(caminho, "/")
	q := r.URL.Query()
	switch {
	case caminho == "auth":
		return "auth"
	case partes[0] == "items" && len(partes) == 2:
		id, ok := componenteSeguro(partes[1])
		if !ok {
			return ""
		}
		return "item-" + id
	case caminho == "accounts":
		itemID, ok := componenteSeguro(q.Get("itemId"))
		if !ok {
			return ""
		}
		return "contas-" + itemID
	case caminho == "categories":
		return "categorias"
	case caminho == "bills":
		accountID, ok := componenteSeguro(q.Get("accountId"))
		if !ok {
			return ""
		}
		return "faturas-" + accountID
	case caminho == "v2/transactions":
		accountID, ok := componenteSeguro(q.Get("accountId"))
		if !ok {
			return ""
		}
		if depois := q.Get("after"); depois != "" {
			after, ok := componenteSeguro(depois)
			if !ok {
				return ""
			}
			return "transacoes-" + accountID + "-" + after
		}
		return "transacoes-" + accountID
	case caminho == "investments":
		itemID, ok := componenteSeguro(q.Get("itemId"))
		if !ok {
			return ""
		}
		return "investimentos-" + itemID
	case partes[0] == "investments" && len(partes) == 3 && partes[2] == "transactions":
		id, ok := componenteSeguro(partes[1])
		if !ok {
			return ""
		}
		return "movimentos-" + id
	case caminho == "loans":
		itemID, ok := componenteSeguro(q.Get("itemId"))
		if !ok {
			return ""
		}
		return "emprestimos-" + itemID
	}
	return ""
}

type pluggyFalsa struct {
	dados  string
	mu     sync.Mutex
	vistos map[string]int
}

func (p *pluggyFalsa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nome := arquivoDoPedido(r)
	if r.Header.Get("X-API-KEY") == "" && nome != "auth" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	p.mu.Lock()
	p.vistos[nome]++
	vez := p.vistos[nome]
	p.mu.Unlock()

	candidatos := []string{nome + ".json"}
	if vez > 1 {
		candidatos = []string{fmt.Sprintf("%s.%d.json", nome, vez), nome + ".json"}
	}
	for _, c := range candidatos {
		corpo, err := os.ReadFile(filepath.Join(p.dados, c))
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(corpo)
			return
		}
	}
	log.Printf("sem resposta para %s %s (%q)", r.Method, r.URL, nome)
	w.WriteHeader(http.StatusNotFound)
}

// certificado gera o certificado do servidor e grava a raiz em caminho.
func certificado(caminho string) (tls.Certificate, error) {
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	modelo := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "pluggy-falsa"},
		NotBefore:             time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Date(2050, 1, 1, 0, 0, 0, 0, time.UTC),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, modelo, &chave.PublicKey, chave)
	if err != nil {
		return tls.Certificate{}, err
	}
	bloco := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(caminho, bloco, 0o600); err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: chave}, nil
}

func main() {
	endereco := flag.String("endereco", "127.0.0.1:5452", "onde escutar")
	dados := flag.String("dados", "", "pasta das respostas")
	raiz := flag.String("certificado", "", "onde gravar a raiz, para SSL_CERT_FILE")
	flag.Parse()

	cert, err := certificado(*raiz)
	if err != nil {
		log.Fatal(err)
	}
	servidor := &http.Server{
		Addr:              *endereco,
		Handler:           &pluggyFalsa{dados: *dados, vistos: map[string]int{}},
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}},
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(servidor.ListenAndServeTLS("", ""))
}
