using System.ComponentModel;
using System.Runtime.Versioning;
using System.Security;
using System.ServiceProcess;
using System.Text.RegularExpressions;
using Microsoft.Win32;

namespace Financas.Gerenciador;

/// <summary>
/// Serviço do Windows <c>FinancasLocal</c> (LocalSystem). Ler o estado e o registro não precisa de administrador;
/// ligar e desligar precisam (o gerenciador do Windows roda <see cref="Executar"/> elevado, pelo UAC).
/// </summary>
[SupportedOSPlatform("windows")]
public sealed partial class ServicoWindows : ServicoDoSite
{
    public const string Nome = "FinancasLocal";
    private static readonly TimeSpan Espera = TimeSpan.FromSeconds(60);

    public override string ComoInstalar => "Instale pelo arquivo FinancasLocal.msi.";

    public override EstadoServico Estado()
    {
        try
        {
            using var servico = new ServiceController(Nome);
            return servico.Status switch
            {
                ServiceControllerStatus.Stopped => EstadoServico.Parado,
                ServiceControllerStatus.Running => EstadoServico.Rodando,
                ServiceControllerStatus.StartPending or ServiceControllerStatus.ContinuePending => EstadoServico.Ligando,
                _ => EstadoServico.Desligando
            };
        }
        catch (InvalidOperationException)
        {
            return EstadoServico.NaoInstalado;
        }
    }

    /// <summary>
    /// Pela chave do serviço no registro: <c>"...\Financas.Api.exe"</c> (.msi) ou
    /// <c>"...\dotnet.exe" "...\Financas.Api.dll"</c> (scripts). A chave é legível sem administrador.
    /// </summary>
    public override string? PastaDaApi()
    {
        try
        {
            using var chave = Registry.LocalMachine.OpenSubKey(@"SYSTEM\CurrentControlSet\Services\" + Nome);
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

    /// <summary>Modo administrador: <c>ligar|desligar|reiniciar</c>. 0 = feito.</summary>
    public static int Executar(string acao)
    {
        try
        {
            using var servico = new ServiceController(Nome);
            if (acao is "desligar" or "reiniciar" && servico.Status != ServiceControllerStatus.Stopped)
            {
                servico.Stop();
                servico.WaitForStatus(ServiceControllerStatus.Stopped, Espera);
            }
            if (acao is "ligar" or "reiniciar")
            {
                servico.Refresh();
                if (servico.Status != ServiceControllerStatus.Running)
                {
                    servico.Start();
                    servico.WaitForStatus(ServiceControllerStatus.Running, Espera);
                }
            }
            return 0;
        }
        catch (Exception e) when (e is InvalidOperationException or System.ServiceProcess.TimeoutException or Win32Exception)
        {
            return 1;
        }
    }
}
