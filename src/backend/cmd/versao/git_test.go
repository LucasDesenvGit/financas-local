package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Os testes deste arquivo usam o git de verdade, num repositório temporário (ADR 0051 e 0070): o formato do merge
// commit do GitHub, o --first-parent e o -G são o contrato do cálculo, e só o git diz se eles se comportam como a regra supõe.

const propsDaBase = `<Project>
  <!-- A base do número: o VersaoBase abaixo muda só na quebra de verdade. -->
  <PropertyGroup>
    <VersaoBase>%s</VersaoBase>
  </PropertyGroup>
</Project>
`

// repoDeTeste é um repositório com a branch develop e um relógio que anda um dia a cada commit.
type repoDeTeste struct {
	t    *testing.T
	raiz string
	dia  int
}

func novoRepo(t *testing.T, base string) *repoDeTeste {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sem o git neste computador")
	}
	g := &repoDeTeste{t: t, raiz: t.TempDir(), dia: 1}
	g.git("init", "-q", "-b", "develop")
	g.escrever(arquivoDaBase, fmt.Sprintf(propsDaBase, base))
	g.git("add", "-A")
	g.git("commit", "-q", "-m", "chore: primeiro commit")
	return g
}

func (g *repoDeTeste) git(argumentos ...string) string {
	g.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "core.autocrlf=false"}, argumentos...)...)
	cmd.Dir = g.raiz
	cmd.Env = g.ambiente()
	var saida, erro bytes.Buffer
	cmd.Stdout, cmd.Stderr = &saida, &erro
	if err := cmd.Run(); err != nil {
		g.t.Fatalf("git %s: %v\n%s", strings.Join(argumentos, " "), err, erro.String())
	}
	return strings.TrimSpace(saida.String())
}

// ambiente isola o git do que há na máquina (configuração global, assinatura de commit) e fixa quem e quando.
func (g *repoDeTeste) ambiente() []string {
	data := fmt.Sprintf("2026-11-%02dT12:00:00+0000", g.dia)
	return append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Teste", "GIT_AUTHOR_EMAIL=teste@exemplo.com", "GIT_COMMITTER_NAME=Teste", "GIT_COMMITTER_EMAIL=teste@exemplo.com",
		"GIT_AUTHOR_DATE="+data, "GIT_COMMITTER_DATE="+data)
}

func (g *repoDeTeste) escrever(arquivo, conteudo string) {
	g.t.Helper()
	caminho := filepath.Join(g.raiz, filepath.FromSlash(arquivo))
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		g.t.Fatal(err)
	}
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		g.t.Fatal(err)
	}
}

// data é a do dia corrente do relógio de teste, como o git a mostra.
func (g *repoDeTeste) data() string { return fmt.Sprintf("2026-11-%02d", g.dia) }

// mesclar faz o que o botão do GitHub faz: um commit na branch do PR (com os arquivos dados, "" apaga) e o merge dela
// no develop, sem fast-forward, com o título do PR no corpo. Devolve a data do merge.
func (g *repoDeTeste) mesclar(branch, titulo string, arquivos map[string]string) string {
	g.t.Helper()
	g.dia++
	g.git("switch", "-q", "-c", branch)
	for arquivo, conteudo := range arquivos {
		if conteudo == "" {
			g.git("rm", "-q", arquivo)
			continue
		}
		g.escrever(arquivo, conteudo)
	}
	if len(arquivos) == 0 {
		g.escrever("marca-"+strings.ReplaceAll(branch, "/", "-")+".txt", branch)
	}
	g.git("add", "-A")
	g.git("commit", "-q", "-m", titulo)
	g.git("switch", "-q", "develop")
	g.dia++
	g.git("merge", "-q", "--no-ff", "-m", "Merge pull request #"+fmt.Sprint(g.dia)+" from dono/"+branch, "-m", titulo, branch)
	return g.data()
}

func (g *repoDeTeste) versao(argumentos ...string) string {
	g.t.Helper()
	var saida bytes.Buffer
	if err := executar(append([]string{"-raiz", g.raiz}, argumentos...), &saida); err != nil {
		g.t.Fatalf("versao %v: %v", argumentos, err)
	}
	return strings.TrimRight(saida.String(), "\n")
}

