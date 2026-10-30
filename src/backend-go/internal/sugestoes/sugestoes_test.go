package sugestoes

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func p[T any](v T) *T { return &v }

var hoje = tipos.NovaData(2026, 10, 15)

type cenario struct {
	db                                         *sql.DB
	fab                                        *testeapoio.Fabrica
	conta                                      testeapoio.Conta
	mercado, transporte, lazer, salario, saude testeapoio.Categoria
}

func novoCenario(t *testing.T) cenario {
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 15))
	c := cenario{db: db, fab: fab, conta: fab.Conta()}
	c.mercado = fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Cor = "Mercado", "#111111" })
	c.transporte = fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Cor = "Transporte", "#222222" })
	c.lazer = fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Cor = "Lazer", "#333333" })
	c.salario = fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Cor, x.Tipo = "Salário", "#444444", 2 })
	c.saude = fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Cor = "Saúde", "#555555" })
	return c
}

func (c cenario) lancar(descricao string, valor string, categoria *int64, ajustes ...func(*testeapoio.Transacao)) int64 {
	v, _ := tipos.NovoDecimal(valor)
	return c.fab.Transacao(c.conta, append([]func(*testeapoio.Transacao){func(x *testeapoio.Transacao) {
		x.Descricao, x.Valor, x.CategoriaId, x.Data = descricao, v, categoria, tipos.NovaData(2026, 10, 1)
	}}, ajustes...)...).Id
}

func manual(x *testeapoio.Transacao) { x.CategoriaManual = true }

// Regra: a avaliação pelo histórico: a mesma chave pesa 3, e o que você escolheu à mão vale 3 votos; a confiança é alta
// com 70% dos votos e pelo menos 2; a primeira palavra (4 letras ou mais) pesa 1,5; a palavra-chave conhecida, 1, só a
// primeira que aparece; receita não sugere categoria de despesa; a categoria que a transação já tem fica de fora; até 3.
func TestAvaliar(t *testing.T) {
	c := novoCenario(t)
	c.lancar("PADARIA SABOR BOM", "-10", &c.mercado.Id, manual)
	c.lancar("PADARIA SABOR BOM", "-10", &c.lazer.Id)
	c.lancar("PADARIA OUTRA", "-10", &c.transporte.Id)
	c.lancar("SALARIO EMPRESA", "5000", &c.salario.Id)
	ix, err := montarIndice(t.Context(), c.db, hoje)
	if err != nil {
		t.Fatal(err)
	}
	opcoes, confianca := ix.avaliar("PADARIA SABOR BOM", false, nil)
	if confianca != "alta" || len(opcoes) != 3 || opcoes[0].CategoriaId != c.mercado.Id || opcoes[1].CategoriaId != c.lazer.Id ||
		opcoes[2].CategoriaId != c.transporte.Id {
		t.Fatalf("avaliar: %+v, %s", opcoes, confianca)
	}
	if opcoes[0].Motivo != "Você já usou para transações iguais (75% das 4 parecidas)" ||
		opcoes[2].Motivo != `Usada em transações com "PADARIA" (20% das 5 parecidas)` {
		t.Errorf("motivos: %q e %q", opcoes[0].Motivo, opcoes[2].Motivo)
	}
	// Sem histórico da chave: a primeira palavra dá média; só a palavra-chave dá baixa.
	if _, confianca := ix.avaliar("PADARIA NOVA", false, nil); confianca != "media" {
		t.Errorf("pela primeira palavra: %s", confianca)
	}
	if opcoes, confianca := ix.avaliar("UBER *TRIP", false, nil); confianca != "baixa" || len(opcoes) != 1 ||
		opcoes[0].CategoriaId != c.transporte.Id || opcoes[0].Motivo != `Palavra-chave "UBER"` {
		t.Errorf("pela palavra-chave: %+v, %s", opcoes, confianca)
	}
	// "BAR " é de Alimentação, que não existe aqui: a palavra-chave é pulada, e vale a primeira das outras (Saúde).
	if opcoes, _ := ix.avaliar("FARMACIA E BAR", false, nil); len(opcoes) != 1 || opcoes[0].CategoriaId != c.saude.Id {
		t.Errorf("só a primeira palavra-chave: %+v", opcoes)
	}
	// Receita não sugere categoria de despesa (a de receita, sim).
	if opcoes, confianca := ix.avaliar("PADARIA SABOR BOM", true, nil); len(opcoes) != 0 || confianca != "nenhuma" {
		t.Errorf("receita com histórico de despesa: %+v, %s", opcoes, confianca)
	}
	if opcoes, _ := ix.avaliar("SALARIO EMPRESA", true, nil); len(opcoes) != 1 || opcoes[0].CategoriaId != c.salario.Id {
		t.Errorf("receita: %+v", opcoes)
	}
	if opcoes, _ := ix.avaliar("SALARIO EMPRESA", false, nil); len(opcoes) != 0 {
		t.Errorf("despesa não sugere receita: %+v", opcoes)
	}
	// A categoria que ela já tem fica de fora.
	if opcoes, _ := ix.avaliar("PADARIA SABOR BOM", false, &c.mercado.Id); opcoes[0].CategoriaId != c.lazer.Id {
		t.Errorf("sem a categoria atual: %+v", opcoes)
	}
	if opcoes, confianca := ix.avaliar("", false, nil); len(opcoes) != 0 || confianca != "nenhuma" {
		t.Errorf("vazio: %+v, %s", opcoes, confianca)
	}
}

