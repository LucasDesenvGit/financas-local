package extrato

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func p[T any](v T) *T { return &v }

// minimoFixo é o parâmetro do mínimo de letras, sem o banco (a interface que o extrato declara).
type minimoFixo struct {
	n   int
	err error
}

func (m minimoFixo) ObterInteiro(context.Context, string) (int, error) { return m.n, m.err }

// O relógio dos testes: 15/10/2026, meio-dia em Brasília (15h em UTC).
func rotasDeTeste(t *testing.T, par parametros) (*http.ServeMux, *sql.DB, *testeapoio.Fabrica) {
	t.Helper()
	db := testeapoio.NovoBancoVazio(t)
	r := testeapoio.HojeEmBrasilia(2026, 10, 15)
	mux := http.NewServeMux()
	Registrar(mux, NovoServico(db, r, par, slog.New(slog.DiscardHandler)))
	return mux, db, testeapoio.NovaFabrica(t, db, r)
}

type pagina struct {
	Itens []struct {
		Id    int64
		Valor json.RawMessage
	}
	Total         int
	TotalReceitas json.RawMessage
	TotalDespesas json.RawMessage
}

func listar(t *testing.T, mux http.Handler, query string) pagina {
	t.Helper()
	resp := testeapoio.Chamar(t, mux, "GET", "/api/transacoes?"+query, "")
	resp.ConferirStatus(t, http.StatusOK)
	var pg pagina
	if err := json.Unmarshal(resp.Corpo, &pg); err != nil {
		t.Fatalf("%s: %v (%s)", query, err, resp.Corpo)
	}
	return pg
}

func (pg pagina) ids() string {
	var ids []string
	for _, i := range pg.Itens {
		ids = append(ids, fmt.Sprint(i.Id))
	}
	return strings.Join(ids, " ")
}

// cenario é uma base pequena: duas contas, uma categoria com subcategoria e natureza herdada, uma sem natureza, e
// transações que exercitam cada filtro.
type cenario struct {
	mux                                                                             http.Handler
	corrente, cartao                                                                testeapoio.Conta
	alimentacao, restaurante, mercado, salario                                      testeapoio.Categoria
	padaria, restauranteJantar, mercadoMes, pix, salarioMes, semCategoria, ignorada int64
}

func novoCenario(t *testing.T) cenario {
	mux, _, fab := rotasDeTeste(t, minimoFixo{n: 4})
	c := cenario{mux: mux}
	c.corrente = fab.Conta(func(x *testeapoio.Conta) { x.Nome = "corrente" })
	c.cartao = fab.Conta(func(x *testeapoio.Conta) { x.Nome, x.Tipo = "Cartão", 3 })
	c.alimentacao = fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Natureza = "Alimentação", p(int64(2)) })
	c.restaurante = fab.Subcategoria(c.alimentacao, func(x *testeapoio.Categoria) { x.Nome = "restaurante" })
	c.mercado = fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Natureza = "Mercado", p(int64(1)) })
	c.salario = fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Tipo = "Salário", 2 })
	lancar := func(conta testeapoio.Conta, dia int, valor, descricao string, ajustes ...func(*testeapoio.Transacao)) int64 {
		v, err := tipos.NovoDecimal(valor)
		if err != nil {
			t.Fatal(err)
		}
		return fab.Transacao(conta, append([]func(*testeapoio.Transacao){func(x *testeapoio.Transacao) {
			x.Data, x.Valor, x.Descricao = tipos.NovaData(2026, 10, dia), v, descricao
		}}, ajustes...)...).Id
	}
	c.padaria = lancar(c.corrente, 1, "-12.75", "PADARIA_BOM 100%", func(x *testeapoio.Transacao) { x.CategoriaId = &c.alimentacao.Id })
	c.restauranteJantar = lancar(c.cartao, 2, "-18.9", "Restaurante Ação", func(x *testeapoio.Transacao) {
		x.CategoriaId, x.Emergencia, x.Observacao = &c.restaurante.Id, true, p(`jantar c:\casa`)
	})
	c.mercadoMes = lancar(c.corrente, 3, "-100.0", "SUPERMERCADO", func(x *testeapoio.Transacao) {
		x.CategoriaId, x.DescricaoOriginal = &c.mercado.Id, p("MERCADO EXEMPLO LTDA")
	})
	c.pix = lancar(c.corrente, 4, "-50.0", "PIX ENVIADO", func(x *testeapoio.Transacao) { x.MeioPagamento = p("PIX") })
	c.salarioMes = lancar(c.corrente, 5, "6500.0", "SALARIO", func(x *testeapoio.Transacao) { x.CategoriaId, x.Tipo = &c.salario.Id, 2 })
	c.semCategoria = lancar(c.corrente, 5, "-0.01", "Tarifa")
	c.ignorada = lancar(c.corrente, 6, "-999.0", "IGNORADA", func(x *testeapoio.Transacao) {
		x.IgnorarRelatorios, x.Estabelecimento = true, p("Loja ignorada")
	})
	return c
}

