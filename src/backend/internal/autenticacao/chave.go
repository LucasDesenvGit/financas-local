package autenticacao

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ArquivoChave é o arquivo, na pasta das chaves (Seguranca:PastaChaves, a dados/chaves), com a chave que assina o
// cookie da sessão e o token CSRF do Go. É do Go: o .NET continua com as chaves do Data Protection na mesma pasta, e
// o cookie de um não vale no outro (todos saem da conta na troca, #85).
const ArquivoChave = "sessao-go.chave"

const tamanhoChaveAssinatura = 32

// CarregarChave lê a chave de assinatura da pasta, e a cria (32 bytes aleatórios, só o dono lê e grava) se ainda
// não existe. Duas partidas ao mesmo tempo não sobrescrevem uma à outra: quem chega depois lê a que já está lá.
func CarregarChave(pasta string) ([]byte, error) {
	caminho := filepath.Join(pasta, ArquivoChave)
	if chave, err := os.ReadFile(caminho); err == nil {
		if len(chave) != tamanhoChaveAssinatura {
			return nil, fmt.Errorf("a chave de sessão %s tem %d bytes, e não %d: apague o arquivo para criar outra (todos saem da conta)",
				caminho, len(chave), tamanhoChaveAssinatura)
		}
		return chave, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("ler a chave de sessão %s: %w", caminho, err)
	}

	if err := os.MkdirAll(pasta, 0o700); err != nil {
		return nil, fmt.Errorf("criar a pasta das chaves %s: %w", pasta, err)
	}
	chave := make([]byte, tamanhoChaveAssinatura)
	if _, err := rand.Read(chave); err != nil {
		return nil, err
	}
	// O_EXCL: se outro processo criou o arquivo agora, fica a dele.
	arquivo, err := os.OpenFile(caminho, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return CarregarChave(pasta)
	}
	if err != nil {
		return nil, fmt.Errorf("criar a chave de sessão %s: %w", caminho, err)
	}
	if _, err := arquivo.Write(chave); err != nil {
		arquivo.Close()
		return nil, fmt.Errorf("gravar a chave de sessão %s: %w", caminho, err)
	}
	if err := arquivo.Close(); err != nil {
		return nil, fmt.Errorf("gravar a chave de sessão %s: %w", caminho, err)
	}
	return chave, nil
}
