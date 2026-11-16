package sincronizacao

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// Regra (#688): o Resultado leva os ids das novas e atualizadas, em ordem, até 200, e o período de todas elas.
func TestLancamentosDaRodada(t *testing.T) {
	dia := func(d int) tipos.Data { return tipos.NovaData(2026, 10, d) }
	muitos := func(n int) []int64 {
		ids := make([]int64, n)
		for i := range ids {
			ids[i] = int64(n - i)
		}
		return ids
	}
	casos := []struct {
		nome        string
		ids         []int64
		datas       []tipos.Data // uma por id; sem datas, todas no dia 5
		esperadoIds []int64      // nil: o Resultado não leva lançamentos
		inicio, fim tipos.Data
	}{
		{"nenhuma", nil, nil, nil, tipos.Data{}, tipos.Data{}},
		{"uma", []int64{7}, []tipos.Data{dia(3)}, []int64{7}, dia(3), dia(3)},
		{"em ordem de id, com a data mais antiga e a mais recente", []int64{9, 2, 5}, []tipos.Data{dia(10), dia(1), dia(20)}, []int64{2, 5, 9}, dia(1), dia(20)},
		{"a mais antiga por último", []int64{1, 2}, []tipos.Data{dia(8), dia(4)}, []int64{1, 2}, dia(4), dia(8)},
		{"200 ainda vão", muitos(200), nil, slices.Sorted(slices.Values(muitos(200))), dia(5), dia(5)},
		{"201 vão sem os ids, com o período", muitos(201), nil, []int64{}, dia(5), dia(5)},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			var l lancamentosDaRodada
			for i, id := range caso.ids {
				data := dia(5)
				if caso.datas != nil {
					data = caso.datas[i]
				}
				l.somar(id, data)
			}
			r := l.resultado()
			if caso.esperadoIds == nil {
				if r != nil {
					t.Fatalf("resultado = %+v, esperado nil", r)
				}
				return
			}
			if r == nil {
				t.Fatal("resultado = nil")
			}
			if !slices.Equal(r.Ids, caso.esperadoIds) || r.Ids == nil {
				t.Errorf("ids = %v, esperado %v", r.Ids, caso.esperadoIds)
			}
			if !r.Inicio.Igual(caso.inicio) || !r.Fim.Igual(caso.fim) {
				t.Errorf("período = %s a %s, esperado %s a %s", r.Inicio, r.Fim, caso.inicio, caso.fim)
			}
		})
	}
}

// Regra (#688): passou de 200, o JSON leva "ids": [] (não null): o front-end lê ids.length para abrir o período.
func TestMuitosVaoComListaVazia(t *testing.T) {
	var l lancamentosDaRodada
	for id := range int64(maximoDeIds + 1) {
		l.somar(id+1, tipos.NovaData(2026, 10, 5))
	}
	corpo, err := json.Marshal(Resultado{Sucesso: true, Lancamentos: l.resultado()})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(corpo), `"lancamentos":{"ids":[],"inicio":"2026-10-05","fim":"2026-10-05"}`) {
		t.Errorf("JSON = %s", corpo)
	}
}

// Regra (#688): a rodada junta a nova e a que mudou de verdade; a sem mudança e a que ela mesma apagou ficam de fora.
func TestRodadaJuntaNovasEAtualizadas(t *testing.T) {
	x := novaRodada(t, false)
	mudou := x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Descricao, tr.DescricaoOriginal, tr.Data = p("t1"), "ANTES", p("ANTES"), tipos.NovaData(2026, 10, 3)
	})
	x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Descricao, tr.DescricaoOriginal, tr.Data = p("t2"), "IGUAL", p("IGUAL"), tipos.NovaData(2026, 10, 1)
		tr.Valor = testeapoio.Valor("-10.0")
	})
	// Pendente de antes da janela, que a Pluggy ainda manda com outra descrição, e que o banco já efetivou (#172): ela
	// conta como atualizada, mas a rodada a apaga.
	apagada := x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Status, tr.Descricao, tr.DescricaoOriginal = p("p1"), pendente, "PEND", p("PEND")
		tr.Data, tr.Valor = tipos.NovaData(2026, 9, 15), testeapoio.Valor("-30.0")
	})
	x.lancar(t, func(tr *testeapoio.Transacao) {
		tr.PluggyTransactionId, tr.Data, tr.Valor = p("e1"), tipos.NovaData(2026, 9, 16), testeapoio.Valor("-30.0")
	})
	novas, atualizadas := x.rodar(t, `{"results":[
		{"id":"t1","date":"2026-10-03T12:00:00Z","description":"DEPOIS","amount":-10,"type":"DEBIT"},
		{"id":"t2","date":"2026-10-01T12:00:00Z","description":"IGUAL","amount":-10,"type":"DEBIT"},
		{"id":"p1","date":"2026-09-15T12:00:00Z","description":"PEND MUDOU","amount":-30,"type":"DEBIT","status":"PENDING"},
		{"id":"n1","date":"2026-10-08T12:00:00Z","description":"NOVA","amount":-5,"type":"DEBIT"}]}`, inicioDoPeriodo, nil)
	if novas != 1 || atualizadas != 2 {
		t.Fatalf("novas %d, atualizadas %d", novas, atualizadas)
	}
	if x.transacao(t, apagada) != nil {
		t.Fatal("o cenário mudou: a pendente já efetivada devia sair")
	}
	nova := x.valor(t, `SELECT "Id" FROM "Transacao" WHERE "PluggyTransactionId" = 'n1'`)
	r := x.r.lancamentos.resultado()
	if r == nil {
		t.Fatal("sem lançamentos")
	}
	if obtido, esperado := fmt.Sprint(r.Ids), fmt.Sprintf("[%d %s]", mudou, nova); obtido != esperado {
		t.Errorf("ids = %s, esperado %s (a sem mudança e a apagada ficam de fora)", obtido, esperado)
	}
	if r.Inicio.String() != "2026-10-03" || r.Fim.String() != "2026-10-08" {
		t.Errorf("período = %s a %s", r.Inicio, r.Fim)
	}
}

