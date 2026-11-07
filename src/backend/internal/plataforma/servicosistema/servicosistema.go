// Pacote servicosistema faz o executável rodar como serviço do sistema (#84): o que UseWindowsService e UseSystemd fazem no
// Program.cs do .NET.
//
// No Linux, o systemd para o serviço com SIGTERM e guarda no journald o que sai na saída de erro: o processo não precisa
// saber que é serviço. No Windows, o processo conversa com o gerenciador de serviços e escreve no Log de Eventos, porque
// lá ninguém lê a saída de erro. O mesmo executável roda nos dois, e também no terminal, onde Ctrl+C para.
package servicosistema

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// Nome é o do serviço: FinancasLocal no Windows, financaslocal.service no systemd. É o mesmo do .NET, para a atualização
// substituir o serviço em vez de criar outro.
const Nome = "FinancasLocal"

// idDoEvento é o Id dos eventos no Log de Eventos: a mensagem já diz o que houve.
const idDoEvento = 1

// Trabalho é o que o processo faz até o ctx acabar. O erro é o motivo de ter parado.
type Trabalho func(ctx context.Context) error

// comSinais roda o trabalho até vir Ctrl+C ou SIGTERM.
func comSinais(trabalho Trabalho) error {
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()
	return trabalho(ctx)
}

// registroDeEventos é o que este pacote usa do eventlog.Log do Windows.
type registroDeEventos interface {
	Warning(id uint32, mensagem string) error
	Error(id uint32, mensagem string) error
}

// paraEventos manda ao Log de Eventos os avisos e erros (Logging:EventLog em Warning, como no appsettings do .NET) e passa
// tudo ao próximo handler. O texto sai de um TextHandler por nível: é ele que guarda os atributos do WithAttrs e do
// WithGroup, e o Log de Eventos separa aviso de erro.
type paraEventos struct {
	proximo     slog.Handler
	aviso, erro slog.Handler
}

func novoParaEventos(proximo slog.Handler, eventos registroDeEventos) slog.Handler {
	opcoes := &slog.HandlerOptions{ReplaceAttr: func(grupos []string, a slog.Attr) slog.Attr {
		// O Log de Eventos já guarda a hora e o nível.
		if len(grupos) == 0 && (a.Key == slog.TimeKey || a.Key == slog.LevelKey) {
			return slog.Attr{}
		}
		return a
	}}
	return paraEventos{
		proximo: proximo,
		aviso:   slog.NewTextHandler(escritorDeEvento(eventos.Warning), opcoes),
		erro:    slog.NewTextHandler(escritorDeEvento(eventos.Error), opcoes),
	}
}

// escritorDeEvento recebe do TextHandler uma linha por registro.
type escritorDeEvento func(id uint32, mensagem string) error

func (e escritorDeEvento) Write(linha []byte) (int, error) {
	if err := e(idDoEvento, strings.TrimSuffix(string(linha), "\n")); err != nil {
		return 0, err
	}
	return len(linha), nil
}

var _ io.Writer = escritorDeEvento(nil)

func (h paraEventos) Enabled(ctx context.Context, nivel slog.Level) bool {
	return h.proximo.Enabled(ctx, nivel)
}

func (h paraEventos) Handle(ctx context.Context, r slog.Record) error {
	var err error
	switch {
	case r.Level >= slog.LevelError:
		err = h.erro.Handle(ctx, r)
	case r.Level >= slog.LevelWarn:
		err = h.aviso.Handle(ctx, r)
	}
	return errors.Join(err, h.proximo.Handle(ctx, r))
}

func (h paraEventos) WithAttrs(atributos []slog.Attr) slog.Handler {
	return paraEventos{h.proximo.WithAttrs(atributos), h.aviso.WithAttrs(atributos), h.erro.WithAttrs(atributos)}
}

func (h paraEventos) WithGroup(nome string) slog.Handler {
	return paraEventos{h.proximo.WithGroup(nome), h.aviso.WithGroup(nome), h.erro.WithGroup(nome)}
}
