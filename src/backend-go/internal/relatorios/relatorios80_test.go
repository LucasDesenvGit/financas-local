package relatorios

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

func naData(d tipos.Data) func(*Lancamento) { return func(l *Lancamento) { l.Data = d } }

// Regra (#80): o fluxo tem todos os meses do período, com 0 nos sem movimento; o mês com movimento tem as três somas no
// formato do banco (a do tipo que não apareceu sai "0.0"); despesas positivas, estorno abatendo; emergência é o valor
// absoluto das saídas marcadas (a entrada marcada não conta).
func TestFluxoMensal(t *testing.T) {
	t.Parallel()
	lancamentos := []Lancamento{
		receita(0, "1000.5"),
		lanc(1, "-200.25", naData(dia(2026, 3, 5)), emergencia),
		lanc(1, "50.0", naData(dia(2026, 3, 9)), emergencia), // estorno marcado: abate a despesa, não a emergência
		lanc(2, "-10.0", naData(dia(2026, 5, 1))),
	}
	lancamentos[0].Data = dia(2026, 3, 1)
	lista := fluxoMensal(lancamentos, dia(2026, 2, 15), dia(2026, 5, 31))
	var partes []string
	for _, m := range lista {
		partes = append(partes, fmt.Sprintf("%d/%d %s %s %s", m.Mes, m.Ano, m.Receitas, m.Despesas, m.Emergencia))
	}
	quer := "2/2026 0 0 0|3/2026 1000.5 150.25 200.25|4/2026 0 0 0|5/2026 0.0 10.0 0.0"
	if obtido := strings.Join(partes, "|"); obtido != quer {
		t.Errorf("fluxo:\n %s\nquer\n %s", obtido, quer)
	}
	r, d := totais(lancamentos)
	if r.String() != "1000.5" || d.String() != "160.25" {
		t.Errorf("totais: %s %s", r, d)
	}
	// O SumAsync do EF Core sobre nenhuma linha volta "0.0" (contrato 13-painel, o mês futuro).
	if r, d := totais(nil); r.String() != "0.0" || d.String() != "0.0" {
		t.Errorf("totais vazios: %s %s", r, d)
	}
}

// Regra (#80): o agrupamento por categoria da projeção nega a soma (despesa líquida de estorno), põe a subcategoria no
// pai (um nível; a categoria cujo pai não existe fica como ela mesma), manda a sem categoria para "Sem categoria" (por
// último, com a cor da projeção) e tira quem não passou de zero.
func TestAgruparPorCategoria(t *testing.T) {
	t.Parallel()
	um, tres, quatro, dois := int64(1), int64(3), int64(4), int64(2)
	grupos := []somaPorCategoria{
		{nil, dec("-5.0")},
		{&um, dec("-10.0")},
		{&dois, dec("3.0")}, // só estorno: fica de fora
		{&tres, dec("-2.5")},
		{&quatro, dec("-1.0")},
	}
	lista := agruparPorCategoria(grupos, categoriasDeTeste())
	var partes []string
	for _, c := range lista {
		id := "-"
		if c.CategoriaId != nil {
			id = fmt.Sprint(*c.CategoriaId)
		}
		partes = append(partes, id+":"+c.Nome+":"+c.Cor+":"+c.Valor.String())
	}
	if obtido := strings.Join(partes, "|"); obtido != "1:Mercado:#111111:12.5|4:Órfã:#444444:1.0|-:Sem categoria:#a9b4c6:5.0" {
		t.Errorf("agrupar: %s", obtido)
	}
	if lista := agruparPorCategoria(nil, categoriasDeTeste()); len(lista) != 0 {
		t.Errorf("vazio: %v", lista)
	}
	if lista := agruparPorCategoria([]somaPorCategoria{{nil, dec("1.0")}}, categoriasDeTeste()); len(lista) != 0 {
		t.Errorf("sem categoria negativa: %v", lista)
	}
}

