using Avalonia.Controls;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Input.Platform;
using Avalonia.Media;
using Avalonia.Platform.Storage;
using Avalonia.Threading;

namespace Financas.Gerenciador;

/// <summary>
/// Janela do gerenciador no Linux (ADR 0032). A mesma ordem e as mesmas frases da do Windows (ADR 0028); muda o que o
/// sistema muda: ligar e desligar pelo <c>systemctl --user</c>, sem senha; a versão nova vem pelos scripts, então a tela
/// mostra o comando para atualizar; e avisa quando o site só fica no ar com a sessão aberta.
/// </summary>
public partial class JanelaPrincipal : Window
{
    private static readonly TimeSpan Intervalo = TimeSpan.FromSeconds(5);
    /// <summary>Depois de ligar ou reiniciar, quanto esperar o site responder antes de dizer que ele não responde.</summary>
    private static readonly TimeSpan EsperaAoLigar = TimeSpan.FromSeconds(30);

    private readonly Instalacao _instalacao = Instalacao.Atual;
    private readonly DispatcherTimer _relogio = new() { Interval = Intervalo };
    private readonly TrayIcon? _bandeja;
    private bool _iniciado, _consultando, _ocupado, _sair;
    private Situacao? _situacao;

    public JanelaPrincipal()
    {
        InitializeComponent();

        BotaoAbrir.Click += async (_, _) => await AbrirPaginaAsync("");
        BotaoLigar.Click += async (_, _) => await ServicoAsync(_situacao?.Estado == EstadoSite.Desligado ? "ligar" : "desligar");
        BotaoReiniciar.Click += async (_, _) => await ServicoAsync("reiniciar");
        BotaoCopiarComando.Click += async (_, _) => await CopiarComandoAsync();
        BotaoMais.Click += (_, _) => AlternarMais();
        BotaoCopia.Click += async (_, _) => await SalvarCopiaAsync();
        BotaoErros.Click += async (_, _) => await AbrirPaginaAsync("configuracoes?aba=erros");
        BotaoRegistro.Click += async (_, _) => await new JanelaRegistro().ShowDialog(this);
        BotaoManual.Click += async (_, _) => await AbrirPaginaAsync("manual");
        BotaoSemLogin.Click += async (_, _) => await ManterNoArSemLoginAsync();
        IniciarComSistema.IsChecked = Preferencias.IniciarComSistema;
        IniciarComSistema.IsCheckedChanged += (_, _) => MudarIniciarComSistema();
        Local.Text = $"Dados em {_instalacao.PastaDados}. Endereço: {_instalacao.Site}";
        MostrarComoAtualizar();
        MostrarVersao();

        if (Bandeja.Disponivel) _bandeja = CriarBandeja();
        _relogio.Tick += async (_, _) => await AtualizarSituacaoAsync();
        Opened += async (_, _) => await IniciarAsync();
        Closing += AoFechar;
    }

    /// <summary>Uma vez só: ao abrir a janela ou, começando na bandeja, sem ela.</summary>
    public async Task IniciarAsync()
    {
        if (_iniciado) return;
        _iniciado = true;
        await AtualizarSituacaoAsync();
        _relogio.Start();
    }

    public void Mostrar()
    {
        Show();
        if (WindowState == WindowState.Minimized) WindowState = WindowState.Normal;
        // O KDE e o GNOME podem negar o foco a um pedido vindo de outro processo (proteção contra roubo de foco): aí a
        // janela aparece, mas pode ficar atrás da ativa.
        Activate();
    }

    /// <summary>Com "Iniciar com o sistema" e a bandeja, fechar a janela deixa o gerenciador lá, avisando se o site cair.</summary>
    private void AoFechar(object? remetente, WindowClosingEventArgs e)
    {
        if (!_sair && _bandeja is not null && Preferencias.IniciarComSistema && e.CloseReason == WindowCloseReason.WindowClosing)
        {
            e.Cancel = true;
            Hide();
            return;
        }
        _relogio.Stop();
        if (_bandeja is not null) _bandeja.IsVisible = false;
    }

    private TrayIcon CriarBandeja()
    {
        var abrir = new NativeMenuItem("Abrir o site");
        abrir.Click += async (_, _) => await AbrirPaginaAsync("");
        var mostrar = new NativeMenuItem("Mostrar o gerenciador");
        mostrar.Click += (_, _) => Mostrar();
        var sair = new NativeMenuItem("Sair");
        sair.Click += (_, _) =>
        {
            _sair = true;
            Close();
            // Começando na bandeja, a janela nunca abriu e o Close() não encerra: o app sai de qualquer jeito.
            (Avalonia.Application.Current?.ApplicationLifetime as IClassicDesktopStyleApplicationLifetime)?.Shutdown();
        };
        var bandeja = new TrayIcon
        {
            Icon = Bandeja.Icone(EstadoSite.Mudando),
            ToolTipText = "Finanças Local",
            Menu = new NativeMenu { Items = { abrir, mostrar, new NativeMenuItemSeparator(), sair } }
        };
        bandeja.Clicked += (_, _) => Mostrar();
        TrayIcon.SetIcons(Avalonia.Application.Current!, [bandeja]);
        return bandeja;
    }

