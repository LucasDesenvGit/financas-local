package servicosistema

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// registrador é o handler seguinte da cadeia: guarda o que recebeu.
type registrador struct {
	mensagens []string
	atributos int
	grupos    int
}

func (r *registrador) Enabled(_ context.Context, nivel slog.Level) bool {
	return nivel >= slog.LevelInfo
}
func (r *registrador) Handle(_ context.Context, rec slog.Record) error {
	r.mensagens = append(r.mensagens, rec.Message)
	return nil
}
func (r *registrador) WithAttrs(a []slog.Attr) slog.Handler { r.atributos += len(a); return r }
func (r *registrador) WithGroup(string) slog.Handler        { r.grupos++; return r }

type eventosFalsos struct {
	avisos, erros []string
	id            uint32
	falha         error
}

func (e *eventosFalsos) Warning(id uint32, m string) error {
	e.id = id
	e.avisos = append(e.avisos, m)
	return e.falha
}
func (e *eventosFalsos) Error(id uint32, m string) error {
	e.id = id
	e.erros = append(e.erros, m)
	return e.falha
}

func registro(nivel slog.Level, mensagem string) slog.Record {
	return slog.NewRecord(time.Time{}, nivel, mensagem, 0)
}

// Regra: o Log de Eventos recebe os avisos e erros, cada um no seu nível, e o Info não (Logging:EventLog em Warning, como
// no appsettings do .NET). O próximo handler recebe tudo.
func TestSoAvisoEErroVaoParaOLogDeEventos(t *testing.T) {
	casos := []struct {
		nome          string
		nivel         slog.Level
		avisos, erros int
	}{
		{"debug", slog.LevelDebug, 0, 0},
		{"informação", slog.LevelInfo, 0, 0},
		{"aviso", slog.LevelWarn, 1, 0},
		{"entre aviso e erro", slog.LevelError - 1, 1, 0},
		{"erro", slog.LevelError, 0, 1},
		{"acima de erro", slog.LevelError + 4, 0, 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			eventos, proximo := &eventosFalsos{}, &registrador{}
			if err := novoParaEventos(proximo, eventos).Handle(t.Context(), registro(c.nivel, "texto")); err != nil {
				t.Fatal(err)
			}
			if len(eventos.avisos) != c.avisos || len(eventos.erros) != c.erros {
				t.Errorf("avisos = %v, erros = %v", eventos.avisos, eventos.erros)
			}
			if len(proximo.mensagens) != 1 {
				t.Errorf("o próximo recebeu %v", proximo.mensagens)
			}
		})
	}
}

func TestTextoDoEvento(t *testing.T) {
	eventos, proximo := &eventosFalsos{}, &registrador{}
	log := slog.New(novoParaEventos(proximo, eventos)).With("origem", "Open Finance").WithGroup("rede")
	log.Warn("sem resposta", "tentativas", 3)
	if len(eventos.avisos) != 1 {
		t.Fatalf("avisos = %v", eventos.avisos)
	}
	texto := eventos.avisos[0]
	for _, trecho := range []string{`msg="sem resposta"`, `origem="Open Finance"`, "rede.tentativas=3"} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("sem %q em %q", trecho, texto)
		}
	}
	for _, trecho := range []string{"time=", "level=", "\n"} {
		if strings.Contains(texto, trecho) {
			t.Errorf("com %q em %q", trecho, texto)
		}
	}
	if eventos.id != idDoEvento {
		t.Errorf("id = %d", eventos.id)
	}
	if proximo.atributos != 1 || proximo.grupos != 1 {
		t.Errorf("o próximo não recebeu os atributos e o grupo: %+v", proximo)
	}
}

// Regra: falhar ao escrever no Log de Eventos não impede o próximo handler, e o erro volta para quem quiser saber.
func TestFalhaNoLogDeEventos(t *testing.T) {
	falha := errors.New("log cheio")
	eventos, proximo := &eventosFalsos{falha: falha}, &registrador{}
	h := novoParaEventos(proximo, eventos)
	err := h.Handle(t.Context(), registro(slog.LevelError, "falhou"))
	if !errors.Is(err, falha) {
		t.Errorf("erro = %v", err)
	}
	if len(proximo.mensagens) != 1 {
		t.Errorf("mensagens = %v", proximo.mensagens)
	}
}

func TestEnabledVemDoProximo(t *testing.T) {
	h := novoParaEventos(&registrador{}, &eventosFalsos{})
	if h.Enabled(t.Context(), slog.LevelDebug) || !h.Enabled(t.Context(), slog.LevelInfo) {
		t.Error("Enabled não seguiu o próximo handler")
	}
}
