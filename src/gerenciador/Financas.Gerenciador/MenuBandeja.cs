using System.ComponentModel;
using System.Drawing.Drawing2D;
using System.Runtime.InteropServices;

namespace Financas.Gerenciador;

/// <summary>
/// #289: o menu do botão direito no ícone da bandeja com o visual do site (cores do <see cref="Tema"/>, como a janela).
/// - No topo, a situação do site, sem ação: o ponto colorido sempre acompanhado do texto (WCAG 1.4.1; Nielsen 1).
/// - Ícones da fonte do próprio Windows, os mesmos dos botões da janela; "Abrir o site" em destaque.
/// - Itens com pelo menos 32 px de altura (alvo ≥ 24 px, WCAG 2.5.8); o item escolhido pelo teclado fica realçado igual
///   ao do mouse (WCAG 2.1.1 e 2.4.7); cantos arredondados no Windows 11.
/// - Alto contraste do Windows ligado: fica o desenho do sistema, que respeita as cores escolhidas pela pessoa.
/// </summary>
internal sealed class MenuBandeja : ContextMenuStrip
{
    private readonly ItemSituacao _situacao = new();

    public MenuBandeja(Action abrirSite, Action mostrar, Action sair)
    {
        Font = new Font("Segoe UI", 9.75f);
        ShowCheckMargin = false;
        // A coluna do ícone é a margem de imagem do próprio menu: assim ele já calcula a largura com o ícone.
        ShowImageMargin = true;
        ImageScalingSize = new Size(20, 20);
        Padding = new Padding(6);
        if (!SystemInformation.HighContrast) Renderer = new RenderizadorMenu();

        var abrir = new ItemMenu("Abrir o site", IconeBotao.AbrirSite, abrirSite) { Font = new Font("Segoe UI Semibold", 9.75f) };
        var janela = new ItemMenu("Mostrar o gerenciador", IconeMenu.Janela, mostrar) { ShortcutKeyDisplayString = "duplo clique", ShowShortcutKeys = true };
        Items.AddRange([_situacao, new ToolStripSeparator(), abrir, janela, new ToolStripSeparator(), new ItemMenu("Sair", IconeMenu.Sair, sair)]);
    }

    /// <summary>A situação mostrada no topo (a mesma do ícone e da janela).</summary>
    public void MostrarSituacao(Situacao situacao) => _situacao.Mostrar(situacao);

    protected override void OnHandleCreated(EventArgs e)
    {
        base.OnHandleCreated(e);
        // Windows 11: cantos arredondados (DWMWA_WINDOW_CORNER_PREFERENCE = 33, DWMWCP_ROUND = 2). No Windows 10 a
        // chamada só devolve erro e os cantos ficam retos.
        if (SystemInformation.HighContrast) return;
        var redondo = 2;
        try { _ = DwmSetWindowAttribute(Handle, 33, ref redondo, sizeof(int)); }
        catch (Exception ex) when (ex is DllNotFoundException or EntryPointNotFoundException) { }
    }

    [DllImport("dwmapi.dll")]
    private static extern int DwmSetWindowAttribute(IntPtr janela, int atributo, ref int valor, int tamanho);
}

/// <summary>Ícones do menu que os botões da janela não usam (Segoe Fluent Icons / Segoe MDL2 Assets).</summary>
internal static class IconeMenu
{
    public const char Janela = '';
    public const char Sair = '';
}

/// <summary>Item com ícone (só desenho: o nome acessível continua sendo o texto) e altura mínima de 32 px.</summary>
internal sealed class ItemMenu : ToolStripMenuItem
{
    public char Icone { get; }

    public ItemMenu(string texto, char icone, Action acao) : base(texto)
    {
        Icone = icone;
        Click += (_, _) => acao();
        Padding = new Padding(4, 0, 4, 0);
    }

