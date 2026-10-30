package extrato

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// regraDescricao grava uma regra de descrição (texto, tipo e nome novo) e devolve o Id.
func regraDescricao(t *testing.T, db *sql.DB, texto string, tipo int, novo string) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO RegraDescricao (Texto, TipoCorrespondencia, NovaDescricao, Ativa, CriadoEm)
		VALUES (?, ?, ?, 1, '2026-10-01 00:00:00')`, texto, tipo, novo)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

var proximoDoBanco atomic.Int64

// doBanco: a transação veio do Open Finance, com essa descrição do banco (e o nome automático, igual a ela).
func doBanco(original string) func(*testeapoio.Transacao) {
	return func(x *testeapoio.Transacao) {
		id := fmt.Sprintf("tx-%d", proximoDoBanco.Add(1))
		x.PluggyTransactionId, x.DescricaoOriginal, x.Descricao = &id, &original, original
	}
}

type resultadoRenomear struct {
	Alteradas       int
	RegraId         *int64
	RegrasCriadas   int
	RegrasAlteradas int
	Desfazer        *string
}

func lerResultado(t *testing.T, resp testeapoio.Resposta) resultadoRenomear {
	t.Helper()
	var r resultadoRenomear
	if err := json.Unmarshal(resp.Corpo, &r); err != nil {
		t.Fatalf("%v: %s", err, resp.Corpo)
	}
	return r
}

// Regra: a prévia do renomear: sem texto, a descrição do banco (com "é igual a") ou o sugerido (com os outros tipos);
// o tipo pelo nome ou pelo número; o lançamento manual não tem o que sugerir; os erros do tipo e do id.
func TestPreviaRenomear(t *testing.T) {
	mux, _, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	luz := fab.Transacao(conta, doBanco("DEBITO ENERGIA 10/2026"))
	fab.Transacao(conta, doBanco("DEBITO ENERGIA 11/2026"))
	manual := fab.Transacao(conta)
	resp := testeapoio.Chamar(t, mux, "GET", fmt.Sprintf("/api/transacoes/%d/renomear/previa", luz.Id), "")
	resp.ConferirStatus(t, http.StatusOK)
	resp.ConferirJSON(t, `{"descricaoOriginal":"DEBITO ENERGIA 10/2026","textoSugerido":"DEBITO ENERGIA","quantidade":0,"exemplos":[]}`)
	resp = testeapoio.Chamar(t, mux, "GET", fmt.Sprintf("/api/transacoes/%d/renomear/previa?tipo=Contem", luz.Id), "")
	resp.ConferirJSON(t, `{"descricaoOriginal":"DEBITO ENERGIA 10/2026","textoSugerido":"DEBITO ENERGIA","quantidade":1,
		"exemplos":["DEBITO ENERGIA 11/2026"]}`)
	resp = testeapoio.Chamar(t, mux, "GET", fmt.Sprintf("/api/transacoes/%d/renomear/previa?texto=%%20debito%%20&tipo=2", luz.Id), "")
	resp.ConferirJSON(t, `{"descricaoOriginal":"DEBITO ENERGIA 10/2026","textoSugerido":"DEBITO ENERGIA","quantidade":1,
		"exemplos":["DEBITO ENERGIA 11/2026"]}`)
	resp = testeapoio.Chamar(t, mux, "GET", fmt.Sprintf("/api/transacoes/%d/renomear/previa", manual.Id), "")
	resp.ConferirJSON(t, `{"descricaoOriginal":null,"textoSugerido":"","quantidade":0,"exemplos":[]}`)
	casos := []struct {
		caminho string
		status  int
		erro    string
	}{
		{fmt.Sprintf("/api/transacoes/%d/renomear/previa?tipo=9", luz.Id), 400, `{"tipo":["The value '9' is invalid."]}`},
		{fmt.Sprintf("/api/transacoes/%d/renomear/previa?tipo=x", luz.Id), 400, `{"tipo":["The value 'x' is not valid."]}`},
		{"/api/transacoes/999/renomear/previa", 404, ""},
		{"/api/transacoes/abc/renomear/previa", 404, ""},
	}
	for _, c := range casos {
		resp := testeapoio.Chamar(t, mux, "GET", c.caminho, "")
		resp.ConferirStatus(t, c.status)
		if c.erro != "" {
			var corpo struct{ Errors json.RawMessage }
			if err := json.Unmarshal(resp.Corpo, &corpo); err != nil {
				t.Fatal(err)
			}
			testeapoio.ConferirJSON(t, corpo.Errors, c.erro)
		}
	}
}

// Regra: renomear só nesta (escolhido à mão, sem regra) e o Desfazer, que volta tudo e vale uma vez só; voltar ao nome
// do banco; o mesmo nome conta, porque muda o "escolhido à mão".
func TestRenomearSoNesta(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	regra := regraDescricao(t, db, "PIX ENVIADO", 1, "Pix")
	pix := fab.Transacao(conta, doBanco("PIX ENVIADO JOAO"), func(x *testeapoio.Transacao) { x.Descricao, x.RegraDescricaoId = "Pix", &regra })

	resp := testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/transacoes/%d/descricao", pix.Id), `{"descricao":"  Aluguel do João "}`)
	resp.ConferirStatus(t, http.StatusOK)
	r := lerResultado(t, resp)
	if r.Alteradas != 1 || r.RegraId != nil || r.Desfazer == nil {
		t.Fatalf("%s", resp.Corpo)
	}
	if d, m, g := coluna(t, db, pix.Id, "Descricao"), coluna(t, db, pix.Id, "DescricaoManual"), coluna(t, db, pix.Id, "RegraDescricaoId"); d != "Aluguel do João" || m != int64(1) || g != nil {
		t.Errorf("renomeada: %v %v %v", d, m, g)
	}
	resp = testeapoio.Chamar(t, mux, "POST", "/api/transacoes/descricao/desfazer/"+*r.Desfazer, "")
	resp.ConferirStatus(t, http.StatusOK)
	resp.ConferirJSON(t, `{"transacoesAtualizadas":1}`)
	if d, m, g := coluna(t, db, pix.Id, "Descricao"), coluna(t, db, pix.Id, "DescricaoManual"), coluna(t, db, pix.Id, "RegraDescricaoId"); d != "Pix" || m != int64(0) || g != regra {
		t.Errorf("desfeita: %v %v %v", d, m, g)
	}
	resp = testeapoio.Chamar(t, mux, "POST", "/api/transacoes/descricao/desfazer/"+*r.Desfazer, "")
	resp.ConferirStatus(t, http.StatusNotFound)
	resp.ConferirJSON(t, `{"mensagem":"Não dá mais para desfazer: o prazo passou ou já foi desfeito."}`)

	resp = testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/transacoes/%d/descricao", pix.Id), `{"voltarOriginal":true}`)
	resp.ConferirStatus(t, http.StatusOK)
	if d, m := coluna(t, db, pix.Id, "Descricao"), coluna(t, db, pix.Id, "DescricaoManual"); d != "PIX ENVIADO JOAO" || m != int64(1) {
		t.Errorf("de volta ao do banco: %v %v", d, m)
	}
	resp = testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/transacoes/%d/descricao", pix.Id), `{"descricao":"PIX ENVIADO JOAO"}`)
	if r := lerResultado(t, resp); r.Alteradas != 0 || r.Desfazer != nil {
		t.Errorf("nada mudou: %s", resp.Corpo)
	}
}

// Regra: renomear criando a regra: sem texto, "é igual a" com a descrição do banco inteira; vale nas outras de nome
// automático (e não nas escolhidas à mão); o Desfazer apaga a regra e volta os nomes.
func TestRenomearCriandoRegra(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	uma := fab.Transacao(conta, doBanco("RENDIMENTO"))
	outra := fab.Transacao(conta, doBanco("RENDIMENTO"))
	aMao := fab.Transacao(conta, doBanco("RENDIMENTO"), func(x *testeapoio.Transacao) { x.Descricao, x.DescricaoManual = "Meu", true })

	resp := testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/transacoes/%d/descricao", uma.Id),
		`{"descricao":"Rendimento da poupança","aplicarNasOutras":true}`)
	resp.ConferirStatus(t, http.StatusOK)
	r := lerResultado(t, resp)
	if r.Alteradas != 2 || r.RegraId == nil || r.Desfazer == nil {
		t.Fatalf("%s", resp.Corpo)
	}
	for _, id := range []int64{uma.Id, outra.Id} {
		if d, g := coluna(t, db, id, "Descricao"), coluna(t, db, id, "RegraDescricaoId"); d != "Rendimento da poupança" || g != *r.RegraId {
			t.Errorf("%d: %v %v", id, d, g)
		}
	}
	if d := coluna(t, db, aMao.Id, "Descricao"); d != "Meu" {
		t.Errorf("a escolhida à mão mudou: %v", d)
	}
	var tipo int
	var texto string
	if err := db.QueryRow(`SELECT TipoCorrespondencia, Texto FROM RegraDescricao WHERE Id = ?`, *r.RegraId).Scan(&tipo, &texto); err != nil || tipo != 3 || texto != "RENDIMENTO" {
		t.Errorf("a regra: %d %q %v", tipo, texto, err)
	}
	testeapoio.Chamar(t, mux, "POST", "/api/transacoes/descricao/desfazer/"+*r.Desfazer, "").ConferirStatus(t, http.StatusOK)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM RegraDescricao`).Scan(&n); err != nil || n != 0 {
		t.Errorf("a regra ficou: %d %v", n, err)
	}
	if d, g := coluna(t, db, outra.Id, "Descricao"), coluna(t, db, outra.Id, "RegraDescricaoId"); d != "RENDIMENTO" || g != nil {
		t.Errorf("a outra não voltou: %v %v", d, g)
	}
}

