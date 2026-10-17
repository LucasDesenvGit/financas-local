using Financas.Api.Data;
using Financas.Api.Dtos;
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

    /// <summary>Banco que vale para cada conta (ADR 0024): escolhido, herdado da conta do cartão ou detectado.</summary>
    public async Task<Dictionary<int, string?>> BancosAsync(CancellationToken ct = default)
    {
        var contas = await _db.Contas.AsNoTracking()
            .Select(c => new ContaParaBanco(c.Id, c.Nome, c.ConexaoBancaria != null ? c.ConexaoBancaria.NomeInstituicao : null,
                c.Banco, c.ContaVinculadaId))
            .ToListAsync(ct);
        return BancoService.Efetivos(contas);
    }

    /// <summary>
    /// UX-12/#103: o que cada cartão lançou no período — só as transações do período (compras e a parcela do mês),
    /// não o total informado da fatura nem a dívida total. Pagamento de fatura e transferências ficam de fora; despesa
    /// marcada para ignorar nos relatórios entra (ela continua na conta do cartão). A fatura com vencimento no período
    /// vai só como referência (vencimento e situação). Usado pelo Painel ("Faturas do mês") e por Contas e cartões.
    /// </summary>
    public async Task<List<FaturaMesDto>> FaturasDoMesAsync(DateOnly inicio, DateOnly fim, DateOnly hoje, CancellationToken ct = default)
    {
        var cartoes = await _db.Contas.AsNoTracking()
            .Where(c => c.Tipo == TipoConta.CartaoCredito && !c.Arquivada).Select(c => new { c.Id, c.Nome }).ToListAsync(ct);
        var idsCartoes = cartoes.Select(c => c.Id).ToList();
        var lancadoNoMes = await _db.Transacoes.AsNoTracking()
            .Where(t => idsCartoes.Contains(t.ContaId) && t.Data >= inicio && t.Data <= fim && t.Tipo == TipoTransacao.Despesa)
            .GroupBy(t => t.ContaId)
            .Select(g => new
            {
                ContaId = g.Key,
                Valor = -g.Sum(t => t.Valor),
                Parcelas = -g.Where(t => t.ParcelaTotal != null && t.ParcelaTotal > 1).Sum(t => t.Valor)
            })
            .ToDictionaryAsync(x => x.ContaId, ct);
        var faturasVencendo = (await _db.FaturasCartao.AsNoTracking()
                .Where(f => f.DataVencimento >= inicio && f.DataVencimento <= fim && idsCartoes.Contains(f.ContaId))
                .Select(f => new { f.Id, f.ContaId, f.DataVencimento, f.DataFechamento, f.ValorTotal, f.ValorPago })
                .ToListAsync(ct))
            .GroupBy(f => f.ContaId).ToDictionary(g => g.Key, g => g.OrderBy(f => f.DataVencimento).First());
        var bancos = await BancosAsync(ct);
        return cartoes
            .Select(c =>
            {
                var lancado = lancadoNoMes.GetValueOrDefault(c.Id);
                var fatura = faturasVencendo.GetValueOrDefault(c.Id);
                return new FaturaMesDto(c.Id, c.Nome, Math.Round(Math.Max(0m, lancado?.Valor ?? 0m), 2),
                    Math.Round(Math.Max(0m, lancado?.Parcelas ?? 0m), 2), fatura?.Id, fatura?.DataVencimento,
                    fatura is null ? null : SituacaoFatura(Math.Abs(fatura.ValorTotal), fatura.ValorPago, fatura.DataFechamento,
                        fatura.DataVencimento, hoje),
                    bancos.GetValueOrDefault(c.Id));
            })
            .OrderByDescending(f => f.ValorMes).ThenBy(f => f.Cartao)
            .ToList();
    }

    /// <summary>
    /// Paga quando os pagamentos informados cobrem o total; fechada quando já fechou (ou venceu) sem pagamento
    /// informado; aberta enquanto ainda recebe compras. A instituição nem sempre informa o pagamento, por isso uma
    /// fatura vencida sem pagamento fica "fechada", não "atrasada" (evita alarme falso).
    /// </summary>
    public static string SituacaoFatura(decimal total, decimal? pago, DateOnly? fechamento, DateOnly vencimento, DateOnly hoje)
    {
        if (total <= 0 || (pago is { } p && Math.Abs(p) >= total - 0.01m)) return "paga";
        return (fechamento is { } f && f <= hoje) || vencimento < hoje ? "fechada" : "aberta";
    }

    /// <summary>Remove vínculos com faturas antes de excluir contas (evita conflito de FK sem cascata).</summary>
    public Task DesvincularFaturasAsync(IReadOnlyCollection<int> contaIds, CancellationToken ct = default)
        => _db.Transacoes
            .Where(t => contaIds.Contains(t.ContaId) && t.FaturaCartaoId != null)
            .ExecuteUpdateAsync(s => s.SetProperty(t => t.FaturaCartaoId, (int?)null), ct);
}
