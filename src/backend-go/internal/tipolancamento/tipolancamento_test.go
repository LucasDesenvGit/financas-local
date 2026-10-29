package tipolancamento

import (
	"testing"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

func txt(s string) *string { return &s }
func sim() *bool           { v := true; return &v }
func nao() *bool           { v := false; return &v }

// Regra: a ordem do TipoLancamentoService.Classificar (sistema → cartão → dinheiro → fatura pela conta → operação
// informada → descrição), palavras inteiras e "mesma titularidade".
func TestClassificar(t *testing.T) {
	casos := []struct {
		nome         string
		valor        string
		descricao    string
		original     *string
		meio         *string
		mesmoTitular *bool
		doSistema    bool
		tipoConta    int64
		esperado     Tipo
	}{
		{"lançada pelo sistema é entre contas", "-10", "qualquer", nil, nil, nil, true, ContaCorrente, EntreContas},
		{"no cartão, saída é compra", "-10", "LOJA", nil, nil, nil, false, CartaoCredito, CompraCartao},
		{"no cartão, entrada com PAGAMENTO é fatura", "10", "PAGAMENTO RECEBIDO", nil, nil, nil, false, CartaoCredito, PagamentoFatura},
		{"no cartão, entrada sem palavra de pagamento é estorno", "10", "ESTORNO LOJA", nil, nil, nil, false, CartaoCredito, EstornoCartao},
		{"no cartão, PAG não acha PAGAMENTO (palavra inteira)", "10", "PAGAMENTOS DIVERSOS", nil, nil, nil, false, CartaoCredito, EstornoCartao},
		{"no cartão, anuidade é tarifa", "-10", "ANUIDADE DIFERENCIADA", nil, nil, nil, false, CartaoCredito, TarifaJuros},
		{"no cartão, a operação informada vence o texto", "10", "PAGAMENTO", nil, txt(" estorno "), nil, false, CartaoCredito, EstornoCartao},
		{"no cartão, PAGAMENTO informado de saída é compra", "-10", "x", nil, txt("PAGAMENTO"), nil, false, CartaoCredito, CompraCartao},
		{"conta de dinheiro", "-10", "PIX", nil, nil, nil, false, ContaDinheiro, Dinheiro},
		{"fatura pela conta, pelo texto", "-10", "PAGTO FATURA NUBANK", nil, txt("BOLETO"), nil, false, ContaCorrente, PagamentoFatura},
		{"fatura pela conta, pela expressão", "-10", "PAG CARTAO", nil, nil, nil, false, ContaCorrente, PagamentoFatura},
		{"fatura sem pagamento não é fatura (conta de luz)", "-10", "FATURA ENERGIA", nil, nil, nil, false, ContaCorrente, Outro},
		{"PIX informado, saída", "-10", "x", nil, txt("pix"), nil, false, ContaCorrente, PixEnviado},
		{"PIX informado com mesmo titular é entre contas", "10", "x", nil, txt("PIX"), sim(), false, ContaCorrente, EntreContas},
		{"PIX informado com outro titular fica PIX", "10", "x", nil, txt("PIX"), nao(), false, ContaCorrente, PixRecebido},
		{"PAGAMENTO_FATURA de entrada vira outro e cai na descrição", "10", "PIX RECEBIDO", nil, txt("PAGAMENTO_FATURA"), nil, false, ContaCorrente, PixRecebido},
		{"CARTAO de entrada cai na descrição", "10", "DEPOSITO", nil, txt("CARTAO"), nil, false, ContaCorrente, Deposito},
		{"operação desconhecida cai na descrição", "-10", "TED ENVIADA", nil, txt("XYZ"), nil, false, ContaCorrente, TransferenciaEnviada},
		{"pela descrição original, não pelo nome dado", "-10", "Aluguel", txt("PIX ENVIADO IMOBILIARIA"), nil, nil, false, ContaCorrente, PixEnviado},
		{"descrição com acento e pontuação", "-10", "Débito automático - Energia", nil, nil, nil, false, ContaCorrente, Boleto},
		{"rendimento", "1", "RENDIMENTO POUPANCA", nil, nil, nil, false, ContaCorrente, Rendimento},
		{"compra no débito só de saída", "10", "COMPRA DEBITO", nil, nil, nil, false, ContaCorrente, Outro},
		{"nada indica", "-10", "LOJA FICTICIA", nil, nil, nil, false, ContaCorrente, Outro},
		{"tarifa", "-10", "TARIFA PACOTE DE SERVICOS", nil, nil, nil, false, ContaCorrente, TarifaJuros},
		{"TED informado de entrada", "10", "x", nil, txt("TED"), nil, false, ContaCorrente, TransferenciaRecebida},
		{"transferência na mesma instituição", "-10", "x", nil, txt("TRANSFERENCIA_MESMA_INSTITUICAO"), nil, false, ContaCorrente, TransferenciaEnviada},
		{"TED informado com mesmo titular", "-10", "x", nil, txt("DOC"), sim(), false, ContaCorrente, EntreContas},
		{"convênio é boleto", "-10", "x", nil, txt("CONVENIO_ARRECADACAO"), nil, false, ContaCorrente, Boleto},
		{"cartão de débito de saída", "-10", "x", nil, txt("CARTAO"), nil, false, ContaCorrente, CompraDebito},
		{"saque informado", "-10", "x", nil, txt("SAQUE"), nil, false, ContaCorrente, Saque},
		{"depósito informado", "10", "x", nil, txt("DEPOSITO"), nil, false, ContaCorrente, Deposito},
		{"tarifa avulsa informada", "-10", "x", nil, txt("TARIFA_SERVICOS_AVULSOS"), nil, false, ContaCorrente, TarifaJuros},
		{"juros do cheque especial", "-10", "x", nil, txt("ENCARGOS_JUROS_CHEQUE_ESPECIAL"), nil, false, ContaCorrente, TarifaJuros},
		{"rendimento informado", "10", "x", nil, txt("RENDIMENTO_APLIC_FINANCEIRA"), nil, false, ContaCorrente, Rendimento},
		{"resgate informado", "10", "x", nil, txt("RESGATE_APLIC_FINANCEIRA"), nil, false, ContaCorrente, AplicacaoResgate},
		{"portabilidade de salário", "10", "x", nil, txt("PORTABILIDADE_SALARIO"), nil, false, ContaCorrente, Salario},
		{"crédito informado", "10", "x", nil, txt("OPERACAO_CREDITO"), nil, false, ContaCorrente, Credito},
		{"PAGAMENTO_FATURA informado de saída", "-10", "x", nil, txt("PAGAMENTO_FATURA"), nil, false, ContaCorrente, PagamentoFatura},
		{"no cartão, fatura informada", "10", "x", nil, txt("PAGAMENTO_FATURA"), nil, false, CartaoCredito, PagamentoFatura},
		{"no cartão, cashback", "10", "x", nil, txt("CASHBACK"), nil, false, CartaoCredito, EstornoCartao},
		{"no cartão, tarifa informada", "-10", "x", nil, txt("TARIFA"), nil, false, CartaoCredito, TarifaJuros},
		{"no cartão, crédito contratado", "-10", "x", nil, txt("OPERACOES_CREDITO_CONTRATADAS_CARTAO"), nil, false, CartaoCredito, Credito},
		{"no cartão, PAGAMENTO informado de entrada é estorno", "10", "x", nil, txt("PAGAMENTO"), nil, false, CartaoCredito, EstornoCartao},
		{"pela descrição, PIX com mesmo titular", "-10", "PIX ENVIADO", nil, nil, sim(), false, ContaCorrente, EntreContas},
		{"pela descrição, PIX recebido", "10", "PIX JOAO", nil, nil, nil, false, ContaCorrente, PixRecebido},
		{"pela descrição, TRANSF com mesmo titular", "-10", "TRANSF ENTRE CONTAS", nil, nil, sim(), false, ContaCorrente, EntreContas},
		{"pela descrição, TED recebida", "10", "TED RECEBIDA", nil, nil, nil, false, ContaCorrente, TransferenciaRecebida},
		{"pela descrição, saque", "-10", "SAQUE 24H", nil, nil, nil, false, ContaCorrente, Saque},
		{"pela descrição, depósito", "10", "DEPOSITO EM DINHEIRO", nil, nil, nil, false, ContaCorrente, Deposito},
		{"pela descrição, resgate", "10", "RESGATE CDB", nil, nil, nil, false, ContaCorrente, AplicacaoResgate},
		{"pela descrição, salário", "10", "SALARIO EMPRESA", nil, nil, nil, false, ContaCorrente, Salario},
		{"pela descrição, compra no débito", "-10", "COMPRA DEBITO MERCADO", nil, nil, nil, false, ContaCorrente, CompraDebito},
		{"PIX enviado com mesmo titular é entre contas", "-10", "x", nil, txt("PIX"), sim(), false, ContaCorrente, EntreContas},
		{"TED recebida com mesmo titular é entre contas", "10", "x", nil, txt("TED"), sim(), false, ContaCorrente, EntreContas},
		{"boleto com mesmo titular continua boleto", "-10", "x", nil, txt("BOLETO"), sim(), false, ContaCorrente, Boleto},
		{"saque com mesmo titular continua saque", "-10", "x", nil, txt("SAQUE"), sim(), false, ContaCorrente, Saque},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			v, err := tipos.NovoDecimal(c.valor)
			if err != nil {
				t.Fatal(err)
			}
			if got := Classificar(v, c.descricao, c.original, c.meio, c.mesmoTitular, c.doSistema, c.tipoConta); got != c.esperado {
				t.Errorf("Classificar = %d, esperado %d", got, c.esperado)
			}
		})
	}
}
