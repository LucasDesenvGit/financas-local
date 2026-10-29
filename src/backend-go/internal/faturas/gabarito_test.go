package faturas

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// gabarito é o testdata/faturas.json, gravado pela sonda testdata/sonda-csharp com o ContaService da própria API:
// as entradas (faturas importadas, lançamentos e pagamentos) e o que o C# respondeu para cada combinação.
type gabarito struct {
	Conjuntos []struct {
		Nome    string   `json:"nome"`
		Faturas []Fatura `json:"faturas"`
	} `json:"conjuntos"`
	CiclosBrutos []json.RawMessage `json:"ciclos"`
	Cobrancas    []struct {
		Conjunto      string                `json:"conjunto"`
		Faturas       []Fatura              `json:"faturas"`
		DiaFechamento *int64                `json:"diaFechamento"`
		Meses         map[string]tipos.Data `json:"meses"`
	} `json:"cobrancas"`
	Lancamentos []Lancamento `json:"lancamentos"`
	Pagamentos  []Pagamento  `json:"pagamentos"`
	Pagos       []struct {
		Vencimentos []tipos.Data `json:"vencimentos"`
		Vencimento  tipos.Data   `json:"vencimento"`
		Pago        string       `json:"pago"`
	} `json:"pagos"`
	PagosQueValem []struct {
		Informado *string `json:"informado"`
		Lancado   string  `json:"lancado"`
		Pago      *string `json:"pago"`
	} `json:"pagosQueValem"`
	Situacoes []struct {
		Total      string      `json:"total"`
		Pago       *string     `json:"pago"`
		Fechamento *tipos.Data `json:"fechamento"`
		Vencimento tipos.Data  `json:"vencimento"`
		Situacao   string      `json:"situacao"`
	} `json:"situacoes"`
}

// ciclo é uma linha de "ciclos": a entrada e o ciclo que o C# calculou.
type ciclo struct {
	Faturas       string      `json:"faturas"`
	Ano           int         `json:"ano"`
	Mes           int         `json:"mes"`
	DiaFechamento *int64      `json:"diaFechamento"`
	DiaVencimento *int64      `json:"diaVencimento"`
	Inicio        tipos.Data  `json:"inicio"`
	Fim           tipos.Data  `json:"fim"`
	Fechamento    *tipos.Data `json:"fechamento"`
	Vencimento    *tipos.Data `json:"vencimento"`
	Fatura        *int64      `json:"fatura"`
}

func lerGabarito(t *testing.T) (gabarito, []ciclo) {
	t.Helper()
	dados, err := os.ReadFile("testdata/faturas.json")
	if err != nil {
		t.Fatal(err)
	}
	var g gabarito
	if err := json.Unmarshal(dados, &g); err != nil {
		t.Fatal(err)
	}
	ciclos := make([]ciclo, len(g.CiclosBrutos))
	for i, bruto := range g.CiclosBrutos {
		if err := json.Unmarshal(bruto, &ciclos[i]); err != nil {
			t.Fatal(err)
		}
	}
	if len(ciclos) < 1000 || len(g.Cobrancas) < 20 || len(g.Lancamentos) < 20 || len(g.Situacoes) < 200 {
		t.Fatalf("gabarito pequeno (%d ciclos, %d cobranças): a sonda mudou?", len(ciclos), len(g.Cobrancas))
	}
	return g, ciclos
}

func txt[T interface{ String() string }](v *T) string {
	if v == nil {
		return "<nil>"
	}
	return (*v).String()
}

