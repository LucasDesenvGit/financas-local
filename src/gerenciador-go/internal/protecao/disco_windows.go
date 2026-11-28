package protecao

import (
	"bytes"
	"context"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// lerBitLocker pergunta ao Explorer (Shell.Application) o estado do BitLocker do disco ("C:"), sem pedir
// administrador, como o cadeado do disco no Explorer (o manage-bde e o Get-BitLockerVolume pedem). Trocado nos testes.
var lerBitLocker = func(letra string) (string, error) {
	ctx, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelar()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"(New-Object -ComObject Shell.Application).NameSpace('"+letra+"').Self.ExtendedProperty('System.Volume.BitLockerProtection')")
	// Sem a janela do console piscando (o gerenciador é uma janela, sem console).
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	var saida bytes.Buffer
	cmd.Stdout = &saida
	err := cmd.Run()
	return saida.String(), err
}

// SemCriptografia diz se o BitLocker está desligado no disco da pasta. Pasta de rede, falha ou outro estado é "não dá
// para saber" (false).
func SemCriptografia(pasta string) bool {
	letra := LetraDoDisco(pasta)
	if letra == "" {
		return false
	}
	saida, err := lerBitLocker(letra)
	return err == nil && BitLockerDesligado(saida)
}
