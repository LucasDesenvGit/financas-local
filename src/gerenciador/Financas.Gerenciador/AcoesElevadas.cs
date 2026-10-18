using System.ComponentModel;
using System.Diagnostics;
using Microsoft.Data.Sqlite;

namespace Financas.Gerenciador;

/// <summary>
/// O que precisa de administrador: ligar/desligar o serviço e copiar o banco (a pasta de dados é só de SYSTEM e
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

    /// <summary>Argumento entre aspas para a linha de comando (caminhos do Windows não têm aspas).</summary>
    public static string Aspas(string valor) => "\"" + valor.TrimEnd('\\') + "\"";
}
