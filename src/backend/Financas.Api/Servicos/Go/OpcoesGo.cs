using System.Diagnostics;
using System.Globalization;
using System.Text.RegularExpressions;

namespace Financas.Api.Servicos.Go;

/// <summary>
/// Convivência com o back-end em Go (#231, ADR 0049): o que o .NET repassa ao Go e o que deixa de fazer.
/// Lido uma vez, na partida; mudar a configuração vale ao reiniciar o serviço, sem reinstalar.
/// <list type="bullet">
/// <item><c>Go:Prefixos</c>: prefixos de rota repassados ao Go. Uma entrada por assunto, cada uma com os prefixos
/// dele separados por ";" (ex.: <c>"cambio": "/api/cambio;/api/cripto"</c>); entrada vazia é assunto desligado.
/// Prefixo fora da lista continua no .NET. Voltar um assunto para o .NET é esvaziar a entrada dele (#363).</item>
/// <item><c>Go:Tarefas</c>: tarefas em segundo plano que passaram para o Go e ficam desligadas no .NET
/// (<see cref="TarefasConhecidas"/>), separadas por ";".</item>
/// <item><c>Go:Executavel</c>: caminho do executável do Go (padrão: <c>financas-go</c> ao lado da API).</item>
/// <item><c>Go:InicioDoRelogio</c>: <b>só para teste</b> (#363), vazio por padrão. O relógio do Go começa nesse
/// instante, na partida do .NET, e anda dali (<see cref="InstanteDoRelogio"/>).</item>
/// </list>
/// Por que entradas, e não um texto só nem uma lista JSON: assuntos feitos em paralelo mexem cada um na sua linha,
/// sem conflito no merge; e um <c>appsettings</c> de cima desliga um assunto sozinho (<c>"cambio": ""</c>). O texto
/// com ";" continua valendo e ganha das entradas: <c>"Prefixos": ""</c> (ou <c>Go__Prefixos=""</c>) desliga tudo.
/// Lista JSON, não: no .NET, uma lista vazia num <c>appsettings</c> de cima não apaga a do de baixo.
/// </summary>
public sealed partial record OpcoesGo(IReadOnlyList<string> Prefixos, IReadOnlySet<string> Tarefas, string Executavel,
    string CaminhoBanco, DateTimeOffset? InicioDoRelogio = null)
{
    /// <summary>Tarefas do .NET que um assunto em Go pode assumir (os <c>AddHostedService</c> do Program.cs).</summary>
    public static readonly IReadOnlySet<string> TarefasConhecidas =
        new HashSet<string>(StringComparer.OrdinalIgnoreCase) { "Sincronizacao", "Cambio", "Cripto" };

    /// <summary>Quando a configuração foi lida (na partida do .NET), no relógio monotônico.</summary>
    private readonly long _lidoEm = Stopwatch.GetTimestamp();

    /// <summary>Sem prefixo e sem tarefa, o Go nem é iniciado.</summary>
    public bool Ligado => Prefixos.Count > 0 || Tarefas.Count > 0;

    /// <summary>A tarefa passou para o Go: o .NET não a registra (não roda duas vezes).</summary>
    public bool TarefaNoGo(string nome) => Tarefas.Contains(nome);

    /// <summary>Nome do executável publicado ao lado da API pelos scripts e pelo .msi.</summary>
    public static string NomeExecutavel => OperatingSystem.IsWindows() ? "financas-go.exe" : "financas-go";

    /// <summary>
    /// A hora em que o relógio do Go começa ao subir agora: <c>Go:InicioDoRelogio</c> mais o tempo desde a partida do
    /// .NET (monotônico). É o mesmo relógio que a libfaketime dá ao .NET no contrato (<c>FAKETIME="@..."</c>, que
    /// começa na partida e anda), e o Go reiniciado continua nele, sem voltar ao início. Null: relógio do sistema.
    /// </summary>
    public DateTimeOffset? InstanteDoRelogio() => InicioDoRelogio + Stopwatch.GetElapsedTime(_lidoEm);

    // Só /api/<algo>, em minúsculas: "/" ou "/api" levariam o login e o front-end junto.
    [GeneratedRegex("^/api(/[a-z0-9-]+)+$")]
    private static partial Regex PrefixoValido();

    // RFC 3339 com o fuso explícito (o Go lê assim; sem fuso, dependeria do computador). Fração de até 7 casas.
    [GeneratedRegex(@"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,7})?(Z|[+-]\d{2}:\d{2})$")]
    private static partial Regex InstanteValido();

    public static OpcoesGo Ler(IConfiguration cfg, string caminhoBanco)
    {
        var prefixos = LerPrefixos(cfg.GetSection("Go:Prefixos")).Select(p => p.TrimEnd('/')).Distinct(StringComparer.OrdinalIgnoreCase).ToList();
        foreach (var prefixo in prefixos)
        {
            if (!PrefixoValido().IsMatch(prefixo))
                throw new InvalidOperationException(
                    $"Go:Prefixos tem \"{prefixo}\": cada prefixo começa com /api/ e tem só letras minúsculas, números e \"-\".");
            // Login, sessão, CSRF e os erros do sistema ficam no .NET até a inversão (#85): o gravador de erros é um
            // só, e atrás do .NET é o do .NET (o Go grava os dele só quando roda sozinho, #298).
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
        return new OpcoesGo(prefixos, tarefas, executavel, Path.GetFullPath(caminhoBanco), LerInicioDoRelogio(cfg["Go:InicioDoRelogio"]));
    }

    /// <summary>
    /// O texto de <c>Go:Prefixos</c>, se houver (inclusive vazio: desliga tudo); senão, as entradas dela, uma por
    /// assunto. Cada uma pode ter vários prefixos separados por ";".
    /// </summary>
    private static IEnumerable<string> LerPrefixos(IConfigurationSection secao)
    {
        if (secao.Value is not null) return Separar(secao.Value);
        return secao.GetChildren().SelectMany(entrada => entrada.Value is null
            ? throw new InvalidOperationException(
                $"Go:Prefixos:{entrada.Key} tem de ser texto, com os prefixos do assunto separados por \";\" (vazio desliga).")
            : Separar(entrada.Value));
    }

    /// <summary>
    /// Só para teste (#363): o instante validado, nunca o texto como veio. Vazio é o relógio do sistema.
    /// </summary>
    private static DateTimeOffset? LerInicioDoRelogio(string? texto)
    {
        if (string.IsNullOrWhiteSpace(texto)) return null;
        texto = texto.Trim();
        if (!InstanteValido().IsMatch(texto)
            || !DateTimeOffset.TryParse(texto, CultureInfo.InvariantCulture, DateTimeStyles.None, out var instante))
            throw new InvalidOperationException(
                $"Go:InicioDoRelogio \"{texto}\" não é um instante com fuso, como 2026-10-15T01:00:00Z. "
                + "É só para teste: no uso normal, deixe vazio.");
        return instante.ToUniversalTime();
    }

    private static IEnumerable<string> Separar(string? texto)
        => (texto ?? "").Split(';', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
}
