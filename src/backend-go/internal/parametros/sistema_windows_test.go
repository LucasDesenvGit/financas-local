package parametros

import (
	"regexp"
	"strconv"
	"testing"
)

// Regra: no Windows, o sistema aparece como o OSDescription do .NET, "Microsoft Windows 10.0.26200", pela versão de
// verdade (o Windows 11 também diz 10.0, com o build acima de 22000) (#459).
func TestDescricaoNativaNoWindows(t *testing.T) {
	descricao := descricaoNativa()
	partes := regexp.MustCompile(`^Microsoft Windows (\d+)\.(\d+)\.(\d+)$`).FindStringSubmatch(descricao)
	if partes == nil {
		t.Fatalf("%q fora do formato do .NET", descricao)
	}
	maior, _ := strconv.Atoi(partes[1])
	build, _ := strconv.Atoi(partes[3])
	if maior < 10 || build == 0 {
		t.Errorf("%q: versão que o Finanças Local não roda", descricao)
	}
}
