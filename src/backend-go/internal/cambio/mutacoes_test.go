package cambio

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Testes que faltavam, achados pelo teste de mutação (#422).

// Regra: as correções de cada moeda se somam no resultado.
func TestAtualizarTodasSomaAsCorrigidas(t *testing.T) {
	t.Parallel()
	agora := testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0)
	fonte := &fonteFalsa{
		moedas: []Moeda{{Codigo: "EUR", Nome: "Euro"}, {Codigo: "USD", Nome: "Dólar"}},
		fechamentos: map[string][]Fechamento{
			"EUR": {{Data: dia(14), Compra: valor(t, "6.2981"), Venda: valor(t, "6.2995")}},
			"USD": {{Data: dia(14), Compra: valor(t, "5.4123"), Venda: valor(t, "5.4129")}},
		},
	}
	s, _, fab := novoServicoDeTeste(t, agora, fonte)
	// As duas já guardadas com outro valor: as duas são corrigidas.
	fab.CotacaoMoeda(func(c *testeapoio.CotacaoMoeda) { c.Moeda, c.Compra, c.Venda = "EUR", valor(t, "1"), valor(t, "1") })
	fab.CotacaoMoeda(func(c *testeapoio.CotacaoMoeda) { c.Compra, c.Venda = valor(t, "1"), valor(t, "1") })
	resultado, err := s.AtualizarTodas(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if resultado.Novas != 0 || resultado.Corrigidas != 2 || len(resultado.Falhas) != 0 {
		t.Errorf("resultado %+v, esperado 2 corrigidas", resultado)
	}
}

// Regra: hoje sem o fechamento, a API é consultada de novo só depois de passados 30 minutos da última consulta; e a
// consulta que dá certo não deixa aviso.
func TestObterCotacaoDeHoje(t *testing.T) {
	t.Parallel()
	agora := testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0) // 18:00 UTC

	t.Run("consultou há exatamente 30 minutos: não consulta", func(t *testing.T) {
		t.Parallel()
		fonte := &fonteFalsa{}
		s, _, fab := novoServicoDeTeste(t, agora, fonte)
		fab.Configuracao(prefixoUltimaConsulta+"USD", "2026-10-14T17:30:00.0000000Z")
		if _, err := s.ObterCotacao(t.Context(), "USD", nil); err != nil {
			t.Fatal(err)
		}
		if c := fonte.consultou(); len(c) != 0 {
			t.Errorf("consultou a API: %v", c)
		}
	})

	t.Run("consulta que dá certo não avisa", func(t *testing.T) {
		t.Parallel()
		fonte := &fonteFalsa{fechamentos: map[string][]Fechamento{"USD": {{Data: dia(14), Compra: valor(t, "5.4123"), Venda: valor(t, "5.4129")}}}}
		db := testeapoio.NovoBancoVazio(t)
		log, saida := logDoTeste()
		s := NovoServico(db, fonte, agora, log)
		c, err := s.ObterCotacao(t.Context(), "USD", nil)
		if err != nil || c == nil {
			t.Fatalf("cotação %v, erro %v", c, err)
		}
		if strings.Contains(saida.String(), "Não foi possível") {
			t.Errorf("aviso sem falha:\n%s", saida.String())
		}
	})
}

// Regra: a lista de moedas guardada só vale com alguma moeda e com menos de 7 dias; senão, consulta a API.
func TestListarMoedasNasBordas(t *testing.T) {
	t.Parallel()
	agora := testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0) // 18:00 UTC
	casos := []struct {
		nome, guardada, consultadaEm string
	}{
		{"consultada há pouco, mas nada guardado", "", "2026-10-14T12:00:00.0000000Z"},
		{"guardada há exatamente 7 dias", `[{"Codigo":"USD","Nome":"Dolar"}]`, "2026-10-07T18:00:00.0000000Z"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			fonte := &fonteFalsa{moedas: []Moeda{{Codigo: "EUR", Nome: "Euro"}}}
			s, _, fab := novoServicoDeTeste(t, agora, fonte)
			if c.guardada != "" {
				fab.Configuracao("Cambio.Moedas", c.guardada)
			}
			fab.Configuracao("Cambio.MoedasConsultadasEm", c.consultadaEm)
			moedas, err := s.ListarMoedas(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(fonte.consultou()) != 1 || jsonDe(t, moedas) != `[{"codigo":"EUR","nome":"Euro"}]` {
				t.Errorf("consultas %v, moedas %s: esperado consultar a API", fonte.consultou(), jsonDe(t, moedas))
			}
		})
	}
}

// Regra: sem a lista de moedas (sem internet e nada guardado), a moeda não é recusada como "não publicada".
func TestCotacaoSemListaDeMoedasNaoRecusaAMoeda(t *testing.T) {
	t.Parallel()
	fonte := &fonteFalsa{errMoedas: errors.New("sem rede"), falham: map[string]bool{"USD": true}}
	s, _, _ := novoServicoDeTeste(t, testeapoio.AgoraEmBrasilia(2026, 10, 14, 15, 0), fonte)
	mux := http.NewServeMux()
	log, _ := logDoTeste()
	RegistrarCom(mux, s, log)
	resposta := testeapoio.Chamar(t, mux, "GET", "/api/cambio/cotacao?moeda=USD", "")
	resposta.ConferirStatus(t, http.StatusServiceUnavailable)
}

