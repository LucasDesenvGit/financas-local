using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos.Cambio;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Investimentos;

/// <summary>Valores de investimento usados no patrimônio, em reais (sem consultas externas).</summary>
public record ComposicaoInvestimentos(decimal Carteira, decimal OpenFinanceRendaVariavel, decimal OpenFinanceDemais,
    decimal Reservas)
{
    /// <summary>Moedas de investimentos que ficaram fora dos totais por falta de cotação guardada.</summary>
    public IReadOnlyCollection<string> MoedasSemCotacao { get; init; } = Array.Empty<string>();

    public decimal RendaVariavel => Carteira + OpenFinanceRendaVariavel;
    public decimal Total => Carteira + OpenFinanceRendaVariavel + OpenFinanceDemais + Reservas;
}

/// <summary>
/// Carteira de renda variável (FIIs, ações, ETFs, BDRs) lançada manualmente, com preço médio,
/// valorização, proventos e comparação com a Selic. Também compara os investimentos do Open Finance com a Selic.
/// </summary>
public class CarteiraService
{
    private static readonly TimeSpan ValidadeCotacao = TimeSpan.FromHours(4);
    private static readonly string[] TiposRendaVariavelPluggy = { "EQUITY", "ETF" };

    private readonly FinancasDbContext _db;
    private readonly ContaService _contas;
    private readonly CambioService _cambio;
    private readonly ReservaService _reservas;

    public CarteiraService(FinancasDbContext db, ContaService contas, CambioService cambio, ReservaService reservas)
    {
        _db = db;
        _contas = contas;
        _cambio = cambio;
        _reservas = reservas;
    }

    /* ============================== Cálculo de posição ============================== */

    /// <summary>
    /// Resultado do processamento das operações de um ativo (método do preço médio). <see cref="Custo"/>,
    /// <see cref="PrecoMedio"/> e <see cref="LucroRealizado"/> estão em reais (cada operação pelo seu câmbio);
    /// <see cref="CustoNaMoeda"/> e <see cref="PrecoMedioNaMoeda"/> estão na moeda do ativo.
    /// </summary>
    public sealed class Posicao
    {
        public decimal Quantidade { get; set; }
        public decimal Custo { get; set; }
        public decimal CustoNaMoeda { get; set; }
        public decimal LucroRealizado { get; set; }
        public decimal PrecoMedio => Quantidade > 0 ? Custo / Quantidade : 0m;
        public decimal PrecoMedioNaMoeda => Quantidade > 0 ? CustoNaMoeda / Quantidade : 0m;
    }

    public static Posicao Calcular(IEnumerable<OperacaoAtivo> operacoes, DateOnly? ate = null)
    {
        var p = new Posicao();
        foreach (var op in operacoes.OrderBy(o => o.Data).ThenBy(o => o.Tipo).ThenBy(o => o.Id))
        {
            if (ate is DateOnly limite && op.Data > limite) break;
            if (op.Tipo == TipoOperacao.Compra)
            {
                var naMoeda = op.Quantidade * op.PrecoUnitario + op.Taxas;
                p.Custo += naMoeda * op.Cambio;
                p.CustoNaMoeda += naMoeda;
                p.Quantidade += op.Quantidade;
            }
            else if (p.Quantidade > 0)
            {
                var qtd = Math.Min(op.Quantidade, p.Quantidade);
                var medio = p.Custo / p.Quantidade;
                var medioNaMoeda = p.CustoNaMoeda / p.Quantidade;
                // Venda em reais pelo câmbio do dia da venda: o lucro inclui a variação da moeda.
                var liquido = (qtd * op.PrecoUnitario - op.Taxas) * op.Cambio;
                p.LucroRealizado += liquido - medio * qtd;
                p.Custo -= medio * qtd;
                p.CustoNaMoeda -= medioNaMoeda * qtd;
                p.Quantidade -= qtd;
                if (p.Quantidade <= 0.0000005m) { p.Quantidade = 0; p.Custo = 0; p.CustoNaMoeda = 0; }
            }
        }
        return p;
    }

    /// <summary>Valor da operação em reais, pelo câmbio da própria operação.</summary>
    private static decimal ValorOperacao(OperacaoAtivo op)
        => (op.Tipo == TipoOperacao.Compra
            ? op.Quantidade * op.PrecoUnitario + op.Taxas
            : op.Quantidade * op.PrecoUnitario - op.Taxas) * op.Cambio;

