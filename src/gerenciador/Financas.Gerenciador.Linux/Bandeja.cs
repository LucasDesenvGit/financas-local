using Avalonia;
using Avalonia.Controls;
using Avalonia.Media;
using Avalonia.Media.Imaging;
using Avalonia.Platform;

namespace Financas.Gerenciador;

/// <summary>
/// Ícone na bandeja (StatusNotifierItem) com um ponto da cor da situação, e o aviso de "o site caiu" pelo
/// <c>notify-send</c>. A cor nunca é a única pista (WCAG 1.4.1): a dica e a janela dizem a situação em texto.
/// </summary>
internal static class Bandeja
{
    public static readonly Uri IconeApp = new("avares://Financas.Gerenciador.Linux/Recursos/icone.png");
    private static readonly Dictionary<EstadoSite, WindowIcon> PorEstado = new();

    /// <summary>
    /// Há onde mostrar o ícone? O KDE, o XFCE e outros têm o StatusNotifierWatcher; o GNOME puro só com extensão
    /// (AppIndicator). Sem ele, "Iniciar com o sistema" abre a janela minimizada em vez de só o ícone.
    /// </summary>
    public static bool Disponivel { get; } = Comando.Rodar(TimeSpan.FromSeconds(5), "busctl", "--user", "--no-legend", "list")
        is { Ok: true } saida && saida.Texto.Contains("org.kde.StatusNotifierWatcher", StringComparison.Ordinal);

    public static WindowIcon Icone(EstadoSite estado)
    {
        if (PorEstado.TryGetValue(estado, out var icone)) return icone;
        var cor = estado switch
        {
            EstadoSite.NoAr => Color.FromRgb(0x2e, 0x9e, 0x6b),
            EstadoSite.Desligado => Color.FromRgb(0xd2, 0x54, 0x6b),
            EstadoSite.SemResposta or EstadoSite.Mudando => Color.FromRgb(0xe0, 0xa1, 0x00),
            _ => Colors.Gray
        };
        using var app = new Bitmap(AssetLoader.Open(IconeApp));
        var imagem = new RenderTargetBitmap(new PixelSize(64, 64), new Vector(96, 96));
        using (var desenho = imagem.CreateDrawingContext())
        {
            desenho.DrawImage(app, new Rect(0, 0, 64, 64));
            desenho.DrawEllipse(Brushes.White, null, new Point(49, 49), 15, 15);
            desenho.DrawEllipse(new SolidColorBrush(cor), null, new Point(49, 49), 11, 11);
        }
        icone = new WindowIcon(imagem);
        PorEstado[estado] = icone;
        return icone;
    }

    /// <summary>Notificação da área de trabalho; sem o notify-send (libnotify), não avisa e segue.</summary>
    public static void Avisar(string texto)
    {
        var icone = Path.Combine(AppContext.BaseDirectory, "icone.png");
        _ = Comando.RodarAsync("notify-send", "--app-name=Finanças Local", "--icon=" + icone, "Finanças Local", texto);
    }
}
