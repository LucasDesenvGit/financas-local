package carteira

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/configuracao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/contas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/desfazer"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/reservas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/selic"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

const (
	// validadeDaCotacao: cotação mais velha que isso conta como desatualizada.
	validadeDaCotacao = 4 * time.Hour
	// limiteDeMeses é o tamanho máximo da evolução mensal.
	limiteDeMeses = 240
	// tipoContaInvestimento é o TipoConta.Investimento do C#.
	tipoContaInvestimento = 4
)

var (
	// selicRelevante: abaixo disso, o "% da Selic" do período não quer dizer nada.
	selicRelevante = mustDecimal("0.05")
	// diferencaDeCotas: a diferença de cotas abaixo disso é resto de arredondamento.
	diferencaDeCotas = mustDecimal("0.000001")
	// inicioDaSerieSelic é a data mais antiga que a comparação com a Selic usa.
	inicioDaSerieSelic = tipos.NovaData(2000, time.January, 1)
)

// conversao é o que a carteira usa do câmbio. A interface é de quem usa: o de verdade é o internal/cambio.
type conversao interface {
	ObterConversor(ctx context.Context, historicoDesde *tipos.Data) (*cambio.Conversor, error)
	ObterCotacao(ctx context.Context, moeda string, data *tipos.Data) (*cambio.Cotacao, error)
	ListarMoedas(ctx context.Context) ([]cambio.Moeda, error)
}

// listaDeReservas é o que a carteira usa das reservas.
type listaDeReservas interface {
	Listar(ctx context.Context) ([]reservas.ReservaDto, error)
}

// taxaSelic é o que a carteira usa da Selic.
type taxaSelic interface {
	ObterSerie(ctx context.Context, inicio tipos.Data) (*selic.Serie, error)
	MetaAtual(ctx context.Context) (*tipos.Decimal, error)
}

// atualizacao é o que a carteira usa das cotações.
type atualizacao interface {
	Atualizar(ctx context.Context, somente []int64) (cotacoes.Resultado, error)
	FechamentosDiarios(ctx context.Context, ticker string, de, ate tipos.Data) ([]cotacoes.Fechamento, error)
}

// Servico é o CarteiraService.
type Servico struct {
	db       *sql.DB
	cambio   conversao
	reservas listaDeReservas
	selic    taxaSelic
	cotacoes atualizacao
	config   *configuracao.Servico
	cofre    configuracao.Cofre
	desfazer *desfazer.Memoria[estadoAnterior]
	relogio  relogio.Relogio
	log      *slog.Logger
}

// NovoServico monta o serviço com as dependências.
func NovoServico(db *sql.DB, c conversao, r listaDeReservas, s taxaSelic, a atualizacao, cofre configuracao.Cofre, rel relogio.Relogio, log *slog.Logger) *Servico {
	return &Servico{db: db, cambio: c, reservas: r, selic: s, cotacoes: a, config: configuracao.Novo(db, rel), cofre: cofre, desfazer: desfazer.Nova[estadoAnterior](rel), relogio: rel, log: log}
}

// SelicResumoDto é o do C#.
type SelicResumoDto struct {
	MetaAnual        *tipos.Decimal `json:"metaAnual"`
	Acumulado12Meses *tipos.Decimal `json:"acumulado12Meses"`
	AcumuladoAno     *tipos.Decimal `json:"acumuladoAno"`
	AtualizadaAte    *tipos.Data    `json:"atualizadaAte"`
}

// UltimoProventoDto é o do C#.
type UltimoProventoDto struct {
	DataCom      tipos.Data    `json:"dataCom"`
	ValorPorCota tipos.Decimal `json:"valorPorCota"`
}

// PosicaoAtivoDto é o do C#, na ordem do record: preço médio, preço atual, anterior e último provento na moeda do ativo;
// custo, valores e resultados em reais.
type PosicaoAtivoDto struct {
	Id                       int64              `json:"id"`
	Ticker                   string             `json:"ticker"`
	Nome                     *string            `json:"nome"`
	Tipo                     int64              `json:"tipo"`
	Moeda                    string             `json:"moeda"`
	Quantidade               tipos.Decimal      `json:"quantidade"`
	PrecoMedio               tipos.Decimal      `json:"precoMedio"`
	Custo                    tipos.Decimal      `json:"custo"`
	PrecoAtual               *tipos.Decimal     `json:"precoAtual"`
	PrecoAtualEmReais        *tipos.Decimal     `json:"precoAtualEmReais"`
	PrecoAnterior            *tipos.Decimal     `json:"precoAnterior"`
	VariacaoDia              *tipos.Decimal     `json:"variacaoDia"`
	ValorMercado             tipos.Decimal      `json:"valorMercado"`
	Valorizacao              tipos.Decimal      `json:"valorizacao"`
	ValorizacaoPercentual    *tipos.Decimal     `json:"valorizacaoPercentual"`
	LucroRealizado           tipos.Decimal      `json:"lucroRealizado"`
	ProventosRecebidos       tipos.Decimal      `json:"proventosRecebidos"`
	Proventos12Meses         tipos.Decimal      `json:"proventos12Meses"`
	DividendYield12Meses     *tipos.Decimal     `json:"dividendYield12Meses"`
	ResultadoTotal           tipos.Decimal      `json:"resultadoTotal"`
	ResultadoTotalPercentual *tipos.Decimal     `json:"resultadoTotalPercentual"`
	ValorSeFosseSelic        tipos.Decimal      `json:"valorSeFosseSelic"`
	DiferencaSelic           tipos.Decimal      `json:"diferencaSelic"`
	ResultadoSelicPercentual *tipos.Decimal     `json:"resultadoSelicPercentual"`
	PrimeiraCompra           *tipos.Data        `json:"primeiraCompra"`
	UltimoProvento           *UltimoProventoDto `json:"ultimoProvento"`
	PrecoManual              bool               `json:"precoManual"`
	PrecoAtualizadoEm        *tipos.DataHora    `json:"precoAtualizadoEm"`
	FonteCotacao             *string            `json:"fonteCotacao"`
	SemCotacao               bool               `json:"semCotacao"`
	Segmento                 *string            `json:"segmento"`
}

