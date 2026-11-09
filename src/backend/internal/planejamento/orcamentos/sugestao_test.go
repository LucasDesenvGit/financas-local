package orcamentos

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// #675: os casos da sugestão que a mutação mostrou sem teste. Hoje é 14/10/2026, salvo quando o teste diz outro dia:
// a base são os 6 meses completos de abril a setembro.

// cenario monta o banco da sugestão: uma conta corrente, um cartão (sem limite, salvo quando o teste dá) e uma
// transação sem categoria em 01/01, para os meses da base serem completos.
type cenario struct {
	t        *testing.T
	fab      *testeapoio.Fabrica
	mux      *http.ServeMux
	corrente testeapoio.Conta
}

func novoCenario(t *testing.T, ano int, mes time.Month, dia int) *cenario {
	t.Helper()
	_, fab, mux := ambienteEm(t, testeapoio.HojeEmBrasilia(ano, mes, dia))
	return &cenario{t: t, fab: fab, mux: mux, corrente: fab.Conta()}
}

// marco grava a transação sem categoria que marca o primeiro dia com dados.
func (c *cenario) marco(dia tipos.Data) {
	c.fab.Transacao(c.corrente, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor = dia, testeapoio.Valor("-1.0") })
}

func (c *cenario) categoria(nome string, natureza int64, ajustes ...func(*testeapoio.Categoria)) testeapoio.Categoria {
	return c.fab.Categoria(func(x *testeapoio.Categoria) {
		x.Nome = nome
		if natureza != 0 {
			x.Natureza = &natureza
		}
		for _, a := range ajustes {
			a(x)
		}
	})
}

// gasto grava uma despesa da categoria no dia, na conta dada.
func (c *cenario) gasto(conta testeapoio.Conta, categoria testeapoio.Categoria, dia tipos.Data, valor string,
	ajustes ...func(*testeapoio.Transacao)) {
	c.fab.Transacao(conta, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Valor, tr.CategoriaId = dia, testeapoio.Valor(valor), &categoria.Id
		for _, a := range ajustes {
			a(tr)
		}
	})
}

// todoMes grava o mesmo gasto no dia 5 de abril a setembro de 2026.
func (c *cenario) todoMes(conta testeapoio.Conta, categoria testeapoio.Categoria, valor string) {
	for m := time.April; m <= time.September; m++ {
		c.gasto(conta, categoria, testeapoio.Dia(2026, m, 5), valor)
	}
}

// renda grava o salário no dia 5 de janeiro a setembro de 2026.
func (c *cenario) renda(valor string) {
	salario := c.fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Salário", 2 })
	for m := time.January; m <= time.September; m++ {
		c.gasto(c.corrente, salario, testeapoio.Dia(2026, m, 5), valor, func(tr *testeapoio.Transacao) { tr.Tipo = 2 })
	}
}

func (c *cenario) cartao(limite, fatura string) testeapoio.Conta {
	l := testeapoio.Valor(limite)
	cartao := c.fab.Conta(func(x *testeapoio.Conta) { x.Tipo, x.Nome, x.LimiteCredito = tipoCartao, "Cartão", &l })
	c.fab.Fatura(cartao, testeapoio.Dia(2026, 10, 10), testeapoio.Valor(fatura))
	return cartao
}

func (c *cenario) sugerir() (SugestaoDto, map[string]ItemSugestaoDto) {
	c.t.Helper()
	s := lerJSON[SugestaoDto](c.t, testeapoio.Chamar(c.t, c.mux, "GET", "/api/orcamentos/sugestao?ano=2026&mes=11", "").Corpo)
	itens := map[string]ItemSugestaoDto{}
	for _, i := range s.Itens {
		itens[i.Nome] = i
	}
	return s, itens
}

func igual(v tipos.Decimal, quer string) bool { return d(v).Equal(dec(quer)) }

func sugerido(i ItemSugestaoDto) string {
	if i.Sugerido == nil {
		return "nil"
	}
	return d(*i.Sugerido).String()
}

