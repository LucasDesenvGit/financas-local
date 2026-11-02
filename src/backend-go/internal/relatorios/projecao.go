package relatorios

import (
	"context"
	"fmt"
	"slices"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/parcelas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// CorSemCategoriaDaProjecao é a cor de "Sem categoria" na projeção e na visão anual (outra que a do gráfico de categorias).
const CorSemCategoriaDaProjecao = "#a9b4c6"

const (
	tipoContaCartao       = 3
	tipoContaInvestimento = 4
	origemOpenFinance     = 2
)

// Situações de um mês da projeção.
const (
	Realizado   = "realizado"
	EmAndamento = "em andamento"
	Projetado   = "projetado"
)

// ProjecaoMesDto é um mês da projeção.
type ProjecaoMesDto struct {
	Ano            int            `json:"ano"`
	Mes            int            `json:"mes"`
	Situacao       string         `json:"situacao"`
	Receitas       tipos.Decimal  `json:"receitas"`
	Despesas       tipos.Decimal  `json:"despesas"`
	Parcelas       tipos.Decimal  `json:"parcelas"`
	Resultado      tipos.Decimal  `json:"resultado"`
	SaldoProjetado *tipos.Decimal `json:"saldoProjetado"`
}

// ProjecaoCategoriaDto é a média mensal de uma categoria (despesas à vista) e o valor do último mês completo.
type ProjecaoCategoriaDto struct {
	CategoriaId *int64        `json:"categoriaId"`
	Nome        string        `json:"nome"`
	Cor         string        `json:"cor"`
	MediaMensal tipos.Decimal `json:"mediaMensal"`
	UltimoMes   tipos.Decimal `json:"ultimoMes"`
}

// ProjecaoDto é a projeção de receitas, despesas e saldo.
type ProjecaoDto struct {
	MesesConsiderados    int                    `json:"mesesConsiderados"`
	MediaReceitas        tipos.Decimal          `json:"mediaReceitas"`
	MediaDespesas        tipos.Decimal          `json:"mediaDespesas"`
	SaldoDisponivelAtual tipos.Decimal          `json:"saldoDisponivelAtual"`
	Meses                []ProjecaoMesDto       `json:"meses"`
	Categorias           []ProjecaoCategoriaDto `json:"categorias"`
}

// ContaDoSaldo são as colunas de Conta que o saldo de partida usa.
type ContaDoSaldo struct {
	Id, Tipo, Origem int64
}

// ContasNoSaldo são as contas não arquivadas que entram no saldo (IncluirNoSaldo), na ordem do Id.
func (s *Servico) ContasNoSaldo(ctx context.Context) ([]ContaDoSaldo, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Tipo", "Origem" FROM "Conta"
		WHERE NOT "Arquivada" AND "IncluirNoSaldo" ORDER BY "Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler as contas do saldo: %w", err)
	}
	defer linhas.Close()
	var lista []ContaDoSaldo
	for linhas.Next() {
		var c ContaDoSaldo
		if err := linhas.Scan(&c.Id, &c.Tipo, &c.Origem); err != nil {
			return nil, fmt.Errorf("ler conta do saldo: %w", err)
		}
		lista = append(lista, c)
	}
	return lista, linhas.Err()
}

