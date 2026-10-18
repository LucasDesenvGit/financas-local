using System.Diagnostics;

namespace Financas.Gerenciador;

/// <summary>
/// Janela do gerenciador (ADR 0028). Primeiro a resposta à pergunta "o site está funcionando?" e as três ações do
/// dia a dia; a versão nova logo abaixo; o resto atrás de "Mais opções" (Progressive Disclosure). Tudo em botões e
/// caixas de marcar nativos, com teclas de atalho (&amp;) e nomes acessíveis (WCAG 2.1.1, 4.1.2).
/// </summary>
internal sealed class FormPrincipal : Form
{
    private static readonly TimeSpan Intervalo = TimeSpan.FromSeconds(5);

    private readonly Instalacao _instalacao = Instalacao.Atual;
    private readonly RotuloAnunciado _titulo = new() { AutoSize = true, Font = new Font("Segoe UI", 13f, FontStyle.Bold) };
    private readonly Label _detalhe = new() { AutoSize = true, MaximumSize = new Size(430, 0) };
    private readonly Button _abrir = Botao("&Abrir o site");
    private readonly Button _ligar = Botao("&Ligar");
    private readonly Button _reiniciar = Botao("&Reiniciar");
    private readonly RotuloAnunciado _versao = new() { AutoSize = true, MaximumSize = new Size(430, 0) };
    private readonly Button _atualizar = Botao("A&tualizar");
    private readonly LinkLabel _notas = new() { AutoSize = true, Text = "O que mudou", Visible = false };
    private readonly Button _procurar = Botao("&Procurar versão nova");
    private readonly ProgressBar _progresso = new() { Width = 300, Visible = false, AccessibleName = "Progresso do download" };
    private readonly Button _cancelar = Botao("&Cancelar download");
    private readonly Button _mais = Botao("&Mais opções ▾");
    private readonly FlowLayoutPanel _painelMais = new()
    {
        FlowDirection = FlowDirection.TopDown, AutoSize = true, Visible = false, WrapContents = false
    };
    private readonly CheckBox _iniciarComWindows = new() { AutoSize = true, Text = "&Iniciar com o Windows (fica na área de notificação)" };
    private readonly CheckBox _procurarAoAbrir = new() { AutoSize = true, Text = "Procurar &versão nova ao abrir" };
    private readonly NotifyIcon _bandeja = new() { Visible = true };
    private readonly System.Windows.Forms.Timer _relogio = new() { Interval = (int)Intervalo.TotalMilliseconds };

    private readonly bool _comecarNaBandeja;
    private bool _janelaJaMostrada;
    private bool _iniciado;
    private bool _consultando;
    private RegisteredWaitHandle? _esperaPedido;
    private bool _sair;
    private bool _ocupado;
    private Situacao? _situacao;
    private VersaoDisponivel? _disponivel;
    private string? _resultadoProcura;
    private CancellationTokenSource? _download;

