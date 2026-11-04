package pluggy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/tipos"
)

// As respostas em testdata/ são fictícias, no formato da Pluggy (PluggyModelos.cs): nunca a de uma conta de verdade.

const chave = "chave-ficticia"

var sessao = Sessao{chave: chave}

// pluggyFalsa é a Pluggy num serviço falso (testeapoio.ServicoDeFora), com os cabeçalhos do .NET, e as esperas guardadas
// em vez de dormir.
func pluggyFalsa(t *testing.T) (*Cliente, *testeapoio.ServicoDeFora, *[]time.Duration) {
	t.Helper()
	falsa := testeapoio.NovoServicoDeFora(t, "X-API-KEY")
	falsa.Exigir("User-Agent", "FinancasLocal/1.0")
	c, err := NovoCliente(falsa.URL()+"/", falsa.Cliente(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	esperas := &[]time.Duration{}
	c.esperar = func(_ context.Context, d time.Duration) error {
		*esperas = append(*esperas, d)
		return nil
	}
	return c, falsa, esperas
}

// transporteQueFalha faz o papel da rede: todo pedido falha com o erro dado, sem sair para a rede.
type transporteQueFalha struct {
	err     error
	pedidos int
}

func (f *transporteQueFalha) RoundTrip(*http.Request) (*http.Response, error) {
	f.pedidos++
	return nil, f.err
}

// erroDeTempo é o erro de rede com Timeout(): o que o http.Client dá quando o limite de 100 s acaba.
type erroDeTempo struct{}

func (erroDeTempo) Error() string   { return "tempo acabou" }
func (erroDeTempo) Timeout() bool   { return true }
func (erroDeTempo) Temporary() bool { return true }

func clienteSemRede(t *testing.T, err error) (*Cliente, *transporteQueFalha, *[]time.Duration, *bytes.Buffer) {
	t.Helper()
	transporte := &transporteQueFalha{err: err}
	log := &bytes.Buffer{}
	c, erro := NovoCliente("https://pluggy.teste/", &http.Client{Transport: transporte}, slog.New(slog.NewJSONHandler(log, nil)))
	if erro != nil {
		t.Fatal(erro)
	}
	esperas := &[]time.Duration{}
	c.esperar = func(_ context.Context, d time.Duration) error {
		*esperas = append(*esperas, d)
		return nil
	}
	return c, transporte, esperas, log
}

func TestNovoCliente(t *testing.T) {
	casos := []struct {
		nome, url, erro string
	}{
		{"sem URL vale a de produção", "", ""},
		{"https", "https://pluggy.teste/", ""},
		{"http é recusado", "http://pluggy.teste/", "deve usar HTTPS"},
		{"URL quebrada", "https://[::1", "URL da Pluggy inválida"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := NovoCliente(c.url, nil, slog.New(slog.DiscardHandler))
			if c.erro == "" && err != nil {
				t.Fatalf("NovoCliente(%q) = %v, esperado sem erro", c.url, err)
			}
			if c.erro != "" && (err == nil || !strings.Contains(err.Error(), c.erro)) {
				t.Fatalf("NovoCliente(%q) = %v, esperado %q", c.url, err, c.erro)
			}
		})
	}

	c, err := NovoCliente("", nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if c.base.String() != URLPadrao || c.http.Timeout != 100*time.Second {
		t.Errorf("cliente padrão: %s, %v; esperado %s, 100 s", c.base, c.http.Timeout, URLPadrao)
	}
	c, err = NovoCliente("https://pluggy.teste/api", nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if c.base.Path != "/api/" {
		t.Errorf("caminho da base = %q, esperado com a barra no fim", c.base.Path)
	}
}

func TestCriarSessao(t *testing.T) {
	casos := []struct {
		nome     string
		resposta testeapoio.RespostaDeFora
		mensagem string
		status   int
	}{
		{"400", testeapoio.RespostaDeFora{Status: 400}, "Pluggy recusou o Client ID / Client Secret. Verifique as credenciais em Configurações.", 400},
		{"401", testeapoio.NaoAutorizado, "Pluggy recusou o Client ID / Client Secret. Verifique as credenciais em Configurações.", 401},
		{"403", testeapoio.RespostaDeFora{Status: 403}, "Pluggy recusou o Client ID / Client Secret. Verifique as credenciais em Configurações.", 403},
		{"500", testeapoio.ErroDoServidor, "Falha ao autenticar na Pluggy (HTTP 500).", 500},
		{"outro status fora dos 2xx", testeapoio.RespostaDeFora{Status: 304}, "Falha ao autenticar na Pluggy (HTTP 304).", 304},
		{"sem a chave", testeapoio.RespostaDeFora{Corpo: `{}`}, "A Pluggy não retornou uma API key.", 0},
		{"chave em branco", testeapoio.RespostaDeFora{Corpo: `{"apiKey":"  "}`}, "A Pluggy não retornou uma API key.", 0},
		{"corpo null", testeapoio.RespostaDeFora{Corpo: `null`}, "A Pluggy não retornou uma API key.", 0},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c, falsa, esperas := pluggyFalsa(t)
			falsa.Em("POST", "/auth", caso.resposta)
			_, err := c.CriarSessao(t.Context(), "id", "segredo")
			var erro *Erro
			if !errors.As(err, &erro) || erro.Mensagem != caso.mensagem || erro.Status != caso.status {
				t.Fatalf("CriarSessao = %#v, esperado %q (status %d)", err, caso.mensagem, caso.status)
			}
			if len(falsa.Pedidos()) != 1 || len(*esperas) != 0 {
				t.Errorf("%d pedidos e %d esperas: a autenticação não tenta de novo", len(falsa.Pedidos()), len(*esperas))
			}
		})
	}

	t.Run("sucesso", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("POST", "/auth", testeapoio.RespostaDeFora{Corpo: `{"apiKey":"` + chave + `"}`})
		s, err := c.CriarSessao(t.Context(), "id-\"1", "segredo")
		if err != nil {
			t.Fatal(err)
		}
		if s.chave != chave {
			t.Errorf("chave = %q", s.chave)
		}
		p := falsa.Pedidos()[0]
		if p.Corpo != `{"clientId":"id-\"1","clientSecret":"segredo"}` {
			t.Errorf("corpo = %s", p.Corpo)
		}
		if tipo := p.Cabecalhos.Get("Content-Type"); tipo != "application/json; charset=utf-8" {
			t.Errorf("Content-Type = %q", tipo)
		}
	})

	t.Run("JSON quebrado", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("POST", "/auth", testeapoio.JSONQuebrado)
		if _, err := c.CriarSessao(t.Context(), "id", "segredo"); !errors.Is(err, ErrResposta) {
			t.Fatalf("CriarSessao = %v, esperado ErrResposta", err)
		}
	})

	t.Run("sem rede não tenta de novo", func(t *testing.T) {
		c, rede, esperas, _ := clienteSemRede(t, errors.New("sem rota"))
		if _, err := c.CriarSessao(t.Context(), "id", "segredo"); !errors.Is(err, ErrRede) {
			t.Fatalf("CriarSessao = %v, esperado ErrRede", err)
		}
		if rede.pedidos != 1 || len(*esperas) != 0 {
			t.Errorf("%d pedidos e %d esperas", rede.pedidos, len(*esperas))
		}
	})
}

