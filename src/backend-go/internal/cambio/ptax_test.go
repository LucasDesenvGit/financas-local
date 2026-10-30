package cambio

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// As respostas em testdata/ foram gravadas da PTAX de verdade (olinda.bcb.gov.br), em 24/10/2026:
//   - moedas.json: a lista de moedas;
//   - usd-08-a-14-10-2026.json: os fechamentos do dólar de 08 a 14/10/2026 (10 e 11 são fim de semana; 12, feriado);
//   - usd-fim-de-semana-e-feriado.json: de 10 a 12/10, sem nenhum boletim ("value": []);
//   - usd-todos-os-boletins.json: 13 e 14/10 sem o $filter (Abertura, Intermediário e Fechamento).

// ptaxGravada é a PTAX num serviço falso (testeapoio.ServicoDeFora, #460) que dá a mesma resposta a todo pedido e
// confere os cabeçalhos do .NET. A URL base tem o mesmo caminho da de produção, para conferir que o endereço montado é
// o mesmo do .NET.
func ptaxGravada(t *testing.T, resposta testeapoio.RespostaDeFora) (*ClientePTAX, *testeapoio.ServicoDeFora) {
	t.Helper()
	ptax := testeapoio.NovoServicoDeFora(t)
	ptax.Exigir("User-Agent", "FinancasLocal/1.0")
	ptax.Exigir("Accept", "application/json")
	ptax.Em("GET", "*", resposta)
	c, err := NovoClientePTAX(ptax.URL()+"/olinda/servico/PTAX/versao/v1/odata/", ptax.Cliente())
	if err != nil {
		t.Fatal(err)
	}
	return c, ptax
}

// arquivo é a resposta gravada em testdata/, com o Content-Type da PTAX.
func arquivo(nome string) testeapoio.RespostaDeFora {
	return testeapoio.RespostaDeFora{Arquivo: nome, TipoConteudo: "application/json;charset=UTF-8"}
}

func TestListarMoedasDaPTAX(t *testing.T) {
	t.Parallel()
	c, ptax := ptaxGravada(t, arquivo("moedas.json"))
	moedas, err := c.ListarMoedas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ptax.URIs()[0] != "/olinda/servico/PTAX/versao/v1/odata/Moedas?$format=json" {
		t.Errorf("endereço %s", ptax.URIs()[0])
	}
	codigos := []string{}
	for _, m := range moedas {
		codigos = append(codigos, m.Codigo)
	}
	if strings.Join(codigos, ",") != "AUD,CAD,CHF,DKK,EUR,GBP,JPY,NOK,SEK,USD" {
		t.Errorf("moedas %v", codigos)
	}
	if moedas[0].Nome != "Dólar australiano" {
		t.Errorf("nome %q (acento perdido?)", moedas[0].Nome)
	}
}

func TestBuscarFechamentosDaPTAX(t *testing.T) {
	t.Parallel()
	dia := func(d int) tipos.Data { return tipos.NovaData(2026, 10, d) }
	casos := []struct {
		nome, arquivo string
		de, ate       tipos.Data
		esperado      []string // "data compra venda", com a escala lida ("5.01130")
	}{
		{"semana com fim de semana e feriado", "usd-08-a-14-10-2026.json", dia(8), dia(14), []string{
			"2026-10-08 5.01130 5.01190", "2026-10-09 4.98860 4.98920", "2026-10-13 5.00900 5.00960", "2026-10-14 5.04350 5.04410",
		}},
		{"só fim de semana e feriado: nenhum boletim", "usd-fim-de-semana-e-feriado.json", dia(10), dia(12), []string{}},
		{"API sem o $filter: só o Fechamento vale", "usd-todos-os-boletins.json", dia(13), dia(14), []string{
			"2026-10-13 5.00900 5.00960", "2026-10-14 5.04350 5.04410",
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			cliente, ptax := ptaxGravada(t, arquivo(c.arquivo))
			fechamentos, err := cliente.BuscarFechamentos(context.Background(), "USD", c.de, c.ate)
			if err != nil {
				t.Fatal(err)
			}
			obtido := []string{}
			for _, f := range fechamentos {
				obtido = append(obtido, f.Data.String()+" "+f.Compra.String()+" "+f.Venda.String())
			}
			if strings.Join(obtido, "|") != strings.Join(c.esperado, "|") {
				t.Errorf("obtido %v\nesperado %v", obtido, c.esperado)
			}
			// O mesmo endereço do CambioService.BaixarAsync, com as datas em MM-dd-yyyy.
			esperado := "/olinda/servico/PTAX/versao/v1/odata/CotacaoMoedaPeriodo(moeda=@moeda,dataInicial=@dataInicial,dataFinalCotacao=@dataFinalCotacao)" +
				"?@moeda='USD'&@dataInicial='" + mesDiaAno(c.de) + "'&@dataFinalCotacao='" + mesDiaAno(c.ate) + "'&$filter=tipoBoletim%20eq%20'Fechamento'&$format=json"
			if ptax.URIs()[0] != esperado {
				t.Errorf("endereço\n%s\nesperado\n%s", ptax.URIs()[0], esperado)
			}
		})
	}
}

