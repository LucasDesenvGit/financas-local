using Avalonia;
using Avalonia.Controls;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Markup.Xaml;
using Avalonia.Threading;

namespace Financas.Gerenciador;

public partial class App : Application
{
    /// <summary>O socket da instância única (Program), que traz os pedidos de "mostrar a janela".</summary>
    internal static InstanciaUnica? Instancia { get; set; }

    public override void Initialize() => AvaloniaXamlLoader.Load(this);

    public override void OnFrameworkInitializationCompleted()
    {
        if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
        {
            var janela = new JanelaPrincipal();
            janela.Closed += (_, _) => desktop.Shutdown();
            if (Instancia is not null)
                Instancia.PedidoParaMostrar += () => Dispatcher.UIThread.Post(janela.Mostrar);

            // Iniciando com o sistema: só o ícone da bandeja; sem bandeja (GNOME sem extensão), a janela minimizada.
            if (desktop.Args?.Contains("--bandeja") == true && Bandeja.Disponivel)
            {
                _ = janela.IniciarAsync();
            }
            else
            {
                if (desktop.Args?.Contains("--bandeja") == true) janela.WindowState = WindowState.Minimized;
                desktop.MainWindow = janela;
                janela.Show();
            }
        }
        base.OnFrameworkInitializationCompleted();
    }
}
