//go:build !windows

package servico

import "errors"

// No Linux, o systemctl --user entra com a #417.

const ComoInstalar = "Instale pelo scripts/geral/instalar.sh."

var errLinux = errors.New("o serviço no Linux entra com a #417")

func Consultar() (Estado, error) { return NaoInstalado, errLinux }
func Executar(string) error      { return errLinux }
