using System.Threading.Channels;
using Financas.Api.Entidades;

namespace Financas.Api.Servicos.Diagnostico;

/// <summary>Um erro ou aviso ainda não gravado (texto ainda sem limpeza).</summary>
public sealed record RegistroErro(DateTime Em, NivelErro Nivel, string Origem, string Categoria, string Mensagem,
    string? Detalhe, string? Rota, string? Usuario);

/// <summary>
/// Fila em memória entre quem registra o erro (logger, endpoint do front-end) e quem grava no banco
/// (<see cref="GravadorErrosHostedService"/>). Registrar nunca espera nem lança: com a fila cheia (banco travado,
/// erro em laço), o registro é descartado; o log de eventos do sistema continua recebendo tudo.
/// </summary>
public sealed class FilaErros
{
    public const string OrigemFrontEnd = "Front-end";
    /// <summary>Erros do front-end aceitos por hora: a tela é do próprio usuário, mas um laço não pode lotar o banco.</summary>
    public const int MaximoFrontEndPorHora = 30;

    private readonly Channel<RegistroErro> _canal = Channel.CreateBounded<RegistroErro>(
        new BoundedChannelOptions(1000) { FullMode = BoundedChannelFullMode.DropWrite, SingleReader = true });
    private readonly Queue<DateTime> _frontEnd = new();
    private readonly Lock _trava = new();

    public ChannelReader<RegistroErro> Leitor => _canal.Reader;

    public bool Registrar(RegistroErro registro) => _canal.Writer.TryWrite(registro);

    /// <summary>Erro vindo do navegador, limitado a <see cref="MaximoFrontEndPorHora"/>.</summary>
    public bool RegistrarDoFrontEnd(RegistroErro registro)
    {
        lock (_trava)
        {
            var limite = registro.Em.AddHours(-1);
            while (_frontEnd.Count > 0 && _frontEnd.Peek() < limite) _frontEnd.Dequeue();
            if (_frontEnd.Count >= MaximoFrontEndPorHora) return false;
            _frontEnd.Enqueue(registro.Em);
        }
        return Registrar(registro);
    }
}
