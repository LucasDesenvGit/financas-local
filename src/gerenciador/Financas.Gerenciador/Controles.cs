using System.ComponentModel;
using System.Drawing.Drawing2D;

namespace Financas.Gerenciador;

/// <summary>Formas arredondadas, na escala da tela (DPI).</summary>
internal static class Desenho
{
    /// <summary>
    /// #337: o ícone "download" do Lucide (o mesmo do site, licença ISC), desenhado no retângulo dado, com traço
    /// arredondado de 2/24 do tamanho: a bandeja (U com os cantos de baixo arredondados) e a seta para baixo.
    /// </summary>
    public static void IconeBaixar(Graphics g, RectangleF r, Color cor)
    {
        var s = r.Width / 24f;
        PointF P(float x, float y) => new(r.X + x * s, r.Y + y * s);
        using var caneta = new Pen(cor, 2f * s) { StartCap = LineCap.Round, EndCap = LineCap.Round, LineJoin = LineJoin.Round };
        using var bandeja = new GraphicsPath();
        bandeja.AddLine(P(3, 15), P(3, 19));
        bandeja.AddArc(r.X + 3 * s, r.Y + 17 * s, 4 * s, 4 * s, 180, -90);
        bandeja.AddLine(P(5, 21), P(19, 21));
        bandeja.AddArc(r.X + 17 * s, r.Y + 17 * s, 4 * s, 4 * s, 90, -90);
        bandeja.AddLine(P(21, 19), P(21, 15));
        g.DrawPath(caneta, bandeja);
        g.DrawLines(caneta, new[] { P(7, 10), P(12, 15), P(17, 10) });
        g.DrawLine(caneta, P(12, 15), P(12, 3));
    }

    public static GraphicsPath Arredondado(RectangleF r, float raio)
    {
        var d = Math.Min(raio * 2, Math.Min(r.Width, r.Height));
        var caminho = new GraphicsPath();
        if (d <= 0) { caminho.AddRectangle(r); return caminho; }
        caminho.AddArc(r.X, r.Y, d, d, 180, 90);
        caminho.AddArc(r.Right - d, r.Y, d, d, 270, 90);
        caminho.AddArc(r.Right - d, r.Bottom - d, d, d, 0, 90);
        caminho.AddArc(r.X, r.Bottom - d, d, d, 90, 90);
        caminho.CloseFigure();
        return caminho;
    }

    public static Color Misturar(Color a, Color b, float quantoDeB)
        => Color.FromArgb(
            (int)(a.R + (b.R - a.R) * quantoDeB), (int)(a.G + (b.G - a.G) * quantoDeB), (int)(a.B + (b.B - a.B) * quantoDeB));
}

/// <summary>Faixa do topo com o degradê da marca, o cofrinho e o nome (a mesma identidade do site).</summary>
internal sealed class Cabecalho : Control
{
    private readonly Bitmap _icone = new(Icones.App.ToBitmap(), new Size(128, 128));

    public Cabecalho()
    {
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.ResizeRedraw
                 | ControlStyles.UserPaint, true);
        TabStop = false;
        AccessibleRole = AccessibleRole.StaticText;
        AccessibleName = "Finanças Local, gerenciador do site";
        Margin = Padding.Empty;
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        var tema = Tema.Atual;
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        g.InterpolationMode = InterpolationMode.HighQualityBicubic;
        using (var fundo = new LinearGradientBrush(ClientRectangle, tema.MarcaInicio, tema.MarcaFim, LinearGradientMode.ForwardDiagonal))
            g.FillRectangle(fundo, ClientRectangle);

        var margem = LogicalToDeviceUnits(18);
        var lado = LogicalToDeviceUnits(40);
        var topo = (Height - lado) / 2;
        using (var moldura = Desenho.Arredondado(new RectangleF(margem - 2, topo - 2, lado + 4, lado + 4), LogicalToDeviceUnits(12)))
        using (var branco = new SolidBrush(Color.FromArgb(150, Color.White)))
            g.FillPath(branco, moldura);
        g.DrawImage(_icone, new Rectangle(margem, topo, lado, lado));

