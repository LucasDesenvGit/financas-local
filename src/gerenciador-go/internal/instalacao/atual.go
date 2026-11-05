package instalacao

import (
	"os"
	"path/filepath"
)

// Atual é a instalação deste computador: a pasta da API pelo serviço do sistema; sem o serviço, a do gerenciador.
func Atual() Instalacao {
	pasta := pastaDoServico()
	if pasta == "" {
		if exe, err := os.Executable(); err == nil {
			pasta = filepath.Dir(exe)
		}
	}
	return Ler(pasta)
}
