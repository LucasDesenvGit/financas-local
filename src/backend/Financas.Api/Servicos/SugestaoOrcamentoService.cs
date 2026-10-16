using System.Globalization;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// Sugestão de orçamentos por categoria (#39): parte dos hábitos de gasto, cabe na renda e considera o uso do limite
/// dos cartões. Só sugere — nada é gravado aqui; quem grava é <c>OrcamentosController.AplicarSugestao</c>, depois da
/// revisão do usuário. Os percentuais ajustáveis ficam na Parametrização (grupo "Orçamentos").
///
/// Regra, por categoria principal (subcategorias somam nela, como nos orçamentos):
///  1. base = mediana mensal dos últimos 6 meses completos, sem emergências e sem compras parceladas (as parcelas
///     já entram como gasto certo no resumo); com gasto em menos de 2 meses, não sugere ("sem dados suficientes");
///  2. folga: essenciais + FolgaEssenciais%; estilo de vida e sem classificação, + 0%;
///  3. cartão: com uso do limite acima do nível de atenção, estilo de vida pago no cartão perde CorteCartao%;
///  4. encaixe: se o total do mês passar de renda − parcelas − meta de poupança, corta primeiro o estilo de vida e
///     depois o sem classificação, proporcionalmente; essencial nunca é cortado sozinho — a tela diz quanto falta.
///     Categoria que já tem orçamento no mês entra no total com o valor atual (ela vem desmarcada e fica como está)
///     e não é cortada;
///  5. arredondamento para múltiplos de R$ 10.
/// </summary>
public class SugestaoOrcamentoService
{
    private const int MesesBase = 6;
    private const int MinimoMesesComGasto = 2;
    /// <summary>Mês acima de 2,5× a mediana é "pico" (IPVA, viagem): sinalizado, e a mediana já o deixa de fora.</summary>
    private const decimal FatorPico = 2.5m;
    /// <summary>Variação entre os 3 últimos meses e os 3 anteriores para chamar de alta ou queda.</summary>
    private const decimal VariacaoTendencia = 0.15m;
    /// <summary>Corte máximo para caber na renda (julgamento próprio: cortar mais que isso não é realista de cumprir).</summary>
    private const decimal CorteMaximoEstiloDeVida = 0.5m;
    private const decimal CorteMaximoSemNatureza = 0.3m;
    /// <summary>Categoria "paga no cartão": pelo menos metade do gasto da base saiu de cartão de crédito.</summary>
    private const decimal PartePagaNoCartao = 0.5m;
    private const int FaturasParaMedia = 3;

    private static readonly CultureInfo PtBr = new("pt-BR");

    private readonly FinancasDbContext _db;
    private readonly RelatorioService _relatorios;
    private readonly EmprestimoService _emprestimos;
    private readonly ParametroService _parametros;

    public SugestaoOrcamentoService(FinancasDbContext db, RelatorioService relatorios, EmprestimoService emprestimos,
        ParametroService parametros)
    {
        _db = db;
        _relatorios = relatorios;
        _emprestimos = emprestimos;
        _parametros = parametros;
    }

    private sealed class Linha
    {
        public required int CategoriaId { get; init; }
        public required string Nome { get; init; }
        public required string Cor { get; init; }
        public required NaturezaGasto? Natureza { get; init; }
        public decimal[] Meses { get; init; } = [];
        public decimal Mediana { get; set; }
        public decimal Media { get; set; }
        public decimal UltimoMes { get; set; }
        public string? Tendencia { get; set; }
        public int MesesComGasto { get; set; }
        public decimal PagoNoCartao { get; set; }
        public decimal Emergencias { get; set; }
        public decimal? Valor { get; set; }
        public decimal? Atual { get; set; }
        public List<string> Motivos { get; } = [];
    }

