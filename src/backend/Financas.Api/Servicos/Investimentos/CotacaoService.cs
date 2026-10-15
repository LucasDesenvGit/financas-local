using System.Globalization;
using System.Net.Http.Headers;
using System.Text.Json;
using System.Text.RegularExpressions;
using Financas.Api.Data;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Investimentos;

public record ResultadoCotacoes(int Atualizados, int ProventosNovos, List<string> Falhas);

/// <summary>
/// Cotações e proventos de ativos da B3. Fonte principal: Yahoo Finance (pública, sem chave, não oficial).
/// Alternativa opcional: brapi.dev (exige token gratuito, configurado em Investimentos).
/// Somente o código do ativo (ex.: MXRF11) é enviado; nenhuma quantidade, valor ou dado pessoal sai do computador.
/// </summary>
public partial class CotacaoService
{
    public const string ClienteYahoo = "Yahoo";
    public const string ClienteBrapi = "Brapi";
    public const string ChaveTokenBrapi = "Cotacao.BrapiToken";

    private readonly IHttpClientFactory _fabrica;
    private readonly FinancasDbContext _db;
    private readonly ConfiguracaoService _config;
    private readonly ILogger<CotacaoService> _logger;

    public CotacaoService(IHttpClientFactory fabrica, FinancasDbContext db, ConfiguracaoService config,
        ILogger<CotacaoService> logger)
    {
        _fabrica = fabrica;
        _db = db;
        _config = config;
        _logger = logger;
    }

    [GeneratedRegex("^[A-Z][A-Z0-9]{3}[0-9]{1,2}F?$")]
    private static partial Regex FormatoTicker();

    /// <summary>Normaliza e valida um código da B3 (ex.: "mxrf11 " → "MXRF11"). Retorna null se inválido.</summary>
    public static string? NormalizarTicker(string? ticker)
    {
        var t = (ticker ?? string.Empty).Trim().ToUpperInvariant();
        if (t.EndsWith(".SA", StringComparison.Ordinal)) t = t[..^3];
        if (!FormatoTicker().IsMatch(t)) return null;
        // Mercado fracionário (final F) usa a mesma cotação do lote padrão.
        return t.Length >= 6 && t.EndsWith('F') ? t[..^1] : t;
    }

    public async Task<ResultadoCotacoes> AtualizarAsync(IEnumerable<int>? somenteAtivos, CancellationToken ct)
    {
        var consulta = _db.Ativos.Include(a => a.Operacoes).AsQueryable();
        if (somenteAtivos is not null)
        {
            var ids = somenteAtivos.ToList();
            consulta = consulta.Where(a => ids.Contains(a.Id));
        }
        var ativos = await consulta.ToListAsync(ct);
        var token = await _config.ObterAsync(ChaveTokenBrapi, ct);

        var atualizados = 0;
        var proventosNovos = 0;
        var falhas = new List<string>();

        foreach (var ativo in ativos)
        {
            ct.ThrowIfCancellationRequested();
            var primeiraOperacao = ativo.Operacoes.Count > 0 ? ativo.Operacoes.Min(o => o.Data) : Relogio.Hoje;

            CotacaoObtida? cotacao = null;
            try
            {
                cotacao = await YahooAsync(ativo.Ticker, primeiraOperacao, ct);
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                _logger.LogWarning(ex, "Falha ao consultar {Ticker} no Yahoo Finance.", ativo.Ticker);
            }

            if (cotacao?.Preco is null && !string.IsNullOrWhiteSpace(token))
            {
                try
                {
                    cotacao = await BrapiAsync(ativo.Ticker, token, ct) ?? cotacao;
                }
                catch (Exception ex) when (ex is not OperationCanceledException)
                {
                    _logger.LogWarning(ex, "Falha ao consultar {Ticker} na brapi.", ativo.Ticker);
                }
            }

            if (cotacao is null)
            {
                falhas.Add(ativo.Ticker);
                continue;
            }

            if (string.IsNullOrWhiteSpace(ativo.Nome) && !string.IsNullOrWhiteSpace(cotacao.Nome))
                ativo.Nome = Limitar(cotacao.Nome, 150);

            if (!ativo.PrecoManual && cotacao.Preco is decimal preco && preco > 0)
            {
                ativo.PrecoAtual = Math.Round(preco, 4);
                ativo.PrecoAnterior = cotacao.PrecoAnterior is decimal pa && pa > 0 ? Math.Round(pa, 4) : null;
                ativo.PrecoAtualizadoEm = DateTime.UtcNow;
                ativo.FonteCotacao = cotacao.Fonte;
                atualizados++;
            }
            else if (cotacao.Preco is null)
            {
                falhas.Add(ativo.Ticker);
            }

            if (cotacao.Proventos.Count > 0)
            {
                var existentes = (await _db.ProventosAtivo.Where(p => p.AtivoId == ativo.Id).Select(p => p.DataCom)
                    .ToListAsync(ct)).ToHashSet();
                foreach (var (dataCom, valor) in cotacao.Proventos)
                {
                    if (valor <= 0 || !existentes.Add(dataCom)) continue;
                    _db.ProventosAtivo.Add(new ProventoAtivo { AtivoId = ativo.Id, DataCom = dataCom, ValorPorCota = valor });
                    proventosNovos++;
                }
            }

            if (cotacao.Mensais.Count > 0)
                await GravarMensaisAtivoAsync(ativo.Id, cotacao.Mensais, ct);

            await _db.SaveChangesAsync(ct);
            await Task.Delay(250, ct); // gentileza com a fonte gratuita
        }

        if (ativos.Count > 0)
        {
            try { await AtualizarIndiceAsync(CodigoIfix, ct); }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                _logger.LogWarning(ex, "Não foi possível atualizar o IFIX.");
            }
        }

