using System.ComponentModel;
using System.Diagnostics;
using Microsoft.Data.Sqlite;

namespace Financas.Gerenciador;

/// <summary>
/// O que precisa de administrador: ligar/desligar o serviço, copiar o banco e reinstalar pelos scripts (a pasta de dados é só de SYSTEM e
/// Administradores). O gerenciador reabre a si mesmo com "runas" (o UAC mostra o que vai rodar) e espera terminar.
/// </summary>
internal static class AcoesElevadas
{
    private const int CanceladoNoUac = 1223;

    public enum Resultado { Feito, Cancelado, Falhou }

    /// <summary>Roda <c>Financas.Gerenciador.exe argumentos</c> como administrador e espera.</summary>
    public static async Task<Resultado> ExecutarAsync(string argumentos)
    {
        var info = new ProcessStartInfo(Environment.ProcessPath!, argumentos) { UseShellExecute = true, Verb = "runas" };
        try
        {
            using var processo = Process.Start(info)!;
            await processo.WaitForExitAsync();
            return processo.ExitCode == 0 ? Resultado.Feito : Resultado.Falhou;
        }
        catch (Win32Exception e) when (e.NativeErrorCode == CanceladoNoUac)
        {
            return Resultado.Cancelado;
        }
    }

    /// <summary>Modo administrador: <c>--backup "destino.db"</c>. Cópia consistente (VACUUM INTO) com o site no ar.</summary>
    public static int Backup(string destino)
    {
        try
        {
            CopiaDoBanco.Salvar(Instalacao.Atual.CaminhoBanco, destino);
            return 0;
        }
        catch (FileNotFoundException)
        {
            return 2;
        }
        catch (Exception e) when (e is SqliteException or IOException or UnauthorizedAccessException)
        {
            return 1;
        }
    }

    /// <summary>
    /// Modo administrador: <c>--instalar-pelos-scripts</c> (ADR 0040). Roda o <c>instalar-windows.ps1</c> do clone da
    /// instalação de desenvolvimento, depois de o <c>atualizar-windows.ps1</c> ter trazido o código como o usuário. A pasta
    /// vem da própria instalação (nunca da linha de comando), e o git não roda aqui. A saída vai para o log do instalador
    /// (instaladores\instalacao-windows.log), que a janela acompanha.
    /// </summary>
    public static int InstalarPelosScripts()
    {
        if (DesenvolvimentoPeloGit.Raiz(Instalacao.Atual) is not string raiz) return 3;
        var script = Path.Combine(raiz, "scripts", "windows", "instalar-windows.ps1");
        var info = new ProcessStartInfo("powershell.exe") { UseShellExecute = false, CreateNoWindow = true, WorkingDirectory = raiz };
        foreach (var a in new[] { "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script }) info.ArgumentList.Add(a);
        using var processo = Process.Start(info)!;
        processo.WaitForExit();
        return processo.ExitCode;
    }

    /// <summary>Argumento entre aspas para a linha de comando (caminhos do Windows não têm aspas).</summary>
    public static string Aspas(string valor) => "\"" + valor.TrimEnd('\\') + "\"";
}
