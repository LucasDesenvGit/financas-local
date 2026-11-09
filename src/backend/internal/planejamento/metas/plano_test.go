package metas

import (
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/movimentacao/faturas"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// #675: os casos do plano que a mutação mostrou sem teste.

func mesmoValor(t *testing.T, nome string, obtido tipos.Decimal, quer string) {
	t.Helper()
	if !obtido.Decimal().Equal(dec(quer).Decimal()) {
		t.Errorf("%s: %s, quer %s", nome, obtido, quer)
	}
}

// Regra (#149): com exatamente 3 meses completos o plano sai; o histórico vai até o último dia do mês passado
// (inclusive) e não pega o mês atual; só as contas de dinheiro no saldo entram; o melhor mês é o de maior sobra, onde
// quer que esteja.
func TestPlanejarComTresMeses(t *testing.T) {
	t.Parallel()
	_, fab, _, s := ambiente(t, testeapoio.HojeEmBrasilia(2026, 10, 14))
	corrente := fab.Conta()
	fora := fab.Conta(func(c *testeapoio.Conta) { c.Tipo, c.IncluirNoSaldo = 1, false })
	transacao := func(conta testeapoio.Conta, dia tipos.Data, valor string, tipo int64) {
		fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor, tr.Tipo = dia, dec(valor), tipo })
	}
	transacao(corrente, testeapoio.Dia(2026, 7, 1), "4000.0", 2) // julho sobra 1.000
	transacao(corrente, testeapoio.Dia(2026, 7, 2), "-3000.0", 1)
	transacao(corrente, testeapoio.Dia(2026, 8, 1), "5000.0", 2) // agosto sobra 3.000: o melhor
	transacao(corrente, testeapoio.Dia(2026, 8, 2), "-2000.0", 1)
	transacao(corrente, testeapoio.Dia(2026, 9, 1), "3000.0", 2) // setembro sobra 2.000, com o dia 30
	transacao(corrente, testeapoio.Dia(2026, 9, 30), "1000.0", 2)
	transacao(corrente, testeapoio.Dia(2026, 9, 2), "-2000.0", 1)
	transacao(corrente, testeapoio.Dia(2026, 10, 1), "9000.0", 2) // mês atual: fora do histórico
	transacao(fora, testeapoio.Dia(2026, 8, 3), "100000.0", 2)    // fora do saldo: não conta
	p, err := s.Planejar(t.Context())
	if err != nil || !p.Disponivel || p.MesesConsiderados != 3 {
		t.Fatalf("plano: %+v %v", p, err)
	}
	// Receitas: 13.000 ÷ 3; despesas: 7.000 ÷ 3.
	mesmoValor(t, "receitas depois", p.Meses[1].Receitas, "4333.33")
	mesmoValor(t, "despesas depois", p.Meses[1].Despesas, "2333.33")
	mesmoValor(t, "melhor mês", p.MelhorMesRecente, "3000")
}

// cartaoDoPlano monta o plano com um cartão que fecha no dia 5 e vence no 15, com compras à vista de R$ 304,40 por
// mês (R$ 10 por dia: 913,20 ÷ 3 ÷ 30,44) em 02/07, 20/08 e 20/09, e receitas de 5.100 em julho e 5.000 em agosto e
// setembro. Com importada, a fatura de outubro (fechada em 05/10) veio do banco com R$ 500; com pago, um pagamento de
// R$ 200 foi lançado no cartão em 10/10.
func cartaoDoPlano(t *testing.T, hoje tipos.Data, importada, pago bool) PlanoMetasDto {
	t.Helper()
	_, fab, _, s := ambiente(t, testeapoio.HojeEmBrasilia(hoje.Ano(), hoje.Mes(), hoje.Dia()))
	corrente := fab.Conta()
	for m, valor := range map[time.Month]string{time.July: "5100.0", time.August: "5000.0", time.September: "5000.0"} {
		fab.Transacao(corrente, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Tipo = testeapoio.Dia(2026, m, 1), dec(valor), 2
		})
	}
	cartao := fab.Conta(func(c *testeapoio.Conta) {
		c.Tipo, c.DiaFechamento, c.DiaVencimento = tipoCartao, ptr(int64(5)), ptr(int64(15))
	})
	for _, dia := range []tipos.Data{testeapoio.Dia(2026, 7, 2), testeapoio.Dia(2026, 8, 20), testeapoio.Dia(2026, 9, 20)} {
		fab.Transacao(cartao, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor = dia, dec("-304.4") })
	}
	if importada {
		fab.Fatura(cartao, testeapoio.Dia(2026, 10, 15), dec("-500.0"), func(f *testeapoio.FaturaCartao) {
			f.DataFechamento = ptr(testeapoio.Dia(2026, 10, 5))
		})
	}
	if pago {
		fab.Transacao(cartao, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Tipo = testeapoio.Dia(2026, 10, 10), dec("200.0"), 3
		})
	}
	p, err := s.Planejar(t.Context())
	if err != nil || !p.Disponivel {
		t.Fatalf("plano: %+v %v", p, err)
	}
	return p
}

