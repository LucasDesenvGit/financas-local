package categorizacao

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// Os testes dos acertos da partida (#444) rodam num SQLite de verdade, com o esquema real. A fábrica grava os dados com
// o relógio de ontem; os acertos rodam "hoje", para o AtualizadoEm mostrar o que mudou e o que ficou como estava.
var (
	ontem = testeapoio.AgoraEmBrasilia(2026, 10, 14, 12, 0)
	hoje  = testeapoio.AgoraEmBrasilia(2026, 10, 15, 12, 0)
)

func agoraDe(r relogio.Relogio) tipos.DataHora { return tipos.NovaDataHora(r.Agora()) }

// emTx roda fn numa transação (a que a partida abre para cada rotina) e confirma.
func emTx[T any](t *testing.T, db *sql.DB, fn func(tx *sql.Tx) (T, error)) T {
	t.Helper()
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := fn(tx)
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return r
}

type estadoDaTransacao struct {
	tipo         int64
	ignorar      bool
	categoria    *int64
	atualizadoEm string
}

func estadoDe(t *testing.T, db *sql.DB, id int64) estadoDaTransacao {
	t.Helper()
	var e estadoDaTransacao
	err := db.QueryRowContext(t.Context(), `SELECT Tipo, IgnorarRelatorios, CategoriaId, AtualizadoEm FROM Transacao WHERE Id = ?`, id).
		Scan(&e.tipo, &e.ignorar, &e.categoria, &e.atualizadoEm)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (e estadoDaTransacao) mudouEm(r relogio.Relogio) bool {
	return e.atualizadoEm == agoraDe(r).String()
}

func contar(t *testing.T, db *sql.DB, consulta string, argumentos ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(t.Context(), consulta, argumentos...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// ------------------------------------------------------------------------------------ regras padrão novas

type cenaDeRegras struct {
	db            *sql.DB
	fab           *testeapoio.Fabrica
	conta         testeapoio.Conta
	investimentos testeapoio.Categoria
	mercado       testeapoio.Categoria
}

func novaCenaDeRegras(t *testing.T) cenaDeRegras {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, ontem)
	return cenaDeRegras{db: db, fab: fab, conta: fab.Conta(),
		investimentos: fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Investimentos", CategoriaTransferencia }),
		mercado:       fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Mercado" })}
}

func (c cenaDeRegras) criar(t *testing.T) int {
	t.Helper()
	return emTx(t, c.db, func(tx *sql.Tx) (int, error) { return CriarRegrasPadrao(t.Context(), tx, hoje) })
}

// Regra: CriarRegrasPadrao (#169, #172, ADR 0038): passa para "Investimentos" a transação que bate com a regra nova (o
// texto do banco, ou o nome se não houver; sem acento nem caixa) e que não tem categoria escolhida à mão nem grupo de
// transferência, e muda o tipo e o "fora dos relatórios" junto (AplicarTipo).
func TestCriarRegrasPadraoMudaAsTransacoes(t *testing.T) {
	t.Parallel()
	grupo := tipos.NovoGuid()
	casos := []struct {
		nome      string
		descricao string
		original  *string
		manual    bool
		grupo     *tipos.Guid
		muda      bool
	}{
		{"Méli Dólar pelo nome, sem texto do banco", "Meli Dólar", nil, false, nil, true},
		{"Méli Dólar em maiúsculas", "MELI DOLAR", nil, false, nil, true},
		{"o texto do banco vale mais que o nome", "Câmbio", p("Compra de Meli Dólar"), false, nil, true},
		{"o nome não vale quando há texto do banco", "Meli Dólar", p("Pix enviado"), false, nil, false},
		{"o texto do banco vazio também vale mais que o nome", "Meli Dólar", p(""), false, nil, false},
		{"com acento no e (#567)", "Méli Dólar", nil, false, nil, true},
		{"em maiúsculas com acento (#567)", "COMPRA MÉLI DÓLAR", nil, false, nil, true},
		{"com acento no texto do banco (#567)", "Câmbio", p("Venda de Méli Dólar"), false, nil, true},
		{"outra palavra no lugar de Méli não é a regra", "MALI DOLAR", nil, false, nil, false},
		{"sem o espaço, não é a regra", "MELIDOLAR", nil, false, nil, false},
		{"aplicação em RDB, com acento", "Aplicação RDB Nubank", nil, false, nil, true},
		{"resgate de CDB", "RESGATE CDB", nil, false, nil, true},
		{"resgate de aplicação", "Resgate de aplicação", nil, false, nil, true},
		{"resgate aplicação", "resgate aplicacao automatica", nil, false, nil, true},
		{"aplicação em poupança", "APLICACAO POUPANCA", nil, false, nil, true},
		{"resgate de poupança", "Resgate Poupança", nil, false, nil, true},
		{"resgate de pontos não é movimento seu", "Resgate de pontos", nil, false, nil, false},
		{"rendimento de aplicação é receita", "Rendimento de aplicação financeira", nil, false, nil, false},
		{"categoria escolhida à mão fica", "MELI DOLAR", nil, true, nil, false},
		{"lançada pelo site (grupo de transferência) fica", "MELI DOLAR", nil, false, &grupo, false},
	}
	c := novaCenaDeRegras(t)
	ids := make([]int64, len(casos))
	for i, caso := range casos {
		ids[i] = c.fab.Transacao(c.conta, func(tr *testeapoio.Transacao) {
			tr.Descricao, tr.DescricaoOriginal, tr.CategoriaManual, tr.TransferenciaGrupo = caso.descricao, caso.original, caso.manual, caso.grupo
			tr.CategoriaId = &c.mercado.Id
		}).Id
	}
	jaNaCategoria := c.fab.Transacao(c.conta, func(tr *testeapoio.Transacao) {
		tr.Descricao, tr.CategoriaId, tr.Tipo, tr.IgnorarRelatorios = "MELI DOLAR", &c.investimentos.Id, 3, true
	}).Id

	mudaram := c.criar(t)

	esperadas := 0
	for i, caso := range casos {
		e := estadoDe(t, c.db, ids[i])
		if caso.muda {
			esperadas++
			if e.categoria == nil || *e.categoria != c.investimentos.Id || e.tipo != transacoes.Transferencia || !e.ignorar || !e.mudouEm(hoje) {
				t.Errorf("%s: deveria ir para Investimentos, fora dos relatórios; ficou %+v", caso.nome, e)
			}
		} else if e.categoria == nil || *e.categoria != c.mercado.Id || e.tipo != transacoes.Despesa || e.ignorar || e.mudouEm(hoje) {
			t.Errorf("%s: não deveria mudar; ficou %+v", caso.nome, e)
		}
	}
	if mudaram != esperadas {
		t.Errorf("mudaram %d, esperado %d", mudaram, esperadas)
	}
	if e := estadoDe(t, c.db, jaNaCategoria); e.mudouEm(hoje) {
		t.Errorf("a que já está na categoria não é tocada: %+v", e)
	}
}

// Regra: CriarRegrasPadrao cria cada regra uma vez, com a prioridade 50 e o "contém"; a marca em Configuracao guarda
// o instante (ToString("O"), em UTC); e a segunda partida não muda nada.
func TestCriarRegrasPadraoCriaAsRegrasEAMarca(t *testing.T) {
	t.Parallel()
	c := novaCenaDeRegras(t)
	c.criar(t)

	linhas, err := c.db.QueryContext(t.Context(), `SELECT Texto, TipoCorrespondencia, CategoriaId, Prioridade, Ativa, CriadoEm FROM RegraCategorizacao ORDER BY Id`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	var textos []string
	for linhas.Next() {
		var texto, criadoEm string
		var tipo, categoria, prioridade int64
		var ativa bool
		if err := linhas.Scan(&texto, &tipo, &categoria, &prioridade, &ativa, &criadoEm); err != nil {
			t.Fatal(err)
		}
		if tipo != Contem || categoria != c.investimentos.Id || prioridade != 50 || !ativa || criadoEm != agoraDe(hoje).String() {
			t.Errorf("regra %q: tipo %d, categoria %d, prioridade %d, ativa %v, criada em %s", texto, tipo, categoria, prioridade, ativa, criadoEm)
		}
		textos = append(textos, texto)
	}
	esperadas := []string{"MELI DOLAR", "APLICACAO RDB", "RESGATE RDB", "APLICACAO CDB", "RESGATE CDB", "APLICACAO POUPANCA",
		"RESGATE POUPANCA", "RESGATE DE APLICACAO", "RESGATE APLICACAO"}
	if len(textos) != len(esperadas) {
		t.Fatalf("regras %v, esperado %v", textos, esperadas)
	}
	for i := range esperadas {
		if textos[i] != esperadas[i] {
			t.Errorf("regra %d é %q, esperado %q", i, textos[i], esperadas[i])
		}
	}
	for _, chave := range []string{ChaveRegraMeliDolar, ChaveRegrasAplicacaoResgate} {
		var valor string
		var protegido bool
		var atualizadoEm string
		if err := c.db.QueryRowContext(t.Context(), `SELECT Valor, Protegido, AtualizadoEm FROM Configuracao WHERE Chave = ?`, chave).Scan(&valor, &protegido, &atualizadoEm); err != nil {
			t.Fatalf("marca %s: %v", chave, err)
		}
		if valor != "2026-10-15T15:00:00.0000000Z" || protegido || atualizadoEm != agoraDe(hoje).String() {
			t.Errorf("marca %s: valor %q, protegido %v, atualizada em %s", chave, valor, protegido, atualizadoEm)
		}
	}

	// A segunda partida, um dia depois, não muda nada: nem as regras, nem as marcas.
	if n := emTx(t, c.db, func(tx *sql.Tx) (int, error) {
		return CriarRegrasPadrao(t.Context(), tx, testeapoio.AgoraEmBrasilia(2026, 10, 16, 9, 0))
	}); n != 0 {
		t.Errorf("segunda partida mudou %d", n)
	}
	if n := contar(t, c.db, `SELECT COUNT(*) FROM RegraCategorizacao`); n != len(esperadas) {
		t.Errorf("%d regras depois da segunda partida", n)
	}
	if n := contar(t, c.db, `SELECT COUNT(*) FROM Configuracao WHERE Chave IN (?, ?) AND AtualizadoEm = ?`,
		ChaveRegraMeliDolar, ChaveRegrasAplicacaoResgate, agoraDe(hoje).String()); n != 2 {
		t.Errorf("as marcas não deveriam ser regravadas: %d com a data da primeira partida", n)
	}
}

// Regra: a marca em Configuracao faz cada grupo ser criado uma vez só (a regra que você apagou não volta), marca sem valor
// não conta, e uma regra com o mesmo texto que já existe (comparado sem normalizar) não é duplicada nem muda de categoria.
func TestCriarRegrasPadraoRespeitaMarcaERegraExistente(t *testing.T) {
	t.Parallel()
	aplicacao := []string{"APLICACAO RDB@Investimentos", "RESGATE RDB@Investimentos", "APLICACAO CDB@Investimentos", "RESGATE CDB@Investimentos",
		"APLICACAO POUPANCA@Investimentos", "RESGATE POUPANCA@Investimentos", "RESGATE DE APLICACAO@Investimentos", "RESGATE APLICACAO@Investimentos"}
	dolar := "MELI DOLAR@Investimentos"
	marca := func(c cenaDeRegras, chave string) { c.fab.Configuracao(chave, "2026-01-01T00:00:00.0000000Z") }
	casos := []struct {
		nome     string
		preparar func(c cenaDeRegras)
		depois   []string
	}{
		{"as duas marcas presentes: nada é criado, mesmo sem as regras", func(c cenaDeRegras) {
			marca(c, ChaveRegraMeliDolar)
			marca(c, ChaveRegrasAplicacaoResgate)
		}, nil},
		{"só a marca do dólar: as de aplicação e resgate são criadas", func(c cenaDeRegras) { marca(c, ChaveRegraMeliDolar) }, aplicacao},
		{"só a marca de aplicação e resgate: a do dólar é criada", func(c cenaDeRegras) { marca(c, ChaveRegrasAplicacaoResgate) }, []string{dolar}},
		{"marca sem valor não vale como criada", func(c cenaDeRegras) {
			if _, err := c.db.Exec(`INSERT INTO Configuracao (Chave, Valor, Protegido, AtualizadoEm) VALUES (?, NULL, 0, '2026-01-01 00:00:00')`, ChaveRegraMeliDolar); err != nil {
				t.Fatal(err)
			}
			marca(c, ChaveRegrasAplicacaoResgate)
		}, []string{dolar}},
		{"regra com o mesmo texto já existe: não duplica e fica na categoria dela", func(c cenaDeRegras) {
			c.fab.RegraCategorizacao(c.mercado, func(r *testeapoio.RegraCategorizacao) { r.Texto = "MELI DOLAR" })
		}, aplicacao},
		{"o texto da regra existente é comparado sem normalizar", func(c cenaDeRegras) {
			c.fab.RegraCategorizacao(c.mercado, func(r *testeapoio.RegraCategorizacao) { r.Texto = "meli dolar" })
		}, append([]string{dolar}, aplicacao...)},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			t.Parallel()
			c := novaCenaDeRegras(t)
			caso.preparar(c)
			antes := regrasDoBanco(t, c.db)
			c.criar(t)
			depois := regrasDoBanco(t, c.db)
			esperadas := append(antes, caso.depois...)
			if len(depois) != len(esperadas) {
				t.Fatalf("regras %v, esperado %v", depois, esperadas)
			}
			for i := range esperadas {
				if depois[i] != esperadas[i] {
					t.Errorf("regra %d é %q, esperado %q", i, depois[i], esperadas[i])
				}
			}
		})
	}
}

// regrasDoBanco são as regras na ordem do Id, como "TEXTO@Categoria".
func regrasDoBanco(t *testing.T, db *sql.DB) []string {
	t.Helper()
	linhas, err := db.QueryContext(t.Context(), `SELECT r.Texto || '@' || c.Nome FROM RegraCategorizacao r JOIN Categoria c ON c.Id = r.CategoriaId ORDER BY r.Id`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	var lista []string
	for linhas.Next() {
		var x string
		if err := linhas.Scan(&x); err != nil {
			t.Fatal(err)
		}
		lista = append(lista, x)
	}
	return lista
}

// Regra: a categoria é a primeira de transferência chamada "Investimentos" (sem acento nem caixa, pelo Id); sem ela
// nenhuma regra nasce e a marca não é gravada, para as regras nascerem quando a categoria voltar (#568).
func TestCriarRegrasPadraoSemACategoria(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome       string
		nomes      []string // as categorias, na ordem, todas de transferência
		tipoDoNome int64
		regras     int
		categoria  int // índice em nomes da escolhida; -1 nenhuma
	}{
		{"sem a categoria", []string{"Outras"}, CategoriaTransferencia, 0, -1},
		{"nome parecido não vale", []string{"Meus investimentos"}, CategoriaTransferencia, 0, -1},
		{"do tipo despesa não vale", []string{"Investimentos"}, CategoriaDespesa, 0, -1},
		{"sem acento nem caixa", []string{"INVESTIMENTOS"}, CategoriaTransferencia, 9, 0},
		{"a de menor Id", []string{"investimentos", "Investimentos"}, CategoriaTransferencia, 9, 0},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			t.Parallel()
			db := testeapoio.NovoBancoVazio(t)
			fab := testeapoio.NovaFabrica(t, db, ontem)
			var ids []int64
			for _, nome := range caso.nomes {
				ids = append(ids, fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = nome, caso.tipoDoNome }).Id)
			}
			emTx(t, db, func(tx *sql.Tx) (int, error) { return CriarRegrasPadrao(t.Context(), tx, hoje) })
			if n := contar(t, db, `SELECT COUNT(*) FROM RegraCategorizacao`); n != caso.regras {
				t.Errorf("%d regras, esperado %d", n, caso.regras)
			}
			if caso.categoria >= 0 {
				if n := contar(t, db, `SELECT COUNT(*) FROM RegraCategorizacao WHERE CategoriaId = ?`, ids[caso.categoria]); n != caso.regras {
					t.Errorf("%d regras na categoria certa", n)
				}
			}
			marcas := 0
			if caso.categoria >= 0 {
				marcas = 2
			}
			if n := contar(t, db, `SELECT COUNT(*) FROM Configuracao WHERE Chave IN (?, ?)`, ChaveRegraMeliDolar, ChaveRegrasAplicacaoResgate); n != marcas {
				t.Errorf("%d marcas gravadas, esperado %d", n, marcas)
			}
		})
	}
}