        var x = margem + lado + LogicalToDeviceUnits(14);
        using var titulo = new Font("Segoe UI Semibold", 14f);
        using var subtitulo = new Font("Segoe UI", 9.5f);
        var alturaTitulo = TextRenderer.MeasureText(g, "Finanças Local", titulo).Height;
        var alturaSub = TextRenderer.MeasureText(g, "Gerenciador do site", subtitulo).Height;
        var y = (Height - alturaTitulo - alturaSub + LogicalToDeviceUnits(2)) / 2;
        TextRenderer.DrawText(g, "Finanças Local", titulo, new Point(x, y), tema.MarcaTexto, TextFormatFlags.NoPadding);
        TextRenderer.DrawText(g, "Gerenciador do site", subtitulo, new Point(x, y + alturaTitulo - LogicalToDeviceUnits(2)),
            Desenho.Misturar(tema.MarcaTexto, tema.MarcaInicio, 0.15f), TextFormatFlags.NoPadding);
    }

    protected override void Dispose(bool disposing)
    {
        if (disposing) _icone.Dispose();
        base.Dispose(disposing);
    }
}

/// <summary>
/// Cartão com cantos arredondados (como os blocos do site). O conteúdo vai em <see cref="Conteudo"/>, que cresce com o
/// que tem dentro.
/// </summary>
internal sealed class Cartao : Panel
{
    public FlowLayoutPanel Conteudo { get; } = new()
    {
        FlowDirection = FlowDirection.TopDown, WrapContents = false, AutoSize = true, AutoSizeMode = AutoSizeMode.GrowAndShrink,
        Margin = Padding.Empty
    };

    public Cartao(int largura)
    {
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.ResizeRedraw
                 | ControlStyles.UserPaint | ControlStyles.SupportsTransparentBackColor, true);
        var tema = Tema.Atual;
        BackColor = tema.Fundo;
        AutoSize = true;
        AutoSizeMode = AutoSizeMode.GrowAndShrink;
        Padding = new Padding(16, 14, 16, 14);
        Margin = new Padding(0, 0, 0, 12);
        MinimumSize = new Size(largura, 0);
        Conteudo.BackColor = tema.Superficie;
        Conteudo.MinimumSize = new Size(largura - Padding.Horizontal, 0);
        Conteudo.Dock = DockStyle.Fill;
        Controls.Add(Conteudo);
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        var tema = Tema.Atual;
        e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
        var r = new RectangleF(0.5f, 0.5f, Width - 1.5f, Height - 1.5f);
        using var caminho = Desenho.Arredondado(r, LogicalToDeviceUnits(12));
        using var fundo = new SolidBrush(tema.Superficie);
        using var borda = new Pen(tema.Linha);
        e.Graphics.FillPath(fundo, caminho);
        e.Graphics.DrawPath(borda, caminho);
    }
}

internal enum EstiloBotao { Primario, Secundario, Discreto }

/// <summary>Códigos dos ícones usados nos botões; os mesmos na Segoe Fluent Icons e na Segoe MDL2 Assets.</summary>
internal static class IconeBotao
{
    public const char AbrirSite = '\uE8A7';
    public const char Ligar = '\uE7E8';
    public const char Reiniciar = '\uE72C';
    public const char Baixar = '\uE896';
    public const char Procurar = '\uE721';
    public const char Informacao = '\uE946';
    public const char Cancelar = '\uE711';
    public const char Erros = '\uE7BA';
    public const char Manual = '\uE897';
    public const char Salvar = '\uE74E';
}

/// <summary>
/// Botão arredondado nas cores do site. Continua sendo um <see cref="Button"/> (teclado, Enter/Espaço, atalho com &amp;,
/// leitor de tela); só o desenho muda, com contorno de foco visível (WCAG 2.4.7) e altura de pelo menos 34 px (2.5.8).
/// </summary>
internal sealed class Botao : Button
{
    private bool _sobre, _apertado;
    private EstiloBotao _estilo;
    /// <summary>Ícone à esquerda do texto. Só desenho: o nome acessível continua sendo o texto.</summary>
    private readonly char? _icone;

