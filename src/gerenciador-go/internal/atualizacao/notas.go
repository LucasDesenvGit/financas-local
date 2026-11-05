package atualizacao

import (
	"regexp"
	"strings"
)

// Notas são as notas de uma versão separadas em "o que mudou desde a sua versão" e "o resto" (#294, Progressive
// Disclosure), como o NotasDaVersao.cs. As notas trazem o CHANGELOG com uma seção por versão, um nível abaixo:
// "### 2.26.1 — 2026-10-21 · desenvolvimento". As novidades são as seções de versão mais nova que a instalada; o resto
// (como instalar, a lista de PRs por versão de desenvolvimento e as versões que você já tem) fica atrás de "Ver tudo".
// Sem seção de versão (notas antigas): os blocos de PRs por versão de desenvolvimento, entre os marcadores, mais novos
// que a instalada; sem eles também, as notas inteiras.
type Notas struct {
	Novidades           string
	Resto               string
	QuantasVersoesNovas int
}

const (
	marcadorInicio = "<!-- por-versao:inicio -->"
	marcadorFim    = "<!-- por-versao:fim -->"
)

var (
	tituloDeVersao = regexp.MustCompile(`^### (\d+\.\d+\.\d+) — `)
	tituloDeBloco  = regexp.MustCompile(`^### (\d+\.\d+\.\d+-dev\.\d+) · `)
)

// TemResto: há o que esconder atrás de "Ver tudo".
func (n Notas) TemResto() bool { return strings.TrimSpace(n.Resto) != "" }

type trecho struct {
	versao      Versao
	inicio, fim int
}

// SepararNotas separa as notas pela versão instalada (nil quando não se sabe).
func SepararNotas(notas string, instalada *Versao) Notas {
	linhas := strings.Split(strings.ReplaceAll(notas, "\r\n", "\n"), "\n")
	titulo := "## O que mudou"
	if instalada != nil {
		titulo = "## O que mudou desde a sua versão (" + instalada.String() + ")"
	}

	// 1) Seções de versão do CHANGELOG mais novas que a instalada (pelo X.Y.Z: a dev.N da 2.25.0 já tem a 2.25.0).
	var novas []trecho
	for _, s := range trechos(linhas, tituloDeVersao, false) {
		if instalada == nil || s.versao.MaisNovaNaBase(*instalada) {
			novas = append(novas, s)
		}
	}
	if instalada == nil {
		novas = novas[:min(len(novas), 1)]
	}
	if len(novas) > 0 {
		return montar(linhas, novas, titulo)
	}

	// 2) Notas antigas, sem seção de versão: os blocos de PRs mais novos que a instalada.
	blocos := trechos(linhas, tituloDeBloco, true)
	var novos []trecho
	for _, b := range blocos {
		if instalada == nil || b.versao.Comparar(*instalada) > 0 {
			novos = append(novos, b)
		}
	}
	if len(novos) == 0 && len(blocos) > 0 {
		novos = blocos[:1]
	}
	if len(novos) > 0 {
		return montar(linhas, novos, titulo)
	}

	// 3) Nada para separar: as notas inteiras.
	return Notas{Novidades: strings.TrimSpace(strings.Join(linhas, "\n"))}
}

// trechos: cada título que casa, até o próximo título de mesmo nível ou acima ("### " ou "## ").
func trechos(linhas []string, padrao *regexp.Regexp, dentroDosMarcadores bool) []trecho {
	var achados []trecho
	dentro := false
	for i, linha := range linhas {
		if strings.Contains(linha, marcadorInicio) {
			dentro = true
			continue
		}
		if strings.Contains(linha, marcadorFim) {
			dentro = false
			continue
		}
		if dentro != dentroDosMarcadores {
			continue
		}
		m := padrao.FindStringSubmatch(linha)
		if m == nil {
			continue
		}
		versao, _ := LerVersao(m[1])
		fim := i + 1
		for fim < len(linhas) && !strings.HasPrefix(linhas[fim], "### ") && !strings.HasPrefix(linhas[fim], "## ") &&
			!strings.Contains(linhas[fim], marcadorFim) {
			fim++
		}
		achados = append(achados, trecho{versao, i, fim})
	}
	return achados
}

// montar: as novidades em cima, com o título; o resto, na ordem original, sem elas.
func montar(linhas []string, novos []trecho, titulo string) Notas {
	usadas := map[int]bool{}
	partes := []string{titulo, ""}
	for _, t := range novos {
		// range sobre o tamanho, e não i++: um i-- trocado pelo teste de mutação encheria o mapa sem parar (#433).
		for i := range t.fim - t.inicio {
			usadas[t.inicio+i] = true
		}
		partes = append(partes, strings.TrimSpace(strings.Join(linhas[t.inicio:t.fim], "\n"))+"\n")
	}
	var resto []string
	for i, l := range linhas {
		if !usadas[i] {
			resto = append(resto, l)
		}
	}
	return Notas{Novidades: strings.TrimSpace(strings.Join(partes, "\n")), Resto: strings.TrimSpace(strings.Join(resto, "\n")),
		QuantasVersoesNovas: len(novos)}
}
