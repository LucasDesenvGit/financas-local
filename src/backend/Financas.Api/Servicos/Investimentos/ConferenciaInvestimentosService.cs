using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos.Cambio;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Caching.Memory;
using System.Text.Json;

namespace Financas.Api.Servicos.Investimentos;

/// <summary>
/// "Conferir investimentos" (ADR 0027): procura o que costuma sair errado com a renda variável que chega pelo
/// Open Finance e sugere a correção. Nada muda sem você confirmar, e a correção pode ser desfeita por alguns minutos.
/// <list type="bullet">
/// <item>FII cadastrado como ação ou "outro": o nome do fundo ou o emissor informado pela corretora diz que é FII.</item>
/// <item>Cotas que faltam em relação à posição da corretora: a corretora pode deixar uma compra fora do histórico.
/// O débito de compra do extrato que nenhuma operação explica é sugerido para lançar a compra que faltou.</item>
/// <item>Créditos da corretora: o Nubank usa a mesma descrição ("Valor recebido de Investimentos") para o troco das
/// compras (debita um pouco a mais e devolve a sobra), para o valor de uma venda e para os proventos. Troco e venda
/// vão para a categoria da compra; provento conta como receita.</item>
/// <item>#173 a): taxa da B3 que falta nas compras. O débito da compra menos o troco que voltou menos cotas × preço é
/// a taxa (alguns centavos por ordem); ela entra no custo da compra, como no app da corretora.</item>
/// </list>
/// #173 b) (ADR 0044): depois da sincronização, o troco "sem dúvida" vai sozinho para a categoria da compra, com aviso
/// e "Desfazer" no Painel; o resto continua só com prévia e confirmação.
/// </summary>
public class ConferenciaInvestimentosService
{
    /// <summary>Troco de até 2,5% do débito (a sobra devolvida pelo Nubank fica abaixo de 2%).</summary>
    private const decimal TrocoMaximo = 0.025m;
    /// <summary>O débito da compra fica entre o custo da operação e 4% acima (troco ainda não devolvido + taxas).</summary>
    private const decimal FolgaDebito = 0.04m;
    /// <summary>A operação (data do pregão) vem até 3 dias antes ou 1 dia depois do débito no extrato.</summary>
    private const int DiasAntesDoDebito = 3;
    private const int DiasDepoisDoDebito = 1;
    /// <summary>O troco cai no dia do débito ou até 3 dias depois; a sobra de centavos, até 5 dias depois.</summary>
    private const int DiasDoTroco = 3;
    private const int DiasDaSobra = 5;
    /// <summary>O valor de uma venda cai até 5 dias depois do pregão (D+2 úteis), com até 2% (ou R$ 0,05) de diferença.</summary>
    private const int DiasDaVenda = 5;
    private const decimal FolgaVenda = 0.02m;
    /// <summary>Preço de uma compra que faltou: até 3,5% de diferença da cotação de fechamento daquele dia.</summary>
    private const decimal FolgaPreco = 0.035m;
    /// <summary>
    /// #173 a): taxa da B3 sugerida só até 1% do valor das cotas e R$ 5 (a de verdade é de centavos a cada R$ 1.000);
    /// acima disso é troco que não voltou ou outra coisa, e nada é sugerido.
    /// </summary>
    private const decimal TaxaMaxima = 0.01m;
    private const decimal TaxaMaximaReais = 5m;
    private static readonly TimeSpan PrazoDesfazer = TimeSpan.FromMinutes(10);
    /// <summary>#173 b): os trocos classificados na sincronização, para o aviso e o "Desfazer" do Painel.</summary>
    public const string ChaveTrocosAutomaticos = "Investimentos.TrocosAutomaticos";
    private const int MaximoTrocosGuardados = 500;

    private readonly FinancasDbContext _db;
    private readonly CotacaoService _cotacoes;
    private readonly CategorizacaoService _categorizacao;
    private readonly IMemoryCache _cache;
    private readonly AuditoriaService _auditoria;
    private readonly ConfiguracaoService _config;

    public ConferenciaInvestimentosService(FinancasDbContext db, CotacaoService cotacoes,
        CategorizacaoService categorizacao, IMemoryCache cache, AuditoriaService auditoria, ConfiguracaoService config)
    {
        _config = config;
        _db = db;
        _cotacoes = cotacoes;
        _categorizacao = categorizacao;
        _cache = cache;
        _auditoria = auditoria;
    }

    /// <summary>Débito ou crédito do extrato que pode ser compra de renda variável, troco, venda ou provento.</summary>
    private sealed record Movimento(long Id, int ContaId, string Conta, DateOnly Data, string Descricao,
        string? DescricaoOriginal, string? PluggyCategoriaId, decimal Valor, TipoTransacao Tipo, string? Categoria,
        bool CategoriaManual);

    /// <summary>Uma ordem de compra: uma operação ou, quando a corretora divide a execução, as do mesmo ativo e dia.</summary>
    private sealed record Ordem(List<OperacaoAtivo> Operacoes, decimal Custo);

    private sealed record CompraSemOperacao(Movimento Debito, Movimento? Troco);

    /// <summary>Crédito da corretora que não é provento: troco da <c>Compra</c> ou valor da <c>Venda</c>.</summary>
    private sealed record CreditoDeOperacao(Movimento Credito, Movimento? Compra, (string Ticker, OperacaoAtivo Op)? Venda);

    /// <summary>#173 a): a taxa da B3 de um dia de compras e a parte de cada operação.</summary>
    private sealed record TaxaDoDia(DateOnly Data, string Conta, decimal Debitado, decimal Trocos, decimal ValorDasCotas,
        decimal Taxa, List<(OperacaoAtivo Op, string Ticker, decimal Taxa)> Operacoes);

    /// <summary><c>TrocosSemDuvida</c> (#173 b): os trocos dos dias em que a conta fecha (só sobra a taxa da B3).</summary>
    private sealed record Analise(List<CompraSemOperacao> SemOperacao, List<CreditoDeOperacao> DeOperacao,
        List<Movimento> Creditos, List<TaxaDoDia> Taxas, HashSet<long> TrocosSemDuvida);

    /// <summary>#173 b): troco classificado na sincronização, com o que ele era antes (para o "Desfazer").</summary>
    private sealed record TrocoAutomatico(long Id, int? CategoriaId, bool CategoriaManual, TipoTransacao Tipo,
        bool IgnorarRelatorios, int Aplicada);

