package segredos

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// entropia separa este uso do DPAPI de outros da mesma conta (o "propósito" do Data Protection).
var entropia = []byte("FinancasLocal.Segredos.v2")

// proteger cifra a chave com o DPAPI da conta que roda o programa (CryptProtectData): só essa conta, neste
// computador, consegue abrir o arquivo. É a mesma proteção que o .NET usa hoje nas chaves do Data Protection.
func proteger(chave []byte) ([]byte, error) {
	var saida windows.DataBlob
	err := windows.CryptProtectData(blob(chave), nil, blob(entropia), 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &saida)
	if err != nil {
		return nil, fmt.Errorf("CryptProtectData: %w", err)
	}
	return copiarELiberar(saida), nil
}

// desproteger é o CryptUnprotectData: falha se o arquivo foi mexido ou é de outra conta ou outro computador.
func desproteger(guardada []byte) ([]byte, error) {
	if len(guardada) == 0 {
		return nil, fmt.Errorf("arquivo vazio")
	}
	var saida windows.DataBlob
	err := windows.CryptUnprotectData(blob(guardada), nil, blob(entropia), 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &saida)
	if err != nil {
		return nil, fmt.Errorf("CryptUnprotectData: %w", err)
	}
	return copiarELiberar(saida), nil
}

func blob(dados []byte) *windows.DataBlob {
	return &windows.DataBlob{Size: uint32(len(dados)), Data: unsafe.SliceData(dados)}
}

// copiarELiberar copia o resultado para a memória do Go e devolve ao Windows a que ele alocou (LocalFree).
func copiarELiberar(b windows.DataBlob) []byte {
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(b.Data)))
	return append([]byte(nil), unsafe.Slice(b.Data, b.Size)...)
}
