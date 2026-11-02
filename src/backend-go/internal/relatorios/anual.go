package relatorios

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// Naturezas de gasto (o enum NaturezaGasto do C#).
const (
	NaturezaEssencial    = 1
	NaturezaEstiloDeVida = 2
)

// ResumoAnoDto são as receitas, as despesas, o resultado e a taxa de poupança de um período.
type ResumoAnoDto struct {
	Receitas     tipos.Decimal `json:"receitas"`
	Despesas     tipos.Decimal `json:"despesas"`
	Resultado    tipos.Decimal `json:"resultado"`
	TaxaPoupanca tipos.Decimal `json:"taxaPoupanca"`
}

// MesComparativoDto é um mês do ano comparado com o mesmo mês do ano de comparação.
type MesComparativoDto struct {
	Mes                int            `json:"mes"`
	Receitas           tipos.Decimal  `json:"receitas"`
	Despesas           tipos.Decimal  `json:"despesas"`
	ReceitasAnterior   tipos.Decimal  `json:"receitasAnterior"`
	DespesasAnterior   tipos.Decimal  `json:"despesasAnterior"`
	PatrimonioEstimado *tipos.Decimal `json:"patrimonioEstimado"`
	Futuro             bool           `json:"futuro"`
}

// CategoriaComparativaDto é o gasto de uma categoria (pela raiz) no ano, no ano de comparação e mês a mês.
type CategoriaComparativaDto struct {
	CategoriaId   *int64          `json:"categoriaId"`
	Nome          string          `json:"nome"`
	Cor           string          `json:"cor"`
	Natureza      *int64          `json:"natureza"`
	Valor         tipos.Decimal   `json:"valor"`
	ValorAnterior tipos.Decimal   `json:"valorAnterior"`
	Meses         []tipos.Decimal `json:"meses"`
}

// DistribuicaoRendaDto divide as despesas do ano pela natureza.
type DistribuicaoRendaDto struct {
	Receitas        tipos.Decimal `json:"receitas"`
	Essenciais      tipos.Decimal `json:"essenciais"`
	EstiloDeVida    tipos.Decimal `json:"estiloDeVida"`
	NaoClassificado tipos.Decimal `json:"naoClassificado"`
	Poupanca        tipos.Decimal `json:"poupanca"`
}

// SaudeFinanceiraDto são os indicadores de saúde financeira.
type SaudeFinanceiraDto struct {
	PatrimonioAtual           tipos.Decimal  `json:"patrimonioAtual"`
	ReservaLiquida            tipos.Decimal  `json:"reservaLiquida"`
	GastoMensalMedio          tipos.Decimal  `json:"gastoMensalMedio"`
	GastoEssencialMensalMedio tipos.Decimal  `json:"gastoEssencialMensalMedio"`
	MesesDeReserva            *tipos.Decimal `json:"mesesDeReserva"`
	MesesDeReservaEssencial   *tipos.Decimal `json:"mesesDeReservaEssencial"`
	RendaMensalMedia          tipos.Decimal  `json:"rendaMensalMedia"`
	ParcelasMensais           tipos.Decimal  `json:"parcelasMensais"`
	ComprometimentoParcelas   *tipos.Decimal `json:"comprometimentoParcelas"`
	MesesAnalisados           int            `json:"mesesAnalisados"`
	ParcelasFinanciamentos    tipos.Decimal  `json:"parcelasFinanciamentos"`
	ParcelasEmprestimos       tipos.Decimal  `json:"parcelasEmprestimos"`
	DividaFinanciamentos      tipos.Decimal  `json:"dividaFinanciamentos"`
	DividaEmprestimos         tipos.Decimal  `json:"dividaEmprestimos"`
}

// PainelAnualDto é a visão anual comparativa com a saúde financeira.
type PainelAnualDto struct {
	Ano                        int                       `json:"ano"`
	AnoComparacao              int                       `json:"anoComparacao"`
	MesLimite                  int                       `json:"mesLimite"`
	AnosDisponiveis            []int                     `json:"anosDisponiveis"`
	Resumo                     ResumoAnoDto              `json:"resumo"`
	ResumoAnteriorMesmoPeriodo ResumoAnoDto              `json:"resumoAnteriorMesmoPeriodo"`
	ResumoAnteriorCompleto     ResumoAnoDto              `json:"resumoAnteriorCompleto"`
	Meses                      []MesComparativoDto       `json:"meses"`
	Categorias                 []CategoriaComparativaDto `json:"categorias"`
	Distribuicao               DistribuicaoRendaDto      `json:"distribuicao"`
	Saude                      SaudeFinanceiraDto        `json:"saude"`
}

