using System.Net;
using System.Security.Claims;
using Financas.Api.Dtos;
using Financas.Api.Seguranca;
using Microsoft.AspNetCore.Antiforgery;

namespace Financas.Api.Servicos.Go;

/// <summary>
/// Repasse dos prefixos de <c>Go:Prefixos</c> ao Go (#231, ADR 0049), com <see cref="HttpClient"/> (sem biblioteca
/// de proxy: o repasse é temporário e sai na inversão, #85).
/// <list type="bullet">
/// <item>É um endpoint (<c>app.Map</c>), e não um middleware: assim a <c>FallbackPolicy</c> exige o login (401 do
/// próprio .NET, sem chegar ao Go), e a ordem -1 ganha dos controllers e do <c>MapFallback</c> de <c>/api</c>.</item>
/// <item>O CSRF é conferido aqui: o filtro <c>AutoValidateAntiforgeryTokenAttribute</c> é do MVC e não vale para
/// endpoint fora dele. Mesma regra do filtro: GET, HEAD, OPTIONS e TRACE não conferem; o resto sem token → 400.</item>
/// <item>Vão ao Go o método, o caminho, a query, o corpo e os cabeçalhos, menos os de conexão, o cookie e o token
/// CSRF. Os cabeçalhos <c>X-Financas-*</c> que vierem de fora são apagados; o .NET põe o segredo interno e o Id de
/// quem está logado. Status, cabeçalhos e corpo da resposta voltam como o Go mandou.</item>
/// <item>Go fora do ar (subindo, reiniciando, sem executável) → 503 com mensagem.</item>
/// </list>
/// </summary>
public static class RepasseGo
{
    public const string NomeCliente = "Go";
    public const string CabecalhoSegredo = "X-Financas-Segredo";
    public const string CabecalhoUsuario = "X-Financas-Usuario";
    private const string PrefixoCabecalhoInterno = "X-Financas-";

    public const string MensagemIndisponivel =
        "Esta parte do sistema está indisponível no momento. Tente de novo em alguns segundos; se continuar, veja Configurações › Erros.";

    /// <summary>Cabeçalhos que não passam adiante: os de conexão (RFC 9110, 7.6.1), o cookie e o token CSRF.</summary>
    private static readonly HashSet<string> NaoRepassar = new(StringComparer.OrdinalIgnoreCase)
    {
        "Connection", "Keep-Alive", "Proxy-Connection", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer",
        "Transfer-Encoding", "Upgrade", "Host", "Cookie", "Authorization", ConstantesSeguranca.CabecalhoXsrf
    };

    /// <summary>Cabeçalhos da resposta do Go que o Kestrel e o .NET cuidam por conta própria.</summary>
    private static readonly HashSet<string> NaoDevolver = new(StringComparer.OrdinalIgnoreCase)
    {
        "Connection", "Keep-Alive", "Proxy-Connection", "TE", "Trailer", "Transfer-Encoding", "Upgrade", "Date",
        "Server", "Set-Cookie"
    };

    /// <summary>Um endpoint por prefixo da lista. Sem prefixos, nada é mapeado e tudo continua no .NET.</summary>
    public static void MapearRepasseGo(this IEndpointRouteBuilder app, OpcoesGo opcoes)
    {
        foreach (var prefixo in opcoes.Prefixos)
            app.Map(prefixo + "/{**resto}", RepassarAsync)
                .WithOrder(-1)
                .WithDisplayName($"Repasse ao Go: {prefixo}");
    }

