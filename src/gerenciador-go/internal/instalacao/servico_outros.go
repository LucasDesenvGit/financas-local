//go:build !windows

package instalacao

// pastaDoServico: no Linux, a unidade do systemd --user entra com a #417.
func pastaDoServico() string { return "" }
