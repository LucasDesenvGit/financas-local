package regrasdescricao

import (
	"database/sql"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/categorizacao"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

func dec(s string) *tipos.Decimal {
	d, err := tipos.NovoDecimal(s)
	if err != nil {
		panic(err)
	}
	return &d
}

func p[T any](v T) *T { return &v }

// gravarRegra grava uma regra de descrição ativa e devolve o Id.
func gravarRegra(t *testing.T, db *sql.DB, texto string, tipo int64, nova string, minimo, maximo *tipos.Decimal, criadoEm string, ativa bool) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO RegraDescricao (Texto, TipoCorrespondencia, NovaDescricao, ValorMinimo, ValorMaximo, Ativa, CriadoEm)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, texto, tipo, nova, minimo, maximo, ativa, criadoEm)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// Regra: a ordem do CarregarAsync: com faixa primeiro; depois igual, começa com, contém; texto mais longo (UTF-16);
// a mais antiga; o menor Id. Desligada e texto que fica vazio não entram.
func TestCarregarNaOrdemDoCSharp(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	contem := gravarRegra(t, db, "LOJA", categorizacao.Contem, "contém", nil, nil, "2026-01-01 00:00:00", true)
	igual := gravarRegra(t, db, "LOJA", categorizacao.Igual, "igual", nil, nil, "2026-01-02 00:00:00", true)
	comeca := gravarRegra(t, db, "LOJA", categorizacao.ComecaCom, "começa", nil, nil, "2026-01-03 00:00:00", true)
	longa := gravarRegra(t, db, "LOJA GRANDE", categorizacao.Contem, "longa", nil, nil, "2026-01-04 00:00:00", true)
	faixa := gravarRegra(t, db, "L", categorizacao.Contem, "faixa", dec("1"), nil, "2026-01-05 00:00:00", true)
	antiga := gravarRegra(t, db, "LOJB", categorizacao.Contem, "antiga", nil, nil, "2025-12-31 23:59:59.9", true)
	emoji := gravarRegra(t, db, "😀X", categorizacao.Contem, "emoji conta 3 (UTF-16)", nil, nil, "2026-01-06 00:00:00", true)
	gravarRegra(t, db, "LOJA", categorizacao.Igual, "desligada", nil, nil, "2026-01-01 00:00:00", false)
	gravarRegra(t, db, "   ", categorizacao.Igual, "vazia", nil, nil, "2026-01-01 00:00:00", true)

	rs, err := Carregar(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, r := range rs.regras {
		ids = append(ids, r.id)
	}
	esperado := []int64{faixa, igual, comeca, longa, antiga, contem, emoji}
	// "😀X" tem 3 unidades UTF-16 (o "LOJA", 4): fica depois das de 4, antes de nenhuma outra de 3 ou menos.
	if len(ids) != len(esperado) {
		t.Fatalf("regras %v, esperado %v", ids, esperado)
	}
	for i := range ids {
		if ids[i] != esperado[i] {
			t.Fatalf("ordem %v, esperado %v", ids, esperado)
		}
	}
}

// Regra: NaFaixa (#241): sem o sinal, pontas inclusivas e opcionais.
func TestNaFaixa(t *testing.T) {
	casos := []struct {
		nome           string
		valor          string
		minimo, maximo *tipos.Decimal
		esperado       bool
	}{
		{"sem faixa", "-5", nil, nil, true},
		{"despesa negativa pelo valor absoluto", "-1850", dec("1800"), dec("1900"), true},
		{"ponta de baixo inclusiva", "1800.00", dec("1800.0"), nil, true},
		{"ponta de cima inclusiva", "-1900", nil, dec("1900"), true},
		{"abaixo", "1799.99", dec("1800"), nil, false},
		{"acima", "1900.01", nil, dec("1900"), false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := NaFaixa(*dec(c.valor), c.minimo, c.maximo); got != c.esperado {
				t.Errorf("NaFaixa = %v, esperado %v", got, c.esperado)
			}
		})
	}
}

