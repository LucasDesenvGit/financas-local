package cambio

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// AtualizarTodas é o AtualizarTodasAsync: baixa os últimos 7 dias de cada moeda da lista, insere as novas e corrige
// as que o Banco Central republicou. Só marca a atualização como feita se nenhuma moeda falhar.
func (s *Servico) AtualizarTodas(ctx context.Context) (ResultadoAtualizacao, error) {
	moedas, err := s.ListarMoedas(ctx)
	if err != nil {
		return ResultadoAtualizacao{}, err
	}
	if len(moedas) == 0 {
		return ResultadoAtualizacao{Falhas: []string{"lista de moedas"}}, nil
	}

	hoje := relogio.Hoje(s.relogio)
	resultado := ResultadoAtualizacao{Falhas: []string{}}
	for _, m := range moedas {
		if err := ctx.Err(); err != nil {
			return resultado, err
		}
		novas, corrigidas, err := s.baixar(ctx, m.Codigo, hoje.SomarDias(-diasRetroativos), hoje, nil)
		if err != nil {
			if ctx.Err() != nil {
				return resultado, ctx.Err()
			}
			s.avisar("Não foi possível atualizar a cotação de uma moeda no Banco Central.", "moeda", m.Codigo, "erro", err)
			resultado.Falhas = append(resultado.Falhas, m.Codigo)
		} else {
			resultado.Novas += novas
			resultado.Corrigidas += corrigidas
		}
		if err := s.esperar(ctx, pausaEntreMoedas); err != nil {
			return resultado, err
		}
	}

	if len(resultado.Falhas) > 0 {
		return resultado, nil
	}
	return resultado, s.config.DefinirData(ctx, ChaveUltimaAtualizacao, s.relogio.Agora())
}

// Ultimas é o UltimasAsync: a última cotação guardada de cada moeda, sem consultar a internet.
func (s *Servico) Ultimas(ctx context.Context) (CotacoesAtuais, error) {
	hoje := relogio.Hoje(s.relogio)
	guardadas, err := s.ultimasGuardadas(ctx)
	if err != nil {
		return CotacoesAtuais{}, err
	}
	cotacoes := make([]Cotacao, 0, len(guardadas))
	for _, g := range guardadas {
		cotacoes = append(cotacoes, paraCotacao(g, hoje))
	}
	atualizadoEm, err := s.config.ObterData(ctx, ChaveUltimaAtualizacao)
	if err != nil {
		return CotacoesAtuais{}, err
	}
	return CotacoesAtuais{AtualizadoEm: atualizadoEm, Cotacoes: cotacoes}, nil
}

// ObterCotacao é o ObterCotacaoAsync: a PTAX de fechamento da moeda na data (sem data, ou no futuro: hoje), ou a do
// último dia útil até 7 dias antes. Sem a cotação do dia no banco, consulta a API (hoje, no máximo a cada 30 min);
// sem internet, segue com o que estiver guardado. nil: não há cotação nem na API nem no banco.
func (s *Servico) ObterCotacao(ctx context.Context, moeda string, data *tipos.Data) (*Cotacao, error) {
	hoje := relogio.Hoje(s.relogio)
	dia := hoje
	if data != nil && !data.Depois(hoje) {
		dia = *data
	}
	um := tipos.DecimalDeInteiro(1)
	if moeda == MoedaBase {
		return &Cotacao{Moeda: MoedaBase, DataSolicitada: dia, Data: dia, Compra: um, Venda: um, Fonte: fonteFixa}, nil
	}

	exata, err := s.lerCotacao(ctx, `WHERE "Moeda" = ? AND "Data" = ?`, moeda, dia)
	if err != nil || exata != nil {
		return cotacaoOuNil(exata, dia), err
	}

	// Hoje sem boletim de fechamento (antes das ~13h, fim de semana, feriado): evita bater na API a cada chamada.
	chaveConsulta := prefixoUltimaConsulta + moeda
	consultarAPI := true
	agora := s.relogio.Agora()
	if dia.Igual(hoje) {
		ultima, err := s.config.ObterData(ctx, chaveConsulta)
		if err != nil {
			return nil, err
		}
		consultarAPI = ultima == nil || agora.Sub(ultima.Tempo()) > intervaloDeConsultaHoje
	}
	if consultarAPI {
		var depois func(*sql.Tx) error
		if dia.Igual(hoje) {
			depois = func(tx *sql.Tx) error { return s.config.ComTx(tx).DefinirData(ctx, chaveConsulta, agora) }
		}
		if _, _, err := s.baixar(ctx, moeda, dia.SomarDias(-diasRetroativos), dia, depois); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			s.avisar("Não foi possível consultar a cotação de uma moeda no Banco Central.", "moeda", moeda, "erro", err)
		}
	}

	ultima, err := s.lerCotacao(ctx, `WHERE "Moeda" = ? AND "Data" <= ? AND "Data" >= ? ORDER BY "Data" DESC LIMIT 1`,
		moeda, dia, dia.SomarDias(-diasRetroativos))
	return cotacaoOuNil(ultima, dia), err
}

