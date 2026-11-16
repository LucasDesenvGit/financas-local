package contasagendadas

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Regra (#691, ADR 0063): o "Pago com" um cofrinho só vale com uma conta do Open Finance e fora do cartão; o nome é
// aparado, e em branco é o saldo da conta.
func TestSalvarComCofrinho(t *testing.T) {
	t.Parallel()
	db, fab, mux := ambiente(t)
	manual := fab.Conta()
	of := fab.Conta(func(c *testeapoio.Conta) { c.Origem = 2 })
	corpo := func(descricao string, forma int, conta *int64, reserva string) string {
		contaJSON := "null"
		if conta != nil {
			contaJSON = fmt.Sprint(*conta)
		}
		return fmt.Sprintf(`{"descricao":%q,"valor":10,"proximoVencimento":"2026-11-01","recorrencia":1,"formaCobranca":%d,`+
			`"contaId":%s,"reservaNome":%q}`, descricao, forma, contaJSON, reserva)
	}
	casos := []struct {
		nome, corpo string
		status      int
		mensagem    string
	}{
		{"sem conta", corpo("x", 2, nil, "Emergência"), 400, "Escolha a conta do cofrinho."},
		{"conta manual", corpo("x", 2, &manual.Id, "Emergência"), 400,
			"O cofrinho tem de ser de uma conta do Open Finance. Em conta manual, lance a despesa pelo Pagar."},
		{"no cartão", corpo("x", 3, &of.Id, "Emergência"), 400, "A conta cobrada no cartão não sai de um cofrinho."},
		{"nome grande demais", corpo("x", 2, &of.Id, strings.Repeat("a", 101)), 400, ""},
		{"débito automático pelo cofrinho", corpo("Luz", 2, &of.Id, "  Emergência "), 204, ""},
		{"em branco é o saldo da conta", corpo("Água", 2, &of.Id, "  "), 204, ""},
	}
	for _, c := range casos {
		resposta := testeapoio.Chamar(t, mux, "POST", "/api/contas-agendadas", c.corpo)
		if resposta.Status != c.status {
			t.Errorf("%s: status %d: %s", c.nome, resposta.Status, resposta.Corpo)
			continue
		}
		if c.mensagem != "" {
			if m := lerJSON[map[string]string](t, resposta.Corpo)["mensagem"]; m != c.mensagem {
				t.Errorf("%s: %q", c.nome, m)
			}
		}
	}
	for descricao, quer := range map[string]string{"Luz": "Emergência", "Água": "<nil>"} {
		var reserva *string
		if err := db.QueryRow(`SELECT "ReservaNome" FROM "ContaAgendada" WHERE "Descricao" = ?`, descricao).Scan(&reserva); err != nil {
			t.Fatal(err)
		}
		if obtido := fmt.Sprint(valorOu(reserva)); obtido != quer {
			t.Errorf("%s: ReservaNome %s, esperado %s", descricao, obtido, quer)
		}
	}
}

func valorOu(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// Regra (#691): o "Pagar" com o cofrinho lança a despesa na conta do Open Finance, saindo dele, e a lista mostra a
// baixa; "Ajustar valor" muda o lançamento e "Desfazer" o apaga e volta o vencimento. Sem cofrinho, na conta do Open
// Finance, continua recusado.
func TestPagarPeloCofrinho(t *testing.T) {
	t.Parallel()
	db, fab, mux := ambiente(t)
	of := fab.Conta(func(c *testeapoio.Conta) { c.Origem = 2 })
	semCofrinho := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) { a.ContaId = &of.Id })
	if r := testeapoio.Chamar(t, mux, "POST", fmt.Sprintf("/api/contas-agendadas/%d/pagar", semCofrinho.Id),
		`{"lancarTransacao":true}`); r.Status != 400 {
		t.Errorf("pagar sem cofrinho no Open Finance: %d %s", r.Status, r.Corpo)
	}
	agendada := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.Descricao, a.ContaId, a.ReservaNome = "Escola", &of.Id, ptr("Emergência")
	})
	caminho := fmt.Sprintf("/api/contas-agendadas/%d", agendada.Id)
	if r := testeapoio.Chamar(t, mux, "DELETE", caminho+"/pagamento-cofrinho", ""); r.Status != 400 {
		t.Errorf("desfazer sem pagamento: %d %s", r.Status, r.Corpo)
	}
	if r := testeapoio.Chamar(t, mux, "POST", caminho+"/pagar", `{"lancarTransacao":true,"data":"2026-10-13"}`); r.Status != 204 {
		t.Fatalf("pagar pelo cofrinho: %d %s", r.Status, r.Corpo)
	}
	if r := testeapoio.Chamar(t, mux, "PUT", caminho+"/pagamento-cofrinho", `{"valor":0}`); r.Status != 400 {
		t.Errorf("ajustar para zero: %d %s", r.Status, r.Corpo)
	}
	if r := testeapoio.Chamar(t, mux, "PUT", caminho+"/pagamento-cofrinho", `{"valor":123.45}`); r.Status != 204 {
		t.Errorf("ajustar: %d %s", r.Status, r.Corpo)
	}
	type ultimo struct {
		Data         string
		Valor        tipos.Decimal
		Automatico   bool
		PodeDesfazer bool
	}
	type linhaDaLista struct {
		Id                 int64
		ReservaNome        *string
		ProximoVencimento  string
		UltimoPeloCofrinho *ultimo
	}
	var linha *linhaDaLista
	for _, c := range lerJSON[[]linhaDaLista](t, testeapoio.Chamar(t, mux, "GET", "/api/contas-agendadas", "").Corpo) {
		if c.Id == agendada.Id {
			linha = &c
		}
	}
	if linha == nil || linha.ReservaNome == nil || *linha.ReservaNome != "Emergência" || linha.ProximoVencimento != "2026-11-14" ||
		linha.UltimoPeloCofrinho == nil || linha.UltimoPeloCofrinho.Data != "2026-10-13" ||
		linha.UltimoPeloCofrinho.Valor.Decimal().Cmp(testeapoio.Valor("123.45").Decimal()) != 0 ||
		linha.UltimoPeloCofrinho.Automatico || !linha.UltimoPeloCofrinho.PodeDesfazer {
		t.Errorf("na lista: %+v", linha)
	}
	var valor, reserva string
	if err := db.QueryRow(`SELECT "Valor", "ReservaNome" FROM "Transacao" WHERE "ContaId" = ?`, of.Id).Scan(&valor, &reserva); err != nil ||
		valor != "-123.45" || reserva != "Emergência" {
		t.Errorf("lançamento: %s %s %v", valor, reserva, err)
	}
	if r := testeapoio.Chamar(t, mux, "DELETE", caminho+"/pagamento-cofrinho", ""); r.Status != 204 {
		t.Fatalf("desfazer: %d %s", r.Status, r.Corpo)
	}
	var proximo string
	var lancamentos int
	if err := db.QueryRow(`SELECT "ProximoVencimento", (SELECT COUNT(*) FROM "Transacao") FROM "ContaAgendada" WHERE "Id" = ?`,
		agendada.Id).Scan(&proximo, &lancamentos); err != nil || proximo != "2026-10-14" || lancamentos != 0 {
		t.Errorf("depois de desfazer: %s, %d lançamentos, %v", proximo, lancamentos, err)
	}
}
