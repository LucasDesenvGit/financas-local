namespace Financas.Gerenciador;

internal static class Program
{
    private const string NomeUnico = @"Local\FinancasLocalGerenciador";
    private const string NomePedidoMostrar = @"Local\FinancasLocalGerenciador.Mostrar";

    /// <summary>
    /// Sem argumentos: a janela. <c>--bandeja</c>: só o ícone da área de notificação (início com o Windows).
    /// <c>--servico ligar|desligar|reiniciar</c>, <c>--backup "destino"</c> e <c>--instalar-pelos-scripts</c> (só na
    /// instalação de desenvolvimento, ADR 0040): ações que o próprio gerenciador
    /// roda como administrador (UAC), sem janela.
    /// </summary>
    [STAThread]
    private static int Main(string[] args)
    {
        if (args.Length == 2 && args[0] == "--servico" && args[1] is "ligar" or "desligar" or "reiniciar")
            return ServicoWindows.Executar(args[1]);
        if (args.Length == 2 && args[0] == "--backup") return AcoesElevadas.Backup(args[1]);
        if (args is ["--instalar-pelos-scripts"]) return AcoesElevadas.InstalarPelosScripts();

        using var unico = new Mutex(true, NomeUnico, out var primeiro);
        if (!primeiro)
        {
            // Já está aberto (talvez só na área de notificação): pede para mostrar a janela.
            if (EventWaitHandle.TryOpenExisting(NomePedidoMostrar, out var pedido)) using (pedido) pedido.Set();
            return 0;
        }

        ApplicationConfiguration.Initialize();
        using var pedidoParaMostrar = new EventWaitHandle(false, EventResetMode.AutoReset, NomePedidoMostrar);
        Application.Run(new FormPrincipal(args.Contains("--bandeja"), pedidoParaMostrar));
        return 0;
    }
}
