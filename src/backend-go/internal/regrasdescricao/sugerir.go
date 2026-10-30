package regrasdescricao

import (
	"strings"
	"unicode"
)

// SugerirTexto é o SugerirTexto do C#: o texto sugerido para a regra a partir da descrição do banco, sem os pedaços
// com número (o "1/10", a data, o código da compra). O C# troca cada pedaço sem espaço que tem um dígito por um espaço
// (o Regex "\S*\d\S*"), junta os espaços ("\s+") e apara ' ', '-', '*', '/', '.' e ','. Sobrando menos de 3 letras,
// fica a descrição inteira, aparada.
//
// As classes do Regex do .NET são de Unicode: \d é o dígito decimal (Nd) e \s, o espaço (os separadores Z e os de
// controle). O RE2 do Go entende \d só como 0 a 9, por isso o laço à mão. O C# olha um char (UTF-16) por vez: a letra
// fora do plano básico chega em duas metades, que não são letra nem dígito.
func SugerirTexto(descricaoOriginal string) string {
	// Trocar cada pedaço com dígito por um espaço, juntar os espaços e aparar as pontas é o mesmo que juntar com um
	// espaço só os pedaços sem dígito.
	var mantidos []string
	for _, pedaco := range strings.Fields(descricaoOriginal) {
		if !strings.ContainsFunc(pedaco, func(r rune) bool { return unicode.Is(unicode.Nd, r) }) {
			mantidos = append(mantidos, pedaco)
		}
	}
	sem := strings.Trim(strings.Join(mantidos, " "), " -*/.,")
	letras := 0
	for _, r := range sem {
		if r < 0x10000 && unicode.IsLetter(r) {
			letras++
		}
	}
	if letras >= 3 {
		return sem
	}
	return strings.TrimFunc(descricaoOriginal, unicode.IsSpace)
}
