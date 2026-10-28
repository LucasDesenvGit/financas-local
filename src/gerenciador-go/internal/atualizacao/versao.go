// Package atualizacao é a versão nova pelo GitHub Releases (ADR 0028 e 0040), como o Atualizacoes.cs: procurar a
// última versão publicada ou de desenvolvimento, baixar o .msi conferindo o SHA-256, abrir o instalador e separar as
// notas no que mudou desde a versão instalada.
package atualizacao

import (
	"regexp"
	"strconv"
	"strings"
)

// Versao é o número do Finanças Local: X.Y.Z (publicada) ou X.Y.Z-dev.N (gerada do develop pela CI), como o
// VersaoDoSistema do C#.
type Versao struct {
	Base [3]int
	// Dev é o N da de desenvolvimento; -1 na publicada.
	Dev int
}

var padraoVersao = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-dev\.(\d+))?`)

// LerVersao lê "2.44.1", "v2.44.1" ou "2.44.1-dev.7" (o que vier depois é ignorado, como o "+commit").
func LerVersao(texto string) (Versao, bool) {
	m := padraoVersao.FindStringSubmatch(strings.TrimLeft(strings.TrimSpace(texto), "vV"))
	if m == nil {
		return Versao{}, false
	}
	v := Versao{Dev: -1}
	for i := range 3 {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return Versao{}, false
		}
		v.Base[i] = n
	}
	if m[4] != "" {
		n, err := strconv.Atoi(m[4])
		if err != nil {
			return Versao{}, false
		}
		v.Dev = n
	}
	return v, true
}

// EhDesenvolvimento: X.Y.Z-dev.N.
func (v Versao) EhDesenvolvimento() bool { return v.Dev >= 0 }

// Comparar: pelo X.Y.Z e, no mesmo X.Y.Z, pelo N; a publicada vem depois das de desenvolvimento do mesmo número.
func (v Versao) Comparar(outra Versao) int {
	if c := v.compararBase(outra); c != 0 {
		return c
	}
	return comparar(v.ordemDev(), outra.ordemDev())
}

// MaisNovaNaBase: o X.Y.Z é maior (a dev.N da 2.25.0 já tem a 2.25.0).
func (v Versao) MaisNovaNaBase(outra Versao) bool { return v.compararBase(outra) > 0 }

func (v Versao) compararBase(outra Versao) int {
	for i := range 3 {
		if c := comparar(v.Base[i], outra.Base[i]); c != 0 {
			return c
		}
	}
	return 0
}

func (v Versao) ordemDev() int {
	if v.Dev < 0 {
		return int(^uint(0) >> 1)
	}
	return v.Dev
}

func comparar(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func (v Versao) String() string {
	s := strconv.Itoa(v.Base[0]) + "." + strconv.Itoa(v.Base[1]) + "." + strconv.Itoa(v.Base[2])
	if v.EhDesenvolvimento() {
		s += "-dev." + strconv.Itoa(v.Dev)
	}
	return s
}
