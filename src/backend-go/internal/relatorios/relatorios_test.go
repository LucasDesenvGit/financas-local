package relatorios

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func dec(texto string) tipos.Decimal { return testeapoio.Valor(texto) }

func dia(ano int, mes time.Month, d int) tipos.Data { return testeapoio.Dia(ano, mes, d) }

// textoDe escreve uma lista de decimais como o JSON a escreve, separados por barra: a escala também é conferida.
func textoDe(valores []tipos.Decimal) string {
	partes := make([]string, len(valores))
	for i, v := range valores {
		partes[i] = v.String()
	}
	return strings.Join(partes, "|")
}

func comoJSON(t *testing.T, v any) string {
	t.Helper()
	dados, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(dados)
}

// Regra: a soma do ef_sum (#447). O EF Core somava no SQL com o ef_sum do Microsoft.Data.Sqlite, que soma decimal exato
// e devolve o resultado como texto "0.0###...": a escala da soma é a do texto (pelo menos uma casa, sem zero à direita).
// Sem linhas, o SumAsync dava null e o C# usava 0m (sem casas).
func TestSomaDoEFCore(t *testing.T) {
	casos := []struct {
		nome      string
		valores   []string
		esperado  string
		temLinhas bool
	}{
		{"sem linhas é zero sem casas", nil, "0", false},
		{"uma linha", []string{"-12.50"}, "-12.5", true},
		{"zero à direita sai da soma", []string{"10.25", "10.75"}, "21.0", true},
		{"inteiro ganha uma casa", []string{"10", "5"}, "15.0", true},
		{"negativo com positivo", []string{"-100.00", "30.00"}, "-70.0", true},
		{"soma zero", []string{"-0.125", "0.125"}, "0.0", true},
		{"um valor zero", []string{"0.0"}, "0.0", true},
		{"oito casas", []string{"0.12345678", "0.00000001"}, "0.12345679", true},
		{"meio centavo não é arredondado", []string{"0.005", "0.005"}, "0.01", true},
		{"meio centavo sozinho", []string{"-0.005"}, "-0.005", true},
		{"muitas linhas", []string{"1.1", "2.2", "3.3", "4.4"}, "11.0", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			valores := []tipos.Decimal{}
			for _, v := range c.valores {
				valores = append(valores, dec(v))
			}
			soma, temLinhas := somaDoEFCore(valores)
			if soma.String() != c.esperado || temLinhas != c.temLinhas {
				t.Errorf("somaDoEFCore(%v) = %s, %v; esperado %s, %v", c.valores, soma, temLinhas, c.esperado, c.temLinhas)
			}
			if obtido := somaOuZero(valores); obtido.String() != c.esperado {
				t.Errorf("somaOuZero(%v) = %s, esperado %s", c.valores, obtido, c.esperado)
			}
		})
	}
}

// Regra: Math.Max(decimal, decimal) do C#: com os dois iguais, o primeiro (Math.Max(0m, 0.0m) é 0, sem casas).
func TestMaiorDeDois(t *testing.T) {
	casos := []struct {
		nome     string
		a, b     string
		esperado string
	}{
		{"o primeiro é maior", "5.50", "2.0", "5.50"},
		{"o segundo é maior", "0", "7.25", "7.25"},
		{"negativo contra zero", "0", "-5.0", "0"},
		{"iguais com escalas diferentes: o primeiro", "0", "0.0", "0"},
		{"iguais com escalas diferentes: o primeiro de novo", "0.0", "0", "0.0"},
		{"meio centavo", "0", "0.005", "0.005"},
		{"meio centavo negativo", "0", "-0.005", "0"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if obtido := maiorDeDois(dec(c.a), dec(c.b)); obtido.String() != c.esperado {
				t.Errorf("maiorDeDois(%s, %s) = %s, esperado %s", c.a, c.b, obtido, c.esperado)
			}
		})
	}
}

