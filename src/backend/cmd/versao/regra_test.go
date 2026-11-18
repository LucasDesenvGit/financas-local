package main

import "testing"

// pr é o merge do botão do GitHub: o assunto traz o número e a branch, e o corpo, o título do PR.
func pr(branch, titulo string) commit {
	return commit{pais: 2, assunto: "Merge pull request #1 from dono/" + branch, corpo: titulo}
}

func squash(assunto string) commit { return commit{pais: 1, assunto: assunto} }

// Regra (ADR 0070, regras 2 e 3): cada merge da linha principal sobe a versão pelo tipo do título do PR, com a tabela
// da 0039; a main e as release/* de volta não sobem.
func TestCalcular(t *testing.T) {
	base := numero{3, 24, 0}
	casos := []struct {
		nome    string
		commits []commit
		quer    string
	}{
		{"sem merge, a própria base", nil, "3.24.0"},
		{"fix sobe o último", []commit{pr("fix/a", "fix(api): corrige")}, "3.24.1"},
		{"feat sobe o do meio e zera o último", []commit{pr("fix/a", "fix: a"), pr("fix/b", "fix: b"), pr("feature/c", "feat(web): c")}, "3.25.0"},
		{"feat e depois fix", []commit{pr("feature/c", "feat: c"), pr("fix/a", "fix: a")}, "3.25.1"},
		{"feat(banco)! só acrescenta: sobe o do meio, como a 0039", []commit{pr("feature/b", "feat(banco)!: tabela nova")}, "3.25.0"},
		{"feat com escopo composto", []commit{pr("feature/b", "feat(open-finance): x")}, "3.25.0"},
		{"cada tipo que não é feat sobe o último", []commit{
			pr("a", "docs: a"), pr("b", "build(deps): b"), pr("c", "ci: c"), pr("d", "refactor(web): d"), pr("e", "perf: e"),
			pr("f", "test: f"), pr("g", "chore: g"), pr("h", "style: h"),
		}, "3.24.8"},
		{"Dependabot", []commit{pr("dependabot/npm_and_yarn/src/frontend/develop/x-1.0", "chore(deps): bump x")}, "3.24.1"},
		{"título fora do padrão sobe o último", []commit{pr("a", "Atualiza uma coisa")}, "3.24.1"},
		{"feature não é feat", []commit{pr("a", "feature: x")}, "3.24.1"},
		{"feat no meio do título não vale", []commit{pr("a", "fix: corrige o feat: da tela")}, "3.24.1"},
		{"espaços antes do tipo", []commit{pr("a", "  feat: x")}, "3.25.0"},
		{"o título é a primeira linha do corpo, mesmo com linha em branco antes", []commit{
			{pais: 2, assunto: "Merge pull request #7 from dono/a", corpo: "\n\nfeat: x\n\nmais texto"},
		}, "3.25.0"},
		{"a main de volta não sobe", []commit{pr("fix/a", "fix: a"), pr("main", "chore: traz a main de volta para o develop (3.24.1)")}, "3.24.1"},
		{"uma release/* de volta não sobe", []commit{pr("release/3.24.1", "chore: versão 3.24.1"), pr("fix/a", "fix: a")}, "3.24.1"},
		{"branch cujo nome só termina em main sobe", []commit{pr("feature/menu-main", "fix: a")}, "3.24.1"},
		{"branch com release no meio do caminho sobe", []commit{pr("feature/release-notes", "fix: a"), pr("dependabot/x/main/y", "fix: b")}, "3.24.2"},
		{"merge local sem corpo sobe o último", []commit{{pais: 2, assunto: "Merge branch 'feature/x' into develop"}}, "3.24.1"},
		{"merge local da main não sobe", []commit{{pais: 2, assunto: "Merge branch 'main' into develop"}}, "3.24.0"},
		{"merge local da origin/main não sobe", []commit{{pais: 2, assunto: "Merge remote-tracking branch 'origin/main' into develop"}}, "3.24.0"},
		{"merge local de release não sobe", []commit{{pais: 2, assunto: "Merge branch 'release/3.24.1'"}}, "3.24.0"},
		{"squash do GitHub, no assunto", []commit{squash("feat(web): tela nova (#12)"), squash("fix: a (#13)")}, "3.25.1"},
		{"commit direto, sem número de PR, não sobe", []commit{squash("fix: direto no develop"), squash("chore: versão 3.24.0")}, "3.24.0"},
		{"o commit da publicação, na release, não sobe", []commit{pr("fix/a", "fix: a"), squash("chore: versão 3.24.1")}, "3.24.1"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if obtido := calcular(base, c.commits).String(); obtido != c.quer {
				t.Errorf("obtido %s, quer %s", obtido, c.quer)
			}
		})
	}
}

// Regra: a base nova (a quebra de verdade) recomeça a conta dela, e não da anterior.
func TestCalcularRecomecaDaBase(t *testing.T) {
	commits := []commit{pr("fix/a", "fix: a"), pr("feature/b", "feat: b")}
	if obtido := calcular(numero{4, 0, 0}, commits); obtido.String() != "4.1.0" {
		t.Errorf("obtido %s, quer 4.1.0", obtido)
	}
}

// Regra: a versão depois de cada commit; o que não sobe repete a anterior.
func TestVersoesDosCommits(t *testing.T) {
	commits := []commit{pr("a", "fix: a"), pr("main", "chore: volta"), pr("b", "feat: b"), pr("c", "fix: c")}
	var obtido []string
	for _, v := range versoesDosCommits(numero{1, 0, 0}, commits) {
		obtido = append(obtido, v.String())
	}
	quer := []string{"1.0.1", "1.0.1", "1.1.0", "1.1.1"}
	if len(obtido) != len(quer) {
		t.Fatalf("obtido %v, quer %v", obtido, quer)
	}
	for i := range quer {
		if obtido[i] != quer[i] {
			t.Errorf("obtido %v, quer %v", obtido, quer)
		}
	}
}

// Regra: a base é o <VersaoBase> do Directory.Build.props; sem ele (ou fora do X.Y.Z), é erro, e não uma base qualquer.
func TestLerBase(t *testing.T) {
	casos := []struct {
		nome, props, quer string
		erro              bool
	}{
		{"com espaços", "<Project><PropertyGroup>\n  <VersaoBase> 3.24.0 </VersaoBase>\n</PropertyGroup></Project>", "3.24.0", false},
		{"a propriedade antiga não vale", "<Version>3.16.0</Version>", "", true},
		{"sem a propriedade", "<Project/>", "", true},
		{"vazia", "<VersaoBase></VersaoBase>", "", true},
		{"formato errado", "<VersaoBase>3.24</VersaoBase>", "", true},
		{"com sufixo", "<VersaoBase>3.24.0-dev</VersaoBase>", "", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			n, err := lerBase(c.props)
			if (err != nil) != c.erro || (!c.erro && n.String() != c.quer) {
				t.Errorf("obtido %v, %v; quer %q (erro %v)", n, err, c.quer, c.erro)
			}
		})
	}
}
