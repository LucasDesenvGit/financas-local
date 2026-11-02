package painel

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/carteira"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/contasagendadas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/poucoinformativa"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/reservas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func pTexto(s string) *string { return &s }

func ptr[T any](v T) *T { return &v }

// cenaDoMes monta o painel com um cenário que passa por todos os avisos.
func cenaDoMes(t *testing.T, c composicaoFalsa, r reservasFalsas, cob cobrancasFalsas) (cena, map[string]int64) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 10, 14)
	par := parametrosFalsos{valores: map[string]int{chaveDiasVencimentos: 15, chaveDiasConsentimento: 10,
		poucoinformativa.ChaveMinimoLetras: 3}}
	s := NovoServico(db, c, r, cambioFalso{}, par, cob, rel, slog.New(slog.DiscardHandler))
	mux := http.NewServeMux()
	Registrar(mux, s)
	fab := testeapoio.NovaFabrica(t, db, rel)
	agora := rel.Agora()
	ids := map[string]int64{}

	fab.Conta(func(x *testeapoio.Conta) { x.SaldoInicial = dec("1000.0") })
	fab.Conta(func(x *testeapoio.Conta) { x.Tipo, x.SaldoInicial = tipoContaCartao, dec("-300.0") })
	fab.Conta(func(x *testeapoio.Conta) { x.Tipo, x.SaldoInicial = tipoContaInvestimento, dec("200.0") })
	fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Moeda = "Conta em dólar", "USD" })
	fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Moeda, x.IncluirNoSaldo = "Conta em euro", "EUR", false })
	movimento := fab.Conta(func(x *testeapoio.Conta) { x.IncluirNoSaldo = false })
	transacao := func(d tipos.Data, valor, descricao string, ajustes ...func(*testeapoio.Transacao)) {
		fab.Transacao(movimento, append([]func(*testeapoio.Transacao){func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Descricao = d, dec(valor), descricao
		}}, ajustes...)...)
	}
	transacao(testeapoio.Dia(2026, 10, 3), "500.0", "SALARIO EMPRESA", func(tr *testeapoio.Transacao) { tr.Tipo = 2 })
	transacao(testeapoio.Dia(2026, 10, 4), "-40.0", "HOSPITAL CENTRAL", func(tr *testeapoio.Transacao) { tr.Emergencia = true })
	transacao(testeapoio.Dia(2026, 10, 5), "-999.0", "HOSPITAL IGNORADO", func(tr *testeapoio.Transacao) {
		tr.Emergencia, tr.IgnorarRelatorios = true, true
	})
	transacao(testeapoio.Dia(2026, 10, 1), "-1.0", "PIX ENVIADO")
	transacao(testeapoio.Dia(2026, 10, 2), "-1.0", "pix enviado ")
	transacao(testeapoio.Dia(2026, 9, 30), "-1.0", "TED 123")
	transacao(testeapoio.Dia(2026, 11, 5), "-10.0", "PARCELA FUTURA LOJA")

	fab.ContaAgendada(func(x *testeapoio.ContaAgendada) {
		x.Descricao, x.ProximoVencimento = "Luz", testeapoio.Dia(2026, 10, 10)
	})

	expira := func(dias int) *tipos.DataHora { d := tipos.NovaDataHora(agora.AddDate(0, 0, dias)); return &d }
	a := fab.ConexaoBancaria(func(x *testeapoio.ConexaoBancaria) {
		x.NomeInstituicao, x.ConsentimentoExpiraEm = pTexto("Banco A"), expira(5)
	})
	fab.ConexaoBancaria(func(x *testeapoio.ConexaoBancaria) {
		x.NomeInstituicao, x.ConsentimentoExpiraEm = pTexto("Banco B"), expira(-1)
	})
	fab.ConexaoBancaria(func(x *testeapoio.ConexaoBancaria) {
		x.NomeInstituicao, x.UltimoErro = pTexto("Banco C"), pTexto("falhou")
	})
	fab.ConexaoBancaria(func(x *testeapoio.ConexaoBancaria) {
		x.NomeInstituicao, x.Status = pTexto("Banco D"), pTexto("LOGIN_ERROR")
	})
	fab.ConexaoBancaria(func(x *testeapoio.ConexaoBancaria) {
		x.NomeInstituicao, x.ConsentimentoExpiraEm, x.Ativa = pTexto("Banco E"), expira(-1), false
	})
	fab.ConexaoBancaria(func(x *testeapoio.ConexaoBancaria) {
		x.NomeInstituicao, x.ConsentimentoExpiraEm = pTexto("Banco F"), expira(30)
	})

	fab.Emprestimo(a, func(e *testeapoio.Emprestimo) {
		e.Natureza, e.Produto, e.SaldoDevedor, e.ParcelasEmAtraso = 2, "Carro", dec("50.0"), ptr(int64(2))
	})
	fab.Emprestimo(a, func(e *testeapoio.Emprestimo) {
		e.Produto, e.SaldoDevedor, e.ParcelasEmAtraso = "Quitado", dec("0.0"), ptr(int64(1))
	})
	fab.Emprestimo(a, func(e *testeapoio.Emprestimo) {
		e.Produto, e.SaldoDevedor, e.ParcelasEmAtraso = "Pessoal", dec("5.0"), ptr(int64(1))
	})

	erro := func(nivel int, resolvido bool, dias int) {
		t.Helper()
		ultima := tipos.NovaDataHora(agora.AddDate(0, 0, -dias))
		if _, err := db.Exec(`INSERT INTO "ErroSistema" ("PrimeiraEm", "UltimaEm", "Nivel", "Origem", "Categoria", "Mensagem",
			"Assinatura", "Resolvido") VALUES (?, ?, ?, 'API', 'x', 'm', ?, ?)`, ultima, ultima, nivel,
			"a"+time.Duration(dias).String()+string(rune('0'+nivel)), resolvido); err != nil {
			t.Fatal(err)
		}
	}
	erro(4, false, 1)
	erro(5, false, 2)
	erro(3, false, 1)  // aviso: não conta
	erro(4, true, 1)   // resolvido: não conta
	erro(4, false, 10) // de mais de 7 dias: não conta
	return cena{db, fab, mux, s}, ids
}