        return new ResultadoCotacoes(atualizados, proventosNovos, falhas);
    }

    private sealed record CotacaoObtida(decimal? Preco, decimal? PrecoAnterior, string? Nome, string Fonte,
        List<(DateOnly DataCom, decimal Valor)> Proventos)
    {
        public List<(DateOnly Mes, decimal Fechamento)> Mensais { get; init; } = new();
    }

    private async Task<CotacaoObtida?> YahooAsync(string ticker, DateOnly primeiraOperacao, CancellationToken ct)
    {
        var http = _fabrica.CreateClient(ClienteYahoo);
        var simbolo = Uri.EscapeDataString(ticker + ".SA");

        // 1) Preço atual e fechamento anterior.
        decimal? preco = null, anterior = null;
        string? nome = null;
        using (var resp = await http.GetAsync($"v8/finance/chart/{simbolo}?range=5d&interval=1d", ct))
        {
            if (resp.IsSuccessStatusCode)
            {
                using var doc = JsonDocument.Parse(await resp.Content.ReadAsStringAsync(ct));
                if (Resultado(doc) is JsonElement r && r.TryGetProperty("meta", out var meta))
                {
                    preco = Numero(meta, "regularMarketPrice");
                    nome = Texto(meta, "longName") ?? Texto(meta, "shortName");
                    var fechamentos = Fechamentos(r);
                    // Fechamento anterior = penúltimo pregão (o último é o de hoje, se houver).
                    if (fechamentos.Count >= 2) anterior = fechamentos[^2];
                    anterior ??= Numero(meta, "previousClose") ?? Numero(meta, "chartPreviousClose");
                }
            }
        }

        // 2) Proventos desde a primeira compra (o histórico mensal traz todos os eventos de dividendos).
        var proventos = new List<(DateOnly, decimal)>();
        var mensais = new List<(DateOnly Mes, decimal Fechamento)>();
        var anos = Relogio.Hoje.Year - primeiraOperacao.Year;
        var faixa = anos >= 9 ? "max" : anos >= 4 ? "10y" : "5y";
        using (var resp = await http.GetAsync($"v8/finance/chart/{simbolo}?range={faixa}&interval=1mo&events=div", ct))
        {
            if (resp.IsSuccessStatusCode)
            {
                using var doc = JsonDocument.Parse(await resp.Content.ReadAsStringAsync(ct));
                if (Resultado(doc) is JsonElement rm) mensais = FechamentosMensais(rm);
                if (Resultado(doc) is JsonElement r
                    && r.TryGetProperty("events", out var eventos)
                    && eventos.TryGetProperty("dividends", out var dividendos)
                    && dividendos.ValueKind == JsonValueKind.Object)
                {
                    foreach (var item in dividendos.EnumerateObject())
                    {
                        var valor = Numero(item.Value, "amount");
                        if (valor is null || !item.Value.TryGetProperty("date", out var dataJs)
                            || !dataJs.TryGetInt64(out var segundos)) continue;
                        // O Yahoo informa a data "ex"; a data-com é o pregão anterior.
                        var dataEx = Relogio.ParaDataLocal(DateTimeOffset.FromUnixTimeSeconds(segundos));
                        proventos.Add((PregaoAnterior(dataEx), Math.Round(valor.Value, 8)));
                    }
                }
            }
        }

        if (preco is null && proventos.Count == 0 && mensais.Count == 0) return null;
        return new CotacaoObtida(preco, anterior, nome, "Yahoo Finance", proventos) { Mensais = mensais };
    }

    public const string CodigoIfix = "IFIX";

    private async Task GravarMensaisAtivoAsync(int ativoId, List<(DateOnly Mes, decimal Fechamento)> mensais,
        CancellationToken ct)
    {
        var existentes = await _db.PrecosMensaisAtivo.Where(p => p.AtivoId == ativoId).ToDictionaryAsync(p => p.Mes, ct);
        foreach (var (mes, fechamento) in mensais)
        {
            if (fechamento <= 0) continue;
            if (existentes.TryGetValue(mes, out var atual)) atual.Fechamento = Math.Round(fechamento, 4);
            else
            {
                var novo = new PrecoMensalAtivo { AtivoId = ativoId, Mes = mes, Fechamento = Math.Round(fechamento, 4) };
                _db.PrecosMensaisAtivo.Add(novo);
                existentes[mes] = novo;
            }
        }
    }

    /// <summary>Histórico mensal de um índice (IFIX), no máximo uma vez a cada 12 horas.</summary>
    private async Task AtualizarIndiceAsync(string codigo, CancellationToken ct)
    {
        var chave = $"Indice.{codigo}.ConsultadoEm";
        var consultado = await _config.ObterDataAsync(chave, ct);
        if (consultado is not null && DateTime.UtcNow - consultado.Value.ToUniversalTime() < TimeSpan.FromHours(12)) return;

        var http = _fabrica.CreateClient(ClienteYahoo);
        List<(DateOnly Mes, decimal Fechamento)> mensais = new();
        // O Yahoo já publicou o IFIX com símbolos diferentes; tenta os conhecidos.
        foreach (var simbolo in new[] { "IFIX.SA", "^IFIX" })
        {
            using var resp = await http.GetAsync($"v8/finance/chart/{Uri.EscapeDataString(simbolo)}?range=10y&interval=1mo", ct);
            if (!resp.IsSuccessStatusCode) continue;
            using var doc = JsonDocument.Parse(await resp.Content.ReadAsStringAsync(ct));
            if (Resultado(doc) is JsonElement r) mensais = FechamentosMensais(r);
            if (mensais.Count > 0) break;
        }

        await _config.DefinirAsync(chave, DateTime.UtcNow.ToString("O"), ct: ct);
        if (mensais.Count > 0)
        {
            var existentes = await _db.IndicesMensais.Where(i => i.Codigo == codigo).ToDictionaryAsync(i => i.Mes, ct);
            foreach (var (mes, fechamento) in mensais)
            {
                if (fechamento <= 0) continue;
                if (existentes.TryGetValue(mes, out var atual)) atual.Fechamento = Math.Round(fechamento, 4);
                else
                {
                    var novo = new IndiceMensal { Codigo = codigo, Mes = mes, Fechamento = Math.Round(fechamento, 4) };
                    _db.IndicesMensais.Add(novo);
                    existentes[mes] = novo;
                }
            }
        }
        await _db.SaveChangesAsync(ct);
    }

    /// <summary>
    /// Fechamentos diários de um ativo entre duas datas (para descobrir o preço da cota no dia de uma compra).
    /// Retorna vazio se a fonte não responder.
    /// </summary>
    public async Task<SortedDictionary<DateOnly, decimal>> FechamentosDiariosAsync(string ticker, DateOnly de, DateOnly ate,
        CancellationToken ct)
    {
        var resultado = new SortedDictionary<DateOnly, decimal>();
        try
        {
            var http = _fabrica.CreateClient(ClienteYahoo);
            var inicio = new DateTimeOffset(de.AddDays(-7).ToDateTime(TimeOnly.MinValue), TimeSpan.Zero).ToUnixTimeSeconds();
            var fim = new DateTimeOffset(ate.AddDays(2).ToDateTime(TimeOnly.MinValue), TimeSpan.Zero).ToUnixTimeSeconds();
            var simbolo = Uri.EscapeDataString(ticker + ".SA");
            using var resp = await http.GetAsync($"v8/finance/chart/{simbolo}?period1={inicio}&period2={fim}&interval=1d", ct);
            if (!resp.IsSuccessStatusCode) return resultado;
            using var doc = JsonDocument.Parse(await resp.Content.ReadAsStringAsync(ct));
            if (Resultado(doc) is not JsonElement r) return resultado;
            foreach (var (data, fechamento) in SerieFechamentos(r)) resultado[data] = fechamento;
        }
        catch (Exception ex) when (ex is not OperationCanceledException)
        {
            _logger.LogWarning(ex, "Falha ao consultar o histórico de {Ticker}.", ticker);
        }
        return resultado;
    }

    private static List<(DateOnly Mes, decimal Fechamento)> FechamentosMensais(JsonElement resultado)
        => SerieFechamentos(resultado)
            .GroupBy(x => new DateOnly(x.Data.Year, x.Data.Month, 1))
            .Select(g => (g.Key, g.OrderBy(x => x.Data).Last().Fechamento))
            .ToList();

    /// <summary>Pares (data, fechamento) alinhando "timestamp" com "indicators.quote[0].close".</summary>
    private static List<(DateOnly Data, decimal Fechamento)> SerieFechamentos(JsonElement resultado)
    {
        var lista = new List<(DateOnly, decimal)>();
        if (!resultado.TryGetProperty("timestamp", out var ts) || ts.ValueKind != JsonValueKind.Array) return lista;
        if (!resultado.TryGetProperty("indicators", out var ind) || !ind.TryGetProperty("quote", out var q)
            || q.ValueKind != JsonValueKind.Array || q.GetArrayLength() == 0
            || !q[0].TryGetProperty("close", out var fech) || fech.ValueKind != JsonValueKind.Array) return lista;
        var n = Math.Min(ts.GetArrayLength(), fech.GetArrayLength());
        for (var i = 0; i < n; i++)
        {
            if (!ts[i].TryGetInt64(out var segundos)) continue;
            var f = fech[i];
            if (f.ValueKind != JsonValueKind.Number || !f.TryGetDecimal(out var valor) || valor <= 0) continue;
            lista.Add((Relogio.ParaDataLocal(DateTimeOffset.FromUnixTimeSeconds(segundos)), valor));
        }
        return lista;
    }

    private async Task<CotacaoObtida?> BrapiAsync(string ticker, string token, CancellationToken ct)
    {
        var http = _fabrica.CreateClient(ClienteBrapi);
        using var req = new HttpRequestMessage(HttpMethod.Get, $"api/quote/{Uri.EscapeDataString(ticker)}");
        // Token no cabeçalho (não fica em logs de URL).
        req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token);
        using var resp = await http.SendAsync(req, ct);
        if (!resp.IsSuccessStatusCode) return null;
        using var doc = JsonDocument.Parse(await resp.Content.ReadAsStringAsync(ct));
        if (!doc.RootElement.TryGetProperty("results", out var resultados) || resultados.ValueKind != JsonValueKind.Array
            || resultados.GetArrayLength() == 0) return null;
        var r = resultados[0];
        var preco = Numero(r, "regularMarketPrice");
        if (preco is null) return null;
        return new CotacaoObtida(preco, Numero(r, "regularMarketPreviousClose"),
            Texto(r, "longName") ?? Texto(r, "shortName"), "brapi.dev", new());
    }

    /* ------------------------------ leitura defensiva de JSON ------------------------------ */

    private static JsonElement? Resultado(JsonDocument doc)
    {
        if (!doc.RootElement.TryGetProperty("chart", out var chart)) return null;
        if (!chart.TryGetProperty("result", out var resultado) || resultado.ValueKind != JsonValueKind.Array
            || resultado.GetArrayLength() == 0) return null;
        return resultado[0];
    }

    private static List<decimal> Fechamentos(JsonElement resultado)
    {
        var lista = new List<decimal>();
        if (resultado.TryGetProperty("indicators", out var ind) && ind.TryGetProperty("quote", out var q)
            && q.ValueKind == JsonValueKind.Array && q.GetArrayLength() > 0
            && q[0].TryGetProperty("close", out var fech) && fech.ValueKind == JsonValueKind.Array)
        {
            foreach (var f in fech.EnumerateArray())
                if (f.ValueKind == JsonValueKind.Number && f.TryGetDecimal(out var v)) lista.Add(v);
        }
        return lista;
    }

    private static decimal? Numero(JsonElement elemento, string nome)
        => elemento.ValueKind == JsonValueKind.Object && elemento.TryGetProperty(nome, out var v)
           && v.ValueKind == JsonValueKind.Number && v.TryGetDecimal(out var d) ? d : null;

    private static string? Texto(JsonElement elemento, string nome)
        => elemento.ValueKind == JsonValueKind.Object && elemento.TryGetProperty(nome, out var v)
           && v.ValueKind == JsonValueKind.String ? v.GetString() : null;

    private static DateOnly PregaoAnterior(DateOnly data)
    {
        var d = data.AddDays(-1);
        while (d.DayOfWeek is DayOfWeek.Saturday or DayOfWeek.Sunday) d = d.AddDays(-1);
        return d;
    }

    private static string Limitar(string texto, int tamanho) => texto.Length <= tamanho ? texto : texto[..tamanho];
}
