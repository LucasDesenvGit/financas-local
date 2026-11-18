package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// copiar leva um arquivo de dentro do repositório para o repositório de teste, no mesmo caminho relativo.
func (g *repoDeTeste) copiar(origem, destino string) {
	g.t.Helper()
	dados, err := os.ReadFile(origem)
	if err != nil {
		g.t.Fatal(err)
	}
	g.escrever(destino, string(dados))
}

// lancar roda o scripts/dev/lancar-versao.sh no repositório de teste e devolve a saída e o erro.
func (g *repoDeTeste) lancar(argumentos ...string) (string, error) {
	g.t.Helper()
	cmd := exec.Command("bash", append([]string{"scripts/dev/lancar-versao.sh"}, argumentos...)...)
	cmd.Dir = g.raiz
	cmd.Env = append(g.ambiente(), "GOTOOLCHAIN=local")
	var saida bytes.Buffer
	cmd.Stdout, cmd.Stderr = &saida, &saida
	err := cmd.Run()
	return saida.String(), err
}

// Regra (passo 5 da 0070): o scripts/dev/lancar-versao.sh só roda na release/X.Y.Z, com a árvore limpa e o X.Y.Z igual
// ao número calculado no commit; então junta changelog.d/ no CHANGELOG.md e faz o commit "chore: versão X.Y.Z".
func TestLancarVersao(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("o script é em bash; no Windows o bash pode ser o do WSL")
	}
	for _, programa := range []string{"bash", "go"} {
		if _, err := exec.LookPath(programa); err != nil {
			t.Skipf("sem o %s neste computador", programa)
		}
	}
	g := novoRepo(t, "3.24.0")
	// O repositório de teste leva o script e o cálculo, como o projeto, num módulo Go mínimo (só a biblioteca padrão).
	gomod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	diretivaGo := regexp.MustCompile(`(?m)^go \S+$`).FindString(string(gomod))
	g.escrever("src/backend/go.mod", "module versaodeteste\n\n"+diretivaGo+"\n")
	g.copiar(filepath.Join("..", "..", "..", "..", "scripts", "dev", "lancar-versao.sh"), "scripts/dev/lancar-versao.sh")
	fontes, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fontes {
		if !strings.HasSuffix(f, "_test.go") {
			g.copiar(f, "src/backend/cmd/versao/"+f)
		}
	}
	g.escrever("CHANGELOG.md", "# Changelog\n\n## [3.3.14] — 2026-11-05\n\n### Correções\n- antiga\n")
	g.escrever("changelog.d/LEIA-ME.md", "como escrever")
	g.git("add", "-A")
	g.git("commit", "-q", "-m", "chore: versão e script de teste")
	g.mesclar("feature/a", "feat: a", map[string]string{"changelog.d/feature-a.md": entradaA})
	g.mesclar("fix/b", "fix: b", map[string]string{"changelog.d/fix-b.md": entradaB})

	// Fora da release/X.Y.Z, e com a versão errada, o script recusa sem mexer em nada.
	if saida, err := g.lancar("3.25.1"); err == nil || !strings.Contains(saida, "rode na release/3.25.1") {
		t.Errorf("no develop: %v\n%s", err, saida)
	}
	if saida, err := g.lancar("abc"); err == nil || !strings.Contains(saida, "AAAA-MM-DD") {
		t.Errorf("versão fora do formato: %v\n%s", err, saida)
	}
	g.git("switch", "-q", "-c", "release/3.25.9")
	if saida, err := g.lancar("3.25.9"); err == nil || !strings.Contains(saida, "calculado neste commit e 3.25.1, nao 3.25.9") {
		t.Errorf("número diferente do calculado: %v\n%s", err, saida)
	}
	g.git("switch", "-q", "-c", "release/3.25.1")
	g.escrever("sujeira.txt", "x")
	if saida, err := g.lancar("3.25.1"); err == nil || !strings.Contains(saida, "mudancas nao commitadas") {
		t.Errorf("árvore suja: %v\n%s", err, saida)
	}
	if err := os.Remove(filepath.Join(g.raiz, "sujeira.txt")); err != nil {
		t.Fatal(err)
	}

	saida, err := g.lancar("3.25.1", "--data", "2026-11-20")
	if err != nil {
		t.Fatalf("lancar-versao.sh: %v\n%s", err, saida)
	}
	if assunto := g.git("log", "-1", "--format=%s"); assunto != "chore: versão 3.25.1" {
		t.Errorf("commit: %q", assunto)
	}
	if sujo := g.git("status", "--short"); sujo != "" {
		t.Errorf("a árvore devia estar limpa depois do commit: %q", sujo)
	}
	changelog, err := os.ReadFile(filepath.Join(g.raiz, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(changelog), "# Changelog\n\n## [3.25.1] — 2026-11-20\n\n### Funcionalidades\n- **A** nova (#1).") ||
		!strings.Contains(string(changelog), "### Correções\n- **B** corrigido (#2).\n\n## [3.3.14] — 2026-11-05") {
		t.Errorf("CHANGELOG.md:\n%s", changelog)
	}
	if restam := g.git("ls-files", "changelog.d"); restam != "changelog.d/LEIA-ME.md" {
		t.Errorf("changelog.d/ depois: %q", restam)
	}
	// O commit da publicação não sobe o número: a tag vX.Y.Z do commit da release é a do develop.
	if obtido := g.versao(); obtido != "3.25.1" {
		t.Errorf("versão depois do commit da publicação: %s, quer 3.25.1", obtido)
	}
}
