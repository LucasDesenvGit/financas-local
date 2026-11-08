package erros

import (
	"context"
	"encoding"
	"fmt"
	"log/slog"
)

// ComLimpeza passa o log do serviço (journald no Linux, Log de Eventos no Windows) pelo mesmo Limpar dos erros gravados
// no banco (#531): a mensagem e todo atributo de texto, de erro ou de outro valor que vira texto saem sem segredo, CPF
// e valor em reais. Número, booleano, hora e duração passam como vieram: o Limpar só tira o que é texto.
//
// Fica por fora de tudo o que escreve fora do processo, para nenhum caminho escapar. A tabela de erros já limpa ao
// gravar (LimparECortar); limpar duas vezes não muda nada, porque o "[removido]" não casa com nenhum padrão.
func ComLimpeza(proximo slog.Handler) slog.Handler {
	return limpeza{proximo: proximo}
}

type limpeza struct{ proximo slog.Handler }

func (h limpeza) Enabled(ctx context.Context, nivel slog.Level) bool {
	return h.proximo.Enabled(ctx, nivel)
}

func (h limpeza) Handle(ctx context.Context, rec slog.Record) error {
	limpo := slog.NewRecord(rec.Time, rec.Level, Limpar(rec.Message), rec.PC)
	rec.Attrs(func(a slog.Attr) bool {
		limpo.AddAttrs(limparAtributo(a))
		return true
	})
	return h.proximo.Handle(ctx, limpo)
}

func (h limpeza) WithAttrs(attrs []slog.Attr) slog.Handler {
	limpos := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		limpos[i] = limparAtributo(a)
	}
	return limpeza{proximo: h.proximo.WithAttrs(limpos)}
}

func (h limpeza) WithGroup(nome string) slog.Handler {
	return limpeza{proximo: h.proximo.WithGroup(nome)}
}

// limparAtributo limpa o valor que vira texto no log. O que não é texto (KindAny) vira texto aqui, do mesmo jeito que
// o TextHandler o escreveria (Error, MarshalText ou %+v), para o Limpar poder olhar.
func limparAtributo(a slog.Attr) slog.Attr {
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, Limpar(v.String()))
	case slog.KindGroup:
		grupo := v.Group()
		limpos := make([]slog.Attr, len(grupo))
		for i, g := range grupo {
			limpos[i] = limparAtributo(g)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(limpos...)}
	case slog.KindAny:
		return slog.String(a.Key, Limpar(comoTexto(v.Any())))
	default:
		return slog.Attr{Key: a.Key, Value: v}
	}
}

func comoTexto(valor any) string {
	switch x := valor.(type) {
	case error:
		return x.Error()
	case encoding.TextMarshaler:
		if b, err := x.MarshalText(); err == nil {
			return string(b)
		}
	case []byte:
		return string(x)
	}
	return fmt.Sprintf("%+v", valor)
}
