using System.Text.Json.Serialization;

namespace Financas.Api.Servicos.Pluggy;

/* Modelos da API da Pluggy (https://api.pluggy.ai). Apenas os campos utilizados. */

public class PluggyAuthResposta
{
    [JsonPropertyName("apiKey")] public string? ApiKey { get; set; }
}

public class PluggyPagina<T>
{
    [JsonPropertyName("results")] public List<T> Results { get; set; } = new();
    [JsonPropertyName("page")] public int Page { get; set; }
    [JsonPropertyName("total")] public int Total { get; set; }
    [JsonPropertyName("totalPages")] public int TotalPages { get; set; }
}

public class PluggyPaginaCursor<T>
{
    [JsonPropertyName("results")] public List<T> Results { get; set; } = new();
    /// <summary>Link para a próxima página (contém o parâmetro "after") ou null.</summary>
    [JsonPropertyName("next")] public string? Next { get; set; }
}

public class PluggyConector
{
    [JsonPropertyName("id")] public long Id { get; set; }
    [JsonPropertyName("name")] public string? Name { get; set; }
    [JsonPropertyName("primaryColor")] public string? PrimaryColor { get; set; }
    [JsonPropertyName("isOpenFinance")] public bool? IsOpenFinance { get; set; }
}

public class PluggyErroExecucao
{
    [JsonPropertyName("code")] public string? Code { get; set; }
    [JsonPropertyName("message")] public string? Message { get; set; }
}

public class PluggyItem
{
    [JsonPropertyName("id")] public string Id { get; set; } = string.Empty;
    [JsonPropertyName("connector")] public PluggyConector? Connector { get; set; }
    [JsonPropertyName("status")] public string? Status { get; set; }
    [JsonPropertyName("executionStatus")] public string? ExecutionStatus { get; set; }
    [JsonPropertyName("lastUpdatedAt")] public DateTimeOffset? LastUpdatedAt { get; set; }
    [JsonPropertyName("consentExpiresAt")] public DateTimeOffset? ConsentExpiresAt { get; set; }
    [JsonPropertyName("error")] public PluggyErroExecucao? Error { get; set; }
}

public class PluggyDadosCredito
{
    [JsonPropertyName("creditLimit")] public double? CreditLimit { get; set; }
    [JsonPropertyName("availableCreditLimit")] public double? AvailableCreditLimit { get; set; }
    [JsonPropertyName("balanceCloseDate")] public DateTimeOffset? BalanceCloseDate { get; set; }
    [JsonPropertyName("balanceDueDate")] public DateTimeOffset? BalanceDueDate { get; set; }
}

public class PluggyConta
{
    [JsonPropertyName("id")] public string Id { get; set; } = string.Empty;
    [JsonPropertyName("itemId")] public string? ItemId { get; set; }
    /// <summary>BANK ou CREDIT</summary>
    [JsonPropertyName("type")] public string? Type { get; set; }
    /// <summary>CHECKING_ACCOUNT, SAVINGS_ACCOUNT ou CREDIT_CARD</summary>
    [JsonPropertyName("subtype")] public string? Subtype { get; set; }
    [JsonPropertyName("number")] public string? Number { get; set; }
    [JsonPropertyName("name")] public string? Name { get; set; }
    [JsonPropertyName("marketingName")] public string? MarketingName { get; set; }
    [JsonPropertyName("balance")] public double? Balance { get; set; }
    [JsonPropertyName("currencyCode")] public string? CurrencyCode { get; set; }
    [JsonPropertyName("creditData")] public PluggyDadosCredito? CreditData { get; set; }
}

public class PluggyMetadadosCartao
{
    [JsonPropertyName("installmentNumber")] public int? InstallmentNumber { get; set; }
    [JsonPropertyName("totalInstallments")] public int? TotalInstallments { get; set; }
    [JsonPropertyName("billId")] public string? BillId { get; set; }
}

public class PluggyEstabelecimento
{
    [JsonPropertyName("name")] public string? Name { get; set; }
    [JsonPropertyName("businessName")] public string? BusinessName { get; set; }
}

public class PluggyTransacao
{
    [JsonPropertyName("id")] public string Id { get; set; } = string.Empty;
    [JsonPropertyName("accountId")] public string? AccountId { get; set; }
    [JsonPropertyName("date")] public DateTimeOffset? Date { get; set; }
    [JsonPropertyName("description")] public string? Description { get; set; }
    [JsonPropertyName("descriptionRaw")] public string? DescriptionRaw { get; set; }
    [JsonPropertyName("amount")] public double? Amount { get; set; }
    [JsonPropertyName("amountInAccountCurrency")] public double? AmountInAccountCurrency { get; set; }
    /// <summary>DEBIT (saída) ou CREDIT (entrada)</summary>
    [JsonPropertyName("type")] public string? Type { get; set; }
    /// <summary>POSTED ou PENDING</summary>
    [JsonPropertyName("status")] public string? Status { get; set; }
    [JsonPropertyName("category")] public string? Category { get; set; }
    [JsonPropertyName("categoryId")] public string? CategoryId { get; set; }
    [JsonPropertyName("creditCardMetadata")] public PluggyMetadadosCartao? CreditCardMetadata { get; set; }
    [JsonPropertyName("merchant")] public PluggyEstabelecimento? Merchant { get; set; }
}

