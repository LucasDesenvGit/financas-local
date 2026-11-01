package contasagendadas

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// ambiente é o banco vazio, a fábrica e o mux com as rotas, com hoje = 14/10/2026 em Brasília.
func ambiente(t *testing.T) (*sql.DB, *testeapoio.Fabrica, *http.ServeMux) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 14)
	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, r, slog.New(slog.DiscardHandler)))
	return db, testeapoio.NovaFabrica(t, db, r), mux
}

func lerJSON[T any](t *testing.T, corpo []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(corpo, &v); err != nil {
		t.Fatalf("%v: %s", err, corpo)
	}
	return v
}

// Regra: criar e editar validam a categoria, a conta e a data limite, gravam o que o Aplicar dá e conciliam logo; a
// conta a pagar que não existe é 404 antes da validação; o id que não é número é 404 sem corpo.
func TestSalvar(t *testing.T) {
	t.Parallel()
	db, fab, mux := ambiente(t)
	manual := fab.Conta()
	casos := []struct {
		nome, metodo, caminho, corpo string
		status                       int
		mensagem                     string
	}{
		{"categoria inválida", "POST", "/api/contas-agendadas",
			`{"descricao":"x","valor":10,"proximoVencimento":"2026-11-01","recorrencia":1,"categoriaId":999,"contaId":null}`, 400, "Categoria inválida."},
		{"conta inválida", "POST", "/api/contas-agendadas",
			`{"descricao":"x","valor":10,"proximoVencimento":"2026-11-01","recorrencia":1,"categoriaId":null,"contaId":999}`, 400, "Conta inválida."},
		{"limite antes do vencimento", "POST", "/api/contas-agendadas",
			`{"descricao":"x","valor":10,"proximoVencimento":"2026-11-01","recorrencia":1,"categoriaId":null,"contaId":null,"dataLimite":"2026-10-31"}`,
			400, "A data limite (até 31/10/2026) não pode ser antes do próximo vencimento (01/11/2026)."},
		{"limite igual ao vencimento vale", "POST", "/api/contas-agendadas",
			`{"descricao":"Igual","valor":10,"proximoVencimento":"2026-11-01","recorrencia":1,"categoriaId":null,"contaId":null,"dataLimite":"2026-11-01"}`, 204, ""},
		{"única com limite antes vale", "POST", "/api/contas-agendadas",
			`{"descricao":"Única","valor":10,"proximoVencimento":"2026-11-01","recorrencia":0,"categoriaId":null,"contaId":null,"dataLimite":"2026-10-31"}`, 204, ""},
		{"editar a que não existe", "PUT", "/api/contas-agendadas/99",
			`{"descricao":"x","valor":10,"proximoVencimento":"2026-11-01","recorrencia":1,"categoriaId":999,"contaId":null}`, 404, "Registro não encontrado."},
		{"id que não é número", "PUT", "/api/contas-agendadas/abc", `{}`, 404, ""},
		{"id grande demais", "DELETE", "/api/contas-agendadas/3000000000", "", 404, ""},
	}
	for _, c := range casos {
		resposta := testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo)
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
	var limite, unicaLimite *string
	_ = db.QueryRow(`SELECT "DataLimite" FROM "ContaAgendada" WHERE "Descricao" = 'Igual'`).Scan(&limite)
	_ = db.QueryRow(`SELECT "DataLimite" FROM "ContaAgendada" WHERE "Descricao" = 'Única'`).Scan(&unicaLimite)
	if limite == nil || *limite != "2026-11-01" || unicaLimite != nil {
		t.Errorf("limites gravados: %v, %v", limite, unicaLimite)
	}

	// Editar mantém a chave do extrato que já existe e concilia.
	agendada := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) { a.ChaveExtrato = ptr("ANTIGA") })
	fab.Transacao(manual, func(tr *testeapoio.Transacao) {
		tr.Data, tr.Valor, tr.Descricao = dia(13), testeapoio.Valor("-50.0"), "DEB AUT ACADEMIA"
	})
	corpo := `{"descricao":"Academia","valor":50,"proximoVencimento":"2026-10-15","recorrencia":1,"categoriaId":null,` +
		`"contaId":` + itoa(manual.Id) + `,"formaCobranca":2,"textoCobranca":"academia","chaveExtrato":""}`
	if r := testeapoio.Chamar(t, mux, "PUT", "/api/contas-agendadas/"+itoa(agendada.Id), corpo); r.Status != 204 {
		t.Fatalf("editar: %d %s", r.Status, r.Corpo)
	}
	var chave, ultimo, proximo string
	_ = db.QueryRow(`SELECT "ChaveExtrato", "UltimoPagamentoEm", "ProximoVencimento" FROM "ContaAgendada" WHERE "Id" = ?`,
		agendada.Id).Scan(&chave, &ultimo, &proximo)
	if chave != "ANTIGA" || ultimo != "2026-10-13" || proximo != "2026-11-15" {
		t.Errorf("editada: %s %s %s", chave, ultimo, proximo)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

// Regra (#69): pagar avança o vencimento (a única é desativada; passou da data limite, encerrada) e, se pedido, lança
// a despesa na conta manual, com a categoria (manual) e o tipo; na conta do Open Finance ou sem conta, não lança.
func TestPagar(t *testing.T) {
	t.Parallel()
	db, fab, mux := ambiente(t)
	manual := fab.Conta()
	openFinance := fab.Conta(func(c *testeapoio.Conta) { c.Origem = 2 })
	categoria := fab.Categoria()
	mensal := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.Descricao, a.Valor, a.ProximoVencimento, a.ContaId, a.CategoriaId = "Aluguel", testeapoio.Valor("1800.5"), dia(31), &manual.Id, &categoria.Id
	})
	unica := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) { a.Recorrencia = 0 })
	limite := testeapoio.Dia(2026, 11, 15)
	comLimite := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) { a.ProximoVencimento, a.DataLimite = dia(20), &limite })
	noOpenFinance := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) { a.ContaId = &openFinance.Id })

	for _, c := range []struct {
		id     int64
		corpo  string
		status int
	}{
		{mensal.Id, `{"lancarTransacao":true}`, 204},
		{unica.Id, `{"lancarTransacao":false,"data":"2026-10-01"}`, 204},
		{comLimite.Id, `{"lancarTransacao":false}`, 204},
		{noOpenFinance.Id, `{"lancarTransacao":true}`, 400},
		{unica.Id + 1000, `{"lancarTransacao":false}`, 404},
	} {
		if r := testeapoio.Chamar(t, mux, "POST", "/api/contas-agendadas/"+itoa(c.id)+"/pagar", c.corpo); r.Status != c.status {
			t.Errorf("pagar %d: %d %s", c.id, r.Status, r.Corpo)
		}
	}
	type estado struct {
		ultimo, proximo string
		ativa           bool
	}
	ler := func(id int64) estado {
		var e estado
		var ultimo *string
		_ = db.QueryRow(`SELECT "UltimoPagamentoEm", "ProximoVencimento", "Ativa" FROM "ContaAgendada" WHERE "Id" = ?`, id).
			Scan(&ultimo, &e.proximo, &e.ativa)
		if ultimo != nil {
			e.ultimo = *ultimo
		}
		return e
	}
	quer := map[int64]estado{
		mensal.Id:        {"2026-10-14", "2026-11-30", true},
		unica.Id:         {"2026-10-01", "2026-10-14", false},
		comLimite.Id:     {"2026-10-14", "2026-11-20", false},
		noOpenFinance.Id: {"", "2026-10-14", true},
	}
	for id, e := range quer {
		if obtido := ler(id); obtido != e {
			t.Errorf("conta %d: %+v, quer %+v", id, obtido, e)
		}
	}
	var (
		valor                    string
		tipo, status             int64
		categoriaId              *int64
		manualCategoria, ignorar bool
		descricao, data          string
	)
	if err := db.QueryRow(`SELECT "Valor", "Tipo", "Status", "CategoriaId", "CategoriaManual", "IgnorarRelatorios",
		"Descricao", "Data" FROM "Transacao" WHERE "ContaId" = ?`, manual.Id).Scan(&valor, &tipo, &status, &categoriaId,
		&manualCategoria, &ignorar, &descricao, &data); err != nil {
		t.Fatal(err)
	}
	if valor != "-1800.5" || tipo != 1 || status != 1 || categoriaId == nil || *categoriaId != categoria.Id || !manualCategoria ||
		ignorar || descricao != "Aluguel" || data != "2026-10-14" {
		t.Errorf("despesa: %s %d %d %v %v %v %s %s", valor, tipo, status, categoriaId, manualCategoria, ignorar, descricao, data)
	}
}