    /// <summary>Estado de antes da correção, guardado só na memória da API para o "Desfazer".</summary>
    private sealed record EstadoAnterior(List<(int AtivoId, TipoAtivo Tipo)> Tipos, List<int> OperacoesCriadas,
        List<(long Id, int? CategoriaId, bool CategoriaManual, TipoTransacao Tipo, bool IgnorarRelatorios, int? Aplicada)> Transacoes,
        List<(int OperacaoId, decimal Antes, decimal Aplicada)> Taxas);

    /* ============================== Conferir ============================== */

    public async Task<ConferenciaInvestimentosDto> ConferirAsync(CancellationToken ct)
    {
        var ativos = await _db.Ativos.AsNoTracking().Include(a => a.Operacoes).OrderBy(a => a.Ticker).ToListAsync(ct);
        var investimentos = await _db.Investimentos.AsNoTracking()
            .Where(i => i.ConexaoBancaria!.Ativa && (i.Tipo == "EQUITY" || i.Tipo == "ETF"))
            .ToListAsync(ct);

        var tipos = TiposParaCorrigir(ativos, investimentos);
        var analise = await AnalisarExtratoAsync(ativos, ct);
        var cotas = await CotasDiferentesAsync(ativos, investimentos, analise.SemOperacao, ct);

        var categoria = await CategoriaDoTrocoAsync(ct);
        var trocos = analise.DeOperacao
            .Where(c => c.Credito.Tipo != TipoTransacao.Transferencia)
            .OrderBy(c => c.Credito.Data).ThenBy(c => c.Credito.Valor)
            .Select(c => new CreditoDaCorretoraDto(c.Credito.Id, c.Credito.Data, c.Credito.Valor, c.Credito.Descricao,
                c.Credito.Conta, c.Credito.Categoria, c.Compra?.Data, c.Compra?.Valor, c.Venda?.Ticker,
                c.Venda?.Op.Quantidade, c.Venda?.Op.Data))
            .ToList();
        var rendimentos = await RendimentosComoInvestimentoAsync(analise, ct);
        var compras = analise.SemOperacao.OrderBy(c => c.Debito.Data).ThenBy(c => c.Debito.Valor)
            .Select(c => new CompraSemOperacaoDto(c.Debito.Id, c.Debito.Data, c.Debito.Valor, c.Troco?.Valor ?? 0m,
                c.Debito.Conta, c.Debito.Descricao))
            .ToList();
        var taxas = analise.Taxas.Select(t => new TaxaB3Dto(t.Data, t.Conta, t.Debitado, t.Trocos, t.ValorDasCotas, t.Taxa,
                t.Operacoes.Select(o => new TaxaB3OperacaoDto(o.Op.Id, o.Ticker, o.Op.Quantidade, o.Taxa)).ToList()))
            .ToList();
        return new ConferenciaInvestimentosDto(tipos, cotas, compras, trocos, rendimentos,
            categoria is null ? null : new CategoriaDoTrocoDto(categoria.Value.Id, categoria.Value.Nome), taxas);
    }

    /// <summary>
    /// #170: trocos e vendas da corretora que ainda contam como receita, para o aviso do Painel. A correção continua
    /// sendo escolhida pelo usuário, com prévia (ADR 0027): aqui só se conta.
    /// </summary>
    public async Task<(int Quantidade, decimal Total)> CreditosComoReceitaAsync(CancellationToken ct)
    {
        var ativos = await _db.Ativos.AsNoTracking().Include(a => a.Operacoes).ToListAsync(ct);
        var creditos = (await AnalisarExtratoAsync(ativos, ct)).DeOperacao
            .Where(c => c.Credito.Tipo != TipoTransacao.Transferencia).ToList();
        return (creditos.Count, creditos.Sum(c => c.Credito.Valor));
    }

    /// <summary>
    /// #173: créditos da corretora que são provento — os "Valor recebido de Investimentos" que não são troco de compra
    /// nem valor de venda. Vale com ou sem a conferência aplicada: a categoria do crédito não entra na conta.
    /// </summary>
    public async Task<List<(long Id, DateOnly Data, decimal Valor)>> ProventosDaCorretoraAsync(CancellationToken ct)
    {
        var ativos = await _db.Ativos.AsNoTracking().Include(a => a.Operacoes).ToListAsync(ct);
        var analise = await AnalisarExtratoAsync(ativos, ct);
        var deOperacao = analise.DeOperacao.Select(c => c.Credito.Id).ToHashSet();
        return analise.Creditos.Where(c => !deOperacao.Contains(c.Id)).Select(c => (c.Id, c.Data, c.Valor)).ToList();
    }

    /// <summary>FII cadastrado como ação ou "outro" (o nome do fundo ou o emissor informado pela corretora).</summary>
    private static List<TipoParaCorrigirDto> TiposParaCorrigir(List<Ativo> ativos, List<Investimento> investimentos)
    {
        var lista = new List<TipoParaCorrigirDto>();
        foreach (var a in ativos.Where(a => (a.Tipo is TipoAtivo.Acao or TipoAtivo.Outro) && a.Moeda == CambioService.MoedaBase))
        {
            string? motivo = null;
            if (ClassificacaoAtivo.PareceFii(a.Nome)) motivo = $"o nome do fundo é “{a.Nome}”";
            else if (investimentos.FirstOrDefault(i => TickerDe(i) == a.Ticker
                         && (ClassificacaoAtivo.PareceFii(i.Emissor) || ClassificacaoAtivo.PareceFii(i.Nome))) is { } inv)
                motivo = $"a corretora informa “{(ClassificacaoAtivo.PareceFii(inv.Emissor) ? inv.Emissor : inv.Nome)}”";
            if (motivo is not null) lista.Add(new TipoParaCorrigirDto(a.Id, a.Ticker, a.Nome, a.Tipo, motivo));
        }
        return lista;
    }