// Regra (#80): o resumo soma os meses até o limite; a taxa de poupança é (receitas − despesas) ÷ receitas × 100 em uma
// casa pelo método bancário, e 0 sem receitas.
func TestResumir(t *testing.T) {
	t.Parallel()
	meses := []MesValoresDto{
		{Mes: 1, Receitas: dec("100.0"), Despesas: dec("33.35")},
		{Mes: 2, Receitas: dec("100.0"), Despesas: dec("33.35")},
		{Mes: 3, Receitas: dec("900.0"), Despesas: dec("0")},
	}
	casos := []struct {
		ate  int
		quer string
	}{
		{0, "0 0 0 0"},
		{1, "100.0 33.35 66.65 66.6"}, // 66.65 → 66.6: o 5 vai para o par
		{2, "200.0 66.70 133.30 66.6"},
		{3, "1100.0 66.70 1033.30 93.9"},
	}
	for _, c := range casos {
		r := resumir(meses, c.ate)
		if obtido := fmt.Sprintf("%s %s %s %s", r.Receitas, r.Despesas, r.Resultado, r.TaxaPoupanca); obtido != c.quer {
			t.Errorf("até %d: %s, quer %s", c.ate, obtido, c.quer)
		}
	}
}

// Regra (#80, conferido à mão): a projeção usa a média dos meses completos (receitas e despesas à vista), o mês atual
// com o realizado e a fração dos dias que faltam, as parcelas futuras de cada mês e o saldo de partida (contas que não
// são investimento, mais as parcelas futuras dos cartões do Open Finance a partir do mês que vem).
func TestProjecao(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 14)
	fab := testeapoio.NovaFabrica(t, db, r)
	s := NovoServico(db, r)
	corrente := fab.Conta()
	investimento := fab.Conta(func(c *testeapoio.Conta) { c.Tipo = 4 })
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Tipo, c.Origem = tipoContaCartao, origemOpenFinance })
	mercado := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Mercado" })

	p, err := s.Projecao(t.Context(), 2, nil)
	// Sem transação nenhuma, a janela é a de 6 meses (JanelaHistorico, #447), com tudo zerado: a soma vazia é "0.0", e a
	// média das despesas sai 0 porque o Math.Max(0m, −0.0) fica com o primeiro no empate.
	if err != nil || p.MesesConsiderados != 6 || len(p.Meses) != 9 || p.Meses[6].Situacao != EmAndamento ||
		p.MediaReceitas.String() != "0.0" || p.MediaDespesas.String() != "0" || p.Meses[8].SaldoProjetado == nil ||
		p.Meses[8].SaldoProjetado.String() != "0.0" {
		t.Fatalf("sem histórico: %+v %v", p, err)
	}

	for m := 4; m <= 9; m++ {
		fab.Transacao(corrente, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Tipo = testeapoio.Dia(2026, monthOf(m), 1), dec("3000.0"), transacoes.Receita
		})
		despesaDoMes(fab, corrente, mercado, testeapoio.Dia(2026, monthOf(m), 2), "-600.0")
	}
	fab.Transacao(corrente, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Valor, tr.Tipo = testeapoio.Dia(2026, 10, 5), dec("1000.0"), transacoes.Receita
	})
	despesaDoMes(fab, corrente, mercado, testeapoio.Dia(2026, 10, 6), "-100.0")
	// Compra no cartão do Open Finance em 3 vezes: a 1ª em outubro, as outras duas projetadas para novembro e dezembro.
	fab.Transacao(cartao, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Valor, tr.Descricao = testeapoio.Dia(2026, 10, 3), dec("-50.0"), "LOJA 1/3"
		tr.ParcelaNumero, tr.ParcelaTotal = ptr(int64(1)), ptr(int64(3))
	})
	saldos := map[int64]tipos.Decimal{corrente.Id: dec("2000.0"), investimento.Id: dec("9999.0"), cartao.Id: dec("-150.0")}

	p, err = s.Projecao(t.Context(), 2, saldos)
	if err != nil {
		t.Fatal(err)
	}
	// Médias: 3000 e 600. Mês atual: recebido 1000 < 3000; saído 150 (100 + a parcela de outubro, 50) + 600 × 17/31.
	// Saldo de partida: 2000 − 150 (+100 das duas parcelas de nov e dez no cartão do Open Finance) = 1950.
	var partes []string
	for _, m := range p.Meses {
		saldo := "null"
		if m.SaldoProjetado != nil {
			saldo = m.SaldoProjetado.String()
		}
		partes = append(partes, fmt.Sprintf("%d %s %s %s %s %s", m.Mes, m.Situacao, m.Receitas, m.Despesas, m.Parcelas, saldo))
	}
	quer := []string{
		"4 realizado 3000.0 600.0 0 null", "5 realizado 3000.0 600.0 0 null", "6 realizado 3000.0 600.0 0 null",
		"7 realizado 3000.0 600.0 0 null", "8 realizado 3000.0 600.0 0 null", "9 realizado 3000.0 600.0 0 null",
		"10 em andamento 3000.0 479.03 0 3620.97", // 1950 + (3000 − 1000) − (479.03… − 150)
		"11 projetado 3000.0 650.0 50.0 5970.97",  // Math.Round(50.0m, 2) continua 50.0
		"12 projetado 3000.0 650.0 50.0 8320.97",
	}
	if obtido := strings.Join(partes, "|"); obtido != strings.Join(quer, "|") {
		t.Errorf("meses:\n %s\nquer\n %s", strings.Join(partes, "\n "), strings.Join(quer, "\n "))
	}
	if p.MesesConsiderados != 6 || p.MediaReceitas.String() != "3000.0" || p.MediaDespesas.String() != "600.0" ||
		p.SaldoDisponivelAtual.String() != "1950.0" || len(p.Categorias) != 1 || p.Categorias[0].UltimoMes.String() != "600.0" {
		t.Errorf("resumo: %+v", p)
	}
}