// Regra (#205): o "é a mesma" grava a chave aparada e o desfazer a apaga; a que não existe é 404; excluir apaga.
func TestLigarEExcluir(t *testing.T) {
	t.Parallel()
	db, fab, mux := ambiente(t)
	a := fab.ContaAgendada()
	caminho := "/api/contas-agendadas/" + itoa(a.Id)
	for _, c := range []struct {
		metodo, caminho, corpo string
		status                 int
		chave                  *string
	}{
		{"POST", caminho + "/ligar-extrato", `{"chaveExtrato":"  LUZ  "}`, 204, ptr("LUZ")},
		{"POST", "/api/contas-agendadas/999/ligar-extrato", `{"chaveExtrato":"LUZ"}`, 404, ptr("LUZ")},
		{"DELETE", caminho + "/ligar-extrato", "", 204, nil},
		{"DELETE", "/api/contas-agendadas/999/ligar-extrato", "", 404, nil},
	} {
		if r := testeapoio.Chamar(t, mux, c.metodo, c.caminho, c.corpo); r.Status != c.status {
			t.Errorf("%s %s: %d %s", c.metodo, c.caminho, r.Status, r.Corpo)
		}
		var chave *string
		_ = db.QueryRow(`SELECT "ChaveExtrato" FROM "ContaAgendada" WHERE "Id" = ?`, a.Id).Scan(&chave)
		if !iguais(chave, c.chave) {
			t.Errorf("%s %s: chave %v", c.metodo, c.caminho, chave)
		}
	}
	if r := testeapoio.Chamar(t, mux, "DELETE", caminho, ""); r.Status != 204 {
		t.Errorf("excluir: %d", r.Status)
	}
	if r := testeapoio.Chamar(t, mux, "DELETE", caminho, ""); r.Status != 404 {
		t.Errorf("excluir de novo: %d", r.Status)
	}
	if r := testeapoio.Chamar(t, mux, "GET", "/api/contas-agendadas", ""); r.Status != 200 || string(r.Corpo) != "[]" {
		t.Errorf("listar vazia: %d %s", r.Status, r.Corpo)
	}
}