func (g *repoDeTeste) falha(argumentos ...string) error {
	g.t.Helper()
	var saida bytes.Buffer
	err := executar(append([]string{"-raiz", g.raiz}, argumentos...), &saida)
	if err == nil {
		g.t.Fatalf("versao %v devolveu %q; devia falhar", argumentos, saida.String())
	}
	return err
}

// Regra (ADR 0070): o número parte do <VersaoBase> e sobe a cada merge da linha principal, pelo tipo do título; o merge
// que traz a main de volta não sobe, e mexer só no comentário do Directory.Build.props não recomeça a conta.
func TestGitVersaoSobeMergeAMerge(t *testing.T) {
	g := novoRepo(t, "3.24.0")
	passos := []struct {
		nome, branch, titulo string
		arquivos             map[string]string
		quer                 string
	}{
		{"fix", "fix/a", "fix(api): a", nil, "3.24.1"},
		{"feat sobe o do meio", "feature/b", "feat(web): b", nil, "3.25.0"},
		{"a main de volta", "main", "chore: traz a main de volta para o develop (3.25.0)", nil, "3.25.0"},
		{"uma release de volta", "release/3.25.0", "chore: versão 3.25.0", nil, "3.25.0"},
		{"docs", "docs/c", "docs: c", nil, "3.25.1"},
		{"só o comentário do props: sobe como um chore, sem recomeçar da base", "chore/comentario", "chore: comentário",
			map[string]string{arquivoDaBase: strings.Replace(fmt.Sprintf(propsDaBase, "3.24.0"), "A base do número", "A base da versão", 1)}, "3.25.2"},
		{"quebra de verdade: a base nova vale como está", "feature/quebra", "feat!: quebra",
			map[string]string{arquivoDaBase: fmt.Sprintf(propsDaBase, "4.0.0")}, "4.0.0"},
		{"depois da quebra, a conta é da base nova", "fix/d", "fix: d", nil, "4.0.1"},
	}
	for _, p := range passos {
		g.mesclar(p.branch, p.titulo, p.arquivos)
		if obtido := g.versao(); obtido != p.quer {
			t.Errorf("%s: obtido %s, quer %s", p.nome, obtido, p.quer)
		}
	}
	// Um commit antigo continua com o número que tinha.
	if obtido := g.versao("-ref", "HEAD~3"); obtido != "3.25.1" {
		t.Errorf("HEAD~3: obtido %s, quer 3.25.1", obtido)
	}
	if obtido := g.versao("-ref", "HEAD~8"); obtido != "3.24.0" {
		t.Errorf("HEAD~8 (o primeiro commit): obtido %s, quer 3.24.0", obtido)
	}
}

// Regra: o mesmo commit dá o mesmo número em qualquer clone (é o que torna o cálculo no build confiável).
func TestGitMesmoCommitMesmoNumero(t *testing.T) {
	g := novoRepo(t, "3.24.0")
	g.mesclar("fix/a", "fix: a", nil)
	g.mesclar("feature/b", "feat: b", nil)
	copia := t.TempDir()
	g.git("clone", "-q", g.raiz, copia)
	var saida bytes.Buffer
	if err := executar([]string{"-raiz", copia}, &saida); err != nil {
		t.Fatal(err)
	}
	if obtido := strings.TrimSpace(saida.String()); obtido != g.versao() || obtido != "3.25.0" {
		t.Errorf("a cópia deu %s e o original %s; quer 3.25.0 nos dois", obtido, g.versao())
	}
}

