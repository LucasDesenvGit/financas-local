package sincronizacao

import (
	"encoding/json"
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend/internal/openfinance/pluggy"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/patrimonio/emprestimos"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/testeapoio"
	"github.com/LucasDesenvGit/financas-local/src/backend/internal/plataforma/tipos"
)

func p[T any](v T) *T { return &v }

func n(texto string) *pluggy.Numero {
	v := pluggy.Numero(texto)
	return &v
}

func textoDe(d *tipos.Decimal) string {
	if d == nil {
		return "<nil>"
	}
	return d.String()
}

// Regra: o (decimal)double vem antes do arredondamento para longe do zero: o ruído perto do meio centavo sobe, como no .NET.
func TestArredondar(t *testing.T) {
	casos := []struct {
		nome     string
		numero   *pluggy.Numero
		esperado string
	}{
		{"sem valor", nil, "0"},
		{"com ruído do double", n("19.884999999999998"), "19.89"},
		{"meio centavo sobe", n("1.005"), "1.01"},
		{"meio centavo negativo", n("-1.005"), "-1.01"},
		{"meio centavo par também sobe", n("2.125"), "2.13"},
		{"menos casas ficam", n("25.5"), "25.5"},
		{"inteiro", n("100"), "100"},
		{"zero", n("0"), "0"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, err := arredondar(c.numero)
			if err != nil {
				t.Fatal(err)
			}
			if d.String() != c.esperado {
				t.Errorf("arredondar = %s, esperado %s", d, c.esperado)
			}
		})
	}
	if _, err := arredondar(n("1e400")); err == nil {
		t.Error("arredondar(1e400) sem erro")
	}
	if d, err := arredondarNulo(nil); d != nil || err != nil {
		t.Errorf("arredondarNulo(nil) = %v, %v", d, err)
	}
	if _, err := arredondarNulo(n("1e400")); err == nil {
		t.Error("arredondarNulo(1e400) sem erro")
	}
}

func TestCalcularValor(t *testing.T) {
	casos := []struct {
		nome           string
		valor, naConta *pluggy.Numero
		tipo           *string
		cartao         bool
		esperado       string
	}{
		{"cartão: compra positiva vira saída", n("25.5"), nil, nil, true, "-25.5"},
		{"cartão: estorno negativo vira entrada", n("-25.5"), nil, nil, true, "25.5"},
		{"DEBIT sai, mesmo positivo", n("10"), nil, p("DEBIT"), false, "-10"},
		{"DEBIT sai, negativo", n("-10"), nil, p("debit"), false, "-10"},
		{"CREDIT entra, mesmo negativo", n("-10"), nil, p("CREDIT"), false, "10"},
		{"sem tipo fica como veio", n("-10"), nil, nil, false, "-10"},
		{"tipo desconhecido fica como veio", n("10"), nil, p("OUTRO"), false, "10"},
		{"o valor na moeda da conta vale antes", n("100"), n("98.755"), p("DEBIT"), false, "-98.76"},
		{"sem valor", nil, nil, p("DEBIT"), false, "0"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, err := calcularValor(&pluggy.Transacao{Amount: c.valor, AmountInAccountCurrency: c.naConta, Type: c.tipo}, c.cartao)
			if err != nil {
				t.Fatal(err)
			}
			if d.Decimal().String() != c.esperado {
				t.Errorf("calcularValor = %s, esperado %s", d, c.esperado)
			}
		})
	}
	if _, err := calcularValor(&pluggy.Transacao{Amount: n("1e400")}, false); err == nil {
		t.Error("calcularValor(1e400) sem erro")
	}
}