func TestStatusDeErroDoGet(t *testing.T) {
	longo := strings.Repeat("é", 301)
	casos := []struct {
		nome     string
		resposta testeapoio.RespostaDeFora
		mensagem string
		status   int
		pedidos  int
	}{
		{"401", testeapoio.NaoAutorizado, "Acesso negado pela Pluggy. Verifique as credenciais e a autorização do Meu Pluggy.", 401, 1},
		{"403", testeapoio.RespostaDeFora{Status: 403}, "Acesso negado pela Pluggy. Verifique as credenciais e a autorização do Meu Pluggy.", 403, 1},
		{"404", testeapoio.RespostaDeFora{Status: 404}, "Recurso não encontrado na Pluggy (items/item-1). Confira o Item ID.", 404, 1},
		{"400, com o corpo", testeapoio.RespostaDeFora{Status: 400, Corpo: "ruim"}, "Erro HTTP 400 da Pluggy em items/item-1: ruim", 400, 1},
		{"corpo cortado em 300 caracteres", testeapoio.RespostaDeFora{Status: 400, Corpo: longo}, "Erro HTTP 400 da Pluggy em items/item-1: " + strings.Repeat("é", 300), 400, 1},
		{"429 esgota as 4 tentativas", testeapoio.RespostaDeFora{Status: 429, Corpo: "calma"}, "Erro HTTP 429 da Pluggy em items/item-1: calma", 429, 4},
		{"500 esgota as 4 tentativas", testeapoio.RespostaDeFora{Status: 500, Corpo: "caiu"}, "Erro HTTP 500 da Pluggy em items/item-1: caiu", 500, 4},
		{"503 esgota as 4 tentativas", testeapoio.RespostaDeFora{Status: 503}, "Erro HTTP 503 da Pluggy em items/item-1: ", 503, 4},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c, falsa, _ := pluggyFalsa(t)
			falsa.Em("GET", "/items/item-1", caso.resposta)
			_, err := c.ObterItem(t.Context(), sessao, "item-1")
			var erro *Erro
			if !errors.As(err, &erro) || erro.Mensagem != caso.mensagem || erro.Status != caso.status {
				t.Fatalf("ObterItem = %v, esperado %q (status %d)", err, caso.mensagem, caso.status)
			}
			if n := len(falsa.Pedidos()); n != caso.pedidos {
				t.Errorf("%d pedidos, esperado %d", n, caso.pedidos)
			}
		})
	}
}

