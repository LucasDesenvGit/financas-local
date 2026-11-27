package main

import (
	"regexp"
	"strings"
)

// commit é um commit da linha principal (o primeiro pai), do que a regra precisa: quantos pais tem, a primeira linha
// da mensagem e o resto dela.
type commit struct {
	hash    string
	pais    int
	assunto string
	corpo   string
	data    string // AAAA-MM-DD, a do commit
}

var (
	// "Merge pull request #12 from dono/feature/x": o merge do botão do GitHub, com o título do PR no corpo.
	mergeDePR = regexp.MustCompile(`^Merge pull request #(\d+) from `)
	// Os merges que só trazem a main ou uma release/* de volta (ADR 0070, regra 3): o conteúdo já foi contado. A
	// origem é o fim do assunto (o "dependabot/.../main/x" ou o "feature/release-notes" não são).
	voltaDaMain = regexp.MustCompile(`^Merge pull request #\d+ from [^/\s]+/(?:main|release/\S+)$`)
	voltaLocal  = regexp.MustCompile(`^Merge (?:remote-tracking )?branch '(?:origin/)?(?:main|release/[^']+)'`)
	// "titulo (#12)": o assunto de um squash do GitHub. Commit direto, sem o número do PR, não é um PR.
	assuntoDeSquash = regexp.MustCompile(`\(#(\d+)\)\s*$`)
	// feat, feat(escopo), feat(escopo)!: o único tipo que sobe o número do meio.
	tipoFuncionalidade = regexp.MustCompile(`^\s*feat(?:\([^)]*\))?!?\s*:`)
)

// contaNaSubida diz se o commit sobe a versão: todo merge da linha principal, menos os que trazem a main ou uma
// release/* de volta; e, se o repositório passar a fazer squash, o commit cujo assunto termina em "(#N)". O resto
// (commit direto, o "chore: versão X.Y.Z" da publicação) não sobe.
func (c commit) contaNaSubida() bool {
	if c.pais >= 2 {
		return !voltaDaMain.MatchString(c.assunto) && !voltaLocal.MatchString(c.assunto)
	}
	return assuntoDeSquash.MatchString(c.assunto)
}

// titulo é o título do PR: no merge do GitHub, a primeira linha do corpo; no squash ou em outro merge, o assunto.
func (c commit) titulo() string {
	if c.pais >= 2 && mergeDePR.MatchString(c.assunto) {
		for linha := range strings.SplitSeq(c.corpo, "\n") {
			if l := strings.TrimSpace(linha); l != "" {
				return l
			}
		}
	}
	return c.assunto
}

// numeroDoPR é o número do PR, do assunto do merge do GitHub ou do squash; "" num commit direto.
func (c commit) numeroDoPR() string {
	if m := mergeDePR.FindStringSubmatch(c.assunto); c.pais >= 2 && m != nil {
		return m[1]
	}
	if m := assuntoDeSquash.FindStringSubmatch(c.assunto); m != nil {
		return m[1]
	}
	return ""
}

// funcionalidade: o título é de feat.
func (c commit) funcionalidade() bool { return tipoFuncionalidade.MatchString(c.titulo()) }

// versoesDosCommits é a versão depois de cada commit, na ordem dada (do mais antigo ao mais novo), partindo da base.
// A versão de um commit que não sobe é a do anterior.
func versoesDosCommits(base numero, commits []commit) []numero {
	versoes := make([]numero, len(commits))
	atual := base
	for i, c := range commits {
		if c.contaNaSubida() {
			atual = atual.subir(c.funcionalidade())
		}
		versoes[i] = atual
	}
	return versoes
}

// calcular é a versão depois de todos os commits (a base, se não houver nenhum).
func calcular(base numero, commits []commit) numero {
	versoes := versoesDosCommits(base, commits)
	if len(versoes) == 0 {
		return base
	}
	return versoes[len(versoes)-1]
}