// GarantirHistorico é o GarantirHistoricoAsync: deixa no banco as cotações diárias da moeda desde a data (para
// converter operações e proventos antigos). Só consulta a API se faltar o começo do período, um ano por consulta;
// falha de rede não interrompe quem chamou (vira aviso).
func (s *Servico) GarantirHistorico(ctx context.Context, moeda string, desde tipos.Data) error {
	if moeda == MoedaBase {
		return nil
	}
	hoje := relogio.Hoje(s.relogio)
	if desde.Depois(hoje) {
		return nil
	}
	var minimo sql.Null[tipos.Data]
	if err := s.db.QueryRowContext(ctx, `SELECT MIN("Data") FROM "CotacaoMoeda" WHERE "Moeda" = ?`, moeda).Scan(&minimo); err != nil {
		return fmt.Errorf("ler o começo do histórico de %s: %w", moeda, err)
	}
	// Já cobre o começo (com folga de um fim de semana prolongado): nada a baixar.
	if minimo.Valid && !minimo.V.Depois(desde.SomarDias(diasRetroativos)) {
		return nil
	}
	ate := hoje
	if minimo.Valid {
		ate = minimo.V.SomarDias(-1)
	}
	for parteInicio := desde.SomarDias(-diasRetroativos); !parteInicio.Depois(ate); parteInicio = parteInicio.SomarAnos(1) {
		parteFim := parteInicio.SomarAnos(1).SomarDias(-1)
		if parteFim.Depois(ate) {
			parteFim = ate
		}
		if _, _, err := s.baixar(ctx, moeda, parteInicio, parteFim, nil); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.avisar("Não foi possível baixar o histórico de uma moeda no Banco Central.", "moeda", moeda, "erro", err)
			return nil
		}
	}
	return nil
}

// ObterConversor é o ObterConversorAsync: o conversor com a última cotação de cada moeda e, com historicoDesde, as
// cotações diárias desde 7 dias antes da data, para converter pela data. Não consulta a internet.
func (s *Servico) ObterConversor(ctx context.Context, historicoDesde *tipos.Data) (*Conversor, error) {
	ultimas, err := s.ultimasGuardadas(ctx)
	if err != nil {
		return nil, err
	}
	var historico []Guardada
	if historicoDesde != nil {
		historico, err = s.lerCotacoes(ctx, `WHERE "Data" >= ?`, historicoDesde.SomarDias(-diasRetroativos))
		if err != nil {
			return nil, err
		}
	}
	return NovoConversor(ultimas, historico), nil
}

