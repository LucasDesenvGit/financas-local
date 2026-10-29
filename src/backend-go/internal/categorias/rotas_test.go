package categorias

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

func p[T any](v T) *T { return &v }

// Regra: o CategoriasController: criar (aparado, natureza só em despesa), listar por tipo e nome, as mensagens do
// ValidarAsync, editar, e excluir (transações sem categoria e regra em cascata).
func TestCategoriasPelaRota(t *testing.T) {
	mux, db, fab := rotasDeTeste(t)
	lazer := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Lazer" })
	sub := fab.Subcategoria(lazer, func(c *testeapoio.Categoria) { c.Nome = "Shows" })
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, mux, metodo, caminho, corpo)
	}

	cinema := chamar("POST", "/api/categorias", fmt.Sprintf(`{"nome":"  Cinema ","tipo":1,"cor":"#123ABC","icone":" filme ","categoriaPaiId":%d,"natureza":2}`, lazer.Id))
	cinema.ConferirJSON(t, fmt.Sprintf(`{"id":3,"nome":"Cinema","tipo":1,"cor":"#123ABC","icone":"filme","categoriaPaiId":%d,"ativa":true,"natureza":2}`, lazer.Id))
	chamar("POST", "/api/categorias", `{"nome":"Bicos","tipo":2,"cor":"#0F766E","icone":"  ","ativa":false,"natureza":1}`).
		ConferirJSON(t, `{"id":4,"nome":"Bicos","tipo":2,"cor":"#0F766E","icone":null,"categoriaPaiId":null,"ativa":false,"natureza":null}`)
	chamar("GET", "/api/categorias", "").ConferirJSON(t, fmt.Sprintf(`[
		{"id":3,"nome":"Cinema","tipo":1,"cor":"#123ABC","icone":"filme","categoriaPaiId":%[1]d,"ativa":true,"natureza":2},
		{"id":1,"nome":"Lazer","tipo":1,"cor":"#64748b","icone":null,"categoriaPaiId":null,"ativa":true,"natureza":null},
		{"id":2,"nome":"Shows","tipo":1,"cor":"#64748b","icone":null,"categoriaPaiId":%[1]d,"ativa":true,"natureza":null},
		{"id":4,"nome":"Bicos","tipo":2,"cor":"#0F766E","icone":null,"categoriaPaiId":null,"ativa":false,"natureza":null}]`, lazer.Id))

	mensagens := []struct{ metodo, caminho, corpo, mensagem string }{
		{"POST", "/api/categorias", `{"nome":" Lazer ","tipo":1,"cor":"#64748b"}`, "Já existe uma categoria com esse nome."},
		{"POST", "/api/categorias", `{"nome":"X","tipo":1,"cor":"#64748b","categoriaPaiId":999}`, "Categoria pai inválida."},
		{"POST", "/api/categorias", fmt.Sprintf(`{"nome":"X","tipo":1,"cor":"#64748b","categoriaPaiId":%d}`, sub.Id), "Só é permitido um nível de subcategoria."},
		{"POST", "/api/categorias", fmt.Sprintf(`{"nome":"X","tipo":2,"cor":"#64748b","categoriaPaiId":%d}`, lazer.Id), "A subcategoria deve ter o mesmo tipo da categoria pai."},
		{"PUT", "/api/categorias/3", `{"nome":"Cinema","tipo":1,"cor":"#64748b","categoriaPaiId":3}`, "Uma categoria não pode ser pai dela mesma."},
		{"PUT", fmt.Sprintf("/api/categorias/%d", lazer.Id), `{"nome":"Lazer","tipo":1,"cor":"#64748b","categoriaPaiId":4}`, "A subcategoria deve ter o mesmo tipo da categoria pai."},
		{"PUT", "/api/categorias/4", fmt.Sprintf(`{"nome":"Bicos","tipo":1,"cor":"#64748b","categoriaPaiId":%d}`, lazer.Id), "Esta categoria possui subcategorias e não pode virar subcategoria."},
		{"PUT", "/api/categorias/3", `{"nome":"Lazer","tipo":1,"cor":"#64748b"}`, "Já existe uma categoria com esse nome."},
		{"DELETE", fmt.Sprintf("/api/categorias/%d", lazer.Id), "", "Exclua ou mova as subcategorias antes."},
	}
	// "Bicos" não tem filhas: a mensagem das filhas é da "Lazer"; ajusta a linha que confere isso.
	mensagens[6] = struct{ metodo, caminho, corpo, mensagem string }{"PUT", fmt.Sprintf("/api/categorias/%d", lazer.Id),
		`{"nome":"Lazer","tipo":1,"cor":"#64748b","categoriaPaiId":3}`, "Só é permitido um nível de subcategoria."}
	for _, m := range mensagens {
		resp := chamar(m.metodo, m.caminho, m.corpo)
		resp.ConferirStatus(t, http.StatusBadRequest)
		resp.ConferirJSON(t, fmt.Sprintf(`{"mensagem":%q}`, m.mensagem))
	}
	chamar("PUT", "/api/categorias/999", `{"nome":"X","tipo":1,"cor":"#64748b"}`).ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	chamar("PUT", "/api/categorias/3", `{"nome":"Cinema e teatro","tipo":1,"cor":"#abcdef","ativa":false,"natureza":1}`).
		ConferirJSON(t, `{"id":3,"nome":"Cinema e teatro","tipo":1,"cor":"#abcdef","icone":null,"categoriaPaiId":null,"ativa":false,"natureza":1}`)

	// Excluir: a transação fica sem categoria (e deixa de ser manual); a regra sai em cascata.
	conta := fab.Conta()
	tr := fab.Transacao(conta, func(x *testeapoio.Transacao) { x.CategoriaId, x.CategoriaManual = p(int64(3)), true })
	fab.RegraCategorizacao(testeapoio.Categoria{Id: 3})
	chamar("DELETE", "/api/categorias/999", "").ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	chamar("DELETE", "/api/categorias/3", "").ConferirStatus(t, http.StatusNoContent)
	var categoria *int64
	var manual bool
	var regras int
	if err := db.QueryRow(`SELECT CategoriaId, CategoriaManual FROM Transacao WHERE Id = ?`, tr.Id).Scan(&categoria, &manual); err != nil || categoria != nil || manual {
		t.Errorf("a transação ficou com categoria %v, manual %v (%v)", categoria, manual, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM RegraCategorizacao WHERE CategoriaId = 3`).Scan(&regras); err != nil || regras != 0 {
		t.Errorf("sobraram %d regras da categoria apagada (%v)", regras, err)
	}
}

// Regra: o RegrasController: listar por prioridade (com o nome da categoria), criar, editar, excluir e o reaplicar
// (recategoriza as de categoria automática, conta as que mudaram, e uma segunda vez não muda nada).
func TestRegrasPelaRota(t *testing.T) {
	mux, db, fab := rotasDeTeste(t)
	mercado := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Mercado" })
	lazer := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Lazer" })
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, mux, metodo, caminho, corpo)
	}

	chamar("POST", "/api/regras", fmt.Sprintf(`{"texto":" supermercado ","tipoCorrespondencia":1,"categoriaId":%d,"prioridade":"80"}`, mercado.Id)).
		ConferirJSON(t, `{"id":1,"texto":"supermercado","tipoCorrespondencia":1,"categoriaId":1,"categoriaNome":"Mercado","prioridade":80,"ativa":true}`)
	chamar("POST", "/api/regras", fmt.Sprintf(`{"texto":"cinema","tipoCorrespondencia":2,"categoriaId":%d,"prioridade":10,"ativa":false}`, lazer.Id)).ConferirStatus(t, http.StatusOK)
	chamar("POST", "/api/regras", `{"texto":"x y","tipoCorrespondencia":1,"categoriaId":999,"prioridade":1}`).ConferirJSON(t, `{"mensagem":"Categoria inválida."}`)
	chamar("GET", "/api/regras", "").ConferirJSON(t, `[
		{"id":2,"texto":"cinema","tipoCorrespondencia":2,"categoriaId":2,"categoriaNome":"Lazer","prioridade":10,"ativa":false},
		{"id":1,"texto":"supermercado","tipoCorrespondencia":1,"categoriaId":1,"categoriaNome":"Mercado","prioridade":80,"ativa":true}]`)
	chamar("PUT", "/api/regras/2", `{"texto":"CINEMA","tipoCorrespondencia":1,"categoriaId":2,"prioridade":5}`).
		ConferirJSON(t, `{"id":2,"texto":"CINEMA","tipoCorrespondencia":1,"categoriaId":2,"categoriaNome":"Lazer","prioridade":5,"ativa":true}`)
	chamar("PUT", "/api/regras/999", `{"texto":"nada","tipoCorrespondencia":1,"categoriaId":2,"prioridade":1}`).ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	chamar("PUT", "/api/regras/2", `{"texto":"nada","tipoCorrespondencia":1,"categoriaId":999,"prioridade":1}`).ConferirJSON(t, `{"mensagem":"Categoria inválida."}`)

	conta := fab.Conta()
	nova := func(descricao string, categoria *int64, manual bool) int64 {
		return fab.Transacao(conta, func(x *testeapoio.Transacao) {
			x.Descricao, x.DescricaoOriginal, x.CategoriaId, x.CategoriaManual = descricao, p(descricao), categoria, manual
		}).Id
	}
	nova("SUPERMERCADO JA CERTO", &mercado.Id, false) // a primeira do lote não muda: as seguintes continuam
	semRegra := nova("SUPERMERCADO EXEMPLO", nil, false)
	cinema := nova("CINEMA CENTRO", &mercado.Id, false)
	manual := nova("CINEMA MANUAL", &mercado.Id, true)
	solta := nova("LOJA", &lazer.Id, false)
	loteDoReaplicar = 2 // vários lotes, como com mais de 2000 transações
	t.Cleanup(func() { loteDoReaplicar = 2000 })
	chamar("POST", "/api/regras/reaplicar", "").ConferirJSON(t, `{"transacoesAtualizadas":3}`)
	chamar("POST", "/api/regras/reaplicar", "").ConferirJSON(t, `{"transacoesAtualizadas":0}`)
	categoriaDe := func(id int64) *int64 {
		var c *int64
		if err := db.QueryRow(`SELECT CategoriaId FROM Transacao WHERE Id = ?`, id).Scan(&c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	for _, c := range []struct {
		nome     string
		id       int64
		esperado *int64
	}{{"sem regra ganha a da regra", semRegra, &mercado.Id}, {"regra de prioridade menor vence", cinema, &lazer.Id},
		{"manual fica", manual, &mercado.Id}, {"sem regra nem Pluggy fica sem", solta, nil}} {
		if got := categoriaDe(c.id); !mesmoId(got, c.esperado) {
			t.Errorf("%s: categoria %v", c.nome, got)
		}
	}
	chamar("DELETE", "/api/regras/999", "").ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	chamar("DELETE", "/api/regras/2", "").ConferirStatus(t, http.StatusNoContent)
}

// Regra: o RegrasDescricaoController: criar renomeia e recategoriza, a lista conta as transações, a prévia, o
// Invalida, editar (404 depois dos atributos) e excluir (as transações voltam ao nome do banco).
func TestRegrasDescricaoPelaRota(t *testing.T) {
	mux, db, fab := rotasDeTeste(t)
	moradia := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Moradia" })
	fab.RegraCategorizacao(moradia, func(r *testeapoio.RegraCategorizacao) { r.Texto = "ALUGUEL" })
	conta := fab.Conta()
	manual := fab.Transacao(conta, func(x *testeapoio.Transacao) { // categoria manual, primeira da lista: fica
		x.Descricao, x.DescricaoOriginal, x.CategoriaManual, x.Valor = "PIX IMOBILIARIA", p("PIX IMOBILIARIA"), true, tipos.Decimal{}
		x.Valor, _ = tipos.NovoDecimal("-1850")
	})
	// Uma TED na genérica, com o par na poupança e ainda como despesa: o reclassificar das transferências, que roda
	// depois de a regra mudar uma categoria, a acerta.
	generica := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome, c.Tipo = "Transferências", 3 })
	ted := fab.Transacao(conta, func(x *testeapoio.Transacao) {
		x.CategoriaId, x.Descricao, x.Tipo = &generica.Id, "TED", transacoes.Despesa
	})
	fab.Transacao(fab.Conta(), func(x *testeapoio.Transacao) { x.Valor, _ = tipos.NovoDecimal("10.00") })
	var ids []int64
	for _, valor := range []string{"-1850", "-1850", "-90"} {
		ids = append(ids, fab.Transacao(conta, func(x *testeapoio.Transacao) {
			x.Descricao, x.DescricaoOriginal = "PIX IMOBILIARIA", p("PIX IMOBILIARIA")
			x.Valor, _ = tipos.NovoDecimal(valor)
		}).Id)
	}
	chamar := func(metodo, caminho, corpo string) testeapoio.Resposta {
		return testeapoio.Chamar(t, mux, metodo, caminho, corpo)
	}

	chamar("GET", "/api/regras-descricao/previa?texto=imobiliaria&tipo=1&valorMinimo=1000&valorMaximo=", "").ConferirJSON(t, `{"quantidade":3,"exemplos":["PIX IMOBILIARIA"]}`)
	chamar("POST", "/api/regras-descricao", `{"texto":"PIX IMOBILIARIA","tipoCorrespondencia":3,"novaDescricao":" Aluguel ","valorMinimo":1800,"valorMaximo":"1900.00"}`).
		ConferirJSON(t, `{"id":1,"texto":"PIX IMOBILIARIA","tipoCorrespondencia":3,"novaDescricao":"Aluguel","ativa":true,"transacoes":3,"valorMinimo":1800,"valorMaximo":1900.00}`)
	var categoriaManual *int64
	var tipoTed int64
	if err := db.QueryRow(`SELECT CategoriaId FROM Transacao WHERE Id = ?`, manual.Id).Scan(&categoriaManual); err != nil || categoriaManual != nil {
		t.Errorf("a de categoria manual foi recategorizada: %v %v", categoriaManual, err)
	}
	if err := db.QueryRow(`SELECT Tipo FROM Transacao WHERE Id = ?`, ted.Id).Scan(&tipoTed); err != nil || tipoTed != transacoes.Transferencia {
		t.Errorf("a TED com par não foi reclassificada como transferência: %d %v", tipoTed, err)
	}
	var descricao string
	var categoria *int64
	var tipo int64
	if err := db.QueryRow(`SELECT Descricao, CategoriaId, Tipo FROM Transacao WHERE Id = ?`, ids[0]).Scan(&descricao, &categoria, &tipo); err != nil ||
		descricao != "Aluguel" || categoria == nil || *categoria != moradia.Id || tipo != transacoes.Despesa {
		t.Errorf("renomeada e recategorizada pela regra ALUGUEL: %q %v %d %v", descricao, categoria, tipo, err)
	}
	// A lista lê do banco, no formato do EF Core (1800.0); a resposta do criar devolve o valor como veio, como o C#.
	chamar("GET", "/api/regras-descricao", "").ConferirJSON(t,
		`[{"id":1,"texto":"PIX IMOBILIARIA","tipoCorrespondencia":3,"novaDescricao":"Aluguel","ativa":true,"transacoes":3,"valorMinimo":1800.0,"valorMaximo":1900.0}]`)

	chamar("POST", "/api/regras-descricao", `{"texto":"xx","tipoCorrespondencia":0,"novaDescricao":"Y"}`).ConferirJSON(t, `{"mensagem":"Tipo de correspondência inválido."}`)
	chamar("POST", "/api/regras-descricao", `{"texto":"xx","tipoCorrespondencia":1,"novaDescricao":"Y","valorMinimo":2,"valorMaximo":1}`).
		ConferirJSON(t, `{"mensagem":"O valor \"de\" é maior que o valor \"até\": troque os dois."}`)
	chamar("PUT", "/api/regras-descricao/999", `{"texto":"nada","tipoCorrespondencia":1,"novaDescricao":"Nada"}`).ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	chamar("PUT", "/api/regras-descricao/999", `{"texto":"x","tipoCorrespondencia":1,"novaDescricao":"Nada"}`).ConferirStatus(t, http.StatusBadRequest)
	chamar("PUT", "/api/regras-descricao/1", `{"texto":"PIX IMOBILIARIA","tipoCorrespondencia":3,"novaDescricao":"Aluguel apto"}`).
		ConferirJSON(t, `{"id":1,"texto":"PIX IMOBILIARIA","tipoCorrespondencia":3,"novaDescricao":"Aluguel apto","ativa":true,"transacoes":4,"valorMinimo":null,"valorMaximo":null}`)

	chamar("DELETE", "/api/regras-descricao/999", "").ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	chamar("DELETE", "/api/regras-descricao/1", "").ConferirStatus(t, http.StatusNoContent)
	var regra *int64
	if err := db.QueryRow(`SELECT Descricao, RegraDescricaoId, CategoriaId FROM Transacao WHERE Id = ?`, ids[2]).Scan(&descricao, &regra, &categoria); err != nil ||
		descricao != "PIX IMOBILIARIA" || regra != nil || categoria != nil {
		t.Errorf("de volta ao nome do banco e sem a categoria da regra ALUGUEL: %q %v %v %v", descricao, regra, categoria, err)
	}
	chamar("GET", "/api/regras-descricao", "").ConferirJSON(t, `[]`)
}
