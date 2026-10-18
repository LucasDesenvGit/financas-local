using System.Diagnostics;

namespace Financas.Gerenciador;

/// <summary>
/// Janela do gerenciador (ADR 0028). Primeiro a resposta à pergunta "o site está funcionando?" e as três ações do
/// dia a dia; a versão nova logo abaixo; o resto atrás de "Mais opções" (Progressive Disclosure). Nas cores e no
/// desenho do site (cabeçalho com a marca, cartões, botões arredondados; Nielsen 4), claro ou escuro como o Windows.
/// Os controles continuam botões e caixas de marcar do Windows por baixo, com teclas de atalho (&amp;), foco visível e
/// nomes acessíveis (WCAG 2.1.1, 2.4.7, 4.1.2).
/// </summary>
internal sealed class FormPrincipal : Form
{
    private static readonly TimeSpan Intervalo = TimeSpan.FromSeconds(5);
    /// <summary>Largura dos cartões e do texto dentro deles (em 96 DPI; a janela escala com a tela).</summary>
    private const int LarguraCartao = 448, LarguraConteudo = LarguraCartao - 32;

    private readonly Tema _tema = Tema.Atual;
    private readonly Instalacao _instalacao = Instalacao.Atual;
    private readonly PontoSituacao _ponto = new();
    private readonly RotuloAnunciado _titulo = new()
    {
        AutoSize = true, Font = new Font("Segoe UI Semibold", 13f), Margin = new Padding(0, 0, 0, 2)
    };
    private readonly Label _detalhe = new() { AutoSize = true, MaximumSize = new Size(LarguraConteudo - 40, 0), Margin = Padding.Empty };
    private readonly Botao _abrir = new("&Abrir o site", EstiloBotao.Primario, IconeBotao.AbrirSite);
    private readonly Botao _ligar = new("&Ligar", icone: IconeBotao.Ligar);
    private readonly Botao _reiniciar = new("&Reiniciar", icone: IconeBotao.Reiniciar);
    private readonly RotuloAnunciado _versao = new() { AutoSize = true, MaximumSize = new Size(LarguraConteudo, 0), Margin = Padding.Empty };
    private readonly Botao _atualizar = new("A&tualizar", EstiloBotao.Primario, IconeBotao.Baixar);
    private readonly Botao _notas = new("O &que mudou", EstiloBotao.Discreto, IconeBotao.Informacao) { Visible = false };
    private readonly Botao _procurar = new("&Procurar versão nova", icone: IconeBotao.Procurar);
    private readonly Botao _baixarMsi = new("&Baixar o instalador .msi", EstiloBotao.Discreto, IconeBotao.Baixar);
    private readonly BarraProgresso _progresso = new() { Visible = false };
    private readonly Botao _cancelar = new("&Cancelar download", EstiloBotao.Discreto, IconeBotao.Cancelar);
    private readonly Botao _mais = new("&Mais opções ▾", EstiloBotao.Discreto);
    private readonly Cartao _painelMais = new(LarguraCartao) { Visible = false };
    private readonly Interruptor _iniciarComWindows = new("&Iniciar com o Windows (fica na área de notificação)");
    private readonly Botao _erros = new("Ver os &erros do site", icone: IconeBotao.Erros);
    private readonly Botao _manual = new("Abrir o ma&nual", icone: IconeBotao.Manual);
    private readonly ToolTip _dica = new() { InitialDelay = 300, AutoPopDelay = 8000 };
    private const string DicaSiteDesligado = "É preciso ligar o site para ver isto. Use o botão “Ligar” acima.";
    private readonly Interruptor _procurarAoAbrir = new("Procurar &versão nova ao abrir");
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
        SuspendLayout();
        Text = "Gerenciador do Finanças Local";
        Icon = Icones.App;
        FormBorderStyle = FormBorderStyle.FixedSingle;
        MaximizeBox = false;
        // Medidas pensadas em 96 DPI e escaladas para a tela (125%, 150%…).
        AutoScaleDimensions = new SizeF(96f, 96f);
        AutoScaleMode = AutoScaleMode.Dpi;
        AutoSize = true;
        AutoSizeMode = AutoSizeMode.GrowAndShrink;
        Font = new Font("Segoe UI", 9.75f);
        StartPosition = FormStartPosition.CenterScreen;
        BackColor = _tema.Fundo;
        ForeColor = _tema.Texto;

