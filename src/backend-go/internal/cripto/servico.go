// Pacote cripto é a cotação de criptomoedas em reais (ADR 0048, #81): o último negócio no Mercado Bitcoin, guardado
// a cada 30 minutos por 30 dias, e a rota GET /api/cripto/cotacoes. Porte do CriptoService, do CriptoController e do
// CriptoHostedService do .NET (o contrato, casos/03-cripto.json, confere).
package cripto

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Moedas são as acompanhadas, na ordem da tela (CriptoService.Moedas). Moeda nova: conferir antes que o Mercado
// Bitcoin tem o par XXX-BRL.
var Moedas = []string{"BTC", "ETH", "SOL", "USDT"}

// ChaveUltimaAtualizacao é a chave da Configuracao, a mesma do .NET.
const ChaveUltimaAtualizacao = "Cripto.UltimaAtualizacao"

// Fonte, como aparece no JSON.
const Fonte = "Mercado Bitcoin"

const (
	diasGuardados = 30 // por quantos dias a cotação fica guardada
	// janela da variação em 24 h: a cotação guardada entre 22 e 26 h antes da última, a mais perto de 24 h.
	horasAlvo, horasMinimas, horasMaximas = 24, 22, 26
)

// Cotacao é o CotacaoCriptoDto: o preço em reais e a variação contra ~24 h antes (null sem dado).
type Cotacao struct {
	Moeda       string                `json:"moeda"`
	Preco       tipos.Decimal         `json:"preco"`
	DataHora    configuracao.Instante `json:"dataHora"`
	Variacao24h *tipos.Decimal        `json:"variacao24h"`
	Fonte       string                `json:"fonte"`
}

// Cotacoes é o CotacoesCriptoDto.
type Cotacoes struct {
	AtualizadoEm *configuracao.Instante `json:"atualizadoEm"`
	Cotacoes     []Cotacao              `json:"cotacoes"`
}

// ResultadoAtualizacao é o ResultadoAtualizacaoCripto do .NET.
type ResultadoAtualizacao struct {
	Novas  int
	Falhas []string
}

// fonteDeNegocios é o que o serviço usa do cliente (a interface fica com quem usa, para os testes trocarem).
type fonteDeNegocios interface {
	UltimosNegocios(ctx context.Context, moedas []string) (map[string]Negocio, error)
}

// Servico é o CriptoService.
type Servico struct {
	db      *sql.DB
	config  *configuracao.Servico
	fonte   fonteDeNegocios
	relogio relogio.Relogio
	log     *slog.Logger
}

// NovoServico cria o serviço. fonte é o cliente do Mercado Bitcoin (NovoCliente).
func NovoServico(db *sql.DB, fonte fonteDeNegocios, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, config: configuracao.Novo(db, r), fonte: fonte, relogio: r, log: log}
}

// guardada é uma linha de CotacaoCripto.
type guardada struct {
	Moeda    string
	DataHora tipos.DataHora // UTC, como o EF Core grava (sem fuso)
	Preco    tipos.Decimal
}

// Atualizar é o AtualizarAsync: consulta o último negócio das moedas e grava os que ainda não estão guardados. Só
// marca a atualização como feita se nenhuma moeda faltar na resposta. Também apaga o que passou de 30 dias.
func (s *Servico) Atualizar(ctx context.Context) (ResultadoAtualizacao, error) {
	lidas, err := s.fonte.UltimosNegocios(ctx, Moedas)
	if err != nil {
		return ResultadoAtualizacao{}, err
	}
	agora := s.relogio.Agora()
	resultado := ResultadoAtualizacao{Falhas: []string{}}
	for _, m := range Moedas {
		if _, ok := lidas[m]; !ok {
			resultado.Falhas = append(resultado.Falhas, m)
		}
	}

	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		// Em ordem de moeda, para o resultado não depender da ordem do map (o .NET segue a da resposta).
		for _, m := range Moedas {
			n, ok := lidas[m]
			if !ok {
				continue
			}
			dataHora := tipos.NovaDataHora(n.DataHora)
			var existe int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "CotacaoCripto" WHERE "Moeda" = ? AND "DataHora" = ?`,
				m, dataHora).Scan(&existe); err != nil {
				return fmt.Errorf("conferir a cotação de %s: %w", m, err)
			}
			if existe > 0 {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO "CotacaoCripto" ("Moeda", "DataHora", "Preco", "AtualizadoEm") VALUES (?, ?, ?, ?)`,
				m, dataHora, n.Preco, tipos.NovaDataHora(agora)); err != nil {
				return fmt.Errorf("gravar a cotação de %s: %w", m, err)
			}
			resultado.Novas++
		}
		if len(resultado.Falhas) == 0 {
			return s.config.ComTx(tx).DefinirData(ctx, ChaveUltimaAtualizacao, agora)
		}
		return nil
	})
	if err != nil {
		return ResultadoAtualizacao{}, err
	}
	if len(resultado.Falhas) > 0 {
		s.log.Warn("O Mercado Bitcoin não devolveu a cotação de algumas moedas.", "origem", "Câmbio",
			"moedas", strings.Join(resultado.Falhas, ", "))
	}

	// O ExecuteDeleteAsync do .NET roda depois do SaveChangesAsync, fora da transação dele. A comparação da data
	// em TEXT ("2026-10-13 21:00:00") é a mesma que o EF Core faz.
	limite := tipos.NovaDataHora(agora.AddDate(0, 0, -diasGuardados))
	if _, err := s.db.ExecContext(ctx, `DELETE FROM "CotacaoCripto" WHERE "DataHora" < ?`, limite); err != nil {
		return resultado, fmt.Errorf("apagar as cotações antigas: %w", err)
	}
	return resultado, nil
}

