namespace Financas.Api.Servicos.Cambio;

/// <summary>
/// Mantém as cotações de câmbio guardadas no banco para uso offline. Atualiza uma vez por dia e de novo
/// depois do boletim de fechamento da PTAX (publicado por volta das 13h no horário de Brasília).
/// </summary>
public class CambioHostedService : BackgroundService
{
    /// <summary>Margem depois do fechamento da PTAX para o boletim já estar disponível na API.</summary>
    private static readonly TimeOnly HorarioFechamento = new(13, 30);

    private readonly IServiceScopeFactory _scopes;
    private readonly ILogger<CambioHostedService> _logger;
    private DateTime? _ultimaTentativa;

    public CambioHostedService(IServiceScopeFactory scopes, ILogger<CambioHostedService> logger)
    {
        _scopes = scopes;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        try
        {
            await Task.Delay(TimeSpan.FromMinutes(1), stoppingToken);
        }
        catch (OperationCanceledException)
        {
            return;
        }

        using var timer = new PeriodicTimer(TimeSpan.FromMinutes(15));
        do
        {
            try
            {
                await VerificarEExecutarAsync(stoppingToken);
            }
            catch (OperationCanceledException) when (stoppingToken.IsCancellationRequested)
            {
                break;
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Falha na atualização automática do câmbio.");
            }
        } while (await ProximoCicloAsync(timer, stoppingToken));
    }

    private static async Task<bool> ProximoCicloAsync(PeriodicTimer timer, CancellationToken ct)
    {
        try
        {
            return await timer.WaitForNextTickAsync(ct);
        }
        catch (OperationCanceledException)
        {
            return false;
        }
    }

    private async Task VerificarEExecutarAsync(CancellationToken ct)
    {
        using var scope = _scopes.CreateScope();
        var config = scope.ServiceProvider.GetRequiredService<ConfiguracaoService>();

        var ultima = await config.ObterDataAsync(CambioService.ChaveUltimaAtualizacao, ct);
        if (!Vencida(ultima)) return;
        // Sem internet ou API fora do ar: não tenta de novo a cada ciclo.
        if (_ultimaTentativa is not null && DateTime.UtcNow - _ultimaTentativa.Value < TimeSpan.FromMinutes(30)) return;
        _ultimaTentativa = DateTime.UtcNow;

        var cambio = scope.ServiceProvider.GetRequiredService<CambioService>();
        var resultado = await cambio.AtualizarTodasAsync(ct);
        if (resultado.Falhas.Count > 0)
            _logger.LogWarning("Câmbio atualizado parcialmente (novas: {Novas}, corrigidas: {Corrigidas}); falharam: {Falhas}",
                resultado.Novas, resultado.Corrigidas, string.Join(", ", resultado.Falhas));
        else
            _logger.LogInformation("Câmbio atualizado (novas: {Novas}, corrigidas: {Corrigidas}).",
                resultado.Novas, resultado.Corrigidas);
    }

    /// <summary>Vencida se nunca rodou, se não rodou hoje, ou se já passou do fechamento e a última foi antes dele.</summary>
    private static bool Vencida(DateTime? ultimaUtc)
    {
        if (ultimaUtc is null) return true;
        var ultimaLocal = Relogio.ParaHoraLocal(ultimaUtc.Value);
        var agora = Relogio.AgoraLocal;
        if (DateOnly.FromDateTime(ultimaLocal) < DateOnly.FromDateTime(agora)) return true;
        var fechamentoHoje = agora.Date + HorarioFechamento.ToTimeSpan();
        return agora >= fechamentoHoje && ultimaLocal < fechamentoHoje;
    }
}
