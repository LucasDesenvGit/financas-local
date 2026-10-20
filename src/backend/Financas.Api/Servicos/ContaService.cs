using System.Linq.Expressions;
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
    /// #170: nome de cada conexão para mostrar. Pelo Meu Pluggy todas se chamam "MeuPluggy" (o conector); vale o banco
    /// das contas dela (escolhido ou detectado), depois o emissor dos investimentos, e o nome do conector por último.
    /// </summary>
    public async Task<Dictionary<int, string>> NomesDasConexoesAsync(CancellationToken ct = default)
    {
        var bancos = await BancosAsync(ct);
        var contas = await _db.Contas.AsNoTracking().Where(c => c.ConexaoBancariaId != null)
            .Select(c => new { c.Id, ConexaoId = c.ConexaoBancariaId!.Value }).ToListAsync(ct);
        var investimentos = await _db.Investimentos.AsNoTracking()
            .Select(i => new { i.ConexaoBancariaId, i.Emissor, i.Nome }).ToListAsync(ct);
        var conexoes = await _db.ConexoesBancarias.AsNoTracking().Select(c => new { c.Id, c.NomeInstituicao }).ToListAsync(ct);
        return conexoes.ToDictionary(c => c.Id, c =>
        {
            var nomes = contas.Where(x => x.ConexaoId == c.Id).Select(x => BancoService.Nome(bancos.GetValueOrDefault(x.Id)))
                .OfType<string>().Distinct().ToList();
            if (nomes.Count == 0)
                nomes = investimentos.Where(i => i.ConexaoBancariaId == c.Id)
                    .Select(i => BancoService.Nome(BancoService.Detectar(i.Emissor, i.Nome))).OfType<string>().Distinct().ToList();
            return nomes.Count > 0 ? string.Join(" + ", nomes) : c.NomeInstituicao ?? "Instituição";
        });
    }

    /// <summary>
    /// #144: a fatura de cada cartão que fecha no mês — o ciclo vai do fechamento anterior até o deste mês, não do dia
    /// 1 ao fim do mês (que pegava só parte das compras e misturava duas faturas). Fatura já fechada e importada do
    /// banco vale o total que ele informou; fatura ainda aberta é a soma das transações cobradas nela: compras, a parcela
    /// que cai nela e estornos descontados. "Cobradas nela" segue <see cref="MesesDeCobranca"/> (#167): o que o banco
    /// já ligou à fatura anterior não entra de novo, e a parcela vai para a fatura da 1ª mais (k − 1). Pagamento de
    /// fatura e transferências não entram; despesa marcada para ignorar nos relatórios entra (ela continua na conta do
    /// cartão). Cartão sem fechamento conhecido (manual sem "Dia de fechamento") fica com o mês do calendário.
    /// #251: a situação considera também o pagamento lançado no cartão (<see cref="PagoNasTransacoes"/>), e, quando a
    /// fatura do mês já fechou, vem junto a seguinte, ainda aberta (<c>Proxima</c>).
    /// Usado pelo Painel ("Faturas do mês") e por Contas e cartões.
    /// </summary>
    public async Task<List<FaturaMesDto>> FaturasDoMesAsync(DateOnly inicio, DateOnly fim, DateOnly hoje, CancellationToken ct = default)
    {
        var cartoes = await _db.Contas.AsNoTracking()
            .Where(c => c.Tipo == TipoConta.CartaoCredito && !c.Arquivada)
            .Select(c => new CartaoDoCiclo(c.Id, c.Nome, c.DiaFechamento, c.DiaVencimento)).ToListAsync(ct);
        var idsCartoes = cartoes.Select(c => c.Id).ToList();
        var faturas = (await _db.FaturasCartao.AsNoTracking()
                .Where(f => idsCartoes.Contains(f.ContaId))
                .Select(f => new FaturaDoCiclo(f.Id, f.ContaId, f.DataFechamento, f.DataVencimento, f.ValorTotal, f.ValorPago))
                .ToListAsync(ct))
            .ToLookup(f => f.ContaId);
        var ciclos = cartoes.ToDictionary(c => c.Id,
            c => CicloDaFatura(inicio, fim, c.DiaFechamento, c.DiaVencimento, faturas[c.Id].ToList()));

        // #167: os lançamentos vão para a fatura em que o banco cobra, e não só pela data. Carrega uma janela larga
        // para achar a parcela já ligada a uma fatura de cada compra (até 24 meses) e as parcelas com data futura.
        var desde = ciclos.Values.Select(c => c.Inicio).DefaultIfEmpty(inicio).Min().AddMonths(-24);
        var transacoes = (await _db.Transacoes.AsNoTracking()
                .Where(t => idsCartoes.Contains(t.ContaId) && t.Data >= desde
                            && t.Tipo != TipoTransacao.Transferencia
                            && (t.MeioPagamento == null || t.MeioPagamento != "PAGAMENTO_FATURA"))
                .Select(t => new LancamentoFatura(t.Id, t.ContaId, t.Data, t.Valor, t.FaturaCartaoId, t.ParcelaNumero,
                    t.ParcelaTotal, t.DescricaoOriginal ?? t.Descricao))
                .ToListAsync(ct))
            .ToLookup(t => t.ContaId);
        var pagamentos = (await _db.Transacoes.AsNoTracking()
                .Where(t => idsCartoes.Contains(t.ContaId) && t.Data >= desde)
                .Where(EhPagamentoNoCartao)
                .Select(t => new PagamentoNoCartao(t.ContaId, t.Data, t.Valor))
                .ToListAsync(ct))
            .ToLookup(p => p.ContaId);
        var bancos = await BancosAsync(ct);

        FaturaMesDto DoCiclo(CartaoDoCiclo c, CicloFatura ciclo, Dictionary<long, DateOnly> meses, FaturaMesDto? proxima)
        {
            var mesDoCiclo = PrimeiroDia(ciclo.Fechamento ?? ciclo.Fim);
            var doCiclo = transacoes[c.Id].Where(t => meses[t.Id] == mesDoCiclo).ToList();
            var lancado = Math.Max(0m, -doCiclo.Sum(t => t.Valor));
            var parcelas = Math.Max(0m, -doCiclo.Where(t => t.Parcelado).Sum(t => t.Valor));
            var fatura = ciclo.Fatura;
            var totalInformado = fatura is null ? 0m : Math.Abs(fatura.ValorTotal);
            // Fatura fechada vale o total do banco; sem ele (aberta, ou o banco mandou zero), a soma do ciclo.
            var valor = ciclo.Fechamento is not null && totalInformado > 0 ? totalInformado : lancado;
            var situacao = ciclo.Vencimento is not { } vencimento ? null
                : fatura is null && ciclo.Fechamento > hoje ? "aberta"
                : SituacaoFatura(ciclo.Fechamento is null ? totalInformado : valor,
                    Pago(fatura?.ValorPago, PagoNasTransacoes(vencimento, faturas[c.Id].Select(f => f.DataVencimento), pagamentos[c.Id])),
                    fatura?.DataFechamento ?? ciclo.Fechamento, vencimento, hoje);
            return new FaturaMesDto(c.Id, c.Nome, Math.Round(valor, 2), Math.Round(parcelas, 2), fatura?.Id,
                ciclo.Vencimento, situacao, bancos.GetValueOrDefault(c.Id), ciclo.Fechamento, proxima);
        }

        return cartoes
            .Select(c =>
            {
                var ciclo = ciclos[c.Id];
                var meses = MesesDeCobranca(transacoes[c.Id].ToList(), faturas[c.Id].ToList(), c.DiaFechamento);
                // #251: fatura do mês já fechada → a seguinte, a que ainda recebe compras, vem junto.
                FaturaMesDto? proxima = null;
                if (ciclo.Fechamento is { } fechou && fechou <= hoje)
                {
                    var seguinte = CicloDaFatura(fim.AddDays(1), fim.AddDays(1).AddMonths(1).AddDays(-1), c.DiaFechamento,
                        c.DiaVencimento, faturas[c.Id].ToList());
                    if (seguinte.Fechamento > hoje) proxima = DoCiclo(c, seguinte, meses, null);
                }
                return DoCiclo(c, ciclo, meses, proxima);
            })
            .OrderByDescending(f => f.ValorMes).ThenBy(f => f.Cartao)
            .ToList();
    }

    private sealed record CartaoDoCiclo(int Id, string Nome, byte? DiaFechamento, byte? DiaVencimento);

    /// <summary>Entrada lançada no cartão que paga a fatura (dado mínimo para <see cref="PagoNasTransacoes"/>).</summary>
    internal sealed record PagamentoNoCartao(int ContaId, DateOnly Data, decimal Valor);

    /// <summary>
    /// #251: entrada no cartão que é pagamento da fatura: a que virou transferência (pagamento pelo texto, pela
    /// categoria "Pagamento de fatura" ou transferência sua para o cartão; ADR 0036) ou a que o banco mandou como
    /// PAGAMENTO_FATURA. Estorno é despesa com valor positivo e fica de fora.
    /// </summary>
    internal static readonly Expression<Func<Transacao, bool>> EhPagamentoNoCartao =
        t => t.Valor > 0 && (t.Tipo == TipoTransacao.Transferencia || t.MeioPagamento == "PAGAMENTO_FATURA");

    /// <summary>
    /// #251: quanto da fatura que vence em <paramref name="vencimento"/> foi pago pelos lançamentos do cartão: os
    /// pagamentos depois do vencimento anterior e até este. O banco nem sempre manda os pagamentos da fatura (o Nubank
    /// não manda), mas o "Pagamento recebido" chega no cartão; e no Nubank dá para pagar antes do fechamento, por isso a
    /// janela começa no vencimento anterior e não no fechamento. O vencimento anterior é o da fatura importada até 45
    /// dias antes ou, sem ela, um mês antes. Pagamento atrasado (depois do vencimento) conta para a fatura seguinte.
    /// </summary>
    internal static decimal PagoNasTransacoes(DateOnly vencimento, IEnumerable<DateOnly> vencimentosImportados,
        IEnumerable<PagamentoNoCartao> pagamentos)
    {
        var anterior = vencimentosImportados.Where(v => v < vencimento && v >= vencimento.AddDays(-45))
            .DefaultIfEmpty(vencimento.AddMonths(-1)).Max();
        return pagamentos.Where(p => p.Data > anterior && p.Data <= vencimento).Sum(p => p.Valor);
    }

    /// <summary>#251: o pago que vale: o maior entre o que o banco informou e o lançado no cartão; null = nenhum dos dois.</summary>
    internal static decimal? Pago(decimal? informado, decimal lancado)
        => informado is null && lancado <= 0 ? null : Math.Max(Math.Abs(informado ?? 0m), lancado);

    /// <summary>Lançamento do cartão, com o que é preciso para saber em que fatura ele é cobrado (#167).</summary>
    internal sealed record LancamentoFatura(long Id, int ContaId, DateOnly Data, decimal Valor, int? FaturaCartaoId,
        short? ParcelaNumero, short? ParcelaTotal, string Descricao)
    {
        public bool Parcelado => ParcelaTotal is > 1;
    }

    /// <summary>
    /// #167: o mês de fechamento da fatura em que cada lançamento de um cartão é cobrado (dia 1 do mês).
    /// <list type="number">
    /// <item>Ligado pelo banco a uma fatura importada (<c>FaturaCartaoId</c>): o mês do fechamento dela. Um lançamento
    /// datado depois do fechamento mas já cobrado na fatura anterior não entra de novo na fatura aberta.</item>
    /// <item>Parcela de uma compra que tem outra parcela já ligada a uma fatura: o mês dessa fatura mais a diferença de
    /// número. A data que o Open Finance manda para parcelas futuras pode cair do outro lado do fechamento.</item>
    /// <item>O resto, pela data: o primeiro fechamento a partir dela (o de uma fatura importada até 40 dias depois ou,
    /// sem ela, o dia de fechamento do cartão). Sem dia de fechamento conhecido, o mês do calendário.</item>
    /// </list>
    /// A mesma compra é reconhecida como em <see cref="ParcelasFuturas"/>: total de parcelas, valor em reais inteiros e
    /// as 8 primeiras letras da descrição.
    /// </summary>
    internal static Dictionary<long, DateOnly> MesesDeCobranca(IReadOnlyList<LancamentoFatura> lancamentos,
        IReadOnlyList<FaturaDoCiclo> faturas, byte? diaFechamento)
    {
        // Sem data de fechamento, a fatura fecha uns 10 dias antes do vencimento.
        var mesDaFatura = faturas.ToDictionary(f => f.Id, f => PrimeiroDia(f.DataFechamento ?? f.DataVencimento.AddDays(-10)));
        var fechamentos = faturas.Where(f => f.DataFechamento is not null).Select(f => f.DataFechamento!.Value)
            .Distinct().Order().ToList();
        int? dia = diaFechamento ?? (fechamentos.Count > 0 ? fechamentos[^1].Day : (int?)null);
        DateOnly PelaData(DateOnly data)
        {
            var importado = fechamentos.FirstOrDefault(f => f >= data && f.DayNumber - data.DayNumber <= 40);
            if (importado != default) return PrimeiroDia(importado);
            if (dia is not int d) return PrimeiroDia(data);
            var noMes = DiaNoMes(data, d);
            return PrimeiroDia(data <= noMes ? noMes : DiaNoMes(data.AddMonths(1), d));
        }

        var meses = new Dictionary<long, DateOnly>();
        foreach (var t in lancamentos)
            if (t.FaturaCartaoId is int f && mesDaFatura.TryGetValue(f, out var mes))
                meses[t.Id] = mes;
        foreach (var compra in lancamentos.Where(t => t.Parcelado && t.ParcelaNumero is not null)
                     .GroupBy(t => (t.ParcelaTotal, Math.Round(Math.Abs(t.Valor), 0), ParcelasFuturas.Compra(t.Descricao))))
        {
            var ancora = compra.Where(t => meses.ContainsKey(t.Id)).MaxBy(t => t.ParcelaNumero);
            if (ancora is null) continue;
            foreach (var t in compra.Where(t => !meses.ContainsKey(t.Id)))
                meses[t.Id] = meses[ancora.Id].AddMonths(t.ParcelaNumero!.Value - ancora.ParcelaNumero!.Value);
        }
        foreach (var t in lancamentos)
            if (!meses.ContainsKey(t.Id)) meses[t.Id] = PelaData(t.Data);
        return meses;
    }

    private static DateOnly PrimeiroDia(DateOnly data) => new(data.Year, data.Month, 1);

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
    /// Paga quando os pagamentos (os que o banco informou ou os lançados no cartão, #251) cobrem o total; fechada
    /// quando já fechou (ou venceu) sem pagamento que cubra; aberta enquanto ainda recebe compras. A instituição nem
    /// sempre informa o pagamento, por isso uma fatura vencida sem pagamento fica "fechada", não "atrasada" (evita
    /// alarme falso).
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
