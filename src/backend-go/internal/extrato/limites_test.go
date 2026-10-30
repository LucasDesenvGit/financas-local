package extrato

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// eventos são os eventos da auditoria, na ordem em que entraram.
func eventos(t *testing.T, db *sql.DB) []string {
	t.Helper()
	linhas, err := db.Query(`SELECT Evento FROM AuditoriaAcesso ORDER BY Id`)
	if err != nil {
		t.Fatal(err)
	}
	defer linhas.Close()
	var lista []string
	for linhas.Next() {
		var e string
		if err := linhas.Scan(&e); err != nil {
			t.Fatal(err)
		}
		lista = append(lista, e)
	}
	return lista
}

// lancarEmMassa grava n transações da conta num INSERT só, com a descrição dada.
func lancarEmMassa(t *testing.T, db *sql.DB, conta int64, n int, descricao string) {
	t.Helper()
	if _, err := db.Exec(`WITH RECURSIVE n(i) AS (SELECT 0 UNION ALL SELECT i + 1 FROM n WHERE i + 1 < ?)
		INSERT INTO "Transacao" ("ContaId", "Data", "Descricao", "Valor", "Tipo", "CriadoEm", "AtualizadoEm")
		SELECT ?, '2026-10-01', ?, '-1.0', 1, '2026-10-01 00:00:00', '2026-10-01 00:00:00' FROM n`, n, conta, descricao); err != nil {
		t.Fatal(err)
	}
}

// Regra: pelo filtro, 5000 exatas ainda vão (o limite é "mais de 5000").
func TestCategoriasEmLoteCom5000(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	categoria := fab.Categoria()
	lancarEmMassa(t, db, conta.Id, 5000, "LOJA")
	resp := testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote", fmt.Sprintf(`{"filtro":{"busca":"loja"},"categoriaId":%d}`, categoria.Id))
	resp.ConferirStatus(t, http.StatusOK)
	var r resultadoCategorias
	if err := json.Unmarshal(resp.Corpo, &r); err != nil || r.Alteradas != 5000 {
		t.Errorf("%s %v", resp.Corpo, err)
	}
}