func participante(t *testing.T, documento string) *pluggy.Participante {
	t.Helper()
	var r pluggy.Participante
	if err := json.Unmarshal([]byte(`{"documentNumber":`+documento+`}`), &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

func TestMesmoTitular(t *testing.T) {
	const cpf, cnpj = `"00000000191"`, `"11222333000181"`
	casos := []struct {
		nome, pagador, recebedor string
		esperado                 *bool
	}{
		{"mesmo CPF", cpf, cpf, p(true)},
		{"mesmo CPF com pontuação", `"000.000.001-91"`, cpf, p(true)},
		{"CPF em objeto", `{"value":"00000000191"}`, cpf, p(true)},
		{"documentos diferentes", cpf, cnpj, p(false)},
		{"mesmo CNPJ", cnpj, cnpj, p(true)},
		{"mascarado não dá para saber", `"***.000.001-**"`, cpf, nil},
		{"recebedor mascarado", cpf, `"***"`, nil},
		{"tamanho errado", `"123"`, cpf, nil},
		{"em branco", `"  "`, cpf, nil},
		{"sem documento", `null`, cpf, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			obtido := mesmoTitular(&pluggy.DadosPagamento{Payer: participante(t, c.pagador), Receiver: participante(t, c.recebedor)})
			if (obtido == nil) != (c.esperado == nil) || (obtido != nil && *obtido != *c.esperado) {
				t.Errorf("mesmoTitular = %v, esperado %v", obtido, c.esperado)
			}
		})
	}
	if obtido := mesmoTitular(nil); obtido != nil {
		t.Errorf("sem dados = %v", *obtido)
	}
	if obtido := mesmoTitular(&pluggy.DadosPagamento{}); obtido != nil {
		t.Errorf("sem participantes = %v", *obtido)
	}
}

func TestNormalizarCor(t *testing.T) {
	casos := []struct {
		entrada, esperado *string
	}{
		{nil, nil},
		{p(""), nil},
		{p("EF3E42"), p("#ef3e42")},
		{p("#EF3E42"), p("#ef3e42")},
		{p("  ##ef3e42 "), p("#ef3e42")},
		{p("EF3E4"), nil},
		{p("EF3E422"), nil},
		{p("not-hex"), nil},
		{p("GGGGGG"), nil},
	}
	for _, c := range casos {
		obtido := normalizarCor(c.entrada)
		if (obtido == nil) != (c.esperado == nil) || (obtido != nil && *obtido != *c.esperado) {
			t.Errorf("normalizarCor(%v) = %v, esperado %v", c.entrada, obtido, c.esperado)
		}
	}
}

func TestNormalizarMoeda(t *testing.T) {
	casos := []struct {
		entrada  *string
		esperado string
	}{
		{nil, "BRL"},
		{p("usd"), "USD"},
		{p("BRL"), "BRL"},
		{p("USD "), "BRL"},
		{p("US"), "BRL"},
		{p(""), "BRL"},
	}
	for _, c := range casos {
		if obtido := normalizarMoeda(c.entrada); obtido != c.esperado {
			t.Errorf("normalizarMoeda(%v) = %s, esperado %s", c.entrada, obtido, c.esperado)
		}
	}
}

func TestLimitar(t *testing.T) {
	if limitar(nil, 3) != nil {
		t.Error("limitar(nil) não é nil")
	}
	if obtido := limitar(p("abcd"), 3); *obtido != "abc" {
		t.Errorf("limitar = %q", *obtido)
	}
	if obtido := limitar(p("ab"), 3); *obtido != "ab" {
		t.Errorf("limitar = %q", *obtido)
	}
}

func TestParcela(t *testing.T) {
	v := func(i pluggy.Inteiro) *pluggy.Inteiro { return &i }
	casos := []struct {
		entrada  *pluggy.Inteiro
		esperado *int64
	}{
		{nil, nil},
		{v(2), p(int64(2))},
		{v(32767), p(int64(32767))},
		{v(40000), p(int64(-25536))}, // o (short) do C#, sem conferir
	}
	for _, c := range casos {
		obtido := parcela(c.entrada)
		if (obtido == nil) != (c.esperado == nil) || (obtido != nil && *obtido != *c.esperado) {
			t.Errorf("parcela = %v, esperado %v", obtido, c.esperado)
		}
	}
	if inteiroDe(nil) != nil || *inteiroDe(v(5)) != 5 {
		t.Error("inteiroDe errado")
	}
}

func TestMeioDePagamento(t *testing.T) {
	casos := []struct {
		nome     string
		pt       pluggy.Transacao
		esperado *string
	}{
		{"sem nada", pluggy.Transacao{}, nil},
		{"pela operação", pluggy.Transacao{OperationType: p(" pix ")}, p("PIX")},
		{"pelo pagamento", pluggy.Transacao{PaymentData: &pluggy.DadosPagamento{PaymentMethod: p("ted")}}, p("TED")},
		{"a operação vale antes", pluggy.Transacao{OperationType: p("BOLETO"), PaymentData: &pluggy.DadosPagamento{PaymentMethod: p("ted")}}, p("BOLETO")},
		{"pagamento sem meio", pluggy.Transacao{PaymentData: &pluggy.DadosPagamento{}}, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			obtido := meioDePagamento(&c.pt)
			if (obtido == nil) != (c.esperado == nil) || (obtido != nil && *obtido != *c.esperado) {
				t.Errorf("meioDePagamento = %v, esperado %v", obtido, c.esperado)
			}
		})
	}
}