// Regra: -x no decimal do C# troca o sinal e guarda a escala.
func TestNegar(t *testing.T) {
	casos := []struct{ entrada, esperado string }{
		{"12.50", "-12.50"}, {"-12.50", "12.50"}, {"0", "0"}, {"0.0", "0.0"}, {"0.00000001", "-0.00000001"}, {"-3.0", "3.0"},
	}
	for _, c := range casos {
		if obtido := negar(dec(c.entrada)); obtido.String() != c.esperado {
			t.Errorf("negar(%s) = %s, esperado %s", c.entrada, obtido, c.esperado)
		}
	}
}

// Dinheiro é TEXT no SQLite: somar, comparar e ordenar no SQL dá resultado errado ("9.0" > "10.0" como texto) e perde
// casas no REAL. Este pacote só traz as linhas e faz a conta em Go (CLAUDE.md, "Código Go"). O teste lê o código-fonte
// do pacote e falha se o SQL ganhar SUM, AVG, TOTAL, ou ORDER BY/comparação numa coluna de dinheiro.
func TestNenhumaContaDeDinheiroNoSQL(t *testing.T) {
	arquivos, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	proibidos := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(SUM|AVG|TOTAL|CAST)\s*\(`),
		regexp.MustCompile(`(?i)ORDER BY[^` + "`" + `]*"(Valor|SaldoDevedor|ValorParcela)"`),
		regexp.MustCompile(`(?i)"(Valor|SaldoDevedor|ValorParcela)"\s*(<|>|<=|>=|BETWEEN)`),
	}
	conferidos := 0
	for _, arquivo := range arquivos {
		if strings.HasSuffix(arquivo, "_test.go") {
			continue
		}
		fonte, err := os.ReadFile(arquivo)
		if err != nil {
			t.Fatal(err)
		}
		conferidos++
		for _, p := range proibidos {
			if achado := p.FindString(string(fonte)); achado != "" {
				t.Errorf("%s tem %q: dinheiro não se soma, ordena nem compara no SQL", arquivo, achado)
			}
		}
	}
	if conferidos == 0 {
		t.Fatal("nenhum arquivo do pacote foi conferido")
	}
}

// Regra: Relevantes (ADR 0031 e 0036): as transações entre as duas datas (as duas valem), sem transferências (Tipo 3) e
// sem as marcadas para ignorar nos relatórios. Pendente entra. Numa lista que nunca é null.
func TestRelevantes(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2026, 10, 22))
	conta := fab.Conta()
	categoria := fab.Categoria()
	em := func(d tipos.Data, ajustes ...func(*testeapoio.Transacao)) testeapoio.Transacao {
		return fab.Transacao(conta, append([]func(*testeapoio.Transacao){func(tr *testeapoio.Transacao) { tr.Data = d }}, ajustes...)...)
	}
	antes := em(dia(2026, 9, 30))
	primeiro := em(dia(2026, 10, 1))
	ultimo := em(dia(2026, 10, 31), func(tr *testeapoio.Transacao) {
		tr.Valor = dec("2500.00")
		tr.Tipo = 2
		tr.CategoriaId = &categoria.Id
		tr.Emergencia = true
		tr.ParcelaTotal = new(int64(3))
	})
	depois := em(dia(2026, 11, 1))
	ignorada := em(dia(2026, 10, 15), func(tr *testeapoio.Transacao) { tr.IgnorarRelatorios = true })
	transferencia := em(dia(2026, 10, 15), func(tr *testeapoio.Transacao) { tr.Tipo = 3 })
	pendente := em(dia(2026, 10, 10), func(tr *testeapoio.Transacao) { tr.Status = 2; tr.Valor = dec("-0.005") })
	estorno := em(dia(2026, 10, 20), func(tr *testeapoio.Transacao) { tr.Valor = dec("30.00") }) // despesa positiva

	s := NovoServico(db, testeapoio.HojeEmBrasilia(2026, 10, 22))
	obtidos, err := s.Relevantes(t.Context(), dia(2026, 10, 1), dia(2026, 10, 31))
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, l := range obtidos {
		ids = append(ids, l.Id)
	}
	esperados := []int64{primeiro.Id, ultimo.Id, pendente.Id, estorno.Id}
	if len(ids) != len(esperados) {
		t.Fatalf("relevantes = %v, esperado %v (antes %d, depois %d, ignorada %d, transferência %d ficam de fora)",
			ids, esperados, antes.Id, depois.Id, ignorada.Id, transferencia.Id)
	}
	for i := range ids {
		if ids[i] != esperados[i] {
			t.Errorf("relevantes = %v, esperado %v, na ordem do Id", ids, esperados)
			break
		}
	}

	// As colunas chegam como foram gravadas.
	l := obtidos[1]
	if l.ContaId != conta.Id || !l.Data.Igual(dia(2026, 10, 31)) || l.Valor.String() != "2500.0" || l.Tipo != 2 ||
		l.CategoriaId == nil || *l.CategoriaId != categoria.Id || l.ParcelaTotal == nil || *l.ParcelaTotal != 3 || !l.Emergencia {
		t.Errorf("lançamento lido = %+v", l)
	}
	if semOpcionais := obtidos[0]; semOpcionais.CategoriaId != nil || semOpcionais.ParcelaTotal != nil || semOpcionais.Emergencia {
		t.Errorf("campos opcionais vazios chegaram como %+v, esperado nil, nil, false", semOpcionais)
	}
	if obtidos[2].Valor.String() != "-0.005" {
		t.Errorf("meio centavo = %s, esperado -0.005 (nada é arredondado)", obtidos[2].Valor)
	}

	// Período sem nada, e período invertido: lista vazia, e não null.
	for _, p := range []struct{ de, ate tipos.Data }{{dia(2025, 1, 1), dia(2025, 1, 31)}, {dia(2026, 10, 31), dia(2026, 10, 1)}} {
		vazio, err := s.Relevantes(t.Context(), p.de, p.ate)
		if err != nil {
			t.Fatal(err)
		}
		if vazio == nil || len(vazio) != 0 || comoJSON(t, vazio) != "[]" {
			t.Errorf("de %s a %s: %s, esperado []", p.de, p.ate, comoJSON(t, vazio))
		}
	}
}