// Regra (#39, passo 1): a base é das despesas (valor negativo, tipo despesa, com categoria), sem as compras
// parceladas (a de 1 parcela entra); a subcategoria soma na principal; a categoria inativa não entra nem aparece.
func TestSugerirOQueEntraNaBase(t *testing.T) {
	t.Parallel()
	c := novoCenario(t, 2026, 10, 14)
	c.marco(testeapoio.Dia(2026, 1, 1))
	mercado := c.categoria("Mercado", naturezaEstiloVida)
	feira := c.categoria("Feira", naturezaEstiloVida, func(x *testeapoio.Categoria) { x.CategoriaPaiId = &mercado.Id })
	antiga := c.categoria("Antiga", naturezaEstiloVida, func(x *testeapoio.Categoria) { x.Ativa = false })
	c.todoMes(c.corrente, mercado, "-100.0")
	c.todoMes(c.corrente, antiga, "-100.0")
	c.gasto(c.corrente, mercado, testeapoio.Dia(2026, 5, 10), "50.0")                                                  // estorno
	c.gasto(c.corrente, mercado, testeapoio.Dia(2026, 6, 10), "-70.0", func(tr *testeapoio.Transacao) { tr.Tipo = 2 }) // receita
	tres, uma := int64(3), int64(1)
	c.gasto(c.corrente, mercado, testeapoio.Dia(2026, 7, 10), "-500.0", func(tr *testeapoio.Transacao) { tr.ParcelaTotal = &tres })
	c.gasto(c.corrente, mercado, testeapoio.Dia(2026, 8, 10), "-40.0", func(tr *testeapoio.Transacao) { tr.ParcelaTotal = &uma })
	c.gasto(c.corrente, feira, testeapoio.Dia(2026, 9, 10), "-20.0")

	_, itens := c.sugerir()
	// Mercado: 100 por mês, mais 40 em agosto (1 parcela) e 20 em setembro (Feira): 660 ÷ 6 = 110.
	m, ok := itens["Mercado"]
	if !ok || !igual(m.Media, "110") || !igual(m.UltimoMes, "120") || m.MesesComGasto != 6 {
		t.Errorf("mercado: %+v", m)
	}
	nomes := []string{}
	for n := range itens {
		nomes = append(nomes, n)
	}
	if len(itens) != 1 {
		t.Errorf("itens: %v (quer só o Mercado)", nomes)
	}
}

// Regra (#39, passo 1): a base que passa da virada do ano põe cada mês no lugar dele (hoje 10/02/2027: agosto a
// janeiro).
func TestSugerirNaViradaDoAno(t *testing.T) {
	t.Parallel()
	c := novoCenario(t, 2027, 2, 10)
	c.marco(testeapoio.Dia(2026, 8, 1))
	mercado := c.categoria("Mercado", naturezaEstiloVida)
	c.gasto(c.corrente, mercado, testeapoio.Dia(2026, 12, 5), "-100.0")
	c.gasto(c.corrente, mercado, testeapoio.Dia(2027, 1, 5), "-100.0")
	_, itens := c.sugerir()
	if m := itens["Mercado"]; m.MesesComGasto != 2 || !igual(m.UltimoMes, "100") || m.Sugerido == nil {
		t.Errorf("mercado: %+v", m)
	}
}

// Regra (#39, passo 3): sem cartão, ou com o uso do limite baixo, nada é cortado pelo cartão e não há aviso dele.
func TestSugerirCartaoSemCorte(t *testing.T) {
	t.Parallel()
	for _, comCartao := range []bool{false, true} {
		c := novoCenario(t, 2026, 10, 14)
		c.marco(testeapoio.Dia(2026, 1, 1))
		lazer := c.categoria("Lazer", naturezaEstiloVida)
		onde := c.corrente
		if comCartao {
			onde = c.cartao("10000.0", "-1000.0") // 10% do limite: abaixo da atenção (30%)
		}
		c.todoMes(onde, lazer, "-1000.0")
		s, itens := c.sugerir()
		if l := itens["Lazer"]; sugerido(l) != "1000" || len(l.Motivos) != 1 {
			t.Errorf("cartão %v: lazer %+v", comCartao, l)
		}
		for _, a := range s.Avisos {
			if strings.Contains(a, "cartões") {
				t.Errorf("cartão %v: aviso %q", comCartao, a)
			}
		}
		if (s.NivelCartao != nil) != comCartao || (comCartao && *s.NivelCartao != "ok") {
			t.Errorf("cartão %v: nível %v", comCartao, s.NivelCartao)
		}
	}
}