// Regra (#568): quem não tinha a categoria "Investimentos" na primeira partida recebe as regras padrão novas na primeira
// partida depois de criá-la, uma vez só; e a regra que você apaga depois não volta.
func TestCriarRegrasPadraoQuandoACategoriaVolta(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, ontem)
	conta := fab.Conta()
	dolar := fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Descricao = "MELI DOLAR" }).Id
	partida := func(dia int) int {
		return emTx(t, db, func(tx *sql.Tx) (int, error) {
			return CriarRegrasPadrao(t.Context(), tx, testeapoio.AgoraEmBrasilia(2026, 10, dia, 9, 0))
		})
	}

	if n := partida(15); n != 0 {
		t.Errorf("sem a categoria, mudaram %d", n)
	}
	investimentos := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Investimentos", CategoriaTransferencia })

	if n := partida(16); n != 1 {
		t.Errorf("com a categoria de volta, mudaram %d, esperado 1 (a do dólar)", n)
	}
	if n := contar(t, db, `SELECT COUNT(*) FROM RegraCategorizacao WHERE CategoriaId = ?`, investimentos.Id); n != 9 {
		t.Errorf("%d regras na categoria, esperado 9", n)
	}
	if e := estadoDe(t, db, dolar); e.categoria == nil || *e.categoria != investimentos.Id {
		t.Errorf("a transação do dólar deveria ir para Investimentos: %+v", e)
	}

	if _, err := db.ExecContext(t.Context(), `DELETE FROM RegraCategorizacao WHERE Texto = 'MELI DOLAR'`); err != nil {
		t.Fatal(err)
	}
	if n := partida(17); n != 0 {
		t.Errorf("terceira partida mudou %d", n)
	}
	if n := contar(t, db, `SELECT COUNT(*) FROM RegraCategorizacao`); n != 8 {
		t.Errorf("%d regras: a que você apagou não volta, e nenhuma é duplicada", n)
	}
}

