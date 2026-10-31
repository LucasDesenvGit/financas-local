package carteira

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Regra: o painel dos FIIs é o do FiiService do C#, sobre o mesmo cenário (testdata/fiis.json): a renda de cada fundo, as
// compras comparadas com hoje, os proventos por mês, a evolução contra Selic e IFIX, o crédito ligado ao fundo, os 180 meses,
// a base do IFIX, o piso do retorno e a carteira sem fundos.
func TestFiisIgualAoCSharp(t *testing.T) {
	dados, err := os.ReadFile("testdata/fiis.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Hoje   []string `json:"hoje"`
		Passos []struct {
			Nome      string          `json:"nome"`
			Resultado json.RawMessage `json:"resultado"`
		} `json:"passos"`
	}
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Passos) < 6 {
		t.Fatalf("%d passos: a sonda mudou?", len(g.Passos))
	}
	db := testeapoio.NovoBancoVazio(t)
	cenario, err := os.ReadFile("testdata/fiis-cenario.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(cenario)); err != nil {
		t.Fatal(err)
	}
	hoje, err := tipos.LerData(g.Hoje[0])
	if err != nil {
		t.Fatal(err)
	}
	r := testeapoio.HojeEmBrasilia(hoje.Ano(), hoje.Mes(), hoje.Dia())
	s := NovoServico(db, nil, nil, nil, cotacoesFalsas{}, cofreFalso{}, r, slog.New(slog.DiscardHandler))
	serie := serieDoGabarito(relogio.Hoje(r))
	for _, p := range g.Passos {
		if sql, ok := strings.CutPrefix(p.Nome, "mudar: "); ok {
			if _, err := db.Exec(sql); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if !t.Run(p.Nome, func(t *testing.T) {
			painel, err := s.MontarFiis(t.Context(), serie)
			if err != nil {
				t.Fatal(err)
			}
			obtido, err := json.Marshal(painel)
			if err != nil {
				t.Fatal(err)
			}
			testeapoio.ConferirJSON(t, obtido, string(p.Resultado))
		}) {
			t.FailNow() // cada passo parte do banco do anterior
		}
	}
}

// cenaDosFiis é a cena com o banco de testdata/fiis-cenario.sql, o mesmo da sonda.
func cenaDosFiis(t *testing.T, sf selicFalsa) cena {
	t.Helper()
	c := novaCena(t, cambioFalso{}, reservasFalsas{}, sf)
	cenario, err := os.ReadFile("testdata/fiis-cenario.sql")
	if err != nil {
		t.Fatal(err)
	}
	executar(t, c, string(cenario))
	return c
}

// Regra: a rota dos FIIs pede a Selic desde a primeira operação de FII, ou desde hoje se não houver nenhuma; a operação de
// ação não conta, e a falha da Selic é o 500.
func TestRotaDosFiis(t *testing.T) {
	t.Parallel()
	const rota = "/api/carteira/fiis"
	t.Run("sem FII", func(t *testing.T) {
		t.Parallel()
		var inicios []string
		c := novaCena(t, cambioFalso{}, reservasFalsas{}, selicFalsa{inicios: &inicios})
		c.fab.OperacaoAtivo(c.fab.Ativo(func(a *testeapoio.Ativo) { a.Ticker = "PETR4" }))
		r := testeapoio.Chamar(t, c.mux, "GET", rota, "")
		r.ConferirStatus(t, http.StatusOK)
		if len(inicios) != 1 || inicios[0] != "2026-10-30" {
			t.Errorf("inícios %v", inicios)
		}
		for _, trecho := range []string{`"fiis":[]`, `"lotes":[]`, `"evolucao":[]`, `"rentabilidade":[]`, `"segmentos":[]`, `"historicos":[]`, `"mesAMes":[]`, `"rentabilidadeTotal":null`, `"quantidadeFiis":0`} {
			if !strings.Contains(string(r.Corpo), trecho) {
				t.Errorf("sem %s em %s", trecho, r.Corpo)
			}
		}
	})
	t.Run("com FII", func(t *testing.T) {
		t.Parallel()
		var inicios []string
		c := cenaDosFiis(t, selicFalsa{inicios: &inicios})
		testeapoio.Chamar(t, c.mux, "GET", rota, "").ConferirStatus(t, http.StatusOK)
		if len(inicios) != 1 || inicios[0] != "2008-03-03" {
			t.Errorf("inícios %v", inicios)
		}
	})
	t.Run("sem a Selic", func(t *testing.T) {
		t.Parallel()
		c := cenaDosFiis(t, selicFalsa{erro: errors.New("falhou")})
		testeapoio.Chamar(t, c.mux, "GET", rota, "").ConferirStatus(t, http.StatusInternalServerError)
	})
}

