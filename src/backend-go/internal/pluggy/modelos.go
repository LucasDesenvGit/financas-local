package pluggy

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"time"

	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/relogio"
	"github.com/LucasDesenvGit/financas-local/src/backend-go/internal/tipos"
)

// Os modelos da API da Pluggy (https://api.pluggy.ai), com os nomes dela e só os campos usados, como o PluggyModelos.cs.
// Campo ausente ou null é nil. Nenhum erro de leitura traz o valor lido: ele pode ser um dado bancário.

type paginaNumerada[T any] struct {
	Results    []T     `json:"results"`
	TotalPages Inteiro `json:"totalPages"`
}

type paginaComCursor struct {
	Results []Transacao `json:"results"`
	Next    *string     `json:"next"` // o link da próxima página, com o parâmetro "after"
}

func (p *paginaComCursor) proximo() string {
	if p.Next == nil {
		return ""
	}
	return *p.Next
}

// Numero é o double? do .NET com AllowReadingFromString: vem como número ou como texto. Fica como veio, em texto: o
// dinheiro só passa pelo double dentro do Decimal.
type Numero string

var formatoDoNumero = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// UnmarshalJSON lê o número ou o texto com um número.
func (n *Numero) UnmarshalJSON(b []byte) error {
	texto, err := textoDoNumero(b)
	if err != nil {
		return err
	}
	if !formatoDoNumero.MatchString(texto) {
		return errors.New("número da Pluggy inválido")
	}
	*n = Numero(texto)
	return nil
}

// Decimal é o (decimal)double do .NET: o C# lê todo valor da Pluggy em double e o converte, guardando 15 dígitos.
// Passar pelo double é o que dá o mesmo centavo: 19.884999999999998 vira 19.885 e arredonda para 19,89, e o decimal
// direto do texto daria 19,88. O float64 não sai deste arquivo.
func (n Numero) Decimal() (tipos.Decimal, error) {
	v, err := n.double()
	if err != nil {
		return tipos.Decimal{}, err
	}
	return tipos.DecimalDeFloat64(v)
}

func (n Numero) double() (float64, error) {
	v, err := strconv.ParseFloat(string(n), 64)
	if err != nil {
		return 0, errors.New("número da Pluggy fora do limite")
	}
	return v, nil
}

// SomaDosPagamentos é o pagamentos.Sum(p => p.Amount ?? 0) do C#: a soma é em double, e só o total vira decimal.
func SomaDosPagamentos(lista []PagamentoFatura) (tipos.Decimal, error) {
	soma := 0.0
	for _, p := range lista {
		if p.Amount == nil {
			continue
		}
		v, err := p.Amount.double()
		if err != nil {
			return tipos.Decimal{}, err
		}
		soma += v
	}
	return tipos.DecimalDeFloat64(soma)
}

// Inteiro é o int do .NET com AllowReadingFromString.
type Inteiro int32

// UnmarshalJSON lê o número ou o texto com um número inteiro.
func (n *Inteiro) UnmarshalJSON(b []byte) error {
	texto, err := textoDoNumero(b)
	if err != nil {
		return err
	}
	v, err := strconv.ParseInt(texto, 10, 32)
	if err != nil {
		return errors.New("número inteiro da Pluggy inválido")
	}
	*n = Inteiro(v)
	return nil
}

// textoDoNumero tira as aspas do número.
func textoDoNumero(b []byte) (string, error) {
	if len(b) == 0 || b[0] != '"' {
		return string(b), nil
	}
	var texto string
	if err := json.Unmarshal(b, &texto); err != nil {
		return "", errors.New("número da Pluggy inválido")
	}
	return texto, nil
}

// Instante é o DateTimeOffset? do .NET, nos formatos que o System.Text.Json lê. Sem fuso, vale a hora de Brasília.
type Instante struct{ t time.Time }

var (
	formatosComFuso = []string{"2006-01-02T15:04:05Z07:00", "2006-01-02T15:04Z07:00", "2006-01-02T15:04:05Z07", "2006-01-02T15:04Z07"}
	formatosSemFuso = []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"}
)

// UnmarshalJSON lê a data.
func (i *Instante) UnmarshalJSON(b []byte) error {
	var texto string
	if err := json.Unmarshal(b, &texto); err != nil {
		return errors.New("data da Pluggy inválida")
	}
	for _, formato := range formatosComFuso {
		if t, err := time.Parse(formato, texto); err == nil {
			i.t = t
			return nil
		}
	}
	for _, formato := range formatosSemFuso {
		if t, err := time.ParseInLocation(formato, texto, relogio.Brasilia); err == nil {
			i.t = t
			return nil
		}
	}
	return errors.New("data da Pluggy inválida")
}

// UTC é o UtcDateTime do .NET.
func (i Instante) UTC() time.Time { return i.t.UTC() }

// Data é o Relogio.ParaDataLocal do .NET: o dia em Brasília.
func (i Instante) Data() tipos.Data { return tipos.DataDe(relogio.ParaHoraLocal(i.t)) }