func TestEsperasDoTransitorio(t *testing.T) {
	segundos := func(s ...int) []time.Duration {
		esperado := []time.Duration{}
		for _, n := range s {
			esperado = append(esperado, time.Duration(n)*time.Second)
		}
		return esperado
	}
	com := func(status int, retry string) testeapoio.RespostaDeFora {
		r := testeapoio.RespostaDeFora{Status: status}
		if retry != "" {
			r.Cabecalhos = map[string]string{"Retry-After": retry}
		}
		return r
	}
	casos := []struct {
		nome     string
		resposta testeapoio.RespostaDeFora
		esperas  []time.Duration
	}{
		{"sem Retry-After, 3 s vezes a tentativa", com(429, ""), segundos(3, 6, 9)},
		{"500 espera igual", com(500, ""), segundos(3, 6, 9)},
		{"Retry-After em segundos", com(429, "5"), segundos(5, 5, 5)},
		{"Retry-After zero", com(429, "0"), segundos(0, 0, 0)},
		{"limite de 1 minuto", com(429, "61"), segundos(60, 60, 60)},
		{"exatamente 1 minuto", com(429, "60"), segundos(60, 60, 60)},
		{"Retry-After em data é ignorado", com(503, "Wed, 21 Oct 2026 07:28:00 GMT"), segundos(3, 6, 9)},
		{"Retry-After negativo é ignorado", com(503, "-1"), segundos(3, 6, 9)},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c, falsa, esperas := pluggyFalsa(t)
			falsa.Em("GET", "/items/item-1", caso.resposta)
			if _, err := c.ObterItem(t.Context(), sessao, "item-1"); err == nil {
				t.Fatal("ObterItem sem erro")
			}
			if !slices.Equal(*esperas, caso.esperas) {
				t.Errorf("esperas = %v, esperado %v", *esperas, caso.esperas)
			}
		})
	}

	t.Run("volta ao normal na tentativa seguinte", func(t *testing.T) {
		c, falsa, esperas := pluggyFalsa(t)
		falsa.Em("GET", "/items/item-1", com(429, ""), testeapoio.ErroDoServidor, testeapoio.RespostaDeFora{Arquivo: "item.json"})
		item, err := c.ObterItem(t.Context(), sessao, "item-1")
		if err != nil {
			t.Fatal(err)
		}
		if item.Id != "item-ficticio-1" || len(*esperas) != 2 {
			t.Errorf("item %q depois de %d esperas", item.Id, len(*esperas))
		}
	})
}

func TestFalhaDeRedeDoGet(t *testing.T) {
	const itemSecreto = "item-que-nao-vai-para-o-log"
	c, rede, esperas, log := clienteSemRede(t, errors.New("sem rota até o host"))
	_, err := c.ObterItem(t.Context(), sessao, itemSecreto)
	if !errors.Is(err, ErrRede) {
		t.Fatalf("ObterItem = %v, esperado ErrRede", err)
	}
	if rede.pedidos != 3 {
		t.Errorf("%d pedidos, esperado 3", rede.pedidos)
	}
	if !slices.Equal(*esperas, []time.Duration{2 * time.Second, 4 * time.Second}) {
		t.Errorf("esperas = %v, esperado 2 s e 4 s", *esperas)
	}
	if !strings.Contains(err.Error(), "sem rota até o host") {
		t.Errorf("erro sem a causa: %v", err)
	}
	escrito := log.String()
	if !strings.Contains(escrito, `"recurso":"items"`) || !strings.Contains(escrito, `"origem":"Open Finance"`) {
		t.Errorf("log sem o recurso ou a origem: %s", escrito)
	}
	for nome, texto := range map[string]string{"erro": err.Error(), "log": escrito} {
		if strings.Contains(texto, itemSecreto) || strings.Contains(texto, chave) || strings.Contains(texto, "pluggy.teste") {
			t.Errorf("%s com o id, a chave ou a URL: %s", nome, texto)
		}
	}
}