    public FormPrincipal(bool comecarNaBandeja, EventWaitHandle pedidoParaMostrar)
    {
        _comecarNaBandeja = comecarNaBandeja;
        Text = "Gerenciador do Finanças Local";
        Icon = Icones.App;
        FormBorderStyle = FormBorderStyle.FixedSingle;
        MaximizeBox = false;
        AutoScaleMode = AutoScaleMode.Dpi;
        AutoSize = true;
        AutoSizeMode = AutoSizeMode.GrowAndShrink;
        Font = new Font("Segoe UI", 9.75f);
        StartPosition = FormStartPosition.CenterScreen;
        Padding = new Padding(16);

        _titulo.AccessibleRole = AccessibleRole.StaticText;
        _cancelar.Visible = false;
        _atualizar.Visible = false;

        var acoes = Linha(_abrir, _ligar, _reiniciar);
        var linhaVersao = Linha(_atualizar, _procurar, _notas);
        var linhaDownload = Linha(_progresso, _cancelar);

        var salvarCopia = Botao("Salvar uma &cópia do banco…");
        var erros = Botao("Ver os &erros do site");
        var manual = Botao("Abrir o ma&nual");
        _painelMais.Controls.AddRange([salvarCopia, Linha(erros, manual), _iniciarComWindows, _procurarAoAbrir,
            new Label { AutoSize = true, ForeColor = SystemColors.GrayText, MaximumSize = new Size(430, 0),
                Text = $"Dados em {_instalacao.PastaDados}. Endereço: {_instalacao.Site}" }]);

        var coluna = new FlowLayoutPanel
        {
            FlowDirection = FlowDirection.TopDown, AutoSize = true, WrapContents = false, Dock = DockStyle.Fill
        };
        coluna.Controls.AddRange([_titulo, _detalhe, Espaco(), acoes, Espaco(), Separador(), _versao, linhaVersao,
            linhaDownload, Espaco(), _mais, _painelMais]);
        Controls.Add(coluna);

        _abrir.Click += (_, _) => AbrirPagina("");
        _ligar.Click += async (_, _) => await ServicoAsync(_situacao?.Estado == EstadoSite.Desligado ? "ligar" : "desligar");
        _reiniciar.Click += async (_, _) => await ServicoAsync("reiniciar");
        _procurar.Click += async (_, _) => await ProcurarAsync(silencioso: false);
        _atualizar.Click += async (_, _) => await AtualizarAsync();
        _cancelar.Click += (_, _) => _download?.Cancel();
        _notas.LinkClicked += (_, _) => { if (_disponivel is not null) new FormNotas(_disponivel).ShowDialog(this); };
        _mais.Click += (_, _) => AlternarMais();
        salvarCopia.Click += async (_, _) => await SalvarCopiaAsync();
        erros.Click += (_, _) => AbrirPagina("configuracoes?aba=erros");
        manual.Click += (_, _) => AbrirPagina("manual");
        _iniciarComWindows.Checked = Preferencias.IniciarComWindows;
        _iniciarComWindows.CheckedChanged += (_, _) => Preferencias.IniciarComWindows = _iniciarComWindows.Checked;
        // Instalação pelos scripts: a versão nova vem pelos scripts, não pelo .msi do GitHub (que trocaria a instalação).
        _procurar.Visible = _procurarAoAbrir.Visible = _instalacao.PeloMsi;
        _procurarAoAbrir.Checked = Preferencias.ProcurarAoAbrir;
        _procurarAoAbrir.CheckedChanged += (_, _) => Preferencias.ProcurarAoAbrir = _procurarAoAbrir.Checked;

        _bandeja.Icon = Icones.Bandeja(EstadoSite.Mudando);
        _bandeja.Text = "Finanças Local";
        _bandeja.DoubleClick += (_, _) => Mostrar();
        var menu = new ContextMenuStrip();
        menu.Items.Add("Abrir o site", null, (_, _) => AbrirPagina(""));
        menu.Items.Add("Mostrar o gerenciador", null, (_, _) => Mostrar());
        menu.Items.Add(new ToolStripSeparator());
        menu.Items.Add("Sair", null, (_, _) => { _sair = true; Close(); });
        _bandeja.ContextMenuStrip = menu;

        _relogio.Tick += async (_, _) => await AtualizarSituacaoAsync();
        MostrarVersao();

        // Outra cópia do gerenciador aberta pede para mostrar esta (uma só por usuário).
        _esperaPedido = ThreadPool.RegisterWaitForSingleObject(pedidoParaMostrar, (_, _) => PedirParaMostrar(), null, -1, false);
    }

    protected override async void OnLoad(EventArgs e)
    {
        base.OnLoad(e);
        await IniciarAsync();
    }

    /// <summary>Uma vez só: pela janela (Load) ou, começando na área de notificação, sem ela.</summary>
    private async Task IniciarAsync()
    {
        if (_iniciado) return;
        _iniciado = true;
        await AtualizarSituacaoAsync();
        _relogio.Start();
        if (_instalacao.PeloMsi && Preferencias.ProcurarAoAbrir) await ProcurarAsync(silencioso: true);
    }

