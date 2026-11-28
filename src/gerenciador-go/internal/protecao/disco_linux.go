package protecao

import (
	"context"
	"time"

	"golang.org/x/sys/unix"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/comando"
)

// rodar e cifradaPeloSistemaDeArquivos são trocados nos testes, para não depender dos discos da máquina.
var (
	rodar = comando.Rodar
	// cifradaPeloSistemaDeArquivos: a pasta criptografada pelo próprio ext4 ou f2fs (fscrypt), que o lsblk não mostra.
	cifradaPeloSistemaDeArquivos = func(pasta string) bool {
		var st unix.Statx_t
		if unix.Statx(unix.AT_FDCWD, pasta, 0, unix.STATX_BASIC_STATS, &st) != nil {
			return false
		}
		return st.Attributes_mask&st.Attributes&unix.STATX_ATTR_ENCRYPTED != 0
	}
)

// SemCriptografia diz se o disco da pasta não tem criptografia: o findmnt dá o dispositivo da pasta, e o lsblk -s, ele
// e os que estão por baixo (LVM, partição, disco); sem nenhum "crypt" (LUKS), é sem criptografia. Qualquer falha, ou
// uma origem que não é dispositivo, é "não dá para saber" (false).
func SemCriptografia(pasta string) bool {
	if pasta == "" || cifradaPeloSistemaDeArquivos(pasta) {
		return false
	}
	ctx := context.Background()
	saida, err := rodar(ctx, 5*time.Second, "findmnt", "-n", "-o", "SOURCE", "--target", pasta)
	if err != nil {
		return false
	}
	origem := OrigemDoMonte(saida)
	if origem == "" {
		return false
	}
	tipos, err := rodar(ctx, 5*time.Second, "lsblk", "-s", "-n", "-r", "-o", "TYPE", origem)
	return err == nil && TiposSemCriptografia(tipos)
}