func TestTempoEsgotadoNaoTentaDeNovo(t *testing.T) {
	t.Run("pelo erro de rede", func(t *testing.T) {
		c, rede, esperas, _ := clienteSemRede(t, erroDeTempo{})
		_, err := c.ObterItem(t.Context(), sessao, "item-1")
		if !errors.Is(err, ErrTempoEsgotado) || errors.Is(err, ErrRede) {
			t.Fatalf("ObterItem = %v, esperado ErrTempoEsgotado, e não ErrRede", err)
		}
		if rede.pedidos != 1 || len(*esperas) != 0 {
			t.Errorf("%d pedidos e %d esperas", rede.pedidos, len(*esperas))
		}
	})
	t.Run("pelo limite do http.Client", func(t *testing.T) {
		falsa := testeapoio.NovoServicoDeFora(t, "X-API-KEY")
		falsa.Em("GET", "*", testeapoio.TempoEsgotado)
		curto := falsa.Cliente()
		curto.Timeout = 30 * time.Millisecond
		c, err := NovoCliente(falsa.URL()+"/", curto, slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.ObterItem(t.Context(), sessao, "item-1"); !errors.Is(err, ErrTempoEsgotado) {
			t.Fatalf("ObterItem = %v, esperado ErrTempoEsgotado", err)
		}
	})
}

func TestCancelamento(t *testing.T) {
	t.Run("durante o pedido", func(t *testing.T) {
		c, rede, _, _ := clienteSemRede(t, context.Canceled)
		ctx, cancelar := context.WithCancel(t.Context())
		cancelar()
		if _, err := c.ObterItem(ctx, sessao, "item-1"); !errors.Is(err, context.Canceled) {
			t.Fatalf("ObterItem = %v, esperado o cancelamento", err)
		}
		if rede.pedidos > 1 {
			t.Errorf("%d pedidos depois de cancelar", rede.pedidos)
		}
	})
	t.Run("durante a espera da rede", func(t *testing.T) {
		c, rede, _, _ := clienteSemRede(t, errors.New("sem rota"))
		c.esperar = func(context.Context, time.Duration) error { return context.Canceled }
		if _, err := c.ObterItem(t.Context(), sessao, "item-1"); !errors.Is(err, context.Canceled) {
			t.Fatalf("ObterItem = %v, esperado o cancelamento", err)
		}
		if rede.pedidos != 1 {
			t.Errorf("%d pedidos, esperado 1", rede.pedidos)
		}
	})
	t.Run("durante a espera do transitório", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/items/item-1", testeapoio.MuitosPedidos)
		c.esperar = func(context.Context, time.Duration) error { return context.Canceled }
		if _, err := c.ObterItem(t.Context(), sessao, "item-1"); !errors.Is(err, context.Canceled) {
			t.Fatalf("ObterItem = %v, esperado o cancelamento", err)
		}
		if n := len(falsa.Pedidos()); n != 1 {
			t.Errorf("%d pedidos, esperado 1", n)
		}
	})
}

func TestDormir(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		inicio := time.Now()
		if err := dormir(t.Context(), 6*time.Second); err != nil {
			t.Fatal(err)
		}
		if d := time.Since(inicio); d != 6*time.Second {
			t.Errorf("dormiu %v, esperado 6 s", d)
		}
	})
	synctest.Test(t, func(t *testing.T) {
		ctx, cancelar := context.WithTimeout(t.Context(), time.Second)
		defer cancelar()
		inicio := time.Now()
		if err := dormir(ctx, time.Minute); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("dormir = %v, esperado o fim do contexto", err)
		}
		if d := time.Since(inicio); d != time.Second {
			t.Errorf("dormiu %v, esperado sair em 1 s", d)
		}
	})
}

// corpoQueFalha é a resposta que cai no meio do corpo.
type corpoQueFalha struct{}

func (corpoQueFalha) Read([]byte) (int, error) { return 0, errors.New("a conexão caiu") }
func (corpoQueFalha) Close() error             { return nil }

type transporteDeResposta struct{ pedidos int }

func (f *transporteDeResposta) RoundTrip(req *http.Request) (*http.Response, error) {
	f.pedidos++
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: corpoQueFalha{}, Request: req}, nil
}

