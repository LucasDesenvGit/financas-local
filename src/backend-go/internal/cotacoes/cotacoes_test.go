package cotacoes

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/segredos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

type cena struct {
	s     *Servico
	db    *sql.DB
	fab   *testeapoio.Fabrica
	srv   *testeapoio.ServicoDeFora
	saida *bytes.Buffer
}

// novaCena é o serviço num banco vazio, hoje 30/10/2026 às 15h em Brasília, com o Yahoo e a brapi no serviço falso, onde
// o que não tem rota é 404.
func novaCena(t *testing.T) cena {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.AgoraEmBrasilia(2026, 10, 30, 15, 0)
	srv := testeapoio.NovoServicoDeFora(t, "Authorization")
	srv.Em("GET", "*", testeapoio.RespostaDeFora{Status: http.StatusNotFound, Corpo: "{}"})
	cliente, err := NovoCliente(srv.URL()+"/yahoo", srv.URL()+"/brapi", srv.Cliente())
	if err != nil {
		t.Fatal(err)
	}
	var saida bytes.Buffer
	s := NovoServico(db, cliente, semHistorico{}, cofreFalso{}, r, slog.New(slog.NewTextHandler(&saida, nil)))
	s.esperar = func(context.Context, time.Duration) error { return nil }
	return cena{s: s, db: db, fab: testeapoio.NovaFabrica(t, db, r), srv: srv, saida: &saida}
}

const precoDez = `{"chart":{"result":[{"meta":{"regularMarketPrice":10}}]}}`

// Regra: o que o CotacaoService faz com cada resposta estranha do Yahoo e da brapi (testdata/formatos-gabarito.json): o
// que vira falha, o que vira aviso (as exceções do C#), o que passa sem nada, e o erro na segunda consulta, que perde o
// preço da primeira.
func TestFormatosIguaisAoCSharp(t *testing.T) {
	bruto, err := os.ReadFile("testdata/formatos.json")
	if err != nil {
		t.Fatal(err)
	}
	var casos []struct {
		Nome      string `json:"nome"`
		Dia       string `json:"dia"`
		Mes       string `json:"mes"`
		Brapi     string `json:"brapi"`
		StatusDia int    `json:"statusDia"`
		StatusMes int    `json:"statusMes"`
	}
	if err := json.Unmarshal(bruto, &casos); err != nil {
		t.Fatal(err)
	}
	bruto, err = os.ReadFile("testdata/formatos-gabarito.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Casos []struct {
			Nome      string `json:"nome"`
			Resultado struct {
				Atualizados int `json:"atualizados"`
				Falhas      int `json:"falhas"`
			} `json:"resultado"`
			Avisos    int             `json:"avisos"`
			Ativo     json.RawMessage `json:"ativo"`
			Proventos json.RawMessage `json:"proventos"`
			Mensais   json.RawMessage `json:"mensais"`
		} `json:"casos"`
	}
	if err := json.Unmarshal(bruto, &g); err != nil {
		t.Fatal(err)
	}
	if len(casos) != len(g.Casos) || len(casos) < 100 {
		t.Fatalf("%d casos, %d no gabarito", len(casos), len(g.Casos))
	}
	c := novaCena(t)
	if _, err := c.db.Exec(`INSERT INTO Configuracao (Chave, Valor, Protegido, AtualizadoEm) VALUES (?, 'token', 1, '2026-01-01 00:00:00')`, ChaveTokenBrapi); err != nil {
		t.Fatal(err)
	}
	vazioOu := func(s string) string {
		if s == "" {
			return "{}"
		}
		return s
	}
	for i, x := range casos {
		esperado := g.Casos[i]
		if _, err := c.db.Exec(`DELETE FROM ProventoAtivo; DELETE FROM PrecoMensalAtivo; DELETE FROM Ativo;
			INSERT INTO Ativo (Id, Ticker, Tipo, Moeda, PrecoManual, CriadoEm) VALUES (1, 'ZZZZ3', 2, 'BRL', 0, '2025-01-01 00:00:00')`); err != nil {
			t.Fatal(err)
		}
		c.srv.Em("GET", "/yahoo/v8/finance/chart/ZZZZ3.SA", testeapoio.RespostaDeFora{Status: x.StatusDia, Corpo: vazioOu(x.Dia)},
			testeapoio.RespostaDeFora{Status: x.StatusMes, Corpo: vazioOu(x.Mes)})
		brapi := testeapoio.RespostaDeFora{Status: http.StatusNotFound, Corpo: "{}"}
		if x.Brapi != "" {
			brapi = testeapoio.RespostaDeFora{Corpo: x.Brapi}
		}
		c.srv.Em("GET", "/brapi/api/quote/ZZZZ3", brapi)
		c.saida.Reset()
		r, err := c.s.Atualizar(t.Context(), nil)
		if err != nil {
			t.Fatalf("%s: %v", x.Nome, err)
		}
		if r.Atualizados != esperado.Resultado.Atualizados || len(r.Falhas) != esperado.Resultado.Falhas {
			t.Errorf("%s: atualizados %d, falhas %d; o C# dá %+v", x.Nome, r.Atualizados, len(r.Falhas), esperado.Resultado)
		}
		if avisos := strings.Count(c.saida.String(), "Falha ao consultar"); avisos != esperado.Avisos {
			t.Errorf("%s: %d avisos, o C# dá %d: %s", x.Nome, avisos, esperado.Avisos, c.saida.String())
		}
		conferirIgual(t, x.Nome+", ativo", linhasDe(t, c.db, `SELECT Nome, PrecoAtual, PrecoAnterior, FonteCotacao FROM Ativo`), esperado.Ativo)
		conferirIgual(t, x.Nome+", proventos", linhasDe(t, c.db, `SELECT DataCom, ValorPorCota FROM ProventoAtivo ORDER BY DataCom`), esperado.Proventos)
		conferirIgual(t, x.Nome+", mensais", linhasDe(t, c.db, `SELECT Mes, Fechamento FROM PrecoMensalAtivo ORDER BY Mes`), esperado.Mensais)
	}
}

