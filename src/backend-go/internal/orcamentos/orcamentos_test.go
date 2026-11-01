package orcamentos

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/parametros"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func ambiente(t *testing.T) (*sql.DB, *testeapoio.Fabrica, *http.ServeMux) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 14)
	log := slog.New(slog.DiscardHandler)
	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, parametros.Novo(parametros.Opcoes{DB: db, Relogio: r, Log: log}), r, log))
	return db, testeapoio.NovaFabrica(t, db, r), mux
}

func lerJSON[T any](t *testing.T, corpo []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(corpo, &v); err != nil {
		t.Fatalf("%v: %s", err, corpo)
	}
	return v
}

// Regra: os textos em pt-BR do .NET: o ToString("C0") (R$, espaço comum, milhar com ponto, meio para longe do
// zero), o mês "MMM/yy" sem o ponto e o {x:0}.
func TestFormatos(t *testing.T) {
	t.Parallel()
	for entrada, quer := range map[string]string{
		"1234.5": "R$ 1.235", "999.5": "R$ 1.000", "12": "R$ 12", "1234567.49": "R$ 1.234.567",
		"0.4": "R$ 0", "-1500": "-R$ 1.500",
	} {
		if obtido := reais(dec(entrada)); obtido != quer {
			t.Errorf("reais(%s) = %q, quer %q", entrada, obtido, quer)
		}
	}
	inicio := testeapoio.Dia(2026, 4, 1)
	for indice, quer := range map[int]string{0: "abr/26", 3: "jul/26", 8: "dez/26", 9: "jan/27", -1: "abr/26"} {
		if obtido := nomeMes(inicio, indice); obtido != quer {
			t.Errorf("nomeMes(%d) = %q", indice, obtido)
		}
	}
	for entrada, quer := range map[string]string{"2.5": "3", "5.00": "5", "0.4": "0", "12.49": "12"} {
		if obtido := inteiro(dec(entrada)); obtido != quer {
			t.Errorf("inteiro(%s) = %q", entrada, obtido)
		}
	}
}

// Regra (#39, passos 1 e 2): a base é a mediana (ou a média, com gasto irregular), com o pico, a tendência e as
// emergências nos motivos, e a folga nos essenciais; com gasto em menos de 2 meses, não sugere.
func TestCalcularBase(t *testing.T) {
	t.Parallel()
	inicio := testeapoio.Dia(2026, 4, 1)
	essencial, estilo := int64(naturezaEssencial), int64(naturezaEstiloVida)
	casos := []struct {
		nome       string
		meses      []string
		natureza   *int64
		emergencia string
		valor      string // "" = sem sugestão
		tendencia  string
		motivos    []string
	}{
		{"estável", []string{"100", "100", "100", "100", "100", "100"}, &estilo, "0", "100", "estavel",
			[]string{"mediana de 6 meses"}},
		{"essencial com folga", []string{"100", "100", "100", "100", "100", "100"}, &essencial, "0", "105", "estavel",
			[]string{"mediana de 6 meses", "+5% de folga (essencial)"}},
		{"pico e alta", []string{"100", "100", "100", "100", "100", "300"}, &estilo, "0", "100", "alta",
			[]string{"mediana de 6 meses", "pico em set/26 (R$ 300) fora da base", "tendência de alta nos últimos 3 meses (não aumenta o orçamento)"}},
		{"queda", []string{"200", "200", "200", "100", "100", "100"}, nil, "0", "150", "queda",
			[]string{"mediana de 6 meses"}},
		{"irregular usa a média", []string{"0", "0", "0", "0", "300", "300"}, nil, "50", "100", "alta",
			[]string{"gasto irregular (2 de 6 meses): usa a média mensal", "tendência de alta nos últimos 3 meses (não aumenta o orçamento)",
				"sem as emergências (R$ 50)"}},
		{"um mês só", []string{"0", "0", "0", "80", "0", "0"}, nil, "0", "", "",
			[]string{"gasto em só 1 dos últimos 6 meses (jul/26): sem dados suficientes"}},
		{"nenhum mês", []string{"0", "0", "0", "0", "0", "0"}, nil, "0", "", "", nil},
	}
	for _, c := range casos {
		l := &linha{natureza: c.natureza, emergencias: dec(c.emergencia)}
		for _, m := range c.meses {
			l.meses = append(l.meses, dec(m))
		}
		calcularBase(l, len(l.meses), inicio, decimal.Zero, dec("0.05"))
		valor := ""
		if l.valor != nil {
			valor = l.valor.String()
		}
		tendencia := ""
		if l.tendencia != nil {
			tendencia = *l.tendencia
		}
		if valor != c.valor || tendencia != c.tendencia || strings.Join(l.motivos, "|") != strings.Join(c.motivos, "|") {
			t.Errorf("%s: valor %q, tendência %q, motivos %q", c.nome, valor, tendencia, l.motivos)
		}
	}
}

