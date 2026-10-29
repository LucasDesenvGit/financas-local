package relatorios

import (
	"strconv"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/transacoes"
)

// lanc monta um lançamento de despesa da categoria dada (0 = sem categoria).
func lanc(categoria int64, valor string, ajustes ...func(*Lancamento)) Lancamento {
	l := Lancamento{Tipo: transacoes.Despesa, Valor: dec(valor)}
	if categoria != 0 {
		l.CategoriaId = &categoria
	}
	for _, ajustar := range ajustes {
		ajustar(&l)
	}
	return l
}

func receita(categoria int64, valor string) Lancamento {
	return lanc(categoria, valor, func(l *Lancamento) { l.Tipo = transacoes.Receita })
}

func emergencia(l *Lancamento) { l.Emergencia = true }

// categoriasDeTeste: 1 Mercado e 2 Transporte (raízes); 3 Feira (filha de 1); 4 Órfã (o pai 99 não existe); 5 Neto (filho
// de 3, que é filho de 1); 6 Salário; 8 Pai e 9 Filho (filho de 8).
func categoriasDeTeste() map[int64]categoriaLida {
	pai := func(id int64) *int64 { return &id }
	return map[int64]categoriaLida{
		1: {nome: "Mercado", cor: "#111111"},
		2: {nome: "Transporte", cor: "#222222"},
		3: {nome: "Feira", cor: "#333333", paiId: pai(1)},
		4: {nome: "Órfã", cor: "#444444", paiId: pai(99)},
		5: {nome: "Neto", cor: "#555555", paiId: pai(3)},
		6: {nome: "Salário", cor: "#666666"},
		8: {nome: "Pai", cor: "#888888"},
		9: {nome: "Filho", cor: "#999999", paiId: pai(8)},
	}
}

// resumo escreve o resultado como "id:nome:valor", separado por barras ("-" é sem categoria): a ordem e a escala do
// valor também são conferidas.
func resumo(lista []ValorCategoriaDto) string {
	partes := []string{}
	for _, v := range lista {
		id := "-"
		if v.CategoriaId != nil {
			id = strconv.FormatInt(*v.CategoriaId, 10)
		}
		partes = append(partes, id+":"+v.Nome+":"+v.Valor.String())
	}
	return strings.Join(partes, "|")
}