// TotaisRendaVariavelDto é o do C#.
type TotaisRendaVariavelDto struct {
	ValorMercado             tipos.Decimal  `json:"valorMercado"`
	Custo                    tipos.Decimal  `json:"custo"`
	Valorizacao              tipos.Decimal  `json:"valorizacao"`
	ValorizacaoPercentual    *tipos.Decimal `json:"valorizacaoPercentual"`
	LucroRealizado           tipos.Decimal  `json:"lucroRealizado"`
	ProventosRecebidos       tipos.Decimal  `json:"proventosRecebidos"`
	Proventos12Meses         tipos.Decimal  `json:"proventos12Meses"`
	ResultadoTotal           tipos.Decimal  `json:"resultadoTotal"`
	ValorSeFosseSelic        tipos.Decimal  `json:"valorSeFosseSelic"`
	DiferencaSelic           tipos.Decimal  `json:"diferencaSelic"`
	ResultadoTotalPercentual *tipos.Decimal `json:"resultadoTotalPercentual"`
	ResultadoSelicPercentual *tipos.Decimal `json:"resultadoSelicPercentual"`
}

// EvolucaoCarteiraDto é o do C#.
type EvolucaoCarteiraDto struct {
	Ano             int            `json:"ano"`
	Mes             int            `json:"mes"`
	Aplicado        tipos.Decimal  `json:"aplicado"`
	AplicadoNaSelic tipos.Decimal  `json:"aplicadoNaSelic"`
	ValorMercado    *tipos.Decimal `json:"valorMercado"`
}

// DistribuicaoInvestimentoDto é o do C#.
type DistribuicaoInvestimentoDto struct {
	Nome  string        `json:"nome"`
	Valor tipos.Decimal `json:"valor"`
}

// InvestimentoComparadoDto é o do C#: saldo, valor aplicado e rentabilidade na moeda do investimento; SaldoEmReais é nil
// sem cotação guardada da moeda.
type InvestimentoComparadoDto struct {
	Id                int64          `json:"id"`
	Nome              string         `json:"nome"`
	Tipo              string         `json:"tipo"`
	Subtipo           *string        `json:"subtipo"`
	Codigo            *string        `json:"codigo"`
	Emissor           *string        `json:"emissor"`
	NomeInstituicao   *string        `json:"nomeInstituicao"`
	Moeda             string         `json:"moeda"`
	Saldo             tipos.Decimal  `json:"saldo"`
	SaldoEmReais      *tipos.Decimal `json:"saldoEmReais"`
	ValorAplicado     *tipos.Decimal `json:"valorAplicado"`
	DataAplicacao     *tipos.Data    `json:"dataAplicacao"`
	DataVencimento    *tipos.Data    `json:"dataVencimento"`
	TaxaAnual         *tipos.Decimal `json:"taxaAnual"`
	Rentabilidade     *tipos.Decimal `json:"rentabilidade"`
	SelicNoPeriodo    *tipos.Decimal `json:"selicNoPeriodo"`
	PercentualDaSelic *tipos.Decimal `json:"percentualDaSelic"`
	RendaVariavel     bool           `json:"rendaVariavel"`
	JaNaCarteira      bool           `json:"jaNaCarteira"`
	AtualizadoEm      tipos.DataHora `json:"atualizadoEm"`
	NomeExibicao      string         `json:"nomeExibicao"`
	NomePersonalizado *string        `json:"nomePersonalizado"`
}

// CarteiraDto é o do C#.
type CarteiraDto struct {
	Selic                     SelicResumoDto                `json:"selic"`
	RendaVariavel             TotaisRendaVariavelDto        `json:"rendaVariavel"`
	Posicoes                  []PosicaoAtivoDto             `json:"posicoes"`
	Evolucao                  []EvolucaoCarteiraDto         `json:"evolucao"`
	OpenFinance               []InvestimentoComparadoDto    `json:"openFinance"`
	TotalOpenFinance          tipos.Decimal                 `json:"totalOpenFinance"`
	TotalInvestimentosManuais tipos.Decimal                 `json:"totalInvestimentosManuais"`
	TotalGeral                tipos.Decimal                 `json:"totalGeral"`
	Distribuicao              []DistribuicaoInvestimentoDto `json:"distribuicao"`
	CotacoesAtualizadasEm     *tipos.DataHora               `json:"cotacoesAtualizadasEm"`
	CotacoesDesatualizadas    bool                          `json:"cotacoesDesatualizadas"`
	OpenFinanceNaoRegistrados int                           `json:"openFinanceNaoRegistrados"`
	MoedasSemCotacao          []string                      `json:"moedasSemCotacao"`
	Reservas                  []reservas.ReservaDto         `json:"reservas"`
	TotalReservas             tipos.Decimal                 `json:"totalReservas"`
	CotasDiferentes           []string                      `json:"cotasDiferentes"`
}

// OperacaoAtivoDto é o do C#.
type OperacaoAtivoDto struct {
	Id                int64          `json:"id"`
	AtivoId           int64          `json:"ativoId"`
	Ticker            string         `json:"ticker"`
	Tipo              int64          `json:"tipo"`
	Data              tipos.Data     `json:"data"`
	Quantidade        tipos.Decimal  `json:"quantidade"`
	PrecoUnitario     tipos.Decimal  `json:"precoUnitario"`
	Taxas             tipos.Decimal  `json:"taxas"`
	Moeda             string         `json:"moeda"`
	Cambio            tipos.Decimal  `json:"cambio"`
	ValorTotal        tipos.Decimal  `json:"valorTotal"`
	Observacao        *string        `json:"observacao"`
	PrecoAtual        *tipos.Decimal `json:"precoAtual"`
	VariacaoPreco     *tipos.Decimal `json:"variacaoPreco"`
	ValorAtual        *tipos.Decimal `json:"valorAtual"`
	ValorSeFosseSelic tipos.Decimal  `json:"valorSeFosseSelic"`
	Origem            int64          `json:"origem"`
}

