package pluggy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/tipos"
)

func TestNumero(t *testing.T) {
	casos := []struct {
		nome, json, decimal string
		erro                bool
	}{
		{"número", `12.5`, "12.5", false},
		{"negativo", `-3`, "-3", false},
		{"expoente", `1e2`, "100", false},
		{"texto", `"12.5"`, "12.5", false},
		{"texto com sinal", `"+1.5"`, "1.5", false},
		{"texto com expoente", `"1E2"`, "100", false},
		{"texto com escape", `"1\u0032"`, "12", false},
		{"texto que começa no ponto", `".5"`, "0.5", false},
		{"texto que termina no ponto", `"5."`, "5", false},
		{"texto vazio", `""`, "", true},
		{"texto que não é número", `"abc"`, "", true},
		{"vírgula", `"1,5"`, "", true},
		{"dois pontos", `"1.2.3"`, "", true},
		{"dois sinais", `"--1"`, "", true},
		{"só o sinal", `"-"`, "", true},
		{"só o ponto", `"."`, "", true},
		{"NaN", `"NaN"`, "", true},
		{"infinito", `"Infinity"`, "", true},
		{"Inf", `"Inf"`, "", true},
		{"hexadecimal", `"0x10"`, "", true},
		{"espaço", `" 1"`, "", true},
		{"booleano", `true`, "", true},
		{"objeto", `{}`, "", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var n Numero
			err := json.Unmarshal([]byte(c.json), &n)
			if (err != nil) != c.erro {
				t.Fatalf("Unmarshal(%s) = %v, esperado erro: %v", c.json, err, c.erro)
			}
			if err != nil {
				return
			}
			d, err := n.Decimal()
			if err != nil {
				t.Fatal(err)
			}
			if d.String() != c.decimal {
				t.Errorf("Decimal(%s) = %s, esperado %s", c.json, d, c.decimal)
			}
		})
	}
	t.Run("null em campo opcional é nil", func(t *testing.T) {
		var conta Conta
		if err := json.Unmarshal([]byte(`{"balance":null}`), &conta); err != nil {
			t.Fatal(err)
		}
		if conta.Balance != nil {
			t.Errorf("Balance = %v", *conta.Balance)
		}
	})
}

// O (decimal)double do .NET guarda 15 dígitos. Os valores abaixo saem diferentes do decimal direto do texto, e é isso
// que dá o mesmo centavo do .NET (ADR 0049, contrato idêntico).
func TestNumeroPassaPeloDoubleComoONET(t *testing.T) {
	casos := []struct{ nome, texto, decimal string }{
		{"ruído perto do meio centavo", "19.884999999999998", "19.885"},
		{"ruído acima", "0.30000000000000004", "0.3"},
		{"16 dígitos", "1234567890123456", "1234567890123460"},
		{"zero", "0", "0"},
		{"menos zero", "-0.0", "0"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, err := Numero(c.texto).Decimal()
			if err != nil {
				t.Fatal(err)
			}
			if d.String() != c.decimal {
				t.Errorf("Decimal(%s) = %s, esperado %s", c.texto, d, c.decimal)
			}
		})
	}
}

func TestNumeroForaDoLimite(t *testing.T) {
	for _, texto := range []string{"1e400", "1e29", "-1e29"} {
		if _, err := Numero(texto).Decimal(); err == nil {
			t.Errorf("Decimal(%s) sem erro", texto)
		}
	}
}

func TestInteiro(t *testing.T) {
	casos := []struct {
		nome, json string
		esperado   int32
		erro       bool
	}{
		{"número", `7`, 7, false},
		{"texto", `"7"`, 7, false},
		{"negativo", `-7`, -7, false},
		{"o maior int", `2147483647`, 2147483647, false},
		{"fração", `1.5`, 0, true},
		{"com casa decimal", `1.0`, 0, true},
		{"expoente", `1e2`, 0, true},
		{"acima do int", `2147483648`, 0, true},
		{"texto que não é número", `"sete"`, 0, true},
		{"null em campo obrigatório", `null`, 0, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var n Inteiro
			err := json.Unmarshal([]byte(c.json), &n)
			if (err != nil) != c.erro {
				t.Fatalf("Unmarshal(%s) = %v, esperado erro: %v", c.json, err, c.erro)
			}
			if err == nil && int32(n) != c.esperado {
				t.Errorf("Unmarshal(%s) = %v, esperado %v", c.json, n, c.esperado)
			}
		})
	}
}

