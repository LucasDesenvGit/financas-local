package investimentos

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/carteira"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// moedaBase é a moeda do histórico (CambioService.MoedaBase).
const moedaBase = "BRL"

// HistoricoInvestimentosDto é o histórico mês a mês (#173 e).
type HistoricoInvestimentosDto struct {
	Desde *tipos.Data             `json:"desde"`
	Meses []MesInvestimentosOfDto `json:"meses"`
}

// MesInvestimentosOfDto é um mês: o rendimento (soma dos que têm valor aplicado; null se nenhum tem) e o saldo no fim.
// Desde: o primeiro registro, no mês em que o histórico começa.
type MesInvestimentosOfDto struct {
	Ano             int                            `json:"ano"`
	Mes             int                            `json:"mes"`
	Desde           *tipos.Data                    `json:"desde"`
	Rendimento      *tipos.Decimal                 `json:"rendimento"`
	Saldo           tipos.Decimal                  `json:"saldo"`
	PorInvestimento []RendimentoInvestimentoMesDto `json:"porInvestimento"`
}

// RendimentoInvestimentoMesDto é um investimento num mês.
type RendimentoInvestimentoMesDto struct {
	Nome       string         `json:"nome"`
	Rendimento *tipos.Decimal `json:"rendimento"`
	Saldo      tipos.Decimal  `json:"saldo"`
}

// registro é uma linha de HistoricoInvestimento.
type registro struct {
	pluggyId, nome, tipo string
	data                 tipos.Data
	saldo                tipos.Decimal
	aplicado             *tipos.Decimal
}

// EntraNoHistorico: o que não é renda variável (a carteira própria cuida dos FIIs e das ações) e está em reais.
func EntraNoHistorico(tipo, moeda string) bool {
	return tipo != "EQUITY" && tipo != "ETF" && moeda == moedaBase
}

// Investimento é o que a gravação do histórico precisa de um investimento sincronizado.
type Investimento struct {
	PluggyInvestmentId string
	ConexaoBancariaId  int64
	Nome, Tipo, Moeda  string
	Saldo              tipos.Decimal
	ValorAplicado      *tipos.Decimal
}

// RegistrarHistorico é o RegistrarAsync do C#: guarda o saldo de hoje de cada investimento que entra no histórico (a
// última sincronização do dia vale: o registro do dia é atualizado). Roda dentro da transação de quem chama (a
// sincronização do Open Finance, #83).
func RegistrarHistorico(ctx context.Context, q consultas.DBTX, lista []Investimento, hoje tipos.Data, agora time.Time) error {
	for _, i := range lista {
		if !EntraNoHistorico(i.Tipo, i.Moeda) {
			continue
		}
		res, err := q.ExecContext(ctx, `UPDATE "HistoricoInvestimento" SET "ConexaoBancariaId" = ?, "Nome" = ?, "Tipo" = ?,
			"Saldo" = ?, "ValorAplicado" = ?, "AtualizadoEm" = ? WHERE "PluggyInvestmentId" = ? AND "Data" = ?`,
			i.ConexaoBancariaId, i.Nome, i.Tipo, i.Saldo, i.ValorAplicado, tipos.NovaDataHora(agora), i.PluggyInvestmentId, hoje)
		if err != nil {
			return fmt.Errorf("atualizar o histórico de %s: %w", i.PluggyInvestmentId, err)
		}
		if n, err := res.RowsAffected(); err != nil || n > 0 {
			if err != nil {
				return fmt.Errorf("atualizar o histórico de %s: %w", i.PluggyInvestmentId, err)
			}
			continue
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO "HistoricoInvestimento" ("PluggyInvestmentId", "ConexaoBancariaId",
			"Nome", "Tipo", "Data", "Saldo", "ValorAplicado", "AtualizadoEm") VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			i.PluggyInvestmentId, i.ConexaoBancariaId, i.Nome, i.Tipo, hoje, i.Saldo, i.ValorAplicado,
			tipos.NovaDataHora(agora)); err != nil {
			return fmt.Errorf("gravar o histórico de %s: %w", i.PluggyInvestmentId, err)
		}
	}
	return nil
}

// atual é o nome de um investimento que ainda existe, para o nome de exibição.
type atual struct {
	nome, tipo             string
	emissor, personalizado *string
}

