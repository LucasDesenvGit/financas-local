//go:build !windows

package preferencias

// No Linux, as preferências e o tema entram com o Wails testado lá (#417).

func IniciarComSistema() bool { return false }
func ProcurarAoAbrir() bool   { return true }
func Escuro() bool            { return false }
