using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Serialization;
using Microsoft.AspNetCore.WebUtilities;

namespace Financas.Api.Servicos.Pluggy;

/// <summary>Sessão autenticada na Pluggy (API key válida por cerca de 2 horas).</summary>
public sealed class PluggySessao
{
    internal PluggySessao(string apiKey) => ApiKey = apiKey;
    internal string ApiKey { get; }
}

/// <summary>
/// Cliente HTTP para a API da Pluggy. Escrito sem SDK de terceiros.
/// A API key nunca é registrada em log e só é enviada para o host configurado (api.pluggy.ai).
/// </summary>
public class PluggyClient
{
    private const int TamanhoPagina = 500;
    private static readonly JsonSerializerOptions Json = new(JsonSerializerDefaults.Web)
    {
        NumberHandling = JsonNumberHandling.AllowReadingFromString,
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull
    };

    private readonly HttpClient _http;
    private readonly ILogger<PluggyClient> _logger;

    public PluggyClient(HttpClient http, ILogger<PluggyClient> logger)
    {
        _http = http;
        _logger = logger;
    }

    public async Task<PluggySessao> CriarSessaoAsync(string clientId, string clientSecret, CancellationToken ct)
    {
        using var req = new HttpRequestMessage(HttpMethod.Post, "auth")
        {
            Content = JsonContent.Create(new { clientId, clientSecret }, options: Json)
        };
        using var resp = await _http.SendAsync(req, ct);
        if (!resp.IsSuccessStatusCode)
        {
            var status = (int)resp.StatusCode;
            throw new PluggyException(status is 400 or 401 or 403
                ? "Pluggy recusou o Client ID / Client Secret. Verifique as credenciais em Configurações."
                : $"Falha ao autenticar na Pluggy (HTTP {status}).", status);
        }
        var dados = await resp.Content.ReadFromJsonAsync<PluggyAuthResposta>(Json, ct);
        if (string.IsNullOrWhiteSpace(dados?.ApiKey))
            throw new PluggyException("A Pluggy não retornou uma API key.");
        return new PluggySessao(dados.ApiKey);
    }

    public Task<PluggyItem> ObterItemAsync(PluggySessao s, string itemId, CancellationToken ct)
        => GetAsync<PluggyItem>(s, $"items/{Uri.EscapeDataString(itemId)}", ct);

    public async Task<List<PluggyConta>> ListarContasAsync(PluggySessao s, string itemId, CancellationToken ct)
        => (await GetAsync<PluggyPagina<PluggyConta>>(s, $"accounts?itemId={Uri.EscapeDataString(itemId)}", ct)).Results;

    public async Task<List<PluggyCategoria>> ListarCategoriasAsync(PluggySessao s, CancellationToken ct)
        => (await GetAsync<PluggyPagina<PluggyCategoria>>(s, "categories", ct)).Results;

    public Task<List<PluggyFatura>> ListarFaturasAsync(PluggySessao s, string accountId, CancellationToken ct)
        => ListarPaginadoAsync<PluggyFatura>(s, $"bills?accountId={Uri.EscapeDataString(accountId)}", ct);

    public Task<List<PluggyInvestimento>> ListarInvestimentosAsync(PluggySessao s, string itemId, CancellationToken ct)
        => ListarPaginadoAsync<PluggyInvestimento>(s, $"investments?itemId={Uri.EscapeDataString(itemId)}", ct);

    public Task<List<PluggyEmprestimo>> ListarEmprestimosAsync(PluggySessao s, string itemId, CancellationToken ct)
        => ListarPaginadoAsync<PluggyEmprestimo>(s, $"loans?itemId={Uri.EscapeDataString(itemId)}", ct);

    public Task<List<PluggyMovimentoInvestimento>> ListarMovimentosInvestimentoAsync(PluggySessao s, string investmentId,
        CancellationToken ct)
        => ListarPaginadoAsync<PluggyMovimentoInvestimento>(s, $"investments/{Uri.EscapeDataString(investmentId)}/transactions", ct);

