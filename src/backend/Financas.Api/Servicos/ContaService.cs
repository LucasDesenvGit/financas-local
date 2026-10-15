using Financas.Api.Data;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

public class ContaService
{
    private readonly FinancasDbContext _db;

    public ContaService(FinancasDbContext db) => _db = db;

    /// <summary>
    /// Saldo atual de cada conta. Contas Open Finance: saldo informado pela instituição.
    /// Contas manuais: saldo inicial + transações efetivadas.
    /// </summary>
    public async Task<Dictionary<int, decimal>> CalcularSaldosAsync(CancellationToken ct = default)
    {
        var contas = await _db.Contas.AsNoTracking()
            .Select(c => new { c.Id, c.Origem, c.Saldo, c.SaldoInicial })
            .ToListAsync(ct);

        var somasManuais = await _db.Transacoes.AsNoTracking()
            .Where(t => t.Conta!.Origem == OrigemConta.Manual && t.Status == StatusTransacao.Efetivada)
            .GroupBy(t => t.ContaId)
            .Select(g => new { ContaId = g.Key, Soma = g.Sum(t => t.Valor) })
            .ToDictionaryAsync(x => x.ContaId, x => x.Soma, ct);

        return contas.ToDictionary(
            c => c.Id,
            c => c.Origem == OrigemConta.OpenFinance
                ? c.Saldo
                : c.SaldoInicial + (somasManuais.TryGetValue(c.Id, out var s) ? s : 0m));
    }

    /// <summary>Remove vínculos com faturas antes de excluir contas (evita conflito de FK sem cascata).</summary>
    public Task DesvincularFaturasAsync(IReadOnlyCollection<int> contaIds, CancellationToken ct = default)
        => _db.Transacoes
            .Where(t => contaIds.Contains(t.ContaId) && t.FaturaCartaoId != null)
            .ExecuteUpdateAsync(s => s.SetProperty(t => t.FaturaCartaoId, (int?)null), ct);
}
