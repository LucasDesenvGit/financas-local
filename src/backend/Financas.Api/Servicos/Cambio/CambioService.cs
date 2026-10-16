using System.Globalization;
using System.Text.Json;
using System.Text.RegularExpressions;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Cambio;

/// <summary>
/// Cotação de moedas estrangeiras pela PTAX oficial do Banco Central (API pública Olinda, sem chave).
/// Guarda o boletim de fechamento de cada dia útil em cache no banco local.
/// Somente o código da moeda e datas são enviados; nenhum valor ou dado pessoal sai do computador.
/// </summary>
public record ResultadoAtualizacaoCambio(int Novas, int Corrigidas, List<string> Falhas);

public partial class CambioService
{
    public const string MoedaBase = "BRL";

    private const string ChaveMoedas = "Cambio.Moedas";
    private const string ChaveMoedasEm = "Cambio.MoedasConsultadasEm";
    private const string PrefixoUltimaConsulta = "Cambio.UltimaConsulta.";
    public const string ChaveUltimaAtualizacao = "Cambio.UltimaAtualizacao";
    private const string BoletimFechamento = "Fechamento";

    /// <summary>Quantos dias para trás procurar o último dia útil (cobre fim de semana + feriado prolongado).</summary>
    private const int DiasRetroativos = 7;

    private readonly HttpClient _http;
    private readonly FinancasDbContext _db;
    private readonly ConfiguracaoService _config;
    private readonly ILogger<CambioService> _logger;

    public CambioService(HttpClient http, FinancasDbContext db, ConfiguracaoService config, ILogger<CambioService> logger)
    {
        _http = http;
        _db = db;
        _config = config;
        _logger = logger;
    }

    [GeneratedRegex("^[A-Z]{3}$")]
    private static partial Regex FormatoMoeda();

    /// <summary>Normaliza e valida um código ISO 4217 (ex.: " usd" → "USD"). Retorna null se inválido.</summary>
    public static string? NormalizarMoeda(string? moeda)
    {
        var m = (moeda ?? string.Empty).Trim().ToUpperInvariant();
        return FormatoMoeda().IsMatch(m) ? m : null;
    }

    /// <summary>Moedas com PTAX publicada pelo Banco Central, consultadas no máximo uma vez por semana.</summary>
    public async Task<List<MoedaDto>> ListarMoedasAsync(CancellationToken ct)
    {
        var emCache = Desserializar(await _config.ObterAsync(ChaveMoedas, ct));
        var consultadaEm = await _config.ObterDataAsync(ChaveMoedasEm, ct);
        if (emCache.Count > 0 && consultadaEm is not null
            && DateTime.UtcNow - consultadaEm.Value.ToUniversalTime() < TimeSpan.FromDays(7))
            return emCache;

        try
        {
            using var resp = await _http.GetAsync("Moedas?$format=json", ct);
            resp.EnsureSuccessStatusCode();
            using var doc = JsonDocument.Parse(await resp.Content.ReadAsStringAsync(ct));
            var moedas = Valores(doc)
                .Select(e => new MoedaDto(Texto(e, "simbolo") ?? string.Empty, Texto(e, "nomeFormatado") ?? string.Empty))
                .Where(m => NormalizarMoeda(m.Codigo) is not null)
                .OrderBy(m => m.Codigo)
                .ToList();
            if (moedas.Count > 0)
            {
                await _config.DefinirAsync(ChaveMoedas, JsonSerializer.Serialize(moedas), ct: ct);
                await _config.DefinirAsync(ChaveMoedasEm, DateTime.UtcNow.ToString("O"), ct: ct);
                await _db.SaveChangesAsync(ct);
                return moedas;
            }
        }
        catch (Exception ex) when (ex is not OperationCanceledException)
        {
            // Sem internet ou API indisponível: segue com a lista em cache.
            _logger.LogWarning(ex, "Não foi possível consultar a lista de moedas no Banco Central.");
            _db.ChangeTracker.Clear();
        }
        return emCache;
    }