    /* ------------------------------------------------------------ situação */

    private async Task AtualizarSituacaoAsync()
    {
        // Uma consulta por vez: com o site demorando, o relógio não empilha pedidos.
        if (_ocupado || _consultando) return;
        _consultando = true;
        var anterior = _situacao;
        Situacao nova;
        try
        {
            nova = await ConsultarAsync();
        }
        finally
        {
            _consultando = false;
        }
        Exibir(nova);
        // Escondido (na bandeja): avisa quando o site cai.
        if (!IsVisible && anterior?.Estado == EstadoSite.NoAr && nova.Estado is EstadoSite.Desligado or EstadoSite.SemResposta)
            Bandeja.Avisar(nova.Titulo);
    }

    private async Task<Situacao> ConsultarAsync()
    {
        try
        {
            // O systemctl é síncrono: fora da thread da interface.
            return await Task.Run(() => VerificadorSituacao.ConsultarAsync(_instalacao, CancellationToken.None));
        }
        catch (Exception erro)
        {
            return new Situacao(EstadoSite.SemResposta, "Não deu para ver a situação do site.", erro.Message);
        }
    }

    private void Exibir(Situacao situacao)
    {
        var versaoMudou = situacao.VersaoSite is not null && situacao.VersaoSite != _situacao?.VersaoSite;
        _situacao = situacao;
        if (versaoMudou) MostrarVersao();
        Titulo.Text = situacao.Titulo;
        Detalhe.Text = situacao.Detalhe;
        Ponto.Fill = Recurso(situacao.Estado switch
        {
            EstadoSite.NoAr => "Positivo",
            EstadoSite.Desligado => "Negativo",
            EstadoSite.SemResposta or EstadoSite.Mudando => "Alerta",
            _ => "TextoSuave"
        });

        var instalado = situacao.Estado != EstadoSite.NaoInstalado;
        var desligado = situacao.Estado == EstadoSite.Desligado;
        BotaoAbrir.IsEnabled = situacao.Estado is EstadoSite.NoAr or EstadoSite.SemResposta;
        // O botão em destaque é a próxima ação provável: abrir o site no ar, ligar o desligado.
        BotaoAbrir.Classes.Set("primario", !desligado);
        BotaoLigar.Classes.Set("primario", desligado);
        BotaoLigar.Content = desligado ? "_Ligar" : "_Desligar";
        BotaoLigar.IsEnabled = instalado && situacao.Estado != EstadoSite.Mudando && !_ocupado;
        BotaoReiniciar.IsEnabled = instalado && situacao.Estado is EstadoSite.NoAr or EstadoSite.SemResposta && !_ocupado;
        if (_bandeja is not null)
        {
            _bandeja.Icon = Bandeja.Icone(situacao.Estado);
            _bandeja.ToolTipText = "Finanças Local: " + situacao.Titulo;
        }
    }

    private async Task ServicoAsync(string acao)
    {
        _ocupado = true;
        Exibir(new Situacao(EstadoSite.Mudando, acao switch
        {
            "ligar" => "Ligando o site…",
            "desligar" => "Desligando o site…",
            _ => "Reiniciando o site…"
        }, "Aguarde alguns segundos."));
        var saida = await ServicoLinux.ExecutarAsync(acao);
        Situacao nova;
        if (saida.Ok && acao != "desligar")
        {
            // O serviço sobe antes do site responder: espera responder, para não dizer "não responde" à toa (Nielsen 1).
            var limite = DateTime.UtcNow + EsperaAoLigar;
            while ((nova = await ConsultarAsync()).Estado is EstadoSite.SemResposta or EstadoSite.Mudando
                   && DateTime.UtcNow < limite)
                await Task.Delay(1000);
        }
        else
        {
            nova = await ConsultarAsync();
        }
        _ocupado = false;
        Exibir(nova);
        if (!saida.Ok)
            Detalhe.Text = $"Não deu certo: {saida.Mensagem} Em “Mais opções”, “Ver o registro do serviço” mostra o motivo.";
    }

    /* ------------------------------------------------------------ versão */

    private void MostrarVersao()
    {
        var gerenciador = _instalacao.VersaoTexto;
        if (_situacao?.VersaoSite is not string site)
        {
            TextoVersao.Text = $"Gerenciador na versão {gerenciador}. A do site aparece quando ele estiver no ar.";
            return;
        }
        TextoVersao.Text = site == gerenciador
            ? $"Site e gerenciador na versão {site}."
            : $"Site na versão {site} e gerenciador na {gerenciador}. Para deixar os dois iguais, rode o comando abaixo.";
    }

