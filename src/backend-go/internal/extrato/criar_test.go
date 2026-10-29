package extrato

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// Regra: o lançamento manual: a descrição e a observação aparadas, o sinal pelo tipo (receita positiva, o resto
// negativo, a escala que veio), a categoria das regras quando não vem (e a escolhida, manual), o tipo pela categoria,
// emergência só para saída, ignorar fora das transferências, 201 com o Location e o DTO lido do banco.
func TestCriar(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta(func(x *testeapoio.Conta) { x.Nome = "Carteira" })
	mercado := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome = "Mercado" })
	fab.RegraCategorizacao(mercado, func(r *testeapoio.RegraCategorizacao) { r.Texto = "FEIRA" })
	salario := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Salário", 2 })

	resp := testeapoio.Chamar(t, mux, "POST", "/api/transacoes", fmt.Sprintf(`{"contaId":%d,"data":"2026-10-10",
		"descricao":"  Feira do bairro ","valor":120.350,"tipo":1,"observacao":"  ","ignorarRelatorios":true,"emergencia":true}`, conta.Id))
	resp.ConferirStatus(t, http.StatusCreated)
	var criada struct{ Id int64 }
	if err := json.Unmarshal(resp.Corpo, &criada); err != nil {
		t.Fatal(err)
	}
	if local := resp.Cabecalho.Get("Location"); local != fmt.Sprintf("/api/transacoes/%d", criada.Id) {
		t.Errorf("Location = %q", local)
	}
	resp.ConferirJSON(t, fmt.Sprintf(`{"id":%d,"contaId":%d,"contaNome":"Carteira","openFinance":false,"data":"2026-10-10",
		"descricao":"Feira do bairro","descricaoOriginal":null,"estabelecimento":null,"valor":-120.35,"tipo":1,"status":1,
		"categoriaId":%d,"categoriaNome":"Mercado","categoriaCor":"#64748b","parcelaNumero":null,"parcelaTotal":null,
		"ignorarRelatorios":true,"observacao":null,"emergencia":true,"tipoLancamento":0,"regraDescricaoId":null}`,
		criada.Id, conta.Id, mercado.Id))
	var manual, emergencia int64
	var valor, criadoEm string
	if err := db.QueryRow(`SELECT CategoriaManual, Emergencia, Valor, CriadoEm FROM Transacao WHERE Id = ?`, criada.Id).
		Scan(&manual, &emergencia, &valor, &criadoEm); err != nil {
		t.Fatal(err)
	}
	if manual != 0 || emergencia != 1 || valor != "-120.35" || criadoEm != "2026-10-15 15:00:00" {
		t.Errorf("gravado: manual %d, emergência %d, valor %q, criado em %q", manual, emergencia, valor, criadoEm)
	}

	receita := testeapoio.Chamar(t, mux, "POST", "/api/transacoes", fmt.Sprintf(`{"contaId":%d,"data":"2026-10-11",
		"descricao":"Bico","valor":50,"tipo":2,"categoriaId":%d,"observacao":" pago em dinheiro ","emergencia":true,"status":2}`,
		conta.Id, salario.Id))
	receita.ConferirStatus(t, http.StatusCreated)
	var r struct {
		Valor                   json.RawMessage
		Tipo, Status            int64
		Emergencia              bool
		Observacao, CategoriaId *json.RawMessage
	}
	if err := json.Unmarshal(receita.Corpo, &r); err != nil {
		t.Fatal(err)
	}
	if string(r.Valor) != "50.0" || r.Tipo != 2 || r.Status != 2 || r.Emergencia || string(*r.Observacao) != `"pago em dinheiro"` {
		t.Errorf("receita: %s", receita.Corpo)
	}
}

