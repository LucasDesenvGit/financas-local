using System.Globalization;
using System.Text.Json;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Cripto;

public record ResultadoAtualizacaoCripto(int Novas, List<string> Falhas);

/// <summary>
/// Cotação de criptomoedas em reais pela API pública v4 do Mercado Bitcoin (ADR 0048), sem chave. Guarda o último
/// negócio de cada moeda a cada atualização (CriptoHostedService), por 30 dias, para a tela funcionar sem internet e
/// para a variação em 24 h. Só os pares fixos são enviados; nenhum valor ou dado pessoal sai do computador.
/// </summary>
public class CriptoService
{
    /// <summary>As moedas acompanhadas, na ordem da tela. Moeda nova: conferir antes que o Mercado Bitcoin tem XXX-BRL.</summary>
    public static readonly string[] Moedas = ["BTC", "ETH", "SOL", "USDT"];

    public const string ChaveUltimaAtualizacao = "Cripto.UltimaAtualizacao";
    public const string Fonte = "Mercado Bitcoin";
    /// <summary>Por quantos dias a cotação fica guardada.</summary>
    private const int DiasGuardados = 30;

    private readonly HttpClient _http;
    private readonly FinancasDbContext _db;
    private readonly ConfiguracaoService _config;
    private readonly ILogger<CriptoService> _logger;

    public CriptoService(HttpClient http, FinancasDbContext db, ConfiguracaoService config, ILogger<CriptoService> logger)
    {
        _http = http;
        _db = db;
        _config = config;
        _logger = logger;
    }

    /// <summary>
    /// Consulta o último negócio de todas as moedas numa chamada só e grava as que ainda não estão guardadas.
    /// Também apaga o que passou de 30 dias. Só marca a atualização como feita se nenhuma moeda faltar na resposta.
    /// </summary>
    public async Task<ResultadoAtualizacaoCripto> AtualizarAsync(CancellationToken ct)
    {
        var pares = string.Join(',', Moedas.Select(m => $"{m}-BRL"));
        using var resp = await _http.GetAsync($"tickers?symbols={pares}", ct);
        resp.EnsureSuccessStatusCode();
        using var doc = JsonDocument.Parse(await resp.Content.ReadAsStringAsync(ct));
        if (doc.RootElement.ValueKind != JsonValueKind.Array)
            throw new InvalidOperationException("Resposta inesperada do Mercado Bitcoin (não é uma lista).");

        var lidas = new Dictionary<string, (decimal Preco, DateTime DataHora)>();
        foreach (var item in doc.RootElement.EnumerateArray())
        {
            var par = Texto(item, "pair");
            var moeda = par?.EndsWith("-BRL", StringComparison.Ordinal) == true ? par[..^4] : null;
            var preco = Decimal(item, "last");
            var data = DataHora(item);
            if (moeda is null || !Moedas.Contains(moeda) || preco is not > 0 || data is null) continue;
            lidas[moeda] = (preco.Value, data.Value);
        }

        var novas = 0;
        foreach (var (moeda, (preco, dataHora)) in lidas)
        {
            var existe = await _db.CotacoesCripto.AnyAsync(c => c.Moeda == moeda && c.DataHora == dataHora, ct);
            if (existe) continue;
            _db.CotacoesCripto.Add(new CotacaoCripto { Moeda = moeda, DataHora = dataHora, Preco = preco });
            novas++;
        }

        var falhas = Moedas.Where(m => !lidas.ContainsKey(m)).ToList();
        if (falhas.Count == 0)
            // DefinirAsync não salva sozinho: entra no mesmo SaveChangesAsync das cotações.
            await _config.DefinirAsync(ChaveUltimaAtualizacao, DateTime.UtcNow.ToString("O"), ct: ct);
        else
            _logger.LogWarning("O Mercado Bitcoin não devolveu a cotação de {Moedas}.", string.Join(", ", falhas));
        await _db.SaveChangesAsync(ct);

        var limite = DateTime.UtcNow.AddDays(-DiasGuardados);
        await _db.CotacoesCripto.Where(c => c.DataHora < limite).ExecuteDeleteAsync(ct);
        return new ResultadoAtualizacaoCripto(novas, falhas);
    }

