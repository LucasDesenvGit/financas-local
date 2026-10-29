// Pacote texto tem as duas normalizações de texto das regras do .NET, conferidas contra o próprio C# por uma sonda
// (testdata/sonda-csharp grava testdata/normalizar.json, o gabarito dos testes).
//
//   - ParaRegras é o CategorizacaoService.Normalizar: as regras de categoria e de descrição comparam o texto sem acento
//     e em maiúsculas;
//   - ParaPalavras é o Normalizar do TipoLancamentoService: além disso, só letras e dígitos, separados por um espaço,
//     para procurar palavras inteiras ("PIX", "PAGAMENTO CARTAO").
package texto

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ParaRegras é o CategorizacaoService.Normalizar do C#: tira os espaços das pontas, decompõe (FormD), descarta os
// acentos (as marcas sem espaço, a categoria Mn), recompõe (FormC) e passa para maiúsculas como o ToUpperInvariant.
// Só espaços (ou nada) vira "".
func ParaRegras(valor string) string {
	valor = strings.TrimFunc(valor, unicode.IsSpace)
	if valor == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range norm.NFD.String(valor) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return strings.Map(maiusculaInvariante, norm.NFC.String(b.String()))
}

// ParaPalavras é o Normalizar do TipoLancamentoService do C#: decompõe (FormD, sem recompor depois), descarta os
// acentos, deixa só letras e dígitos (em maiúsculas) e troca o resto por um espaço, sem espaços repetidos nem nas pontas.
//
// O C# olha um char (UTF-16) por vez: um caractere fora do plano básico, como um emoji ou o "𝐀" matemático, chega em
// duas metades (surrogates), que não são letra, e vira espaço. Por isso a mesma regra aqui (r > 0xFFFF).
func ParaPalavras(valor string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(valor) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r <= 0xFFFF && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(maiusculaInvariante(r))
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// TemPalavra diz se algum dos termos (palavra ou expressão já normalizada) aparece inteiro no texto normalizado por
// ParaPalavras: o Tem do TipoLancamentoService (" " + texto + " " contém " " + termo + " ").
func TemPalavra(texto string, termos ...string) bool {
	comEspacos := " " + texto + " "
	for _, t := range termos {
		if strings.Contains(comEspacos, " "+t+" ") {
			return true
		}
	}
	return false
}

// MaiusculasInvariantes é o ToUpperInvariant do .NET para um texto (ver maiusculaInvariante).
func MaiusculasInvariantes(s string) string { return strings.Map(maiusculaInvariante, s) }

// maiusculaInvariante é o ToUpperInvariant do .NET para um caractere: o mapeamento simples do Unicode, menos o "ı"
// (i sem ponto, U+0131), que o .NET deixa como está (o "I" dele é só do turco). Conferido no gabarito.
func maiusculaInvariante(r rune) rune {
	if r == 'ı' {
		return r
	}
	return unicode.ToUpper(r)
}
