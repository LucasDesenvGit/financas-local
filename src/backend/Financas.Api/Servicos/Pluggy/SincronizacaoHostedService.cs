using Financas.Api.Data;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Pluggy;

/// <summary>
/// Executa a sincronização automática no intervalo configurado.
/// Para funcionar com o IIS, o pool de aplicativos é configurado como "AlwaysRunning" e sem tempo ocioso.
/// </summary>
public class SincronizacaoHostedService : BackgroundService
{
    private readonly IServiceScopeFactory _scopes;
    private readonly ILogger<SincronizacaoHostedService> _logger;
    private DateTime? _ultimaTentativa;

    public SincronizacaoHostedService(IServiceScopeFactory scopes, ILogger<SincronizacaoHostedService> logger)
    {
        _scopes = scopes;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        try
        {
            await Task.Delay(TimeSpan.FromMinutes(2), stoppingToken);
        }
        catch (OperationCanceledException)
        {
            return;
        }

        using var timer = new PeriodicTimer(TimeSpan.FromMinutes(5));
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
                _logger.LogError(ex, "Falha na verificação da sincronização automática.");
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
        if (SincronizacaoService.EmExecucao) return;

        using var scope = _scopes.CreateScope();
        var config = scope.ServiceProvider.GetRequiredService<ConfiguracaoService>();
        var db = scope.ServiceProvider.GetRequiredService<FinancasDbContext>();

        if (!await config.ObterBoolAsync(ConfiguracaoService.SincronizacaoHabilitada, true, ct)) return;
        if (!await db.ConexoesBancarias.AnyAsync(c => c.Ativa, ct)) return;

        var intervalo = Math.Clamp(await config.ObterInteiroAsync(ConfiguracaoService.IntervaloHoras, 6, ct), 1, 48);
        var ultima = await config.ObterDataAsync(ConfiguracaoService.UltimaSincronizacaoGeral, ct);
        if (ultima is not null && DateTime.UtcNow - ultima.Value.ToUniversalTime() < TimeSpan.FromHours(intervalo)) return;
        // Evita repetir tentativas a cada ciclo quando a sincronização está falhando (ex.: credenciais inválidas).
        if (_ultimaTentativa is not null && DateTime.UtcNow - _ultimaTentativa.Value < TimeSpan.FromHours(1)) return;
        _ultimaTentativa = DateTime.UtcNow;

        var sincronizacao = scope.ServiceProvider.GetRequiredService<SincronizacaoService>();
        var resultado = await sincronizacao.SincronizarAsync(null, "Automatica", ct);
        _logger.LogInformation("Sincronização automática: {Mensagem} (novas: {Novas}, atualizadas: {Atualizadas})",
            resultado.Mensagem, resultado.TransacoesNovas, resultado.TransacoesAtualizadas);
    }
}
