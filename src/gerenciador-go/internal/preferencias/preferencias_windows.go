// Package preferencias lê e grava as preferências do gerenciador, por usuário, nos mesmos lugares do Preferencias.cs
// (nada disso precisa de administrador).
//
// A entrada de iniciar com o Windows tem o mesmo nome da do C#: ligar no gerenciador em Go troca o que abre com o
// Windows, como vai ser quando o .msi instalar o em Go (#277).
package preferencias

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	chave        = `Software\FinancasLocal\Gerenciador`
	chaveIniciar = `Software\Microsoft\Windows\CurrentVersion\Run`
	nomeIniciar  = "FinancasLocalGerenciador"
)

// IniciarComSistema: abre na área de notificação quando o Windows inicia (padrão: não).
func IniciarComSistema() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, chaveIniciar, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(nomeIniciar)
	return err == nil
}

// DefinirIniciarComSistema grava (ou apaga) a entrada do Windows que abre o gerenciador com --bandeja.
func DefinirIniciarComSistema(ligado bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, chaveIniciar, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !ligado {
		if err := k.DeleteValue(nomeIniciar); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(nomeIniciar, `"`+strings.TrimRight(exe, `\`)+`" --bandeja`)
}

// ProcurarAoAbrir: procura versão nova ao abrir (padrão: sim).
func ProcurarAoAbrir() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, chave, registry.QUERY_VALUE)
	if err != nil {
		return true
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("ProcurarAoAbrir")
	return err != nil || v != 0
}

// DefinirProcurarAoAbrir grava a preferência (DWORD 1 ou 0, como o C#).
func DefinirProcurarAoAbrir(ligado bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, chave, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	valor := uint32(0)
	if ligado {
		valor = 1
	}
	return k.SetDWordValue("ProcurarAoAbrir", valor)
}

// ReceberDesenvolvimento: "Procurar versão nova" olha o pré-lançamento "desenvolvimento" (ADR 0040). Sem a escolha
// gravada, vale o tipo da versão instalada (padrao), como era antes do interruptor.
func ReceberDesenvolvimento(padrao bool) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, chave, registry.QUERY_VALUE)
	if err != nil {
		return padrao
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("ReceberDesenvolvimento")
	if err != nil {
		return padrao
	}
	return v != 0
}

// DefinirReceberDesenvolvimento grava a escolha (DWORD 1 ou 0).
func DefinirReceberDesenvolvimento(ligado bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, chave, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	valor := uint32(0)
	if ligado {
		valor = 1
	}
	return k.SetDWordValue("ReceberDesenvolvimento", valor)
}

// Escuro: o Windows em modo escuro de aplicativos (HKCU …\Personalize\AppsUseLightTheme = 0), como o Tema.Detectar.
func Escuro() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 0
}

// PastaDocumentos é onde o "Salvar uma cópia do banco" começa (Meus documentos).
func PastaDocumentos() string {
	if pasta, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0); err == nil {
		return pasta
	}
	casa, _ := os.UserHomeDir()
	return filepath.Join(casa, "Documents")
}
