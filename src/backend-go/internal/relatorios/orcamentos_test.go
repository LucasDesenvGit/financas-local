package relatorios

import (
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// despesaDoMes grava uma despesa na data dada, na categoria dada.
func despesaDoMes(fab *testeapoio.Fabrica, conta testeapoio.Conta, categoria testeapoio.Categoria, d tipos.Data, valor string,
	ajustes ...func(*testeapoio.Transacao)) {
	fab.Transacao(conta, append([]func(*testeapoio.Transacao){func(tr *testeapoio.Transacao) {
		tr.Data = d
		tr.Valor = dec(valor)
		tr.CategoriaId = &categoria.Id
	}}, ajustes...)...)
}

func marcarEmergencia(tr *testeapoio.Transacao) { tr.Emergencia = true }

// Regras de OrcamentosDoMes: gasto do mês contra o limite, percentual com uma casa (arredondamento bancário) e a
// situação (ok, alerta a partir de 80%, emergencia, estourado).
func TestOrcamentosDoMesSituacaoEPercentual(t *testing.T) {
	t.Parallel()
	relogio := testeapoio.HojeEmBrasilia(2026, 10, 22)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, relogio)
	conta := fab.Conta()

	type gasto struct {
		valor      string
		emergencia bool
	}
	casos := []struct {
		nome       string
		limite     string
		gastos     []gasto
		gastoTexto string // o gasto, com a escala
		percentual string
		emergTexto string
		situacao   string
	}{
		{"sem gasto", "500.00", nil, "0", "0", "0", "ok"},
		{"abaixo de 80%", "500.00", []gasto{{"-100.00", false}}, "100.0", "20.0", "0", "ok"},
		{"79,9% ainda é ok", "1000.00", []gasto{{"-799.00", false}}, "799.0", "79.9", "0", "ok"},
		{"exatamente 80% é alerta", "500.00", []gasto{{"-400.00", false}}, "400.0", "80.0", "0", "alerta"},
		{"79,95% arredonda para 80,0 e vira alerta", "500.00", []gasto{{"-399.75", false}}, "399.75", "80.0", "0", "alerta"},
		{"79,94% fica ok", "10000.00", []gasto{{"-7994.00", false}}, "7994.0", "79.9", "0", "ok"},
		// 500.0 ÷ 500.0 é 1 no decimal do C# (a escala natural é 0), e o percentual sai 100, sem casa.
		{"exatamente o limite ainda não estourou", "500.00", []gasto{{"-500.00", false}}, "500.0", "100", "0", "alerta"},
		{"passou do limite", "500.00", []gasto{{"-600.00", false}}, "600.0", "120.0", "0", "estourado"},
		{"um centavo acima do limite", "500.00", []gasto{{"-500.01", false}}, "500.01", "100.0", "0", "estourado"},
		{"passou, mas sem a emergência cabe", "500.00", []gasto{{"-450.00", false}, {"-100.00", true}}, "550.0", "110.0", "100.0", "emergencia"},
		{"sem a emergência ainda passa", "500.00", []gasto{{"-600.00", false}, {"-100.00", true}}, "700.0", "140.0", "100.0", "estourado"},
		{"sem a emergência cai exatamente no limite", "500.00", []gasto{{"-500.00", false}, {"-50.00", true}}, "550.0", "110.0", "50.0", "emergencia"},
		{"emergência dentro do limite não muda a situação", "500.00", []gasto{{"-100.00", false}, {"-50.00", true}}, "150.0", "30.0", "50.0", "ok"},
		{"estorno abate o gasto", "500.00", []gasto{{"-100.00", false}, {"40.00", false}}, "60.0", "12.0", "0", "ok"},
		{"limite zero com gasto: percentual 0 e estourado", "0.00", []gasto{{"-10.00", false}}, "10.0", "0", "0", "estourado"},
		{"limite zero sem gasto", "0.00", nil, "0", "0", "0", "ok"},
		{"arredondamento bancário: 12,25 vai para 12,2", "500.00", []gasto{{"-61.25", false}}, "61.25", "12.2", "0", "ok"},
		{"arredondamento bancário: 12,35 vai para 12,4", "500.00", []gasto{{"-61.75", false}}, "61.75", "12.4", "0", "ok"},
		{"meio centavo no gasto", "1000.00", []gasto{{"-0.005", false}}, "0.005", "0.0", "0", "ok"},
		{"oito casas no gasto", "1000.00", []gasto{{"-0.12345678", false}}, "0.12345678", "0.0", "0", "ok"},
		{"valor do limite com casas", "333.33", []gasto{{"-100.00", false}}, "100.0", "30.0", "0", "ok"},
	}
	for i, c := range casos {
		categoria := fab.Categoria(func(cat *testeapoio.Categoria) { cat.Nome = c.nome })
		fab.Orcamento(categoria, func(o *testeapoio.Orcamento) { o.Valor = dec(c.limite) })
		for j, g := range c.gastos {
			// Dia 1 e dia 31: as duas pontas do mês entram.
			d := dia(2026, 10, 1+30*(j%2))
			if g.emergencia {
				despesaDoMes(fab, conta, categoria, d, g.valor, marcarEmergencia)
			} else {
				despesaDoMes(fab, conta, categoria, d, g.valor)
			}
		}
		_ = i
	}

	s := NovoServico(db, relogio)
	obtidos, err := s.OrcamentosDoMes(t.Context(), 2026, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(obtidos) != len(casos) {
		t.Fatalf("%d orçamentos, esperado %d", len(obtidos), len(casos))
	}
	porNome := map[string]OrcamentoDto{}
	for _, o := range obtidos {
		porNome[o.CategoriaNome] = o
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			o, ok := porNome[c.nome]
			if !ok {
				t.Fatalf("o orçamento %q não veio", c.nome)
			}
			if o.Gasto.String() != c.gastoTexto || o.Percentual.String() != c.percentual ||
				o.GastoEmergencia.String() != c.emergTexto || o.Situacao != c.situacao {
				t.Errorf("gasto %s, percentual %s, emergência %s, situação %s; esperado %s, %s, %s, %s",
					o.Gasto, o.Percentual, o.GastoEmergencia, o.Situacao, c.gastoTexto, c.percentual, c.emergTexto, c.situacao)
			}
		})
	}
}