// lerAtivos lê os ativos pelo ticker, com as operações pelo Id e os proventos pela data-com.
func (s *Servico) lerAtivos(ctx context.Context, onde string, argumentos ...any) ([]Ativo, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Ticker", "Nome", "Tipo", "Moeda", "PrecoAtual", "PrecoAnterior",
		"PrecoAtualizadoEm", "PrecoManual", "FonteCotacao", "Segmento" FROM "Ativo" `+onde+` ORDER BY "Ticker", "Id"`, argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler os ativos: %w", err)
	}
	var ativos []Ativo
	indices := map[int64]int{}
	for linhas.Next() {
		var a Ativo
		if err := linhas.Scan(&a.Id, &a.Ticker, &a.Nome, &a.Tipo, &a.Moeda, &a.PrecoAtual, &a.PrecoAnterior, &a.PrecoAtualizadoEm,
			&a.PrecoManual, &a.FonteCotacao, &a.Segmento); err != nil {
			linhas.Close()
			return nil, fmt.Errorf("ler ativo: %w", err)
		}
		indices[a.Id] = len(ativos)
		ativos = append(ativos, a)
	}
	linhas.Close()
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os ativos: %w", err)
	}
	if err := s.lerOperacoes(ctx, ativos, indices); err != nil {
		return nil, err
	}
	return ativos, s.lerProventos(ctx, ativos, indices)
}

func (s *Servico) lerOperacoes(ctx context.Context, ativos []Ativo, indices map[int64]int) error {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "AtivoId", "Tipo", "Data", "Quantidade", "PrecoUnitario", "Taxas", "Cambio",
		"Observacao", "Origem", "TransacaoId" FROM "OperacaoAtivo" ORDER BY "Id"`)
	if err != nil {
		return fmt.Errorf("ler as operações: %w", err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var o Operacao
		if err := linhas.Scan(&o.Id, &o.AtivoId, &o.Tipo, &o.Data, &o.Quantidade, &o.PrecoUnitario, &o.Taxas, &o.Cambio,
			&o.Observacao, &o.Origem, &o.TransacaoId); err != nil {
			return fmt.Errorf("ler operação: %w", err)
		}
		if i, ok := indices[o.AtivoId]; ok {
			ativos[i].Operacoes = append(ativos[i].Operacoes, o)
		}
	}
	return linhas.Err()
}

func (s *Servico) lerProventos(ctx context.Context, ativos []Ativo, indices map[int64]int) error {
	linhas, err := s.db.QueryContext(ctx, `SELECT "AtivoId", "DataCom", "ValorPorCota" FROM "ProventoAtivo" ORDER BY "AtivoId", "DataCom"`)
	if err != nil {
		return fmt.Errorf("ler os proventos: %w", err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var id int64
		var p Provento
		if err := linhas.Scan(&id, &p.DataCom, &p.ValorPorCota); err != nil {
			return fmt.Errorf("ler provento: %w", err)
		}
		if i, ok := indices[id]; ok {
			ativos[i].Proventos = append(ativos[i].Proventos, p)
		}
	}
	return linhas.Err()
}

// investimento é uma linha de Investimento com a conexão ativa.
type investimento struct {
	Id, ConexaoBancariaId                int64
	Nome, Tipo, Moeda                    string
	Subtipo, Codigo, Emissor, Personal   *string
	Saldo                                tipos.Decimal
	ValorAplicado, TaxaAnual, Quantidade *tipos.Decimal
	DataAplicacao, DataVencimento        *tipos.Data
	AtualizadoEm                         tipos.DataHora
}

// lerRendaVariavel lê as ações e os ETFs das conexões ativas, para a conferência.
func (s *Servico) lerRendaVariavel(ctx context.Context) ([]investimento, error) {
	return s.lerInvestimentosOnde(ctx, ` AND i."Tipo" IN ('EQUITY', 'ETF')`)
}

// lerInvestimentosOnde lê os investimentos das conexões ativas, com o filtro a mais.
func (s *Servico) lerInvestimentosOnde(ctx context.Context, filtro string) ([]investimento, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT i."Id", i."ConexaoBancariaId", i."Nome", i."Tipo", i."Moeda", i."Subtipo", i."Codigo",
		i."Emissor", i."NomePersonalizado", i."Saldo", i."ValorAplicado", i."TaxaAnual", i."Quantidade", i."DataAplicacao",
		i."DataVencimento", i."AtualizadoEm" FROM "Investimento" AS i
		INNER JOIN "ConexaoBancaria" AS c ON i."ConexaoBancariaId" = c."Id" WHERE c."Ativa"`+filtro)
	if err != nil {
		return nil, fmt.Errorf("ler os investimentos: %w", err)
	}
	defer linhas.Close()
	var lista []investimento
	for linhas.Next() {
		var i investimento
		if err := linhas.Scan(&i.Id, &i.ConexaoBancariaId, &i.Nome, &i.Tipo, &i.Moeda, &i.Subtipo, &i.Codigo, &i.Emissor, &i.Personal,
			&i.Saldo, &i.ValorAplicado, &i.TaxaAnual, &i.Quantidade, &i.DataAplicacao, &i.DataVencimento, &i.AtualizadoEm); err != nil {
			return nil, fmt.Errorf("ler investimento: %w", err)
		}
		lista = append(lista, i)
	}
	return lista, linhas.Err()
}

// lerInvestimentos lê os investimentos das conexões ativas, por tipo e pelo maior saldo. O EF ordena o saldo com a
// colação EF_DECIMAL, pelo valor; no SQL, o texto "100.5" viria antes de "9.5", por isso a segunda ordem é aqui.
func (s *Servico) lerInvestimentos(ctx context.Context) ([]investimento, error) {
	lista, err := s.lerInvestimentosOnde(ctx, ` ORDER BY i."Tipo"`)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(lista, func(a, b investimento) int {
		return cmp.Or(cmp.Compare(a.Tipo, b.Tipo), b.Saldo.Decimal().Cmp(a.Saldo.Decimal()))
	})
	return lista, nil
}

// InicioDaSerieSelic é o InicioSerieSelicAsync: desde quando a Selic é necessária (o começo do ano ou um ano atrás, a
// primeira operação e a primeira aplicação), não antes de 2000.
func (s *Servico) InicioDaSerieSelic(ctx context.Context) (tipos.Data, error) {
	hoje := relogio.Hoje(s.relogio)
	inicio := relogio.PrimeiroDiaMes(hoje.Ano(), time.January)
	if umAno := hoje.SomarAnos(-1); umAno.Antes(inicio) {
		inicio = umAno
	}
	var operacao, aplicacao sql.Null[tipos.Data]
	if err := s.db.QueryRowContext(ctx, `SELECT MIN("Data") FROM "OperacaoAtivo"`).Scan(&operacao); err != nil {
		return tipos.Data{}, fmt.Errorf("ler a primeira operação: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(i."DataAplicacao") FROM "Investimento" AS i
		INNER JOIN "ConexaoBancaria" AS c ON i."ConexaoBancariaId" = c."Id" WHERE c."Ativa"`).Scan(&aplicacao); err != nil {
		return tipos.Data{}, fmt.Errorf("ler a primeira aplicação: %w", err)
	}
	if operacao.Valid && operacao.V.Antes(inicio) {
		inicio = operacao.V
	}
	if aplicacao.Valid && aplicacao.V.Antes(inicio) {
		inicio = aplicacao.V
	}
	if inicio.Antes(inicioDaSerieSelic) {
		return inicioDaSerieSelic, nil
	}
	return inicio, nil
}

