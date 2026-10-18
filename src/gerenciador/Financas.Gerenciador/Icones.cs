using System.Drawing.Drawing2D;
using System.Reflection;
using System.Runtime.InteropServices;

namespace Financas.Gerenciador;

/// <summary>
/// Ícone do app (o mesmo cofrinho do site) e, na área de notificação, o ícone com um ponto da cor da situação.
/// A cor nunca é a única pista (WCAG 1.4.1): o texto da dica e o da janela dizem a situação.
/// </summary>
internal static class Icones
{
    public static Icon App { get; } = Carregar();

    private static readonly Dictionary<EstadoSite, Icon> PorEstado = new();

    public static Icon Bandeja(EstadoSite estado)
    {
        if (PorEstado.TryGetValue(estado, out var icone)) return icone;
        var cor = estado switch
        {
            EstadoSite.NoAr => Color.FromArgb(0x2e, 0x9e, 0x6b),
            EstadoSite.Desligado => Color.FromArgb(0xd2, 0x54, 0x6b),
            EstadoSite.SemResposta or EstadoSite.Mudando => Color.FromArgb(0xe0, 0xa1, 0x00),
            _ => Color.Gray
        };
        using var imagem = new Bitmap(32, 32);
        using (var g = Graphics.FromImage(imagem))
        {
            g.SmoothingMode = SmoothingMode.AntiAlias;
            g.InterpolationMode = InterpolationMode.HighQualityBicubic;
            using var base_ = App.ToBitmap();
            g.DrawImage(base_, new Rectangle(0, 0, 32, 32));
            using var borda = new SolidBrush(Color.White);
            using var preenchimento = new SolidBrush(cor);
            g.FillEllipse(borda, 17, 17, 15, 15);
            g.FillEllipse(preenchimento, 19, 19, 11, 11);
        }
        var alca = imagem.GetHicon();
        icone = (Icon)Icon.FromHandle(alca).Clone();
        DestroyIcon(alca);
        PorEstado[estado] = icone;
        return icone;
    }

    private static Icon Carregar()
    {
        using var fluxo = Assembly.GetExecutingAssembly().GetManifestResourceStream("icone.ico");
        return fluxo is null ? SystemIcons.Application : new Icon(fluxo);
    }

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool DestroyIcon(IntPtr alca);
}
