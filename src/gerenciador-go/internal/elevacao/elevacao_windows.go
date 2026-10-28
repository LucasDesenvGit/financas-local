package elevacao

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SHELLEXECUTEINFOW (shellapi.h). O x/sys/windows só traz o ShellExecute, que não devolve o processo para esperar.
type shellExecuteInfo struct {
	tamanho     uint32
	mascara     uint32
	janela      uintptr
	verbo       *uint16
	arquivo     *uint16
	parametros  *uint16
	pasta       *uint16
	mostrar     int32
	instancia   uintptr
	listaIDs    uintptr
	classe      *uint16
	chaveClasse uintptr
	teclaAtalho uint32
	iconeOuTela uintptr
	processo    windows.Handle
}

// SEE_MASK_NOCLOSEPROCESS devolve o processo criado, para esperar e ler o código de saída; SEE_MASK_NOASYNC termina
// tudo antes de voltar, porque a chamada sai de uma goroutine, sem laço de mensagens (documentação do SHELLEXECUTEINFOW).
const (
	seeMaskNoCloseProcess = 0x40
	seeMaskNoAsync        = 0x100
)

var shellExecuteEx = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// Executar roda "gerenciador.exe argumentos" como administrador e espera. Cancelar o aviso do UAC dá Cancelado.
func Executar(argumentos ...string) Resultado {
	exe, err := os.Executable()
	if err != nil {
		return Falhou
	}
	partes := make([]string, len(argumentos))
	for i, a := range argumentos {
		partes[i] = syscall.EscapeArg(a)
	}
	verbo, _ := windows.UTF16PtrFromString("runas")
	arquivo, _ := windows.UTF16PtrFromString(exe)
	parametros, _ := windows.UTF16PtrFromString(strings.Join(partes, " "))
	info := shellExecuteInfo{mascara: seeMaskNoCloseProcess | seeMaskNoAsync, verbo: verbo, arquivo: arquivo, parametros: parametros,
		mostrar: windows.SW_HIDE}
	info.tamanho = uint32(unsafe.Sizeof(info))
	if ok, _, err := shellExecuteEx.Call(uintptr(unsafe.Pointer(&info))); ok == 0 {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return Cancelado
		}
		return Falhou
	}
	defer windows.CloseHandle(info.processo)
	if _, err := windows.WaitForSingleObject(info.processo, windows.INFINITE); err != nil {
		return Falhou
	}
	var codigo uint32
	if err := windows.GetExitCodeProcess(info.processo, &codigo); err != nil || codigo != 0 {
		return Falhou
	}
	return Feito
}
