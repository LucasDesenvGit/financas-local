package parametros

import (
	"bytes"

	"golang.org/x/sys/unix"
)

// descricaoNativa é o OSDescription do .NET no Linux: o nome, a versão e a compilação do kernel (uname).
func descricaoNativa() string {
	var u unix.Utsname
	if err := unix.Uname(&u); err != nil {
		return ""
	}
	texto := func(b []byte) string { return string(bytes.TrimRight(b, "\x00")) }
	return texto(u.Sysname[:]) + " " + texto(u.Release[:]) + " " + texto(u.Version[:])
}