// Regra: a lista: da data mais recente para a mais antiga (o Id no empate), os totais pelo tipo fora das ignoradas e
// das transferências, na escala do banco ("0.0" sem nada) e com a despesa sem o sinal, e a página além do fim vazia.
func TestListarTotaisEPaginas(t *testing.T) {
	c := novoCenario(t)
	todas := listar(t, c.mux, "")
	if todas.ids() != fmt.Sprintf("%d %d %d %d %d %d %d", c.ignorada, c.semCategoria, c.salarioMes, c.pix, c.mercadoMes,
		c.restauranteJantar, c.padaria) || todas.Total != 7 {
		t.Errorf("ordem padrão: %s (%d)", todas.ids(), todas.Total)
	}
	if string(todas.TotalReceitas) != "6500.0" || string(todas.TotalDespesas) != "181.66" {
		t.Errorf("totais: %s e %s", todas.TotalReceitas, todas.TotalDespesas)
	}
	vazia := listar(t, c.mux, "pagina=3&tamanhoPagina=5")
	if len(vazia.Itens) != 0 || vazia.Total != 7 {
		t.Errorf("página além do fim: %s (%d)", vazia.ids(), vazia.Total)
	}
	nada := listar(t, c.mux, "busca=nao-existe")
	if string(nada.TotalReceitas) != "0.0" || string(nada.TotalDespesas) != "0.0" || nada.Itens == nil {
		t.Errorf("sem nada: %s e %s, itens %v", nada.TotalReceitas, nada.TotalDespesas, nada.Itens)
	}
	if pg := listar(t, c.mux, "tamanhoPagina=2&pagina=2"); pg.ids() != fmt.Sprintf("%d %d", c.salarioMes, c.pix) {
		t.Errorf("página 2: %s", pg.ids())
	}
}

