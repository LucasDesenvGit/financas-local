package autenticacao

import (
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

var chaveDeTeste = []byte("0123456789abcdef0123456789abcdef")

func TestSessaoAssinada(t *testing.T) {
	agora := time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)
	s := conteudoSessao{Usuario: 1, Login: "ana.teste", Carimbo: tipos.NovoGuid(), Emitido: agora.Unix(), Expira: agora.Add(2 * time.Hour).Unix()}
	valor := valorSessao(chaveDeTeste, s)

	if lida, ok := lerSessao(chaveDeTeste, valor, agora); !ok || lida != s {
		t.Fatalf("ida e volta: %v %+v", ok, lida)
	}
	partes := strings.SplitN(valor, ".", 2)
	outro := s
	outro.Usuario = 2
	forjado := valorSessao([]byte("outra-chave-outra-chave-outra-ch"), outro)

	for nome, c := range map[string]struct {
		valor string
		agora time.Time
	}{
		"expirada":                  {valor, agora.Add(2 * time.Hour)},
		"assinatura de outra chave": {forjado, agora},
		"conteúdo trocado":          {strings.SplitN(valorSessao(chaveDeTeste, outro), ".", 2)[0] + "." + partes[1], agora},
		"sem assinatura":            {partes[0], agora},
		"lixo":                      {"a.b", agora},
	} {
		t.Run(nome, func(t *testing.T) {
			if _, ok := lerSessao(chaveDeTeste, c.valor, c.agora); ok {
				t.Error("aceitou")
			}
		})
	}
}

// A renovação deslizante do .NET: só depois da metade da janela.
func TestRenovacaoDaSessao(t *testing.T) {
	inicio := time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)
	s := conteudoSessao{Emitido: inicio.Unix(), Expira: inicio.Add(120 * time.Minute).Unix()}
	for _, c := range []struct {
		depois  time.Duration
		renovar bool
	}{{0, false}, {59 * time.Minute, false}, {61 * time.Minute, true}, {119 * time.Minute, true}} {
		if got := s.precisaRenovar(inicio.Add(c.depois)); got != c.renovar {
			t.Errorf("%v depois: %v", c.depois, got)
		}
	}
}

// O token CSRF é ligado ao segredo do cookie e ao usuário: o de quem ainda não entrou não vale depois do login.
func TestTokenCsrf(t *testing.T) {
	anonimo := tokenCsrf(chaveDeTeste, "segredo", 0)
	if !csrfValido(chaveDeTeste, "segredo", anonimo, 0) {
		t.Error("o token do anônimo vale para o anônimo")
	}
	for nome, c := range map[string]struct {
		segredo, cabecalho string
		usuario            int64
	}{
		"token do anônimo depois do login": {"segredo", anonimo, 1},
		"outro segredo":                    {"outro", anonimo, 0},
		"sem cabeçalho":                    {"segredo", "", 0},
		"sem cookie":                       {"", anonimo, 0},
	} {
		t.Run(nome, func(t *testing.T) {
			if csrfValido(chaveDeTeste, c.segredo, c.cabecalho, c.usuario) {
				t.Error("aceitou")
			}
		})
	}
}

func TestLimitadorJanelaFixaPorIP(t *testing.T) {
	agora := time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)
	l := novoLimitador(5, time.Minute)
	l.agora = func() time.Time { return agora }
	for i := 1; i <= 5; i++ {
		if !l.permitir("127.0.0.1") {
			t.Fatalf("%dª recusada", i)
		}
	}
	if l.permitir("127.0.0.1") {
		t.Error("6ª no mesmo minuto passou")
	}
	if !l.permitir("127.0.0.2") {
		t.Error("outro IP tem a própria cota")
	}
	agora = agora.Add(59 * time.Second)
	if l.permitir("127.0.0.1") {
		t.Error("a janela é de um minuto inteiro desde o 1º pedido")
	}
	agora = agora.Add(time.Second)
	if !l.permitir("127.0.0.1") {
		t.Error("na janela seguinte volta a passar")
	}
}
