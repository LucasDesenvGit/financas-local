using Avalonia;

[assembly: System.Runtime.Versioning.SupportedOSPlatform("linux")]

namespace Financas.Gerenciador;

internal static class Program
{
    /// <summary>
    /// Sem argumentos: a janela. <c>--bandeja</c>: só o ícone da bandeja (início com o sistema).
    /// <c>--situacao</c>: escreve a situação no terminal e sai (0 = site no ar); serve também para a CI.
    /// </summary>
    [STAThread]
    private static int Main(string[] args)
    {
        if (args is ["--situacao"]) return Situacao();

        using var unica = InstanciaUnica.Tentar();
        if (unica is null) return 0; // já estava aberto: a outra cópia mostra a janela
        App.Instancia = unica;
        return AppBuilder.Configure<App>().UsePlatformDetect()
            .StartWithClassicDesktopLifetime(args, Avalonia.Controls.ShutdownMode.OnExplicitShutdown);
    }

    private static int Situacao()
    {
        var instalacao = Instalacao.Atual;
        var situacao = VerificadorSituacao.ConsultarAsync(instalacao, CancellationToken.None).GetAwaiter().GetResult();
        Console.WriteLine(situacao.Titulo);
        Console.WriteLine(situacao.Detalhe);
        Console.WriteLine($"Pasta da API: {instalacao.PastaPrograma}");
        Console.WriteLine($"Banco: {instalacao.CaminhoBanco}");
        Console.WriteLine($"Gerenciador: {instalacao.VersaoTexto}");
        return situacao.Estado == EstadoSite.NoAr ? 0 : 1;
    }
}