func TestEstabelecimento(t *testing.T) {
	if estabelecimento(nil) != nil {
		t.Error("sem estabelecimento")
	}
	if e := estabelecimento(&pluggy.Estabelecimento{Name: p("Nome"), BusinessName: p("Razão")}); *e != "Razão" {
		t.Errorf("a razão social vale antes: %q", *e)
	}
	if e := estabelecimento(&pluggy.Estabelecimento{Name: p("Nome")}); *e != "Nome" {
		t.Errorf("nome: %q", *e)
	}
}

func TestNaturezaDoCredito(t *testing.T) {
	casos := []struct {
		nome                      string
		kind, modalidade, produto *string
		esperado                  int64
	}{
		{"pela família", p("financing"), nil, nil, emprestimos.NaturezaFinanciamento},
		{"pela modalidade", nil, p("FINANCIAMENTO_HABITACIONAL_SFH"), nil, emprestimos.NaturezaFinanciamento},
		{"aquisição de bens", nil, p("aquisicao_bens_veiculos_automotores"), nil, emprestimos.NaturezaFinanciamento},
		{"pelo produto, sem modalidade", nil, nil, p("Crédito Financiado"), emprestimos.NaturezaFinanciamento},
		{"pelo produto, modalidade em branco", nil, p("  "), p("FINANC"), emprestimos.NaturezaFinanciamento},
		{"o produto não vale com modalidade", nil, p("CHEQUE_ESPECIAL"), p("Financiado"), emprestimos.NaturezaEmprestimo},
		{"crédito pessoal", p("LOAN"), p("CREDITO_PESSOAL_SEM_CONSIGNACAO"), nil, emprestimos.NaturezaEmprestimo},
		{"sem nada", nil, nil, nil, emprestimos.NaturezaEmprestimo},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if obtido := naturezaDoCredito(c.kind, c.modalidade, c.produto); obtido != c.esperado {
				t.Errorf("naturezaDoCredito = %d, esperado %d", obtido, c.esperado)
			}
		})
	}
}

func TestComLimite(t *testing.T) {
	casos := []struct {
		nome     string
		numero   *pluggy.Numero
		limite   int64
		casas    int32
		esperado string
	}{
		{"sem valor", nil, 1000, 8, "<nil>"},
		{"dentro", n("0.0975"), 1000, 8, "0.0975"},
		{"arredonda pelo método bancário", n("12.34565"), 99999, 4, "12.3456"},
		{"negativo dentro", n("-5"), 1000, 8, "-5"},
		{"no limite fica de fora", n("1000"), 1000, 8, "<nil>"},
		{"negativo no limite", n("-1000"), 1000, 8, "<nil>"},
		{"acima", n("123456"), 99999, 4, "<nil>"},
		{"nem cabe no decimal", n("1e400"), 1000, 8, "<nil>"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if obtido := textoDe(comLimite(c.numero, c.limite, c.casas)); obtido != c.esperado {
				t.Errorf("comLimite = %s, esperado %s", obtido, c.esperado)
			}
		})
	}
}

