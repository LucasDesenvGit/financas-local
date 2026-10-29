package emprestimos

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/banco/consultas"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func dec(texto string) tipos.Decimal { return testeapoio.Valor(texto) }

func comoJSON(t *testing.T, v any) string {
	t.Helper()
	dados, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(dados)
}

// Regra: ParcelaMensal (ADR 0014) converte a parcela para o equivalente mensal: anual ÷ 12, semestral ÷ 6, trimestral ÷ 3,
// o resto como está. Sem parcela, ou com parcela zero ou negativa, é 0. A divisão é a do decimal do C# (até 28 casas, sem
// arredondar); os resultados com muitas casas vêm de uma conta independente (a regra do decimal, feita em Python).
func TestParcelaMensal(t *testing.T) {
	texto := func(s string) *string { return &s }
	valor := func(s string) *tipos.Decimal { v := dec(s); return &v }
	casos := []struct {
		nome          string
		parcela       *tipos.Decimal
		periodicidade *string
		esperado      string
	}{
		{"sem parcela", nil, texto("MONTHLY"), "0"},
		{"parcela zero", valor("0.0"), texto("MONTHLY"), "0"},
		{"parcela negativa", valor("-100.0"), texto("MONTHLY"), "0"},
		{"parcela negativa anual", valor("-1200.0"), texto("YEARLY"), "0"},
		{"meio centavo positivo ainda é parcela", valor("0.005"), texto("MONTHLY"), "0.005"},
		{"mensal fica como está", valor("1500.50"), texto("MONTHLY"), "1500.50"},
		{"sem periodicidade", valor("1500.50"), nil, "1500.50"},
		{"periodicidade vazia", valor("1500.50"), texto(""), "1500.50"},
		{"irregular fica como está", valor("1500.50"), texto("IRREGULAR"), "1500.50"},
		{"minúsculas de mensal", valor("1500.50"), texto("monthly"), "1500.50"},
		{"com espaços não é reconhecida", valor("1200.0"), texto(" YEARLY "), "1200.0"},
		{"anual em inglês", valor("1200.0"), texto("YEARLY"), "100.0"},
		{"anual em português", valor("1200.0"), texto("ANUAL"), "100.0"},
		{"anual em minúsculas", valor("1200.0"), texto("yearly"), "100.0"},
		{"anual com dízima", valor("1000.0"), texto("YEARLY"), "83.33333333333333333333333333"},
		{"semestral em inglês", valor("600.0"), texto("SEMIANNUALLY"), "100.0"},
		{"semestral em português", valor("600.0"), texto("SEMESTRAL"), "100.0"},
		{"semestral com maiúsculas e minúsculas", valor("600.0"), texto("Semestral"), "100.0"},
		{"trimestral em inglês", valor("300.0"), texto("QUARTERLY"), "100.0"},
		{"trimestral em português com dízima", valor("100.0"), texto("TRIMESTRAL"), "33.333333333333333333333333333"},
		{"anual com meio centavo", valor("0.30"), texto("YEARLY"), "0.025"},
		{"oito casas", valor("0.12345678"), texto("MONTHLY"), "0.12345678"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			obtido, err := ParcelaMensal(c.parcela, c.periodicidade)
			if err != nil {
				t.Fatal(err)
			}
			if obtido.String() != c.esperado {
				t.Errorf("ParcelaMensal = %s, esperado %s", obtido, c.esperado)
			}
		})
	}
}

