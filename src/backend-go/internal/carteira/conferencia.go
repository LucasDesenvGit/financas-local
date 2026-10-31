package carteira

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cambio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/cotacoes"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// "Conferir investimentos" (ADR 0027): procura o que costuma sair errado com a renda variável que chega pelo Open Finance
// e sugere a correção. Nada muda sem você confirmar, e a correção pode ser desfeita por alguns minutos:
//   - FII cadastrado como ação ou "outro": o nome do fundo, ou o emissor informado pela corretora, diz que é FII;
//   - cotas que faltam em relação à corretora: o débito de compra que nenhuma operação explica é sugerido;
//   - créditos da corretora: o Nubank usa a mesma descrição para o troco das compras, o valor de uma venda e os proventos.
//     Troco e venda vão para a categoria da compra; provento conta como receita;
//   - #173 a): a taxa da B3 que falta nas compras (o débito menos o troco menos cotas × preço).
//
// #173 b) (ADR 0044): depois da sincronização, o troco "sem dúvida" vai sozinho para a categoria da compra, com aviso e
// "Desfazer" no Painel.

const (
	// A operação (data do pregão) vem até 3 dias antes ou 1 dia depois do débito no extrato.
	diasAntesDoDebito, diasDepoisDoDebito = 3, 1
	// O troco cai no dia do débito ou até 3 dias depois; a sobra de centavos, até 5 dias depois.
	diasDoTroco, diasDaSobra = 3, 5
	// O valor de uma venda cai até 5 dias depois do pregão (D+2 úteis).
	diasDaVenda = 5
	// ChaveTrocosAutomaticos guarda os trocos classificados na sincronização, para o aviso e o "Desfazer" do Painel.
	ChaveTrocosAutomaticos = "Investimentos.TrocosAutomaticos"
	maximoTrocosGuardados  = 500

	tipoCategoriaReceita, tipoCategoriaTransferencia = 2, 3
	tipoTransacaoTransferencia                       = 3
)

var (
	// trocoMaximo: troco de até 2,5% do débito (a sobra devolvida pelo Nubank fica abaixo de 2%).
	trocoMaximo = mustDecimal("0.025")
	// folgaDebito: o débito da compra fica entre o custo da operação e 4% acima (troco ainda não devolvido + taxas).
	folgaDebito = mustDecimal("0.04")
	// folgaDebitoAbaixo: arredondamento para baixo do custo.
	folgaDebitoAbaixo = mustDecimal("-0.002")
	// folgaVenda e folgaVendaReais: até 2% (ou R$ 0,05) de diferença no valor de uma venda.
	folgaVenda, folgaVendaReais = mustDecimal("0.02"), mustDecimal("0.05")
	// folgaPreco: preço de uma compra que faltou, até 3,5% de diferença da cotação de fechamento daquele dia.
	folgaPreco = mustDecimal("0.035")
	// taxaMaxima e taxaMaximaReais: a taxa da B3 sugerida só até 1% do valor das cotas e R$ 5 (a de verdade é de centavos a
	// cada R$ 1.000); acima disso é troco que não voltou ou outra coisa.
	taxaMaxima, taxaMaximaReais = mustDecimal("0.01"), mustDecimal("5")
)

// movimento é um débito ou crédito do extrato que pode ser compra de renda variável, troco, venda ou provento.
type movimento struct {
	id, contaId       int64
	conta             string
	data              tipos.Data
	descricao         string
	original          *string
	pluggyCategoriaId *string
	valor             tipos.Decimal
	tipo              int64
	categoria         *string
	categoriaManual   bool
}

// ordem é uma ordem de compra: uma operação ou, quando a corretora divide a execução, as do mesmo ativo e dia.
type ordem struct {
	operacoes []*Operacao
	custo     tipos.Decimal
}

type compraSemOperacao struct {
	debito movimento
	troco  *movimento
}

// creditoDeOperacao é o crédito da corretora que não é provento: troco da compra, ou valor da venda.
type creditoDeOperacao struct {
	credito     movimento
	compra      *movimento
	vendaTicker string
	venda       *Operacao
}

type taxaDaOperacao struct {
	operacao *Operacao
	ticker   string
	taxa     tipos.Decimal
}

// taxaDoDia é a taxa da B3 de um dia de compras (#173 a), e a parte de cada operação.
type taxaDoDia struct {
	data                                  tipos.Data
	conta                                 string
	debitado, trocos, valorDasCotas, taxa tipos.Decimal
	operacoes                             []taxaDaOperacao
}

// analise é o que o extrato diz. trocosSemDuvida (#173 b): os trocos dos dias em que a conta fecha, só sobrando a taxa.
type analise struct {
	semOperacao     []compraSemOperacao
	deOperacao      []creditoDeOperacao
	creditos        []movimento
	taxas           []taxaDoDia
	trocosSemDuvida map[int64]bool
}

func custoDaOperacao(o *Operacao) tipos.Decimal {
	return tipos.Somar(tipos.Multiplicar(o.Quantidade, o.PrecoUnitario), o.Taxas)
}

func somarOperacoes(lista []*Operacao, campo func(*Operacao) tipos.Decimal) tipos.Decimal {
	return somarPor(lista, campo)
}

func cotasVezesPreco(o *Operacao) tipos.Decimal {
	return tipos.Multiplicar(o.Quantidade, o.PrecoUnitario)
}

func valorAbsoluto(d tipos.Decimal) tipos.Decimal { return tipos.DeDecimal(d.Decimal().Abs()) }

func negado(d tipos.Decimal) tipos.Decimal { return tipos.Subtrair(tipos.Zero, d) }

func maiorQue(a, b tipos.Decimal) bool { return a.Decimal().GreaterThan(b.Decimal()) }

func menorQue(a, b tipos.Decimal) bool { return a.Decimal().LessThan(b.Decimal()) }