// Regra (#39, passo 3): com o cartão em alerta, só o estilo de vida pago (pelo menos metade) no cartão é cortado;
// o pago menos no cartão, o essencial e o sem natureza ficam.
func TestSugerirCartaoEmAlertaSoCortaOEstiloDeVidaNoCartao(t *testing.T) {
	t.Parallel()
	c := novoCenario(t, 2026, 10, 14)
	c.marco(testeapoio.Dia(2026, 1, 1))
	cartao := c.cartao("1000.0", "-800.0") // 80%: alerta
	lazer := c.categoria("Lazer", naturezaEstiloVida)
	cinema := c.categoria("Cinema", naturezaEstiloVida)
	farmacia := c.categoria("Farmácia", naturezaEssencial)
	diversos := c.categoria("Diversos", 0)
	c.todoMes(cartao, lazer, "-1000.0")
	c.todoMes(cartao, cinema, "-400.0")
	c.todoMes(c.corrente, cinema, "-600.0")
	c.todoMes(cartao, farmacia, "-1000.0")
	c.todoMes(c.corrente, diversos, "-500.0")
	_, itens := c.sugerir()
	for nome, quer := range map[string]string{"Lazer": "900", "Cinema": "1000", "Farmácia": "1050", "Diversos": "500"} {
		if obtido := sugerido(itens[nome]); obtido != quer {
			t.Errorf("%s: %s, quer %s (%v)", nome, obtido, quer, itens[nome].Motivos)
		}
	}
}

// Regra (#39, passo 5): o essencial arredonda para a dezena mais próxima; o estilo de vida e o sem natureza, para
// baixo. Sem renda, nada é cortado para caber.
func TestSugerirArredondamento(t *testing.T) {
	t.Parallel()
	c := novoCenario(t, 2026, 10, 14)
	c.marco(testeapoio.Dia(2026, 1, 1))
	c.todoMes(c.corrente, c.categoria("Mercado", naturezaEssencial), "-806.0") // + 5% = 846,30
	c.todoMes(c.corrente, c.categoria("Lazer", naturezaEstiloVida), "-965.0")
	c.todoMes(c.corrente, c.categoria("Outros", 0), "-965.0")
	_, itens := c.sugerir()
	for nome, quer := range map[string]string{"Mercado": "850", "Lazer": "960", "Outros": "960"} {
		if obtido := sugerido(itens[nome]); obtido != quer {
			t.Errorf("%s: %s, quer %s", nome, obtido, quer)
		}
	}
}

// Regra (#39, passos 4 e 5): o arredondamento do essencial para cima pode passar do disponível; aí o maior que não é
// essencial perde R$ 10 até caber. Renda 998, sem poupança: Mercado 846,30 vira 850, e o Lazer 150 vira 140.
func TestSugerirArredondamentoQuePassaDoDisponivel(t *testing.T) {
	t.Parallel()
	c := novoCenario(t, 2026, 10, 14)
	c.fab.Configuracao("Orcamento.MetaPoupanca", "0")
	c.renda("998.0")
	c.todoMes(c.corrente, c.categoria("Mercado", naturezaEssencial), "-806.0")
	c.todoMes(c.corrente, c.categoria("Lazer", naturezaEstiloVida), "-150.0")
	s, itens := c.sugerir()
	if sugerido(itens["Mercado"]) != "850" || sugerido(itens["Lazer"]) != "140" || !igual(s.TotalSugerido, "990") ||
		s.FaltaParaCaber != nil {
		t.Errorf("mercado %s, lazer %s, total %s, falta %v", sugerido(itens["Mercado"]), sugerido(itens["Lazer"]),
			s.TotalSugerido, s.FaltaParaCaber)
	}
}

// Regra (#39, passo 4): o estilo de vida é cortado primeiro (até 50%); o que ainda sobra sai do sem natureza (até
// 30%). Renda 1000, sem poupança: Lazer 100 e Outros 1100 passam 200; o Lazer perde 50 e o Outros, 150.
func TestSugerirCorteDoSemNatureza(t *testing.T) {
	t.Parallel()
	c := novoCenario(t, 2026, 10, 14)
	c.fab.Configuracao("Orcamento.MetaPoupanca", "0")
	c.renda("1000.0")
	c.todoMes(c.corrente, c.categoria("Lazer", naturezaEstiloVida), "-100.0")
	c.todoMes(c.corrente, c.categoria("Outros", 0), "-1100.0")
	s, itens := c.sugerir()
	if sugerido(itens["Lazer"]) != "50" || sugerido(itens["Outros"]) != "950" || s.FaltaParaCaber != nil {
		t.Errorf("lazer %s, outros %s, falta %v", sugerido(itens["Lazer"]), sugerido(itens["Outros"]), s.FaltaParaCaber)
	}
}