func opcional(d tipos.Decimal) *tipos.Decimal { return &d }

// noCodigo é o CodigoNaCarteira: o código do investimento é de um ativo em carteira, sem diferença de maiúsculas.
func noCodigo(codigo *string, tickers map[string]bool) bool {
	t := NormalizarTicker(codigo)
	if t == nil {
		t = NormalizarTickerExterior(codigo)
	}
	return t != nil && tickers[texto.MaiusculasInvariantes(*t)]
}

// posicaoDoAtivo monta a posição de um ativo e devolve os fluxos dela (+ aplicação, − resgate: vendas e proventos).
func (s *Servico) posicaoDoAtivo(a *Ativo, serie *selic.Serie, conversor *cambio.Conversor, hoje tipos.Data, c *conta) (PosicaoAtivoDto, Posicao, []fluxo, error) {
	ha12Meses := hoje.SomarAnos(-1)
	operacoes := slices.Clone(a.Operacoes)
	slices.SortStableFunc(operacoes, func(x, y Operacao) int {
		return cmp.Or(cmp.Compare(x.Data.NumeroDoDia(), y.Data.NumeroDoDia()), cmp.Compare(x.Id, y.Id))
	})
	p, err := Calcular(operacoes, nil)
	if err != nil {
		return PosicaoAtivoDto{}, p, nil, err
	}

	var fluxos []fluxo
	for _, o := range operacoes {
		fluxos = append(fluxos, fluxoDaOperacao(o))
	}
	// Proventos pela quantidade em carteira na data-com.
	recebidos, ultimos12 := tipos.Zero, tipos.Zero
	var jaPagos []Provento
	for _, pr := range a.Proventos {
		if !pr.DataCom.Depois(hoje) {
			jaPagos = append(jaPagos, pr)
		}
	}
	for _, pr := range jaPagos {
		naData, err := Calcular(operacoes, &pr.DataCom)
		if err != nil {
			return PosicaoAtivoDto{}, p, nil, err
		}
		if !positivo(naData.Quantidade) {
			continue
		}
		if valor := conversor.ParaReaisNaData(tipos.Multiplicar(naData.Quantidade, pr.ValorPorCota), a.Moeda, pr.DataCom); valor != nil {
			recebidos = tipos.Somar(recebidos, *valor)
			if pr.DataCom.Depois(ha12Meses) {
				ultimos12 = tipos.Somar(ultimos12, *valor)
			}
			fluxos = append(fluxos, fluxo{pr.DataCom, tipos.Subtrair(tipos.Zero, *valor)})
		}
	}

	emReais := precoEmReais(a, conversor)
	precoDeMercado := p.PrecoMedio
	if emReais != nil {
		precoDeMercado = *emReais
	}
	valorMercado := tipos.Multiplicar(p.Quantidade, precoDeMercado)
	valorizacao := tipos.Subtrair(valorMercado, p.Custo)
	compras := tipos.Zero
	var primeiraCompra *tipos.Data
	for i := range operacoes {
		if operacoes[i].Tipo == operacaoCompra {
			compras = tipos.Somar(compras, valorDaOperacao(operacoes[i]))
			if primeiraCompra == nil {
				primeiraCompra = &operacoes[i].Data
			}
		}
	}
	resultado := tipos.Somar(tipos.Somar(valorizacao, p.LucroRealizado), recebidos)
	valorSelic := c.naSelic(fluxos, serie, hoje)
	// O mesmo dinheiro na Selic, com os mesmos resgates: a diferença entre os valores finais.
	resultadoSelic := tipos.Subtrair(valorSelic, somaDosFluxos(fluxos))

	d := PosicaoAtivoDto{Id: a.Id, Ticker: a.Ticker, Nome: a.Nome, Tipo: a.Tipo, Moeda: a.Moeda, Quantidade: p.Quantidade,
		PrecoMedio: tipos.Arredondar(p.PrecoMedioNaMoeda, 4), Custo: tipos.Arredondar(p.Custo, 2), PrecoAtual: a.PrecoAtual,
		PrecoAnterior: a.PrecoAnterior, ValorMercado: tipos.Arredondar(valorMercado, 2), Valorizacao: tipos.Arredondar(valorizacao, 2),
		LucroRealizado: tipos.Arredondar(p.LucroRealizado, 2), ProventosRecebidos: tipos.Arredondar(recebidos, 2),
		Proventos12Meses: tipos.Arredondar(ultimos12, 2), ResultadoTotal: tipos.Arredondar(resultado, 2),
		ValorSeFosseSelic: tipos.Arredondar(valorSelic, 2), DiferencaSelic: tipos.Arredondar(tipos.Subtrair(valorMercado, valorSelic), 2),
		PrimeiraCompra: primeiraCompra, PrecoManual: a.PrecoManual, PrecoAtualizadoEm: a.PrecoAtualizadoEm, FonteCotacao: a.FonteCotacao,
		SemCotacao: emReais == nil, Segmento: a.Segmento}
	if emReais != nil {
		d.PrecoAtualEmReais = opcional(tipos.Arredondar(*emReais, 4))
	}
	if a.PrecoAtual != nil && a.PrecoAnterior != nil && positivo(*a.PrecoAnterior) {
		d.VariacaoDia = opcional(c.variacao(*a.PrecoAtual, *a.PrecoAnterior))
	}
	if positivo(p.Custo) {
		d.ValorizacaoPercentual = opcional(c.percentual(valorizacao, p.Custo, 2))
	}
	if positivo(valorMercado) && positivo(ultimos12) {
		d.DividendYield12Meses = opcional(c.percentual(ultimos12, valorMercado, 2))
	}
	if positivo(compras) {
		d.ResultadoTotalPercentual = opcional(c.percentual(resultado, compras, 2))
		d.ResultadoSelicPercentual = opcional(c.percentual(resultadoSelic, compras, 2))
	}
	if len(a.Proventos) > 0 {
		// O MaxBy da data-com: os proventos vêm na ordem dela.
		u := a.Proventos[len(a.Proventos)-1]
		d.UltimoProvento = &UltimoProventoDto{DataCom: u.DataCom, ValorPorCota: u.ValorPorCota}
	}
	if d.Segmento == nil && a.Tipo == tipoFii {
		d.Segmento = sugerirSegmento(a.Ticker)
	}
	return d, p, fluxos, c.err
}

