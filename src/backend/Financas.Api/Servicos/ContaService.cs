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
    /// #144: a fatura de cada cartão que fecha no mês — o ciclo vai do fechamento anterior até o deste mês, não do dia
    /// 1 ao fim do mês (que pegava só parte das compras e misturava duas faturas). Fatura já fechada e importada do
    /// banco vale o total que ele informou; fatura ainda aberta é a soma das transações do ciclo: compras, a parcela
    /// que cai nele e estornos descontados (parcelas futuras têm data futura e ficam de fora, #103). Pagamento de
    /// fatura e transferências não entram; despesa marcada para ignorar nos relatórios entra (ela continua na conta do
    /// cartão). Cartão sem fechamento conhecido (manual sem "Dia de fechamento") fica com o mês do calendário.
    /// Usado pelo Painel ("Faturas do mês") e por Contas e cartões.
    /// </summary>
    public async Task<List<FaturaMesDto>> FaturasDoMesAsync(DateOnly inicio, DateOnly fim, DateOnly hoje, CancellationToken ct = default)
    {
        var cartoes = await _db.Contas.AsNoTracking()
            .Where(c => c.Tipo == TipoConta.CartaoCredito && !c.Arquivada)
            .Select(c => new { c.Id, c.Nome, c.DiaFechamento, c.DiaVencimento }).ToListAsync(ct);
        var idsCartoes = cartoes.Select(c => c.Id).ToList();
        var faturas = (await _db.FaturasCartao.AsNoTracking()
                .Where(f => idsCartoes.Contains(f.ContaId))
                .Select(f => new FaturaDoCiclo(f.Id, f.ContaId, f.DataFechamento, f.DataVencimento, f.ValorTotal, f.ValorPago))
                .ToListAsync(ct))
            .ToLookup(f => f.ContaId);
        var ciclos = cartoes.ToDictionary(c => c.Id,
            c => CicloDaFatura(inicio, fim, c.DiaFechamento, c.DiaVencimento, faturas[c.Id].ToList()));

        var desde = ciclos.Values.Select(c => c.Inicio).DefaultIfEmpty(inicio).Min();
        var ate = ciclos.Values.Select(c => c.Fim).DefaultIfEmpty(fim).Max();
        var transacoes = (await _db.Transacoes.AsNoTracking()
                .Where(t => idsCartoes.Contains(t.ContaId) && t.Data >= desde && t.Data <= ate
                            && t.Tipo != TipoTransacao.Transferencia
                            && (t.MeioPagamento == null || t.MeioPagamento != "PAGAMENTO_FATURA"))
                .Select(t => new { t.ContaId, t.Data, t.Valor, Parcela = t.ParcelaTotal != null && t.ParcelaTotal > 1 })
                .ToListAsync(ct))
            .ToLookup(t => t.ContaId);
        var bancos = await BancosAsync(ct);
        return cartoes
            .Select(c =>
            {
                var ciclo = ciclos[c.Id];
                var doCiclo = transacoes[c.Id].Where(t => t.Data >= ciclo.Inicio && t.Data <= ciclo.Fim).ToList();
                var lancado = Math.Max(0m, -doCiclo.Sum(t => t.Valor));
                var parcelas = Math.Max(0m, -doCiclo.Where(t => t.Parcela).Sum(t => t.Valor));
                var fatura = ciclo.Fatura;
                var totalInformado = fatura is null ? 0m : Math.Abs(fatura.ValorTotal);
                // Fatura fechada vale o total do banco; sem ele (aberta, ou o banco mandou zero), a soma do ciclo.
                var valor = ciclo.Fechamento is not null && totalInformado > 0 ? totalInformado : lancado;
                var situacao = ciclo.Vencimento is not { } vencimento ? null
                    : fatura is null && ciclo.Fechamento > hoje ? "aberta"
                    : SituacaoFatura(ciclo.Fechamento is null ? totalInformado : valor, fatura?.ValorPago,
                        fatura?.DataFechamento ?? ciclo.Fechamento, vencimento, hoje);
                return new FaturaMesDto(c.Id, c.Nome, Math.Round(valor, 2), Math.Round(parcelas, 2), fatura?.Id,
                    ciclo.Vencimento, situacao, bancos.GetValueOrDefault(c.Id), ciclo.Fechamento);
            })
            .OrderByDescending(f => f.ValorMes).ThenBy(f => f.Cartao)
            .ToList();
    }

    /// <summary>Fatura importada, só com o que o ciclo precisa.</summary>
    internal sealed record FaturaDoCiclo(int Id, int ContaId, DateOnly? DataFechamento, DateOnly DataVencimento,
        decimal ValorTotal, decimal? ValorPago);

    /// <summary>Período das transações (<c>Inicio</c> a <c>Fim</c>, inclusive) e a fatura desse ciclo, se já veio do banco.</summary>
    internal sealed record CicloFatura(DateOnly Inicio, DateOnly Fim, DateOnly? Fechamento, DateOnly? Vencimento,
        FaturaDoCiclo? Fatura);

    /// <summary>
    /// #144: ciclo da fatura que fecha no mês de <paramref name="fim"/>. O fechamento é o da fatura importada desse
    /// mês ou, ainda sem ela, o dia de fechamento do cartão (o informado ou o da última fatura); o banco mexe um ou
    /// dois dias por causa de fim de semana, por isso o fechamento anterior vem da fatura importada quando há. Sem
    /// dia de fechamento conhecido, o mês do calendário, com a fatura que vence nele como referência.
    /// </summary>
    internal static CicloFatura CicloDaFatura(DateOnly inicio, DateOnly fim, byte? diaFechamento, byte? diaVencimento,
        IReadOnlyList<FaturaDoCiclo> faturas)
    {
        var fechamentos = faturas.Where(f => f.DataFechamento is not null).Select(f => f.DataFechamento!.Value)
            .Distinct().Order().ToList();
        int? dia = diaFechamento ?? (fechamentos.Count > 0 ? fechamentos[^1].Day : (int?)null);
        if (dia is null)
        {
            var vencendo = faturas.Where(f => f.DataVencimento >= inicio && f.DataVencimento <= fim).MinBy(f => f.DataVencimento);
            return new CicloFatura(inicio, fim, null, vencendo?.DataVencimento, vencendo);
        }

        var fechamento = fechamentos.Where(d => d >= inicio && d <= fim).Select(d => (DateOnly?)d).LastOrDefault()
                         ?? DiaNoMes(fim, dia.Value);
        var anterior = fechamentos.Where(d => d < fechamento && d >= fechamento.AddDays(-45)).Select(d => (DateOnly?)d).LastOrDefault()
                       ?? DiaNoMes(fechamento.AddMonths(-1), dia.Value);
        var fatura = faturas
            .Where(f => f.DataFechamento is { } d
                ? d > anterior && d <= fechamento
                : f.DataVencimento > fechamento && f.DataVencimento <= fechamento.AddDays(25))
            .MaxBy(f => f.DataVencimento);

        var vencimento = fatura?.DataVencimento;
        int? diaVenc = diaVencimento ?? faturas.MaxBy(f => f.DataVencimento)?.DataVencimento.Day;
        if (vencimento is null && diaVenc is { } dv)
        {
            var noMes = DiaNoMes(fechamento, dv);
            vencimento = noMes > fechamento ? noMes : DiaNoMes(fechamento.AddMonths(1), dv);
        }
        return new CicloFatura(anterior.AddDays(1), fechamento, fechamento, vencimento, fatura);
    }

    /// <summary>O dia no mês de <paramref name="referencia"/>, limitado ao último dia dele (ex.: dia 31 em fevereiro).</summary>
    private static DateOnly DiaNoMes(DateOnly referencia, int dia)
        => new(referencia.Year, referencia.Month, Math.Min(dia, DateTime.DaysInMonth(referencia.Year, referencia.Month)));

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
