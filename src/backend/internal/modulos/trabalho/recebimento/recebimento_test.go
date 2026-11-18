package recebimento

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/analises/relatorios"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/emprego"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/holerite"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

var v = testeapoio.Valor

func liquido(s string) *tipos.Decimal { d := v(s); return &d }

func TestPontuar(t *testing.T) {
	t.Parallel()
	conta := int64(7)
	casos := []struct {
		nome    string
		t       Transacao
		liquido *tipos.Decimal
		conta   *int64
		pontos  int
		motivos string
	}{
		{"valor igual ao líquido", Transacao{Livre: v("4738.02")}, liquido("4738.02"), nil, 3, MotivoValor},
		{"5% acima ainda é perto", Transacao{Livre: v("4974.92")}, liquido("4738.02"), nil, 3, MotivoValor},
		{"5% abaixo ainda é perto", Transacao{Livre: v("4501.12")}, liquido("4738.02"), nil, 3, MotivoValor},
		{"mais de 5% abaixo: parte", Transacao{Livre: v("2000.00")}, liquido("4738.02"), nil, 1, MotivoParte},
		{"acima de 5%: nada", Transacao{Livre: v("5000.00")}, liquido("4738.02"), nil, 0, ""},
		{"sem líquido", Transacao{Livre: v("4738.02")}, nil, nil, 0, ""},
		{"o banco marcou salário, na conta do salário", Transacao{Livre: v("10"), Salario: true, ContaId: 7}, nil, &conta, 3,
			MotivoSalario + " | " + MotivoContaSalario},
		{"cita o empregador, sem acento", Transacao{Livre: v("10"), Descricao: "PIX RECEBIDO - FICTICIA COMERCIO"}, nil, nil, 2, MotivoEmpregador},
		{"outra conta", Transacao{Livre: v("10"), ContaId: 8}, nil, &conta, 0, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			p, m := Pontuar(c.t, c.liquido, "Fictícia Comércio Ltda", c.conta)
			if p != c.pontos || strings.Join(m, " | ") != c.motivos {
				t.Fatalf("pontos %d %v; quer %d %q", p, m, c.pontos, c.motivos)
			}
		})
	}
}

func TestCitaEmpregador(t *testing.T) {
	t.Parallel()
	casos := map[string]bool{
		"TED ACME SA":            true,  // "ACME" tem 4 letras
		"PAGTO LTDA":             false, // "Ltda" não conta
		"ACMEX":                  false, // palavra inteira
		"salario acme industria": true,
		"":                       false,
	}
	for descricao, quer := range casos {
		if got := citaEmpregador(descricao, "Acme Indústria Ltda"); got != quer {
			t.Errorf("%q: %v; quer %v", descricao, got, quer)
		}
	}
	if citaEmpregador("TED DA SA", "DA SA") {
		t.Error("palavras com menos de 4 letras não contam")
	}
}

type mux struct{ *http.ServeMux }

func (m mux) HandleFunc(p string, h http.HandlerFunc) { m.ServeMux.HandleFunc(p, h) }

