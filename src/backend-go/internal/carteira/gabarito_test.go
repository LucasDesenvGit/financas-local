package carteira

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/reservas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/selic"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// gabarito é o testdata/carteira.json, gravado pela sonda das contas (internal/faturas/testdata/sonda-csharp).
type gabarito struct {
	Hoje          []string                     `json:"hoje"`
	Agora         []time.Time                  `json:"agora"`
	InicioDaSelic []string                     `json:"inicioDaSelic"`
	Carteira      []json.RawMessage            `json:"carteira"`
	Operacoes     []map[string]json.RawMessage `json:"operacoes"`
}

func lerGabarito(t *testing.T) gabarito {
	t.Helper()
	dados, err := os.ReadFile("testdata/carteira.json")
	if err != nil {
		t.Fatal(err)
	}
	var g gabarito
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// serieDoGabarito é a mesma série da sonda: os dias úteis desde 02/01/2024, até ontem.
func serieDoGabarito(hoje tipos.Data) *selic.Serie {
	var taxas []selic.Taxa
	for dia := tipos.NovaData(2024, time.January, 2); dia.Antes(hoje); dia = dia.SomarDias(1) {
		if s := time.Date(dia.Ano(), dia.Mes(), dia.Dia(), 0, 0, 0, 0, time.UTC).Weekday(); s == time.Saturday || s == time.Sunday {
			continue
		}
		taxa := testeapoio.Valor("0.052531")
		if dia.Dia()%2 == 0 {
			taxa = testeapoio.Valor("0.040168")
		}
		taxas = append(taxas, selic.Taxa{Data: dia, Taxa: taxa})
	}
	return selic.NovaSerie(taxas)
}

// cenarioDoGabarito aplica o testdata/cenario.sql num banco novo, com o relógio no instante em que a sonda rodou, e
// monta o serviço com as reservas e o câmbio de verdade. A PTAX fica num serviço falso sem nenhuma rota: ir à internet é
// erro do teste.
func cenarioDoGabarito(t *testing.T, g gabarito) (*Servico, *sql.DB, relogio.Relogio) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	cenario, err := os.ReadFile("testdata/cenario.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(cenario)); err != nil {
		t.Fatal(err)
	}
	r := relogio.Fixo{Instante: g.Agora[0]}
	ptax := testeapoio.NovoServicoDeFora(t)
	cliente, err := cambio.NovoClientePTAX(ptax.URL(), ptax.Cliente())
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	c := cambio.NovoServico(db, cliente, r, log)
	return NovoServico(db, c, reservas.NovoServico(db, c, r, log), nil, r, log), db, r
}

// Regra: a visão completa é a do CarteiraService do C# sobre o mesmo cenário: posições, totais, evolução, Open Finance,
// distribuição, reservas e cotas diferentes, número a número e com as mesmas casas.
func TestMontarIgualAoCSharp(t *testing.T) {
	g := lerGabarito(t)
	s, _, r := cenarioDoGabarito(t, g)
	meta := testeapoio.Valor("15.00")
	carteira, err := s.Montar(t.Context(), serieDoGabarito(relogio.Hoje(r)), &meta)
	if err != nil {
		t.Fatal(err)
	}
	obtido, err := json.Marshal(carteira)
	if err != nil {
		t.Fatal(err)
	}
	testeapoio.ConferirJSON(t, obtido, string(g.Carteira[0]))
}

// Regra: as operações de cada ativo são as do OperacoesAsync do C#, e o ativo que não existe é nil.
func TestOperacoesIgualAoCSharp(t *testing.T) {
	g := lerGabarito(t)
	s, _, r := cenarioDoGabarito(t, g)
	serie := serieDoGabarito(relogio.Hoje(r))
	for id, esperado := range g.Operacoes[0] {
		var numero int64
		if err := json.Unmarshal([]byte(id), &numero); err != nil {
			t.Fatal(err)
		}
		lista, err := s.Operacoes(t.Context(), numero, serie)
		if err != nil {
			t.Fatal(err)
		}
		obtido, err := json.Marshal(lista)
		if err != nil {
			t.Fatal(err)
		}
		testeapoio.ConferirJSON(t, obtido, string(esperado))
	}
}

// Regra: a Selic começa na aplicação mais antiga da conexão ativa, como no C#.
func TestInicioDaSerieIgualAoCSharp(t *testing.T) {
	g := lerGabarito(t)
	s, _, _ := cenarioDoGabarito(t, g)
	inicio, err := s.InicioDaSerieSelic(t.Context())
	if err != nil || inicio.String() != g.InicioDaSelic[0] {
		t.Errorf("início %s, %v; o C# dá %s", inicio, err, g.InicioDaSelic[0])
	}
}

