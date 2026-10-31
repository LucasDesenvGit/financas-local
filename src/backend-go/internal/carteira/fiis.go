package carteira

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"slices"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/respostas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/selic"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// A análise dos fundos imobiliários (FiiService do C#): renda (DY, yield on cost), cada compra comparada com hoje, proventos
// por mês, evolução do patrimônio e rentabilidade comparada com Selic e IFIX.

const (
	mesesDosProventos = 24
	maximoDeMeses     = 180
	// #173 c): o provento cai até tantos dias depois da data-com.
	diasDoPagamentoDoProvento = 40
	mesesDoHistorico          = 36
	semSegmento               = "Sem segmento"
)

var (
	// toleranciaDoProvento (#173 c): diferença aceita entre o crédito e cotas × valor por cota (arredondamento do banco).
	toleranciaDoProvento = mustDecimal("0.05")
	// pisoDoRetorno: o retorno do mês não fica abaixo disso.
	pisoDoRetorno = mustDecimal("-0.95")
	// direitoDeSubscricao é o ClassificacaoAtivo.EhSubscricao.
	direitoDeSubscricao = regexp.MustCompile(`^[A-Z]{4}1[2-5]$`)
)

// FiiDto é o do C#.
type FiiDto struct {
	Id                        int64          `json:"id"`
	Ticker                    string         `json:"ticker"`
	Nome                      *string        `json:"nome"`
	Segmento                  *string        `json:"segmento"`
	SegmentoSugerido          bool           `json:"segmentoSugerido"`
	Quantidade                tipos.Decimal  `json:"quantidade"`
	PrecoMedio                tipos.Decimal  `json:"precoMedio"`
	Custo                     tipos.Decimal  `json:"custo"`
	PrecoAtual                *tipos.Decimal `json:"precoAtual"`
	VariacaoDia               *tipos.Decimal `json:"variacaoDia"`
	ValorMercado              tipos.Decimal  `json:"valorMercado"`
	Valorizacao               tipos.Decimal  `json:"valorizacao"`
	ValorizacaoPercentual     *tipos.Decimal `json:"valorizacaoPercentual"`
	UltimoRendimento          *tipos.Decimal `json:"ultimoRendimento"`
	DataComUltimo             *tipos.Data    `json:"dataComUltimo"`
	RendimentosPorCota12Meses tipos.Decimal  `json:"rendimentosPorCota12Meses"`
	DyMensal                  *tipos.Decimal `json:"dyMensal"`
	Dy12Meses                 *tipos.Decimal `json:"dy12Meses"`
	YieldOnCost12Meses        *tipos.Decimal `json:"yieldOnCost12Meses"`
	YieldOnCostMensal         *tipos.Decimal `json:"yieldOnCostMensal"`
	RendaMensalEstimada       tipos.Decimal  `json:"rendaMensalEstimada"`
	Proventos12Meses          tipos.Decimal  `json:"proventos12Meses"`
	ProventosTotal            tipos.Decimal  `json:"proventosTotal"`
	RecebidoExtrato12Meses    tipos.Decimal  `json:"recebidoExtrato12Meses"`
	PesoCarteira              tipos.Decimal  `json:"pesoCarteira"`
	FonteCotacao              *string        `json:"fonteCotacao"`
	PrecoManual               bool           `json:"precoManual"`
}

// LoteFiiDto é cada compra de FII comparada com a cotação e a renda de hoje.
type LoteFiiDto struct {
	OperacaoId             int64          `json:"operacaoId"`
	AtivoId                int64          `json:"ativoId"`
	Ticker                 string         `json:"ticker"`
	Data                   tipos.Data     `json:"data"`
	Quantidade             tipos.Decimal  `json:"quantidade"`
	PrecoPago              tipos.Decimal  `json:"precoPago"`
	ValorPago              tipos.Decimal  `json:"valorPago"`
	PrecoAtual             *tipos.Decimal `json:"precoAtual"`
	ValorAtual             *tipos.Decimal `json:"valorAtual"`
	VariacaoPercentual     *tipos.Decimal `json:"variacaoPercentual"`
	DyNaCompra             *tipos.Decimal `json:"dyNaCompra"`
	YieldOnCost12Meses     *tipos.Decimal `json:"yieldOnCost12Meses"`
	DyAtual12Meses         *tipos.Decimal `json:"dyAtual12Meses"`
	RendaMensalLote        tipos.Decimal  `json:"rendaMensalLote"`
	ProventosRecebidosLote tipos.Decimal  `json:"proventosRecebidosLote"`
	Origem                 int64          `json:"origem"`
}

// ValorAtivoDto é o do C#.
type ValorAtivoDto struct {
	Ticker string        `json:"ticker"`
	Valor  tipos.Decimal `json:"valor"`
}

// ProventoMesDto é o do C#.
type ProventoMesDto struct {
	Ano             int             `json:"ano"`
	Mes             int             `json:"mes"`
	Total           tipos.Decimal   `json:"total"`
	RecebidoExtrato tipos.Decimal   `json:"recebidoExtrato"`
	PorAtivo        []ValorAtivoDto `json:"porAtivo"`
}

// EvolucaoFiiMesDto é o do C#.
type EvolucaoFiiMesDto struct {
	Ano          int           `json:"ano"`
	Mes          int           `json:"mes"`
	Aplicado     tipos.Decimal `json:"aplicado"`
	ValorMercado tipos.Decimal `json:"valorMercado"`
}

// RendimentoPorFiiDto é quanto um fundo pagou no mês (#173 c); Ticker nil é o crédito sem fundo identificado.
type RendimentoPorFiiDto struct {
	Ticker *string       `json:"ticker"`
	Valor  tipos.Decimal `json:"valor"`
}

