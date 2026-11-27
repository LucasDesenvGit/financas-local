// Gera, a partir do banco/ e do scripts/geral/2-criar-banco.sh, as cópias idênticas em
// internal/plataforma/banco/esquema/embutido/, que vão dentro do executável (o //go:embed não sai do módulo).
//
// Roda pelo go generate ./... (a partir de qualquer pasta do módulo).
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/banco/esquema"
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
}

// irParaRaizDoModulo sobe até a pasta do go.mod: o go generate roda na pasta do pacote, e os caminhos do esquema
// são a partir de src/backend.
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
