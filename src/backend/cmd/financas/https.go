package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/servidor"
)

// appsettingsHTTPS é o que a instalação grava no appsettings.Production.json (ou no configuracao.json do .msi): o
// certificado e, se pedidos, o banco, a pasta das chaves e a URL. Os nomes Kestrel:Endpoints:Https são os que o
// internal/plataforma/config lê e que as instalações já feitas trazem.
type appsettingsHTTPS struct {
	Banco *struct {
		Caminho string
	} `json:",omitempty"`
	Seguranca *struct {
		PastaChaves string
	} `json:",omitempty"`
	Kestrel struct {
		Endpoints struct {
			Https struct {
				Url         string `json:",omitempty"`
				Certificate struct {
					Path     string
					Password string
				}
			}
		}
	}
}

// prepararHTTPS é o subcomando que os instaladores e a CI chamam (#775). Lê a senha do .pfx da configuração anterior, ou
// cria uma se faltar; gera o .pfx ou reaproveita o que abre com a senha e ainda vale (a parte pública vai para o .pem
// ao lado); e grava a configuração com o certificado e a senha. A senha fica só nesse arquivo: não passa pela linha de
// comando nem pelo script. Escreve "gerado" ou "reaproveitado". Pôr o certificado na loja de confiança do sistema continua
// sendo do instalador de cada sistema.
func prepararHTTPS(argumentos []string, saida io.Writer, r relogio.Relogio) error {
	opcoes := flag.NewFlagSet("preparar-https", flag.ContinueOnError)
	var (
		pfx         = opcoes.String("pfx", "", "caminho do .pfx; a parte pública vai para o .pem ao lado")
		config      = opcoes.String("config", "", "arquivo de configuração a gravar (appsettings.Production.json ou configuracao.json)")
		configAtual = opcoes.String("config-atual", "", "de onde ler a senha que já valia (padrão: o próprio -config)")
		url         = opcoes.String("url", "", "Kestrel:Endpoints:Https:Url, quando o appsettings.json não basta (ex.: https://127.0.0.1:5443)")
		banco       = opcoes.String("banco", "", "caminho do banco, para o Banco:Caminho (vazio: não grava)")
		chaves      = opcoes.String("chaves", "", "pasta das chaves, para o Seguranca:PastaChaves (vazio: não grava)")
	)
	if err := opcoes.Parse(argumentos); err != nil {
		return err
	}
	if *pfx == "" || *config == "" {
		return errors.New("preparar-https: informe -pfx e -config")
	}
	if *configAtual == "" {
		*configAtual = *config
	}

	senha := senhaDe(*configAtual)
	if senha == "" {
		aleatorio := make([]byte, 24)
		if _, err := rand.Read(aleatorio); err != nil {
			return fmt.Errorf("preparar-https: criar a senha: %w", err)
		}
		senha = base64.StdEncoding.EncodeToString(aleatorio)
	}
	gerou, err := servidor.GarantirCertificado(*pfx, senha, r.Agora())
	if err != nil {
		return err
	}

	var c appsettingsHTTPS
	c.Kestrel.Endpoints.Https.Url = *url
	c.Kestrel.Endpoints.Https.Certificate.Password = senha
	if c.Kestrel.Endpoints.Https.Certificate.Path, err = caminhoComBarras(*pfx); err != nil {
		return err
	}
	if *banco != "" {
		caminho, err := caminhoComBarras(*banco)
		if err != nil {
			return err
		}
		c.Banco = &struct{ Caminho string }{caminho}
	}
	if *chaves != "" {
		pasta, err := caminhoComBarras(*chaves)
		if err != nil {
			return err
		}
		c.Seguranca = &struct{ PastaChaves string }{pasta}
	}
	dados, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*config, append(dados, '\n'), 0o600); err != nil {
		return fmt.Errorf("preparar-https: gravar a configuração: %w", err)
	}

	resultado := "reaproveitado"
	if gerou {
		resultado = "gerado"
	}
	_, err = fmt.Fprintln(saida, resultado)
	return err
}

// caminhoComBarras é o caminho absoluto com "/" até no Windows, como o appsettings sempre o trouxe (sem "\\" para escapar).
func caminhoComBarras(caminho string) (string, error) {
	absoluto, err := filepath.Abs(caminho)
	if err != nil {
		return "", fmt.Errorf("preparar-https: %w", err)
	}
	return filepath.ToSlash(absoluto), nil
}

// senhaDe lê a senha do .pfx da configuração que já valia. Arquivo que falta, JSON ruim ou sem a senha dão vazio, e
// quem chama cria outra.
func senhaDe(arquivo string) string {
	dados, err := os.ReadFile(arquivo)
	if err != nil {
		return ""
	}
	var c appsettingsHTTPS
	// O Windows PowerShell 5.1 pode gravar a marca do UTF-8 no começo do arquivo.
	if json.Unmarshal(bytes.TrimPrefix(dados, []byte{0xEF, 0xBB, 0xBF}), &c) != nil {
		return ""
	}
	return c.Kestrel.Endpoints.Https.Certificate.Password
}