    public async Task<SugestaoOrcamentoDto> SugerirAsync(int ano, int mes, CancellationToken ct)
    {
        var folga = await _parametros.ObterInteiroAsync(ConfiguracaoService.OrcamentoFolgaEssenciais, ct) / 100m;
        var percentualPoupanca = await _parametros.ObterInteiroAsync(ConfiguracaoService.OrcamentoMetaPoupanca, ct);
        var atencao = await _parametros.ObterInteiroAsync(ConfiguracaoService.OrcamentoCartaoAtencao, ct);
        var alerta = await _parametros.ObterInteiroAsync(ConfiguracaoService.OrcamentoCartaoAlerta, ct);
        var corteCartao = await _parametros.ObterInteiroAsync(ConfiguracaoService.OrcamentoCorteCartao, ct) / 100m;
        var avisos = new List<string>();

        // ---------------------------------------------------------------- Renda, parcelas e poupança
        var (renda, parcelasCartao, mesesRenda) = await _relatorios.RendaEParcelasAsync(ct);
        var credito = await _emprestimos.TotaisAsync(ct);
        var parcelas = parcelasCartao + credito.ParcelaMensalFinanciamentos + credito.ParcelaMensalEmprestimos;
        var metaPoupanca = renda * percentualPoupanca / 100m;
        var disponivel = renda - parcelas - metaPoupanca;

        // ---------------------------------------------------------------- Cartões: uso do limite
        var cartoes = await UsoDosCartoesAsync(ct);
        var limiteTotal = cartoes.Sum(c => c.Limite);
        decimal? usoGeral = limiteTotal > 0 ? Math.Round(cartoes.Sum(c => c.FaturaMedia) / limiteTotal * 100m, 1) : null;
        string? nivelCartao = usoGeral is not { } u ? null : u >= alerta ? "alerta" : u >= atencao ? "atencao" : "ok";

        // ---------------------------------------------------------------- Histórico por categoria principal
        var (inicio, fim, mesesBase) = await _relatorios.MesesCompletosAsync(MesesBase, ct);
        var categorias = await _db.Categorias.AsNoTracking().Where(c => c.Tipo == TipoCategoria.Despesa)
            .Select(c => new { c.Id, c.Nome, c.Cor, c.CategoriaPaiId, c.Natureza, c.Ativa }).ToListAsync(ct);
        var raizDe = categorias.ToDictionary(c => c.Id, c => c.CategoriaPaiId is int pai && categorias.Any(x => x.Id == pai) ? pai : c.Id);
        var idsCartoes = await _db.Contas.AsNoTracking().Where(c => c.Tipo == TipoConta.CartaoCredito)
            .Select(c => c.Id).ToListAsync(ct);

        var despesas = mesesBase == 0 ? [] : await _relatorios.Relevantes(inicio, fim)
            .Where(t => t.Valor < 0 && t.Tipo == TipoTransacao.Despesa && t.CategoriaId != null
                        && (t.ParcelaTotal == null || t.ParcelaTotal <= 1))
            .Select(t => new { t.Data, t.Valor, CategoriaId = t.CategoriaId!.Value, t.ContaId, t.Emergencia })
            .ToListAsync(ct);

        var orcamentosDoMes = await _db.Orcamentos.AsNoTracking().Where(o => o.Ano == ano && o.Mes == mes)
            .ToDictionaryAsync(o => o.CategoriaId, o => o.Valor, ct);

        var linhas = categorias.Where(c => c.Ativa && raizDe[c.Id] == c.Id).Select(c => new Linha
        {
            CategoriaId = c.Id, Nome = c.Nome, Cor = c.Cor, Natureza = c.Natureza, Meses = new decimal[mesesBase],
            Atual = orcamentosDoMes.TryGetValue(c.Id, out var atual) ? atual : null
        }).ToDictionary(l => l.CategoriaId);

        decimal totalCartao = 0;
        var porCartao = new Dictionary<int, decimal>();
        foreach (var d in despesas)
        {
            if (!raizDe.TryGetValue(d.CategoriaId, out var raiz) || !linhas.TryGetValue(raiz, out var linha)) continue;
            var valor = -d.Valor;
            if (d.Emergencia) { linha.Emergencias += valor; continue; }
            var indice = (d.Data.Year - inicio.Year) * 12 + d.Data.Month - inicio.Month;
            if (indice < 0 || indice >= mesesBase) continue;
            linha.Meses[indice] += valor;
            if (idsCartoes.Contains(d.ContaId))
            {
                porCartao[raiz] = porCartao.GetValueOrDefault(raiz) + valor;
                totalCartao += valor;
            }
        }

        // ---------------------------------------------------------------- 1 e 2: base e folga
        foreach (var l in linhas.Values)
        {
            var total = l.Meses.Sum();
            l.MesesComGasto = l.Meses.Count(v => v > 0);
            l.Media = mesesBase > 0 ? total / mesesBase : 0;
            l.Mediana = Mediana(l.Meses);
            l.UltimoMes = mesesBase > 0 ? l.Meses[^1] : 0;
            l.PagoNoCartao = total > 0 ? porCartao.GetValueOrDefault(l.CategoriaId) / total : 0;
            if (mesesBase >= 6)
            {
                var recente = l.Meses[^3..].Average();
                var anterior = l.Meses[..3].Average();
                l.Tendencia = anterior <= 0 ? (recente > 0 ? "alta" : "estavel")
                    : recente > anterior * (1 + VariacaoTendencia) ? "alta"
                    : recente < anterior * (1 - VariacaoTendencia) ? "queda" : "estavel";
            }

            if (l.MesesComGasto < MinimoMesesComGasto)
            {
                if (l.MesesComGasto == 1)
                    l.Motivos.Add($"gasto em só 1 dos últimos {mesesBase} meses ({NomeMes(inicio, Array.FindIndex(l.Meses, v => v > 0))}): sem dados suficientes");
                continue;
            }

            decimal baseMensal;
            if (l.Mediana > 0)
            {
                baseMensal = l.Mediana;
                l.Motivos.Add($"mediana de {mesesBase} meses");
            }
            else
            {
                baseMensal = l.Media;
                l.Motivos.Add($"gasto irregular ({l.MesesComGasto} de {mesesBase} meses): usa a média mensal");
            }

            var maior = l.Meses.Max();
            if (l.Mediana > 0 && maior > l.Mediana * FatorPico)
                l.Motivos.Add($"pico em {NomeMes(inicio, Array.IndexOf(l.Meses, maior))} ({Reais(maior)}) fora da base");
            if (l.Tendencia == "alta") l.Motivos.Add("tendência de alta nos últimos 3 meses (não aumenta o orçamento)");
            if (l.Emergencias > 0) l.Motivos.Add($"sem as emergências ({Reais(l.Emergencias)})");

            if (l.Natureza == NaturezaGasto.Essencial && folga > 0)
            {
                baseMensal *= 1 + folga;
                l.Motivos.Add($"+{folga * 100:0}% de folga (essencial)");
            }
            l.Valor = baseMensal;
        }

        var comValor = linhas.Values.Where(l => l.Valor is not null).ToList();
        // Só o que vai ser aplicado entra nos cortes; quem já tem orçamento fica com o atual.
        var aplicaveis = comValor.Where(l => l.Atual is null).ToList();
        var fixos = linhas.Values.Sum(l => l.Atual ?? 0m);
        decimal TotalDoMes() => fixos + aplicaveis.Sum(l => l.Valor!.Value);
        var semClassificacao = comValor.Count(l => l.Natureza is null);

        // ---------------------------------------------------------------- 3: uso do limite do cartão
        if (nivelCartao is "atencao" or "alerta" && corteCartao > 0)
        {
            foreach (var l in comValor.Where(l => l.Natureza == NaturezaGasto.EstiloDeVida && l.PagoNoCartao >= PartePagaNoCartao))
            {
                l.Valor *= 1 - corteCartao;
                l.Motivos.Add($"−{corteCartao * 100:0}%: pago no cartão, com {usoGeral:0}% do limite em uso");
            }
        }
        if (nivelCartao == "alerta")
            avisos.Add($"Seus cartões estão usando {usoGeral:0}% do limite (acima de {alerta}%). Vale reduzir o que passa pelo cartão até a fatura baixar.");
        else if (nivelCartao == "atencao")
            avisos.Add($"Seus cartões estão usando {usoGeral:0}% do limite (acima de {atencao}%): a sugestão aperta o estilo de vida pago no cartão.");

        // ---------------------------------------------------------------- 4: caber na renda
        decimal? faltaParaCaber = null;
        if (renda <= 0)
        {
            avisos.Add("Sem receitas nos últimos meses: a sugestão usa só o seu histórico de gastos, sem teto de renda.");
        }
        else
        {
            var excesso = TotalDoMes() - disponivel;
            excesso = Cortar(aplicaveis.Where(l => l.Natureza == NaturezaGasto.EstiloDeVida).ToList(), excesso, CorteMaximoEstiloDeVida);
            excesso = Cortar(aplicaveis.Where(l => l.Natureza is null).ToList(), excesso, CorteMaximoSemNatureza);
            if (excesso > 0.005m) faltaParaCaber = Math.Round(excesso, 2);
        }

        // ---------------------------------------------------------------- 5: arredondar para R$ 10
        foreach (var l in comValor)
        {
            // Essencial arredonda para o mais próximo; o resto, para baixo (cabe na renda depois de arredondar).
            var dezenas = l.Valor!.Value / 10m;
            dezenas = l.Natureza == NaturezaGasto.Essencial ? Math.Round(dezenas, MidpointRounding.AwayFromZero) : Math.Floor(dezenas);
            l.Valor = Math.Max(10m, dezenas * 10m);
        }
        if (renda > 0 && faltaParaCaber is null)
        {
            // O arredondamento dos essenciais pode passar alguns reais do disponível: tira R$ 10 do maior estilo de vida.
            var cortaveis = aplicaveis.Where(l => l.Natureza != NaturezaGasto.Essencial).ToList();
            while (TotalDoMes() > disponivel && cortaveis.FirstOrDefault(l => l.Valor > 10m) is not null)
                cortaveis.Where(l => l.Valor > 10m).MaxBy(l => l.Valor!.Value)!.Valor -= 10m;
        }

        // Total do mês com a sugestão (o que a tela mostra ao abrir). O que falta é medido depois do arredondamento.
        var totalSugerido = TotalDoMes();
        if (faltaParaCaber is not null) faltaParaCaber = Math.Max(0m, totalSugerido - Math.Round(disponivel, 2));
        if (faltaParaCaber is { } falta)
            avisos.Add($"Mesmo cortando estilo de vida e o que está sem classificação, faltam {Reais(falta)} para caber em " +
                       $"renda − parcelas − poupança. Os essenciais não foram cortados: revise os valores ou a meta de poupança ({percentualPoupanca}%).");
        if (mesesBase == 0)
            avisos.Add("Ainda não há um mês completo de transações: sem histórico para sugerir.");
        else if (mesesBase < MesesBase)
            avisos.Add($"Só {mesesBase} {(mesesBase == 1 ? "mês completo" : "meses completos")} de histórico: a sugestão fica menos precisa.");
        if (semClassificacao > 0)
            avisos.Add($"{semClassificacao} {(semClassificacao == 1 ? "categoria está" : "categorias estão")} sem natureza (essencial ou estilo de vida). " +
                       "Classifique em Categorias e regras para a sugestão ficar melhor.");

        var itens = linhas.Values
            .OrderByDescending(l => l.Valor ?? -1).ThenByDescending(l => l.Media).ThenBy(l => l.Nome)
            .Where(l => l.Valor is not null || l.MesesComGasto > 0 || l.Atual is not null)
            .Select(l => new ItemSugestaoOrcamentoDto(l.CategoriaId, l.Nome, l.Cor, l.Natureza, Math.Round(l.Mediana, 2),
                Math.Round(l.Media, 2), Math.Round(l.UltimoMes, 2), l.Tendencia, l.MesesComGasto,
                Math.Round(l.PagoNoCartao * 100m, 0), l.Valor, l.Atual, l.Valor is not null && l.Atual is null, l.Motivos))
            .ToList();

        return new SugestaoOrcamentoDto((short)ano, (byte)mes, mesesBase, mesesRenda, Math.Round(renda, 2),
            Math.Round(parcelasCartao, 2), credito.ParcelaMensalFinanciamentos, credito.ParcelaMensalEmprestimos,
            percentualPoupanca, Math.Round(metaPoupanca, 2), Math.Round(disponivel, 2), totalSugerido, faltaParaCaber,
            usoGeral, nivelCartao, atencao, alerta, cartoes, avisos, itens);
    }

