package extrato

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
)

// Regra (#758, ADR 0063): o "Novo lançamento" grava o gasto pago direto de um cofrinho numa conta do Open Finance,
// como o "registrar saída" das Reservas: lançamento do app (sem PluggyTransactionId), despesa, com o nome do cofrinho
// aparado em ReservaNome, e o DTO devolve o cofrinho.
func TestCriarPeloCofrinho(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conexao := fab.ConexaoBancaria()
	mp := fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Origem, x.ConexaoBancariaId = "Mercado Pago", 2, &conexao.Id })

	resp := testeapoio.Chamar(t, mux, "POST", "/api/transacoes", fmt.Sprintf(`{"contaId":%d,"data":"2026-10-10",
		"descricao":"Compra no Mercado Livre","valor":89.9,"tipo":1,"reservaNome":"  Cofrinho da Viagem "}`, mp.Id))
	resp.ConferirStatus(t, http.StatusCreated)
	var criada struct {
		Id          int64
		OpenFinance bool
		Valor       json.RawMessage
		ReservaNome *string
	}
	if err := json.Unmarshal(resp.Corpo, &criada); err != nil {
		t.Fatal(err)
	}
	if criada.OpenFinance || string(criada.Valor) != "-89.9" || criada.ReservaNome == nil || *criada.ReservaNome != "Cofrinho da Viagem" {
		t.Errorf("POST pelo cofrinho = %s; esperado despesa do app com o cofrinho aparado", resp.Corpo)
	}
	if nome := coluna(t, db, criada.Id, "ReservaNome"); nome != "Cofrinho da Viagem" {
		t.Errorf("ReservaNome gravado = %v", nome)
	}
	if pluggy := coluna(t, db, criada.Id, "PluggyTransactionId"); pluggy != nil {
		t.Errorf("PluggyTransactionId = %v; esperado nulo (lançamento do app)", pluggy)
	}
}

// Regra (#758): editar o lançamento do cofrinho funciona (antes, o PUT recusava a conta do Open Finance), e o
// cofrinho pode ser trocado.
func TestAtualizarPeloCofrinho(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conexao := fab.ConexaoBancaria()
	mp := fab.Conta(func(x *testeapoio.Conta) { x.Origem, x.ConexaoBancariaId = 2, &conexao.Id })
	viagem := "Viagem"
	gasto := fab.Transacao(mp, func(x *testeapoio.Transacao) { x.Valor, x.ReservaNome = testeapoio.Valor("-50.0"), &viagem })

	resp := testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/transacoes/%d", gasto.Id), fmt.Sprintf(`{"contaId":%d,
		"data":"2026-10-11","descricao":"Presente","valor":55,"tipo":1,"reservaNome":"Casa"}`, mp.Id))
	resp.ConferirStatus(t, http.StatusOK)
	if nome := coluna(t, db, gasto.Id, "ReservaNome"); nome != "Casa" {
		t.Errorf("ReservaNome depois do PUT = %v; esperado Casa", nome)
	}
	if valor := coluna(t, db, gasto.Id, "Valor"); valor != "-55.0" {
		t.Errorf("Valor depois do PUT = %v; esperado -55.0", valor)
	}
}

// Regra (#758): o cofrinho só vale em conta do Open Finance, só para despesa e nunca no Méli Dólar; sem ele, a conta
// do Open Finance continua recusada.
func TestCofrinhoComErro(t *testing.T) {
	mux, _, fab := rotasDeTeste(t, minimoFixo{n: 4})
	manual := fab.Conta()
	conexao := fab.ConexaoBancaria()
	mp := fab.Conta(func(x *testeapoio.Conta) { x.Origem, x.ConexaoBancariaId = 2, &conexao.Id })
	corpo := func(conta int64, extra string) string {
		return fmt.Sprintf(`{"contaId":%d,"data":"2026-10-10","descricao":"Compra","valor":10,"tipo":1%s}`, conta, extra)
	}
	casos := []struct{ nome, corpo, mensagem string }{
		{"conta manual com cofrinho", corpo(manual.Id, `,"reservaNome":"Viagem"`),
			"O cofrinho só vale em conta do Open Finance: em conta manual, o gasto sai do saldo da conta."},
		{"Open Finance sem cofrinho", corpo(mp.Id, `,"reservaNome":"   "`),
			"Não é possível lançar manualmente em contas do Open Finance. Os lançamentos chegam pela sincronização."},
		{"receita do cofrinho", corpo(mp.Id, `,"tipo":2,"reservaNome":"Viagem"`), "Do cofrinho só se lança despesa."},
		{"Méli Dólar", corpo(mp.Id, `,"reservaNome":"Méli Dólar"`), "O Méli Dólar não tem saída pelo site: ela é em dólar."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			resp := testeapoio.Chamar(t, mux, "POST", "/api/transacoes", c.corpo)
			resp.ConferirStatus(t, http.StatusBadRequest)
			resp.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, c.mensagem))
		})
	}
	longo := testeapoio.Chamar(t, mux, "POST", "/api/transacoes", corpo(mp.Id, fmt.Sprintf(`,"reservaNome":"%0101d"`, 0)))
	longo.ConferirStatus(t, http.StatusBadRequest)
}