func ehCompraSemCodigo(descricao string) bool {
	t := texto.ParaRegras(descricao)
	return strings.Contains(t, "RENDA VARIAVEL") && !strings.Contains(t, "VENDA") && !strings.Contains(t, "RESGATE")
}

func ehCreditoDaCorretora(descricao string) bool {
	t := texto.ParaRegras(descricao)
	return strings.Contains(t, "INVEST") && !strings.Contains(t, "RESGATE") && !strings.Contains(t, "FUNDO")
}

// ordensPerto são as ordens livres perto da data do débito: cada operação e, com mais de uma, as do mesmo ativo e dia juntas.
func ordensPerto(livres []*Operacao, data tipos.Data) []ordem {
	var lista []ordem
	var grupos [][]*Operacao
	for _, o := range livres {
		if o.Data.Antes(data.SomarDias(-diasAntesDoDebito)) || o.Data.Depois(data.SomarDias(diasDepoisDoDebito)) {
			continue
		}
		lista = append(lista, ordem{[]*Operacao{o}, custoDaOperacao(o)})
		i := slices.IndexFunc(grupos, func(g []*Operacao) bool { return g[0].AtivoId == o.AtivoId && g[0].Data.Igual(o.Data) })
		if i < 0 {
			grupos = append(grupos, []*Operacao{o})
		} else {
			grupos[i] = append(grupos[i], o)
		}
	}
	for _, g := range grupos {
		if len(g) > 1 {
			lista = append(lista, ordem{g, somarOperacoes(g, custoDaOperacao)})
		}
	}
	return lista
}

// dividirTaxa divide a taxa do dia pelas operações, pelo valor de cada uma, com o resto de centavos na maior.
func dividirTaxa(ops []*Operacao, taxa tipos.Decimal, c *conta) []taxaDaOperacao {
	total := somarOperacoes(ops, cotasVezesPreco)
	partes := make([]taxaDaOperacao, len(ops))
	maior := 0
	for i, o := range ops {
		partes[i] = taxaDaOperacao{operacao: o, taxa: tipos.Zero}
		if positivo(total) {
			partes[i].taxa = tipos.Arredondar(c.dividir(tipos.Multiplicar(tipos.Multiplicar(taxa, o.Quantidade), o.PrecoUnitario), total), 2)
		}
		if maiorQue(cotasVezesPreco(o), cotasVezesPreco(ops[maior])) {
			maior = i
		}
	}
	soma := somarPor(partes, func(p taxaDaOperacao) tipos.Decimal { return p.taxa })
	partes[maior].taxa = tipos.Subtrair(tipos.Somar(partes[maior].taxa, taxa), soma)
	return slices.DeleteFunc(partes, func(p taxaDaOperacao) bool { return !positivo(p.taxa) })
}