// Regra: as recusas do renomear: sem a descrição, o lançamento manual virando regra, o tipo fora do enum, o texto
// curto, o texto que não bate e a regra mais específica (nos dois últimos, a regra criada não fica).
func TestRenomearComErro(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	luz := fab.Transacao(conta, doBanco("DEBITO AUTOMATICO ENERGIA"))
	regraDescricao(t, db, "DEBITO AUTOMATICO ENERGIA", 1, "Conta de luz")
	rendimento := fab.Transacao(conta, doBanco("RENDIMENTO"))
	manual := fab.Transacao(conta)
	casos := []struct {
		nome     string
		id       int64
		corpo    string
		mensagem string
	}{
		{"sem a descrição", rendimento.Id, `{"descricao":"  "}`, "Escreva a nova descrição."},
		{"manual virando regra", manual.Id, `{"descricao":"X","aplicarNasOutras":true}`,
			"Só transações que vieram do banco podem virar regra: esta foi lançada à mão."},
		{"tipo fora do enum", rendimento.Id, `{"descricao":"X","tipoCorrespondencia":9}`, "Tipo de correspondência inválido."},
		{"texto curto", rendimento.Id, `{"descricao":"X","aplicarNasOutras":true,"texto":"1 -"}`,
			"O texto da regra precisa de pelo menos 2 letras ou números."},
		{"texto que não bate", rendimento.Id, `{"descricao":"X","aplicarNasOutras":true,"tipoCorrespondencia":1,"texto":"XYZW"}`,
			`A descrição do banco desta transação ("RENDIMENTO") não bate com "XYZW".`},
		{"regra mais específica", luz.Id, `{"descricao":"Luz","aplicarNasOutras":true,"tipoCorrespondencia":1,"texto":"ENERGIA"}`,
			`Já existe uma regra mais específica para esta descrição ("DEBITO AUTOMATICO ENERGIA"). Edite-a em Categorias › Regras automáticas.`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			resp := testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/transacoes/%d/descricao", c.id), c.corpo)
			resp.ConferirStatus(t, http.StatusBadRequest)
			resp.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, c.mensagem))
		})
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM RegraDescricao`).Scan(&n); err != nil || n != 1 {
		t.Errorf("ficou regra criada: %d %v", n, err)
	}
	testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/999/descricao", `{"descricao":"X"}`).ConferirStatus(t, http.StatusNotFound)
	testeapoio.Chamar(t, mux, "PUT", "/api/transacoes/abc/descricao", `{"descricao":"X"}`).ConferirStatus(t, http.StatusNotFound)
}

// Regra: renomear em lote: só nelas (à mão) ou com uma regra "é igual a" por descrição do banco; a que já existe passa
// a usar o nome novo (e o Desfazer volta o nome dela); o lançamento manual fica à mão; a inexistente é 404.
func TestRenomearEmLote(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	existente := regraDescricao(t, db, "MERCADO", 3, "Mercado")
	salario := fab.Transacao(conta, doBanco("SALARIO"))
	outroSalario := fab.Transacao(conta, doBanco("SALARIO"))
	mercado := fab.Transacao(conta, doBanco("MERCADO"), func(x *testeapoio.Transacao) { x.Descricao, x.RegraDescricaoId = "Mercado", &existente })
	manual := fab.Transacao(conta)

	resp := testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/descricao/em-lote",
		fmt.Sprintf(`{"ids":[%d,%d,%d],"descricao":" Só nelas ","criarRegras":false}`, salario.Id, manual.Id, salario.Id))
	resp.ConferirStatus(t, http.StatusOK)
	r := lerResultado(t, resp)
	if r.Alteradas != 2 || r.RegrasCriadas != 0 || r.Desfazer == nil {
		t.Fatalf("%s", resp.Corpo)
	}
	testeapoio.Chamar(t, mux, "POST", "/api/transacoes/descricao/desfazer/"+*r.Desfazer, "").ConferirJSON(t, `{"transacoesAtualizadas":2}`)

	resp = testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/descricao/em-lote",
		fmt.Sprintf(`{"ids":[%d,%d,%d],"descricao":"Novo","criarRegras":true}`, salario.Id, mercado.Id, manual.Id))
	resp.ConferirStatus(t, http.StatusOK)
	r = lerResultado(t, resp)
	if r.Alteradas != 4 || r.RegrasCriadas != 1 || r.RegrasAlteradas != 1 || r.Desfazer == nil {
		t.Fatalf("%s", resp.Corpo)
	}
	for _, id := range []int64{salario.Id, outroSalario.Id, mercado.Id, manual.Id} {
		if d := coluna(t, db, id, "Descricao"); d != "Novo" {
			t.Errorf("%d: %v", id, d)
		}
	}
	if m := coluna(t, db, manual.Id, "DescricaoManual"); m != int64(1) {
		t.Errorf("o manual não ficou à mão: %v", m)
	}
	testeapoio.Chamar(t, mux, "POST", "/api/transacoes/descricao/desfazer/"+*r.Desfazer, "").ConferirStatus(t, http.StatusOK)
	var nome string
	if err := db.QueryRow(`SELECT NovaDescricao FROM RegraDescricao WHERE Id = ?`, existente).Scan(&nome); err != nil || nome != "Mercado" {
		t.Errorf("a regra que já existia: %q %v", nome, err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM RegraDescricao`).Scan(&n); err != nil || n != 1 {
		t.Errorf("a regra criada ficou: %d %v", n, err)
	}

	resp = testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/descricao/em-lote", fmt.Sprintf(`{"ids":[%d,999],"descricao":"X"}`, salario.Id))
	resp.ConferirStatus(t, http.StatusNotFound)
	resp.ConferirJSON(t, `{"mensagem":"Alguma transação não existe mais. Atualize a lista."}`)
}

// Regra: no máximo 50 regras de uma vez pelo renomear em lote.
func TestRenomearEmLoteComRegrasDemais(t *testing.T) {
	mux, _, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	ids := ""
	for i := range 51 {
		x := fab.Transacao(conta, doBanco(fmt.Sprintf("LOJA %d", i)))
		if i > 0 {
			ids += ","
		}
		ids += fmt.Sprint(x.Id)
	}
	resp := testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/descricao/em-lote", `{"ids":[`+ids+`],"descricao":"X","criarRegras":true}`)
	resp.ConferirStatus(t, http.StatusBadRequest)
	resp.ConferirJSON(t, `{"mensagem":"As selecionadas têm 51 descrições do banco diferentes; o limite é 50 regras de uma vez. Selecione menos, ou crie uma regra \"contém\" em Categorias › Regras automáticas."}`)
}
