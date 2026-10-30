package extrato

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

type resultadoCategorias struct {
	Alteradas int
	Desfazer  *string
}

// Regra: categorias em lote por transação: cada uma com a sua, à mão ou pelas regras; a que fica com a mesma categoria e
// o mesmo "à mão" não conta; o Desfazer volta categoria, "à mão", tipo e "fora dos relatórios", uma vez só.
func TestCategoriasEmLotePorTransacao(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	mercado := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome = "Mercado" })
	fab.RegraCategorizacao(mercado, func(r *testeapoio.RegraCategorizacao) { r.Texto = "FEIRA" })
	lazer := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome = "Lazer" })
	feira := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao = "FEIRA" })
	cinema := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao, x.CategoriaId, x.CategoriaManual = "CINEMA", &lazer.Id, true })

	resp := testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote", fmt.Sprintf(`{"itens":[
		{"id":%d,"categoriaId":%d,"automatica":false},{"id":%d,"categoriaId":%d}]}`, feira.Id, lazer.Id, cinema.Id, lazer.Id))
	resp.ConferirStatus(t, http.StatusOK)
	var r resultadoCategorias
	if err := json.Unmarshal(resp.Corpo, &r); err != nil || r.Alteradas != 1 || r.Desfazer == nil {
		t.Fatalf("%s %v", resp.Corpo, err)
	}
	if c, m := coluna(t, db, feira.Id, "CategoriaId"), coluna(t, db, feira.Id, "CategoriaManual"); c != lazer.Id || m != int64(1) {
		t.Errorf("feira: %v %v", c, m)
	}
	resp = testeapoio.Chamar(t, mux, "POST", "/api/transacoes/em-lote/desfazer/"+*r.Desfazer, "")
	resp.ConferirJSON(t, `{"transacoesAtualizadas":1}`)
	if c, m := coluna(t, db, feira.Id, "CategoriaId"), coluna(t, db, feira.Id, "CategoriaManual"); c != nil || m != int64(0) {
		t.Errorf("desfeita: %v %v", c, m)
	}
	testeapoio.Chamar(t, mux, "POST", "/api/transacoes/em-lote/desfazer/"+*r.Desfazer, "").ConferirStatus(t, http.StatusNotFound)

	resp = testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote", fmt.Sprintf(`{"itens":[{"id":%d,"automatica":true}]}`, feira.Id))
	if err := json.Unmarshal(resp.Corpo, &r); err != nil || r.Alteradas != 1 {
		t.Fatalf("automática: %s", resp.Corpo)
	}
	if c := coluna(t, db, feira.Id, "CategoriaId"); c != mercado.Id {
		t.Errorf("automática: %v", c)
	}
}

// Regra: pelo filtro (os mesmos campos da busca, no corpo): todas as que ele acha; sem nenhuma, sem chave. O Desfazer
// não passa por cima da que mudou de categoria depois, e a categoria apagada volta como nenhuma.
func TestCategoriasEmLotePeloFiltro(t *testing.T) {
	mux, db, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	lazer := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome = "Lazer" })
	apagada := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome = "Apagada" })
	um := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao, x.CategoriaId = "STREAMFLIX", &apagada.Id })
	dois := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao = "STREAMFLIX" })
	fora := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.Descricao = "OUTRA" })

	resp := testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote", fmt.Sprintf(`{"filtro":{"busca":"streamflix"},
		"categoriaId":%d}`, lazer.Id))
	var r resultadoCategorias
	if err := json.Unmarshal(resp.Corpo, &r); err != nil || r.Alteradas != 2 || r.Desfazer == nil {
		t.Fatalf("%s %v", resp.Corpo, err)
	}
	if c := coluna(t, db, fora.Id, "CategoriaId"); c != nil {
		t.Errorf("a de fora do filtro mudou: %v", c)
	}
	if _, err := db.Exec(`UPDATE Transacao SET CategoriaId = NULL WHERE Id = ?`, dois.Id); err != nil { // mudou depois
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = OFF; DELETE FROM Categoria WHERE Id = ?`, apagada.Id); err != nil {
		t.Fatal(err)
	}
	resp = testeapoio.Chamar(t, mux, "POST", "/api/transacoes/em-lote/desfazer/"+*r.Desfazer, "")
	resp.ConferirJSON(t, `{"transacoesAtualizadas":1}`)
	if c, m := coluna(t, db, um.Id, "CategoriaId"), coluna(t, db, um.Id, "CategoriaManual"); c != nil || m != int64(0) {
		t.Errorf("a da categoria apagada: %v %v", c, m)
	}

	resp = testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote", `{"filtro":{"busca":"nada assim"},"automatica":true}`)
	resp.ConferirJSON(t, `{"alteradas":0,"desfazer":null}`)
	resp = testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote", fmt.Sprintf(`{"filtro":{"inicio":"2026-10-01",
		"fim":"2026-10-31","contaId":%d,"categoriaIds":[%d],"tipo":1,"naturezas":[],"ids":[%d],"pagina":2},"automatica":true}`,
		conta.Id, lazer.Id, um.Id))
	resp.ConferirStatus(t, http.StatusOK)
}

// Regra: as recusas: itens e filtro juntos ou nenhum dos dois (a lista vazia conta como nenhum), sem categoria e sem
// automática, categoria inexistente, transação repetida, inexistente (404), o filtro inválido (com "Filtro." na
// frente), o item que não é número (cita o record do item), o item nulo (o 500 do C#).
func TestCategoriasEmLoteComErro(t *testing.T) {
	mux, _, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta()
	x := fab.Transacao(conta)
	casos := []struct {
		nome, corpo string
		status      int
		trecho      string
	}{
		{"itens e filtro", fmt.Sprintf(`{"itens":[{"id":%d,"automatica":true}],"filtro":{},"automatica":true}`, x.Id), 400,
			"Informe as transações ou o filtro (um dos dois)."},
		{"nenhum dos dois", `{"categoriaId":1}`, 400, "Informe as transações ou o filtro (um dos dois)."},
		{"lista vazia", `{"itens":[],"categoriaId":1}`, 400, "Informe as transações ou o filtro (um dos dois)."},
		{"sem categoria", fmt.Sprintf(`{"itens":[{"id":%d}]}`, x.Id), 400, "Escolha uma categoria ou a categorização automática."},
		{"categoria inexistente", fmt.Sprintf(`{"itens":[{"id":%d,"categoriaId":999}]}`, x.Id), 400, "Categoria inválida."},
		{"repetida", fmt.Sprintf(`{"itens":[{"id":%d,"automatica":true},{"id":%d,"automatica":true}]}`, x.Id, x.Id), 400,
			"Transação repetida na lista."},
		{"inexistente", `{"itens":[{"id":999,"automatica":true}]}`, 404, "Alguma transação não existe mais. Atualize a lista."},
		{"filtro inválido", `{"filtro":{"tamanhoPagina":0,"busca":"` + strings.Repeat("x", 101) + `","naturezas":[1,2,1]},"automatica":true}`,
			400, `"Filtro.TamanhoPagina":["The field TamanhoPagina must be between 1 and 500."]`},
		{"item que não é número", `{"itens":[{"id":"x","automatica":true}]}`, 400,
			`could not be converted to Financas.Api.Dtos.AlteracaoCategoriaDto. Path: $.itens[0].id`},
		{"item que não é objeto", `{"itens":[5]}`, 400, `could not be converted to Financas.Api.Dtos.AlteracaoCategoriaDto. Path: $.itens[0]`},
		{"itens que não são lista", `{"itens":5}`, 400,
			"could not be converted to System.Collections.Generic.List`1[Financas.Api.Dtos.AlteracaoCategoriaDto]. Path: $.itens"},
		{"categoria que não é número", `{"itens":[],"categoriaId":"x"}`, 400,
			"could not be converted to System.Nullable`1[System.Int32]. Path: $.categoriaId"},
		{"filtro que não é objeto", `{"filtro":5}`, 400, "could not be converted to Financas.Api.Dtos.FiltroTransacoesDto. Path: $.filtro"},
		{"campo do filtro", `{"filtro":{"tipo":"Receita"}}`, 400,
			"could not be converted to System.Nullable`1[Financas.Api.Entidades.TipoTransacao]. Path: $.filtro.tipo"},
		{"data do filtro", `{"filtro":{"inicio":5}}`, 400,
			"could not be converted to System.Nullable`1[System.DateOnly]. Path: $.filtro.inicio"},
		{"item nulo", `{"itens":[null]}`, 500, "Ocorreu um erro interno"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			resp := testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote", c.corpo)
			resp.ConferirStatus(t, c.status)
			if !strings.Contains(string(resp.Corpo), c.trecho) {
				t.Errorf("sem %q em %s", c.trecho, resp.Corpo)
			}
		})
	}
	resp := testeapoio.Chamar(t, mux, "PATCH", "/api/transacoes/em-lote",
		`{"filtro":{"tamanhoPagina":0,"busca":"`+strings.Repeat("x", 101)+`","naturezas":[1,2,1]},"automatica":true}`)
	for _, chave := range []string{`"Filtro.Busca"`, `"Filtro.Naturezas"`} {
		if !strings.Contains(string(resp.Corpo), chave) {
			t.Errorf("sem %s em %s", chave, resp.Corpo)
		}
	}
	testeapoio.Chamar(t, mux, "POST", "/api/transacoes/em-lote/desfazer/naoexiste", "").ConferirStatus(t, http.StatusNotFound)
}
