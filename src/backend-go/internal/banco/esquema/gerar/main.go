// Gera, a partir do banco/ e do scripts/geral/2-criar-banco.sh:
//   - internal/banco/esquema/embutido/: cópias idênticas, que vão dentro do executável (o //go:embed não sai do módulo);
//   - internal/banco/esquema/esquema-sqlc.sql: o esquema para o sqlc.
//
// Roda pelo go generate ./... (a partir de qualquer pasta do módulo).
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/esquema"
)

func main() {
	if err := irParaRaizDoModulo(); err != nil {
		log.Fatal(err)
	}
	for copia, origem := range esquema.Copias {
		conteudo, err := os.ReadFile(origem)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(esquema.PastaEmbutido, copia), conteudo, 0o644); err != nil {
			log.Fatal(err)
		}
	}
	estrutura, err := os.ReadFile(esquema.ArquivoEstrutura)
	if err != nil {
		log.Fatal(err)
	}
	script, err := os.ReadFile(esquema.ArquivoColunas)
	if err != nil {
		log.Fatal(err)
	}
	texto, err := esquema.GerarEsquemaSqlc(context.Background(), string(estrutura), string(script))
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(esquema.ArquivoGerado, []byte(texto), 0o644); err != nil {
		log.Fatal(err)
	}
}

// irParaRaizDoModulo sobe até a pasta do go.mod: o go generate roda na pasta do pacote, e os caminhos do esquema
// são a partir de src/backend-go.
func irParaRaizDoModulo() error {
	pasta, err := os.Getwd()
	if err != nil {
		return err
	}
	for {
		if _, err := os.Stat(filepath.Join(pasta, "go.mod")); err == nil {
			return os.Chdir(pasta)
		}
		acima := filepath.Dir(pasta)
		if acima == pasta {
			return os.ErrNotExist
		}
		pasta = acima
	}
}