    /// <summary>
    /// Cotas de cada ativo de renda variável na corretora, somando as conexões, pela última sincronização.
    /// Posição zerada (resgate total, direito de subscrição exercido) fica de fora.
    /// </summary>
    public static Dictionary<string, (decimal Quantidade, DateTime Em)> CotasNaCorretora(IEnumerable<Investimento> investimentos)
        => investimentos
            .Where(i => (i.Tipo == "EQUITY" || i.Tipo == "ETF") && i.Quantidade > 0 && i.Saldo > 0)
            .Select(i => (Ticker: TickerDe(i), i.Quantidade, i.AtualizadoEm))
            .Where(x => x.Ticker is not null)
            .GroupBy(x => x.Ticker!)
            .ToDictionary(g => g.Key, g => (g.Sum(x => x.Quantidade!.Value), g.Max(x => x.AtualizadoEm)));

    private static string? TickerDe(Investimento i)
        => CotacaoService.NormalizarTicker(i.Codigo) ?? CotacaoService.NormalizarTicker(i.Nome);

    /// <summary>
    /// Cotas na corretora × na carteira. Quando faltam cotas, sugere o débito de compra sem operação cujo valor
    /// (sem o troco), dividido pelas cotas que faltam, dá um preço perto do fechamento daquele dia.
    /// </summary>
    private async Task<List<CotasDiferentesDto>> CotasDiferentesAsync(List<Ativo> ativos, List<Investimento> investimentos,
        List<CompraSemOperacao> semOperacao, CancellationToken ct)
    {
        var diferencas = new List<(Ativo Ativo, decimal NaCorretora, decimal NaCarteira, DateTime Em)>();
        foreach (var (ticker, (quantidade, em)) in CotasNaCorretora(investimentos))
        {
            if (ativos.FirstOrDefault(a => a.Ticker == ticker) is not { } ativo) continue; // a importação cuida
            var naCarteira = CarteiraService.Calcular(ativo.Operacoes).Quantidade;
            if (Math.Abs(quantidade - naCarteira) >= 0.000001m) diferencas.Add((ativo, quantidade, naCarteira, em));
        }

        // Candidatos: (ativo, débito, preço, fechamento), do mais perto da cotação para o mais longe.
        var candidatos = new List<(int AtivoId, long DebitoId, decimal Preco, decimal Fechamento, decimal Desvio)>();
        foreach (var (ativo, naCorretora, naCarteira, em) in diferencas.Where(d => d.NaCorretora > d.NaCarteira))
        {
            var faltam = naCorretora - naCarteira;
            var debitos = semOperacao.Where(c => c.Debito.Data <= DateOnly.FromDateTime(em)).ToList();
            if (debitos.Count == 0) continue;
            var fechamentos = await FechamentosAsync(ativo, debitos.Min(c => c.Debito.Data), debitos.Max(c => c.Debito.Data), ct);
            foreach (var c in debitos)
            {
                if (FechamentoEm(fechamentos, c.Debito.Data) is not decimal fechamento || fechamento <= 0) continue;
                var preco = Math.Round((-c.Debito.Valor - (c.Troco?.Valor ?? 0m)) / faltam, 4);
                var desvio = Math.Abs(preco / fechamento - 1);
                if (desvio <= FolgaPreco) candidatos.Add((ativo.Id, c.Debito.Id, preco, fechamento, desvio));
            }
        }
        var sugestoes = new Dictionary<int, (long DebitoId, decimal Preco, decimal Fechamento)>();
        var debitosSugeridos = new HashSet<long>();
        foreach (var c in candidatos.OrderBy(c => c.Desvio))
        {
            if (sugestoes.ContainsKey(c.AtivoId) || debitosSugeridos.Contains(c.DebitoId)) continue;
            debitosSugeridos.Add(c.DebitoId);
            sugestoes[c.AtivoId] = (c.DebitoId, c.Preco, c.Fechamento);
        }

        return diferencas.OrderBy(d => d.Ativo.Ticker).Select(d =>
        {
            var tem = sugestoes.TryGetValue(d.Ativo.Id, out var s);
            return new CotasDiferentesDto(d.Ativo.Id, d.Ativo.Ticker, d.NaCorretora, d.NaCarteira, d.Em,
                tem ? s.DebitoId : null, tem ? s.Preco : null, tem ? s.Fechamento : null);
        }).ToList();
    }

    /// <summary>Fechamentos diários do período (Yahoo); sem resposta, o fechamento do mês guardado no banco.</summary>
    private async Task<SortedDictionary<DateOnly, decimal>> FechamentosAsync(Ativo ativo, DateOnly de, DateOnly ate,
        CancellationToken ct)
    {
        var diarios = await _cotacoes.FechamentosDiariosAsync(ativo.Ticker, de, ate, ct);
        if (diarios.Count > 0) return diarios;
        var inicio = new DateOnly(de.Year, de.Month, 1);
        var mensais = await _db.PrecosMensaisAtivo.AsNoTracking()
            .Where(p => p.AtivoId == ativo.Id && p.Mes >= inicio && p.Mes <= ate)
            .ToListAsync(ct);
        // O fechamento do mês vale para os dias daquele mês.
        var resultado = new SortedDictionary<DateOnly, decimal>();
        foreach (var p in mensais) resultado[p.Mes] = p.Fechamento;
        return resultado;
    }

    private static decimal? FechamentoEm(SortedDictionary<DateOnly, decimal> fechamentos, DateOnly data)
    {
        if (fechamentos.Count == 0) return null;
        if (fechamentos.TryGetValue(data, out var exato)) return exato;
        var anterior = fechamentos.Where(f => f.Key <= data).Select(f => (decimal?)f.Value).LastOrDefault();
        return anterior ?? fechamentos.First().Value;
    }

