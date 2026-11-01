package metas

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/parametros"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/reservas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func dec(s string) tipos.Decimal { return testeapoio.Valor(s) }

func ptr[T any](v T) *T { return &v }

func ambiente(t *testing.T, r relogio.Relogio) (*sql.DB, *testeapoio.Fabrica, *http.ServeMux, *Servico) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	log := slog.New(slog.DiscardHandler)
	s := NovoServico(db, reservas.NovoServico(db, nil, r, log), parametros.Novo(parametros.Opcoes{DB: db, Relogio: r, Log: log}), r, log)
	mux := http.NewServeMux()
	Registrar(mux, s)
	return db, testeapoio.NovaFabrica(t, db, r), mux, s
}

func lerJSON[T any](t *testing.T, corpo []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(corpo, &v); err != nil {
		t.Fatalf("%v: %s", err, corpo)
	}
	return v
}

// Regra (#89): o progresso é o valor guardado mais o saldo positivo da conta (sem cofrinho) ou o saldo do cofrinho
// (nunca os dois); o percentual tem 1 casa e para em 100; a sugestão por mês divide o que falta pelos meses até a data
// (no mínimo 1); a meta que chega ao alvo conta como concluída; o cofrinho que sumiu do extrato é avisado.
func TestParaDto(t *testing.T) {
	t.Parallel()
	hoje := testeapoio.Dia(2026, 10, 14)
	saldos := map[int64]tipos.Decimal{1: dec("300.0"), 2: dec("-50.0")}
	lista := []reservas.ReservaDto{{ContaId: 1, Nome: "Caixinha Viagem", Saldo: dec("800.0"), ConferenciaAntiga: true}}
	casos := []struct {
		nome                        string
		m                           meta
		atual, percentual, sugestao string
		concluida, conferir, sumiu  bool
		reserva                     string
	}{
		{"só o guardado", meta{valorAlvo: dec("1000.0"), valorAtual: dec("250.0"), dataAlvo: ptr(testeapoio.Dia(2026, 12, 31))},
			"250.0", "25.0", "375.0", false, false, false, ""},
		{"com o saldo da conta", meta{valorAlvo: dec("1000.0"), valorAtual: dec("100.0"), contaId: ptr(int64(1))},
			"400.0", "40.0", "", false, false, false, ""},
		{"saldo negativo da conta não conta", meta{valorAlvo: dec("1000.0"), valorAtual: dec("100.0"), contaId: ptr(int64(2))},
			"100.0", "10.0", "", false, false, false, ""},
		{"com o cofrinho (não soma a conta)", meta{valorAlvo: dec("1000.0"), valorAtual: dec("100.0"), contaId: ptr(int64(1)),
			reservaNome: ptr("caixinha viagem")}, "900.0", "90.0", "", false, true, false, "Caixinha Viagem"},
		{"cofrinho que sumiu", meta{valorAlvo: dec("1000.0"), valorAtual: dec("100.0"), contaId: ptr(int64(1)),
			reservaNome: ptr("Outra")}, "100.0", "10.0", "", false, false, true, "Outra"},
		{"passou do alvo", meta{valorAlvo: dec("100.0"), valorAtual: dec("150.0"), dataAlvo: ptr(testeapoio.Dia(2027, 1, 1))},
			"150.0", "100", "", true, false, false, ""},
		{"data no mês atual divide por 1", meta{valorAlvo: dec("90.0"), valorAtual: dec("0"), dataAlvo: ptr(testeapoio.Dia(2026, 10, 31))},
			"0", "0", "90.0", false, false, false, ""},
		{"concluída à mão não sugere", meta{valorAlvo: dec("90.0"), valorAtual: dec("0"), concluida: true,
			dataAlvo: ptr(testeapoio.Dia(2027, 10, 31))}, "0", "0", "", true, false, false, ""},
		{"um terço arredonda pelo método bancário", meta{valorAlvo: dec("3.0"), valorAtual: dec("1.0")}, "1.0", "33.3", "", false, false, false, ""},
	}
	for _, c := range casos {
		dto, err := paraDto(c.m, saldos, lista, hoje)
		if err != nil {
			t.Fatal(err)
		}
		sugestao, reserva := "", ""
		if dto.AporteMensalSugerido != nil {
			sugestao = dto.AporteMensalSugerido.String()
		}
		if dto.ReservaNome != nil {
			reserva = *dto.ReservaNome
		}
		if dto.ValorAtual.String() != c.atual || dto.Percentual.String() != c.percentual || sugestao != c.sugestao ||
			dto.Concluida != c.concluida || dto.ReservaConferir != c.conferir || dto.ReservaNaoEncontrada != c.sumiu || reserva != c.reserva {
			t.Errorf("%s: atual %s, %% %s, sugestão %q, concluída %t, conferir %t, sumiu %t, reserva %q", c.nome, dto.ValorAtual,
				dto.Percentual, sugestao, dto.Concluida, dto.ReservaConferir, dto.ReservaNaoEncontrada, reserva)
		}
	}
}