// Regra: o banco falhar em qualquer leitura dos FIIs é o 500, e não um painel pela metade.
func TestFiisComErroDoBanco(t *testing.T) {
	t.Parallel()
	casos := []struct{ nome, comando string }{
		{"as operações", `ALTER TABLE OperacaoAtivo RENAME TO Outra`},
		{"os ativos", `ALTER TABLE Ativo RENAME TO Outro`},
		{"o extrato", `ALTER TABLE Transacao RENAME TO Outra`},
		{"os fechamentos", `ALTER TABLE PrecoMensalAtivo RENAME TO Outra`},
		{"o valor do fechamento", `UPDATE PrecoMensalAtivo SET Fechamento = 'abc'`},
		{"o IFIX", `ALTER TABLE IndiceMensal RENAME TO Outro`},
		{"o valor do IFIX", `UPDATE IndiceMensal SET Fechamento = 'abc'`},
		{"o valor da operação", `UPDATE OperacaoAtivo SET Taxas = 'abc' WHERE Id = 1`},
		{"o crédito da corretora", `UPDATE Transacao SET Valor = 'abc' WHERE Id = 202`},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			c := cenaDosFiis(t, selicFalsa{})
			executar(t, c, x.comando)
			testeapoio.Chamar(t, c.mux, "GET", "/api/carteira/fiis", "").ConferirStatus(t, http.StatusInternalServerError)
		})
	}
}

