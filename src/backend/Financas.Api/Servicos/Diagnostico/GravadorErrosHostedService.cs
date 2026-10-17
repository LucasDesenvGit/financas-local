using System.Security.Cryptography;
using System.Text;
using Financas.Api.Data;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Diagnostico;

/// <summary>
/// Grava no banco os erros da <see cref="FilaErros"/> (#11, ADR 0022), já limpos de dados sensíveis, e apaga os
/// antigos uma vez por dia. A mesma ocorrência (assinatura) ainda em aberto e vista nas últimas 24 horas só soma
/// +1 em Ocorrencias; depois de marcada como resolvida, se acontecer de novo, vira uma linha nova.
/// </summary>
public class GravadorErrosHostedService : BackgroundService
{
    private const int MaximoPorLote = 100;
    private static readonly TimeSpan JanelaAgrupamento = TimeSpan.FromHours(24);

    private readonly FilaErros _fila;
    private readonly IServiceScopeFactory _scopes;
    private readonly ILogger<GravadorErrosHostedService> _logger;
    private DateTime _ultimaLimpeza = DateTime.MinValue;

    public GravadorErrosHostedService(FilaErros fila, IServiceScopeFactory scopes, ILogger<GravadorErrosHostedService> logger)
    {
        _fila = fila;
        _scopes = scopes;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        try
        {
            while (!stoppingToken.IsCancellationRequested)
            {
                // Acorda com erro novo na fila ou, no máximo, a cada hora (para a limpeza diária rodar mesmo sem erros).
                using (var espera = CancellationTokenSource.CreateLinkedTokenSource(stoppingToken))
                {
                    espera.CancelAfter(TimeSpan.FromHours(1));
                    try { await _fila.Leitor.WaitToReadAsync(espera.Token); }
                    catch (OperationCanceledException) when (!stoppingToken.IsCancellationRequested) { }
                }
                await GravarLoteAsync(stoppingToken);
                await LimparSeVenceuAsync(stoppingToken);
            }
        }
        catch (OperationCanceledException) when (stoppingToken.IsCancellationRequested)
        {
            // Desligando: grava o que ainda estiver na fila, sem esperar por mais.
            using var prazo = new CancellationTokenSource(TimeSpan.FromSeconds(5));
            try { await GravarLoteAsync(prazo.Token); } catch (Exception) { /* sem banco no desligamento: paciência */ }
        }
    }

    private async Task GravarLoteAsync(CancellationToken ct)
    {
        var lote = new List<RegistroErro>();
        while (lote.Count < MaximoPorLote && _fila.Leitor.TryRead(out var r)) lote.Add(r);
        if (lote.Count == 0) return;
        try
        {
            using var scope = _scopes.CreateScope();
            var db = scope.ServiceProvider.GetRequiredService<FinancasDbContext>();
            foreach (var r in lote) await GravarAsync(db, r, ct);
            await db.SaveChangesAsync(ct);
        }
        catch (Exception ex) when (ex is not OperationCanceledException)
        {
            // Categoria fora da captura: vai só para o log de eventos, sem voltar para a fila.
            _logger.LogError(ex, "Não foi possível gravar {Quantidade} erro(s) na tabela de erros.", lote.Count);
        }
    }

    private static async Task GravarAsync(FinancasDbContext db, RegistroErro r, CancellationToken ct)
    {
        var mensagem = Sanitizador.LimparECortar(r.Mensagem, 1000)!;
        var detalhe = Sanitizador.LimparECortar(r.Detalhe, 8000);
        var assinatura = Assinatura(r.Categoria, mensagem, detalhe);
        var desde = r.Em - JanelaAgrupamento;

        // Primeiro o que já está na memória deste lote (ainda não salvo), depois o banco.
        var existente = db.ErrosSistema.Local.FirstOrDefault(e => e.Assinatura == assinatura && !e.Resolvido && e.UltimaEm >= desde)
            ?? await db.ErrosSistema
                .Where(e => e.Assinatura == assinatura && !e.Resolvido && e.UltimaEm >= desde)
                .OrderByDescending(e => e.UltimaEm)
                .FirstOrDefaultAsync(ct);
        if (existente is not null)
        {
            existente.Ocorrencias++;
            existente.UltimaEm = r.Em;
            if (r.Nivel > existente.Nivel) existente.Nivel = r.Nivel;
            existente.Rota = Sanitizador.LimparECortar(r.Rota, 300) ?? existente.Rota;
            existente.Usuario = r.Usuario is null ? existente.Usuario : Sanitizador.LimparECortar(r.Usuario, 100);
            return;
        }
        db.ErrosSistema.Add(new ErroSistema
        {
            PrimeiraEm = r.Em,
            UltimaEm = r.Em,
            Nivel = r.Nivel,
            Origem = r.Origem.Length > 30 ? r.Origem[..30] : r.Origem,
            Categoria = r.Categoria.Length > 200 ? r.Categoria[..200] : r.Categoria,
            Mensagem = mensagem,
            Detalhe = detalhe,
            Rota = Sanitizador.LimparECortar(r.Rota, 300),
            Usuario = Sanitizador.LimparECortar(r.Usuario, 100),
            Assinatura = assinatura
        });
    }

    /// <summary>Categoria + mensagem + tipo da exceção (primeira linha do detalhe, até o ":").</summary>
    public static string Assinatura(string categoria, string mensagem, string? detalhe)
    {
        var tipo = detalhe is null ? "" : detalhe.Split('\n', 2)[0].Split(':', 2)[0].Trim();
        var bytes = SHA256.HashData(Encoding.UTF8.GetBytes($"{categoria}\n{mensagem}\n{tipo}"));
        return Convert.ToHexString(bytes, 0, 16);
    }

    private async Task LimparSeVenceuAsync(CancellationToken ct)
    {
        if (DateTime.UtcNow - _ultimaLimpeza < TimeSpan.FromDays(1)) return;
        _ultimaLimpeza = DateTime.UtcNow;
        try
        {
            using var scope = _scopes.CreateScope();
            var parametros = scope.ServiceProvider.GetRequiredService<ParametroService>();
            var dias = await parametros.ObterInteiroAsync(ConfiguracaoService.DiasRetencaoErros, ct);
            var limite = DateTime.UtcNow.AddDays(-dias);
            var db = scope.ServiceProvider.GetRequiredService<FinancasDbContext>();
            await db.ErrosSistema.Where(e => e.UltimaEm < limite).ExecuteDeleteAsync(ct);
        }
        catch (Exception ex) when (ex is not OperationCanceledException)
        {
            _logger.LogError(ex, "Não foi possível apagar os erros antigos.");
        }
    }
}