func TestValorDaParcela(t *testing.T) {
	dia := func(d int) tipos.Data { return tipos.NovaData(2026, 10, d) }
	v := testeapoio.Valor
	casos := []struct {
		nome     string
		pagos    []pagamento
		aVencer  *int64
		saldo    tipos.Decimal
		esperado string
	}{
		{"a última paga", []pagamento{{dia(1), v("10.0"), false}, {dia(5), v("20.0"), false}, {dia(3), v("30.0"), false}}, p(int64(3)), v("90.0"), "20.0"},
		{"a extraordinária não conta", []pagamento{{dia(1), v("10.0"), false}, {dia(5), v("500.0"), true}}, p(int64(3)), v("90.0"), "10.0"},
		{"na mesma data, a primeira", []pagamento{{dia(1), v("10.0"), false}, {dia(1), v("20.0"), false}}, nil, v("90.0"), "10.0"},
		{"sem pagamento, o saldo pelas parcelas", nil, p(int64(3)), v("100.0"), "33.33"},
		{"só extraordinária, o saldo pelas parcelas", []pagamento{{dia(1), v("10.0"), true}}, p(int64(4)), v("100.0"), "25.0"},
		{"sem parcelas a vencer", nil, p(int64(0)), v("100.0"), "<nil>"},
		{"sem a contagem", nil, nil, v("100.0"), "<nil>"},
		{"sem saldo", nil, p(int64(3)), tipos.Zero, "<nil>"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, err := valorDaParcela(c.pagos, c.aVencer, c.saldo)
			if err != nil {
				t.Fatal(err)
			}
			if obtido := textoDe(d); obtido != c.esperado {
				t.Errorf("valorDaParcela = %s, esperado %s", obtido, c.esperado)
			}
		})
	}
}

func TestReservasDoOpenFinance(t *testing.T) {
	raw := func(s ...string) []json.RawMessage {
		lista := make([]json.RawMessage, len(s))
		for i, x := range s {
			lista[i] = json.RawMessage(x)
		}
		return lista
	}
	casos := []struct {
		nome   string
		dados  *pluggy.DadosBancarios
		total  string
		json   string
		campos int
	}{
		{"sem dados", nil, "<nil>", "", 0},
		{"sem reservas", &pluggy.DadosBancarios{}, "<nil>", "", 0},
		{"automático quando não há detalhe", &pluggy.DadosBancarios{AutomaticallyInvestedBalance: n("75.125")}, "75.12", "", 0},
		{"automático zero não conta", &pluggy.DadosBancarios{AutomaticallyInvestedBalance: n("0")}, "<nil>", "", 0},
		{"com nome e valor", &pluggy.DadosBancarios{ReservedBalances: raw(`{"name":"Viagem","amount":250.505}`)}, "250.50", `[{"nome":"Viagem","valor":250.50}]`, 2},
		{"o detalhe vale antes do automático", &pluggy.DadosBancarios{AutomaticallyInvestedBalance: n("10"), ReservedBalances: raw(`{"amount":1}`)}, "1", `[{"nome":"Reservado","valor":1}]`, 1},
		{"valor dentro de objeto", &pluggy.DadosBancarios{ReservedBalances: raw(`{"balance":{"currency":"BRL","value":10.5}}`)}, "10.5", `[{"nome":"Reservado","valor":10.5}]`, 1},
		{"o primeiro campo de valor vale", &pluggy.DadosBancarios{ReservedBalances: raw(`{"amount":1,"balance":2}`)}, "1", `[{"nome":"Reservado","valor":1}]`, 2},
		{"maiúsculas no nome do campo", &pluggy.DadosBancarios{ReservedBalances: raw(`{"Amount":1,"NAME":"Caixa"}`)}, "1", `[{"nome":"Caixa","valor":1}]`, 2},
		{"sem valor é ignorado", &pluggy.DadosBancarios{ReservedBalances: raw(`{"name":"Só nome"}`)}, "<nil>", "", 1},
		{"valor em texto não vale", &pluggy.DadosBancarios{ReservedBalances: raw(`{"amount":"10"}`)}, "<nil>", "", 1},
		{"fora de objeto é ignorado", &pluggy.DadosBancarios{ReservedBalances: raw(`"texto"`, `1`)}, "<nil>", "", 0},
		{"duas reservas somam", &pluggy.DadosBancarios{ReservedBalances: raw(`{"amount":1.5}`, `{"value":2}`)}, "3.5", `[{"nome":"Reservado","valor":1.5},{"nome":"Reservado","valor":2}]`, 2},
		{"nome em branco", &pluggy.DadosBancarios{ReservedBalances: raw(`{"name":"  ","amount":1}`)}, "1", `[{"nome":"Reservado","valor":1}]`, 2},
		{"acento e aspas como o .NET", &pluggy.DadosBancarios{ReservedBalances: raw(`{"name":"Férias \"já\"","amount":1}`)}, "1", "[{\"nome\":\"F\\u00E9rias \\u0022j\\u00E1\\u0022\",\"valor\":1}]", 2},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			total, texto, campos, err := reservasDoOpenFinance(c.dados)
			if err != nil {
				t.Fatal(err)
			}
			if obtido := textoDe(total); obtido != c.total {
				t.Errorf("total = %s, esperado %s", obtido, c.total)
			}
			obtido := ""
			if texto != nil {
				obtido = *texto
			}
			if obtido != c.json {
				t.Errorf("json = %s, esperado %s", obtido, c.json)
			}
			if len(campos) != c.campos {
				t.Errorf("campos = %v, esperado %d", campos, c.campos)
			}
		})
	}
	t.Run("nome com mais de 100 caracteres", func(t *testing.T) {
		longo := make([]byte, 120)
		for i := range longo {
			longo[i] = 'a'
		}
		_, texto, _, err := reservasDoOpenFinance(&pluggy.DadosBancarios{ReservedBalances: raw(`{"name":"` + string(longo) + `","amount":1}`)})
		if err != nil {
			t.Fatal(err)
		}
		if esperado := `[{"nome":"` + string(longo[:100]) + `","valor":1}]`; *texto != esperado {
			t.Errorf("json = %s", *texto)
		}
	})
}