// Projecao é o ProjecaoAsync do C#: o mês a mês com base no histórico. saldos são os de contas.Saldos (o
// CalcularSaldosAsync), que o pacote de regra não importa.
//   - receitas futuras: a média mensal das receitas dos últimos meses completos (até 6), em duas casas;
//   - despesas futuras: a média das despesas à vista (sem parcelados) mais as parcelas já contratadas do mês (#165);
//   - mês atual: o realizado até hoje mais a média proporcional aos dias que faltam (receitas: o maior entre o recebido
//     e a média);
//   - saldo projetado: parte do saldo disponível (contas, menos investimentos), com as parcelas futuras dos cartões do
//     Open Finance de volta (#257: o saldo do cartão já tem a dívida toda, e os meses tiram as parcelas de novo).
func (s *Servico) Projecao(ctx context.Context, mesesFuturos int, saldos map[int64]tipos.Decimal) (ProjecaoDto, error) {
	hoje := s.hoje()
	inicioMesAtual := tipos.NovaData(hoje.Ano(), hoje.Mes(), 1)
	fimHistorico := inicioMesAtual.SomarDias(-1)
	inicioHistorico, meses, err := s.JanelaHistorico(ctx)
	if err != nil {
		return ProjecaoDto{}, err
	}
	p := ProjecaoDto{MesesConsiderados: meses, MediaReceitas: tipos.Zero, MediaDespesas: tipos.Zero,
		Meses: []ProjecaoMesDto{}, Categorias: []ProjecaoCategoriaDto{}}
	var historico []MesValoresDto
	if meses > 0 {
		lancamentos, err := s.Relevantes(ctx, inicioHistorico, fimHistorico)
		if err != nil {
			return ProjecaoDto{}, err
		}
		historico = fluxoMensal(lancamentos, inicioHistorico, fimHistorico)
		if err := s.medias(ctx, &p, lancamentos, inicioMesAtual); err != nil {
			return ProjecaoDto{}, err
		}
	}

	futuras, err := s.parcelasFuturas(ctx, hoje)
	if err != nil {
		return ProjecaoDto{}, err
	}
	parcelasPorMes := map[int]tipos.Decimal{}
	for _, f := range futuras {
		k := f.Data.Ano()*12 + int(f.Data.Mes())
		parcelasPorMes[k] = tipos.Somar(zeroSeAusente(parcelasPorMes, k), f.Valor)
	}

	contas, err := s.ContasNoSaldo(ctx)
	if err != nil {
		return ProjecaoDto{}, err
	}
	saldoDisponivel := tipos.Zero
	cartoesOpenFinance := map[int64]bool{}
	for _, c := range contas {
		if c.Tipo != tipoContaInvestimento {
			saldoDisponivel = tipos.Somar(saldoDisponivel, zeroSeAusente(saldos, c.Id))
		}
		if c.Tipo == tipoContaCartao && c.Origem == origemOpenFinance {
			cartoesOpenFinance[c.Id] = true
		}
	}
	inicioProximoMes := inicioMesAtual.SomarMeses(1)
	noSaldoDoCartao := tipos.Zero
	for _, f := range futuras {
		if cartoesOpenFinance[f.ContaId] && f.Data.NumeroDoDia() >= inicioProximoMes.NumeroDoDia() {
			noSaldoDoCartao = tipos.Somar(noSaldoDoCartao, f.Valor)
		}
	}
	saldoDisponivel = tipos.Somar(saldoDisponivel, tipos.Arredondar(noSaldoDoCartao, 2))

	for _, h := range historico {
		p.Meses = append(p.Meses, ProjecaoMesDto{Ano: h.Ano, Mes: h.Mes, Situacao: Realizado, Receitas: h.Receitas,
			Despesas: h.Despesas, Parcelas: tipos.Zero, Resultado: tipos.Subtrair(h.Receitas, h.Despesas)})
	}

	// Mês atual.
	receitasAtual, despesasAtual, err := s.Totais(ctx, inicioMesAtual, hoje)
	if err != nil {
		return ProjecaoDto{}, err
	}
	diasMes := inicioMesAtual.SomarMeses(1).SomarDias(-1).Dia()
	fracao, err := tipos.Dividir(tipos.DecimalDeInteiro(int64(diasMes-hoje.Dia())), tipos.DecimalDeInteiro(int64(diasMes)))
	if err != nil {
		return ProjecaoDto{}, err
	}
	receitasMes := maiorDeDois(receitasAtual, p.MediaReceitas)
	despesasMes := tipos.Arredondar(tipos.Somar(despesasAtual, tipos.Multiplicar(p.MediaDespesas, fracao)), 2)
	saldo := tipos.Subtrair(tipos.Somar(saldoDisponivel, tipos.Subtrair(receitasMes, receitasAtual)),
		tipos.Subtrair(despesasMes, despesasAtual))
	saldoMes := tipos.Arredondar(saldo, 2)
	p.Meses = append(p.Meses, ProjecaoMesDto{Ano: hoje.Ano(), Mes: int(hoje.Mes()), Situacao: EmAndamento,
		Receitas: receitasMes, Despesas: despesasMes, Parcelas: tipos.Zero, Resultado: tipos.Subtrair(receitasMes, despesasMes),
		SaldoProjetado: &saldoMes})

	// Meses futuros.
	for k := 1; k <= mesesFuturos; k++ {
		m := inicioMesAtual.SomarMeses(k)
		parcelasMes := tipos.Arredondar(zeroSeAusente(parcelasPorMes, m.Ano()*12+int(m.Mes())), 2)
		despesas := tipos.Somar(p.MediaDespesas, parcelasMes)
		saldo = tipos.Somar(saldo, tipos.Subtrair(p.MediaReceitas, despesas))
		saldoFuturo := tipos.Arredondar(saldo, 2)
		p.Meses = append(p.Meses, ProjecaoMesDto{Ano: m.Ano(), Mes: int(m.Mes()), Situacao: Projetado,
			Receitas: p.MediaReceitas, Despesas: despesas, Parcelas: parcelasMes,
			Resultado: tipos.Subtrair(p.MediaReceitas, despesas), SaldoProjetado: &saldoFuturo})
	}
	p.SaldoDisponivelAtual = tipos.Arredondar(saldoDisponivel, 2)
	return p, nil
}