// MesAMes é o MesAMesAsync do C#: o rendimento de cada mês, por investimento, é quanto o saldo cresceu além do valor
// aplicado ((saldo − aplicado) no fim do mês menos no fim do mês anterior), arredondado em duas casas pelo método
// bancário. Assim, o que entrou e saiu não conta como rendimento; sem o valor aplicado, o rendimento é null. O mês em
// que o histórico começa conta a partir do primeiro registro (Desde). O saldo do fim do mês é o do último dia com
// registro no mês (o investimento resgatado antes dele entra com 0).
func (s *Servico) MesAMes(ctx context.Context) (HistoricoInvestimentosDto, error) {
	registros, err := s.lerRegistros(ctx)
	if err != nil {
		return HistoricoInvestimentosDto{}, err
	}
	if len(registros) == 0 {
		return HistoricoInvestimentosDto{Meses: []MesInvestimentosOfDto{}}, nil
	}
	atuais, err := s.lerAtuais(ctx)
	if err != nil {
		return HistoricoInvestimentosDto{}, err
	}
	nome := func(r registro) string {
		if i, ok := atuais[r.pluggyId]; ok {
			return carteira.NomeDeExibicao(i.nome, i.tipo, i.emissor, i.personalizado)
		}
		return carteira.NomeDeExibicao(r.nome, r.tipo, nil, nil)
	}
	// O GroupBy do LINQ: as séries na ordem em que o investimento aparece pela primeira vez.
	var ordem []string
	series := map[string][]registro{}
	for _, r := range registros {
		if _, ok := series[r.pluggyId]; !ok {
			ordem = append(ordem, r.pluggyId)
		}
		series[r.pluggyId] = append(series[r.pluggyId], r)
	}
	primeiro := registros[0].data
	hoje := relogio.Hoje(s.relogio)
	meses := []MesInvestimentosOfDto{}
	for mes := tipos.NovaData(primeiro.Ano(), primeiro.Mes(), 1); mes.NumeroDoDia() <= hoje.NumeroDoDia(); mes = mes.SomarMeses(1) {
		fimDoMes := relogio.UltimoDiaMes(mes.Ano(), mes.Mes())
		ultimoDia, temNoMes := 0, false
		for _, r := range registros {
			if d := r.data.NumeroDoDia(); d >= mes.NumeroDoDia() && d <= fimDoMes.NumeroDoDia() {
				ultimoDia, temNoMes = max(ultimoDia, d), true
			}
		}
		if !temNoMes {
			continue
		}
		itens := []RendimentoInvestimentoMesDto{}
		for _, id := range ordem {
			serie := series[id]
			fim := ultimoAte(serie, fimDoMes.NumeroDoDia())
			if fim == nil || fim.data.NumeroDoDia() < mes.NumeroDoDia() {
				continue
			}
			inicio := ultimoAte(serie, mes.NumeroDoDia()-1)
			if inicio == nil {
				inicio = primeiroDesde(serie, mes.NumeroDoDia())
			}
			var rendimento *tipos.Decimal
			if inicio.aplicado != nil && fim.aplicado != nil {
				v := tipos.Arredondar(tipos.Subtrair(tipos.Subtrair(fim.saldo, *fim.aplicado),
					tipos.Subtrair(inicio.saldo, *inicio.aplicado)), 2)
				rendimento = &v
			}
			saldo := tipos.Zero
			if fim.data.NumeroDoDia() == ultimoDia {
				saldo = fim.saldo
			}
			itens = append(itens, RendimentoInvestimentoMesDto{Nome: nome(*fim), Rendimento: rendimento, Saldo: saldo})
		}
		m := MesInvestimentosOfDto{Ano: mes.Ano(), Mes: int(mes.Mes()), Saldo: tipos.Zero}
		if primeiro.NumeroDoDia() > mes.NumeroDoDia() {
			d := primeiro
			m.Desde = &d
		}
		for _, i := range itens {
			if i.Rendimento != nil {
				soma := tipos.Zero
				if m.Rendimento != nil {
					soma = *m.Rendimento
				}
				soma = tipos.Somar(soma, *i.Rendimento)
				m.Rendimento = &soma
			}
			m.Saldo = tipos.Somar(m.Saldo, i.Saldo)
		}
		// OrderByDescending(Saldo).ThenBy(Nome): o nome pela comparação padrão do C# (a da cultura).
		slices.SortStableFunc(itens, func(a, b RendimentoInvestimentoMesDto) int {
			if c := b.Saldo.Decimal().Cmp(a.Saldo.Decimal()); c != 0 {
				return c
			}
			return texto.CompararNaCultura(a.Nome, b.Nome)
		})
		m.PorInvestimento = itens
		meses = append(meses, m)
	}
	return HistoricoInvestimentosDto{Desde: &primeiro, Meses: meses}, nil
}

// ultimoAte é o último registro da série até o dia (LastOrDefault, a série está em ordem de data).
func ultimoAte(serie []registro, dia int) *registro {
	for k := len(serie) - 1; k >= 0; k-- {
		if serie[k].data.NumeroDoDia() <= dia {
			return &serie[k]
		}
	}
	return nil
}

// primeiroDesde é o primeiro registro da série a partir do dia (First).
func primeiroDesde(serie []registro, dia int) *registro {
	for k := range serie {
		if serie[k].data.NumeroDoDia() >= dia {
			return &serie[k]
		}
	}
	return nil
}

// lerRegistros: o histórico pela data (o OrderBy do EF Core: o empate fica na ordem da tabela, o Id).
func (s *Servico) lerRegistros(ctx context.Context) ([]registro, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "PluggyInvestmentId", "Nome", "Tipo", "Data", "Saldo", "ValorAplicado"
		FROM "HistoricoInvestimento" ORDER BY "Data", "Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler o histórico dos investimentos: %w", err)
	}
	defer linhas.Close()
	var lista []registro
	for linhas.Next() {
		var r registro
		if err := linhas.Scan(&r.pluggyId, &r.nome, &r.tipo, &r.data, &r.saldo, &r.aplicado); err != nil {
			return nil, fmt.Errorf("ler registro do histórico: %w", err)
		}
		lista = append(lista, r)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler o histórico dos investimentos: %w", err)
	}
	return lista, nil
}

func (s *Servico) lerAtuais(ctx context.Context) (map[string]atual, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "PluggyInvestmentId", "Nome", "Tipo", "Emissor", "NomePersonalizado"
		FROM "Investimento"`)
	if err != nil {
		return nil, fmt.Errorf("ler os investimentos do histórico: %w", err)
	}
	defer linhas.Close()
	atuais := map[string]atual{}
	for linhas.Next() {
		var id string
		var a atual
		if err := linhas.Scan(&id, &a.nome, &a.tipo, &a.emissor, &a.personalizado); err != nil {
			return nil, fmt.Errorf("ler investimento do histórico: %w", err)
		}
		atuais[id] = a
	}
	return atuais, linhas.Err()
}