// Como no HttpClient, que lê o corpo dentro do SendAsync: a falha no meio do corpo é falha de rede e tenta de novo.
func TestFalhaNoMeioDoCorpo(t *testing.T) {
	transporte := &transporteDeResposta{}
	c, err := NovoCliente("https://pluggy.teste/", &http.Client{Transport: transporte}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	c.esperar = func(context.Context, time.Duration) error { return nil }
	if _, err := c.ObterItem(t.Context(), sessao, "item-1"); !errors.Is(err, ErrRede) {
		t.Fatalf("ObterItem = %v, esperado ErrRede", err)
	}
	if transporte.pedidos != 3 {
		t.Errorf("%d pedidos, esperado 3", transporte.pedidos)
	}
}

func TestTamanhoMaximoDaResposta(t *testing.T) {
	corpo := `{"id":"item-1"}`
	casos := []struct {
		nome   string
		limite int64
		passa  bool
	}{
		{"no limite", int64(len(corpo)), true},
		{"um byte acima", int64(len(corpo)) - 1, false},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c, falsa, _ := pluggyFalsa(t)
			c.tamanhoMaximo = caso.limite
			falsa.Em("GET", "/items/item-1", testeapoio.RespostaDeFora{Corpo: corpo})
			_, err := c.ObterItem(t.Context(), sessao, "item-1")
			if caso.passa && err != nil {
				t.Fatalf("ObterItem = %v", err)
			}
			if !caso.passa && !errors.Is(err, ErrResposta) {
				t.Fatalf("ObterItem = %v, esperado ErrResposta", err)
			}
		})
	}
}

func TestStatus2xxEhSucesso(t *testing.T) {
	for _, status := range []int{200, 201, 299} {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/items/item-1", testeapoio.RespostaDeFora{Status: status, Arquivo: "item.json"})
		if _, err := c.ObterItem(t.Context(), sessao, "item-1"); err != nil {
			t.Errorf("status %d: %v", status, err)
		}
		falsa.Em("POST", "/auth", testeapoio.RespostaDeFora{Status: status, Corpo: `{"apiKey":"k"}`})
		if _, err := c.CriarSessao(t.Context(), "id", "segredo"); err != nil {
			t.Errorf("status %d: %v", status, err)
		}
	}
}

func TestErroMostraAMensagem(t *testing.T) {
	if err := error(&Erro{Mensagem: "para quem usa", Status: 400}); err.Error() != "para quem usa" {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestErroAoListarCategorias(t *testing.T) {
	c, falsa, _ := pluggyFalsa(t)
	falsa.Em("GET", "/categories", testeapoio.RespostaDeFora{Status: 400})
	if _, err := c.ListarCategorias(t.Context(), sessao); err == nil {
		t.Fatal("ListarCategorias sem erro")
	}
}

func TestRespostasForaDoFormato(t *testing.T) {
	t.Run("corpo null é resposta vazia", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/accounts", testeapoio.RespostaDeFora{Corpo: " null\n"})
		_, err := c.ListarContas(t.Context(), sessao, "item-1")
		var erro *Erro
		if !errors.As(err, &erro) || erro.Mensagem != "Resposta vazia da Pluggy em accounts." || erro.Status != 0 {
			t.Fatalf("ListarContas = %#v", err)
		}
	})
	t.Run("JSON quebrado", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/accounts", testeapoio.JSONQuebrado)
		if _, err := c.ListarContas(t.Context(), sessao, "item-1"); !errors.Is(err, ErrResposta) {
			t.Fatalf("ListarContas = %v, esperado ErrResposta", err)
		}
	})
	t.Run("número fora do formato não traz o valor", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/accounts", testeapoio.RespostaDeFora{Corpo: `{"results":[{"id":"c","balance":"valor-secreto"}]}`})
		_, err := c.ListarContas(t.Context(), sessao, "item-1")
		if !errors.Is(err, ErrResposta) {
			t.Fatalf("ListarContas = %v, esperado ErrResposta", err)
		}
		if strings.Contains(err.Error(), "valor-secreto") {
			t.Errorf("erro com o valor lido: %v", err)
		}
	})
}