func TestEscaparComoONet(t *testing.T) {
	casos := []struct{ entrada, esperado string }{
		{"Viagem", "Viagem"},
		{"férias", "f\\u00E9rias"},
		{"a\"b", "a\\u0022b"},
		{"a\\b", "a\\\\b"},
		{"a&b", "a\\u0026b"},
		{"a'b", "a\\u0027b"},
		{"a+b", "a\\u002Bb"},
		{"a<b>", "a\\u003Cb\\u003E"},
		{"a`b", "a\\u0060b"},
		{"a\nb\rc\td\be\ff", "a\\nb\\rc\\td\\be\\ff"},
		{"\x01", "\\u0001"},
		{"\x7f", "\\u007F"},
		{"😀", "\\uD83D\\uDE00"},
	}
	for _, c := range casos {
		if obtido := escaparComoONet(c.entrada); obtido != c.esperado {
			t.Errorf("escaparComoONet(%q) = %s, esperado %s", c.entrada, obtido, c.esperado)
		}
	}
}
func TestSugerirCategoriaPadrao(t *testing.T) {
	casos := []struct {
		descricao, pai *string
		esperado       string
	}{
		{p("Groceries"), nil, "Mercado"},
		{p("  groceries "), nil, "Mercado"},
		{p("GROCERIES"), nil, "Mercado"},
		{p("Sem mapa"), p("Groceries"), "Mercado"},
		{nil, p("Salary"), "Salário"},
		{p("Salary"), p("Groceries"), "Salário"},
		{p("Sem mapa"), p("Também não"), ""},
		{nil, nil, ""},
	}
	for _, c := range casos {
		if obtido := sugerirCategoriaPadrao(c.descricao, c.pai); obtido != c.esperado {
			t.Errorf("sugerirCategoriaPadrao = %q, esperado %q", obtido, c.esperado)
		}
	}
}

func TestTipoPorPluggy(t *testing.T) {
	casos := []struct {
		nome     string
		tipo     string
		subtipo  *string
		ticker   string
		emissor  *string
		titulo   string
		esperado int64
	}{
		{"ETF", "ETF", nil, "BOVA11", nil, "ETF", ativoEtf},
		{"FII pelo emissor, com o subtipo STOCK", "EQUITY", p("STOCK"), "FALS11", p("FALSO FDO INV IMOB - FII"), "", ativoFii},
		{"FII pelo nome", "EQUITY", p("STOCK"), "FALS11", nil, "FALSO FII", ativoFii},
		{"fundo imobiliário", "EQUITY", p("real_estate_fund"), "XPTO3", nil, "", ativoFii},
		{"BDR", "EQUITY", p("BDR"), "AAPL34", nil, "", ativoBdr},
		{"ação", "EQUITY", p("STOCK"), "FALS11", nil, "", ativoAcao},
		{"sem subtipo, final 11", "EQUITY", nil, "FALS11", nil, "", ativoFii},
		{"sem subtipo, final 12", "EQUITY", nil, "FALS12", nil, "", ativoFii},
		{"sem subtipo, final 13", "EQUITY", p("OUTRO"), "FALS13", nil, "", ativoFii},
		{"sem subtipo, final 3", "EQUITY", nil, "FALS3", nil, "", ativoAcao},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if obtido := tipoPorPluggy(c.tipo, c.subtipo, c.ticker, c.emissor, c.titulo); obtido != c.esperado {
				t.Errorf("tipoPorPluggy = %d, esperado %d", obtido, c.esperado)
			}
		})
	}
}

