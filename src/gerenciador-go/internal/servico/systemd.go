package servico

import "strings"

// Unidade do systemd --user que roda o site no Linux, escrita pelo 3-publicar-atualizar.sh. É do próprio usuário:
// ligar, desligar e ler o registro não pedem senha nem administrador (ADR 0033).
const Unidade = "financaslocal.service"

// Propriedades lê a saída do "systemctl show" (uma Chave=valor por linha), como o Propriedades do ServicoLinux.cs.
func Propriedades(texto string) map[string]string {
	p := map[string]string{}
	for linha := range strings.SplitSeq(texto, "\n") {
		if chave, valor, ok := strings.Cut(linha, "="); ok && chave != "" {
			p[chave] = strings.TrimSpace(valor)
		}
	}
	return p
}

// EstadoDaUnidade é o Estado do ServicoLinux.cs: unidade que não carregou não está instalada; inactive e failed são
// "parado".
func EstadoDaUnidade(p map[string]string) Estado {
	if p["LoadState"] != "loaded" {
		return NaoInstalado
	}
	switch p["ActiveState"] {
	case "active", "reloading":
		return Rodando
	case "activating":
		return Ligando
	case "deactivating":
		return Desligando
	default:
		return Parado
	}
}

// verbos do systemctl para cada ação do gerenciador.
var verbos = map[string]string{"ligar": "start", "desligar": "stop", "reiniciar": "restart"}
