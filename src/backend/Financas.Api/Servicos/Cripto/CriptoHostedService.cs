namespace Financas.Api.Servicos.Cripto;

/// <summary>
/// Mantém a cotação de criptomoedas guardada no banco (ADR 0048): consulta o Mercado Bitcoin a cada 30 minutos.
/// Cripto negocia 24 h por dia, então não há horário de fechamento como na PTAX.
/// </summary>
public class CriptoHostedService : BackgroundService
{
    private static readonly TimeSpan Intervalo = TimeSpan.FromMinutes(30);

    private readonly IServiceScopeFactory _scopes;
    private readonly ILogger<CriptoHostedService> _logger;

    public CriptoHostedService(IServiceScopeFactory scopes, ILogger<CriptoHostedService> logger)
    {
        _scopes = scopes;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        try
        {
            // Deixa a API subir antes da primeira consulta (como o câmbio).
            await Task.Delay(TimeSpan.FromMinutes(1), stoppingToken);
        }
        catch (OperationCanceledException)
        {
            return;
        }

        using var timer = new PeriodicTimer(Intervalo);
        do
        {
            try
            {
                using var scope = _scopes.CreateScope();
                var cripto = scope.ServiceProvider.GetRequiredService<CriptoService>();
                var resultado = await cripto.AtualizarAsync(stoppingToken);
                _logger.LogInformation("Cotação de criptomoedas atualizada (novas: {Novas}).", resultado.Novas);
            }
            catch (OperationCanceledException) when (stoppingToken.IsCancellationRequested)
            {
                break;
            }
            catch (HttpRequestException ex)
            {
                // Sem internet ou fonte fora do ar: tenta de novo no próximo ciclo; a tela mostra a última guardada.
                _logger.LogWarning(ex, "Não foi possível consultar a cotação de criptomoedas no Mercado Bitcoin.");
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Falha na atualização automática da cotação de criptomoedas.");
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
}