// baixar é o BaixarAsync: consulta a API (fora da transação) e grava numa transação curta as novas e as que
// mudaram. depois, se não for nil, roda na mesma transação (o SaveChangesAsync do .NET grava tudo junto).
func (s *Servico) baixar(ctx context.Context, moeda string, de, ate tipos.Data, depois func(*sql.Tx) error) (novas, corrigidas int, err error) {
	fechamentos, err := s.fonte.BuscarFechamentos(ctx, moeda, de, ate)
	if err != nil {
		return 0, 0, err
	}
	agora := tipos.NovaDataHora(s.relogio.Agora())
	err = banco.EmTransacao(ctx, s.db, func(tx *sql.Tx) error {
		existentes := map[string]Guardada{} // pela data em texto: o == do time.Time não serve de chave
		linhas, err := lerCotacoesCom(ctx, tx, `WHERE "Moeda" = ? AND "Data" >= ? AND "Data" <= ?`, moeda, de, ate)
		if err != nil {
			return err
		}
		for _, g := range linhas {
			existentes[g.Data.String()] = g
		}
		for _, f := range fechamentos {
			if atual, ok := existentes[f.Data.String()]; ok {
				// O Banco Central pode republicar um boletim corrigido: grava o valor mais recente. O == do decimal
				// do C# não olha a escala (5.0113 == 5.01130).
				if atual.Compra.Decimal().Equal(f.Compra.Decimal()) && atual.Venda.Decimal().Equal(f.Venda.Decimal()) {
					continue
				}
				if _, err := tx.ExecContext(ctx, `UPDATE "CotacaoMoeda" SET "Compra" = ?, "Venda" = ?, "AtualizadoEm" = ? WHERE "Moeda" = ? AND "Data" = ?`,
					f.Compra, f.Venda, agora, moeda, f.Data); err != nil {
					return fmt.Errorf("corrigir a cotação de %s em %s: %w", moeda, f.Data, err)
				}
				existentes[f.Data.String()] = Guardada{Moeda: moeda, Data: f.Data, Compra: f.Compra, Venda: f.Venda}
				corrigidas++
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO "CotacaoMoeda" ("Moeda", "Data", "Compra", "Venda", "AtualizadoEm") VALUES (?, ?, ?, ?, ?)`,
				moeda, f.Data, f.Compra, f.Venda, agora); err != nil {
				return fmt.Errorf("gravar a cotação de %s em %s: %w", moeda, f.Data, err)
			}
			existentes[f.Data.String()] = Guardada{Moeda: moeda, Data: f.Data, Compra: f.Compra, Venda: f.Venda}
			novas++
		}
		if depois != nil {
			return depois(tx)
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return novas, corrigidas, nil
}

// ultimasGuardadas é o UltimasCotacoesAsync: a linha da data mais recente de cada moeda, em ordem de moeda.
func (s *Servico) ultimasGuardadas(ctx context.Context) ([]Guardada, error) {
	// MAX numa data em TEXT "AAAA-MM-DD" ordena certo (não é dinheiro).
	lista, err := s.lerCotacoes(ctx,
		`WHERE "Data" = (SELECT MAX(u."Data") FROM "CotacaoMoeda" u WHERE u."Moeda" = "CotacaoMoeda"."Moeda")`)
	if err != nil {
		return nil, err
	}
	// O OrderBy(c => c.Moeda) do .NET compara pela cultura; o código da moeda é só A-Z, em que dá o mesmo.
	slices.SortStableFunc(lista, func(a, b Guardada) int { return strings.Compare(a.Moeda, b.Moeda) })
	return lista, nil
}

func (s *Servico) lerCotacao(ctx context.Context, filtro string, args ...any) (*Guardada, error) {
	lista, err := s.lerCotacoes(ctx, filtro, args...)
	if err != nil || len(lista) == 0 {
		return nil, err
	}
	return &lista[0], nil
}

func (s *Servico) lerCotacoes(ctx context.Context, filtro string, args ...any) ([]Guardada, error) {
	return lerCotacoesCom(ctx, s.db, filtro, args...)
}

type consultador interface {
	QueryContext(ctx context.Context, consulta string, args ...any) (*sql.Rows, error)
}

// lerCotacoesCom lê linhas de CotacaoMoeda com o filtro (texto fixo deste pacote; os valores vão por "?").
func lerCotacoesCom(ctx context.Context, db consultador, filtro string, args ...any) ([]Guardada, error) {
	linhas, err := db.QueryContext(ctx, `SELECT "Moeda", "Data", "Compra", "Venda" FROM "CotacaoMoeda" `+filtro, args...)
	if err != nil {
		return nil, fmt.Errorf("ler as cotações: %w", err)
	}
	defer linhas.Close()
	lista := []Guardada{}
	for linhas.Next() {
		var g Guardada
		if err := linhas.Scan(&g.Moeda, &g.Data, &g.Compra, &g.Venda); err != nil {
			return nil, fmt.Errorf("ler as cotações: %w", err)
		}
		lista = append(lista, g)
	}
	return lista, linhas.Err()
}

func paraCotacao(g Guardada, solicitada tipos.Data) Cotacao {
	return Cotacao{Moeda: g.Moeda, DataSolicitada: solicitada, Data: g.Data, Compra: g.Compra, Venda: g.Venda, Fonte: fontePTAX}
}

func cotacaoOuNil(g *Guardada, solicitada tipos.Data) *Cotacao {
	if g == nil {
		return nil
	}
	c := paraCotacao(*g, solicitada)
	return &c
}