func ptr[T any](v T) *T { return &v }

// Regra (#80, conferido à mão): a visão anual soma o ano até o mês atual (taxa de poupança em uma casa), compara com o
// mesmo período e com o ano inteiro de comparação, estima o patrimônio no fim de cada mês pelo de hoje menos o
// resultado dos meses depois, põe as despesas na categoria raiz (a natureza da subcategoria ou, sem ela, a do pai),
// divide a renda pela natureza e calcula a saúde financeira dos meses completos.
func TestPainelAnual(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 14)
	fab := testeapoio.NovaFabrica(t, db, r)
	conta := fab.Conta()
	essencial, estilo := int64(NaturezaEssencial), int64(NaturezaEstiloDeVida)
	casa := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Natureza = "Casa", &essencial })
	aluguel := fab.Subcategoria(casa, func(c *testeapoio.Categoria) { c.Nome = "Aluguel" })
	lazer := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Natureza = "Lazer", &estilo })
	outros := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Outros" })
	receitaEm := func(d tipos.Data, v string) {
		fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor, tr.Tipo = d, dec(v), transacoes.Receita })
	}
	receitaEm(testeapoio.Dia(2025, 11, 10), "1000.0")
	despesaDoMes(fab, conta, aluguel, testeapoio.Dia(2025, 11, 11), "-400.0")
	receitaEm(testeapoio.Dia(2026, 1, 10), "2000.0")
	despesaDoMes(fab, conta, aluguel, testeapoio.Dia(2026, 1, 11), "-500.0")
	despesaDoMes(fab, conta, lazer, testeapoio.Dia(2026, 1, 12), "-100.0")
	fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor = testeapoio.Dia(2026, 1, 13), dec("-50.0") })
	despesaDoMes(fab, conta, outros, testeapoio.Dia(2026, 3, 5), "-30.0")
	receitaEm(testeapoio.Dia(2026, 10, 1), "500.0")
	s := NovoServico(db, r)
	p := Patrimonio{Atual: dec("10000.0"), Reserva: dec("3000.0"), Credito: Credito{SaldoFinanciamentos: dec("1.0"),
		SaldoEmprestimos: dec("2.0"), ParcelaMensalFinanciamentos: dec("3.0"), ParcelaMensalEmprestimos: dec("4.0")}}

	a, err := s.PainelAnual(t.Context(), 2026, 2025, p)
	if err != nil {
		t.Fatal(err)
	}
	resumo := func(x ResumoAnoDto) string {
		return fmt.Sprintf("%s %s %s %s", x.Receitas, x.Despesas, x.Resultado, x.TaxaPoupanca)
	}
	if a.MesLimite != 10 || fmt.Sprint(a.AnosDisponiveis) != "[2026 2025]" || resumo(a.Resumo) != "2500.0 680.0 1820.0 72.8" ||
		resumo(a.ResumoAnteriorMesmoPeriodo) != "0 0 0 0" || resumo(a.ResumoAnteriorCompleto) != "1000.0 400.0 600.0 60.0" {
		t.Errorf("resumos: %d %v %s / %s / %s", a.MesLimite, a.AnosDisponiveis, resumo(a.Resumo),
			resumo(a.ResumoAnteriorMesmoPeriodo), resumo(a.ResumoAnteriorCompleto))
	}
	patrimonio := func(m MesComparativoDto) string {
		if m.PatrimonioEstimado == nil {
			return "null"
		}
		return m.PatrimonioEstimado.String()
	}
	if patrimonio(a.Meses[8]) != "9500.0" || patrimonio(a.Meses[9]) != "10000.0" || patrimonio(a.Meses[10]) != "null" ||
		!a.Meses[10].Futuro || a.Meses[0].Despesas.String() != "650.0" || a.Meses[10].DespesasAnterior.String() != "400.0" {
		t.Errorf("meses: set %s, out %s, nov %s %t", patrimonio(a.Meses[8]), patrimonio(a.Meses[9]), patrimonio(a.Meses[10]),
			a.Meses[10].Futuro)
	}
	var categorias []string
	for _, c := range a.Categorias {
		natureza := "null"
		if c.Natureza != nil {
			natureza = fmt.Sprint(*c.Natureza)
		}
		categorias = append(categorias, fmt.Sprintf("%s:%s:%s:%s:%s", c.Nome, natureza, c.Valor, c.ValorAnterior, c.Meses[0]))
	}
	if obtido := strings.Join(categorias, "|"); obtido !=
		"Casa:1:500.0:0:500.0|Lazer:2:100.0:0:100.0|Sem categoria:null:50.0:0:50.0|Outros:null:30.0:0:0" {
		t.Errorf("categorias: %s", obtido)
	}
	d := a.Distribuicao
	if fmt.Sprintf("%s %s %s %s %s", d.Receitas, d.Essenciais, d.EstiloDeVida, d.NaoClassificado, d.Poupanca) !=
		"2500.0 500.0 100.0 80.0 1820.0" {
		t.Errorf("distribuição: %+v", d)
	}
	// Saúde: meses completos de dezembro/2025 a setembro/2026 (10); gasto 680 ÷ 10 = 68.0; essencial 500 ÷ 10 = 50.0
	// (o aluguel, filho da Casa); reserva 3000 ÷ 68 = 44.1 meses e 3000 ÷ 50 = 60; renda 2000 ÷ 10 = 200.0, e o
	// comprometimento (0 do cartão + 3 + 4) ÷ 200 × 100 = 3.5.
	h := a.Saude
	texto := func(v *tipos.Decimal) string {
		if v == nil {
			return "null"
		}
		return v.String()
	}
	if h.MesesAnalisados != 10 || h.GastoMensalMedio.String() != "68.0" || h.GastoEssencialMensalMedio.String() != "50.0" ||
		texto(h.MesesDeReserva) != "44.1" || texto(h.MesesDeReservaEssencial) != "60" || h.PatrimonioAtual.String() != "10000.0" ||
		h.ReservaLiquida.String() != "3000.0" || h.ParcelasFinanciamentos.String() != "3.0" || h.DividaEmprestimos.String() != "2.0" ||
		texto(h.ComprometimentoParcelas) != "3.5" || h.RendaMensalMedia.String() != "200.0" || len(a.Meses) != 12 {
		t.Errorf("saúde: %+v %s %s", h, texto(h.MesesDeReserva), texto(h.MesesDeReservaEssencial))
	}

	// Ano futuro: nenhum mês até hoje; as categorias aparecem pelo ano de comparação (janeiro, o mínimo).
	f, err := s.PainelAnual(t.Context(), 2027, 2026, p)
	if err != nil {
		t.Fatal(err)
	}
	categorias = nil
	for _, c := range f.Categorias {
		categorias = append(categorias, c.Nome+":"+c.Valor.String()+":"+c.ValorAnterior.String())
	}
	if f.MesLimite != 0 || patrimonio(f.Meses[0]) != "null" || strings.Join(categorias, "|") != "Casa:0:500.0|Lazer:0:100.0|Sem categoria:0:50.0" {
		t.Errorf("ano futuro: %d %v", f.MesLimite, categorias)
	}

	// O ano atual sem transação entra na frente da lista de anos; o ano passado vai até dezembro.
	depois := NovoServico(db, testeapoio.HojeEmBrasilia(2027, 2, 1))
	pa, err := depois.PainelAnual(t.Context(), 2025, 2024, p)
	if err != nil || fmt.Sprint(pa.AnosDisponiveis) != "[2027 2026 2025]" || pa.MesLimite != 12 {
		t.Errorf("ano passado: %v %d %v", pa.AnosDisponiveis, pa.MesLimite, err)
	}
}