    /// <summary>
    /// Liga os débitos de compra do extrato às operações da carteira e separa, nos créditos da corretora, o que é troco
    /// de compra ou valor de venda (o resto é provento).
    /// <list type="number">
    /// <item>Débito × ordem: a combinação com a menor sobra do débito sobre o custo primeiro.</item>
    /// <item>Um troco por compra: os créditos mais perto da data e, no mesmo dia, os menores (o provento que cai no
    /// mesmo dia costuma ser maior que a sobra de 1 a 2% de uma compra).</item>
    /// <item>Sobra de centavos devolvida depois: com todas as compras do dia ligadas a operações, os créditos menores
    /// entram enquanto a soma dos trocos não passa do que os débitos tiveram a mais que o custo.</item>
    /// <item>Venda: crédito com o valor da venda (quantidade × preço − taxas) até 3 dias depois do pregão.</item>
    /// <item>#173 a): taxa da B3 do dia, quando todos os débitos do dia têm operação, nenhuma operação tem taxa lançada e o
    /// troco já teve tempo de voltar: débitos − trocos − cotas × preço, se ficar entre R$ 0,01 e o máximo.</item>
    /// </list>
    /// </summary>
    private async Task<Analise> AnalisarExtratoAsync(List<Ativo> ativos, CancellationToken ct)
    {
        var semOperacao = new List<CompraSemOperacao>();
        var deOperacao = new List<CreditoDeOperacao>();
        var taxas = new List<TaxaDoDia>();
        var semDuvida = new HashSet<long>();
        var operacoes = ativos.Where(a => a.Moeda == CambioService.MoedaBase)
            .SelectMany(a => a.Operacoes.Select(o => (Ativo: a, Op: o))).ToList();
        if (operacoes.Count == 0) return new Analise(semOperacao, deOperacao, new(), taxas, semDuvida);
        var tickerDoAtivo = ativos.ToDictionary(a => a.Id, a => a.Ticker);
        var compras = operacoes.Where(x => x.Op.Tipo == TipoOperacao.Compra).Select(x => x.Op).ToList();

        var desde = operacoes.Min(x => x.Op.Data).AddDays(-7);
        var ligadas = operacoes.Where(x => x.Op.TransacaoId != null).GroupBy(x => x.Op.TransacaoId!.Value)
            .ToDictionary(g => g.Key, g => g.First().Op);
        // A corretora pode dividir a execução: várias operações ligadas ao mesmo débito.
        var operacoesDoDebito = operacoes.Where(x => x.Op.TransacaoId != null).GroupBy(x => x.Op.TransacaoId!.Value)
            .ToDictionary(g => g.Key, g => g.Select(x => x.Op).ToList());
        var idsLigadas = ligadas.Keys.ToList();
        var movimentos = await _db.Transacoes.AsNoTracking()
            .Where(t => t.Data >= desde && t.TransferenciaGrupo == null
                && ((t.Valor < 0 && (idsLigadas.Contains(t.Id) || EF.Functions.Like(t.Descricao, "%renda vari%")
                        || (t.DescricaoOriginal != null && EF.Functions.Like(t.DescricaoOriginal, "%renda vari%"))))
                    || (t.Valor > 0 && (EF.Functions.Like(t.Descricao, "%invest%")
                        || (t.DescricaoOriginal != null && EF.Functions.Like(t.DescricaoOriginal, "%invest%"))))))
            .Select(t => new Movimento(t.Id, t.ContaId, t.Conta!.Nome, t.Data, t.Descricao, t.DescricaoOriginal,
                t.PluggyCategoriaId, t.Valor, t.Tipo, t.Categoria != null ? t.Categoria.Nome : null, t.CategoriaManual))
            .ToListAsync(ct);

        var debitos = movimentos.Where(m => m.Valor < 0
            && (ligadas.TryGetValue(m.Id, out var op) ? op.Tipo == TipoOperacao.Compra : EhCompra(m.Descricao))).ToList();
        var creditos = movimentos.Where(m => m.Valor > 0 && !ligadas.ContainsKey(m.Id) && EhCreditoDaCorretora(m.Descricao))
            .ToList();
        var livres = compras.Where(o => o.TransacaoId == null).ToHashSet();
        var creditosUsados = new HashSet<long>();

        foreach (var dia in debitos.GroupBy(d => (d.ContaId, d.Data)).OrderBy(g => g.Key.Data))
        {
            var (contaId, data) = dia.Key;
            var pares = new List<(Movimento Debito, decimal Custo, List<OperacaoAtivo> Operacoes)>();
            foreach (var d in dia.Where(d => ligadas.ContainsKey(d.Id)))
                pares.Add((d, operacoesDoDebito[d.Id].Sum(Custo), operacoesDoDebito[d.Id]));

            // 1) Débito × ordem.
            var semLigacao = dia.Where(d => !ligadas.ContainsKey(d.Id)).ToList();
            var candidatas = (from d in semLigacao
                              from ordem in OrdensPerto(livres, data)
                              where ordem.Custo > 0
                              let sobra = -d.Valor / ordem.Custo - 1
                              where sobra >= -0.002m && sobra <= FolgaDebito
                              orderby sobra
                              select (Debito: d, Ordem: ordem)).ToList();
            var debitosLigados = new HashSet<long>();
            foreach (var (d, ordem) in candidatas)
            {
                if (debitosLigados.Contains(d.Id) || !ordem.Operacoes.All(livres.Contains)) continue;
                debitosLigados.Add(d.Id);
                livres.ExceptWith(ordem.Operacoes);
                pares.Add((d, ordem.Custo, ordem.Operacoes));
            }
            var soltos = semLigacao.Where(d => !debitosLigados.Contains(d.Id)).ToList();

            var creditosPerto = creditos.Where(c => c.ContaId == contaId && c.Data >= data
                && c.Data <= data.AddDays(DiasDaSobra) && !creditosUsados.Contains(c.Id))
                .OrderBy(c => c.Data).ThenBy(c => c.Valor).ToList();

            // 2) Um troco por compra.
            var trocoDe = new Dictionary<long, Movimento>();
            var abertos = dia.OrderBy(d => -d.Valor).ToList();
            foreach (var c in creditosPerto.Where(c => c.Data <= data.AddDays(DiasDoTroco)))
            {
                var d = abertos.FirstOrDefault(x => !trocoDe.ContainsKey(x.Id) && c.Valor <= -x.Valor * TrocoMaximo);
                if (d is null) continue;
                trocoDe[d.Id] = c;
                creditosUsados.Add(c.Id);
                deOperacao.Add(new CreditoDeOperacao(c, d, null));
            }

            // 3) Sobra de centavos.
            var sobras = new List<Movimento>();
            if (pares.Count == dia.Count())
            {
                var restante = pares.Sum(p => -p.Debito.Valor - p.Custo) - trocoDe.Values.Sum(c => c.Valor);
                var maior = pares.MaxBy(p => -p.Debito.Valor).Debito;
                foreach (var c in creditosPerto.Where(c => !creditosUsados.Contains(c.Id)).OrderBy(c => c.Valor))
                {
                    if (c.Valor > restante) break;
                    restante -= c.Valor;
                    sobras.Add(c);
                    creditosUsados.Add(c.Id);
                    deOperacao.Add(new CreditoDeOperacao(c, maior, null));
                }
            }

            // 5) A conta do dia, quando todas as compras têm operação e o troco já teve os dias dele para voltar.
            if (pares.Count == dia.Count() && data <= Relogio.Hoje.AddDays(-DiasDaSobra))
            {
                var ops = pares.SelectMany(p => p.Operacoes).Distinct().ToList();
                var debitado = pares.Sum(p => -p.Debito.Valor);
                var trocosDoDia = trocoDe.Values.Concat(sobras).ToList();
                var trocos = trocosDoDia.Sum(c => c.Valor);
                var cotas = ops.Sum(o => o.Quantidade * o.PrecoUnitario);
                var maximo = Math.Min(cotas * TaxaMaxima, TaxaMaximaReais);
                // #173 b): sobra só a taxa da B3 (a lançada ou a que falta): os trocos do dia são sem dúvida.
                var semTaxa = Math.Round(debitado - trocos - cotas - ops.Sum(o => o.Taxas), 2);
                if (semTaxa >= 0 && semTaxa <= maximo) semDuvida.UnionWith(trocosDoDia.Select(c => c.Id));
                // #173 a): a taxa da B3 que falta.
                var taxa = Math.Round(debitado - trocos - cotas, 2);
                if (ops.All(o => o.Taxas == 0) && taxa >= 0.01m && taxa <= maximo)
                    taxas.Add(new TaxaDoDia(data, dia.First().Conta, debitado, trocos, Math.Round(cotas, 2), taxa,
                        DividirTaxa(ops, taxa).Select(p => (p.Op, tickerDoAtivo[p.Op.AtivoId], p.Taxa)).ToList()));
            }
            foreach (var d in soltos) semOperacao.Add(new CompraSemOperacao(d, trocoDe.GetValueOrDefault(d.Id)));
        }

        // 4) Venda: o valor da venda volta como crédito da corretora.
        foreach (var (ativo, venda) in operacoes.Where(x => x.Op.Tipo == TipoOperacao.Venda && x.Op.TransacaoId == null)
                     .OrderBy(x => x.Op.Data))
        {
            var valor = venda.Quantidade * venda.PrecoUnitario - venda.Taxas;
            if (valor <= 0) continue;
            var credito = creditos.Where(c => !creditosUsados.Contains(c.Id) && c.Data >= venda.Data
                    && c.Data <= venda.Data.AddDays(DiasDaVenda)
                    && Math.Abs(c.Valor - valor) <= Math.Max(0.05m, valor * FolgaVenda))
                .OrderBy(c => Math.Abs(c.Valor - valor)).FirstOrDefault();
            if (credito is null) continue;
            creditosUsados.Add(credito.Id);
            deOperacao.Add(new CreditoDeOperacao(credito, null, (ativo.Ticker, venda)));
        }
        return new Analise(semOperacao, deOperacao, creditos, taxas, semDuvida);
    }