    /// <summary>Começando com o Windows, abre só o ícone da área de notificação.</summary>
    protected override void SetVisibleCore(bool value)
    {
        if (!_janelaJaMostrada && _comecarNaBandeja)
        {
            _janelaJaMostrada = true;
            if (!IsHandleCreated) CreateHandle();
            _ = IniciarAsync();
            value = false;
        }
        _janelaJaMostrada = true;
        base.SetVisibleCore(value);
    }

    /// <summary>Com "Iniciar com o Windows", fechar a janela deixa o gerenciador na área de notificação.</summary>
    protected override void OnFormClosing(FormClosingEventArgs e)
    {
        if (!_sair && e.CloseReason == CloseReason.UserClosing && Preferencias.IniciarComWindows)
        {
            e.Cancel = true;
            Hide();
            return;
        }
        _download?.Cancel();
        _relogio.Stop();
        _esperaPedido?.Unregister(null);
        _bandeja.Visible = false;
        base.OnFormClosing(e);
    }

    /// <summary>Chega por outra thread; a janela pode estar fechando, e aí o pedido é ignorado.</summary>
    private void PedirParaMostrar()
    {
        try
        {
            if (IsHandleCreated && !IsDisposed) BeginInvoke(new Action(Mostrar));
        }
        catch (Exception e) when (e is InvalidOperationException or ObjectDisposedException) { }
    }