func TestBuscarFechamentosIgnoraOQueOdotnetIgnora(t *testing.T) {
	t.Parallel()
	// Cada linha ruim tem um motivo do BaixarAsync para ser pulada; só a última vale.
	corpo := `{"value":[
		{"tipoBoletim":"Fechamento","dataHoraCotacao":"2026-10","cotacaoCompra":5.1,"cotacaoVenda":5.2},
		{"tipoBoletim":"Fechamento","dataHoraCotacao":"2026-13-40 13:00:00","cotacaoCompra":5.1,"cotacaoVenda":5.2},
		{"tipoBoletim":"Fechamento","dataHoraCotacao":"2026-10-14 13:00:00","cotacaoCompra":"5.1","cotacaoVenda":5.2},
		{"tipoBoletim":"Fechamento","dataHoraCotacao":"2026-10-14 13:00:00","cotacaoCompra":0,"cotacaoVenda":5.2},
		{"tipoBoletim":"Fechamento","dataHoraCotacao":"2026-10-14 13:00:00","cotacaoCompra":-1,"cotacaoVenda":5.2},
		{"tipoBoletim":"Abertura","dataHoraCotacao":"2026-10-14 10:00:00","cotacaoCompra":5.1,"cotacaoVenda":5.2},
		{"tipoBoletim":"Fechamento","dataHoraCotacao":"2026-10-15 13:00:00","cotacaoCompra":5.10,"cotacaoVenda":5.20}
	]}`
	c, _ := ptaxGravada(t, testeapoio.RespostaDeFora{Corpo: corpo})
	fechamentos, err := c.BuscarFechamentos(context.Background(), "USD", tipos.NovaData(2026, 10, 1), tipos.NovaData(2026, 10, 15))
	if err != nil {
		t.Fatal(err)
	}
	if len(fechamentos) != 1 || fechamentos[0].Data.String() != "2026-10-15" || fechamentos[0].Compra.String() != "5.10" {
		t.Errorf("fechamentos %+v", fechamentos)
	}
}

func TestPTAXComFalha(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		resposta testeapoio.RespostaDeFora
		vazio    bool // sem erro, lista vazia
	}{
		{"erro 500", testeapoio.ErroDoServidor, false},
		{"não é JSON", testeapoio.RespostaDeFora{Corpo: "<html>manutenção</html>", TipoConteudo: "text/html"}, false},
		{"JSON quebrado", testeapoio.JSONQuebrado, false},
		{"JSON sem a lista value", testeapoio.RespostaDeFora{Corpo: `{"erro":1}`}, true},
		{"value que não é lista", testeapoio.RespostaDeFora{Corpo: `{"value":{}}`}, true},
		{"página vazia", testeapoio.PaginaVazia("value"), true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			cliente, _ := ptaxGravada(t, c.resposta)
			moedas, err := cliente.ListarMoedas(context.Background())
			if c.vazio {
				if err != nil || len(moedas) != 0 {
					t.Errorf("esperado lista vazia sem erro, veio %v, %v", moedas, err)
				}
				return
			}
			if err == nil {
				t.Errorf("esperado erro, veio %v", moedas)
			}
		})
	}
}

func TestPTAXComTempoEsgotado(t *testing.T) {
	t.Parallel()
	// O servidor só responde quando o cliente desiste: o tempo limite do cliente (aqui, curto) é que encerra.
	ptax := testeapoio.NovoServicoDeFora(t)
	ptax.Em("GET", "*", testeapoio.TempoEsgotado)
	cliente := ptax.Cliente()
	cliente.Timeout = 50 * time.Millisecond
	c, err := NovoClientePTAX(ptax.URL(), cliente)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListarMoedas(context.Background()); err == nil || !strings.Contains(err.Error(), "consultar a PTAX") {
		t.Errorf("esperado erro de tempo esgotado, veio %v", err)
	}
}

func TestClientePTAXSoHTTPS(t *testing.T) {
	t.Parallel()
	if _, err := NovoClientePTAX("http://olinda.bcb.gov.br/", nil); err == nil {
		t.Error("HTTP sem S aceito")
	}
	c, err := NovoClientePTAX(URLPadraoPTAX, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.http.Timeout != 30*time.Second {
		t.Errorf("tempo limite %v, o .NET usa 30 s", c.http.Timeout)
	}
}