// Conector é a instituição do item.
type Conector struct {
	Name         *string `json:"name"`
	PrimaryColor *string `json:"primaryColor"`
}

// ErroExecucao é o erro da última execução do item na Pluggy.
type ErroExecucao struct {
	Message *string `json:"message"`
}

// Item é uma conexão do Meu Pluggy.
type Item struct {
	Id               string        `json:"id"`
	Connector        *Conector     `json:"connector"`
	Status           *string       `json:"status"`
	ExecutionStatus  *string       `json:"executionStatus"`
	LastUpdatedAt    *Instante     `json:"lastUpdatedAt"`
	ConsentExpiresAt *Instante     `json:"consentExpiresAt"`
	Error            *ErroExecucao `json:"error"`
}

// DadosCredito são os dados do cartão.
type DadosCredito struct {
	CreditLimit          *Numero   `json:"creditLimit"`
	AvailableCreditLimit *Numero   `json:"availableCreditLimit"`
	BalanceCloseDate     *Instante `json:"balanceCloseDate"`
	BalanceDueDate       *Instante `json:"balanceDueDate"`
}

// DadosBancarios é o Account.bankData (#168). Cada item de ReservedBalances fica como veio: a documentação não detalha
// os campos, e um formato novo não pode derrubar a leitura da conta.
type DadosBancarios struct {
	ClosingBalance               *Numero           `json:"closingBalance"`
	AutomaticallyInvestedBalance *Numero           `json:"automaticallyInvestedBalance"`
	HasReservedBalance           *bool             `json:"hasReservedBalance"`
	ReservedBalances             []json.RawMessage `json:"reservedBalances"`
}

// Conta é uma conta ou um cartão do item. Type é BANK ou CREDIT.
type Conta struct {
	Id            string          `json:"id"`
	Type          *string         `json:"type"`
	Subtype       *string         `json:"subtype"`
	Number        *string         `json:"number"`
	Name          *string         `json:"name"`
	MarketingName *string         `json:"marketingName"`
	Balance       *Numero         `json:"balance"`
	CurrencyCode  *string         `json:"currencyCode"`
	CreditData    *DadosCredito   `json:"creditData"`
	BankData      *DadosBancarios `json:"bankData"`
}

// MetadadosCartao são as parcelas e a fatura da transação no cartão.
type MetadadosCartao struct {
	InstallmentNumber *Inteiro `json:"installmentNumber"`
	TotalInstallments *Inteiro `json:"totalInstallments"`
	BillId            *string  `json:"billId"`
}

// Estabelecimento é onde a compra foi feita.
type Estabelecimento struct {
	Name         *string `json:"name"`
	BusinessName *string `json:"businessName"`
}

// Participante é o pagador ou o recebedor. DocumentNumber fica como veio: pela documentação é { type, value }, mas um
// banco que mande outro formato não pode derrubar a página de transações.
type Participante struct {
	DocumentNumber json.RawMessage `json:"documentNumber"`
}

// Documento é o número do documento, venha como objeto ({ value }) ou como texto. Vazio: não deu para ler.
func (p *Participante) Documento() string {
	if p == nil || len(p.DocumentNumber) == 0 {
		return ""
	}
	var texto *string
	if p.DocumentNumber[0] == '"' {
		_ = json.Unmarshal(p.DocumentNumber, &texto)
	} else if p.DocumentNumber[0] == '{' {
		var objeto struct {
			Value json.RawMessage `json:"value"`
		}
		if json.Unmarshal(p.DocumentNumber, &objeto) == nil && len(objeto.Value) > 0 && objeto.Value[0] == '"' {
			_ = json.Unmarshal(objeto.Value, &texto)
		}
	}
	if texto == nil {
		return ""
	}
	return *texto
}

// DadosPagamento são lidos só para saber se pagador e recebedor são a mesma pessoa e descartados: nome e documento não
// são gravados (#111, ADR 0025).
type DadosPagamento struct {
	PaymentMethod *string       `json:"paymentMethod"`
	Payer         *Participante `json:"payer"`
	Receiver      *Participante `json:"receiver"`
}

// Transacao é uma transação. Type é DEBIT (saída) ou CREDIT (entrada); Status é POSTED ou PENDING.
type Transacao struct {
	Id                      string           `json:"id"`
	Date                    *Instante        `json:"date"`
	Description             *string          `json:"description"`
	Amount                  *Numero          `json:"amount"`
	AmountInAccountCurrency *Numero          `json:"amountInAccountCurrency"`
	Type                    *string          `json:"type"`
	Status                  *string          `json:"status"`
	CategoryId              *string          `json:"categoryId"`
	CreditCardMetadata      *MetadadosCartao `json:"creditCardMetadata"`
	Merchant                *Estabelecimento `json:"merchant"`
	OperationType           *string          `json:"operationType"`
	PaymentData             *DadosPagamento  `json:"paymentData"`
}

// PagamentoFatura é um pagamento da fatura.
type PagamentoFatura struct {
	Amount *Numero `json:"amount"`
}