// Regra (#39, passo 4): o corte é proporcional, até o máximo do grupo, e devolve o excesso que sobrou.
func TestCortar(t *testing.T) {
	t.Parallel()
	novo := func(v string) *linha { x := dec(v); return &linha{valor: &x} }
	a, b := novo("300"), novo("100")
	if sobra := cortar([]*linha{a, b}, dec("100"), corteMaximoEstiloDeVida); !sobra.IsZero() ||
		a.valor.String() != "225" || b.valor.String() != "75" || a.motivos[0] != "−25% para caber na renda" {
		t.Errorf("corte de 25%%: %s %s %v %s", a.valor, b.valor, a.motivos, sobra)
	}
	c := novo("100")
	if sobra := cortar([]*linha{c}, dec("80"), corteMaximoSemNatureza); sobra.String() != "50" || c.valor.String() != "70" {
		t.Errorf("no máximo de 30%%: %s %s", c.valor, sobra)
	}
	if sobra := cortar(nil, dec("10"), corteMaximoSemNatureza); sobra.String() != "10" {
		t.Error("grupo vazio")
	}
	if sobra := cortar([]*linha{novo("100")}, dec("-5"), corteMaximoSemNatureza); sobra.String() != "-5" {
		t.Error("sem excesso")
	}
}