// zeroSeAusente é o GetValueOrDefault de um dicionário de decimais: 0 sem casas quando a chave não existe.
func zeroSeAusente[K comparable](m map[K]tipos.Decimal, k K) tipos.Decimal {
	if v, ok := m[k]; ok {
		return v
	}
	return tipos.Zero
}

// aVista: despesa sem parcelamento (ParcelaTotal nulo ou até 1).
func aVista(l Lancamento) bool {
	return l.Tipo == transacoes.Despesa && (l.ParcelaTotal == nil || *l.ParcelaTotal <= 1)
}

// medias preenche a média das receitas, a das despesas à vista e as categorias da projeção.
func (s *Servico) medias(ctx context.Context, p *ProjecaoDto, lancamentos []Lancamento, inicioMesAtual tipos.Data) error {
	meses := tipos.DecimalDeInteiro(int64(p.MesesConsiderados))
	somaReceitas := somaOuZero(valoresDe(lancamentos, func(l Lancamento) bool { return l.Tipo == transacoes.Receita }))
	somaDespesas := somaOuZero(valoresDe(lancamentos, aVista))
	mediaReceitas, err := tipos.Dividir(somaReceitas, meses)
	if err != nil {
		return err
	}
	mediaDespesas, err := tipos.Dividir(maiorDeDois(tipos.Zero, negar(somaDespesas)), meses)
	if err != nil {
		return err
	}
	p.MediaReceitas, p.MediaDespesas = tipos.Arredondar(mediaReceitas, 2), tipos.Arredondar(mediaDespesas, 2)

	categorias, err := s.lerCategorias(ctx)
	if err != nil {
		return err
	}
	inicioUltimoMes := inicioMesAtual.SomarMeses(-1)
	media := agruparPorCategoria(somasPorCategoria(lancamentos, aVista), categorias)
	ultimo := agruparPorCategoria(somasPorCategoria(lancamentos, func(l Lancamento) bool {
		return aVista(l) && l.Data.NumeroDoDia() >= inicioUltimoMes.NumeroDoDia()
	}), categorias)
	for _, c := range media {
		mediaMensal, err := tipos.Dividir(c.Valor, meses)
		if err != nil {
			return err
		}
		valorUltimo := tipos.Zero
		for _, u := range ultimo {
			if mesmoId(u.CategoriaId, c.CategoriaId) && u.Nome == c.Nome {
				valorUltimo = u.Valor
				break
			}
		}
		p.Categorias = append(p.Categorias, ProjecaoCategoriaDto{CategoriaId: c.CategoriaId, Nome: c.Nome, Cor: c.Cor,
			MediaMensal: tipos.Arredondar(mediaMensal, 2), UltimoMes: valorUltimo})
	}
	slices.SortStableFunc(p.Categorias, func(a, b ProjecaoCategoriaDto) int {
		return b.MediaMensal.Decimal().Cmp(a.MediaMensal.Decimal())
	})
	return nil
}

func mesmoId(a, b *int64) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

// somaPorCategoria é um grupo do GroupBy(t => t.CategoriaId) feito no SQLite: a soma no formato do banco.
type somaPorCategoria struct {
	categoriaId *int64
	soma        tipos.Decimal
}