// Regra (#158): os quatro avisos das recorrentes ativas.
func TestAvisos(t *testing.T) {
	t.Parallel()
	db, fab, mux := ambiente(t)
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Nome, c.Tipo = "Cartão novo", 3 })
	antigo := fab.Conta(func(c *testeapoio.Conta) { c.Nome, c.Arquivada = "Cartão antigo", true })
	cobrar := func(conta testeapoio.Conta, d tipos.Data, valor, descricao string) {
		fab.Transacao(conta, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Descricao = d, testeapoio.Valor(valor), descricao
		})
	}
	// No cartão arquivado, mudou para o cartão novo e aumentou.
	streaming := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.Descricao, a.Valor, a.ProximoVencimento, a.FormaCobranca, a.TextoCobranca, a.ContaId = "Streaming", testeapoio.Valor("39.9"),
			dia(20), 3, ptr("STREAMFLIX"), &antigo.Id
	})
	cobrar(cartao, testeapoio.Dia(2026, 9, 20), "-44.9", "STREAMFLIX")
	// Débito automático que não veio (venceu há 6 dias).
	luz := fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.Descricao, a.ProximoVencimento, a.FormaCobranca, a.TextoCobranca = "Luz", dia(8), 2, ptr("ENEL")
	})
	// Valor que varia sempre (água): não avisa o aumento.
	fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.Descricao, a.Valor, a.ProximoVencimento, a.TextoCobranca = "Água", testeapoio.Valor("100.0"), dia(25), ptr("SABESP")
	})
	for i, v := range []string{"-150.0", "-90.0", "-120.0"} {
		cobrar(cartao, dia(5-i), v, "SABESP")
	}
	// À mão que venceu: não é "não veio"; e a única não entra.
	fab.ContaAgendada(func(a *testeapoio.ContaAgendada) { a.ProximoVencimento, a.TextoCobranca = dia(1), ptr("ALUGUEL") })
	fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.Recorrencia, a.ProximoVencimento, a.FormaCobranca, a.TextoCobranca = 0, dia(1), 2, ptr("IPVA")
	})

	r := testeapoio.Chamar(t, mux, "GET", "/api/contas-agendadas/avisos", "")
	avisos := lerJSON[[]AvisoDto](t, r.Corpo)
	tipos := map[int64][]string{}
	for _, a := range avisos {
		tipos[a.ContaAgendadaId] = append(tipos[a.ContaAgendadaId], a.Tipo+": "+a.Texto)
	}
	quer := map[int64][]string{
		streaming.Id: {
			"cartao-arquivado: Cartão antigo está arquivado, mas Streaming ainda cobra nele: troque nos serviços.",
			"mudou-de-cartao: A última cobrança de Streaming (20/09) veio em Cartão novo, e não em Cartão antigo.",
			"aumentou: Streaming aumentou na cobrança de 20/09.",
		},
		luz.Id: {"nao-veio: A cobrança de Luz esperada em 08/10 não apareceu. Cancelada? Cartão recusado?"},
	}
	if len(tipos) != len(quer) {
		t.Errorf("avisos: %v", tipos)
	}
	for id, lista := range quer {
		if len(tipos[id]) != len(lista) {
			t.Errorf("conta %d: %v", id, tipos[id])
			continue
		}
		for i := range lista {
			if tipos[id][i] != lista[i] {
				t.Errorf("conta %d, aviso %d: %q", id, i, tipos[id][i])
			}
		}
	}
	for _, a := range avisos {
		if a.Tipo == "aumentou" && (a.ValorAnterior.String() != "39.9" || a.ValorNovo.String() != "44.9") {
			t.Errorf("valores do aumento: %v %v", a.ValorAnterior, a.ValorNovo)
		}
	}
	_ = db
}

