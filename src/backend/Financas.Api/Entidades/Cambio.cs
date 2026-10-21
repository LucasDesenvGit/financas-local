namespace Financas.Api.Entidades;

/// <summary>Cotação PTAX de fechamento de uma moeda em reais (cache local do Banco Central).</summary>
public class CotacaoMoeda
{
    /// <summary>Código ISO 4217 (ex.: USD, EUR).</summary>
    public string Moeda { get; set; } = string.Empty;
    /// <summary>Dia útil da cotação.</summary>
    public DateOnly Data { get; set; }
    /// <summary>Reais por 1 unidade da moeda — taxa de compra.</summary>
    public decimal Compra { get; set; }
    /// <summary>Reais por 1 unidade da moeda — taxa de venda.</summary>
    public decimal Venda { get; set; }
    /// <summary>Quando a cotação foi gravada ou corrigida pela última vez (UTC).</summary>
    public DateTime AtualizadoEm { get; set; } = DateTime.UtcNow;
}

/// <summary>
/// Cotação de criptomoeda em reais (ADR 0048): o último negócio no Mercado Bitcoin, guardado a cada 30 minutos pela
/// API (CriptoHostedService), por 30 dias. Cripto negocia 24 h por dia: a chave é moeda + data e hora da fonte.
/// </summary>
public class CotacaoCripto
{
    /// <summary>Código da criptomoeda (ex.: BTC, ETH).</summary>
    public string Moeda { get; set; } = string.Empty;
    /// <summary>Data e hora do último negócio, informada pela fonte (UTC).</summary>
    public DateTime DataHora { get; set; }
    /// <summary>Reais por 1 unidade da criptomoeda (último negócio).</summary>
    public decimal Preco { get; set; }
    /// <summary>Quando a cotação foi gravada (UTC).</summary>
    public DateTime AtualizadoEm { get; set; } = DateTime.UtcNow;
}