// Regra: sem o histórico (cópia baixada em .zip, mesmo dentro de outro repositório, ou clone raso), o número é a base
// com o sufixo -sem-git, e as seções do CHANGELOG, que dependem da história, dão erro.
func TestGitSemHistorico(t *testing.T) {
	t.Run("sem .git", func(t *testing.T) {
		pasta := t.TempDir()
		if err := os.WriteFile(filepath.Join(pasta, arquivoDaBase), []byte(fmt.Sprintf(propsDaBase, "3.24.0")), 0o644); err != nil {
			t.Fatal(err)
		}
		var saida bytes.Buffer
		if err := executar([]string{"-raiz", pasta}, &saida); err != nil || strings.TrimSpace(saida.String()) != "3.24.0-sem-git" {
			t.Errorf("obtido %q, %v; quer 3.24.0-sem-git", saida.String(), err)
		}
		if err := executar([]string{"-raiz", pasta, "-changelog"}, &saida); err == nil {
			t.Error("-changelog sem git devia falhar")
		}
	})
	t.Run("copia dentro de outro repositório", func(t *testing.T) {
		g := novoRepo(t, "9.9.9")
		g.mesclar("feature/x", "feat: x", nil)
		copia := filepath.Join(g.raiz, "baixado")
		if err := os.MkdirAll(copia, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(copia, arquivoDaBase), []byte(fmt.Sprintf(propsDaBase, "3.24.0")), 0o644); err != nil {
			t.Fatal(err)
		}
		var saida bytes.Buffer
		if err := executar([]string{"-raiz", copia}, &saida); err != nil || strings.TrimSpace(saida.String()) != "3.24.0-sem-git" {
			t.Errorf("obtido %q, %v; quer 3.24.0-sem-git (o .git de fora não vale)", saida.String(), err)
		}
	})
	t.Run("clone raso", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("o file:// do clone raso muda de forma no Windows; o raso é o mesmo caso do sem .git")
		}
		g := novoRepo(t, "3.24.0")
		g.mesclar("feature/x", "feat: x", nil)
		g.mesclar("fix/y", "fix: y", nil)
		raso := t.TempDir()
		g.git("clone", "-q", "--depth", "1", "file://"+filepath.ToSlash(g.raiz), raso)
		var saida bytes.Buffer
		if err := executar([]string{"-raiz", raso}, &saida); err != nil || strings.TrimSpace(saida.String()) != "3.24.0-sem-git" {
			t.Errorf("obtido %q, %v; quer 3.24.0-sem-git (clone raso não tem os merges)", saida.String(), err)
		}
	})
}