// evolucaoMensal é o dinheiro aplicado (líquido) contra o mesmo dinheiro rendendo Selic, mês a mês. Só compras e vendas:
// os proventos ficam de fora para a linha do "aplicado" fazer sentido.
func evolucaoMensal(ativos []Ativo, serie *selic.Serie, hoje tipos.Data, mercado tipos.Decimal, c *conta) []EvolucaoCarteiraDto {
	evolucao := []EvolucaoCarteiraDto{}
	var aplicacoes []fluxo
	for i := range ativos {
		for _, o := range ativos[i].Operacoes {
			aplicacoes = append(aplicacoes, fluxoDaOperacao(o))
		}
	}
	if len(aplicacoes) == 0 {
		return evolucao
	}
	primeiro := aplicacoes[0].data
	for _, f := range aplicacoes {
		if f.data.Antes(primeiro) {
			primeiro = f.data
		}
	}
	mes := relogio.PrimeiroDiaMes(primeiro.Ano(), primeiro.Mes())
	for n := 0; n < limiteDeMeses && !mes.Depois(hoje); n++ {
		fim := relogio.UltimoDiaMes(mes.Ano(), mes.Mes())
		if fim.Depois(hoje) {
			fim = hoje
		}
		var ateAqui []fluxo
		for _, f := range aplicacoes {
			if !f.data.Depois(fim) {
				ateAqui = append(ateAqui, f)
			}
		}
		item := EvolucaoCarteiraDto{Ano: mes.Ano(), Mes: int(mes.Mes()), Aplicado: tipos.Arredondar(somaDosFluxos(ateAqui), 2),
			AplicadoNaSelic: tipos.Arredondar(c.naSelic(ateAqui, serie, fim), 2)}
		if fim.Igual(hoje) {
			item.ValorMercado = &mercado
		}
		evolucao = append(evolucao, item)
		mes = relogio.PrimeiroDiaMes(mes.Ano(), mes.Mes()+1)
	}
	return evolucao
}

// comparadoComSelic é o investimento do Open Finance comparado com a Selic do período. A rentabilidade é na moeda dele,
// sem a variação do câmbio.
func comparadoComSelic(i investimento, serie *selic.Serie, conversor *cambio.Conversor, hoje tipos.Data, instituicoes map[int64]string,
	tickers map[string]bool, c *conta) InvestimentoComparadoDto {
	d := InvestimentoComparadoDto{Id: i.Id, Nome: i.Nome, Tipo: i.Tipo, Subtipo: i.Subtipo, Codigo: i.Codigo, Emissor: i.Emissor,
		Moeda: i.Moeda, Saldo: i.Saldo, SaldoEmReais: conversor.ParaReais(i.Saldo, i.Moeda), ValorAplicado: i.ValorAplicado,
		DataAplicacao: i.DataAplicacao, DataVencimento: i.DataVencimento, TaxaAnual: i.TaxaAnual, RendaVariavel: ehRendaVariavel(i.Tipo),
		JaNaCarteira: noCodigo(i.Codigo, tickers), AtualizadoEm: i.AtualizadoEm, NomeExibicao: nomeDeExibicao(i.Nome, i.Tipo, i.Emissor, i.Personal),
		NomePersonalizado: i.Personal}
	if nome, ok := instituicoes[i.ConexaoBancariaId]; ok {
		d.NomeInstituicao = &nome
	}
	if i.ValorAplicado != nil && positivo(*i.ValorAplicado) {
		d.Rentabilidade = opcional(c.variacao(i.Saldo, *i.ValorAplicado))
	}
	if i.DataAplicacao != nil && i.DataAplicacao.Antes(hoje) && !serie.Vazia() {
		periodo := percentualDaSerie(c.fator(serie, *i.DataAplicacao, hoje))
		d.SelicNoPeriodo = &periodo
		if d.Rentabilidade != nil && periodo.Decimal().GreaterThan(selicRelevante.Decimal()) {
			d.PercentualDaSelic = opcional(c.percentual(*d.Rentabilidade, periodo, 1))
		}
	}
	return d
}

