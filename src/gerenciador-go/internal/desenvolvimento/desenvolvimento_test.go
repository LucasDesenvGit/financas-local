package desenvolvimento

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/gerenciador-go/internal/instalacao"
)

// clone monta uma pasta de projeto como a da instalação pelos scripts: publicado/api e o script de atualizar.
func clone(t *testing.T, comGit bool) (raiz string, inst instalacao.Instalacao) {
	t.Helper()
	raiz = t.TempDir()
	api := filepath.Join(raiz, "publicado", "api")
	for _, p := range []string{api, filepath.Join(raiz, "scripts", "geral")} {
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(raiz, "scripts", "geral", "atualizar.sh"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if comGit {
		if err := os.Mkdir(filepath.Join(raiz, ".git"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return raiz, instalacao.Instalacao{PastaPrograma: api}
}

// Regra: a raiz é a pasta dois níveis acima da API, só pelos scripts, com o .git (pasta ou arquivo, num worktree) e o
// script de atualizar.
func TestRaiz(t *testing.T) {
	t.Parallel()
	raiz, inst := clone(t, true)
	if obtida := Raiz(inst); obtida != raiz {
		t.Errorf("clone: %q", obtida)
	}
	inst.PeloMsi = true
	if Raiz(inst) != "" {
		t.Error("pelo .msi")
	}
	if Raiz(instalacao.Instalacao{}) != "" {
		t.Error("sem pasta")
	}
	_, semGit := clone(t, false)
	if Raiz(semGit) != "" {
		t.Error("sem .git")
	}
	worktree, inst2 := clone(t, false)
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if Raiz(inst2) != worktree {
		t.Error(".git como arquivo")
	}
	semScript, inst3 := clone(t, true)
	if err := os.Remove(filepath.Join(semScript, "scripts", "geral", "atualizar.sh")); err != nil {
		t.Fatal(err)
	}
	if Raiz(inst3) != "" {
		t.Error("sem o script")
	}
}

func git(t *testing.T, raiz string, argumentos ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", raiz, "-c", "user.name=Teste", "-c", "user.email=teste@exemplo",
		"-c", "commit.gpgsign=false"}, argumentos...)...)
	if saida, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", argumentos, err, saida)
	}
}

// Regra: só um clone na branch develop dá para atualizar; outra branch, sem commit (fora de uma branch) ou sem git,
// não.
func TestDisponivel(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sem git")
	}
	raiz, inst := clone(t, false)
	git(t, raiz, "init", "--quiet", "--initial-branch", "develop")
	if Disponivel(t.Context(), inst) != "" {
		t.Error("sem commit: disponível")
	}
	git(t, raiz, "commit", "--quiet", "--allow-empty", "-m", "primeiro")
	if obtida := Disponivel(t.Context(), inst); obtida != raiz {
		t.Errorf("develop: %q", obtida)
	}
	git(t, raiz, "switch", "--quiet", "-c", "outra")
	if BranchAtual(t.Context(), raiz) != "outra" || Disponivel(t.Context(), inst) != "" {
		t.Error("outra branch: disponível")
	}
	git(t, raiz, "switch", "--quiet", "--detach")
	if BranchAtual(t.Context(), raiz) != "" {
		t.Error("fora de uma branch")
	}
	_, semClone := clone(t, false)
	if Disponivel(t.Context(), semClone) != "" {
		t.Error("sem clone: disponível")
	}
}

// O próprio executável do teste faz o papel do script: escreve as linhas pedidas e sai com o código pedido.
func TestMain(m *testing.M) {
	if os.Getenv("DESENVOLVIMENTO_NETO") == "1" {
		// Abre um "neto" que herda a saída e fica 6 s vivo, e espera ser morto: como o git que o PowerShell abre.
		exe, _ := os.Executable()
		neto := exec.Command(exe)
		neto.Env = append(os.Environ(), "DESENVOLVIMENTO_NETO=0", "DESENVOLVIMENTO_AJUDANTE=0", "DESENVOLVIMENTO_DORMIR=1")
		neto.Stdout, neto.Stderr = os.Stdout, os.Stderr
		_ = neto.Start()
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	if os.Getenv("DESENVOLVIMENTO_DORMIR") == "1" {
		time.Sleep(6 * time.Second)
		os.Exit(0)
	}
	if os.Getenv("DESENVOLVIMENTO_AJUDANTE") == "1" {
		fmt.Print("\x1b[36m==> Buscando\x1b[0m\r\nlinha 2\n")
		fmt.Fprint(os.Stderr, "no erro\n")
		fmt.Print("sem quebra no fim")
		if os.Getenv("DESENVOLVIMENTO_ESPERAR") == "1" {
			time.Sleep(time.Minute)
		}
		var codigo int
		fmt.Sscan(os.Getenv("DESENVOLVIMENTO_CODIGO"), &codigo)
		os.Exit(codigo)
	}
	os.Exit(m.Run())
}

func ajudante(ctx context.Context, t *testing.T, codigo string, esperar bool) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, exe)
	ambiente := "0"
	if esperar {
		ambiente = "1"
	}
	t.Setenv("DESENVOLVIMENTO_AJUDANTE", "1")
	t.Setenv("DESENVOLVIMENTO_CODIGO", codigo)
	t.Setenv("DESENVOLVIMENTO_ESPERAR", ambiente)
	return cmd
}

