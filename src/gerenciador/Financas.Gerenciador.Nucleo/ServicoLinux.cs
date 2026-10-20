using System.Runtime.Versioning;
using System.Text.RegularExpressions;

namespace Financas.Gerenciador;

/// <summary>
/// Unidade <c>financaslocal.service</c> do <c>systemd --user</c>, escrita pelo <c>3-publicar-atualizar.sh</c>. É do
/// próprio usuário: ligar, desligar e ler o registro não pedem senha nem administrador (ADR 0033).
/// </summary>
[UnsupportedOSPlatform("windows")]
public sealed partial class ServicoLinux : ServicoDoSite
{
    public const string Unidade = "financaslocal.service";

    public override string ComoInstalar => "Instale pelos scripts: rode scripts/geral/instalar.sh na pasta do projeto.";

    public override EstadoServico Estado()
    {
        var p = Propriedades("LoadState", "ActiveState");
        if (p.GetValueOrDefault("LoadState") != "loaded") return EstadoServico.NaoInstalado;
        return p.GetValueOrDefault("ActiveState") switch
        {
            "active" or "reloading" => EstadoServico.Rodando,
            "activating" => EstadoServico.Ligando,
            "deactivating" => EstadoServico.Desligando,
            _ => EstadoServico.Parado // inactive, failed
        };
    }

    /// <summary>A <c>WorkingDirectory</c> da unidade (a pasta da API); sem ela, a pasta do Financas.Api.dll do ExecStart.</summary>
    public override string? PastaDaApi()
    {
        Dictionary<string, string> p;
        try
        {
            p = Propriedades("LoadState", "WorkingDirectory", "ExecStart");
        }
        catch (InvalidOperationException)
        {
            return null;
        }
        if (p.GetValueOrDefault("LoadState") != "loaded") return null;
        if (p.GetValueOrDefault("WorkingDirectory") is { Length: > 0 } pasta && pasta.StartsWith('/')) return pasta;
        var achado = DllDaApi().Match(p.GetValueOrDefault("ExecStart") ?? "");
        return achado.Success ? Path.GetDirectoryName(achado.Groups[1].Value) : null;
    }

    [GeneratedRegex(@"(/\S*?Financas\.Api\.dll)")]
    private static partial Regex DllDaApi();

    /// <summary><c>ligar|desligar|reiniciar</c> pelo <c>systemctl --user</c>, que espera o serviço mudar.</summary>
    public static Task<Saida> ExecutarAsync(string acao)
    {
        var verbo = acao switch
        {
            "ligar" => "start",
            "desligar" => "stop",
            "reiniciar" => "restart",
            _ => throw new ArgumentOutOfRangeException(nameof(acao), acao, null)
        };
        return Comando.RodarAsync("systemctl", "--user", verbo, Unidade);
    }

    /// <summary>
    /// O serviço do usuário só roda com a sessão aberta, a não ser com o "linger" do systemd-logind. null quando não
    /// dá para saber (sem logind).
    /// </summary>
    public static bool? SobeSemLogin()
    {
        var saida = Comando.Rodar("loginctl", "show-user", Environment.UserName, "-p", "Linger", "--value");
        return saida.Ok ? saida.Texto.Trim() == "yes" : null;
    }

    /// <summary>
    /// <c>loginctl enable-linger</c> para o próprio usuário. Na maioria das distribuições a sessão ativa pode fazer isso
    /// sem senha; se a política pedir, o próprio sistema mostra a janela de autenticação.
    /// </summary>
    public static Task<Saida> ManterNoArSemLoginAsync() => Comando.RodarAsync("loginctl", "enable-linger");

    /// <summary>As últimas linhas do registro do serviço (journald): o que a API escreveu, inclusive ao não ligar.</summary>
    public static Task<Saida> RegistroAsync(int linhas)
        => Comando.RodarAsync("journalctl", "--user", "-u", Unidade, "-n", linhas.ToString(), "--no-pager", "-o", "short-iso");

    private static Dictionary<string, string> Propriedades(params string[] nomes)
    {
        var argumentos = new List<string> { "--user", "show", Unidade };
        foreach (var nome in nomes) argumentos.AddRange(["-p", nome]);
        var saida = Comando.Rodar(TimeSpan.FromSeconds(10), "systemctl", [.. argumentos]);
        if (!saida.Ok)
            throw new InvalidOperationException("Não deu para falar com o systemd do usuário (systemctl --user). " + saida.Mensagem);
        var resultado = new Dictionary<string, string>();
        foreach (var linha in saida.Texto.Split('\n'))
        {
            var igual = linha.IndexOf('=');
            if (igual > 0) resultado[linha[..igual]] = linha[(igual + 1)..].Trim();
        }
        return resultado;
    }
}
