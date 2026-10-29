package sugestoes

import (
	"slices"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// lancarVarias grava n transações sem categoria num INSERT só (com a Fabrica, uma a uma, seriam segundos). Na
// descrição, o "#" vira o índice em três letras ("AAA", "AAB"...): cada uma fica com a sua chave.
func (c cenario) lancarVarias(t *testing.T, n int, descricao, valor, data string) {
	t.Helper()
	if _, err := c.db.ExecContext(t.Context(), `WITH RECURSIVE n(i) AS (SELECT 0 UNION ALL SELECT i + 1 FROM n WHERE i + 1 < ?)
		INSERT INTO "Transacao" ("ContaId", "Data", "Descricao", "Valor", "Tipo", "CriadoEm", "AtualizadoEm")
		SELECT ?, ?, replace(?, '#', char(65 + i / 676 % 26, 65 + i / 26 % 26, 65 + i % 26)), ?, 1, ?, ? FROM n`,
		n, c.conta.Id, data, descricao, valor, data+" 00:00:00", data+" 00:00:00"); err != nil {
		t.Fatal(err)
	}
}

// Regra: os limites da avaliação. A primeira palavra entra no índice com 4 letras ou mais (e a descrição sem palavras
// não entra); a confiança é alta com 70% dos votos exatos e com 2 votos exatos; a categoria de mais pontos vem antes da
// que entrou primeiro; os pontos da mesma categoria somam entre as fontes; o motivo é o da fonte de mais pontos (no
// empate, a primeira); só a primeira palavra-chave conta.
func TestAvaliarNosLimites(t *testing.T) {
	c := novoCenario(t)
	alimentacao := c.fab.Categoria(func(x *testeapoio.Categoria) { x.Nome, x.Cor = "Alimentação", "#666666" })
	c.lancar("PIX ENVIADO", "-1", &c.lazer.Id) // sem palavras: não entra no índice (nem quebra)
	c.lancar("BAR DO ZE", "-1", &c.lazer.Id)
	c.lancar("UBER TRIP 123", "-1", &c.transporte.Id)
	for range 7 {
		c.lancar("CASA SETE", "-1", &c.lazer.Id)
	}
	for range 3 {
		c.lancar("CASA SETE", "-1", &c.mercado.Id)
	}
	c.lancar("BOLO DUAS", "-1", &c.mercado.Id)
	c.lancar("BOLO DUAS", "-1", &c.mercado.Id)
	c.lancar("ROSA AZUL", "-1", &c.lazer.Id)
	c.lancar("ROSA AZUL", "-1", &c.mercado.Id, manual)
	c.lancar("BAR LUA", "-1", &c.lazer.Id)
	c.lancar("BAR LUA", "-1", &alimentacao.Id)
	c.lancar("BAR SOL", "-1", &c.lazer.Id, manual)
	c.lancar("BAR SOL", "-1", &alimentacao.Id)
	c.lancar("BAR MAR", "-1", &c.lazer.Id)
	c.lancar("BAR MAR", "-1", &c.lazer.Id)
	c.lancar("BAR MAR", "-1", &alimentacao.Id)
	ix, err := montarIndice(t.Context(), c.db, hoje)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ix.porPalavra["BAR"]; ok {
		t.Error("a primeira palavra de 3 letras entrou no índice")
	}
	if _, ok := ix.porPalavra["UBER"]; !ok {
		t.Error("a primeira palavra de 4 letras ficou fora do índice")
	}
	ids := func(opcoes []OpcaoDto) []int64 {
		lista := []int64{}
		for _, o := range opcoes {
			lista = append(lista, o.CategoriaId)
		}
		return lista
	}
	casos := []struct {
		nome, texto, confianca string
		categorias             []int64
		motivos                map[int]string
	}{
		{"70% exatos", "CASA SETE", "alta", []int64{c.lazer.Id, c.mercado.Id}, nil},
		{"2 votos exatos", "BOLO DUAS", "alta", []int64{c.mercado.Id}, nil},
		{"a de mais pontos antes da que entrou primeiro", "ROSA AZUL", "alta", []int64{c.mercado.Id, c.lazer.Id}, nil},
		{"os pontos somam entre as fontes", "BAR LUA", "media", []int64{alimentacao.Id, c.lazer.Id}, nil},
		{"o motivo da fonte de mais pontos", "BAR SOL", "alta", []int64{c.lazer.Id, alimentacao.Id},
			map[int]string{1: `Palavra-chave "BAR"`}},
		{"no empate de pontos, o motivo da primeira fonte", "BAR MAR", "media", []int64{c.lazer.Id, alimentacao.Id},
			map[int]string{1: "Você já usou para transações iguais (33% das 3 parecidas)"}},
		{"só a primeira palavra-chave", "UBER FARMACIA", "media", []int64{c.transporte.Id}, nil},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			opcoes, confianca := ix.avaliar(caso.texto, false, nil)
			if confianca != caso.confianca || !slices.Equal(ids(opcoes), caso.categorias) {
				t.Fatalf("avaliar(%q): %+v, %s", caso.texto, opcoes, confianca)
			}
			for i, motivo := range caso.motivos {
				if opcoes[i].Motivo != motivo {
					t.Errorf("motivo %d: %q, esperado %q", i, opcoes[i].Motivo, motivo)
				}
			}
		})
	}
}