func TestInstante(t *testing.T) {
	casos := []struct {
		nome, json, utc, data string
	}{
		{"Z com milissegundos", `"2026-10-15T03:00:00.000Z"`, "2026-10-15T03:00:00Z", "2026-10-15"},
		{"meia-noite em Brasília é o mesmo dia", `"2026-10-15T03:00:00Z"`, "2026-10-15T03:00:00Z", "2026-10-15"},
		{"um segundo antes é o dia anterior", `"2026-10-15T02:59:59Z"`, "2026-10-15T02:59:59Z", "2026-10-14"},
		{"com deslocamento", `"2026-10-15T00:30:00-03:00"`, "2026-10-15T03:30:00Z", "2026-10-15"},
		{"deslocamento só com horas", `"2026-10-15T00:30:00-03"`, "2026-10-15T03:30:00Z", "2026-10-15"},
		{"sem segundos", `"2026-10-15T00:30Z"`, "2026-10-15T00:30:00Z", "2026-10-14"},
		{"sem segundos, deslocamento só com horas", `"2026-10-15T00:30-03"`, "2026-10-15T03:30:00Z", "2026-10-15"},
		{"7 casas", `"2026-10-15T03:00:00.1234567Z"`, "2026-10-15T03:00:00.1234567Z", "2026-10-15"},
		{"sem fuso vale Brasília", `"2026-10-15T00:30:00"`, "2026-10-15T03:30:00Z", "2026-10-15"},
		{"sem fuso e sem segundos", `"2026-10-15T00:30"`, "2026-10-15T03:30:00Z", "2026-10-15"},
		{"só a data", `"2026-10-15"`, "2026-10-15T03:00:00Z", "2026-10-15"},
		{"horário de verão antigo", `"2017-12-15T02:00:00Z"`, "2017-12-15T02:00:00Z", "2017-12-15"},
		{"virada de ano", `"2027-01-01T02:59:59Z"`, "2027-01-01T02:59:59Z", "2026-12-31"},
		{"dia 29 de fevereiro", `"2024-02-29T12:00:00Z"`, "2024-02-29T12:00:00Z", "2024-02-29"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var i Instante
			if err := json.Unmarshal([]byte(c.json), &i); err != nil {
				t.Fatal(err)
			}
			if obtido := i.UTC().Format(time.RFC3339Nano); obtido != c.utc {
				t.Errorf("UTC = %s, esperado %s", obtido, c.utc)
			}
			if obtido := i.Data().String(); obtido != c.data {
				t.Errorf("Data = %s, esperado %s", obtido, c.data)
			}
		})
	}

	invalidos := []struct{ nome, json string }{
		{"vazio", `""`},
		{"texto", `"ontem"`},
		{"mês 13", `"2026-13-01"`},
		{"30 de fevereiro", `"2026-02-30"`},
		{"sem T", `"2026-10-15 03:00:00"`},
		{"número", `20261015`},
		{"null direto", `null`},
	}
	for _, c := range invalidos {
		t.Run(c.nome, func(t *testing.T) {
			var i Instante
			err := i.UnmarshalJSON([]byte(c.json))
			if err == nil {
				t.Fatalf("UnmarshalJSON(%s) sem erro", c.json)
			}
			if c.json != `null` && strings.Contains(err.Error(), strings.Trim(c.json, `"`)) && c.json != `""` {
				t.Errorf("erro com o valor lido: %v", err)
			}
		})
	}
}