// lerMovimentos lê os lançamentos que podem interessar. O LIKE fica no SQL, para dar o mesmo do EF; a comparação do valor,
// que é texto no banco, fica em Go.
func (s *Servico) lerMovimentos(ctx context.Context, desde tipos.Data, ligadas map[int64]*Operacao) ([]movimento, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT t."Id", t."ContaId", c."Nome", t."Data", t."Descricao", t."DescricaoOriginal", t."PluggyCategoriaId",
		t."Valor", t."Tipo", x."Nome", t."CategoriaManual",
		t."Descricao" LIKE '%renda vari%' OR (t."DescricaoOriginal" IS NOT NULL AND t."DescricaoOriginal" LIKE '%renda vari%'),
		t."Descricao" LIKE '%invest%' OR (t."DescricaoOriginal" IS NOT NULL AND t."DescricaoOriginal" LIKE '%invest%')
		FROM "Transacao" AS t INNER JOIN "Conta" AS c ON t."ContaId" = c."Id" LEFT JOIN "Categoria" AS x ON t."CategoriaId" = x."Id"
		WHERE t."Data" >= ? AND t."TransferenciaGrupo" IS NULL`, desde)
	if err != nil {
		return nil, fmt.Errorf("ler o extrato da corretora: %w", err)
	}
	defer linhas.Close()
	var lista []movimento
	for linhas.Next() {
		var m movimento
		var ehRenda, ehInvest bool
		if err := linhas.Scan(&m.id, &m.contaId, &m.conta, &m.data, &m.descricao, &m.original, &m.pluggyCategoriaId, &m.valor, &m.tipo, &m.categoria,
			&m.categoriaManual, &ehRenda, &ehInvest); err != nil {
			return nil, fmt.Errorf("ler lançamento da corretora: %w", err)
		}
		_, ligada := ligadas[m.id]
		if (m.valor.Decimal().IsNegative() && (ligada || ehRenda)) || (positivo(m.valor) && ehInvest) {
			lista = append(lista, m)
		}
	}
	return lista, linhas.Err()
}

// analisarDia liga as compras de um dia às operações e separa os trocos.
type parDoDia struct {
	debito    movimento
	custo     tipos.Decimal
	operacoes []*Operacao
}

// analisador guarda o estado da análise entre os dias.
type analisador struct {
	a          *analise
	c          *conta
	hoje       tipos.Data
	livres     []*Operacao
	usados     map[int64]bool
	ligadas    map[int64]*Operacao
	doDebito   map[int64][]*Operacao
	tickerDoId map[int64]string
}

func (x *analisador) ligarOrdens(dia []movimento, data tipos.Data) (pares []parDoDia, soltos []movimento) {
	type candidata struct {
		debito movimento
		ordem  ordem
		sobra  tipos.Decimal
	}
	var candidatas []candidata
	var semLigacao []movimento
	for _, d := range dia {
		if _, ok := x.ligadas[d.id]; ok {
			ops := x.doDebito[d.id]
			pares = append(pares, parDoDia{d, somarOperacoes(ops, custoDaOperacao), ops})
			continue
		}
		semLigacao = append(semLigacao, d)
		for _, o := range ordensPerto(x.livres, data) {
			if !positivo(o.custo) {
				continue
			}
			sobra := tipos.Subtrair(x.c.dividir(negado(d.valor), o.custo), um)
			if !menorQue(sobra, folgaDebitoAbaixo) && !maiorQue(sobra, folgaDebito) {
				candidatas = append(candidatas, candidata{d, o, sobra})
			}
		}
	}
	slices.SortStableFunc(candidatas, func(a, b candidata) int { return a.sobra.Decimal().Cmp(b.sobra.Decimal()) })
	ligados := map[int64]bool{}
	for _, c := range candidatas {
		todasLivres := !slices.ContainsFunc(c.ordem.operacoes, func(o *Operacao) bool { return !slices.Contains(x.livres, o) })
		if ligados[c.debito.id] || !todasLivres {
			continue
		}
		ligados[c.debito.id] = true
		x.livres = slices.DeleteFunc(x.livres, func(o *Operacao) bool { return slices.Contains(c.ordem.operacoes, o) })
		pares = append(pares, parDoDia{c.debito, c.ordem.custo, c.ordem.operacoes})
	}
	for _, d := range semLigacao {
		if !ligados[d.id] {
			soltos = append(soltos, d)
		}
	}
	return pares, soltos
}

// contaDoDia é o passo 5: com todas as compras ligadas e o tempo do troco passado, a taxa da B3 que falta e os trocos sem
// dúvida (sobra só a taxa, a lançada ou a que falta).
func (x *analisador) contaDoDia(pares []parDoDia, trocosDoDia []movimento, conta string, data tipos.Data) {
	var ops []*Operacao
	for _, p := range pares {
		for _, o := range p.operacoes {
			if !slices.Contains(ops, o) {
				ops = append(ops, o)
			}
		}
	}
	debitado := somarPor(pares, func(p parDoDia) tipos.Decimal { return negado(p.debito.valor) })
	trocos := somarPor(trocosDoDia, func(m movimento) tipos.Decimal { return m.valor })
	cotas := somarOperacoes(ops, cotasVezesPreco)
	maximo := tipos.Menor(tipos.Multiplicar(cotas, taxaMaxima), taxaMaximaReais)
	taxa := tipos.Arredondar(tipos.Subtrair(tipos.Subtrair(debitado, trocos), cotas), 2)
	semTaxa := tipos.Arredondar(tipos.Subtrair(tipos.Subtrair(tipos.Subtrair(debitado, trocos), cotas), somarOperacoes(ops, func(o *Operacao) tipos.Decimal { return o.Taxas })), 2)
	if !semTaxa.Decimal().IsNegative() && !maiorQue(semTaxa, maximo) {
		for _, m := range trocosDoDia {
			x.a.trocosSemDuvida[m.id] = true
		}
	}
	semTaxasLancadas := !slices.ContainsFunc(ops, func(o *Operacao) bool { return !o.Taxas.Decimal().IsZero() })
	if semTaxasLancadas && !menorQue(taxa, centavo) && !maiorQue(taxa, maximo) {
		partes := dividirTaxa(ops, taxa, x.c)
		for i := range partes {
			partes[i].ticker = x.tickerDoId[partes[i].operacao.AtivoId]
		}
		x.a.taxas = append(x.a.taxas, taxaDoDia{data, conta, debitado, trocos, tipos.Arredondar(cotas, 2), taxa, partes})
	}
}

func (x *analisador) analisarDia(dia, creditos []movimento) {
	data, contaId := dia[0].data, dia[0].contaId
	pares, soltos := x.ligarOrdens(dia, data)

	var perto []movimento
	for _, c := range creditos {
		if c.contaId == contaId && !c.data.Antes(data) && !c.data.Depois(data.SomarDias(diasDaSobra)) && !x.usados[c.id] {
			perto = append(perto, c)
		}
	}
	slices.SortStableFunc(perto, func(a, b movimento) int {
		return cmp.Or(cmp.Compare(a.data.NumeroDoDia(), b.data.NumeroDoDia()), a.valor.Decimal().Cmp(b.valor.Decimal()))
	})

	// 2) Um troco por compra: os créditos mais perto da data e, no mesmo dia, os menores.
	trocoDe := map[int64]movimento{}
	var trocos []movimento
	abertos := slices.Clone(dia)
	slices.SortStableFunc(abertos, func(a, b movimento) int { return b.valor.Decimal().Cmp(a.valor.Decimal()) })
	for _, c := range perto {
		if c.data.Depois(data.SomarDias(diasDoTroco)) {
			continue // em ordem de data: daqui em diante, tudo passou do prazo do troco
		}
		i := slices.IndexFunc(abertos, func(d movimento) bool {
			_, tem := trocoDe[d.id]
			return !tem && !maiorQue(c.valor, tipos.Multiplicar(negado(d.valor), trocoMaximo))
		})
		if i < 0 {
			continue
		}
		d := abertos[i]
		trocoDe[d.id] = c
		trocos = append(trocos, c)
		x.usados[c.id] = true
		x.a.deOperacao = append(x.a.deOperacao, creditoDeOperacao{credito: c, compra: &d})
	}

	// 3) Sobra de centavos: com todas as compras ligadas, os créditos menores entram enquanto a soma dos trocos não passa do
	// que os débitos tiveram a mais que o custo.
	completo := len(pares) == len(dia)
	var sobras []movimento
	if completo {
		restante := tipos.Subtrair(somarPor(pares, func(p parDoDia) tipos.Decimal { return tipos.Subtrair(negado(p.debito.valor), p.custo) }),
			somarPor(trocos, func(m movimento) tipos.Decimal { return m.valor }))
		maior := pares[0].debito
		for _, p := range pares {
			if menorQue(p.debito.valor, maior.valor) {
				maior = p.debito
			}
		}
		var livresDoDia []movimento
		for _, c := range perto {
			if !x.usados[c.id] {
				livresDoDia = append(livresDoDia, c)
			}
		}
		slices.SortStableFunc(livresDoDia, func(a, b movimento) int { return a.valor.Decimal().Cmp(b.valor.Decimal()) })
		for _, c := range livresDoDia {
			if maiorQue(c.valor, restante) {
				break // em ordem de valor: os próximos também passam do que sobra
			}
			restante = tipos.Subtrair(restante, c.valor)
			sobras = append(sobras, c)
			x.usados[c.id] = true
			x.a.deOperacao = append(x.a.deOperacao, creditoDeOperacao{credito: c, compra: &maior})
		}
	}

	// 5) A conta do dia, quando o troco já teve os dias dele para voltar.
	if completo && !data.Depois(x.hoje.SomarDias(-diasDaSobra)) {
		x.contaDoDia(pares, append(slices.Clone(trocos), sobras...), dia[0].conta, data)
	}
	for _, d := range soltos {
		item := compraSemOperacao{debito: d}
		if t, ok := trocoDe[d.id]; ok {
			item.troco = &t
		}
		x.a.semOperacao = append(x.a.semOperacao, item)
	}
}

// ligarVendas é o passo 4: o valor da venda volta como crédito da corretora.
func (x *analisador) ligarVendas(vendas []*Operacao, creditos []movimento) {
	vendas = slices.Clone(vendas)
	slices.SortStableFunc(vendas, func(a, b *Operacao) int { return cmp.Compare(a.Data.NumeroDoDia(), b.Data.NumeroDoDia()) })
	for _, venda := range vendas {
		valor := tipos.Subtrair(cotasVezesPreco(venda), venda.Taxas)
		if !positivo(valor) {
			continue
		}
		folga := tipos.Maior(folgaVendaReais, tipos.Multiplicar(valor, folgaVenda))
		var melhor *movimento
		for i := range creditos {
			c := &creditos[i]
			diferenca := valorAbsoluto(tipos.Subtrair(c.valor, valor))
			if x.usados[c.id] || c.data.Antes(venda.Data) || c.data.Depois(venda.Data.SomarDias(diasDaVenda)) || maiorQue(diferenca, folga) {
				continue
			}
			if melhor == nil || menorQue(diferenca, valorAbsoluto(tipos.Subtrair(melhor.valor, valor))) {
				melhor = c
			}
		}
		if melhor != nil {
			x.usados[melhor.id] = true
			x.a.deOperacao = append(x.a.deOperacao, creditoDeOperacao{credito: *melhor, vendaTicker: x.tickerDoId[venda.AtivoId], venda: venda})
		}
	}
}

// analisarExtrato é o AnalisarExtratoAsync: liga os débitos de compra às operações e separa, nos créditos da corretora, o
// que é troco de compra ou valor de venda (o resto é provento).
func (s *Servico) analisarExtrato(ctx context.Context, ativos []Ativo) (*analise, error) {
	a := &analise{trocosSemDuvida: map[int64]bool{}}
	var c conta
	x := &analisador{a: a, c: &c, hoje: relogio.Hoje(s.relogio), usados: map[int64]bool{}, ligadas: map[int64]*Operacao{}, doDebito: map[int64][]*Operacao{},
		tickerDoId: map[int64]string{}}
	var vendas []*Operacao
	var desde *tipos.Data
	for i := range ativos {
		x.tickerDoId[ativos[i].Id] = ativos[i].Ticker
		if ativos[i].Moeda != cambio.MoedaBase {
			continue
		}
		for j := range ativos[i].Operacoes {
			o := &ativos[i].Operacoes[j]
			if desde == nil || o.Data.Antes(*desde) {
				desde = &o.Data
			}
			switch {
			case o.TransacaoId != nil:
				if _, ok := x.ligadas[*o.TransacaoId]; !ok {
					x.ligadas[*o.TransacaoId] = o
				}
				x.doDebito[*o.TransacaoId] = append(x.doDebito[*o.TransacaoId], o)
			case o.Tipo == operacaoCompra:
				x.livres = append(x.livres, o)
			default:
				vendas = append(vendas, o)
			}
		}
	}
	if desde == nil {
		return a, nil
	}
	movimentos, err := s.lerMovimentos(ctx, desde.SomarDias(-7), x.ligadas)
	if err != nil {
		return nil, err
	}
	var debitos []movimento
	for _, m := range movimentos {
		op, ligada := x.ligadas[m.id]
		switch {
		case m.valor.Decimal().IsNegative():
			if (ligada && op.Tipo == operacaoCompra) || (!ligada && ehCompraSemCodigo(m.descricao)) {
				debitos = append(debitos, m)
			}
		case positivo(m.valor) && !ligada && ehCreditoDaCorretora(m.descricao):
			a.creditos = append(a.creditos, m)
		}
	}
	// Os dias, por conta e data, na ordem da data.
	var dias [][]movimento
	for _, d := range debitos {
		i := slices.IndexFunc(dias, func(g []movimento) bool { return g[0].contaId == d.contaId && g[0].data.Igual(d.data) })
		if i < 0 {
			dias = append(dias, []movimento{d})
		} else {
			dias[i] = append(dias[i], d)
		}
	}
	slices.SortStableFunc(dias, func(a, b []movimento) int { return cmp.Compare(a[0].data.NumeroDoDia(), b[0].data.NumeroDoDia()) })
	for _, dia := range dias {
		x.analisarDia(dia, a.creditos)
	}
	x.ligarVendas(vendas, a.creditos)
	return a, c.err
}

// ativosPeloId são os ativos na ordem do Id, a que o EF devolve sem o OrderBy.
func (s *Servico) ativosPeloId(ctx context.Context) ([]Ativo, error) {
	ativos, err := s.lerAtivos(ctx, "")
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(ativos, func(a, b Ativo) int { return cmp.Compare(a.Id, b.Id) })
	return ativos, nil
}

// CreditoRecebido é um crédito de provento da corretora.
type CreditoRecebido struct {
	Id    int64
	Data  tipos.Data
	Valor tipos.Decimal
}

// ProventosDaCorretora é o ProventosDaCorretoraAsync (#173): os créditos da corretora que não são troco de compra nem valor
// de venda. Vale com ou sem a conferência aplicada: a categoria do crédito não entra na conta.
func (s *Servico) ProventosDaCorretora(ctx context.Context) ([]CreditoRecebido, error) {
	ativos, err := s.ativosPeloId(ctx)
	if err != nil {
		return nil, err
	}
	a, err := s.analisarExtrato(ctx, ativos)
	if err != nil {
		return nil, err
	}
	deOperacao := idsDeOperacao(a)
	var lista []CreditoRecebido
	for _, c := range a.creditos {
		if !deOperacao[c.id] {
			lista = append(lista, CreditoRecebido{c.id, c.data, c.valor})
		}
	}
	return lista, nil
}

func idsDeOperacao(a *analise) map[int64]bool {
	ids := map[int64]bool{}
	for _, c := range a.deOperacao {
		ids[c.credito.id] = true
	}
	return ids
}

// CreditosComoReceita é o CreditosComoReceitaAsync (#170): os trocos e vendas da corretora que ainda contam como receita,
// para o aviso do Painel.
func (s *Servico) CreditosComoReceita(ctx context.Context) (int, tipos.Decimal, error) {
	ativos, err := s.ativosPeloId(ctx)
	if err != nil {
		return 0, tipos.Zero, err
	}
	a, err := s.analisarExtrato(ctx, ativos)
	if err != nil {
		return 0, tipos.Zero, err
	}
	quantidade, total := 0, tipos.Zero
	for _, c := range a.deOperacao {
		if c.credito.tipo != tipoTransacaoTransferencia {
			quantidade++
			total = tipos.Somar(total, c.credito.valor)
		}
	}
	return quantidade, total, nil
}

// categoriaDoTroco é o CategoriaDoTrocoAsync: "Investimentos", a mesma da compra, ou outra do tipo transferência.
func (s *Servico) categoriaDoTroco(ctx context.Context) (*CategoriaDoTrocoDto, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Nome" FROM "Categoria" WHERE "Tipo" = ? ORDER BY "Nome"`, tipoCategoriaTransferencia)
	if err != nil {
		return nil, fmt.Errorf("ler as categorias de transferência: %w", err)
	}
	defer linhas.Close()
	var categorias []CategoriaDoTrocoDto
	for linhas.Next() {
		var c CategoriaDoTrocoDto
		if err := linhas.Scan(&c.Id, &c.Nome); err != nil {
			return nil, fmt.Errorf("ler categoria: %w", err)
		}
		categorias = append(categorias, c)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as categorias de transferência: %w", err)
	}
	i := slices.IndexFunc(categorias, func(c CategoriaDoTrocoDto) bool { return c.Nome == "Investimentos" })
	if i == -1 {
		i = slices.IndexFunc(categorias, func(c CategoriaDoTrocoDto) bool { return strings.Contains(texto.ParaRegras(c.Nome), "INVEST") })
	}
	if i < 0 {
		return nil, nil
	}
	return &categorias[i], nil
}