    /// <summary>
    /// Baixa os últimos dias de todas as moedas e grava no banco: insere as novas e corrige as que mudaram.
    /// Só marca a atualização como concluída se nenhuma moeda falhar.
    /// </summary>
    public async Task<ResultadoAtualizacaoCambio> AtualizarTodasAsync(CancellationToken ct)
    {
        var moedas = await ListarMoedasAsync(ct);
        if (moedas.Count == 0) return new ResultadoAtualizacaoCambio(0, 0, new() { "lista de moedas" });

        var hoje = Relogio.Hoje;
        int novas = 0, corrigidas = 0;
        var falhas = new List<string>();
        foreach (var moeda in moedas.Select(m => m.Codigo))
        {
            ct.ThrowIfCancellationRequested();
            try
            {
                var (n, c) = await BaixarAsync(moeda, hoje.AddDays(-DiasRetroativos), hoje, ct);
                await _db.SaveChangesAsync(ct);
                novas += n;
                corrigidas += c;
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                _logger.LogWarning(ex, "Não foi possível atualizar a cotação de {Moeda} no Banco Central.", moeda);
                _db.ChangeTracker.Clear();
                falhas.Add(moeda);
            }
            await Task.Delay(200, ct); // gentileza com a API pública
        }

        if (falhas.Count == 0)
        {
            await _config.DefinirAsync(ChaveUltimaAtualizacao, DateTime.UtcNow.ToString("O"), ct: ct);
            await _db.SaveChangesAsync(ct);
        }
        return new ResultadoAtualizacaoCambio(novas, corrigidas, falhas);
    }

    /// <summary>Última cotação guardada de cada moeda, sem consultar a internet.</summary>
    public async Task<CotacoesAtuaisDto> UltimasAsync(CancellationToken ct)
    {
        var ultimasDatas = await _db.CotacoesMoeda.AsNoTracking()
            .GroupBy(c => c.Moeda)
            .Select(g => new { Moeda = g.Key, Data = g.Max(c => c.Data) })
            .ToListAsync(ct);
        var moedas = ultimasDatas.Select(u => u.Moeda).ToList();
        var datas = ultimasDatas.Select(u => u.Data).Distinct().ToList();
        var chaves = ultimasDatas.Select(u => (u.Moeda, u.Data)).ToHashSet();

        var linhas = await _db.CotacoesMoeda.AsNoTracking()
            .Where(c => moedas.Contains(c.Moeda) && datas.Contains(c.Data))
            .ToListAsync(ct);
        var hoje = Relogio.Hoje;
        var cotacoes = linhas.Where(c => chaves.Contains((c.Moeda, c.Data)))
            .OrderBy(c => c.Moeda)
            .Select(c => ParaDto(c, hoje))
            .ToList();
        return new CotacoesAtuaisDto(await _config.ObterDataAsync(ChaveUltimaAtualizacao, ct), cotacoes);
    }

    /// <summary>
    /// Cotação PTAX de fechamento da moeda na data (ou no último dia útil anterior, se a data não teve boletim).
    /// Retorna null se não houver cotação nem na API nem em cache.
    /// </summary>
    public async Task<CotacaoMoedaDto?> ObterCotacaoAsync(string moeda, DateOnly? data, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var dia = data is null || data > hoje ? hoje : data.Value;

        if (moeda == MoedaBase) return new CotacaoMoedaDto(MoedaBase, dia, dia, 1m, 1m, "Fixa");

        var exata = await _db.CotacoesMoeda.AsNoTracking()
            .FirstOrDefaultAsync(c => c.Moeda == moeda && c.Data == dia, ct);
        if (exata is not null) return ParaDto(exata, dia);

        // Hoje sem boletim de fechamento (antes das ~13h, fim de semana, feriado): evita bater na API a cada chamada.
        var chaveConsulta = PrefixoUltimaConsulta + moeda;
        var consultarApi = true;
        if (dia == hoje)
        {
            var ultimaConsulta = await _config.ObterDataAsync(chaveConsulta, ct);
            consultarApi = ultimaConsulta is null
                || DateTime.UtcNow - ultimaConsulta.Value.ToUniversalTime() > TimeSpan.FromMinutes(30);
        }

        if (consultarApi)
        {
            try
            {
                await BaixarAsync(moeda, dia.AddDays(-DiasRetroativos), dia, ct);
                if (dia == hoje) await _config.DefinirAsync(chaveConsulta, DateTime.UtcNow.ToString("O"), ct: ct);
                await _db.SaveChangesAsync(ct);
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                _logger.LogWarning(ex, "Não foi possível consultar a cotação de {Moeda} no Banco Central.", moeda);
                _db.ChangeTracker.Clear();
            }
        }

        var inicio = dia.AddDays(-DiasRetroativos);
        var ultima = await _db.CotacoesMoeda.AsNoTracking()
            .Where(c => c.Moeda == moeda && c.Data <= dia && c.Data >= inicio)
            .OrderByDescending(c => c.Data)
            .FirstOrDefaultAsync(ct);
        return ultima is null ? null : ParaDto(ultima, dia);
    }