// Regra: a transação que o CriarRegrasPadrao muda segue o AplicarTipo: no cartão, a entrada que é de transferência
// (aplicação, resgate) continua sendo transferência fora dos relatórios.
func TestCriarRegrasPadraoNoCartaoSegueOAplicarTipo(t *testing.T) {
	t.Parallel()
	c := novaCenaDeRegras(t)
	cartao := c.fab.Conta(func(x *testeapoio.Conta) { x.Tipo = 3 })
	entrada := c.fab.Transacao(cartao, func(tr *testeapoio.Transacao) {
		tr.Descricao, tr.Valor, tr.Tipo = "Resgate RDB", testeapoio.Valor("50"), 1
	})
	c.criar(t)
	if e := estadoDe(t, c.db, entrada.Id); e.tipo != transacoes.Transferencia || !e.ignorar || e.categoria == nil || *e.categoria != c.investimentos.Id {
		t.Errorf("entrada no cartão: %+v", e)
	}
}

// ------------------------------------------------------------------------------------ entradas nos cartões

// Regra: ReclassificarEntradasNosCartoes (#166, ADR 0036): entrada no cartão é pagamento de fatura (transferência) ou
// estorno (despesa positiva), nunca receita. Só olha crédito positivo, sem grupo de transferência, que é receita ou
// transferência sem categoria escolhida à mão; só grava, e só carimba, o que mudou.
func TestReclassificarEntradasNosCartoes(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, ontem)
	categoria := func(nome string, tipo int64) testeapoio.Categoria {
		return fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = nome, tipo })
	}
	pagamento := categoria("Pagamento de fatura", CategoriaTransferencia)
	generica := categoria("Transferências", CategoriaTransferencia)
	mercado := categoria("Mercado", CategoriaDespesa)
	transporte := categoria("Transporte", CategoriaDespesa)
	fab.RegraCategorizacao(transporte, func(r *testeapoio.RegraCategorizacao) { r.Texto = "UBER" })
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Tipo = 3 })
	corrente := fab.Conta()
	grupo := tipos.NovoGuid()

	type entrada struct {
		nome       string
		conta      testeapoio.Conta
		valor      string
		descricao  string
		tipo       int64
		categoria  *int64
		manual     bool
		ignorar    bool
		grupo      *tipos.Guid
		depois     int64
		ignorarDep bool
		catDepois  *int64
		muda       bool
	}
	casos := []entrada{
		{"pagamento da fatura vira transferência, na categoria certa", cartao, "100", "PAGAMENTO RECEBIDO", transacoes.Receita, nil, false, false, nil, transacoes.Transferencia, true, &pagamento.Id, true},
		{"estorno com categoria vira despesa e abate o gasto", cartao, "50", "ESTORNO LOJA", transacoes.Receita, &mercado.Id, false, false, nil, transacoes.Despesa, false, &mercado.Id, true},
		{"entrada qualquer no cartão é despesa positiva, não receita", cartao, "30", "CREDITO", transacoes.Receita, nil, false, false, nil, transacoes.Despesa, false, nil, true},
		{"transferência (não manual) com texto de estorno vira despesa na categoria do texto", cartao, "30", "Estorno de Uber - NuPay", transacoes.Transferencia, &generica.Id, false, true, nil, transacoes.Despesa, false, &transporte.Id, true},
		{"receita com categoria de transferência escolhida à mão vira transferência", cartao, "30", "PIX", transacoes.Receita, &generica.Id, true, false, nil, transacoes.Transferencia, true, &generica.Id, true},
		{"transferência manual não entra", cartao, "30", "ESTORNO", transacoes.Transferencia, &generica.Id, true, true, nil, transacoes.Transferencia, true, &generica.Id, false},
		{"com grupo de transferência não entra", cartao, "30", "ESTORNO LOJA", transacoes.Receita, nil, false, false, &grupo, transacoes.Receita, false, nil, false},
		{"valor negativo não entra", cartao, "-30", "LOJA", transacoes.Receita, nil, false, false, nil, transacoes.Receita, false, nil, false},
		{"valor zero não entra", cartao, "0", "LOJA", transacoes.Receita, nil, false, false, nil, transacoes.Receita, false, nil, false},
		{"valor positivo pequeno entra", cartao, "0.01", "LOJA", transacoes.Receita, nil, false, false, nil, transacoes.Despesa, false, nil, true},
		{"conta corrente não entra", corrente, "30", "ESTORNO LOJA", transacoes.Receita, nil, false, false, nil, transacoes.Receita, false, nil, false},
		{"despesa positiva (já estorno) não entra", cartao, "30", "ESTORNO LOJA", transacoes.Despesa, nil, false, false, nil, transacoes.Despesa, false, nil, false},
		{"já está certa: não é carimbada", cartao, "100", "PAGAMENTO RECEBIDO", transacoes.Transferencia, &pagamento.Id, false, true, nil, transacoes.Transferencia, true, &pagamento.Id, false},
		{"transferência sem categoria, com texto de pagamento, só ganha a categoria", cartao, "100", "PAGAMENTO RECEBIDO", transacoes.Transferencia, nil, false, true, nil, transacoes.Transferencia, true, &pagamento.Id, true},
	}
	ids := make([]int64, len(casos))
	for i, c := range casos {
		ids[i] = fab.Transacao(c.conta, func(tr *testeapoio.Transacao) {
			tr.Valor, tr.Descricao, tr.DescricaoOriginal, tr.Tipo = testeapoio.Valor(c.valor), c.descricao, &c.descricao, c.tipo
			tr.CategoriaId, tr.CategoriaManual, tr.IgnorarRelatorios, tr.TransferenciaGrupo = c.categoria, c.manual, c.ignorar, c.grupo
		}).Id
	}

	rodar := func() int {
		return emTx(t, db, func(tx *sql.Tx) (int, error) { return ReclassificarEntradasNosCartoes(t.Context(), tx, agoraDe(hoje)) })
	}
	esperadas := 0
	for _, c := range casos {
		if c.muda {
			esperadas++
		}
	}
	if n := rodar(); n != esperadas {
		t.Errorf("mudaram %d, esperado %d", n, esperadas)
	}
	for i, c := range casos {
		e := estadoDe(t, db, ids[i])
		if e.tipo != c.depois || e.ignorar != c.ignorarDep || !mesmoId(e.categoria, c.catDepois) || e.mudouEm(hoje) != c.muda {
			t.Errorf("%s: tipo %d, ignorar %v, categoria %v, carimbada %v; esperado %d, %v, %v, %v",
				c.nome, e.tipo, e.ignorar, derreferenciar(e.categoria), e.mudouEm(hoje), c.depois, c.ignorarDep, derreferenciar(c.catDepois), c.muda)
		}
	}
	// Idempotente: o que foi acertado não é acertado de novo.
	if n := rodar(); n != 0 {
		t.Errorf("segunda partida mudou %d", n)
	}
}

