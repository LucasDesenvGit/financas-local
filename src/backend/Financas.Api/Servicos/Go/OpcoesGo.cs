using System.Text.RegularExpressions;

namespace Financas.Api.Servicos.Go;

/// <summary>
/// Convivência com o back-end em Go (#231, ADR 0049): o que o .NET repassa ao Go e o que deixa de fazer.
/// Lido uma vez, na partida; mudar a configuração vale ao reiniciar o serviço, sem reinstalar.
/// <list type="bullet">
/// <item><c>Go:Prefixos</c>: prefixos de rota repassados ao Go, separados por ";" (ex.: <c>/api/go;/api/trabalho</c>).
/// Prefixo fora da lista continua no .NET. Voltar um assunto para o .NET é tirar o prefixo dela.</item>
/// <item><c>Go:Tarefas</c>: tarefas em segundo plano que passaram para o Go e ficam desligadas no .NET
/// (<see cref="TarefasConhecidas"/>), separadas por ";".</item>
/// <item><c>Go:Executavel</c>: caminho do executável do Go (padrão: <c>financas-go</c> ao lado da API).</item>
/// </list>
/// Texto separado por ";", e não lista JSON, de propósito: no .NET, uma lista vazia num appsettings de cima não
/// apaga a lista do de baixo, e <c>"Prefixos": ""</c> apaga (o mesmo jeito do <c>AllowedHosts</c>).
/// </summary>
public sealed partial record OpcoesGo(IReadOnlyList<string> Prefixos, IReadOnlySet<string> Tarefas, string Executavel,
    string CaminhoBanco)
{
    /// <summary>Tarefas do .NET que um assunto em Go pode assumir (os <c>AddHostedService</c> do Program.cs).</summary>
    public static readonly IReadOnlySet<string> TarefasConhecidas =
        new HashSet<string>(StringComparer.OrdinalIgnoreCase) { "Sincronizacao", "Cambio", "Cripto" };

    /// <summary>Sem prefixo e sem tarefa, o Go nem é iniciado.</summary>
    public bool Ligado => Prefixos.Count > 0 || Tarefas.Count > 0;

    /// <summary>A tarefa passou para o Go: o .NET não a registra (não roda duas vezes).</summary>
    public bool TarefaNoGo(string nome) => Tarefas.Contains(nome);

    /// <summary>Nome do executável publicado ao lado da API pelos scripts e pelo .msi.</summary>
    public static string NomeExecutavel => OperatingSystem.IsWindows() ? "financas-go.exe" : "financas-go";

    // Só /api/<algo>, em minúsculas: "/" ou "/api" levariam o login e o front-end junto.
    [GeneratedRegex("^/api(/[a-z0-9-]+)+$")]
    private static partial Regex PrefixoValido();

    public static OpcoesGo Ler(IConfiguration cfg, string caminhoBanco)
    {
        var prefixos = Separar(cfg["Go:Prefixos"]).Select(p => p.TrimEnd('/')).Distinct(StringComparer.OrdinalIgnoreCase).ToList();
        foreach (var prefixo in prefixos)
        {
            if (!PrefixoValido().IsMatch(prefixo))
                throw new InvalidOperationException(
                    $"Go:Prefixos tem \"{prefixo}\": cada prefixo começa com /api/ e tem só letras minúsculas, números e \"-\".");
            // Login, sessão e CSRF ficam no .NET até a inversão (#85); os erros, até a #298.
            if (prefixo is "/api/auth" or "/api/erros" || prefixo.StartsWith("/api/auth/") || prefixo.StartsWith("/api/erros/"))
                throw new InvalidOperationException($"Go:Prefixos tem \"{prefixo}\", que fica no .NET até a inversão (ADR 0049).");
        }

        var tarefas = new HashSet<string>(Separar(cfg["Go:Tarefas"]), StringComparer.OrdinalIgnoreCase);
        var desconhecidas = tarefas.Where(t => !TarefasConhecidas.Contains(t)).ToList();
        if (desconhecidas.Count > 0)
            throw new InvalidOperationException(
                $"Go:Tarefas tem {string.Join(", ", desconhecidas)}: as conhecidas são {string.Join(", ", TarefasConhecidas)}.");

        var executavel = cfg["Go:Executavel"];
        executavel = string.IsNullOrWhiteSpace(executavel)
            ? Path.Combine(AppContext.BaseDirectory, NomeExecutavel)
            : Path.GetFullPath(executavel, AppContext.BaseDirectory);
        return new OpcoesGo(prefixos, tarefas, executavel, Path.GetFullPath(caminhoBanco));
    }

    private static IEnumerable<string> Separar(string? texto)
        => (texto ?? "").Split(';', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
}