// Regra: os limites do aviso da regra (#254): 3 conflitos exatos avisam; 20% exatos também; 15% não. As transações
// que não têm o texto não contam (nem param a contagem). Os exemplos são as duas categorias de mais conflitos.
func TestAvisoRegraNosLimites(t *testing.T) {
	c := novoCenario(t)
	c.lancar("OUTRA COISA", "-1", &c.lazer.Id)
	lancar := func(descricao string, conflitos, iguais int) {
		for range conflitos {
			c.lancar(descricao, "-1", &c.lazer.Id)
		}
		for range iguais {
			c.lancar(descricao, "-1", &c.mercado.Id)
		}
	}
	lancar("ALFA", 3, 7)
	lancar("BETA", 4, 16)
	lancar("GAMA", 3, 17)
	// Os exemplos são as duas categorias de mais transações, e não as que apareceram primeiro.
	c.lancar("DELTA", "-1", &c.saude.Id)
	for range 3 {
		c.lancar("DELTA", "-1", &c.lazer.Id)
	}
	c.lancar("DELTA", "-1", &c.transporte.Id)
	c.lancar("DELTA", "-1", &c.transporte.Id)
	ix, err := montarIndice(t.Context(), c.db, hoje)
	if err != nil {
		t.Fatal(err)
	}
	casos := []struct{ texto, esperado string }{
		{"ALFA", `"ALFA" também aparece em 3 transações de outras categorias (Lazer).`},
		{"BETA", `"BETA" também aparece em 4 transações de outras categorias (Lazer).`},
		{"GAMA", ""},
		{"DELTA", `"DELTA" também aparece em 6 transações de outras categorias (Lazer, Transporte).`},
	}
	for _, caso := range casos {
		obtido := ""
		if a := ix.avisoRegra(caso.texto, &c.mercado.Id); a != nil {
			obtido = *a
		}
		if obtido != caso.esperado {
			t.Errorf("avisoRegra(%q): %q, esperado %q", caso.texto, obtido, caso.esperado)
		}
	}
}

// Regra: os grupos de mais transações vêm antes (e só os 300 primeiros ficam), e no empate os de maior valor; cada
// sugestão leva até 1000 ids.
func TestGruposNosLimites(t *testing.T) {
	t.Run("os maiores antes do corte", func(t *testing.T) {
		c := novoCenario(t)
		c.lancarVarias(t, 300, "LOJA #", "-5.0", "2026-10-02") // mais novas: vêm antes na consulta
		c.lancarVarias(t, 2, "GRUPO DUPLO", "-1.0", "2026-10-01")
		lista, err := Grupos(t.Context(), c.db, hoje, true, 365)
		if err != nil {
			t.Fatal(err)
		}
		if len(lista) != 300 || lista[0].Chave != "GRUPO DUPLO" || lista[0].Quantidade != 2 {
			t.Fatalf("%d grupos, o primeiro %+v", len(lista), lista[0])
		}
	})
	t.Run("no empate, o de maior valor", func(t *testing.T) {
		c := novoCenario(t)
		c.lancarVarias(t, 1, "AZUL CLARO", "-1.0", "2026-10-02")
		c.lancarVarias(t, 1, "VERDE ESCURO", "-9.0", "2026-10-01")
		lista, err := Grupos(t.Context(), c.db, hoje, true, 365)
		if err != nil {
			t.Fatal(err)
		}
		if len(lista) != 2 || lista[0].Chave != "VERDE ESCURO" {
			t.Errorf("%+v", lista)
		}
	})
	t.Run("até 1000 ids", func(t *testing.T) {
		c := novoCenario(t)
		c.lancarVarias(t, 1001, "MESMA LOJA", "-1.0", "2026-10-01")
		lista, err := Grupos(t.Context(), c.db, hoje, true, 365)
		if err != nil {
			t.Fatal(err)
		}
		if len(lista) != 1 || lista[0].Quantidade != 1001 || len(lista[0].TransacaoIds) != 1000 {
			t.Errorf("%+v", lista)
		}
	})
}

// Regra: a mesma categoria, inclusive "sem categoria" (o int? do C#).
func TestMesmoId(t *testing.T) {
	casos := []struct {
		a, b     *int64
		esperado bool
	}{
		{nil, nil, true}, {p(int64(1)), p(int64(1)), true}, {p(int64(1)), p(int64(2)), false},
		{nil, p(int64(1)), false}, {p(int64(1)), nil, false},
	}
	for _, caso := range casos {
		if obtido := mesmoId(caso.a, caso.b); obtido != caso.esperado {
			t.Errorf("mesmoId(%v, %v) = %v", caso.a, caso.b, obtido)
		}
	}
}

// Regra: com todas as transações, o grupo de categorias diferentes entra mesmo que a sugestão seja a de uma delas.
func TestGrupoDeCategoriasDiferentes(t *testing.T) {
	c := novoCenario(t)
	c.lancar("POSTO SHELL", "-1", &c.transporte.Id)
	c.lancar("POSTO SHELL", "-1", &c.lazer.Id)
	c.lancar("POSTO SHELL", "-1", &c.transporte.Id)
	lista, err := Grupos(t.Context(), c.db, hoje, false, 365)
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != 1 || lista[0].CategoriaId == nil || *lista[0].CategoriaId != c.transporte.Id {
		t.Errorf("%+v", lista)
	}
}

// Regra: a sugestão para uma transação traz as categorias das parecidas.
func TestParaTransacaoComHistorico(t *testing.T) {
	c := novoCenario(t)
	c.lancar("POSTO SHELL", "-1", &c.transporte.Id)
	c.lancar("POSTO SHELL", "-1", &c.transporte.Id)
	id := c.lancar("POSTO SHELL", "-1", &c.lazer.Id)
	opcoes, err := ParaTransacao(t.Context(), c.db, hoje, id)
	if err != nil || len(opcoes) != 1 || opcoes[0].CategoriaId != c.transporte.Id {
		t.Errorf("%+v, %v", opcoes, err)
	}
}
