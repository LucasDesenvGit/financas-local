// Pacote faturas é a fatura do cartão do ContaService do C#: o ciclo de cada fatura (#144), o mês em que cada
// lançamento é cobrado (#167), o que foi pago (#251) e a situação, e a fatura do mês de cada cartão, que o Painel e
// Contas e cartões mostram (#323).
//
// As regras de data e de pagamento foram conferidas contra o próprio C#: a sonda testdata/sonda-csharp grava o
// testdata/faturas.json com os resultados do ContaService para centenas de combinações de mês, dia de fechamento, dia
// de vencimento e faturas importadas.
package faturas

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/bancos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/parcelas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/texto"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Situações da fatura, no texto que o front-end recebe.
const (
	Aberta  = "aberta"
	Fechada = "fechada"
	Paga    = "paga"
)

// Fatura é a fatura importada do banco, só com o que o ciclo precisa (o FaturaDoCiclo do C#).
type Fatura struct {
	Id             int64
	ContaId        int64
	DataFechamento *tipos.Data
	DataVencimento tipos.Data
	ValorTotal     tipos.Decimal
	ValorPago      *tipos.Decimal
}

// Ciclo é o período das compras de uma fatura (Inicio a Fim, inclusive) e a fatura desse ciclo, se já veio do banco.
type Ciclo struct {
	Inicio, Fim            tipos.Data
	Fechamento, Vencimento *tipos.Data
	Fatura                 *Fatura
}

// Lancamento é um lançamento do cartão, com o que é preciso para saber em que fatura ele é cobrado.
type Lancamento struct {
	Id             int64
	ContaId        int64
	Data           tipos.Data
	Valor          tipos.Decimal
	FaturaCartaoId *int64
	ParcelaNumero  *int64
	ParcelaTotal   *int64
	Descricao      string // DescricaoOriginal ?? Descricao
}

// Parcelado: compra em mais de uma parcela.
func (l Lancamento) Parcelado() bool { return l.ParcelaTotal != nil && *l.ParcelaTotal > 1 }

// Pagamento é a entrada lançada no cartão que paga a fatura.
type Pagamento struct {
	ContaId int64
	Data    tipos.Data
	Valor   tipos.Decimal
}

func primeiroDia(d tipos.Data) tipos.Data { return tipos.NovaData(d.Ano(), d.Mes(), 1) }