    public override Size GetPreferredSize(Size proposto)
    {
        var tamanho = base.GetPreferredSize(proposto);
        var escala = Owner?.DeviceDpi / 96f ?? 1f;
        // Altura de pelo menos 32 px; a largura é a do próprio menu (texto, coluna do ícone e atalho).
        return new Size(tamanho.Width, Math.Max(tamanho.Height, (int)(32 * escala)));
    }
}

/// <summary>
/// A primeira linha: "Finanças Local · No ar", com o ponto na cor da situação. É um item desativado: o menu o alinha
/// com os outros (o ponto na coluna dos ícones), e o teclado e o clique passam por cima dele. O leitor de tela lê o
/// texto inteiro ("Finanças Local: No ar").
/// </summary>
internal sealed class ItemSituacao : ToolStripMenuItem
{
    [DesignerSerializationVisibility(DesignerSerializationVisibility.Hidden)]
    public EstadoSite Estado { get; private set; } = EstadoSite.Mudando;
    [DesignerSerializationVisibility(DesignerSerializationVisibility.Hidden)]
    public string Curto { get; private set; } = "Verificando…";

    public ItemSituacao() : base("Finanças Local: Verificando…") => Enabled = false;

    public void Mostrar(Situacao situacao)
    {
        Estado = situacao.Estado;
        Curto = situacao.Curto;
        Text = "Finanças Local: " + Curto;
        Invalidate();
    }

    public override Size GetPreferredSize(Size proposto)
    {
        var tamanho = base.GetPreferredSize(proposto);
        var escala = Owner?.DeviceDpi / 96f ?? 1f;
        return new Size(tamanho.Width, Math.Max(tamanho.Height, (int)(34 * escala)));
    }
}

/// <summary>
/// Desenho do menu nas cores do site: fundo Superficie, realce PrimariaSuave, linha Linha. O texto realçado usa
/// PrimariaHover, e não Primaria: no tema claro, Primaria sobre PrimariaSuave dá 4,03:1, abaixo dos 4,5:1 do WCAG 1.4.3;
/// PrimariaHover dá 4,78:1 (escuro: 6,2:1 ou mais).
/// </summary>
internal sealed class RenderizadorMenu : ToolStripRenderer
{
    private static readonly string? FonteIcones = new[] { "Segoe Fluent Icons", "Segoe MDL2 Assets" }
        .FirstOrDefault(nome => FontFamily.Families.Any(f => f.Name == nome));

    protected override void OnRenderToolStripBackground(ToolStripRenderEventArgs e)
    {
        using var fundo = new SolidBrush(Tema.Atual.Superficie);
        e.Graphics.FillRectangle(fundo, e.AffectedBounds);
    }

    protected override void OnRenderToolStripBorder(ToolStripRenderEventArgs e)
    {
        using var borda = new Pen(Tema.Atual.Linha);
        var r = e.AffectedBounds;
        e.Graphics.DrawRectangle(borda, r.X, r.Y, r.Width - 1, r.Height - 1);
    }

    protected override void OnRenderMenuItemBackground(ToolStripItemRenderEventArgs e)
    {
        if (!e.Item.Selected || !e.Item.Enabled) return;
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        var escala = (e.ToolStrip?.DeviceDpi ?? 96) / 96f;
        var r = new RectangleF(1, 1, e.Item.Width - 2, e.Item.Height - 2);
        using var caminho = Desenho.Arredondado(r, 6 * escala);
        using var realce = new SolidBrush(Tema.Atual.PrimariaSuave);
        g.FillPath(realce, caminho);
    }