// Regra: salvar cria ou altera o do mês, só para categoria de despesa; aplicar recusa a repetida e a de receita;
// copiar não sobrescreve e o anterior de janeiro é dezembro; o período é conferido.
func TestRotas(t *testing.T) {
	t.Parallel()
	db, fab, mux := ambiente(t)
	mercado := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Mercado" })
	lazer := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Água" })
	salario := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Salário", 2 })
	id := func(n int64) string { b, _ := json.Marshal(n); return string(b) }
	casos := []struct {
		metodo, caminho, corpo string
		status                 int
		mensagem               string
	}{
		{"GET", "/api/orcamentos?ano=1999", "", 400, "Período inválido."},
		{"GET", "/api/orcamentos/sugestao?mes=0", "", 400, "Período inválido."},
		{"GET", "/api/orcamentos?mes=x", "", 400, ""},
		{"PUT", "/api/orcamentos", `{"categoriaId":` + id(mercado.Id) + `,"ano":2026,"mes":10,"valor":500}`, 204, ""},
		{"PUT", "/api/orcamentos", `{"categoriaId":` + id(mercado.Id) + `,"ano":2026,"mes":10,"valor":600.5}`, 204, ""},
		{"PUT", "/api/orcamentos", `{"categoriaId":` + id(lazer.Id) + `,"ano":2026,"mes":12,"valor":80}`, 204, ""},
		{"PUT", "/api/orcamentos", `{"categoriaId":` + id(salario.Id) + `,"ano":2026,"mes":10,"valor":1}`, 400,
			"Orçamentos são definidos para categorias de despesa."},
		{"PUT", "/api/orcamentos", `{"categoriaId":999,"ano":2026,"mes":10,"valor":1}`, 400, "Categoria inválida."},
		{"POST", "/api/orcamentos/aplicar-sugestao", `{"ano":2026,"mes":10,"itens":[{"categoriaId":` + id(mercado.Id) +
			`,"valor":1},{"categoriaId":` + id(mercado.Id) + `,"valor":2}]}`, 400, "Categoria repetida na sugestão."},
		{"POST", "/api/orcamentos/aplicar-sugestao", `{"ano":2026,"mes":10,"itens":[{"categoriaId":` + id(salario.Id) +
			`,"valor":1}]}`, 400, "Orçamentos são definidos para categorias de despesa."},
		{"POST", "/api/orcamentos/aplicar-sugestao", `{"ano":2026,"mes":10,"itens":[{"categoriaId":999,"valor":1}]}`, 400,
			"Orçamentos são definidos para categorias de despesa."},
		{"POST", "/api/orcamentos/aplicar-sugestao", `{"ano":2026,"mes":10,"itens":[{"categoriaId":` + id(mercado.Id) +
			`,"valor":610},{"categoriaId":` + id(lazer.Id) + `,"valor":90}]}`, 200, ""},
		{"DELETE", "/api/orcamentos/999", "", 404, "Registro não encontrado."},
		{"DELETE", "/api/orcamentos/x", "", 404, ""},
	}
	for _, c := range casos {
		r := testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo)
		if r.Status != c.status {
			t.Errorf("%s %s: %d %s", c.metodo, c.caminho, r.Status, r.Corpo)
			continue
		}
		if c.mensagem != "" {
			if m := lerJSON[map[string]string](t, r.Corpo)["mensagem"]; m != c.mensagem {
				t.Errorf("%s %s: %q", c.metodo, c.caminho, m)
			}
		}
	}
	// Outubro: Mercado 610 e Água 90 (aplicados). Copiar para novembro copia os dois; de novo, nada.
	for i, quer := range []int{2, 0} {
		r := testeapoio.Chamar(t, mux, "POST", "/api/orcamentos/copiar-mes-anterior", `{"ano":2026,"mes":11}`)
		if n := lerJSON[ResultadoDto](t, r.Corpo).TransacoesAtualizadas; n != quer {
			t.Errorf("copiar %d: %d", i, n)
		}
	}
	// Janeiro de 2027 copia o de dezembro de 2026 (Água 80).
	if n := lerJSON[ResultadoDto](t, testeapoio.Chamar(t, mux, "POST", "/api/orcamentos/copiar-mes-anterior",
		`{"ano":2027,"mes":1}`).Corpo).TransacoesAtualizadas; n != 1 {
		t.Errorf("copiar para janeiro: %d", n)
	}
	// A lista de novembro, pelo nome na ordem pt-BR (Água antes de Mercado).
	lista := lerJSON[[]map[string]any](t, testeapoio.Chamar(t, mux, "GET", "/api/orcamentos?ano=2026&mes=11", "").Corpo)
	if len(lista) != 2 || lista[0]["categoriaNome"] != "Água" || lista[1]["categoriaNome"] != "Mercado" || lista[1]["valor"] != 610.0 {
		t.Errorf("novembro: %v", lista)
	}
	var orcamento int64
	_ = db.QueryRow(`SELECT "Id" FROM "Orcamento" WHERE "Ano" = 2027`).Scan(&orcamento)
	if r := testeapoio.Chamar(t, mux, "DELETE", "/api/orcamentos/"+id(orcamento), ""); r.Status != 204 {
		t.Errorf("excluir: %d", r.Status)
	}
}

