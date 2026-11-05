package preferencias

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/comando"
)

// No Linux, o gerenciador é instalado pelos scripts, e não pelo .msi: não procura versão nova no GitHub, então só
// "Iniciar com o sistema" fica gravado. As outras preferências respondem o padrão.

func arquivoDoInicio() string {
	casa, _ := os.UserHomeDir()
	return ArquivoDoInicio(os.Getenv("XDG_CONFIG_HOME"), casa)
}

// IniciarComSistema: abre na bandeja quando a sessão começa (padrão: não).
func IniciarComSistema() bool {
	_, err := os.Stat(arquivoDoInicio())
	return err == nil
}

// DefinirIniciarComSistema grava (ou apaga) o atalho em ~/.config/autostart, com o ícone ao lado do executável, como
// o do C#.
func DefinirIniciarComSistema(ligado bool) error {
	arquivo := arquivoDoInicio()
	if !ligado {
		if err := os.Remove(arquivo); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(arquivo), 0o755); err != nil {
		return err
	}
	return os.WriteFile(arquivo, []byte(AtalhoDoInicio(exe, filepath.Join(filepath.Dir(exe), "icone.png"))), 0o644)
}

func ProcurarAoAbrir() bool                    { return true }
func DefinirProcurarAoAbrir(bool) error        { return errors.ErrUnsupported }
func ReceberDesenvolvimento(padrao bool) bool  { return padrao }
func DefinirReceberDesenvolvimento(bool) error { return errors.ErrUnsupported }

// Escuro: a preferência de cor do sistema pelo portal do freedesktop (a mesma que o KDE e o GNOME mudam nas
// configurações). Sem portal, claro.
func Escuro() bool {
	saida, err := comando.Rodar(context.Background(), 3*time.Second, "busctl", "--user", "call",
		"org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop", "org.freedesktop.portal.Settings",
		"ReadOne", "ss", "org.freedesktop.appearance", "color-scheme")
	return err == nil && EscuroPeloPortal(saida)
}

// PastaDocumentos é onde o "Salvar uma cópia do banco" começa.
func PastaDocumentos() string {
	casa, _ := os.UserHomeDir()
	saida, _ := comando.Rodar(context.Background(), 3*time.Second, "xdg-user-dir", "DOCUMENTS")
	return PastaDeDocumentos(saida, casa)
}