// Credito são os totais de crédito do Open Finance que a saúde financeira usa (emprestimos.Totais).
type Credito struct {
	SaldoFinanciamentos, SaldoEmprestimos, ParcelaMensalFinanciamentos, ParcelaMensalEmprestimos tipos.Decimal
}

// Patrimonio é o que a visão anual recebe já calculado de outros assuntos (o pacote de regra não os importa): o
// patrimônio de hoje (contas, investimentos, menos as dívidas), a reserva líquida e os totais de crédito.
type Patrimonio struct {
	Atual, Reserva tipos.Decimal
	Credito        Credito
}

// categoriaComNatureza são as colunas de Categoria que a visão anual usa.
type categoriaComNatureza struct {
	nome, cor string
	paiId     *int64
	natureza  *int64
}

// raiz é a categoria em que uma despesa conta na visão anual.
type raiz struct {
	id        *int64
	nome, cor string
	natureza  *int64
}

// PainelAnual é o PainelAnualAsync do C#: o ano comparado com outro, mês a mês, com o patrimônio estimado no fim de
// cada mês, as categorias pela raiz, a distribuição da renda pela natureza e a saúde financeira.
//   - mesLimite: 12 no ano passado, o mês atual no ano atual, 0 no ano futuro; o resumo soma os meses até ele, e o do
//     ano de comparação, os mesmos meses (no mínimo janeiro) e o ano inteiro;
//   - taxa de poupança: (receitas − despesas) ÷ receitas × 100, em uma casa (0 sem receitas);
//   - patrimônio no fim de um mês: o de hoje menos o resultado dos meses depois dele, até hoje, em duas casas;
//   - categorias: a subcategoria conta no pai (a natureza é a dela ou, sem natureza, a do pai), do maior valor para o
//     menor e, no empate, pelo ano de comparação.
func (s *Servico) PainelAnual(ctx context.Context, ano, anoComparacao int, p Patrimonio) (PainelAnualDto, error) {
	hoje := s.hoje()
	mesLimite := 0
	switch {
	case ano < hoje.Ano():
		mesLimite = 12
	case ano == hoje.Ano():
		mesLimite = int(hoje.Mes())
	}
	anos, err := s.anosDisponiveis(ctx, hoje.Ano())
	if err != nil {
		return PainelAnualDto{}, err
	}
	fluxoAno, err := s.FluxoMensal(ctx, tipos.NovaData(ano, 1, 1), tipos.NovaData(ano, 12, 31))
	if err != nil {
		return PainelAnualDto{}, err
	}
	fluxoAnterior, err := s.FluxoMensal(ctx, tipos.NovaData(anoComparacao, 1, 1), tipos.NovaData(anoComparacao, 12, 31))
	if err != nil {
		return PainelAnualDto{}, err
	}
	resumo, err := resumir(fluxoAno, mesLimite)
	if err != nil {
		return PainelAnualDto{}, err
	}
	resumoAnterior, err := resumir(fluxoAnterior, max(mesLimite, 1))
	if err != nil {
		return PainelAnualDto{}, err
	}
	resumoCompleto, err := resumir(fluxoAnterior, 12)
	if err != nil {
		return PainelAnualDto{}, err
	}

	ateHoje, err := s.FluxoMensal(ctx, tipos.NovaData(ano, 1, 1), hoje)
	if err != nil {
		return PainelAnualDto{}, err
	}
	meses := make([]MesComparativoDto, 0, 12)
	for mes := 1; mes <= 12; mes++ {
		atual, anterior := fluxoAno[mes-1], fluxoAnterior[mes-1]
		m := MesComparativoDto{Mes: mes, Receitas: atual.Receitas, Despesas: atual.Despesas,
			ReceitasAnterior: anterior.Receitas, DespesasAnterior: anterior.Despesas, Futuro: mes > mesLimite}
		if !m.Futuro {
			fimMes := relogio.UltimoDiaMes(ano, monthOf(mes))
			depois := tipos.Zero
			for _, r := range ateHoje {
				if tipos.NovaData(r.Ano, monthOf(r.Mes), 1).NumeroDoDia() > fimMes.NumeroDoDia() {
					depois = tipos.Somar(depois, tipos.Subtrair(r.Receitas, r.Despesas))
				}
			}
			v := tipos.Arredondar(tipos.Subtrair(p.Atual, depois), 2)
			m.PatrimonioEstimado = &v
		}
		meses = append(meses, m)
	}

	categorias, err := s.lerCategoriasComNatureza(ctx)
	if err != nil {
		return PainelAnualDto{}, err
	}
	raizDe := func(id *int64) raiz {
		if id == nil {
			return raiz{nome: "Sem categoria", cor: CorSemCategoriaDaProjecao}
		}
		c, ok := categorias[*id]
		if !ok {
			return raiz{nome: "Sem categoria", cor: CorSemCategoriaDaProjecao}
		}
		if c.paiId != nil {
			if pai, okPai := categorias[*c.paiId]; okPai {
				natureza := c.natureza
				if natureza == nil {
					natureza = pai.natureza
				}
				return raiz{c.paiId, pai.nome, pai.cor, natureza}
			}
		}
		return raiz{id, c.nome, c.cor, c.natureza}
	}

	despesasAno, err := s.despesasDoAno(ctx, ano, mesLimite)
	if err != nil {
		return PainelAnualDto{}, err
	}
	anteriores, err := s.Relevantes(ctx, tipos.NovaData(anoComparacao, 1, 1), relogio.UltimoDiaMes(anoComparacao, monthOf(max(mesLimite, 1))))
	if err != nil {
		return PainelAnualDto{}, err
	}
	despesasAnterior := somasPorCategoria(anteriores, func(l Lancamento) bool { return l.Tipo == transacoes.Despesa })

	type item struct {
		r               raiz
		valor, anterior tipos.Decimal
		meses           [12]tipos.Decimal
	}
	porRaiz := map[string]*item{}
	var ordem []string
	chave := func(r raiz) string {
		if r.id == nil {
			return "sem"
		}
		return strconv.FormatInt(*r.id, 10)
	}
	obter := func(r raiz) *item {
		k := chave(r)
		if it, ok := porRaiz[k]; ok {
			return it
		}
		it := &item{r: r, valor: tipos.Zero, anterior: tipos.Zero}
		for i := range it.meses {
			it.meses[i] = tipos.Zero
		}
		porRaiz[k] = it
		ordem = append(ordem, k)
		return it
	}
	for _, d := range despesasAno {
		it := obter(raizDe(d.categoriaId))
		it.valor = tipos.Somar(it.valor, d.soma)
		it.meses[d.mes-1] = tipos.Somar(it.meses[d.mes-1], d.soma)
	}
	for _, d := range despesasAnterior {
		it := obter(raizDe(d.categoriaId))
		it.anterior = tipos.Somar(it.anterior, negar(d.soma))
	}
	lista := make([]CategoriaComparativaDto, 0, len(ordem))
	for _, k := range ordem {
		it := porRaiz[k]
		lista = append(lista, CategoriaComparativaDto{CategoriaId: it.r.id, Nome: it.r.nome, Cor: it.r.cor,
			Natureza: it.r.natureza, Valor: it.valor, ValorAnterior: it.anterior, Meses: it.meses[:]})
	}
	slices.SortStableFunc(lista, func(a, b CategoriaComparativaDto) int {
		if c := b.Valor.Decimal().Cmp(a.Valor.Decimal()); c != 0 {
			return c
		}
		return b.ValorAnterior.Decimal().Cmp(a.ValorAnterior.Decimal())
	})

	somaNatureza := func(natureza *int64) tipos.Decimal {
		soma := tipos.Zero
		for _, d := range despesasAno {
			n := raizDe(d.categoriaId).natureza
			if (n == nil && natureza == nil) || (n != nil && natureza != nil && *n == *natureza) {
				soma = tipos.Somar(soma, d.soma)
			}
		}
		return soma
	}
	essencial, estilo := int64(NaturezaEssencial), int64(NaturezaEstiloDeVida)
	distribuicao := DistribuicaoRendaDto{Receitas: resumo.Receitas, Essenciais: somaNatureza(&essencial),
		EstiloDeVida: somaNatureza(&estilo), NaoClassificado: somaNatureza(nil), Poupanca: resumo.Resultado}

	var essenciais []int64
	for id, c := range categorias {
		eh := c.natureza != nil && *c.natureza == NaturezaEssencial
		if !eh && c.natureza == nil && c.paiId != nil {
			if pai, ok := categorias[*c.paiId]; ok && pai.natureza != nil && *pai.natureza == NaturezaEssencial {
				eh = true
			}
		}
		if eh {
			essenciais = append(essenciais, id)
		}
	}
	saude, err := s.saudeFinanceira(ctx, p, essenciais)
	if err != nil {
		return PainelAnualDto{}, err
	}
	return PainelAnualDto{Ano: ano, AnoComparacao: anoComparacao, MesLimite: mesLimite, AnosDisponiveis: anos,
		Resumo: resumo, ResumoAnteriorMesmoPeriodo: resumoAnterior, ResumoAnteriorCompleto: resumoCompleto, Meses: meses,
		Categorias: lista, Distribuicao: distribuicao, Saude: saude}, nil
}