    /// <summary>Fonte de ícones do próprio Windows (11: Segoe Fluent Icons; 10: Segoe MDL2 Assets); null se nenhuma existir.</summary>
    private static readonly string? FonteIcones = new[] { "Segoe Fluent Icons", "Segoe MDL2 Assets" }
        .FirstOrDefault(nome => FontFamily.Families.Any(f => f.Name == nome));

    private Font? _fonteIcone;
    private Font? FonteIcone => FonteIcones is null ? null : _fonteIcone ??= new Font(FonteIcones, Font.Size + 0.5f);
    private string? GlifoIcone => _icone is char c && c != IconeBotao.Baixar && FonteIcones is not null ? c.ToString() : null;
    private int VaoIcone => LogicalToDeviceUnits(8);
    /// <summary>#337: a seta de "baixar" é desenhada (o "download" do Lucide, o mesmo do site), e não um caractere da fonte de
    /// ícones, que saía fino e com outro peso que o texto em semibold.</summary>
    private bool IconeDesenhado => _icone == IconeBotao.Baixar;
    private int TamanhoIconeDesenhado => LogicalToDeviceUnits(16);

    [DesignerSerializationVisibility(DesignerSerializationVisibility.Hidden)]
    public EstiloBotao Estilo
    {
        get => _estilo;
        set { _estilo = value; Invalidate(); }
    }