// Fatura é uma fatura do cartão.
type Fatura struct {
	Id                   string            `json:"id"`
	DueDate              *Instante         `json:"dueDate"`
	BillClosingDate      *Instante         `json:"billClosingDate"`
	TotalAmount          *Numero           `json:"totalAmount"`
	MinimumPaymentAmount *Numero           `json:"minimumPaymentAmount"`
	Payments             []PagamentoFatura `json:"payments"`
}

// Investimento é um investimento do item.
type Investimento struct {
	Id              string    `json:"id"`
	Name            *string   `json:"name"`
	Type            *string   `json:"type"`
	Subtype         *string   `json:"subtype"`
	Code            *string   `json:"code"`
	Issuer          *string   `json:"issuer"`
	CurrencyCode    *string   `json:"currencyCode"`
	Balance         *Numero   `json:"balance"`
	AmountOriginal  *Numero   `json:"amountOriginal"`
	AmountProfit    *Numero   `json:"amountProfit"`
	Quantity        *Numero   `json:"quantity"`
	AnnualRate      *Numero   `json:"annualRate"`
	FixedAnnualRate *Numero   `json:"fixedAnnualRate"`
	DueDate         *Instante `json:"dueDate"`
	PurchaseDate    *Instante `json:"purchaseDate"`
	IssueDate       *Instante `json:"issueDate"`
	Status          *string   `json:"status"`
}

// MovimentoInvestimento é uma movimentação (GET /investments/{id}/transactions). Type é BUY, SELL, TAX, TRANSFER...;
// Value é o preço unitário.
type MovimentoInvestimento struct {
	Id        string             `json:"id"`
	Type      *string            `json:"type"`
	Quantity  *Numero            `json:"quantity"`
	Value     *Numero            `json:"value"`
	Amount    *Numero            `json:"amount"`
	Date      *Instante          `json:"date"`
	TradeDate *Instante          `json:"tradeDate"`
	Expenses  map[string]*Numero `json:"expenses"`
}

// TaxaEmprestimo é uma taxa do contrato. TaxPeriodicity é AA (ao ano) ou AM (ao mês).
type TaxaEmprestimo struct {
	TaxPeriodicity                *string `json:"taxPeriodicity"`
	ReferentialRateIndexerType    *string `json:"referentialRateIndexerType"`
	ReferentialRateIndexerSubType *string `json:"referentialRateIndexerSubType"`
	PreFixedRate                  *Numero `json:"preFixedRate"`
}

// ParcelasEmprestimo são as contagens de parcelas do contrato.
type ParcelasEmprestimo struct {
	TotalNumberOfInstallments *Inteiro `json:"totalNumberOfInstallments"`
	PaidInstallments          *Inteiro `json:"paidInstallments"`
	DueInstallments           *Inteiro `json:"dueInstallments"`
	PastDueInstallments       *Inteiro `json:"pastDueInstallments"`
}

// PagamentoEmprestimo é um pagamento feito.
type PagamentoEmprestimo struct {
	IsOverParcelPayment *bool     `json:"isOverParcelPayment"`
	PaidDate            *Instante `json:"paidDate"`
	PaidAmount          *Numero   `json:"paidAmount"`
}

// PagamentosEmprestimo são o saldo devedor e os pagamentos feitos.
type PagamentosEmprestimo struct {
	ContractOutstandingBalance *Numero               `json:"contractOutstandingBalance"`
	Releases                   []PagamentoEmprestimo `json:"releases"`
}

// Emprestimo é um contrato de crédito (GET /loans). Kind é a família (LOAN, FINANCING); Type, a modalidade do Open Finance.
type Emprestimo struct {
	Id                      string                `json:"id"`
	Kind                    *string               `json:"kind"`
	ContractNumber          *string               `json:"contractNumber"`
	ProductName             *string               `json:"productName"`
	Type                    *string               `json:"type"`
	ContractDate            *Instante             `json:"contractDate"`
	ContractAmount          *Numero               `json:"contractAmount"`
	CurrencyCode            *string               `json:"currencyCode"`
	DueDate                 *Instante             `json:"dueDate"`
	InstallmentPeriodicity  *string               `json:"installmentPeriodicity"`
	FirstInstallmentDueDate *Instante             `json:"firstInstallmentDueDate"`
	Cet                     *Numero               `json:"CET"`
	AmortizationScheduled   *string               `json:"amortizationScheduled"`
	InterestRates           []TaxaEmprestimo      `json:"interestRates"`
	Installments            *ParcelasEmprestimo   `json:"installments"`
	Payments                *PagamentosEmprestimo `json:"payments"`
}

// Categoria é uma categoria da Pluggy.
type Categoria struct {
	Id                    string  `json:"id"`
	Description           *string `json:"description"`
	DescriptionTranslated *string `json:"descriptionTranslated"`
	ParentId              *string `json:"parentId"`
	ParentDescription     *string `json:"parentDescription"`
}
