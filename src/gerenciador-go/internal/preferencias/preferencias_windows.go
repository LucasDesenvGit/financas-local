// Package preferencias lê as preferências do gerenciador, por usuário, nos mesmos lugares do Preferencias.cs (nada
// disso precisa de administrador).
//
// Só leitura por enquanto: gravar a entrada de iniciar com o Windows trocaria o gerenciador em C# pelo em Go no
// início do Windows, e isso só vale quando o .msi instalar o em Go (#419, #277).
package preferencias

import "golang.org/x/sys/windows/registry"

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
