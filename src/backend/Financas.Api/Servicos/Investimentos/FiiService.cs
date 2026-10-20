using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Investimentos;

/// <summary>
/// Análise dos fundos imobiliários: renda (DY, yield on cost), cada compra comparada com hoje, proventos por mês,
/// evolução do patrimônio e rentabilidade comparada com Selic e IFIX.
/// </summary>
public class FiiService
{
    private const int MesesProventos = 24;
    private const int MaximoMeses = 180;
    /// <summary>#173 c): o provento cai até tantos dias depois da data-com.</summary>
    private const int DiasPagamentoProvento = 40;
    /// <summary>#173 c): diferença aceita entre o crédito e cotas × valor por cota (arredondamento do banco).</summary>
    private const decimal ToleranciaProvento = 0.05m;

    private readonly FinancasDbContext _db;
    private readonly ExtratoCarteiraService _extrato;
    private readonly ConferenciaInvestimentosService _conferencia;

    public FiiService(FinancasDbContext db, ExtratoCarteiraService extrato, ConferenciaInvestimentosService conferencia)
    {
        _db = db;
        _extrato = extrato;
        _conferencia = conferencia;
    }

    /// <summary>
    /// #173 c): de que fundo é cada crédito de provento. O crédito com o código do fundo já diz. O "Valor recebido de
    /// Investimentos" (sem código) é ligado ao provento cotas na data-com × valor por cota que dá o mesmo valor (até
    /// <see cref="ToleranciaProvento"/>), pago até <see cref="DiasPagamentoProvento"/> dias depois da data-com. Cada provento
    /// esperado serve a um crédito só. O que não liga fica sem fundo (null), sem sumir da soma do mês.
    /// </summary>
    private static Dictionary<long, string?> FundoDeCadaCredito(List<Fundo> fundos,
        List<(long Id, string Ticker, DateOnly Data, decimal Valor)> comCodigo, List<(long Id, DateOnly Data, decimal Valor)> creditos)
    {
        var esperados = fundos.SelectMany(f => f.Proventos.Select(p => (Ticker: f.Ativo.Ticker, p.DataCom,
                Valor: Math.Round(f.QuantidadeEm(p.DataCom) * p.ValorPorCota, 2))))
            .Where(e => e.Valor > 0).ToList();
        var usados = new HashSet<int>();
        var resultado = new Dictionary<long, string?>();
        int? Melhor(DateOnly data, decimal valor, string? ticker) => esperados
            .Select((e, i) => (e, i))
            .Where(x => !usados.Contains(x.i) && (ticker is null || x.e.Ticker == ticker)
                        && x.e.DataCom <= data && data <= x.e.DataCom.AddDays(DiasPagamentoProvento)
                        && Math.Abs(x.e.Valor - valor) <= ToleranciaProvento)
            .OrderBy(x => Math.Abs(x.e.Valor - valor)).ThenBy(x => data.DayNumber - x.e.DataCom.DayNumber)
            .Select(x => (int?)x.i).FirstOrDefault();

        // Primeiro os que já dizem o fundo, para os sem código não pegarem o provento deles.
        foreach (var c in comCodigo.OrderBy(c => c.Data))
        {
            resultado[c.Id] = c.Ticker;
            if (Melhor(c.Data, c.Valor, c.Ticker) is int i) usados.Add(i);
        }
        foreach (var c in creditos.Where(c => !resultado.ContainsKey(c.Id)).OrderBy(c => c.Data))
        {
            var i = Melhor(c.Data, c.Valor, null);
            if (i is int achado) usados.Add(achado);
            resultado[c.Id] = i is int j ? esperados[j].Ticker : null;
        }
        return resultado;
    }

    private sealed class Fundo
    {
        public required Ativo Ativo { get; init; }
        public required List<OperacaoAtivo> Operacoes { get; init; }
        public required List<ProventoAtivo> Proventos { get; init; }
        public required SortedDictionary<DateOnly, decimal> Mensais { get; init; }
        public CarteiraService.Posicao Posicao { get; set; } = new();

        public decimal QuantidadeEm(DateOnly data) => CarteiraService.Calcular(Operacoes, data).Quantidade;