    /// <summary>#173 a): a taxa do dia dividida pelas operações, pelo valor de cada uma (o resto de centavos na maior).</summary>
    private static List<(OperacaoAtivo Op, decimal Taxa)> DividirTaxa(List<OperacaoAtivo> ops, decimal taxa)
    {
        var total = ops.Sum(o => o.Quantidade * o.PrecoUnitario);
        var partes = ops.Select(o => (Op: o, Taxa: total > 0 ? Math.Round(taxa * o.Quantidade * o.PrecoUnitario / total, 2) : 0m))
            .ToList();
        var maior = partes.IndexOf(partes.MaxBy(p => p.Op.Quantidade * p.Op.PrecoUnitario));
        partes[maior] = (partes[maior].Op, partes[maior].Taxa + taxa - partes.Sum(p => p.Taxa));
        return partes.Where(p => p.Taxa > 0).ToList();
    }

    /// <summary>
    /// Provento contado como investimento: crédito com a mesma descrição dos trocos (a corretora usa uma só), que não
    /// é troco nem venda e está numa categoria de transferência. Volta para a categoria das regras, se ela for de
    /// receita (a regra padrão leva "Valor recebido de Investimentos" para Rendimentos).
    /// </summary>
    private async Task<List<RendimentoComoInvestimentoDto>> RendimentosComoInvestimentoAsync(Analise analise,
        CancellationToken ct)
    {
        var deOperacao = analise.DeOperacao.Select(c => c.Credito.Id).ToHashSet();
        var descricoes = analise.DeOperacao.Select(c => CategorizacaoService.Normalizar(c.Credito.Descricao)).ToHashSet();
        var candidatos = analise.Creditos.Where(c => !deOperacao.Contains(c.Id) && c.Tipo == TipoTransacao.Transferencia
            && descricoes.Contains(CategorizacaoService.Normalizar(c.Descricao))).ToList();
        if (candidatos.Count == 0) return new();

        await _categorizacao.CarregarAsync(ct);
        var categorias = await _db.Categorias.AsNoTracking().ToDictionaryAsync(c => c.Id, c => (c.Nome, c.Tipo), ct);
        var lista = new List<RendimentoComoInvestimentoDto>();
        foreach (var c in candidatos.OrderBy(c => c.Data).ThenBy(c => c.Valor))
        {
            if (CategoriaDeRendimento(c.DescricaoOriginal ?? c.Descricao, c.PluggyCategoriaId, categorias) is not int id)
                continue;
            lista.Add(new RendimentoComoInvestimentoDto(c.Id, c.Data, c.Valor, c.Descricao, c.Conta, c.Categoria,
                categorias[id].Nome));
        }
        return lista;
    }

    /// <summary>A categoria que as regras dão ao crédito, se for de receita (senão, o crédito fica como está).</summary>
    private int? CategoriaDeRendimento(string descricao, string? pluggyCategoriaId,
        Dictionary<int, (string Nome, TipoCategoria Tipo)> categorias)
        => _categorizacao.Resolver(descricao, pluggyCategoriaId) is int id
           && categorias.TryGetValue(id, out var c) && c.Tipo == TipoCategoria.Receita ? id : null;