// Regra: um número do JSON é o TryGetDecimal: só número, com expoente, e nada fora do decimal; nem o expoente gigante,
// que não pode virar um número de um bilhão de dígitos.
func TestDecimalDoJSON(t *testing.T) {
	t.Parallel()
	casos := []struct {
		cru      string
		esperado string
	}{
		{"10", "10"}, {"10.50", "10.50"}, {"-5", "-5"}, {"0", "0"}, {" 7 ", "7"}, {"1e2", "100"}, {"1.5e2", "150"}, {"1E-2", "0.01"},
		{"1e-400", "0"}, {"1e-999999999", "0"}, {"1e30", ""}, {"8e28", ""}, {"7.99e28", ""}, {"7.9e28", "79000000000000000000000000000"}, {"1e400", ""}, {"1e999999999", ""},
		{`"5"`, ""}, {"null", ""}, {"true", ""}, {"[1]", ""}, {"{}", ""}, {"", ""}, {"+5", ""}, {"/", ""}, {":", ""},
	}
	for _, c := range casos {
		obtido := ""
		if d := decimalDoJSON(json.RawMessage(c.cru)); d != nil {
			obtido = d.String()
		}
		if obtido != c.esperado {
			t.Errorf("decimalDoJSON(%q) = %q, esperado %q", c.cru, obtido, c.esperado)
		}
	}
}