    /// <summary>
    /// Corta o grupo proporcionalmente até <paramref name="excesso"/> ou até <paramref name="corteMaximo"/> do total
    /// dele. Devolve o excesso que sobrou.
    /// </summary>
    private static decimal Cortar(List<Linha> grupo, decimal excesso, decimal corteMaximo)
    {
        if (excesso <= 0 || grupo.Count == 0) return excesso;
        var total = grupo.Sum(l => l.Valor!.Value);
        if (total <= 0) return excesso;
        var corte = Math.Min(excesso, total * corteMaximo);
        var fracao = corte / total;
        foreach (var l in grupo)
        {
            l.Valor *= 1 - fracao;
            l.Motivos.Add($"−{fracao * 100:0}% para caber na renda");
        }
        return excesso - corte;
    }

    /// <summary>Uso do limite por cartão: média das últimas faturas fechadas ÷ limite (cartões ativos com limite).</summary>
    private async Task<List<CartaoUsoLimiteDto>> UsoDosCartoesAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var cartoes = await _db.Contas.AsNoTracking()
            .Where(c => c.Tipo == TipoConta.CartaoCredito && !c.Arquivada && c.LimiteCredito != null && c.LimiteCredito > 0)
            .Select(c => new { c.Id, c.Nome, Limite = c.LimiteCredito!.Value }).ToListAsync(ct);
        var resultado = new List<CartaoUsoLimiteDto>();
        foreach (var c in cartoes)
        {
            var faturas = await _db.FaturasCartao.AsNoTracking()
                .Where(f => f.ContaId == c.Id && f.DataVencimento <= hoje)
                .OrderByDescending(f => f.DataVencimento).Take(FaturasParaMedia)
                .Select(f => f.ValorTotal).ToListAsync(ct);
            if (faturas.Count == 0) continue;
            var media = faturas.Average(v => Math.Abs(v));
            resultado.Add(new CartaoUsoLimiteDto(c.Id, c.Nome, c.Limite, Math.Round(media, 2), Math.Round(media / c.Limite * 100m, 1)));
        }
        return resultado.OrderByDescending(c => c.Percentual).ToList();
    }

    private static decimal Mediana(decimal[] valores)
    {
        if (valores.Length == 0) return 0;
        var ordenados = valores.Order().ToArray();
        var meio = ordenados.Length / 2;
        return ordenados.Length % 2 == 1 ? ordenados[meio] : (ordenados[meio - 1] + ordenados[meio]) / 2;
    }

    private static string NomeMes(DateOnly inicio, int indice)
        => inicio.AddMonths(Math.Max(0, indice)).ToString("MMM/yy", PtBr).Replace(".", "");

    private static string Reais(decimal valor) => valor.ToString("C0", PtBr);
}
