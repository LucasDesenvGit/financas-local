using System.ComponentModel;
using System.Diagnostics;

namespace Financas.Gerenciador;

/// <summary>Saída de um programa do sistema (systemctl, loginctl, journalctl…).</summary>
public sealed record Saida(int Codigo, string Texto, string Erro)
{
    public bool Ok => Codigo == 0;

    /// <summary>A mensagem de erro do programa, ou o texto, para mostrar quando algo não deu certo.</summary>
    public string Mensagem => string.IsNullOrWhiteSpace(Erro) ? Texto.Trim() : Erro.Trim();
}

/// <summary>
/// Roda um programa com argumentos separados (sem shell: nada é interpretado), espera e devolve a saída. Usado só com
/// argumentos fixos do próprio gerenciador.
/// </summary>
public static partial class Comando
{
    public static Saida Rodar(string programa, params string[] argumentos)
        => Rodar(TimeSpan.FromSeconds(30), programa, argumentos);

    public static Saida Rodar(TimeSpan limite, string programa, params string[] argumentos)
    {
        var info = new ProcessStartInfo(programa)
        {
            RedirectStandardOutput = true, RedirectStandardError = true, UseShellExecute = false, CreateNoWindow = true
        };
        foreach (var argumento in argumentos) info.ArgumentList.Add(argumento);
        // Saída limpa para ler: sem paginador nem cores do systemd.
        info.Environment["SYSTEMD_PAGER"] = "";
        info.Environment["SYSTEMD_COLORS"] = "0";
        try
        {
            using var processo = Process.Start(info)!;
            var texto = processo.StandardOutput.ReadToEndAsync();
            var erro = processo.StandardError.ReadToEndAsync();
            if (!processo.WaitForExit(limite))
            {
                try { processo.Kill(entireProcessTree: true); } catch (InvalidOperationException) { }
                return new Saida(-1, "", $"{programa} não respondeu em {limite.TotalSeconds:0} segundos.");
            }
            return new Saida(processo.ExitCode, texto.Result, erro.Result);
        }
        catch (Win32Exception)
        {
            return new Saida(-1, "", $"O programa {programa} não foi encontrado neste computador.");
        }
    }

    public static Task<Saida> RodarAsync(string programa, params string[] argumentos)
        => Task.Run(() => Rodar(programa, argumentos));
}