// Regra: o TryGetInt64: só número (o resto é a exceção do C#), inteiro, nos 64 bits.
func TestInteiroDoJSON(t *testing.T) {
	t.Parallel()
	casos := []struct {
		cru     string
		valor   int64
		ok, err bool
	}{
		{cru: "5", valor: 5, ok: true}, {cru: "-5", valor: -5, ok: true}, {cru: "0", ok: true}, {cru: "9", valor: 9, ok: true}, {cru: "1.5"}, {cru: "9223372036854775808"},
		{cru: `"5"`, err: true}, {cru: "null", err: true}, {cru: "/", err: true}, {cru: ":", err: true}, {cru: "", err: true},
	}
	for _, c := range casos {
		n, ok, err := inteiroDoJSON(json.RawMessage(c.cru))
		if n != c.valor || ok != c.ok || (err != nil) != c.err {
			t.Errorf("inteiroDoJSON(%q) = %d, %v, %v", c.cru, n, ok, err)
		}
	}
}

// Regra: o tipo do JSON olha só o começo, depois dos espaços, e o vazio não quebra.
func TestTipoDoJSON(t *testing.T) {
	t.Parallel()
	if tipoDoJSON(nil) != 0 || tipoDoJSON(json.RawMessage(" \t\r\n")) != 0 || tipoDoJSON(json.RawMessage(" \n[1]")) != '[' {
		t.Error("tipoDoJSON")
	}
}

// Regra: só HTTPS; a URL base ganha a barra do fim, como o BaseAddress; o cliente padrão tem os 20 s do Program.cs.
func TestNovoCliente(t *testing.T) {
	t.Parallel()
	if _, err := NovoCliente("http://exemplo.test/", URLPadraoBrapi, nil); err == nil {
		t.Error("Yahoo sem HTTPS")
	}
	if _, err := NovoCliente(URLPadraoYahoo, "http://exemplo.test/", nil); err == nil {
		t.Error("brapi sem HTTPS")
	}
	if _, err := NovoCliente("://quebrada", URLPadraoBrapi, nil); err == nil {
		t.Error("URL quebrada")
	}
	c := ClientePadrao()
	if c.yahoo.String() != URLPadraoYahoo || c.brapi.String() != URLPadraoBrapi || c.http.Timeout != 20*time.Second {
		t.Errorf("%s %s %s", c.yahoo, c.brapi, c.http.Timeout)
	}
	c, err := NovoCliente("https://exemplo.test/y", "https://exemplo.test/b/", http.DefaultClient)
	if err != nil || c.yahoo.Path != "/y/" || c.brapi.Path != "/b/" || c.http != http.DefaultClient {
		t.Errorf("%s %s, %v", c.yahoo.Path, c.brapi.Path, err)
	}
}

// Regra: a faixa do histórico cresce com a primeira operação: 5 anos; de 4 anos atrás, 10 anos; de 9, tudo.
func TestFaixaDoHistorico(t *testing.T) {
	t.Parallel()
	casos := []struct {
		ano   int
		faixa string
	}{{2026, "5y"}, {2023, "5y"}, {2022, "10y"}, {2018, "10y"}, {2017, "max"}, {2000, "max"}}
	for _, x := range casos {
		c := novaCena(t)
		cliente, err := NovoCliente(c.srv.URL(), c.srv.URL(), c.srv.Cliente())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := cliente.Yahoo(t.Context(), "ABCD3.SA", testeapoio.Dia(x.ano, 6, 1), 2026); err != nil {
			t.Fatal(err)
		}
		uris := c.srv.URIs()
		if len(uris) != 2 || uris[1] != "/v8/finance/chart/ABCD3.SA?range="+x.faixa+"&interval=1mo&events=div" {
			t.Errorf("%d: %v", x.ano, uris)
		}
	}
}

// Regra: a data-com é o pregão anterior à data "ex", pulando o fim de semana.
func TestPregaoAnterior(t *testing.T) {
	t.Parallel()
	casos := map[string]string{"2026-03-03": "2026-03-02", "2026-03-02": "2026-02-27", "2026-03-01": "2026-02-27", "2026-02-28": "2026-02-27"}
	for ex, com := range casos {
		d, err := tipos.LerData(ex)
		if err != nil {
			t.Fatal(err)
		}
		if p := pregaoAnterior(d); p.String() != com {
			t.Errorf("pregaoAnterior(%s) = %s", ex, p)
		}
	}
}

