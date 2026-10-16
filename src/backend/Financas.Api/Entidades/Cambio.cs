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
