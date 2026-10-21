// Pacote config lê a mesma configuração que a API .NET lê: appsettings.json, appsettings.<Ambiente>.json (os
// scripts e o .msi geram o appsettings.Production.json) e, quando há "Dados:Pasta" (instalação pelo .msi), o
// configuracao.json dessa pasta. Assim os scripts não precisam mudar para o Go (#73).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Config é o que o servidor usa da configuração.
type Config struct {
	PastaBase          string   // onde ficam os appsettings e o wwwroot (a pasta do executável)
	BancoCaminho       string   // Banco:Caminho
	PastaChaves        string   // Seguranca:PastaChaves
	MinutosSessao      int      // Seguranca:MinutosSessao
	PluggyURLBase      string   // Pluggy:UrlBase
	Endereco           string   // host:porta, de Kestrel:Endpoints:Https:Url
	CertificadoCaminho string   // Kestrel:Endpoints:Https:Certificate:Path
	CertificadoSenha   string   // Kestrel:Endpoints:Https:Certificate:Password (nunca vai para o log)
	HostsPermitidos    []string // AllowedHosts, separados por ";"
}

// Carregar lê a configuração da pasta base, no ambiente dado ("Production" se vazio).
func Carregar(pastaBase, ambiente string) (Config, error) {
	if ambiente == "" {
		ambiente = "Production"
	}
	arvore := map[string]any{}
	for _, nome := range []string{"appsettings.json", "appsettings." + ambiente + ".json"} {
		if err := mesclarArquivo(arvore, filepath.Join(pastaBase, nome)); err != nil {
			return Config{}, err
		}
	}

	// Instalação pelo .msi (ADR 0029): a pasta de dados define o banco e as chaves e traz o configuracao.json,
	// com o certificado e a senha dele. Igual ao Program.cs.
	if pastaDados := texto(arvore, "Dados:Pasta"); pastaDados != "" {
		pastaDados, err := filepath.Abs(expandirVariaveis(pastaDados))
		if err != nil {
			return Config{}, err
		}
		definir(arvore, "Banco:Caminho", filepath.Join(pastaDados, "sql", "FinancasLocal.db"))
		definir(arvore, "Seguranca:PastaChaves", filepath.Join(pastaDados, "chaves"))
		if err := mesclarArquivo(arvore, filepath.Join(pastaDados, "configuracao.json")); err != nil {
			return Config{}, err
		}
	}

	c := Config{
		PastaBase:          pastaBase,
		BancoCaminho:       caminho(pastaBase, texto(arvore, "Banco:Caminho"), filepath.Join("dados", "sql", "FinancasLocal.db")),
		PastaChaves:        caminho(pastaBase, texto(arvore, "Seguranca:PastaChaves"), "chaves"),
		MinutosSessao:      120,
		PluggyURLBase:      texto(arvore, "Pluggy:UrlBase"),
		Endereco:           "127.0.0.1:5443",
		CertificadoCaminho: texto(arvore, "Kestrel:Endpoints:Https:Certificate:Path"),
		CertificadoSenha:   texto(arvore, "Kestrel:Endpoints:Https:Certificate:Password"),
	}
	if c.CertificadoCaminho != "" {
		c.CertificadoCaminho = caminho(pastaBase, c.CertificadoCaminho, "")
	}
	if minutos := texto(arvore, "Seguranca:MinutosSessao"); minutos != "" {
		n, err := strconv.Atoi(minutos)
		if err != nil {
			return Config{}, fmt.Errorf("Seguranca:MinutosSessao inválido %q: %w", minutos, err)
		}
		c.MinutosSessao = n
	}
	if endereco := texto(arvore, "Kestrel:Endpoints:Https:Url"); endereco != "" {
		u, err := url.Parse(endereco)
		if err != nil || u.Host == "" {
			return Config{}, fmt.Errorf("Kestrel:Endpoints:Https:Url inválido %q", endereco)
		}
		c.Endereco = u.Host
	}
	for _, host := range strings.Split(texto(arvore, "AllowedHosts"), ";") {
		if host = strings.TrimSpace(host); host != "" {
			c.HostsPermitidos = append(c.HostsPermitidos, host)
		}
	}
	return c, nil
}

