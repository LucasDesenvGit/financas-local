package atualizacao

import (
	"regexp"
	"strings"
)

// secaoDoChangelog é o título de cada versão no CHANGELOG.md: "## [2.45.0] — 2026-10-28 · desenvolvimento" (#294).
var secaoDoChangelog = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\]`)

// SecoesDoChangelog são as seções do CHANGELOG.md mais novas que a versão de antes, até a nova (pelo X.Y.Z), na
// ordem do arquivo: o "O que mudou" depois de atualizar pelo develop (#432). Sem a de antes, só a da versão nova.
func SecoesDoChangelog(changelog string, antes *Versao, nova Versao) string {
	var escolhidas []string
	var atual []string
	dentro := false
	fechar := func() {
		if dentro {
			escolhidas = append(escolhidas, strings.TrimSpace(strings.Join(atual, "\n")))
		}
		atual, dentro = nil, false
	}
	for linha := range strings.SplitSeq(strings.ReplaceAll(changelog, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(linha, "## ") {
			fechar()
			if m := secaoDoChangelog.FindStringSubmatch(linha); m != nil {
				v, _ := LerVersao(m[1])
				dentro = v.compararBase(nova) <= 0 && (antes == nil && v.compararBase(nova) == 0 || antes != nil && v.MaisNovaNaBase(*antes))
			}
		}
		if dentro {
			atual = append(atual, linha)
		}
	}
	fechar()
	return strings.Join(escolhidas, "\n\n")
}

// versaoDoProps é a <Version> do Directory.Build.props.
var versaoDoProps = regexp.MustCompile(`<Version>\s*([^<\s]+)\s*</Version>`)

// VersaoDoProjeto lê a versão do Directory.Build.props (a única fonte da versão, CLAUDE.md).
func VersaoDoProjeto(props string) (Versao, bool) {
	m := versaoDoProps.FindStringSubmatch(props)
	if m == nil {
		return Versao{}, false
	}
	return LerVersao(m[1])
}