// Regra: cada consulta da projeção e da visão anual que falha vira um erro com a causa (errors.Is), e nunca uma resposta
// pela metade. Cada consulta falha uma vez, da 1ª à última vez que aparece.
func TestProjecaoEVisaoAnualComErroDoBanco(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 14)
	fab := testeapoio.NovaFabrica(t, db, r)
	conta := fab.Conta()
	mercado := fab.Categoria()
	for m := 4; m <= 10; m++ {
		despesaDoMes(fab, conta, mercado, testeapoio.Dia(2026, monthOf(m), 2), "-60.0")
		fab.Transacao(conta, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Tipo = testeapoio.Dia(2026, monthOf(m), 1), dec("100.0"), transacoes.Receita
		})
	}
	chamadas := map[string]func(s *Servico) error{
		"projeção": func(s *Servico) error { _, err := s.Projecao(t.Context(), 2, nil); return err },
		"anual": func(s *Servico) error {
			_, err := s.PainelAnual(t.Context(), 2026, 2025, Patrimonio{Reserva: dec("1.0")})
			return err
		},
	}
	for nome, chamar := range chamadas {
		if err := chamar(NovoServico(db, r)); err != nil {
			t.Fatalf("%s sem falha: %v", nome, err)
		}
		for _, trecho := range []string{`FROM "Transacao"`, `FROM "Categoria"`, `FROM "Conta"`} {
			falhas := 0
			for vez := 1; vez < 50; vez++ {
				err := chamar(NovoServico(falhaEm(db, trecho, vez), r))
				if err == nil {
					break
				}
				if !errors.Is(err, errDeTeste) {
					t.Errorf("%s, %s (vez %d): %v", nome, trecho, vez, err)
				}
				falhas++
			}
			if falhas == 0 && !(nome == "anual" && trecho == `FROM "Conta"`) {
				t.Errorf("%s: nenhuma consulta com %s", nome, trecho)
			}
		}
	}
}