    public Botao(string texto, EstiloBotao estilo = EstiloBotao.Secundario, char? icone = null)
    {
        _icone = icone;
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.UserPaint, true);
        Text = texto;
        Estilo = estilo;
        FlatStyle = FlatStyle.Flat;
        FlatAppearance.BorderSize = 0;
        UseVisualStyleBackColor = false;
        Cursor = Cursors.Hand;
        AutoSize = true;
        AutoSizeMode = AutoSizeMode.GrowAndShrink;
        Padding = new Padding(14, 6, 14, 6);
        MinimumSize = new Size(0, 34);
        Margin = new Padding(0, 0, 8, 8);
        Font = new Font("Segoe UI Semibold", 9.75f);
    }

    public override Size GetPreferredSize(Size proposto)
    {
        var texto = TextRenderer.MeasureText(Text, Font, Size.Empty, TextFormatFlags.SingleLine);
        if (IconeDesenhado)
            texto.Width += TamanhoIconeDesenhado + VaoIcone;
        else if (GlifoIcone is string glifo)
            texto.Width += TextRenderer.MeasureText(glifo, FonteIcone, Size.Empty, TextFormatFlags.SingleLine | TextFormatFlags.NoPadding).Width + VaoIcone;
        return new Size(texto.Width + Padding.Horizontal + LogicalToDeviceUnits(4),
            Math.Max(texto.Height + Padding.Vertical, LogicalToDeviceUnits(34)));
    }

    protected override void OnMouseEnter(EventArgs e) { _sobre = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _sobre = _apertado = false; Invalidate(); base.OnMouseLeave(e); }
    protected override void OnMouseDown(MouseEventArgs e) { _apertado = true; Invalidate(); base.OnMouseDown(e); }
    protected override void OnMouseUp(MouseEventArgs e) { _apertado = false; Invalidate(); base.OnMouseUp(e); }
    protected override void OnEnabledChanged(EventArgs e) { Invalidate(); base.OnEnabledChanged(e); }
    protected override void OnGotFocus(EventArgs e) { Invalidate(); base.OnGotFocus(e); }
    protected override void OnLostFocus(EventArgs e) { Invalidate(); base.OnLostFocus(e); }

    protected override void OnFontChanged(EventArgs e) { _fonteIcone?.Dispose(); _fonteIcone = null; base.OnFontChanged(e); }

    protected override void Dispose(bool disposing)
    {
        if (disposing) _fonteIcone?.Dispose();
        base.Dispose(disposing);
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        var tema = Tema.Atual;
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        g.Clear(Parent?.BackColor ?? tema.Superficie);

        Color fundo, texto, borda;
        switch (Estilo)
        {
            case EstiloBotao.Primario:
                fundo = _apertado ? Desenho.Misturar(tema.PrimariaHover, Color.Black, 0.08f) : _sobre ? tema.PrimariaHover : tema.Primaria;
                texto = tema.PrimariaTexto;
                borda = fundo;
                break;
            case EstiloBotao.Discreto:
                fundo = _sobre || _apertado ? tema.PrimariaSuave : Parent?.BackColor ?? tema.Superficie;
                texto = tema.Primaria;
                borda = fundo;
                break;
            default:
                fundo = _apertado ? tema.Superficie3 : _sobre ? tema.PrimariaSuave : tema.Superficie2;
                texto = tema.Texto;
                borda = _sobre ? Desenho.Misturar(tema.Linha, tema.Primaria, 0.45f) : tema.Linha;
                break;
        }
        if (!Enabled)
        {
            fundo = Desenho.Misturar(fundo, Parent?.BackColor ?? tema.Superficie, 0.55f);
            texto = Desenho.Misturar(texto, Parent?.BackColor ?? tema.Superficie, 0.5f);
            borda = Desenho.Misturar(borda, Parent?.BackColor ?? tema.Superficie, 0.55f);
        }

        var inset = LogicalToDeviceUnits(2);
        var r = new RectangleF(inset, inset, Width - inset * 2 - 1, Height - inset * 2 - 1);
        using (var caminho = Desenho.Arredondado(r, LogicalToDeviceUnits(8)))
        using (var pincel = new SolidBrush(fundo))
        using (var caneta = new Pen(borda))
        {
            g.FillPath(pincel, caminho);
            g.DrawPath(caneta, caminho);
        }

        if (Focused && ShowFocusCues)
        {
            var anel = new RectangleF(0.5f, 0.5f, Width - 2, Height - 2);
            using var caminhoFoco = Desenho.Arredondado(anel, LogicalToDeviceUnits(10));
            using var canetaFoco = new Pen(tema.Primaria, LogicalToDeviceUnits(2));
            g.DrawPath(canetaFoco, caminhoFoco);
        }

        var flags = TextFormatFlags.HorizontalCenter | TextFormatFlags.VerticalCenter | TextFormatFlags.SingleLine;
        if (!ShowKeyboardCues) flags |= TextFormatFlags.HidePrefix;
        const TextFormatFlags medir = TextFormatFlags.SingleLine | TextFormatFlags.NoPadding;
        if (IconeDesenhado)
        {
            // Ícone desenhado + vão + texto, o conjunto centralizado no botão.
            var tam = TamanhoIconeDesenhado;
            var largTexto = TextRenderer.MeasureText(Text, Font, Size.Empty, medir | (flags & TextFormatFlags.HidePrefix)).Width;
            var x0 = (Width - (tam + VaoIcone + largTexto)) / 2;
            Desenho.IconeBaixar(g, new RectangleF(x0, (Height - tam) / 2f, tam, tam), texto);
            TextRenderer.DrawText(g, Text, Font, new Rectangle(x0 + tam + VaoIcone, 0, largTexto + 2, Height), texto,
                (flags & ~TextFormatFlags.HorizontalCenter) | TextFormatFlags.NoPadding);
            return;
        }
        if (GlifoIcone is not string glifo)
        {
            TextRenderer.DrawText(g, Text, Font, ClientRectangle, texto, flags);
            return;
        }
        // Ícone + vão + texto, o conjunto centralizado no botão.
        var tamIcone = TextRenderer.MeasureText(glifo, FonteIcone, Size.Empty, medir);
        var tamTexto = TextRenderer.MeasureText(Text, Font, Size.Empty, medir | (flags & TextFormatFlags.HidePrefix));
        var x = (Width - (tamIcone.Width + VaoIcone + tamTexto.Width)) / 2;
        TextRenderer.DrawText(g, glifo, FonteIcone, new Rectangle(x, 0, tamIcone.Width, Height), texto,
            TextFormatFlags.VerticalCenter | medir);
        TextRenderer.DrawText(g, Text, Font, new Rectangle(x + tamIcone.Width + VaoIcone, 0, tamTexto.Width + 2, Height), texto,
            (flags & ~TextFormatFlags.HorizontalCenter) | TextFormatFlags.NoPadding);
    }
}