    protected override void OnRenderItemText(ToolStripItemTextRenderEventArgs e)
    {
        var tema = Tema.Atual;
        var realcado = e.Item.Selected && e.Item.Enabled;
        // O atalho da direita ("duplo clique") é texto de apoio: menor e suave.
        if (e.Item is ToolStripMenuItem m && e.Text == m.ShortcutKeyDisplayString)
        {
            var base_ = e.TextFont ?? e.Item.Font;
            using var pequena = new Font(base_.FontFamily, base_.Size - 1.5f);
            var escalaAtalho = (e.ToolStrip?.DeviceDpi ?? 96) / 96f;
            var areaAtalho = new Rectangle(0, 0, e.Item.Width - (int)(12 * escalaAtalho), e.Item.Height);
            TextRenderer.DrawText(e.Graphics, e.Text, pequena, areaAtalho, realcado ? tema.PrimariaHover : tema.TextoSuave,
                TextFormatFlags.VerticalCenter | TextFormatFlags.Right | TextFormatFlags.SingleLine);
            return;
        }
        var escala = (e.ToolStrip?.DeviceDpi ?? 96) / 96f;
        if (e.Item is ItemSituacao situacao)
        {
            DesenharSituacao(e, situacao, escala);
            return;
        }
        var cor = realcado ? tema.PrimariaHover : tema.Texto;
        var texto = e.TextRectangle;
        if (e.Item is ItemMenu item && FonteIcones is not null)
        {
            using var fonteIcone = new Font(FonteIcones, 10.5f);
            // O ícone fica centrado na coluna à esquerda do texto (a margem de imagem do menu).
            var areaIcone = new Rectangle(0, 0, Math.Max(texto.X - (int)(4 * escala), (int)(24 * escala)), e.Item.Height);
            TextRenderer.DrawText(e.Graphics, item.Icone.ToString(), fonteIcone, areaIcone, realcado ? tema.PrimariaHover : tema.TextoSuave,
                TextFormatFlags.VerticalCenter | TextFormatFlags.HorizontalCenter | TextFormatFlags.NoPadding);
            texto = new Rectangle(texto.X, 0, texto.Width, e.Item.Height);
        }
        TextRenderer.DrawText(e.Graphics, e.Text, e.TextFont, texto, cor,
            TextFormatFlags.VerticalCenter | TextFormatFlags.Left | TextFormatFlags.SingleLine | TextFormatFlags.NoPrefix);
    }

    private static void DesenharSituacao(ToolStripItemTextRenderEventArgs e, ItemSituacao item, float escala)
    {
        var tema = Tema.Atual;
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        var fonte = e.TextFont ?? item.Font;
        var ponto = 9f * escala;
        var coluna = Math.Max(e.TextRectangle.X - 4 * escala, 24 * escala);
        using (var pincel = new SolidBrush(tema.DaSituacao(item.Estado)))
            g.FillEllipse(pincel, (coluna - ponto) / 2f, (item.Height - ponto) / 2f, ponto, ponto);
        const TextFormatFlags formato = TextFormatFlags.VerticalCenter | TextFormatFlags.SingleLine | TextFormatFlags.NoPadding | TextFormatFlags.NoPrefix;
        var area = new Rectangle(e.TextRectangle.X, 0, item.Width - e.TextRectangle.X, item.Height);
        using var negrito = new Font("Segoe UI Semibold", fonte.Size);
        TextRenderer.DrawText(g, "Finanças Local", negrito, area, tema.Texto, formato);
        var largura = TextRenderer.MeasureText(g, "Finanças Local", negrito, area.Size, formato).Width;
        area.X += largura;
        area.Width -= largura;
        TextRenderer.DrawText(g, " · " + item.Curto, fonte, area, tema.TextoSuave, formato);
    }

    /// <summary>A margem do ícone fica com o fundo do menu (o padrão pinta uma faixa).</summary>
    protected override void OnRenderImageMargin(ToolStripRenderEventArgs e) { }

    protected override void OnRenderSeparator(ToolStripSeparatorRenderEventArgs e)
    {
        using var linha = new Pen(Tema.Atual.Linha);
        var meio = e.Item.Height / 2;
        var margem = (int)(4 * (e.ToolStrip?.DeviceDpi ?? 96) / 96f);
        e.Graphics.DrawLine(linha, margem, meio, e.Item.Width - margem, meio);
    }
}