    /// <summary>Cotação atual do ativo em reais (null sem cotação do ativo ou sem cotação da moeda).</summary>
    private static decimal? PrecoAtualEmReais(Ativo a, ConversorMoeda conversor)
        => a.PrecoAtual is decimal preco && conversor.Taxa(a.Moeda) is decimal taxa ? preco * taxa : null;

    /// <summary>Quantidade em carteira ao fim de cada data-com (para calcular proventos).</summary>
    private static decimal QuantidadeEm(List<OperacaoAtivo> operacoes, DateOnly data)
        => Calcular(operacoes, data).Quantidade;

    /* ============================== Patrimônio ============================== */

    /// <summary>
    /// Valor dos investimentos para compor o patrimônio. Ativos da carteira manual usam a última cotação
    /// (ou o custo, se ainda não houver cotação). Investimentos do Open Finance com o mesmo código de um ativo
    /// da carteira não são somados de novo. Reservas estimadas pelo extrato (<see cref="ReservaService"/>) entram
    /// como renda fixa. Investimentos em moeda estrangeira são convertidos para reais;
    /// os sem cotação ficam de fora e aparecem em <see cref="ComposicaoInvestimentos.MoedasSemCotacao"/>.
    /// </summary>
    public async Task<ComposicaoInvestimentos> ComposicaoAsync(CancellationToken ct)
    {
        var ativos = await _db.Ativos.AsNoTracking().Include(a => a.Operacoes).ToListAsync(ct);
        var conversor = await _cambio.ObterConversorAsync(ct);
        decimal carteira = 0;
        var tickersEmCarteira = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        foreach (var a in ativos)
        {
            var p = Calcular(a.Operacoes);
            if (p.Quantidade <= 0) continue;
            tickersEmCarteira.Add(a.Ticker);
            // Sem cotação (do ativo ou da moeda), vale o custo em reais.
            carteira += PrecoAtualEmReais(a, conversor) is decimal preco ? p.Quantidade * preco : p.Custo;
        }

        var openFinance = await _db.Investimentos.AsNoTracking().Where(i => i.ConexaoBancaria!.Ativa)
            .Select(i => new { i.Saldo, i.Moeda, i.Codigo, i.Tipo }).ToListAsync(ct);
        decimal ofRendaVariavel = 0, ofDemais = 0;
        foreach (var i in openFinance)
        {
            if (CodigoNaCarteira(i.Codigo, tickersEmCarteira)) continue;
            if (conversor.ParaReais(i.Saldo, i.Moeda) is not decimal saldo) continue;
            if (EhRendaVariavel(i.Tipo)) ofRendaVariavel += saldo;
            else ofDemais += saldo;
        }
        var reservas = await _reservas.TotalAsync(ct);
        return new ComposicaoInvestimentos(Math.Round(carteira, 2), ofRendaVariavel, ofDemais, reservas)
        {
            MoedasSemCotacao = conversor.MoedasSemCotacao.ToList()
        };
    }

    private static bool EhRendaVariavel(string? tipoPluggy)
        => tipoPluggy is not null && TiposRendaVariavelPluggy.Contains(tipoPluggy.ToUpperInvariant());

    private static bool CodigoNaCarteira(string? codigo, HashSet<string> tickers)
        => (CotacaoService.NormalizarTicker(codigo) ?? CotacaoService.NormalizarTickerExterior(codigo)) is string t
           && tickers.Contains(t);

    /* ============================== Visão completa ============================== */

