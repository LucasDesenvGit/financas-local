using Financas.Api.Entidades;

namespace Financas.Api.Servicos.Cambio;

/// <summary>
/// Converte valores para reais com as cotações guardadas no banco (sem consultar a internet).
/// Usa a taxa de compra da PTAX: é a que vale para avaliar um bem que está em moeda estrangeira.
/// Moedas sem cotação ficam registradas em <see cref="MoedasSemCotacao"/>, para quem somou avisar o usuário.
/// </summary>
public sealed class ConversorMoeda
{
    /// <summary>Quantos dias para trás uma cotação histórica ainda vale (fim de semana + feriado prolongado).</summary>
    private const int DiasValidadeHistorico = 7;

    private readonly Dictionary<string, decimal> _taxas;
    private readonly Dictionary<string, SortedList<DateOnly, decimal>> _historico;
    private readonly SortedSet<string> _semCotacao = new(StringComparer.Ordinal);

    public ConversorMoeda(IEnumerable<CotacaoMoeda> ultimas, IEnumerable<CotacaoMoeda>? historico = null)
    {
        _taxas = ultimas.ToDictionary(c => c.Moeda, c => c.Compra, StringComparer.Ordinal);
        _historico = (historico ?? Enumerable.Empty<CotacaoMoeda>())
            .GroupBy(c => c.Moeda, StringComparer.Ordinal)
            .ToDictionary(g => g.Key, g => new SortedList<DateOnly, decimal>(g.ToDictionary(c => c.Data, c => c.Compra)),
                StringComparer.Ordinal);
    }

    /// <summary>Moedas que apareceram numa conversão e não tinham cotação, em ordem alfabética.</summary>
    public IReadOnlyCollection<string> MoedasSemCotacao => _semCotacao;

    /// <summary>Reais por 1 unidade da moeda (última cotação), ou null se não houver cotação guardada.</summary>
    public decimal? Taxa(string? moeda)
    {
        var codigo = Normalizar(moeda);
        if (codigo == CambioService.MoedaBase) return 1m;
        if (_taxas.TryGetValue(codigo, out var taxa)) return taxa;
        _semCotacao.Add(codigo);
        return null;
    }

    /// <summary>Valor em reais pela última cotação, ou null se não houver cotação guardada para a moeda.</summary>
    public decimal? ParaReais(decimal valor, string? moeda)
        => Taxa(moeda) is decimal taxa ? Math.Round(valor * taxa, 2) : null;

    /// <summary>
    /// Valor em reais pela cotação da data (último dia útil anterior, até 7 dias). Sem histórico carregado para a data,
    /// usa a última cotação guardada.
    /// </summary>
    public decimal? ParaReais(decimal valor, string? moeda, DateOnly data)
    {
        var codigo = Normalizar(moeda);
        if (codigo == CambioService.MoedaBase) return valor;
        if (_historico.TryGetValue(codigo, out var serie) && TaxaAte(serie, data) is decimal taxa)
            return Math.Round(valor * taxa, 2);
        return ParaReais(valor, codigo);
    }

    private static string Normalizar(string? moeda) => CambioService.NormalizarMoeda(moeda) ?? CambioService.MoedaBase;

    /// <summary>Taxa do dia útil mais recente até a data (busca binária), dentro da validade.</summary>
    private static decimal? TaxaAte(SortedList<DateOnly, decimal> serie, DateOnly data)
    {
        var datas = serie.Keys;
        int inicio = 0, fim = datas.Count - 1, achado = -1;
        while (inicio <= fim)
        {
            var meio = (inicio + fim) / 2;
            if (datas[meio] <= data) { achado = meio; inicio = meio + 1; }
            else fim = meio - 1;
        }
        if (achado < 0 || datas[achado] < data.AddDays(-DiasValidadeHistorico)) return null;
        return serie.Values[achado];
    }
}
