//go:build !windows && !linux

package parametros

// descricaoNativa fora do Windows e do Linux (onde o site não roda): só o nome do sistema.
func descricaoNativa() string { return "" }