// Regra: o preço médio igual ao do C# em 60 sequências sorteadas de compras e vendas, com e sem a data limite, número a
// número e com as mesmas casas: a divisão, a multiplicação e o arredondamento nos 96 bits do decimal.
func TestPosicaoIgualAoCSharp(t *testing.T) {
	t.Parallel()
	dados, err := os.ReadFile("testdata/posicoes.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Sequencias []struct {
			Ate       *string `json:"ate"`
			Operacoes []struct {
				Id            int64  `json:"id"`
				Tipo          int64  `json:"tipo"`
				Data          string `json:"data"`
				Quantidade    string `json:"quantidade"`
				PrecoUnitario string `json:"precoUnitario"`
				Taxas         string `json:"taxas"`
				Cambio        string `json:"cambio"`
			} `json:"operacoes"`
			Quantidade        string `json:"quantidade"`
			Custo             string `json:"custo"`
			CustoNaMoeda      string `json:"custoNaMoeda"`
			LucroRealizado    string `json:"lucroRealizado"`
			PrecoMedio        string `json:"precoMedio"`
			PrecoMedioNaMoeda string `json:"precoMedioNaMoeda"`
		} `json:"sequencias"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Sequencias) < 60 {
		t.Fatal("gabarito pequeno: a sonda mudou?")
	}
	lerData := func(texto string) tipos.Data {
		d, err := tipos.LerData(texto)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	for i, seq := range g.Sequencias {
		var operacoes []Operacao
		for _, o := range seq.Operacoes {
			operacoes = append(operacoes, Operacao{Id: o.Id, Tipo: o.Tipo, Data: lerData(o.Data), Quantidade: testeapoio.Valor(o.Quantidade),
				PrecoUnitario: testeapoio.Valor(o.PrecoUnitario), Taxas: testeapoio.Valor(o.Taxas), Cambio: testeapoio.Valor(o.Cambio)})
		}
		var ate *tipos.Data
		if seq.Ate != nil {
			d := lerData(*seq.Ate)
			ate = &d
		}
		p, err := Calcular(operacoes, ate)
		if err != nil {
			t.Fatalf("sequência %d: %v", i, err)
		}
		conferir := map[string][2]string{
			"quantidade": {p.Quantidade.String(), seq.Quantidade}, "custo": {p.Custo.String(), seq.Custo},
			"custo na moeda": {p.CustoNaMoeda.String(), seq.CustoNaMoeda}, "lucro": {p.LucroRealizado.String(), seq.LucroRealizado},
			"preço médio": {p.PrecoMedio.String(), seq.PrecoMedio}, "preço médio na moeda": {p.PrecoMedioNaMoeda.String(), seq.PrecoMedioNaMoeda},
		}
		for nome, c := range conferir {
			if c[0] != c[1] {
				t.Errorf("sequência %d, %s: %s; o C# dá %s", i, nome, c[0], c[1])
			}
		}
	}
}

// Regra: os textos são os do C#: os tickers da B3 e do exterior, o segmento sugerido e o nome de exibição.
func TestTextoIgualAoCSharp(t *testing.T) {
	t.Parallel()
	dados, err := os.ReadFile("testdata/texto.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Tickers []struct {
			Ticker   *string `json:"ticker"`
			B3       *string `json:"b3"`
			Exterior *string `json:"exterior"`
			Real     *string `json:"real"`
			Dolar    *string `json:"dolar"`
		} `json:"tickers"`
		Segmentos []struct {
			Ticker   string  `json:"ticker"`
			Segmento *string `json:"segmento"`
		} `json:"segmentos"`
		Nomes []struct {
			Nome          string  `json:"Nome"`
			Tipo          string  `json:"Tipo"`
			Emissor       *string `json:"Emissor"`
			Personalizado *string `json:"Personalizado"`
			Exibicao      string  `json:"exibicao"`
		} `json:"nomes"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Tickers) < 30 || len(g.Segmentos) < 6 || len(g.Nomes) < 18 {
		t.Fatal("gabarito pequeno: a sonda mudou?")
	}
	texto := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}
	for _, c := range g.Tickers {
		conferir := map[string][2]*string{"b3": {NormalizarTicker(c.Ticker), c.B3}, "exterior": {NormalizarTickerExterior(c.Ticker), c.Exterior},
			"real": {NormalizarTickerDaMoeda(c.Ticker, "BRL"), c.Real}, "dólar": {NormalizarTickerDaMoeda(c.Ticker, "USD"), c.Dolar}}
		for nome, r := range conferir {
			if texto(r[0]) != texto(r[1]) {
				t.Errorf("%s(%q) = %s; o C# dá %s", nome, texto(c.Ticker), texto(r[0]), texto(r[1]))
			}
		}
	}
	for _, c := range g.Segmentos {
		if s := sugerirSegmento(c.Ticker); texto(s) != texto(c.Segmento) {
			t.Errorf("sugerirSegmento(%q) = %s; o C# dá %s", c.Ticker, texto(s), texto(c.Segmento))
		}
	}
	for _, c := range g.Nomes {
		if n := nomeDeExibicao(c.Nome, c.Tipo, c.Emissor, c.Personalizado); n != c.Exibicao {
			t.Errorf("nomeDeExibicao(%q, %q, %s, %s) = %q; o C# dá %q", c.Nome, c.Tipo, texto(c.Emissor), texto(c.Personalizado), n, c.Exibicao)
		}
	}
}
