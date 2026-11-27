// Package protecao confere onde fica a pasta de dados (#68, itens 2 e 14): o disco sem criptografia e a pasta
// sincronizada com a nuvem. O gerenciador só avisa; o banco e as cópias continuam sem criptografia (docs/SEGURANCA.md).
// Na dúvida, não avisa: o aviso só aparece quando o sistema diz com certeza.
package protecao

import (
	"path/filepath"
	"strings"
)

// marcas são pedaços do nome das pastas que os clientes de nuvem sincronizam, em minúsculas ("meu drive" é o "My Drive"
// do Google Drive em português).
var marcas = []string{"onedrive", "dropbox", "google drive", "googledrive", "my drive", "meu drive", "iclouddrive"}

// variaveisDoOneDrive: no Windows, o OneDrive diz onde está a pasta dele, que pode ter outro nome.
var variaveisDoOneDrive = []string{"OneDrive", "OneDriveConsumer", "OneDriveCommercial"}

// NaNuvem diz se o caminho parece estar numa pasta sincronizada com a nuvem: uma pasta do caminho com o nome de um
// cliente de nuvem, ou dentro da pasta que as variáveis do OneDrive apontam. Sem diferença de maiúsculas nem de barras.
func NaNuvem(caminho string, ambiente func(string) string) bool {
	c := normalizar(caminho)
	if c == "" {
		return false
	}
	for _, v := range variaveisDoOneDrive {
		if p := normalizar(ambiente(v)); p != "" && (c == p || strings.HasPrefix(c, p+"/")) {
			return true
		}
	}
	for _, pasta := range strings.Split(c, "/") {
		for _, m := range marcas {
			if strings.Contains(pasta, m) {
				return true
			}
		}
	}
	return false
}

// normalizar: minúsculas, barras normais e sem a barra do fim.
func normalizar(caminho string) string {
	return strings.TrimRight(strings.ToLower(strings.ReplaceAll(caminho, `\`, "/")), "/")
}

// LetraDoDisco é a letra do disco de um caminho do Windows ("C:"), ou "" (pasta de rede, caminho relativo).
func LetraDoDisco(caminho string) string {
	if len(caminho) < 2 || caminho[1] != ':' {
		return ""
	}
	l := caminho[0] | 0x20
	if l < 'a' || l > 'z' {
		return ""
	}
	return strings.ToUpper(caminho[:2])
}

// BitLockerDesligado lê o System.Volume.BitLockerProtection do Explorer (que não pede administrador): 2 é "desligado".
// Os outros (1 ligado, 3 criptografando, 4 descriptografando, 5 suspenso, 6 bloqueado, 8 esperando ativar) e o vazio
// (disco sem suporte) não avisam.
func BitLockerDesligado(saida string) bool { return strings.TrimSpace(saida) == "2" }

// OrigemDoMonte é o dispositivo do findmnt -no SOURCE, sem o subvolume do btrfs ("/dev/sda2[/home]" vira "/dev/sda2");
// "" quando não é um dispositivo (tmpfs, rede, FUSE, ZFS), em que não dá para saber.
func OrigemDoMonte(saida string) string {
	o := strings.TrimSpace(saida)
	if i := strings.IndexByte(o, '['); i >= 0 {
		o = o[:i]
	}
	if !strings.HasPrefix(o, "/dev/") || filepath.Clean(o) != o {
		return ""
	}
	return o
}

// TiposSemCriptografia lê o lsblk -s -n -r -o TYPE (o dispositivo e os que estão por baixo dele): true quando há
// algum e nenhum é "crypt" (dm-crypt, que é o do LUKS); sem nenhum, não dá para saber.
func TiposSemCriptografia(saida string) bool {
	tipos := strings.Fields(saida)
	for _, t := range tipos {
		if t == "crypt" {
			return false
		}
	}
	return len(tipos) > 0
}