// Regra: o que o cálculo não entende é erro, e não um número chutado.
func TestGitErros(t *testing.T) {
	g := novoRepo(t, "3.24.0")
	g.mesclar("fix/a", "fix: a", nil)
	g.falha("-ref", "nao-existe")
	g.falha("-publicar", "3.25.0")                        // sem a data
	g.falha("-publicar", "3.25", "-data", "2026-11-20")   // versão fora do formato
	g.falha("-publicar", "3.25.0", "-data", "20/11/2026") // data fora do formato
	g.falha("-opcao-que-nao-existe")
	if _, err := acharRaiz(t.TempDir()); err == nil {
		t.Error("achou a raiz numa pasta sem Directory.Build.props")
	}

	// O último commit que mexeu na base trocou o <VersaoBase> pelo <Version> antigo: sem base, é erro.
	antigo := novoRepo(t, "3.24.0")
	antigo.escrever(arquivoDaBase, "<Project><PropertyGroup><Version>3.16.0</Version></PropertyGroup></Project>")
	antigo.git("commit", "-q", "-a", "-m", "volta a propriedade antiga")
	antigo.falha()
	semBase := t.TempDir()
	if err := os.WriteFile(filepath.Join(semBase, arquivoDaBase), []byte("<Project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := executar([]string{"-raiz", semBase}, new(bytes.Buffer)); err == nil {
		t.Error("props sem a base devia falhar, mesmo sem git")
	}
}

// Regra (passo 3 da 0070): na tag, o número conferido é o do commit da release, que entrou pelo merge na main; sem o
// -publicada, a main não é a linha do develop e o número não vale.
func TestGitPublicada(t *testing.T) {
	g := novoRepo(t, "3.24.0")
	g.git("branch", "main")
	g.mesclar("feature/a", "feat: a", nil)
	g.mesclar("fix/b", "fix: b", nil)
	if g.versao() != "3.25.1" {
		t.Fatalf("develop: %s", g.versao())
	}
	g.git("switch", "-q", "-c", "release/3.25.1")
	g.dia++
	g.escrever("marca-da-release.txt", "x")
	g.git("add", "-A")
	g.git("commit", "-q", "-m", "chore: versão 3.25.1")
	g.git("switch", "-q", "main")
	g.dia++
	g.git("merge", "-q", "--no-ff", "-m", "Merge pull request #99 from dono/release/3.25.1", "-m", "chore: versão 3.25.1", "release/3.25.1")

	if obtido := g.versao("-publicada"); obtido != "3.25.1" {
		t.Errorf("-publicada na main: obtido %s, quer 3.25.1", obtido)
	}
	if obtido := g.versao(); obtido == "3.25.1" {
		t.Errorf("sem -publicada, a main não devia dar o número do develop (deu %s)", obtido)
	}
	// Commit que não é merge (uma tag direta): vale ele mesmo.
	g.git("switch", "-q", "develop")
	g.git("commit", "-q", "--allow-empty", "-m", "chore: direto no develop")
	if obtido := g.versao("-publicada"); obtido != "3.25.1" {
		t.Errorf("-publicada num commit sem segundo pai: obtido %s, quer 3.25.1", obtido)
	}
}

const (
	entradaA = "### Funcionalidades\n- **A** nova (#1).\n  - Técnico: coluna nova.\n"
	entradaB = "### Correções\n- **B** corrigido (#2).\n"
	entradaC = "### Correções\n- **C** também (#3).\n\n### Publicação e documentação\n- **C** documentado (#3).\n"
	entradaD = "### Publicação e documentação\n- **D** do primeiro commit.\n"
)

// Regra (passo 1 e 4 da 0070): -changelog junta o que está em changelog.d/ com o número do merge que trouxe cada arquivo,
// uma seção por versão, a mais nova primeiro, no formato que o notas.sh e o gerenciador leem; o LEIA-ME.md e o que
// já foi apagado ficam de fora, e o que veio antes da base entra na seção da base.
func TestGitChangelogPorVersao(t *testing.T) {
	g := novoRepo(t, "3.24.0")
	g.escrever("changelog.d/LEIA-ME.md", "como escrever uma entrada")
	g.escrever("changelog.d/d.md", entradaD)
	g.git("add", "-A")
	g.git("commit", "-q", "-m", "docs: entradas do começo")
	dataBase := g.data()
	dataA := g.mesclar("feature/a", "feat(web): a", map[string]string{"changelog.d/feature-a.md": entradaA})
	g.mesclar("fix/sem-entrada", "ci: sem arquivo", nil)
	dataB := g.mesclar("fix/b", "fix: b", map[string]string{"changelog.d/fix-b.md": entradaB})
	g.mesclar("main", "chore: traz a main de volta", nil)
	g.mesclar("fix/apagada", "fix: apagada", map[string]string{"changelog.d/fix-apagada.md": entradaB})
	g.mesclar("chore/apaga", "chore: apaga", map[string]string{"changelog.d/fix-apagada.md": ""})
	dataC := g.mesclar("fix/c", "fix: c", map[string]string{"changelog.d/fix-c.md": entradaC, "changelog.d/anotacao.txt": "não é .md"})

	quer := fmt.Sprintf(`## [3.25.5] — %s · desenvolvimento

### Correções
- **C** também (#3).

### Publicação e documentação
- **C** documentado (#3).

## [3.25.2] — %s · desenvolvimento

### Correções
- **B** corrigido (#2).

## [3.25.0] — %s · desenvolvimento

### Funcionalidades
- **A** nova (#1).
  - Técnico: coluna nova.

## [3.24.0] — %s · desenvolvimento

### Publicação e documentação
- **D** do primeiro commit.
`, dataC, dataB, dataA, dataBase)
	if obtido := g.versao("-changelog"); obtido != strings.TrimRight(quer, "\n") {
		t.Errorf("-changelog:\n%s\nquer:\n%s", obtido, quer)
	}
	if obtido := g.versao(); obtido != "3.25.5" {
		t.Errorf("versão: %s, quer 3.25.5", obtido)
	}
}

// Regra (passo 5 da 0070): -publicar junta tudo o que está em changelog.d/ numa seção só, por tipo, sem o "· desenvolvimento",
// no topo do CHANGELOG.md (antes da última versão publicada), e tira os arquivos do git, sem apagar o LEIA-ME.md.
func TestGitPublicar(t *testing.T) {
	g := novoRepo(t, "3.24.0")
	g.escrever("changelog.d/LEIA-ME.md", "x")
	g.escrever("CHANGELOG.md", "# Changelog\n\nFormato.\n\n## [3.3.14] — 2026-11-05\n\n### Correções\n- antiga\n")
	g.git("add", "-A")
	g.git("commit", "-q", "-m", "docs: leia-me")
	g.mesclar("feature/a", "feat: a", map[string]string{"changelog.d/feature-a.md": entradaA})
	g.mesclar("fix/b", "fix: b", map[string]string{"changelog.d/fix-b.md": entradaB})
	g.mesclar("fix/c", "fix: c", map[string]string{"changelog.d/fix-c.md": entradaC})
	secao := `## [3.26.0] — 2026-11-20

### Funcionalidades
- **A** nova (#1).
  - Técnico: coluna nova.

### Correções
- **C** também (#3).
- **B** corrigido (#2).

### Publicação e documentação
- **C** documentado (#3).`
	if obtido := g.versao("-publicar", "3.26.0", "-data", "2026-11-20"); obtido != secao {
		t.Errorf("-publicar:\n%s\nquer:\n%s", obtido, secao)
	}
	changelog, err := os.ReadFile(filepath.Join(g.raiz, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	quer := "# Changelog\n\nFormato.\n\n" + secao + "\n\n## [3.3.14] — 2026-11-05\n\n### Correções\n- antiga\n"
	if string(changelog) != quer {
		t.Errorf("CHANGELOG.md:\n%s\nquer:\n%s", changelog, quer)
	}
	// Tudo no índice, para o commit da publicação: o CHANGELOG.md mudado, as três entradas apagadas, o LEIA-ME.md de pé.
	if obtido := g.git("status", "--short"); obtido != "M  CHANGELOG.md\nD  changelog.d/feature-a.md\nD  changelog.d/fix-b.md\nD  changelog.d/fix-c.md" {
		t.Errorf("git status:\n%s", obtido)
	}
	// Depois do commit, nada resta para publicar.
	g.git("commit", "-q", "-m", "chore: versão 3.26.0")
	if obtido := g.versao("-changelog"); obtido != "" {
		t.Errorf("-changelog depois de publicar: %q", obtido)
	}
	g.falha("-publicar", "3.26.1", "-data", "2026-11-21") // sem entradas, não publica
}

// publicarNaMain faz a publicação inteira de uma versão: a release/X.Y.Z com o "chore: versão" (que junta e apaga as
// entradas), o merge dela na main e a tag. Devolve o nome da tag.
func (g *repoDeTeste) publicarNaMain(versao string) string {
	g.t.Helper()
	g.git("switch", "-q", "-c", "release/"+versao)
	g.dia++
	g.versao("-publicar", versao, "-data", g.data())
	g.git("commit", "-q", "-m", "chore: versão "+versao)
	g.git("switch", "-q", "main")
	g.dia++
	g.git("merge", "-q", "--no-ff", "-m", "Merge pull request #9"+fmt.Sprint(g.dia)+" from dono/release/"+versao, "-m", "chore: versão "+versao, "release/"+versao)
	g.git("tag", "v"+versao)
	g.git("switch", "-q", "develop")
	return "v" + versao
}

// Regra (#757, ADR 0070): as notas da versão publicada listam pelo título os PRs que não trouxeram arquivo para
// changelog.d/. O -sem-changelog, na tag (-publicada), dá os PRs desde a tag anterior: fica de fora o que trouxe entrada
// (já está no CHANGELOG), a volta da main e o commit direto; o squash não repete o número.
func TestGitSemChangelog(t *testing.T) {
	g := novoRepo(t, "3.24.0")
	g.escrever("changelog.d/LEIA-ME.md", "x")
	g.escrever("CHANGELOG.md", "# Changelog\n\n## [3.3.14] — 2026-11-05\n\n### Correções\n- antiga\n")
	g.git("add", "-A")
	g.git("commit", "-q", "-m", "docs: leia-me")
	g.git("branch", "main")
	g.mesclar("feature/a", "feat: a", map[string]string{"changelog.d/feature-a.md": entradaA})
	g.mesclar("ci/x", "ci: x sem arquivo", nil)
	g.mesclar("fix/leia-me", "docs: só o LEIA-ME", map[string]string{"changelog.d/LEIA-ME.md": "y"})
	primeira := g.publicarNaMain("3.25.2")

	g.dia++
	g.git("merge", "-q", "--no-ff", "-m", "Merge pull request #50 from dono/main", "-m", "chore: traz a main de volta", "main")
	g.mesclar("fix/c", "fix: c", map[string]string{"changelog.d/fix-c.md": entradaC})
	// Antes da ADR 0070, o PR escrevia direto no CHANGELOG.md: a nota dele já está lá.
	antes, err := os.ReadFile(filepath.Join(g.raiz, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	g.mesclar("fix/antiga", "fix: do jeito antigo", map[string]string{"CHANGELOG.md": strings.Replace(string(antes),
		"## [", "## [3.25.3] — 2026-11-10 · desenvolvimento\n\n### Correções\n- antiga (#4).\n\n## [", 1)})
	g.mesclar("dependabot/npm/x", "build(deps): sobe x de 1.0 para 1.1", nil)
	g.dia++
	g.escrever("squash.txt", "s")
	g.git("add", "-A")
	g.git("commit", "-q", "-m", "test: squash (#77)")
	g.git("commit", "-q", "--allow-empty", "-m", "chore: direto no develop")
	segunda := g.publicarNaMain("3.25.6")

	n := func(titulo string) string { // o número que o mesclar deu ao merge do PR
		assunto := strings.Fields(g.git("log", "--first-parent", "develop", "--format=%s", "--grep", titulo, "-n1"))
		return strings.TrimPrefix(assunto[3], "#") // "Merge pull request #N from ..."
	}
	quer := "- test: squash (#77)\n- build(deps): sobe x de 1.0 para 1.1 (#" + n("build(deps)") + ")"
	if obtido := g.versao("-ref", segunda, "-publicada", "-sem-changelog", "-desde", primeira); obtido != quer {
		t.Errorf("-sem-changelog da segunda versão:\n%s\nquer:\n%s", obtido, quer)
	}
	quer = "- docs: só o LEIA-ME (#" + n("LEIA-ME") + ")\n- ci: x sem arquivo (#" + n("ci: x") + ")"
	if obtido := g.versao("-ref", primeira, "-publicada", "-sem-changelog"); obtido != quer {
		t.Errorf("-sem-changelog da primeira versão, desde a base:\n%s\nquer:\n%s", obtido, quer)
	}
	if err := g.falha("-ref", segunda, "-publicada", "-sem-changelog", "-desde", "v9.9.9"); !strings.Contains(err.Error(), "-desde") {
		t.Errorf("-desde inexistente: %v", err)
	}
}

// Regra: a seção entra antes da primeira versão do CHANGELOG, e no fim se ele ainda não tem nenhuma.
func TestInserirSecao(t *testing.T) {
	casos := []struct{ nome, changelog, quer string }{
		{"antes da primeira versão", "# C\n\n## [2.0.0] — x\nvelha\n", "# C\n\nNOVA\n## [2.0.0] — x\nvelha\n"},
		{"o arquivo começa pela versão", "## [2.0.0] — x\n", "NOVA\n## [2.0.0] — x\n"},
		{"sem versão nenhuma", "# C\n\nsó o formato\n", "# C\n\nsó o formato\n\nNOVA\n"},
	}
	for _, c := range casos {
		if obtido := inserirSecao(c.changelog, "NOVA\n"); obtido != c.quer {
			t.Errorf("%s: obtido %q, quer %q", c.nome, obtido, c.quer)
		}
	}
}