// Regra: cada filtro: período, conta, categoria com as subcategorias, sem categoria (OU), natureza da categoria ou da
// pai, sem natureza, tipo pelo número e pelo nome, emergência, ids, busca (sem caixa em ASCII; % _ e \ como texto; na
// descrição do banco, no estabelecimento e na observação), lançamento calculado e descrição pouco informativa.
func TestFiltros(t *testing.T) {
	c := novoCenario(t)
	casos := []struct {
		query    string
		esperado []int64
	}{
		{"inicio=2026-10-04&fim=10/05/2026", []int64{c.semCategoria, c.salarioMes, c.pix}},
		{fmt.Sprintf("contaId=%d", c.cartao.Id), []int64{c.restauranteJantar}},
		{fmt.Sprintf("categoriaId=%d", c.alimentacao.Id), []int64{c.restauranteJantar, c.padaria}},
		{fmt.Sprintf("categoriaIds[0]=%d&semCategoria=true", c.mercado.Id), []int64{c.ignorada, c.semCategoria, c.pix, c.mercadoMes}},
		{fmt.Sprintf("categoriaId=%d&categoriaIds=%d", c.mercado.Id, c.mercado.Id), []int64{c.mercadoMes}},
		{"naturezas=EstiloDeVida", []int64{c.restauranteJantar, c.padaria}},
		{"naturezas=1&semNatureza=true", []int64{c.ignorada, c.semCategoria, c.salarioMes, c.pix, c.mercadoMes}},
		{"tipo=Receita", []int64{c.salarioMes}},
		{"tipo=1&somenteEmergencias=true", []int64{c.restauranteJantar}},
		{fmt.Sprintf("ids=%d&ids=%d&ids=999", c.pix, c.padaria), []int64{c.pix, c.padaria}},
		{"busca=supermercado", []int64{c.mercadoMes}},
		{"busca=exemplo", []int64{c.mercadoMes}},
		{"busca=%25", []int64{c.padaria}},
		{"busca=_", []int64{c.padaria}},
		{"busca=c:%5Ccasa", []int64{c.restauranteJantar}},
		{"busca=a%C3%A7%C3%A3o", []int64{c.restauranteJantar}},
		{"busca=A%C3%87%C3%83O", nil},
		{"busca=loja%20ignorada", []int64{c.ignorada}},
		{"busca=%20%20", []int64{c.ignorada, c.semCategoria, c.salarioMes, c.pix, c.mercadoMes, c.restauranteJantar, c.padaria}},
		{"lancamento=pix", []int64{c.pix}},
		{"lancamento=cartao", []int64{c.restauranteJantar}},
		{"poucoInformativa=true", []int64{c.pix}},
	}
	for _, caso := range casos {
		esperado := make([]string, len(caso.esperado))
		for i, id := range caso.esperado {
			esperado[i] = fmt.Sprint(id)
		}
		if obtido := listar(t, c.mux, caso.query+"&tamanhoPagina=50").ids(); obtido != strings.Join(esperado, " ") {
			t.Errorf("%s: %q, esperado %q", caso.query, obtido, strings.Join(esperado, " "))
		}
	}
}

// Regra: as ordens (#244): valor sem o sinal, com a ordem do EF_DECIMAL em pt-BR (#499: "12.75" vale 1275 e fica
// depois de "100.0" e de "18.9"), categoria sem diferença de maiúsculas com as sem categoria no fim, conta sem
// diferença de maiúsculas, e data crescente; a data mais recente e o Id desempatam.
func TestOrdens(t *testing.T) {
	c := novoCenario(t)
	casos := []struct {
		query    string
		esperado []int64
	}{
		{"ordem=valor&direcao=asc", []int64{c.semCategoria, c.restauranteJantar, c.pix, c.mercadoMes, c.padaria, c.ignorada, c.salarioMes}},
		{"ordem=valor", []int64{c.salarioMes, c.ignorada, c.padaria, c.mercadoMes, c.pix, c.restauranteJantar, c.semCategoria}},
		{"ordem=categoria&direcao=asc", []int64{c.padaria, c.mercadoMes, c.restauranteJantar, c.salarioMes, c.ignorada, c.semCategoria, c.pix}},
		{"ordem=categoria", []int64{c.salarioMes, c.restauranteJantar, c.mercadoMes, c.padaria, c.ignorada, c.semCategoria, c.pix}},
		{"ordem=conta&direcao=asc", []int64{c.restauranteJantar, c.ignorada, c.semCategoria, c.salarioMes, c.pix, c.mercadoMes, c.padaria}},
		{"ordem=conta", []int64{c.ignorada, c.semCategoria, c.salarioMes, c.pix, c.mercadoMes, c.padaria, c.restauranteJantar}},
		{"ordem=data&direcao=asc", []int64{c.padaria, c.restauranteJantar, c.mercadoMes, c.pix, c.salarioMes, c.semCategoria, c.ignorada}},
	}
	for _, caso := range casos {
		esperado := make([]string, len(caso.esperado))
		for i, id := range caso.esperado {
			esperado[i] = fmt.Sprint(id)
		}
		if obtido := listar(t, c.mux, caso.query).ids(); obtido != strings.Join(esperado, " ") {
			t.Errorf("%s: %q, esperado %q", caso.query, obtido, strings.Join(esperado, " "))
		}
	}
	if pg := listar(t, c.mux, "ordem=valor&tamanhoPagina=2&pagina=5"); len(pg.Itens) != 0 {
		t.Errorf("ordem por valor além do fim: %s", pg.ids())
	}
	if pg := listar(t, c.mux, "ordem=valor&tamanhoPagina=2&pagina=2"); pg.ids() != fmt.Sprintf("%d %d", c.padaria, c.mercadoMes) {
		t.Errorf("ordem por valor, página 2: %s", pg.ids())
	}
}

