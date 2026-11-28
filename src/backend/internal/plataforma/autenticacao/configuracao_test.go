package autenticacao

import (
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// Regra (#68, item 4): o token da configuração inicial é um HMAC do instante com o segredo do gerenciador, e vale 10
// minutos (com um pouco de tolerância para o futuro).
func TestTokenDeConfiguracaoValido(t *testing.T) {
	agora := testeapoio.InstanteDosTestes
	token := TokenDeConfiguracao(segredoDeTeste, agora)
	instante := token[:strings.Index(token, ".")]
	assinatura := token[len(instante)+1:]
	casos := []struct {
		nome, segredo, token string
		quando               time.Time
		valido               bool
	}{
		{"recém-gerado", segredoDeTeste, token, agora, true},
		{"no último segundo da validade", segredoDeTeste, token, agora.Add(ValidadeTokenConfiguracao), true},
		{"vencido", segredoDeTeste, token, agora.Add(ValidadeTokenConfiguracao + time.Second), false},
		{"do futuro, dentro da tolerância", segredoDeTeste, token, agora.Add(-toleranciaRelogio), true},
		{"do futuro, além da tolerância", segredoDeTeste, token, agora.Add(-toleranciaRelogio - time.Second), false},
		{"outro segredo", "outro", token, agora, false},
		{"sem segredo carregado", "", TokenDeConfiguracao("", agora), agora, false},
		{"assinatura trocada", segredoDeTeste, instante + "." + strings.Repeat("0", 64), agora, false},
		{"instante trocado (assinatura de outro)", segredoDeTeste, "1." + assinatura, agora, false},
		{"sem ponto", segredoDeTeste, instante, agora, false},
		{"instante que não é número", segredoDeTeste, "x." + assinatura, agora, false},
		{"vazio", segredoDeTeste, "", agora, false},
		{"com espaços nas pontas", segredoDeTeste, " " + token + "\n", agora, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := TokenDeConfiguracaoValido(c.segredo, c.token, c.quando); got != c.valido {
				t.Errorf("TokenDeConfiguracaoValido = %v, esperado %v", got, c.valido)
			}
		})
	}
}

// O valor de referência também está no gerenciador (src/gerenciador-go/internal/configuracao), que não pode importar
// este pacote: se a fórmula mudar aqui, o teste de lá muda junto.
func TestTokenDeConfiguracaoValorDeReferencia(t *testing.T) {
	const esperado = "1760000000.23c730ff718f1b04d3deb18b165ad8b68b6d169f5c9e6986e66c034025f9323f"
	if got := TokenDeConfiguracao(segredoDeTeste, time.Unix(1760000000, 0)); got != esperado {
		t.Errorf("token = %q, esperado %q", got, esperado)
	}
}

// Bug do item 4: outro processo do computador criava o usuário antes do dono. Sem o token (ou com um que não vale), o
// configurar dá 403 e nenhum usuário é criado; com o token do segredo, cria.
func TestConfigurarExigeOTokenDoGerenciador(t *testing.T) {
	a := novoAmbiente(t)
	a.auth.limite = novoLimitador(1000, time.Minute)
	corpo := credenciais{"ana.teste", "Senha-Forte-123"}
	intruso := a.navegador()
	for _, c := range []struct{ nome, token string }{
		{"sem token", ""},
		{"token de outro segredo", TokenDeConfiguracao("outro", a.relogio.Agora())},
		{"token vencido", TokenDeConfiguracao(segredoDeTeste, a.relogio.Agora().Add(-ValidadeTokenConfiguracao-time.Minute))},
	} {
		t.Run(c.nome, func(t *testing.T) {
			intruso.semToken, intruso.tokenDeConfiguracao = c.token == "", c.token
			intruso.esperar(t, "POST", "/api/auth/configurar", corpo, 403, "gerenciador")
		})
	}
	intruso.esperar(t, "GET", "/api/auth/estado", nil, 200, `"configurado":false`)

	dono := a.navegador()
	dono.esperar(t, "POST", "/api/auth/configurar", corpo, 200, `"autenticado":true`)
	dono.esperar(t, "GET", "/api/auth/estado", nil, 200, `"configurado":true`)
}

// Sem segredo carregado no site, ninguém configura (falha fechada), nem com um token feito com texto vazio.
func TestConfigurarSemSegredoCarregadoRecusa(t *testing.T) {
	a := novoAmbiente(t)
	a.auth.o.ChaveConfiguracao = ""
	n := a.navegador()
	n.tokenDeConfiguracao = TokenDeConfiguracao("", a.relogio.Agora())
	n.esperar(t, "POST", "/api/auth/configurar", credenciais{"ana.teste", "Senha-Forte-123"}, 403, "gerenciador")
}
