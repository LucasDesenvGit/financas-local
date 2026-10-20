using System.Reflection;
using System.Text.Json;

namespace Financas.Gerenciador;

/// <summary>
/// Onde está o site: pasta da API (achada pelo serviço do sistema, <see cref="ServicoDoSite"/>), banco, endereço e
/// versão. Lê os mesmos <c>appsettings</c> da API, para não ter uma segunda configuração. Serve para as instalações:
/// <list type="bullet">
/// <item><b>.msi</b> (Windows): <c>Dados:Pasta</c> (ex.: %ProgramData%\FinancasLocal\dados) define o banco; atualiza
/// pelo GitHub.</item>
/// <item><b>scripts</b> (<c>instalar-windows.cmd</c>/<c>scripts/geral/instalar.sh</c>/<c>3-publicar-atualizar.sh</c>, Windows ou
/// Linux): <c>Banco:Caminho</c> define o banco; atualiza pelos scripts, então o gerenciador não oferece o .msi.</item>
/// </list>
/// </summary>
public sealed class Instalacao
{
    /// <summary>De onde vêm as versões novas (Releases públicos; nada do usuário é enviado).</summary>
    public const string Repositorio = "LucasDesenvGit/financas-local";
    private const string PastaDadosPadrao = @"%ProgramData%\FinancasLocal\dados";

    public static Instalacao Atual { get; } = new();

    /// <summary>Pasta da API (a do serviço FinancasLocal; sem o serviço, a do próprio gerenciador).</summary>
    public string PastaPrograma { get; }
    public string PastaDados { get; }
    public string CaminhoBanco { get; }
    /// <summary>Instalado pelo .msi: só então o gerenciador procura e instala versões novas do GitHub.</summary>
    public bool PeloMsi { get; }
    public Uri Site { get; }
    public Version Versao { get; }
    public string VersaoTexto { get; }

    private Instalacao()
    {
        PastaPrograma = ServicoDoSite.Atual.PastaDaApi() ?? AppContext.BaseDirectory;
        var producao = LerJson(Path.Combine(PastaPrograma, "appsettings.Production.json"));
        var geral = LerJson(Path.Combine(PastaPrograma, "appsettings.json"));

        var dados = Texto(producao, "Dados", "Pasta");
        var banco = Texto(producao, "Banco", "Caminho") ?? Texto(geral, "Banco", "Caminho");
        PeloMsi = dados is not null;
        if (dados is null && banco is not null)
        {
            // Instalação pelos scripts: o banco fica onde o appsettings diz (relativo à pasta da API, se não for absoluto).
            CaminhoBanco = Path.GetFullPath(Path.Combine(PastaPrograma, banco.Replace('/', Path.DirectorySeparatorChar)));
            var pastaSql = Path.GetDirectoryName(CaminhoBanco)!;
            PastaDados = string.Equals(Path.GetFileName(pastaSql), "sql", StringComparison.OrdinalIgnoreCase)
                ? Path.GetDirectoryName(pastaSql)! : pastaSql;
        }
        else
        {
            PastaDados = Path.GetFullPath(Environment.ExpandEnvironmentVariables(dados ?? PastaDadosPadrao));
            CaminhoBanco = Path.Combine(PastaDados, "sql", "FinancasLocal.db");
        }

        var url = Texto(producao, "Kestrel", "Endpoints", "Https", "Url")
                  ?? Texto(geral, "Kestrel", "Endpoints", "Https", "Url")
                  ?? "https://127.0.0.1:5443";
        Site = new Uri(url.TrimEnd('/') + "/");

        // A do executável do gerenciador (Directory.Build.props, a mesma do site).
        VersaoTexto = (Assembly.GetEntryAssembly() ?? Assembly.GetExecutingAssembly()).GetCustomAttribute<AssemblyInformationalVersionAttribute>()?
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
