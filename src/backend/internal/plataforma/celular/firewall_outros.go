//go:build !windows

package celular

import "os"

// FirewallDoSistema é o do Linux: não mexe no firewall, só avisa (ver linux).
func FirewallDoSistema() Firewall { return linux{rodar: rodarDeVerdade, lerArquivo: os.ReadFile} }
