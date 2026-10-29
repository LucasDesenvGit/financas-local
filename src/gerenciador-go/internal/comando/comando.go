// Package comando roda um programa do sistema (systemctl, loginctl, journalctl, busctl…) e devolve a saída, como o
// Comando.cs: argumentos separados, sem shell (nada é interpretado), com limite de tempo, e só com argumentos fixos do
// próprio gerenciador.
package comando

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Rodar espera o programa terminar e devolve o que ele escreveu. Com código de saída diferente de zero, o erro traz a
// mensagem do programa (o que ele escreveu no stderr; sem nada lá, o stdout), para mostrar na janela.
func Rodar(ctx context.Context, limite time.Duration, programa string, argumentos ...string) (string, error) {
	ctx, cancelar := context.WithTimeout(ctx, limite)
	defer cancelar()
	cmd := exec.CommandContext(ctx, programa, argumentos...)
	// Saída limpa para ler: sem paginador nem cores do systemd.
	cmd.Env = append(os.Environ(), "SYSTEMD_PAGER=", "SYSTEMD_COLORS=0")
	var texto, erro bytes.Buffer
	cmd.Stdout, cmd.Stderr = &texto, &erro
	err := cmd.Run()
	switch {
	case err == nil:
		return texto.String(), nil
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return "", fmt.Errorf("%s não respondeu em %.0f segundos", programa, limite.Seconds())
	case errors.Is(err, exec.ErrNotFound):
		return "", fmt.Errorf("o programa %s não foi encontrado neste computador", programa)
	}
	mensagem := strings.TrimSpace(erro.String())
	if mensagem == "" {
		mensagem = strings.TrimSpace(texto.String())
	}
	if mensagem == "" {
		return "", fmt.Errorf("%s: %w", programa, err)
	}
	return "", errors.New(mensagem)
}
