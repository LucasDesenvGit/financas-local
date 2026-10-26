package respostas

import (
	"bytes"
	"fmt"
)

// ErroDeSintaxe é o erro do System.Text.Json para o JSON que não abre: a mensagem do Utf8JsonReader, o caminho e a
// posição (linha a partir de zero e o byte na linha), como o ASP.NET põe em "errors". O erro do Go (err) não diz o
// bastante (em que ponto o leitor estava, qual propriedade lia), então o corpo é relido aqui, byte a byte, até o
// primeiro problema. O gabarito é o do próprio .NET (testdata/erros-json-dotnet.json, gravado pela sonda-json).
//
//   - fim inesperado: a mensagem depende do que se esperava (fim do texto, um valor, nome de propriedade, o resto do
//     número ou do literal); a posição é o fim do corpo, menos quando ele acaba numa vírgula, que o .NET devolve;
//   - caractere errado: a mensagem diz onde (início de valor, de nome, depois do nome, depois do valor, depois do JSON
//     inteiro), na posição do caractere;
//   - caminho: "$.<propriedade>" enquanto se lê o valor de uma propriedade do objeto de fora (em qualquer
//     profundidade dentro dele); "$" no resto.
func ErroDeSintaxe(corpo []byte, _ error) *ErroJSON {
	l := leitorSintaxe{corpo: corpo}
	caminho, mensagem, posicao := l.ler()
	return &ErroJSON{caminho, fmt.Sprintf("%s Path: %s | %s.", mensagem, caminho, Posicao(corpo, posicao))}
}

const (
	esperaValor        = iota // um valor (no começo, depois de ":" ou "," numa lista, ou logo depois de "[")
	esperaNome                // um nome de propriedade (depois de "{" ou de "," num objeto)
	esperaDoisPontos          // ":" depois do nome
	esperaVirgulaOuFim        // "," ou o fecho, depois de um valor dentro de objeto ou lista
	terminou                  // o valor de fora acabou: só pode vir espaço
)

const (
	mensagemFimDoTexto   = "Expected end of string, but instead reached end of data."
	mensagemProfundidade = "Expected depth to be zero at the end of the JSON payload. There is an open JSON object or array that should be closed."
	mensagemInicioNome   = "Expected start of a property name or value, but instead reached end of data."
	mensagemUmValor      = "Expected a value, but instead reached end of data."
)

type leitorSintaxe struct {
	corpo      []byte
	i          int
	pilha      []byte // '{' ou '['
	estado     int
	logoApos   bool   // acabou de abrir "{" ou "[" (o "}" ou "]" vale) ou de ler "," (não vale: vírgula sobrando)
	nomeDeFora string // a propriedade do objeto de fora cujo valor está sendo lido
}

func (l *leitorSintaxe) caminho() string {
	if l.nomeDeFora == "" {
		return "$"
	}
	return CaminhoDaChave("$", l.nomeDeFora)
}