    /// <summary>Busca os fechamentos do período e prepara a gravação (não salva). Retorna (novas, corrigidas).</summary>
    private async Task<(int Novas, int Corrigidas)> BaixarAsync(string moeda, DateOnly de, DateOnly ate, CancellationToken ct)
    {
        var url = "CotacaoMoedaPeriodo(moeda=@moeda,dataInicial=@dataInicial,dataFinalCotacao=@dataFinalCotacao)"
            + $"?@moeda='{Uri.EscapeDataString(moeda)}'&@dataInicial='{de:MM-dd-yyyy}'&@dataFinalCotacao='{ate:MM-dd-yyyy}'"
            + "&$filter=tipoBoletim%20eq%20'" + BoletimFechamento + "'&$format=json";
        using var resp = await _http.GetAsync(url, ct);
        resp.EnsureSuccessStatusCode();
        using var doc = JsonDocument.Parse(await resp.Content.ReadAsStringAsync(ct));

        var existentes = await _db.CotacoesMoeda
            .Where(c => c.Moeda == moeda && c.Data >= de && c.Data <= ate)
            .ToDictionaryAsync(c => c.Data, ct);
        int novas = 0, corrigidas = 0;

        foreach (var item in Valores(doc))
        {
            // O filtro já vem na URL; a checagem repete por segurança caso a API ignore o $filter.
            if (Texto(item, "tipoBoletim") != BoletimFechamento) continue;
            var dataHora = Texto(item, "dataHoraCotacao");
            if (dataHora is null || dataHora.Length < 10
                || !DateOnly.TryParseExact(dataHora[..10], "yyyy-MM-dd", CultureInfo.InvariantCulture,
                    DateTimeStyles.None, out var data)) continue;
            var compra = Numero(item, "cotacaoCompra");
            var venda = Numero(item, "cotacaoVenda");
            if (compra is not > 0 || venda is not > 0) continue;
            if (existentes.TryGetValue(data, out var atual))
            {
                // O Banco Central pode republicar um boletim corrigido: grava o valor mais recente.
                if (atual.Compra == compra.Value && atual.Venda == venda.Value) continue;
                atual.Compra = compra.Value;
                atual.Venda = venda.Value;
                atual.AtualizadoEm = DateTime.UtcNow;
                corrigidas++;
                continue;
            }
            var nova = new CotacaoMoeda { Moeda = moeda, Data = data, Compra = compra.Value, Venda = venda.Value };
            _db.CotacoesMoeda.Add(nova);
            existentes[data] = nova;
            novas++;
        }
        return (novas, corrigidas);
    }

    private static CotacaoMoedaDto ParaDto(CotacaoMoeda c, DateOnly solicitada)
        => new(c.Moeda, solicitada, c.Data, c.Compra, c.Venda, "PTAX/Banco Central");

    private static IEnumerable<JsonElement> Valores(JsonDocument doc)
        => doc.RootElement.TryGetProperty("value", out var v) && v.ValueKind == JsonValueKind.Array
            ? v.EnumerateArray()
            : Enumerable.Empty<JsonElement>();

    private static string? Texto(JsonElement e, string nome)
        => e.TryGetProperty(nome, out var v) && v.ValueKind == JsonValueKind.String ? v.GetString() : null;

    private static decimal? Numero(JsonElement e, string nome)
        => e.TryGetProperty(nome, out var v) && v.ValueKind == JsonValueKind.Number && v.TryGetDecimal(out var d) ? d : null;

    private static List<MoedaDto> Desserializar(string? json)
    {
        if (string.IsNullOrWhiteSpace(json)) return new();
        try { return JsonSerializer.Deserialize<List<MoedaDto>>(json) ?? new(); }
        catch (JsonException) { return new(); }
    }
}