// infoDaCategoria é o nome e o tipo de uma categoria.
type infoDaCategoria struct {
	nome string
	tipo int64
}

func (s *Servico) lerCategorias(ctx context.Context) (map[int64]infoDaCategoria, error) {
	linhas, err := s.db.QueryContext(ctx, `SELECT "Id", "Nome", "Tipo" FROM "Categoria"`)
	if err != nil {
		return nil, fmt.Errorf("ler as categorias: %w", err)
	}
	defer linhas.Close()
	categorias := map[int64]infoDaCategoria{}
	for linhas.Next() {
		var id int64
		var c infoDaCategoria
		if err := linhas.Scan(&id, &c.nome, &c.tipo); err != nil {
			return nil, fmt.Errorf("ler categoria: %w", err)
		}
		categorias[id] = c
	}
	return categorias, linhas.Err()
}

// categoriaDeRendimento é a categoria que as regras dão ao crédito, se for de receita; senão, o crédito fica como está.
func categoriaDeRendimento(rs *categorizacao.Regras, descricao string, pluggy *string, categorias map[int64]infoDaCategoria) *int64 {
	id := rs.ResolverTexto(descricao, pluggy)
	if id == nil || categorias[*id].tipo != tipoCategoriaReceita {
		return nil
	}
	return id
}