func percentualDaSerie(fator tipos.Decimal) tipos.Decimal {
	return tipos.Arredondar(tipos.Multiplicar(tipos.Subtrair(fator, um), cem), 2)
}

// somarPor soma o campo da lista.
func somarPor[T any](lista []T, campo func(T) tipos.Decimal) tipos.Decimal {
	soma := tipos.Zero
	for _, x := range lista {
		soma = tipos.Somar(soma, campo(x))
	}
	return soma
}

// cotaNaCorretora são as cotas de um ativo na corretora, somando as conexões, pela última sincronização.
type cotaNaCorretora struct {
	ticker     string
	quantidade tipos.Decimal
	em         tipos.DataHora
}

// tickerDoInvestimento é o TickerDe: o do código, ou o do nome.
func tickerDoInvestimento(i investimento) *string {
	if t := NormalizarTicker(i.Codigo); t != nil {
		return t
	}
	return NormalizarTicker(&i.Nome)
}

// cotasNaCorretora é o CotasNaCorretora: as cotas de ações e ETFs do Open Finance, por ticker, na ordem em que aparecem. A
// posição zerada (resgate total, direito de subscrição exercido) fica de fora.
func cotasNaCorretora(lista []investimento) []cotaNaCorretora {
	var cotas []cotaNaCorretora
	for _, i := range lista {
		if (i.Tipo != "EQUITY" && i.Tipo != "ETF") || i.Quantidade == nil || !positivo(*i.Quantidade) || !positivo(i.Saldo) {
			continue
		}
		t := tickerDoInvestimento(i)
		if t == nil {
			continue
		}
		j := slices.IndexFunc(cotas, func(c cotaNaCorretora) bool { return c.ticker == *t })
		if j < 0 {
			cotas = append(cotas, cotaNaCorretora{*t, *i.Quantidade, i.AtualizadoEm})
			continue
		}
		cotas[j].quantidade = tipos.Somar(cotas[j].quantidade, *i.Quantidade)
		if i.AtualizadoEm.Tempo().After(cotas[j].em.Tempo()) {
			cotas[j].em = i.AtualizadoEm
		}
	}
	return cotas
}

// totaisDaRendaVariavel é o resumo das posições.
func totaisDaRendaVariavel(posicoes, abertas []PosicaoAtivoDto, comprasTotais, resultadoSelicTotal tipos.Decimal, c *conta) TotaisRendaVariavelDto {
	custo := somarPor(abertas, func(p PosicaoAtivoDto) tipos.Decimal { return p.Custo })
	mercado := somarPor(abertas, func(p PosicaoAtivoDto) tipos.Decimal { return p.ValorMercado })
	resultado := somarPor(posicoes, func(p PosicaoAtivoDto) tipos.Decimal { return p.ResultadoTotal })
	selicTotal := somarPor(posicoes, func(p PosicaoAtivoDto) tipos.Decimal { return p.ValorSeFosseSelic })
	t := TotaisRendaVariavelDto{ValorMercado: mercado, Custo: custo, Valorizacao: tipos.Subtrair(mercado, custo),
		LucroRealizado:     somarPor(posicoes, func(p PosicaoAtivoDto) tipos.Decimal { return p.LucroRealizado }),
		ProventosRecebidos: somarPor(posicoes, func(p PosicaoAtivoDto) tipos.Decimal { return p.ProventosRecebidos }),
		Proventos12Meses:   somarPor(posicoes, func(p PosicaoAtivoDto) tipos.Decimal { return p.Proventos12Meses }),
		ResultadoTotal:     resultado, ValorSeFosseSelic: selicTotal, DiferencaSelic: tipos.Subtrair(mercado, selicTotal)}
	if positivo(custo) {
		t.ValorizacaoPercentual = opcional(c.percentual(tipos.Subtrair(mercado, custo), custo, 2))
	}
	if positivo(comprasTotais) {
		t.ResultadoTotalPercentual = opcional(c.percentual(resultado, comprasTotais, 2))
		t.ResultadoSelicPercentual = opcional(c.percentual(resultadoSelicTotal, comprasTotais, 2))
	}
	return t
}