func TestDocumento(t *testing.T) {
	casos := []struct{ nome, json, esperado string }{
		{"objeto com value", `{"documentNumber":{"type":"CPF","value":"00000000191"}}`, "00000000191"},
		{"texto", `{"documentNumber":"00000000191"}`, "00000000191"},
		{"mascarado", `{"documentNumber":"***.000.001-**"}`, "***.000.001-**"},
		{"objeto sem value", `{"documentNumber":{"type":"CPF"}}`, ""},
		{"value que não é texto", `{"documentNumber":{"value":123}}`, ""},
		{"value null", `{"documentNumber":{"value":null}}`, ""},
		{"número", `{"documentNumber":123}`, ""},
		{"null", `{"documentNumber":null}`, ""},
		{"lista", `{"documentNumber":["1"]}`, ""},
		{"ausente", `{}`, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var p Participante
			if err := json.Unmarshal([]byte(c.json), &p); err != nil {
				t.Fatal(err)
			}
			if obtido := p.Documento(); obtido != c.esperado {
				t.Errorf("Documento = %q, esperado %q", obtido, c.esperado)
			}
		})
	}
	t.Run("sem participante", func(t *testing.T) {
		var p *Participante
		if obtido := p.Documento(); obtido != "" {
			t.Errorf("Documento = %q", obtido)
		}
	})
}

// O resto dos campos: cada modelo lido do arquivo no formato da Pluggy, para um nome errado na etiqueta não passar.

func TestLerItem(t *testing.T) {
	c, falsa, _ := pluggyFalsa(t)
	falsa.Em("GET", "/items/item-1", testeapoio.RespostaDeFora{Arquivo: "item.json"})
	item, err := c.ObterItem(t.Context(), sessao, "item-1")
	if err != nil {
		t.Fatal(err)
	}
	if item.Id != "item-ficticio-1" || *item.Connector.Name != "Banco Fictício" || *item.Connector.PrimaryColor != "EF3E42" ||
		*item.Status != "UPDATED" || *item.ExecutionStatus != "SUCCESS" || *item.Error.Message != "Instabilidade na instituição" {
		t.Errorf("item = %+v", item)
	}
	if item.LastUpdatedAt.Data() != tipos.NovaData(2026, 10, 14) || item.ConsentExpiresAt.Data() != tipos.NovaData(2027, 4, 14) {
		t.Errorf("datas: %v, %v", item.LastUpdatedAt, item.ConsentExpiresAt)
	}
}

func TestLerContas(t *testing.T) {
	c, falsa, _ := pluggyFalsa(t)
	falsa.Em("GET", "/accounts", testeapoio.RespostaDeFora{Arquivo: "conta-e-cartao.json"})
	contas, err := c.ListarContas(t.Context(), sessao, "item-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(contas) != 2 {
		t.Fatalf("%d contas", len(contas))
	}
	banco, cartao := contas[0], contas[1]
	if banco.Id != "conta-corrente-1" || *banco.Type != "BANK" || *banco.Subtype != "CHECKING_ACCOUNT" ||
		*banco.Number != "0001/12345-0" || *banco.Name != "Conta Corrente" || *banco.MarketingName != "Conta Fictícia" ||
		*banco.Balance != "1520.35" || *banco.CurrencyCode != "BRL" || banco.CreditData != nil {
		t.Errorf("conta = %+v", banco)
	}
	dados := banco.BankData
	if *dados.ClosingBalance != "1520.35" || *dados.AutomaticallyInvestedBalance != "300.5" || !*dados.HasReservedBalance ||
		len(dados.ReservedBalances) != 2 {
		t.Errorf("bankData = %+v", dados)
	}
	credito := cartao.CreditData
	if cartao.MarketingName != nil || cartao.BankData != nil || *credito.CreditLimit != "5000" || *credito.AvailableCreditLimit != "4159.9" ||
		credito.BalanceCloseDate.Data() != tipos.NovaData(2026, 10, 20) || credito.BalanceDueDate.Data() != tipos.NovaData(2026, 10, 28) {
		t.Errorf("cartão = %+v, %+v", cartao, credito)
	}
}