// Os DTOs da conferência, os do C#.
type (
	// TipoParaCorrigirDto é o ativo cadastrado como ação ou "outro" que é FII.
	TipoParaCorrigirDto struct {
		AtivoId   int64   `json:"ativoId"`
		Ticker    string  `json:"ticker"`
		Nome      *string `json:"nome"`
		TipoAtual int64   `json:"tipoAtual"`
		Motivo    string  `json:"motivo"`
	}
	// CotasDiferentesDto são as cotas na corretora × na carteira, com o débito sugerido quando faltam.
	CotasDiferentesDto struct {
		AtivoId          int64          `json:"ativoId"`
		Ticker           string         `json:"ticker"`
		NaCorretora      tipos.Decimal  `json:"naCorretora"`
		NaCarteira       tipos.Decimal  `json:"naCarteira"`
		PosicaoEm        tipos.DataHora `json:"posicaoEm"`
		DebitoSugeridoId *int64         `json:"debitoSugeridoId"`
		PrecoSugerido    *tipos.Decimal `json:"precoSugerido"`
		FechamentoNoDia  *tipos.Decimal `json:"fechamentoNoDia"`
	}
	// CompraSemOperacaoDto é o débito de compra que nenhuma operação explica.
	CompraSemOperacaoDto struct {
		TransacaoId int64         `json:"transacaoId"`
		Data        tipos.Data    `json:"data"`
		Valor       tipos.Decimal `json:"valor"`
		Troco       tipos.Decimal `json:"troco"`
		Conta       string        `json:"conta"`
		Descricao   string        `json:"descricao"`
	}
	// CreditoDaCorretoraDto é o crédito que é troco de uma compra ou o valor de uma venda.
	CreditoDaCorretoraDto struct {
		TransacaoId     int64          `json:"transacaoId"`
		Data            tipos.Data     `json:"data"`
		Valor           tipos.Decimal  `json:"valor"`
		Descricao       string         `json:"descricao"`
		Conta           string         `json:"conta"`
		Categoria       *string        `json:"categoria"`
		CompraData      *tipos.Data    `json:"compraData"`
		CompraValor     *tipos.Decimal `json:"compraValor"`
		VendaTicker     *string        `json:"vendaTicker"`
		VendaQuantidade *tipos.Decimal `json:"vendaQuantidade"`
		VendaData       *tipos.Data    `json:"vendaData"`
	}
	// RendimentoComoInvestimentoDto é o provento numa categoria de transferência.
	RendimentoComoInvestimentoDto struct {
		TransacaoId         int64         `json:"transacaoId"`
		Data                tipos.Data    `json:"data"`
		Valor               tipos.Decimal `json:"valor"`
		Descricao           string        `json:"descricao"`
		Conta               string        `json:"conta"`
		Categoria           *string       `json:"categoria"`
		CategoriaAutomatica string        `json:"categoriaAutomatica"`
	}
	// CategoriaDoTrocoDto é a categoria para onde vai o troco.
	CategoriaDoTrocoDto struct {
		Id   int64  `json:"id"`
		Nome string `json:"nome"`
	}
	// TaxaB3OperacaoDto é a parte de uma operação na taxa do dia.
	TaxaB3OperacaoDto struct {
		OperacaoId int64         `json:"operacaoId"`
		Ticker     string        `json:"ticker"`
		Quantidade tipos.Decimal `json:"quantidade"`
		Taxa       tipos.Decimal `json:"taxa"`
	}
	// TaxaB3Dto é a taxa da B3 de um dia de compras.
	TaxaB3Dto struct {
		Data          tipos.Data          `json:"data"`
		Conta         string              `json:"conta"`
		Debitado      tipos.Decimal       `json:"debitado"`
		Trocos        tipos.Decimal       `json:"trocos"`
		ValorDasCotas tipos.Decimal       `json:"valorDasCotas"`
		Taxa          tipos.Decimal       `json:"taxa"`
		Operacoes     []TaxaB3OperacaoDto `json:"operacoes"`
	}
	// ConferenciaInvestimentosDto é o resultado da conferência.
	ConferenciaInvestimentosDto struct {
		Tipos              []TipoParaCorrigirDto           `json:"tipos"`
		Cotas              []CotasDiferentesDto            `json:"cotas"`
		ComprasSemOperacao []CompraSemOperacaoDto          `json:"comprasSemOperacao"`
		Trocos             []CreditoDaCorretoraDto         `json:"trocos"`
		Rendimentos        []RendimentoComoInvestimentoDto `json:"rendimentos"`
		CategoriaDoTroco   *CategoriaDoTrocoDto            `json:"categoriaDoTroco"`
		Taxas              []TaxaB3Dto                     `json:"taxas"`
	}
)