func TestRecebimentoNaoContaDuasVezes(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	rel := testeapoio.HojeEmBrasilia(2026, 11, 13)
	log := slog.New(slog.DiscardHandler)
	empregos := emprego.NovoServico(db, rel, log)
	holerites := holerite.NovoServico(db, empregos, rel, log)
	m := mux{http.NewServeMux()}
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
	salarioCat := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Salário", 2 })
	conta := fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Conta salário" })
	outra := fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Outra" })
	credito := func(c testeapoio.Conta, dia tipos.Data, valor, descricao string, meio *string) testeapoio.Transacao {
		return fab.Transacao(c, func(x *testeapoio.Transacao) {
			x.Data, x.Valor, x.Tipo, x.Descricao, x.MeioPagamento = dia, v(valor), 2, descricao, meio
		})
	}
	// Outubro de 2026: 5500 de salário, líquido 4738,02 (os casos do holerite). Pago em dois créditos: o adiantamento
	// (2000, dia 20/10) e o saldo (2738,02, dia 05/11, marcado pelo banco como folha de pagamento).
	r := chamar("POST", "/api/trabalho/empregos", fmt.Sprintf(`{"empregador":"Fictícia Comércio Ltda","admissao":"2024-03-11","salarioBruto":5500,"contaId":%d}`, conta.Id))
	var e emprego.Emprego
	_ = json.Unmarshal(r.Corpo, &e)
	var h holerite.Holerite
	_ = json.Unmarshal(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", e.Id), `{"mes":"2026-10"}`).Corpo, &h)
	adiantamento := credito(conta, testeapoio.Dia(2026, 10, 20), "2000.00", "ADIANTAMENTO FICTICIA COMERCIO", nil)
	folha := "FOLHA_PAGAMENTO"
	saldo := credito(conta, testeapoio.Dia(2026, 11, 5), "2738.02", "CREDITO", &folha)
	credito(outra, testeapoio.Dia(2026, 10, 25), "50.00", "PIX DE AMIGO", nil) // não parece salário
	// Só cita o empregador (2 pontos, o mínimo): outra conta e acima do líquido.
	bonus := credito(outra, testeapoio.Dia(2026, 10, 28), "6000.00", "BONUS FICTICIA", nil)
	credito(conta, testeapoio.Dia(2026, 11, 20), "4738.02", "FICTICIA COMERCIO", nil)                                     // fora da janela (depois do dia 15)
	fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Data, x.Valor = testeapoio.Dia(2026, 10, 25), v("-4738.02") }) // débito

	relatorio := relatorios.NovoServico(db, rel)
	receitaDe := func() string {
		t.Helper()
		receitas, _, err := relatorio.Totais(t.Context(), testeapoio.Dia(2026, 10, 1), testeapoio.Dia(2026, 11, 30))
		if err != nil {
			t.Fatal(err)
		}
		return receitas.String()
	}
	antes := receitaDe()

	caminho := fmt.Sprintf("/api/trabalho/holerites/%d/recebimentos", h.Id)
	c := ler(chamar("GET", caminho, ""))
	if c.Liquido.String() != "4738.02" || c.Recebido.String() != "0.00" || len(c.Sugestoes) != 3 || c.Sugestoes[2].Id != bonus.Id ||
		len(c.Outros) != 1 || c.Outros[0].Descricao != "PIX DE AMIGO" {
		t.Fatalf("antes = %+v", c)
	}
	// Os dois empatam (parte do líquido e na conta do salário; um cita o empregador, o outro o banco marcou folha): vale
	// a data, o adiantamento primeiro.
	if c.Sugestoes[0].Id != adiantamento.Id || c.Sugestoes[1].Id != saldo.Id ||
		strings.Join(c.Sugestoes[1].Motivos, " | ") != MotivoParte+" | "+MotivoSalario+" | "+MotivoContaSalario {
		t.Fatalf("ordem das sugestões = %+v", c.Sugestoes)
	}

	// Ligar os dois: o adiantamento inteiro e o saldo.
	c = ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, adiantamento.Id)))
	if len(c.Sugestoes) != 2 || c.Sugestoes[0].Id != saldo.Id { // o ligado sai das sugestões, e os outros ficam
		t.Fatalf("depois do adiantamento = %+v", c.Sugestoes)
	}
	// Ligar um crédito maior que o que falta, sem dizer o valor, leva só o que falta (2738,02); desfaço em seguida.
	c = ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, bonus.Id)))
	if c.Recebimentos[1].ValorLigado.String() != "2738.02" {
		t.Fatalf("o que falta = %+v", c.Recebimentos[1])
	}
	ler(chamar("DELETE", fmt.Sprintf("%s/%d", caminho, c.Recebimentos[1].Id), ""))
	c = ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, saldo.Id)))
	if c.Recebido.String() != "4738.02" || c.Diferenca.String() != "0.00" || len(c.Recebimentos) != 2 || len(c.Sugestoes) != 1 {
		t.Fatalf("ligado = %+v", c)
	}
	// A regra de ouro: a receita dos relatórios não mudou.
	if depois := receitaDe(); depois != antes {
		t.Fatalf("a receita mudou ao ligar: %s → %s", antes, depois)
	}
	// A transação sem categoria foi para "Salário", como escolha manual.
	var categoria *int64
	var manual bool
	if err := db.QueryRowContext(t.Context(), `SELECT "CategoriaId", "CategoriaManual" FROM "Transacao" WHERE "Id" = ?`, saldo.Id).
		Scan(&categoria, &manual); err != nil || categoria == nil || *categoria != salarioCat.Id || !manual {
		t.Fatalf("categoria = %v, manual %v (%v)", categoria, manual, err)
	}

	// Com o líquido completo, ligar sem dizer o valor leva o crédito inteiro.
	c = ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d}`, bonus.Id)))
	if c.Recebido.String() != "10738.02" || c.Diferenca.String() != "6000.00" {
		t.Fatalf("bônus = recebido %s", c.Recebido)
	}
	ler(chamar("DELETE", fmt.Sprintf("%s/%d", caminho, c.Recebimentos[2].Id), ""))
	c = ler(chamar("GET", caminho, ""))
	// Desligar e excluir o holerite também não mudam a receita.
	ler(chamar("DELETE", fmt.Sprintf("%s/%d", caminho, c.Recebimentos[0].Id), ""))
	chamar("DELETE", fmt.Sprintf("%s/%d", caminho, c.Recebimentos[0].Id), "").ConferirStatus(t, http.StatusNotFound)
	if depois := receitaDe(); depois != antes {
		t.Fatalf("a receita mudou ao desligar: %s → %s", antes, depois)
	}

	casos := []struct{ nome, corpo, mensagem string }{
		{"já ligado", fmt.Sprintf(`{"transacaoId":%d}`, saldo.Id), "Este crédito já está ligado ao holerite."},
		{"débito", `{"transacaoId":999}`, "Escolha um crédito (uma entrada de dinheiro)."},
		{"parte maior que o crédito", fmt.Sprintf(`{"transacaoId":%d,"valor":2000.01}`, adiantamento.Id),
			"A parte ligada vai de R$ 0,01 até o que sobra do crédito (2000,00)."},
		{"parte zerada", fmt.Sprintf(`{"transacaoId":%d,"valor":0}`, adiantamento.Id),
			"A parte ligada vai de R$ 0,01 até o que sobra do crédito (2000,00)."},
		{"JSON quebrado", `{"transacaoId":`, "Os dados enviados não estão no formato esperado."},
	}
	for _, k := range casos {
		t.Run(k.nome, func(t *testing.T) {
			r := chamar("POST", caminho, k.corpo)
			r.ConferirStatus(t, http.StatusBadRequest)
			r.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, k.mensagem))
		})
	}
	// Uma parte do crédito: o resto fica livre para outro holerite.
	c = ler(chamar("POST", caminho, fmt.Sprintf(`{"transacaoId":%d,"valor":500}`, adiantamento.Id)))
	if c.Recebido.String() != "3238.02" || c.Diferenca.String() != "-1500.00" {
		t.Fatalf("parte = recebido %s, diferença %s", c.Recebido, c.Diferenca)
	}
	var nov holerite.Holerite
	_ = json.Unmarshal(chamar("POST", fmt.Sprintf("/api/trabalho/empregos/%d/holerites", e.Id), `{"mes":"2026-11"}`).Corpo, &nov)
	c = ler(chamar("POST", fmt.Sprintf("/api/trabalho/holerites/%d/recebimentos", nov.Id), fmt.Sprintf(`{"transacaoId":%d}`, adiantamento.Id)))
	if c.Recebido.String() != "1500.00" {
		t.Fatalf("o resto do crédito = %s", c.Recebido)
	}
	if depois := receitaDe(); depois != antes {
		t.Fatalf("a receita mudou com um crédito em dois holerites: %s → %s", antes, depois)
	}
	chamar("GET", "/api/trabalho/holerites/999/recebimentos", "").ConferirStatus(t, http.StatusNotFound)
	chamar("POST", "/api/trabalho/holerites/x/recebimentos", `{"transacaoId":1}`).ConferirStatus(t, http.StatusNotFound)
}