// Regra: a cadeia (#250): cada regra olha o nome dado pela anterior, uma vez só (sem ciclo A→B→A), até 10 passos.
func TestCadeia(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	a := gravarRegra(t, db, "PIX TRANSF", categorizacao.Contem, "Aluguel", nil, nil, "2026-01-01 00:00:00", true)
	b := gravarRegra(t, db, "Aluguel", categorizacao.Igual, "Aluguel apto 12", nil, nil, "2026-01-02 00:00:00", true)
	gravarRegra(t, db, "APTO 12", categorizacao.Contem, "PIX TRANSF de volta", nil, nil, "2026-01-03 00:00:00", true)
	rs, err := Carregar(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	cadeia := rs.Cadeia(p("PIX TRANSF 123"), *dec("-10"))
	if len(cadeia) != 3 || cadeia[0].Id != a || cadeia[1].Id != b || cadeia[2].NovaDescricao != "PIX TRANSF de volta" {
		t.Fatalf("cadeia %+v: esperado A, B e a de volta, sem repetir A", cadeia)
	}
	if got := rs.Cadeia(nil, *dec("-10")); len(got) != 0 {
		t.Errorf("sem descrição do banco, a cadeia é vazia: %+v", got)
	}

	// Dez regras em fila: a cadeia para no décimo passo.
	db2 := testeapoio.NovoBancoVazio(t)
	for i := 0; i < 12; i++ {
		gravarRegra(t, db2, "PASSO "+string(rune('A'+i)), categorizacao.Igual, "PASSO "+string(rune('A'+i+1)), nil, nil, "2026-01-01 00:00:00", true)
	}
	rs2, _ := Carregar(t.Context(), db2)
	if got := rs2.Cadeia(p("PASSO A"), *dec("1")); len(got) != MaximoNaCadeia {
		t.Errorf("cadeia de %d passos, esperado o máximo de %d", len(got), MaximoNaCadeia)
	}
}

// Regra: Aplicar só mexe em nome automático (do banco ou dado por regra); volta ao nome do banco sem regra; nome
// escolhido à mão fica.
func TestAplicar(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	regra := gravarRegra(t, db, "DEBITO ENERGIA", categorizacao.ComecaCom, "Conta de luz", nil, nil, "2026-01-01 00:00:00", true)
	rs, _ := Carregar(t.Context(), db)
	casos := []struct {
		nome                string
		descricao, original string
		regraId             *int64
		manual              bool
		mudou               bool
		descricaoDepois     string
		regraDepois         *int64
	}{
		{"nome do banco ganha o da regra", "DEBITO ENERGIA FICTICIA", "DEBITO ENERGIA FICTICIA", nil, false, true, "Conta de luz", &regra},
		{"já com o nome da regra, nada muda", "Conta de luz", "DEBITO ENERGIA FICTICIA", &regra, false, false, "Conta de luz", &regra},
		{"nome dado por regra que não vale mais volta ao do banco", "Antigo", "LOJA", p(int64(99)), false, true, "LOJA", nil},
		{"nome escolhido à mão fica", "Minha conta", "DEBITO ENERGIA FICTICIA", nil, true, false, "Minha conta", nil},
		{"nome editado antes das regras fica", "Editado", "DEBITO ENERGIA FICTICIA", nil, false, false, "Editado", nil},
		{"mesmo nome com outra regra troca só o Id", "Conta de luz", "DEBITO ENERGIA FICTICIA", p(int64(99)), false, true, "Conta de luz", &regra},
	}
	sem := &transacoes.Transacao{Descricao: "Lançamento manual", Valor: *dec("-1")} // sem descrição do banco
	if rs.Aplicar(sem) || sem.Descricao != "Lançamento manual" {
		t.Errorf("lançamento sem descrição do banco mudou: %q", sem.Descricao)
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			original := c.original
			tr := &transacoes.Transacao{Descricao: c.descricao, DescricaoOriginal: &original, RegraDescricaoId: c.regraId,
				DescricaoManual: c.manual, Valor: *dec("-100")}
			if got := rs.Aplicar(tr); got != c.mudou || tr.Descricao != c.descricaoDepois || !mesmoId(tr.RegraDescricaoId, c.regraDepois) {
				t.Errorf("Aplicar = %v, %q, %v; esperado %v, %q, %v", got, tr.Descricao, tr.RegraDescricaoId, c.mudou, c.descricaoDepois, c.regraDepois)
			}
		})
	}
}