    /// <summary>
    /// A última cotação guardada de cada moeda, com a variação contra a guardada perto de 24 h antes (entre 22 e
    /// 26 h). Sem cotação nessa janela (site desligado, primeiras 24 h), a variação fica null. Não consulta a internet.
    /// </summary>
    public async Task<CotacoesCriptoDto> UltimasAsync(CancellationToken ct)
    {
        var desde = DateTime.UtcNow.AddDays(-2);
        var recentes = await _db.CotacoesCripto.AsNoTracking()
            .Where(c => Moedas.Contains(c.Moeda) && c.DataHora >= desde)
            .ToListAsync(ct);
        // Sem nada nos últimos 2 dias (site desligado): a última de cada moeda, mesmo antiga, com a data dela.
        if (recentes.Select(c => c.Moeda).Distinct().Count() < Moedas.Length)
        {
            var faltam = Moedas.Except(recentes.Select(c => c.Moeda)).ToList();
            foreach (var moeda in faltam)
            {
                var ultima = await _db.CotacoesCripto.AsNoTracking().Where(c => c.Moeda == moeda)
                    .OrderByDescending(c => c.DataHora).FirstOrDefaultAsync(ct);
                if (ultima is not null) recentes.Add(ultima);
            }
        }

        var cotacoes = new List<CotacaoCriptoDto>();
        foreach (var moeda in Moedas)
        {
            var daMoeda = recentes.Where(c => c.Moeda == moeda).OrderByDescending(c => c.DataHora).ToList();
            var ultima = daMoeda.FirstOrDefault();
            if (ultima is null) continue;
            var alvo = ultima.DataHora.AddHours(-24);
            var anterior = daMoeda
                .Where(c => c.DataHora <= ultima.DataHora.AddHours(-22) && c.DataHora >= ultima.DataHora.AddHours(-26))
                .OrderBy(c => Math.Abs((c.DataHora - alvo).Ticks))
                .FirstOrDefault();
            decimal? variacao = anterior is { Preco: > 0 }
                ? Math.Round((ultima.Preco - anterior.Preco) / anterior.Preco * 100m, 2)
                : null;
            cotacoes.Add(new CotacaoCriptoDto(moeda, ultima.Preco, DateTime.SpecifyKind(ultima.DataHora, DateTimeKind.Utc),
                variacao, Fonte));
        }
        return new CotacoesCriptoDto(await _config.ObterDataAsync(ChaveUltimaAtualizacao, ct), cotacoes);
    }

    private static string? Texto(JsonElement e, string nome)
        => e.TryGetProperty(nome, out var v) && v.ValueKind == JsonValueKind.String ? v.GetString() : null;

    /// <summary>O Mercado Bitcoin manda os preços como texto ("419184.00000000").</summary>
    private static decimal? Decimal(JsonElement e, string nome)
    {
        if (!e.TryGetProperty(nome, out var v)) return null;
        if (v.ValueKind == JsonValueKind.Number && v.TryGetDecimal(out var n)) return n;
        return v.ValueKind == JsonValueKind.String
            && decimal.TryParse(v.GetString(), NumberStyles.Number, CultureInfo.InvariantCulture, out var d) ? d : null;
    }

    /// <summary>
    /// "date" do último negócio. A especificação diz nanossegundos, mas a resposta vem em segundos (10 dígitos);
    /// aceita os dois pelo tamanho do número.
    /// </summary>
    private static DateTime? DataHora(JsonElement e)
    {
        if (!e.TryGetProperty("date", out var v) || v.ValueKind != JsonValueKind.Number || !v.TryGetInt64(out var n) || n <= 0)
            return null;
        var segundos = n > 100_000_000_000_000L ? n / 1_000_000_000L : n > 100_000_000_000L ? n / 1000L : n;
        return DateTimeOffset.FromUnixTimeSeconds(segundos).UtcDateTime;
    }
}