// Regra: as categorias da projeção vão da maior média para a menor (a gravada depois pode vir antes), e a sem
// categoria entra quando passa de zero.
func TestProjecaoOrdemDasCategorias(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 14)
	fab := testeapoio.NovaFabrica(t, db, r)
	conta := fab.Conta()
	mercado := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Mercado" })
	lazer := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Lazer" })
	for m := 4; m <= 9; m++ {
		despesaDoMes(fab, conta, mercado, testeapoio.Dia(2026, monthOf(m), 2), "-10.0")
		despesaDoMes(fab, conta, lazer, testeapoio.Dia(2026, monthOf(m), 3), "-30.0")
		fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor = testeapoio.Dia(2026, monthOf(m), 4), dec("-20.0") })
	}
	p, err := NovoServico(db, r).Projecao(t.Context(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for _, c := range p.Categorias {
		nomes = append(nomes, c.Nome+":"+c.MediaMensal.String())
	}
	if strings.Join(nomes, " ") != "Lazer:30.0 Sem categoria:20.0 Mercado:10.0" {
		t.Errorf("categorias: %v", nomes)
	}
}

// Regra: dividir por zero onde o divisor devia ser positivo é bug: entra em pânico em vez de devolver um número errado.
func TestDividirPorPositivo(t *testing.T) {
	t.Parallel()
	if v := dividirPorPositivo(dec("10.0"), dec("4")); v.String() != "2.5" {
		t.Errorf("10.0 ÷ 4 = %s", v)
	}
	defer func() {
		if recover() == nil {
			t.Error("dividir por zero não entrou em pânico")
		}
	}()
	dividirPorPositivo(dec("1.0"), tipos.Zero)
}