    /// <summary>Ordens livres perto da data do débito: cada operação e, com mais de uma, as do mesmo ativo e dia juntas.</summary>
    private static IEnumerable<Ordem> OrdensPerto(HashSet<OperacaoAtivo> livres, DateOnly data)
    {
        var perto = livres.Where(o => o.Data >= data.AddDays(-DiasAntesDoDebito) && o.Data <= data.AddDays(DiasDepoisDoDebito))
            .ToList();
        foreach (var o in perto) yield return new Ordem([o], Custo(o));
        foreach (var g in perto.GroupBy(o => (o.AtivoId, o.Data)).Where(g => g.Count() > 1))
            yield return new Ordem(g.ToList(), g.Sum(Custo));
    }

    private static decimal Custo(OperacaoAtivo o) => o.Quantidade * o.PrecoUnitario + o.Taxas;

    /// <summary>Débito de compra na corretora sem o código do ativo (o Nubank escreve "Compra de Renda Variável").</summary>
    private static bool EhCompra(string descricao)
    {
        var texto = CategorizacaoService.Normalizar(descricao);
        return texto.Contains("RENDA VARIAVEL") && !texto.Contains("VENDA") && !texto.Contains("RESGATE");
    }

    /// <summary>Crédito da corretora que pode ser troco, venda ou provento ("Valor recebido de Investimentos").</summary>
    private static bool EhCreditoDaCorretora(string descricao)
    {
        var texto = CategorizacaoService.Normalizar(descricao);
        return texto.Contains("INVEST") && !texto.Contains("RESGATE") && !texto.Contains("FUNDO");
    }

    /// <summary>Categoria do troco: "Investimentos" (a mesma da compra) ou outra do tipo transferência.</summary>
    private async Task<(int Id, string Nome)?> CategoriaDoTrocoAsync(CancellationToken ct)
    {
        var categorias = await _db.Categorias.AsNoTracking().Where(c => c.Tipo == TipoCategoria.Transferencia)
            .OrderBy(c => c.Nome).Select(c => new { c.Id, c.Nome }).ToListAsync(ct);
        var escolhida = categorias.FirstOrDefault(c => c.Nome == "Investimentos")
                        ?? categorias.FirstOrDefault(c => CategorizacaoService.Normalizar(c.Nome).Contains("INVEST"));
        return escolhida is null ? null : (escolhida.Id, escolhida.Nome);
    }

    /* ============================== Corrigir e desfazer ============================== */

    /// <summary>
    /// Aplica as correções escolhidas numa gravação só: tipo FII, compras que faltaram (ligadas ao débito do extrato),
    /// troco e venda na categoria da compra (como escolha manual, para as regras não trocarem de volta) e provento de
    /// volta para a categoria automática.
    /// </summary>
    public async Task<(ResultadoCorrecaoInvestimentosDto? Resultado, string? Erro)> CorrigirAsync(CorrigirInvestimentosDto dto,
        string? login, CancellationToken ct)
    {
        var anterior = new EstadoAnterior(new(), new(), new(), new());

        var idsAtivos = (dto.AtivosParaFii ?? new()).Distinct().ToList();
        var ativos = await _db.Ativos.Where(a => idsAtivos.Contains(a.Id)).ToListAsync(ct);
        if (ativos.Count != idsAtivos.Count) return (null, "Algum ativo não existe mais. Confira de novo.");
        foreach (var a in ativos.Where(a => a.Tipo != TipoAtivo.Fii))
        {
            if (a.Moeda != CambioService.MoedaBase) return (null, $"{a.Ticker} é do exterior: não pode ser FII.");
            anterior.Tipos.Add((a.Id, a.Tipo));
            a.Tipo = TipoAtivo.Fii;
        }

        // #173 a): a taxa é recalculada aqui (não vem da tela) e só vale para a operação que ainda a tem sugerida.
        var idsTaxas = (dto.Taxas ?? new()).Distinct().ToList();
        if (idsTaxas.Count > 0)
        {
            var todos = await _db.Ativos.AsNoTracking().Include(a => a.Operacoes).ToListAsync(ct);
            var sugeridas = (await AnalisarExtratoAsync(todos, ct)).Taxas.SelectMany(t => t.Operacoes)
                .ToDictionary(o => o.Op.Id, o => o.Taxa);
            if (idsTaxas.Any(id => !sugeridas.ContainsKey(id)))
                return (null, "Alguma taxa da B3 mudou desde a conferência. Confira de novo.");
            foreach (var o in await _db.OperacoesAtivo.Where(o => idsTaxas.Contains(o.Id)).ToListAsync(ct))
            {
                anterior.Taxas.Add((o.Id, o.Taxas, sugeridas[o.Id]));
                o.Taxas = sugeridas[o.Id];
            }
        }

        var novas = new List<OperacaoAtivo>();
        var itens = dto.Compras ?? new();
        if (itens.Count > 0)
        {
            var idsDebitos = itens.Select(i => i.TransacaoId).ToList();
            if (idsDebitos.Distinct().Count() != idsDebitos.Count) return (null, "O mesmo débito foi escolhido para duas compras.");
            var debitos = await _db.Transacoes.AsNoTracking().Where(t => idsDebitos.Contains(t.Id))
                .Select(t => new { t.Id, t.Data, t.Valor }).ToDictionaryAsync(t => t.Id, ct);
            var jaLigados = await _db.OperacoesAtivo.AnyAsync(o => o.TransacaoId != null && idsDebitos.Contains(o.TransacaoId.Value), ct);
            if (jaLigados) return (null, "Algum débito já tem uma operação na carteira. Confira de novo.");
            var tickers = itens.Select(i => CotacaoService.NormalizarTicker(i.Ticker)).ToList();
            var ativosDasCompras = await _db.Ativos.Where(a => tickers.Contains(a.Ticker)).ToDictionaryAsync(a => a.Ticker, ct);
            for (var i = 0; i < itens.Count; i++)
            {
                var item = itens[i];
                if (!debitos.TryGetValue(item.TransacaoId, out var debito) || debito.Valor >= 0)
                    return (null, "Escolha um débito de compra do extrato.");
                if (tickers[i] is not string ticker || !ativosDasCompras.TryGetValue(ticker, out var ativo))
                    return (null, $"{item.Ticker} não está na carteira.");
                if (item.Tipo != TipoOperacao.Compra || item.Quantidade <= 0 || item.PrecoUnitario <= 0)
                    return (null, $"Informe cotas e preço da compra de {ticker}.");
                var op = new OperacaoAtivo
                {
                    Ativo = ativo,
                    Tipo = TipoOperacao.Compra,
                    Data = debito.Data,
                    Quantidade = Math.Round(item.Quantidade, 6),
                    PrecoUnitario = Math.Round(item.PrecoUnitario, 6),
                    Taxas = Math.Round(Math.Max(0, item.Taxas), 2),
                    Origem = OrigemOperacao.Extrato,
                    TransacaoId = item.TransacaoId,
                    Observacao = "Compra que faltou no Open Finance (Conferir investimentos)"
                };
                _db.OperacoesAtivo.Add(op);
                novas.Add(op);
            }
        }

        var idsTrocos = (dto.Trocos ?? new()).Distinct().ToList();
        var idsRendimentos = (dto.Rendimentos ?? new()).Distinct().ToList();
        if (idsTrocos.Intersect(idsRendimentos).Any()) return (null, "Um crédito não pode ser troco e rendimento ao mesmo tempo.");
        var idsCreditos = idsTrocos.Concat(idsRendimentos).ToList();
        int trocos = 0, rendimentos = 0;
        if (idsCreditos.Count > 0)
        {
            var transacoes = await _db.Transacoes.Where(t => idsCreditos.Contains(t.Id)).ToDictionaryAsync(t => t.Id, ct);
            if (transacoes.Count != idsCreditos.Count || transacoes.Values.Any(t => t.Valor <= 0))
                return (null, "Algum crédito não existe mais. Confira de novo.");
            await _categorizacao.CarregarAsync(ct);
            var categorias = await _db.Categorias.AsNoTracking().ToDictionaryAsync(c => c.Id, c => (c.Nome, c.Tipo), ct);

            (int Id, string Nome)? categoriaTroco = null;
            if (idsTrocos.Count > 0 && (categoriaTroco = await CategoriaDoTrocoAsync(ct)) is null)
                return (null, "Crie uma categoria do tipo transferência (ex.: Investimentos) para o troco das compras.");

            foreach (var id in idsCreditos)
            {
                var t = transacoes[id];
                var ehTroco = idsTrocos.Contains(id);
                int? nova = ehTroco ? categoriaTroco!.Value.Id
                    : CategoriaDeRendimento(t.DescricaoOriginal ?? t.Descricao, t.PluggyCategoriaId, categorias);
                if (nova is null) continue; // as regras não dizem que é rendimento: fica como está
                if (t.CategoriaId == nova && t.CategoriaManual == ehTroco) continue;
                anterior.Transacoes.Add((t.Id, t.CategoriaId, t.CategoriaManual, t.Tipo, t.IgnorarRelatorios, nova));
                t.CategoriaId = nova;
                t.CategoriaManual = ehTroco; // o provento volta para a categoria automática
                _categorizacao.AplicarTipo(t);
                t.AtualizadoEm = DateTime.UtcNow;
                if (ehTroco) trocos++;
                else rendimentos++;
            }
        }

        if (anterior.Tipos.Count == 0 && novas.Count == 0 && anterior.Transacoes.Count == 0 && anterior.Taxas.Count == 0)
            return (new ResultadoCorrecaoInvestimentosDto(0, 0, 0, 0, 0, null), null);
        _auditoria.Registrar(
            $"ConferenciaInvestimentos ({anterior.Tipos.Count} tipo, {novas.Count} compra, {trocos} troco, {rendimentos} rendimento, "
            + $"{anterior.Taxas.Count} taxa)", login);
        await _db.SaveChangesAsync(ct);
        anterior.OperacoesCriadas.AddRange(novas.Select(o => o.Id));
        var chave = Guid.NewGuid().ToString("N");
        _cache.Set(ChaveDesfazer(chave), anterior, PrazoDesfazer);
        return (new ResultadoCorrecaoInvestimentosDto(anterior.Tipos.Count, novas.Count, trocos, rendimentos,
            anterior.Taxas.Count, chave), null);
    }