        _titulo.AccessibleRole = AccessibleRole.StaticText;
        _detalhe.ForeColor = _tema.TextoSuave;
        _cancelar.Visible = false;
        _atualizar.Visible = false;

        // Situação: o ponto colorido e o título respondem "está funcionando?"; as ações do dia a dia logo embaixo.
        var situacao = new Cartao(LarguraCartao);
        situacao.Conteudo.Controls.AddRange([Linha(_ponto, Coluna(_titulo, _detalhe)), Espaco(14), Linha(_abrir, _ligar, _reiniciar)]);

        var versao = new Cartao(LarguraCartao);
        versao.Conteudo.Controls.AddRange([Subtitulo("Versão"), _versao, Espaco(10), Linha(_atualizar, _procurar, _notas, _baixarMsi),
            Linha(_progresso, _cancelar)]);

        var salvarCopia = new Botao("Salvar uma &cópia do banco…", icone: IconeBotao.Salvar);
        var linhaSite = Linha(_erros, _manual);
        DicaQuandoBloqueado(linhaSite);
        _painelMais.Conteudo.Controls.AddRange([Linha(salvarCopia), linhaSite, Espaco(4),
            _iniciarComWindows, _procurarAoAbrir, Espaco(6),
            new Label { AutoSize = true, ForeColor = _tema.TextoSuave, MaximumSize = new Size(LarguraConteudo, 0),
                Margin = Padding.Empty, UseMnemonic = false, Text = $"Dados em {_instalacao.PastaDados}. Endereço: {_instalacao.Site}" }]);

        var corpo = Coluna(situacao, versao, _mais, _painelMais);
        corpo.Padding = new Padding(16, 16, 16, 8);
        var raiz = Coluna(new Cabecalho { Size = new Size(LarguraCartao + 32, 72) }, corpo);
        Controls.Add(raiz);