func composicaoDoMes() composicaoFalsa {
	return composicaoFalsa{c: carteira.Composicao{Carteira: dec("100.0"), OpenFinanceRendaVariavel: dec("10.0"),
		OpenFinanceDemais: dec("20.0"), Reservas: dec("5.0"), MoedasSemCotacao: []string{"GBP"}},
		creditos: 2, totalCreditos: dec("12.5"), trocos: &carteira.TrocosAutomaticosDto{}}
}

func reservasDoMes() reservasFalsas {
	return reservasFalsas{lista: []reservas.ReservaDto{{Nome: "Caixinha", PodeTerSaldoAnterior: true}, {Nome: "Ok"},
		{Nome: "Velha", ConferenciaAntiga: true}}}
}

func cobrancasDoMes() cobrancasFalsas {
	return cobrancasFalsas{avisos: []contasagendadas.AvisoDto{{Tipo: "nao-veio", Texto: "A cobrança da luz não veio."},
		{Tipo: "valor-mudou", Texto: "fica na tela"}, {Tipo: "cartao-arquivado", Texto: "O cartão X foi arquivado."}}}
}

// Regra (#80, o PainelController): o painel soma o saldo das contas, a dívida dos cartões (só o negativo) e a conta de
// investimento no investido (com a carteira); o patrimônio tira as dívidas; o mês traz as emergências (sem as
// ignoradas), as últimas e as futuras transações e o aviso das descrições pouco informativas (a mais repetida, sem
// diferença de maiúsculas e de espaços, como apareceu primeiro); os avisos vêm na ordem do C#.
func TestPainelDoMes(t *testing.T) {
	t.Parallel()
	c, _ := cenaDoMes(t, composicaoDoMes(), reservasDoMes(), cobrancasDoMes())
	r := testeapoio.Chamar(t, c.mux, "GET", "/api/painel", "")
	var p PainelDto
	if err := json.Unmarshal(r.Corpo, &p); err != nil || r.Status != 200 {
		t.Fatalf("%d %s %v", r.Status, r.Corpo, err)
	}
	igual := func(nome string, a tipos.Decimal, b string) {
		t.Helper()
		if !a.Decimal().Equal(dec(b).Decimal()) {
			t.Errorf("%s: %s, quer %s", nome, a, b)
		}
	}
	igual("saldo", p.SaldoContas, "1000")
	igual("cartões", p.DividaCartoes, "300")
	igual("investido", p.TotalInvestido, "335")
	igual("patrimônio", p.PatrimonioLiquido, "980")
	igual("renda variável", p.RendaVariavel, "110")
	igual("sem renda variável", p.PatrimonioSemRendaVariavel, "870")
	igual("receitas", p.ReceitasMes, "500")
	igual("emergência", p.DespesasEmergenciaMes, "40")
	igual("financiamentos", p.DividaFinanciamentos, "50")
	if p.QuantidadeFuturas != 1 || len(p.TransacoesFuturas) != 1 || len(p.Evolucao) != 12 || p.DiasProximosVencimentos != 15 ||
		p.PoucoInformativas != 3 || p.PoucoInformativasDesde == nil || p.PoucoInformativasDesde.String() != "2026-07-17" ||
		p.ExemploPoucoInformativa == nil || *p.ExemploPoucoInformativa != "pix enviado" || p.CreditosCorretoraComoReceita != 2 ||
		p.TrocosAutomaticos == nil || p.TotalFaturasDoMes.String() != "0" {
		t.Errorf("painel: futuras %d, evolução %d, pouco informativas %d %v %v, créditos %d", p.QuantidadeFuturas,
			len(p.Evolucao), p.PoucoInformativas, p.PoucoInformativasDesde, p.ExemploPoucoInformativa, p.CreditosCorretoraComoReceita)
	}
	quer := []string{
		`Conta "Luz" vencida desde 10/10.`,
		"Investimentos em GBP ficaram fora do patrimônio: não há cotação de câmbio guardada para essa moeda.",
		`A conta Conta em dólar é em USD, mas o site trata as contas como reais, sem conversão: o saldo dela entra no total como se fosse em reais. Desmarque "Incluir no saldo" em Contas e cartões. As transações dela entram nos relatórios do mesmo jeito.`,
		"A conta Conta em euro é em EUR, mas o site trata as contas como reais, sem conversão. As transações dela entram nos relatórios do mesmo jeito.",
		"A cobrança da luz não veio. Veja em Contas a pagar.",
		"O cartão X foi arquivado. Veja em Contas a pagar.",
		"Confira o saldo da(s) reserva(s) Caixinha, Velha em Investimentos › Reservas: o valor estimado pelo extrato pode estar diferente do real.",
		"O consentimento do Open Finance de Banco A expira em 19/10/2026. Renove no Meu Pluggy.",
		"O consentimento do Open Finance de Banco B expirou. Renove no Meu Pluggy.",
		"Falha na última sincronização de Banco C. Veja em Configurações › Open Finance.",
		"Banco D precisa de uma ação sua no Meu Pluggy para voltar a sincronizar.",
		"2 erros do sistema nos últimos 7 dias. Veja em Configurações › Erros.",
		`Financiamento "Carro" com 2 parcela(s) em atraso.`,
		`Empréstimo "Pessoal" com 1 parcela(s) em atraso.`,
	}
	if strings.Join(p.Alertas, "\n") != strings.Join(quer, "\n") {
		t.Errorf("alertas:\n%s\nquer\n%s", strings.Join(p.Alertas, "\n"), strings.Join(quer, "\n"))
	}

	r = testeapoio.Chamar(t, c.mux, "GET", "/api/painel/avisos", "")
	var a AvisosDto
	if err := json.Unmarshal(r.Corpo, &a); err != nil || r.Status != 200 || a.Ano != 2026 || a.Mes != 10 ||
		strings.Join(a.Alertas, "\n") != strings.Join(quer, "\n") || a.PoucoInformativas != 3 {
		t.Errorf("avisos: %d %s", r.Status, r.Corpo)
	}
}