// somasPorCategoria agrupa os lançamentos que passam pelo CategoriaId, na ordem do GROUP BY do SQLite (os sem categoria
// primeiro, depois pelo Id).
func somasPorCategoria(lancamentos []Lancamento, passa func(Lancamento) bool) []somaPorCategoria {
	var semId []tipos.Decimal
	porId := map[int64][]tipos.Decimal{}
	var ids []int64
	for _, l := range lancamentos {
		if !passa(l) {
			continue
		}
		if l.CategoriaId == nil {
			semId = append(semId, l.Valor)
			continue
		}
		if _, ok := porId[*l.CategoriaId]; !ok {
			ids = append(ids, *l.CategoriaId)
		}
		porId[*l.CategoriaId] = append(porId[*l.CategoriaId], l.Valor)
	}
	slices.Sort(ids)
	var grupos []somaPorCategoria
	if len(semId) > 0 {
		grupos = append(grupos, somaPorCategoria{nil, somaOuZero(semId)})
	}
	for _, id := range ids {
		grupos = append(grupos, somaPorCategoria{&id, somaOuZero(porId[id])})
	}
	return grupos
}

// agruparPorCategoria é o AgruparPorCategoriaAsync do C#: as somas viram despesas líquidas de estornos (o valor negado,
// ADR 0036), a subcategoria soma na categoria pai, e só fica quem passou de zero; "Sem categoria" (sem Id, ou com uma
// categoria que não existe) vai por último, se passou de zero.
func agruparPorCategoria(grupos []somaPorCategoria, categorias map[int64]categoriaLida) []ValorCategoriaDto {
	type total struct {
		nome, cor string
		valor     tipos.Decimal
	}
	resultado := map[int64]total{}
	var ordem []int64
	semCategoria := tipos.Zero
	for _, g := range grupos {
		valor := negar(g.soma)
		if g.categoriaId == nil {
			semCategoria = tipos.Somar(semCategoria, valor)
			continue
		}
		id := *g.categoriaId
		cat, existe := categorias[id]
		if !existe {
			semCategoria = tipos.Somar(semCategoria, valor)
			continue
		}
		if cat.paiId != nil {
			if pai, existePai := categorias[*cat.paiId]; existePai {
				id, cat = *cat.paiId, pai
			}
		}
		if atual, ja := resultado[id]; ja {
			resultado[id] = total{atual.nome, atual.cor, tipos.Somar(atual.valor, valor)}
		} else {
			resultado[id] = total{cat.nome, cat.cor, valor}
			ordem = append(ordem, id)
		}
	}
	lista := []ValorCategoriaDto{}
	for _, id := range ordem {
		if t := resultado[id]; t.valor.Decimal().IsPositive() {
			lista = append(lista, ValorCategoriaDto{CategoriaId: &id, Nome: t.nome, Cor: t.cor, Valor: t.valor})
		}
	}
	if semCategoria.Decimal().IsPositive() {
		lista = append(lista, ValorCategoriaDto{Nome: "Sem categoria", Cor: CorSemCategoriaDaProjecao, Valor: semCategoria})
	}
	return lista
}

// parcelasFuturas são as parcelas por vir (#165): as que o banco já mandou com data futura e as projetadas depois da
// última de cada compra. A base são as saídas recentes (até parcelas.DiasRecentes dias) parceladas, de qualquer conta,
// sem as ignoradas nos relatórios. O valor é conferido em Go (dinheiro é texto no banco).
func (s *Servico) parcelasFuturas(ctx context.Context, hoje tipos.Data) ([]parcelas.Futura, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "ContaId", "Data", "Valor", "ParcelaNumero", "ParcelaTotal",
		COALESCE("DescricaoOriginal", "Descricao") FROM "Transacao"
		WHERE "Data" >= ? AND "IgnorarRelatorios" = 0 AND "ParcelaTotal" IS NOT NULL AND "ParcelaTotal" > 1
		AND "ParcelaNumero" IS NOT NULL ORDER BY "Id"`, hoje.SomarDias(-parcelas.DiasRecentes))
	if err != nil {
		return nil, fmt.Errorf("ler as parcelas recentes: %w", err)
	}
	defer linhas.Close()
	var base []parcelas.Parcela
	for linhas.Next() {
		var p parcelas.Parcela
		if err := linhas.Scan(&p.Id, &p.ContaId, &p.Data, &p.Valor, &p.Numero, &p.Total, &p.Descricao); err != nil {
			return nil, fmt.Errorf("ler parcela recente: %w", err)
		}
		if p.Valor.Decimal().IsNegative() {
			base = append(base, p)
		}
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as parcelas recentes: %w", err)
	}
	return parcelas.Projetar(base, hoje), nil
}