func mesmoId(a, b *int64) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

func derreferenciar(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

// Regra: sem a categoria "Pagamento de fatura", o pagamento no cartão fica sem categoria e o que já está como deveria (a
// transferência fora dos relatórios, sem categoria) não é carimbado nem contado.
func TestReclassificarEntradasSemACategoriaDePagamento(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, ontem)
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Tipo = 3 })
	novoPagamento := func(tipo int64, ignorar bool) int64 {
		return fab.Transacao(cartao, func(tr *testeapoio.Transacao) {
			tr.Valor, tr.Descricao, tr.Tipo, tr.IgnorarRelatorios = testeapoio.Valor("100"), "PAGAMENTO RECEBIDO", tipo, ignorar
		}).Id
	}
	jaCerto, comoReceita := novoPagamento(transacoes.Transferencia, true), novoPagamento(transacoes.Receita, false)
	if n := emTx(t, db, func(tx *sql.Tx) (int, error) { return ReclassificarEntradasNosCartoes(t.Context(), tx, agoraDe(hoje)) }); n != 1 {
		t.Errorf("mudaram %d, esperado 1", n)
	}
	if e := estadoDe(t, db, jaCerto); e.tipo != transacoes.Transferencia || !e.ignorar || e.categoria != nil || e.mudouEm(hoje) {
		t.Errorf("a que já estava certa não muda: %+v", e)
	}
	if e := estadoDe(t, db, comoReceita); e.tipo != transacoes.Transferencia || !e.ignorar || e.categoria != nil || !e.mudouEm(hoje) {
		t.Errorf("a que veio como receita vira transferência, sem categoria: %+v", e)
	}
}