// Regra (#39): a sugestão de ponta a ponta: renda das receitas, o essencial com folga, o estilo de vida pago no
// cartão cortado pelo uso do limite (com o alerta), o encaixe na renda e o arredondamento para R$ 10.
func TestSugerir(t *testing.T) {
	t.Parallel()
	_, fab, mux := ambiente(t)
	corrente := fab.Conta()
	limite := testeapoio.Valor("1000.0")
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Tipo, c.Nome, c.LimiteCredito = tipoCartao, "Cartão", &limite })
	essencial, estilo := int64(naturezaEssencial), int64(naturezaEstiloVida)
	mercado := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Natureza = "Mercado", &essencial })
	lazer := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Natureza = "Lazer", &estilo })
	salario := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Salário", 2 })
	for m := time.Month(1); m <= 9; m++ {
		dia := testeapoio.Dia(2026, m, 5)
		fab.Transacao(corrente, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Tipo, tr.CategoriaId = dia, testeapoio.Valor("2000.0"), 2, &salario.Id
		})
		fab.Transacao(corrente, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.CategoriaId = dia, testeapoio.Valor("-800.0"), &mercado.Id
		})
		fab.Transacao(cartao, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.CategoriaId = dia, testeapoio.Valor("-1500.0"), &lazer.Id
		})
	}
	fab.Fatura(cartao, testeapoio.Dia(2026, 10, 10), testeapoio.Valor("-800.0"))
	fab.Configuracao("Orcamento.MetaPoupanca", "10")

	s := lerJSON[SugestaoDto](t, testeapoio.Chamar(t, mux, "GET", "/api/orcamentos/sugestao?ano=2026&mes=11", "").Corpo)
	// Renda 2000 (18000 ÷ 9 meses), poupança 10%: disponível 1800. Mercado: 800 + 5% = 840. Lazer 1500 − 10% do
	// cartão (uso de 80%, alerta) = 1350, cortado até caber: 1800 − 840 = 960, e arredondado para baixo.
	if s.RendaMedia.String() != "2000.0" || s.Disponivel.String() != "1800.0" || s.NivelCartao == nil || *s.NivelCartao != "alerta" ||
		s.UsoLimiteCartoes.String() != "80.0" || len(s.Cartoes) != 1 {
		t.Errorf("resumo: renda %s, disponível %s, nível %q, uso %v", s.RendaMedia, s.Disponivel, *s.NivelCartao, s.UsoLimiteCartoes)
	}
	itens := map[string]ItemSugestaoDto{}
	for _, i := range s.Itens {
		itens[i.Nome] = i
	}
	if m := itens["Mercado"]; m.Sugerido == nil || m.Sugerido.String() != "840" || !m.Selecionado {
		t.Errorf("mercado: %+v", m)
	}
	if l := itens["Lazer"]; l.Sugerido == nil || l.Sugerido.String() != "960" || l.PagoNoCartaoPercentual.String() != "100" ||
		len(l.Motivos) != 3 || l.Motivos[1] != "−10%: pago no cartão, com 80% do limite em uso" {
		t.Errorf("lazer: %+v", l)
	}
	if s.TotalSugerido.String() != "1800" || s.FaltaParaCaber != nil || len(s.Avisos) != 1 ||
		!strings.HasPrefix(s.Avisos[0], "Seus cartões estão usando 80% do limite (acima de 70%).") {
		t.Errorf("total %s, falta %v, avisos %q", s.TotalSugerido, s.FaltaParaCaber, s.Avisos)
	}

	// Com o Mercado já orçado em 2000 no mês, ele fica fixo e o estilo de vida não basta: falta para caber.
	fab.Orcamento(mercado, func(o *testeapoio.Orcamento) { o.Ano, o.Mes, o.Valor = 2026, 11, testeapoio.Valor("2000.0") })
	s = lerJSON[SugestaoDto](t, testeapoio.Chamar(t, mux, "GET", "/api/orcamentos/sugestao?ano=2026&mes=11", "").Corpo)
	if s.FaltaParaCaber == nil || len(s.Avisos) != 2 || !strings.Contains(s.Avisos[1], "faltam R$ ") {
		t.Errorf("sem caber: falta %v, avisos %q", s.FaltaParaCaber, s.Avisos)
	}
}