    /// <summary>
    /// Desfaz uma correção: volta o tipo, apaga as compras lançadas, volta categoria, "manual", tipo e "ignorar nos
    /// relatórios" dos créditos e a taxa das operações. O que mudou de novo depois disso fica como está.
    /// </summary>
    public async Task<int?> DesfazerAsync(string chave, string? login, CancellationToken ct)
    {
        if (!_cache.TryGetValue(ChaveDesfazer(chave), out EstadoAnterior? anterior) || anterior is null) return null;
        _cache.Remove(ChaveDesfazer(chave));
        var desfeitas = 0;

        var idsAtivos = anterior.Tipos.Select(t => t.AtivoId).ToList();
        var ativos = await _db.Ativos.Where(a => idsAtivos.Contains(a.Id)).ToDictionaryAsync(a => a.Id, ct);
        foreach (var (id, tipo) in anterior.Tipos)
        {
            if (!ativos.TryGetValue(id, out var a) || a.Tipo != TipoAtivo.Fii) continue;
            a.Tipo = tipo;
            desfeitas++;
        }

        var operacoes = await _db.OperacoesAtivo.Where(o => anterior.OperacoesCriadas.Contains(o.Id)).ToListAsync(ct);
        _db.OperacoesAtivo.RemoveRange(operacoes);
        desfeitas += operacoes.Count;

        var idsTaxas = anterior.Taxas.Select(t => t.OperacaoId).ToList();
        var comTaxa = await _db.OperacoesAtivo.Where(o => idsTaxas.Contains(o.Id)).ToDictionaryAsync(o => o.Id, ct);
        foreach (var (id, antes, aplicada) in anterior.Taxas)
        {
            if (!comTaxa.TryGetValue(id, out var o) || o.Taxas != aplicada) continue;
            o.Taxas = antes;
            desfeitas++;
        }

        var idsTransacoes = anterior.Transacoes.Select(t => t.Id).ToList();
        var transacoes = await _db.Transacoes.Where(t => idsTransacoes.Contains(t.Id)).ToDictionaryAsync(t => t.Id, ct);
        var categorias = (await _db.Categorias.Select(c => c.Id).ToListAsync(ct)).ToHashSet();
        foreach (var antes in anterior.Transacoes)
        {
            if (!transacoes.TryGetValue(antes.Id, out var t) || t.CategoriaId != antes.Aplicada) continue;
            t.CategoriaId = antes.CategoriaId is int c && !categorias.Contains(c) ? null : antes.CategoriaId;
            t.CategoriaManual = antes.CategoriaManual;
            t.Tipo = antes.Tipo;
            t.IgnorarRelatorios = antes.IgnorarRelatorios;
            t.AtualizadoEm = DateTime.UtcNow;
            desfeitas++;
        }
        _auditoria.Registrar($"ConferenciaInvestimentosDesfeita ({desfeitas})", login);
        await _db.SaveChangesAsync(ct);
        return desfeitas;
    }