// Regra: a tarefa não roda se a última atualização já foi depois do fechamento de hoje; depois de uma tentativa que
// falhou, tenta de novo com 30 minutos exatos; e a atualização sem falha é informação, não aviso.
func TestTarefaNasBordas(t *testing.T) {
	t.Parallel()
	inicio := time.Date(2026, 10, 14, 17, 0, 0, 0, time.UTC) // 14h em Brasília, depois do fechamento das 13h30
	moedas := []Moeda{{Codigo: "USD", Nome: "Dolar"}}
	fechamentos := map[string][]Fechamento{"USD": {{Data: dia(14), Compra: valor(t, "5.4123"), Venda: valor(t, "5.4129")}}}

	t.Run("já atualizado depois do fechamento: não roda", func(t *testing.T) {
		t.Parallel()
		r := &relogioAjustavel{t: inicio}
		fonte := &fonteFalsa{moedas: moedas, fechamentos: fechamentos}
		s, _, fab := novoServicoDeTeste(t, r, fonte)
		fab.Configuracao(ChaveUltimaAtualizacao, "2026-10-14T16:45:00.0000000Z") // 13h45 em Brasília
		log, _ := logDoTeste()
		if err := NovaTarefa(s, r, log).Ciclo(t.Context()); err != nil {
			t.Fatal(err)
		}
		if c := fonte.consultou(); len(c) != 0 {
			t.Errorf("rodou: %v", c)
		}
	})

	t.Run("tenta de novo com 30 minutos exatos", func(t *testing.T) {
		t.Parallel()
		r := &relogioAjustavel{t: inicio}
		fonte := &fonteFalsa{moedas: moedas, falham: map[string]bool{"USD": true}}
		s, _, _ := novoServicoDeTeste(t, r, fonte)
		log, _ := logDoTeste()
		tarefa := NovaTarefa(s, r, log)
		if err := tarefa.Ciclo(t.Context()); err != nil {
			t.Fatal(err)
		}
		antes := len(fonte.consultou())
		r.Adiantar(30 * time.Minute)
		if err := tarefa.Ciclo(t.Context()); err != nil {
			t.Fatal(err)
		}
		if depois := len(fonte.consultou()); depois == antes {
			t.Error("não tentou de novo com 30 minutos")
		}
	})

	t.Run("sem falha: informa, sem aviso", func(t *testing.T) {
		t.Parallel()
		r := &relogioAjustavel{t: inicio}
		s, _, _ := novoServicoDeTeste(t, r, &fonteFalsa{moedas: moedas, fechamentos: fechamentos})
		log, saida := logDoTeste()
		if err := NovaTarefa(s, r, log).Ciclo(t.Context()); err != nil {
			t.Fatal(err)
		}
		if texto := saida.String(); !strings.Contains(texto, "Câmbio atualizado.") || strings.Contains(texto, "level=WARN") {
			t.Errorf("log:\n%s", texto)
		}
	})
}

// Regra: a URL base sem a barra no fim ganha a barra, como o BaseAddress do HttpClient.
func TestPTAXComURLBaseSemBarraNoFim(t *testing.T) {
	t.Parallel()
	ptax := testeapoio.NovoServicoDeFora(t)
	ptax.Em("GET", "*", testeapoio.PaginaVazia("value"))
	c, err := NovoClientePTAX(ptax.URL()+"/odata", ptax.Cliente())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListarMoedas(context.Background()); err != nil {
		t.Fatal(err)
	}
	if pedido := ptax.URIs()[0]; pedido != "/odata/Moedas?$format=json" {
		t.Errorf("endereço %s", pedido)
	}
}

// Regra: as bordas da resposta da PTAX: status 299 vale; o corpo com exatamente o tamanho máximo também; e a data
// com só os 10 caracteres do dia é lida.
func TestPTAXNasBordas(t *testing.T) {
	t.Parallel()
	soODia := `{"value":[{"tipoBoletim":"Fechamento","dataHoraCotacao":"2026-10-14","cotacaoCompra":5.1,"cotacaoVenda":5.2}]}`
	casos := []struct {
		nome   string
		status int
		corpo  string
	}{
		{"status 299", 299, soODia},
		// O espaço vai antes: cortado um byte do fim, o JSON quebra.
		{"corpo com o tamanho máximo exato", http.StatusOK, strings.Repeat(" ", tamanhoMaximoRes-len(soODia)) + soODia},
		{"data só com o dia", http.StatusOK, soODia},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			cliente, _ := ptaxGravada(t, testeapoio.RespostaDeFora{Status: c.status, Corpo: c.corpo})
			fechamentos, err := cliente.BuscarFechamentos(context.Background(), "USD", tipos.NovaData(2026, 10, 1), tipos.NovaData(2026, 10, 15))
			if err != nil || len(fechamentos) != 1 || fechamentos[0].Data.String() != "2026-10-14" {
				t.Errorf("fechamentos %+v, erro %v", fechamentos, err)
			}
		})
	}
}

// Regra: status fora de 200 a 299 é erro, mesmo com um JSON que daria para ler.
func TestPTAXComErroEJSONValido(t *testing.T) {
	t.Parallel()
	cliente, _ := ptaxGravada(t, testeapoio.RespostaDeFora{Status: http.StatusServiceUnavailable, Corpo: `{"value":[]}`})
	if _, err := cliente.ListarMoedas(context.Background()); err == nil {
		t.Error("503 aceito")
	}
}