// Regra: sem cartão, as rotinas dos cartões não fazem nada (e não falham).
func TestRotinasDosCartoesSemCartao(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, ontem)
	conta := fab.Conta()
	fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Valor, tr.Tipo = testeapoio.Valor("30"), 2 })
	if n := emTx(t, db, func(tx *sql.Tx) (int, error) { return ReclassificarEntradasNosCartoes(t.Context(), tx, agoraDe(hoje)) }); n != 0 {
		t.Errorf("entradas: %d", n)
	}
	if n := emTx(t, db, func(tx *sql.Tx) (int, error) {
		return IgnorarComprasParceladasEstornadas(t.Context(), tx, relogio.Hoje(hoje), agoraDe(hoje))
	}); n != 0 {
		t.Errorf("compras parceladas: %d", n)
	}
}

// ------------------------------------------------------------------------------------ compras parceladas estornadas

// Regra: IgnorarComprasParceladasEstornadas (#257, ADR 0036): o estorno do total de uma compra parcelada sai dos
// relatórios com a compra e as parcelas já lançadas; só olha despesa de cartão, sem grupo de transferência, dos últimos
// 460 dias (400 do estorno da compra mais 60 de folga); só carimba o que passou a ficar fora.
func TestIgnorarComprasParceladasEstornadas(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, ontem)
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Tipo = 3 })
	corrente := fab.Conta()
	hojeEmBrasilia := relogio.Hoje(hoje)
	grupo := tipos.NovoGuid()

	lancar := func(conta testeapoio.Conta, dias int, valor, descricao string, parcela, total int64, ajustes ...func(*testeapoio.Transacao)) int64 {
		return fab.Transacao(conta, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Descricao, tr.DescricaoOriginal = hojeEmBrasilia.SomarDias(dias), testeapoio.Valor(valor), descricao, &descricao
			if total > 0 {
				tr.ParcelaNumero, tr.ParcelaTotal = &parcela, &total
			}
			for _, a := range ajustes {
				a(tr)
			}
		}).Id
	}
	noGrupo := func(tr *testeapoio.Transacao) { tr.TransferenciaGrupo = &grupo }
	receita := func(tr *testeapoio.Transacao) { tr.Tipo = transacoes.Receita }
	jaIgnorada := func(tr *testeapoio.Transacao) { tr.IgnorarRelatorios = true }

	// A compra cancelada: 3 × 100, a 3ª ainda por vir (data de hoje), e o estorno do total.
	c1, c2, c3 := lancar(cartao, -60, "-100", "LOJA ALFA 01/03", 1, 3), lancar(cartao, -30, "-100", "LOJA ALFA 02/03", 2, 3), lancar(cartao, 0, "-100", "LOJA ALFA 03/03", 3, 3)
	e1 := lancar(cartao, -20, "300", "ESTORNO DE LOJA ALFA", 0, 0)
	// Estorno de uma parcela só: não casa.
	p1, parcial := lancar(cartao, -60, "-100", "LOJA BETA 01/03", 1, 3), lancar(cartao, -20, "100", "ESTORNO LOJA BETA", 0, 0)
	// O estorno que o banco ainda manda como receita não entra: é a rotina das entradas nos cartões que o vira despesa.
	r1, receitaEstorno := lancar(cartao, -60, "-100", "LOJA GAMA 01/03", 1, 3), lancar(cartao, -20, "300", "ESTORNO LOJA GAMA", 0, 0, receita)
	// Lançamento com grupo de transferência, de conta corrente: não entram.
	g1, g2 := lancar(cartao, -60, "-100", "LOJA DELTA 01/03", 1, 3, noGrupo), lancar(cartao, -20, "300", "ESTORNO LOJA DELTA", 0, 0)
	k1, k2 := lancar(corrente, -60, "-100", "LOJA EPSILON 01/03", 1, 3), lancar(corrente, -20, "300", "ESTORNO LOJA EPSILON", 0, 0)
	// A janela: a compra de 460 dias atrás entra, a de 461 não.
	j1, j2 := lancar(cartao, -460, "-100", "LOJA ZETA 01/03", 1, 3), lancar(cartao, -300, "300", "ESTORNO LOJA ZETA", 0, 0)
	f1, f2 := lancar(cartao, -461, "-100", "LOJA ETA 01/03", 1, 3), lancar(cartao, -300, "300", "ESTORNO LOJA ETA", 0, 0)
	// Já fora dos relatórios: continua, sem carimbo novo, e fica fora da contagem.
	i1, i2 := lancar(cartao, -60, "-100", "LOJA TETA 01/03", 1, 3, jaIgnorada), lancar(cartao, -20, "300", "ESTORNO LOJA TETA", 0, 0, jaIgnorada)

	rodar := func() int {
		return emTx(t, db, func(tx *sql.Tx) (int, error) {
			return IgnorarComprasParceladasEstornadas(t.Context(), tx, hojeEmBrasilia, agoraDe(hoje))
		})
	}
	// A compra da janela só casa dentro de 400 dias: o estorno j2 está a 160 dias dela. O da compra de 461 dias (f1) não
	// é lido, então o estorno f2 não acha a 1ª parcela.
	if n := rodar(); n != 6 {
		t.Errorf("mudaram %d, esperado 6", n)
	}
	fora := []int64{c1, c2, c3, e1, j1, j2}
	dentro := []int64{p1, parcial, r1, receitaEstorno, g1, g2, k1, k2, f1, f2}
	for _, id := range fora {
		if e := estadoDe(t, db, id); !e.ignorar || !e.mudouEm(hoje) {
			t.Errorf("a transação %d deveria ficar fora dos relatórios, carimbada: %+v", id, e)
		}
	}
	for _, id := range dentro {
		if e := estadoDe(t, db, id); e.ignorar || e.mudouEm(hoje) {
			t.Errorf("a transação %d não deveria mudar: %+v", id, e)
		}
	}
	for _, id := range []int64{i1, i2} {
		if e := estadoDe(t, db, id); !e.ignorar || e.mudouEm(hoje) {
			t.Errorf("a transação %d já estava fora e não leva carimbo novo: %+v", id, e)
		}
	}
	// Idempotente.
	if n := rodar(); n != 0 {
		t.Errorf("segunda partida mudou %d", n)
	}
}

