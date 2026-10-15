using System.Globalization;
using System.Text.Json;
using Financas.Api.Data;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Investimentos;

/// <summary>Série de fatores acumulados da Selic diária, para comparar rentabilidades.</summary>
public sealed class SerieSelic
{
    private readonly List<DateOnly> _datas;
    private readonly double[] _acumulado; // _acumulado[i] = produto de (1 + taxa) de todas as datas anteriores a _datas[i]

    public SerieSelic(IEnumerable<TaxaSelicDiaria> taxas)
    {
        var ordenadas = taxas.OrderBy(t => t.Data).ToList();
        _datas = ordenadas.Select(t => t.Data).ToList();
        _acumulado = new double[ordenadas.Count + 1];
        _acumulado[0] = 1d;
        for (var i = 0; i < ordenadas.Count; i++)
            _acumulado[i + 1] = _acumulado[i] * (1d + (double)ordenadas[i].Taxa / 100d);
    }

    public bool Vazia => _datas.Count == 0;
    public DateOnly? PrimeiraData => _datas.Count > 0 ? _datas[0] : null;
    public DateOnly? UltimaData => _datas.Count > 0 ? _datas[^1] : null;

    /// <summary>Fator de correção da Selic aplicando as taxas dos dias em [de, ate).</summary>
    public decimal Fator(DateOnly de, DateOnly ate)
    {
        if (ate <= de || _datas.Count == 0) return 1m;
        var i = IndiceInferior(de);
        var j = IndiceInferior(ate);
        return (decimal)(_acumulado[j] / _acumulado[i]);
    }

    private int IndiceInferior(DateOnly data)
    {
        var indice = _datas.BinarySearch(data);
        return indice >= 0 ? indice : ~indice;
    }
}

/// <summary>
/// Taxa Selic oficial do Banco Central (API pública do SGS, sem chave).
/// Série 11 = Selic diária (% a.d.), série 432 = meta Selic (% a.a.). Os dados ficam em cache no banco local.
/// Somente datas são enviadas ao Banco Central; nenhum dado seu sai do computador.
/// </summary>
public class SelicService
{
    private const string ChaveMeta = "Selic.Meta";
    private const string ChaveMetaEm = "Selic.MetaConsultadaEm";
    private const string ChaveUltimaConsulta = "Selic.UltimaConsulta";

    private readonly HttpClient _http;
    private readonly FinancasDbContext _db;
    private readonly ConfiguracaoService _config;
    private readonly ILogger<SelicService> _logger;

    public SelicService(HttpClient http, FinancasDbContext db, ConfiguracaoService config, ILogger<SelicService> logger)
    {
        _http = http;
        _db = db;
        _config = config;
        _logger = logger;
    }

    /// <summary>Garante o cache desde <paramref name="inicio"/> até hoje e devolve a série.</summary>
    public async Task<SerieSelic> ObterSerieAsync(DateOnly inicio, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        if (inicio > hoje) inicio = hoje;

        var minimo = await _db.TaxasSelic.MinAsync(t => (DateOnly?)t.Data, ct);
        var maximo = await _db.TaxasSelic.MaxAsync(t => (DateOnly?)t.Data, ct);

        var faixas = new List<(DateOnly De, DateOnly Ate)>();
        if (minimo is null || maximo is null)
        {
            faixas.Add((inicio, hoje));
        }
        else
        {
            // Começo: só busca se faltar mais que um feriado prolongado.
            if (inicio < minimo.Value.AddDays(-5)) faixas.Add((inicio, minimo.Value.AddDays(-1)));
            var ultimaConsulta = await _config.ObterDataAsync(ChaveUltimaConsulta, ct);
            var consultarFim = maximo.Value < hoje.AddDays(-1)
                && (ultimaConsulta is null || DateTime.UtcNow - ultimaConsulta.Value.ToUniversalTime() > TimeSpan.FromHours(6));
            if (consultarFim) faixas.Add((maximo.Value.AddDays(1), hoje));
        }

        if (faixas.Count > 0)
        {
            try
            {
                foreach (var (de, ate) in faixas)
                {
                    // A API aceita no máximo 10 anos por consulta.
                    for (var parteInicio = de; parteInicio <= ate; parteInicio = parteInicio.AddYears(9))
                    {
                        var parteFim = parteInicio.AddYears(9).AddDays(-1);
                        if (parteFim > ate) parteFim = ate;
                        await BaixarAsync(parteInicio, parteFim, ct);
                    }
                }
                await _config.DefinirAsync(ChaveUltimaConsulta, DateTime.UtcNow.ToString("O"), ct: ct);
                await _db.SaveChangesAsync(ct);
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                // Sem internet ou API indisponível: segue com o que já está em cache.
                _logger.LogWarning(ex, "Não foi possível atualizar a Selic no Banco Central.");
                _db.ChangeTracker.Clear();
            }
        }

        var taxas = await _db.TaxasSelic.AsNoTracking().Where(t => t.Data >= inicio).ToListAsync(ct);
        return new SerieSelic(taxas);
    }

