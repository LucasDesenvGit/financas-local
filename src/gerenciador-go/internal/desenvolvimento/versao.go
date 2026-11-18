package desenvolvimento

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// A versão do clone não está mais num arquivo (ADR 0070): é calculada pelo src/backend/cmd/versao, em Go, a partir da
// base do Directory.Build.props e dos merges do git. O gerenciador pede o cálculo ao clone, com o mesmo Go da instalação.

// tempoDoCalculo cobre a compilação do cmd/versao na primeira vez (as seguintes saem do cache do Go).
const tempoDoCalculo = 3 * time.Minute

// Go é o go do projeto (ferramentas/go/bin, que a instalação pelos scripts baixa) ou, sem ele, o do PATH; "" se não
// houver nenhum. O segundo resultado diz se é o do projeto, que traz o ambiente próprio (módulos e cache em ferramentas/).
func Go(raiz string) (string, bool) {
	nome := "go"
	if runtime.GOOS == "windows" {
		nome = "go.exe"
	}
	caminhoDoProjeto := filepath.Join(raiz, "ferramentas", "go", "bin", nome)
	if _, err := os.Stat(caminhoDoProjeto); err == nil {
		return caminhoDoProjeto, true
	}
	if caminho, err := exec.LookPath("go"); err == nil {
		return caminho, false
	}
	return "", false
}

// CalcularVersao roda "go run ./cmd/versao <argumentos>" em src/backend do clone e devolve o que ele imprimiu: sem
// argumento, o número do HEAD; com -changelog, as seções do que ainda não foi publicado.
func CalcularVersao(ctx context.Context, raiz string, argumentos ...string) (string, error) {
	programa, doProjeto := Go(raiz)
	if programa == "" {
		return "", fmt.Errorf("o Go não foi encontrado (nem em %s, nem no PATH)", filepath.Join(raiz, "ferramentas", "go", "bin"))
	}
	ctx, cancelar := context.WithTimeout(ctx, tempoDoCalculo)
	defer cancelar()
	cmd := exec.CommandContext(ctx, programa, append([]string{"run", "./cmd/versao"}, argumentos...)...)
	cmd.Dir = filepath.Join(raiz, "src", "backend")
	// Como as instalações pelos scripts: nunca baixar outro Go sozinho; com o Go do projeto, os módulos e o cache dele.
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CGO_ENABLED=0")
	if doProjeto {
		cmd.Env = append(cmd.Env, "GOPATH="+filepath.Join(raiz, "ferramentas", "go-caminho"),
			"GOCACHE="+filepath.Join(raiz, "ferramentas", "go-cache"), "GOFLAGS=-modcacherw")
	}
	semJanela(cmd)
	var saida, erro bytes.Buffer
	cmd.Stdout, cmd.Stderr = &saida, &erro
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go run ./cmd/versao %s: %w: %s", strings.Join(argumentos, " "), err, strings.TrimSpace(erro.String()))
	}
	return strings.TrimSpace(saida.String()), nil
}