// Regras de TotaisPorCategoria (ADR 0036): despesa sai positiva e já líquida do estorno; categoria só com estorno fica de
// fora; subcategoria soma na categoria pai (um nível só); sem categoria entra por último nos empates; do maior para o menor.
func TestTotaisPorCategoria(t *testing.T) {
	casos := []struct {
		nome          string
		lancamentos   []Lancamento
		tipo          int64
		agrupar       bool
		soEmergencia  bool
		esperado      string
		categoriasMod func(map[int64]categoriaLida)
	}{
		{"uma despesa vira positiva", []Lancamento{lanc(1, "-50.00")}, 1, true, false, "1:Mercado:50.0", nil},
		{"soma na categoria, na escala do ef_sum", []Lancamento{lanc(1, "-10.25"), lanc(1, "-10.75")}, 1, true, false, "1:Mercado:21.0", nil},
		{"estorno abate a despesa (ADR 0036)", []Lancamento{lanc(1, "-100.00"), lanc(1, "30.00")}, 1, true, false, "1:Mercado:70.0", nil},
		{"categoria só com estorno fica de fora", []Lancamento{lanc(2, "30.00")}, 1, true, false, "", nil},
		{"estorno que zera o gasto: zero não passa de zero", []Lancamento{lanc(2, "-30.00"), lanc(2, "30.00")}, 1, true, false, "", nil},
		{"despesa zero fica de fora", []Lancamento{lanc(2, "0.0")}, 1, true, false, "", nil},
		{"receita não troca o sinal", []Lancamento{receita(6, "3000.00")}, 2, true, false, "6:Salário:3000.0", nil},
		{"receita negativa fica de fora", []Lancamento{receita(6, "-10.00")}, 2, true, false, "", nil},
		{"só o tipo pedido: a receita não entra nas despesas", []Lancamento{lanc(1, "-5.0"), receita(1, "100.0")}, 1, true, false, "1:Mercado:5.0", nil},
		{"só o tipo pedido: a despesa não entra nas receitas", []Lancamento{lanc(6, "-5.0"), receita(6, "100.0")}, 2, true, false, "6:Salário:100.0", nil},
		{"tipo que não é despesa nem receita não troca o sinal", []Lancamento{lanc(1, "-5.0", func(l *Lancamento) { l.Tipo = 0 })}, 0, true, false, "", nil},
		{"sem categoria", []Lancamento{lanc(0, "-20.00")}, 1, true, false, "-:Sem categoria:20.0", nil},
		{"sem categoria só com estorno some", []Lancamento{lanc(0, "20.00")}, 1, true, false, "", nil},
		{"sem categoria de receita", []Lancamento{receita(0, "20.00")}, 2, true, false, "-:Sem categoria:20.0", nil},
		{"categoria que não existe cai em sem categoria", []Lancamento{lanc(77, "-5.50"), lanc(0, "-4.50")}, 1, true, false, "-:Sem categoria:10.0", nil},
		{"categoria que não existe sozinha", []Lancamento{lanc(77, "-5.50")}, 1, true, false, "-:Sem categoria:5.5", nil},
		{"agrupa a subcategoria no pai", []Lancamento{lanc(3, "-10.00"), lanc(1, "-5.00")}, 1, true, false, "1:Mercado:15.0", nil},
		{"sem agrupar, cada uma por si", []Lancamento{lanc(3, "-10.00"), lanc(1, "-5.00")}, 1, false, false, "3:Feira:10.0|1:Mercado:5.0", nil},
		{"agrupa só um nível: o neto vai para o pai, e não para o avô", []Lancamento{lanc(5, "-7.00")}, 1, true, false, "3:Feira:7.0", nil},
		{"pai que não existe: fica na própria", []Lancamento{lanc(4, "-8.00")}, 1, true, false, "4:Órfã:8.0", nil},
		{"o pai só com estorno leva o filho junto para fora", []Lancamento{lanc(3, "-10.00"), lanc(1, "30.00")}, 1, true, false, "", nil},
		{"do maior para o menor", []Lancamento{lanc(1, "-10.00"), lanc(2, "-30.00"), lanc(6, "-20.00")}, 1, true, false, "2:Transporte:30.0|6:Salário:20.0|1:Mercado:10.0", nil},
		{"empate fica na ordem do Id da categoria", []Lancamento{lanc(2, "-10.00"), lanc(1, "-10.00")}, 1, true, false, "1:Mercado:10.0|2:Transporte:10.0", nil},
		{"empate: sem categoria por último", []Lancamento{lanc(0, "-10.00"), lanc(2, "-10.00"), lanc(1, "-10.00")}, 1, true, false, "1:Mercado:10.0|2:Transporte:10.0|-:Sem categoria:10.0", nil},
		{"o pai entra na posição do primeiro filho", []Lancamento{lanc(2, "-10.00"), lanc(3, "-10.00")}, 1, true, false, "2:Transporte:10.0|1:Mercado:10.0", nil},
		{"só gasto de emergência", []Lancamento{lanc(1, "-10.00", emergencia), lanc(1, "-90.00"), lanc(2, "-5.00")}, 1, true, true, "1:Mercado:10.0", nil},
		{"emergência com estorno também marcado", []Lancamento{lanc(1, "-10.00", emergencia), lanc(1, "4.00", emergencia)}, 1, true, true, "1:Mercado:6.0", nil},
		{"sem filtro de emergência, tudo entra", []Lancamento{lanc(1, "-10.00", emergencia), lanc(1, "-90.00")}, 1, true, false, "1:Mercado:100.0", nil},
		{"oito casas", []Lancamento{lanc(1, "-0.12345678")}, 1, true, false, "1:Mercado:0.12345678", nil},
		{"meio centavo não é arredondado", []Lancamento{lanc(1, "-0.005")}, 1, true, false, "1:Mercado:0.005", nil},
		{"soma de meios centavos", []Lancamento{lanc(1, "-0.005"), lanc(1, "-0.005")}, 1, true, false, "1:Mercado:0.01", nil},
		{"o pai soma com o próprio gasto: o nome é o do pai", []Lancamento{lanc(1, "-5.00"), lanc(3, "-10.00"), lanc(5, "-1.00")}, 1, true, false, "1:Mercado:15.0|3:Feira:1.0", nil},
		// Os totais saem em ordem de Id: a categoria 1 sem cadastro não pode parar a leitura das seguintes.
		{"categoria que não existe não interrompe as outras", []Lancamento{lanc(1, "-5.00"), lanc(2, "-10.00"), lanc(6, "-1.00")}, 1, true, false,
			"2:Transporte:10.0|-:Sem categoria:5.0|6:Salário:1.0", func(c map[int64]categoriaLida) { delete(c, 1) }},
		{"sem lançamentos", nil, 1, true, false, "", nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			categorias := categoriasDeTeste()
			if c.categoriasMod != nil {
				c.categoriasMod(categorias)
			}
			obtido := totaisPorCategoria(c.lancamentos, categorias, c.tipo, c.agrupar, c.soEmergencia)
			if resumo(obtido) != c.esperado {
				t.Errorf("totais = %q, esperado %q", resumo(obtido), c.esperado)
			}
			if obtido == nil || comoJSON(t, obtido) == "null" {
				t.Errorf("a lista nunca é null (vazia é []), veio %s", comoJSON(t, obtido))
			}
		})
	}
}

// A cor também vem da categoria que ficou com o total: a do pai, quando o filho foi somado nele; "#94a3b8" sem categoria.
func TestTotaisPorCategoriaCores(t *testing.T) {
	obtido := totaisPorCategoria([]Lancamento{lanc(3, "-10.00"), lanc(0, "-1.00")}, categoriasDeTeste(), 1, true, false)
	if len(obtido) != 2 || obtido[0].Cor != "#111111" || obtido[1].Cor != "#94a3b8" || obtido[1].CategoriaId != nil {
		t.Errorf("totais = %+v, esperado a cor do pai e a de sem categoria", obtido)
	}
}

