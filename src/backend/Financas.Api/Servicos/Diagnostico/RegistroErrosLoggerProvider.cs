using Financas.Api.Entidades;
using Microsoft.Extensions.Logging.Abstractions;

namespace Financas.Api.Servicos.Diagnostico;

/// <summary>
/// Provedor de log que manda para a tabela de erros (#11, ADR 0022) os avisos e erros do código do próprio sistema:
/// categorias <c>Financas.*</c>, o tratador de exceções não tratadas da API (categoria "Erro") e a queda de serviços
/// em segundo plano (<c>Microsoft.Extensions.Hosting</c>). O resto do framework (EF Core, Kestrel) fica de fora,
/// para não duplicar nem gravar SQL. Os outros provedores (console, log de eventos) continuam recebendo tudo.
/// </summary>
[ProviderAlias("TabelaDeErros")]
public sealed class RegistroErrosLoggerProvider : ILoggerProvider
{
    /// <summary>Categoria do tratador de exceções em Program.cs.</summary>
    public const string CategoriaApi = "Erro";

    private readonly FilaErros _fila;
    private readonly IHttpContextAccessor _http;

    public RegistroErrosLoggerProvider(FilaErros fila, IHttpContextAccessor http)
    {
        _fila = fila;
        _http = http;
    }

    public ILogger CreateLogger(string categoria)
        => Capturar(categoria) ? new RegistroErrosLogger(categoria, Origem(categoria), _fila, _http) : NullLogger.Instance;

    public void Dispose() { }

    /// <summary>O gravador fica de fora: uma falha ao gravar não pode virar outro erro a gravar.</summary>
    public static bool Capturar(string categoria)
        => categoria != typeof(GravadorErrosHostedService).FullName
           && (categoria == CategoriaApi
               || categoria.StartsWith("Financas.", StringComparison.Ordinal)
               || categoria.StartsWith("Microsoft.Extensions.Hosting", StringComparison.Ordinal));

    /// <summary>Agrupamento para o filtro da tela, pela pasta do serviço que registrou.</summary>
    public static string Origem(string categoria) => categoria switch
    {
        CategoriaApi => "API",
        _ when categoria.StartsWith("Financas.Api.Controllers", StringComparison.Ordinal) => "API",
        _ when categoria.StartsWith("Financas.Api.Servicos.Pluggy", StringComparison.Ordinal) => "Open Finance",
        _ when categoria.StartsWith("Financas.Api.Servicos.Investimentos", StringComparison.Ordinal) => "Investimentos",
        _ when categoria.StartsWith("Financas.Api.Servicos.Cambio", StringComparison.Ordinal) => "Câmbio",
        _ => "Sistema"
    };

    private sealed class RegistroErrosLogger : ILogger
    {
        private readonly string _categoria;
        private readonly string _origem;
        private readonly FilaErros _fila;
        private readonly IHttpContextAccessor _http;

        public RegistroErrosLogger(string categoria, string origem, FilaErros fila, IHttpContextAccessor http)
        {
            _categoria = categoria;
            _origem = origem;
            _fila = fila;
            _http = http;
        }

        public IDisposable? BeginScope<TState>(TState state) where TState : notnull => null;

        public bool IsEnabled(LogLevel nivel) => nivel is LogLevel.Warning or LogLevel.Error or LogLevel.Critical;

        public void Log<TState>(LogLevel nivel, EventId eventId, TState state, Exception? excecao,
            Func<TState, Exception?, string> formatar)
        {
            if (!IsEnabled(nivel)) return;
            string? rota = null, usuario = null;
            try
            {
                // Log feito durante uma requisição (inclusive o que ela disparou): rota sem query string e usuário.
                if (_http.HttpContext is { } ctx)
                {
                    rota = $"{ctx.Request.Method} {ctx.Request.Path}";
                    usuario = ctx.User.Identity?.IsAuthenticated == true ? ctx.User.Identity.Name : null;
                }
            }
            catch (ObjectDisposedException)
            {
                // Requisição já terminou (trabalho em segundo plano que herdou o contexto): fica sem rota.
            }
            var mensagem = formatar(state, excecao);
            if (string.IsNullOrWhiteSpace(mensagem)) mensagem = excecao?.Message ?? "(sem mensagem)";
            _fila.Registrar(new RegistroErro(DateTime.UtcNow, (NivelErro)(int)nivel, _origem, _categoria, mensagem,
                excecao?.ToString(), rota, usuario));
        }
    }
}
