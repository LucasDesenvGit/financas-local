// Pacote parcelas tem o que o ParcelasFuturas do C# usa para reconhecer as parcelas de uma mesma compra.
package parcelas

import (
	"unicode"
	"unicode/utf16"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
)

// LetrasDaCompra é quantas letras da descrição identificam a compra.
const LetrasDaCompra = 8

// Compra é o ParcelasFuturas.Compra do C#: as 8 primeiras letras da descrição em maiúsculas (ToUpperInvariant), sem o
// número da parcela ("03/10", "PARC 3 DE 10"). Duas parcelas da mesma compra têm a mesma.
//
// O número sai pela regex \bPARC(ELA)?\b\.?|\d{1,3}\s*(/|DE)\s*\d{1,3}, aqui escrita à mão: no .NET, \d, \s e \b valem
// para qualquer alfabeto (o "٣" árabe é dígito, o espaço sem quebra é espaço), e no regexp do Go (RE2) só para ASCII.
// O .NET também olha um char (UTF-16) por vez: o caractere fora do plano básico, como o "𝐀" matemático, chega em duas
// metades que não são letra, dígito, espaço nem parte de palavra. Por isso o texto é lido em UTF-16.
func Compra(descricao string) string {
	cs := utf16.Encode([]rune(texto.MaiusculasInvariantes(descricao)))
	letras := make([]rune, 0, LetrasDaCompra)
	for i := 0; i < len(cs) && len(letras) < LetrasDaCompra; {
		if fim, ok := numeroDaParcela(cs, i); ok {
			i = fim // o trecho vira um espaço, que não é letra
			continue
		}
		if r := rune(cs[i]); !utf16.IsSurrogate(r) && unicode.IsLetter(r) {
			letras = append(letras, r)
		}
		i++
	}
	return string(letras)
}

// numeroDaParcela tenta a regex na posição i, como o Regex.Replace: a primeira alternativa, depois a segunda. Devolve
// onde o trecho reconhecido termina.
func numeroDaParcela(cs []uint16, i int) (int, bool) {
	// \bPARC(ELA)?\b\.? : "PARC" é palavra, então a fronteira antes pede que o char anterior não seja de palavra.
	if tem(cs, i, "PARC") && (i == 0 || !dePalavra(cs[i-1])) {
		fim := -1
		switch {
		case tem(cs, i+4, "ELA") && !dePalavraEm(cs, i+7):
			fim = i + 7
		case !dePalavraEm(cs, i+4): // (ELA)? volta atrás quando falta a fronteira depois de "PARCELA"
			fim = i + 4
		}
		if fim >= 0 {
			if fim < len(cs) && cs[fim] == '.' {
				fim++
			}
			return fim, true
		}
	}
	// \d{1,3}\s*(/|DE)\s*\d{1,3}: com mais de 3 dígitos seguidos, os de antes não servem (depois deles vem dígito, e
	// não espaço, "/" nem "DE"), então só o maior número de dígitos pode dar certo.
	j := i + digitos(cs, i)
	if j == i {
		return 0, false
	}
	j = espacos(cs, j)
	switch {
	case j < len(cs) && cs[j] == '/':
		j++
	case tem(cs, j, "DE"):
		j += 2
	default:
		return 0, false
	}
	j = espacos(cs, j)
	if n := digitos(cs, j); n > 0 {
		return j + n, true
	}
	return 0, false
}

func tem(cs []uint16, i int, s string) bool {
	if i+len(s) > len(cs) {
		return false
	}
	for k := range len(s) {
		if cs[i+k] != uint16(s[k]) {
			return false
		}
	}
	return true
}

// digitos conta os dígitos (\d do .NET: \p{Nd}) a partir de i, até 3.
func digitos(cs []uint16, i int) int {
	n := 0
	for n < 3 && i+n < len(cs) && unicode.Is(unicode.Nd, rune(cs[i+n])) && !utf16.IsSurrogate(rune(cs[i+n])) {
		n++
	}
	return n
}

// espacos pula os espaços (\s do .NET: [\f\n\r\t\v\x85\p{Z}]).
func espacos(cs []uint16, i int) int {
	for i < len(cs) {
		r := rune(cs[i])
		if !(r == '\f' || r == '\n' || r == '\r' || r == '\t' || r == '\v' || r == 0x85 || unicode.Is(unicode.Z, r)) {
			break
		}
		i++
	}
	return i
}

// dePalavra é o que conta como palavra para o \b do .NET: letra, marca sem espaço (Mn), dígito (Nd), conector (Pc) e
// os juntadores de largura zero (ZWJ e ZWNJ).
func dePalavra(c uint16) bool {
	r := rune(c)
	if utf16.IsSurrogate(r) {
		return false
	}
	return unicode.IsLetter(r) || unicode.In(r, unicode.Mn, unicode.Nd, unicode.Pc) || r == 0x200C || r == 0x200D
}

func dePalavraEm(cs []uint16, i int) bool { return i < len(cs) && dePalavra(cs[i]) }