// Regra (#62, #158): a sugestão mensal do extrato, com o próximo vencimento nunca no passado; a já cadastrada some; no
// cartão, valor que varia mais de 5% não é assinatura; a anual tem duas cobranças a um ano.
func TestSugerir(t *testing.T) {
	t.Parallel()
	_, fab, mux := ambiente(t)
	corrente := fab.Conta(func(c *testeapoio.Conta) { c.Nome = "Corrente" })
	cartao := fab.Conta(func(c *testeapoio.Conta) { c.Nome, c.Tipo = "Cartão", 3 })
	categoria := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Moradia" })
	gastar := func(conta testeapoio.Conta, d tipos.Data, valor, descricao string, cat *int64) {
		fab.Transacao(conta, func(tr *testeapoio.Transacao) {
			tr.Data, tr.Valor, tr.Descricao, tr.CategoriaId = d, testeapoio.Valor(valor), descricao, cat
		})
	}
	// Condomínio no dia 5, três meses: o de outubro não veio, então a sugestão é novembro, com o aviso.
	for _, m := range []int{9, 8, 7} {
		gastar(corrente, testeapoio.Dia(2026, toMes(m), 5), "-700.0", "DEB AUT CONDOMINIO PARQUE", &categoria.Id)
	}
	// Assinatura no cartão com o preço que variou 10%: não é sugestão.
	for i, v := range []string{"-50.0", "-45.0", "-50.0"} {
		gastar(cartao, testeapoio.Dia(2026, toMes(10-i), 2), v, "LOJA VARIAVEL", nil)
	}
	// Seguro anual: duas cobranças a 346 dias, dentro da janela de 400 dias; os valores variam mais de 15%.
	gastar(corrente, testeapoio.Dia(2026, 10, 1), "-1200.0", "SEGURO ANUAL CASA", nil)
	gastar(corrente, testeapoio.Dia(2025, 10, 20), "-900.0", "SEGURO ANUAL CASA", nil)

	sugestoes := lerJSON[[]SugestaoDto](t, testeapoio.Chamar(t, mux, "GET", "/api/contas-agendadas/sugestoes", "").Corpo)
	if len(sugestoes) != 2 {
		t.Fatalf("sugestões: %+v", sugestoes)
	}
	condominio, seguro := sugestoes[0], sugestoes[1]
	if condominio.Chave != "CONDOMINIO PARQUE" || condominio.Valor.String() != "700.0" || condominio.ProximoVencimento.String() != "2026-11-05" ||
		condominio.VencimentoSemPagamento == nil || condominio.VencimentoSemPagamento.String() != "2026-10-05" ||
		condominio.FormaCobranca != 2 || condominio.DiaHabitual != 5 || condominio.Meses != 3 || condominio.Pagamentos != 3 ||
		condominio.CategoriaId == nil || *condominio.CategoriaId != categoria.Id || condominio.ContaNome != "Corrente" ||
		*condominio.TextoCobranca != "CONDOMINIO" || condominio.Recorrencia != 1 {
		t.Errorf("condomínio: %+v", condominio)
	}
	if seguro.Chave != "SEGURO ANUAL CASA" || seguro.Recorrencia != 2 || seguro.ProximoVencimento.String() != "2027-10-01" ||
		seguro.VencimentoSemPagamento != nil || seguro.Valor.String() != "1050.0" || !seguro.ValorVariavel ||
		seguro.ValorMinimo.String() != "900.0" || seguro.ValorMaximo.String() != "1200.0" {
		t.Errorf("seguro: %+v", seguro)
	}

	// Cadastrada pela sugestão (a chave do extrato), ela some.
	fab.ContaAgendada(func(a *testeapoio.ContaAgendada) {
		a.Descricao, a.ChaveExtrato = "Meu prédio", ptr("CONDOMINIO PARQUE")
	})
	sugestoes = lerJSON[[]SugestaoDto](t, testeapoio.Chamar(t, mux, "GET", "/api/contas-agendadas/sugestoes", "").Corpo)
	if len(sugestoes) != 1 || sugestoes[0].Chave != "SEGURO ANUAL CASA" {
		t.Errorf("depois de cadastrar: %+v", sugestoes)
	}
}

func toMes(m int) time.Month { return time.Month(m) }