// tiposParaCorrigir são os FIIs cadastrados como ação ou "outro".
func tiposParaCorrigir(ativos []Ativo, investimentos []investimento) []TipoParaCorrigirDto {
	lista := []TipoParaCorrigirDto{}
	for _, a := range ativos {
		if (a.Tipo != tipoAcao && a.Tipo != tipoOutro) || a.Moeda != cambio.MoedaBase {
			continue
		}
		motivo := ""
		if cotacoes.PareceFii(a.Nome) {
			motivo = "o nome do fundo é “" + *a.Nome + "”"
		} else {
			for _, i := range investimentos {
				t := tickerDoInvestimento(i)
				if t == nil || *t != a.Ticker {
					continue
				}
				if cotacoes.PareceFii(i.Emissor) {
					motivo = "a corretora informa “" + *i.Emissor + "”"
					break
				}
				if cotacoes.PareceFii(&i.Nome) {
					motivo = "a corretora informa “" + i.Nome + "”"
					break
				}
			}
		}
		if motivo != "" {
			lista = append(lista, TipoParaCorrigirDto{a.Id, a.Ticker, a.Nome, a.Tipo, motivo})
		}
	}
	return lista
}

// fechamentosDoPeriodo são os fechamentos diários do Yahoo; sem resposta, o fechamento do mês guardado no banco.
func (s *Servico) fechamentosDoPeriodo(ctx context.Context, a *Ativo, de, ate tipos.Data) ([]cotacoes.Fechamento, error) {
	diarios, err := s.cotacoes.FechamentosDiarios(ctx, a.Ticker, de, ate)
	if err != nil {
		return nil, err
	}
	if len(diarios) > 0 {
		return emOrdemDeData(diarios), nil
	}
	linhas, err := s.db.QueryContext(ctx, `SELECT "Mes", "Fechamento" FROM "PrecoMensalAtivo" WHERE "AtivoId" = ? AND "Mes" >= ? AND "Mes" <= ?`,
		a.Id, relogio.PrimeiroDiaMes(de.Ano(), de.Mes()), ate)
	if err != nil {
		return nil, fmt.Errorf("ler os fechamentos mensais: %w", err)
	}
	defer linhas.Close()
	var mensais []cotacoes.Fechamento
	for linhas.Next() {
		var f cotacoes.Fechamento
		if err := linhas.Scan(&f.Data, &f.Valor); err != nil {
			return nil, fmt.Errorf("ler fechamento mensal: %w", err)
		}
		mensais = append(mensais, f)
	}
	return emOrdemDeData(mensais), linhas.Err()
}

