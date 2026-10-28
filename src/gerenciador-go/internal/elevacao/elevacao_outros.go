//go:build !windows

package elevacao

// Executar: no Linux, o systemctl --user não pede administrador (#417).
func Executar(...string) Resultado { return Falhou }