// Regra: o provento de um crédito é o livre que dá o valor mais perto, até a tolerância, com a data-com até 40 dias antes; no
// empate, o de data mais perto; e o primeiro, se a data também empatar.
func TestMelhorProvento(t *testing.T) {
	t.Parallel()
	dia := func(mes time.Month, d int) tipos.Data { return tipos.NovaData(2025, mes, d) }
	onze, perto, longe := testeapoio.Valor("11.0"), testeapoio.Valor("11.02"), testeapoio.Valor("11.06")
	b := "BBBB11"
	casos := []struct {
		nome      string
		esperados []proventoEsperado
		usados    map[int]bool
		data      tipos.Data
		ticker    *string
		achado    int
	}{
		{"sem nenhum", nil, nil, dia(4, 10), nil, -1},
		{"exato", []proventoEsperado{{"AAAA11", dia(3, 31), onze}}, nil, dia(4, 10), nil, 0},
		{"na tolerância", []proventoEsperado{{"AAAA11", dia(3, 31), perto}}, nil, dia(4, 10), nil, 0},
		{"fora da tolerância", []proventoEsperado{{"AAAA11", dia(3, 31), longe}}, nil, dia(4, 10), nil, -1},
		{"antes da data-com", []proventoEsperado{{"AAAA11", dia(3, 31), onze}}, nil, dia(3, 30), nil, -1},
		{"no dia da data-com", []proventoEsperado{{"AAAA11", dia(3, 31), onze}}, nil, dia(3, 31), nil, 0},
		{"no 40º dia", []proventoEsperado{{"AAAA11", dia(3, 31), onze}}, nil, dia(5, 10), nil, 0},
		{"no 41º dia", []proventoEsperado{{"AAAA11", dia(3, 31), onze}}, nil, dia(5, 11), nil, -1},
		{"o primeiro é melhor que o seguinte", []proventoEsperado{{"AAAA11", dia(3, 31), onze}, {"AAAA11", dia(4, 2), perto}}, nil, dia(4, 10), nil, 0},
		{"o seguinte é melhor que o primeiro", []proventoEsperado{{"AAAA11", dia(3, 31), perto}, {"AAAA11", dia(4, 2), onze}}, nil, dia(4, 10), nil, 1},
		{"valor mais perto vale mais que a data", []proventoEsperado{{"AAAA11", dia(3, 31), onze}, {"BBBB11", dia(4, 30), perto}}, nil, dia(5, 5), nil, 0},
		{"no empate, a data mais perto", []proventoEsperado{{"AAAA11", dia(3, 31), onze}, {"BBBB11", dia(4, 30), onze}}, nil, dia(5, 5), nil, 1},
		{"no empate, sem trocar pela data mais longe", []proventoEsperado{{"AAAA11", dia(4, 30), onze}, {"BBBB11", dia(3, 31), onze}}, nil, dia(5, 5), nil, 0},
		{"tudo igual, o primeiro", []proventoEsperado{{"AAAA11", dia(3, 31), onze}, {"BBBB11", dia(3, 31), onze}}, nil, dia(4, 10), nil, 0},
		{"só o do fundo", []proventoEsperado{{"AAAA11", dia(3, 31), onze}, {"BBBB11", dia(3, 31), onze}}, nil, dia(4, 10), &b, 1},
		{"o usado fica de fora", []proventoEsperado{{"AAAA11", dia(3, 31), onze}, {"BBBB11", dia(3, 31), onze}}, map[int]bool{0: true}, dia(4, 10), nil, 1},
	}
	for _, x := range casos {
		t.Run(x.nome, func(t *testing.T) {
			t.Parallel()
			l := ligacaoDeProventos{x.esperados, x.usados}
			if i := l.melhor(x.data, onze, x.ticker); i != x.achado {
				t.Errorf("melhor = %d, esperado %d", i, x.achado)
			}
		})
	}
}

// Regra: o crédito com o código guarda o provento do fundo, mesmo sendo o primeiro da lista; o sem código fica com o que
// sobrou ou sem fundo, e cada provento serve a um crédito só.
func TestFundoDeCadaCredito(t *testing.T) {
	t.Parallel()
	dia := func(d int) tipos.Data { return tipos.NovaData(2026, 5, d) }
	dez := testeapoio.Valor("10.0")
	f := &fundo{ativo: &Ativo{Ticker: "AAAA11"}, operacoes: []Operacao{{Tipo: operacaoCompra, Data: dia(1), Quantidade: dez, PrecoUnitario: dez, Taxas: tipos.Zero, Cambio: um}},
		proventos: []Provento{{dia(2), testeapoio.Valor("1.0")}, {dia(3), testeapoio.Valor("0.0")}}}
	fundos := []*fundo{f}
	var c conta
	semCodigo := c.fundoDeCadaCredito(fundos, nil, []CreditoRecebido{{1, dia(5), dez}, {2, dia(6), dez}})
	if semCodigo[1] == nil || *semCodigo[1] != "AAAA11" || semCodigo[2] != nil {
		t.Errorf("sem código: %v, %v", semCodigo[1], semCodigo[2])
	}
	comCodigo := c.fundoDeCadaCredito(fundos, []ProventoRecebido{{Id: 1, Ticker: "AAAA11", Data: dia(5), Valor: dez}},
		[]CreditoRecebido{{1, dia(5), dez}, {2, dia(6), dez}})
	if comCodigo[1] == nil || *comCodigo[1] != "AAAA11" || comCodigo[2] != nil {
		t.Errorf("com código: %v, %v", comCodigo[1], comCodigo[2])
	}
	if c.err != nil {
		t.Error(c.err)
	}
}
