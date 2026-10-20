using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// Assistente de metas (#90, #149, ADR 0035): o dinheiro que entra e sai de cada mês, que é o que dá para guardar. O
/// plano em si (quanto vai para cada meta, os degraus, o percentual e o ajuste manual) é calculado na tela
/// (<c>metas-plano.ts</c>), para recalcular na hora quando o usuário mexe.
///
/// Sobra do mês pelo caixa (e não pela data da compra, como a Projeção):
///  - receitas e despesas das contas de dinheiro (corrente, poupança, dinheiro, outras), sem transferências;
///  - menos as faturas de cartão que <b>vencem</b> no mês. A compra no cartão sai do caixa quando a fatura vence;
///  - mês atual: o que já entrou e saiu, mais a média do que falta (receitas: o maior entre o recebido e a média);
///  - meses seguintes: as médias dos últimos meses completos (até 6) e a fatura estimada de cada mês.
/// Fatura estimada: já fechada, vale o total do banco; aberta, o que já foi lançado mais a média diária de compras à
/// vista nos dias que faltam; futura, a média de compras à vista. Nas três, mais as parcelas já contratadas que caem
/// nela (projetadas a partir das parcelas recentes, como na Projeção). É o que mostra a fatura caindo quando as parcelas
/// acabam.
///
/// Limites para o mês atual: o saldo previsto nas contas no fim do mês (depois de pagar a fatura) e o melhor mês recente
/// (quanto de fato sobrou). Julgamento próprio, não validado com uso real.
/// </summary>
public class AssistenteMetasService
{
    public const int MinimoMesesHistorico = 3;
    /// <summary>Até onde o caixa é estimado (a data provável de uma meta pode estar longe).</summary>
    private const int HorizonteMeses = 120;
    private const decimal DiasPorMes = 30.44m;

    private static readonly TipoConta[] TiposCaixa = [TipoConta.ContaCorrente, TipoConta.Poupanca, TipoConta.Dinheiro, TipoConta.Outro];

    private readonly FinancasDbContext _db;
    private readonly RelatorioService _relatorios;
    private readonly ContaService _contas;
    private readonly ParametroService _parametros;

    public AssistenteMetasService(FinancasDbContext db, RelatorioService relatorios, ContaService contas, ParametroService parametros)
    {
        _db = db;
        _relatorios = relatorios;
        _contas = contas;
        _parametros = parametros;
    }

    public async Task<PlanoMetasDto> PlanejarAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var mesAtual = new DateOnly(hoje.Year, hoje.Month, 1);
        var margem = await _parametros.ObterInteiroAsync(ConfiguracaoService.MetasMargemSeguranca, ct);
        var (inicioHistorico, mesesConsiderados) = await _relatorios.JanelaHistoricoAsync(ct);
        if (mesesConsiderados < MinimoMesesHistorico)
            return new PlanoMetasDto(false, mesesConsiderados, (short)mesAtual.Year, (byte)mesAtual.Month, margem, 0, 0, 0, 0, []);
        var fimHistorico = mesAtual.AddDays(-1);

        var contas = await _db.Contas.AsNoTracking().Where(c => !c.Arquivada)
            .Select(c => new { c.Id, c.Tipo, c.IncluirNoSaldo, c.DiaFechamento, c.DiaVencimento }).ToListAsync(ct);
        var idsCaixa = contas.Where(c => c.IncluirNoSaldo && TiposCaixa.Contains(c.Tipo)).Select(c => c.Id).ToList();
        var cartoes = contas.Where(c => c.Tipo == TipoConta.CartaoCredito).ToList();
        var idsCartoes = cartoes.Select(c => c.Id).ToList();

        // Contas de dinheiro: receitas e despesas como nos relatórios (sem transferências nem "ignorar nos relatórios").
        var movimentoCaixa = await _relatorios.Relevantes(inicioHistorico, hoje)
            .Where(t => idsCaixa.Contains(t.ContaId))
            .Select(t => new { t.Data, t.Valor, t.Tipo }).ToListAsync(ct);
        decimal Receitas(DateOnly de, DateOnly ate) => movimentoCaixa.Where(t => t.Data >= de && t.Data <= ate && t.Tipo == TipoTransacao.Receita).Sum(t => t.Valor);
        decimal Despesas(DateOnly de, DateOnly ate) => -movimentoCaixa.Where(t => t.Data >= de && t.Data <= ate && t.Tipo == TipoTransacao.Despesa).Sum(t => t.Valor);
        var mediaReceitas = Receitas(inicioHistorico, fimHistorico) / mesesConsiderados;
        var mediaDespesas = Despesas(inicioHistorico, fimHistorico) / mesesConsiderados;