// Regra: no empate de pontos, vale a categoria que apareceu primeiro no histórico (a ordem de inserção do Dictionary
// do C#), e a confiança é média (50%).
func TestEmpateNaOrdemDoHistorico(t *testing.T) {
	c := novoCenario(t)
	c.lancar("LOJA AZUL CENTRO", "-1", &c.lazer.Id)
	c.lancar("LOJA AZUL CENTRO", "-1", &c.mercado.Id)
	ix, err := montarIndice(t.Context(), c.db, hoje)
	if err != nil {
		t.Fatal(err)
	}
	opcoes, confianca := ix.avaliar("LOJA AZUL CENTRO", false, nil)
	if confianca != "media" || opcoes[0].CategoriaId != c.lazer.Id || opcoes[1].CategoriaId != c.mercado.Id {
		t.Errorf("empate: %+v, %s", opcoes, confianca)
	}
}

// Regra: o histórico é dos últimos 730 dias e só das transações com categoria.
func TestHistoricoDe730Dias(t *testing.T) {
	c := novoCenario(t)
	c.lancar("POSTO VELHO", "-1", &c.transporte.Id, func(x *testeapoio.Transacao) { x.Data = hoje.SomarDias(-731) })
	c.lancar("POSTO NO LIMITE", "-1", &c.transporte.Id, func(x *testeapoio.Transacao) { x.Data = hoje.SomarDias(-730) })
	c.lancar("POSTO SEM CATEGORIA", "-1", nil)
	ix, err := montarIndice(t.Context(), c.db, hoje)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ix.porChave["POSTO VELHO"]; ok {
		t.Error("o de 731 dias entrou")
	}
	if _, ok := ix.porChave["POSTO LIMITE"]; !ok {
		t.Error("o de 730 dias ficou fora")
	}
	if len(ix.textos) != 1 {
		t.Errorf("%d textos, esperado 1", len(ix.textos))
	}
}

// Regra: o aviso da regra (#254): a regra igual já existe (para a mesma categoria ou para outra); o texto aparece em
// pelo menos 3 transações de outras categorias e em 20% das que ele acha (com as duas categorias de mais transações);
// sem texto, nenhum aviso.
func TestAvisoRegra(t *testing.T) {
	c := novoCenario(t)
	c.fab.RegraCategorizacao(c.mercado, func(r *testeapoio.RegraCategorizacao) { r.Texto = "feira livre" })
	for range 3 {
		c.lancar("POSTO SHELL", "-1", &c.lazer.Id)
	}
	c.lancar("POSTO SHELL", "-1", &c.saude.Id)
	c.lancar("POSTO SHELL", "-1", &c.transporte.Id)
	ix, err := montarIndice(t.Context(), c.db, hoje)
	if err != nil {
		t.Fatal(err)
	}
	casos := []struct {
		texto     string
		categoria *int64
		esperado  string
	}{
		{"FEIRA LIVRE", &c.mercado.Id, `A regra "FEIRA LIVRE" já existe.`},
		{"FEIRA LIVRE", &c.lazer.Id, `Já existe a regra "FEIRA LIVRE" para Mercado.`},
		{"FEIRA LIVRE", nil, `Já existe a regra "FEIRA LIVRE" para Mercado.`},
		{"POSTO", &c.transporte.Id, `"POSTO" também aparece em 4 transações de outras categorias (Lazer, Saúde).`},
		{"POSTO", &c.lazer.Id, ""}, // 2 de outras categorias: menos que 3
		{"", &c.lazer.Id, ""},
		{"NADA", nil, ""},
	}
	for _, caso := range casos {
		obtido := ""
		if a := ix.avisoRegra(caso.texto, caso.categoria); a != nil {
			obtido = *a
		}
		if obtido != caso.esperado {
			t.Errorf("avisoRegra(%q): %q, esperado %q", caso.texto, obtido, caso.esperado)
		}
	}
}