func monthOf(mes int) time.Month { return time.Month(mes) }

// resumir soma os meses até o mês limite (o Sum do LINQ, a partir de 0).
func resumir(meses []MesValoresDto, ateMes int) (ResumoAnoDto, error) {
	r, d := tipos.Zero, tipos.Zero
	for _, m := range meses {
		if m.Mes <= ateMes {
			r, d = tipos.Somar(r, m.Receitas), tipos.Somar(d, m.Despesas)
		}
	}
	resultado := tipos.Subtrair(r, d)
	taxa := tipos.Zero
	if r.Decimal().IsPositive() {
		razao, err := tipos.Dividir(resultado, r)
		if err != nil {
			return ResumoAnoDto{}, err
		}
		taxa = tipos.Arredondar(tipos.Multiplicar(razao, tipos.DecimalDeInteiro(100)), 1)
	}
	return ResumoAnoDto{Receitas: r, Despesas: d, Resultado: resultado, TaxaPoupanca: taxa}, nil
}

// anosDisponiveis são os anos com alguma transação (de qualquer tipo), do mais novo para o mais velho, com o ano atual
// no começo se não houver transação nele.
func (s *Servico) anosDisponiveis(ctx context.Context, anoAtual int) ([]int, error) {
	// O ano é o começo do texto da data ("2026-10-14"); com 4 dígitos, a ordem do texto é a do número.
	linhas, err := s.db.QueryContext(ctx, `SELECT DISTINCT substr("Data", 1, 4) AS ano FROM "Transacao" ORDER BY ano DESC`)
	if err != nil {
		return nil, fmt.Errorf("ler os anos com transação: %w", err)
	}
	defer linhas.Close()
	anos := []int{}
	for linhas.Next() {
		var textoDoAno string
		if err := linhas.Scan(&textoDoAno); err != nil {
			return nil, fmt.Errorf("ler ano com transação: %w", err)
		}
		a, err := strconv.Atoi(textoDoAno)
		if err != nil {
			return nil, fmt.Errorf("ano da transação %q: %w", textoDoAno, err)
		}
		anos = append(anos, a)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os anos com transação: %w", err)
	}
	if !slices.Contains(anos, anoAtual) {
		anos = append([]int{anoAtual}, anos...)
	}
	return anos, nil
}