// Regra: o mês do painel vem da query (ou é o atual); fora de 2000..2100 ou de 1..12 é "Período inválido."; o que não
// é número é o 400 automático.
func TestPainelPeriodo(t *testing.T) {
	t.Parallel()
	c := cenaPadraoDoMes(t)
	for _, caminho := range []string{"/api/painel?ano=2000&mes=1", "/api/painel?ano=2100&mes=12", "/api/painel?mes=1"} {
		if r := testeapoio.Chamar(t, c.mux, "GET", caminho, ""); r.Status != 200 {
			t.Errorf("%s: %d %s", caminho, r.Status, r.Corpo)
		}
	}
	for _, caminho := range []string{"/api/painel?ano=1999", "/api/painel?ano=2101", "/api/painel?mes=0", "/api/painel?mes=13"} {
		r := testeapoio.Chamar(t, c.mux, "GET", caminho, "")
		if r.Status != 400 || !strings.Contains(string(r.Corpo), "Período inválido.") {
			t.Errorf("%s: %d %s", caminho, r.Status, r.Corpo)
		}
	}
	if r := testeapoio.Chamar(t, c.mux, "GET", "/api/painel?mes=x", ""); r.Status != 400 || !strings.Contains(string(r.Corpo), "errors") {
		t.Errorf("mês que não é número: %d %s", r.Status, r.Corpo)
	}
}