// Subcategorias: o orçamento do pai conta o gasto das filhas; o da própria subcategoria conta o gasto direto dela; as
// duas coisas ao mesmo tempo funcionam. Também vale para os gastos de emergência.
func TestOrcamentosDoMesComSubcategorias(t *testing.T) {
	t.Parallel()
	relogio := testeapoio.HojeEmBrasilia(2026, 10, 22)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, relogio)
	conta := fab.Conta()
	mercado := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Mercado" })
	feira := fab.Subcategoria(mercado, func(c *testeapoio.Categoria) { c.Nome = "Feira" })
	padaria := fab.Subcategoria(mercado, func(c *testeapoio.Categoria) { c.Nome = "Padaria" })
	fab.Orcamento(mercado, func(o *testeapoio.Orcamento) { o.Valor = dec("1000.00") })
	fab.Orcamento(feira, func(o *testeapoio.Orcamento) { o.Valor = dec("200.00") })
	// A filha só com estorno: o total dela sai da lista agrupada, e o orçamento dela usa o gasto direto (nenhum).
	fab.Orcamento(padaria, func(o *testeapoio.Orcamento) { o.Valor = dec("50.00") })

	despesaDoMes(fab, conta, mercado, dia(2026, 10, 3), "-100.00")
	despesaDoMes(fab, conta, mercado, dia(2026, 10, 7), "-10.00", marcarEmergencia) // emergência do próprio pai
	despesaDoMes(fab, conta, feira, dia(2026, 10, 4), "-150.00", marcarEmergencia)
	despesaDoMes(fab, conta, feira, dia(2026, 10, 5), "-60.00")
	despesaDoMes(fab, conta, padaria, dia(2026, 10, 6), "20.00") // estorno

	obtidos, err := NovoServico(db, relogio).OrcamentosDoMes(t.Context(), 2026, 10)
	if err != nil {
		t.Fatal(err)
	}
	var linhas []string
	for _, o := range obtidos {
		linhas = append(linhas, o.CategoriaNome+" gasto "+o.Gasto.String()+" emergência "+o.GastoEmergencia.String()+" "+o.Situacao)
	}
	esperado := []string{
		// Mercado: 100 + 10 + 150 + 60 + (-20 da padaria) = 300 → 30.0%, com 10 + 150 de emergência: as filhas somam
		// dentro do pai (a padaria só com estorno também).
		"Feira gasto 210.0 emergência 150.0 emergencia",
		"Mercado gasto 300.0 emergência 160.0 ok",
		"Padaria gasto 0 emergência 0 ok",
	}
	if strings.Join(linhas, "\n") != strings.Join(esperado, "\n") {
		t.Errorf("orçamentos:\n%s\nesperado:\n%s", strings.Join(linhas, "\n"), strings.Join(esperado, "\n"))
	}
}

