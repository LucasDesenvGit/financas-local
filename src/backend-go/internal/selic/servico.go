package selic

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

const (
	chaveMeta           = "Selic.Meta"
	chaveMetaEm         = "Selic.MetaConsultadaEm"
	chaveUltimaConsulta = "Selic.UltimaConsulta"
	// folgaDoComeco: só busca o começo se faltar mais que um feriado prolongado.
	folgaDoComeco = 5
	// anosPorConsulta: a API aceita no máximo 10 anos por consulta.
	anosPorConsulta = 9
	// validadeDaConsulta: quanto tempo esperar para procurar taxas novas no Banco Central.
	validadeDaConsulta = 6 * time.Hour
	// validadeDaMeta: a meta é consultada no máximo uma vez nesse tempo.
	validadeDaMeta = 12 * time.Hour
)

// fonte é o que o serviço usa do Banco Central. A interface é de quem usa: o de verdade é o Cliente.
type fonte interface {
	BuscarTaxas(ctx context.Context, de, ate tipos.Data) ([]Taxa, error)
	BuscarMeta(ctx context.Context) (*string, error)
}

// Servico é o SelicService.
type Servico struct {
	db      *sql.DB
	config  *configuracao.Servico
	fonte   fonte
	relogio relogio.Relogio
	log     *slog.Logger
}

// NovoServico monta o serviço com o banco, a fonte, o relógio e o log.
func NovoServico(db *sql.DB, f fonte, r relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, config: configuracao.Novo(db, r), fonte: f, relogio: r, log: log}
}

// avisar registra o aviso com a origem do C#: o SelicService fica em Servicos.Investimentos.
func (s *Servico) avisar(msg string, err error) { s.log.Warn(msg, "origem", "Investimentos", "erro", err) }

// faixa é um período que falta buscar.
type faixa struct{ de, ate tipos.Data }

// ObterSerie é o ObterSerieAsync: garante o guardado desde o início até hoje e devolve a série. Sem internet, ou com a
// API fora do ar, segue com o que já está guardado.
func (s *Servico) ObterSerie(ctx context.Context, inicio tipos.Data) (*Serie, error) {
	hoje := relogio.Hoje(s.relogio)
	if inicio.Depois(hoje) {
		inicio = hoje
	}
	faltam, err := s.faixasQueFaltam(ctx, inicio, hoje)
	if err != nil {
		return nil, err
	}
	if len(faltam) > 0 {
		if err := s.baixarFaixas(ctx, faltam); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			s.avisar("Não foi possível atualizar a Selic no Banco Central.", err)
		}
	}
	taxas, err := s.lerTaxas(ctx, inicio)
	if err != nil {
		return nil, err
	}
	return NovaSerie(taxas), nil
}

func (s *Servico) faixasQueFaltam(ctx context.Context, inicio, hoje tipos.Data) ([]faixa, error) {
	var minimo, maximo sql.Null[tipos.Data]
	if err := s.db.QueryRowContext(ctx, `SELECT MIN("Data"), MAX("Data") FROM "TaxaSelicDiaria"`).Scan(&minimo, &maximo); err != nil {
		return nil, fmt.Errorf("ler as datas da Selic guardada: %w", err)
	}
	if !minimo.Valid {
		return []faixa{{inicio, hoje}}, nil
	}
	var faltam []faixa
	if inicio.Antes(minimo.V.SomarDias(-folgaDoComeco)) {
		faltam = append(faltam, faixa{inicio, minimo.V.SomarDias(-1)})
	}
	ultima, err := s.config.ObterData(ctx, chaveUltimaConsulta)
	if err != nil {
		return nil, err
	}
	if maximo.V.Antes(hoje.SomarDias(-1)) && (ultima == nil || s.relogio.Agora().Sub(ultima.Tempo()) > validadeDaConsulta) {
		faltam = append(faltam, faixa{maximo.V.SomarDias(1), hoje})
	}
	return faltam, nil
}

// baixarFaixas baixa as faixas em partes e anota a consulta. Cada parte é gravada na hora: o que veio antes de um erro
// fica guardado.
func (s *Servico) baixarFaixas(ctx context.Context, faltam []faixa) error {
	for _, f := range faltam {
		for parte := f.de; !parte.Depois(f.ate); parte = parte.SomarAnos(anosPorConsulta) {
			fim := parte.SomarAnos(anosPorConsulta).SomarDias(-1)
			if fim.Depois(f.ate) {
				fim = f.ate
			}
			if err := s.baixar(ctx, parte, fim); err != nil {
				return err
			}
		}
	}
	return s.config.DefinirData(ctx, chaveUltimaConsulta, s.relogio.Agora())
}