func TestEscapar(t *testing.T) {
	t.Parallel()
	for entrada, esperado := range map[string]string{"^IFIX": "%5EIFIX", "SAP.DE": "SAP.DE", "BRK-B": "BRK-B", "A B": "A%20B", "A+B": "A%2BB", "a/b": "a%2Fb"} {
		if e := escapar(entrada); e != esperado {
			t.Errorf("escapar(%q) = %q", entrada, e)
		}
	}
}

// Regra: o token vai só no cabeçalho, para a brapi, e não fica em lugar nenhum; o Yahoo vai com o agente de navegador.
func TestCabecalhosDasFontes(t *testing.T) {
	t.Parallel()
	c := novaCena(t)
	c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "ABCD3" })
	if _, err := c.db.Exec(`INSERT INTO Configuracao (Chave, Valor, Protegido, AtualizadoEm) VALUES (?, 'token-secreto', 1, '2026-01-01 00:00:00')`, ChaveTokenBrapi); err != nil {
		t.Fatal(err)
	}
	if _, err := c.s.Atualizar(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	var yahoo, brapi *testeapoio.Pedido
	pedidos := c.srv.Pedidos()
	for i := range pedidos {
		p := &pedidos[i]
		if strings.HasPrefix(p.Caminho, "/brapi/") {
			brapi = p
		} else if strings.Contains(p.Caminho, "ABCD3") {
			yahoo = p
		}
	}
	if yahoo == nil || brapi == nil {
		t.Fatalf("pedidos %v", pedidos)
	}
	if yahoo.TemSegredo("Authorization") || yahoo.Cabecalhos.Get("User-Agent") != agenteNavegador || yahoo.Cabecalhos.Get("Accept") != "application/json" {
		t.Errorf("Yahoo: %v", yahoo.Cabecalhos)
	}
	if !brapi.TemSegredo("Authorization") || brapi.Cabecalhos.Get("User-Agent") != agenteDoCliente || brapi.URI != "/brapi/api/quote/ABCD3" {
		t.Errorf("brapi: %+v", brapi)
	}
}

// cofreQueFalha só decifra com o erro que o teste escolher.
type cofreQueFalha struct{ erro error }

func (c cofreQueFalha) Cifrar(_, s string) (string, error)   { return s, nil }
func (c cofreQueFalha) Decifrar(_, _ string) (string, error) { return "", c.erro }

// Regra: sem o cofre, ou sem token, ou com ele em branco, não há brapi; o token do .NET, que o Go não lê até a #85, é só
// um aviso; outro erro do cofre para a atualização.
func TestToken(t *testing.T) {
	t.Parallel()
	const quebrado = "quebrou"
	casos := []struct {
		nome  string
		valor *string
		cofre any
		brapi bool
		erro  bool
		aviso bool
	}{
		{nome: "com token", valor: new("abc"), cofre: cofreFalso{}, brapi: true},
		{nome: "sem o cofre", valor: new("abc")},
		{nome: "sem a configuração", cofre: cofreFalso{}},
		{nome: "em branco", valor: new("   "), cofre: cofreFalso{}},
		{nome: "gravado pelo .NET", valor: new("abc"), cofre: cofreQueFalha{segredos.ErrInformarDeNovo}, aviso: true},
		{nome: "cofre com erro", valor: new("abc"), cofre: cofreQueFalha{errors.New(quebrado)}, erro: true},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := novaCena(t)
			c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "ABCD3" })
			c.s.cofre = nil
			if f, ok := x.cofre.(cofreFalso); ok {
				c.s.cofre = f
			}
			if f, ok := x.cofre.(cofreQueFalha); ok {
				c.s.cofre = f
			}
			if x.valor != nil {
				if _, err := c.db.Exec(`INSERT INTO Configuracao (Chave, Valor, Protegido, AtualizadoEm) VALUES (?, ?, 1, '2026-01-01 00:00:00')`, ChaveTokenBrapi, *x.valor); err != nil {
					t.Fatal(err)
				}
			}
			_, err := c.s.Atualizar(t.Context(), nil)
			if (err != nil) != x.erro {
				t.Fatalf("erro %v", err)
			}
			usouBrapi := false
			for _, p := range c.srv.Pedidos() {
				usouBrapi = usouBrapi || strings.HasPrefix(p.Caminho, "/brapi/")
			}
			if usouBrapi != x.brapi || strings.Contains(c.saida.String(), "informado de novo") != x.aviso {
				t.Errorf("brapi %v, log %q", usouBrapi, c.saida.String())
			}
		})
	}
}