    private static string ChaveDesfazer(string chave) => "conferencia-investimentos:" + chave;

    /* ============================== #173 b): trocos na sincronização (ADR 0044) ============================== */

    /// <summary>
    /// Depois da sincronização: o troco de compra "sem dúvida" — de um dia em que todas as compras têm operação, o troco
    /// já teve 5 dias para voltar e débitos − trocos − custo das cotas sobra só a taxa da B3 — que ainda conta como
    /// receita vai para a categoria da compra, como escolha manual. Crédito com categoria escolhida à mão e valor de venda
    /// ficam de fora. O que mudou fica guardado para o aviso e o "Desfazer" do Painel.
    /// </summary>
    public async Task<int> ClassificarTrocosSemDuvidaAsync(CancellationToken ct)
    {
        var ativos = await _db.Ativos.AsNoTracking().Include(a => a.Operacoes).ToListAsync(ct);
        var analise = await AnalisarExtratoAsync(ativos, ct);
        var ids = analise.DeOperacao
            .Where(c => c.Compra is not null && analise.TrocosSemDuvida.Contains(c.Credito.Id)
                && c.Credito.Tipo != TipoTransacao.Transferencia && !c.Credito.CategoriaManual)
            .Select(c => c.Credito.Id).ToList();
        if (ids.Count == 0 || await CategoriaDoTrocoAsync(ct) is not { } categoria) return 0;

        var guardados = await TrocosGuardadosAsync(ct);
        foreach (var t in await _db.Transacoes.Where(t => ids.Contains(t.Id)).ToListAsync(ct))
        {
            guardados.Add(new TrocoAutomatico(t.Id, t.CategoriaId, t.CategoriaManual, t.Tipo, t.IgnorarRelatorios, categoria.Id));
            t.CategoriaId = categoria.Id;
            t.CategoriaManual = true; // como na conferência: as regras não trocam de volta
            _categorizacao.AplicarTipo(t);
            t.AtualizadoEm = DateTime.UtcNow;
        }
        await GuardarTrocosAsync(guardados.TakeLast(MaximoTrocosGuardados).ToList(), ct);
        _auditoria.Registrar($"TrocosAutomaticos ({ids.Count})", null);
        await _db.SaveChangesAsync(ct);
        return ids.Count;
    }

    /// <summary>Os trocos classificados na sincronização que ainda estão na categoria aplicada (para o aviso do Painel).</summary>
    public async Task<TrocosAutomaticosDto?> ResumoTrocosAutomaticosAsync(CancellationToken ct)
    {
        var guardados = await TrocosGuardadosAsync(ct);
        if (guardados.Count == 0) return null;
        var ids = guardados.Select(g => g.Id).ToList();
        var aplicada = guardados.ToDictionary(g => g.Id, g => g.Aplicada);
        var atuais = (await _db.Transacoes.AsNoTracking().Where(t => ids.Contains(t.Id))
                .Select(t => new { t.Id, t.Data, t.Valor, t.CategoriaId }).ToListAsync(ct))
            .Where(t => t.CategoriaId == aplicada[t.Id]).OrderBy(t => t.Data).ToList();
        return atuais.Count == 0 ? null
            : new TrocosAutomaticosDto(atuais.Count, atuais.Sum(t => t.Valor), atuais.Select(t => t.Id).ToList(),
                atuais[0].Data, atuais[^1].Data);
    }

    /// <summary>
    /// Volta os trocos classificados na sincronização para a categoria de antes. A categoria de antes fica como escolha
    /// manual, para a próxima sincronização não classificar de novo o que você disse que não é troco.
    /// </summary>
    public async Task<int> DesfazerTrocosAutomaticosAsync(string? login, CancellationToken ct)
    {
        var guardados = await TrocosGuardadosAsync(ct);
        var ids = guardados.Select(g => g.Id).ToList();
        var transacoes = await _db.Transacoes.Where(t => ids.Contains(t.Id)).ToDictionaryAsync(t => t.Id, ct);
        var categorias = (await _db.Categorias.Select(c => c.Id).ToListAsync(ct)).ToHashSet();
        var desfeitos = 0;
        foreach (var g in guardados)
        {
            if (!transacoes.TryGetValue(g.Id, out var t) || t.CategoriaId != g.Aplicada) continue;
            t.CategoriaId = g.CategoriaId is int c && !categorias.Contains(c) ? null : g.CategoriaId;
            t.CategoriaManual = true;
            t.Tipo = g.Tipo;
            t.IgnorarRelatorios = g.IgnorarRelatorios;
            t.AtualizadoEm = DateTime.UtcNow;
            desfeitos++;
        }
        await GuardarTrocosAsync(new(), ct);
        _auditoria.Registrar($"TrocosAutomaticosDesfeitos ({desfeitos})", login);
        await _db.SaveChangesAsync(ct);
        return desfeitos;
    }

    /// <summary>"Ok" no aviso do Painel: os trocos continuam como estão e o aviso some.</summary>
    public async Task DispensarTrocosAutomaticosAsync(CancellationToken ct)
    {
        await GuardarTrocosAsync(new(), ct);
        await _db.SaveChangesAsync(ct);
    }

    private async Task<List<TrocoAutomatico>> TrocosGuardadosAsync(CancellationToken ct)
    {
        var texto = await _config.ObterAsync(ChaveTrocosAutomaticos, ct);
        if (string.IsNullOrWhiteSpace(texto)) return new();
        try { return JsonSerializer.Deserialize<List<TrocoAutomatico>>(texto) ?? new(); }
        catch (JsonException) { return new(); }
    }

    private Task GuardarTrocosAsync(List<TrocoAutomatico> itens, CancellationToken ct)
        => _config.DefinirAsync(ChaveTrocosAutomaticos, itens.Count == 0 ? null : JsonSerializer.Serialize(itens), ct: ct);
}
