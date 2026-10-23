using System.Runtime.InteropServices;
using Microsoft.Win32;

namespace Financas.Gerenciador;

/// <summary>
/// As cores do site (<c>src/frontend/src/styles.css</c>), para o gerenciador parecer parte do Finanças Local (Nielsen 4).
/// Claro ou escuro conforme o Windows ("Modo de aplicativo"). O texto suave é um pouco mais escuro que o do site, para
/// manter contraste de 4,5:1 no texto pequeno (WCAG 1.4.3).
/// </summary>
internal sealed record Tema(
    bool Escuro, Color Fundo, Color Superficie, Color Superficie2, Color Superficie3, Color Linha, Color Texto,
    Color TextoSuave, Color Primaria, Color PrimariaHover, Color PrimariaTexto, Color PrimariaSuave, Color Positivo,
    Color Negativo, Color Alerta, Color MarcaInicio, Color MarcaFim, Color MarcaTexto)
{
    public static Tema Atual { get; private set; } = Detectar() ? CriarEscuro() : CriarClaro();

    private static Tema CriarClaro() => new(false,
        Fundo: Cor("#f5f4fa"), Superficie: Cor("#ffffff"), Superficie2: Cor("#f4f3fa"), Superficie3: Cor("#eae8f4"),
        Linha: Cor("#e2dfee"), Texto: Cor("#2b2939"), TextoSuave: Cor("#66637e"), Primaria: Cor("#6267da"),
        PrimariaHover: Cor("#575bcc"), PrimariaTexto: Cor("#ffffff"), PrimariaSuave: Cor("#edecfe"),
        Positivo: Cor("#2a9a6a"), Negativo: Cor("#d2546b"), Alerta: Cor("#b27612"),
        MarcaInicio: Cor("#b9b6ff"), MarcaFim: Cor("#9fd2f6"), MarcaTexto: Cor("#23214a"));

    private static Tema CriarEscuro() => new(true,
        Fundo: Cor("#13121a"), Superficie: Cor("#1b1a25"), Superficie2: Cor("#22212e"), Superficie3: Cor("#2b2a3a"),
        Linha: Cor("#2e2d3e"), Texto: Cor("#e4e2f0"), TextoSuave: Cor("#a9a6c2"), Primaria: Cor("#a3a6ff"),
        PrimariaHover: Cor("#b3b6ff"), PrimariaTexto: Cor("#16152a"), PrimariaSuave: Cor("#2a2950"),
        Positivo: Cor("#7fd9ad"), Negativo: Cor("#ff9db0"), Alerta: Cor("#f3c56f"),
        MarcaInicio: Cor("#b9b6ff"), MarcaFim: Cor("#9fd2f6"), MarcaTexto: Cor("#23214a"));

    /// <summary>Cor do ponto da situação (sempre acompanhada do texto: a cor nunca é a única pista, WCAG 1.4.1).</summary>
    public Color DaSituacao(EstadoSite estado) => estado switch
    {
        EstadoSite.NoAr => Positivo,
        EstadoSite.Desligado => Negativo,
        EstadoSite.SemResposta or EstadoSite.Mudando => Alerta,
        _ => TextoSuave
    };

    /// <summary>Barra de título escura no tema escuro (Windows 10 20H1+ e 11).</summary>
    public void AplicarBarraDeTitulo(IntPtr janela)
    {
        if (!Escuro) return;
        var sim = 1;
        try { DwmSetWindowAttribute(janela, 20, ref sim, sizeof(int)); }
        catch (Exception e) when (e is DllNotFoundException or EntryPointNotFoundException) { }
    }

    /// <summary>
    /// #337: no tema escuro, a barra de rolagem nativa (TextBox, RichTextBox) fica escura, com o tema "DarkMode_Explorer"
    /// do próprio Windows (10 1809+ e 11). O WinForms não deixa pintar a barra nativa; sem o tema, ela fica clara e larga,
    /// destoando da janela. No tema claro, nada muda. Vale também quando o controle recria a janela (HandleCreated).
    /// </summary>
    public void AplicarRolagem(Control controle)
    {
        if (!Escuro) return;
        void Aplicar()
        {
            try { SetWindowTheme(controle.Handle, "DarkMode_Explorer", null); }
            catch (Exception e) when (e is DllNotFoundException or EntryPointNotFoundException) { }
        }
        if (controle.IsHandleCreated) Aplicar();
        controle.HandleCreated += (_, _) => Aplicar();
    }

    /// <summary>Windows em modo escuro de aplicativos (HKCU …\Personalize\AppsUseLightTheme = 0).</summary>
    private static bool Detectar()
    {
        try
        {
            using var chave = Registry.CurrentUser.OpenSubKey(@"Software\Microsoft\Windows\CurrentVersion\Themes\Personalize");
            return chave?.GetValue("AppsUseLightTheme") is int claro && claro == 0;
        }
        catch (Exception e) when (e is System.Security.SecurityException or UnauthorizedAccessException or IOException)
        {
            return false;
        }
    }

    private static Color Cor(string hex) => ColorTranslator.FromHtml(hex);

    [DllImport("uxtheme.dll", CharSet = CharSet.Unicode)]
    private static extern int SetWindowTheme(IntPtr janela, string? nomeApp, string? listaIds);

    [DllImport("dwmapi.dll")]
    private static extern int DwmSetWindowAttribute(IntPtr janela, int atributo, ref int valor, int tamanho);
}