func TestRedirecionamento(t *testing.T) {
	t.Run("para outro host não leva a chave", func(t *testing.T) {
		outro := testeapoio.NovoServicoDeFora(t, "X-API-KEY")
		outro.Em("GET", "*", testeapoio.RespostaDeFora{Corpo: `{}`})
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/items/item-1", testeapoio.RespostaDeFora{Status: 302, Cabecalhos: map[string]string{"Location": outro.URL() + "/roubo"}})
		_, err := c.ObterItem(t.Context(), sessao, "item-1")
		var erro *Erro
		if !errors.As(err, &erro) || erro.Status != 302 {
			t.Fatalf("ObterItem = %v, esperado o erro com o status 302", err)
		}
		if n := len(outro.Pedidos()); n != 0 {
			t.Errorf("o outro host recebeu %d pedidos", n)
		}
	})
	t.Run("no mesmo host segue", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/items/item-1", testeapoio.RespostaDeFora{Status: 302, Cabecalhos: map[string]string{"Location": "/items/item-2"}})
		falsa.Em("GET", "/items/item-2", testeapoio.RespostaDeFora{Arquivo: "item.json"})
		if _, err := c.ObterItem(t.Context(), sessao, "item-1"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("para http no mesmo host não segue", func(t *testing.T) {
		c, _, _ := pluggyFalsa(t)
		req, err := http.NewRequest(http.MethodGet, "http://"+c.base.Host+"/x", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.soNoMesmoHost(req, nil); !errors.Is(err, http.ErrUseLastResponse) {
			t.Fatalf("soNoMesmoHost = %v, esperado parar", err)
		}
	})
	t.Run("para de seguir no décimo", func(t *testing.T) {
		c, _, _ := pluggyFalsa(t)
		req, err := http.NewRequest(http.MethodGet, c.base.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.soNoMesmoHost(req, make([]*http.Request, 9)); err != nil {
			t.Errorf("com 9 anteriores = %v, esperado seguir", err)
		}
		if err := c.soNoMesmoHost(req, make([]*http.Request, 10)); err == nil || errors.Is(err, http.ErrUseLastResponse) {
			t.Errorf("com 10 anteriores = %v, esperado erro", err)
		}
	})
}

func TestEnderecos(t *testing.T) {
	const id = "a b/c+d"
	const escapado = "a%20b%2Fc%2Bd"
	casos := []struct {
		nome    string
		chamar  func(*Cliente) error
		rota    string
		uri     string
		arquivo string
	}{
		{"item", func(c *Cliente) error { _, e := c.ObterItem(t.Context(), sessao, id); return e },
			"*", "/items/" + escapado, "item.json"},
		{"contas", func(c *Cliente) error { _, e := c.ListarContas(t.Context(), sessao, id); return e },
			"/accounts", "/accounts?itemId=" + escapado, "conta-e-cartao.json"},
		{"categorias", func(c *Cliente) error { _, e := c.ListarCategorias(t.Context(), sessao); return e },
			"/categories", "/categories", ""},
		{"faturas", func(c *Cliente) error { _, e := c.ListarFaturas(t.Context(), sessao, id); return e },
			"/bills", "/bills?accountId=" + escapado + "&page=1&pageSize=500", ""},
		{"investimentos", func(c *Cliente) error { _, e := c.ListarInvestimentos(t.Context(), sessao, id); return e },
			"/investments", "/investments?itemId=" + escapado + "&page=1&pageSize=500", ""},
		{"empréstimos", func(c *Cliente) error { _, e := c.ListarEmprestimos(t.Context(), sessao, id); return e },
			"/loans", "/loans?itemId=" + escapado + "&page=1&pageSize=500", ""},
		{"movimentos", func(c *Cliente) error { _, e := c.ListarMovimentosInvestimento(t.Context(), sessao, id); return e },
			"*", "/investments/" + escapado + "/transactions?page=1&pageSize=500", ""},
		{"transações", func(c *Cliente) error {
			_, e := c.ListarTransacoes(t.Context(), sessao, id, tipos.NovaData(2026, 10, 5))
			return e
		}, "/v2/transactions", "/v2/transactions?accountId=" + escapado + "&dateFrom=2026-10-05", ""},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c, falsa, _ := pluggyFalsa(t)
			resposta := testeapoio.PaginaVazia("results")
			if caso.arquivo != "" {
				resposta = testeapoio.RespostaDeFora{Arquivo: caso.arquivo}
			}
			falsa.Em("GET", caso.rota, resposta)
			if err := caso.chamar(c); err != nil {
				t.Fatal(err)
			}
			if uris := falsa.URIs(); len(uris) != 1 || uris[0] != caso.uri {
				t.Errorf("URIs = %v, esperado %q", uris, caso.uri)
			}
			p := falsa.Pedidos()[0]
			if !p.TemSegredo("X-API-KEY") || p.Cabecalhos.Get("Accept") != "application/json" {
				t.Errorf("cabeçalhos: %v", p.Cabecalhos)
			}
		})
	}
}

func TestPaginacaoNumerada(t *testing.T) {
	pagina := func(total int, ids ...string) testeapoio.RespostaDeFora {
		itens := []string{}
		for _, id := range ids {
			itens = append(itens, `{"id":"`+id+`"}`)
		}
		return testeapoio.RespostaDeFora{Corpo: fmt.Sprintf(`{"totalPages":%d,"results":[%s]}`, total, strings.Join(itens, ","))}
	}
	casos := []struct {
		nome      string
		respostas []testeapoio.RespostaDeFora
		ids       []string
		pedidos   int
	}{
		{"uma página", []testeapoio.RespostaDeFora{pagina(1, "a")}, []string{"a"}, 1},
		{"sem páginas", []testeapoio.RespostaDeFora{pagina(0)}, []string{}, 1},
		{"duas páginas", []testeapoio.RespostaDeFora{pagina(2, "a"), pagina(2, "b")}, []string{"a", "b"}, 2},
		{"página vazia para antes do total", []testeapoio.RespostaDeFora{pagina(3, "a"), pagina(3)}, []string{"a"}, 2},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c, falsa, _ := pluggyFalsa(t)
			falsa.Em("GET", "/bills", caso.respostas...)
			faturas, err := c.ListarFaturas(t.Context(), sessao, "cartao-1")
			if err != nil {
				t.Fatal(err)
			}
			if faturas == nil {
				t.Fatal("lista nil")
			}
			ids := []string{}
			for _, f := range faturas {
				ids = append(ids, f.Id)
			}
			if !slices.Equal(ids, caso.ids) || len(falsa.Pedidos()) != caso.pedidos {
				t.Errorf("ids %v em %d pedidos, esperado %v em %d", ids, len(falsa.Pedidos()), caso.ids, caso.pedidos)
			}
			for i, uri := range falsa.URIs() {
				if !strings.HasSuffix(uri, "&page="+strconv.Itoa(i+1)+"&pageSize=500") {
					t.Errorf("pedido %d: %s", i, uri)
				}
			}
		})
	}

	t.Run("para na página 200", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/bills", pagina(9999, "a"))
		faturas, err := c.ListarFaturas(t.Context(), sessao, "cartao-1")
		if err != nil {
			t.Fatal(err)
		}
		if len(faturas) != 200 || len(falsa.Pedidos()) != 200 {
			t.Errorf("%d faturas em %d pedidos, esperado 200", len(faturas), len(falsa.Pedidos()))
		}
	})

	t.Run("erro no meio devolve o erro", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/bills", pagina(2, "a"), testeapoio.RespostaDeFora{Status: 400})
		if _, err := c.ListarFaturas(t.Context(), sessao, "cartao-1"); err == nil {
			t.Fatal("ListarFaturas sem erro")
		}
	})
}