// Regra: depois de gravar, as transferências são conferidas de novo (ADR 0031): lançado em "Transferências" sem o
// outro lado numa conta sua, volta a ser despesa e entra nos relatórios; com o par (mesmo valor, outra conta, até 5
// dias), continua transferência fora dos relatórios.
func TestCriarAcertaAsTransferencias(t *testing.T) {
	mux, _, fab := rotasDeTeste(t, minimoFixo{n: 4})
	transferencias := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Transferências", 3 })
	corrente := fab.Conta()
	poupanca := fab.Conta(func(x *testeapoio.Conta) { x.Tipo = 2 })
	entrada := fab.Transacao(poupanca, func(x *testeapoio.Transacao) { x.Valor, x.Tipo = testeapoio.Valor("300.0"), 2 })
	lancar := func(valor string) (tipo int64, ignorar bool) {
		t.Helper()
		resp := testeapoio.Chamar(t, mux, "POST", "/api/transacoes", fmt.Sprintf(`{"contaId":%d,"data":"%s",
			"descricao":"TRANSFERENCIA","valor":%s,"tipo":1,"categoriaId":%d}`, corrente.Id, entrada.Data, valor, transferencias.Id))
		resp.ConferirStatus(t, http.StatusCreated)
		var dto struct {
			Tipo              int64
			IgnorarRelatorios bool
		}
		if err := json.Unmarshal(resp.Corpo, &dto); err != nil {
			t.Fatal(err)
		}
		return dto.Tipo, dto.IgnorarRelatorios
	}
	if tipo, ignorar := lancar("40"); tipo != 1 || ignorar {
		t.Errorf("sem par: tipo %d, ignorar %v; esperado despesa nos relatórios", tipo, ignorar)
	}
	if tipo, ignorar := lancar("300"); tipo != 3 || !ignorar {
		t.Errorf("com par: tipo %d, ignorar %v; esperado transferência fora dos relatórios", tipo, ignorar)
	}
}

// Regra: as mensagens do lançamento manual (o ValidarManualAsync) e as do model binding do SalvarTransacaoDto.
func TestCriarComErro(t *testing.T) {
	mux, _, fab := rotasDeTeste(t, minimoFixo{n: 4})
	manual := fab.Conta()
	conexao := fab.ConexaoBancaria()
	daConexao := fab.Conta(func(x *testeapoio.Conta) { x.Origem, x.ConexaoBancariaId = 2, &conexao.Id })
	corpo := func(conta int64, extra string) string {
		return fmt.Sprintf(`{"contaId":%d,"data":"2026-10-10","descricao":"Compra","valor":10,"tipo":1%s}`, conta, extra)
	}
	mensagens := []struct{ corpo, mensagem string }{
		{corpo(manual.Id, `,"tipo":3`), "Para transferências use a opção 'Transferência'."},
		{corpo(999, ""), "Conta inválida."},
		{corpo(daConexao.Id, ""), "Não é possível lançar manualmente em contas do Open Finance. Os lançamentos chegam pela sincronização."},
		{corpo(manual.Id, `,"categoriaId":999`), "Categoria inválida."},
	}
	for _, m := range mensagens {
		resp := testeapoio.Chamar(t, mux, "POST", "/api/transacoes", m.corpo)
		resp.ConferirStatus(t, http.StatusBadRequest)
		resp.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, m.mensagem))
	}
	resp := testeapoio.Chamar(t, mux, "POST", "/api/transacoes", fmt.Sprintf(`{"contaId":%d,"data":"2026-10-10",
		"descricao":"  ","valor":0,"tipo":1,"observacao":"%s"}`, manual.Id, fmt.Sprintf("%01001d", 0)))
	resp.ConferirStatus(t, http.StatusBadRequest)
	var problema struct{ Errors json.RawMessage }
	if err := json.Unmarshal(resp.Corpo, &problema); err != nil {
		t.Fatal(err)
	}
	testeapoio.ConferirJSON(t, problema.Errors, `{"Descricao":["The Descricao field is required."],
		"Valor":["The field Valor must be between 0,01 and 999999999."],
		"Observacao":["The field Observacao must be a string with a maximum length of 1000."]}`)
	for _, data := range []string{`"10/10/2026"`, `"2026-10-1"`, `null`, `20261010`} {
		r := testeapoio.Chamar(t, mux, "POST", "/api/transacoes", fmt.Sprintf(`{"contaId":%d,"data":%s,"descricao":"x","valor":1,"tipo":1}`,
			manual.Id, data))
		r.ConferirStatus(t, http.StatusBadRequest)
	}
}
