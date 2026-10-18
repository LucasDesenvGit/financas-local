using Financas.Api.Dtos;

namespace Financas.Api.Servicos;

/// <summary>
/// Assistente de metas (#90): quanto guardar este mês em cada meta pela sobra prevista (receitas − despesas), e não
/// pela divisão igual. A sobra de cada mês é a mesma da tela de Projeção (<see cref="RelatorioService.ProjecaoAsync"/>):
/// mês atual "em andamento" e, nos seguintes, média de receitas − (média de despesas à vista + parcelas do mês).
///
/// Regra, mês a mês, a partir do atual:
///  1. capacidade = max(0, sobra × (1 − margem de segurança));
///  2. cada meta com prazo pede o que falta × capacidade do mês ÷ capacidade somada até o prazo — mês apertado pede
///     menos, mês folgado pede mais. Como o pedido é recalculado todo mês sobre o que ainda falta, o que não coube
///     num mês vai para os seguintes;
///  3. se a capacidade cobre todos os pedidos, todas as metas com prazo recebem o pedido, e o que sobra vai para as
///     metas sem prazo (ou de prazo vencido) pela ordem de prioridade;
///  4. se não cobre, decide a prioridade (1ª primeiro): meta com prazo recebe até o pedido; meta sem prazo, até uma
///     parte justa (capacidade ÷ metas em aberto) — para uma meta sem prazo de 1ª prioridade não ficar sem nada;
///  5. nenhuma meta com prazo recebe mais que o pedido do mês, e nada é negativo.
/// A prioridade só decide quando falta dinheiro (#90, comentário do usuário). Julgamento próprio: a parte justa das
/// metas sem prazo no aperto — não validado com uso real.
/// </summary>
public class AssistenteMetasService
{
    public const int MinimoMesesHistorico = 3;
    /// <summary>Até onde a data provável é procurada (meta sem prazo ou com capacidade baixa).</summary>
    private const int HorizonteMeses = 120;
    private const int MesesNoPlano = 12;

    private readonly RelatorioService _relatorios;
    private readonly ParametroService _parametros;

    public AssistenteMetasService(RelatorioService relatorios, ParametroService parametros)
    {
        _relatorios = relatorios;
        _parametros = parametros;
    }

