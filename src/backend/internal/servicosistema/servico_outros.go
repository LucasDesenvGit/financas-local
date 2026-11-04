//go:build !windows

package servicosistema

import "log/slog"

// Rodar roda o trabalho até o SIGTERM do systemd ou o Ctrl+C do terminal.
func Rodar(trabalho Trabalho) error { return comSinais(trabalho) }

// ComLogDoSistema devolve o handler como está: o journald guarda a saída de erro do serviço.
func ComLogDoSistema(proximo slog.Handler) slog.Handler { return proximo }
