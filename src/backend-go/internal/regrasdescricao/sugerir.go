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
	var b strings.Builder
	espaco := false
	escrever := func(pedaco string) {
		if pedaco == "" {
			return
		}
		if strings.ContainsFunc(pedaco, func(r rune) bool { return unicode.Is(unicode.Nd, r) }) {
			espaco = true
			return
		}
		if espaco && b.Len() > 0 {
			b.WriteByte(' ')
		}
		espaco = false
		b.WriteString(pedaco)
	}
	inicio := 0
	for i, r := range descricaoOriginal {
		if unicode.IsSpace(r) {
			escrever(descricaoOriginal[inicio:i])
			espaco = true
			inicio = i + len(string(r))
		}
	}
	escrever(descricaoOriginal[inicio:])
	sem := strings.Trim(b.String(), " -*/.,")
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