// Regra: a prévia conta as de nome automático que batem pela descrição do banco ou pelo nome de agora, na faixa, e dá
// até 3 exemplos diferentes, na ordem do banco; sem texto, zero.
func TestPrevia(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	conta := fab.Conta()
	nova := func(descricao, original, valor string, manual bool) {
		fab.Transacao(conta, func(tr *testeapoio.Transacao) {
			tr.Descricao, tr.DescricaoOriginal, tr.Valor, tr.DescricaoManual = descricao, p(original), *dec(valor), manual
		})
	}
	nova("PIX A", "PIX A", "-10", false)
	nova("PIX B", "PIX B", "-200", false)
	nova("PIX A", "PIX A", "-10", false)
	nova("Aluguel", "TED 123", "-1850", false) // nome editado à mão antes das regras: não é automático
	nova("PIX C", "PIX C", "-10", true)        // manual
	nova("PIX D", "PIX D", "-10", false)
	nova("PIX E", "PIX E", "-10", false) // o quarto exemplo diferente: fica de fora
	quantidade, exemplos, err := Previa(t.Context(), db, "pix", categorizacao.Contem, 0, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if quantidade != 5 || len(exemplos) != 3 || exemplos[0] != "PIX A" || exemplos[1] != "PIX B" || exemplos[2] != "PIX D" {
		t.Errorf("prévia = %d, %q; esperado 5, [PIX A PIX B PIX D]", quantidade, exemplos)
	}
	quantidade, _, _ = Previa(t.Context(), db, "pix", categorizacao.Contem, 0, dec("100"), nil)
	if quantidade != 1 {
		t.Errorf("com a faixa a partir de 100, %d; esperado 1", quantidade)
	}
	quantidade, exemplos, _ = Previa(t.Context(), db, "   ", categorizacao.Contem, 0, nil, nil)
	if quantidade != 0 || exemplos == nil || len(exemplos) != 0 {
		t.Errorf("sem texto: %d, %v; esperado 0 e [] (não null)", quantidade, exemplos)
	}
}

// Regra: Reaplicar devolve só as que mudaram, com AtualizadoEm = agora, sem gravar.
func TestReaplicarNaoGrava(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	conta := fab.Conta()
	gravarRegra(t, db, "LOJA", categorizacao.Contem, "Loja do bairro", nil, nil, "2026-01-01 00:00:00", true)
	muda := fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Descricao, tr.DescricaoOriginal = "LOJA X", p("LOJA X") })
	fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Descricao, tr.DescricaoOriginal = "OUTRA", p("OUTRA") })
	agora := tipos.NovaDataHora(time.Date(2026, 10, 15, 15, 0, 0, 0, time.UTC))
	mudaram, err := Reaplicar(t.Context(), db, agora)
	if err != nil {
		t.Fatal(err)
	}
	if len(mudaram) != 1 || mudaram[0].Id != muda.Id || mudaram[0].Descricao != "Loja do bairro" || mudaram[0].AtualizadoEm.String() != agora.String() {
		t.Fatalf("mudaram %+v", mudaram)
	}
	var noBanco string
	if err := db.QueryRow(`SELECT Descricao FROM Transacao WHERE Id = ?`, muda.Id).Scan(&noBanco); err != nil || noBanco != "LOJA X" {
		t.Errorf("o Reaplicar gravou (%q, %v): quem grava é quem chama", noBanco, err)
	}
}

// Regra: mesmoId compara dois Ids anuláveis (nil só é igual a nil).
func TestMesmoId(t *testing.T) {
	casos := []struct {
		a, b     *int64
		esperado bool
	}{{nil, nil, true}, {nil, p(int64(1)), false}, {p(int64(1)), nil, false}, {p(int64(1)), p(int64(1)), true}, {p(int64(1)), p(int64(2)), false}}
	for _, c := range casos {
		if got := mesmoId(c.a, c.b); got != c.esperado {
			t.Errorf("mesmoId(%v, %v) = %v", c.a, c.b, got)
		}
	}
}
