package leitor

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/modulos/trabalho/leitor/ficticios"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

// ler gera o PDF, lê e interpreta, como o site faz.
func ler(t *testing.T, paginas [][]ficticios.Texto) Lido {
	t.Helper()
	linhas, err := LerPDF(ficticios.GerarPDF(paginas, ""), "")
	if err != nil {
		t.Fatalf("LerPDF: %v", err)
	}
	lido, err := Interpretar(linhas)
	if err != nil {
		t.Fatalf("Interpretar: %v", err)
	}
	return lido
}

func emTexto(p *tipos.Decimal) string {
	if p == nil {
		return "-"
	}
	return p.Decimal().StringFixed(2)
}

// rubricas resume as rubricas em texto, uma por linha: "código|descrição|referência|valor|P ou D".
func rubricas(l Lido) string {
	var b strings.Builder
	for _, r := range l.Rubricas {
		lado := "D"
		if r.Provento {
			lado = "P"
		}
		fmt.Fprintf(&b, "%s|%s|%s|%s|%s\n", r.Codigo, r.Descricao, r.Referencia, r.Valor.Decimal().StringFixed(2), lado)
	}
	return b.String()
}

const rubricasDaViaClassica = "001|SALARIO|30,00|5000.00|P\n101|HORAS EXTRAS 50%|10,00|340.91|P\n102|DSR S/ HORAS EXTRAS||68.18|P\n" +
	"998|INSS|12,02|601.22|D\n999|IRRF|22,50|437.45|D\n210|VALE TRANSPORTE 6%|6,00|300.00|D\n305|ASSIST. MEDICA||150.00|D\n"

func TestInterpretarOsQuatroEstilos(t *testing.T) {
	casos := []struct {
		nome       string
		paginas    [][]ficticios.Texto
		empregador string
		cnpj       string
		mes        string
		rubricas   string
		rodape     string // vencimentos descontos líquido salário baseInss baseFgts fgts baseIrrf
	}{
		{"clássico", [][]ficticios.Texto{ficticios.ViaClassica()}, "EMPRESA FICTÍCIA DE TESTES LTDA", "12345678000195", "2026-10-01",
			rubricasDaViaClassica, "5409.09 1488.67 3920.42 5000.00 5409.09 5409.09 432.73 4807.87"},
		{"duas vias na página", [][]ficticios.Texto{ficticios.DuasVias()}, "EMPRESA FICTÍCIA DE TESTES LTDA", "12345678000195", "2026-10-01",
			rubricasDaViaClassica, "5409.09 1488.67 3920.42 5000.00 5409.09 5409.09 432.73 4807.87"},
		{"proventos e qtde", [][]ficticios.Texto{ficticios.ProventosEQtde()}, "COMÉRCIO EXEMPLO S.A.", "98765432000110", "2026-10-01",
			"|Salário Mensal|220:00|4000.00|P\n|Adicional Noturno 20%|20:00|109.09|P\n|Repouso Remunerado||21.82|P\n" +
				"|Contrib. Previdenciária||351.90|D\n|Imposto de Renda||32.47|D\n|Desc. VT||240.00|D\n|Vale Refeição||80.00|D\n" +
				"|Pensão Alimentícia||400.00|D\n|Adto. Salarial||1200.00|D\n",
			"4130.91 2304.37 1826.54 4000.00 4130.91 4130.91 330.47 3379.01"},
		{"créditos e débitos", ficticios.CreditosEDebitos(), "INDÚSTRIA DE TESTE EIRELI", "11222333000181", "2026-10-01",
			"0010|Salário|30d|3200.00|P\n0250|H.E. 100%|5:00|145.45|P\n0251|Reflexo DSR H.E.||29.09|P\n" +
				"0600|I.N.S.S.|8,63%|289.74|D\n0610|Seguro de Vida||12.50|D\n0620|Plano Odontológico||25.90|D\n",
			"3374.54 328.14 3046.40 - - - 269.96 -"},
		{"eventos e reais", [][]ficticios.Texto{ficticios.EventosEReais()}, "SERVIÇOS DIGITAIS DE EXEMPLO ME", "55444333000122", "2026-10-01",
			"1000|SALARIO BASE|30,00|8000.00|P\n1050|GRATIFICACAO||800.00|P\n1100|PLR||1500.00|P\n" +
				"5001|INSS FOLHA|14,00|988.07|D\n5002|IRRF FOLHA|27,50|1623.18|D\n5100|PREVIDENCIA PRIVADA||400.00|D\n" +
				"5200|PLANO DE SAUDE||320.00|D\n5300|ADIANTAMENTO QUINZENAL||3200.00|D\n5400|CONTRIB. SINDICAL||45.00|D\n",
			"10300.00 6576.25 3723.75 - 8800.00 8800.00 704.00 7811.93"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			l := ler(t, c.paginas)
			if l.Empregador != c.empregador || l.Cnpj == nil || *l.Cnpj != c.cnpj || l.Mes == nil || l.Mes.String() != c.mes {
				t.Errorf("cabeçalho: %q %v %v, quero %q %s %s", l.Empregador, l.Cnpj, l.Mes, c.empregador, c.cnpj, c.mes)
			}
			if got := rubricas(l); got != c.rubricas {
				t.Errorf("rubricas:\n%s\nquero:\n%s", got, c.rubricas)
			}
			rodape := strings.Join([]string{emTexto(l.TotalVencimentos), emTexto(l.TotalDescontos), emTexto(l.Liquido), emTexto(l.SalarioBase),
				emTexto(l.BaseInss), emTexto(l.BaseFgts), emTexto(l.FgtsDoMes), emTexto(l.BaseIrrf)}, " ")
			if rodape != c.rodape {
				t.Errorf("rodapé: %s, quero %s", rodape, c.rodape)
			}
			if len(l.Provas) != 3 {
				t.Fatalf("provas: %d, quero 3", len(l.Provas))
			}
			for _, p := range l.Provas {
				if !p.Bateu || !p.Diferenca.Decimal().IsZero() || !p.Impresso.Decimal().Equal(p.Encontrado.Decimal()) {
					t.Errorf("a prova %q não bateu: %+v", p.Descricao, p)
				}
			}
		})
	}
}

