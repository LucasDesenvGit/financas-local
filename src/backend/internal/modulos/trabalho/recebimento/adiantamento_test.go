package recebimento

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// #768: o exemplo da issue. Adiantamento de 3.200,00 no dia 15; proventos 10.300,00; descontos 6.576,25 (com o
// adiantamento); líquido 3.723,75 no dia 30.
func TestPontuarComAdiantamento(t *testing.T) {
	t.Parallel()
	adiantamento := decimal.RequireFromString("3200.00")
	casos := []struct {
		nome         string
		livre        string
		adiantamento decimal.Decimal
		pontos       int
		motivos      string
	}{
		{"perto do adiantamento", "3200.00", adiantamento, 3, MotivoAdiantamento},
		{"5% do adiantamento ainda é perto", "3360.00", adiantamento, 3, MotivoAdiantamento},
		{"perto do saldo", "3723.75", adiantamento, 3, MotivoSaldo},
		{"a soma dos dois não é um dos dois", "6923.75", adiantamento, 0, ""},
		{"sem adiantamento o líquido segue 'valor perto'", "3723.75", decimal.Zero, 3, MotivoValor},
		{"sem adiantamento o de 3.200 é só a parte", "3200.00", decimal.Zero, 1, MotivoParte},
		{"entre os dois, mais perto do saldo", "3600.00", adiantamento, 3, MotivoSaldo},
		{"perto dos dois: vale o mais perto (o adiantamento)", "3600.00", decimal.RequireFromString("3700.00"), 3, MotivoAdiantamento},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			p, m := Pontuar(Transacao{Livre: v(c.livre)}, liquido("3723.75"), c.adiantamento, "Fictícia Comércio Ltda", nil)
			if p != c.pontos || strings.Join(m, " | ") != c.motivos {
				t.Fatalf("pontos %d %v; quer %d %q", p, m, c.pontos, c.motivos)
			}
			if (c.motivos == MotivoAdiantamento || c.motivos == MotivoSaldo) && !Sugerir(p, m) {
				t.Fatal("o adiantamento e o saldo são motivos fortes")
			}
		})
	}
}

func TestAdiantamentoDe(t *testing.T) {
	t.Parallel()
	c := holerite.Calculo{Descontos: []holerite.Linha{
		{Tipo: holerite.TipoInss, Valor: v("900.00")},
		{Tipo: holerite.TipoAdiantamento, Valor: v("3000.10")},
		{Tipo: holerite.TipoAdiantamento, Valor: v("199.90")},
	}}
	if got := adiantamentoDe(c); got.String() != "3200" {
		t.Fatalf("a soma das linhas do adiantamento = %s", got)
	}
	if got := adiantamentoDe(holerite.Calculo{}); !got.IsZero() {
		t.Fatalf("sem a linha = %s", got)
	}
}