    private static async Task RepassarAsync(HttpContext ctx, ProcessoGo go, IAntiforgery antiforgery,
        IHttpClientFactory fabrica)
    {
        var metodo = ctx.Request.Method;
        if (!(HttpMethods.IsGet(metodo) || HttpMethods.IsHead(metodo) || HttpMethods.IsOptions(metodo) || HttpMethods.IsTrace(metodo)))
        {
            try
            {
                await antiforgery.ValidateRequestAsync(ctx);
            }
            catch (AntiforgeryValidationException)
            {
                // Igual ao filtro do MVC (AntiforgeryValidationFailedResult): 400 sem corpo.
                ctx.Response.StatusCode = StatusCodes.Status400BadRequest;
                return;
            }
        }

        var destino = go.DestinoAtual;
        if (destino is null)
        {
            await Indisponivel(ctx);
            return;
        }

        var usuario = ctx.User.FindFirstValue(ClaimTypes.NameIdentifier);
        if (string.IsNullOrEmpty(usuario))
        {
            // A FallbackPolicy já exige login; sem o Id na sessão, nada vai ao Go.
            ctx.Response.StatusCode = StatusCodes.Status401Unauthorized;
            return;
        }

        var endereco = new Uri(destino.Base, ctx.Request.Path.ToUriComponent() + ctx.Request.QueryString.ToUriComponent());
        using var pedido = new HttpRequestMessage(new HttpMethod(metodo), endereco);
        if (ctx.Request.ContentLength > 0 || ctx.Request.Headers.TransferEncoding.Count > 0)
            pedido.Content = new StreamContent(ctx.Request.Body);
        else if (ctx.Request.ContentType is not null)
            // Corpo vazio com Content-Type: sem um conteúdo, o cabeçalho não passa, e o Go responderia 415 (sem tipo) no
            // lugar do 400 do corpo vazio que o .NET dá (contrato 04-contas, "criar com corpo vazio").
            pedido.Content = new ByteArrayContent([]);
        foreach (var (nome, valores) in ctx.Request.Headers)
        {
            if (NaoRepassar.Contains(nome) || nome.StartsWith(PrefixoCabecalhoInterno, StringComparison.OrdinalIgnoreCase))
                continue;
            if (!pedido.Headers.TryAddWithoutValidation(nome, (IEnumerable<string?>)valores))
                pedido.Content?.Headers.TryAddWithoutValidation(nome, (IEnumerable<string?>)valores);
        }
        pedido.Headers.TryAddWithoutValidation(CabecalhoSegredo, destino.Segredo);
        pedido.Headers.TryAddWithoutValidation(CabecalhoUsuario, usuario);

        HttpResponseMessage resposta;
        try
        {
            resposta = await fabrica.CreateClient(NomeCliente)
                .SendAsync(pedido, HttpCompletionOption.ResponseHeadersRead, ctx.RequestAborted);
        }
        catch (Exception ex) when (ex is HttpRequestException
                                   || (ex is TaskCanceledException && !ctx.RequestAborted.IsCancellationRequested))
        {
            // O Go caiu no meio, ou não respondeu no prazo do cliente.
            await Indisponivel(ctx);
            return;
        }

        using (resposta)
        {
            ctx.Response.StatusCode = (int)resposta.StatusCode;
            foreach (var (nome, valores) in resposta.Headers.Concat(resposta.Content.Headers))
            {
                if (!NaoDevolver.Contains(nome)) ctx.Response.Headers[nome] = valores.ToArray();
            }
            await resposta.Content.CopyToAsync(ctx.Response.Body, ctx.RequestAborted);
        }
    }

    private static Task Indisponivel(HttpContext ctx)
    {
        ctx.Response.StatusCode = StatusCodes.Status503ServiceUnavailable;
        return ctx.Response.WriteAsJsonAsync(new MensagemDto(MensagemIndisponivel));
    }

    /// <summary>O cliente do repasse: sem cookie, sem redirecionamento, sem proxy do sistema, direto no 127.0.0.1.</summary>
    public static void AdicionarClienteGo(this IServiceCollection servicos)
    {
        servicos.AddHttpClient(NomeCliente, c => c.Timeout = TimeSpan.FromSeconds(100))
            .ConfigurePrimaryHttpMessageHandler(() => new SocketsHttpHandler
            {
                UseCookies = false,
                AllowAutoRedirect = false,
                UseProxy = false,
                AutomaticDecompression = DecompressionMethods.None,
                ConnectTimeout = TimeSpan.FromSeconds(5)
            })
            // O HttpClientFactory registraria cada repasse como log de informação; o .NET não registra requisição.
            .RemoveAllLoggers();
    }
}