// cotaDiferente é a diferença de cotas de um ativo.
type cotaDiferente struct {
	ativo                   *Ativo
	naCorretora, naCarteira tipos.Decimal
	em                      tipos.DataHora
}

// sugestaoDeCompra é o débito sugerido para as cotas que faltam.
type sugestaoDeCompra struct {
	ativoId, debitoId         int64
	preco, fechamento, desvio tipos.Decimal
}

// candidatosDeCompra são os débitos sem operação que explicam as cotas que faltam, pelo preço perto do fechamento do dia.
func (s *Servico) candidatosDeCompra(ctx context.Context, d cotaDiferente, semOperacao []compraSemOperacao, c *conta) ([]sugestaoDeCompra, error) {
	faltam := tipos.Subtrair(d.naCorretora, d.naCarteira)
	ateEm := tipos.DataDe(d.em.Tempo())
	var debitos []compraSemOperacao
	for _, x := range semOperacao {
		if !x.debito.data.Depois(ateEm) {
			debitos = append(debitos, x)
		}
	}
	if len(debitos) == 0 {
		return nil, nil
	}
	de, ate := debitos[0].debito.data, debitos[0].debito.data
	for _, x := range debitos {
		if x.debito.data.Antes(de) {
			de = x.debito.data
		}
		if x.debito.data.Depois(ate) {
			ate = x.debito.data
		}
	}
	fechamentos, err := s.fechamentosDoPeriodo(ctx, d.ativo, de, ate)
	if err != nil {
		return nil, err
	}
	var lista []sugestaoDeCompra
	for _, x := range debitos {
		fechamento := fechamentoEm(fechamentos, x.debito.data)
		if fechamento == nil || !positivo(*fechamento) {
			continue
		}
		pago := negado(x.debito.valor)
		if x.troco != nil {
			pago = tipos.Subtrair(pago, x.troco.valor)
		}
		preco := tipos.Arredondar(c.dividir(pago, faltam), 4)
		desvio := valorAbsoluto(tipos.Subtrair(c.dividir(preco, *fechamento), um))
		if !maiorQue(desvio, folgaPreco) {
			lista = append(lista, sugestaoDeCompra{d.ativo.Id, x.debito.id, preco, *fechamento, desvio})
		}
	}
	return lista, nil
}

// cotasDiferentes é o CotasDiferentesAsync: cotas na corretora × na carteira. Quando faltam cotas, sugere o débito de compra
// sem operação cujo valor (sem o troco), dividido pelas cotas que faltam, dá um preço perto do fechamento daquele dia.
func (s *Servico) cotasDiferentes(ctx context.Context, ativos []Ativo, investimentos []investimento, semOperacao []compraSemOperacao) ([]CotasDiferentesDto, error) {
	var diferencas []cotaDiferente
	for _, cota := range cotasNaCorretora(investimentos) {
		i := slices.IndexFunc(ativos, func(a Ativo) bool { return a.Ticker == cota.ticker })
		if i < 0 {
			continue // a importação cuida
		}
		p, err := Calcular(ativos[i].Operacoes, nil)
		if err != nil {
			return nil, err
		}
		if !menorQue(valorAbsoluto(tipos.Subtrair(cota.quantidade, p.Quantidade)), diferencaDeCotas) {
			diferencas = append(diferencas, cotaDiferente{&ativos[i], cota.quantidade, p.Quantidade, cota.em})
		}
	}
	var c conta
	var candidatos []sugestaoDeCompra
	for _, d := range diferencas {
		if !maiorQue(d.naCorretora, d.naCarteira) {
			continue
		}
		lista, err := s.candidatosDeCompra(ctx, d, semOperacao, &c)
		if err != nil {
			return nil, err
		}
		candidatos = append(candidatos, lista...)
	}
	slices.SortStableFunc(candidatos, func(a, b sugestaoDeCompra) int { return a.desvio.Decimal().Cmp(b.desvio.Decimal()) })
	sugestoes := map[int64]sugestaoDeCompra{}
	debitosSugeridos := map[int64]bool{}
	for _, x := range candidatos {
		if _, tem := sugestoes[x.ativoId]; tem || debitosSugeridos[x.debitoId] {
			continue
		}
		debitosSugeridos[x.debitoId] = true
		sugestoes[x.ativoId] = x
	}
	slices.SortStableFunc(diferencas, func(a, b cotaDiferente) int { return texto.CompararNaCultura(a.ativo.Ticker, b.ativo.Ticker) })
	lista := []CotasDiferentesDto{}
	for _, d := range diferencas {
		item := CotasDiferentesDto{AtivoId: d.ativo.Id, Ticker: d.ativo.Ticker, NaCorretora: d.naCorretora, NaCarteira: d.naCarteira, PosicaoEm: d.em}
		if x, tem := sugestoes[d.ativo.Id]; tem {
			item.DebitoSugeridoId, item.PrecoSugerido, item.FechamentoNoDia = &x.debitoId, &x.preco, &x.fechamento
		}
		lista = append(lista, item)
	}
	return lista, c.err
}