// Regra (#39, passo 4): sem caber mesmo cortando, o arredondamento não tira mais R$ 10 de ninguém.
func TestSugerirSemCaberNaoCortaMais(t *testing.T) {
	t.Parallel()
	c := novoCenario(t, 2026, 10, 14)
	c.fab.Configuracao("Orcamento.MetaPoupanca", "0")
	c.renda("500.0")
	c.todoMes(c.corrente, c.categoria("Mercado", naturezaEssencial), "-1000.0")
	c.todoMes(c.corrente, c.categoria("Lazer", naturezaEstiloVida), "-100.0")
	s, itens := c.sugerir()
	if s.FaltaParaCaber == nil || sugerido(itens["Lazer"]) != "50" || sugerido(itens["Mercado"]) != "1050" {
		t.Errorf("falta %v, lazer %s, mercado %s", s.FaltaParaCaber, sugerido(itens["Lazer"]), sugerido(itens["Mercado"]))
	}
}

// Regra: sem nenhum mês completo, a base não divide por zero.
func TestCalcularBaseSemMeses(t *testing.T) {
	t.Parallel()
	l := &linha{}
	calcularBase(l, 0, testeapoio.Dia(2026, 10, 1), decimal.Zero, dec("0.05"))
	if l.valor != nil || !l.media.IsZero() {
		t.Errorf("valor %v, média %s", l.valor, l.media)
	}
}

// Regra: a mediana de uma quantidade ímpar é o valor do meio; de uma par, a média dos dois do meio.
func TestMediana(t *testing.T) {
	t.Parallel()
	casos := []struct {
		valores []string
		quer    string
	}{{[]string{"10", "1", "2"}, "2"}, {[]string{"3"}, "3"}, {[]string{"1", "2", "3", "10"}, "2.5"}, {nil, "0"}}
	for _, c := range casos {
		var v []decimal.Decimal
		for _, s := range c.valores {
			v = append(v, dec(s))
		}
		if obtido := mediana(v); !obtido.Equal(dec(c.quer)) {
			t.Errorf("mediana(%v) = %s", c.valores, obtido)
		}
	}
}

// Regra (#39): os itens da tela vão pelo sugerido, mesmo com a média em outra ordem (os sem sugestão no fim, pela
// média); ficam só os com sugestão,
// gasto ou orçamento atual; "selecionado" é o que tem sugestão e ainda não tem orçamento.
func TestItens(t *testing.T) {
	t.Parallel()
	valor := func(s string) *decimal.Decimal { v := dec(s); return &v }
	linhas := []*linha{
		{nome: "A", valor: valor("100"), media: dec("500"), mesesComGasto: 6},
		{nome: "B", valor: valor("300"), media: dec("50"), mesesComGasto: 6},
		{nome: "C", media: dec("10"), mesesComGasto: 1},
		{nome: "E"},
		{nome: "F", atual: valor("50")},
		{nome: "G", valor: valor("200"), atual: valor("200"), media: dec("200"), mesesComGasto: 6},
	}
	var nomes, selecionados []string
	for _, i := range itens(linhas) {
		nomes = append(nomes, i.Nome)
		if i.Selecionado {
			selecionados = append(selecionados, i.Nome)
		}
	}
	if !slices.Equal(nomes, []string{"B", "G", "A", "C", "F"}) || !slices.Equal(selecionados, []string{"B", "A"}) {
		t.Errorf("itens %v, selecionados %v", nomes, selecionados)
	}
}

// Regra: o período vale de janeiro de 2000 a dezembro de 2100, inclusive.
func TestPeriodoNosLimites(t *testing.T) {
	t.Parallel()
	_, _, mux := ambiente(t)
	for _, q := range []string{"ano=2000&mes=1", "ano=2100&mes=12"} {
		if r := testeapoio.Chamar(t, mux, "GET", "/api/orcamentos?"+q, ""); r.Status != http.StatusOK {
			t.Errorf("%s: %d %s", q, r.Status, r.Corpo)
		}
	}
}

// Regra: o pedido cancelado não vira 500 nem log de erro, venha o cancelamento no erro ou no contexto do pedido.
func TestErroInternoDoPedidoCancelado(t *testing.T) {
	t.Parallel()
	s := &Servico{log: nil}
	cancelado, cancelar := context.WithCancel(context.Background())
	cancelar()
	for nome, c := range map[string]struct {
		ctx context.Context
		err error
	}{
		"contexto cancelado": {cancelado, errors.New("o banco parou")},
		"erro de cancelado":  {context.Background(), context.Canceled},
	} {
		w := httptest.NewRecorder()
		s.erroInterno(w, httptest.NewRequest("GET", "/api/orcamentos", nil).WithContext(c.ctx), "teste", c.err)
		if w.Code != http.StatusOK || w.Body.Len() != 0 {
			t.Errorf("%s: %d %s", nome, w.Code, w.Body)
		}
	}
}
