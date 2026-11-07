//go:build !windows

package servicosistema

import (
	"context"
	"errors"
	"log/slog"
	"syscall"
	"testing"
)

// Regra: o SIGTERM do systemd acaba o ctx do trabalho, e o erro dele volta para quem chamou. É por aqui que o serviço para
// com elegância no Linux.
func TestSigtermParaOTrabalho(t *testing.T) {
	falha := errors.New("parou")
	err := Rodar(func(ctx context.Context) error {
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		<-ctx.Done()
		return falha
	})
	if !errors.Is(err, falha) {
		t.Errorf("erro = %v", err)
	}
}

// Regra: no Linux, quem guarda o log é o journald, pela saída de erro. O handler fica como está.
func TestLogDoSistemaNoLinux(t *testing.T) {
	proximo := slog.DiscardHandler
	if h := ComLogDoSistema(proximo); h != proximo {
		t.Errorf("handler = %T", h)
	}
}
