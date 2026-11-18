package main

import (
	"fmt"
	"regexp"
	"strconv"
)

// numero é o X.Y.Z da versão (a 0070 sobe o do meio e o último; o primeiro só muda pela base).
type numero struct{ x, y, z int }

func (n numero) String() string { return fmt.Sprintf("%d.%d.%d", n.x, n.y, n.z) }

// subir é a tabela da ADR 0039, que a 0070 manteve: funcionalidade (feat) sobe o do meio e zera o último; o resto,
// o último.
func (n numero) subir(funcionalidade bool) numero {
	if funcionalidade {
		return numero{n.x, n.y + 1, 0}
	}
	return numero{n.x, n.y, n.z + 1}
}

var textoDeNumero = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

func lerNumero(texto string) (numero, error) {
	m := textoDeNumero.FindStringSubmatch(texto)
	if m == nil {
		return numero{}, fmt.Errorf("versão %q fora do formato X.Y.Z", texto)
	}
	var partes [3]int
	for i := range partes {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return numero{}, fmt.Errorf("versão %q: %w", texto, err)
		}
		partes[i] = n
	}
	return numero{partes[0], partes[1], partes[2]}, nil
}

// arquivoDaBase é onde mora a base; propriedadeDaBase, o elemento dela.
const (
	arquivoDaBase = "Directory.Build.props"
	elementoBase  = "VersaoBase"
)

var versaoBaseNoProps = regexp.MustCompile(`<` + elementoBase + `>\s*([^<\s]*)\s*</` + elementoBase + `>`)

// lerBase lê o <VersaoBase> do conteúdo do Directory.Build.props.
func lerBase(props string) (numero, error) {
	m := versaoBaseNoProps.FindStringSubmatch(props)
	if m == nil {
		return numero{}, fmt.Errorf("sem <%s>X.Y.Z</%s> no %s", elementoBase, elementoBase, arquivoDaBase)
	}
	return lerNumero(m[1])
}
