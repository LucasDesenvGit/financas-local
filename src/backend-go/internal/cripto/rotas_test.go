package cripto

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func TestRotaDasCotacoes(t *testing.T) {
	t.Parallel()
	s, _, fab, saida := novoServicoDeTeste(t, relogio.Fixo{Instante: utc(15, 1, 0)}, &fonteFalsa{})
	fab.CotacaoCripto(func(c *testeapoio.CotacaoCripto) { c.DataHora = tipos.NovaDataHora(utc(14, 21, 0)) })
	fab.Configuracao(ChaveUltimaAtualizacao, "2026-10-14T21:00:03.1200000Z")
	mux := http.NewServeMux()
	RegistrarCom(mux, s, s.log)
	_ = saida

	resposta := testeapoio.Chamar(t, mux, "GET", "/api/cripto/cotacoes", "")
	resposta.ConferirStatus(t, http.StatusOK)
	resposta.ConferirJSON(t, `{"atualizadoEm":"2026-10-14T21:00:03.12Z","cotacoes":[`+
		`{"moeda":"BTC","preco":612345.67,"dataHora":"2026-10-14T21:00:00Z","variacao24h":null,"fonte":"Mercado Bitcoin"}]}`)

	semLogin := testeapoio.ChamarComo(t, mux, 0, "GET", "/api/cripto/cotacoes", "")
	semLogin.ConferirStatus(t, http.StatusUnauthorized)
}

func TestCicloDaTarefa(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome       string
		err        error
		voltaErro  bool
		avisoNoLog string
	}{
		{"sem internet é aviso, e tenta no próximo ciclo", fmt.Errorf("%w: status 503", ErrRede), false,
			"Não foi possível consultar a cotação de criptomoedas no Mercado Bitcoin."},
		{"resposta estranha é erro", errors.New("resposta inesperada do Mercado Bitcoin (não é uma lista)"), true, ""},
		{"tudo certo", nil, false, "Cotação de criptomoedas atualizada."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			fonte := &fonteFalsa{err: c.err, negocios: map[string]Negocio{}}
			s, _, _, saida := novoServicoDeTeste(t, relogio.Fixo{Instante: utc(15, 1, 0)}, fonte)
			err := NovaTarefa(s, s.log).Ciclo(t.Context())
			if (err != nil) != c.voltaErro {
				t.Errorf("erro %v, esperado erro: %v", err, c.voltaErro)
			}
			if c.avisoNoLog != "" && !strings.Contains(saida.String(), c.avisoNoLog) {
				t.Errorf("o log não tem %q:\n%s", c.avisoNoLog, saida.String())
			}
			// A origem na tela de erros é a do .NET desde a #298: "Cripto", não "Câmbio".
			for _, linha := range strings.Split(saida.String(), "\n") {
				if c.avisoNoLog != "" && strings.Contains(linha, c.avisoNoLog) && !strings.Contains(linha, "origem=Cripto") {
					t.Errorf("a linha do log não tem a origem Cripto: %s", linha)
				}
			}
		})
	}
}
