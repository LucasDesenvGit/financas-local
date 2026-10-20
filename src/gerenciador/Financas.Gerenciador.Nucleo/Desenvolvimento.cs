using System.ComponentModel;
using System.Diagnostics;
using System.Text.RegularExpressions;

namespace Financas.Gerenciador;

/// <summary>
/// Número de versão do Finanças Local: <c>X.Y.Z</c> (versão publicada) ou <c>X.Y.Z-dev.N</c> (gerada do develop pela
/// CI, N = número da execução). Só a de desenvolvimento ganha o caminho de atualizar pelo develop (ADR 0040).
/// </summary>
public sealed partial record VersaoDoSistema(Version Base, int? Dev) : IComparable<VersaoDoSistema>
{
    public bool EhDesenvolvimento => Dev is not null;

    public static VersaoDoSistema? Ler(string? texto)
    {
        var achado = texto is null ? null : Padrao().Match(texto.Trim().TrimStart('v', 'V'));
        if (achado is not { Success: true } || !Version.TryParse(achado.Groups[1].Value, out var versao)) return null;
        return new VersaoDoSistema(versao, achado.Groups[2].Success ? int.Parse(achado.Groups[2].Value) : null);
    }

    /// <summary>Pelo X.Y.Z e, no mesmo X.Y.Z, pelo N; a publicada vem depois das de desenvolvimento do mesmo número.</summary>
    public int CompareTo(VersaoDoSistema? outra)
    {
        if (outra is null) return 1;
        var porBase = Base.CompareTo(outra.Base);
        if (porBase != 0) return porBase;
        return (Dev ?? int.MaxValue).CompareTo(outra.Dev ?? int.MaxValue);
    }

    public override string ToString() => Dev is int n ? $"{Base}-dev.{n}" : Base.ToString();

    [GeneratedRegex(@"^(\d+\.\d+\.\d+)(?:-dev\.(\d+))?")]
    private static partial Regex Padrao();
}

/// <summary>
/// Instalação de desenvolvimento pelos scripts (ADR 0040): a pasta do projeto é um clone git na branch develop. Só
/// então o gerenciador oferece "Atualizar para a última do develop", que roda o <c>scripts/geral/atualizar.sh</c>
/// (Linux) ou o <c>scripts/windows/atualizar-windows.ps1</c> (Windows). Clone em outra branch, numa tag ou sem git:
/// nada aparece, como numa versão publicada.
/// </summary>
public static class DesenvolvimentoPeloGit
{
    public const string Branch = "develop";

    /// <summary>Códigos de saída do atualizar.sh e do atualizar-windows.ps1.</summary>
    public const int Atualizado = 0, Recusado = 2, SemNovidade = 10;

    /// <summary>A pasta do projeto, se a instalação é pelos scripts e a pasta é um clone com o script de atualizar.</summary>
    public static string? Raiz(Instalacao instalacao)
    {
        if (instalacao.PeloMsi) return null;
        var raiz = Path.GetFullPath(Path.Combine(instalacao.PastaPrograma, "..", ".."));
        var git = Path.Combine(raiz, ".git");
        var temScript = File.Exists(Path.Combine(raiz, "scripts", "geral", "atualizar.sh"));
        return (Directory.Exists(git) || File.Exists(git)) && temScript ? raiz : null;
    }

    /// <summary>A branch do clone (pelo git), ou null sem git ou fora de uma branch.</summary>
    public static string? BranchAtual(string raiz)
    {
        var saida = Comando.Rodar(TimeSpan.FromSeconds(10), "git", "-C", raiz, "rev-parse", "--abbrev-ref", "HEAD");
        var branch = saida.Texto.Trim();
        return saida.Ok && branch.Length > 0 && branch != "HEAD" ? branch : null;
    }

    /// <summary>A pasta do projeto, se dá para atualizar pelo develop (clone na branch develop, com git).</summary>
    public static string? Disponivel(Instalacao instalacao)
        => Raiz(instalacao) is string raiz && BranchAtual(raiz) == Branch ? raiz : null;
}

public static partial class Comando
{
    /// <summary>
    /// Roda e entrega cada linha (saída e erro, sem as cores do terminal) enquanto o programa roda, para a tela mostrar
    /// ao vivo. Devolve o código de saída; -1 se o programa não existe ou foi cancelado.
    /// </summary>
    public static async Task<int> RodarAoVivoAsync(string programa, IEnumerable<string> argumentos, Action<string> linha,
        CancellationToken ct)
    {
        var info = new ProcessStartInfo(programa)
        {
            RedirectStandardOutput = true, RedirectStandardError = true, RedirectStandardInput = true,
            UseShellExecute = false, CreateNoWindow = true
        };
        foreach (var argumento in argumentos) info.ArgumentList.Add(argumento);
        info.Environment["GIT_TERMINAL_PROMPT"] = "0";
        Process processo;
        try { processo = Process.Start(info)!; }
        catch (Win32Exception)
        {
            linha($"O programa {programa} não foi encontrado neste computador.");
            return -1;
        }
        using (processo)
        {
            processo.StandardInput.Close();
            void Entregar(object _, DataReceivedEventArgs e)
            {
                if (e.Data is string texto) linha(Cores().Replace(texto, ""));
            }
            processo.OutputDataReceived += Entregar;
            processo.ErrorDataReceived += Entregar;
            processo.BeginOutputReadLine();
            processo.BeginErrorReadLine();
            try
            {
                await processo.WaitForExitAsync(ct);
            }
            catch (OperationCanceledException)
            {
                try { processo.Kill(entireProcessTree: true); } catch (InvalidOperationException) { }
                return -1;
            }
            processo.WaitForExit(); // espera as últimas linhas chegarem
            return processo.ExitCode;
        }
    }

    [GeneratedRegex(@"\x1B\[[0-9;]*[A-Za-z]")]
    private static partial Regex Cores();
}