func inserirMeta(t *testing.T, db *sql.DB, nome string, prioridade int64, concluida bool, dataAlvo string) int64 {
	t.Helper()
	var data any
	if dataAlvo != "" {
		data = dataAlvo
	}
	res, err := db.Exec(`INSERT INTO "Meta" ("Nome", "ValorAlvo", "ValorAtual", "DataAlvo", "Cor", "Concluida", "Prioridade",
		"CriadoEm") VALUES (?, '1000.0', '0.0', ?, '#000000', ?, ?, '2026-01-01 12:00:00')`, nome, data, concluida, prioridade)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// Regra: a lista põe as concluídas no fim, depois a prioridade (0 no fim), a data (sem data no fim) e o nome; a nova
// ordem põe as da lista primeiro e as outras na ordem que tinham; repetida e inexistente são recusadas.
func TestOrdem(t *testing.T) {
	t.Parallel()
	db, _, mux, _ := ambiente(t, testeapoio.HojeEmBrasilia(2026, 10, 14))
	feita := inserirMeta(t, db, "Feita", 1, true, "")
	semOrdem := inserirMeta(t, db, "Sem ordem", 0, false, "2027-01-01")
	segundaB := inserirMeta(t, db, "B", 2, false, "")
	segundaA := inserirMeta(t, db, "A", 2, false, "")
	cedo := inserirMeta(t, db, "Cedo", 2, false, "2026-12-01")
	nomes := func() string {
		var partes []string
		for _, m := range lerJSON[[]MetaDto](t, testeapoio.Chamar(t, mux, "GET", "/api/metas", "").Corpo) {
			partes = append(partes, m.Nome)
		}
		return strings.Join(partes, ", ")
	}
	if n := nomes(); n != "Cedo, A, B, Sem ordem, Feita" {
		t.Errorf("lista: %s", n)
	}
	casos := []struct {
		corpo    string
		status   int
		mensagem string
	}{
		{`{"ids":[1,1]}`, 400, "Meta repetida na ordem."},
		{`{"ids":[1,999]}`, 400, "Meta inválida."},
		{`{"ids":[]}`, 400, ""},
		{`{"ids":[` + itoa(semOrdem) + `]}`, 204, ""},
	}
	for _, c := range casos {
		r := testeapoio.Chamar(t, mux, "PUT", "/api/metas/prioridades", c.corpo)
		if r.Status != c.status || (c.mensagem != "" && lerJSON[map[string]string](t, r.Corpo)["mensagem"] != c.mensagem) {
			t.Errorf("%s: %d %s", c.corpo, r.Status, r.Corpo)
		}
	}
	// "Sem ordem" em 1º; depois as outras pela prioridade que tinham, o empate pelo Id (Feita, B, A, Cedo).
	quer := map[int64]int64{semOrdem: 1, feita: 2, segundaB: 3, segundaA: 4, cedo: 5}
	for id, p := range quer {
		var obtido int64
		_ = db.QueryRow(`SELECT "Prioridade" FROM "Meta" WHERE "Id" = ?`, id).Scan(&obtido)
		if obtido != p {
			t.Errorf("meta %d: prioridade %d, quer %d", id, obtido, p)
		}
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

// Regra: criar entra no fim da ordem, com o nome sem espaços nas pontas; o cofrinho precisa da conta e de existir nela
// (sem diferenciar maiúsculas), mas editar mantém o que já estava gravado mesmo que tenha sumido; sem conta, o cofrinho
// é descartado; conta e meta inexistentes são recusadas.
func TestCriarEEditar(t *testing.T) {
	t.Parallel()
	db, fab, mux, _ := ambiente(t, testeapoio.HojeEmBrasilia(2026, 10, 14))
	conta := fab.Conta(func(c *testeapoio.Conta) {
		c.Origem, c.SaldoReservado, c.ReservasInformadas = 2, ptr(dec("800.0")), ptr(`[{"nome":"Caixinha viagem","valor":800.0}]`)
	})
	outra := fab.Conta()
	inserirMeta(t, db, "Antiga", 7, false, "")
	base := `"valorAlvo":1000,"valorAtual":0,"cor":"#123abc","concluida":false`
	r := testeapoio.Chamar(t, mux, "POST", "/api/metas", `{"nome":"  Carro  ",`+base+`,"icone":"carro"}`)
	carro := lerJSON[MetaDto](t, r.Corpo)
	if r.Status != 200 || carro.Nome != "Carro" || carro.Prioridade != 8 || carro.Icone == nil || *carro.Icone != "carro" {
		t.Errorf("criar: %d %s", r.Status, r.Corpo)
	}
	r = testeapoio.Chamar(t, mux, "POST", "/api/metas", `{"nome":"Viagem",`+base+`,"contaId":`+itoa(conta.Id)+`,"reservaNome":" CAIXINHA viagem "}`)
	viagem := lerJSON[MetaDto](t, r.Corpo)
	if r.Status != 200 || viagem.ReservaNome == nil || *viagem.ReservaNome != "Caixinha viagem" || viagem.SaldoReserva == nil {
		t.Errorf("criar com cofrinho: %d %s", r.Status, r.Corpo)
	}
	// O cofrinho some do extrato: editar mantendo o mesmo nome continua aceito; trocar para outro que não existe, não.
	if _, err := db.Exec(`UPDATE "Conta" SET "SaldoReservado" = NULL, "ReservasInformadas" = NULL WHERE "Id" = ?`, conta.Id); err != nil {
		t.Fatal(err)
	}
	conta1 := `,"contaId":` + itoa(conta.Id)
	casos := []struct {
		metodo, caminho, corpo string
		status                 int
		mensagem               string
	}{
		{"PUT", "/api/metas/" + itoa(viagem.Id), `{"nome":"Viagem",` + base + conta1 + `,"reservaNome":"caixinha VIAGEM"}`, 200, ""},
		{"PUT", "/api/metas/" + itoa(viagem.Id), `{"nome":"Viagem",` + base + conta1 + `,"reservaNome":"Outra"}`, 400,
			"Cofrinho inválido para esta conta."},
		{"PUT", "/api/metas/" + itoa(viagem.Id), `{"nome":"Viagem",` + base + `,"contaId":` + itoa(outra.Id) +
			`,"reservaNome":"Caixinha viagem"}`, 400, "Cofrinho inválido para esta conta."},
		{"POST", "/api/metas", `{"nome":"X",` + base + `,"reservaNome":"Caixinha viagem"}`, 400, "Escolha a conta do cofrinho."},
		{"POST", "/api/metas", `{"nome":"X",` + base + `,"contaId":999}`, 400, "Conta inválida."},
		{"PUT", "/api/metas/999", `{"nome":"X",` + base + `}`, 404, "Registro não encontrado."},
		{"PUT", "/api/metas/" + itoa(carro.Id), `{"nome":"X",` + base + `,"contaId":999}`, 400, "Conta inválida."},
		{"PUT", "/api/metas/x", `{"nome":"X",` + base + `}`, 404, ""},
		{"POST", "/api/metas", `{"nome":"X","valorAlvo":0,"valorAtual":0,"cor":"azul","concluida":false}`, 400, ""},
	}
	for _, c := range casos {
		r := testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo)
		if r.Status != c.status || (c.mensagem != "" && lerJSON[map[string]string](t, r.Corpo)["mensagem"] != c.mensagem) {
			t.Errorf("%s %s %s: %d %s", c.metodo, c.caminho, c.corpo, r.Status, r.Corpo)
		}
	}
	// Sem conta, o cofrinho é descartado.
	r = testeapoio.Chamar(t, mux, "PUT", "/api/metas/"+itoa(viagem.Id), `{"nome":"Viagem",`+base+`,"reservaNome":""}`)
	if m := lerJSON[MetaDto](t, r.Corpo); r.Status != 200 || m.ReservaNome != nil || m.ContaId != nil {
		t.Errorf("sem conta: %d %s", r.Status, r.Corpo)
	}
}

// Regra (#318): o aporte soma ou retira, para no zero, conclui ao chegar ao alvo e guarda só o que de fato mudou, com
// a data de hoje; os aportes saem do mais antigo ao mais novo, com a hora de Brasília; excluir a meta leva os aportes.
func TestAportes(t *testing.T) {
	t.Parallel()
	db, _, mux, _ := ambiente(t, testeapoio.AgoraEmBrasilia(2026, 10, 14, 21, 30))
	id := inserirMeta(t, db, "Meta", 1, false, "")
	caminho := "/api/metas/" + itoa(id)
	passos := []struct {
		valor, atual string
		concluida    bool
	}{{"300.5", "300.5", false}, {"-1000", "0", false}, {"-10", "0", false}, {"1000", "1000", true}}
	for _, p := range passos {
		r := testeapoio.Chamar(t, mux, "POST", caminho+"/aporte", `{"valor":`+p.valor+`}`)
		m := lerJSON[MetaDto](t, r.Corpo)
		if r.Status != 200 || !m.ValorAtual.Decimal().Equal(dec(p.atual).Decimal()) || m.Concluida != p.concluida {
			t.Errorf("aporte %s: %d %s", p.valor, r.Status, r.Corpo)
		}
	}
	if _, err := db.Exec(`INSERT INTO "MetaAporte" ("MetaId", "Data", "Valor", "CriadoEm") VALUES (?, '2026-08-06', '5.0',
		'2026-08-06 12:00:00')`, id); err != nil {
		t.Fatal(err)
	}
	lista := lerJSON[[]map[string]any](t, testeapoio.Chamar(t, mux, "GET", caminho+"/aportes", "").Corpo)
	// O de agosto primeiro; depois os três de hoje (a retirada de −10 com a meta no zero não grava nada).
	if len(lista) != 4 || lista[0]["hora"] != "09:00" || lista[1]["valor"] != 300.5 || lista[2]["valor"] != -300.5 ||
		lista[3]["valor"] != 1000.0 || lista[1]["data"] != "2026-10-14" || lista[1]["hora"] != "21:30" {
		t.Errorf("aportes: %v", lista)
	}
	for _, c := range []struct {
		metodo, caminho, corpo string
		status                 int
	}{
		{"POST", "/api/metas/999/aporte", `{"valor":1}`, 404},
		{"POST", caminho + "/aporte", `{"valor":1000000000}`, 400},
		{"POST", "/api/metas/x/aporte", `{"valor":1}`, 404},
		{"GET", "/api/metas/999/aportes", "", 404},
		{"GET", "/api/metas/x/aportes", "", 404},
		{"DELETE", "/api/metas/999", "", 404},
		{"DELETE", "/api/metas/x", "", 404},
		{"DELETE", caminho, "", 204},
	} {
		if r := testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo); r.Status != c.status {
			t.Errorf("%s %s: %d %s", c.metodo, c.caminho, r.Status, r.Corpo)
		}
	}
	var sobraram int
	_ = db.QueryRow(`SELECT COUNT(*) FROM "MetaAporte"`).Scan(&sobraram)
	if sobraram != 0 {
		t.Errorf("os aportes ficaram: %d", sobraram)
	}
}

// Regra (#149): com menos de 3 meses completos o plano não sai; com histórico, o mês atual é o realizado mais a média
// do que falta (receitas: o maior entre o recebido e a média), os seguintes são as médias, e o saldo no fim do mês
// tira o que ainda vai sair. Conferido à mão: média de despesas 3.000, 17 de 31 dias por vir.
func TestPlanejarCaixa(t *testing.T) {
	t.Parallel()
	_, fab, mux, s := ambiente(t, testeapoio.HojeEmBrasilia(2026, 10, 14))
	corrente := fab.Conta()
	fab.Conta(func(c *testeapoio.Conta) { c.Tipo, c.IncluirNoSaldo = 1, false }) // fora do saldo: não conta
	transacao := func(dia tipos.Data, valor string, tipo int64) {
		fab.Transacao(corrente, func(tr *testeapoio.Transacao) { tr.Data, tr.Valor, tr.Tipo = dia, dec(valor), tipo })
	}
	transacao(testeapoio.Dia(2026, 8, 1), "6000.0", 2)
	transacao(testeapoio.Dia(2026, 8, 2), "-3000.0", 1)
	p, err := s.Planejar(t.Context())
	if err != nil || p.Disponivel || p.MesesConsiderados != 2 || len(p.Meses) != 0 || p.MargemPercentual != 10 {
		t.Fatalf("com 2 meses: %+v %v", p, err)
	}
	for m := time.Month(4); m <= 7; m++ {
		transacao(testeapoio.Dia(2026, m, 1), "6000.0", 2)
		transacao(testeapoio.Dia(2026, m, 2), "-3000.0", 1)
	}
	transacao(testeapoio.Dia(2026, 9, 1), "6000.0", 2)
	transacao(testeapoio.Dia(2026, 9, 2), "-1000.0", 1) // setembro, o melhor mês: sobrou 5.000
	transacao(testeapoio.Dia(2026, 9, 3), "-2000.0", 1)
	transacao(testeapoio.Dia(2026, 9, 4), "1000.0", 2)
	transacao(testeapoio.Dia(2026, 10, 5), "5000.0", 2)
	transacao(testeapoio.Dia(2026, 10, 10), "-1000.0", 1)

	r := testeapoio.Chamar(t, mux, "GET", "/api/metas/plano", "")
	p = lerJSON[PlanoMetasDto](t, r.Corpo)
	if r.Status != 200 || !p.Disponivel || p.MesesConsiderados != 6 || len(p.Meses) != horizonteMeses || p.Ano != 2026 || p.Mes != 10 {
		t.Fatalf("plano: %d %+v", r.Status, p)
	}
	// Receitas: média 37.000 ÷ 6 = 6.166,67 > recebido 5.000. Despesas: 1.000 + 3.000 × 17/31 = 2.645,16.
	atual, seguinte, ultimo := p.Meses[0], p.Meses[1], p.Meses[horizonteMeses-1]
	for nome, par := range map[string][2]tipos.Decimal{
		"receitas do mês":   {atual.Receitas, dec("6166.67")},
		"despesas do mês":   {atual.Despesas, dec("2645.16")},
		"sobra do mês":      {atual.Sobra, dec("3521.51")},
		"receitas depois":   {seguinte.Receitas, dec("6166.67")},
		"despesas depois":   {seguinte.Despesas, dec("3000")},
		"saldo das contas":  {p.SaldoContas, dec("23000")},
		"saldo no fim":      {p.SaldoFimDoMes, dec("22521.51")},
		"melhor mês":        {p.MelhorMesRecente, dec("4000")},
		"fatura a pagar":    {p.FaturaAPagar, dec("0")},
		"faturas do último": {ultimo.Faturas, dec("0")},
	} {
		if !par[0].Decimal().Equal(par[1].Decimal()) {
			t.Errorf("%s: %s, quer %s", nome, par[0], par[1])
		}
	}
	if ultimo.Ano != 2036 || ultimo.Mes != 9 {
		t.Errorf("último mês: %d/%d", ultimo.Mes, ultimo.Ano)
	}
}

// Regra (#144, #165, #251): a fatura fechada vale o total do banco e entra no mês do vencimento; a que vence de hoje
// em diante entra no "a pagar", menos o que já foi pago; as parcelas futuras de uma compra caem nas faturas seguintes,
// e a soma das parcelas do horizonte é a lançada mais as projetadas.
func TestPlanejarCartao(t *testing.T) {
	t.Parallel()
	_, fab, _, s := ambiente(t, testeapoio.HojeEmBrasilia(2026, 10, 14))
	corrente := fab.Conta()
	for m := time.Month(4); m <= 10; m++ {
		fab.Transacao(corrente, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Tipo = testeapoio.Dia(2026, m, 1), dec("5000.0"), 2
		})
	}
	cartao := fab.Conta(func(c *testeapoio.Conta) {
		c.Tipo, c.DiaFechamento, c.DiaVencimento = tipoCartao, ptr(int64(5)), ptr(int64(15))
	})
	fab.Fatura(cartao, testeapoio.Dia(2026, 10, 15), dec("-500.0"), func(f *testeapoio.FaturaCartao) {
		f.DataFechamento, f.ValorPago = ptr(testeapoio.Dia(2026, 10, 5)), ptr(dec("200.0"))
	})
	fab.Transacao(cartao, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Valor, tr.Descricao = testeapoio.Dia(2026, 10, 8), dec("-100.0"), "LOJA 3/5"
		tr.ParcelaNumero, tr.ParcelaTotal = ptr(int64(3)), ptr(int64(5))
	})
	p, err := s.Planejar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !p.FaturaAPagar.Decimal().Equal(dec("300").Decimal()) || !p.Meses[0].Faturas.Decimal().Equal(dec("500").Decimal()) {
		t.Errorf("a pagar %s, fatura de outubro %s", p.FaturaAPagar, p.Meses[0].Faturas)
	}
	soma, meses := tipos.Zero, 0
	for _, m := range p.Meses {
		if m.Parcelas.Decimal().IsPositive() {
			soma, meses = tipos.Somar(soma, m.Parcelas), meses+1
		}
	}
	if !soma.Decimal().Equal(dec("300").Decimal()) || meses != 3 {
		t.Errorf("parcelas: %s em %d meses", soma, meses)
	}
}

// Regra: falha do banco é 500 com a mensagem genérica.
func TestErroDoBanco(t *testing.T) {
	t.Parallel()
	db, _, mux, _ := ambiente(t, testeapoio.HojeEmBrasilia(2026, 10, 14))
	_ = db.Close()
	for _, c := range []struct{ metodo, caminho, corpo string }{
		{"GET", "/api/metas", ""},
		{"GET", "/api/metas/plano", ""},
		{"PUT", "/api/metas/prioridades", `{"ids":[1]}`},
		{"POST", "/api/metas", `{"nome":"X","valorAlvo":1,"valorAtual":0,"cor":"#000000","concluida":false,"contaId":1}`},
		{"PUT", "/api/metas/1", `{"nome":"X","valorAlvo":1,"valorAtual":0,"cor":"#000000","concluida":false}`},
		{"POST", "/api/metas/1/aporte", `{"valor":1}`},
		{"GET", "/api/metas/1/aportes", ""},
		{"DELETE", "/api/metas/1", ""},
	} {
		if r := testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo); r.Status != http.StatusInternalServerError {
			t.Errorf("%s %s: %d", c.metodo, c.caminho, r.Status)
		}
	}
}
