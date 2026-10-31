package carteira

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

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