func TestLerTransacoes(t *testing.T) {
	c, falsa, _ := pluggyFalsa(t)
	falsa.Em("GET", "/v2/transactions", testeapoio.RespostaDeFora{Arquivo: "transacoes-pagina-1.json"},
		testeapoio.RespostaDeFora{Arquivo: "transacoes-pagina-2.json"})
	lidas, err := c.ListarTransacoes(t.Context(), sessao, "cartao-1", tipos.NovaData(2026, 10, 1))
	if err != nil {
		t.Fatal(err)
	}
	a, b := lidas[0], lidas[1]
	if a.Date.Data() != tipos.NovaData(2026, 10, 14) || *a.Description != "PADARIA FICTICIA" || *a.Amount != "25.5" ||
		a.AmountInAccountCurrency != nil || *a.Type != "DEBIT" || *a.Status != "POSTED" || *a.CategoryId != "05000000" ||
		*a.CreditCardMetadata.InstallmentNumber != 2 || *a.CreditCardMetadata.TotalInstallments != 6 ||
		*a.CreditCardMetadata.BillId != "fatura-1" || *a.Merchant.Name != "Padaria" ||
		*a.Merchant.BusinessName != "Padaria Ficticia LTDA" || *a.OperationType != "CARTAO" || a.PaymentData != nil {
		t.Errorf("transação = %+v", a)
	}
	if *b.AmountInAccountCurrency != "98.76" || b.CategoryId != nil || b.Merchant != nil || b.OperationType != nil ||
		*b.PaymentData.PaymentMethod != "PIX" || b.PaymentData.Payer.Documento() != "00000000191" ||
		b.PaymentData.Receiver.Documento() != "***.000.001-**" {
		t.Errorf("transação = %+v", b)
	}
}

func TestLerFaturas(t *testing.T) {
	c, falsa, _ := pluggyFalsa(t)
	falsa.Em("GET", "/bills", testeapoio.RespostaDeFora{Corpo: `{"totalPages":1,"results":[{"id":"f-1",
		"dueDate":"2026-10-28T03:00:00.000Z","billClosingDate":"2026-10-20T03:00:00.000Z","totalAmount":840.1,
		"minimumPaymentAmount":"84.01","payments":[{"amount":100},{"amount":"40.1"}]}]}`})
	faturas, err := c.ListarFaturas(t.Context(), sessao, "cartao-1")
	if err != nil {
		t.Fatal(err)
	}
	f := faturas[0]
	if f.Id != "f-1" || f.DueDate.Data() != tipos.NovaData(2026, 10, 28) || f.BillClosingDate.Data() != tipos.NovaData(2026, 10, 20) ||
		*f.TotalAmount != "840.1" || *f.MinimumPaymentAmount != "84.01" || len(f.Payments) != 2 || *f.Payments[1].Amount != "40.1" {
		t.Errorf("fatura = %+v", f)
	}
}

func TestLerInvestimentos(t *testing.T) {
	c, falsa, _ := pluggyFalsa(t)
	falsa.Em("GET", "/investments", testeapoio.RespostaDeFora{Arquivo: "investimentos.json"})
	lidos, err := c.ListarInvestimentos(t.Context(), sessao, "item-1")
	if err != nil {
		t.Fatal(err)
	}
	i := lidos[0]
	if i.Id != "inv-1" || *i.Name != "FII FICTICIO" || *i.Type != "EQUITY" || *i.Subtype != "REAL_ESTATE_FUND" || *i.Code != "FICT11" ||
		*i.Issuer != "FICTICIO FDO INV IMOB - FII" || *i.CurrencyCode != "BRL" || *i.Balance != "1050.75" || *i.AmountOriginal != "1000" ||
		*i.AmountProfit != "50.75" || *i.Quantity != "10.5" || i.AnnualRate != nil || *i.FixedAnnualRate != "12.5" ||
		i.DueDate.Data() != tipos.NovaData(2030, 1, 15) || i.PurchaseDate != nil || i.IssueDate.Data() != tipos.NovaData(2025, 3, 10) ||
		*i.Status != "ACTIVE" {
		t.Errorf("investimento = %+v", i)
	}
}

func TestLerMovimentos(t *testing.T) {
	c, falsa, _ := pluggyFalsa(t)
	falsa.Em("GET", "*", testeapoio.RespostaDeFora{Arquivo: "movimentos.json"})
	lidos, err := c.ListarMovimentosInvestimento(t.Context(), sessao, "inv-1")
	if err != nil {
		t.Fatal(err)
	}
	m := lidos[0]
	if m.Id != "mov-1" || *m.Type != "BUY" || *m.Quantity != "3" || *m.Value != "100.25" || *m.Amount != "300.75" ||
		m.Date.Data() != tipos.NovaData(2026, 2, 2) || m.TradeDate.Data() != tipos.NovaData(2026, 1, 30) {
		t.Errorf("movimento = %+v", m)
	}
	if *m.Expenses["serviceTax"] != "1.5" || *m.Expenses["brokerageFee"] != "0.25" || m.Expenses["incomeTax"] != nil {
		t.Errorf("despesas = %+v", m.Expenses)
	}
}

