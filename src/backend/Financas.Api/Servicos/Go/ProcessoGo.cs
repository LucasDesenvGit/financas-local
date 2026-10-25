using System.ComponentModel;
using System.Diagnostics;
using System.Globalization;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace Financas.Api.Servicos.Go;

/// <summary>
/// Sobe o Go como processo filho do .NET e o mantém de pé (#231, ADR 0049). Continua um serviço só no systemd, no
/// Serviço do Windows e no .msi.
/// <list type="bullet">
/// <item>Só começa depois do <c>EsquemaBanco</c> e das rotinas de partida: os serviços em segundo plano iniciam no
/// <c>app.Run()</c>. O Go não aplica esquema.</item>
/// <item>A cada partida do Go, um segredo novo (32 bytes aleatórios) vai pela entrada padrão, que fica aberta: quando
/// ela fecha (o .NET parou ou caiu), o Go para sozinho. No Windows, o Job Object encerra o Go mesmo travado.</item>
/// <item>O Go escolhe a porta (só 127.0.0.1) e a escreve na saída padrão: <c>{"porta":N}</c>.</item>
/// <item>A saída de erro do Go (log em JSON, uma linha por registro) vira log do .NET na categoria
/// <c>Financas.Go</c>: avisos e erros vão para Configurações › Erros (ADR 0022).</item>
/// <item>Se o Go cair, as rotas dele respondem 503 e ele é iniciado de novo, esperando 1 s, 2 s, 4 s… até 1 min.</item>
/// </list>
/// </summary>
public sealed class ProcessoGo : BackgroundService
{
    /// <summary>Categoria dos registros vindos do Go; <c>Financas.Go.&lt;origem&gt;</c> quando o Go diz a origem.</summary>
    public const string CategoriaLog = "Financas.Go";

    /// <summary>Origens da tela de erros que o Go pode usar no atributo "origem" do log.</summary>
    private static readonly HashSet<string> OrigensConhecidas = new(StringComparer.Ordinal)
        { "API", "Open Finance", "Investimentos", "Câmbio", "Cripto", "Sistema" };

    private static readonly TimeSpan EsperaInicial = TimeSpan.FromSeconds(1);
    private static readonly TimeSpan EsperaMaxima = TimeSpan.FromMinutes(1);
    private static readonly TimeSpan PrazoParaAvisarAPorta = TimeSpan.FromSeconds(30);
    /// <summary>O Go espera as requisições em andamento por até 8 s; passou de 10 s, é encerrado à força.</summary>
    private static readonly TimeSpan PrazoParaParar = TimeSpan.FromSeconds(10);
    private const int LinhasGuardadas = 60;

    /// <summary>Onde o Go atende e o segredo desta partida dele.</summary>
    public sealed record Destino(Uri Base, string Segredo, int Pid);

    private readonly OpcoesGo _opcoes;
    private readonly ILogger<ProcessoGo> _log;
    private readonly ILoggerFactory _fabrica;
    private volatile Destino? _destino;

    public ProcessoGo(OpcoesGo opcoes, ILogger<ProcessoGo> log, ILoggerFactory fabrica)
    {
        _opcoes = opcoes;
        _log = log;
        _fabrica = fabrica;
    }

    /// <summary>O Go no ar agora, ou null enquanto ele sobe, reinicia ou está fora.</summary>
    public Destino? DestinoAtual => _destino;

    protected override async Task ExecuteAsync(CancellationToken parar)
    {
        if (!File.Exists(_opcoes.Executavel))
        {
            _log.LogError("O executável do Go não foi encontrado em {Executavel}: as rotas {Prefixos} respondem 503. "
                + "Publique de novo (scripts ou .msi) ou, em desenvolvimento, compile o src/backend-go.",
                _opcoes.Executavel, string.Join(";", _opcoes.Prefixos));
            return;
        }

        var espera = EsperaInicial;
        while (!parar.IsCancellationRequested)
        {
            var inicio = Stopwatch.GetTimestamp();
            try
            {
                await RodarUmaVezAsync(parar);
            }
            catch (OperationCanceledException) when (parar.IsCancellationRequested)
            {
                break;
            }
            catch (Exception ex)
            {
                _log.LogError(ex, "O Go não subiu: {Mensagem}", ex.Message);
            }
            finally
            {
                _destino = null;
            }
            if (parar.IsCancellationRequested) break;

            // Ficou mais de um minuto no ar: a queda não é de partida, recomeça a espera do começo.
            if (Stopwatch.GetElapsedTime(inicio) > EsperaMaxima) espera = EsperaInicial;
            _log.LogWarning("O Go está fora do ar; as rotas dele respondem 503. Nova tentativa em {Segundos} s.",
                (int)espera.TotalSeconds);
            try { await Task.Delay(espera, parar); }
            catch (OperationCanceledException) { break; }
            espera = TimeSpan.FromTicks(Math.Min(espera.Ticks * 2, EsperaMaxima.Ticks));
        }
    }

