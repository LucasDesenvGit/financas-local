package transacoes

import (
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/tipos"
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

// Regra: linha que não dá para ler volta como erro, sem lista pela metade.
func TestListarComLinhaInvalida(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	gravada := fab.Transacao(fab.Conta())
	if _, err := db.Exec(`UPDATE Transacao SET Valor = 'sem número' WHERE Id = ?`, gravada.Id); err != nil {
		t.Fatal(err)
	}
	lista, err := Listar(t.Context(), db, ``)
	if err == nil || !strings.Contains(err.Error(), "ler transação") || lista != nil {
		t.Errorf("Listar = %v, %v", lista, err)
	}
}

// Regra: a regra de descrição aplicada vai no UPDATE, e o que falhou ao gravar continua pendente: a mudança não se perde.
func TestGravarRegraDeDescricaoEErroDoBanco(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	gravada := fab.Transacao(fab.Conta())
	lista, err := Listar(t.Context(), db, `WHERE "Id" = ?`, gravada.Id)
	if err != nil {
		t.Fatal(err)
	}
	tr := lista[0]
	tr.RegraDescricaoId = p(int64(7))
	if err := Gravar(t.Context(), db, tr); err != nil {
		t.Fatal(err)
	}
	var regra int64
	if err := db.QueryRow(`SELECT RegraDescricaoId FROM Transacao WHERE Id = ?`, tr.Id).Scan(&regra); err != nil || regra != 7 {
		t.Fatalf("RegraDescricaoId = %d, %v", regra, err)
	}

	tr.Descricao = "Mudou"
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for nome, gravar := range map[string]func() error{
		"Gravar":      func() error { return Gravar(t.Context(), db, tr) },
		"GravarTodas": func() error { return GravarTodas(t.Context(), db, lista) },
	} {
		if err := gravar(); err == nil || !strings.Contains(err.Error(), "gravar a transação") {
			t.Errorf("%s = %v", nome, err)
		}
		if !tr.Mudou() {
			t.Errorf("%s falhou e a mudança se perdeu", nome)
		}
	}
}

// Regra: as colunas que as telas mudam também vão no UPDATE quando mudam (conta, data, descrição do banco, valor,
// status, emergência, nome escolhido à mão, observação); o valor compara como decimal, como o EF Core: 60 e 60.00 são o
// mesmo, e o texto do banco fica como estava.
func TestGravarAsColunasDasTelas(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	conta, outra := fab.Conta(), fab.Conta()
	gravada := fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Valor = testeapoio.Valor("-60.0") })
	ler := func() *Transacao {
		t.Helper()
		lista, err := Listar(t.Context(), db, `WHERE "Id" = ?`, gravada.Id)
		if err != nil || len(lista) != 1 {
			t.Fatalf("Listar = %v, %v", lista, err)
		}
		return lista[0]
	}
	tr := ler()
	tr.Valor = testeapoio.Valor("-60.00")
	if tr.Mudou() {
		t.Error("60.00 e 60.0 são o mesmo valor")
	}
	tr.ContaId, tr.Data, tr.DescricaoOriginal, tr.Valor = outra.Id, tipos.NovaData(2026, 10, 1), p("DO BANCO"), testeapoio.Valor("61.5")
	tr.Status, tr.Emergencia, tr.DescricaoManual, tr.Observacao = 2, true, true, p("nota")
	if err := Gravar(t.Context(), db, tr); err != nil {
		t.Fatal(err)
	}
	var contaId, status int64
	var data, original, valor, observacao string
	var emergencia, manual bool
	if err := db.QueryRow(`SELECT ContaId, Data, DescricaoOriginal, Valor, Status, Emergencia, DescricaoManual, Observacao
		FROM Transacao WHERE Id = ?`, tr.Id).Scan(&contaId, &data, &original, &valor, &status, &emergencia, &manual, &observacao); err != nil {
		t.Fatal(err)
	}
	if contaId != outra.Id || data != "2026-10-01" || original != "DO BANCO" || valor != "61.5" || status != 2 || !emergencia ||
		!manual || observacao != "nota" {
		t.Errorf("gravado: %d %s %s %s %d %v %v %s", contaId, data, original, valor, status, emergencia, manual, observacao)
	}
	tr = ler()
	tr.DescricaoOriginal, tr.Observacao = nil, nil
	if err := Gravar(t.Context(), db, tr); err != nil {
		t.Fatal(err)
	}
	var originalNulo, observacaoNula *string
	if err := db.QueryRow(`SELECT DescricaoOriginal, Observacao FROM Transacao WHERE Id = ?`, tr.Id).Scan(&originalNulo, &observacaoNula); err != nil {
		t.Fatal(err)
	}
	if originalNulo != nil || observacaoNula != nil {
		t.Errorf("de texto para nulo: %v, %v", originalNulo, observacaoNula)
	}
}

