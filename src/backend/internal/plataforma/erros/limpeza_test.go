package erros

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// textoFixo implementa encoding.TextMarshaler, como o tipo de dinheiro e o de data do projeto. É uma struct para o
// MarshalText dar um texto diferente do %+v ("{texto:...}"), e o teste saber qual dos dois foi usado. Com erro, o
// texto sai pelo %+v, como no TextHandler.
type textoFixo struct {
	texto  string
	falhar bool
}

func (t textoFixo) MarshalText() ([]byte, error) {
	if t.falhar {
		return nil, errors.New("sem texto")
	}
	return []byte(t.texto), nil
}

// valorComposto é um valor sem Error nem MarshalText: o TextHandler o escreve com %+v.
type valorComposto struct{ Pix string }

// logLimpo é o log do serviço como no main: o TextHandler por dentro do ComLimpeza, sem a hora (fixa no teste).
func logLimpo() (*slog.Logger, *bytes.Buffer) {
	var saida bytes.Buffer
	semHora := &slog.HandlerOptions{ReplaceAttr: func(grupos []string, a slog.Attr) slog.Attr {
		if len(grupos) == 0 && a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return a
	}}
	return slog.New(ComLimpeza(slog.NewTextHandler(&saida, semHora))), &saida
}

// Regra (#531): a mensagem e cada tipo de atributo saem pelo Limpar; o que não é texto passa como veio.
func TestComLimpeza(t *testing.T) {
	casos := []struct {
		nome     string
		logar    func(*slog.Logger)
		esperado string
	}{
		{"mensagem com CPF e valor",
			func(l *slog.Logger) { l.Error("Pix de R$ 1.234,56 para o CPF 123.456.789-09 recusado") },
			`level=ERROR msg="Pix de R$ [removido] para o CPF [removido] recusado"`},
		{"atributo de texto",
			func(l *slog.Logger) {
				l.Warn("resposta da Pluggy", "corpo", `{"clientSecret":"abc123","cpf":"12345678909"}`)
			},
			`level=WARN msg="resposta da Pluggy" corpo="{\"clientSecret\":\"[removido]\",\"cpf\":\"[removido]\"}"`},
		{"atributo de erro, embrulhado",
			func(l *slog.Logger) {
				l.Error("sincronizar", "erro", fmt.Errorf("GET https://api.pluggy.ai/items?apiKey=xyz: %w", errors.New("saldo R$ -10,00")))
			},
			`level=ERROR msg=sincronizar erro="GET https://api.pluggy.ai/items?[removido] saldo R$ [removido]"`},
		{"atributo que é TextMarshaler",
			func(l *slog.Logger) { l.Info("valor", "v", textoFixo{texto: "R$ 99,90"}) },
			`level=INFO msg=valor v="R$ [removido]"`},
		{"atributo que é TextMarshaler, com erro",
			func(l *slog.Logger) { l.Info("valor", "v", textoFixo{texto: "R$ 99,90", falhar: true}) },
			`level=INFO msg=valor v="{texto:R$ [removido] falhar:true}"`},
		{"atributo de outro tipo, escrito com %+v",
			func(l *slog.Logger) { l.Info("pix", "dados", valorComposto{Pix: "123.456.789-09"}) },
			`level=INFO msg=pix dados={Pix:[removido]}`},
		{"atributo de bytes",
			func(l *slog.Logger) { l.Info("corpo", "b", []byte("token=segredo")) },
			`level=INFO msg=corpo b="token=[removido]"`},
		{"grupo, com o With e o WithGroup",
			func(l *slog.Logger) {
				l.With("conta", "Bearer abc.def").WithGroup("http").Warn("lento", "url", "https://x.test/a?cpf=1", "status", 503)
			},
			`level=WARN msg=lento conta="Bearer [removido]" http.url=https://x.test/a?[removido] http.status=503`},
		{"grupo aninhado num atributo",
			func(l *slog.Logger) {
				l.Info("g", slog.Group("pedido", "senha", "senha=123", slog.Group("x", "cpf", "111.222.333-44")))
			},
			`level=INFO msg=g pedido.senha="senha=[removido]" pedido.x.cpf=[removido]`},
		{"número, booleano, duração e IP passam como vieram",
			func(l *slog.Logger) {
				l.Info("ok", "conta", 12345678909, "ativa", true, "tempo", 2*time.Second, "ip", net.IPv4(127, 0, 0, 1))
			},
			`level=INFO msg=ok conta=12345678909 ativa=true tempo=2s ip=127.0.0.1`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			log, saida := logLimpo()
			c.logar(log)
			if obtido := strings.TrimSuffix(saida.String(), "\n"); obtido != c.esperado {
				t.Errorf("\nobtido   %s\nesperado %s", obtido, c.esperado)
			}
		})
	}
}

// Regra (#531): o nível continua sendo o do handler de dentro.
func TestComLimpezaRespeitaONivel(t *testing.T) {
	var saida bytes.Buffer
	log := slog.New(ComLimpeza(slog.NewTextHandler(&saida, &slog.HandlerOptions{Level: slog.LevelWarn})))
	log.Info("CPF 123.456.789-09")
	if saida.Len() != 0 {
		t.Errorf("o Info passou pelo handler de nível Warn: %q", saida.String())
	}
	if log.Enabled(t.Context(), slog.LevelInfo) || !log.Enabled(t.Context(), slog.LevelWarn) {
		t.Error("Enabled não é o do handler de dentro")
	}
}

// Regra (#531): montado como no main (tabela por fora, limpeza por dentro), o log do serviço sai limpo, e a tabela
// continua recebendo o erro original, com o tipo na primeira linha do detalhe (é ele que agrupa as ocorrências).
func TestComLimpezaNoMain(t *testing.T) {
	var saida bytes.Buffer
	fila := NovaFila()
	log := slog.New(NovoHandler(ComLimpeza(slog.NewTextHandler(&saida, nil)), fila, testeapoio.RelogioDosTestes()))
	log.Error("falhou", "origem", "Open Finance", "erro", &net.OpError{Op: "dial", Err: errors.New("R$ 5,00 para 123.456.789-09")})

	if s := saida.String(); strings.Contains(s, "5,00") || strings.Contains(s, "789-09") || !strings.Contains(s, "[removido]") {
		t.Errorf("o log do serviço não saiu limpo: %q", s)
	}
	r := <-fila.canal
	if r.Detalhe == nil || !strings.HasPrefix(*r.Detalhe, "*net.OpError: dial: ") {
		t.Errorf("a tabela perdeu o tipo do erro: %q", *r.Detalhe)
	}
}