func (s *Servico) baixar(ctx context.Context, de, ate tipos.Data) error {
	taxas, err := s.fonte.BuscarTaxas(ctx, de, ate)
	if err != nil {
		return err
	}
	if taxas == nil {
		return nil
	}
	return banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		guardadas, err := tx.QueryContext(ctx, `SELECT "Data" FROM "TaxaSelicDiaria" WHERE "Data" >= ? AND "Data" <= ?`, de, ate)
		if err != nil {
			return fmt.Errorf("ler a Selic guardada: %w", err)
		}
		defer guardadas.Close()
		existentes := map[string]bool{}
		for guardadas.Next() {
			var d tipos.Data
			if err := guardadas.Scan(&d); err != nil {
				return fmt.Errorf("ler a Selic guardada: %w", err)
			}
			existentes[d.String()] = true
		}
		if err := guardadas.Err(); err != nil {
			return fmt.Errorf("ler a Selic guardada: %w", err)
		}
		for _, t := range taxas {
			if existentes[t.Data.String()] {
				continue
			}
			existentes[t.Data.String()] = true
			if _, err := tx.ExecContext(ctx, `INSERT INTO "TaxaSelicDiaria" ("Data", "Taxa") VALUES (?, ?)`, t.Data, t.Taxa); err != nil {
				return fmt.Errorf("gravar a Selic de %s: %w", t.Data, err)
			}
		}
		return nil
	})
}

func (s *Servico) lerTaxas(ctx context.Context, inicio tipos.Data) ([]Taxa, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Data", "Taxa" FROM "TaxaSelicDiaria" WHERE "Data" >= ?`, inicio)
	if err != nil {
		return nil, fmt.Errorf("ler a Selic: %w", err)
	}
	defer linhas.Close()
	var taxas []Taxa
	for linhas.Next() {
		var t Taxa
		if err := linhas.Scan(&t.Data, &t.Taxa); err != nil {
			return nil, fmt.Errorf("ler a Selic: %w", err)
		}
		taxas = append(taxas, t)
	}
	return taxas, linhas.Err()
}

// lerMeta é o decimal.TryParse da meta guardada: nil se não houver, ou se não for um número.
func lerMeta(texto *string) *tipos.Decimal {
	if texto == nil {
		return nil
	}
	v, ok := tipos.LerNumero(*texto)
	if !ok {
		return nil
	}
	return &v
}

// MetaAtual é o MetaAtualAsync: a meta Selic atual (% ao ano), consultada no máximo a cada 12 horas. Sem internet, a
// guardada, mesmo velha.
func (s *Servico) MetaAtual(ctx context.Context) (*tipos.Decimal, error) {
	consultadaEm, err := s.config.ObterData(ctx, chaveMetaEm)
	if err != nil {
		return nil, err
	}
	atual, err := s.config.Obter(ctx, chaveMeta)
	if err != nil {
		return nil, err
	}
	if atual != nil && consultadaEm != nil && s.relogio.Agora().Sub(consultadaEm.Tempo()) < validadeDaMeta {
		return lerMeta(atual), nil
	}
	meta, err := s.consultarMeta(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		s.avisar("Não foi possível consultar a meta Selic.", err)
	}
	if meta != nil {
		return meta, nil
	}
	return lerMeta(atual), nil
}

// consultarMeta busca a meta e a grava com a hora da consulta. nil se a resposta não trouxe um número; nesse caso, nada
// é gravado.
func (s *Servico) consultarMeta(ctx context.Context) (*tipos.Decimal, error) {
	texto, err := s.fonte.BuscarMeta(ctx)
	if err != nil {
		return nil, err
	}
	meta := lerMeta(texto)
	if meta == nil {
		return nil, nil
	}
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		config := s.config.ComTx(tx)
		guardar := meta.String()
		if err := config.Definir(ctx, chaveMeta, &guardar); err != nil {
			return err
		}
		return config.DefinirData(ctx, chaveMetaEm, s.relogio.Agora())
	})
	if err != nil {
		return nil, err
	}
	return meta, nil
}