// gastoDoMes é um grupo (categoria, mês) das despesas do ano, com o valor já positivo.
type gastoDoMes struct {
	categoriaId *int64
	mes         int
	soma        tipos.Decimal
}

// despesasDoAno agrupa as despesas do ano até o mês limite por categoria e mês, como o GROUP BY do SQLite (sem
// categoria primeiro, depois pelo Id e pelo mês), com a soma no formato do banco, negada. No ano futuro, nada.
func (s *Servico) despesasDoAno(ctx context.Context, ano, mesLimite int) ([]gastoDoMes, error) {
	if mesLimite == 0 {
		return nil, nil
	}
	lancamentos, err := s.Relevantes(ctx, tipos.NovaData(ano, 1, 1), relogio.UltimoDiaMes(ano, monthOf(mesLimite)))
	if err != nil {
		return nil, err
	}
	type chave struct {
		semId bool
		id    int64
		mes   int
	}
	grupos := map[chave][]tipos.Decimal{}
	var chaves []chave
	for _, l := range lancamentos {
		if l.Tipo != transacoes.Despesa {
			continue
		}
		k := chave{semId: l.CategoriaId == nil, mes: int(l.Data.Mes())}
		if l.CategoriaId != nil {
			k.id = *l.CategoriaId
		}
		if _, ok := grupos[k]; !ok {
			chaves = append(chaves, k)
		}
		grupos[k] = append(grupos[k], l.Valor)
	}
	slices.SortFunc(chaves, func(a, b chave) int {
		if a.semId != b.semId {
			if a.semId {
				return -1
			}
			return 1
		}
		if a.id != b.id {
			return int(a.id - b.id)
		}
		return a.mes - b.mes
	})
	lista := make([]gastoDoMes, 0, len(chaves))
	for _, k := range chaves {
		d := gastoDoMes{mes: k.mes, soma: negar(somaOuZero(grupos[k]))}
		if !k.semId {
			id := k.id
			d.categoriaId = &id
		}
		lista = append(lista, d)
	}
	return lista, nil
}