    /// <summary>Meta Selic atual (% a.a.), consultada no máximo uma vez a cada 12 horas.</summary>
    public async Task<decimal?> MetaAtualAsync(CancellationToken ct)
    {
        var consultadaEm = await _config.ObterDataAsync(ChaveMetaEm, ct);
        var atual = await _config.ObterAsync(ChaveMeta, ct);
        if (atual is not null && consultadaEm is not null
            && DateTime.UtcNow - consultadaEm.Value.ToUniversalTime() < TimeSpan.FromHours(12))
            return decimal.TryParse(atual, NumberStyles.Number, CultureInfo.InvariantCulture, out var v) ? v : null;

        try
        {
            using var resp = await _http.GetAsync("dados/serie/bcdata.sgs.432/dados/ultimos/1?formato=json", ct);
            resp.EnsureSuccessStatusCode();
            using var doc = JsonDocument.Parse(await resp.Content.ReadAsStringAsync(ct));
            var valor = doc.RootElement.EnumerateArray().Select(e => e.GetProperty("valor").GetString()).LastOrDefault();
            if (decimal.TryParse(valor, NumberStyles.Number, CultureInfo.InvariantCulture, out var meta))
            {
                await _config.DefinirAsync(ChaveMeta, meta.ToString(CultureInfo.InvariantCulture), ct: ct);
                await _config.DefinirAsync(ChaveMetaEm, DateTime.UtcNow.ToString("O"), ct: ct);
                await _db.SaveChangesAsync(ct);
                return meta;
            }
        }
        catch (Exception ex) when (ex is not OperationCanceledException)
        {
            _logger.LogWarning(ex, "Não foi possível consultar a meta Selic.");
        }
        return decimal.TryParse(atual, NumberStyles.Number, CultureInfo.InvariantCulture, out var anterior) ? anterior : null;
    }

    private async Task BaixarAsync(DateOnly de, DateOnly ate, CancellationToken ct)
    {
        var url = $"dados/serie/bcdata.sgs.11/dados?formato=json&dataInicial={de:dd/MM/yyyy}&dataFinal={ate:dd/MM/yyyy}";
        using var resp = await _http.GetAsync(url, ct);
        if (resp.StatusCode == System.Net.HttpStatusCode.NotFound) return; // período sem dias úteis
        resp.EnsureSuccessStatusCode();

        using var doc = JsonDocument.Parse(await resp.Content.ReadAsStringAsync(ct));
        if (doc.RootElement.ValueKind != JsonValueKind.Array) return;

        var existentes = (await _db.TaxasSelic.AsNoTracking()
                .Where(t => t.Data >= de && t.Data <= ate).Select(t => t.Data).ToListAsync(ct))
            .ToHashSet();

        foreach (var item in doc.RootElement.EnumerateArray())
        {
            var dataTexto = item.TryGetProperty("data", out var d) ? d.GetString() : null;
            var valorTexto = item.TryGetProperty("valor", out var v) ? v.GetString() : null;
            if (!DateOnly.TryParseExact(dataTexto, "dd/MM/yyyy", CultureInfo.InvariantCulture, DateTimeStyles.None, out var data))
                continue;
            if (!decimal.TryParse(valorTexto, NumberStyles.Number, CultureInfo.InvariantCulture, out var taxa)) continue;
            if (!existentes.Add(data)) continue;
            _db.TaxasSelic.Add(new TaxaSelicDiaria { Data = data, Taxa = taxa });
        }
        await _db.SaveChangesAsync(ct);
    }
}