// Regra: o nome oficial só corrige o tipo da ação da B3 que parece FII, e só quando o ativo não tinha nome.
func TestTipoCorrigidoPeloNome(t *testing.T) {
	t.Parallel()
	const fundo = `{"chart":{"result":[{"meta":{"regularMarketPrice":10,"longName":"FII Fictício"}}]}}`
	casos := []struct {
		nome         string
		tipo         int64
		moeda        string
		nomeGuardado *string
		esperado     int64
	}{
		{nome: "ação da B3", tipo: 2, moeda: "BRL", esperado: 1},
		{nome: "ETF da B3", tipo: 3, moeda: "BRL", esperado: 3},
		{nome: "ação no exterior", tipo: 2, moeda: "USD", esperado: 2},
		{nome: "já com nome", tipo: 2, moeda: "BRL", nomeGuardado: new("Ação"), esperado: 2},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := novaCena(t)
			a := c.fab.Ativo(func(a *testeapoio.Ativo) {
				a.Ticker, a.Tipo, a.Moeda, a.Nome = "ABCD3", x.tipo, x.moeda, x.nomeGuardado
			})
			c.srv.Em("GET", "*", testeapoio.RespostaDeFora{Corpo: fundo})
			if _, err := c.s.Atualizar(t.Context(), []int64{a.Id}); err != nil {
				t.Fatal(err)
			}
			var tipo int64
			if err := c.db.QueryRow(`SELECT Tipo FROM Ativo WHERE Id = ?`, a.Id).Scan(&tipo); err != nil {
				t.Fatal(err)
			}
			if tipo != x.esperado {
				t.Errorf("tipo %d, esperado %d", tipo, x.esperado)
			}
		})
	}
}

// Regra: só os ativos pedidos entram, e sem nenhum ativo o índice nem é consultado.
func TestSomenteOsAtivosPedidos(t *testing.T) {
	t.Parallel()
	c := novaCena(t)
	um := c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "ABCD3" })
	c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "WXYZ3" })
	if r, err := c.s.Atualizar(t.Context(), []int64{}); err != nil || len(r.Falhas) != 0 || len(c.srv.Pedidos()) != 0 {
		t.Fatalf("nenhum: %+v, %v, %v", r, err, c.srv.URIs())
	}
	r, err := c.s.Atualizar(t.Context(), []int64{um.Id})
	if err != nil || strings.Join(r.Falhas, ",") != "ABCD3" {
		t.Fatalf("%+v, %v", r, err)
	}
	if strings.Contains(strings.Join(c.srv.URIs(), " "), "WXYZ3") {
		t.Errorf("pediu o outro: %v", c.srv.URIs())
	}
}