// Ordem: do maior percentual para o menor; empate na ordem do Id do orçamento (o OrderByDescending do LINQ é estável).
func TestOrcamentosDoMesOrdemPorPercentual(t *testing.T) {
	t.Parallel()
	relogio := testeapoio.HojeEmBrasilia(2026, 10, 22)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, relogio)
	conta := fab.Conta()
	// O Id do orçamento segue a ordem de criação: A, B, C, D.
	for _, c := range []struct{ nome, gasto string }{{"A cinquenta", "-50.00"}, {"B noventa", "-90.00"}, {"C cinquenta", "-50.00"}, {"D zero", "0.0"}} {
		categoria := fab.Categoria(func(cat *testeapoio.Categoria) { cat.Nome = c.nome })
		fab.Orcamento(categoria, func(o *testeapoio.Orcamento) { o.Valor = dec("100.00") })
		if c.gasto != "0.0" {
			despesaDoMes(fab, conta, categoria, dia(2026, 10, 10), c.gasto)
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
	if esperado := "B noventa|A cinquenta|C cinquenta|D zero"; strings.Join(nomes, "|") != esperado {
		t.Errorf("ordem = %s, esperado %s", strings.Join(nomes, "|"), esperado)
	}
}

// O que não conta no gasto: transferência, "ignorar nos relatórios", outro mês e receita. Conta: pendente, o dia 1 e o
// último dia do mês (29 de fevereiro num ano bissexto).
func TestOrcamentosDoMesSoContaOQueEhRelevanteDoMes(t *testing.T) {
	t.Parallel()
	relogio := testeapoio.HojeEmBrasilia(2024, 2, 10)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, relogio)
	conta := fab.Conta()
	categoria := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Lazer" })
	fab.Orcamento(categoria, func(o *testeapoio.Orcamento) { o.Ano = 2024; o.Mes = 2; o.Valor = dec("1000.00") })
	fab.Orcamento(categoria, func(o *testeapoio.Orcamento) { o.Ano = 2024; o.Mes = 3; o.Valor = dec("2000.00") })

	despesaDoMes(fab, conta, categoria, dia(2024, 1, 31), "-1.00") // janeiro: fora
	despesaDoMes(fab, conta, categoria, dia(2024, 2, 1), "-2.00")  // primeiro dia: dentro
	despesaDoMes(fab, conta, categoria, dia(2024, 2, 29), "-4.00") // último dia do bissexto: dentro
	despesaDoMes(fab, conta, categoria, dia(2024, 3, 1), "-8.00")  // março: fora
	despesaDoMes(fab, conta, categoria, dia(2024, 2, 15), "-16.00", func(tr *testeapoio.Transacao) { tr.Status = 2 })
	despesaDoMes(fab, conta, categoria, dia(2024, 2, 15), "-32.00", func(tr *testeapoio.Transacao) { tr.IgnorarRelatorios = true })
	despesaDoMes(fab, conta, categoria, dia(2024, 2, 15), "-64.00", func(tr *testeapoio.Transacao) { tr.Tipo = 3 })
	despesaDoMes(fab, conta, categoria, dia(2024, 2, 15), "128.00", func(tr *testeapoio.Transacao) { tr.Tipo = 2 }) // receita

	s := NovoServico(db, relogio)
	fevereiro, err := s.OrcamentosDoMes(t.Context(), 2024, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(fevereiro) != 1 || fevereiro[0].Gasto.String() != "22.0" || fevereiro[0].Ano != 2024 || fevereiro[0].Mes != 2 {
		t.Errorf("fevereiro = %+v, esperado 1 orçamento com gasto 22.0 (2 + 4 + 16)", fevereiro)
	}
	marco, err := s.OrcamentosDoMes(t.Context(), 2024, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(marco) != 1 || marco[0].Gasto.String() != "8.0" || marco[0].Valor.String() != "2000.0" {
		t.Errorf("março = %+v, esperado gasto 8.0 de 2000.0", marco)
	}
}

// Sem orçamento no mês: a lista vazia ([]), e não null; o orçamento de outro mês ou ano não aparece.
func TestOrcamentosDoMesSemOrcamento(t *testing.T) {
	t.Parallel()
	relogio := testeapoio.HojeEmBrasilia(2026, 10, 22)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, relogio)
	categoria := fab.Categoria()
	fab.Orcamento(categoria, func(o *testeapoio.Orcamento) { o.Ano = 2026; o.Mes = 9 })
	fab.Orcamento(categoria, func(o *testeapoio.Orcamento) { o.Ano = 2025; o.Mes = 10 })

	s := NovoServico(db, relogio)
	// O mês 13 e o ano 0 não existem; o C# também só responde a lista vazia, sem chegar a montar as datas.
	for _, p := range []struct{ ano, mes int }{{2026, 10}, {2026, 13}, {0, 0}} {
		obtidos, err := s.OrcamentosDoMes(t.Context(), p.ano, p.mes)
		if err != nil {
			t.Fatal(err)
		}
		if obtidos == nil || comoJSON(t, obtidos) != "[]" {
			t.Errorf("%d/%d: %s, esperado []", p.mes, p.ano, comoJSON(t, obtidos))
		}
	}
}

// O JSON é o do record OrcamentoDto do C#, na ordem dos campos: o valor com a escala e sem aspas, ano e mês como número.
func TestOrcamentoDtoJSON(t *testing.T) {
	t.Parallel()
	relogio := testeapoio.HojeEmBrasilia(2026, 10, 22)
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, relogio)
	conta := fab.Conta()
	categoria := fab.Categoria(func(c *testeapoio.Categoria) { c.Nome = "Mercado"; c.Cor = "#aa0000" })
	fab.Orcamento(categoria, func(o *testeapoio.Orcamento) { o.Valor = dec("500.00") })
	despesaDoMes(fab, conta, categoria, dia(2026, 10, 2), "-100.00")

	obtidos, err := NovoServico(db, relogio).OrcamentosDoMes(t.Context(), 2026, 10)
	if err != nil {
		t.Fatal(err)
	}
	esperado := `[{"id":1,"categoriaId":1,"categoriaNome":"Mercado","categoriaCor":"#aa0000","ano":2026,"mes":10,` +
		`"valor":500.0,"gasto":100.0,"percentual":20.0,"gastoEmergencia":0,"situacao":"ok"}]`
	if obtido := comoJSON(t, obtidos); obtido != esperado {
		t.Errorf("JSON = %s, esperado %s", obtido, esperado)
	}
}
