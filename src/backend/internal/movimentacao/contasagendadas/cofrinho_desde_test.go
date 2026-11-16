package contasagendadas

import (
	"fmt"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Regra (#691): a baixa automática pelo cofrinho vale a partir de hoje quando a conta a pagar passa a ter o "Pago com",
// ou muda a forma de cobrança, a conta ou é reativada; mudar o resto (ou só o nome do cofrinho) mantém o dia de antes,
// e sem cofrinho não há dia.
func TestReservaDesde(t *testing.T) {
	t.Parallel()
	hoje, antigo := dia(14), dia(1)
	base := conta{ContaId: ptr(int64(7)), FormaCobranca: 2, Ativa: true, ReservaNome: ptr("Emergência"), ReservaDesde: &antigo}
	casos := []struct {
		nome   string
		antes  func(*conta)
		depois func(*conta)
		quer   *tipos.Data
	}{
		{"sem mudança: o de antes", nil, nil, &antigo},
		{"outra descrição e valor: o de antes", nil, func(c *conta) { c.Descricao, c.Valor = "Luz", testeapoio.Valor("90.0") }, &antigo},
		{"outro cofrinho: o de antes", nil, func(c *conta) { c.ReservaNome = ptr("Viagem") }, &antigo},
		{"conta nova ou sem cofrinho antes: hoje", func(c *conta) { c.ReservaNome, c.ReservaDesde = nil, nil }, nil, &hoje},
		{"cofrinho sem o dia gravado: hoje", func(c *conta) { c.ReservaDesde = nil }, nil, &hoje},
		{"passou a débito automático: hoje", func(c *conta) { c.FormaCobranca = 1 }, nil, &hoje},
		{"outra conta: hoje", nil, func(c *conta) { c.ContaId = ptr(int64(8)) }, &hoje},
		{"sem conta antes: hoje", func(c *conta) { c.ContaId = nil }, nil, &hoje},
		{"reativada: hoje", func(c *conta) { c.Ativa = false }, nil, &hoje},
		{"encerrada agora: o de antes", nil, func(c *conta) { c.Ativa = false }, &antigo},
		{"tirou o cofrinho: nenhum", nil, func(c *conta) { c.ReservaNome = nil }, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			antes, depois := base, base
			if c.antes != nil {
				c.antes(&antes)
			}
			if c.depois != nil {
				c.depois(&depois)
			}
			if obtido := reservaDesde(antes, depois, hoje); fmt.Sprint(valorOuData(obtido)) != fmt.Sprint(valorOuData(c.quer)) {
				t.Errorf("reservaDesde = %v, esperado %v", valorOuData(obtido), valorOuData(c.quer))
			}
		})
	}
}

func valorOuData(d *tipos.Data) any {
	if d == nil {
		return nil
	}
	return d.String()
}

// Regra (#691, da revisão da #693): cadastrar uma conta em débito automático com o "Pago com" e o vencimento meses
// atrás não lança despesas nos meses passados (a conta fica vencida, com o "Pagar"); editar depois mantém o dia. O
// "Pagar" pelo cofrinho de conta encerrada é recusado.
func TestCofrinhoSemBaixaRetroativa(t *testing.T) {
	t.Parallel()
	db, fab, mux := ambiente(t)
	of := fab.Conta(func(c *testeapoio.Conta) { c.Origem = 2 })
	corpo := func(descricao string) string {
		return fmt.Sprintf(`{"descricao":%q,"valor":800,"proximoVencimento":"2026-04-10","recorrencia":1,"formaCobranca":2,`+
			`"contaId":%d,"reservaNome":"Emergência"}`, descricao, of.Id)
	}
	if r := testeapoio.Chamar(t, mux, "POST", "/api/contas-agendadas", corpo("Condomínio")); r.Status != 204 {
		t.Fatalf("criar: %d %s", r.Status, r.Corpo)
	}
	var id int64
	estado := func() string {
		t.Helper()
		var proximo string
		var desde *string
		var lancamentos int
		if err := db.QueryRow(`SELECT "Id", "ProximoVencimento", "ReservaDesde", (SELECT COUNT(*) FROM "Transacao")
			FROM "ContaAgendada" WHERE "ContaId" = ?`, of.Id).Scan(&id, &proximo, &desde, &lancamentos); err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("próximo %s, desde %v, %d lançamentos", proximo, valorOu(desde), lancamentos)
	}
	quer := "próximo 2026-04-10, desde 2026-10-14, 0 lançamentos"
	if obtido := estado(); obtido != quer {
		t.Errorf("depois de criar: %s, esperado %s", obtido, quer)
	}
	if _, err := db.Exec(`UPDATE "ContaAgendada" SET "ReservaDesde" = '2026-03-01' WHERE "Id" = ?`, id); err != nil {
		t.Fatal(err)
	}
	if r := testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/contas-agendadas/%d", id), corpo("Condomínio Ed. Sol")); r.Status != 204 {
		t.Fatalf("editar: %d %s", r.Status, r.Corpo)
	}
	// Com o dia de antes do vencimento, a edição concilia e põe em dia: os vencimentos de abril a outubro.
	if obtido, quer := estado(), "próximo 2026-11-10, desde 2026-03-01, 7 lançamentos"; obtido != quer {
		t.Errorf("depois de editar: %s, esperado %s", obtido, quer)
	}

	encerrada := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.ContaId, a.ReservaNome, a.Ativa = &of.Id, ptr("Emergência"), false
	})
	r := testeapoio.Chamar(t, mux, "POST", fmt.Sprintf("/api/contas-agendadas/%d/pagar", encerrada.Id), `{"lancarTransacao":true}`)
	if m := lerJSON[map[string]string](t, r.Corpo)["mensagem"]; r.Status != 400 ||
		m != "A conta a pagar está encerrada. Reative-a para pagar pelo cofrinho." {
		t.Errorf("pagar pelo cofrinho a encerrada: %d %s", r.Status, r.Corpo)
	}
}