    private async Task RodarUmaVezAsync(CancellationToken parar)
    {
        var segredo = Convert.ToHexString(RandomNumberGenerator.GetBytes(32));
        var inicio = new ProcessStartInfo(_opcoes.Executavel)
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            // Sem BOM: o segredo é a primeira linha da entrada, e um BOM viraria parte dele.
            StandardInputEncoding = new UTF8Encoding(false),
            StandardOutputEncoding = Encoding.UTF8,
            StandardErrorEncoding = Encoding.UTF8,
            WorkingDirectory = Path.GetDirectoryName(_opcoes.Executavel)!
        };
        foreach (var argumento in new[] { "-atras-do-dotnet", "-banco", _opcoes.CaminhoBanco, "-pasta", AppContext.BaseDirectory })
            inicio.ArgumentList.Add(argumento);
        // Só para teste (#363): o relógio do Go continua o do .NET (Go:InicioDoRelogio + o tempo desde a partida).
        // Vai o instante já validado e formatado aqui, nunca o texto da configuração.
        if (_opcoes.InstanteDoRelogio() is { } relogio)
        {
            inicio.ArgumentList.Add("-inicio-do-relogio");
            inicio.ArgumentList.Add(relogio.ToString("O", CultureInfo.InvariantCulture));
        }