    /// <param name="metas">Metas já com o progresso calculado, na ordem de prioridade (1ª primeiro).</param>
    public async Task<PlanoMetasDto> PlanejarAsync(IReadOnlyList<MetaDto> metas, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var mesAtual = new DateOnly(hoje.Year, hoje.Month, 1);
        var margem = await _parametros.ObterInteiroAsync(ConfiguracaoService.MetasMargemSeguranca, ct);
        var ativas = metas.Where(m => !m.Concluida && m.ValorAlvo - m.ValorAtual > 0).ToList();

        var projecao = await _relatorios.ProjecaoAsync(HorizonteMeses, ct);
        if (projecao.MesesConsiderados < MinimoMesesHistorico)
        {
            return new PlanoMetasDto(false, projecao.MesesConsiderados, (short)mesAtual.Year, (byte)mesAtual.Month,
                0, margem, 0, 0, 0, false, ativas.Select(m => new PlanoMetaDto(m.Id, m.ValorAlvo - m.ValorAtual, null,
                    null, m.AporteMensalSugerido, null, null, null, [])).ToList());
        }

        var futuros = projecao.Meses.Where(m => m.Situacao != "realizado").ToList();
        var n = futuros.Count;
        var sobra = futuros.Select(m => m.Resultado).ToArray();
        var capacidade = sobra.Select(s => Math.Max(0m, s * (100 - margem) / 100m)).ToArray();
        var restante = capacidade.ToArray();

        var falta = ativas.ToDictionary(m => m.Id, m => m.ValorAlvo - m.ValorAtual);
        var faltaInicial = new Dictionary<int, decimal>(falta);
        var alocado = ativas.ToDictionary(m => m.Id, _ => new decimal[n]);
        var pedidoMesAtual = new Dictionary<int, decimal>();
        int? IndicePrazo(DateOnly? alvo) => alvo is { } a ? (a.Year - mesAtual.Year) * 12 + a.Month - mesAtual.Month : null;

        for (var k = 0; k < n; k++)
        {
            // Pedido do mês de cada meta com prazo (pela capacidade dos meses até o prazo).
            var pedidos = new Dictionary<int, decimal>();
            foreach (var m in ativas)
            {
                var f = falta[m.Id];
                if (f <= 0 || IndicePrazo(m.DataAlvo) is not int prazo || prazo < k) continue;
                var ate = Math.Min(prazo, n - 1);
                var soma = 0m;
                for (var i = k; i <= ate; i++) soma += capacidade[i];
                pedidos[m.Id] = Math.Min(f, soma > 0 ? f * capacidade[k] / soma : (k == ate ? f : 0m));
                if (k == 0) pedidoMesAtual[m.Id] = pedidos[m.Id];
            }
            // Passo 1: com folga, todas as com prazo recebem o pedido; no aperto, decide a prioridade (sem prazo leva
            // até uma parte justa).
            var folga = restante[k] >= pedidos.Values.Sum();
            var abertas = ativas.Count(m => falta[m.Id] > 0);
            var parteJusta = abertas > 0 ? capacidade[k] / abertas : 0m;
            foreach (var m in ativas)
            {
                var f = falta[m.Id];
                if (f <= 0) continue;
                var limite = pedidos.TryGetValue(m.Id, out var ped) ? ped : folga ? 0m : parteJusta;
                var dar = Math.Min(Math.Min(limite, restante[k]), f);
                alocado[m.Id][k] += dar;
                restante[k] -= dar;
                falta[m.Id] -= dar;
            }
            // Passo 2: o que sobrou do mês vai para as metas sem prazo (ou de prazo vencido), pela prioridade.
            foreach (var m in ativas)
            {
                if (restante[k] <= 0) break;
                if (falta[m.Id] <= 0 || IndicePrazo(m.DataAlvo) is int p && p >= k) continue;
                var dar = Math.Min(restante[k], falta[m.Id]);
                alocado[m.Id][k] += dar;
                restante[k] -= dar;
                falta[m.Id] -= dar;
            }
        }

        var planos = ativas.Select(m =>
        {
            var a = alocado[m.Id];
            DateOnly? provavel = null;
            var acumulado = 0m;
            for (var k = 0; k < n; k++)
            {
                acumulado += a[k];
                if (acumulado >= faltaInicial[m.Id] - 0.005m) { provavel = mesAtual.AddMonths(k); break; }
            }
            var prazo = IndicePrazo(m.DataAlvo);
            bool? noPrazo = prazo is int p ? provavel is { } d && (d.Year - mesAtual.Year) * 12 + d.Month - mesAtual.Month <= p : null;
            decimal? manterPrazo = noPrazo == false && prazo is int pr && pr >= 1
                ? Math.Round((faltaInicial[m.Id] - a[0]) / pr, 2)
                : null;
            var plano = Enumerable.Range(0, Math.Min(MesesNoPlano, n)).Select(k => new PlanoMesMetaDto(
                (short)futuros[k].Ano, (byte)futuros[k].Mes, Math.Round(sobra[k], 2), futuros[k].Parcelas, Math.Round(a[k], 2))).ToList();
            return new PlanoMetaDto(m.Id, Math.Round(faltaInicial[m.Id], 2), Math.Round(a[0], 2),
                pedidoMesAtual.TryGetValue(m.Id, out var ped) ? Math.Round(ped, 2) : null, m.AporteMensalSugerido,
                provavel, noPrazo, manterPrazo, plano);
        }).ToList();

        return new PlanoMetasDto(true, projecao.MesesConsiderados, (short)mesAtual.Year, (byte)mesAtual.Month,
            Math.Round(sobra[0], 2), margem, Math.Round(capacidade[0], 2),
            Math.Round(planos.Sum(p => p.PedidoMes ?? 0m), 2), Math.Round(planos.Sum(p => p.Recomendado ?? 0m), 2),
            sobra[0] <= 0, planos);
    }
}