// diaNoMes é o dia no mês da referência, limitado ao último dia dele (dia 31 em fevereiro é 28 ou 29).
func diaNoMes(referencia tipos.Data, dia int) tipos.Data {
	ultimo := time.Date(referencia.Ano(), referencia.Mes()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return tipos.NovaData(referencia.Ano(), referencia.Mes(), min(dia, ultimo))
}

// fechamentosImportados: as datas de fechamento das faturas importadas, sem repetir, da mais velha para a mais nova.
func fechamentosImportados(faturas []Fatura) []tipos.Data {
	var datas []tipos.Data
	for _, f := range faturas {
		if f.DataFechamento != nil && !slices.ContainsFunc(datas, f.DataFechamento.Igual) {
			datas = append(datas, *f.DataFechamento)
		}
	}
	slices.SortFunc(datas, comparar)
	return datas
}

func comparar(a, b tipos.Data) int { return a.DiasDesde(b) }

// diaDeFechamento é o informado no cartão ou, sem ele, o dia da última fatura importada; nil quando nenhum dos dois.
func diaDeFechamento(informado *int64, fechamentos []tipos.Data) *int {
	switch {
	case informado != nil:
		d := int(*informado)
		return &d
	case len(fechamentos) > 0:
		d := fechamentos[len(fechamentos)-1].Dia()
		return &d
	}
	return nil
}

// CicloDaFatura é o ciclo da fatura que fecha no mês de fim (#144). O fechamento é o da fatura importada desse mês ou,
// ainda sem ela, o dia de fechamento do cartão (o informado ou o da última fatura); o banco mexe um ou dois dias por
// causa de fim de semana, por isso o fechamento anterior vem da fatura importada quando há (até 45 dias antes). Sem dia
// de fechamento conhecido, o ciclo é o mês do calendário, com a fatura que vence nele como referência.
func CicloDaFatura(inicio, fim tipos.Data, diaFechamento, diaVencimento *int64, faturas []Fatura) Ciclo {
	fechamentos := fechamentosImportados(faturas)
	dia := diaDeFechamento(diaFechamento, fechamentos)
	if dia == nil {
		// MinBy: o primeiro de menor vencimento, na ordem das faturas.
		var vencendo *Fatura
		for i := range faturas {
			f := &faturas[i]
			if !f.DataVencimento.Antes(inicio) && !f.DataVencimento.Depois(fim) &&
				(vencendo == nil || f.DataVencimento.Antes(vencendo.DataVencimento)) {
				vencendo = f
			}
		}
		c := Ciclo{Inicio: inicio, Fim: fim, Fatura: vencendo}
		if vencendo != nil {
			c.Vencimento = &vencendo.DataVencimento
		}
		return c
	}

	fechamento := diaNoMes(fim, *dia)
	for _, d := range fechamentos { // o último do mês, se houver
		if !d.Antes(inicio) && !d.Depois(fim) {
			fechamento = d
		}
	}
	anterior := diaNoMes(fechamento.SomarMeses(-1), *dia)
	for _, d := range fechamentos {
		if d.Antes(fechamento) && !d.Antes(fechamento.SomarDias(-45)) {
			anterior = d
		}
	}
	// MaxBy: o primeiro de maior vencimento, na ordem das faturas.
	var fatura *Fatura
	for i := range faturas {
		f := &faturas[i]
		var noCiclo bool
		if f.DataFechamento != nil {
			noCiclo = f.DataFechamento.Depois(anterior) && !f.DataFechamento.Depois(fechamento)
		} else {
			noCiclo = f.DataVencimento.Depois(fechamento) && !f.DataVencimento.Depois(fechamento.SomarDias(25))
		}
		if noCiclo && (fatura == nil || f.DataVencimento.Depois(fatura.DataVencimento)) {
			fatura = f
		}
	}

	var vencimento *tipos.Data
	if fatura != nil {
		vencimento = &fatura.DataVencimento
	}
	diaVenc := diaDoVencimento(diaVencimento, faturas)
	if vencimento == nil && diaVenc != nil {
		v := diaNoMes(fechamento, *diaVenc)
		if !v.Depois(fechamento) {
			v = diaNoMes(fechamento.SomarMeses(1), *diaVenc)
		}
		vencimento = &v
	}
	return Ciclo{Inicio: anterior.SomarDias(1), Fim: fechamento, Fechamento: &fechamento, Vencimento: vencimento, Fatura: fatura}
}

// diaDoVencimento é o informado no cartão ou, sem ele, o da fatura de vencimento mais novo.
func diaDoVencimento(informado *int64, faturas []Fatura) *int {
	if informado != nil {
		d := int(*informado)
		return &d
	}
	var ultima *Fatura
	for i := range faturas {
		if ultima == nil || faturas[i].DataVencimento.Depois(ultima.DataVencimento) {
			ultima = &faturas[i]
		}
	}
	if ultima == nil {
		return nil
	}
	d := ultima.DataVencimento.Dia()
	return &d
}

// MesesDeCobranca é o mês de fechamento (dia 1) da fatura em que cada lançamento de um cartão é cobrado (#167):
//  1. ligado pelo banco a uma fatura importada (FaturaCartaoId): o mês do fechamento dela (sem fechamento, o do
//     vencimento menos 10 dias);
//  2. parcela de uma compra que tem outra parcela já ligada a uma fatura: o mês dessa fatura mais a diferença de número
//     (a mesma compra: total de parcelas, valor em reais inteiros e parcelas.Compra da descrição);
//  3. o resto, pela data: o primeiro fechamento a partir dela (o de uma fatura importada até 40 dias depois ou, sem
//     ela, o dia de fechamento do cartão). Sem dia de fechamento conhecido, o mês do calendário.
func MesesDeCobranca(lancamentos []Lancamento, faturas []Fatura, diaFechamento *int64) map[int64]tipos.Data {
	mesDaFatura := make(map[int64]tipos.Data, len(faturas))
	for _, f := range faturas {
		if f.DataFechamento != nil {
			mesDaFatura[f.Id] = primeiroDia(*f.DataFechamento)
		} else {
			mesDaFatura[f.Id] = primeiroDia(f.DataVencimento.SomarDias(-10))
		}
	}
	fechamentos := fechamentosImportados(faturas)
	dia := diaDeFechamento(diaFechamento, fechamentos)
	pelaData := func(data tipos.Data) tipos.Data {
		for _, f := range fechamentos {
			if !f.Antes(data) && f.DiasDesde(data) <= 40 {
				return primeiroDia(f)
			}
		}
		if dia == nil {
			return primeiroDia(data)
		}
		noMes := diaNoMes(data, *dia)
		if data.Depois(noMes) {
			noMes = diaNoMes(data.SomarMeses(1), *dia)
		}
		return primeiroDia(noMes)
	}

	meses := make(map[int64]tipos.Data, len(lancamentos))
	for _, t := range lancamentos {
		if t.FaturaCartaoId != nil {
			if mes, ok := mesDaFatura[*t.FaturaCartaoId]; ok {
				meses[t.Id] = mes
			}
		}
	}
	type chave struct {
		total  int64
		reais  string
		compra string
	}
	var ordem []chave
	compras := map[chave][]Lancamento{}
	for _, t := range lancamentos {
		if !t.Parcelado() || t.ParcelaNumero == nil {
			continue
		}
		// Math.Round(Math.Abs(valor), 0): o arredondamento bancário; a chave compara o valor, não a escala.
		reais := tipos.Arredondar(tipos.DeDecimal(t.Valor.Decimal().Abs()), 0).Decimal().String()
		k := chave{*t.ParcelaTotal, reais, parcelas.Compra(t.Descricao)}
		if _, ok := compras[k]; !ok {
			ordem = append(ordem, k)
		}
		compras[k] = append(compras[k], t)
	}
	for _, k := range ordem {
		// MaxBy: a primeira de maior número entre as já ligadas a uma fatura.
		var ancora *Lancamento
		for i, t := range compras[k] {
			if _, ok := meses[t.Id]; ok && (ancora == nil || *t.ParcelaNumero > *ancora.ParcelaNumero) {
				ancora = &compras[k][i]
			}
		}
		if ancora == nil {
			continue
		}
		for _, t := range compras[k] {
			if _, ok := meses[t.Id]; !ok {
				meses[t.Id] = meses[ancora.Id].SomarMeses(int(*t.ParcelaNumero - *ancora.ParcelaNumero))
			}
		}
	}
	for _, t := range lancamentos {
		if _, ok := meses[t.Id]; !ok {
			meses[t.Id] = pelaData(t.Data)
		}
	}
	return meses
}

// PagoNasTransacoes é quanto da fatura que vence em vencimento foi pago pelos lançamentos do cartão (#251): os
// pagamentos depois do vencimento anterior e até este. O vencimento anterior é o da fatura importada até 45 dias antes
// ou, sem ela, um mês antes. Pagamento atrasado (depois do vencimento) conta para a fatura seguinte.
func PagoNasTransacoes(vencimento tipos.Data, vencimentosImportados []tipos.Data, pagamentos []Pagamento) tipos.Decimal {
	anterior, achou := tipos.Data{}, false
	for _, v := range vencimentosImportados {
		if v.Antes(vencimento) && !v.Antes(vencimento.SomarDias(-45)) && (!achou || v.Depois(anterior)) {
			anterior, achou = v, true
		}
	}
	if !achou {
		anterior = vencimento.SomarMeses(-1)
	}
	soma := tipos.Zero
	for _, p := range pagamentos {
		if p.Data.Depois(anterior) && !p.Data.Depois(vencimento) {
			soma = tipos.Somar(soma, p.Valor)
		}
	}
	return soma
}

// Pago é o pago que vale (#251): o maior entre o que o banco informou e o lançado no cartão; nil quando nenhum dos dois.
func Pago(informado *tipos.Decimal, lancado tipos.Decimal) *tipos.Decimal {
	if informado == nil && !lancado.Decimal().IsPositive() {
		return nil
	}
	base := tipos.Zero
	if informado != nil {
		base = tipos.DeDecimal(informado.Decimal().Abs())
	}
	maior := tipos.Maior(base, lancado)
	return &maior
}

// centavo é a folga do "paga": um centavo a menos ainda cobre o total.
var centavo, _ = tipos.NovoDecimal("0.01")

// Situacao é a do C#: paga quando os pagamentos cobrem o total (com um centavo de folga); fechada quando já fechou (ou
// venceu) sem pagamento que cubra; aberta enquanto ainda recebe compras. A instituição nem sempre informa o pagamento,
// por isso a fatura vencida sem pagamento fica "fechada", e não "atrasada" (evita alarme falso).
func Situacao(total tipos.Decimal, pago *tipos.Decimal, fechamento *tipos.Data, vencimento, hoje tipos.Data) string {
	if !total.Decimal().IsPositive() ||
		(pago != nil && pago.Decimal().Abs().Cmp(tipos.Subtrair(total, centavo).Decimal()) >= 0) {
		return Paga
	}
	if (fechamento != nil && !fechamento.Depois(hoje)) || vencimento.Antes(hoje) {
		return Fechada
	}
	return Aberta
}

// DoMes é o FaturaMesDto do C#: a fatura de um cartão que fecha no mês escolhido.
type DoMes struct {
	ContaId     int64         `json:"contaId"`
	Cartao      string        `json:"cartao"`
	ValorMes    tipos.Decimal `json:"valorMes"`
	ParcelasMes tipos.Decimal `json:"parcelasMes"`
	FaturaId    *int64        `json:"faturaId"`
	Vencimento  *tipos.Data   `json:"vencimento"`
	Situacao    *string       `json:"situacao"`
	Banco       *string       `json:"banco"`
	Fechamento  *tipos.Data   `json:"fechamento"`
	Proxima     *DoMes        `json:"proxima"`
}

type cartao struct {
	id                           int64
	nome                         string
	diaFechamento, diaVencimento *int64
}

// DoMesDosCartoes é o FaturasDoMesAsync do C#: a fatura de cada cartão (não arquivado) que fecha no mês de inicio a
// fim. O ciclo vai do fechamento anterior até o deste mês (#144). Fatura já fechada e importada do banco vale o total
// que ele informou; fatura ainda aberta é a soma das transações cobradas nela (MesesDeCobranca, #167): compras, a
// parcela que cai nela e estornos descontados. Pagamento de fatura e transferências não entram; despesa marcada para
// ignorar nos relatórios entra (ela continua na conta do cartão). Quando a fatura do mês já fechou, vem junto a
// seguinte, ainda aberta (Proxima, #251). A ordem: o valor do mês, do maior para o menor, e o nome do cartão pela
// cultura pt-BR (o ThenBy do C#, em memória).
func DoMesDosCartoes(ctx context.Context, q consultas.DBTX, inicio, fim, hoje tipos.Data) ([]DoMes, error) {
	cartoes, err := lerCartoes(ctx, q)
	if err != nil || len(cartoes) == 0 {
		return []DoMes{}, err
	}
	ids := make([]any, len(cartoes))
	for i, c := range cartoes {
		ids[i] = c.id
	}
	faturas, err := LerFaturas(ctx, q, `WHERE "ContaId" IN (`+marcadores(len(ids))+`)`, ids...)
	if err != nil {
		return nil, err
	}
	ciclos := make(map[int64]Ciclo, len(cartoes))
	desde := inicio
	for i, c := range cartoes {
		ciclos[c.id] = CicloDaFatura(inicio, fim, c.diaFechamento, c.diaVencimento, faturas[c.id])
		if i == 0 || ciclos[c.id].Inicio.Antes(desde) {
			desde = ciclos[c.id].Inicio
		}
	}
	// #167: uma janela larga, para achar a parcela já ligada a uma fatura de cada compra (até 24 meses) e as parcelas
	// com data futura.
	desde = desde.SomarMeses(-24)
	lancamentos, err := lerLancamentos(ctx, q, ids, desde)
	if err != nil {
		return nil, err
	}
	pagamentos, err := LerPagamentos(ctx, q, ids, desde)
	if err != nil {
		return nil, err
	}
	bancosDasContas, err := bancos.Carregar(ctx, q)
	if err != nil {
		return nil, err
	}

	doCiclo := func(c cartao, ciclo Ciclo, meses map[int64]tipos.Data, proxima *DoMes) DoMes {
		mesDoCiclo := ciclo.Fim
		if ciclo.Fechamento != nil {
			mesDoCiclo = *ciclo.Fechamento
		}
		mesDoCiclo = primeiroDia(mesDoCiclo)
		soma, somaParcelas := tipos.Zero, tipos.Zero
		for _, t := range lancamentos[c.id] {
			if meses[t.Id].Igual(mesDoCiclo) {
				soma = tipos.Somar(soma, t.Valor)
				if t.Parcelado() {
					somaParcelas = tipos.Somar(somaParcelas, t.Valor)
				}
			}
		}
		lancado := tipos.Maior(tipos.Zero, negativo(soma))
		valorParcelas := tipos.Maior(tipos.Zero, negativo(somaParcelas))
		fatura := ciclo.Fatura
		totalInformado := tipos.Zero
		if fatura != nil {
			totalInformado = tipos.DeDecimal(fatura.ValorTotal.Decimal().Abs())
		}
		// Fatura fechada vale o total do banco; sem ele (aberta, ou o banco mandou zero), a soma do ciclo.
		valor := lancado
		if ciclo.Fechamento != nil && totalInformado.Decimal().IsPositive() {
			valor = totalInformado
		}
		var situacao *string
		switch {
		case ciclo.Vencimento == nil:
		case fatura == nil && ciclo.Fechamento != nil && ciclo.Fechamento.Depois(hoje):
			s := Aberta
			situacao = &s
		default:
			total := valor
			if ciclo.Fechamento == nil {
				total = totalInformado
			}
			var informado *tipos.Decimal
			fechamento := ciclo.Fechamento
			if fatura != nil {
				informado = fatura.ValorPago
				if fatura.DataFechamento != nil {
					fechamento = fatura.DataFechamento
				}
			}
			pago := Pago(informado, PagoNasTransacoes(*ciclo.Vencimento, Vencimentos(faturas[c.id]), pagamentos[c.id]))
			s := Situacao(total, pago, fechamento, *ciclo.Vencimento, hoje)
			situacao = &s
		}
		d := DoMes{ContaId: c.id, Cartao: c.nome, ValorMes: tipos.Arredondar(valor, 2),
			ParcelasMes: tipos.Arredondar(valorParcelas, 2), Vencimento: ciclo.Vencimento, Situacao: situacao,
			Banco: bancosDasContas[c.id], Fechamento: ciclo.Fechamento, Proxima: proxima}
		if fatura != nil {
			d.FaturaId = &fatura.Id
		}
		return d
	}

	lista := make([]DoMes, 0, len(cartoes))
	seguinteInicio := fim.SomarDias(1)
	seguinteFim := seguinteInicio.SomarMeses(1).SomarDias(-1)
	for _, c := range cartoes {
		ciclo := ciclos[c.id]
		meses := MesesDeCobranca(lancamentos[c.id], faturas[c.id], c.diaFechamento)
		// #251: fatura do mês já fechada → a seguinte, a que ainda recebe compras, vem junto.
		var proxima *DoMes
		if ciclo.Fechamento != nil && !ciclo.Fechamento.Depois(hoje) {
			seguinte := CicloDaFatura(seguinteInicio, seguinteFim, c.diaFechamento, c.diaVencimento, faturas[c.id])
			if seguinte.Fechamento != nil && seguinte.Fechamento.Depois(hoje) {
				p := doCiclo(c, seguinte, meses, nil)
				proxima = &p
			}
		}
		lista = append(lista, doCiclo(c, ciclo, meses, proxima))
	}
	slices.SortStableFunc(lista, func(a, b DoMes) int {
		if c := b.ValorMes.Decimal().Cmp(a.ValorMes.Decimal()); c != 0 {
			return c
		}
		return texto.CompararNaCultura(a.Cartao, b.Cartao)
	})
	return lista, nil
}

func negativo(d tipos.Decimal) tipos.Decimal { return tipos.DeDecimal(d.Decimal().Neg()) }

// Vencimentos: as datas de vencimento das faturas, na ordem delas.
func Vencimentos(faturas []Fatura) []tipos.Data {
	datas := make([]tipos.Data, len(faturas))
	for i, f := range faturas {
		datas[i] = f.DataVencimento
	}
	return datas
}

func marcadores(n int) string { return strings.TrimSuffix(strings.Repeat("?, ", n), ", ") }

// lerCartoes: os cartões não arquivados, na ordem do Id (a da tabela, que o C# lê sem ORDER BY).
func lerCartoes(ctx context.Context, q consultas.DBTX) ([]cartao, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "Nome", "DiaFechamento", "DiaVencimento" FROM "Conta"
		WHERE "Tipo" = 3 AND NOT "Arquivada" ORDER BY "Id"`)
	if err != nil {
		return nil, fmt.Errorf("ler os cartões: %w", err)
	}
	defer linhas.Close()
	var lista []cartao
	for linhas.Next() {
		var c cartao
		if err := linhas.Scan(&c.id, &c.nome, &c.diaFechamento, &c.diaVencimento); err != nil {
			return nil, fmt.Errorf("ler cartão: %w", err)
		}
		lista = append(lista, c)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os cartões: %w", err)
	}
	return lista, nil
}

// LerFaturas lê as faturas importadas que casam com o filtro, por cartão. A ordem é a do índice
// IX_FaturaCartao_Conta_Vencimento (ContaId, DataVencimento DESC), que a consulta do C# usa sem ORDER BY: ela decide o
// empate do MinBy e do MaxBy.
func LerFaturas(ctx context.Context, q consultas.DBTX, onde string, argumentos ...any) (map[int64][]Fatura, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "ContaId", "DataFechamento", "DataVencimento", "ValorTotal",
		"ValorPago" FROM "FaturaCartao" `+onde+` ORDER BY "ContaId", "DataVencimento" DESC, "Id"`, argumentos...)
	if err != nil {
		return nil, fmt.Errorf("ler as faturas: %w", err)
	}
	defer linhas.Close()
	porCartao := map[int64][]Fatura{}
	for linhas.Next() {
		var f Fatura
		if err := linhas.Scan(&f.Id, &f.ContaId, &f.DataFechamento, &f.DataVencimento, &f.ValorTotal, &f.ValorPago); err != nil {
			return nil, fmt.Errorf("ler fatura: %w", err)
		}
		porCartao[f.ContaId] = append(porCartao[f.ContaId], f)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler as faturas: %w", err)
	}
	return porCartao, nil
}

