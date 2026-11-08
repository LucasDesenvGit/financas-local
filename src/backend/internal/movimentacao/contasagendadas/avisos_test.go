package contasagendadas

import (
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Os testes deste arquivo vieram da #640: cada um pega mutações que sobreviviam no teste de mutação (o comentário diz
// quais), sem mudar o comportamento.

// Regra (#158): o "aumentou" olha as três últimas cobranças da mesma conta ou cartão da última: as de outra conta e
// as mais antigas que a terceira não dizem se o valor varia sempre. Variando sempre, não avisa, mesmo com o aumento;
// diferença de até 5%, também não; com menos de três, não é "varia sempre". Mutações: o "&&" e o "< 3" das três
// últimas, os "&&" do "varia sempre", o "||" do "não avisa" e o "&&" do maiorQue.
func TestAumento(t *testing.T) {
	t.Parallel()
	const a, b = 1, 2
	cobrou := func(conta int64, dia int, valor string) cobranca {
		return cobranca{data: testeapoio.Dia(2026, 10, dia), valor: testeapoio.Valor(valor), contaId: conta}
	}
	conta := recorrente{id: 1, descricao: "Internet", valor: testeapoio.Valor("100.0"), contaId: ptr(int64(a))}
	casos := []struct {
		nome   string
		minhas []cobranca // mais recente primeiro
		avisa  bool
	}{
		{"as três últimas de A não variam; a de B e a quarta de A ficam de fora",
			[]cobranca{cobrou(a, 20, "110.0"), cobrou(b, 18, "50.0"), cobrou(a, 15, "100.0"), cobrou(a, 10, "100.0"),
				cobrou(a, 5, "50.0")}, true},
		{"varia sempre: não avisa, mesmo com o aumento", []cobranca{cobrou(a, 20, "150.0"), cobrou(a, 15, "100.0"),
			cobrou(a, 10, "100.0")}, false},
		{"duas que variam: ainda não é \"varia sempre\", que precisa de três", []cobranca{cobrou(a, 20, "150.0"),
			cobrou(a, 15, "100.0")}, true},
		{"diferença de 2%: não avisa", []cobranca{cobrou(a, 20, "102.0")}, false},
		{"diferença de 6%: avisa", []cobranca{cobrou(a, 20, "106.0")}, true},
	}
	for _, c := range casos {
		if _, avisa := aumento(conta, c.minhas, c.minhas[0]); avisa != c.avisa {
			t.Errorf("%s: %v", c.nome, avisa)
		}
	}
}

// Regra (#158): a última cobrança é a de data mais recente, e não a gravada por último; a conta a pagar sem cobrança
// não interrompe os avisos das outras. Mutações: a ordem por data (o "-" trocado) e o "continue" da sem cobrança
// trocado por "break".
func TestAvisosPelaDataMaisRecente(t *testing.T) {
	t.Parallel()
	_, fab, mux := ambiente(t)
	cartaoA := fab.Conta(func(c *testeapoio.Conta) { c.Nome, c.Tipo = "Cartão A", 3 })
	cartaoB := fab.Conta(func(c *testeapoio.Conta) { c.Nome, c.Tipo = "Cartão B", 3 })
	// A primeira conta a pagar não tem cobrança nenhuma no extrato (e é à mão, então não é "não veio").
	fab.ContaAgendada(func(a *testeapoio.ContaAgendada) { a.Descricao, a.TextoCobranca = "Revista", ptr("REVISTA SEMANAL") })
	internet := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.Descricao, a.Valor, a.ProximoVencimento, a.TextoCobranca, a.ContaId = "Internet", testeapoio.Valor("100.0"),
			dia(25), ptr("FIBRANET"), &cartaoA.Id
	})
	// Gravadas da mais antiga para a mais recente: a de setembro, no cartão A, tem o id menor.
	for _, c := range []struct {
		conta testeapoio.Conta
		data  tipos.Data
		valor string
	}{{cartaoA, testeapoio.Dia(2026, 9, 1), "-100.0"}, {cartaoB, dia(1), "-150.0"}} {
		fab.Transacao(c.conta, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Descricao = c.data, testeapoio.Valor(c.valor), "FIBRANET"
		})
	}

	avisos := lerJSON[[]AvisoDto](t, testeapoio.Chamar(t, mux, "GET", "/api/contas-agendadas/avisos", "").Corpo)
	var achados []string
	for _, a := range avisos {
		if a.ContaAgendadaId == internet.Id {
			achados = append(achados, a.Tipo)
		}
	}
	if len(achados) != 2 || achados[0] != "mudou-de-cartao" || achados[1] != "aumentou" {
		t.Errorf("avisos da internet: %v (todos: %+v)", achados, avisos)
	}
}

// Regra: a conta a pagar com uma categoria que existe é gravada. Mutação: o "err != nil" da categoria trocado por
// "err == nil" (toda categoria viraria inválida).
func TestSalvarComCategoria(t *testing.T) {
	t.Parallel()
	_, fab, mux := ambiente(t)
	categoria := fab.Categoria()
	corpo := `{"descricao":"Luz","valor":10,"proximoVencimento":"2026-11-01","recorrencia":1,"categoriaId":` +
		itoa(categoria.Id) + `,"contaId":null}`
	if r := testeapoio.Chamar(t, mux, "POST", "/api/contas-agendadas", corpo); r.Status != 204 {
		t.Errorf("status %d: %s", r.Status, r.Corpo)
	}
}
