package preferencias

import (
	"path/filepath"
	"strings"
)

// O que vale no Linux pelos padrões do freedesktop (KDE, GNOME, XFCE…), separado da leitura e da gravação para dar
// para testar em qualquer sistema.

// NomeDoInicio é o atalho em ~/.config/autostart: o mesmo nome do gerenciador em C#, como no Windows (ligar no em Go
// troca o que abre com o sistema).
const NomeDoInicio = "financaslocal-gerenciador.desktop"

// ArquivoDoInicio é onde fica o atalho de iniciar com o sistema: $XDG_CONFIG_HOME/autostart, ou ~/.config/autostart.
func ArquivoDoInicio(configHome, casa string) string {
	if configHome == "" {
		configHome = filepath.Join(casa, ".config")
	}
	return filepath.Join(configHome, "autostart", NomeDoInicio)
}

// AtalhoDoInicio é o .desktop que abre o gerenciador com --bandeja, com os textos do Preferencias.cs do Linux.
func AtalhoDoInicio(executavel, icone string) string {
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Gerenciador do Finanças Local\n" +
		"Comment=Avisa se o site do Finanças Local cair\n" +
		"Exec=" + aspasDoExec(executavel) + " --bandeja\n" +
		"Icon=" + icone + "\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"
}

// aspasDoExec põe o argumento do Exec= entre aspas, escapando \ " ` $ (especificação Desktop Entry).
func aspasDoExec(valor string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`).Replace(valor) + `"`
}

// EscuroPeloPortal lê a resposta do busctl ao portal (org.freedesktop.appearance color-scheme): "v u 1" é "prefere
// escuro"; 0 (sem preferência) e 2 (prefere claro) ficam no claro, como o padrão do Windows. A leitura antiga (Read,
// e não ReadOne) responde com uma variante dentro da outra: "v v u 1".
func EscuroPeloPortal(saida string) bool {
	campos := strings.Fields(saida)
	return len(campos) >= 3 && campos[len(campos)-2] == "u" && campos[len(campos)-1] == "1"
}

// PastaDeDocumentos é a do xdg-user-dir; sem ela (ou com ela na própria pasta pessoal, que é o que ele responde sem a
// configuração), ~/Documents.
func PastaDeDocumentos(xdg, casa string) string {
	xdg = strings.TrimSpace(xdg)
	if xdg == "" || filepath.Clean(xdg) == filepath.Clean(casa) {
		return filepath.Join(casa, "Documents")
	}
	return xdg
}