        /// <summary>Preço da cota no mês (fechamento mensal; no mês atual, a cotação atual).</summary>
        public decimal PrecoNoMes(DateOnly mes, DateOnly mesAtual)
        {
            if (mes == mesAtual && Ativo.PrecoAtual is decimal atual) return atual;
            if (Mensais.TryGetValue(mes, out var f)) return f;
            var anterior = Mensais.Where(m => m.Key <= mes).Select(m => (decimal?)m.Value).LastOrDefault();
            if (anterior is decimal a) return a;
            var medio = CarteiraService.Calcular(Operacoes, Ultimo(mes)).PrecoMedio;
            return medio > 0 ? medio : Ativo.PrecoAtual ?? 0m;
        }
    }

    private static DateOnly Ultimo(DateOnly mes) => Relogio.UltimoDiaMes(mes.Year, mes.Month);

    public async Task<PainelFiisDto> MontarAsync(SerieSelic selic, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var ha12 = hoje.AddYears(-1);
        var mesAtual = new DateOnly(hoje.Year, hoje.Month, 1);

        var ativos = await _db.Ativos.AsNoTracking()
            .Where(a => a.Tipo == TipoAtivo.Fii)
            .Include(a => a.Operacoes)
            .Include(a => a.Proventos)
            .Include(a => a.PrecosMensais)
            .ToListAsync(ct);

        var fundos = ativos.Where(a => a.Operacoes.Count > 0).Select(a => new Fundo
        {
            Ativo = a,
            Operacoes = a.Operacoes.OrderBy(o => o.Data).ThenBy(o => o.Tipo).ThenBy(o => o.Id).ToList(),
            Proventos = a.Proventos.Where(p => p.DataCom <= hoje).OrderBy(p => p.DataCom).ToList(),
            Mensais = new SortedDictionary<DateOnly, decimal>(a.PrecosMensais.ToDictionary(p => p.Mes, p => p.Fechamento))
        }).ToList();
        foreach (var f in fundos) f.Posicao = CarteiraService.Calcular(f.Operacoes);

        var tickers = fundos.Select(f => f.Ativo.Ticker).ToHashSet();
        var recebidos = (await _extrato.ProventosRecebidosAsync(ct)).Where(r => tickers.Contains(r.Ticker)).ToList();
        // #173: provento recebido no mês = crédito com o código do fundo ou crédito da corretora que não é troco nem
        // venda ("Valor recebido de Investimentos", sem código). O mesmo crédito conta uma vez só.
        var creditosProvento = recebidos.Select(r => (r.Id, r.Data, r.Valor))
            .Concat(fundos.Count > 0 ? await _conferencia.ProventosDaCorretoraAsync(ct) : [])
            .DistinctBy(r => r.Id).ToList();

        var fundoDoCredito = FundoDeCadaCredito(fundos, recebidos, creditosProvento);

        var abertos = fundos.Where(f => f.Posicao.Quantidade > 0).ToList();
        var totalMercado = abertos.Sum(f => f.Posicao.Quantidade * (f.Ativo.PrecoAtual ?? f.Posicao.PrecoMedio));

        /* ---------------- por fundo ---------------- */
        var lista = new List<FiiDto>();
        foreach (var f in abertos)
        {
            var a = f.Ativo;
            var p = f.Posicao;
            var preco = a.PrecoAtual;
            var valorMercado = p.Quantidade * (preco ?? p.PrecoMedio);
            var ultimo = f.Proventos.LastOrDefault();
            var porCota12 = f.Proventos.Where(x => x.DataCom > ha12).Sum(x => x.ValorPorCota);
            decimal proventos12 = 0, proventosTotal = 0;
            foreach (var pr in f.Proventos)
            {
                var valor = f.QuantidadeEm(pr.DataCom) * pr.ValorPorCota;
                proventosTotal += valor;
                if (pr.DataCom > ha12) proventos12 += valor;
            }
            var extrato12 = recebidos.Where(r => r.Ticker == a.Ticker && r.Data > ha12).Sum(r => r.Valor);
            var segmento = a.Segmento ?? SegmentosFii.Sugerir(a.Ticker);

            lista.Add(new FiiDto(a.Id, a.Ticker, a.Nome, segmento, a.Segmento is null && segmento is not null,
                p.Quantidade, Math.Round(p.PrecoMedio, 4), Math.Round(p.Custo, 2), preco,
                preco is decimal pa && a.PrecoAnterior is decimal pant && pant > 0 ? Math.Round((pa / pant - 1) * 100, 2) : null,
                Math.Round(valorMercado, 2), Math.Round(valorMercado - p.Custo, 2),
                p.Custo > 0 ? Math.Round((valorMercado - p.Custo) / p.Custo * 100, 2) : null,
                ultimo?.ValorPorCota, ultimo?.DataCom, Math.Round(porCota12, 6),
                Pct(ultimo?.ValorPorCota, preco), Pct(porCota12, preco), Pct(porCota12, p.PrecoMedio),
                Pct(ultimo?.ValorPorCota, p.PrecoMedio),
                Math.Round((ultimo?.ValorPorCota ?? 0) * p.Quantidade, 2), Math.Round(proventos12, 2),
                Math.Round(proventosTotal, 2), Math.Round(extrato12, 2),
                totalMercado > 0 ? Math.Round(valorMercado / totalMercado * 100, 2) : 0,
                a.FonteCotacao, a.PrecoManual));
        }
        lista = lista.OrderByDescending(x => x.ValorMercado).ToList();

        /* ---------------- cada compra x hoje ---------------- */
        var lotes = new List<LoteFiiDto>();
        foreach (var f in fundos)
        {
            var preco = f.Ativo.PrecoAtual;
            var porCota12 = f.Proventos.Where(x => x.DataCom > ha12).Sum(x => x.ValorPorCota);
            var ultimo = f.Proventos.LastOrDefault()?.ValorPorCota ?? 0;
            var temHistorico = f.Proventos.Count > 0;
            foreach (var o in f.Operacoes.Where(x => x.Tipo == TipoOperacao.Compra))
            {
                // Renda dos 12 meses anteriores à compra, sobre o preço pago (o "DY que você comprou").
                var antes = f.Proventos.Where(x => x.DataCom <= o.Data && x.DataCom > o.Data.AddYears(-1)).ToList();
                decimal? dyNaCompra = temHistorico && f.Proventos[0].DataCom <= o.Data.AddMonths(-11) && antes.Count > 0
                    ? Pct(antes.Sum(x => x.ValorPorCota), o.PrecoUnitario) : null;
                var recebidosLote = f.Proventos.Where(x => x.DataCom >= o.Data).Sum(x => x.ValorPorCota) * o.Quantidade;
                var valorPago = o.Quantidade * o.PrecoUnitario + o.Taxas;
                lotes.Add(new LoteFiiDto(o.Id, f.Ativo.Id, f.Ativo.Ticker, o.Data, o.Quantidade, o.PrecoUnitario,
                    Math.Round(valorPago, 2),
                    preco, preco is decimal pa ? Math.Round(o.Quantidade * pa, 2) : null,
                    preco is decimal pb && o.PrecoUnitario > 0 ? Math.Round((pb / o.PrecoUnitario - 1) * 100, 2) : null,
                    dyNaCompra, Pct(porCota12, o.PrecoUnitario), Pct(porCota12, preco),
                    Math.Round(ultimo * o.Quantidade, 2), Math.Round(recebidosLote, 2), o.Origem));
            }
        }
        lotes = lotes.OrderByDescending(l => l.Data).ThenBy(l => l.Ticker).ToList();

        /* ---------------- proventos por mês (pela data-com) ---------------- */
        var proventosMes = new List<ProventoMesDto>();
        var primeiraData = fundos.Count > 0 ? fundos.Min(f => f.Operacoes[0].Data) : hoje;
        var inicioProventos = new DateOnly(primeiraData.Year, primeiraData.Month, 1);
        var limite = mesAtual.AddMonths(-(MesesProventos - 1));
        if (inicioProventos < limite) inicioProventos = limite;
        for (var mes = inicioProventos; mes <= mesAtual; mes = mes.AddMonths(1))
        {
            var fim = Ultimo(mes);
            var porAtivo = new List<ValorAtivoDto>();
            foreach (var f in fundos)
            {
                var soma = f.Proventos.Where(x => x.DataCom >= mes && x.DataCom <= fim)
                    .Sum(x => f.QuantidadeEm(x.DataCom) * x.ValorPorCota);
                if (soma > 0) porAtivo.Add(new ValorAtivoDto(f.Ativo.Ticker, Math.Round(soma, 2)));
            }
            var extrato = recebidos.Where(r => r.Data >= mes && r.Data <= fim).Sum(r => r.Valor);
            proventosMes.Add(new ProventoMesDto(mes.Year, mes.Month, porAtivo.Sum(x => x.Valor), Math.Round(extrato, 2),
                porAtivo.OrderByDescending(x => x.Valor).ToList()));
        }

        /* ---------------- evolução e rentabilidade (mensal) ---------------- */
        var evolucao = new List<EvolucaoFiiMesDto>();
        var rentabilidade = new List<RentabilidadeMesDto>();
        var mesAMes = new List<MesAMesFiiDto>();
        var ifix = await _db.IndicesMensais.AsNoTracking().Where(i => i.Codigo == CotacaoService.CodigoIfix)
            .ToDictionaryAsync(i => i.Mes, i => i.Fechamento, ct);
        if (fundos.Count > 0)
        {
            var inicio = new DateOnly(primeiraData.Year, primeiraData.Month, 1);
            if (inicio < mesAtual.AddMonths(-MaximoMeses)) inicio = mesAtual.AddMonths(-MaximoMeses);
            var baseIfix = ifix.GetValueOrDefault(inicio.AddMonths(-1));
            if (baseIfix <= 0) baseIfix = ifix.GetValueOrDefault(inicio);

            decimal acumulado = 1m, valorAnterior = 0m;
            var inicioSelic = primeiraData;
            for (var mes = inicio; mes <= mesAtual; mes = mes.AddMonths(1))
            {
                var fim = Ultimo(mes);
                if (fim > hoje) fim = hoje;
                decimal aplicado = 0, valor = 0, fluxo = 0, proventos = 0, aportes = 0, subscricoes = 0, resgates = 0;
                var idsAportes = new HashSet<long>();
                foreach (var f in fundos)
                {
                    var posicao = CarteiraService.Calcular(f.Operacoes, fim);
                    aplicado += posicao.Custo;
                    if (posicao.Quantidade > 0) valor += posicao.Quantidade * f.PrecoNoMes(mes, mesAtual);
                    var subscricao = ClassificacaoAtivo.EhSubscricao(f.Ativo.Ticker);
                    foreach (var o in f.Operacoes.Where(x => x.Data >= mes && x.Data <= fim))
                    {
                        if (o.TransacaoId is long transacaoId) idsAportes.Add(transacaoId);
                        // #173 a): cotas × preço, sem a taxa da B3, como as "Entradas" do app (a taxa fica no custo e no
                        // preço médio da operação, o "valor final da compra").
                        if (o.Tipo == TipoOperacao.Compra)
                        {
                            var pago = o.Quantidade * o.PrecoUnitario;
                            fluxo += pago;
                            if (subscricao) subscricoes += pago; else aportes += pago;
                        }
                        else
                        {
                            var recebido = o.Quantidade * o.PrecoUnitario;
                            fluxo -= recebido;
                            resgates += recebido;
                        }
                    }
                    proventos += f.Proventos.Where(x => x.DataCom >= mes && x.DataCom <= fim)
                        .Sum(x => f.QuantidadeEm(x.DataCom) * x.ValorPorCota);
                }

                // Rentabilidade do mês ponderada no tempo (aportes no início do mês), proventos somados ao retorno.
                var basePeriodo = valorAnterior + fluxo;
                var retorno = basePeriodo > 0 ? (valor + proventos - basePeriodo) / basePeriodo : 0m;
                if (retorno < -0.95m) retorno = -0.95m;
                acumulado *= 1 + retorno;
                var mesFim = Ultimo(mes);
                var creditosDoMes = creditosProvento.Where(r => r.Data >= mes && r.Data <= mesFim).ToList();
                var porFii = creditosDoMes.GroupBy(r => fundoDoCredito.GetValueOrDefault(r.Id))
                    .Select(g => new RendimentoPorFiiDto(g.Key, Math.Round(g.Sum(r => r.Valor), 2)))
                    .OrderBy(r => r.Ticker is null).ThenByDescending(r => r.Valor).ToList();
                mesAMes.Add(new MesAMesFiiDto(mes.Year, mes.Month, Math.Round(aportes, 2), Math.Round(subscricoes, 2),
                    Math.Round(resgates, 2), Math.Round(creditosDoMes.Sum(r => r.Valor), 2),
                    Math.Round(proventos, 2), Math.Round(valor - basePeriodo, 2), Math.Round(valor, 2), porFii,
                    [.. idsAportes.Order()], [.. creditosDoMes.Select(r => r.Id).Distinct().Order()]));
                valorAnterior = valor;

                var ateSelic = fim == hoje ? hoje : fim.AddDays(1);
                var selicAcumulada = (selic.Fator(inicioSelic, ateSelic) - 1) * 100;
                decimal? ifixAcumulado = baseIfix > 0 && ifix.TryGetValue(mes, out var fi)
                    ? Math.Round((fi / baseIfix - 1) * 100, 2) : null;

                evolucao.Add(new EvolucaoFiiMesDto(mes.Year, mes.Month, Math.Round(aplicado, 2), Math.Round(valor, 2)));
                rentabilidade.Add(new RentabilidadeMesDto(mes.Year, mes.Month, Math.Round((acumulado - 1) * 100, 2),
                    Math.Round(selicAcumulada, 2), ifixAcumulado));
            }
        }

        /* ---------------- segmentos e resumo ---------------- */
        var segmentos = lista.GroupBy(x => x.Segmento ?? "Sem segmento")
            .Select(g => new DistribuicaoInvestimentoDto(g.Key, g.Sum(x => x.ValorMercado)))
            .OrderByDescending(x => x.Valor).ToList();

        var custo = lista.Sum(x => x.Custo);
        var mercado = lista.Sum(x => x.ValorMercado);
        var renda12 = fundos.Sum(f => f.Proventos.Where(x => x.DataCom > ha12).Sum(x => f.QuantidadeEm(x.DataCom) * x.ValorPorCota));
        var porCotaPonderado = lista.Sum(x => x.RendimentosPorCota12Meses * x.Quantidade);
        var mesesComRenda = Math.Min(12, Math.Max(1, (hoje.DayNumber - primeiraData.DayNumber) / 30));
        var ultimaRent = rentabilidade.LastOrDefault();

        var resumo = new ResumoFiisDto(mercado, custo, mercado - custo,
            custo > 0 ? Math.Round((mercado - custo) / custo * 100, 2) : null,
            Math.Round(renda12, 2), Math.Round(renda12 / mesesComRenda, 2), lista.Sum(x => x.RendaMensalEstimada),
            Pct(porCotaPonderado, mercado), Pct(porCotaPonderado, custo),
            Math.Round(lista.Sum(x => x.RecebidoExtrato12Meses), 2),
            ultimaRent?.Carteira, ultimaRent?.Selic, ultimaRent?.Ifix, lista.Count);

        // Histórico de cotação (36 meses) para comparar com o preço de cada compra.
        var inicioHistorico = mesAtual.AddMonths(-35);
        var historicos = fundos.Select(f =>
        {
            var pontos = f.Mensais.Where(m => m.Key >= inicioHistorico && m.Key < mesAtual)
                .Select(m => new PontoPrecoDto(m.Key.Year, m.Key.Month, m.Value)).ToList();
            var atual = f.Ativo.PrecoAtual ?? f.Mensais.GetValueOrDefault(mesAtual);
            if (atual > 0) pontos.Add(new PontoPrecoDto(mesAtual.Year, mesAtual.Month, atual));
            return new HistoricoPrecoDto(f.Ativo.Ticker, pontos);
        }).Where(h => h.Pontos.Count > 0).ToList();

        return new PainelFiisDto(resumo, lista, lotes, proventosMes, evolucao, rentabilidade, segmentos, historicos,
            SegmentosFii.Todos.ToList(), mesAMes);
    }

    private static decimal? Pct(decimal? parte, decimal? total)
        => parte is decimal p && total is decimal t && t > 0 ? Math.Round(p / t * 100, 2) : null;
}