func espaco(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

// ler anda o corpo até o primeiro problema e devolve o caminho, a mensagem e a posição. Corpo sem problema (não
// devia chegar aqui) sai como fim inesperado.
func (l *leitorSintaxe) ler() (string, string, int) {
	for {
		for l.i < len(l.corpo) && espaco(l.corpo[l.i]) {
			l.i++
		}
		if l.i >= len(l.corpo) {
			return l.fimInesperado()
		}
		c := l.corpo[l.i]
		switch l.estado {
		case terminou:
			return "$", fmt.Sprintf("'%c' is invalid after a single JSON value. Expected end of data.", c), l.i
		case esperaDoisPontos:
			if c != ':' {
				return "$", fmt.Sprintf("'%c' is invalid after a property name. Expected a ':'.", c), l.i
			}
			l.i++
			l.estado = esperaValor
		case esperaNome:
			if c == '}' && l.logoApos {
				l.fechar()
				continue
			}
			if c == '}' {
				return "$", "The JSON object contains a trailing comma at the end which is not supported in this mode. Change the reader options.", l.i
			}
			if c != '"' {
				return "$", fmt.Sprintf("'%c' is an invalid start of a property name. Expected a '\"'.", c), l.i
			}
			nome, ok := l.texto()
			if !ok {
				return "$", mensagemFimDoTexto, len(l.corpo)
			}
			if len(l.pilha) == 1 {
				l.nomeDeFora = nome
			}
			l.estado = esperaDoisPontos
		case esperaVirgulaOuFim:
			fecho := byte('}')
			if l.pilha[len(l.pilha)-1] == '[' {
				fecho = ']'
			}
			switch c {
			case ',':
				l.i++
				l.logoApos = false
				if fecho == '}' {
					l.estado = esperaNome
				} else {
					l.estado = esperaValor
				}
			case fecho:
				l.fechar()
			default:
				return "$", fmt.Sprintf("'%c' is invalid after a value. Expected either ',', '}', or ']'.", c), l.i
			}
		case esperaValor:
			if caminho, mensagem, posicao, erro := l.valor(c); erro {
				return caminho, mensagem, posicao
			}
		}
	}
}

// valor lê um valor que começa em c; devolve true e o erro se achou um.
func (l *leitorSintaxe) valor(c byte) (string, string, int, bool) {
	switch {
	case c == '{' || c == '[':
		l.pilha = append(l.pilha, c)
		l.i++
		l.logoApos = true
		if c == '{' {
			l.estado = esperaNome
		}
		return "", "", 0, false
	case c == ']' && l.logoApos && len(l.pilha) > 0 && l.pilha[len(l.pilha)-1] == '[':
		l.fechar()
		return "", "", 0, false
	case c == '"':
		if _, ok := l.texto(); !ok {
			return l.caminho(), mensagemFimDoTexto, len(l.corpo), true
		}
	case c == '-' || (c >= '0' && c <= '9'):
		for l.i < len(l.corpo) && bytes.IndexByte([]byte("0123456789+-.eE"), l.corpo[l.i]) >= 0 {
			l.i++
		}
		if l.i >= len(l.corpo) && len(l.pilha) > 0 {
			return l.caminho(), fmt.Sprintf("'%c' is an invalid end of a number. Expected a delimiter.", l.corpo[l.i-1]), len(l.corpo), true
		}
	case c == 't' || c == 'f' || c == 'n':
		literal := map[byte]string{'t': "true", 'f': "false", 'n': "null"}[c]
		inicio := l.i
		for l.i < len(l.corpo) && l.i-inicio < len(literal) && l.corpo[l.i] == literal[l.i-inicio] {
			l.i++
		}
		if l.i-inicio < len(literal) {
			lido := string(l.corpo[inicio:l.i])
			if l.i < len(l.corpo) {
				lido += string(l.corpo[l.i])
			}
			return l.caminho(), fmt.Sprintf("'%s' is an invalid JSON literal. Expected the literal '%s'.", lido, literal), l.i, true
		}
	default:
		return l.caminho(), fmt.Sprintf("'%c' is an invalid start of a value.", c), l.i, true
	}
	l.depoisDoValor()
	return "", "", 0, false
}

// texto lê uma string a partir das aspas; false se o corpo acaba antes de ela fechar.
func (l *leitorSintaxe) texto() (string, bool) {
	inicio := l.i + 1
	for j := inicio; j < len(l.corpo); j++ {
		switch l.corpo[j] {
		case '\\':
			j++
		case '"':
			l.i = j + 1
			return string(l.corpo[inicio:j]), true
		}
	}
	return "", false
}

func (l *leitorSintaxe) fechar() {
	l.pilha = l.pilha[:len(l.pilha)-1]
	l.i++
	l.depoisDoValor()
}

func (l *leitorSintaxe) depoisDoValor() {
	l.logoApos = false
	if len(l.pilha) == 0 {
		l.estado = terminou
		return
	}
	if len(l.pilha) == 1 {
		l.nomeDeFora = "" // o valor da propriedade de fora acabou
	}
	l.estado = esperaVirgulaOuFim
}

// fimInesperado é o erro do corpo que acaba antes de o JSON fechar.
func (l *leitorSintaxe) fimInesperado() (string, string, int) {
	fim := len(l.corpo)
	switch l.estado {
	case esperaDoisPontos:
		return "$", mensagemUmValor, fim
	case esperaNome:
		if l.logoApos {
			return "$", mensagemProfundidade, fim
		}
		// Depois de ",": o .NET devolve a vírgula quando ela é o último byte.
		if fim > 0 && l.corpo[fim-1] == ',' {
			fim--
		}
		return "$", mensagemInicioNome, fim
	case esperaValor:
		if len(l.pilha) == 0 {
			return "$", mensagemInicioNome, fim
		}
		if l.pilha[len(l.pilha)-1] == '[' && !l.logoApos {
			if l.corpo[fim-1] == ',' {
				fim--
			}
			return l.caminho(), mensagemInicioNome, fim
		}
		return l.caminho(), mensagemProfundidade, fim
	}
	return "$", mensagemProfundidade, fim
}