    /// <summary>
    /// Instalado pelos scripts: a versão nova vem do código. Com a pasta do projeto (dois níveis acima de publicado/api),
    /// mostra o comando pronto para copiar (Nielsen 6: reconhecer em vez de lembrar).
    /// </summary>
    private void MostrarComoAtualizar()
    {
        var raiz = Path.GetFullPath(Path.Combine(_instalacao.PastaPrograma, "..", ".."));
        if (!File.Exists(Path.Combine(raiz, "scripts", "3-publicar-atualizar.sh")))
        {
            TextoComoAtualizar.Text = "Instalado pelos scripts: para atualizar, rode git pull, scripts/3-publicar-atualizar.sh "
                                      + "e scripts/instalar-gerenciador.sh na pasta do projeto.";
            CaixaComando.IsVisible = BotaoCopiarComando.IsVisible = false;
            return;
        }
        TextoComoAtualizar.Text = "Instalado pelos scripts: a versão nova vem do código. Para atualizar o site e o gerenciador, "
                                  + "rode no terminal (o site fica fora do ar por alguns segundos e o banco ganha uma cópia antes):";
        Comando.Text = $"cd {AspasShell(raiz)} && git pull && scripts/3-publicar-atualizar.sh && scripts/instalar-gerenciador.sh";
    }

    private async Task CopiarComandoAsync()
    {
        if (Clipboard is null || Comando.Text is null) return;
        await Clipboard.SetTextAsync(Comando.Text);
        ComandoCopiado.Text = "Copiado. Cole no terminal.";
    }

    private static string AspasShell(string valor) => "'" + valor.Replace("'", "'\\''") + "'";

    /* ------------------------------------------------------------ mais opções */

    private void AlternarMais()
    {
        PainelMais.IsVisible = !PainelMais.IsVisible;
        BotaoMais.Content = PainelMais.IsVisible ? "_Menos opções ▴" : "_Mais opções ▾";
        Avalonia.Automation.AutomationProperties.SetName(BotaoMais, PainelMais.IsVisible
            ? "Menos opções (esconder)"
            : "Mais opções: cópia do banco, erros, registro, manual e preferências");
        if (PainelMais.IsVisible) MostrarSemLogin();
    }

    private void MudarIniciarComSistema()
    {
        try
        {
            Preferencias.IniciarComSistema = IniciarComSistema.IsChecked == true;
        }
        catch (Exception e) when (e is IOException or UnauthorizedAccessException)
        {
            IniciarComSistema.IsChecked = Preferencias.IniciarComSistema;
        }
    }

    private void MostrarSemLogin()
    {
        var sobe = ServicoLinux.SobeSemLogin();
        BlocoSemLogin.IsVisible = sobe is not null;
        BotaoSemLogin.IsVisible = sobe == false;
        TextoSemLogin.Text = sobe == true
            ? "O site sobe junto com o computador, mesmo antes de você entrar na sua conta."
            : "Hoje o site só fica no ar enquanto você está com a sessão aberta: ao sair da conta, ele desliga.";
    }

    private async Task ManterNoArSemLoginAsync()
    {
        BotaoSemLogin.IsEnabled = false;
        var saida = await ServicoLinux.ManterNoArSemLoginAsync();
        BotaoSemLogin.IsEnabled = true;
        MostrarSemLogin();
        if (!saida.Ok) TextoSemLogin.Text = $"Não deu certo: {saida.Mensagem} No terminal: loginctl enable-linger";
    }

    private async Task SalvarCopiaAsync()
    {
        var arquivo = await StorageProvider.SaveFilePickerAsync(new FilePickerSaveOptions
        {
            Title = "Salvar uma cópia do banco",
            SuggestedFileName = $"FinancasLocal-{DateTime.Now:yyyy-MM-dd}.db",
            DefaultExtension = "db",
            FileTypeChoices = [new FilePickerFileType("Banco do Finanças Local") { Patterns = ["*.db"] }],
            ShowOverwritePrompt = true,
            SuggestedStartLocation = await StorageProvider.TryGetWellKnownFolderAsync(WellKnownFolder.Documents)
        });
        if (arquivo is null) return;
        string texto;
        if (arquivo.TryGetLocalPath() is not string destino)
        {
            texto = "Escolha uma pasta deste computador.";
        }
        else
        {
            try
            {
                await Task.Run(() => CopiaDoBanco.Salvar(_instalacao.CaminhoBanco, destino));
                texto = $"Cópia salva em {destino}. Ela tem todos os seus dados financeiros: guarde em lugar seguro.";
            }
            catch (FileNotFoundException)
            {
                texto = $"O banco não foi encontrado em {_instalacao.CaminhoBanco}.";
            }
            catch (Exception e) when (e is Microsoft.Data.Sqlite.SqliteException or IOException or UnauthorizedAccessException)
            {
                texto = "Não deu para copiar o banco. Confira se a pasta escolhida aceita gravação.";
            }
        }
        ResultadoCopia.Text = texto;
        ResultadoCopia.IsVisible = true;
    }

    private async Task AbrirPaginaAsync(string caminho) => await Launcher.LaunchUriAsync(_instalacao.Pagina(caminho));

    private IBrush? Recurso(string chave)
        => this.TryFindResource(chave, ActualThemeVariant, out var valor) ? valor as IBrush : null;
}