public class PluggyPagamentoFatura
{
    [JsonPropertyName("amount")] public double? Amount { get; set; }
}

public class PluggyFatura
{
    [JsonPropertyName("id")] public string Id { get; set; } = string.Empty;
    [JsonPropertyName("dueDate")] public DateTimeOffset? DueDate { get; set; }
    [JsonPropertyName("billClosingDate")] public DateTimeOffset? BillClosingDate { get; set; }
    [JsonPropertyName("totalAmount")] public double? TotalAmount { get; set; }
    [JsonPropertyName("minimumPaymentAmount")] public double? MinimumPaymentAmount { get; set; }
    [JsonPropertyName("payments")] public List<PluggyPagamentoFatura>? Payments { get; set; }
}

public class PluggyInvestimento
{
    [JsonPropertyName("id")] public string Id { get; set; } = string.Empty;
    [JsonPropertyName("name")] public string? Name { get; set; }
    [JsonPropertyName("type")] public string? Type { get; set; }
    [JsonPropertyName("subtype")] public string? Subtype { get; set; }
    [JsonPropertyName("code")] public string? Code { get; set; }
    [JsonPropertyName("issuer")] public string? Issuer { get; set; }
    [JsonPropertyName("currencyCode")] public string? CurrencyCode { get; set; }
    [JsonPropertyName("balance")] public double? Balance { get; set; }
    [JsonPropertyName("amountOriginal")] public double? AmountOriginal { get; set; }
    [JsonPropertyName("amountProfit")] public double? AmountProfit { get; set; }
    [JsonPropertyName("quantity")] public double? Quantity { get; set; }
    /// <summary>Valor unitário atual (cota/ação), quando a instituição informa.</summary>
    [JsonPropertyName("value")] public double? Value { get; set; }
    [JsonPropertyName("annualRate")] public double? AnnualRate { get; set; }
    [JsonPropertyName("fixedAnnualRate")] public double? FixedAnnualRate { get; set; }
    [JsonPropertyName("dueDate")] public DateTimeOffset? DueDate { get; set; }
    [JsonPropertyName("purchaseDate")] public DateTimeOffset? PurchaseDate { get; set; }
    [JsonPropertyName("issueDate")] public DateTimeOffset? IssueDate { get; set; }
    [JsonPropertyName("status")] public string? Status { get; set; }
}

/// <summary>Movimentação de um investimento (GET /investments/{id}/transactions).</summary>
public class PluggyMovimentoInvestimento
{
    [JsonPropertyName("id")] public string Id { get; set; } = string.Empty;
    /// <summary>BUY, SELL, TAX, TRANSFER...</summary>
    [JsonPropertyName("type")] public string? Type { get; set; }
    [JsonPropertyName("movementType")] public string? MovementType { get; set; }
    [JsonPropertyName("quantity")] public double? Quantity { get; set; }
    /// <summary>Preço unitário.</summary>
    [JsonPropertyName("value")] public double? Value { get; set; }
    [JsonPropertyName("amount")] public double? Amount { get; set; }
    [JsonPropertyName("netAmount")] public double? NetAmount { get; set; }
    [JsonPropertyName("date")] public DateTimeOffset? Date { get; set; }
    [JsonPropertyName("tradeDate")] public DateTimeOffset? TradeDate { get; set; }
    [JsonPropertyName("description")] public string? Description { get; set; }
    [JsonPropertyName("brokerageNumber")] public string? BrokerageNumber { get; set; }
    [JsonPropertyName("expenses")] public Dictionary<string, double?>? Expenses { get; set; }
}

public class PluggyCategoria
{
    [JsonPropertyName("id")] public string Id { get; set; } = string.Empty;
    [JsonPropertyName("description")] public string? Description { get; set; }
    [JsonPropertyName("descriptionTranslated")] public string? DescriptionTranslated { get; set; }
    [JsonPropertyName("parentId")] public string? ParentId { get; set; }
    [JsonPropertyName("parentDescription")] public string? ParentDescription { get; set; }
}

public class PluggyException : Exception
{
    public int? StatusCode { get; }
    public PluggyException(string mensagem, int? statusCode = null, Exception? interna = null)
        : base(mensagem, interna) => StatusCode = statusCode;
}
