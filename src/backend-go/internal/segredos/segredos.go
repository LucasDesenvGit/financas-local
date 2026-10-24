// Pacote segredos é o esquema novo dos segredos da tabela Configuracao (Client Secret da Pluggy, token da brapi),
// que substitui o ASP.NET Data Protection na inversão (#85, ADR 0049; a 0005 fica substituída).
//
//   - Cifra: AES-256-GCM com nonce aleatório de 96 bits, novo a cada gravação (cipher.NewGCMWithRandomNonce: quem
//     sorteia o nonce é a biblioteca padrão). O valor gravado é "v2:" + base64(nonce|cifra);
//     o prefixo de versão deixa trocar o esquema no futuro sem adivinhar o formato.
//   - A chave da Configuracao (ex.: "Pluggy.ClientSecret") entra como dado autenticado (AAD): o valor cifrado de um
//     segredo não serve se for copiado para a linha de outro.
//   - Chave: 32 bytes em dados/chaves/segredos.chave. No Windows, o arquivo guarda a chave protegida pelo DPAPI
//     (CryptProtectData, da conta que roda o site), como o .NET faz hoje com as chaves do Data Protection; no Linux, a
//     chave crua num arquivo que só o dono lê (0600).
//   - Valor antigo (do Data Protection, sem "v2:"): não se lê, e Decifrar devolve ErrInformarDeNovo. Ninguém apaga
//     nem regrava a linha sozinho: quem usa informa o segredo de novo na tela.
//
// Até a #85 este pacote não está ligado a nenhuma rota: quem lê e grava segredo no site é o .NET.
package segredos

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ArquivoChave é o arquivo da chave, na pasta das chaves (Seguranca:PastaChaves).
const ArquivoChave = "segredos.chave"

const (
	prefixo      = "v2:"
	tamanhoChave = 32 // AES-256
)

var (
	// ErrInformarDeNovo é o valor gravado pelo .NET (Data Protection), que o Go não lê: o segredo precisa ser
	// informado de novo.
	ErrInformarDeNovo = errors.New("o segredo foi gravado pela versão anterior e precisa ser informado de novo")
	// ErrAdulterado é o valor "v2:" que não confere com a chave: mudou no banco, veio de outra instalação ou de
	// outra linha, ou a chave foi trocada.
	ErrAdulterado = errors.New("o segredo gravado não confere com a chave desta instalação: informe-o de novo")
)

// Cofre cifra e decifra os segredos com a chave da instalação.
type Cofre struct {
	aead cipher.AEAD
}

// Abrir lê a chave da pasta, ou a cria se ainda não existe. Arquivo de chave que não se lê (adulterado, de outra conta
// do Windows, com outro tamanho) é erro, nunca uma chave nova em cima: os segredos gravados ficariam perdidos.
func Abrir(pasta string) (*Cofre, error) {
	chave, err := carregarChave(pasta)
	if err != nil {
		return nil, err
	}
	return novoCofre(chave)
}

func novoCofre(chave []byte) (*Cofre, error) {
	bloco, err := aes.NewCipher(chave)
	if err != nil {
		return nil, fmt.Errorf("chave dos segredos: %w", err)
	}
	aead, err := cipher.NewGCMWithRandomNonce(bloco)
	if err != nil {
		return nil, fmt.Errorf("chave dos segredos: %w", err)
	}
	return &Cofre{aead: aead}, nil
}

// Cifrar devolve o valor a gravar na Configuracao para o segredo da chave dada.
func (c *Cofre) Cifrar(chave, segredo string) (string, error) {
	// Com NewGCMWithRandomNonce o nonce não é passado: o Seal sorteia um e o põe na frente (nonce|cifra|etiqueta).
	cifrado := c.aead.Seal(nil, nil, []byte(segredo), []byte(chave))
	return prefixo + base64.StdEncoding.EncodeToString(cifrado), nil
}

// Decifrar devolve o segredo gravado na chave dada. Valor sem "v2:" é ErrInformarDeNovo; "v2:" que não confere é
// ErrAdulterado. Nenhum dos erros traz o valor.
func (c *Cofre) Decifrar(chave, gravado string) (string, error) {
	codificado, ok := strings.CutPrefix(gravado, prefixo)
	if !ok {
		return "", ErrInformarDeNovo
	}
	cifrado, err := base64.StdEncoding.DecodeString(codificado)
	if err != nil {
		return "", ErrAdulterado
	}
	segredo, err := c.aead.Open(nil, nil, cifrado, []byte(chave))
	if err != nil {
		return "", ErrAdulterado
	}
	return string(segredo), nil
}

// carregarChave lê a chave (desprotegendo-a no Windows) ou cria uma. Duas partidas ao mesmo tempo não sobrescrevem
// uma à outra (O_EXCL): quem chega depois lê a que já está lá.
func carregarChave(pasta string) ([]byte, error) {
	caminho := filepath.Join(pasta, ArquivoChave)
	if guardada, err := os.ReadFile(caminho); err == nil {
		chave, err := desproteger(guardada)
		if err != nil {
			return nil, fmt.Errorf("a chave dos segredos %s não se lê (%w): restaure o arquivo da cópia, ou apague-o e informe os segredos de novo", caminho, err)
		}
		if len(chave) != tamanhoChave {
			return nil, fmt.Errorf("a chave dos segredos %s tem %d bytes, e não %d: restaure o arquivo da cópia, ou apague-o e informe os segredos de novo",
				caminho, len(chave), tamanhoChave)
		}
		return chave, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("ler a chave dos segredos %s: %w", caminho, err)
	}

	if err := os.MkdirAll(pasta, 0o700); err != nil {
		return nil, fmt.Errorf("criar a pasta das chaves %s: %w", pasta, err)
	}
	chave := make([]byte, tamanhoChave)
	if _, err := rand.Read(chave); err != nil {
		return nil, err
	}
	guardada, err := proteger(chave)
	if err != nil {
		return nil, fmt.Errorf("proteger a chave dos segredos: %w", err)
	}
	arquivo, err := os.OpenFile(caminho, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return carregarChave(pasta)
	}
	if err != nil {
		return nil, fmt.Errorf("criar a chave dos segredos %s: %w", caminho, err)
	}
	if _, err := arquivo.Write(guardada); err != nil {
		arquivo.Close()
		return nil, fmt.Errorf("gravar a chave dos segredos %s: %w", caminho, err)
	}
	if err := arquivo.Close(); err != nil {
		return nil, fmt.Errorf("gravar a chave dos segredos %s: %w", caminho, err)
	}
	return chave, nil
}