func TestPaginacaoPorCursor(t *testing.T) {
	t.Run("segue só o cursor do link", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/v2/transactions", testeapoio.RespostaDeFora{Arquivo: "transacoes-pagina-1.json"},
			testeapoio.RespostaDeFora{Arquivo: "transacoes-pagina-2.json"})
		transacoes, err := c.ListarTransacoes(t.Context(), sessao, "cartao-1", tipos.NovaData(2026, 10, 1))
		if err != nil {
			t.Fatal(err)
		}
		if len(transacoes) != 2 || transacoes[0].Id != "t-1" || transacoes[1].Id != "t-2" {
			t.Fatalf("transações = %+v", transacoes)
		}
		esperado := []string{
			"/v2/transactions?accountId=cartao-1&dateFrom=2026-10-01",
			"/v2/transactions?accountId=cartao-1&dateFrom=2026-10-01&after=cursor%2F2%2B%20fim",
		}
		if !slices.Equal(falsa.URIs(), esperado) {
			t.Errorf("URIs = %v", falsa.URIs())
		}
	})

	t.Run("sem link, sem transações", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/v2/transactions", testeapoio.PaginaVazia("results"))
		transacoes, err := c.ListarTransacoes(t.Context(), sessao, "cartao-1", tipos.NovaData(2026, 10, 1))
		if err != nil {
			t.Fatal(err)
		}
		if transacoes == nil || len(transacoes) != 0 {
			t.Errorf("transações = %#v, esperado lista vazia", transacoes)
		}
	})

	t.Run("erro no meio devolve o erro", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/v2/transactions", testeapoio.RespostaDeFora{Arquivo: "transacoes-pagina-1.json"}, testeapoio.RespostaDeFora{Status: 400})
		if _, err := c.ListarTransacoes(t.Context(), sessao, "cartao-1", tipos.NovaData(2026, 10, 1)); err == nil {
			t.Fatal("ListarTransacoes sem erro")
		}
	})

	sempreMais := testeapoio.RespostaDeFora{Corpo: `{"next":"https://x/?after=mais","results":[]}`}
	ultimaNaFronteira := func(paginas int) []testeapoio.RespostaDeFora {
		r := make([]testeapoio.RespostaDeFora, paginas-1, paginas)
		for i := range r {
			r[i] = sempreMais
		}
		return append(r, testeapoio.RespostaDeFora{Corpo: `{"results":[]}`})
	}
	t.Run("a página 1000 ainda passa", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/v2/transactions", ultimaNaFronteira(1000)...)
		if _, err := c.ListarTransacoes(t.Context(), sessao, "cartao-1", tipos.NovaData(2026, 10, 1)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a página 1001 passa do limite, mesmo sendo a última", func(t *testing.T) {
		c, falsa, _ := pluggyFalsa(t)
		falsa.Em("GET", "/v2/transactions", ultimaNaFronteira(1001)...)
		_, err := c.ListarTransacoes(t.Context(), sessao, "cartao-1", tipos.NovaData(2026, 10, 1))
		var erro *Erro
		if !errors.As(err, &erro) || erro.Mensagem != "Paginação de transações excedeu o limite de segurança." {
			t.Fatalf("ListarTransacoes = %v", err)
		}
		if n := len(falsa.Pedidos()); n != 1001 {
			t.Errorf("%d pedidos, esperado 1001", n)
		}
	})
}

