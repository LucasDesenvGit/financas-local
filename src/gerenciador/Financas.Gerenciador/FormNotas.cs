using System.Diagnostics;

namespace Financas.Gerenciador;

/// <summary>"O que mudou" na versão nova: as notas do Release, só para leitura, com o link para a página.</summary>
internal sealed class FormNotas : Form
{
    public FormNotas(VersaoDisponivel versao)
    {
        var tema = Tema.Atual;
        SuspendLayout();
        Text = $"O que mudou na versão {versao.Versao}";
        Icon = Icones.App;
        Font = new Font("Segoe UI", 9.75f);
        AutoScaleDimensions = new SizeF(96f, 96f);
        AutoScaleMode = AutoScaleMode.Dpi;
        Size = new Size(620, 480);
        MinimizeBox = false;
        StartPosition = FormStartPosition.CenterParent;
        Padding = new Padding(16);
        BackColor = tema.Fundo;
        ForeColor = tema.Texto;

        var texto = new TextBox
        {
            Multiline = true, ReadOnly = true, ScrollBars = ScrollBars.Vertical, Dock = DockStyle.Fill,
            BorderStyle = BorderStyle.FixedSingle, BackColor = tema.Superficie, ForeColor = tema.Texto,
            Text = string.IsNullOrWhiteSpace(versao.Notas) ? "Esta versão não tem notas." : versao.Notas.Replace("\r\n", "\n").Replace("\n", "\r\n"),
            AccessibleName = $"Notas da versão {versao.Versao}"
        };
        var fechar = new Botao("&Fechar", EstiloBotao.Primario) { DialogResult = DialogResult.OK, Margin = new Padding(8, 0, 0, 0) };
        var pagina = new Botao("Ver a página da versão no &GitHub", EstiloBotao.Discreto, IconeBotao.AbrirSite) { Margin = Padding.Empty };
        pagina.Click += (_, _) => Process.Start(new ProcessStartInfo(versao.Pagina.ToString()) { UseShellExecute = true });
        var rodape = new FlowLayoutPanel
        {
            FlowDirection = FlowDirection.RightToLeft, Dock = DockStyle.Bottom, AutoSize = true,
            AutoSizeMode = AutoSizeMode.GrowAndShrink, WrapContents = false, Padding = new Padding(0, 12, 0, 0)
        };
        rodape.Controls.AddRange([fechar, pagina]);
        AcceptButton = CancelButton = fechar;
        Controls.Add(texto);
        Controls.Add(rodape);
        ResumeLayout(false);
    }

    protected override void OnHandleCreated(EventArgs e)
    {
        base.OnHandleCreated(e);
        Tema.Atual.AplicarBarraDeTitulo(Handle);
    }
}
