//go:build !windows

package desenvolvimento

import (
	"context"
	"os/exec"
)

// No Linux, o atualizar.sh e o serviço do usuário entram com a #417.

func semJanela(*exec.Cmd) {}

func TrazerCodigo(context.Context, string, func(string)) int { return -1 }

func ReinstalarElevado() int { return 3 }
