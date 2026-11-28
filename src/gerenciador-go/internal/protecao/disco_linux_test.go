package protecao

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Regra (#68, item 2): avisa só quando o findmnt e o lsblk dizem que não há "crypt"; qualquer falha, origem que não é
// dispositivo ou pasta criptografada pelo sistema de arquivos não avisa. Troca os comandos (não roda em paralelo).
func TestSemCriptografia(t *testing.T) {
	rodarAntes, fscryptAntes := rodar, cifradaPeloSistemaDeArquivos
	t.Cleanup(func() { rodar, cifradaPeloSistemaDeArquivos = rodarAntes, fscryptAntes })
	erro := errors.New("falhou")
	casos := []struct {
		nome        string
		pasta       string
		fscrypt     bool
		findmnt     string
		erroFindmnt error
		lsblk       string
		erroLsblk   error
		esperado    bool
	}{
		{"partição comum", "/home/ana/dados", false, "/dev/sda2[/home]\n", nil, "part\ndisk\n", nil, true},
		{"LUKS", "/home/ana/dados", false, "/dev/mapper/luks-1[/home]\n", nil, "crypt\npart\ndisk\n", nil, false},
		{"fscrypt", "/home/ana/dados", true, "/dev/sda2\n", nil, "part\ndisk\n", nil, false},
		{"findmnt falha", "/home/ana/dados", false, "", erro, "part\ndisk\n", nil, false},
		{"não é dispositivo", "/home/ana/dados", false, "tmpfs\n", nil, "part\ndisk\n", nil, false},
		{"lsblk falha", "/home/ana/dados", false, "/dev/sda2\n", nil, "", erro, false},
		{"sem pasta", "", false, "/dev/sda2\n", nil, "part\ndisk\n", nil, false},
	}
	for _, c := range casos {
		var pedidos []string
		cifradaPeloSistemaDeArquivos = func(string) bool { return c.fscrypt }
		rodar = func(_ context.Context, _ time.Duration, programa string, argumentos ...string) (string, error) {
			pedidos = append(pedidos, programa+" "+strings.Join(argumentos, " "))
			if programa == "findmnt" {
				return c.findmnt, c.erroFindmnt
			}
			return c.lsblk, c.erroLsblk
		}
		if obtido := SemCriptografia(c.pasta); obtido != c.esperado {
			t.Errorf("%s: %v, esperado %v (%q)", c.nome, obtido, c.esperado, pedidos)
		}
		if c.nome == "partição comum" && (len(pedidos) != 2 || pedidos[0] != "findmnt -n -o SOURCE --target /home/ana/dados" ||
			pedidos[1] != "lsblk -s -n -r -o TYPE /dev/sda2") {
			t.Errorf("comandos: %q", pedidos)
		}
	}
}