// A data é texto no SQLite: a comparação funciona na virada de ano e em ano bissexto (29/02 existe, e entra no fim de
// fevereiro).
func TestRelevantesNaViradaDeAnoEEmFevereiroBissexto(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.HojeEmBrasilia(2024, 3, 10))
	conta := fab.Conta()
	datas := []tipos.Data{dia(2023, 12, 31), dia(2024, 1, 1), dia(2024, 2, 28), dia(2024, 2, 29), dia(2024, 3, 1)}
	for _, d := range datas {
		fab.Transacao(conta, func(tr *testeapoio.Transacao) { tr.Data = d })
	}
	s := NovoServico(db, testeapoio.HojeEmBrasilia(2024, 3, 10))
	casos := []struct {
		nome    string
		de, ate tipos.Data
		datas   string
	}{
		{"virada de ano", dia(2023, 12, 31), dia(2024, 1, 1), "2023-12-31,2024-01-01"},
		{"fevereiro bissexto inteiro", dia(2024, 2, 1), dia(2024, 2, 29), "2024-02-28,2024-02-29"},
		{"o fim é o dia 28", dia(2024, 2, 1), dia(2024, 2, 28), "2024-02-28"},
		{"o começo é o dia 29", dia(2024, 2, 29), dia(2024, 3, 31), "2024-02-29,2024-03-01"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			obtidos, err := s.Relevantes(t.Context(), c.de, c.ate)
			if err != nil {
				t.Fatal(err)
			}
			var d []string
			for _, l := range obtidos {
				d = append(d, l.Data.String())
			}
			if strings.Join(d, ",") != c.datas {
				t.Errorf("de %s a %s: %v, esperado %s", c.de, c.ate, d, c.datas)
			}
		})
	}
}
