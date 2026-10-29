package desenvolvimento

import (
	"context"
	"os/exec"
	"path/filepath"
)

// No Linux, o serviço é do próprio usuário: trazer o develop e reinstalar rodam no gerenciador, sem senha, com a saída
// ao vivo, e a reinstalação é o instalar.sh que o atualizar.sh rodaria.

func semJanela(*exec.Cmd) {}

// TrazerCodigo traz o develop pelo atualizar.sh --so-codigo.
func TrazerCodigo(ctx context.Context, raiz string, linha func(string)) int {
	return RodarAoVivo(ctx, exec.CommandContext(ctx, "bash", filepath.Join(raiz, "scripts", "geral", "atualizar.sh"), "--so-codigo"), linha)
}

// Reinstalar roda o instalar.sh do clone, na pasta dele.
func Reinstalar(ctx context.Context, raiz string, linha func(string)) int {
	cmd := exec.CommandContext(ctx, "bash", filepath.Join(raiz, "scripts", "geral", "instalar.sh"))
	cmd.Dir = raiz
	return RodarAoVivo(ctx, cmd, linha)
}

// ReinstalarElevado é do Windows (--instalar-pelos-scripts): 3, não é instalação de desenvolvimento.
func ReinstalarElevado() int { return 3 }
