using System.Globalization;
using System.Text.RegularExpressions;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Investimentos;

/// <summary>
/// Lê as transações do extrato procurando códigos da B3 (ex.: "COMPRA MXRF11 10 x 9,85" ou "RENDIMENTO HGLG11").
/// Compras e vendas viram sugestões de operações para a carteira; créditos de rendimento viram "proventos recebidos".
/// Quando o extrato não traz quantidade ou preço, eles são calculados pelo valor da transação e pela cotação do dia.
/// </summary>
public partial class ExtratoCarteiraService
{
    private const int AnosAnalisados = 10;

    private readonly FinancasDbContext _db;
    private readonly CotacaoService _cotacoes;

    public ExtratoCarteiraService(FinancasDbContext db, CotacaoService cotacoes)
    {
        _db = db;
        _cotacoes = cotacoes;
    }

    [GeneratedRegex(@"\b([A-Z]{4}(?:11|12|13|31|32|33|34|35|39|3|4|5|6|7|8))F?\b")]
    private static partial Regex RegexTicker();

    [GeneratedRegex(@"\b(REND|RENDIMENTO|RENDIMENTOS|DIVID|DIVIDENDO|DIVIDENDOS|PROVENTO|PROVENTOS|JCP|JUROS S|JRS CAP|JUROS SOBRE CAPITAL|AMORTIZ|AMORTIZACAO|AMORTIZAÇÃO)")]
    private static partial Regex RegexProvento();

    [GeneratedRegex(@"\b(COMPRA|COMPRAS|C/|BUY|APLICACAO|APLICAÇÃO|LIQUIDACAO|LIQUIDAÇÃO|NOTA|CORRETAGEM|B3|BOVESPA|ACOES|AÇÕES|FII|CORRETORA|INVEST)")]
    private static partial Regex RegexInvestimento();

    [GeneratedRegex(@"\b(VENDA|VENDAS|V/|SELL)\b")]
    private static partial Regex RegexVenda();

    [GeneratedRegex(@"\b\d{1,2}/\d{1,2}(/\d{2,4})?\b")]
    private static partial Regex RegexData();

    [GeneratedRegex(@"(?<![\d,.])(\d{1,3}(?:\.\d{3})+(?:,\d+)?|\d+,\d+|\d+\.\d+|\d+)(?![\d,.])")]
    private static partial Regex RegexNumero();

    private sealed record Candidata(long Id, DateOnly Data, string Descricao, decimal Valor, string ContaNome,
        bool ContaInvestimento, bool CategoriaInvestimento);

    /// <summary>Transações com código de ativo, já classificadas.</summary>
    private async Task<List<(Candidata T, string Ticker, bool Provento)>> LocalizarAsync(bool somenteProventos,
        CancellationToken ct)
    {
        var desde = Relogio.Hoje.AddYears(-AnosAnalisados);
        var idCategoriaInvestimentos = await _db.Categorias.AsNoTracking()
            .Where(c => c.Nome == "Investimentos").Select(c => (int?)c.Id).FirstOrDefaultAsync(ct);
        var tickersCarteira = (await _db.Ativos.AsNoTracking().Select(a => a.Ticker).ToListAsync(ct)).ToHashSet();

        var consulta = _db.Transacoes.AsNoTracking().Where(t => t.Data >= desde && t.TransferenciaGrupo == null);
        if (somenteProventos)
            consulta = consulta.Where(t => t.Valor > 0 && (
                t.Descricao.Contains("REND") || t.Descricao.Contains("DIVID") || t.Descricao.Contains("PROVENT")
                || t.Descricao.Contains("JCP") || t.Descricao.Contains("JUROS") || t.Descricao.Contains("JRS")
                || t.Descricao.Contains("AMORT")
                || (t.DescricaoOriginal != null && (t.DescricaoOriginal.Contains("REND") || t.DescricaoOriginal.Contains("DIVID")
                    || t.DescricaoOriginal.Contains("PROVENT") || t.DescricaoOriginal.Contains("JCP")
                    || t.DescricaoOriginal.Contains("JUROS") || t.DescricaoOriginal.Contains("AMORT")))));
        var transacoes = await consulta
            .Select(t => new
            {
                t.Id, t.Data, t.Descricao, t.DescricaoOriginal, t.Valor, t.CategoriaId,
                ContaNome = t.Conta!.Nome, ContaTipo = t.Conta.Tipo
            })
            .ToListAsync(ct);

        var lista = new List<(Candidata, string, bool)>();
        foreach (var t in transacoes)
        {
            var texto = $"{t.Descricao} {t.DescricaoOriginal}".ToUpperInvariant();
            var achado = RegexTicker().Match(texto);
            if (!achado.Success) continue;
            var ticker = CotacaoService.NormalizarTicker(achado.Groups[1].Value);
            if (ticker is null) continue;

            var contaInvest = t.ContaTipo == TipoConta.Investimento;
            var categoriaInvest = idCategoriaInvestimentos is int idc && t.CategoriaId == idc;
            var provento = t.Valor > 0 && RegexProvento().IsMatch(texto);
            // Evita falsos positivos: exige contexto de investimento ou um ativo que já está na carteira.
            var contexto = provento || contaInvest || categoriaInvest || tickersCarteira.Contains(ticker)
                           || RegexInvestimento().IsMatch(texto) || RegexVenda().IsMatch(texto);
            if (!contexto) continue;

            var descricao = string.IsNullOrWhiteSpace(t.DescricaoOriginal) ? t.Descricao : t.DescricaoOriginal!;
            lista.Add((new Candidata(t.Id, t.Data, descricao, t.Valor, t.ContaNome, contaInvest, categoriaInvest),
                ticker, provento));
        }
        return lista;
    }