// MesAMesFiiDto é o mês da carteira de FIIs como no app da corretora (#173).
type MesAMesFiiDto struct {
	Ano                  int                   `json:"ano"`
	Mes                  int                   `json:"mes"`
	Aportes              tipos.Decimal         `json:"aportes"`
	Subscricoes          tipos.Decimal         `json:"subscricoes"`
	Resgates             tipos.Decimal         `json:"resgates"`
	RendimentosRecebidos tipos.Decimal         `json:"rendimentosRecebidos"`
	RendimentosEstimados tipos.Decimal         `json:"rendimentosEstimados"`
	Valorizacao          tipos.Decimal         `json:"valorizacao"`
	Saldo                tipos.Decimal         `json:"saldo"`
	PorFii               []RendimentoPorFiiDto `json:"porFii"`
	IdsAportes           []int64               `json:"idsAportes"`
	IdsRendimentos       []int64               `json:"idsRendimentos"`
}

// RentabilidadeMesDto é o do C#.
type RentabilidadeMesDto struct {
	Ano      int            `json:"ano"`
	Mes      int            `json:"mes"`
	Carteira tipos.Decimal  `json:"carteira"`
	Selic    tipos.Decimal  `json:"selic"`
	Ifix     *tipos.Decimal `json:"ifix"`
}

// ResumoFiisDto é o do C#.
type ResumoFiisDto struct {
	ValorMercado           tipos.Decimal  `json:"valorMercado"`
	Custo                  tipos.Decimal  `json:"custo"`
	Valorizacao            tipos.Decimal  `json:"valorizacao"`
	ValorizacaoPercentual  *tipos.Decimal `json:"valorizacaoPercentual"`
	Renda12Meses           tipos.Decimal  `json:"renda12Meses"`
	RendaMediaMensal       tipos.Decimal  `json:"rendaMediaMensal"`
	RendaMensalEstimada    tipos.Decimal  `json:"rendaMensalEstimada"`
	Dy12Meses              *tipos.Decimal `json:"dy12Meses"`
	YieldOnCost12Meses     *tipos.Decimal `json:"yieldOnCost12Meses"`
	RecebidoExtrato12Meses tipos.Decimal  `json:"recebidoExtrato12Meses"`
	RentabilidadeTotal     *tipos.Decimal `json:"rentabilidadeTotal"`
	RentabilidadeSelic     *tipos.Decimal `json:"rentabilidadeSelic"`
	RentabilidadeIfix      *tipos.Decimal `json:"rentabilidadeIfix"`
	QuantidadeFiis         int            `json:"quantidadeFiis"`
}

// PontoPrecoDto é o do C#.
type PontoPrecoDto struct {
	Ano        int           `json:"ano"`
	Mes        int           `json:"mes"`
	Fechamento tipos.Decimal `json:"fechamento"`
}

// HistoricoPrecoDto é o do C#.
type HistoricoPrecoDto struct {
	Ticker string          `json:"ticker"`
	Pontos []PontoPrecoDto `json:"pontos"`
}

// PainelFiisDto é o do C#.
type PainelFiisDto struct {
	Resumo               ResumoFiisDto                 `json:"resumo"`
	Fiis                 []FiiDto                      `json:"fiis"`
	Lotes                []LoteFiiDto                  `json:"lotes"`
	Proventos            []ProventoMesDto              `json:"proventos"`
	Evolucao             []EvolucaoFiiMesDto           `json:"evolucao"`
	Rentabilidade        []RentabilidadeMesDto         `json:"rentabilidade"`
	Segmentos            []DistribuicaoInvestimentoDto `json:"segmentos"`
	Historicos           []HistoricoPrecoDto           `json:"historicos"`
	SegmentosDisponiveis []string                      `json:"segmentosDisponiveis"`
	MesAMes              []MesAMesFiiDto               `json:"mesAMes"`
}

type fechamentoMensal struct {
	mes   tipos.Data
	valor tipos.Decimal
}

// fundo é um FII com operações, na ordem de Calcular, os proventos até hoje e os fechamentos mensais, por mês.
type fundo struct {
	ativo     *Ativo
	operacoes []Operacao
	proventos []Provento
	mensais   []fechamentoMensal
	posicao   Posicao
}

func (c *conta) posicaoEm(f *fundo, data *tipos.Data) Posicao {
	p, err := Calcular(f.operacoes, data)
	if err != nil && c.err == nil {
		c.err = err
	}
	return p
}

func (c *conta) quantidadeEm(f *fundo, data tipos.Data) tipos.Decimal {
	return c.posicaoEm(f, &data).Quantidade
}

func ultimoDoMes(mes tipos.Data) tipos.Data { return relogio.UltimoDiaMes(mes.Ano(), mes.Mes()) }

func (f *fundo) mensalEm(mes tipos.Data) *tipos.Decimal {
	i := slices.IndexFunc(f.mensais, func(m fechamentoMensal) bool { return m.mes.Igual(mes) })
	if i < 0 {
		return nil
	}
	return &f.mensais[i].valor
}

// precoNoMes é o preço da cota no mês (fechamento mensal; no mês atual, a cotação atual).
func (c *conta) precoNoMes(f *fundo, mes, mesAtual tipos.Data) tipos.Decimal {
	if mes.Igual(mesAtual) && f.ativo.PrecoAtual != nil {
		return *f.ativo.PrecoAtual
	}
	var anterior *tipos.Decimal
	for i := range f.mensais {
		if !f.mensais[i].mes.Depois(mes) {
			anterior = &f.mensais[i].valor
		}
	}
	if anterior != nil {
		return *anterior
	}
	ultimo := ultimoDoMes(mes)
	if medio := c.posicaoEm(f, &ultimo).PrecoMedio; positivo(medio) {
		return medio
	}
	if f.ativo.PrecoAtual != nil {
		return *f.ativo.PrecoAtual
	}
	return tipos.Zero
}