// Ultimas é o UltimasAsync: a última cotação guardada de cada moeda, com a variação contra a guardada perto de 24 h
// antes (entre 22 e 26 h). Sem cotação nessa janela (site desligado, primeiras 24 h), a variação fica null. Não
// consulta a internet.
func (s *Servico) Ultimas(ctx context.Context) (Cotacoes, error) {
	desde := tipos.NovaDataHora(s.relogio.Agora().AddDate(0, 0, -2))
	recentes, err := s.ler(ctx, `WHERE "Moeda" IN (`+marcadores(len(Moedas))+`) AND "DataHora" >= ?`,
		append(argumentos(Moedas), desde)...)
	if err != nil {
		return Cotacoes{}, err
	}
	// Sem nada nos últimos 2 dias (site desligado): a última de cada moeda, mesmo antiga, com a data dela.
	for _, m := range Moedas {
		if slices.ContainsFunc(recentes, func(g guardada) bool { return g.Moeda == m }) {
			continue
		}
		ultima, err := s.ler(ctx, `WHERE "Moeda" = ? ORDER BY "DataHora" DESC LIMIT 1`, m)
		if err != nil {
			return Cotacoes{}, err
		}
		recentes = append(recentes, ultima...)
	}

	cotacoes := []Cotacao{}
	for _, m := range Moedas {
		daMoeda := []guardada{}
		for _, g := range recentes {
			if g.Moeda == m {
				daMoeda = append(daMoeda, g)
			}
		}
		if len(daMoeda) == 0 {
			continue
		}
		// OrderByDescending(DataHora) do .NET, estável.
		slices.SortStableFunc(daMoeda, func(a, b guardada) int { return b.DataHora.Tempo().Compare(a.DataHora.Tempo()) })
		ultima := daMoeda[0]
		variacao, err := variacao24h(ultima, daMoeda)
		if err != nil {
			return Cotacoes{}, err
		}
		cotacoes = append(cotacoes, Cotacao{
			Moeda:       m,
			Preco:       ultima.Preco,
			DataHora:    configuracao.InstanteUTC(ultima.DataHora.Tempo()), // o DateTime.SpecifyKind(..., Utc) do .NET
			Variacao24h: variacao,
			Fonte:       Fonte,
		})
	}
	atualizadoEm, err := s.config.ObterData(ctx, ChaveUltimaAtualizacao)
	if err != nil {
		return Cotacoes{}, err
	}
	return Cotacoes{AtualizadoEm: atualizadoEm, Cotacoes: cotacoes}, nil
}

// variacao24h é a variação, em %, da última contra a guardada mais perto de 24 h antes, dentro de 22 a 26 h:
// Math.Round((ultima - anterior) / anterior * 100, 2), com a escala do decimal do C#.
func variacao24h(ultima guardada, daMoeda []guardada) (*tipos.Decimal, error) {
	t := ultima.DataHora.Tempo()
	alvo := t.Add(-horasAlvo * time.Hour)
	minimo, maximo := t.Add(-horasMaximas*time.Hour), t.Add(-horasMinimas*time.Hour)
	candidatas := []guardada{}
	for _, g := range daMoeda {
		if h := g.DataHora.Tempo(); !h.After(maximo) && !h.Before(minimo) {
			candidatas = append(candidatas, g)
		}
	}
	// OrderBy(|DataHora - alvo|) estável: no empate (23 h e 25 h), fica a que vem antes em daMoeda, a mais recente.
	slices.SortStableFunc(candidatas, func(a, b guardada) int {
		return distancia(a.DataHora.Tempo(), alvo).Cmp(distancia(b.DataHora.Tempo(), alvo))
	})
	if len(candidatas) == 0 || !candidatas[0].Preco.Decimal().IsPositive() {
		return nil, nil
	}
	anterior := candidatas[0].Preco
	quociente, err := tipos.Dividir(tipos.Subtrair(ultima.Preco, anterior), anterior)
	if err != nil {
		return nil, err
	}
	v := tipos.Arredondar(tipos.Multiplicar(quociente, tipos.DecimalDeInteiro(100)), 2)
	return &v, nil
}

// distancia é o Math.Abs((a - b).Ticks).
func distancia(a, b time.Time) durationAbs {
	d := a.Sub(b)
	if d < 0 {
		d = -d
	}
	return durationAbs(d)
}

type durationAbs time.Duration

func (d durationAbs) Cmp(o durationAbs) int {
	switch {
	case d < o:
		return -1
	case d > o:
		return 1
	}
	return 0
}

func (s *Servico) ler(ctx context.Context, filtro string, args ...any) ([]guardada, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Moeda", "DataHora", "Preco" FROM "CotacaoCripto" `+filtro, args...)
	if err != nil {
		return nil, fmt.Errorf("ler as cotações de cripto: %w", err)
	}
	defer linhas.Close()
	lista := []guardada{}
	for linhas.Next() {
		var g guardada
		if err := linhas.Scan(&g.Moeda, &g.DataHora, &g.Preco); err != nil {
			return nil, fmt.Errorf("ler as cotações de cripto: %w", err)
		}
		lista = append(lista, g)
	}
	return lista, linhas.Err()
}

func marcadores(n int) string { return strings.TrimSuffix(strings.Repeat("?, ", n), ", ") }

func argumentos(lista []string) []any {
	args := make([]any, len(lista))
	for i, v := range lista {
		args[i] = v
	}
	return args
}

// ehDeRede diz se o erro é de rede (vira aviso na tarefa, como a HttpRequestException do .NET).
func ehDeRede(err error) bool { return errors.Is(err, ErrRede) }
