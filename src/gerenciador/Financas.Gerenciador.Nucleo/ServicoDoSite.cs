namespace Financas.Gerenciador;

/// <summary>Estado do serviço do sistema que roda o site, igual para o Windows e o Linux.</summary>
public enum EstadoServico { NaoInstalado, Parado, Ligando, Rodando, Desligando }

/// <summary>
/// O serviço do sistema que roda o site: Serviço do Windows (<see cref="ServicoWindows"/>) ou unidade do
/// <c>systemd --user</c> no Linux (<see cref="ServicoLinux"/>). Ligar e desligar ficam em cada um, porque no Windows
/// pedem administrador (UAC) e no Linux não.
/// </summary>
public abstract class ServicoDoSite
{
    public static ServicoDoSite Atual { get; } = OperatingSystem.IsWindows() ? new ServicoWindows() : new ServicoLinux();

    /// <summary>
    /// Estado do serviço; <see cref="EstadoServico.NaoInstalado"/> quando ele não está registrado. Lança
    /// <see cref="InvalidOperationException"/> quando não dá para perguntar ao sistema.
    /// </summary>
    public abstract EstadoServico Estado();

    /// <summary>Pasta da API (Financas.Api.dll e os appsettings), ou null se o serviço não estiver registrado.</summary>
    public abstract string? PastaDaApi();

    /// <summary>O que dizer quando o site não está instalado.</summary>
    public abstract string ComoInstalar { get; }
}