// distribuicaoPorClasse é a distribuição dos investimentos: as classes da carteira, as do Open Finance (somadas às de
// mesmo nome), as contas de investimento e as reservas. Só o que é maior que zero, do maior para o menor.
func distribuicaoPorClasse(abertas []PosicaoAtivoDto, openFinance []InvestimentoComparadoDto, totalManuais, totalReservas tipos.Decimal) []DistribuicaoInvestimentoDto {
	var lista []DistribuicaoInvestimentoDto
	somar := func(nome string, valor tipos.Decimal) {
		if i := slices.IndexFunc(lista, func(d DistribuicaoInvestimentoDto) bool { return d.Nome == nome }); i >= 0 {
			lista[i].Valor = tipos.Somar(lista[i].Valor, valor)
			return
		}
		lista = append(lista, DistribuicaoInvestimentoDto{Nome: nome, Valor: tipos.Somar(tipos.Zero, valor)})
	}
	for _, p := range abertas {
		somar(nomeDoTipo(p.Tipo), p.ValorMercado)
	}
	for _, i := range openFinance {
		if !i.JaNaCarteira {
			valor := tipos.Zero
			if i.SaldoEmReais != nil {
				valor = *i.SaldoEmReais
			}
			somar(nomeDoTipoPluggy(i.Tipo), valor)
		}
	}
	if !totalManuais.Decimal().IsZero() {
		lista = append(lista, DistribuicaoInvestimentoDto{Nome: "Contas de investimento", Valor: totalManuais})
	}
	if !totalReservas.Decimal().IsZero() {
		lista = append(lista, DistribuicaoInvestimentoDto{Nome: "Reservas", Valor: totalReservas})
	}
	maiores := []DistribuicaoInvestimentoDto{}
	for _, d := range lista {
		if positivo(d.Valor) {
			maiores = append(maiores, d)
		}
	}
	slices.SortStableFunc(maiores, func(a, b DistribuicaoInvestimentoDto) int { return b.Valor.Decimal().Cmp(a.Valor.Decimal()) })
	return maiores
}

// totalDasContasDeInvestimento é o saldo das contas do tipo "Investimento", com o que está fora do saldo e o arquivado de
// fora.
func (s *Servico) totalDasContasDeInvestimento(ctx context.Context) (tipos.Decimal, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id" FROM "Conta" WHERE NOT "Arquivada" AND "IncluirNoSaldo" AND "Tipo" = ?`, tipoContaInvestimento)
	if err != nil {
		return tipos.Zero, fmt.Errorf("ler as contas de investimento: %w", err)
	}
	defer linhas.Close()
	var ids []int64
	for linhas.Next() {
		var id int64
		if err := linhas.Scan(&id); err != nil {
			return tipos.Zero, fmt.Errorf("ler conta de investimento: %w", err)
		}
		ids = append(ids, id)
	}
	if err := linhas.Err(); err != nil {
		return tipos.Zero, fmt.Errorf("ler as contas de investimento: %w", err)
	}
	linhas.Close()
	if len(ids) == 0 {
		return tipos.Zero, nil
	}
	saldos, err := contas.Saldos(ctx, s.db)
	if err != nil {
		return tipos.Zero, err
	}
	return somarPor(ids, func(id int64) tipos.Decimal { return saldos[id] }), nil
}