    /// <summary>Proventos creditados no extrato, por ativo (para comparar com o estimado pela data-com).</summary>
    public async Task<List<(string Ticker, DateOnly Data, decimal Valor)>> ProventosRecebidosAsync(CancellationToken ct)
        => (await LocalizarAsync(true, ct)).Where(x => x.Provento)
            .Select(x => (x.Ticker, x.T.Data, x.T.Valor)).ToList();

    public async Task<List<SugestaoExtratoDto>> SugestoesAsync(CancellationToken ct)
    {
        var achados = (await LocalizarAsync(false, ct)).Where(x => !x.Provento).ToList();
        if (achados.Count == 0) return new();

        var vinculadas = (await _db.OperacoesAtivo.AsNoTracking().Where(o => o.TransacaoId != null)
            .Select(o => o.TransacaoId!.Value).ToListAsync(ct)).ToHashSet();
        var operacoes = await _db.OperacoesAtivo.AsNoTracking()
            .Select(o => new { o.Ativo!.Ticker, o.Data, o.Tipo }).ToListAsync(ct);

        var resultado = new List<SugestaoExtratoDto>();
        foreach (var grupo in achados.GroupBy(x => x.Ticker))
        {
            var de = grupo.Min(x => x.T.Data);
            var ate = grupo.Max(x => x.T.Data);
            var fechamentos = await _cotacoes.FechamentosDiariosAsync(grupo.Key, de, ate, ct);

            foreach (var (t, ticker, _) in grupo.OrderByDescending(x => x.T.Data))
            {
                var texto = t.Descricao.ToUpperInvariant();
                var tipo = RegexVenda().IsMatch(texto) || (t.Valor > 0 && !texto.Contains("COMPRA"))
                    ? TipoOperacao.Venda : TipoOperacao.Compra;
                var valorAbs = Math.Abs(t.Valor);
                var fechamento = FechamentoEm(fechamentos, t.Data);
                var (quantidade, preco, origem) = Interpretar(texto, ticker, valorAbs, fechamento);

                var duplicata = operacoes.Any(o => o.Ticker == ticker && o.Tipo == tipo
                    && Math.Abs(o.Data.DayNumber - t.Data.DayNumber) <= 3);
                resultado.Add(new SugestaoExtratoDto(t.Id, t.Data, t.Descricao, t.Valor, t.ContaNome, ticker, tipo,
                    quantidade, preco, fechamento, origem, vinculadas.Contains(t.Id), duplicata));
            }
        }
        return resultado.OrderByDescending(s => s.Data).ThenBy(s => s.Ticker).ToList();
    }