    /// <summary>Transações via GET /v2/transactions (paginação por cursor). O endpoint v1 será desativado em 2026-12-31.</summary>
    public async Task<List<PluggyTransacao>> ListarTransacoesAsync(PluggySessao s, string accountId, DateOnly dataInicial,
        CancellationToken ct)
    {
        var todas = new List<PluggyTransacao>();
        string? cursor = null;
        var paginas = 0;
        do
        {
            var url = $"v2/transactions?accountId={Uri.EscapeDataString(accountId)}&dateFrom={dataInicial:yyyy-MM-dd}";
            if (cursor is not null) url += $"&after={Uri.EscapeDataString(cursor)}";

            var pagina = await GetAsync<PluggyPaginaCursor<PluggyTransacao>>(s, url, ct);
            todas.AddRange(pagina.Results);
            cursor = ExtrairCursor(pagina.Next);

            if (++paginas > 1000)
                throw new PluggyException("Paginação de transações excedeu o limite de segurança.");
        } while (cursor is not null);

        return todas;
    }

    private async Task<List<T>> ListarPaginadoAsync<T>(PluggySessao s, string caminho, CancellationToken ct)
    {
        var resultado = new List<T>();
        var pagina = 1;
        while (true)
        {
            var sep = caminho.Contains('?') ? '&' : '?';
            var dados = await GetAsync<PluggyPagina<T>>(s, $"{caminho}{sep}page={pagina}&pageSize={TamanhoPagina}", ct);
            resultado.AddRange(dados.Results);
            if (dados.TotalPages <= pagina || dados.Results.Count == 0 || pagina >= 200) break;
            pagina++;
        }
        return resultado;
    }

    /// <summary>
    /// Extrai apenas o parâmetro "after" do link "next". Não seguimos a URL recebida diretamente
    /// para garantir que a API key só seja enviada ao host configurado.
    /// </summary>
    private static string? ExtrairCursor(string? next)
    {
        if (string.IsNullOrWhiteSpace(next)) return null;
        var indice = next.IndexOf('?');
        if (indice < 0) return null;
        var query = QueryHelpers.ParseQuery(next[indice..]);
        return query.TryGetValue("after", out var valor) && !string.IsNullOrWhiteSpace(valor) ? valor.ToString() : null;
    }

    private async Task<T> GetAsync<T>(PluggySessao s, string caminho, CancellationToken ct)
    {
        for (var tentativa = 1; ; tentativa++)
        {
            using var req = new HttpRequestMessage(HttpMethod.Get, caminho);
            req.Headers.Add("X-API-KEY", s.ApiKey);
            req.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));

            HttpResponseMessage resp;
            try
            {
                resp = await _http.SendAsync(req, ct);
            }
            catch (HttpRequestException ex) when (tentativa < 3)
            {
                _logger.LogWarning("Falha de rede ao chamar Pluggy ({Caminho}): {Erro}. Tentativa {T}.",
                    CaminhoSemQuery(caminho), ex.Message, tentativa);
                await Task.Delay(TimeSpan.FromSeconds(2 * tentativa), ct);
                continue;
            }

            using (resp)
            {
                if (resp.IsSuccessStatusCode)
                {
                    var dados = await resp.Content.ReadFromJsonAsync<T>(Json, ct);
                    return dados ?? throw new PluggyException($"Resposta vazia da Pluggy em {CaminhoSemQuery(caminho)}.");
                }

                var status = (int)resp.StatusCode;
                var transitorio = resp.StatusCode == HttpStatusCode.TooManyRequests || status >= 500;
                if (transitorio && tentativa < 4)
                {
                    var espera = resp.Headers.RetryAfter?.Delta ?? TimeSpan.FromSeconds(3 * tentativa);
                    if (espera > TimeSpan.FromSeconds(60)) espera = TimeSpan.FromSeconds(60);
                    await Task.Delay(espera, ct);
                    continue;
                }

                var corpo = await resp.Content.ReadAsStringAsync(ct);
                if (corpo.Length > 300) corpo = corpo[..300];
                throw new PluggyException(status switch
                {
                    401 or 403 => "Acesso negado pela Pluggy. Verifique as credenciais e a autorização do Meu Pluggy.",
                    404 => $"Recurso não encontrado na Pluggy ({CaminhoSemQuery(caminho)}). Confira o Item ID.",
                    _ => $"Erro HTTP {status} da Pluggy em {CaminhoSemQuery(caminho)}: {corpo}"
                }, status);
            }
        }
    }

    private static string CaminhoSemQuery(string caminho)
    {
        var i = caminho.IndexOf('?');
        return i < 0 ? caminho : caminho[..i];
    }
}