/// <summary>
/// Interruptor (liga/desliga) para preferências que valem na hora, sem "Salvar" (NN/g, Toggle-Switch Guidelines). Por
/// baixo continua um <see cref="CheckBox"/>: Espaço alterna e o leitor de tela ouve marcado/desmarcado. O rótulo não muda
/// com o estado (WAI-ARIA APG, Switch Pattern).
/// </summary>
internal sealed class Interruptor : CheckBox
{
    private const int LarguraTrilho = 36, AlturaTrilho = 20, Vao = 10;
    private bool _sobre;

    public Interruptor(string texto)
    {
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.UserPaint, true);
        Text = texto;
        AutoSize = true;
        Cursor = Cursors.Hand;
        Margin = new Padding(0, 2, 0, 6);
    }

    public override Size GetPreferredSize(Size proposto)
    {
        var texto = TextRenderer.MeasureText(Text, Font, Size.Empty, TextFormatFlags.SingleLine);
        return new Size(LogicalToDeviceUnits(LarguraTrilho + Vao + 4) + texto.Width,
            Math.Max(texto.Height, LogicalToDeviceUnits(AlturaTrilho + 8)));
    }

    protected override void OnMouseEnter(EventArgs e) { _sobre = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _sobre = false; Invalidate(); base.OnMouseLeave(e); }
    protected override void OnGotFocus(EventArgs e) { Invalidate(); base.OnGotFocus(e); }
    protected override void OnLostFocus(EventArgs e) { Invalidate(); base.OnLostFocus(e); }

    protected override void OnPaint(PaintEventArgs e)
    {
        var tema = Tema.Atual;
        var g = e.Graphics;
        var fundoPai = Parent?.BackColor ?? tema.Superficie;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        g.Clear(fundoPai);

        var largura = LogicalToDeviceUnits(LarguraTrilho);
        var altura = LogicalToDeviceUnits(AlturaTrilho);
        var margem = LogicalToDeviceUnits(2);
        var trilho = new RectangleF(margem, (Height - altura) / 2f, largura, altura);

        // Desligado: trilho com borda em contraste de pelo menos 3:1 com o fundo (WCAG 1.4.11); ligado: a cor primária.
        var corTrilho = Checked ? (_sobre ? tema.PrimariaHover : tema.Primaria) : tema.Superficie3;
        var corBorda = Checked ? corTrilho : (_sobre ? tema.Primaria : tema.TextoSuave);
        var corBolinha = Checked ? tema.PrimariaTexto : tema.TextoSuave;
        if (!Enabled)
        {
            corTrilho = Desenho.Misturar(corTrilho, fundoPai, 0.55f);
            corBorda = Desenho.Misturar(corBorda, fundoPai, 0.55f);
            corBolinha = Desenho.Misturar(corBolinha, fundoPai, 0.55f);
        }
        using (var caminho = Desenho.Arredondado(trilho, altura / 2f))
        using (var pincel = new SolidBrush(corTrilho))
        using (var caneta = new Pen(corBorda, LogicalToDeviceUnits(1)))
        {
            g.FillPath(pincel, caminho);
            g.DrawPath(caneta, caminho);
        }
        var diametro = altura - LogicalToDeviceUnits(8);
        var x = Checked ? trilho.Right - LogicalToDeviceUnits(4) - diametro : trilho.X + LogicalToDeviceUnits(4);
        using (var bolinha = new SolidBrush(corBolinha))
            g.FillEllipse(bolinha, x, trilho.Y + LogicalToDeviceUnits(4), diametro, diametro);

        if (Focused && ShowFocusCues)
        {
            var anel = RectangleF.Inflate(trilho, LogicalToDeviceUnits(2) - 0.5f, LogicalToDeviceUnits(2) - 0.5f);
            using var caminhoFoco = Desenho.Arredondado(anel, anel.Height / 2f);
            using var canetaFoco = new Pen(tema.Primaria, LogicalToDeviceUnits(2));
            g.DrawPath(canetaFoco, caminhoFoco);
        }

        var corTexto = Enabled ? tema.Texto : Desenho.Misturar(tema.Texto, fundoPai, 0.5f);
        var flags = TextFormatFlags.VerticalCenter | TextFormatFlags.SingleLine | TextFormatFlags.Left;
        if (!ShowKeyboardCues) flags |= TextFormatFlags.HidePrefix;
        var areaTexto = new Rectangle(LogicalToDeviceUnits(LarguraTrilho + Vao) + margem, 0, Width, Height);
        TextRenderer.DrawText(g, Text, Font, areaTexto, corTexto, flags);
    }
}

