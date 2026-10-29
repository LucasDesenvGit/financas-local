package transferencias

import (
	"database/sql"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

func p[T any](v T) *T { return &v }

func dec(s string) tipos.Decimal {
	d, err := tipos.NovoDecimal(s)
	if err != nil {
		panic(err)
	}
	return d
}

// Regra: PalavrasDoNome (#172): primeiro e último nome, sem conectivos e sem acento; um nome só não vale.
func TestPalavrasDoNome(t *testing.T) {
	casos := []struct {
		nome     *string
		esperado []string
	}{
		{p("Lucas da Silva"), []string{"LUCAS", "SILVA"}},
		{p("  joão  de   souza e lima "), []string{"JOAO", "LIMA"}},
		{p("Maria-José Ávila"), []string{"MARIA", "AVILA"}},
		{p("Ana"), nil},
		{p("Ana d"), nil},
		{nil, nil},
	}
	for _, c := range casos {
		got := PalavrasDoNome(c.nome)
		if len(got) != len(c.esperado) || (len(got) == 2 && (got[0] != c.esperado[0] || got[1] != c.esperado[1])) {
			t.Errorf("PalavrasDoNome(%v) = %q, esperado %q", c.nome, got, c.esperado)
		}
	}
}

// Regra: CitaONome: as duas palavras inteiras, em qualquer pontuação.
func TestCitaONome(t *testing.T) {
	nome := []string{"LUCAS", "SILVA"}
	casos := []struct {
		descricao string
		esperado  bool
	}{
		{"PIX RECEBIDO LUCAS F SILVA", true},
		{"Pix-lucas.silva", true},
		{"PIX LUCASSILVA", false},
		{"PIX LUCAS SOUZA", false},
		{"PIX LUCAS😀SILVA", true},
	}
	for _, c := range casos {
		if got := CitaONome(&c.descricao, nome); got != c.esperado {
			t.Errorf("CitaONome(%q) = %v, esperado %v", c.descricao, got, c.esperado)
		}
	}
}

// Regra: EntreContasSuas: lançada pelo site, o titular informado (vale sempre), o par, a reserva e o seu nome.
func TestEntreContasSuas(t *testing.T) {
	grupo := tipos.NovoGuid()
	casos := []struct {
		nome   string
		t      transacoes.Transacao
		temPar bool
		meu    []string
		sim    bool
	}{
		{"lançada pelo site", transacoes.Transacao{TransferenciaGrupo: &grupo, MesmaTitularidade: p(false)}, false, nil, true},
		{"outro titular nunca é, mesmo com par", transacoes.Transacao{MesmaTitularidade: p(false), Descricao: "x"}, true, nil, false},
		{"mesmo titular", transacoes.Transacao{MesmaTitularidade: p(true), Descricao: "x"}, false, nil, true},
		{"com par", transacoes.Transacao{Descricao: "TED"}, true, nil, true},
		{"reserva", transacoes.Transacao{Descricao: "dinheiro RESERVADO Caixinha"}, false, nil, true},
		{"reserva só no começo", transacoes.Transacao{Descricao: "PIX Dinheiro reservado "}, false, nil, false},
		{"cita o seu nome", transacoes.Transacao{Descricao: "PIX LUCAS F SILVA"}, false, []string{"LUCAS", "SILVA"}, true},
		{"sem nada", transacoes.Transacao{Descricao: "PIX JOAO"}, false, []string{"LUCAS", "SILVA"}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			tr := c.t
			if got := EntreContasSuas(&tr, c.temPar, c.meu); got != c.sim {
				t.Errorf("EntreContasSuas = %v, esperado %v", got, c.sim)
			}
		})
	}
}

// cenario: conta corrente, poupança e cartão, as categorias de transferência e o relógio.
type cenario struct {
	db                              *sql.DB
	fab                             *testeapoio.Fabrica
	corrente, poupanca, cartao      testeapoio.Conta
	transferencias, pagamentoFatura int64
	agora                           tipos.DataHora
}

func novoCenario(t *testing.T) cenario {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	return cenario{
		db: db, fab: fab,
		corrente: fab.Conta(), poupanca: fab.Conta(func(c *testeapoio.Conta) { c.Tipo = 2 }),
		cartao:          fab.Conta(func(c *testeapoio.Conta) { c.Tipo = 3 }),
		transferencias:  fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Transferências", 3 }).Id,
		pagamentoFatura: fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Pagamento de fatura", 3 }).Id,
		agora:           tipos.NovaDataHora(time.Date(2026, 10, 15, 15, 0, 0, 0, time.UTC)),
	}
}

func (c cenario) lancar(conta testeapoio.Conta, valor, descricao string, dia int, categoria *int64, tipo int64, ignorar bool) int64 {
	return c.fab.Transacao(conta, func(tr *testeapoio.Transacao) {
		tr.Valor, tr.Descricao, tr.DescricaoOriginal = dec(valor), descricao, p(descricao)
		tr.Data, tr.CategoriaId, tr.Tipo, tr.IgnorarRelatorios = tipos.NovaData(2026, 10, dia), categoria, tipo, ignorar
	}).Id
}

func (c cenario) estado(t *testing.T, id int64) (tipo int64, ignorar bool, categoria *int64) {
	t.Helper()
	if err := c.db.QueryRow(`SELECT Tipo, IgnorarRelatorios, CategoriaId FROM Transacao WHERE Id = ?`, id).Scan(&tipo, &ignorar, &categoria); err != nil {
		t.Fatal(err)
	}
	return
}