// proventoEsperado é cotas na data-com × valor por cota.
type proventoEsperado struct {
	ticker  string
	dataCom tipos.Data
	valor   tipos.Decimal
}

// ligacaoDeProventos procura o provento esperado de cada crédito. Cada esperado serve a um crédito só.
type ligacaoDeProventos struct {
	esperados []proventoEsperado
	usados    map[int]bool
}

// melhor é o esperado livre que dá o mesmo valor (até a tolerância), pago até 40 dias depois da data-com: o mais perto do
// valor e, no empate, o mais perto da data. -1 se não houver.
func (l *ligacaoDeProventos) melhor(data tipos.Data, valor tipos.Decimal, ticker *string) int {
	achado, menorDiferenca, menorPrazo := -1, tipos.Zero, 0
	for i, e := range l.esperados {
		diferenca := valorAbsoluto(tipos.Subtrair(e.valor, valor))
		if l.usados[i] || (ticker != nil && e.ticker != *ticker) || e.dataCom.Depois(data) || data.Depois(e.dataCom.SomarDias(diasDoPagamentoDoProvento)) ||
			maiorQue(diferenca, toleranciaDoProvento) {
			continue
		}
		prazo := data.NumeroDoDia() - e.dataCom.NumeroDoDia()
		if achado < 0 || menorQue(diferenca, menorDiferenca) || (diferenca.Decimal().Equal(menorDiferenca.Decimal()) && prazo < menorPrazo) {
			achado, menorDiferenca, menorPrazo = i, diferenca, prazo
		}
	}
	return achado
}

func porDataDoCredito(a, b CreditoRecebido) int {
	return cmp.Compare(a.Data.NumeroDoDia(), b.Data.NumeroDoDia())
}

// fundoDeCadaCredito é o FundoDeCadaCredito (#173 c): de que fundo é cada crédito de provento. O crédito com o código do
// fundo já diz. O "Valor recebido de Investimentos" (sem código) é ligado ao provento que dá o mesmo valor. O que não liga
// fica sem fundo, sem sumir da soma do mês.
func (c *conta) fundoDeCadaCredito(fundos []*fundo, comCodigo []ProventoRecebido, creditos []CreditoRecebido) map[int64]*string {
	l := ligacaoDeProventos{usados: map[int]bool{}}
	for _, f := range fundos {
		for _, p := range f.proventos {
			valor := tipos.Arredondar(tipos.Multiplicar(c.quantidadeEm(f, p.DataCom), p.ValorPorCota), 2)
			if positivo(valor) {
				l.esperados = append(l.esperados, proventoEsperado{f.ativo.Ticker, p.DataCom, valor})
			}
		}
	}
	resultado := map[int64]*string{}
	// Primeiro os que já dizem o fundo, para os sem código não pegarem o provento deles.
	ordenados := slices.Clone(comCodigo)
	slices.SortStableFunc(ordenados, func(a, b ProventoRecebido) int { return cmp.Compare(a.Data.NumeroDoDia(), b.Data.NumeroDoDia()) })
	for _, p := range ordenados {
		ticker := p.Ticker
		resultado[p.Id] = &ticker
		if i := l.melhor(p.Data, p.Valor, &ticker); i >= 0 {
			l.usados[i] = true
		}
	}
	var semCodigo []CreditoRecebido
	for _, r := range creditos {
		if _, ok := resultado[r.Id]; !ok {
			semCodigo = append(semCodigo, r)
		}
	}
	slices.SortStableFunc(semCodigo, porDataDoCredito)
	for _, r := range semCodigo {
		resultado[r.Id] = nil
		if i := l.melhor(r.Data, r.Valor, nil); i >= 0 {
			l.usados[i] = true
			resultado[r.Id] = &l.esperados[i].ticker
		}
	}
	return resultado
}