    private void Mostrar()
    {
        Show();
        if (WindowState == FormWindowState.Minimized) WindowState = FormWindowState.Normal;
        Activate();
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
            nova = await VerificadorSituacao.ConsultarAsync(_instalacao, CancellationToken.None);
        }
        catch (Exception erro)
        {
            nova = new Situacao(EstadoSite.SemResposta, "Não deu para ver a situação do site.", erro.Message);
        }
        finally
        {
            _consultando = false;
        }
        Exibir(nova);
        // Escondido na área de notificação: avisa quando o site cai.
        if (!Visible && anterior?.Estado == EstadoSite.NoAr && nova.Estado is EstadoSite.Desligado or EstadoSite.SemResposta)
            _bandeja.ShowBalloonTip(5000, "Finanças Local", nova.Titulo, ToolTipIcon.Warning);
    }

    private void Exibir(Situacao situacao)
    {
        var mudou = _situacao?.Titulo != situacao.Titulo;
        var versaoMudou = situacao.VersaoSite is not null && situacao.VersaoSite != _situacao?.VersaoSite;
        _situacao = situacao;
        if (versaoMudou) MostrarVersao();
        _titulo.Text = situacao.Titulo;
        _detalhe.Text = situacao.Detalhe;
        if (mudou) _titulo.Anunciar();

        var instalado = situacao.Estado != EstadoSite.NaoInstalado;
        _abrir.Enabled = situacao.Estado is EstadoSite.NoAr or EstadoSite.SemResposta;
        _ligar.Text = situacao.Estado == EstadoSite.Desligado ? "&Ligar" : "&Desligar";
        _ligar.Enabled = instalado && situacao.Estado != EstadoSite.Mudando && !_ocupado;
        _reiniciar.Enabled = instalado && situacao.Estado is EstadoSite.NoAr or EstadoSite.SemResposta && !_ocupado;
        _bandeja.Icon = Icones.Bandeja(situacao.Estado);
        _bandeja.Text = Limitar("Finanças Local: " + situacao.Titulo, 127);
    }

    private async Task ServicoAsync(string acao)
    {
        _ocupado = true;
        Exibir(new Situacao(EstadoSite.Mudando, acao switch
        {
            "ligar" => "Ligando o site…",
            "desligar" => "Desligando o site…",
            _ => "Reiniciando o site…"
        }, "O Windows vai pedir permissão de administrador."));
        var resultado = await AcoesElevadas.ExecutarAsync($"--servico {acao}");
        _ocupado = false;
        await AtualizarSituacaoAsync();
        if (resultado == AcoesElevadas.Resultado.Cancelado) _detalhe.Text = "Cancelado: nada mudou.";
        else if (resultado == AcoesElevadas.Resultado.Falhou)
            _detalhe.Text = "Não deu certo. Veja os eventos do Windows (Visualizador de Eventos › Aplicativo, origem FinancasLocal).";
    }

    /* ------------------------------------------------------------ versão nova */

    /// <summary>
    /// Com o site no ar, a versão dele já aparece junto da situação ("Versão 2.1.0 · endereço"): aqui fica só o resultado
    /// da procura por versão nova. Fora do ar, mostra a versão instalada (o .msi instala site e gerenciador juntos).
    /// </summary>
    private void MostrarVersao()
    {
        if (!_instalacao.PeloMsi)
        {
            _versao.Text = "Instalado pelos scripts: para atualizar o site, rode de novo o instalar-windows.cmd "
                           + "(ou o 3-publicar-atualizar.sh) na pasta do projeto.";
            return;
        }
        var instalada = _situacao?.VersaoSite is null ? $"Versão instalada: {_instalacao.VersaoTexto}." : null;
        var procura = _resultadoProcura ?? (instalada is null ? "Versão nova: clique em “Procurar versão nova”." : null);
        _versao.Text = string.Join(" ", new[] { instalada, procura }.Where(t => !string.IsNullOrEmpty(t)));
    }

    /// <summary>A versão que está rodando (a do site, quando ele responde) é a que conta para "tem versão nova".</summary>
    private Version VersaoAtual()
        => _situacao?.VersaoSite is string site && Version.TryParse(site.Split('-')[0], out var v) ? v : _instalacao.Versao;

    private async Task ProcurarAsync(bool silencioso)
    {
        _procurar.Enabled = false;
        _resultadoProcura = "Procurando versão nova…";
        MostrarVersao();
        try
        {
            using var limite = new CancellationTokenSource(TimeSpan.FromSeconds(20));
            _disponivel = await Atualizacoes.ProcurarAsync(limite.Token);
            var nova = _disponivel is not null && _disponivel.Versao > VersaoAtual();
            _resultadoProcura = nova ? $"Tem versão nova: {_disponivel!.Versao}." : "Você está na versão mais nova.";
            MostrarVersao();
            _atualizar.Text = nova ? $"A&tualizar para {_disponivel!.Versao}" : "A&tualizar";
            _atualizar.Visible = nova;
            _notas.Visible = nova;
            if (nova)
            {
                _versao.Anunciar();
                if (!Visible) _bandeja.ShowBalloonTip(5000, "Finanças Local", $"Versão nova: {_disponivel!.Versao}.", ToolTipIcon.Info);
            }
        }
        catch (Exception e) when (e is HttpRequestException or TaskCanceledException or System.Text.Json.JsonException
                                      or KeyNotFoundException or InvalidOperationException)
        {
            _resultadoProcura = "Não deu para procurar versão nova agora (sem internet?). O site continua funcionando.";
            MostrarVersao();
            if (!silencioso) _versao.Anunciar();
        }
        finally
        {
            _procurar.Enabled = true;
        }
    }

    private async Task AtualizarAsync()
    {
        if (_disponivel is not { } versao) return;
        var confirmar = MessageBox.Show(this,
            $"Atualizar o Finanças Local para a versão {versao.Versao}?\n\n" +
            "O instalador abre em seguida e o site fica fora do ar por alguns segundos. Antes de mudar o banco, " +
            "a versão nova guarda uma cópia dele em dados\\backups. Seus dados continuam onde estão.",
            "Atualizar o Finanças Local", MessageBoxButtons.OKCancel, MessageBoxIcon.Question, MessageBoxDefaultButton.Button1);
        if (confirmar != DialogResult.OK) return;

        _download = new CancellationTokenSource();
        _atualizar.Enabled = false;
        _progresso.Value = 0;
        _progresso.Visible = _cancelar.Visible = true;
        _versao.Text = $"Baixando a versão {versao.Versao}…";
        _versao.Anunciar();
        try
        {
            var progresso = new Progress<double>(p => _progresso.Value = (int)Math.Clamp(p * 100, 0, 100));
            var msi = await Atualizacoes.BaixarAsync(versao, progresso, _download.Token);
            _versao.Text = "Abrindo o instalador…";
            Atualizacoes.Instalar(msi);
            _sair = true;
            Close();
        }
        catch (OperationCanceledException)
        {
            _versao.Text = "Download cancelado. Nada mudou.";
            _versao.Anunciar();
        }
        catch (Exception e) when (e is HttpRequestException or InvalidOperationException or IOException
                                      or System.ComponentModel.Win32Exception)
        {
            _versao.Text = e.Message;
            _versao.Anunciar();
        }
        finally
        {
            _atualizar.Enabled = true;
            _progresso.Visible = _cancelar.Visible = false;
            _download?.Dispose();
            _download = null;
        }
    }

    /* ------------------------------------------------------------ mais opções */

    private void AlternarMais()
    {
        _painelMais.Visible = !_painelMais.Visible;
        _mais.Text = _painelMais.Visible ? "&Menos opções ▴" : "&Mais opções ▾";
        _mais.AccessibleName = _painelMais.Visible ? "Menos opções (esconder)" : "Mais opções: cópia do banco, erros, manual e preferências";
    }

    private async Task SalvarCopiaAsync()
    {
        using var dialogo = new SaveFileDialog
        {
            Title = "Salvar uma cópia do banco",
            FileName = $"FinancasLocal-{DateTime.Now:yyyy-MM-dd}.db",
            Filter = "Banco do Finanças Local (*.db)|*.db",
            InitialDirectory = Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments),
            OverwritePrompt = true
        };
        if (dialogo.ShowDialog(this) != DialogResult.OK) return;
        var resultado = await AcoesElevadas.ExecutarAsync($"--backup {AcoesElevadas.Aspas(dialogo.FileName)}");
        var texto = resultado switch
        {
            AcoesElevadas.Resultado.Feito => $"Cópia salva em {dialogo.FileName}. Ela tem todos os seus dados financeiros: guarde em lugar seguro.",
            AcoesElevadas.Resultado.Cancelado => "Cancelado: nenhuma cópia foi feita.",
            _ => "Não deu para copiar o banco. Confira se a pasta escolhida aceita gravação."
        };
        MessageBox.Show(this, texto, "Cópia do banco", MessageBoxButtons.OK,
            resultado == AcoesElevadas.Resultado.Feito ? MessageBoxIcon.Information : MessageBoxIcon.Warning);
    }

    private void AbrirPagina(string caminho)
        => Process.Start(new ProcessStartInfo(_instalacao.Pagina(caminho).ToString()) { UseShellExecute = true });

    /* ------------------------------------------------------------ montagem */

    private static Button Botao(string texto)
        => new() { Text = texto, AutoSize = true, MinimumSize = new Size(0, 32), Padding = new Padding(8, 2, 8, 2) };

    private static FlowLayoutPanel Linha(params Control[] controles)
    {
        var linha = new FlowLayoutPanel { FlowDirection = FlowDirection.LeftToRight, AutoSize = true, WrapContents = true };
        linha.Controls.AddRange(controles);
        return linha;
    }

    private static Control Espaco() => new Panel { Height = 8, Width = 1 };

    private static Control Separador()
        => new Label { AutoSize = false, Height = 1, Width = 430, BorderStyle = BorderStyle.Fixed3D, Margin = new Padding(0, 4, 0, 8) };

    private static string Limitar(string texto, int maximo) => texto.Length <= maximo ? texto : texto[..(maximo - 1)] + "…";

    protected override void Dispose(bool disposing)
    {
        if (disposing)
        {
            _bandeja.Dispose();
            _relogio.Dispose();
            _download?.Dispose();
        }
        base.Dispose(disposing);
    }
}

/// <summary>Rótulo que avisa o leitor de tela quando o texto muda (a situação e a versão).</summary>
internal sealed class RotuloAnunciado : Label
{
    public void Anunciar()
    {
        AccessibleName = Text;
        AccessibilityNotifyClients(AccessibleEvents.NameChange, -1);
    }
}
