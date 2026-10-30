package selic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// clienteDeTeste aponta para um serviço falso que confere os cabeçalhos que o Program.cs manda.
func clienteDeTeste(t *testing.T) (*Cliente, *testeapoio.ServicoDeFora) {
	t.Helper()
	srv := testeapoio.NovoServicoDeFora(t)
	srv.Exigir("User-Agent", "FinancasLocal/1.0")
	srv.Exigir("Accept", "application/json")
	c, err := NovoCliente(srv.URL(), srv.Cliente())
	if err != nil {
		t.Fatal(err)
	}
	return c, srv
}

const caminhoDasTaxas = "/dados/serie/bcdata.sgs.11/dados"

// Regra: a Selic diária: o endereço com as datas em dd/MM/yyyy; a linha sem a data ou a taxa, ou com elas ilegíveis, é
// pulada; 404 é "período sem dia útil", e resposta que não é lista não traz nada; o resto é erro.
func TestBuscarTaxas(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		resposta testeapoio.RespostaDeFora
		esperado []string
		erro     bool
	}{
		{nome: "duas linhas", resposta: testeapoio.RespostaDeFora{Corpo: `[{"data":"02/01/2026","valor":"0.055131"},{"data":"05/01/2026","valor":"1,000.5"}]`},
			esperado: []string{"2026-01-02 0.055131", "2026-01-05 1000.5"}},
		{nome: "as linhas que não dá para ler são puladas", resposta: testeapoio.RespostaDeFora{Corpo: `[{"data":"2026-01-02","valor":"0.1"},
			{"data":"02/01/2026","valor":"abc"},{"valor":"0.1"},{"data":"02/01/2026"},{"data":null,"valor":"0.1"},{"data":"02/01/2026","valor":null},
			{"data":"03/01/2026","valor":"0.2"}]`}, esperado: []string{"2026-01-03 0.2"}},
		{nome: "lista vazia", resposta: testeapoio.RespostaDeFora{Corpo: `[]`}},
		{nome: "período sem dia útil", resposta: testeapoio.RespostaDeFora{Status: http.StatusNotFound, Corpo: `quebrado`}},
		{nome: "um objeto no lugar da lista", resposta: testeapoio.RespostaDeFora{Corpo: ` {"erro":"x"}`}},
		{nome: "erro do servidor", resposta: testeapoio.ErroDoServidor, erro: true},
		{nome: "JSON quebrado", resposta: testeapoio.JSONQuebrado, erro: true},
		{nome: "item que não é objeto", resposta: testeapoio.RespostaDeFora{Corpo: `[1]`}, erro: true},
		{nome: "item nulo", resposta: testeapoio.RespostaDeFora{Corpo: `[null]`}, erro: true},
		{nome: "data que não é texto", resposta: testeapoio.RespostaDeFora{Corpo: `[{"data":1,"valor":"0.1"}]`}, erro: true},
		{nome: "taxa que não é texto", resposta: testeapoio.RespostaDeFora{Corpo: `[{"data":"02/01/2026","valor":0.1}]`}, erro: true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			cliente, srv := clienteDeTeste(t)
			srv.Em("GET", caminhoDasTaxas, c.resposta)
			taxas, err := cliente.BuscarTaxas(t.Context(), testeapoio.Dia(2026, 1, 2), testeapoio.Dia(2026, 12, 31))
			if (err != nil) != c.erro {
				t.Fatalf("erro %v, esperado %v", err, c.erro)
			}
			var obtido []string
			for _, x := range taxas {
				obtido = append(obtido, x.Data.String()+" "+x.Taxa.String())
			}
			if strings.Join(obtido, ",") != strings.Join(c.esperado, ",") {
				t.Errorf("taxas %v, esperado %v", obtido, c.esperado)
			}
			if uri := srv.URIs()[0]; uri != caminhoDasTaxas+"?formato=json&dataInicial=02/01/2026&dataFinal=31/12/2026" {
				t.Errorf("endereço %s", uri)
			}
		})
	}
}