    public async Task<int> ImportarAsync(List<ImportarExtratoItemDto> itens, CancellationToken ct)
    {
        var ids = itens.Select(i => i.TransacaoId).Distinct().ToList();
        var transacoes = await _db.Transacoes.AsNoTracking().Where(t => ids.Contains(t.Id))
            .Select(t => new { t.Id, t.Data }).ToDictionaryAsync(t => t.Id, ct);
        var vinculadas = (await _db.OperacoesAtivo.AsNoTracking()
            .Where(o => o.TransacaoId != null && ids.Contains(o.TransacaoId.Value))
            .Select(o => o.TransacaoId!.Value).ToListAsync(ct)).ToHashSet();

        var novosAtivos = new List<Ativo>();
        var criadas = 0;
        foreach (var item in itens)
        {
            if (!transacoes.TryGetValue(item.TransacaoId, out var t) || !vinculadas.Add(item.TransacaoId)) continue;
            var ticker = CotacaoService.NormalizarTicker(item.Ticker);
            if (ticker is null || item.Quantidade <= 0 || item.PrecoUnitario < 0 || !Enum.IsDefined(item.Tipo)) continue;

            var ativo = _db.Ativos.Local.FirstOrDefault(a => a.Ticker == ticker)
                        ?? await _db.Ativos.FirstOrDefaultAsync(a => a.Ticker == ticker, ct);
            if (ativo is null)
            {
                ativo = new Ativo { Ticker = ticker, Tipo = ticker.EndsWith("11", StringComparison.Ordinal) ? TipoAtivo.Fii : TipoAtivo.Acao };
                _db.Ativos.Add(ativo);
                novosAtivos.Add(ativo);
            }
            _db.OperacoesAtivo.Add(new OperacaoAtivo
            {
                Ativo = ativo,
                Tipo = item.Tipo,
                Data = t.Data,
                Quantidade = Math.Round(item.Quantidade, 6),
                PrecoUnitario = Math.Round(item.PrecoUnitario, 6),
                Taxas = Math.Round(Math.Max(0, item.Taxas), 2),
                Origem = OrigemOperacao.Extrato,
                TransacaoId = item.TransacaoId,
                Observacao = "Importada do extrato"
            });
            criadas++;
        }
        await _db.SaveChangesAsync(ct);

        if (novosAtivos.Count > 0)
        {
            try { await _cotacoes.AtualizarAsync(novosAtivos.Select(a => a.Id).ToList(), ct); }
            catch (Exception ex) when (ex is not OperationCanceledException) { /* cotação fica para depois */ }
        }
        return criadas;
    }

    /* ------------------------------ interpretação do texto ------------------------------ */

    private static decimal? FechamentoEm(SortedDictionary<DateOnly, decimal> fechamentos, DateOnly data)
    {
        if (fechamentos.Count == 0) return null;
        if (fechamentos.TryGetValue(data, out var exato)) return exato;
        // Último pregão até a data (a liquidação costuma vir 2 dias depois do pregão).
        var anterior = fechamentos.Where(f => f.Key <= data).Select(f => (decimal?)f.Value).LastOrDefault();
        return anterior ?? fechamentos.First().Value;
    }

    /// <summary>Descobre quantidade e preço: primeiro pelo texto; depois pelo valor e pela cotação do dia.</summary>
    private static (decimal? Quantidade, decimal? Preco, string? Origem) Interpretar(string texto, string ticker,
        decimal valor, decimal? fechamento)
    {
        var limpo = RegexData().Replace(texto.Replace(ticker, " "), " ");
        var inteiros = new List<decimal>();
        var decimais = new List<decimal>();
        foreach (Match m in RegexNumero().Matches(limpo))
        {
            var bruto = m.Value;
            if (bruto.Contains(',') || (bruto.Contains('.') && !Regex.IsMatch(bruto, @"^\d{1,3}(\.\d{3})+$")))
            {
                var normalizado = bruto.Contains(',') ? bruto.Replace(".", "").Replace(',', '.') : bruto;
                if (decimal.TryParse(normalizado, NumberStyles.Number, CultureInfo.InvariantCulture, out var d) && d > 0)
                    decimais.Add(d);
            }
            else if (decimal.TryParse(bruto.Replace(".", ""), NumberStyles.Integer, CultureInfo.InvariantCulture, out var i)
                     && i > 0 && i < 10_000_000)
                inteiros.Add(i);
        }

        if (valor > 0)
        {
            // 1) Quantidade e preço no texto, conferindo com o valor da transação (até 3% de diferença por taxas).
            foreach (var q in inteiros)
                foreach (var p in decimais)
                    if (Math.Abs(q * p - valor) / valor <= 0.03m) return (q, p, "extrato");

            // 2) Só a quantidade no texto: preço = valor / quantidade, se for plausível perto da cotação do dia.
            foreach (var q in inteiros)
            {
                var p = valor / q;
                if (fechamento is null || (p >= fechamento * 0.7m && p <= fechamento * 1.3m))
                    return (q, Math.Round(p, 4), "quantidade do extrato");
            }

            // 3) Só o preço no texto (diferente do próprio valor da transação).
            foreach (var p in decimais.Where(d => Math.Abs(d - valor) > 0.01m))
            {
                var q = Math.Round(valor / p);
                if (q >= 1 && Math.Abs(q * p - valor) / valor <= 0.03m) return (q, p, "preço do extrato");
            }

            // 4) Nada no texto: quantidade estimada pela cotação de fechamento do dia.
            if (fechamento is decimal f && f > 0)
            {
                var q = Math.Round(valor / f);
                if (q >= 1) return (q, Math.Round(valor / q, 4), "cotação do dia");
            }
        }
        return (null, fechamento, null);
    }
}
