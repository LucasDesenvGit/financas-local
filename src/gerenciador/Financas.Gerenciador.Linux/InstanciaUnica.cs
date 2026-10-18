using System.Net.Sockets;

namespace Financas.Gerenciador;

/// <summary>
/// Um gerenciador por usuário: o primeiro escuta num socket Unix em <c>$XDG_RUNTIME_DIR</c> (só do usuário); o segundo
/// conecta, pede para mostrar a janela do primeiro e sai. Substitui o Mutex + EventWaitHandle do Windows, que não
/// têm nome entre processos no Linux.
/// </summary>
internal sealed class InstanciaUnica : IDisposable
{
    private readonly Socket _escuta;
    private readonly string _caminho;
    private readonly CancellationTokenSource _fim = new();

    /// <summary>Outra cópia pediu para mostrar a janela (chega fora da thread da interface).</summary>
    public event Action? PedidoParaMostrar;

    private InstanciaUnica(Socket escuta, string caminho)
    {
        _escuta = escuta;
        _caminho = caminho;
        _ = EscutarAsync();
    }

    /// <summary>A instância deste processo, ou null se já havia outra aberta (que foi avisada para aparecer).</summary>
    public static InstanciaUnica? Tentar()
    {
        var caminho = Caminho();
        try
        {
            using var cliente = new Socket(AddressFamily.Unix, SocketType.Stream, ProtocolType.Unspecified);
            cliente.Connect(new UnixDomainSocketEndPoint(caminho));
            cliente.Send("mostrar\n"u8);
            return null;
        }
        catch (SocketException)
        {
            // Ninguém escutando: o arquivo, se existir, sobrou de um gerenciador que fechou sem limpar.
            if (File.Exists(caminho)) File.Delete(caminho);
        }

        var escuta = new Socket(AddressFamily.Unix, SocketType.Stream, ProtocolType.Unspecified);
        escuta.Bind(new UnixDomainSocketEndPoint(caminho));
        File.SetUnixFileMode(caminho, UnixFileMode.UserRead | UnixFileMode.UserWrite);
        escuta.Listen(4);
        return new InstanciaUnica(escuta, caminho);
    }

    private static string Caminho()
    {
        var pasta = Environment.GetEnvironmentVariable("XDG_RUNTIME_DIR");
        return string.IsNullOrEmpty(pasta)
            ? Path.Combine(Path.GetTempPath(), $"financaslocal-gerenciador-{Environment.UserName}.sock")
            : Path.Combine(pasta, "financaslocal-gerenciador.sock");
    }

    private async Task EscutarAsync()
    {
        while (!_fim.IsCancellationRequested)
        {
            try
            {
                using var conexao = await _escuta.AcceptAsync(_fim.Token);
                PedidoParaMostrar?.Invoke();
            }
            catch (OperationCanceledException)
            {
                return;
            }
            catch (SocketException)
            {
                if (_fim.IsCancellationRequested) return;
            }
        }
    }

    public void Dispose()
    {
        _fim.Cancel();
        _escuta.Dispose();
        try { File.Delete(_caminho); } catch (IOException) { }
        _fim.Dispose();
    }
}