// Regra: o IFIX é consultado no máximo a cada 12 horas; o primeiro símbolo com dados encerra; sem dados, a hora fica
// gravada do mesmo jeito; o erro vira aviso e não derruba a atualização.
func TestIndice(t *testing.T) {
	t.Parallel()
	const chave = "Indice.IFIX.ConsultadoEm"
	historico := testeapoio.RespostaDeFora{Corpo: `{"chart":{"result":[{"timestamp":[1774962000],"indicators":{"quote":[{"close":[3300]}]}}]}}`}
	casos := []struct {
		nome       string
		consultado string
		primeira   *testeapoio.RespostaDeFora
		pedidos    int
		mensais    int
		erro       bool
	}{
		{nome: "nunca consultado, o primeiro símbolo responde", primeira: &historico, pedidos: 1, mensais: 1},
		{nome: "o segundo símbolo responde", pedidos: 2, mensais: 1},
		{nome: "há 12 horas exatas", consultado: "2026-10-30T06:00:00.0000000Z", pedidos: 2, mensais: 1},
		{nome: "há menos de 12 horas", consultado: "2026-10-30T06:00:01.0000000Z"},
		{nome: "o Yahoo com o JSON quebrado", primeira: &testeapoio.JSONQuebrado, pedidos: 1, erro: true},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := novaCena(t)
			c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "ABCD3" })
			if x.consultado != "" {
				c.fab.Configuracao(chave, x.consultado)
			}
			if x.primeira != nil {
				c.srv.Em("GET", "/yahoo/v8/finance/chart/IFIX.SA", *x.primeira)
			}
			c.srv.Em("GET", "/yahoo/v8/finance/chart/^IFIX", historico)
			if _, err := c.s.Atualizar(t.Context(), nil); err != nil {
				t.Fatal(err)
			}
			pedidos := 0
			for _, u := range c.srv.URIs() {
				if strings.Contains(u, "IFIX") {
					pedidos++
				}
			}
			var mensais int
			if err := c.db.QueryRow(`SELECT COUNT(*) FROM IndiceMensal`).Scan(&mensais); err != nil {
				t.Fatal(err)
			}
			if pedidos != x.pedidos || mensais != x.mensais || strings.Contains(c.saida.String(), "atualizar o IFIX") != x.erro {
				t.Errorf("%d pedidos, %d mensais, log %q", pedidos, mensais, c.saida.String())
			}
		})
	}
}

func TestIndiceSemDadosGuardaAHora(t *testing.T) {
	t.Parallel()
	c := novaCena(t)
	c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "ABCD3" })
	if _, err := c.s.Atualizar(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	var valor string
	if err := c.db.QueryRow(`SELECT Valor FROM Configuracao WHERE Chave = 'Indice.IFIX.ConsultadoEm'`).Scan(&valor); err != nil || valor != "2026-10-30T18:00:00.0000000Z" {
		t.Errorf("%q, %v", valor, err)
	}
}

// Regra: os fechamentos diários: o intervalo com a folga, em segundos UTC; sem a fonte, vazio, com o aviso.
func TestFechamentosDiarios(t *testing.T) {
	t.Parallel()
	c := novaCena(t)
	c.srv.Em("GET", "/yahoo/v8/finance/chart/ABCD3.SA", testeapoio.RespostaDeFora{Corpo: `{"chart":{"result":[{"timestamp":[1774962000],"indicators":{"quote":[{"close":[12.5]}]}}]}}`},
		testeapoio.JSONQuebrado)
	serie, err := c.s.FechamentosDiarios(t.Context(), "ABCD3", testeapoio.Dia(2026, 3, 10), testeapoio.Dia(2026, 3, 20))
	if err != nil || len(serie) != 1 || serie[0].Valor.String() != "12.5" {
		t.Fatalf("%+v, %v", serie, err)
	}
	if uri := c.srv.URIs()[0]; uri != "/yahoo/v8/finance/chart/ABCD3.SA?period1=1772496000&period2=1774137600&interval=1d" {
		t.Errorf("endereço %s", uri)
	}
	c.srv.Em("GET", "/yahoo/v8/finance/chart/SEMN3.SA", testeapoio.RespostaDeFora{Status: http.StatusNotFound, Corpo: "{}"})
	serie, err = c.s.FechamentosDiarios(t.Context(), "SEMN3", testeapoio.Dia(2026, 3, 10), testeapoio.Dia(2026, 3, 20))
	if err != nil || serie != nil || c.saida.Len() != 0 {
		t.Fatalf("ativo que a fonte não tem: %+v, %v, log %q", serie, err, c.saida.String())
	}
	serie, err = c.s.FechamentosDiarios(t.Context(), "ABCD3", testeapoio.Dia(2026, 3, 10), testeapoio.Dia(2026, 3, 20))
	if err != nil || serie != nil || !strings.Contains(c.saida.String(), "histórico de um ativo") {
		t.Errorf("%+v, %v, log %q", serie, err, c.saida.String())
	}
	cancelado, cancelar := context.WithCancel(t.Context())
	cancelar()
	if _, err := c.s.FechamentosDiarios(cancelado, "ABCD3", testeapoio.Dia(2026, 3, 10), testeapoio.Dia(2026, 3, 20)); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelado: %v", err)
	}
}