// O JSON é o do record ValorCategoriaDto do C#: categoriaId, nome, cor, valor, o valor com a escala e sem aspas.
func TestValorCategoriaDtoJSON(t *testing.T) {
	obtido := comoJSON(t, totaisPorCategoria([]Lancamento{lanc(1, "-50.00"), lanc(0, "-1.5")}, categoriasDeTeste(), 1, true, false))
	esperado := `[{"categoriaId":1,"nome":"Mercado","cor":"#111111","valor":50.0},` +
		`{"categoriaId":null,"nome":"Sem categoria","cor":"#94a3b8","valor":1.5}]`
	if obtido != esperado {
		t.Errorf("JSON = %s, esperado %s", obtido, esperado)
	}
}

// Consulta de verdade, no SQLite: só entra o que é relevante, no período, e as categorias vêm do banco (nome e cor da
// categoria, e o pai das subcategorias).
func TestTotaisPorCategoriaNoBanco(t *testing.T) {
	t.Parallel()
	relogio := testeapoio.HojeEmBrasilia(2026, 10, 22)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, relogio)
	conta := fab.Conta()
	mercado := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Mercado"; c.Cor = "#aa0000" })
	feira := fab.Subcategoria(mercado, func(c *testeapoio.Categoria) { c.Nome = "Feira" })
	lazer := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Lazer"; c.Cor = "#00aa00" })
	gasto := func(categoria *testeapoio.Categoria, d tipos.Data, valor string, ajustes ...func(*testeapoio.Transacao)) {
		fab.Transacao(conta, append([]func(*testeapoio.Transacao){func(tr *testeapoio.Transacao) {
			tr.Data = d
			tr.Valor = dec(valor)
			if categoria != nil {
				tr.CategoriaId = &categoria.Id
			}
		}}, ajustes...)...)
	}
	gasto(&mercado, dia(2026, 10, 1), "-40.00")
	gasto(&feira, dia(2026, 10, 31), "-15.50")
	gasto(&feira, dia(2026, 10, 12), "5.00") // estorno
	gasto(&lazer, dia(2026, 10, 15), "-30.00", func(tr *testeapoio.Transacao) { tr.IgnorarRelatorios = true })
	gasto(&lazer, dia(2026, 10, 16), "-30.00", func(tr *testeapoio.Transacao) { tr.Tipo = 3 })
	gasto(&lazer, dia(2026, 9, 30), "-30.00")
	gasto(&lazer, dia(2026, 11, 1), "-30.00")
	gasto(&lazer, dia(2026, 10, 20), "-12.00", func(tr *testeapoio.Transacao) { tr.Status = 2 })
	gasto(nil, dia(2026, 10, 21), "-3.25")
	gasto(&lazer, dia(2026, 10, 22), "100.00", func(tr *testeapoio.Transacao) { tr.Tipo = 2 }) // receita

	s := NovoServico(db, relogio)
	obtido, err := s.TotaisPorCategoria(t.Context(), dia(2026, 10, 1), dia(2026, 10, 31), transacoes.Despesa, true, false)
	if err != nil {
		t.Fatal(err)
	}
	porNome := nomesEValores(obtido)
	if esperado := "Mercado:50.5|Lazer:12.0|Sem categoria:3.25"; porNome != esperado {
		t.Errorf("agrupado no pai = %s, esperado %s", porNome, esperado)
	}
	if obtido[0].CategoriaId == nil || *obtido[0].CategoriaId != mercado.Id || obtido[0].Cor != "#aa0000" {
		t.Errorf("a primeira devia ser o Mercado (%d, #aa0000): %+v", mercado.Id, obtido[0])
	}

	porCategoria, err := s.TotaisPorCategoria(t.Context(), dia(2026, 10, 1), dia(2026, 10, 31), transacoes.Despesa, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if esperado := "Mercado:40.0|Lazer:12.0|Feira:10.5|Sem categoria:3.25"; nomesEValores(porCategoria) != esperado {
		t.Errorf("sem agrupar = %s, esperado %s", nomesEValores(porCategoria), esperado)
	}

	receitas, err := s.TotaisPorCategoria(t.Context(), dia(2026, 10, 1), dia(2026, 10, 31), transacoes.Receita, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if esperado := "Lazer:100.0"; nomesEValores(receitas) != esperado {
		t.Errorf("receitas = %s, esperado %s", nomesEValores(receitas), esperado)
	}

	vazio, err := s.TotaisPorCategoria(t.Context(), dia(2026, 10, 1), dia(2026, 10, 31), transacoes.Despesa, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if vazio == nil || comoJSON(t, vazio) != "[]" {
		t.Errorf("sem emergência marcada: %s, esperado []", comoJSON(t, vazio))
	}
}

func nomesEValores(lista []ValorCategoriaDto) string {
	partes := []string{}
	for _, v := range lista {
		partes = append(partes, v.Nome+":"+v.Valor.String())
	}
	return strings.Join(partes, "|")
}
