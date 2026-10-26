package cripto

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

type fonteFalsa struct {
	negocios map[string]Negocio
	err      error
}

func (f *fonteFalsa) UltimosNegocios(context.Context, []string) (map[string]Negocio, error) {
	return f.negocios, f.err
}

func novoServicoDeTeste(t *testing.T, r relogio.Relogio, fonte *fonteFalsa) (*Servico, *sql.DB, *testeapoio.Fabrica, *bytes.Buffer) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	var saida bytes.Buffer
	s := NovoServico(db, fonte, r, slog.New(slog.NewTextHandler(&saida, nil)))
	return s, db, testeapoio.NovaFabrica(t, db, r), &saida
}

func utc(dia, hora, minuto int) time.Time {
	return time.Date(2026, 10, dia, hora, minuto, 0, 0, time.UTC)
}

func contar(t *testing.T, db *sql.DB, consulta string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(consulta, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAtualizar(t *testing.T) {
	t.Parallel()
	agora := relogio.Fixo{Instante: utc(15, 1, 0)}
	todas := map[string]Negocio{
		"BTC":  {Preco: testeapoio.Valor("618900.12000000"), DataHora: utc(14, 21, 0)},
		"ETH":  {Preco: testeapoio.Valor("11594.00000000"), DataHora: utc(14, 21, 0)},
		"SOL":  {Preco: testeapoio.Valor("900.5"), DataHora: utc(14, 21, 1)},
		"USDT": {Preco: testeapoio.Valor("5.4"), DataHora: utc(14, 20, 59)},
	}
	casos := []struct {
		nome     string
		negocios map[string]Negocio
		novas    int
		falhas   string
		marcou   bool
	}{
		{"todas novas: grava e marca", todas, 4, "", true},
		{"faltou uma na resposta: grava as outras, avisa e não marca", map[string]Negocio{"BTC": todas["BTC"], "ETH": todas["ETH"], "SOL": todas["SOL"]},
			3, "USDT", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			s, db, _, saida := novoServicoDeTeste(t, agora, &fonteFalsa{negocios: c.negocios})
			resultado, err := s.Atualizar(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if resultado.Novas != c.novas || strings.Join(resultado.Falhas, ",") != c.falhas {
				t.Errorf("resultado %+v", resultado)
			}
			marcou := contar(t, db, `SELECT COUNT(*) FROM Configuracao WHERE Chave = ?`, ChaveUltimaAtualizacao) == 1
			if marcou != c.marcou {
				t.Errorf("marcou a atualização: %v, esperado %v", marcou, c.marcou)
			}
			if !c.marcou && !strings.Contains(saida.String(), "moedas=USDT") {
				t.Errorf("sem o aviso da moeda que faltou:\n%s", saida.String())
			}
		})
	}
}

func TestAtualizarNoFormatoDoEfCoreSemRepetirEApagaOsVelhos(t *testing.T) {
	t.Parallel()
	agora := relogio.Fixo{Instante: utc(15, 1, 0)}
	negocios := map[string]Negocio{"BTC": {Preco: testeapoio.Valor("618900.12000000"), DataHora: utc(14, 21, 0)}}
	s, db, fab, _ := novoServicoDeTeste(t, agora, &fonteFalsa{negocios: negocios})
	// Uma de 31 dias atrás (sai), uma de 29 (fica) e a mesma do negócio de agora (não grava de novo).
	fab.CotacaoCripto(func(c *testeapoio.CotacaoCripto) { c.DataHora = tipos.NovaDataHora(utc(15, 1, 0).AddDate(0, 0, -31)) })
	fab.CotacaoCripto(func(c *testeapoio.CotacaoCripto) { c.DataHora = tipos.NovaDataHora(utc(15, 1, 0).AddDate(0, 0, -29)) })
	if _, err := s.Atualizar(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Atualizar(t.Context()); err != nil { // o mesmo negócio de novo
		t.Fatal(err)
	}
	var linhas []string
	r, err := db.Query(`SELECT Moeda, DataHora, Preco, typeof(Preco), AtualizadoEm FROM CotacaoCripto ORDER BY DataHora`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for r.Next() {
		var m, d, p, tipo, a string
		if err := r.Scan(&m, &d, &p, &tipo, &a); err != nil {
			t.Fatal(err)
		}
		linhas = append(linhas, strings.Join([]string{m, d, p, tipo, a}, " "))
	}
	esperado := []string{
		"BTC 2026-09-16 01:00:00 612345.67 text 2026-10-15 01:00:00",
		"BTC 2026-10-14 21:00:00 618900.12 text 2026-10-15 01:00:00", // sem os zeros à direita, como o EF Core
	}
	if strings.Join(linhas, "|") != strings.Join(esperado, "|") {
		t.Errorf("linhas\n%s\nesperado\n%s", strings.Join(linhas, "\n"), strings.Join(esperado, "\n"))
	}
}

func TestUltimas(t *testing.T) {
	t.Parallel()
	agora := relogio.Fixo{Instante: utc(15, 1, 0)}
	casos := []struct {
		nome     string
		cotacoes []testeapoio.CotacaoCripto
		esperado string
	}{
		{"variação contra 24 h antes, com a escala do C#", []testeapoio.CotacaoCripto{
			{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(13, 21, 0)), Preco: testeapoio.Valor("612345.67")},
			{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(14, 21, 0)), Preco: testeapoio.Valor("618900.12")},
		}, `[{"moeda":"BTC","preco":618900.12,"dataHora":"2026-10-14T21:00:00Z","variacao24h":1.07,"fonte":"Mercado Bitcoin"}]`},
		// O EF Core grava 110 como "110.0": o C# lê de volta com uma casa, e a conta fica com ela (10.0, e não 10.00).
		{"variação exata fica com a escala natural", []testeapoio.CotacaoCripto{
			{Moeda: "ETH", DataHora: tipos.NovaDataHora(utc(13, 21, 0)), Preco: testeapoio.Valor("100")},
			{Moeda: "ETH", DataHora: tipos.NovaDataHora(utc(14, 21, 0)), Preco: testeapoio.Valor("110")},
		}, `[{"moeda":"ETH","preco":110.0,"dataHora":"2026-10-14T21:00:00Z","variacao24h":10.0,"fonte":"Mercado Bitcoin"}]`},
		{"queda", []testeapoio.CotacaoCripto{
			{Moeda: "SOL", DataHora: tipos.NovaDataHora(utc(13, 21, 0)), Preco: testeapoio.Valor("900")},
			{Moeda: "SOL", DataHora: tipos.NovaDataHora(utc(14, 21, 0)), Preco: testeapoio.Valor("850.5")},
		}, `[{"moeda":"SOL","preco":850.5,"dataHora":"2026-10-14T21:00:00Z","variacao24h":-5.50,"fonte":"Mercado Bitcoin"}]`},
		{"empate entre 23 h e 25 h: fica a mais recente", []testeapoio.CotacaoCripto{
			{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(13, 20, 0)), Preco: testeapoio.Valor("200")}, // 25 h
			{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(13, 22, 0)), Preco: testeapoio.Valor("100")}, // 23 h
			{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(14, 21, 0)), Preco: testeapoio.Valor("150")},
		}, `[{"moeda":"BTC","preco":150.0,"dataHora":"2026-10-14T21:00:00Z","variacao24h":50.0,"fonte":"Mercado Bitcoin"}]`},
		{"fora da janela de 22 a 26 h: sem variação", []testeapoio.CotacaoCripto{
			{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(13, 18, 59)), Preco: testeapoio.Valor("100")}, // 26 h e 1 min
			{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(13, 23, 1)), Preco: testeapoio.Valor("100")},  // 21 h e 59 min
			{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(14, 21, 0)), Preco: testeapoio.Valor("150")},
		}, `[{"moeda":"BTC","preco":150.0,"dataHora":"2026-10-14T21:00:00Z","variacao24h":null,"fonte":"Mercado Bitcoin"}]`},
		{"site desligado há mais de 2 dias: a última, mesmo antiga", []testeapoio.CotacaoCripto{
			{Moeda: "USDT", DataHora: tipos.NovaDataHora(utc(1, 9, 30)), Preco: testeapoio.Valor("5.41")},
		}, `[{"moeda":"USDT","preco":5.41,"dataHora":"2026-10-01T09:30:00Z","variacao24h":null,"fonte":"Mercado Bitcoin"}]`},
		{"na ordem da tela, não a do banco", []testeapoio.CotacaoCripto{
			{Moeda: "USDT", DataHora: tipos.NovaDataHora(utc(14, 21, 0)), Preco: testeapoio.Valor("5.41")},
			{Moeda: "BTC", DataHora: tipos.NovaDataHora(utc(14, 21, 0)), Preco: testeapoio.Valor("618900.12")},
		}, `[{"moeda":"BTC","preco":618900.12,"dataHora":"2026-10-14T21:00:00Z","variacao24h":null,"fonte":"Mercado Bitcoin"},` +
			`{"moeda":"USDT","preco":5.41,"dataHora":"2026-10-14T21:00:00Z","variacao24h":null,"fonte":"Mercado Bitcoin"}]`},
		{"nada guardado: lista vazia", nil, `[]`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			s, _, fab, _ := novoServicoDeTeste(t, agora, &fonteFalsa{})
			for _, cot := range c.cotacoes {
				fab.CotacaoCripto(func(x *testeapoio.CotacaoCripto) { x.Moeda, x.DataHora, x.Preco = cot.Moeda, cot.DataHora, cot.Preco })
			}
			ultimas, err := s.Ultimas(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			obtido, _ := json.Marshal(ultimas.Cotacoes)
			testeapoio.ConferirJSON(t, obtido, c.esperado)
		})
	}
}
