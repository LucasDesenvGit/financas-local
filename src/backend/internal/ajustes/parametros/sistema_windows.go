package parametros

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// descricaoNativa descreve o sistema no Windows: "Microsoft Windows 10.0.26200" (a versão de verdade, pelo
// RtlGetVersion, que não depende do manifesto do programa).
func descricaoNativa() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("Microsoft Windows %d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}