func TestPrecoDoMovimento(t *testing.T) {
	v := testeapoio.Valor
	casos := []struct {
		nome         string
		valor, total *pluggy.Numero
		quantidade   string
		esperado     string
	}{
		{"o preço unitário", n("100.25"), n("999"), "2", "100.25"},
		{"sem preço, o total dividido", nil, n("200.5"), "2", "100.25"},
		{"preço zero usa o total", n("0"), n("200"), "4", "50"},
		{"total negativo", nil, n("-120.5"), "1", "120.5"},
		{"sem nada", nil, nil, "1", "0"},
		{"total zero", nil, n("0"), "1", "0"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, err := precoDoMovimento(&pluggy.MovimentoInvestimento{Value: c.valor, Amount: c.total}, v(c.quantidade))
			if err != nil {
				t.Fatal(err)
			}
			if d.Decimal().String() != c.esperado {
				t.Errorf("precoDoMovimento = %s, esperado %s", d, c.esperado)
			}
		})
	}
}

func TestTaxasDoMovimento(t *testing.T) {
	d, err := taxasDoMovimento(&pluggy.MovimentoInvestimento{Expenses: map[string]*pluggy.Numero{
		"serviceTax": n("1.5"), "brokerageFee": n("0.25"), "incomeTax": n("9.9"), "negativa": n("-1"), "vazia": nil,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Decimal().String() != "1.75" {
		t.Errorf("taxas = %s: o imposto de renda e as negativas ficam de fora", d)
	}
	if _, err := taxasDoMovimento(&pluggy.MovimentoInvestimento{Expenses: map[string]*pluggy.Numero{"x": n("1e400")}}); err == nil {
		t.Error("sem erro no número fora do limite")
	}
}

func TestLancamentoIgual(t *testing.T) {
	v := testeapoio.Valor
	dia := tipos.NovaData(2026, 10, 15)
	sua := func(ajuste func(*operacao)) *operacao {
		o := &operacao{tipo: operacaoCompra, data: dia, quantidade: v("10.0")}
		if ajuste != nil {
			ajuste(o)
		}
		return o
	}
	casos := []struct {
		nome string
		o    *operacao
		casa bool
	}{
		{"igual", sua(nil), true},
		{"quantidade com outra escala", sua(func(o *operacao) { o.quantidade = v("10.000000") }), true},
		{"5 dias antes", sua(func(o *operacao) { o.data = dia.SomarDias(-5) }), true},
		{"5 dias depois", sua(func(o *operacao) { o.data = dia.SomarDias(5) }), true},
		{"6 dias", sua(func(o *operacao) { o.data = dia.SomarDias(6) }), false},
		{"outro tipo", sua(func(o *operacao) { o.tipo = operacaoVenda }), false},
		{"outra quantidade", sua(func(o *operacao) { o.quantidade = v("11.0") }), false},
		{"já ligada", sua(func(o *operacao) { o.pluggyTransacaoId = p("x") }), false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if obtido := lancamentoIgual([]*operacao{c.o}, operacaoCompra, v("10.0"), dia); (obtido != nil) != c.casa {
				t.Errorf("lancamentoIgual = %v, esperado casar: %v", obtido, c.casa)
			}
		})
	}
}

func TestMovimentoDaCorretora(t *testing.T) {
	casos := []struct {
		nome     string
		o        operacao
		esperado bool
	}{
		{"da corretora", operacao{origem: origemOpenFinance, pluggyTransacaoId: p("mov-1")}, true},
		{"posição provisória", operacao{origem: origemOpenFinance, pluggyTransacaoId: p("posicao:x")}, false},
		{"sem id", operacao{origem: origemOpenFinance}, false},
		{"manual", operacao{origem: 1, pluggyTransacaoId: p("mov-1")}, false},
	}
	for _, c := range casos {
		if obtido := movimentoDaCorretora(&c.o); obtido != c.esperado {
			t.Errorf("%s: %v", c.nome, obtido)
		}
	}
}