// historicoQueFalha é o erro do banco no GarantirHistorico: esse não é da fonte, e para tudo.
type historicoQueFalha struct{}

func (historicoQueFalha) GarantirHistorico(context.Context, string, tipos.Data) error {
	return errors.New("banco fora do ar")
}

// Regra: a fonte fora do ar vira falha, mas o erro do banco ou do câmbio, a pausa cancelada e o pedido cancelado param.
func TestErrosQueParam(t *testing.T) {
	t.Parallel()
	t.Run("o câmbio", func(t *testing.T) {
		t.Parallel()
		c := novaCena(t)
		c.fab.Ativo(func(a *testeapoio.Ativo) { a.Moeda = "USD" })
		c.s.cambio = historicoQueFalha{}
		if _, err := c.s.Atualizar(t.Context(), nil); err == nil {
			t.Error("sem erro")
		}
	})
	t.Run("a pausa", func(t *testing.T) {
		t.Parallel()
		c := novaCena(t)
		c.fab.Ativo()
		c.srv.Em("GET", "*", testeapoio.RespostaDeFora{Corpo: precoDez})
		c.s.esperar = func(context.Context, time.Duration) error { return context.Canceled }
		if _, err := c.s.Atualizar(t.Context(), nil); !errors.Is(err, context.Canceled) {
			t.Errorf("erro %v", err)
		}
	})
	t.Run("o banco ao gravar", func(t *testing.T) {
		t.Parallel()
		c := novaCena(t)
		c.fab.Ativo()
		c.srv.Em("GET", "*", testeapoio.RespostaDeFora{Corpo: precoDez})
		if _, err := c.db.Exec(`CREATE TRIGGER nao_grava BEFORE UPDATE ON Ativo BEGIN SELECT RAISE(ABORT, 'não grava'); END`); err != nil {
			t.Fatal(err)
		}
		if _, err := c.s.Atualizar(t.Context(), nil); err == nil {
			t.Error("sem erro")
		}
	})
	t.Run("cancelado antes de começar", func(t *testing.T) {
		t.Parallel()
		c := novaCena(t)
		c.fab.Ativo()
		cancelado, cancelar := context.WithCancel(t.Context())
		cancelar()
		if _, err := c.s.Atualizar(cancelado, nil); !errors.Is(err, context.Canceled) {
			t.Errorf("erro %v", err)
		}
	})
	t.Run("sem as tabelas", func(t *testing.T) {
		t.Parallel()
		c := novaCena(t)
		if _, err := c.db.Exec(`DROP TABLE Ativo`); err != nil {
			t.Fatal(err)
		}
		if _, err := c.s.Atualizar(t.Context(), nil); err == nil {
			t.Error("sem erro")
		}
	})
}

// Regra: a pausa de verdade espera, e o pedido cancelado a interrompe.
func TestEsperarComContexto(t *testing.T) {
	t.Parallel()
	if err := esperarComContexto(t.Context(), 0); err != nil {
		t.Errorf("sem cancelar: %v", err)
	}
	cancelado, cancelar := context.WithCancel(t.Context())
	cancelar()
	if err := esperarComContexto(cancelado, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelado: %v", err)
	}
}
