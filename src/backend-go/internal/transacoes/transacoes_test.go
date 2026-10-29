package transacoes

import (
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func p[T any](v T) *T { return &v }

// Regra: Listar lê as colunas no formato do EF Core (decimal, data, GUID, bool anulável) e guarda como veio; Gravar
// manda só as colunas que mudaram (o SaveChanges do EF Core), e gravar de novo sem mudança não faz nada.
func TestListarEGravarSoOQueMudou(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	conta := fab.Conta()
	categoria := fab.Categoria()
	grupo := tipos.NovoGuid()
	gravada := fab.Transacao(conta, func(tr *testeapoio.Transacao) {
		tr.DescricaoOriginal, tr.Descricao = p("PIX ORIGINAL"), "PIX ORIGINAL"
		tr.MesmaTitularidade, tr.TransferenciaGrupo, tr.CategoriaId = p(true), &grupo, &categoria.Id
	})
	lista, err := Listar(t.Context(), db, `WHERE "Id" = ?`, gravada.Id)
	if err != nil || len(lista) != 1 {
		t.Fatalf("Listar = %v, %v", lista, err)
	}
	tr := lista[0]
	if tr.Valor.String() != "-10.0" || tr.TransferenciaGrupo == nil || *tr.TransferenciaGrupo != grupo ||
		tr.MesmaTitularidade == nil || !*tr.MesmaTitularidade || tr.CategoriaId == nil || *tr.CategoriaId != categoria.Id {
		t.Fatalf("lida errado: %+v", tr)
	}
	if tr.Mudou() {
		t.Fatal("acabou de ler e já mudou")
	}

	// Muda cada coluna gravável; as outras (Valor, Data, MeioPagamento) não podem ir no UPDATE.
	agora := tipos.NovaDataHora(time.Date(2026, 10, 15, 18, 30, 0, 0, time.UTC))
	tr.Descricao, tr.Tipo, tr.CategoriaId, tr.CategoriaManual = "Aluguel", Transferencia, nil, true
	tr.IgnorarRelatorios, tr.RegraDescricaoId, tr.AtualizadoEm = true, nil, agora
	if _, err := db.Exec(`UPDATE Transacao SET Valor = '-99.0' WHERE Id = ?`, tr.Id); err != nil { // mudança de outra pessoa
		t.Fatal(err)
	}
	if !tr.Mudou() {
		t.Fatal("mudou sete colunas e Mudou() é false")
	}
	if err := Gravar(t.Context(), db, tr); err != nil {
		t.Fatal(err)
	}
	var descricao, valor, atualizado string
	var tipo int64
	var categoriaId *int64
	var manual, ignorar bool
	if err := db.QueryRow(`SELECT Descricao, Valor, Tipo, CategoriaId, CategoriaManual, IgnorarRelatorios, AtualizadoEm FROM Transacao WHERE Id = ?`, tr.Id).
		Scan(&descricao, &valor, &tipo, &categoriaId, &manual, &ignorar, &atualizado); err != nil {
		t.Fatal(err)
	}
	if descricao != "Aluguel" || tipo != Transferencia || categoriaId != nil || !manual || !ignorar || atualizado != agora.String() {
		t.Errorf("gravado: %q %d %v %v %v %q", descricao, tipo, categoriaId, manual, ignorar, atualizado)
	}
	if valor != "-99.0" {
		t.Errorf("o Valor foi no UPDATE (%q): só as colunas que mudaram vão", valor)
	}
	if tr.Mudou() {
		t.Error("depois de gravar, Mudou() continua true")
	}
	if _, err := db.Exec(`UPDATE Transacao SET Descricao = 'outra pessoa' WHERE Id = ?`, tr.Id); err != nil {
		t.Fatal(err)
	}
	if err := GravarTodas(t.Context(), db, lista); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT Descricao FROM Transacao WHERE Id = ?`, tr.Id).Scan(&descricao); err != nil || descricao != "outra pessoa" {
		t.Errorf("gravar sem mudança mexeu no banco: %q %v", descricao, err)
	}
}

// Regra: GravarTodas grava todas as que mudaram, não só a primeira.
func TestGravarTodas(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	conta := fab.Conta()
	fab.Transacao(conta)
	fab.Transacao(conta)
	lista, err := Listar(t.Context(), db, `ORDER BY "Id"`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range lista {
		tr.Descricao = "mudou"
	}
	if err := GravarTodas(t.Context(), db, lista); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Transacao WHERE Descricao = 'mudou'`).Scan(&n); err != nil || n != 2 {
		t.Errorf("gravou %d de 2 (%v)", n, err)
	}
}

// Regra: transação montada à mão (sem ter sido lida) não tem o que gravar.
func TestGravarSemLeituraNaoFazNada(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	if err := Gravar(t.Context(), db, &Transacao{Id: 1, Descricao: "x"}); err != nil {
		t.Fatal(err)
	}
}

// Regra: erro do banco volta como erro, com o contexto.
func TestListarComErroDoBanco(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	// Sem aspas: com aspas, o SQLite trata o nome que não é coluna como texto, e não dá erro.
	if _, err := Listar(t.Context(), db, `WHERE ColunaQueNaoExiste = 1`); err == nil {
		t.Error("consulta errada não deu erro")
	}
}