// Regra: a meta: a última da lista, em texto; o campo é obrigatório, e o que não é lista é erro (aqui, diferente da
// Selic diária, o C# usa o EnumerateArray direto).
func TestBuscarMeta(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		resposta testeapoio.RespostaDeFora
		esperado string
		erro     bool
	}{
		{nome: "a meta", resposta: testeapoio.RespostaDeFora{Corpo: `[{"data":"01/10/2026","valor":"15.00"}]`}, esperado: "15.00"},
		{nome: "a última da lista", resposta: testeapoio.RespostaDeFora{Corpo: `[{"valor":"14.75"},{"valor":"15.00"}]`}, esperado: "15.00"},
		{nome: "lista vazia", resposta: testeapoio.RespostaDeFora{Corpo: `[]`}},
		{nome: "o último status de sucesso", resposta: testeapoio.RespostaDeFora{Status: 299, Corpo: `[{"valor":"15.00"}]`}, esperado: "15.00"},
		{nome: "valor nulo", resposta: testeapoio.RespostaDeFora{Corpo: `[{"valor":null}]`}},
		{nome: "sem o valor", resposta: testeapoio.RespostaDeFora{Corpo: `[{"data":"01/10/2026"}]`}, erro: true},
		{nome: "valor que não é texto", resposta: testeapoio.RespostaDeFora{Corpo: `[{"valor":15}]`}, erro: true},
		{nome: "um objeto no lugar da lista", resposta: testeapoio.RespostaDeFora{Corpo: `{"valor":"15.00"}`}, erro: true},
		{nome: "404", resposta: testeapoio.RespostaDeFora{Status: http.StatusNotFound}, erro: true},
		{nome: "erro do servidor", resposta: testeapoio.ErroDoServidor, erro: true},
		{nome: "JSON quebrado", resposta: testeapoio.JSONQuebrado, erro: true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			cliente, srv := clienteDeTeste(t)
			srv.Em("GET", "/dados/serie/bcdata.sgs.432/dados/ultimos/1", c.resposta)
			meta, err := cliente.BuscarMeta(t.Context())
			if (err != nil) != c.erro {
				t.Fatalf("erro %v, esperado %v", err, c.erro)
			}
			obtido := ""
			if meta != nil {
				obtido = *meta
			}
			if obtido != c.esperado {
				t.Errorf("meta %q, esperado %q", obtido, c.esperado)
			}
			if uri := srv.URIs()[0]; !strings.HasSuffix(uri, "?formato=json") {
				t.Errorf("endereço %s", uri)
			}
		})
	}
}

// Regra: a data é a do DateOnly.TryParseExact com "dd/MM/yyyy": dois dígitos em cada parte, a data tem de existir, e o
// time.Parse do Go não pode aceitar sinal no ano nem o ano zero.
func TestLerDataDoBanco(t *testing.T) {
	t.Parallel()
	validas := map[string]string{"31/01/2026": "2026-01-31", "29/02/2024": "2024-02-29", "01/01/0001": "0001-01-01"}
	for texto, esperado := range validas {
		if d, ok := lerDataDoBanco(texto); !ok || d.String() != esperado {
			t.Errorf("lerDataDoBanco(%q) = %s, %v", texto, d, ok)
		}
	}
	for _, texto := range []string{"1/1/2026", "31/02/2026", "29/02/2026", "2026-01-31", "01/01/0000", "01/01/+026", "01/01/-026", "", " 01/01/2026"} {
		if d, ok := lerDataDoBanco(texto); ok {
			t.Errorf("lerDataDoBanco(%q) = %s, esperado recusar", texto, d)
		}
	}
}

// Regra: só HTTPS; a URL base ganha a barra do fim, como o BaseAddress; sem cliente, o tempo limite é o do Program.cs.
func TestNovoCliente(t *testing.T) {
	t.Parallel()
	for _, url := range []string{"http://api.bcb.gov.br/", "://quebrada"} {
		if _, err := NovoCliente(url, nil); err == nil {
			t.Errorf("NovoCliente(%q) aceitou", url)
		}
	}
	c := ClientePadrao()
	if c.base.String() != URLPadrao || c.http.Timeout != 30*time.Second {
		t.Errorf("base %s, tempo limite %s", c.base, c.http.Timeout)
	}
	c, err := NovoCliente("https://exemplo.test/sgs", http.DefaultClient)
	if err != nil || c.base.Path != "/sgs/" || c.http != http.DefaultClient {
		t.Errorf("caminho %q, %v", c.base.Path, err)
	}
}

// Regra: quando o Banco Central não responde, o erro chega a quem chamou, com o motivo.
func TestBancoCentralForaDoAr(t *testing.T) {
	t.Parallel()
	cliente, srv := clienteDeTeste(t)
	srv.Em("GET", "*", testeapoio.TempoEsgotado)
	cliente.http = srv.Cliente()
	cliente.http.Timeout = 50 * time.Millisecond
	_, err := cliente.BuscarTaxas(t.Context(), testeapoio.Dia(2026, 1, 2), testeapoio.Dia(2026, 1, 3))
	var tempo interface{ Timeout() bool }
	if !errors.As(err, &tempo) || !tempo.Timeout() {
		t.Errorf("BuscarTaxas: %v", err)
	}
	if _, err := cliente.BuscarMeta(t.Context()); !errors.As(err, &tempo) || !tempo.Timeout() {
		t.Errorf("BuscarMeta: %v", err)
	}
}

func TestPedidoCancelado(t *testing.T) {
	t.Parallel()
	cliente, _ := clienteDeTeste(t)
	ctx, cancelar := context.WithCancel(t.Context())
	cancelar()
	if _, err := cliente.BuscarMeta(ctx); !errors.Is(err, context.Canceled) {
		t.Error(fmt.Errorf("erro %w", err))
	}
}
