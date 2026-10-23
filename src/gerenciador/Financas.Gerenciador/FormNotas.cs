using System.Diagnostics;

namespace Financas.Gerenciador;

/// <summary>
/// "O que mudou" na versão nova: as notas do Release, só para leitura, com o link para a página.
/// #294: o Markdown aparece formatado (títulos, listas, negrito e links; <see cref="RenderizadorNotas"/>), e primeiro só
/// o que mudou desde a versão instalada (<see cref="NotasDaVersao"/>); o resto (como instalar, versões anteriores e tudo
/// desde a última publicada) fica atrás de "Ver tudo" (Nielsen, Progressive Disclosure).
/// </summary>
internal sealed class FormNotas : Form
{
    private readonly RichTextBox _texto;
    private readonly NotasDaVersao _notas;
    private readonly Botao? _verTudo;
    private List<LinkNasNotas> _links = [];
    private bool _tudo;

    public FormNotas(VersaoDisponivel versao, VersaoDoSistema? instalada)
    {
        var tema = Tema.Atual;
        _notas = NotasDaVersao.Separar(versao.Notas, instalada);
        SuspendLayout();
        Text = $"O que mudou na versão {versao.Versao}";
        Icon = Icones.App;
        Font = new Font("Segoe UI", 9.75f);
        AutoScaleDimensions = new SizeF(96f, 96f);
        AutoScaleMode = AutoScaleMode.Dpi;
        Size = new Size(680, 540);
        MinimizeBox = false;
        StartPosition = FormStartPosition.CenterParent;
        Padding = new Padding(16);
        BackColor = tema.Fundo;
        ForeColor = tema.Texto;

        _texto = new RichTextBox
        {
            ReadOnly = true, ScrollBars = RichTextBoxScrollBars.Vertical, Dock = DockStyle.Fill, DetectUrls = false,
            BorderStyle = BorderStyle.None, Font = new Font("Segoe UI", 9.75f),
            AccessibleName = $"Notas da versão {versao.Versao}"
        };
        // Depois do ReadOnly: senão a caixa volta para o cinza do sistema.
        _texto.BackColor = tema.Superficie;
        _texto.ForeColor = tema.Texto;
        // Link: clique abre no navegador e o cursor vira a mãozinha em cima dele. Pelo teclado, o caminho é o botão
        // "Ver a página da versão no GitHub", que tem os mesmos links (WCAG 2.1.1).
        _texto.MouseUp += (_, e) => { if (e.Button == MouseButtons.Left && LinkEm(e.Location) is Uri destino) Abrir(destino); };
        _texto.MouseMove += (_, e) => _texto.Cursor = LinkEm(e.Location) is null ? Cursors.IBeam : Cursors.Hand;
        var moldura = new Panel { Dock = DockStyle.Fill, Padding = new Padding(12), BackColor = tema.Superficie };
        moldura.Controls.Add(_texto);

        var fechar = new Botao("&Fechar", EstiloBotao.Primario) { DialogResult = DialogResult.OK, Margin = new Padding(8, 0, 0, 0) };
        var pagina = new Botao("Ver a página da versão no &GitHub", EstiloBotao.Discreto, IconeBotao.AbrirSite) { Margin = Padding.Empty };
        pagina.Click += (_, _) => Abrir(versao.Pagina);
        var rodape = new FlowLayoutPanel
        {
            FlowDirection = FlowDirection.RightToLeft, Dock = DockStyle.Bottom, AutoSize = true,
            AutoSizeMode = AutoSizeMode.GrowAndShrink, WrapContents = false, Padding = new Padding(0, 12, 0, 0)
        };
        rodape.Controls.AddRange([fechar, pagina]);
        if (_notas.TemResto)
        {
            _verTudo = new Botao(TextoVerTudo(), EstiloBotao.Discreto, IconeBotao.Informacao)
            {
                Margin = new Padding(0, 0, 8, 0),
                AccessibleDescription = "Mostra também como instalar, as versões de desenvolvimento anteriores e tudo o que mudou desde a última versão publicada."
            };
            _verTudo.Click += (_, _) => { _tudo = !_tudo; _verTudo.Text = TextoVerTudo(); Mostrar(); };
            rodape.Controls.Add(_verTudo);
        }
        AcceptButton = CancelButton = fechar;
        Controls.Add(moldura);
        Controls.Add(rodape);
        ResumeLayout(false);
    }

    private string TextoVerTudo() => _tudo ? "Só o que é &novo" : "&Ver tudo";

    protected override void OnHandleCreated(EventArgs e)
    {
        base.OnHandleCreated(e);
        Tema.Atual.AplicarBarraDeTitulo(Handle);
        // #337: as notas da versão, com a barra de rolagem escura no tema escuro.
        Tema.Atual.AplicarRolagem(_texto);
        Mostrar();
    }

    private void Mostrar()
    {
        var markdown = string.IsNullOrWhiteSpace(_notas.Novidades) && !_notas.TemResto
            ? "Esta versão não tem notas."
            : _tudo ? _notas.Novidades + "\n\n" + _notas.Resto : _notas.Novidades;
        _texto.SuspendLayout();
        _texto.Clear();
        _links = RenderizadorNotas.Escrever(_texto, markdown, Tema.Atual, DeviceDpi / 96f);
        _texto.Select(0, 0);
        _texto.ScrollToCaret();
        _texto.ResumeLayout();
    }

    private Uri? LinkEm(Point ponto)
    {
        var indice = _texto.GetCharIndexFromPosition(ponto);
        // GetCharIndexFromPosition devolve o caractere mais perto, mesmo longe do texto: confere se o ponto está nele.
        var posicao = _texto.GetPositionFromCharIndex(indice);
        if (Math.Abs(ponto.Y - posicao.Y) > _texto.Font.Height * 1.6) return null;
        return _links.FirstOrDefault(l => indice >= l.Inicio && indice < l.Inicio + l.Tamanho)?.Destino;
    }

    private static void Abrir(Uri destino) => Process.Start(new ProcessStartInfo(destino.ToString()) { UseShellExecute = true });
}
