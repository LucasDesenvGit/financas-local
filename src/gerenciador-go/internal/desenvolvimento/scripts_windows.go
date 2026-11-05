package desenvolvimento

import (
	"context"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
)

// semJanela: os scripts rodam sem abrir a janela do console (a saída vai para a caixa do gerenciador).
func semJanela(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

func powershell(ctx context.Context, script string, argumentos ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "powershell.exe",
		append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script}, argumentos...)...)
}

// TrazerCodigo traz o develop pelo atualizar-windows.ps1 -SoCodigo, como o usuário (o git nunca roda como
// administrador na pasta dele), com a saída ao vivo.
func TrazerCodigo(ctx context.Context, raiz string, linha func(string)) int {
	return RodarAoVivo(ctx, powershell(ctx, filepath.Join(raiz, "scripts", "windows", "atualizar-windows.ps1"), "-SoCodigo"), linha)
}

// ReinstalarElevado roda no processo elevado (--instalar-pelos-scripts): o instalar-windows.ps1 do clone. A pasta
// vem da própria instalação, nunca da linha de comando, e o git não roda aqui. A saída vai para o log do instalador,
// que a janela acompanha. 3: não é instalação de desenvolvimento.
func ReinstalarElevado() int {
	raiz := Raiz(instalacao.Atual())
	if raiz == "" {
		return 3
	}
	cmd := powershell(context.Background(), filepath.Join(raiz, "scripts", "windows", "instalar-windows.ps1"))
	cmd.Dir = raiz
	return RodarAoVivo(context.Background(), cmd, func(string) {})
}