// Regra: a comparação de texto anulável (nulo só é igual a nulo).
func TestTextosIguais(t *testing.T) {
	casos := []struct {
		a, b     *string
		esperado bool
	}{
		{nil, nil, true}, {p("x"), p("x"), true}, {p("x"), p("y"), false}, {nil, p("x"), false}, {p("x"), nil, false},
	}
	for _, c := range casos {
		if obtido := textosIguais(c.a, c.b); obtido != c.esperado {
			t.Errorf("textosIguais(%v, %v) = %v", c.a, c.b, obtido)
		}
	}
}

// Regra: os campos que só a sincronização muda também vão no UPDATE, um a um (#83).
func TestGravarOsCamposDaSincronizacao(t *testing.T) {
	casos := []struct {
		nome, coluna string
		mudar        func(*Transacao)
		esperado     string
	}{
		{"estabelecimento", "Estabelecimento", func(t *Transacao) { t.Estabelecimento = p("Loja") }, "Loja"},
		{"categoria da Pluggy", "PluggyCategoriaId", func(t *Transacao) { t.PluggyCategoriaId = p("05000000") }, "05000000"},
		{"parcela", "ParcelaNumero", func(t *Transacao) { t.ParcelaNumero = p(int64(2)) }, "2"},
		{"total de parcelas", "ParcelaTotal", func(t *Transacao) { t.ParcelaTotal = p(int64(6)) }, "6"},
		{"meio de pagamento", "MeioPagamento", func(t *Transacao) { t.MeioPagamento = p("PIX") }, "PIX"},
		{"mesmo titular", "MesmaTitularidade", func(t *Transacao) { t.MesmaTitularidade = p(true) }, "1"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db := testeapoio.NovoBancoVazio(t)
			fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
			gravada := fab.Transacao(fab.Conta())
			lista, err := Listar(t.Context(), db, `WHERE "Id" = ?`, gravada.Id)
			if err != nil {
				t.Fatal(err)
			}
			tr := lista[0]
			c.mudar(tr)
			if !tr.Mudou() {
				t.Fatal("Mudou = false")
			}
			if err := Gravar(t.Context(), db, tr); err != nil {
				t.Fatal(err)
			}
			var obtido string
			if err := db.QueryRow(`SELECT "`+c.coluna+`" FROM "Transacao" WHERE "Id" = ?`, gravada.Id).Scan(&obtido); err != nil {
				t.Fatal(err)
			}
			if obtido != c.esperado {
				t.Errorf("%s = %q, esperado %q", c.coluna, obtido, c.esperado)
			}
			if tr.Mudou() {
				t.Error("depois de gravar, ainda mudou")
			}
		})
	}
	t.Run("fatura", func(t *testing.T) {
		db := testeapoio.NovoBancoVazio(t)
		fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
		conta := fab.Conta()
		fatura := fab.Fatura(conta, tipos.NovaData(2026, 10, 20), testeapoio.Valor("10.0"))
		gravada := fab.Transacao(conta)
		lista, err := Listar(t.Context(), db, `WHERE "Id" = ?`, gravada.Id)
		if err != nil {
			t.Fatal(err)
		}
		lista[0].FaturaCartaoId = &fatura.Id
		if err := Gravar(t.Context(), db, lista[0]); err != nil {
			t.Fatal(err)
		}
		de, err := Listar(t.Context(), db, `WHERE "Id" = ?`, gravada.Id)
		if err != nil {
			t.Fatal(err)
		}
		if id := de[0].FaturaCartaoId; id == nil || *id != fatura.Id {
			t.Errorf("fatura = %v", id)
		}
	})
}

func TestBoolIguais(t *testing.T) {
	casos := []struct {
		a, b     *bool
		esperado bool
	}{
		{nil, nil, true},
		{p(true), nil, false},
		{nil, p(true), false},
		{p(true), p(true), true},
		{p(true), p(false), false},
	}
	for _, c := range casos {
		if obtido := boolIguais(c.a, c.b); obtido != c.esperado {
			t.Errorf("boolIguais(%v, %v) = %v", c.a, c.b, obtido)
		}
	}
}