func TestLerEmprestimos(t *testing.T) {
	c, falsa, _ := pluggyFalsa(t)
	falsa.Em("GET", "/loans", testeapoio.RespostaDeFora{Arquivo: "emprestimos.json"})
	lidos, err := c.ListarEmprestimos(t.Context(), sessao, "item-1")
	if err != nil {
		t.Fatal(err)
	}
	e := lidos[0]
	if e.Id != "emprestimo-1" || *e.Kind != "FINANCING" || *e.ContractNumber != "CONTRATO-FICTICIO" ||
		*e.ProductName != "Financiamento Fictício" || *e.Type != "FINANCIAMENTO_HABITACIONAL_SFH" ||
		e.ContractDate.Data() != tipos.NovaData(2024, 5, 10) || *e.ContractAmount != "200000" || *e.CurrencyCode != "BRL" ||
		e.DueDate.Data() != tipos.NovaData(2044, 5, 10) || *e.InstallmentPeriodicity != "MENSAL" ||
		e.FirstInstallmentDueDate.Data() != tipos.NovaData(2024, 6, 10) || *e.Cet != "0.0975" || *e.AmortizationScheduled != "SAC" {
		t.Errorf("empréstimo = %+v", e)
	}
	taxa := e.InterestRates[0]
	if *taxa.TaxPeriodicity != "AA" || *taxa.ReferentialRateIndexerType != "POS_FIXADO" ||
		*taxa.ReferentialRateIndexerSubType != "TR_TBF" || *taxa.PreFixedRate != "0.0875" {
		t.Errorf("taxa = %+v", taxa)
	}
	if p := e.Installments; *p.TotalNumberOfInstallments != 240 || *p.PaidInstallments != 29 || *p.DueInstallments != 211 || *p.PastDueInstallments != 0 {
		t.Errorf("parcelas = %+v", p)
	}
	if *e.Payments.ContractOutstandingBalance != "185000.5" {
		t.Errorf("saldo = %v", *e.Payments.ContractOutstandingBalance)
	}
	r := e.Payments.Releases[0]
	if *r.IsOverParcelPayment || r.PaidDate.Data() != tipos.NovaData(2026, 10, 10) || *r.PaidAmount != "1890.12" {
		t.Errorf("pagamento = %+v", r)
	}
}

func TestLerCategorias(t *testing.T) {
	c, falsa, _ := pluggyFalsa(t)
	falsa.Em("GET", "/categories", testeapoio.RespostaDeFora{Corpo: `{"results":[{"id":"05000000","description":"Food",
		"descriptionTranslated":"Alimentação","parentId":"01","parentDescription":"Living"}]}`})
	lidas, err := c.ListarCategorias(t.Context(), sessao)
	if err != nil {
		t.Fatal(err)
	}
	k := lidas[0]
	if k.Id != "05000000" || *k.Description != "Food" || *k.DescriptionTranslated != "Alimentação" || *k.ParentId != "01" ||
		*k.ParentDescription != "Living" {
		t.Errorf("categoria = %+v", k)
	}
}

// Regra: o Sum do C# é em double, e só o total vira decimal: 0.1 + 0.2 sai 0.3 pelos 15 dígitos.
func TestSomaDosPagamentos(t *testing.T) {
	numero := func(s string) *Numero {
		n := Numero(s)
		return &n
	}
	casos := []struct {
		nome     string
		lista    []PagamentoFatura
		esperado string
	}{
		{"nenhum", nil, "0"},
		{"um", []PagamentoFatura{{numero("10.5")}}, "10.5"},
		{"o ruído da soma some", []PagamentoFatura{{numero("0.1")}, {numero("0.2")}}, "0.3"},
		{"sem valor conta zero", []PagamentoFatura{{numero("1")}, {nil}, {numero("2")}}, "3"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, err := SomaDosPagamentos(c.lista)
			if err != nil {
				t.Fatal(err)
			}
			if d.String() != c.esperado {
				t.Errorf("SomaDosPagamentos = %s, esperado %s", d, c.esperado)
			}
		})
	}
	if _, err := SomaDosPagamentos([]PagamentoFatura{{numero("1e400")}}); err == nil {
		t.Error("número fora do limite sem erro")
	}
}
