package transferencias

import (
	"database/sql"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

func (c cenario) reclassificar(t *testing.T) int {
	t.Helper()
	var mudaram int
	if err := bancoEm(c.db, func(tx *sql.Tx) (err error) { mudaram, err = Reclassificar(t.Context(), tx, c.agora); return }); err != nil {
		t.Fatal(err)
	}
	return mudaram
}

func (c cenario) conferirTipo(t *testing.T, nome string, id, tipo int64, ignorar bool) {
	t.Helper()
	gotTipo, gotIgnorar, _ := c.estado(t, id)
	if gotTipo != tipo || gotIgnorar != ignorar {
		t.Errorf("%s: tipo %d, ignorar %v; esperado %d, %v", nome, gotTipo, gotIgnorar, tipo, ignorar)
	}
}

// Regra: cada lançamento (por data e Id) forma par com o lado oposto de data mais próxima, mesmo que ele seja de antes;
// o par usado não vale de novo. Com a escolha errada (o mais longe), o segundo lançamento ficaria sem par.
func TestParDeDataMaisProxima(t *testing.T) {
	c := novoCenario(t)
	cat := &c.transferencias
	a := c.lancar(c.corrente, "-100", "TED A", 10, cat, transacoes.Despesa, false)
	c.lancar(c.poupanca, "100", "VOLTA ANTES", 9, nil, transacoes.Receita, false) // 1 dia antes de A
	c.lancar(c.poupanca, "100", "VOLTA DEPOIS", 13, nil, transacoes.Receita, false)
	d := c.lancar(c.corrente, "-100", "TED D", 15, cat, transacoes.Despesa, false) // só alcança a de 13 (a de 9 está a 6 dias)
	if got := c.reclassificar(t); got != 2 {
		t.Errorf("mudaram %d, esperado 2", got)
	}
	c.conferirTipo(t, "A com a de 1 dia antes", a, transacoes.Transferencia, true)
	c.conferirTipo(t, "D com a que sobrou", d, transacoes.Transferencia, true)
}

// Regra: na mesma distância, o de menor Id.
func TestParEmpatadoFicaComOMenorId(t *testing.T) {
	c := novoCenario(t)
	cat := &c.transferencias
	a := c.lancar(c.corrente, "-50", "TED A", 10, cat, transacoes.Despesa, false)
	c.lancar(c.poupanca, "50", "MENOR ID", 9, nil, transacoes.Receita, false)
	c.lancar(c.poupanca, "50", "MAIOR ID", 11, nil, transacoes.Receita, false)
	e := c.lancar(c.corrente, "-50", "TED E", 15, cat, transacoes.Despesa, false) // alcança só a de 11
	c.reclassificar(t)
	c.conferirTipo(t, "A", a, transacoes.Transferencia, true)
	c.conferirTipo(t, "E com a de maior Id, que sobrou", e, transacoes.Transferencia, true)
}

// Regra: a janela vai até 5 dias antes do primeiro lançamento (inclusive); valor zero não forma par; o que já é
// transferência fora dos relatórios e continua entre contas suas não conta como mudança; um lançamento que não muda
// não para os seguintes.
func TestJanelaZeroEContagem(t *testing.T) {
	c := novoCenario(t)
	cat := &c.transferencias
	semMudar := c.lancar(c.corrente, "-30", "JA DESPESA", 1, cat, transacoes.Despesa, false) // sem par, não é transferência
	limite := c.lancar(c.corrente, "-70", "TED LIMITE", 10, cat, transacoes.Despesa, false)
	c.lancar(c.poupanca, "70", "VOLTA 5 DIAS ANTES", 5, nil, transacoes.Receita, false)
	jaCerta := c.lancar(c.corrente, "-20", "JA CERTA", 12, cat, transacoes.Transferencia, true)
	c.lancar(c.poupanca, "20", "PAR DA CERTA", 12, nil, transacoes.Receita, false)
	zero := c.lancar(c.corrente, "0.00", "ZERO", 13, cat, transacoes.Transferencia, true)
	c.lancar(c.poupanca, "0", "OUTRO ZERO", 13, nil, transacoes.Receita, false)
	depois := c.lancar(c.corrente, "-40", "SEM PAR DEPOIS", 14, cat, transacoes.Transferencia, true)
	if got := c.reclassificar(t); got != 3 {
		t.Errorf("mudaram %d, esperado 3 (limite, zero e o de depois)", got)
	}
	c.conferirTipo(t, "sem par e já despesa", semMudar, transacoes.Despesa, false)
	c.conferirTipo(t, "par a 5 dias antes do primeiro", limite, transacoes.Transferencia, true)
	c.conferirTipo(t, "já certa", jaCerta, transacoes.Transferencia, true)
	c.conferirTipo(t, "zero não forma par: volta a receita", zero, transacoes.Receita, false)
	c.conferirTipo(t, "o de depois também é visto", depois, transacoes.Despesa, false)
}

// Regra: o pagamento de fatura (#172) pega o crédito no cartão de data mais próxima; os pagamentos vão por data, cada
// crédito serve a um só; até 5 dias (inclusive); débito com "cartão" que não é fatura não entra; com duas categorias
// "Pagamento de fatura", vale a de menor Id.
func TestPagamentosDeFaturaPareados(t *testing.T) {
	c := novoCenario(t)
	c.fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Pagamento de Fatura", 3 }) // a segunda: não é a usada
	credito := func(valor string, dia int) {
		c.lancar(c.cartao, valor, "PAGAMENTO RECEBIDO", dia, &c.pagamentoFatura, transacoes.Transferencia, true)
	}
	p1 := c.lancar(c.corrente, "-100", "PAGAMENTO FATURA", 10, nil, transacoes.Despesa, false)
	p2 := c.lancar(c.corrente, "-100", "PAGAMENTO FATURA", 15, nil, transacoes.Despesa, false)
	credito("100", 9)                                                                        // 1 dia de P1, 6 de P2
	credito("100", 12)                                                                       // 2 de P1, 3 de P2
	tarde := c.lancar(c.corrente, "-300", "PGTO FATURA", 20, nil, transacoes.Despesa, false) // criada antes, data depois
	cedo := c.lancar(c.corrente, "-300", "PGTO FATURA", 18, nil, transacoes.Despesa, false)
	credito("300", 19) // 1 dia de cada: fica com o de data menor
	limite := c.lancar(c.corrente, "-40", "PAG FATURA CARTAO", 1, nil, transacoes.Despesa, false)
	credito("40", 6) // 5 dias depois: vale
	longe := c.lancar(c.corrente, "-45", "PAG FATURA CARTAO", 1, nil, transacoes.Despesa, false)
	credito("45", 7) // 6 dias: não vale
	presente := c.lancar(c.corrente, "-60", "CARTAO PRESENTE", 25, nil, transacoes.Despesa, false)
	credito("60", 25)
	if got := c.reclassificar(t); got != 4 {
		t.Errorf("mudaram %d, esperado 4", got)
	}
	pago := func(nome string, id int64, esperado bool) {
		t.Helper()
		tipo, _, categoria := c.estado(t, id)
		if got := tipo == transacoes.Transferencia && categoria != nil && *categoria == c.pagamentoFatura; got != esperado {
			t.Errorf("%s: pago %v (tipo %d, categoria %v); esperado %v", nome, got, tipo, categoria, esperado)
		}
	}
	pago("P1 com o crédito de 1 dia", p1, true)
	pago("P2 com o que sobrou, a 3 dias", p2, true)
	pago("o de data menor fica com o crédito", cedo, true)
	pago("o outro fica sem", tarde, false)
	pago("5 dias vale", limite, true)
	pago("6 dias não vale", longe, false)
	pago("cartão presente não é fatura", presente, false)
}

// Regra: a categoria genérica também no singular ("Transferência"); sem ela (e sem cartão), Reclassificar não quebra e
// não muda nada; sem lançamentos na genérica, também nada.
func TestCategoriaGenerica(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	if id, err := CategoriaGenerica(t.Context(), db); err != nil || id != nil {
		t.Fatalf("sem categoria: %v, %v", id, err)
	}
	c := cenario{db: db, fab: fab, agora: novoCenario(t).agora}
	if got := c.reclassificar(t); got != 0 {
		t.Errorf("banco sem categoria genérica nem cartão: mudaram %d", got)
	}
	singular := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Transferência", 3 })
	if id, err := CategoriaGenerica(t.Context(), db); err != nil || id == nil || *id != singular.Id {
		t.Errorf("no singular: %v, %v", id, err)
	}
	if got := novoCenario(t).reclassificar(t); got != 0 {
		t.Errorf("sem lançamentos na genérica: mudaram %d", got)
	}
}

// Regra: as pontas das regras de nome: o prefixo inteiro da reserva, nome de duas letras.
func TestPontasDoNomeEDaReserva(t *testing.T) {
	if !EhMovimentoDeReserva(p("Dinheiro reservado ")) || EhMovimentoDeReserva(p("Dinheiro reservado")) || EhMovimentoDeReserva(nil) {
		t.Error("o prefixo da reserva é o texto inteiro, com o espaço")
	}
	if got := PalavrasDoNome(p("Lu Silva")); len(got) != 2 || got[0] != "LU" || got[1] != "SILVA" {
		t.Errorf("nome de duas letras: %q", got)
	}
}