// ------------------------------------------------------------------------------------ erro do banco

// Regra: o erro do banco sobe com o contexto de onde veio, em cada ponto de cada acerto, e nenhum devolve "mudou" no meio
// do caminho: é o que a partida registra como aviso. O banco recusa de verdade: tabela que some do caminho (renomeada) ou
// gatilho que aborta a gravação.
func TestAcertosPropagamOErroDoBanco(t *testing.T) {
	t.Parallel()
	recusar := func(tabela, quando string) string {
		return fmt.Sprintf(`CREATE TRIGGER recusa BEFORE %s ON %s BEGIN SELECT RAISE(ABORT, 'banco recusou'); END`, quando, tabela)
	}
	sumir := func(tabela string) string { return fmt.Sprintf(`ALTER TABLE %s RENAME TO %sSumiu`, tabela, tabela) }
	casos := []struct {
		nome     string
		sabotar  string
		acerto   string
		esperado string
	}{
		{"regras: ler a marca", sumir("Configuracao"), "regras", "configuração"},
		{"regras: gravar a marca", recusar("Configuracao", "INSERT"), "regras", "banco recusou"},
		{"regras: achar a categoria", sumir("Categoria"), "regras", "categorias de transferência"},
		{"regras: carregar as regras", sumir("CategoriaPluggy"), "regras", "CategoriaPluggy"},
		{"regras: criar uma regra", recusar("RegraCategorizacao", "INSERT"), "regras", "criar a regra padrão"},
		{"regras: achar as transações", sumir("Transacao"), "regras", "ler transações"},
		{"regras: gravar a transação", recusar("Transacao", "UPDATE"), "regras", "gravar a transação"},
		{"entradas: carregar as regras", sumir("CategoriaPluggy"), "entradas", "CategoriaPluggy"},
		{"entradas: achar as transações", sumir("Transacao"), "entradas", "ler transações"},
		{"entradas: gravar a transação", recusar("Transacao", "UPDATE"), "entradas", "gravar a transação"},
		{"compras: achar as transações", sumir("Transacao"), "compras", "ler transações"},
		{"compras: gravar a transação", recusar("Transacao", "UPDATE"), "compras", "gravar a transação"},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			t.Parallel()
			c := novaCenaDeRegras(t)
			cartao := c.fab.Conta(func(x *testeapoio.Conta) { x.Tipo = 3 })
			c.fab.Transacao(c.conta, func(tr *testeapoio.Transacao) { tr.Descricao = "MELI DOLAR" })
			c.fab.Transacao(cartao, func(tr *testeapoio.Transacao) { tr.Valor, tr.Tipo = testeapoio.Valor("50"), 2 })
			parcela, total := int64(1), int64(3)
			c.fab.Transacao(cartao, func(tr *testeapoio.Transacao) {
				tr.Descricao, tr.Valor, tr.ParcelaNumero, tr.ParcelaTotal = "LOJA 01/03", testeapoio.Valor("-100"), &parcela, &total
			})
			c.fab.Transacao(cartao, func(tr *testeapoio.Transacao) { tr.Descricao, tr.Valor = "ESTORNO LOJA", testeapoio.Valor("300") })
			if _, err := c.db.Exec(caso.sabotar); err != nil {
				t.Fatal(err)
			}
			tx, err := c.db.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			switch caso.acerto {
			case "regras":
				_, err = CriarRegrasPadrao(t.Context(), tx, hoje)
			case "entradas":
				_, err = ReclassificarEntradasNosCartoes(t.Context(), tx, agoraDe(hoje))
			default:
				_, err = IgnorarComprasParceladasEstornadas(t.Context(), tx, relogio.Hoje(hoje), agoraDe(hoje))
			}
			if err == nil || !strings.Contains(err.Error(), caso.esperado) {
				t.Errorf("erro %v, esperado um com %q", err, caso.esperado)
			}
		})
	}
}
