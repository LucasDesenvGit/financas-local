package extrato

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// coluna lê uma coluna de uma transação como o SQLite guarda (texto, número ou NULL).
func coluna(t *testing.T, db *sql.DB, id int64, nome string) any {
	t.Helper()
	var v any
	if err := db.QueryRow(`SELECT "`+nome+`" FROM "Transacao" WHERE "Id" = ?`, id).Scan(&v); err != nil {
		t.Fatalf("ler %s de %d: %v", nome, id, err)
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

func existe(t *testing.T, db *sql.DB, id int64) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "Transacao" WHERE "Id" = ?`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func doOpenFinance(x *testeapoio.Transacao) {
	id := fmt.Sprintf("tx-%s", x.Descricao)
	x.PluggyTransactionId = &id
}

// Regra: alterar um lançamento manual: as regras do lançamento novo (aparar, o sinal pelo tipo, o status, a
// observação em branco nula, a categoria das regras sem ela, emergência só para saída) e a conta nova; só as colunas
// que mudaram vão ao banco.
func TestAtualizar(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	carteira := fab.Conta(func(x *testeapoio.Conta) { x.Nome = "Carteira" })
	poupanca := fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Tipo = "Poupança", 2 })
	mercado := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome = "Mercado" })
	fab.RegraCategorizacao(mercado, func(r *testeapoio.RegraCategorizacao) { r.Texto = "FEIRA" })
	salario := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Salário", 2 })
	feira := fab.Transacao(carteira, func(x *testeapoio.Transacao) {
		x.Descricao, x.Valor, x.CategoriaId, x.CategoriaManual = "Feira", testeapoio.Valor("-60.0"), &salario.Id, true
	})

	resp := testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/transacoes/%d", feira.Id), fmt.Sprintf(`{"contaId":%d,
		"data":"2026-10-11","descricao":"  Feira do sábado ","valor":65.5,"tipo":1,"categoriaId":null,"observacao":" orgânicos ",
		"ignorarRelatorios":false,"emergencia":true}`, carteira.Id))
	resp.ConferirStatus(t, http.StatusOK)
	resp.ConferirJSON(t, fmt.Sprintf(`{"id":%d,"contaId":%d,"contaNome":"Carteira","openFinance":false,"data":"2026-10-11",
		"descricao":"Feira do sábado","descricaoOriginal":null,"estabelecimento":null,"valor":-65.5,"tipo":1,"status":1,
		"categoriaId":%d,"categoriaNome":"Mercado","categoriaCor":"#64748b","parcelaNumero":null,"parcelaTotal":null,
		"ignorarRelatorios":false,"observacao":"orgânicos","emergencia":true,"tipoLancamento":0,"regraDescricaoId":null}`,
		feira.Id, carteira.Id, mercado.Id))
	if manual := coluna(t, db, feira.Id, "CategoriaManual"); manual != int64(0) {
		t.Errorf("sem categoria, deixa de ser manual: %v", manual)
	}
	if em := coluna(t, db, feira.Id, "AtualizadoEm"); em != "2026-10-15 15:00:00" {
		t.Errorf("AtualizadoEm = %v", em)
	}

	resp = testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/transacoes/%d", feira.Id), fmt.Sprintf(`{"contaId":%d,
		"data":"2026-10-12","descricao":"Venda","valor":12.75,"tipo":2,"categoriaId":%d,"observacao":"   ",
		"ignorarRelatorios":true,"status":2,"emergencia":true}`, poupanca.Id, salario.Id))
	resp.ConferirStatus(t, http.StatusOK)
	var dto struct {
		ContaId           int64
		Valor             json.RawMessage
		Tipo, Status      int64
		IgnorarRelatorios bool
		Emergencia        bool
		Observacao        *string
	}
	if err := json.Unmarshal(resp.Corpo, &dto); err != nil {
		t.Fatal(err)
	}
	if dto.ContaId != poupanca.Id || string(dto.Valor) != "12.75" || dto.Tipo != 2 || dto.Status != 2 || !dto.IgnorarRelatorios ||
		dto.Emergencia || dto.Observacao != nil {
		t.Errorf("receita em outra conta: %s", resp.Corpo)
	}
	if manual := coluna(t, db, feira.Id, "CategoriaManual"); manual != int64(1) {
		t.Errorf("com categoria, fica manual: %v", manual)
	}
}

// Regra: o que não se altera pelo PUT: a do Open Finance, a transferência (com par), a inexistente e o id que não é
// long; a validação do corpo vem antes de procurar a transação; as mensagens do lançamento manual valem.
func TestAtualizarComErro(t *testing.T) {
	mux, _, fab := rotasDeTeste(t, minimoFixo{n: 4})
	carteira := fab.Conta()
	conexao := fab.ConexaoBancaria()
	banco := fab.Conta(func(x *testeapoio.Conta) { x.Origem, x.ConexaoBancariaId = 2, &conexao.Id })
	manual := fab.Transacao(carteira)
	aberta := fab.Transacao(banco, doOpenFinance)
	grupo := tipos.NovoGuid()
	transferencia := fab.Transacao(carteira, func(x *testeapoio.Transacao) { x.TransferenciaGrupo = &grupo })
	corpo := func(conta int64, extra string) string {
		return fmt.Sprintf(`{"contaId":%d,"data":"2026-10-10","descricao":"Compra","valor":10,"tipo":1%s}`, conta, extra)
	}
	casos := []struct {
		nome, caminho, corpo string
		status               int
		mensagem             string
	}{
		{"Open Finance", fmt.Sprint(aberta.Id), corpo(carteira.Id, ""), 400,
			"Transações do Open Finance só permitem alterar descrição, categoria e observação."},
		{"transferência", fmt.Sprint(transferencia.Id), corpo(carteira.Id, ""), 400,
			"Para alterar uma transferência, exclua-a e lance novamente."},
		{"inexistente", "999", corpo(carteira.Id, ""), 404, "Registro não encontrado."},
		{"conta do Open Finance", fmt.Sprint(manual.Id), corpo(banco.Id, ""), 400,
			"Não é possível lançar manualmente em contas do Open Finance. Os lançamentos chegam pela sincronização."},
		{"como transferência", fmt.Sprint(manual.Id), corpo(carteira.Id, `,"tipo":3`), 400,
			"Para transferências use a opção 'Transferência'."},
		{"categoria inexistente", fmt.Sprint(manual.Id), corpo(carteira.Id, `,"categoriaId":999`), 400, "Categoria inválida."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			resp := testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/"+c.caminho, c.corpo)
			resp.ConferirStatus(t, c.status)
			resp.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, c.mensagem))
		})
	}
	t.Run("id que não é long", func(t *testing.T) {
		resp := testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/abc", corpo(carteira.Id, ""))
		resp.ConferirStatus(t, http.StatusNotFound)
	})
	t.Run("corpo inválido antes do 404", func(t *testing.T) {
		resp := testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/999", corpo(carteira.Id, `,"valor":0`))
		resp.ConferirStatus(t, http.StatusBadRequest)
		if !strings.Contains(string(resp.Corpo), "The field Valor must be between 0,01 and 999999999.") {
			t.Errorf("%s", resp.Corpo)
		}
	})
}

// Regra: ajustar qualquer transação: o nome trocado fica escolhido à mão (e sem a regra que o deu, #123); o mesmo nome
// não muda nada; a categoria escolhida fica manual, e sem ela as regras escolhem de novo; ignorar nos relatórios não
// vale para transferência; emergência só para saída; a observação aparada.
func TestAjustar(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conexao := fab.ConexaoBancaria()
	banco := fab.Conta(func(x *testeapoio.Conta) { x.Origem, x.ConexaoBancariaId = 2, &conexao.Id })
	mercado := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome = "Mercado" })
	fab.RegraCategorizacao(mercado, func(r *testeapoio.RegraCategorizacao) { r.Texto = "SUPERMERCADO" })
	moradia := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome = "Moradia" })
	transferencias := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Transferências", 3 })
	res, err := db.Exec(`INSERT INTO RegraDescricao (Texto, TipoCorrespondencia, NovaDescricao, Ativa, CriadoEm)
		VALUES ('PIX', 2, 'Pix', 1, '2026-10-01 00:00:00')`)
	if err != nil {
		t.Fatal(err)
	}
	regra, _ := res.LastInsertId()
	pix := fab.Transacao(banco, doOpenFinance, func(x *testeapoio.Transacao) {
		x.Descricao, x.RegraDescricaoId = "Pix", &regra
	})
	mercadoOF := fab.Transacao(banco, doOpenFinance, func(x *testeapoio.Transacao) {
		x.Descricao, x.CategoriaId, x.CategoriaManual = "SUPERMERCADO EXEMPLO", &moradia.Id, true
	})
	receita := fab.Transacao(banco, doOpenFinance, func(x *testeapoio.Transacao) {
		x.Descricao, x.Valor, x.Tipo = "PIX RECEBIDO", testeapoio.Valor("120.0"), 2
	})

	resp := testeapoio.Chamar(t, mux, "PATCH", fmt.Sprintf("/api/transacoes/%d", pix.Id), fmt.Sprintf(`{"descricao":" Pix para o síndico ",
		"categoriaId":%d,"observacao":" condomínio ","ignorarRelatorios":false,"emergencia":true}`, moradia.Id))
	resp.ConferirStatus(t, http.StatusOK)
	if d, m, r := coluna(t, db, pix.Id, "Descricao"), coluna(t, db, pix.Id, "DescricaoManual"), coluna(t, db, pix.Id, "RegraDescricaoId"); d != "Pix para o síndico" || m != int64(1) || r != nil {
		t.Errorf("nome escolhido à mão: %v, manual %v, regra %v", d, m, r)
	}
	if o, e, c := coluna(t, db, pix.Id, "Observacao"), coluna(t, db, pix.Id, "Emergencia"), coluna(t, db, pix.Id, "CategoriaManual"); o != "condomínio" || e != int64(1) || c != int64(1) {
		t.Errorf("observação %v, emergência %v, manual %v", o, e, c)
	}

	resp = testeapoio.Chamar(t, mux, "PATCH", fmt.Sprintf("/api/transacoes/%d", mercadoOF.Id),
		`{"descricao":"SUPERMERCADO EXEMPLO","categoriaId":null,"observacao":null,"ignorarRelatorios":true}`)
	resp.ConferirStatus(t, http.StatusOK)
	if c, m, d := coluna(t, db, mercadoOF.Id, "CategoriaId"), coluna(t, db, mercadoOF.Id, "CategoriaManual"), coluna(t, db, mercadoOF.Id, "DescricaoManual"); c != mercado.Id || m != int64(0) || d != int64(0) {
		t.Errorf("automática: categoria %v, manual %v, nome manual %v", c, m, d)
	}
	if i := coluna(t, db, mercadoOF.Id, "IgnorarRelatorios"); i != int64(1) {
		t.Errorf("ignorar: %v", i)
	}

	resp = testeapoio.Chamar(t, mux, "PATCH", fmt.Sprintf("/api/transacoes/%d", receita.Id),
		`{"descricao":"PIX RECEBIDO","emergencia":true}`)
	resp.ConferirStatus(t, http.StatusOK)
	if e := coluna(t, db, receita.Id, "Emergencia"); e != int64(0) {
		t.Errorf("emergência numa entrada: %v", e)
	}

	// Em Transferências, com o par na outra conta: transferência, e o ignorar do corpo não vale.
	poupanca := fab.Conta(func(x *testeapoio.Conta) { x.Tipo = 2 })
	saida := fab.Transacao(banco, doOpenFinance, func(x *testeapoio.Transacao) { x.Descricao, x.Valor = "TED", testeapoio.Valor("-300.0") })
	fab.Transacao(poupanca, func(x *testeapoio.Transacao) { x.Valor, x.Tipo = testeapoio.Valor("300.0"), 2 })
	resp = testeapoio.Chamar(t, mux, "PATCH", fmt.Sprintf("/api/transacoes/%d", saida.Id),
		fmt.Sprintf(`{"descricao":"TED","categoriaId":%d,"ignorarRelatorios":false}`, transferencias.Id))
	resp.ConferirStatus(t, http.StatusOK)
	if tipo, ignorar := coluna(t, db, saida.Id, "Tipo"), coluna(t, db, saida.Id, "IgnorarRelatorios"); tipo != int64(3) || ignorar != int64(1) {
		t.Errorf("transferência: tipo %v, ignorar %v", tipo, ignorar)
	}

	casos := []struct {
		nome, caminho, corpo string
		status               int
	}{
		{"categoria inexistente", fmt.Sprint(pix.Id), `{"descricao":"Pix","categoriaId":999}`, 400},
		{"inexistente", "999", `{"descricao":"Pix"}`, 404},
		{"id que não é long", "abc", `{"descricao":"Pix"}`, 404},
		{"sem descrição", fmt.Sprint(pix.Id), `{"descricao":""}`, 400},
		{"descrição longa demais", fmt.Sprint(pix.Id), `{"descricao":"` + strings.Repeat("x", 301) + `"}`, 400},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/"+c.caminho, c.corpo).ConferirStatus(t, c.status)
		})
	}
}

// Regra: excluir: só lançamento manual; a transferência sai com o par (o mesmo grupo, inclusive o lado do Open
// Finance); depois, as transferências conferidas de novo.
func TestExcluir(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	carteira := fab.Conta()
	conexao := fab.ConexaoBancaria()
	banco := fab.Conta(func(x *testeapoio.Conta) { x.Origem, x.ConexaoBancariaId = 2, &conexao.Id })
	manual := fab.Transacao(carteira)
	outra := fab.Transacao(carteira)
	aberta := fab.Transacao(banco, doOpenFinance)
	grupo := tipos.NovoGuid()
	lado := fab.Transacao(carteira, func(x *testeapoio.Transacao) { x.TransferenciaGrupo = &grupo })
	par := fab.Transacao(banco, doOpenFinance, func(x *testeapoio.Transacao) { x.TransferenciaGrupo = &grupo })

	testeapoio.Chamar(t, mux, "DELETE", fmt.Sprintf("/api/transacoes/%d", manual.Id), "").ConferirStatus(t, http.StatusNoContent)
	if existe(t, db, manual.Id) || !existe(t, db, outra.Id) {
		t.Error("excluir apagou a errada")
	}
	testeapoio.Chamar(t, mux, "DELETE", fmt.Sprintf("/api/transacoes/%d", lado.Id), "").ConferirStatus(t, http.StatusNoContent)
	if existe(t, db, lado.Id) || existe(t, db, par.Id) {
		t.Error("a transferência não saiu com o par")
	}
	resp := testeapoio.Chamar(t, mux, "DELETE", fmt.Sprintf("/api/transacoes/%d", aberta.Id), "")
	resp.ConferirStatus(t, http.StatusBadRequest)
	resp.ConferirJSON(t, `{"mensagem":"Transações do Open Finance não podem ser excluídas. Marque 'Ignorar nos relatórios' se necessário."}`)
	testeapoio.Chamar(t, mux, "DELETE", "/api/transacoes/999", "").ConferirStatus(t, http.StatusNotFound)
	testeapoio.Chamar(t, mux, "DELETE", "/api/transacoes/abc", "").ConferirStatus(t, http.StatusNotFound)
}

// Regra: emergência em lote: marcar só as saídas, desmarcar todas; a conta é das linhas que casam (inclusive as que já
// estavam assim), e a repetida conta uma vez.
func TestEmergenciaEmLote(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	saida := fab.Transacao(conta)
	entrada := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Valor, x.Tipo = testeapoio.Valor("50.0"), 2 })
	ja := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Emergencia = true })

	resp := testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/emergencia-em-lote",
		fmt.Sprintf(`{"ids":[%d,%d,%d,%d,999],"emergencia":true}`, saida.Id, entrada.Id, ja.Id, saida.Id))
	resp.ConferirStatus(t, http.StatusOK)
	resp.ConferirJSON(t, `{"transacoesAtualizadas":2}`)
	if e := coluna(t, db, saida.Id, "Emergencia"); e != int64(1) {
		t.Errorf("a saída não foi marcada: %v", e)
	}
	if e := coluna(t, db, entrada.Id, "Emergencia"); e != int64(0) {
		t.Errorf("a entrada foi marcada: %v", e)
	}
	resp = testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/emergencia-em-lote",
		fmt.Sprintf(`{"ids":["%d",%d],"emergencia":false}`, saida.Id, entrada.Id))
	resp.ConferirStatus(t, http.StatusOK)
	resp.ConferirJSON(t, `{"transacoesAtualizadas":2}`)
	if e := coluna(t, db, saida.Id, "Emergencia"); e != int64(0) {
		t.Errorf("a saída não foi desmarcada: %v", e)
	}
}

// Regra: revisada em lote (#9): marca e desmarca; a inexistente não conta.
func TestRevisadaEmLote(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	pix := fab.Transacao(fab.Conta(), func(x *testeapoio.Transacao) { x.Descricao = "PIX ENVIADO" })
	resp := testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/revisada-em-lote", fmt.Sprintf(`{"ids":[%d,999],"revisada":true}`, pix.Id))
	resp.ConferirStatus(t, http.StatusOK)
	resp.ConferirJSON(t, `{"transacoesAtualizadas":1}`)
	if r := coluna(t, db, pix.Id, "DescricaoRevisada"); r != int64(1) {
		t.Errorf("revisada: %v", r)
	}
	testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/revisada-em-lote", fmt.Sprintf(`{"ids":[%d]}`, pix.Id)).ConferirStatus(t, http.StatusOK)
	if r := coluna(t, db, pix.Id, "DescricaoRevisada"); r != int64(0) {
		t.Errorf("desmarcada: %v", r)
	}
}

// Regra: categoria em lote: a escolhida fica manual, sem ela as regras escolhem; o tipo acompanha; a conta é das
// transações achadas; categoria inexistente é 400.
func TestCategoriaEmLote(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	mercado := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome = "Mercado" })
	fab.RegraCategorizacao(mercado, func(r *testeapoio.RegraCategorizacao) { r.Texto = "FEIRA" })
	salario := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Salário", 2 })
	feira := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao = "FEIRA" })
	bico := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao, x.Valor, x.Tipo = "BICO", testeapoio.Valor("80.0"), 2 })

	resp := testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/categoria-em-lote",
		fmt.Sprintf(`{"ids":[%d,%d,%d,999],"categoriaId":%d}`, feira.Id, bico.Id, feira.Id, salario.Id))
	resp.ConferirStatus(t, http.StatusOK)
	resp.ConferirJSON(t, `{"transacoesAtualizadas":2}`)
	if c, m := coluna(t, db, feira.Id, "CategoriaId"), coluna(t, db, feira.Id, "CategoriaManual"); c != salario.Id || m != int64(1) {
		t.Errorf("escolhida: %v, manual %v", c, m)
	}
	resp = testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/categoria-em-lote", fmt.Sprintf(`{"ids":[%d],"categoriaId":null}`, feira.Id))
	resp.ConferirStatus(t, http.StatusOK)
	if c, m := coluna(t, db, feira.Id, "CategoriaId"), coluna(t, db, feira.Id, "CategoriaManual"); c != mercado.Id || m != int64(0) {
		t.Errorf("automática: %v, manual %v", c, m)
	}
	resp = testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/categoria-em-lote", fmt.Sprintf(`{"ids":[%d],"categoriaId":999}`, feira.Id))
	resp.ConferirStatus(t, http.StatusBadRequest)
	resp.ConferirJSON(t, `{"mensagem":"Categoria inválida."}`)
}

// Regra: transferência: os dois lados nas contas manuais (o mesmo grupo, a categoria Transferências escolhida, fora
// dos relatórios), só o lado manual quando a outra é do Open Finance; a descrição padrão; as mensagens.
func TestTransferir(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	transferencias := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Transferências", 3 })
	poupanca := fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Tipo = "Poupança", 2 })
	carteira := fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Tipo = "Carteira", 5 })
	conexao := fab.ConexaoBancaria()
	banco := fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Origem, x.ConexaoBancariaId = "Banco", 2, &conexao.Id })
	banco2 := fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Origem, x.ConexaoBancariaId = "Banco 2", 2, &conexao.Id })

	resp := testeapoio.Chamar(t, mux, "POST", "/api/transacoes/transferencia", fmt.Sprintf(`{"contaOrigemId":%d,
		"contaDestinoId":%d,"valor":200.5,"data":"2026-10-13","descricao":"  Saque "}`, poupanca.Id, carteira.Id))
	resp.ConferirStatus(t, http.StatusNoContent)
	linhas, err := db.Query(`SELECT "ContaId", "Descricao", "Valor", "Tipo", "CategoriaId", "CategoriaManual",
		"IgnorarRelatorios", "TransferenciaGrupo", "Data" FROM "Transacao" ORDER BY "Id"`)
	if err != nil {
		t.Fatal(err)
	}
	var lados []string
	grupos := map[string]bool{}
	for linhas.Next() {
		var conta, tipo, categoria, manual, ignorar int64
		var descricao, valor, grupo, data string
		if err := linhas.Scan(&conta, &descricao, &valor, &tipo, &categoria, &manual, &ignorar, &grupo, &data); err != nil {
			t.Fatal(err)
		}
		grupos[grupo] = true
		lados = append(lados, fmt.Sprintf("%d|%s|%s|%d|%v|%d|%d|%s", conta, descricao, valor, tipo, categoria == transferencias.Id,
			manual, ignorar, data))
	}
	linhas.Close()
	esperado := []string{
		fmt.Sprintf("%d|Saque para Carteira|-200.5|3|true|1|1|2026-10-13", poupanca.Id),
		fmt.Sprintf("%d|Saque de Poupança|200.5|3|true|1|1|2026-10-13", carteira.Id),
	}
	if strings.Join(lados, "\n") != strings.Join(esperado, "\n") || len(grupos) != 1 {
		t.Errorf("lados:\n%s\ngrupos: %v", strings.Join(lados, "\n"), grupos)
	}

	resp = testeapoio.Chamar(t, mux, "POST", "/api/transacoes/transferencia", fmt.Sprintf(`{"contaOrigemId":%d,
		"contaDestinoId":%d,"valor":10,"data":"2026-10-13"}`, banco.Id, carteira.Id))
	resp.ConferirStatus(t, http.StatusNoContent)
	var descricao, valor string
	var n int
	if err := db.QueryRow(`SELECT COUNT(*), MAX("Descricao"), MAX("Valor") FROM "Transacao" WHERE "Id" > 2`).Scan(&n, &descricao, &valor); err != nil {
		t.Fatal(err)
	}
	if n != 1 || descricao != "Transferência de Banco" || valor != "10.0" {
		t.Errorf("do Open Finance: %d lado(s), %q, %q", n, descricao, valor)
	}

	casos := []struct {
		nome, corpo, mensagem string
	}{
		{"a mesma conta", fmt.Sprintf(`{"contaOrigemId":%d,"contaDestinoId":%d,"valor":1,"data":"2026-10-13"}`, poupanca.Id, poupanca.Id),
			"Escolha contas diferentes."},
		{"conta inexistente", fmt.Sprintf(`{"contaOrigemId":%d,"contaDestinoId":999,"valor":1,"data":"2026-10-13"}`, poupanca.Id),
			"Conta inválida."},
		{"entre contas do Open Finance", fmt.Sprintf(`{"contaOrigemId":%d,"contaDestinoId":%d,"valor":1,"data":"2026-10-13"}`, banco.Id, banco2.Id),
			"Entre contas do Open Finance as transferências são importadas automaticamente."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			resp := testeapoio.Chamar(t, mux, "POST", "/api/transacoes/transferencia", c.corpo)
			resp.ConferirStatus(t, http.StatusBadRequest)
			resp.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, c.mensagem))
		})
	}
	t.Run("valor zero", func(t *testing.T) {
		resp := testeapoio.Chamar(t, mux, "POST", "/api/transacoes/transferencia",
			fmt.Sprintf(`{"contaOrigemId":%d,"contaDestinoId":%d,"valor":0,"data":"2026-10-13"}`, poupanca.Id, carteira.Id))
		resp.ConferirStatus(t, http.StatusBadRequest)
	})
}

// Regra: o model binding da lista de ids: [Required] (sem a lista ou nula), [MinLength(1)], [MaxLength(1000)], número
// em texto vale, e o item que não é número ou a lista que não é lista são erro do JSON.
func TestListaDeIds(t *testing.T) {
	mux, _, fab := rotasDeTeste(t, minimoFixo{n: 4})
	fab.Conta()
	mil := make([]string, 1001)
	for i := range mil {
		mil[i] = fmt.Sprint(i + 1)
	}
	casos := []struct {
		nome, corpo, erro string
	}{
		{"sem a lista", `{"emergencia":true}`, `"Ids":["The Ids field is required."]`},
		{"a lista nula", `{"ids":null}`, `"Ids":["The Ids field is required."]`},
		{"vazia", `{"ids":[]}`, `"Ids":["The field Ids must be a string or array type with a minimum length of '1'."]`},
		{"1001 ids", `{"ids":[` + strings.Join(mil, ",") + `]}`,
			`"Ids":["The field Ids must be a string or array type with a maximum length of '1000'."]`},
		{"item que não é número", `{"ids":[1,"x"]}`, `"$.ids[1]"`},
		{"não é lista", `{"ids":5}`, `"$.ids"`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			resp := testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/emergencia-em-lote", c.corpo)
			resp.ConferirStatus(t, http.StatusBadRequest)
			if !strings.Contains(string(resp.Corpo), c.erro) {
				t.Errorf("sem %s em %s", c.erro, resp.Corpo)
			}
		})
	}
}
