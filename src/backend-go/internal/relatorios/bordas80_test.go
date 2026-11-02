package relatorios

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// Regra: o fluxo inclui o mês do fim mesmo quando o fim é o dia 1 dele.
func TestFluxoComFimNoDia1(t *testing.T) {
	t.Parallel()
	if lista := fluxoMensal(nil, dia(2026, 3, 15), dia(2026, 5, 1)); len(lista) != 3 || lista[2].Mes != 5 {
		t.Errorf("fluxo: %+v", lista)
	}
}

// Regra (bordas da projeção): a parcela única (ParcelaTotal 1) conta como à vista; a despesa do dia 1 do último mês
// completo entra no "último mês"; a parcela futura de um cartão manual não volta ao saldo de partida (só a do cartão do
// Open Finance, inclusive a que vence no dia 1 do mês que vem); a categoria chamada "Sem categoria" não se confunde
// com as despesas sem categoria.
func TestProjecaoBordas(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 14)
	fab := testeapoio.NovaFabrica(t, db, r)
	conta := fab.Conta()
	manual := fab.Conta(func(c *testeapoio.Conta) { c.Tipo = tipoContaCartao })
	openFinance := fab.Conta(func(c *testeapoio.Conta) { c.Tipo, c.Origem = tipoContaCartao, origemOpenFinance })
	falsaSem := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Sem categoria" })
	for m := 4; m <= 9; m++ {
		despesaDoMes(fab, conta, falsaSem, testeapoio.Dia(2026, monthOf(m), 2), "-6.0")
		fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor = testeapoio.Dia(2026, monthOf(m), 3), dec("-12.0") })
	}
	// No dia 1 de setembro (o último mês completo), parcela única: conta como à vista e no último mês.
	despesaDoMes(fab, conta, falsaSem, testeapoio.Dia(2026, 9, 1), "-60.0", func(tr *testeapoio.Transacao) {
		tr.ParcelaNumero, tr.ParcelaTotal = ptr(int64(1)), ptr(int64(1))
	})
	parcela := func(cartao testeapoio.Conta, desc string) {
		fab.Transacao(cartao, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Descricao = testeapoio.Dia(2026, 10, 1), dec("-100.0"), desc
			tr.ParcelaNumero, tr.ParcelaTotal = ptr(int64(1)), ptr(int64(2))
		})
	}
	parcela(manual, "LOJA MANUAL 1/2")
	parcela(openFinance, "LOJA OF 1/2")
	p, err := NovoServico(db, r).Projecao(t.Context(), 1, map[int64]tipos.Decimal{})
	if err != nil {
		t.Fatal(err)
	}
	var categorias []string
	for _, c := range p.Categorias {
		id := "nil"
		if c.CategoriaId != nil {
			id = "id"
		}
		categorias = append(categorias, fmt.Sprintf("%s:%s:%s:%s", id, c.Nome, c.MediaMensal, c.UltimoMes))
	}
	// A primeira transação é de 02/04: os meses completos são de maio a setembro (5). Categoria falsa: (30 + 60) ÷ 5 =
	// 18.0, último mês 66.0; sem categoria: 60 ÷ 5 = 12.0, último 12.0; média das despesas: 150 ÷ 5 = 30.0. Saldo de
	// partida: só a parcela de novembro do cartão do Open Finance (100.0).
	if strings.Join(categorias, " ") != "id:Sem categoria:18.0:66.0 nil:Sem categoria:12.0:12.0" ||
		p.SaldoDisponivelAtual.String() != "100.0" || p.MediaDespesas.String() != "30.0" {
		t.Errorf("projeção: %v, saldo %s, média %s", categorias, p.SaldoDisponivelAtual, p.MediaDespesas)
	}
}

// Regra: quem começou a usar este mês ainda não tem mês completo: a projeção sai sem histórico e sem médias, e a saúde
// financeira, sem gasto médio nem meses de reserva.
func TestSemMesCompleto(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 14)
	fab := testeapoio.NovaFabrica(t, db, r)
	conta := fab.Conta()
	fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor = testeapoio.Dia(2026, 10, 5), dec("-40.0") })
	fab.Transacao(conta, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Valor, tr.Tipo = testeapoio.Dia(2026, 10, 6), dec("100.0"), transacoes.Receita
	})
	s := NovoServico(db, r)
	p, err := s.Projecao(t.Context(), 1, nil)
	if err != nil || p.MesesConsiderados != 0 || len(p.Meses) != 2 || p.Meses[0].Situacao != EmAndamento ||
		p.MediaDespesas.String() != "0" || len(p.Categorias) != 0 {
		t.Errorf("projeção: %+v %v", p, err)
	}
	a, err := s.PainelAnual(t.Context(), 2026, 2025, Patrimonio{Reserva: dec("10.0")})
	if err != nil || a.Saude.MesesAnalisados != 0 || a.Saude.MesesDeReserva != nil || a.Saude.GastoMensalMedio.String() != "0" {
		t.Errorf("saúde: %+v %v", a.Saude, err)
	}
}

// Regra (visão anual): a categoria com natureza própria vale a dela, mesmo debaixo de um pai essencial (não é
// essencial), e a sem natureza e sem pai não é essencial; no empate de valor e de ano anterior, as categorias ficam na
// ordem do GROUP BY (sem categoria primeiro, depois pelo Id), e não na ordem dos lançamentos.
func TestPainelAnualNaturezaEEmpate(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 14)
	fab := testeapoio.NovaFabrica(t, db, r)
	conta := fab.Conta()
	essencial, estilo := int64(NaturezaEssencial), int64(NaturezaEstiloDeVida)
	a := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Natureza = "A", &essencial })
	b := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "B" })
	luxo := fab.Subcategoria(a, func(c *testeapoio.Categoria) { c.Nome, c.Natureza = "Luxo", &estilo })
	// Gravadas na ordem B, sem categoria, A: a ordem final é sem categoria, A, B.
	despesaDoMes(fab, conta, b, testeapoio.Dia(2026, 2, 1), "-10.0")
	fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor = testeapoio.Dia(2026, 2, 2), dec("-10.0") })
	despesaDoMes(fab, conta, a, testeapoio.Dia(2026, 2, 3), "-10.0")
	despesaDoMes(fab, conta, luxo, testeapoio.Dia(2026, 2, 4), "-90.0")
	anual, err := NovoServico(db, r).PainelAnual(t.Context(), 2026, 2025, Patrimonio{})
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for _, c := range anual.Categorias {
		nomes = append(nomes, c.Nome+":"+c.Valor.String())
	}
	// O luxo (estilo de vida) conta no pai A (a raiz), mas com a natureza dele; o gasto essencial é só o de A (10 em
	// 8 meses completos: de fevereiro a setembro).
	if strings.Join(nomes, " ") != "A:100.0 Sem categoria:10.0 B:10.0" || anual.Distribuicao.Essenciais.String() != "10.0" ||
		anual.Distribuicao.EstiloDeVida.String() != "90.0" || anual.Saude.GastoEssencialMensalMedio.String() != "1.25" {
		t.Errorf("categorias %v, distribuição %+v, essencial %s", nomes, anual.Distribuicao, anual.Saude.GastoEssencialMensalMedio)
	}
}