type linhas struct {
	trava sync.Mutex
	todas []string
}

func (l *linhas) anotar(linha string) {
	l.trava.Lock()
	defer l.trava.Unlock()
	l.todas = append(l.todas, linha)
}

func (l *linhas) texto() string {
	l.trava.Lock()
	defer l.trava.Unlock()
	return strings.Join(l.todas, "|")
}

// Regra: cada linha chega ao vivo, sem as cores do terminal e sem o \r, inclusive o erro e a última sem quebra; o
// código de saída do script volta.
func TestRodarAoVivo(t *testing.T) {
	var l linhas
	codigo := RodarAoVivo(t.Context(), ajudante(t.Context(), t, "2", false), l.anotar)
	if codigo != 2 {
		t.Errorf("código %d", codigo)
	}
	for _, quer := range []string{"==> Buscando", "linha 2", "no erro", "sem quebra no fim"} {
		if !strings.Contains("|"+l.texto()+"|", "|"+quer+"|") {
			t.Errorf("sem a linha %q: %s", quer, l.texto())
		}
	}
	if strings.Contains(l.texto(), "\x1b") || strings.Contains(l.texto(), "\r") {
		t.Errorf("sobrou cor ou \\r: %q", l.texto())
	}
	if RodarAoVivo(t.Context(), ajudante(t.Context(), t, "0", false), func(string) {}) != 0 {
		t.Error("código 0")
	}
}

// Regra: programa que não existe ou cancelado dá -1, com o aviso na caixa.
func TestRodarAoVivoSemProgramaOuCancelado(t *testing.T) {
	var l linhas
	if codigo := RodarAoVivo(t.Context(), exec.Command(filepath.Join(t.TempDir(), "nao-existe.exe")), l.anotar); codigo != -1 ||
		!strings.Contains(l.texto(), "não foi encontrado neste computador") {
		t.Errorf("sem programa: %d %s", codigo, l.texto())
	}
	ctx, cancelar := context.WithCancel(t.Context())
	cmd := ajudante(ctx, t, "0", true)
	time.AfterFunc(500*time.Millisecond, cancelar)
	inicio := time.Now()
	if codigo := RodarAoVivo(ctx, cmd, func(string) {}); codigo != -1 || time.Since(inicio) > 20*time.Second {
		t.Errorf("cancelado: %d em %s", codigo, time.Since(inicio))
	}
}

// Regra: só as linhas novas do log (o que já havia fica de fora); no fim, a última sem quebra também.
func TestAcompanharLog(t *testing.T) {
	t.Parallel()
	log := filepath.Join(t.TempDir(), "instalacao-windows.log")
	if err := os.WriteFile(log, []byte("de uma instalação antiga\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var l linhas
	ctx, cancelar := context.WithCancel(t.Context())
	pronto := make(chan struct{})
	go func() {
		AcompanharLog(ctx, log, l.anotar, 10*time.Millisecond)
		close(pronto)
	}()
	acrescentar := func(texto string) {
		f, err := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Error(err)
			return
		}
		_, _ = f.WriteString(texto)
		f.Close()
	}
	// O acompanhamento anota o tamanho do log ao começar; o que vem depois é novo.
	time.Sleep(200 * time.Millisecond)
	acrescentar("compilando\r\nmeio ")
	time.Sleep(100 * time.Millisecond)
	acrescentar("da linha\nfim sem quebra")
	time.Sleep(100 * time.Millisecond)
	cancelar()
	<-pronto
	if obtido := l.texto(); obtido != "compilando|meio da linha|fim sem quebra" {
		t.Errorf("%q", obtido)
	}
}

// Regra: sem o log (o instalador ainda não começou), nada chega e nada trava.
func TestAcompanharLogSemArquivo(t *testing.T) {
	t.Parallel()
	ctx, cancelar := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancelar()
	var l linhas
	AcompanharLog(ctx, filepath.Join(t.TempDir(), "nao-existe.log"), l.anotar, 10*time.Millisecond)
	if l.texto() != "" {
		t.Errorf("%q", l.texto())
	}
}

// Regra: o log do instalador fica em instaladores, na pasta do projeto.
func TestLog(t *testing.T) {
	t.Parallel()
	if Log(filepath.Join("C:", "projeto")) != filepath.Join("C:", "projeto", "instaladores", "instalacao-windows.log") {
		t.Error(Log("x"))
	}
}

// Regra: cancelado, a espera não fica presa num filho do programa que segura a saída aberta (o git que o PowerShell
// abriu): no máximo 2 s depois de matar o programa. O neto vive 6 s; o teste espera ele acabar, para não deixar o
// executável do teste preso.
func TestCanceladoComNeto(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DESENVOLVIMENTO_NETO", "1")
	ctx, cancelar := context.WithCancel(t.Context())
	time.AfterFunc(500*time.Millisecond, cancelar)
	inicio := time.Now()
	t.Cleanup(func() { time.Sleep(time.Until(inicio.Add(7 * time.Second))) })
	if codigo := RodarAoVivo(ctx, exec.CommandContext(ctx, exe), func(string) {}); codigo != -1 || time.Since(inicio) > 4500*time.Millisecond {
		t.Errorf("código %d em %s", codigo, time.Since(inicio))
	}
}
