using System.Reflection;
using System.Security;
using System.Text.Json;
using System.Text.RegularExpressions;
using Microsoft.Win32;

namespace Financas.Gerenciador;

/// <summary>
/// Onde está o site: pasta da API (achada pelo serviço do Windows), banco, endereço e versão. Lê os mesmos
/// <c>appsettings</c> da API, para não ter uma segunda configuração. Serve para as duas instalações:
/// <list type="bullet">
/// <item><b>.msi</b>: <c>Dados:Pasta</c> (ex.: %ProgramData%\FinancasLocal\dados) define o banco; atualiza pelo
/// GitHub.</item>
/// <item><b>scripts</b> (<c>instalar-windows.cmd</c>/<c>3-publicar-atualizar.sh</c>): <c>Banco:Caminho</c> define o
/// banco; atualiza pelos scripts, então o gerenciador não oferece o .msi.</item>
/// </list>
/// </summary>
internal sealed partial class Instalacao
{
    public const string NomeServico = "FinancasLocal";
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
        PastaPrograma = PastaDoServico() ?? AppContext.BaseDirectory;
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

        VersaoTexto = Assembly.GetExecutingAssembly().GetCustomAttribute<AssemblyInformationalVersionAttribute>()?
            .InformationalVersion.Split('+')[0] ?? "0.0.0";
        Versao = Version.TryParse(VersaoTexto.Split('-')[0], out var v) ? v : new Version(0, 0, 0);
    }

    public Uri Pagina(string caminho) => new(Site, caminho);

    /// <summary>
    /// Pasta da API pelo registro do serviço: <c>"...\Financas.Api.exe"</c> (.msi) ou
    /// <c>"...\dotnet.exe" "...\Financas.Api.dll"</c> (scripts). A chave do serviço é legível sem administrador.
    /// </summary>
    private static string? PastaDoServico()
    {
        try
        {
            using var chave = Registry.LocalMachine.OpenSubKey(@"SYSTEM\CurrentControlSet\Services\" + NomeServico);
            if (chave?.GetValue("ImagePath") is not string linha) return null;
            var achado = ExecutavelDaApi().Match(Environment.ExpandEnvironmentVariables(linha));
            return achado.Success ? Path.GetDirectoryName(achado.Groups[1].Value) : null;
        }
        catch (Exception e) when (e is SecurityException or UnauthorizedAccessException or IOException)
        {
            return null;
        }
    }

    [GeneratedRegex(@"([A-Za-z]:\\[^""]*?Financas\.Api\.(?:dll|exe))", RegexOptions.IgnoreCase)]
    private static partial Regex ExecutavelDaApi();

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