// lerLancamentos: os lançamentos dos cartões desde a data, sem as transferências e os pagamentos de fatura, na ordem do
// índice IX_Transacao_Conta_Data (ContaId, Data DESC), a da consulta do C#.
func lerLancamentos(ctx context.Context, q consultas.DBTX, ids []any, desde tipos.Data) (map[int64][]Lancamento, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "Id", "ContaId", "Data", "Valor", "FaturaCartaoId", "ParcelaNumero",
		"ParcelaTotal", COALESCE("DescricaoOriginal", "Descricao") FROM "Transacao"
		WHERE "ContaId" IN (`+marcadores(len(ids))+`) AND "Data" >= ? AND "Tipo" <> 3
		AND ("MeioPagamento" IS NULL OR "MeioPagamento" <> 'PAGAMENTO_FATURA')
		ORDER BY "ContaId", "Data" DESC, "Id"`, append(slices.Clone(ids), desde)...)
	if err != nil {
		return nil, fmt.Errorf("ler os lançamentos dos cartões: %w", err)
	}
	defer linhas.Close()
	porCartao := map[int64][]Lancamento{}
	for linhas.Next() {
		var t Lancamento
		if err := linhas.Scan(&t.Id, &t.ContaId, &t.Data, &t.Valor, &t.FaturaCartaoId, &t.ParcelaNumero, &t.ParcelaTotal,
			&t.Descricao); err != nil {
			return nil, fmt.Errorf("ler lançamento do cartão: %w", err)
		}
		porCartao[t.ContaId] = append(porCartao[t.ContaId], t)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os lançamentos dos cartões: %w", err)
	}
	return porCartao, nil
}

// LerPagamentos são as entradas nos cartões, desde a data, que pagam a fatura (#251): a que virou transferência
// (pagamento pelo texto, pela categoria "Pagamento de fatura" ou transferência sua para o cartão; ADR 0036) ou a que o
// banco mandou como PAGAMENTO_FATURA. Estorno é despesa com valor positivo e fica de fora. O valor positivo é
// conferido aqui, e não no SQL: dinheiro é texto no banco.
func LerPagamentos(ctx context.Context, q consultas.DBTX, ids []any, desde tipos.Data) (map[int64][]Pagamento, error) {
	linhas, err := q.QueryContext(ctx, `SELECT "ContaId", "Data", "Valor" FROM "Transacao"
		WHERE "ContaId" IN (`+marcadores(len(ids))+`) AND "Data" >= ?
		AND ("Tipo" = 3 OR "MeioPagamento" = 'PAGAMENTO_FATURA')`, append(slices.Clone(ids), desde)...)
	if err != nil {
		return nil, fmt.Errorf("ler os pagamentos nos cartões: %w", err)
	}
	defer linhas.Close()
	porCartao := map[int64][]Pagamento{}
	for linhas.Next() {
		var p Pagamento
		if err := linhas.Scan(&p.ContaId, &p.Data, &p.Valor); err != nil {
			return nil, fmt.Errorf("ler pagamento no cartão: %w", err)
		}
		if p.Valor.Decimal().IsPositive() {
			porCartao[p.ContaId] = append(porCartao[p.ContaId], p)
		}
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler os pagamentos nos cartões: %w", err)
	}
	return porCartao, nil
}
