//go:build !windows

package preferencias

import (
	"errors"
	"os"
	"path/filepath"
)

// No Linux, as preferências e o tema entram com o Wails testado lá (#417).

var errLinux = errors.New("as preferências no Linux entram com a #417")

func IniciarComSistema() bool                  { return false }
func DefinirIniciarComSistema(bool) error      { return errLinux }
func ProcurarAoAbrir() bool                    { return true }
func DefinirProcurarAoAbrir(bool) error        { return errLinux }
func ReceberDesenvolvimento(padrao bool) bool  { return padrao }
func DefinirReceberDesenvolvimento(bool) error { return errLinux }
func Escuro() bool                             { return false }

func PastaDocumentos() string {
	casa, _ := os.UserHomeDir()
	return filepath.Join(casa, "Documents")
}