func (s *Servico) lerCategoriasComNatureza(ctx context.Context) (map[int64]categoriaComNatureza, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Nome", "Cor", "CategoriaPaiId", "Natureza" FROM "Categoria"`)
	if err != nil {
		return nil, fmt.Errorf("ler as categorias da visão anual: %w", err)
	}
	defer linhas.Close()
	categorias := map[int64]categoriaComNatureza{}
	for linhas.Next() {
		var id int64
		var c categoriaComNatureza
		if err := linhas.Scan(&id, &c.nome, &c.cor, &c.paiId, &c.natureza); err != nil {
			return nil, fmt.Errorf("ler categoria da visão anual: %w", err)
		}
		categorias[id] = c
	}
	return categorias, linhas.Err()
}

// saudeFinanceira são os indicadores dos últimos 12 meses completos: o gasto médio (todo e o essencial), a renda e as
// parcelas, e os meses de reserva e o comprometimento da renda com parcelas (cartão, financiamentos e empréstimos).
func (s *Servico) saudeFinanceira(ctx context.Context, p Patrimonio, essenciais []int64) (SaudeFinanceiraDto, error) {
	inicio, fim, meses, err := s.MesesCompletos(ctx, 12)
	if err != nil {
		return SaudeFinanceiraDto{}, err
	}
	renda, gasto, gastoEssencial, parcelas := tipos.Zero, tipos.Zero, tipos.Zero, tipos.Zero
	if meses > 0 {
		if renda, parcelas, _, err = s.RendaEParcelas(ctx); err != nil {
			return SaudeFinanceiraDto{}, err
		}
		lancamentos, err := s.Relevantes(ctx, inicio, fim)
		if err != nil {
			return SaudeFinanceiraDto{}, err
		}
		divisor := tipos.DecimalDeInteiro(int64(meses))
		media := func(passa func(Lancamento) bool) (tipos.Decimal, error) {
			return tipos.Dividir(maiorDeDois(tipos.Zero, negar(somaOuZero(valoresDe(lancamentos, passa)))), divisor)
		}
		if gasto, err = media(func(l Lancamento) bool { return l.Tipo == transacoes.Despesa }); err != nil {
			return SaudeFinanceiraDto{}, err
		}
		if gastoEssencial, err = media(func(l Lancamento) bool {
			return l.Tipo == transacoes.Despesa && l.CategoriaId != nil && slices.Contains(essenciais, *l.CategoriaId)
		}); err != nil {
			return SaudeFinanceiraDto{}, err
		}
	}
	c := p.Credito
	total := tipos.Somar(tipos.Somar(parcelas, c.ParcelaMensalFinanciamentos), c.ParcelaMensalEmprestimos)
	saude := SaudeFinanceiraDto{
		PatrimonioAtual: tipos.Arredondar(p.Atual, 2), ReservaLiquida: tipos.Arredondar(p.Reserva, 2),
		GastoMensalMedio: tipos.Arredondar(gasto, 2), GastoEssencialMensalMedio: tipos.Arredondar(gastoEssencial, 2),
		RendaMensalMedia: tipos.Arredondar(renda, 2), ParcelasMensais: tipos.Arredondar(parcelas, 2), MesesAnalisados: meses,
		ParcelasFinanciamentos: c.ParcelaMensalFinanciamentos, ParcelasEmprestimos: c.ParcelaMensalEmprestimos,
		DividaFinanciamentos: c.SaldoFinanciamentos, DividaEmprestimos: c.SaldoEmprestimos,
	}
	razao := func(a, b tipos.Decimal, vezes int64, casas int32) (*tipos.Decimal, error) {
		if !b.Decimal().IsPositive() {
			return nil, nil
		}
		v, err := tipos.Dividir(a, b)
		if err != nil {
			return nil, err
		}
		if vezes != 1 {
			v = tipos.Multiplicar(v, tipos.DecimalDeInteiro(vezes))
		}
		v = tipos.Arredondar(v, casas)
		return &v, nil
	}
	if saude.MesesDeReserva, err = razao(p.Reserva, gasto, 1, 1); err != nil {
		return SaudeFinanceiraDto{}, err
	}
	if saude.MesesDeReservaEssencial, err = razao(p.Reserva, gastoEssencial, 1, 1); err != nil {
		return SaudeFinanceiraDto{}, err
	}
	if saude.ComprometimentoParcelas, err = razao(total, renda, 100, 1); err != nil {
		return SaudeFinanceiraDto{}, err
	}
	return saude, nil
}