func cenaPadraoDoMes(t *testing.T) cena {
	c, _ := cenaDoMes(t, composicaoDoMes(), reservasDoMes(), cobrancasDoMes())
	return c
}

// Regra: um erro só do sistema é "1 erro", no singular; sem descrição pouco informativa, o exemplo é null; o exemplo
// longo é cortado em 60 chars (UTF-16, como o C#).
func TestPainelSingularECorte(t *testing.T) {
	t.Parallel()
	if s := cortarUtf16(strings.Repeat("a", 59)+"😀b", 60); s != strings.Repeat("a", 59)+"�" && len([]rune(s)) != 60 {
		t.Errorf("corte no meio do par: %q", s)
	}
	if s := cortarUtf16("curto", 60); s != "curto" {
		t.Errorf("curto: %q", s)
	}
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 10, 14)
	s := NovoServico(db, composicaoFalsa{}, reservasFalsas{}, cambioFalso{}, parametrosFalsos{valores: map[string]int{
		poucoinformativa.ChaveMinimoLetras: 3}}, cobrancasFalsas{}, rel, slog.New(slog.DiscardHandler))
	ultima := tipos.NovaDataHora(rel.Agora())
	if _, err := db.Exec(`INSERT INTO "ErroSistema" ("PrimeiraEm", "UltimaEm", "Nivel", "Origem", "Categoria", "Mensagem",
		"Assinatura") VALUES (?, ?, 4, 'API', 'x', 'm', 'a')`, ultima, ultima); err != nil {
		t.Fatal(err)
	}
	a, err := s.Avisos(t.Context())
	if err != nil || strings.Join(a.Alertas, "|") != "1 erro do sistema nos últimos 7 dias. Veja em Configurações › Erros." ||
		a.ExemploPoucoInformativa != nil || a.PoucoInformativas != 0 {
		t.Errorf("avisos: %+v %v", a, err)
	}
	fab := testeapoio.NovaFabrica(t, db, rel)
	fab.Transacao(fab.Conta(), func(tr *testeapoio.Transacao) { tr.Descricao = "PIX " + strings.Repeat("ENVIADO ", 10) })
	if a, err := s.Avisos(t.Context()); err != nil || a.ExemploPoucoInformativa == nil || len(*a.ExemploPoucoInformativa) != 60 {
		t.Errorf("exemplo longo: %+v %v", a.ExemploPoucoInformativa, err)
	}
}

// Regra: a falha de cada fonte do painel (banco, carteira, reservas, parâmetros, contas a pagar) é 500.
func TestPainelComErros(t *testing.T) {
	t.Parallel()
	falha := errors.New("falha de teste")
	cenas := map[string]cena{}
	c, _ := cenaDoMes(t, composicaoFalsa{err: falha}, reservasDoMes(), cobrancasDoMes())
	cenas["composição"] = c
	c, _ = cenaDoMes(t, composicaoFalsa{errCreditos: falha}, reservasDoMes(), cobrancasDoMes())
	cenas["créditos"] = c
	c, _ = cenaDoMes(t, composicaoFalsa{errTrocos: falha}, reservasDoMes(), cobrancasDoMes())
	cenas["trocos"] = c
	c, _ = cenaDoMes(t, composicaoDoMes(), reservasFalsas{errLista: falha}, cobrancasDoMes())
	cenas["reservas"] = c
	c, _ = cenaDoMes(t, composicaoDoMes(), reservasDoMes(), cobrancasFalsas{err: falha})
	cenas["contas a pagar"] = c
	for nome, c := range cenas {
		if r := testeapoio.Chamar(t, c.mux, "GET", "/api/painel", ""); r.Status != 500 {
			t.Errorf("%s: %d", nome, r.Status)
		}
	}
	for nome, cb := range map[string]cobrancasFalsas{"contas a pagar": {err: falha}} {
		c, _ := cenaDoMes(t, composicaoDoMes(), reservasDoMes(), cb)
		if r := testeapoio.Chamar(t, c.mux, "GET", "/api/painel/avisos", ""); r.Status != 500 {
			t.Errorf("avisos, %s: %d", nome, r.Status)
		}
	}
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 10, 14)
	s := NovoServico(db, composicaoDoMes(), reservasDoMes(), cambioFalso{}, parametrosFalsos{err: falha}, cobrancasDoMes(), rel,
		slog.New(slog.DiscardHandler))
	if _, err := s.Painel(t.Context(), 2026, 10); err == nil {
		t.Error("parâmetros: o erro sumiu")
	}
	if _, err := s.Avisos(t.Context()); err == nil {
		t.Error("avisos, parâmetros: o erro sumiu")
	}
	_ = db.Close()
	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, composicaoDoMes(), reservasDoMes(), cambioFalso{}, parametrosFalsos{}, cobrancasDoMes(), rel,
		slog.New(slog.DiscardHandler)))
	for _, caminho := range []string{"/api/painel", "/api/painel/avisos"} {
		if r := testeapoio.Chamar(t, mux, "GET", caminho, ""); r.Status != 500 {
			t.Errorf("banco fechado, %s: %d", caminho, r.Status)
		}
	}
}