// ConferirSoLocal recusa endereço fora do computador: o site só escuta em 127.0.0.1 (ou ::1, ou localhost).
func ConferirSoLocal(endereco string) error {
	host, _, err := net.SplitHostPort(endereco)
	if err != nil {
		return fmt.Errorf("endereço inválido %q: %w", endereco, err)
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("o endereço %q não é local: o site só escuta em 127.0.0.1", endereco)
}

func mesclarArquivo(arvore map[string]any, caminho string) error {
	dados, err := os.ReadFile(caminho)
	if errors.Is(err, os.ErrNotExist) {
		return nil // opcional, como no .NET
	}
	if err != nil {
		return fmt.Errorf("ler %s: %w", caminho, err)
	}
	var novo map[string]any
	if err := json.Unmarshal(dados, &novo); err != nil {
		return fmt.Errorf("ler %s: %w", caminho, err)
	}
	mesclar(arvore, novo)
	return nil
}

// mesclar põe os valores de novo por cima dos de destino. As chaves não diferenciam maiúsculas, como no .NET.
func mesclar(destino, novo map[string]any) {
	for chave, valor := range novo {
		existente := chaveIgual(destino, chave)
		if filho, ok := valor.(map[string]any); ok {
			if atual, ok := destino[existente].(map[string]any); ok {
				mesclar(atual, filho)
				continue
			}
		}
		delete(destino, existente)
		destino[chave] = valor
	}
}

func chaveIgual(m map[string]any, chave string) string {
	for k := range m {
		if strings.EqualFold(k, chave) {
			return k
		}
	}
	return chave
}

// texto lê "Secao:Chave" da árvore, como cfg["Secao:Chave"] no .NET. Número e booleano viram texto.
func texto(arvore map[string]any, caminhoChave string) string {
	var atual any = arvore
	for _, parte := range strings.Split(caminhoChave, ":") {
		m, ok := atual.(map[string]any)
		if !ok {
			return ""
		}
		atual = m[chaveIgual(m, parte)]
	}
	switch v := atual.(type) {
	case string:
		return v
	case nil, map[string]any, []any:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func definir(arvore map[string]any, caminhoChave string, valor string) {
	partes := strings.Split(caminhoChave, ":")
	atual := arvore
	for _, parte := range partes[:len(partes)-1] {
		chave := chaveIgual(atual, parte)
		filho, ok := atual[chave].(map[string]any)
		if !ok {
			filho = map[string]any{}
			atual[chave] = filho
		}
		atual = filho
	}
	ultima := partes[len(partes)-1]
	delete(atual, chaveIgual(atual, ultima))
	atual[ultima] = valor
}

// caminho resolve um caminho relativo a partir da pasta base. (O .NET resolve a partir da pasta atual do processo;
// os scripts e o .msi sempre gravam caminhos absolutos, então só muda em desenvolvimento.)
func caminho(pastaBase, valor, padrao string) string {
	if valor == "" {
		valor = padrao
	}
	if filepath.IsAbs(valor) {
		return filepath.Clean(valor)
	}
	return filepath.Join(pastaBase, valor)
}

var variavelWindows = regexp.MustCompile(`%([^%]+)%`)

// expandirVariaveis troca %VARIAVEL% pelo valor, como Environment.ExpandEnvironmentVariables (o .msi grava
// %ProgramData%\FinancasLocal\dados). Variável que não existe fica como está, também como no .NET.
func expandirVariaveis(valor string) string {
	return variavelWindows.ReplaceAllStringFunc(valor, func(trecho string) string {
		if v, ok := os.LookupEnv(trecho[1 : len(trecho)-1]); ok {
			return v
		}
		return trecho
	})
}