// Regra: a chave da ordem pelo valor é a do EF_DECIMAL em pt-BR (#499): o texto sem os pontos, como número inteiro;
// o negativo entra pelo valor sem o sinal, no formato do banco.
func TestCompararComoOEFDecimal(t *testing.T) {
	casos := []struct {
		a, b     string
		esperado int
	}{
		{"18.9", "12.75", -1}, // 189 < 1275
		{"-18.9", "-12.75", -1},
		{"100.0", "12.75", -1}, // 1000 < 1275
		{"6500.0", "12.75", 1}, // 65000 > 1275
		{"0.01", "0.1", 0},     // 1 = 1
		{"-10.00", "10.0", 0},  // o negativo no formato do banco: 10.0
		{"0.0", "0.01", -1},
	}
	for _, c := range casos {
		a, _ := tipos.NovoDecimal(c.a)
		b, _ := tipos.NovoDecimal(c.b)
		if obtido := compararComoOEFDecimal(a, b); obtido != c.esperado {
			t.Errorf("comparar(%s, %s) = %d, esperado %d", c.a, c.b, obtido, c.esperado)
		}
	}
}

// Regra: os erros do model binding do filtro: a propriedade que não converte cita o nome dela; o item de lista que
// não converte, não; o enum fora da lista é "invalid"; e as validações do DTO (faixas, padrões, tamanhos).
func TestErrosDoFiltro(t *testing.T) {
	mux, _, _ := rotasDeTeste(t, minimoFixo{n: 4})
	casos := []struct{ query, erros string }{
		{"pagina=um&tamanhoPagina=", `{"Pagina":["The value 'um' is not valid for Pagina."],"TamanhoPagina":["The value '' is invalid."]}`},
		{"inicio=2026-13-01&contaId=x&categoriaId=1.5", `{"Inicio":["The value '2026-13-01' is not valid for Inicio."],"ContaId":["The value 'x' is not valid for ContaId."],"CategoriaId":["The value '1.5' is not valid for CategoriaId."]}`},
		{"ids=1&ids=x&categoriaIds=y", `{"Ids":["The value 'x' is not valid."],"CategoriaIds":["The value 'y' is not valid."]}`},
		{"naturezas=9&naturezas=luxo", `{"Naturezas":["The value '9' is invalid.","The value 'luxo' is not valid."]}`},
		{"tipo=abc&somenteEmergencias=sim&semCategoria=", `{"Tipo":["The value 'abc' is not valid for Tipo."],"SomenteEmergencias":["The value 'sim' is not valid for SomenteEmergencias."],"SemCategoria":["The value '' is invalid."]}`},
		{"tipo=9", `{"Tipo":["The value '9' is invalid."]}`},
		{"pagina=0&tamanhoPagina=501&ordem=x&direcao=y&lancamento=PIX", `{"Pagina":["The field Pagina must be between 1 and 2147483647."],"TamanhoPagina":["The field TamanhoPagina must be between 1 and 500."],"Ordem":["The field Ordem must match the regular expression '^(data|categoria|conta|valor)$'."],"Direcao":["The field Direcao must match the regular expression '^(asc|desc)$'."],"Lancamento":["The field Lancamento must match the regular expression '^(cartao|debito|pix|transferencia|entre-contas|fatura|boleto|saque-deposito|tarifas|investimentos|salario|credito|dinheiro|outros)$'."]}`},
		{"busca=" + strings.Repeat("x", 101) + "&naturezas=1&naturezas=2&naturezas=1", `{"Busca":["The field Busca must be a string with a maximum length of 100."],"Naturezas":["The field Naturezas must be a string or array type with a maximum length of '2'."]}`},
		{"ids=" + strings.Repeat("1&ids=", 200) + "1", `{"Ids":["The field Ids must be a string or array type with a maximum length of '200'."]}`},
		{"categoriaIds=" + strings.Repeat("1&categoriaIds=", 200) + "1", `{"CategoriaIds":["The field CategoriaIds must be a string or array type with a maximum length of '200'."]}`},
	}
	for _, c := range casos {
		resp := testeapoio.Chamar(t, mux, "GET", "/api/transacoes?"+c.query, "")
		resp.ConferirStatus(t, http.StatusBadRequest)
		var problema struct{ Errors json.RawMessage }
		if err := json.Unmarshal(resp.Corpo, &problema); err != nil {
			t.Fatal(err)
		}
		testeapoio.ConferirJSON(t, problema.Errors, c.erros)
	}
	// Vazio nos opcionais (data, conta, texto) não é erro: vale null.
	listar(t, mux, "inicio=&contaId=&categoriaId=&ordem=&direcao=&lancamento=&busca=&tipo=")
	listar(t, mux, "tamanhoPagina=500&ordem=data&direcao=desc&lancamento=outros")
}