func TestRecebimentoComAdiantamentoDescontado(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	empregos := emprego.NovoServico(db, rel, log)
	holerites := holerite.NovoServico(db, empregos, rel, log)
	m := http.NewServeMux()
	emprego.Registrar(m, empregos)
	holerite.Registrar(m, holerites)
	Registrar(m, NovoServico(db, empregos, holerites, rel, log))
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, m, metodo, caminho, corpo)
	}
	ler := func(r testeapoio.Resposta) Resposta {
		t.Helper()
		r.ConferirStatus(t, http.StatusOK)
		var c Resposta
		if err := json.Unmarshal(r.Corpo, &c); err != nil {
			t.Fatalf("%s: %v", r.Corpo, err)
		}
		return c
	}
	fab := testeapoio.NovaFabrica(t, db, rel)
	conta := fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Conta salário" })
	credito := func(dia tipos.Data, valor, descricao string) testeapoio.Transacao {
		return fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Data, x.Valor, x.Tipo, x.Descricao = dia, v(valor), 2, descricao })
	}
	var e emprego.Emprego
	_ = json.Unmarshal(chamar("POST", "/api/trabalho/empregos", fmt.Sprintf(
		`{"empregador":"Fictícia Comércio Ltda","admissao":"2024-03-11","salarioBruto":10300,"contaId":%d}`, conta.Id)).Corpo, &e)
	// O mês de outubro de cada rodada: o holerite é apagado no fim dela, e os créditos ficam livres de novo.
	novoHolerite := func() holerite.Holerite {
		var h holerite.Holerite
		_ = json.Unmarshal(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", e.Id), `{"mes":"2026-10"}`).Corpo, &h)
		return h
	}
	// Fecha o holerite com o cálculo da issue (o lido do PDF, que traz a linha do adiantamento).
	fechar := func(h holerite.Holerite, descontos ...holerite.Linha) {
		t.Helper()
		calculo, _ := json.Marshal(holerite.Calculo{Mes: h.Mes, Proventos: []holerite.Linha{}, Descontos: descontos,
			TotalProventos: v("10300.00"), TotalDescontos: v("6576.25"), Liquido: v("3723.75")})
		if _, err := db.ExecContext(t.Context(), `UPDATE "Holerite" SET "Situacao" = ?, "Calculo" = ? WHERE "Id" = ?`,
			holerite.Fechado, string(calculo), h.Id); err != nil {
			t.Fatal(err)
		}
	}
	dia15 := credito(testeapoio.Dia(2026, 10, 15), "3200.00", "ADIANTAMENTO FICTICIA COMERCIO")
	dia30 := credito(testeapoio.Dia(2026, 10, 30), "3723.75", "SALARIO FICTICIA COMERCIO")
	adto := holerite.Linha{Tipo: holerite.TipoAdiantamento, Descricao: "Adiantamento", Valor: v("3200.00")}
	inss := holerite.Linha{Tipo: holerite.TipoInss, Descricao: "INSS", Valor: v("3376.25")}

	// Ligar do dia 15 para o dia 30, e o contrário: o resultado final é o mesmo (a ordem não muda nada).
	for _, ordem := range []struct {
		nome   string
		ligar  [2]testeapoio.Transacao
		partes [2]string
	}{
		{"15 e depois 30", [2]testeapoio.Transacao{dia15, dia30}, [2]string{"3200.00", "3723.75"}},
		{"30 e depois 15", [2]testeapoio.Transacao{dia30, dia15}, [2]string{"3723.75", "3200.00"}},
	} {
		h := novoHolerite()
		fechar(h, adto, inss)
		caminho := fmt.Sprintf("/api/trabalho/holerites/%d/recebimentos", h.Id)
		c := ler(chamar("GET", caminho, ""))
		if c.Liquido.String() != "3723.75" || c.Adiantamento.String() != "3200.00" || c.Esperado.String() != "6923.75" {
			t.Fatalf("%s: esperado = %+v", ordem.nome, c)
		}
		motivos := map[int64]string{}
		for _, sg := range c.Sugestoes {
			motivos[sg.Id] = strings.Join(sg.Motivos, "|")
		}
		if len(c.Sugestoes) != 2 || !strings.HasPrefix(motivos[dia15.Id], MotivoAdiantamento) || !strings.HasPrefix(motivos[dia30.Id], MotivoSaldo) {
			t.Fatalf("%s: sugestões = %+v", ordem.nome, c.Sugestoes)
		}
		for i, tr := range ordem.ligar {
			c = ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, tr.Id)))
			if got := c.Recebimentos[i].ValorLigado.String(); got != ordem.partes[i] { // o crédito inteiro, nunca "só o que falta do líquido"
				t.Fatalf("%s: parte do crédito %d = %s; quer %s", ordem.nome, i+1, got, ordem.partes[i])
			}
		}
		if c.Recebido.String() != "6923.75" || c.Diferenca.String() != "0.00" || c.Esperado.String() != "6923.75" {
			t.Fatalf("%s: recebido %s, diferença %s", ordem.nome, c.Recebido, c.Diferenca)
		}
		// Desligar o do dia 15: falta o adiantamento (negativo), em vez de ficar "a mais".
		for _, rec := range c.Recebimentos {
			if rec.Descricao == dia15.Descricao {
				ler(chamar("DELETE", fmt.Sprintf("%s/%d", caminho, rec.Id), ""))
			}
		}
		if c = ler(chamar("GET", caminho, "")); c.Diferenca.String() != "-3200.00" {
			t.Fatalf("%s: sem o dia 15, diferença %s", ordem.nome, c.Diferenca)
		}
		if _, err := db.ExecContext(t.Context(), `DELETE FROM "Holerite" WHERE "Id" = ?`, h.Id); err != nil {
			t.Fatal(err)
		}
	}

	// Sem a linha do adiantamento nada muda: o esperado é o líquido, e ligar sem valor leva só o que falta dele.
	h := novoHolerite()
	fechar(h, inss)
	caminho := fmt.Sprintf("/api/trabalho/holerites/%d/recebimentos", h.Id)
	c := ler(chamar("GET", caminho, ""))
	if c.Adiantamento != nil || c.Esperado.String() != "3723.75" || c.Liquido.String() != "3723.75" {
		t.Fatalf("sem a linha = %+v", c)
	}
	for _, sg := range c.Sugestoes {
		if sg.Id == dia30.Id && !strings.HasPrefix(strings.Join(sg.Motivos, "|"), MotivoValor) {
			t.Fatalf("sem a linha, o líquido segue 'valor perto': %+v", sg)
		}
		if strings.HasPrefix(strings.Join(sg.Motivos, "|"), MotivoAdiantamento) {
			t.Fatalf("sem a linha, ninguém parece o adiantamento: %+v", sg)
		}
	}
	ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, dia15.Id)))
	c = ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, dia30.Id)))
	if c.Recebimentos[1].ValorLigado.String() != "523.75" || c.Diferenca.String() != "0.00" { // o comportamento de antes
		t.Fatalf("sem a linha, o falta é do líquido: %+v", c)
	}
}