// Regra: Inserir grava todas as colunas e devolve o Id; DescricaoRevisada nasce falsa.
func TestInserir(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	conta := fab.Conta()
	fatura := fab.Fatura(conta, tipos.NovaData(2026, 10, 20), testeapoio.Valor("10.0"))
	categoria := fab.Categoria()
	grupo := tipos.NovoGuid()
	criado := tipos.NovaDataHora(time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC))
	atualizado := tipos.NovaDataHora(time.Date(2026, 10, 15, 19, 0, 0, 0, time.UTC))
	tr := &Transacao{
		ContaId: conta.Id, PluggyTransactionId: p("px-1"), Data: tipos.NovaData(2026, 10, 5), Descricao: "Loja", DescricaoOriginal: p("LOJA"),
		Estabelecimento: p("Loja LTDA"), Valor: testeapoio.Valor("-25.5"), Tipo: Despesa, Status: 2, CategoriaId: &categoria.Id, CategoriaManual: true,
		PluggyCategoriaId: p("05000000"), FaturaCartaoId: &fatura.Id, ParcelaNumero: p(int64(2)), ParcelaTotal: p(int64(6)), IgnorarRelatorios: true,
		Emergencia: true, MeioPagamento: p("PIX"), MesmaTitularidade: p(false), DescricaoManual: true, TransferenciaGrupo: &grupo, Observacao: p("nota"), AtualizadoEm: atualizado,
	}
	if err := Inserir(t.Context(), db, tr, criado); err != nil {
		t.Fatal(err)
	}
	if tr.Id == 0 {
		t.Fatal("Inserir sem o Id")
	}
	lista, err := Listar(t.Context(), db, `WHERE "Id" = ?`, tr.Id)
	if err != nil {
		t.Fatal(err)
	}
	l := lista[0]
	if l.ContaId != conta.Id || *l.PluggyTransactionId != "px-1" || l.Data.String() != "2026-10-05" || l.Descricao != "Loja" || *l.DescricaoOriginal != "LOJA" ||
		*l.Estabelecimento != "Loja LTDA" || l.Valor.String() != "-25.5" || l.Tipo != Despesa || l.Status != 2 || *l.CategoriaId != categoria.Id || !l.CategoriaManual ||
		*l.PluggyCategoriaId != "05000000" || *l.FaturaCartaoId != fatura.Id || *l.ParcelaNumero != 2 || *l.ParcelaTotal != 6 || !l.IgnorarRelatorios || !l.Emergencia ||
		*l.MeioPagamento != "PIX" || *l.MesmaTitularidade || !l.DescricaoManual || *l.TransferenciaGrupo != grupo || *l.Observacao != "nota" {
		t.Errorf("lida = %+v", l)
	}
	if l.AtualizadoEm.String() != atualizado.String() {
		t.Errorf("AtualizadoEm = %s", l.AtualizadoEm)
	}
	var gravado string
	var revisada bool
	if err := db.QueryRow(`SELECT "CriadoEm", "DescricaoRevisada" FROM "Transacao" WHERE "Id" = ?`, tr.Id).Scan(&gravado, &revisada); err != nil {
		t.Fatal(err)
	}
	if gravado != criado.String() || revisada {
		t.Errorf("CriadoEm = %s, revisada = %v", gravado, revisada)
	}

	semGrupo := &Transacao{ContaId: conta.Id, Data: tipos.NovaData(2026, 10, 6), Valor: testeapoio.Valor("1.0"), Tipo: Receita, Status: 1, AtualizadoEm: atualizado}
	if err := Inserir(t.Context(), db, semGrupo, criado); err != nil {
		t.Fatal(err)
	}
	if semGrupo.Id == tr.Id {
		t.Error("Id repetido")
	}
	vazia, err := Listar(t.Context(), db, `WHERE "Id" = ?`, semGrupo.Id)
	if err != nil {
		t.Fatal(err)
	}
	if vazia[0].TransferenciaGrupo != nil {
		t.Errorf("grupo = %v", vazia[0].TransferenciaGrupo)
	}
}

func TestInserirComErro(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	if err := Inserir(t.Context(), db, &Transacao{ContaId: 999, Data: tipos.NovaData(2026, 10, 6), Valor: testeapoio.Valor("1.0"), Tipo: Receita, Status: 1}, tipos.DataHora{}); err == nil {
		t.Fatal("Inserir numa conta que não existe sem erro")
	}
}

func TestExcluir(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	conta := fab.Conta()
	a, b, c := fab.Transacao(conta), fab.Transacao(conta), fab.Transacao(conta)
	if err := Excluir(t.Context(), db, a.Id, c.Id); err != nil {
		t.Fatal(err)
	}
	lista, err := Listar(t.Context(), db, `ORDER BY "Id"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != 1 || lista[0].Id != b.Id {
		t.Errorf("sobraram %d", len(lista))
	}
	if err := Excluir(t.Context(), db); err != nil {
		t.Errorf("sem ids = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Excluir(t.Context(), db, b.Id); err == nil {
		t.Error("banco fechado sem erro")
	}
}