/// <summary>Ponto colorido da situação, com um halo suave. Decorativo: o texto ao lado diz a situação.</summary>
internal sealed class PontoSituacao : Control
{
    private Color _cor = Color.Gray;

    public PontoSituacao()
    {
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.UserPaint
                 | ControlStyles.SupportsTransparentBackColor, true);
        TabStop = false;
        AccessibleRole = AccessibleRole.Graphic;
        AccessibleName = "";
        Size = new Size(28, 28);
        Margin = new Padding(0, 4, 12, 0);
    }

    [DesignerSerializationVisibility(DesignerSerializationVisibility.Hidden)]
    public Color Cor
    {
        get => _cor;
        set { _cor = value; Invalidate(); }
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        g.Clear(Parent?.BackColor ?? Tema.Atual.Superficie);
        using (var halo = new SolidBrush(Color.FromArgb(48, _cor)))
            g.FillEllipse(halo, 0, 0, Width - 1, Height - 1);
        var miolo = Width * 0.46f;
        var x = (Width - miolo) / 2;
        using var cheio = new SolidBrush(_cor);
        g.FillEllipse(cheio, x, x, miolo, miolo);
    }
}

/// <summary>Barra de progresso arredondada nas cores do site (o leitor de tela recebe a porcentagem).</summary>
internal sealed class BarraProgresso : Control
{
    private int _valor;

    public BarraProgresso()
    {
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.UserPaint, true);
        TabStop = false;
        AccessibleRole = AccessibleRole.ProgressBar;
        AccessibleName = "Progresso do download";
        Size = new Size(260, 8);
        Margin = new Padding(0, 14, 12, 0);
    }

    /// <summary>De 0 a 100.</summary>
    [DesignerSerializationVisibility(DesignerSerializationVisibility.Hidden)]
    public int Valor
    {
        get => _valor;
        set
        {
            _valor = Math.Clamp(value, 0, 100);
            AccessibleDescription = $"{_valor}%";
            Invalidate();
        }
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        var tema = Tema.Atual;
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        g.Clear(Parent?.BackColor ?? tema.Superficie);
        var raio = Height / 2f;
        using (var trilho = Desenho.Arredondado(new RectangleF(0, 0, Width - 1, Height - 1), raio))
        using (var pincel = new SolidBrush(tema.Superficie3))
            g.FillPath(pincel, trilho);
        if (_valor <= 0) return;
        var largura = Math.Max(Height, (Width - 1) * _valor / 100f);
        using var cheio = Desenho.Arredondado(new RectangleF(0, 0, largura, Height - 1), raio);
        using var pincelCheio = new SolidBrush(tema.Primaria);
        g.FillPath(pincelCheio, cheio);
    }
}