// Regra: a escolha automática não pula as outras na conferência (a sem categoria e a inexistente depois dela); a que
// não muda não para a conta das seguintes; no Desfazer, a que mudou depois não para as seguintes.
func TestCategoriasEmLoteNaOrdem(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	lazer := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome = "Lazer" })
	jaNaLazer := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao, x.CategoriaId, x.CategoriaManual = "A", &lazer.Id, true })
	um := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao = "B" })
	dois := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao = "C" })
	tres := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao = "D" })

	recusas := []struct{ corpo, mensagem string }{
		{fmt.Sprintf(`{"itens":[{"id":%d,"automatica":true},{"id":%d}]}`, um.Id, dois.Id), "Escolha uma categoria ou a categorização automática."},
		{fmt.Sprintf(`{"itens":[{"id":%d,"automatica":true},{"id":%d,"categoriaId":999}]}`, um.Id, dois.Id), "Categoria inválida."},
	}
	for _, c := range recusas {
		resp := testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote", c.corpo)
		resp.ConferirStatus(t, http.StatusBadRequest)
		resp.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, c.mensagem))
	}

	resp := testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote", fmt.Sprintf(`{"itens":[{"id":%d,"categoriaId":%d},
		{"id":%d,"categoriaId":%d},{"id":%d,"categoriaId":%d},{"id":%d,"categoriaId":%d}]}`,
		jaNaLazer.Id, lazer.Id, um.Id, lazer.Id, dois.Id, lazer.Id, tres.Id, lazer.Id))
	var r resultadoCategorias
	if err := json.Unmarshal(resp.Corpo, &r); err != nil || r.Alteradas != 3 || r.Desfazer == nil {
		t.Fatalf("a que já estava na Lazer vem primeiro e não conta: %s", resp.Corpo)
	}
	if _, err := db.Exec(`UPDATE Transacao SET CategoriaId = NULL WHERE Id = ?`, dois.Id); err != nil {
		t.Fatal(err)
	}
	resp = testeapoio.Chamar(t, mux, "POST", "/api/transacoes/em-lote/desfazer/"+*r.Desfazer, "")
	resp.ConferirJSON(t, `{"transacoesAtualizadas":2}`)
	if c := coluna(t, db, tres.Id, "CategoriaId"); c != nil {
		t.Errorf("a depois da que mudou não voltou: %v", c)
	}
	if got := eventos(t, db); strings.Join(got, "|") != "CategoriaEmLote (3)|CategoriaEmLoteDesfeita (2)" {
		t.Errorf("auditoria: %v", got)
	}
}

// Regra: na reaplicação, a transação que não muda não para as seguintes; o texto da regra com 2 letras exatas vale, e
// a letra fora do plano básico não conta (o C# vê duas metades); a auditoria diz o que foi feito.
func TestRenomearNosLimites(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	fab.Transacao(conta, doBanco("OUTRA COISA")) // de nome automático, antes das outras: a regra nova não a pega
	uma := fab.Transacao(conta, doBanco("RENDIMENTO"))
	outra := fab.Transacao(conta, doBanco("RENDIMENTO"))

	resp := testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/transacoes/%d/descricao", uma.Id),
		`{"descricao":"Rendimento","aplicarNasOutras":true,"tipoCorrespondencia":1,"texto":"EN"}`)
	resp.ConferirStatus(t, http.StatusOK)
	if r := lerResultado(t, resp); r.Alteradas != 2 || r.RegraId == nil {
		t.Errorf("com 2 letras: %s", resp.Corpo)
	}
	if d := coluna(t, db, outra.Id, "Descricao"); d != "Rendimento" {
		t.Errorf("a outra, depois da que não muda: %v", d)
	}
	resp = testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/transacoes/%d/descricao", uma.Id),
		`{"descricao":"X","aplicarNasOutras":true,"texto":"A`+"\U00010000"+`"}`)
	resp.ConferirStatus(t, http.StatusBadRequest)
	resp.ConferirJSON(t, `{"mensagem":"O texto da regra precisa de pelo menos 2 letras ou números."}`)
	testeapoio.Chamar(t, mux, "PUT", fmt.Sprintf("/api/transacoes/%d/descricao", outra.Id), `{"descricao":"Só esta"}`).ConferirStatus(t, http.StatusOK)
	if got := eventos(t, db); strings.Join(got, "|") != "RegraDescricaoCriada (2)|TransacaoRenomeada" {
		t.Errorf("auditoria: %v", got)
	}

	resp = testeapoio.Chamar(t, mux, "GET", fmt.Sprintf("/api/transacoes/%d/renomear/previa?tipo=", uma.Id), "")
	resp.ConferirStatus(t, http.StatusBadRequest)
	if !strings.Contains(string(resp.Corpo), `"tipo":["The value '' is invalid."]`) {
		t.Errorf("tipo vazio: %s", resp.Corpo)
	}
}

// Regra: o renomear em lote: o lançamento manual antes das outras não impede as regras delas; 50 regras exatas ainda
// vão; sem criar regras, nada é reaplicado (a que estava fora do nome da regra fica como está); sem nada mudar, sem
// chave do Desfazer.
func TestRenomearEmLoteNosLimites(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	manual := fab.Transacao(conta)
	var ids []string
	ids = append(ids, fmt.Sprint(manual.Id))
	for i := range 50 {
		ids = append(ids, fmt.Sprint(fab.Transacao(conta, doBanco(fmt.Sprintf("LOJA %c%c", 'A'+i/26, 'A'+i%26))).Id))
	}
	resp := testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/descricao/em-lote",
		`{"ids":[`+strings.Join(ids, ",")+`],"descricao":"Loja","criarRegras":true}`)
	resp.ConferirStatus(t, http.StatusOK)
	if r := lerResultado(t, resp); r.RegrasCriadas != 50 || r.Alteradas != 51 {
		t.Errorf("50 regras: %s", resp.Corpo)
	}

	// Uma transação de nome automático que a regra ainda não renomeou (a regra veio depois, sem reaplicar).
	regraDescricao(t, db, "PADARIA", 1, "Padaria")
	padaria := fab.Transacao(conta, doBanco("PADARIA X"))
	resp = testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/descricao/em-lote",
		fmt.Sprintf(`{"ids":[%d],"descricao":"Avulsa","criarRegras":false}`, manual.Id))
	if r := lerResultado(t, resp); r.Alteradas != 1 {
		t.Errorf("só nelas: %s", resp.Corpo)
	}
	if d := coluna(t, db, padaria.Id, "Descricao"); d != "PADARIA X" {
		t.Errorf("sem criar regras, reaplicou: %v", d)
	}
	resp = testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/descricao/em-lote",
		fmt.Sprintf(`{"ids":[%d],"descricao":"Avulsa","criarRegras":false}`, manual.Id))
	resp.ConferirJSON(t, `{"alteradas":0,"regrasCriadas":0,"regrasAlteradas":0,"desfazer":null}`)
	if got := eventos(t, db); len(got) != 3 || got[0] != "RenomearEmLote (51, 50 regra)" || got[2] != "RenomearEmLote (0, 0 regra)" {
		t.Errorf("auditoria: %v", got)
	}
}
