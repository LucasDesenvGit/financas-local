//go:build windows

package celular

// FirewallDoSistema é o firewall do Windows, pelo netsh.
func FirewallDoSistema() Firewall { return netsh{rodar: rodarDeVerdade} }