// Regra: obter devolve o DTO com a conta, a categoria, o tipo de lançamento calculado e o "veio do Open Finance";
// inexistente é 404 com mensagem; {id:long} que não casa é 404 sem corpo.
func TestObter(t *testing.T) {
	mux, _, fab := rotasDeTeste(t, minimoFixo{n: 4})
	conta := fab.Conta(func(x *testeapoio.Conta) { x.Nome = "Corrente" })
	categoria := fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Cor = "Mercado", "#00ff00" })
	tr := fab.Transacao(conta, func(x *testeapoio.Transacao) {
		x.CategoriaId, x.PluggyTransactionId, x.MeioPagamento = &categoria.Id, p("pluggy-1"), p("PIX")
		x.Descricao, x.ParcelaNumero, x.ParcelaTotal, x.Data = "PIX ENVIADO MERCADO", p(int64(2)), p(int64(3)), tipos.NovaData(2026, 10, 1)
	})
	testeapoio.Chamar(t, mux, "GET", fmt.Sprintf("/api/transacoes/%d", tr.Id), "").ConferirJSON(t, fmt.Sprintf(`{"id":%d,
		"contaId":%d,"contaNome":"Corrente","openFinance":true,"data":"2026-10-01","descricao":"PIX ENVIADO MERCADO",
		"descricaoOriginal":null,"estabelecimento":null,"valor":-10.0,"tipo":1,"status":1,"categoriaId":%d,
		"categoriaNome":"Mercado","categoriaCor":"#00ff00","parcelaNumero":2,"parcelaTotal":3,"ignorarRelatorios":false,
		"observacao":null,"emergencia":false,"tipoLancamento":4,"regraDescricaoId":null}`, tr.Id, conta.Id, categoria.Id))
	inexistente := testeapoio.Chamar(t, mux, "GET", "/api/transacoes/999", "")
	inexistente.ConferirStatus(t, http.StatusNotFound)
	inexistente.ConferirJSON(t, `{"mensagem":"Registro não encontrado."}`)
	for _, caminho := range []string{"/api/transacoes/abc", "/api/transacoes/9223372036854775808"} {
		resp := testeapoio.Chamar(t, mux, "GET", caminho, "")
		resp.ConferirStatus(t, http.StatusNotFound)
		if len(resp.Corpo) != 0 {
			t.Errorf("%s: corpo %s", caminho, resp.Corpo)
		}
	}
}

// Regra: erro do banco (ou dos parâmetros) é o 500 genérico.
func TestErroDoBanco(t *testing.T) {
	mux, db, _ := rotasDeTeste(t, minimoFixo{err: errors.New("parâmetros fora")})
	testeapoio.Chamar(t, mux, "GET", "/api/transacoes?poucoInformativa=true", "").ConferirStatus(t, http.StatusInternalServerError)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"", "lancamento=pix", "ordem=valor", "ordem=conta"} {
		testeapoio.Chamar(t, mux, "GET", "/api/transacoes?"+query, "").ConferirStatus(t, http.StatusInternalServerError)
	}
	testeapoio.Chamar(t, mux, "GET", "/api/transacoes/1", "").ConferirStatus(t, http.StatusInternalServerError)
	testeapoio.Chamar(t, mux, "POST", "/api/transacoes", `{"contaId":1,"data":"2026-10-01","descricao":"x","valor":1,"tipo":1}`).
		ConferirStatus(t, http.StatusInternalServerError)
}