    /// <summary>Data a partir da qual a série da Selic é necessária.</summary>
    public async Task<DateOnly> InicioSerieSelicAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var inicio = new DateOnly(hoje.Year, 1, 1);
        var umAno = hoje.AddYears(-1);
        if (umAno < inicio) inicio = umAno;
        var primeiraOperacao = await _db.OperacoesAtivo.MinAsync(o => (DateOnly?)o.Data, ct);
        var primeiraAplicacao = await _db.Investimentos.Where(i => i.ConexaoBancaria!.Ativa)
            .MinAsync(i => i.DataAplicacao, ct);
        if (primeiraOperacao is DateOnly po && po < inicio) inicio = po;
        if (primeiraAplicacao is DateOnly pa && pa < inicio) inicio = pa;
        var limite = new DateOnly(2000, 1, 1);
        return inicio < limite ? limite : inicio;
    }

    public async Task<CarteiraDto> MontarAsync(SerieSelic selic, decimal? metaSelic, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var ha12Meses = hoje.AddYears(-1);

        var ativos = await _db.Ativos.AsNoTracking()
            .Include(a => a.Operacoes)
            .Include(a => a.Proventos)
            .OrderBy(a => a.Ticker)
            .ToListAsync(ct);
        // Histórico de câmbio desde a primeira operação, para converter proventos pela data-com.
        var primeiraOperacao = ativos.SelectMany(a => a.Operacoes).Select(o => (DateOnly?)o.Data).Min();
        var conversor = await _cambio.ObterConversorAsync(ct, primeiraOperacao);

        var posicoes = new List<PosicaoAtivoDto>();
        var fluxos = new List<(DateOnly Data, decimal Valor)>(); // + aplicação, − resgate (vendas e proventos)
        var tickersEmCarteira = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        DateTime? cotacaoMaisAntiga = null;
        var desatualizadas = false;

        foreach (var a in ativos)
        {
            var operacoes = a.Operacoes.OrderBy(o => o.Data).ThenBy(o => o.Id).ToList();
            var p = Calcular(operacoes);
            if (p.Quantidade > 0) tickersEmCarteira.Add(a.Ticker);

            // Proventos pela quantidade em carteira na data-com.
            decimal proventosRecebidos = 0, proventos12 = 0;
            var fluxosAtivo = operacoes
                .Select(o => (o.Data, Valor: o.Tipo == TipoOperacao.Compra ? ValorOperacao(o) : -ValorOperacao(o)))
                .ToList();
            foreach (var pr in a.Proventos.Where(x => x.DataCom <= hoje))
            {
                var qtd = QuantidadeEm(operacoes, pr.DataCom);
                if (qtd <= 0) continue;
                if (conversor.ParaReais(qtd * pr.ValorPorCota, a.Moeda, pr.DataCom) is not decimal valor) continue;
                proventosRecebidos += valor;
                if (pr.DataCom > ha12Meses) proventos12 += valor;
                fluxosAtivo.Add((pr.DataCom, -valor));
            }
            fluxos.AddRange(fluxosAtivo);

            var precoEmReais = PrecoAtualEmReais(a, conversor);
            var semCotacao = precoEmReais is null;
            var valorMercado = p.Quantidade * (precoEmReais ?? p.PrecoMedio);
            var valorizacao = valorMercado - p.Custo;
            var compras = operacoes.Where(o => o.Tipo == TipoOperacao.Compra).Sum(o => ValorOperacao(o));
            var resultado = valorizacao + p.LucroRealizado + proventosRecebidos;
            var valorSelic = fluxosAtivo.Sum(f => f.Valor * selic.Fator(f.Data, hoje));
            // Mesmo dinheiro na Selic, com os mesmos resgates: diferença entre os valores finais.
            var resultadoSelic = valorSelic - fluxosAtivo.Sum(f => f.Valor);

            var ultimo = a.Proventos.OrderByDescending(x => x.DataCom).FirstOrDefault();

            if (p.Quantidade > 0 && !a.PrecoManual)
            {
                if (a.PrecoAtualizadoEm is null || DateTime.UtcNow - a.PrecoAtualizadoEm.Value > ValidadeCotacao)
                    desatualizadas = true;
                if (a.PrecoAtualizadoEm is DateTime em && (cotacaoMaisAntiga is null || em < cotacaoMaisAntiga))
                    cotacaoMaisAntiga = em;
            }

            posicoes.Add(new PosicaoAtivoDto(
                a.Id, a.Ticker, a.Nome, a.Tipo, a.Moeda,
                p.Quantidade, Math.Round(p.PrecoMedioNaMoeda, 4), Math.Round(p.Custo, 2),
                a.PrecoAtual, precoEmReais is decimal pr2 ? Math.Round(pr2, 4) : null, a.PrecoAnterior,
                a.PrecoAtual is decimal pa && a.PrecoAnterior is decimal pant && pant > 0
                    ? Math.Round((pa / pant - 1) * 100, 2) : null,
                Math.Round(valorMercado, 2), Math.Round(valorizacao, 2),
                p.Custo > 0 ? Math.Round(valorizacao / p.Custo * 100, 2) : null,
                Math.Round(p.LucroRealizado, 2), Math.Round(proventosRecebidos, 2), Math.Round(proventos12, 2),
                valorMercado > 0 && proventos12 > 0 ? Math.Round(proventos12 / valorMercado * 100, 2) : null,
                Math.Round(resultado, 2), compras > 0 ? Math.Round(resultado / compras * 100, 2) : null,
                Math.Round(valorSelic, 2), Math.Round(valorMercado - valorSelic, 2),
                compras > 0 ? Math.Round(resultadoSelic / compras * 100, 2) : null,
                operacoes.FirstOrDefault(o => o.Tipo == TipoOperacao.Compra)?.Data,
                ultimo is null ? null : new UltimoProventoDto(ultimo.DataCom, ultimo.ValorPorCota),
                a.PrecoManual, a.PrecoAtualizadoEm, a.FonteCotacao, semCotacao,
                a.Segmento ?? (a.Tipo == TipoAtivo.Fii ? SegmentosFii.Sugerir(a.Ticker) : null)));
        }

        var abertas = posicoes.Where(p => p.Quantidade > 0).ToList();
        var custo = abertas.Sum(p => p.Custo);
        var mercado = abertas.Sum(p => p.ValorMercado);
        var comprasTotais = ativos.SelectMany(a => a.Operacoes).Where(o => o.Tipo == TipoOperacao.Compra).Sum(o => ValorOperacao(o));
        var resultadoTotal = posicoes.Sum(p => p.ResultadoTotal);
        var valorSelicTotal = posicoes.Sum(p => p.ValorSeFosseSelic);
        var resultadoSelicTotal = valorSelicTotal - fluxos.Sum(f => f.Valor);
        var totais = new TotaisRendaVariavelDto(
            mercado, custo, mercado - custo, custo > 0 ? Math.Round((mercado - custo) / custo * 100, 2) : null,
            posicoes.Sum(p => p.LucroRealizado), posicoes.Sum(p => p.ProventosRecebidos),
            posicoes.Sum(p => p.Proventos12Meses), resultadoTotal, valorSelicTotal, mercado - valorSelicTotal,
            comprasTotais > 0 ? Math.Round(resultadoTotal / comprasTotais * 100, 2) : null,
            comprasTotais > 0 ? Math.Round(resultadoSelicTotal / comprasTotais * 100, 2) : null);

        // Evolução mensal: dinheiro aplicado (líquido) x o mesmo dinheiro rendendo Selic.
        var evolucao = new List<EvolucaoCarteiraDto>();
        // Somente compras e vendas (proventos ficam de fora para a linha do "aplicado" fazer sentido).
        var fluxosAplicacao = ativos.SelectMany(a => a.Operacoes)
            .Select(o => (o.Data, Valor: o.Tipo == TipoOperacao.Compra ? ValorOperacao(o) : -ValorOperacao(o)))
            .ToList();
        if (fluxosAplicacao.Count > 0)
        {
            var primeiro = fluxosAplicacao.Min(f => f.Data);
            var mesAtual = new DateOnly(primeiro.Year, primeiro.Month, 1);
            var limiteMeses = 240;
            while (mesAtual <= hoje && limiteMeses-- > 0)
            {
                var fimMes = Relogio.UltimoDiaMes(mesAtual.Year, mesAtual.Month);
                if (fimMes > hoje) fimMes = hoje;
                var ate = fimMes;
                var aplicado = fluxosAplicacao.Where(f => f.Data <= ate).Sum(f => f.Valor);
                var naSelic = fluxosAplicacao.Where(f => f.Data <= ate).Sum(f => f.Valor * selic.Fator(f.Data, ate));
                evolucao.Add(new EvolucaoCarteiraDto(mesAtual.Year, mesAtual.Month, Math.Round(aplicado, 2),
                    Math.Round(naSelic, 2), fimMes == hoje ? mercado : null));
                mesAtual = mesAtual.AddMonths(1);
            }
        }

        // Open Finance comparado com a Selic.
        var investimentos = await _db.Investimentos.AsNoTracking().Where(i => i.ConexaoBancaria!.Ativa)
            .OrderBy(i => i.Tipo).ThenByDescending(i => i.Saldo)
            .Select(i => new
            {
                i.Id, i.Nome, i.Tipo, i.Subtipo, i.Codigo, i.Emissor, i.Moeda, i.Saldo, i.ValorAplicado, i.DataAplicacao,
                i.DataVencimento, i.TaxaAnual, i.AtualizadoEm, i.NomePersonalizado, i.ConexaoBancariaId
            })
            .ToListAsync(ct);
        var nomesConexoes = await _contas.NomesDasConexoesAsync(ct);
        var openFinance = new List<InvestimentoComparadoDto>();
        foreach (var i in investimentos)
        {
            // Rentabilidade na moeda do investimento (sem a variação do câmbio).
            decimal? rentab = i.ValorAplicado is decimal va && va > 0 ? Math.Round((i.Saldo / va - 1) * 100, 2) : null;
            decimal? selicPeriodo = null, percentual = null;
            if (i.DataAplicacao is DateOnly da && da < hoje && !selic.Vazia)
            {
                selicPeriodo = Math.Round((selic.Fator(da, hoje) - 1) * 100, 2);
                if (rentab is decimal r && selicPeriodo > 0.05m) percentual = Math.Round(r / selicPeriodo.Value * 100, 1);
            }
            openFinance.Add(new InvestimentoComparadoDto(i.Id, i.Nome, i.Tipo, i.Subtipo, i.Codigo, i.Emissor,
                nomesConexoes.GetValueOrDefault(i.ConexaoBancariaId), i.Moeda, i.Saldo, conversor.ParaReais(i.Saldo, i.Moeda), i.ValorAplicado, i.DataAplicacao,
                i.DataVencimento, i.TaxaAnual, rentab, selicPeriodo, percentual, EhRendaVariavel(i.Tipo),
                CodigoNaCarteira(i.Codigo, tickersEmCarteira), i.AtualizadoEm,
                NomeInvestimento.Exibicao(i.Nome, i.Tipo, i.Emissor, i.NomePersonalizado), i.NomePersonalizado));
        }
        // Totais em reais: investimento em moeda sem cotação fica de fora (avisado em MoedasSemCotacao).
        var totalOpenFinance = openFinance.Where(i => !i.JaNaCarteira).Sum(i => i.SaldoEmReais ?? 0m);

        // Contas do tipo "Investimento" cadastradas manualmente.
        var contasInvestimento = await _db.Contas.AsNoTracking()
            .Where(c => !c.Arquivada && c.IncluirNoSaldo && c.Tipo == TipoConta.Investimento)
            .Select(c => c.Id).ToListAsync(ct);
        decimal totalManuais = 0;
        if (contasInvestimento.Count > 0)
        {
            var saldos = await _contas.CalcularSaldosAsync(ct);
            totalManuais = contasInvestimento.Sum(id => saldos.GetValueOrDefault(id));
        }

        // Distribuição por classe.
        var distribuicao = new List<DistribuicaoInvestimentoDto>();
        foreach (var grupo in abertas.GroupBy(p => p.Tipo))
            distribuicao.Add(new DistribuicaoInvestimentoDto(NomeTipo(grupo.Key), grupo.Sum(p => p.ValorMercado)));
        foreach (var grupo in openFinance.Where(i => !i.JaNaCarteira).GroupBy(i => NomeTipoPluggy(i.Tipo)))
        {
            var existente = distribuicao.FindIndex(d => d.Nome == grupo.Key);
            var soma = grupo.Sum(i => i.SaldoEmReais ?? 0m);
            if (existente >= 0) distribuicao[existente] = distribuicao[existente] with { Valor = distribuicao[existente].Valor + soma };
            else distribuicao.Add(new DistribuicaoInvestimentoDto(grupo.Key, soma));
        }
        if (totalManuais != 0) distribuicao.Add(new DistribuicaoInvestimentoDto("Contas de investimento", totalManuais));

        // Reservas ("cofrinhos") que a instituição não informa pelo Open Finance, estimadas pelo extrato.
        var reservas = await _reservas.ListarAsync(ct);
        var totalReservas = reservas.Sum(r => r.Saldo);
        if (totalReservas != 0) distribuicao.Add(new DistribuicaoInvestimentoDto("Reservas", totalReservas));
        distribuicao = distribuicao.Where(d => d.Valor > 0).OrderByDescending(d => d.Valor).ToList();

        var resumoSelic = new SelicResumoDto(metaSelic,
            selic.Vazia ? null : Math.Round((selic.Fator(ha12Meses, hoje) - 1) * 100, 2),
            selic.Vazia ? null : Math.Round((selic.Fator(new DateOnly(hoje.Year, 1, 1), hoje) - 1) * 100, 2),
            selic.UltimaData);

        // Cotas na corretora × na carteira: "Conferir investimentos" mostra a diferença e a correção.
        var naCorretora = ConferenciaInvestimentosService.CotasNaCorretora(await _db.Investimentos.AsNoTracking()
            .Where(i => i.ConexaoBancaria!.Ativa && (i.Tipo == "EQUITY" || i.Tipo == "ETF")).ToListAsync(ct));
        var cotasDiferentes = naCorretora
            .Where(c => posicoes.FirstOrDefault(p => p.Ticker == c.Key) is { } p
                        && Math.Abs(p.Quantidade - c.Value.Quantidade) >= 0.000001m)
            .Select(c => c.Key).OrderBy(t => t, StringComparer.Ordinal).ToList();

        return new CarteiraDto(resumoSelic, totais,
            posicoes.OrderByDescending(p => p.Quantidade > 0).ThenByDescending(p => p.ValorMercado).ToList(),
            evolucao, openFinance, totalOpenFinance, totalManuais, mercado + totalOpenFinance + totalManuais + totalReservas,
            distribuicao, cotacaoMaisAntiga, desatualizadas,
            openFinance.Count(i => i.RendaVariavel && !i.JaNaCarteira && CotacaoService.NormalizarTicker(i.Codigo ?? i.Nome) is not null),
            conversor.MoedasSemCotacao.ToList(), reservas, totalReservas, cotasDiferentes);
    }

    /// <summary>Operações de um ativo, cada compra comparada com o preço atual e com a Selic.</summary>
    public async Task<List<OperacaoAtivoDto>?> OperacoesAsync(int ativoId, SerieSelic selic, CancellationToken ct)
    {
        var ativo = await _db.Ativos.AsNoTracking().Include(a => a.Operacoes)
            .FirstOrDefaultAsync(a => a.Id == ativoId, ct);
        if (ativo is null) return null;
        var hoje = Relogio.Hoje;
        var precoEmReais = PrecoAtualEmReais(ativo, await _cambio.ObterConversorAsync(ct));
        return ativo.Operacoes.OrderByDescending(o => o.Data).ThenByDescending(o => o.Id)
            .Select(o =>
            {
                var valor = ValorOperacao(o);
                var compra = o.Tipo == TipoOperacao.Compra;
                // Variação do preço na moeda do ativo; o "vale hoje" é em reais, com o câmbio atual.
                decimal? variacao = compra && ativo.PrecoAtual is decimal pa && o.PrecoUnitario > 0
                    ? Math.Round((pa / o.PrecoUnitario - 1) * 100, 2) : null;
                decimal? valorAtual = compra && precoEmReais is decimal pr ? Math.Round(o.Quantidade * pr, 2) : null;
                return new OperacaoAtivoDto(o.Id, o.AtivoId, ativo.Ticker, o.Tipo, o.Data, o.Quantidade, o.PrecoUnitario,
                    o.Taxas, ativo.Moeda, o.Cambio, Math.Round(valor, 2), o.Observacao, ativo.PrecoAtual, variacao, valorAtual,
                    Math.Round(valor * selic.Fator(o.Data, hoje), 2), o.Origem);
            })
            .ToList();
    }

    public static string NomeTipo(TipoAtivo tipo) => tipo switch
    {
        TipoAtivo.Fii => "Fundos imobiliários",
        TipoAtivo.Acao => "Ações",
        TipoAtivo.Etf => "ETFs",
        TipoAtivo.Bdr => "BDRs",
        _ => "Outros (renda variável)"
    };

    private static string NomeTipoPluggy(string? tipo) => tipo?.ToUpperInvariant() switch
    {
        "FIXED_INCOME" => "Renda fixa",
        "SECURITY" => "Previdência",
        "MUTUAL_FUND" => "Fundos de investimento",
        "EQUITY" => "Ações",
        "ETF" => "ETFs",
        "COE" => "COE",
        _ => "Outros investimentos"
    };
}
