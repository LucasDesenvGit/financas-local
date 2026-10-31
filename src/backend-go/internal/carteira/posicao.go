package carteira

import (
	"cmp"
	"slices"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/selic"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Operacao é uma linha de OperacaoAtivo. Preço e taxas estão na moeda do ativo, e o câmbio, em reais por 1 unidade dela.
type Operacao struct {
	Id            int64
	AtivoId       int64
	Tipo          int64
	Data          tipos.Data
	Quantidade    tipos.Decimal
	PrecoUnitario tipos.Decimal
	Taxas         tipos.Decimal
	Cambio        tipos.Decimal
	Observacao    *string
	Origem        int64
	TransacaoId   *int64 // o lançamento do extrato que originou a operação
}

// Provento é uma linha de ProventoAtivo: o valor por cota, na moeda do ativo.
type Provento struct {
	DataCom      tipos.Data
	ValorPorCota tipos.Decimal
}

// Ativo é uma linha de Ativo, com as operações e os proventos.
type Ativo struct {
	Id                int64
	Ticker            string
	Nome              *string
	Tipo              int64
	Moeda             string
	PrecoAtual        *tipos.Decimal
	PrecoAnterior     *tipos.Decimal
	PrecoAtualizadoEm *tipos.DataHora
	PrecoManual       bool
	FonteCotacao      *string
	Segmento          *string
	Operacoes         []Operacao
	Proventos         []Provento
}

// conta faz as contas que podem falhar (as exceções do decimal do C#) guardando o primeiro erro, para a fórmula ficar
// legível como a do C#. Quem usa confere o err no fim.
type conta struct{ err error }

// anotar guarda o primeiro erro.
func (c *conta) anotar(err error) {
	if err != nil && c.err == nil {
		c.err = err
	}
}

func (c *conta) dividir(a, b tipos.Decimal) tipos.Decimal {
	r, err := tipos.Dividir(a, b)
	c.anotar(err)
	return r
}

func (c *conta) fator(s *selic.Serie, de, ate tipos.Data) tipos.Decimal {
	r, err := s.Fator(de, ate)
	c.anotar(err)
	return r
}

// percentual é o (valor / base * 100), arredondado.
func (c *conta) percentual(valor, base tipos.Decimal, casas int32) tipos.Decimal {
	return tipos.Arredondar(tipos.Multiplicar(c.dividir(valor, base), cem), casas)
}

// variacao é o ((a / b - 1) * 100), arredondado a 2 casas.
func (c *conta) variacao(a, b tipos.Decimal) tipos.Decimal {
	return tipos.Arredondar(tipos.Multiplicar(tipos.Subtrair(c.dividir(a, b), um), cem), 2)
}

var (
	um  = tipos.DecimalDeInteiro(1)
	cem = tipos.DecimalDeInteiro(100)
	// quaseZero: a quantidade que sobra numa venda abaixo disso é resto de arredondamento.
	quaseZero = mustDecimal("0.0000005")
)

func mustDecimal(texto string) tipos.Decimal {
	d, err := tipos.NovoDecimal(texto)
	if err != nil {
		panic(err) // só com constantes do pacote
	}
	return d
}

func positivo(d tipos.Decimal) bool { return d.Decimal().IsPositive() }

// Posicao é o resultado das operações de um ativo pelo método do preço médio. Custo, PrecoMedio e LucroRealizado estão em
// reais (cada operação pelo seu câmbio); CustoNaMoeda e PrecoMedioNaMoeda, na moeda do ativo.
type Posicao struct {
	Quantidade        tipos.Decimal
	Custo             tipos.Decimal
	CustoNaMoeda      tipos.Decimal
	LucroRealizado    tipos.Decimal
	PrecoMedio        tipos.Decimal
	PrecoMedioNaMoeda tipos.Decimal
}

// ordenarParaPosicao é a ordem do Calcular: a data, a compra antes da venda no mesmo dia, e o Id.
func ordenarParaPosicao(operacoes []Operacao) []Operacao {
	ordenadas := slices.Clone(operacoes)
	slices.SortStableFunc(ordenadas, func(a, b Operacao) int {
		return cmp.Or(cmp.Compare(a.Data.NumeroDoDia(), b.Data.NumeroDoDia()), cmp.Compare(a.Tipo, b.Tipo), cmp.Compare(a.Id, b.Id))
	})
	return ordenadas
}

// Calcular é o Calcular do C#: processa as operações até a data (nil, todas). A venda sai pelo preço médio e nunca passa
// da quantidade em carteira; o lucro vai em reais pelo câmbio do dia da venda, e por isso inclui a variação da moeda.
func Calcular(operacoes []Operacao, ate *tipos.Data) (Posicao, error) {
	var p Posicao
	var c conta
	for _, op := range ordenarParaPosicao(operacoes) {
		if ate != nil && op.Data.Depois(*ate) {
			break
		}
		if op.Tipo == operacaoCompra {
			naMoeda := tipos.Somar(tipos.Multiplicar(op.Quantidade, op.PrecoUnitario), op.Taxas)
			p.Custo = tipos.Somar(p.Custo, tipos.Multiplicar(naMoeda, op.Cambio))
			p.CustoNaMoeda = tipos.Somar(p.CustoNaMoeda, naMoeda)
			p.Quantidade = tipos.Somar(p.Quantidade, op.Quantidade)
		} else if positivo(p.Quantidade) {
			quantidade := tipos.Menor(op.Quantidade, p.Quantidade)
			medio := c.dividir(p.Custo, p.Quantidade)
			medioNaMoeda := c.dividir(p.CustoNaMoeda, p.Quantidade)
			liquido := tipos.Multiplicar(tipos.Subtrair(tipos.Multiplicar(quantidade, op.PrecoUnitario), op.Taxas), op.Cambio)
			custoDaVenda := tipos.Multiplicar(medio, quantidade)
			p.LucroRealizado = tipos.Somar(p.LucroRealizado, tipos.Subtrair(liquido, custoDaVenda))
			p.Custo = tipos.Subtrair(p.Custo, custoDaVenda)
			p.CustoNaMoeda = tipos.Subtrair(p.CustoNaMoeda, tipos.Multiplicar(medioNaMoeda, quantidade))
			p.Quantidade = tipos.Subtrair(p.Quantidade, quantidade)
			if !p.Quantidade.Decimal().GreaterThan(quaseZero.Decimal()) {
				p.Quantidade, p.Custo, p.CustoNaMoeda = tipos.Zero, tipos.Zero, tipos.Zero
			}
		}
	}
	if positivo(p.Quantidade) {
		p.PrecoMedio = c.dividir(p.Custo, p.Quantidade)
		p.PrecoMedioNaMoeda = c.dividir(p.CustoNaMoeda, p.Quantidade)
	}
	return p, c.err
}

// valorDaOperacao é o valor em reais, pelo câmbio da própria operação.
func valorDaOperacao(op Operacao) tipos.Decimal {
	bruto := tipos.Multiplicar(op.Quantidade, op.PrecoUnitario)
	if op.Tipo == operacaoCompra {
		return tipos.Multiplicar(tipos.Somar(bruto, op.Taxas), op.Cambio)
	}
	return tipos.Multiplicar(tipos.Subtrair(bruto, op.Taxas), op.Cambio)
}

// precoEmReais é a cotação atual em reais; nil sem cotação do ativo ou da moeda.
func precoEmReais(a *Ativo, conversor *cambio.Conversor) *tipos.Decimal {
	if a.PrecoAtual == nil {
		return nil
	}
	taxa := conversor.Taxa(a.Moeda)
	if taxa == nil {
		return nil
	}
	r := tipos.Multiplicar(*a.PrecoAtual, *taxa)
	return &r
}

// fluxo é dinheiro entrando (+, aplicação) ou saindo (−, resgate) numa data.
type fluxo struct {
	data  tipos.Data
	valor tipos.Decimal
}

func fluxoDaOperacao(op Operacao) fluxo {
	v := valorDaOperacao(op)
	if op.Tipo != operacaoCompra {
		v = tipos.Subtrair(tipos.Zero, v)
	}
	return fluxo{op.Data, v}
}

func somaDosFluxos(lista []fluxo) tipos.Decimal {
	soma := tipos.Zero
	for _, f := range lista {
		soma = tipos.Somar(soma, f.valor)
	}
	return soma
}

// naSelic é quanto o mesmo dinheiro valeria na data, rendendo a Selic.
func (c *conta) naSelic(lista []fluxo, serie *selic.Serie, ate tipos.Data) tipos.Decimal {
	soma := tipos.Zero
	for _, f := range lista {
		soma = tipos.Somar(soma, tipos.Multiplicar(f.valor, c.fator(serie, f.data, ate)))
	}
	return soma
}
