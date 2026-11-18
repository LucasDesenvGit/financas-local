package cripto

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
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

// Regressão (#762): o 403 que o Mercado Bitcoin devolve desde 14/11/2026 é recusa da fonte, não falta de internet. Vira
// erro com essa mensagem, uma vez enquanto a recusa for a mesma (era um aviso de "rede" a cada 30 minutos, 48 por
// dia); outra recusa registra de novo, e a volta é avisada e zera a conta.
func TestCicloRegistraARecusaDaFonteUmaVez(t *testing.T) {
	t.Parallel()
	fonte := &fonteFalsa{err: fmt.Errorf("%w: status 403", ErrRecusa), negocios: map[string]Negocio{}}
	s, _, _, saida := novoServicoDeTeste(t, relogio.Fixo{Instante: utc(15, 1, 0)}, fonte)
	tarefa := NovaTarefa(s, s.log)
	ciclo := func() {
		t.Helper()
		if err := tarefa.Ciclo(t.Context()); err != nil {
			t.Fatalf("a recusa não volta como erro para o tarefas.Periodica (registraria de novo): %v", err)
		}
	}
	linhas := func(trecho string) (n int) {
		for _, l := range strings.Split(saida.String(), "\n") {
			if strings.Contains(l, trecho) {
				n++
				if !strings.Contains(l, "origem=Cripto") {
					t.Errorf("sem a origem Cripto: %s", l)
				}
			}
		}
		return n
	}
	const recusou, voltou = "recusou a consulta da cotação", "voltou a aceitar"

	for range 3 {
		ciclo()
	}
	if n := linhas(recusou); n != 1 {
		t.Errorf("3 ciclos com o mesmo 403 registraram %d vezes, quer 1:\n%s", n, saida.String())
	}
	if !strings.Contains(saida.String(), "level=ERROR") || strings.Contains(saida.String(), "Não foi possível consultar") {
		t.Errorf("a recusa tem de ser erro, e não o aviso de rede:\n%s", saida.String())
	}
	fonte.err = fmt.Errorf("%w: status 401", ErrRecusa)
	ciclo()
	if n := linhas(recusou); n != 2 {
		t.Errorf("outra recusa (401) devia registrar de novo: %d", n)
	}
	fonte.err = nil
	ciclo()
	ciclo()
	if n := linhas(voltou); n != 1 {
		t.Errorf("a volta devia ser avisada uma vez: %d", n)
	}
	fonte.err = fmt.Errorf("%w: status 401", ErrRecusa)
	ciclo()
	if n := linhas(recusou); n != 3 {
		t.Errorf("a mesma recusa depois de voltar devia registrar de novo: %d", n)
	}
}