        // Cartões: tudo o que entra na fatura (como a "fatura do mês", #144), inclusive o marcado para ignorar nos relatórios
        // e as parcelas que o banco já mandou com data futura.
        // Janela larga: a parcela já ligada a uma fatura de cada compra decide em que fatura caem as outras (#167).
        var desdeCartao = inicioHistorico.AddMonths(-24);
        var lancamentosCartao = await _db.Transacoes.AsNoTracking()
            .Where(t => idsCartoes.Contains(t.ContaId) && t.Data >= desdeCartao
                        && t.Tipo != TipoTransacao.Transferencia
                        && (t.MeioPagamento == null || t.MeioPagamento != "PAGAMENTO_FATURA"))
            .Select(t => new ContaService.LancamentoFatura(t.Id, t.ContaId, t.Data, t.Valor, t.FaturaCartaoId, t.ParcelaNumero,
                t.ParcelaTotal, t.DescricaoOriginal ?? t.Descricao))
            .ToListAsync(ct);
        // Média diária de compras à vista (estornos descontados) de cada cartão, para as faturas ainda abertas ou futuras.
        var compraAVistaPorDia = lancamentosCartao
            .Where(t => t.Data >= inicioHistorico && t.Data <= fimHistorico && !t.Parcelado)
            .GroupBy(t => t.ContaId)
            .ToDictionary(g => g.Key, g => Math.Max(0m, -g.Sum(t => t.Valor)) / mesesConsiderados / DiasPorMes);
        // Só as projetadas: as que já estão no banco entram pelo lançamento (#165). A fatura de cada projetada é a da
        // parcela de origem + os meses projetados (#167).
        var desdeParcelas = hoje.AddDays(-ParcelasFuturas.DiasRecentes);
        var parcelasFuturas = ParcelasFuturas.Projetar(lancamentosCartao
                .Where(t => t.Data >= desdeParcelas && t.Valor < 0 && t.Parcelado && t.ParcelaNumero is not null)
                .Select(t => new ParcelasFuturas.Parcela(t.Id, t.ContaId, t.Data, t.Valor, t.ParcelaNumero!.Value, t.ParcelaTotal!.Value,
                    t.Descricao)), hoje)
            .Where(p => !p.JaNoBanco).ToList();

        var faturasImportadas = (await _db.FaturasCartao.AsNoTracking()
                .Where(f => idsCartoes.Contains(f.ContaId))
                .Select(f => new ContaService.FaturaDoCiclo(f.Id, f.ContaId, f.DataFechamento, f.DataVencimento, f.ValorTotal, f.ValorPago))
                .ToListAsync(ct))
            .ToLookup(f => f.ContaId);
        // #251: pagamento lançado no cartão, para o banco que não manda os pagamentos da fatura.
        var desdePagamentos = hoje.AddMonths(-2);
        var pagamentosCartao = (await _db.Transacoes.AsNoTracking()
                .Where(t => idsCartoes.Contains(t.ContaId) && t.Data >= desdePagamentos)
                .Where(ContaService.EhPagamentoNoCartao)
                .Select(t => new ContaService.PagamentoNoCartao(t.ContaId, t.Data, t.Valor))
                .ToListAsync(ct))
            .ToLookup(p => p.ContaId);

