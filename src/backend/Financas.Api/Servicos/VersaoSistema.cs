using System.Reflection;

namespace Financas.Api.Servicos;

/// <summary>Versão do Finanças Local (Directory.Build.props), sem o "+commit" que o SDK acrescenta.</summary>
public static class VersaoSistema
{
    public static string Atual { get; } = Ler();

    private static string Ler()
    {
        var assembly = typeof(VersaoSistema).Assembly;
        var versao = assembly.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion
                     ?? assembly.GetName().Version?.ToString() ?? "—";
        var mais = versao.IndexOf('+');
        return mais > 0 ? versao[..mais] : versao;
    }
}