// rendimentosComoInvestimento são os proventos contados como investimento: crédito com a mesma descrição dos trocos, que não
// é troco nem venda, numa categoria de transferência. Voltam para a categoria das regras, se ela for de receita.
func (s *Servico) rendimentosComoInvestimento(ctx context.Context, a *analise) ([]RendimentoComoInvestimentoDto, error) {
	lista := []RendimentoComoInvestimentoDto{}
	deOperacao := idsDeOperacao(a)
	descricoes := map[string]bool{}
	for _, c := range a.deOperacao {
		descricoes[texto.ParaRegras(c.credito.descricao)] = true
	}
	var candidatos []movimento
	for _, c := range a.creditos {
		if !deOperacao[c.id] && c.tipo == tipoTransacaoTransferencia && descricoes[texto.ParaRegras(c.descricao)] {
			candidatos = append(candidatos, c)
		}
	}
	if len(candidatos) == 0 {
		return lista, nil
	}
	rs, err := categorizacao.Carregar(ctx, s.db)
	if err != nil {
		return nil, err
	}
	categorias, err := s.lerCategorias(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(candidatos, porDataEValor)
	for _, c := range candidatos {
		descricao := c.descricao
		if c.original != nil {
			descricao = *c.original
		}
		if id := categoriaDeRendimento(rs, descricao, c.pluggyCategoriaId, categorias); id != nil {
			lista = append(lista, RendimentoComoInvestimentoDto{c.id, c.data, c.valor, c.descricao, c.conta, c.categoria, categorias[*id].nome})
		}
	}
	return lista, nil
}

func porDataEValor(a, b movimento) int {
	return cmp.Or(cmp.Compare(a.data.NumeroDoDia(), b.data.NumeroDoDia()), a.valor.Decimal().Cmp(b.valor.Decimal()))
}

// Conferir é o ConferirAsync.
func (s *Servico) Conferir(ctx context.Context) (*ConferenciaInvestimentosDto, error) {
	ativos, err := s.lerAtivos(ctx, "")
	if err != nil {
		return nil, err
	}
	investimentos, err := s.lerRendaVariavel(ctx)
	if err != nil {
		return nil, err
	}
	a, err := s.analisarExtrato(ctx, ativos)
	if err != nil {
		return nil, err
	}
	cotas, err := s.cotasDiferentes(ctx, ativos, investimentos, a.semOperacao)
	if err != nil {
		return nil, err
	}
	categoria, err := s.categoriaDoTroco(ctx)
	if err != nil {
		return nil, err
	}
	rendimentos, err := s.rendimentosComoInvestimento(ctx, a)
	if err != nil {
		return nil, err
	}
	d := &ConferenciaInvestimentosDto{Tipos: tiposParaCorrigir(ativos, investimentos), Cotas: cotas, ComprasSemOperacao: []CompraSemOperacaoDto{},
		Trocos: []CreditoDaCorretoraDto{}, Rendimentos: rendimentos, CategoriaDoTroco: categoria, Taxas: []TaxaB3Dto{}}

	deOperacao := slices.Clone(a.deOperacao)
	slices.SortStableFunc(deOperacao, func(x, y creditoDeOperacao) int { return porDataEValor(x.credito, y.credito) })
	for _, c := range deOperacao {
		if c.credito.tipo == tipoTransacaoTransferencia {
			continue
		}
		item := CreditoDaCorretoraDto{TransacaoId: c.credito.id, Data: c.credito.data, Valor: c.credito.valor, Descricao: c.credito.descricao, Conta: c.credito.conta, Categoria: c.credito.categoria}
		if c.compra != nil {
			item.CompraData, item.CompraValor = &c.compra.data, &c.compra.valor
		}
		if c.venda != nil {
			item.VendaTicker, item.VendaQuantidade, item.VendaData = &c.vendaTicker, &c.venda.Quantidade, &c.venda.Data
		}
		d.Trocos = append(d.Trocos, item)
	}
	semOperacao := slices.Clone(a.semOperacao)
	slices.SortStableFunc(semOperacao, func(x, y compraSemOperacao) int { return porDataEValor(x.debito, y.debito) })
	for _, c := range semOperacao {
		troco := tipos.Zero
		if c.troco != nil {
			troco = c.troco.valor
		}
		d.ComprasSemOperacao = append(d.ComprasSemOperacao, CompraSemOperacaoDto{c.debito.id, c.debito.data, c.debito.valor, troco, c.debito.conta, c.debito.descricao})
	}
	for _, t := range a.taxas {
		item := TaxaB3Dto{t.data, t.conta, t.debitado, t.trocos, t.valorDasCotas, t.taxa, []TaxaB3OperacaoDto{}}
		for _, o := range t.operacoes {
			item.Operacoes = append(item.Operacoes, TaxaB3OperacaoDto{o.operacao.Id, o.ticker, o.operacao.Quantidade, o.taxa})
		}
		d.Taxas = append(d.Taxas, item)
	}
	return d, nil
}

// trocoAutomatico é o troco classificado na sincronização, com o que ele era antes (para o "Desfazer"). Os nomes são os do
// JSON que o C# grava na Configuracao.
type trocoAutomatico struct {
	Id                int64  `json:"Id"`
	CategoriaId       *int64 `json:"CategoriaId"`
	CategoriaManual   bool   `json:"CategoriaManual"`
	Tipo              int64  `json:"Tipo"`
	IgnorarRelatorios bool   `json:"IgnorarRelatorios"`
	Aplicada          int64  `json:"Aplicada"`
}

func (s *Servico) trocosGuardados(ctx context.Context) ([]trocoAutomatico, error) {
	valor, err := s.config.Obter(ctx, ChaveTrocosAutomaticos)
	if err != nil || emBranco(valor) {
		return nil, err
	}
	var lista []trocoAutomatico
	if json.Unmarshal([]byte(*valor), &lista) != nil {
		return nil, nil
	}
	return lista, nil
}