        // Fatura de cada mês, pelo vencimento: soma dos cartões.
        var faturas = new Dictionary<DateOnly, (decimal Total, decimal Parcelas)>();
        var faturaAPagar = 0m;
        foreach (var cartao in cartoes)
        {
            var dele = lancamentosCartao.Where(t => t.ContaId == cartao.Id).ToList();
            var futurasDele = parcelasFuturas.Where(p => p.ContaId == cartao.Id).ToList();
            var importadas = faturasImportadas[cartao.Id].ToList();
            var porDia = compraAVistaPorDia.GetValueOrDefault(cartao.Id);
            var mesDeCobranca = ContaService.MesesDeCobranca(dele, importadas, cartao.DiaFechamento);
            for (var mes = inicioHistorico.AddMonths(-1); mes <= mesAtual.AddMonths(HorizonteMeses); mes = mes.AddMonths(1))
            {
                var ciclo = ContaService.CicloDaFatura(mes, mes.AddMonths(1).AddDays(-1), cartao.DiaFechamento, cartao.DiaVencimento, importadas);
                // Sem dia de fechamento conhecido, o ciclo é o mês do calendário e vence no mês seguinte (a fatura
                // importada que vence no próprio mês é de compras anteriores).
                var semFechamento = ciclo.Fechamento is null;
                var fechamento = ciclo.Fechamento ?? ciclo.Fim;
                var vencimento = semFechamento ? fechamento.AddMonths(1) : ciclo.Vencimento ?? fechamento.AddMonths(1);
                var importada = semFechamento ? null : ciclo.Fatura;
                var mesVencimento = new DateOnly(vencimento.Year, vencimento.Month, 1);
                if (mesVencimento < inicioHistorico) continue;

                var mesDoCiclo = new DateOnly(fechamento.Year, fechamento.Month, 1);
                var lancado = dele.Where(t => mesDeCobranca[t.Id] == mesDoCiclo).ToList();
                var parcelasLancadas = Math.Max(0m, -lancado.Where(t => t.Parcelado).Sum(t => t.Valor));
                decimal total, parcelas;
                if (fechamento <= hoje)
                {
                    var informado = importada is null ? 0m : Math.Abs(importada.ValorTotal);
                    total = informado > 0 ? informado : Math.Max(0m, -lancado.Sum(t => t.Valor));
                    parcelas = parcelasLancadas;
                }
                else
                {
                    var de = ciclo.Inicio > hoje ? ciclo.Inicio : hoje.AddDays(1);
                    var diasQueFaltam = fechamento.DayNumber - de.DayNumber + 1;
                    var projetadas = futurasDele
                        .Where(p => mesDeCobranca.TryGetValue(p.OrigemId, out var m) ? m.AddMonths(p.Passos) == mesDoCiclo
                            : p.Data >= de && p.Data <= fechamento)
                        .Sum(p => p.Valor);
                    total = Math.Max(0m, -lancado.Sum(t => t.Valor)) + porDia * diasQueFaltam + projetadas;
                    parcelas = parcelasLancadas + projetadas;
                }
                faturas[mesVencimento] = (faturas.GetValueOrDefault(mesVencimento).Total + total,
                    faturas.GetValueOrDefault(mesVencimento).Parcelas + parcelas);

                // O que ainda vai sair da conta este mês: fatura que vence de hoje em diante e não está paga.
                if (mesVencimento == mesAtual && vencimento >= hoje)
                    faturaAPagar += Math.Max(0m, total - (ContaService.Pago(importada?.ValorPago, ContaService.PagoNasTransacoes(
                        vencimento, importadas.Select(f => f.DataVencimento), pagamentosCartao[cartao.Id])) ?? 0m));
            }
        }

        // Mês atual: o realizado mais a média do que falta; meses seguintes: as médias e a fatura estimada.
        var diasMes = DateTime.DaysInMonth(hoje.Year, hoje.Month);
        var fracaoRestante = (decimal)(diasMes - hoje.Day) / diasMes;
        var recebido = Receitas(mesAtual, hoje);
        var saido = Despesas(mesAtual, hoje);
        var receitasMes = Math.Max(recebido, mediaReceitas);
        var despesasMes = saido + mediaDespesas * fracaoRestante;
        var meses = new List<MesCaixaDto>(HorizonteMeses);
        for (var k = 0; k < HorizonteMeses; k++)
        {
            var m = mesAtual.AddMonths(k);
            var (receitas, despesas) = k == 0 ? (receitasMes, despesasMes) : (mediaReceitas, mediaDespesas);
            var (fatura, parcelas) = faturas.GetValueOrDefault(m);
            meses.Add(new MesCaixaDto((short)m.Year, (byte)m.Month, Math.Round(receitas, 2), Math.Round(despesas, 2),
                Math.Round(fatura, 2), Math.Round(parcelas, 2), Math.Round(receitas - despesas - fatura, 2)));
        }

        // Saldo previsto nas contas no fim do mês, depois de pagar a fatura que ainda vence nele.
        var saldos = await _contas.CalcularSaldosAsync(ct);
        var saldoContas = idsCaixa.Sum(id => saldos.GetValueOrDefault(id));
        var saldoFimDoMes = saldoContas + (receitasMes - recebido) - (despesasMes - saido) - faturaAPagar;

        // Teste de realidade: quanto de fato sobrou no melhor dos últimos meses completos.
        var melhorMes = Enumerable.Range(0, mesesConsiderados)
            .Select(i => inicioHistorico.AddMonths(i))
            .Select(m => Receitas(m, m.AddMonths(1).AddDays(-1)) - Despesas(m, m.AddMonths(1).AddDays(-1)) - faturas.GetValueOrDefault(m).Total)
            .Max();

        return new PlanoMetasDto(true, mesesConsiderados, (short)mesAtual.Year, (byte)mesAtual.Month, margem,
            Math.Round(saldoContas, 2), Math.Round(faturaAPagar, 2), Math.Round(saldoFimDoMes, 2), Math.Round(melhorMes, 2), meses);
    }
}