// Regras de Totais (ADR 0014): saldo devedor e parcela mensal por natureza; só entram contratos em aberto (saldo devedor
// maior que zero) de conexões ativas. O saldo devedor é comparado e somado em Go, com decimal: no SQLite ele é texto.
func TestTotais(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.RelogioDosTestes())
	ativa := fab.ConexaoBancaria()
	inativa := fab.ConexaoBancaria(func(c *testeapoio.ConexaoBancaria) { c.Ativa = false })
	contrato := func(conexao testeapoio.ConexaoBancaria, natureza int64, saldo string, parcela *string, periodicidade *string) {
		fab.Emprestimo(conexao, func(e *testeapoio.Emprestimo) {
			e.Natureza = natureza
			e.SaldoDevedor = dec(saldo)
			if parcela != nil {
				e.ValorParcela = new(dec(*parcela))
			}
			e.Periodicidade = periodicidade
		})
	}
	mensal, anual := new("MONTHLY"), new("YEARLY")
	contrato(ativa, 2, "200000.00", new("2500.50"), mensal)
	contrato(ativa, 2, "50000.00", new("12000.00"), anual)  // 1000.00 por mês
	contrato(ativa, 1, "10000.00", new("333.33"), mensal)   //
	contrato(ativa, 1, "0.001", nil, nil)                   // sem parcela; saldo de meio centavo ainda é dívida
	contrato(ativa, 1, "0.0", new("999.00"), mensal)        // quitado: fora
	contrato(ativa, 1, "-5.00", new("999.00"), mensal)      // saldo negativo: fora
	contrato(inativa, 1, "99999.00", new("999.00"), mensal) // conexão desativada: fora
	contrato(inativa, 2, "99999.00", new("999.00"), mensal) // idem
	contrato(ativa, 1, "9.0", new("0.00"), mensal)          // "9.0" > "10.0" como texto: o saldo é somado como número
	contrato(ativa, 1, "10.0", new("-1.00"), mensal)        // parcela negativa conta zero

	totais, err := NovoServico(db).Totais(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// Financiamentos: 200000.0 + 50000.0; parcelas 2500.50 + 1000.0 = 3500.5.
	// Empréstimos: 10000.0 + 0.001 + 9.0 + 10.0; parcelas 333.33 + 0 + 0 + 0.
	esperado := `{"saldoFinanciamentos":250000.0,"saldoEmprestimos":10019.001,` +
		`"parcelaMensalFinanciamentos":3500.5,"parcelaMensalEmprestimos":333.33}`
	if obtido := comoJSON(t, totais); obtido != esperado {
		t.Errorf("totais = %s\nesperado %s", obtido, esperado)
	}
}

// A parcela mensal total é arredondada em duas casas pelo método bancário (Math.Round sem modo); o saldo, não.
func TestTotaisArredondaAParcelaPeloMetodoBancario(t *testing.T) {
	t.Parallel()
	casos := []struct {
		nome     string
		parcelas []string // anuais, em reais
		esperado string
	}{
		{"0,025 vai para 0,02 (o par)", []string{"0.30"}, "0.02"},
		{"0,075 vai para 0,08 (o par)", []string{"0.90"}, "0.08"},
		{"soma de duas dízimas", []string{"1000.00", "1000.00"}, "166.67"},
		{"mais de duas casas sem empate arredonda normal", []string{"1000.00"}, "83.33"},
		{"já com duas casas fica como está", []string{"1200.00", "2400.00"}, "300.0"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db := testeapoio.NovoBancoVazio(t)
			fab := testeapoio.NovaFabrica(t, db, testeapoio.RelogioDosTestes())
			conexao := fab.ConexaoBancaria()
			for _, p := range c.parcelas {
				fab.Emprestimo(conexao, func(e *testeapoio.Emprestimo) {
					e.ValorParcela = new(dec(p))
					e.Periodicidade = new("YEARLY")
				})
			}
			totais, err := NovoServico(db).Totais(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if totais.ParcelaMensalEmprestimos.String() != c.esperado {
				t.Errorf("parcela mensal = %s, esperado %s", totais.ParcelaMensalEmprestimos, c.esperado)
			}
		})
	}
}