// Regra: Reclassificar (ADR 0031): na categoria genérica, com par (mesmo valor, sinal oposto, outra conta, até 5
// dias) fica transferência; sem par, volta a despesa ou receita; o "ignorado à mão" depois de reclassificado fica.
func TestReclassificarPelosPares(t *testing.T) {
	c := novoCenario(t)
	cat := &c.transferencias
	saida := c.lancar(c.corrente, "-500.00", "TED PARA POUPANCA", 10, cat, transacoes.Despesa, false)
	entrada := c.lancar(c.poupanca, "500.0", "TED RECEBIDA", 14, nil, transacoes.Receita, false) // 4 dias depois, escala diferente
	semPar := c.lancar(c.corrente, "-80", "TED JOAO", 10, cat, transacoes.Transferencia, true)
	longe := c.lancar(c.corrente, "-300", "TED LONGE", 1, cat, transacoes.Transferencia, true)
	c.lancar(c.poupanca, "300", "TED LONGE VOLTA", 7, nil, transacoes.Receita, false) // 6 dias: longe demais
	mesmaConta := c.lancar(c.corrente, "-70", "TED MESMA CONTA", 10, cat, transacoes.Transferencia, true)
	c.lancar(c.corrente, "70", "VOLTA NA MESMA CONTA", 10, nil, transacoes.Receita, false)
	jaDespesa := c.lancar(c.corrente, "-40", "TED IGNORADA", 10, cat, transacoes.Despesa, true)

	var mudaram int
	if err := bancoEm(c.db, func(tx *sql.Tx) (err error) { mudaram, err = Reclassificar(t.Context(), tx, c.agora); return }); err != nil {
		t.Fatal(err)
	}
	conferir := func(nome string, id, tipo int64, ignorar bool) {
		gotTipo, gotIgnorar, _ := c.estado(t, id)
		if gotTipo != tipo || gotIgnorar != ignorar {
			t.Errorf("%s: tipo %d, ignorar %v; esperado %d, %v", nome, gotTipo, gotIgnorar, tipo, ignorar)
		}
	}
	conferir("com par a 4 dias (escala diferente)", saida, transacoes.Transferencia, true)
	conferir("o outro lado não é da categoria: não muda", entrada, transacoes.Receita, false)
	conferir("sem par volta a despesa", semPar, transacoes.Despesa, false)
	conferir("par a 6 dias não vale", longe, transacoes.Despesa, false)
	conferir("par na mesma conta não vale", mesmaConta, transacoes.Despesa, false)
	conferir("ignorada à mão continua ignorada", jaDespesa, transacoes.Despesa, true)
	if mudaram != 4 {
		t.Errorf("mudaram %d, esperado 4", mudaram)
	}
}

// Regra: pagamento de fatura pela conta (#172): débito com texto de fatura e o crédito do mesmo valor num cartão, até
// 5 dias, vira transferência na categoria "Pagamento de fatura"; sem o crédito, fica (conta de luz também é fatura).
func TestPagamentoDeFaturaPareadoNoCartao(t *testing.T) {
	c := novoCenario(t)
	pago := c.lancar(c.corrente, "-1234.56", "PAGAMENTO FATURA CARTAO", 10, nil, transacoes.Despesa, false)
	c.lancar(c.cartao, "1234.560", "PAGAMENTO RECEBIDO", 12, &c.pagamentoFatura, transacoes.Transferencia, true)
	semCredito := c.lancar(c.corrente, "-99.90", "PAGTO FATURA ENERGIA", 10, nil, transacoes.Despesa, false)
	if err := bancoEm(c.db, func(tx *sql.Tx) error { _, err := Reclassificar(t.Context(), tx, c.agora); return err }); err != nil {
		t.Fatal(err)
	}
	if tipo, ignorar, categoria := c.estado(t, pago); tipo != transacoes.Transferencia || !ignorar || categoria == nil || *categoria != c.pagamentoFatura {
		t.Errorf("pago: tipo %d, ignorar %v, categoria %v", tipo, ignorar, categoria)
	}
	if tipo, ignorar, categoria := c.estado(t, semCredito); tipo != transacoes.Despesa || ignorar || categoria != nil {
		t.Errorf("sem o crédito no cartão: tipo %d, ignorar %v, categoria %v", tipo, ignorar, categoria)
	}
}

// Regra: o seu nome nos Pix (#172) vale só sem titular informado.
func TestReclassificarPeloNome(t *testing.T) {
	c := novoCenario(t)
	c.fab.Configuracao(ChaveMeuNome, "Lucas da Silva")
	cat := &c.transferencias
	meu := c.lancar(c.corrente, "-50", "PIX ENVIADO LUCAS SILVA", 10, cat, transacoes.Despesa, false)
	if err := bancoEm(c.db, func(tx *sql.Tx) error { _, err := Reclassificar(t.Context(), tx, c.agora); return err }); err != nil {
		t.Fatal(err)
	}
	if tipo, ignorar, _ := c.estado(t, meu); tipo != transacoes.Transferencia || !ignorar {
		t.Errorf("Pix com o seu nome: tipo %d, ignorar %v", tipo, ignorar)
	}
}

func bancoEm(db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
