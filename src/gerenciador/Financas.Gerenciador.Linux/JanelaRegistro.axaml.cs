using Avalonia.Controls;

namespace Financas.Gerenciador;

public partial class JanelaRegistro : Window
{
    private const int Linhas = 200;

    public JanelaRegistro()
    {
        InitializeComponent();
        BotaoAtualizar.Click += async (_, _) => await CarregarAsync();
        BotaoFechar.Click += (_, _) => Close();
        Opened += async (_, _) => await CarregarAsync();
    }

    private async Task CarregarAsync()
    {
        Texto.Text = "Carregando…";
        var saida = await ServicoLinux.RegistroAsync(Linhas);
        Texto.Text = saida.Ok
            ? (string.IsNullOrWhiteSpace(saida.Texto) ? "O registro está vazio." : saida.Texto)
            : "Não deu para ler o registro (journalctl). " + saida.Mensagem;
        Texto.CaretIndex = Texto.Text?.Length ?? 0;
    }
}