// Sem contratos, tudo é zero (o decimal 0 sem casas), e não null nem erro.
func TestTotaisSemContratos(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	totais, err := NovoServico(db).Totais(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	esperado := `{"saldoFinanciamentos":0,"saldoEmprestimos":0,"parcelaMensalFinanciamentos":0,"parcelaMensalEmprestimos":0}`
	if obtido := comoJSON(t, totais); obtido != esperado {
		t.Errorf("totais = %s, esperado %s", obtido, esperado)
	}
}

// Oito casas e meio centavo no saldo: a soma é exata, nada é arredondado.
func TestTotaisSomaOSaldoSemArredondar(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.RelogioDosTestes())
	conexao := fab.ConexaoBancaria()
	for _, saldo := range []string{"0.12345678", "0.005", "0.00000001"} {
		fab.Emprestimo(conexao, func(e *testeapoio.Emprestimo) { e.SaldoDevedor = dec(saldo) })
	}
	totais, err := NovoServico(db).Totais(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if totais.SaldoEmprestimos.String() != "0.12845679" {
		t.Errorf("saldo = %s, esperado 0.12845679", totais.SaldoEmprestimos)
	}
}

// Regra: ContratosAtivos: conexão ativa e saldo devedor maior que zero, na ordem do Id, com os campos que os totais usam.
func TestContratosAtivos(t *testing.T) {
	t.Parallel()
	db := testeapoio.NovoBancoVazio(t)
	fab := testeapoio.NovaFabrica(t, db, testeapoio.RelogioDosTestes())
	ativa := fab.ConexaoBancaria()
	inativa := fab.ConexaoBancaria(func(c *testeapoio.ConexaoBancaria) { c.Ativa = false })
	primeiro := fab.Emprestimo(ativa, func(e *testeapoio.Emprestimo) {
		e.Natureza = 2
		e.ValorParcela = new(dec("2500.50"))
		e.Periodicidade = new("MONTHLY")
	})
	fab.Emprestimo(inativa)
	fab.Emprestimo(ativa, func(e *testeapoio.Emprestimo) { e.SaldoDevedor = dec("0.00") })
	ultimo := fab.Emprestimo(ativa, func(e *testeapoio.Emprestimo) { e.SaldoDevedor = dec("0.01") })

	obtidos, err := NovoServico(db).ContratosAtivos(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(obtidos) != 2 || obtidos[0].Id != primeiro.Id || obtidos[1].Id != ultimo.Id {
		t.Fatalf("contratos = %+v, esperado os de Id %d e %d", obtidos, primeiro.Id, ultimo.Id)
	}
	p := obtidos[0]
	if p.Natureza != 2 || p.SaldoDevedor.String() != "5000.0" || p.ValorParcela == nil || p.ValorParcela.String() != "2500.5" ||
		p.Periodicidade == nil || *p.Periodicidade != "MONTHLY" {
		t.Errorf("campos do primeiro = %+v", p)
	}
	if u := obtidos[1]; u.ValorParcela != nil || u.Periodicidade != nil || u.SaldoDevedor.String() != "0.01" {
		t.Errorf("campos do último = %+v, esperado parcela e periodicidade nulas e saldo 0.01", u)
	}

	vazio, err := NovoServico(testeapoio.NovoBancoVazio(t)).ContratosAtivos(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if vazio == nil || len(vazio) != 0 {
		t.Errorf("sem contratos: %v, esperado lista vazia", vazio)
	}
}

// Dinheiro é TEXT no SQLite: somar, comparar e ordenar no SQL dá resultado errado. O pacote só traz as linhas e faz a
// conta em Go (CLAUDE.md, "Código Go"). O teste falha se o SQL ganhar SUM, AVG, TOTAL, ou comparação numa coluna de dinheiro.
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

// bancoQueFalha é o banco de verdade, mas toda consulta devolve erro.
type bancoQueFalha struct{ consultas.DBTX }

var errDeTeste = errors.New("falha de teste")

func (bancoQueFalha) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errDeTeste
}

// A consulta que falha vira erro com a causa (errors.Is), e Totais não devolve totais pela metade.
func TestConsultaQueFalhaViraErro(t *testing.T) {
	t.Parallel()
	s := NovoServico(bancoQueFalha{testeapoio.NovoBancoVazio(t)})
	if _, err := s.ContratosAtivos(t.Context()); !errors.Is(err, errDeTeste) {
		t.Errorf("ContratosAtivos: %v, esperado o erro do banco embrulhado", err)
	}
	if totais, err := s.Totais(t.Context()); !errors.Is(err, errDeTeste) || totais != (TotaisCreditoDto{}) {
		t.Errorf("Totais: %+v, %v; esperado totais zerados e o erro do banco", totais, err)
	}
}

// Dinheiro que não é do formato do decimal vira erro na leitura, e não um zero calado.
func TestDadoForaDoFormatoViraErro(t *testing.T) {
	t.Parallel()
	casos := []struct{ nome, estraga string }{
		{"saldo devedor", `UPDATE Emprestimo SET SaldoDevedor = 'tudo'`},
		{"valor da parcela", `UPDATE Emprestimo SET ValorParcela = 'metade'`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			db := testeapoio.NovoBancoVazio(t)
			fab := testeapoio.NovaFabrica(t, db, testeapoio.RelogioDosTestes())
			fab.Emprestimo(fab.ConexaoBancaria(), func(e *testeapoio.Emprestimo) { e.ValorParcela = new(dec("100.0")) })
			if _, err := db.ExecContext(t.Context(), c.estraga); err != nil {
				t.Fatal(err)
			}
			if _, err := NovoServico(db).Totais(t.Context()); err == nil {
				t.Error("o dado fora do formato devia dar erro")
			}
		})
	}
}