// Regra (#144): a fatura aberta é o lançado mais R$ 10 por dia que falta até o fechamento, a partir de amanhã (ou do
// início do ciclo, se ele ainda não começou). Hoje, 06/10, é o primeiro dia do ciclo de novembro: faltam 30 dias (07/10
// a 05/11); o de dezembro tem os 30 dias dele (06/11 a 05/12).
func TestPlanejarFaturaAberta(t *testing.T) {
	t.Parallel()
	p := cartaoDoPlano(t, testeapoio.Dia(2026, 10, 6), false, false)
	mesmoValor(t, "fatura de novembro", p.Meses[1].Faturas, "300")
	mesmoValor(t, "fatura de dezembro", p.Meses[2].Faturas, "300")
	// Julho: 5.100 − a fatura de 15/07 (a compra de 02/07) = 4.795,60; agosto: 5.000, o melhor.
	mesmoValor(t, "melhor mês", p.MelhorMesRecente, "5000")
}

// Regra (#144, #251): a fatura que fecha hoje já vale o total do banco; a que vence hoje ainda está a pagar; o
// pagamento lançado no cartão nos últimos meses desconta do que falta pagar.
func TestPlanejarFaturaAPagar(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome string
		hoje tipos.Data
		pago bool
		quer string
	}{
		{"fecha hoje", testeapoio.Dia(2026, 10, 5), false, "500"},
		{"vence hoje", testeapoio.Dia(2026, 10, 15), false, "500"},
		{"com pagamento no cartão", testeapoio.Dia(2026, 10, 14), true, "300"},
	}
	for _, c := range casos {
		p := cartaoDoPlano(t, c.hoje, true, c.pago)
		mesmoValor(t, c.nome, p.FaturaAPagar, c.quer)
	}
}

// Regra (#144): a média diária de compras à vista do cartão conta do primeiro ao último dia do histórico (inclusive),
// sem as parceladas.
func TestCompraAVistaPorDia(t *testing.T) {
	t.Parallel()
	inicio, fim := testeapoio.Dia(2026, 7, 1), testeapoio.Dia(2026, 9, 30)
	tres := int64(3)
	lista := []faturas.Lancamento{
		{Data: inicio, Valor: dec("-100.0")},
		{Data: fim, Valor: dec("-100.0")},
		{Data: testeapoio.Dia(2026, 6, 30), Valor: dec("-1000.0")},
		{Data: testeapoio.Dia(2026, 10, 1), Valor: dec("-1000.0")},
		{Data: testeapoio.Dia(2026, 8, 10), Valor: dec("-1000.0"), ParcelaTotal: &tres},
	}
	obtido, err := compraAVistaPorDia(lista, inicio, fim, tipos.DecimalDeInteiro(3))
	if err != nil {
		t.Fatal(err)
	}
	media, _ := tipos.Dividir(dec("200"), tipos.DecimalDeInteiro(3))
	quer, _ := tipos.Dividir(media, diasPorMes)
	if !obtido.Decimal().Equal(quer.Decimal()) {
		t.Errorf("por dia: %s, quer %s", obtido, quer)
	}
}

// Regra (#165): só as parcelas recentes (dos últimos 45 dias, inclusive o primeiro), negativas, de compra parcelada e
// com o número da parcela são projetadas.
func TestProjetarParcelas(t *testing.T) {
	t.Parallel()
	hoje := testeapoio.Dia(2026, 10, 14)
	desde := hoje.SomarDias(-45)
	um, tres := int64(1), int64(3)
	lancamento := func(id int64, dia tipos.Data, valor string, numero, total *int64, descricao string) faturas.Lancamento {
		return faturas.Lancamento{Id: id, ContaId: 9, Data: dia, Valor: dec(valor), ParcelaNumero: numero, ParcelaTotal: total,
			Descricao: descricao}
	}
	lancamentos := map[int64][]faturas.Lancamento{9: {
		lancamento(1, desde, "-100.0", &um, &tres, "VALIDA"),
		lancamento(2, desde.SomarDias(-1), "-100.0", &um, &tres, "ANTES"),
		lancamento(3, desde, "100.0", &um, &tres, "POSITIVA"),
		lancamento(4, desde, "-100.0", &um, nil, "SEM TOTAL"),
		lancamento(5, desde, "-100.0", nil, &tres, "SEM NUMERO"),
	}}
	futuras := projetarParcelas(lancamentos, []contaDoPlano{{id: 9, tipo: tipoCartao}}, hoje)
	// Da válida (1ª em 30/08), a 2ª já passou: sobra a 3ª, em 30/10.
	if len(futuras) != 1 {
		t.Fatalf("futuras: %+v (quer só a 3ª da válida)", futuras)
	}
	for _, f := range futuras {
		if f.OrigemId != 1 {
			t.Errorf("futura de %d: %+v", f.OrigemId, f)
		}
	}
}