// Montar é o MontarAsync: a visão completa da carteira, com a série da Selic e a meta já obtidas.
func (s *Servico) Montar(ctx context.Context, serie *selic.Serie, meta *tipos.Decimal) (*CarteiraDto, error) {
	hoje := relogio.Hoje(s.relogio)
	ativos, err := s.lerAtivos(ctx, "")
	if err != nil {
		return nil, err
	}
	// O histórico de câmbio desde a primeira operação, para converter os proventos pela data-com.
	var primeira *tipos.Data
	for i := range ativos {
		for j := range ativos[i].Operacoes {
			if d := &ativos[i].Operacoes[j].Data; primeira == nil || d.Antes(*primeira) {
				primeira = d
			}
		}
	}
	conversor, err := s.cambio.ObterConversor(ctx, primeira)
	if err != nil {
		return nil, err
	}

	var c conta
	posicoes := []PosicaoAtivoDto{}
	var fluxos []fluxo
	tickers := map[string]bool{}
	var maisAntiga *tipos.DataHora
	desatualizadas := false
	comprasTotais := tipos.Zero
	agora := s.relogio.Agora()
	for i := range ativos {
		a := &ativos[i]
		d, p, fluxosDoAtivo, err := s.posicaoDoAtivo(a, serie, conversor, hoje, &c)
		if err != nil {
			return nil, err
		}
		if positivo(p.Quantidade) {
			tickers[texto.MaiusculasInvariantes(a.Ticker)] = true
			if !a.PrecoManual {
				if a.PrecoAtualizadoEm == nil || agora.Sub(a.PrecoAtualizadoEm.Tempo()) > validadeDaCotacao {
					desatualizadas = true
				}
				if em := a.PrecoAtualizadoEm; em != nil && (maisAntiga == nil || em.Tempo().Before(maisAntiga.Tempo())) {
					maisAntiga = em
				}
			}
		}
		for _, o := range a.Operacoes {
			if o.Tipo == operacaoCompra {
				comprasTotais = tipos.Somar(comprasTotais, valorDaOperacao(o))
			}
		}
		fluxos = append(fluxos, fluxosDoAtivo...)
		posicoes = append(posicoes, d)
	}

	abertas := []PosicaoAtivoDto{}
	for _, p := range posicoes {
		if positivo(p.Quantidade) {
			abertas = append(abertas, p)
		}
	}
	selicTotal := somarPor(posicoes, func(p PosicaoAtivoDto) tipos.Decimal { return p.ValorSeFosseSelic })
	totais := totaisDaRendaVariavel(posicoes, abertas, comprasTotais, tipos.Subtrair(selicTotal, somaDosFluxos(fluxos)), &c)
	evolucao := evolucaoMensal(ativos, serie, hoje, totais.ValorMercado, &c)

	investimentos, err := s.lerInvestimentos(ctx)
	if err != nil {
		return nil, err
	}
	instituicoes, err := contas.NomesDasConexoes(ctx, s.db)
	if err != nil {
		return nil, err
	}
	openFinance := []InvestimentoComparadoDto{}
	totalOpenFinance := tipos.Zero
	naoRegistrados := 0
	for _, i := range investimentos {
		d := comparadoComSelic(i, serie, conversor, hoje, instituicoes, tickers, &c)
		openFinance = append(openFinance, d)
		if d.JaNaCarteira {
			continue
		}
		if d.SaldoEmReais != nil {
			totalOpenFinance = tipos.Somar(totalOpenFinance, *d.SaldoEmReais)
		}
		codigo := d.Codigo
		if codigo == nil {
			codigo = &d.Nome
		}
		if d.RendaVariavel && NormalizarTicker(codigo) != nil {
			naoRegistrados++
		}
	}
	totalManuais, err := s.totalDasContasDeInvestimento(ctx)
	if err != nil {
		return nil, err
	}
	// Reservas ("cofrinhos") que a instituição não informa pelo Open Finance, estimadas pelo extrato.
	dasContas, err := s.reservas.Listar(ctx)
	if err != nil {
		return nil, err
	}
	if dasContas == nil {
		dasContas = []reservas.ReservaDto{}
	}
	totalReservas := somarPor(dasContas, func(r reservas.ReservaDto) tipos.Decimal { return r.Saldo })

	resumo := SelicResumoDto{MetaAnual: meta, AtualizadaAte: serie.UltimaData()}
	if !serie.Vazia() {
		resumo.Acumulado12Meses = opcional(percentualDaSerie(c.fator(serie, hoje.SomarAnos(-1), hoje)))
		resumo.AcumuladoAno = opcional(percentualDaSerie(c.fator(serie, relogio.PrimeiroDiaMes(hoje.Ano(), time.January), hoje)))
	}

	// Cotas na corretora × na carteira: "Conferir investimentos" mostra a diferença e a correção.
	cotasDiferentes := []string{}
	for _, cota := range cotasNaCorretora(investimentos) {
		i := slices.IndexFunc(posicoes, func(p PosicaoAtivoDto) bool { return p.Ticker == cota.ticker })
		if i >= 0 && !posicoes[i].Quantidade.Decimal().Sub(cota.quantidade.Decimal()).Abs().LessThan(diferencaDeCotas.Decimal()) {
			cotasDiferentes = append(cotasDiferentes, cota.ticker)
		}
	}
	slices.Sort(cotasDiferentes)

	ordenadas := slices.Clone(posicoes)
	slices.SortStableFunc(ordenadas, func(a, b PosicaoAtivoDto) int {
		return cmp.Or(cmp.Compare(b.Quantidade.Decimal().Sign(), a.Quantidade.Decimal().Sign()), b.ValorMercado.Decimal().Cmp(a.ValorMercado.Decimal()))
	})
	if c.err != nil {
		return nil, c.err
	}
	return &CarteiraDto{Selic: resumo, RendaVariavel: totais, Posicoes: ordenadas, Evolucao: evolucao, OpenFinance: openFinance,
		TotalOpenFinance: totalOpenFinance, TotalInvestimentosManuais: totalManuais,
		TotalGeral:   tipos.Somar(tipos.Somar(tipos.Somar(totais.ValorMercado, totalOpenFinance), totalManuais), totalReservas),
		Distribuicao: distribuicaoPorClasse(abertas, openFinance, totalManuais, totalReservas), CotacoesAtualizadasEm: maisAntiga,
		CotacoesDesatualizadas: desatualizadas, OpenFinanceNaoRegistrados: naoRegistrados, MoedasSemCotacao: conversor.MoedasSemCotacao(),
		Reservas: dasContas, TotalReservas: totalReservas, CotasDiferentes: cotasDiferentes}, nil
}

// Operacoes é o OperacoesAsync: as operações do ativo, da mais recente para a mais antiga, cada compra comparada com o
// preço atual e com a Selic. nil se o ativo não existe. A variação do preço é na moeda do ativo; o "vale hoje" é em
// reais, com o câmbio atual.
func (s *Servico) Operacoes(ctx context.Context, ativoId int64, serie *selic.Serie) ([]OperacaoAtivoDto, error) {
	ativos, err := s.lerAtivos(ctx, `WHERE "Id" = ?`, ativoId)
	if err != nil {
		return nil, err
	}
	if len(ativos) == 0 {
		return nil, nil
	}
	a := &ativos[0]
	conversor, err := s.cambio.ObterConversor(ctx, nil)
	if err != nil {
		return nil, err
	}
	emReais := precoEmReais(a, conversor)
	hoje := relogio.Hoje(s.relogio)
	operacoes := slices.Clone(a.Operacoes)
	slices.SortStableFunc(operacoes, func(x, y Operacao) int {
		return cmp.Or(cmp.Compare(y.Data.NumeroDoDia(), x.Data.NumeroDoDia()), cmp.Compare(y.Id, x.Id))
	})
	var c conta
	lista := []OperacaoAtivoDto{}
	for _, o := range operacoes {
		valor := valorDaOperacao(o)
		d := OperacaoAtivoDto{Id: o.Id, AtivoId: o.AtivoId, Ticker: a.Ticker, Tipo: o.Tipo, Data: o.Data, Quantidade: o.Quantidade,
			PrecoUnitario: o.PrecoUnitario, Taxas: o.Taxas, Moeda: a.Moeda, Cambio: o.Cambio, ValorTotal: tipos.Arredondar(valor, 2),
			Observacao: o.Observacao, PrecoAtual: a.PrecoAtual, Origem: o.Origem,
			ValorSeFosseSelic: tipos.Arredondar(tipos.Multiplicar(valor, c.fator(serie, o.Data, hoje)), 2)}
		if o.Tipo == operacaoCompra {
			if a.PrecoAtual != nil && positivo(o.PrecoUnitario) {
				d.VariacaoPreco = opcional(c.variacao(*a.PrecoAtual, o.PrecoUnitario))
			}
			if emReais != nil {
				d.ValorAtual = opcional(tipos.Arredondar(tipos.Multiplicar(o.Quantidade, *emReais), 2))
			}
		}
		lista = append(lista, d)
	}
	return lista, c.err
}