func dec(t *testing.T, s string) tipos.Decimal {
	t.Helper()
	d, err := tipos.NovoDecimal(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func numero(v *int64) string {
	if v == nil {
		return "<nil>"
	}
	return tipos.DecimalDeInteiro(*v).String()
}

// Regra: o ciclo da fatura é o do C# (#144) em todas as combinações do gabarito: meses de 28 a 31 dias e o bissexto,
// a virada do ano, dia de fechamento 1, 3, 28, 30 e 31 ou nenhum, dia de vencimento 5, 10 e 31 ou nenhum, e faturas
// importadas mensais (com o fechamento empurrado pelo fim de semana), sem data de fechamento, irregulares ou nenhuma.
func TestCicloDaFaturaIgualAoCSharp(t *testing.T) {
	g, ciclos := lerGabarito(t)
	conjuntos := map[string][]Fatura{}
	for _, c := range g.Conjuntos {
		conjuntos[c.Nome] = c.Faturas
	}
	for _, c := range ciclos {
		inicio := tipos.NovaData(c.Ano, time.Month(c.Mes), 1)
		fim := inicio.SomarMeses(1).SomarDias(-1)
		obtido := CicloDaFatura(inicio, fim, c.DiaFechamento, c.DiaVencimento, conjuntos[c.Faturas])
		var fatura *int64
		if obtido.Fatura != nil {
			fatura = &obtido.Fatura.Id
		}
		if !obtido.Inicio.Igual(c.Inicio) || !obtido.Fim.Igual(c.Fim) || txt(obtido.Fechamento) != txt(c.Fechamento) ||
			txt(obtido.Vencimento) != txt(c.Vencimento) || numero(fatura) != numero(c.Fatura) {
			t.Errorf("%s %d/%d, fechamento %s, vencimento %s: obtido %s a %s, fecha %s, vence %s, fatura %s; o C# dá %s a %s, "+
				"fecha %s, vence %s, fatura %s", c.Faturas, c.Mes, c.Ano, numero(c.DiaFechamento), numero(c.DiaVencimento),
				obtido.Inicio, obtido.Fim, txt(obtido.Fechamento), txt(obtido.Vencimento), numero(fatura),
				c.Inicio, c.Fim, txt(c.Fechamento), txt(c.Vencimento), numero(c.Fatura))
		}
	}
}

// Regra: o mês de cobrança de cada lançamento é o do C# (#167): o da fatura ligada pelo banco, o da parcela âncora
// mais a diferença de número (a mesma compra pelo total, pelos reais inteiros e pelas letras da descrição), ou pela
// data (a fatura importada até 40 dias depois, o dia de fechamento ou o mês do calendário).
func TestMesesDeCobrancaIgualAoCSharp(t *testing.T) {
	g, _ := lerGabarito(t)
	for _, c := range g.Cobrancas {
		obtido := MesesDeCobranca(g.Lancamentos, c.Faturas, c.DiaFechamento)
		if len(obtido) != len(c.Meses) {
			t.Errorf("%s, dia %s: %d meses, o C# dá %d", c.Conjunto, numero(c.DiaFechamento), len(obtido), len(c.Meses))
		}
		for _, l := range g.Lancamentos {
			esperado := c.Meses[tipos.DecimalDeInteiro(l.Id).String()]
			if mes := obtido[l.Id]; !mes.Igual(esperado) {
				t.Errorf("%s, dia %s, lançamento %d (%s, %s): %s, o C# dá %s", c.Conjunto, numero(c.DiaFechamento), l.Id,
					l.Data, l.Descricao, mes, esperado)
			}
		}
	}
}

// Regra: o pago nas transações (#251) soma os pagamentos depois do vencimento anterior (o importado até 45 dias antes
// ou um mês antes) e até este; o pago que vale é o maior entre o informado e o lançado, com a escala do C#.
func TestPagoIgualAoCSharp(t *testing.T) {
	g, _ := lerGabarito(t)
	for _, c := range g.Pagos {
		if obtido := PagoNasTransacoes(c.Vencimento, c.Vencimentos, g.Pagamentos); obtido.String() != c.Pago {
			t.Errorf("vencimento %s com %v: %s, o C# dá %s", c.Vencimento, c.Vencimentos, obtido, c.Pago)
		}
	}
	for _, c := range g.PagosQueValem {
		var informado *tipos.Decimal
		if c.Informado != nil {
			d := dec(t, *c.Informado)
			informado = &d
		}
		obtido := Pago(informado, dec(t, c.Lancado))
		esperado := "<nil>"
		if c.Pago != nil {
			esperado = *c.Pago
		}
		if txt(obtido) != esperado {
			t.Errorf("Pago(%v, %s) = %s, o C# dá %s", txt(informado), c.Lancado, txt(obtido), esperado)
		}
	}
}

// Regra: a situação é a do C#: paga com um centavo de folga (o pago negativo também conta), fechada depois do
// fechamento ou do vencimento, aberta antes.
func TestSituacaoIgualAoCSharp(t *testing.T) {
	g, _ := lerGabarito(t)
	hoje := tipos.NovaData(2026, 10, 14)
	for _, c := range g.Situacoes {
		var pago *tipos.Decimal
		if c.Pago != nil {
			d := dec(t, *c.Pago)
			pago = &d
		}
		if obtido := Situacao(dec(t, c.Total), pago, c.Fechamento, c.Vencimento, hoje); obtido != c.Situacao {
			t.Errorf("total %s, pago %s, fechamento %s, vencimento %s: %s, o C# dá %s", c.Total, txt(pago),
				txt(c.Fechamento), c.Vencimento, obtido, c.Situacao)
		}
	}
}
