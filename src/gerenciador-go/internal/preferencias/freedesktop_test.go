package preferencias

import (
	"path/filepath"
	"testing"
)

// Regra: o atalho de início fica no XDG_CONFIG_HOME; sem ele, em ~/.config.
func TestArquivoDoInicio(t *testing.T) {
	t.Parallel()
	casos := map[[2]string]string{
		{"/cfg", "/home/ana"}: filepath.Join("/cfg", "autostart", "financaslocal-gerenciador.desktop"),
		{"", "/home/ana"}:     filepath.Join("/home/ana", ".config", "autostart", "financaslocal-gerenciador.desktop"),
	}
	for entrada, esperado := range casos {
		if obtido := ArquivoDoInicio(entrada[0], entrada[1]); obtido != esperado {
			t.Errorf("%v: %q", entrada, obtido)
		}
	}
}

// Regra: o atalho abre o gerenciador com --bandeja, com o executável entre aspas e \ " ` $ escapados (Desktop Entry),
// e com os textos do C#.
func TestAtalhoDoInicio(t *testing.T) {
	t.Parallel()
	obtido := AtalhoDoInicio("/home/ana/finanças $x/a\"b`c\\d/gerenciador", "/opt/icone.png")
	esperado := "[Desktop Entry]\nType=Application\nName=Gerenciador do Finanças Local\n" +
		"Comment=Avisa se o site do Finanças Local cair\n" +
		"Exec=\"/home/ana/finanças \\$x/a\\\"b\\`c\\\\d/gerenciador\" --bandeja\n" +
		"Icon=/opt/icone.png\nTerminal=false\nX-GNOME-Autostart-enabled=true\n"
	if obtido != esperado {
		t.Errorf("\n%s\nesperado\n%s", obtido, esperado)
	}
}

// Regra: só o "prefere escuro" (1) do portal é escuro; sem preferência (0), claro (2) ou resposta estranha, claro.
func TestEscuroPeloPortal(t *testing.T) {
	t.Parallel()
	casos := map[string]bool{
		"v u 1\n": true, "v v u 1": true,
		"v u 0": false, "v u 2": false, "": false, "1": false, "v s 1": false, "v u 10": false,
	}
	for saida, esperado := range casos {
		if EscuroPeloPortal(saida) != esperado {
			t.Errorf("%q", saida)
		}
	}
}

// Regra: a pasta de documentos é a do xdg-user-dir; vazia ou igual à pasta pessoal (sem configuração), ~/Documents.
func TestPastaDeDocumentos(t *testing.T) {
	t.Parallel()
	casos := map[string]string{
		"/home/ana/Documentos\n": "/home/ana/Documentos",
		"":                       filepath.Join("/home/ana", "Documents"),
		"/home/ana\n":            filepath.Join("/home/ana", "Documents"),
		"/home/ana/":             filepath.Join("/home/ana", "Documents"),
	}
	for xdg, esperada := range casos {
		if obtida := PastaDeDocumentos(xdg, "/home/ana"); obtida != esperada {
			t.Errorf("%q: %q", xdg, obtida)
		}
	}
}
