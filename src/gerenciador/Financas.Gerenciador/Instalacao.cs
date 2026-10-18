using System.Reflection;
using System.Text.Json;

namespace Financas.Gerenciador;

/// <summary>
/// O que o .msi instalou: pasta do programa (a do próprio gerenciador, ao lado da API), pasta de dados, endereço do
/// site e versão. Lê os mesmos <c>appsettings</c> da API, para não ter uma segunda configuração.
/// </summary>
internal sealed class Instalacao
{
    public const string NomeServico = "FinancasLocal";
    /// <summary>De onde vêm as versões novas (Releases públicos; nada do usuário é enviado).</summary>
    public const string Repositorio = "LucasDesenvGit/financas-local";
    private const string PastaDadosPadrao = @"%ProgramData%\FinancasLocal\dados";

    public static Instalacao Atual { get; } = new();

    public string PastaPrograma { get; }
    public string PastaDados { get; }
    public string CaminhoBanco => Path.Combine(PastaDados, "sql", "FinancasLocal.db");
    public Uri Site { get; }
    public Version Versao { get; }
    public string VersaoTexto { get; }

    private Instalacao()
    {
        PastaPrograma = AppContext.BaseDirectory;
        var producao = LerJson(Path.Combine(PastaPrograma, "appsettings.Production.json"));
        var geral = LerJson(Path.Combine(PastaPrograma, "appsettings.json"));

        var dados = Texto(producao, "Dados", "Pasta") ?? PastaDadosPadrao;
        PastaDados = Path.GetFullPath(Environment.ExpandEnvironmentVariables(dados));

        var url = Texto(producao, "Kestrel", "Endpoints", "Https", "Url")
                  ?? Texto(geral, "Kestrel", "Endpoints", "Https", "Url")
                  ?? "https://127.0.0.1:5443";
        Site = new Uri(url.TrimEnd('/') + "/");

        VersaoTexto = Assembly.GetExecutingAssembly().GetCustomAttribute<AssemblyInformationalVersionAttribute>()?
            .InformationalVersion.Split('+')[0] ?? "0.0.0";
        Versao = Version.TryParse(VersaoTexto.Split('-')[0], out var v) ? v : new Version(0, 0, 0);
    }

    public Uri Pagina(string caminho) => new(Site, caminho);

    private static JsonElement? LerJson(string caminho)
    {
        try
        {
            if (!File.Exists(caminho)) return null;
            using var doc = JsonDocument.Parse(File.ReadAllText(caminho),
                new JsonDocumentOptions { CommentHandling = JsonCommentHandling.Skip, AllowTrailingCommas = true });
            return doc.RootElement.Clone();
        }
        catch (Exception e) when (e is IOException or UnauthorizedAccessException or JsonException)
        {
            return null;
        }
    }

    private static string? Texto(JsonElement? raiz, params string[] caminho)
    {
        if (raiz is not JsonElement atual) return null;
        foreach (var parte in caminho)
        {
            if (atual.ValueKind != JsonValueKind.Object || !atual.TryGetProperty(parte, out atual)) return null;
        }
        return atual.ValueKind == JsonValueKind.String ? atual.GetString() : null;
    }
}