func TestExtrairCursor(t *testing.T) {
	casos := []struct{ nome, proximo, esperado string }{
		{"vazio", "", ""},
		{"sem query", "https://api.pluggy.ai/v2/transactions", ""},
		{"sem after", "https://x/?page=2", ""},
		{"depois de outro parâmetro", "https://x/?a=1&after=abc", "abc"},
		{"antes de outro parâmetro", "https://x/?after=abc&a=1", "abc"},
		{"maiúsculas no nome", "https://x/?AFTER=abc", "abc"},
		{"decodifica", "https://x/?after=a%2Fb%2Bc", "a/b+c"},
		{"mais vira espaço", "https://x/?after=a+b", "a b"},
		{"escape quebrado fica como veio", "https://x/?after=a%zz", "a%zz"},
		{"em branco", "https://x/?after=", ""},
		{"só espaços", "https://x/?after=%20", ""},
		{"host de fora não importa", "https://ladrao.teste/?after=abc", "abc"},
		{"chave parecida não vale", "https://x/?afterwards=abc", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if obtido := extrairCursor(c.proximo); obtido != c.esperado {
				t.Errorf("extrairCursor(%q) = %q, esperado %q", c.proximo, obtido, c.esperado)
			}
		})
	}
}

func TestEscapar(t *testing.T) {
	casos := []struct{ valor, esperado string }{
		{"", ""},
		{"abcXYZ019-_.~", "abcXYZ019-_.~"},
		{"a b", "a%20b"},
		{"a+b", "a%2Bb"},
		{"a/b?c&d=e", "a%2Fb%3Fc%26d%3De"},
		{"ação", "a%C3%A7%C3%A3o"},
		{"z{|}!*'()", "z%7B%7C%7D%21%2A%27%28%29"},
		{"@`[", "%40%60%5B"},
	}
	for _, c := range casos {
		if obtido := escapar(c.valor); obtido != c.esperado {
			t.Errorf("escapar(%q) = %q, esperado %q", c.valor, obtido, c.esperado)
		}
	}
}

func TestRecursoParaLog(t *testing.T) {
	casos := []struct{ caminho, esperado string }{
		{"items/abc", "items"},
		{"accounts?itemId=abc", "accounts"},
		{"categories", "categories"},
		{"investments/abc/transactions?page=1", "investments"},
		{"loans?itemId=abc", "loans"},
		{"v2/transactions?accountId=abc", "v2/transactions"},
		{"bills?accountId=abc", "outro"},
	}
	for _, c := range casos {
		if obtido := recursoParaLog(c.caminho); obtido != c.esperado {
			t.Errorf("recursoParaLog(%q) = %q, esperado %q", c.caminho, obtido, c.esperado)
		}
	}
}

func TestSanitizarParaLog(t *testing.T) {
	if obtido := sanitizarParaLog("a\r\nb\nc\rd"); obtido != "abcd" {
		t.Errorf("sanitizarParaLog = %q", obtido)
	}
}

func TestSessaoNaoMostraAChave(t *testing.T) {
	for _, formato := range []string{"%v", "%s", "%+v"} {
		if texto := fmt.Sprintf(formato, sessao); strings.Contains(texto, chave) {
			t.Errorf("Sprintf(%q) = %q", formato, texto)
		}
	}
}