func (s *Servico) lerMensaisDosFundos(ctx context.Context, fundos []*fundo) error {
	linhas, err := s.db.QueryContext(ctx, `SELECT "AtivoId", "Mes", "Fechamento" FROM "PrecoMensalAtivo"`)
	if err != nil {
		return fmt.Errorf("ler os fechamentos mensais: %w", err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var id int64
		var m fechamentoMensal
		if err := linhas.Scan(&id, &m.mes, &m.valor); err != nil {
			return fmt.Errorf("ler fechamento mensal: %w", err)
		}
		if i := slices.IndexFunc(fundos, func(f *fundo) bool { return f.ativo.Id == id }); i >= 0 {
			fundos[i].mensais = append(fundos[i].mensais, m)
		}
	}
	if err := linhas.Err(); err != nil {
		return fmt.Errorf("ler os fechamentos mensais: %w", err)
	}
	for _, f := range fundos {
		slices.SortStableFunc(f.mensais, func(a, b fechamentoMensal) int { return cmp.Compare(a.mes.NumeroDoDia(), b.mes.NumeroDoDia()) })
	}
	return nil
}

func (s *Servico) lerIfix(ctx context.Context) ([]fechamentoMensal, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Mes", "Fechamento" FROM "IndiceMensal" WHERE "Codigo" = ?`, cotacoes.CodigoIfix)
	if err != nil {
		return nil, fmt.Errorf("ler o IFIX: %w", err)
	}
	defer linhas.Close()
	var lista []fechamentoMensal
	for linhas.Next() {
		var m fechamentoMensal
		if err := linhas.Scan(&m.mes, &m.valor); err != nil {
			return nil, fmt.Errorf("ler o IFIX: %w", err)
		}
		lista = append(lista, m)
	}
	return lista, linhas.Err()
}

func (s *Servico) lerFundos(ctx context.Context, hoje tipos.Data) ([]*fundo, error) {
	ativos, err := s.ativosPeloId(ctx)
	if err != nil {
		return nil, err
	}
	var fundos []*fundo
	for i := range ativos {
		a := &ativos[i]
		if a.Tipo != tipoFii || len(a.Operacoes) == 0 {
			continue
		}
		f := &fundo{ativo: a, operacoes: ordenarParaPosicao(a.Operacoes)}
		for _, p := range a.Proventos {
			if !p.DataCom.Depois(hoje) {
				f.proventos = append(f.proventos, p)
			}
		}
		slices.SortStableFunc(f.proventos, func(x, y Provento) int { return cmp.Compare(x.DataCom.NumeroDoDia(), y.DataCom.NumeroDoDia()) })
		if f.posicao, err = Calcular(f.operacoes, nil); err != nil {
			return nil, err
		}
		fundos = append(fundos, f)
	}
	return fundos, s.lerMensaisDosFundos(ctx, fundos)
}

// pct é o Pct do C#: a parte em % do total, a 2 casas; nil sem um dos dois ou com o total zerado.
func (c *conta) pct(parte, total *tipos.Decimal) *tipos.Decimal {
	if parte == nil || total == nil || !positivo(*total) {
		return nil
	}
	return opcional(c.percentual(*parte, *total, 2))
}

func proventosDesde(lista []Provento, depoisDe tipos.Data) tipos.Decimal {
	soma := tipos.Zero
	for _, p := range lista {
		if p.DataCom.Depois(depoisDe) {
			soma = tipos.Somar(soma, p.ValorPorCota)
		}
	}
	return soma
}

func valorOuZero(d *tipos.Decimal) tipos.Decimal {
	if d == nil {
		return tipos.Zero
	}
	return *d
}

func recebidoEntre[T any](lista []T, de, ate tipos.Data, data func(T) tipos.Data, valor func(T) tipos.Decimal) tipos.Decimal {
	soma := tipos.Zero
	for _, r := range lista {
		if d := data(r); !d.Antes(de) && !d.Depois(ate) {
			soma = tipos.Somar(soma, valor(r))
		}
	}
	return soma
}

// fundoDto é a linha de um fundo em aberto.
func (c *conta) fundoDto(f *fundo, recebidos []ProventoRecebido, totalMercado tipos.Decimal, ha12, hoje tipos.Data) FiiDto {
	a, p := f.ativo, f.posicao
	preco := a.PrecoAtual
	valorMercado := tipos.Multiplicar(p.Quantidade, valorOu(preco, p.PrecoMedio))
	var ultimo *Provento
	if n := len(f.proventos); n > 0 {
		ultimo = &f.proventos[n-1]
	}
	porCota12 := proventosDesde(f.proventos, ha12)
	proventos12, proventosTotal := tipos.Zero, tipos.Zero
	for _, pr := range f.proventos {
		valor := tipos.Multiplicar(c.quantidadeEm(f, pr.DataCom), pr.ValorPorCota)
		proventosTotal = tipos.Somar(proventosTotal, valor)
		if pr.DataCom.Depois(ha12) {
			proventos12 = tipos.Somar(proventos12, valor)
		}
	}
	extrato12 := tipos.Zero
	for _, r := range recebidos {
		if r.Ticker == a.Ticker && r.Data.Depois(ha12) {
			extrato12 = tipos.Somar(extrato12, r.Valor)
		}
	}
	segmento := a.Segmento
	if segmento == nil {
		segmento = sugerirSegmento(a.Ticker)
	}
	d := FiiDto{Id: a.Id, Ticker: a.Ticker, Nome: a.Nome, Segmento: segmento, SegmentoSugerido: a.Segmento == nil && segmento != nil, Quantidade: p.Quantidade,
		PrecoMedio: tipos.Arredondar(p.PrecoMedio, 4), Custo: tipos.Arredondar(p.Custo, 2), PrecoAtual: preco, ValorMercado: tipos.Arredondar(valorMercado, 2),
		Valorizacao: tipos.Arredondar(tipos.Subtrair(valorMercado, p.Custo), 2), RendimentosPorCota12Meses: tipos.Arredondar(porCota12, 6),
		Dy12Meses: c.pct(&porCota12, preco), YieldOnCost12Meses: c.pct(&porCota12, &p.PrecoMedio), Proventos12Meses: tipos.Arredondar(proventos12, 2),
		ProventosTotal: tipos.Arredondar(proventosTotal, 2), RecebidoExtrato12Meses: tipos.Arredondar(extrato12, 2), PesoCarteira: tipos.Zero, FonteCotacao: a.FonteCotacao,
		PrecoManual: a.PrecoManual, RendaMensalEstimada: tipos.Arredondar(tipos.Multiplicar(tipos.Zero, p.Quantidade), 2)}
	if preco != nil && a.PrecoAnterior != nil && positivo(*a.PrecoAnterior) {
		d.VariacaoDia = opcional(c.variacao(*preco, *a.PrecoAnterior))
	}
	if positivo(p.Custo) {
		d.ValorizacaoPercentual = opcional(c.percentual(tipos.Subtrair(valorMercado, p.Custo), p.Custo, 2))
	}
	if ultimo != nil {
		d.UltimoRendimento, d.DataComUltimo = &ultimo.ValorPorCota, &ultimo.DataCom
		d.DyMensal, d.YieldOnCostMensal = c.pct(&ultimo.ValorPorCota, preco), c.pct(&ultimo.ValorPorCota, &p.PrecoMedio)
		d.RendaMensalEstimada = tipos.Arredondar(tipos.Multiplicar(ultimo.ValorPorCota, p.Quantidade), 2)
	}
	if positivo(totalMercado) {
		d.PesoCarteira = c.percentual(valorMercado, totalMercado, 2)
	}
	return d
}

func valorOu(d *tipos.Decimal, outro tipos.Decimal) tipos.Decimal {
	if d == nil {
		return outro
	}
	return *d
}

// lotesDoFundo são as compras comparadas com hoje.
func (c *conta) lotesDoFundo(f *fundo, ha12 tipos.Data) []LoteFiiDto {
	var lotes []LoteFiiDto
	preco := f.ativo.PrecoAtual
	porCota12 := proventosDesde(f.proventos, ha12)
	ultimo := tipos.Zero
	if n := len(f.proventos); n > 0 {
		ultimo = f.proventos[n-1].ValorPorCota
	}
	for _, o := range f.operacoes {
		if o.Tipo != operacaoCompra {
			continue
		}
		// Renda dos 12 meses anteriores à compra, sobre o preço pago (o "DY que você comprou").
		anoAntes := o.Data.SomarAnos(-1)
		antes, temAntes, recebidos := tipos.Zero, false, tipos.Zero
		for _, p := range f.proventos {
			if !p.DataCom.Depois(o.Data) && p.DataCom.Depois(anoAntes) {
				antes, temAntes = tipos.Somar(antes, p.ValorPorCota), true
			}
			if !p.DataCom.Antes(o.Data) {
				recebidos = tipos.Somar(recebidos, p.ValorPorCota)
			}
		}
		l := LoteFiiDto{OperacaoId: o.Id, AtivoId: f.ativo.Id, Ticker: f.ativo.Ticker, Data: o.Data, Quantidade: o.Quantidade, PrecoPago: o.PrecoUnitario,
			ValorPago: tipos.Arredondar(tipos.Somar(tipos.Multiplicar(o.Quantidade, o.PrecoUnitario), o.Taxas), 2), PrecoAtual: preco,
			YieldOnCost12Meses: c.pct(&porCota12, &o.PrecoUnitario), DyAtual12Meses: c.pct(&porCota12, preco),
			RendaMensalLote: tipos.Arredondar(tipos.Multiplicar(ultimo, o.Quantidade), 2), ProventosRecebidosLote: tipos.Arredondar(tipos.Multiplicar(recebidos, o.Quantidade), 2), Origem: o.Origem}
		if len(f.proventos) > 0 && !f.proventos[0].DataCom.Depois(o.Data.SomarMeses(-11)) && temAntes {
			l.DyNaCompra = c.pct(&antes, &o.PrecoUnitario)
		}
		if preco != nil {
			l.ValorAtual = opcional(tipos.Arredondar(tipos.Multiplicar(o.Quantidade, *preco), 2))
			if positivo(o.PrecoUnitario) {
				l.VariacaoPercentual = opcional(c.variacao(*preco, o.PrecoUnitario))
			}
		}
		lotes = append(lotes, l)
	}
	return lotes
}

// proventosDoMes é o que os fundos pagaram no mês, pela data-com.
func (c *conta) proventosDoFundoNoMes(f *fundo, mes, fim tipos.Data) tipos.Decimal {
	soma := tipos.Zero
	for _, p := range f.proventos {
		if !p.DataCom.Antes(mes) && !p.DataCom.Depois(fim) {
			soma = tipos.Somar(soma, tipos.Multiplicar(c.quantidadeEm(f, p.DataCom), p.ValorPorCota))
		}
	}
	return soma
}

func (c *conta) proventosPorMes(fundos []*fundo, recebidos []ProventoRecebido, primeiraData, mesAtual tipos.Data) []ProventoMesDto {
	lista := []ProventoMesDto{}
	inicio := relogio.PrimeiroDiaMes(primeiraData.Ano(), primeiraData.Mes())
	if limite := mesAtual.SomarMeses(-(mesesDosProventos - 1)); inicio.Antes(limite) {
		inicio = limite
	}
	for mes := inicio; !mes.Depois(mesAtual); mes = mes.SomarMeses(1) {
		fim := ultimoDoMes(mes)
		porAtivo := []ValorAtivoDto{}
		for _, f := range fundos {
			if soma := c.proventosDoFundoNoMes(f, mes, fim); positivo(soma) {
				porAtivo = append(porAtivo, ValorAtivoDto{f.ativo.Ticker, tipos.Arredondar(soma, 2)})
			}
		}
		extrato := recebidoEntre(recebidos, mes, fim, func(r ProventoRecebido) tipos.Data { return r.Data }, func(r ProventoRecebido) tipos.Decimal { return r.Valor })
		total := somarPor(porAtivo, func(v ValorAtivoDto) tipos.Decimal { return v.Valor })
		slices.SortStableFunc(porAtivo, func(a, b ValorAtivoDto) int { return b.Valor.Decimal().Cmp(a.Valor.Decimal()) })
		lista = append(lista, ProventoMesDto{mes.Ano(), int(mes.Mes()), total, tipos.Arredondar(extrato, 2), porAtivo})
	}
	return lista
}

// movimentoDoMes é o que as operações dos fundos fizeram no mês.
type movimentoDoMes struct {
	aplicado, valor, fluxo, proventos, aportes, subscricoes, resgates tipos.Decimal
	ids                                                               []int64
}

func (c *conta) movimentoNoMes(fundos []*fundo, mes, fim, mesAtual tipos.Data) movimentoDoMes {
	m := movimentoDoMes{tipos.Zero, tipos.Zero, tipos.Zero, tipos.Zero, tipos.Zero, tipos.Zero, tipos.Zero, []int64{}}
	for _, f := range fundos {
		posicao := c.posicaoEm(f, &fim)
		m.aplicado = tipos.Somar(m.aplicado, posicao.Custo)
		if positivo(posicao.Quantidade) {
			m.valor = tipos.Somar(m.valor, tipos.Multiplicar(posicao.Quantidade, c.precoNoMes(f, mes, mesAtual)))
		}
		subscricao := direitoDeSubscricao.MatchString(f.ativo.Ticker)
		for _, o := range f.operacoes {
			if o.Data.Antes(mes) || o.Data.Depois(fim) {
				continue
			}
			if o.TransacaoId != nil { // o índice único não deixa repetir
				m.ids = append(m.ids, *o.TransacaoId)
			}
			// #173 a): cotas × preço, sem a taxa da B3, como as "Entradas" do app (a taxa fica no custo e no preço médio da
			// operação, o "valor final da compra").
			pago := tipos.Multiplicar(o.Quantidade, o.PrecoUnitario)
			switch {
			case o.Tipo != operacaoCompra:
				m.fluxo = tipos.Subtrair(m.fluxo, pago)
				m.resgates = tipos.Somar(m.resgates, pago)
			case subscricao:
				m.fluxo = tipos.Somar(m.fluxo, pago)
				m.subscricoes = tipos.Somar(m.subscricoes, pago)
			default:
				m.fluxo = tipos.Somar(m.fluxo, pago)
				m.aportes = tipos.Somar(m.aportes, pago)
			}
		}
		m.proventos = tipos.Somar(m.proventos, c.proventosDoFundoNoMes(f, mes, fim))
	}
	slices.Sort(m.ids)
	return m
}

// porFundo são os rendimentos recebidos no mês por fundo: os sem fundo no fim, e os maiores primeiro.
func porFundo(creditos []CreditoRecebido, fundoDoCredito map[int64]*string) []RendimentoPorFiiDto {
	lista := []RendimentoPorFiiDto{}
	for _, r := range creditos {
		ticker := fundoDoCredito[r.Id]
		i := slices.IndexFunc(lista, func(x RendimentoPorFiiDto) bool {
			return (x.Ticker == nil && ticker == nil) || (x.Ticker != nil && ticker != nil && *x.Ticker == *ticker)
		})
		if i < 0 {
			lista = append(lista, RendimentoPorFiiDto{ticker, tipos.Zero})
			i = len(lista) - 1
		}
		lista[i].Valor = tipos.Somar(lista[i].Valor, r.Valor)
	}
	for i := range lista {
		lista[i].Valor = tipos.Arredondar(lista[i].Valor, 2)
	}
	slices.SortStableFunc(lista, func(a, b RendimentoPorFiiDto) int {
		if a.Ticker == nil || b.Ticker == nil {
			return cmp.Compare(semTicker(a), semTicker(b))
		}
		return b.Valor.Decimal().Cmp(a.Valor.Decimal())
	})
	return lista
}

func semTicker(r RendimentoPorFiiDto) int {
	if r.Ticker == nil {
		return 1
	}
	return 0
}

func fechamentoDoMes(lista []fechamentoMensal, mes tipos.Data) *tipos.Decimal {
	if i := slices.IndexFunc(lista, func(m fechamentoMensal) bool { return m.mes.Igual(mes) }); i >= 0 {
		return &lista[i].valor
	}
	return nil
}

// historico é o que a evolução mensal acumula.
type historico struct {
	evolucao      []EvolucaoFiiMesDto
	rentabilidade []RentabilidadeMesDto
	mesAMes       []MesAMesFiiDto
}

// creditosEntre são os créditos do período, com os ids sem repetir, em ordem.
func creditosEntre(lista []CreditoRecebido, de, ate tipos.Data) ([]CreditoRecebido, []int64) {
	creditos, ids := []CreditoRecebido{}, []int64{}
	for _, r := range lista {
		if !r.Data.Antes(de) && !r.Data.Depois(ate) {
			creditos = append(creditos, r)
			ids = append(ids, r.Id)
		}
	}
	slices.Sort(ids)
	return creditos, ids
}

// evolucaoDosFundos é a evolução e a rentabilidade: ponderada no tempo (aportes no início do mês), proventos somados ao
// retorno, contra a Selic e o IFIX.
func (c *conta) evolucaoDosFundos(fundos []*fundo, creditos []CreditoRecebido, fundoDoCredito map[int64]*string, serie *selic.Serie, ifix []fechamentoMensal,
	primeiraData, hoje, mesAtual tipos.Data) historico {
	h := historico{[]EvolucaoFiiMesDto{}, []RentabilidadeMesDto{}, []MesAMesFiiDto{}}
	if len(fundos) == 0 {
		return h
	}
	inicio := relogio.PrimeiroDiaMes(primeiraData.Ano(), primeiraData.Mes())
	if limite := mesAtual.SomarMeses(-maximoDeMeses); inicio.Antes(limite) {
		inicio = limite
	}
	baseIfix := valorOuZero(fechamentoDoMes(ifix, inicio.SomarMeses(-1)))
	if !positivo(baseIfix) {
		baseIfix = valorOuZero(fechamentoDoMes(ifix, inicio))
	}
	acumulado, valorAnterior := um, tipos.Zero
	for mes := inicio; !mes.Depois(mesAtual); mes = mes.SomarMeses(1) {
		mesFim := ultimoDoMes(mes)
		fim := mesFim
		if fim.Depois(hoje) {
			fim = hoje
		}
		m := c.movimentoNoMes(fundos, mes, fim, mesAtual)

		basePeriodo := tipos.Somar(valorAnterior, m.fluxo)
		retorno := tipos.Zero
		if positivo(basePeriodo) {
			retorno = c.dividir(tipos.Subtrair(tipos.Somar(m.valor, m.proventos), basePeriodo), basePeriodo)
		}
		if menorQue(retorno, pisoDoRetorno) {
			retorno = pisoDoRetorno
		}
		acumulado = tipos.Multiplicar(acumulado, tipos.Somar(um, retorno))
		doMes, ids := creditosEntre(creditos, mes, mesFim)
		h.mesAMes = append(h.mesAMes, MesAMesFiiDto{Ano: mes.Ano(), Mes: int(mes.Mes()), Aportes: tipos.Arredondar(m.aportes, 2), Subscricoes: tipos.Arredondar(m.subscricoes, 2),
			Resgates: tipos.Arredondar(m.resgates, 2), RendimentosRecebidos: tipos.Arredondar(somarPor(doMes, func(r CreditoRecebido) tipos.Decimal { return r.Valor }), 2),
			RendimentosEstimados: tipos.Arredondar(m.proventos, 2), Valorizacao: tipos.Arredondar(tipos.Subtrair(m.valor, basePeriodo), 2), Saldo: tipos.Arredondar(m.valor, 2),
			PorFii: porFundo(doMes, fundoDoCredito), IdsAportes: m.ids, IdsRendimentos: ids})
		valorAnterior = m.valor

		ateSelic := fim.SomarDias(1)
		if fim.Igual(hoje) {
			ateSelic = hoje
		}
		selicAcumulada := tipos.Multiplicar(tipos.Subtrair(c.fator(serie, primeiraData, ateSelic), um), cem)
		rentabilidade := RentabilidadeMesDto{Ano: mes.Ano(), Mes: int(mes.Mes()), Carteira: tipos.Arredondar(tipos.Multiplicar(tipos.Subtrair(acumulado, um), cem), 2),
			Selic: tipos.Arredondar(selicAcumulada, 2)}
		if fi := fechamentoDoMes(ifix, mes); fi != nil && positivo(baseIfix) {
			rentabilidade.Ifix = opcional(c.variacao(*fi, baseIfix))
		}
		h.evolucao = append(h.evolucao, EvolucaoFiiMesDto{mes.Ano(), int(mes.Mes()), tipos.Arredondar(m.aplicado, 2), tipos.Arredondar(m.valor, 2)})
		h.rentabilidade = append(h.rentabilidade, rentabilidade)
	}
	return h
}

func segmentosDosFundos(lista []FiiDto) []DistribuicaoInvestimentoDto {
	segmentos := []DistribuicaoInvestimentoDto{}
	for _, f := range lista {
		nome := semSegmento
		if f.Segmento != nil {
			nome = *f.Segmento
		}
		i := slices.IndexFunc(segmentos, func(d DistribuicaoInvestimentoDto) bool { return d.Nome == nome })
		if i < 0 {
			segmentos = append(segmentos, DistribuicaoInvestimentoDto{nome, tipos.Zero})
			i = len(segmentos) - 1
		}
		segmentos[i].Valor = tipos.Somar(segmentos[i].Valor, f.ValorMercado)
	}
	slices.SortStableFunc(segmentos, func(a, b DistribuicaoInvestimentoDto) int { return b.Valor.Decimal().Cmp(a.Valor.Decimal()) })
	return segmentos
}

// historicosDosFundos é a cotação de 36 meses, para comparar com o preço de cada compra.
func historicosDosFundos(fundos []*fundo, mesAtual tipos.Data) []HistoricoPrecoDto {
	lista := []HistoricoPrecoDto{}
	inicio := mesAtual.SomarMeses(-(mesesDoHistorico - 1))
	for _, f := range fundos {
		pontos := []PontoPrecoDto{}
		for _, m := range f.mensais {
			if !m.mes.Antes(inicio) && m.mes.Antes(mesAtual) {
				pontos = append(pontos, PontoPrecoDto{m.mes.Ano(), int(m.mes.Mes()), m.valor})
			}
		}
		atual := valorOuZero(f.mensalEm(mesAtual))
		if f.ativo.PrecoAtual != nil {
			atual = *f.ativo.PrecoAtual
		}
		if positivo(atual) {
			pontos = append(pontos, PontoPrecoDto{mesAtual.Ano(), int(mesAtual.Mes()), atual})
		}
		if len(pontos) > 0 {
			lista = append(lista, HistoricoPrecoDto{f.ativo.Ticker, pontos})
		}
	}
	return lista
}

func (c *conta) resumoDosFundos(lista []FiiDto, fundos []*fundo, h historico, ha12, hoje, primeiraData tipos.Data) ResumoFiisDto {
	custo := somarPor(lista, func(f FiiDto) tipos.Decimal { return f.Custo })
	mercado := somarPor(lista, func(f FiiDto) tipos.Decimal { return f.ValorMercado })
	renda12 := tipos.Zero
	for _, f := range fundos {
		doFundo := tipos.Zero
		for _, p := range f.proventos {
			if p.DataCom.Depois(ha12) {
				doFundo = tipos.Somar(doFundo, tipos.Multiplicar(c.quantidadeEm(f, p.DataCom), p.ValorPorCota))
			}
		}
		renda12 = tipos.Somar(renda12, doFundo)
	}
	porCotaPonderado := somarPor(lista, func(f FiiDto) tipos.Decimal { return tipos.Multiplicar(f.RendimentosPorCota12Meses, f.Quantidade) })
	mesesComRenda := min(12, max(1, (hoje.NumeroDoDia()-primeiraData.NumeroDoDia())/30))
	r := ResumoFiisDto{ValorMercado: mercado, Custo: custo, Valorizacao: tipos.Subtrair(mercado, custo), Renda12Meses: tipos.Arredondar(renda12, 2),
		RendaMediaMensal:       tipos.Arredondar(c.dividir(renda12, tipos.DecimalDeInteiro(int64(mesesComRenda))), 2),
		RendaMensalEstimada:    somarPor(lista, func(f FiiDto) tipos.Decimal { return f.RendaMensalEstimada }),
		Dy12Meses:              c.pct(&porCotaPonderado, &mercado),
		YieldOnCost12Meses:     c.pct(&porCotaPonderado, &custo),
		RecebidoExtrato12Meses: tipos.Arredondar(somarPor(lista, func(f FiiDto) tipos.Decimal { return f.RecebidoExtrato12Meses }), 2),
		QuantidadeFiis:         len(lista)}
	if positivo(custo) {
		r.ValorizacaoPercentual = opcional(c.percentual(tipos.Subtrair(mercado, custo), custo, 2))
	}
	if n := len(h.rentabilidade); n > 0 {
		ultima := h.rentabilidade[n-1]
		r.RentabilidadeTotal, r.RentabilidadeSelic, r.RentabilidadeIfix = &ultima.Carteira, &ultima.Selic, ultima.Ifix
	}
	return r
}

// creditosDeProvento são os recebidos com código mais os créditos da corretora que não são troco nem venda ("Valor recebido
// de Investimentos", sem código), #173. O mesmo crédito conta uma vez só.
func (s *Servico) creditosDeProvento(ctx context.Context, recebidos []ProventoRecebido, temFundos bool) ([]CreditoRecebido, error) {
	lista := []CreditoRecebido{}
	for _, r := range recebidos {
		lista = append(lista, CreditoRecebido{r.Id, r.Data, r.Valor})
	}
	if temFundos {
		corretora, err := s.ProventosDaCorretora(ctx)
		if err != nil {
			return nil, err
		}
		lista = append(lista, corretora...)
	}
	vistos := map[int64]bool{}
	return slices.DeleteFunc(lista, func(r CreditoRecebido) bool {
		repetido := vistos[r.Id]
		vistos[r.Id] = true
		return repetido
	}), nil
}

// MontarFiis é o MontarAsync do FiiService.
func (s *Servico) MontarFiis(ctx context.Context, serie *selic.Serie) (*PainelFiisDto, error) {
	hoje := relogio.Hoje(s.relogio)
	ha12 := hoje.SomarAnos(-1)
	mesAtual := relogio.PrimeiroDiaMes(hoje.Ano(), hoje.Mes())

	fundos, err := s.lerFundos(ctx, hoje)
	if err != nil {
		return nil, err
	}
	todos, err := s.ProventosRecebidos(ctx)
	if err != nil {
		return nil, err
	}
	recebidos := slices.DeleteFunc(todos, func(r ProventoRecebido) bool {
		return !slices.ContainsFunc(fundos, func(f *fundo) bool { return f.ativo.Ticker == r.Ticker })
	})
	creditos, err := s.creditosDeProvento(ctx, recebidos, len(fundos) > 0)
	if err != nil {
		return nil, err
	}
	ifix, err := s.lerIfix(ctx)
	if err != nil {
		return nil, err
	}

	var c conta
	fundoDoCredito := c.fundoDeCadaCredito(fundos, recebidos, creditos)
	abertos := slices.DeleteFunc(slices.Clone(fundos), func(f *fundo) bool { return !positivo(f.posicao.Quantidade) })
	totalMercado := somarPor(abertos, func(f *fundo) tipos.Decimal {
		return tipos.Multiplicar(f.posicao.Quantidade, valorOu(f.ativo.PrecoAtual, f.posicao.PrecoMedio))
	})
	lista := []FiiDto{}
	for _, f := range abertos {
		lista = append(lista, c.fundoDto(f, recebidos, totalMercado, ha12, hoje))
	}
	slices.SortStableFunc(lista, func(a, b FiiDto) int { return b.ValorMercado.Decimal().Cmp(a.ValorMercado.Decimal()) })

	lotes := []LoteFiiDto{}
	for _, f := range fundos {
		lotes = append(lotes, c.lotesDoFundo(f, ha12)...)
	}
	slices.SortStableFunc(lotes, func(a, b LoteFiiDto) int {
		return cmp.Or(cmp.Compare(b.Data.NumeroDoDia(), a.Data.NumeroDoDia()), cmp.Compare(a.Ticker, b.Ticker))
	})

	primeiraData := hoje
	if len(fundos) > 0 {
		primeiraData = fundos[0].operacoes[0].Data
		for _, f := range fundos {
			if f.operacoes[0].Data.Antes(primeiraData) {
				primeiraData = f.operacoes[0].Data
			}
		}
	}
	proventos := c.proventosPorMes(fundos, recebidos, primeiraData, mesAtual)
	h := c.evolucaoDosFundos(fundos, creditos, fundoDoCredito, serie, ifix, primeiraData, hoje, mesAtual)
	painel := &PainelFiisDto{Resumo: c.resumoDosFundos(lista, fundos, h, ha12, hoje, primeiraData), Fiis: lista, Lotes: lotes, Proventos: proventos, Evolucao: h.evolucao,
		Rentabilidade: h.rentabilidade, Segmentos: segmentosDosFundos(lista), Historicos: historicosDosFundos(fundos, mesAtual), SegmentosDisponiveis: segmentosFii, MesAMes: h.mesAMes}
	return painel, c.err
}

// inicioDosFiis é a primeira operação de FII, ou hoje.
func (s *Servico) inicioDosFiis(ctx context.Context) (tipos.Data, error) {
	var inicio sql.Null[tipos.Data]
	err := s.db.QueryRowContext(ctx, `SELECT MIN(o."Data") FROM "OperacaoAtivo" AS o INNER JOIN "Ativo" AS a ON o."AtivoId" = a."Id" WHERE a."Tipo" = ?`, tipoFii).Scan(&inicio)
	if err != nil {
		return tipos.Data{}, fmt.Errorf("ler a primeira operação de FII: %w", err)
	}
	if !inicio.Valid {
		return relogio.Hoje(s.relogio), nil
	}
	return inicio.V, nil
}

// fiis é o GET /api/carteira/fiis.
func (s *Servico) fiis(w http.ResponseWriter, r *http.Request) {
	inicio, err := s.inicioDosFiis(r.Context())
	if err != nil {
		s.erroInterno(w, r, "fiis", err)
		return
	}
	serie, err := s.selic.ObterSerie(r.Context(), inicio)
	if err != nil {
		s.erroInterno(w, r, "fiis", err)
		return
	}
	painel, err := s.MontarFiis(r.Context(), serie)
	if err != nil {
		s.erroInterno(w, r, "fiis", err)
		return
	}
	respostas.JSON(w, http.StatusOK, respostas.ConteudoJSON, painel)
}