// Regra: os grupos: pela chave (ou, sem palavras, pela descrição em maiúsculas), os maiores primeiro (e os de maior
// valor no empate); o total arredondado a 2 casas; com todas as transações, só os grupos em que a sugestão muda a
// categoria de hoje; no fim, a confiança alta primeiro; ficam de fora as de categoria escolhida à mão, as de transferência
// do sistema e as de antes do período. As transações de mesma data vêm na ordem que o SQLite dá à consulta do C#
// (ORDER BY "Data" DESC, sem desempate): aqui, a de Id maior primeiro.
func TestGrupos(t *testing.T) {
	c := novoCenario(t)
	c.lancar("UBER *TRIP", "-10.005", nil)
	c.lancar("UBER *TRIP", "-20", nil)
	c.lancar("123 456", "-5", nil)
	c.lancar("123 456", "-5", nil)
	c.lancar("123 456", "-5", nil)
	c.lancar("XYZW CAFE", "-1", nil)
	c.lancar("POSTO SHELL", "-100", &c.transporte.Id) // já na categoria que seria sugerida
	c.lancar("POSTO SHELL", "-100", &c.transporte.Id, manual)
	c.lancar("PADARIA", "-3", &c.mercado.Id) // a palavra-chave sugere Alimentação, que não existe: sem sugestão
	c.lancar("UBER ANTIGO", "-1", nil, func(x *testeapoio.Transacao) { x.Data = hoje.SomarDias(-400) })
	grupo := tipos.NovoGuid()
	c.lancar("UBER TRANSFERENCIA", "-1", nil, func(x *testeapoio.Transacao) { x.TransferenciaGrupo = &grupo })

	sem, err := Grupos(t.Context(), c.db, hoje, true, 365)
	if err != nil {
		t.Fatal(err)
	}
	dados, _ := json.Marshal(sem)
	testeapoio.ConferirJSON(t, dados, `[
		{"chave":"UBER TRIP","exemplo":"UBER *TRIP","quantidade":2,"total":-30.00,"transacaoIds":[2,1],"categoriaId":2,
		 "categoriaNome":"Transporte","categoriaCor":"#222222","confianca":"baixa","motivo":"Palavra-chave \"UBER\"",
		 "textoRegra":"UBER","alternativas":[],"avisoRegra":null},
		{"chave":"123 456","exemplo":"123 456","quantidade":3,"total":-15.0,"transacaoIds":[5,4,3],"categoriaId":null,
		 "categoriaNome":null,"categoriaCor":null,"confianca":"nenhuma","motivo":null,"textoRegra":"","alternativas":[],
		 "avisoRegra":null},
		{"chave":"XYZW CAFE","exemplo":"XYZW CAFE","quantidade":1,"total":-1.0,"transacaoIds":[6],"categoriaId":null,
		 "categoriaNome":null,"categoriaCor":null,"confianca":"nenhuma","motivo":null,"textoRegra":"XYZW CAFE","alternativas":[],
		 "avisoRegra":null}]`)

	todas, err := Grupos(t.Context(), c.db, hoje, false, 365)
	if err != nil {
		t.Fatal(err)
	}
	if len(todas) != 1 || todas[0].Chave != "UBER TRIP" {
		t.Errorf("com todas: %+v", todas)
	}
	if curto, _ := Grupos(t.Context(), c.db, hoje, true, 1); len(curto) != 0 {
		t.Errorf("1 dia: %+v", curto)
	}
}

// Regra: a sugestão para uma transação: inexistente é ErrNaoEncontrada; a categoria que ela já tem fica de fora.
func TestParaTransacao(t *testing.T) {
	c := novoCenario(t)
	id := c.lancar("POSTO SHELL", "-1", &c.transporte.Id)
	if _, err := ParaTransacao(t.Context(), c.db, hoje, 999); !errors.Is(err, ErrNaoEncontrada) {
		t.Errorf("inexistente: %v", err)
	}
	opcoes, err := ParaTransacao(t.Context(), c.db, hoje, id)
	if err != nil || len(opcoes) != 0 {
		t.Errorf("a própria categoria fica de fora: %+v, %v", opcoes, err)
	}
}

// Regra: erro do banco volta como erro, em cada consulta.
func TestErroDoBanco(t *testing.T) {
	c := novoCenario(t)
	id := c.lancar("X", "-1", nil)
	if err := c.db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Grupos(t.Context(), c.db, hoje, true, 30); err == nil {
		t.Error("Grupos com o banco fechado não deu erro")
	}
	if _, err := ParaTransacao(t.Context(), c.db, hoje, id); err == nil || errors.Is(err, ErrNaoEncontrada) {
		t.Errorf("ParaTransacao com o banco fechado: %v", err)
	}
	if _, err := montarIndice(t.Context(), c.db, hoje); err == nil {
		t.Error("montarIndice com o banco fechado não deu erro")
	}
	_ = p(1)
}
