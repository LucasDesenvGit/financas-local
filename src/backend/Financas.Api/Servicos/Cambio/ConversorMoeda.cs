using Financas.Api.Entidades;

namespace Financas.Api.Servicos.Cambio;

/// <summary>
/// Converte valores para reais com a última cotação guardada de cada moeda (sem consultar a internet).
/// Usa a taxa de compra da PTAX: é a que vale para avaliar um bem que está em moeda estrangeira.
/// Moedas sem cotação ficam registradas em <see cref="MoedasSemCotacao"/>, para quem somou avisar o usuário.
/// </summary>
public sealed class ConversorMoeda
{
    private readonly Dictionary<string, decimal> _taxas;
    private readonly SortedSet<string> _semCotacao = new(StringComparer.Ordinal);

    public ConversorMoeda(IEnumerable<CotacaoMoeda> ultimas)
        => _taxas = ultimas.ToDictionary(c => c.Moeda, c => c.Compra, StringComparer.Ordinal);

    /// <summary>Moedas que apareceram numa conversão e não tinham cotação, em ordem alfabética.</summary>
    public IReadOnlyCollection<string> MoedasSemCotacao => _semCotacao;

    /// <summary>Valor em reais, ou null se não houver cotação guardada para a moeda.</summary>
    public decimal? ParaReais(decimal valor, string? moeda)
    {
        var codigo = CambioService.NormalizarMoeda(moeda) ?? CambioService.MoedaBase;
        if (codigo == CambioService.MoedaBase) return valor;
        if (_taxas.TryGetValue(codigo, out var taxa)) return Math.Round(valor * taxa, 2);
        _semCotacao.Add(codigo);
        return null;
    }
}
