namespace Financas.Gerenciador;

/// <summary>
/// "Iniciar com o sistema": um <c>.desktop</c> em <c>~/.config/autostart</c> (padrão freedesktop, vale no KDE, GNOME,
/// XFCE…) que abre o gerenciador com <c>--bandeja</c>. É do usuário: nada disso precisa de senha.
/// </summary>
internal static class Preferencias
{
    private static string ArquivoInicio => Path.Combine(
        Environment.GetEnvironmentVariable("XDG_CONFIG_HOME") is { Length: > 0 } config
            ? config
            : Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".config"),
        "autostart", "financaslocal-gerenciador.desktop");

    public static bool IniciarComSistema
    {
        get => File.Exists(ArquivoInicio);
        set
        {
            if (!value)
            {
                File.Delete(ArquivoInicio);
                return;
            }
            Directory.CreateDirectory(Path.GetDirectoryName(ArquivoInicio)!);
            File.WriteAllText(ArquivoInicio, $"""
                [Desktop Entry]
                Type=Application
                Name=Gerenciador do Finanças Local
                Comment=Avisa se o site do Finanças Local cair
                Exec={Aspas(Environment.ProcessPath!)} --bandeja
                Icon={Path.Combine(AppContext.BaseDirectory, "icone.png")}
                Terminal=false
                X-GNOME-Autostart-enabled=true

                """);
        }
    }

    /// <summary>Argumento do Exec= entre aspas (especificação Desktop Entry: escapar \ " ` $).</summary>
    private static string Aspas(string valor)
        => "\"" + valor.Replace("\\", "\\\\").Replace("\"", "\\\"").Replace("`", "\\`").Replace("$", "\\$") + "\"";
}