// Regra (#688): a etapa desfeita não deixa id na lista.
func TestEtapaDesfeitaNaoEntraNaLista(t *testing.T) {
	x := novaRodada(t, false)
	if _, err := x.db.Exec(`CREATE TRIGGER "falha" BEFORE INSERT ON "Transacao" WHEN NEW."PluggyTransactionId" = 'n2' BEGIN SELECT RAISE(ABORT, 'falha de teste'); END`); err != nil {
		t.Fatal(err)
	}
	x.falsa.Em("GET", "/v2/transactions", testeapoio.RespostaDeFora{Corpo: `{"results":[
		{"id":"n1","date":"2026-10-05T12:00:00Z","amount":1},{"id":"n2","date":"2026-10-06T12:00:00Z","amount":1}]}`})
	if _, _, err := x.r.sincronizarTransacoes(t.Context(), x.conta, inicioDoPeriodo, nil); err == nil {
		t.Fatal("esperado o erro do gatilho")
	}
	if r := x.r.lancamentos.resultado(); r != nil {
		t.Errorf("lançamentos = %+v, esperado nil", r)
	}
}

// Regra (#688): a sincronização inteira devolve as transações que entraram, e nada quando a segunda rodada não muda nada.
func TestSincronizarDevolveOsLancamentos(t *testing.T) {
	c := novoCenario(t)
	c.pluggyCompleta(t)
	c.falsa.Em("GET", "/v2/transactions", arquivo(t, "transacoes-falsa-cartao"))
	r := c.sincronizar(t)
	if r.Lancamentos == nil {
		t.Fatalf("resultado = %+v", r)
	}
	ids := c.valor(t, `SELECT GROUP_CONCAT("Id", ' ') FROM (SELECT "Id" FROM "Transacao" ORDER BY "Id")`)
	if obtido := fmt.Sprint(r.Lancamentos.Ids); obtido != "["+ids+"]" || len(r.Lancamentos.Ids) != r.TransacoesNovas {
		t.Errorf("ids = %s, esperado [%s]", obtido, ids)
	}
	inicio := c.valor(t, `SELECT MIN("Data") FROM "Transacao"`)
	fim := c.valor(t, `SELECT MAX("Data") FROM "Transacao"`)
	if r.Lancamentos.Inicio.String() != inicio || r.Lancamentos.Fim.String() != fim {
		t.Errorf("período = %s a %s, esperado %s a %s", r.Lancamentos.Inicio, r.Lancamentos.Fim, inicio, fim)
	}
	if r := c.sincronizar(t); r.Lancamentos != nil {
		t.Errorf("segunda rodada = %+v, esperado sem lançamentos", r.Lancamentos)
	}
}

// Regra (#688): a conexão que falha depois de gravar as transações (aqui, nos investimentos) devolve as que já entraram:
// elas estão no banco, e o aviso conta com elas.
func TestConexaoQueFalhaDevolveOQueJaEntrou(t *testing.T) {
	c := novoCenario(t)
	c.pluggyCompleta(t)
	c.falsa.Em("GET", "/v2/transactions", arquivo(t, "transacoes-falsa-cartao"))
	c.falsa.Em("GET", "/investments", testeapoio.JSONQuebrado)
	r := c.sincronizar(t)
	if r.Sucesso || r.TransacoesNovas != 3 {
		t.Fatalf("resultado = %+v, esperado a falha com 3 novas", r)
	}
	ids := c.valor(t, `SELECT GROUP_CONCAT("Id", ' ') FROM (SELECT "Id" FROM "Transacao" ORDER BY "Id")`)
	if r.Lancamentos == nil || fmt.Sprint(r.Lancamentos.Ids) != "["+ids+"]" {
		t.Errorf("lançamentos = %+v, esperado [%s]", r.Lancamentos, ids)
	}
}