// Regra (#206): o card mostra os 6 primeiros orçamentos do mês e os estourados (acima de 100%) do pior para o melhor;
// as faturas do mês são as dos cartões com gasto no mês ou com fatura, e o total soma as que aparecem.
func TestPainelOrcamentosEFaturas(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 10, 14)
	s := NovoServico(db, composicaoDoMes(), reservasFalsas{}, cambioFalso{}, parametrosFalsos{}, cobrancasFalsas{}, rel,
		slog.New(slog.DiscardHandler))
	fab := testeapoio.NovaFabrica(t, db, rel)
	conta := fab.Conta()
	gastos := []string{"5.0", "15.0", "30.0", "1.0", "1.0", "1.0", "100.0"} // de orçamentos de 10.0
	for i, g := range gastos {
		cat := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Cat " + string(rune('A'+i)) })
		fab.Orcamento(cat, func(o *testeapoio.Orcamento) { o.Valor = dec("10.0") })
		fab.Transacao(conta, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.CategoriaId = testeapoio.Dia(2026, 10, 2), dec("-"+g), &cat.Id
		})
	}
	comFatura := fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Tipo = "Com fatura", tipoContaCartao })
	fab.Fatura(comFatura, testeapoio.Dia(2026, 10, 20), dec("0.0"))
	comGasto := fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Tipo = "Com gasto", tipoContaCartao })
	fab.Transacao(comGasto, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor = testeapoio.Dia(2026, 10, 3), dec("-25.0") })
	fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Tipo = "Parado", tipoContaCartao })

	p, err := s.Painel(t.Context(), 2026, 10)
	if err != nil {
		t.Fatal(err)
	}
	nomes := func(l []relatorios.OrcamentoDto) string {
		var n []string
		for _, o := range l {
			n = append(n, o.CategoriaNome)
		}
		return strings.Join(n, ",")
	}
	if len(p.Orcamentos) != 6 || nomes(p.OrcamentosEstourados) != "Cat G,Cat C,Cat B" {
		t.Errorf("orçamentos: %d, estourados %s", len(p.Orcamentos), nomes(p.OrcamentosEstourados))
	}
	var cartoes []string
	for _, f := range p.FaturasDoMes {
		cartoes = append(cartoes, f.Cartao+"="+f.ValorMes.String())
	}
	if strings.Join(cartoes, ",") != "Com gasto=25.0,Com fatura=0" || !p.TotalFaturasDoMes.Decimal().Equal(dec("25").Decimal()) {
		t.Errorf("faturas: %v, total %s", cartoes, p.TotalFaturasDoMes)
	}
}

// Regra: a falha de qualquer consulta do painel e dos avisos é 500 (aqui, uma tabela que sumiu por vez).
func TestPainelSemTabela(t *testing.T) {
	t.Parallel()
	for _, tabela := range []string{"Conta", "Transacao", "Emprestimo", "ConexaoBancaria", "ErroSistema", "FaturaCartao",
		"Investimento", "Orcamento", "Categoria", "ContaAgendada"} {
		t.Run(tabela, func(t *testing.T) {
			t.Parallel()
			c := cenaPadraoDoMes(t)
			if _, err := c.db.Exec(`ALTER TABLE "` + tabela + `" RENAME TO "Sumiu"`); err != nil {
				t.Fatal(err)
			}
			if r := testeapoio.Chamar(t, c.mux, "GET", "/api/painel", ""); r.Status != 500 {
				t.Errorf("painel: %d", r.Status)
			}
			testeapoio.Chamar(t, c.mux, "GET", "/api/painel/avisos", "")
		})
	}
}