// Quando o leitor erra uma linha, a prova diz qual total não bateu e por quanto (WCAG 3.3.1).
func TestProvaMostraQualTotalNaoBateu(t *testing.T) {
	pg := ficticios.ViaClassica()
	for i := range pg {
		switch pg[i].Texto {
		case "68,18":
			pg[i].Texto = "DSR" // o valor sumiu: a linha deixa de ser rubrica
		case "150,00":
			pg[i].Texto = "160,00"
		}
	}
	l := ler(t, [][]ficticios.Texto{pg})
	var got []string
	for _, p := range l.Provas {
		got = append(got, fmt.Sprintf("%s %t %s %s %s", p.Descricao, p.Bateu, p.Impresso.Decimal().StringFixed(2),
			p.Encontrado.Decimal().StringFixed(2), p.Diferenca.Decimal().StringFixed(2)))
	}
	want := "Total de vencimentos false 5409.09 5340.91 -68.18; Total de descontos false 1488.67 1498.67 10.00; " +
		"Líquido false 3920.42 3842.24 -78.18"
	if strings.Join(got, "; ") != want {
		t.Errorf("provas: %s\nquero %s", strings.Join(got, "; "), want)
	}
}

func TestInterpretarSemTabela(t *testing.T) {
	linhas, err := LerPDF(ficticios.GerarPDF([][]ficticios.Texto{ficticios.Pagina(ficticios.Linha(0, 30, "Informe de rendimentos 2026", 300, "Vencimentos"))}, ""), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Interpretar(linhas); !errors.Is(err, ErrSemTabela) {
		t.Errorf("err = %v, quero ErrSemTabela", err)
	}
}

// Sem totais nem líquido, o rodapé acaba a tabela, e não há prova (nada impresso para conferir).
func TestSemTotaisORodapeAcabaATabela(t *testing.T) {
	l := ler(t, [][]ficticios.Texto{ficticios.Pagina(
		ficticios.Linha(0, 30, "ACME FICTÍCIA LTDA", 300, "CNPJ 10.000.000/0001-00"),
		ficticios.Linha(0, 30, "Descrição", 290, "Proventos", 400, "Descontos"),
		ficticios.Linha(0, 30, "Salário", 295, "2.000,00"),
		ficticios.Linha(0, 30, "Linha sem valor"),
		ficticios.Linha(0, 30, "Zerada", 295, "0,00"),
		ficticios.Linha(0, 295, "1,00"),
		ficticios.Linha(0, 30, "Base INSS", 120, "2.000,00"),
		ficticios.Linha(0, 30, "Outro", 295, "9,99"),
	)})
	if got := rubricas(l); got != "|Salário||2000.00|P\n" {
		t.Errorf("rubricas: %q", got)
	}
	if emTexto(l.BaseInss) != "2000.00" || len(l.Provas) != 0 || l.Mes != nil || l.Empregador != "ACME FICTÍCIA LTDA" {
		t.Errorf("base %s, provas %d, mês %v, empregador %q", emTexto(l.BaseInss), len(l.Provas), l.Mes, l.Empregador)
	}
}

// O total sem rótulo para cada coluna ("TOTAL 4.130,91 2.304,37") e o primeiro valor de cada campo é o que vale.
func TestTotalSemRotuloPorColuna(t *testing.T) {
	l := ler(t, [][]ficticios.Texto{ficticios.Pagina(
		ficticios.Linha(0, 30, "Descrição", 290, "Proventos", 400, "Descontos"),
		ficticios.Linha(0, 30, "Salário", 295, "2.000,00"),
		ficticios.Linha(0, 30, "INSS", 400, "150,00"),
		ficticios.Linha(0, 30, "TOTAL", 295, "2.000,00", 400, "150,00"),
		ficticios.Linha(0, 30, "TOTAL", 295, "9.999,00", 400, "999,00"),
		ficticios.Linha(0, 30, "Líquido", 120, "R$", 150, "1.850,00"),
		ficticios.Linha(0, 30, "Líquido", 120, "1,00"),
	)})
	if emTexto(l.TotalVencimentos) != "2000.00" || emTexto(l.TotalDescontos) != "150.00" || emTexto(l.Liquido) != "1850.00" {
		t.Errorf("totais %s %s %s", emTexto(l.TotalVencimentos), emTexto(l.TotalDescontos), emTexto(l.Liquido))
	}
}

// O valor do rótulo: na mesma célula, à direita (passando pela seta, pelo "R$" e pela faixa) ou embaixo, nunca o de
// outro rótulo, nem um número longe dele na linha de baixo.
func TestValorDoRotulo(t *testing.T) {
	cel := func(x Pontos, t string) Celula { return Celula{Texto: t, X0: x, X1: x + Pontos(len([]rune(t)))*5.4} }
	casos := []struct {
		nome   string
		linhas []Linha
		want   string
	}{
		{"mesma célula", []Linha{{Celulas: []Celula{cel(30, "Líquido 1.850,00")}}}, "1850.00"},
		{"à direita", []Linha{{Celulas: []Celula{cel(30, "Líquido"), cel(100, "->"), cel(120, "7,5%"), cel(150, "1.850,00")}}}, "1850.00"},
		{"outro rótulo no caminho", []Linha{{Celulas: []Celula{cel(30, "Líquido"), cel(100, "Base INSS"), cel(200, "1.850,00")}}}, "-"},
		{"embaixo", []Linha{{Celulas: []Celula{cel(30, "Líquido")}}, {Celulas: []Celula{cel(35, "1.850,00")}}}, "1850.00"},
		{"embaixo, pela esquerda", []Linha{{Celulas: []Celula{cel(100, "Líquido")}}, {Celulas: []Celula{cel(50, "1.850,00")}}}, "1850.00"},
		{"embaixo, longe à esquerda", []Linha{{Celulas: []Celula{cel(100, "Líquido")}}, {Celulas: []Celula{cel(40, "1.850,00")}}}, "-"},
		{"embaixo, longe à direita", []Linha{{Celulas: []Celula{cel(100, "Líquido")}}, {Celulas: []Celula{cel(148, "1.850,00")}}}, "-"},
		{"embaixo, quase à direita", []Linha{{Celulas: []Celula{cel(100, "Líquido")}}, {Celulas: []Celula{cel(147, "1.850,00")}}}, "1850.00"},
		{"embaixo sem dinheiro", []Linha{{Celulas: []Celula{cel(30, "Líquido")}}, {Celulas: []Celula{cel(30, "texto")}}}, "-"},
	}
	for _, c := range casos {
		v, ok := valorDoRotulo(c.linhas, 0, 0)
		got := "-"
		if ok {
			got = v.Decimal().StringFixed(2)
		}
		if got != c.want {
			t.Errorf("%s: %s, quero %s", c.nome, got, c.want)
		}
	}
}

func TestLerDinheiro(t *testing.T) {
	casos := []struct {
		s     string
		valor string
		ok    bool
	}{
		{"1.234,56", "1234.56", true}, {"R$ 1.234,56", "1234.56", true}, {"R$1.234,56", "1234.56", true},
		{"289,74-", "289.74", true}, {"0,50", "0.50", true}, {" 12,00 ", "12.00", true}, {"1234,56", "1234.56", true},
		{"1.234.567,89", "1234567.89", true},
		{"1234", "", false}, {"12,5", "", false}, {"1.23,45", "", false}, {"abc", "", false}, {"12.345", "", false},
		{"-12,00", "", false}, {"12,000", "", false}, {"x12,00", "", false},
	}
	for _, c := range casos {
		v, ok := LerDinheiro(c.s)
		if ok != c.ok || (ok && v.Decimal().StringFixed(2) != c.valor) {
			t.Errorf("LerDinheiro(%q) = %s %t, quero %s %t", c.s, v.Decimal().StringFixed(2), ok, c.valor, c.ok)
		}
	}
}

func TestCompetencia(t *testing.T) {
	casos := map[string]string{
		"Outubro/2026": "2026-10-01", "outubro de 2026": "2026-10-01", "MARÇO 2026": "2026-03-01", "Mar-2026": "",
		"marco/2025": "2025-03-01", "Mês/Ano: 10/2026": "2026-10-01", "10/2026": "2026-10-01", "01/10/2026": "",
		"13/2026": "", "Admissão 05/03/2020": "", "10/20266": "", "dezembro2026": "2026-12-01", "x01/2026": "2026-01-01",
	}
	for s, want := range casos {
		got := ""
		if d := competencia(s); d != nil {
			got = d.String()
		}
		if got != want {
			t.Errorf("competencia(%q) = %q, quero %q", s, got, want)
		}
	}
}

func TestEmpregador(t *testing.T) {
	casos := []struct {
		celulas []string
		want    string
	}{
		{[]string{"CNPJ: 12.345.678/0001-95", "Outubro/2026"}, ""},
		{[]string{"Empregador: ACME LTDA", "CNPJ 12.345.678/0001-95"}, "ACME LTDA"},
		{[]string{"ACME LTDA - CNPJ: 12.345.678/0001-95"}, "ACME LTDA"},
		{[]string{"Razão social: BETA S/A"}, "BETA S/A"},
		{[]string{"Recibo de pagamento", "GAMA ME"}, "GAMA ME"},
		{[]string{"AB", "ABCD", "ABC"}, "ABCD"},
		{[]string{"A B C D E", "ABCDE"}, "A B C D E"},
	}
	for _, c := range casos {
		var l Linha
		for _, t := range c.celulas {
			l.Celulas = append(l.Celulas, Celula{Texto: t})
		}
		if got := maisLetras(l); got != c.want {
			t.Errorf("maisLetras(%v) = %q, quero %q", c.celulas, got, c.want)
		}
	}
}
