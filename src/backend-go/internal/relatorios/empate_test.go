package relatorios

import (
	"strconv"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
)

// Empate com muitas categorias: o OrderByDescending do LINQ é estável (o empate fica na ordem do Id da categoria), e o
// sort.Slice do Go não é. Com mais de uma dúzia de itens o sort comum já troca de lugar os iguais.
func TestTotaisPorCategoriaEmpateComMuitasCategoriasFicaNaOrdemDoId(t *testing.T) {
	categorias := map[int64]categoriaLida{}
	lancamentos := []Lancamento{}
	// Três valores que se repetem entre os Ids: 30.0, 10.0 e 20.0.
	valores := []string{"30.0", "10.0", "20.0"}
	for id := int64(100); id < 160; id++ {
		categorias[id] = categoriaLida{nome: "C" + strconv.FormatInt(id, 10)}
		lancamentos = append(lancamentos, lanc(id, "-"+valores[id%3]))
	}
	var esperado []string
	for _, valor := range []string{"30.0", "20.0", "10.0"} {
		for id := int64(100); id < 160; id++ {
			if valores[id%3] == valor {
				esperado = append(esperado, strconv.FormatInt(id, 10)+":C"+strconv.FormatInt(id, 10)+":"+valor)
			}
		}
	}
	obtido := totaisPorCategoria(lancamentos, categorias, 1, false, false)
	if resumo(obtido) != strings.Join(esperado, "|") {
		t.Errorf("totais = %s\nesperado %s", resumo(obtido), strings.Join(esperado, "|"))
	}
}

// Empate de percentual com muitos orçamentos: o OrderByDescending do LINQ é estável (o empate fica na ordem do Id do
// orçamento), e o sort.Slice do Go não é.
func TestOrcamentosDoMesEmpateComMuitosOrcamentosFicaNaOrdemDoId(t *testing.T) {
	t.Parallel()
	relogio := testeapoio.HojeEmBrasilia(2026, 10, 22)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, relogio)
	conta := fab.Conta()
	// O gasto sobre um limite de 100.0: o percentual é o próprio valor (10, 50 e 30), repetido de três em três.
	gastos := []string{"10.00", "50.00", "30.00"}
	for i := range 60 {
		categoria := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Cat " + strconv.Itoa(1000+i) })
		fab.Orcamento(categoria, func(o *testeapoio.Orcamento) { o.Valor = dec("100.00") })
		despesaDoMes(fab, conta, categoria, dia(2026, 10, 10), "-"+gastos[i%3])
	}
	var esperado []string
	for _, resto := range []int{1, 2, 0} { // 50, 30, 10
		for i := range 60 {
			if i%3 == resto {
				esperado = append(esperado, "Cat "+strconv.Itoa(1000+i))
			}
		}
	}
	obtidos, err := NovoServico(db, relogio).OrcamentosDoMes(t.Context(), 2026, 10)
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for _, o := range obtidos {
		nomes = append(nomes, o.CategoriaNome)
	}
	if strings.Join(nomes, "|") != strings.Join(esperado, "|") {
		t.Errorf("ordem = %s\nesperado %s", strings.Join(nomes, "|"), strings.Join(esperado, "|"))
	}
}