        _abrir.Click += (_, _) => AbrirPagina("");
        _ligar.Click += async (_, _) => await ServicoAsync(_situacao?.Estado == EstadoSite.Desligado ? "ligar" : "desligar");
        _reiniciar.Click += async (_, _) => await ServicoAsync("reiniciar");
        _procurar.Click += async (_, _) => await ProcurarAsync(silencioso: false);
        _atualizar.Click += async (_, _) => await AtualizarAsync();
        _cancelar.Click += (_, _) => _download?.Cancel();
        _notas.Click += (_, _) => { if (_disponivel is not null) new FormNotas(_disponivel).ShowDialog(this); };
        _mais.Click += (_, _) => AlternarMais();
        salvarCopia.Click += async (_, _) => await SalvarCopiaAsync();
        _erros.Click += (_, _) => AbrirPagina("configuracoes?aba=erros");
        _manual.Click += (_, _) => AbrirPagina("manual");
        _iniciarComWindows.Checked = Preferencias.IniciarComWindows;
        _iniciarComWindows.CheckedChanged += (_, _) => Preferencias.IniciarComWindows = _iniciarComWindows.Checked;
        // Instalação pelos scripts: o gerenciador não instala o .msi do GitHub sozinho (ele troca a instalação). Mostra
        // o caminho: rodar os scripts de novo ou baixar o .msi, que traz os dados e passa a se atualizar por aqui.
        _procurar.Visible = _procurarAoAbrir.Visible = _instalacao.PeloMsi;
        _baixarMsi.Visible = !_instalacao.PeloMsi;
        _baixarMsi.AccessibleDescription = "Abre a página da versão mais recente no GitHub";
        _baixarMsi.Click += (_, _) => Process.Start(new ProcessStartInfo(
            $"https://github.com/{Instalacao.Repositorio}/releases/latest") { UseShellExecute = true });
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
        ResumeLayout(false);
        PerformLayout();
    }

    protected override void OnHandleCreated(EventArgs e)
    {
        base.OnHandleCreated(e);
        _tema.AplicarBarraDeTitulo(Handle);
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

        _ponto.Cor = _tema.DaSituacao(situacao.Estado);

        var instalado = situacao.Estado != EstadoSite.NaoInstalado;
        var desligado = situacao.Estado == EstadoSite.Desligado;
        _abrir.Enabled = situacao.Estado is EstadoSite.NoAr or EstadoSite.SemResposta;
        // O botão em destaque é a próxima ação provável: abrir o site no ar, ligar o desligado.
        _abrir.Estilo = desligado ? EstiloBotao.Secundario : EstiloBotao.Primario;
        _ligar.Estilo = desligado ? EstiloBotao.Primario : EstiloBotao.Secundario;
        _ligar.Text = desligado ? "&Ligar" : "&Desligar";
        _ligar.Enabled = instalado && situacao.Estado != EstadoSite.Mudando && !_ocupado;
        _reiniciar.Enabled = instalado && situacao.Estado is EstadoSite.NoAr or EstadoSite.SemResposta && !_ocupado;
        // Manual e erros são páginas do site: com ele fora do ar, ficam bloqueados e a dica diz como liberar.
        _erros.Enabled = _manual.Enabled = situacao.Estado == EstadoSite.NoAr;
        _erros.AccessibleDescription = _manual.AccessibleDescription = _manual.Enabled ? null : DicaSiteDesligado;
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
                           + "(ou o 3-publicar-atualizar.sh) na pasta do projeto. Ou passe para o instalador: o .msi "
                           + "traz o banco e as chaves, e daí em diante a versão nova chega por aqui.";
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
        _progresso.Valor = 0;
        _progresso.Visible = _cancelar.Visible = true;
        _versao.Text = $"Baixando a versão {versao.Versao}…";
        _versao.Anunciar();
        try
        {
            var progresso = new Progress<double>(p => _progresso.Valor = (int)(p * 100));
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

    /// <summary>
    /// Botão desabilitado não recebe o mouse no WinForms (o Windows manda para o painel em volta), então a dica
    /// é mostrada pelo painel quando o ponteiro está sobre um botão bloqueado.
    /// </summary>
    private void DicaQuandoBloqueado(Control painel)
    {
        Control? atual = null;
        painel.MouseMove += (_, e) =>
        {
            var filho = painel.GetChildAtPoint(e.Location);
            var alvo = filho is { Enabled: false } ? filho : null;
            if (alvo == atual) return;
            atual = alvo;
            if (alvo is null) _dica.Hide(painel);
            else _dica.Show(DicaSiteDesligado, painel, alvo.Left, alvo.Bottom + 4, 8000);
        };
        painel.MouseLeave += (_, _) => { atual = null; _dica.Hide(painel); };
    }

    /* ------------------------------------------------------------ montagem */

    /// <summary>Lado a lado, quebrando a linha na largura do cartão.</summary>
    private static FlowLayoutPanel Linha(params Control[] controles)
    {
        var linha = new FlowLayoutPanel
        {
            FlowDirection = FlowDirection.LeftToRight, AutoSize = true, AutoSizeMode = AutoSizeMode.GrowAndShrink,
            WrapContents = true, Margin = Padding.Empty, MaximumSize = new Size(LarguraConteudo, 0)
        };
        linha.Controls.AddRange(controles);
        return linha;
    }

    private static FlowLayoutPanel Coluna(params Control[] controles)
    {
        var coluna = new FlowLayoutPanel
        {
            FlowDirection = FlowDirection.TopDown, AutoSize = true, AutoSizeMode = AutoSizeMode.GrowAndShrink,
            WrapContents = false, Margin = Padding.Empty
        };
        coluna.Controls.AddRange(controles);
        return coluna;
    }

    private Label Subtitulo(string texto) => new()
    {
        AutoSize = true, Text = texto, Font = new Font("Segoe UI Semibold", 9f), ForeColor = _tema.TextoSuave,
        Margin = new Padding(0, 0, 0, 6), UseMnemonic = false
    };

    private static Control Espaco(int altura) => new Panel { Height = altura, Width = 1, Margin = Padding.Empty };

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
