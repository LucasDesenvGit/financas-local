package faturas

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func p[T any](v T) *T { return &v }

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	dados, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return dados
}

// Regra: a fatura do mês de cada cartão, lida do banco (hoje é 15/10/2026):
//   - a fechada vale o total que o banco informou, e o pagamento lançado no cartão (entrada que virou transferência)
//     a deixa paga; como já fechou, a seguinte vem junto em "proxima", com o total dela e a situação pelo ciclo;
//   - a parcela vai para a fatura da âncora mais a diferença de número, mesmo com a data do outro lado do fechamento;
//   - transferência e PAGAMENTO_FATURA não entram na soma; estorno desconta;
//   - cartão sem dia de fechamento fica com o mês do calendário, sem vencimento nem situação;
//   - cartão arquivado fica de fora;
//   - a ordem: o valor do mês, do maior para o menor, e o nome pela cultura no empate (árvore antes de Bolsa).
func TestDoMesDosCartoes(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	vazia, err := DoMesDosCartoes(t.Context(), db, tipos.NovaData(2026, 10, 1), tipos.NovaData(2026, 10, 31), tipos.NovaData(2026, 10, 15))
	if err != nil || vazia == nil || len(vazia) != 0 {
		t.Fatalf("sem cartões: %v, %v (esperado lista vazia, não nil)", vazia, err)
	}

	cartao := func(nome string, ajustes ...func(*testeapoio.Conta)) testeapoio.Conta {
		return fab.Conta(append([]func(*testeapoio.Conta){func(c *testeapoio.Conta) { c.Nome, c.Tipo = nome, 3 }}, ajustes...)...)
	}
	d := func(s string) tipos.Decimal {
		v, err := tipos.NovoDecimal(s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	lancar := func(c testeapoio.Conta, data tipos.Data, valor string, ajustes ...func(*testeapoio.Transacao)) {
		fab.Transacao(c, append([]func(*testeapoio.Transacao){func(x *testeapoio.Transacao) {
			x.Data, x.Valor = data, d(valor)
		}}, ajustes...)...)
	}

	principal := cartao("Cartão Fatura", func(c *testeapoio.Conta) { c.DiaFechamento, c.DiaVencimento = p(int64(3)), p(int64(10)) })
	outubro := fab.Fatura(principal, tipos.NovaData(2026, 10, 10), d("474.15"), func(f *testeapoio.FaturaCartao) {
		f.DataFechamento = p(tipos.NovaData(2026, 10, 3))
	})
	novembro := fab.Fatura(principal, tipos.NovaData(2026, 11, 10), d("299.9"), func(f *testeapoio.FaturaCartao) {
		f.DataFechamento = p(tipos.NovaData(2026, 11, 3))
	})
	lancar(principal, tipos.NovaData(2026, 9, 20), "-100.00")
	lancar(principal, tipos.NovaData(2026, 9, 5), "-50.00", func(x *testeapoio.Transacao) {
		x.Descricao, x.ParcelaNumero, x.ParcelaTotal, x.FaturaCartaoId = "LOJA 01/03", p(int64(1)), p(int64(3)), &outubro.Id
	})
	// A 2ª parcela vem com data antes do fechamento de outubro, mas é cobrada em novembro (a âncora + 1).
	lancar(principal, tipos.NovaData(2026, 10, 2), "-50.00", func(x *testeapoio.Transacao) {
		x.Descricao, x.ParcelaNumero, x.ParcelaTotal = "LOJA 02/03", p(int64(2)), p(int64(3))
	})
	lancar(principal, tipos.NovaData(2026, 10, 7), "20.00") // estorno (despesa com valor positivo)
	lancar(principal, tipos.NovaData(2026, 10, 8), "474.15", func(x *testeapoio.Transacao) { x.Tipo = 3 })
	lancar(principal, tipos.NovaData(2026, 10, 9), "-999.00", func(x *testeapoio.Transacao) { x.MeioPagamento = p("PAGAMENTO_FATURA") })
	lancar(principal, tipos.NovaData(2026, 10, 9), "-888.00", func(x *testeapoio.Transacao) { x.Tipo = 3 })

	bolsa := cartao("Bolsa")
	arvore := cartao("árvore")
	zeta := cartao("Zeta")
	lancar(zeta, tipos.NovaData(2026, 10, 2), "-50.00")
	lancar(zeta, tipos.NovaData(2026, 9, 30), "-7.00")
	arquivado := cartao("Arquivado", func(c *testeapoio.Conta) { c.Arquivada = true })
	lancar(arquivado, tipos.NovaData(2026, 10, 2), "-1000.00")

	lista, err := DoMesDosCartoes(t.Context(), db, tipos.NovaData(2026, 10, 1), tipos.NovaData(2026, 10, 31), tipos.NovaData(2026, 10, 15))
	if err != nil {
		t.Fatal(err)
	}
	semFatura := func(c testeapoio.Conta, nome string) string {
		return fmt.Sprintf(`{"contaId":%d,"cartao":%q,"valorMes":0,"parcelasMes":0,"faturaId":null,"vencimento":null,
			"situacao":null,"banco":null,"fechamento":null,"proxima":null}`, c.Id, nome)
	}
	testeapoio.ConferirJSON(t, mustJSON(t, lista), fmt.Sprintf(`[
		{"contaId":%[1]d,"cartao":"Cartão Fatura","valorMes":474.15,"parcelasMes":50.0,"faturaId":%[2]d,
		 "vencimento":"2026-10-10","situacao":"paga","banco":null,"fechamento":"2026-10-03",
		 "proxima":{"contaId":%[1]d,"cartao":"Cartão Fatura","valorMes":299.9,"parcelasMes":50.0,"faturaId":%[3]d,
		  "vencimento":"2026-11-10","situacao":"aberta","banco":null,"fechamento":"2026-11-03","proxima":null}},
		{"contaId":%[4]d,"cartao":"Zeta","valorMes":50.0,"parcelasMes":0,"faturaId":null,"vencimento":null,"situacao":null,
		 "banco":null,"fechamento":null,"proxima":null},
		%[5]s,
		%[6]s]`, principal.Id, outubro.Id, novembro.Id, zeta.Id, semFatura(arvore, "árvore"), semFatura(bolsa, "Bolsa")))
}

// Regra: a fatura aberta (sem total do banco) vale a soma do ciclo, com o arredondamento do C# só onde há mais de 2
// casas; sem pagamento e antes do fechamento, aberta; a do mês seguinte não vem junto enquanto esta não fechou.
func TestDoMesComFaturaAberta(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	c := fab.Conta(func(c *testeapoio.Conta) {
		c.Nome, c.Tipo, c.DiaFechamento, c.DiaVencimento, c.Banco = "Nubank", 3, p(int64(28)), p(int64(5)), p("nubank")
	})
	for _, valor := range []string{"-10.125", "-0.005"} {
		v, _ := tipos.NovoDecimal(valor)
		fab.Transacao(c, func(x *testeapoio.Transacao) { x.Data, x.Valor = tipos.NovaData(2026, 10, 10), v })
	}
	lista, err := DoMesDosCartoes(t.Context(), db, tipos.NovaData(2026, 10, 1), tipos.NovaData(2026, 10, 31), tipos.NovaData(2026, 10, 15))
	if err != nil {
		t.Fatal(err)
	}
	testeapoio.ConferirJSON(t, mustJSON(t, lista), fmt.Sprintf(`[{"contaId":%d,"cartao":"Nubank","valorMes":10.13,
		"parcelasMes":0,"faturaId":null,"vencimento":"2026-11-05","situacao":"aberta","banco":"nubank",
		"fechamento":"2026-10-28","proxima":null}]`, c.Id))
}

// Regra: erro do banco volta como erro, em cada leitura.
func TestDoMesComErroDoBanco(t *testing.T) {
	db := testeapoio.NovoBancoVazio(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := DoMesDosCartoes(t.Context(), db, tipos.NovaData(2026, 10, 1), tipos.NovaData(2026, 10, 31), tipos.NovaData(2026, 10, 15)); err == nil {
		t.Error("banco fechado não deu erro")
	}
}