        using var processo = new Process { StartInfo = inicio };
        var ultimas = new Queue<string>();
        try
        {
            processo.Start();
            if (OperatingSystem.IsWindows())
            {
                try { JobObjectWindows.Atribuir(processo); }
                catch (Win32Exception ex)
                {
                    _log.LogWarning(ex, "O Go subiu fora do Job Object: se o .NET for encerrado à força, ele para pela "
                        + "entrada padrão, mas não se estiver travado.");
                }
            }
            var leituraErros = LerSaidaDeErroAsync(processo.StandardError, ultimas);

            await processo.StandardInput.WriteLineAsync(segredo.AsMemory(), parar);
            await processo.StandardInput.FlushAsync(parar);

            var porta = await LerPortaAsync(processo, parar);
            _destino = new Destino(new Uri($"http://127.0.0.1:{porta}/"), segredo, processo.Id);
            _log.LogInformation("Go no ar em http://127.0.0.1:{Porta} (processo {Pid}), atendendo {Prefixos}.",
                porta, processo.Id, string.Join(";", _opcoes.Prefixos));
            var leituraSaida = LerSaidaAsync(processo.StandardOutput);

            try
            {
                await processo.WaitForExitAsync(parar);
            }
            catch (OperationCanceledException)
            {
                _destino = null;
                await PararAsync(processo);
                throw;
            }

            _destino = null;
            await Task.WhenAny(Task.WhenAll(leituraErros, leituraSaida), Task.Delay(TimeSpan.FromSeconds(2), CancellationToken.None));
            string detalhe;
            lock (ultimas) detalhe = string.Join('\n', ultimas);
            _log.LogError(new ErroDoGo(detalhe), "O Go parou sozinho, com o código {Codigo}.", processo.ExitCode);
        }
        finally
        {
            try
            {
                if (!processo.HasExited) processo.Kill(entireProcessTree: true);
            }
            catch (InvalidOperationException) { /* nem chegou a iniciar */ }
        }
    }

    /// <summary>Espera a linha <c>{"porta":N}</c>. Se o Go sair antes, o erro leva a saída de erro dele.</summary>
    private async Task<int> LerPortaAsync(Process processo, CancellationToken parar)
    {
        using var prazo = CancellationTokenSource.CreateLinkedTokenSource(parar);
        prazo.CancelAfter(PrazoParaAvisarAPorta);
        string? linha;
        try
        {
            linha = await processo.StandardOutput.ReadLineAsync(prazo.Token);
        }
        catch (OperationCanceledException) when (!parar.IsCancellationRequested)
        {
            throw new InvalidOperationException($"o Go não avisou a porta em {PrazoParaAvisarAPorta.TotalSeconds} s.");
        }
        if (linha is null)
        {
            await processo.WaitForExitAsync(parar);
            throw new InvalidOperationException($"o Go saiu com o código {processo.ExitCode} antes de avisar a porta (veja os registros Financas.Go).");
        }
        try
        {
            using var json = JsonDocument.Parse(linha);
            var porta = json.RootElement.GetProperty("porta").GetInt32();
            if (porta is > 0 and <= 65535) return porta;
        }
        catch (Exception ex) when (ex is JsonException or KeyNotFoundException or InvalidOperationException or FormatException)
        {
        }
        throw new InvalidOperationException("a primeira linha do Go não é {\"porta\":N}.");
    }

    /// <summary>Fecha a entrada (o Go para com elegância) e, se ele não parar em 10 s, encerra à força.</summary>
    private async Task PararAsync(Process processo)
    {
        try { processo.StandardInput.Close(); }
        catch (IOException) { /* o Go já tinha saído */ }
        using var prazo = new CancellationTokenSource(PrazoParaParar);
        try
        {
            await processo.WaitForExitAsync(prazo.Token);
            _log.LogInformation("Go parado (código {Codigo}).", processo.ExitCode);
        }
        catch (OperationCanceledException)
        {
            _log.LogWarning("O Go não parou em {Segundos} s; encerrado à força.", (int)PrazoParaParar.TotalSeconds);
            processo.Kill(entireProcessTree: true);
        }
    }

    private async Task LerSaidaDeErroAsync(StreamReader leitor, Queue<string> ultimas)
    {
        await Task.Yield();
        try
        {
            while (await leitor.ReadLineAsync() is { } linha)
            {
                lock (ultimas)
                {
                    ultimas.Enqueue(linha);
                    while (ultimas.Count > LinhasGuardadas) ultimas.Dequeue();
                }
                Registrar(linha);
            }
        }
        catch (Exception ex) when (ex is IOException or ObjectDisposedException or InvalidOperationException)
        {
            // O processo foi descartado (parada ou falha na partida): não há mais o que ler.
        }
    }

    private async Task LerSaidaAsync(StreamReader leitor)
    {
        await Task.Yield();
        var log = _fabrica.CreateLogger(CategoriaLog);
        try
        {
            while (await leitor.ReadLineAsync() is { } linha)
                log.LogInformation("{Linha}", linha);
        }
        catch (Exception ex) when (ex is IOException or ObjectDisposedException or InvalidOperationException)
        {
            // O processo foi descartado: não há mais o que ler.
        }
    }

    /// <summary>
    /// Uma linha do log do Go (slog em JSON: time, level, msg e atributos) vira um registro do .NET com o mesmo nível.
    /// Linha que não é JSON (ex.: o rastro de um pânico) vai como informação, e entra no detalhe do erro de parada.
    /// </summary>
    private void Registrar(string linha)
    {
        if (linha.StartsWith('{'))
        {
            try
            {
                using var json = JsonDocument.Parse(linha);
                var raiz = json.RootElement;
                var nivel = raiz.TryGetProperty("level", out var l) ? l.GetString() : null;
                var mensagem = raiz.TryGetProperty("msg", out var m) ? m.GetString() ?? "" : "";
                var origem = raiz.TryGetProperty("origem", out var o) && o.ValueKind == JsonValueKind.String ? o.GetString() : null;
                var atributos = string.Join(" ", raiz.EnumerateObject()
                    .Where(p => p.Name is not ("time" or "level" or "msg" or "origem"))
                    .Select(p => $"{p.Name}={(p.Value.ValueKind == JsonValueKind.String ? p.Value.GetString() : p.Value.GetRawText())}"));
                var categoria = origem is not null && OrigensConhecidas.Contains(origem) ? $"{CategoriaLog}.{origem}" : CategoriaLog;
                _fabrica.CreateLogger(categoria).Log(Nivel(nivel), "{Mensagem}",
                    atributos.Length > 0 ? $"{mensagem} ({atributos})" : mensagem);
                return;
            }
            catch (JsonException) { /* não era JSON: segue como linha solta */ }
        }
        _fabrica.CreateLogger(CategoriaLog).LogInformation("{Linha}", linha);
    }

    private static LogLevel Nivel(string? nivel) => nivel switch
    {
        _ when nivel is null => LogLevel.Information,
        _ when nivel.StartsWith("ERROR", StringComparison.Ordinal) => LogLevel.Error,
        _ when nivel.StartsWith("WARN", StringComparison.Ordinal) => LogLevel.Warning,
        _ when nivel.StartsWith("DEBUG", StringComparison.Ordinal) => LogLevel.Debug,
        _ => LogLevel.Information
    };

    /// <summary>As últimas linhas da saída de erro do Go, como detalhe do registro de parada.</summary>
    private sealed class ErroDoGo(string saida) : Exception("O Go parou sozinho.")
    {
        public override string ToString() => saida.Length > 0 ? saida : Message;
    }
}
