using System.Diagnostics;

namespace Financas.Gerenciador;

/// <summary>"O que mudou" na versão nova: as notas do Release, só para leitura, com o link para a página.</summary>
internal sealed class FormNotas : Form
{
    public FormNotas(VersaoDisponivel versao)
    {
        Text = $"O que mudou na versão {versao.Versao}";
        Icon = Icones.App;
        Font = new Font("Segoe UI", 9.75f);
        AutoScaleMode = AutoScaleMode.Dpi;
        Size = new Size(620, 480);
        MinimizeBox = false;
        StartPosition = FormStartPosition.CenterParent;
        Padding = new Padding(12);

        var texto = new TextBox
        {
            Multiline = true, ReadOnly = true, ScrollBars = ScrollBars.Vertical, Dock = DockStyle.Fill,
            Text = string.IsNullOrWhiteSpace(versao.Notas) ? "Esta versão não tem notas." : versao.Notas.Replace("\r\n", "\n").Replace("\n", "\r\n"),
            AccessibleName = $"Notas da versão {versao.Versao}"
        };
        var pagina = new LinkLabel { Text = "Ver a página da versão no GitHub", AutoSize = true, Dock = DockStyle.Bottom, Padding = new Padding(0, 8, 0, 0) };
        pagina.LinkClicked += (_, _) => Process.Start(new ProcessStartInfo(versao.Pagina.ToString()) { UseShellExecute = true });
        var fechar = new Button { Text = "&Fechar", AutoSize = true, Dock = DockStyle.Bottom, DialogResult = DialogResult.OK, MinimumSize = new Size(0, 32) };
        AcceptButton = CancelButton = fechar;
        Controls.Add(texto);
        Controls.Add(pagina);
        Controls.Add(fechar);
    }
}
