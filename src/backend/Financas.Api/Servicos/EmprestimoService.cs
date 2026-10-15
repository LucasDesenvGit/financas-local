using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>Regras para separar financiamentos dos demais empréstimos e nomear as modalidades do Open Finance.</summary>
public static class ClassificacaoCredito
{
    /// <summary>
    /// Financiamento = crédito vinculado à compra de um bem (imóvel, veículo, outros bens), conforme as modalidades
    /// do Open Finance Brasil. Todo o resto (crédito pessoal, consignado, cheque especial, home equity...) é empréstimo.
    /// </summary>
    public static NaturezaCredito Natureza(string? kind, string? modalidade, string? produto)
    {
        if (Contem(kind, "FINANCING")) return NaturezaCredito.Financiamento;
        if (Contem(modalidade, "FINANCIAMENTO") || Contem(modalidade, "AQUISICAO_BENS")) return NaturezaCredito.Financiamento;
        if (string.IsNullOrWhiteSpace(modalidade) && Contem(produto, "FINANC")) return NaturezaCredito.Financiamento;
        return NaturezaCredito.Emprestimo;
    }

    public static string DescreverModalidade(string? modalidade, NaturezaCredito natureza)
        => modalidade?.ToUpperInvariant() switch
        {
            "FINANCIAMENTO_HABITACIONAL_SFH" => "Financiamento habitacional (SFH)",
            "FINANCIAMENTO_HABITACIONAL_EXCETO_SFH" => "Financiamento habitacional",
            "AQUISICAO_BENS_VEICULOS_AUTOMOTORES" => "Financiamento de veículo",
            "AQUISICAO_BENS_OUTROS_BENS" => "Financiamento de bens",
            "CREDITO_PESSOAL_COM_CONSIGNACAO" => "Crédito consignado",
            "CREDITO_PESSOAL_SEM_CONSIGNACAO" => "Crédito pessoal",
            "HOME_EQUITY" => "Empréstimo com garantia de imóvel",
            "CHEQUE_ESPECIAL" => "Cheque especial",
            "CONTA_GARANTIDA" => "Conta garantida",
            "MICROCREDITO_PRODUTIVO_ORIENTADO" or "MICROCREDITO" => "Microcrédito",
            null or "" => natureza == NaturezaCredito.Financiamento ? "Financiamento" : "Empréstimo",
            var outra => System.Globalization.CultureInfo.GetCultureInfo("pt-BR").TextInfo
                .ToTitleCase(outra.Replace('_', ' ').ToLowerInvariant())
        };

    private static bool Contem(string? texto, string trecho)
        => texto is not null && texto.Contains(trecho, StringComparison.OrdinalIgnoreCase);
}

/// <summary>Empréstimos e financiamentos vindos do Open Finance, sempre com os totais separados por natureza.</summary>
public class EmprestimoService
{
    private readonly FinancasDbContext _db;

    public EmprestimoService(FinancasDbContext db) => _db = db;

    /// <summary>Saldo devedor e parcela mensal estimada, separados por natureza (só conexões ativas e contratos em aberto).</summary>
    public async Task<TotaisCreditoDto> TotaisAsync(CancellationToken ct)
    {
        var contratos = await ContratosAtivos()
            .Select(e => new { e.Natureza, e.SaldoDevedor, e.ValorParcela, e.Periodicidade })
            .ToListAsync(ct);
        decimal Saldo(NaturezaCredito n) => contratos.Where(c => c.Natureza == n).Sum(c => c.SaldoDevedor);
        decimal Parcela(NaturezaCredito n) => contratos.Where(c => c.Natureza == n).Sum(c => ParcelaMensal(c.ValorParcela, c.Periodicidade));
        return new TotaisCreditoDto(
            Saldo(NaturezaCredito.Financiamento), Saldo(NaturezaCredito.Emprestimo),
            Math.Round(Parcela(NaturezaCredito.Financiamento), 2), Math.Round(Parcela(NaturezaCredito.Emprestimo), 2));
    }

    public async Task<PainelEmprestimosDto> PainelAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var contratos = await _db.Emprestimos.AsNoTracking()
            .Where(e => e.ConexaoBancaria!.Ativa)
            .Select(e => new
            {
                Emprestimo = e,
                Instituicao = e.ConexaoBancaria!.NomeInstituicao,
                TotalPago = e.Pagamentos.Sum(p => (decimal?)p.Valor) ?? 0m,
                UltimoPagamento = e.Pagamentos.Max(p => (DateOnly?)p.Data)
            })
            .ToListAsync(ct);

        var lista = contratos
            .Select(c =>
            {
                var e = c.Emprestimo;
                return new EmprestimoDto(e.Id, e.Natureza, e.Produto, e.Modalidade,
                    ClassificacaoCredito.DescreverModalidade(e.Modalidade, e.Natureza), e.NumeroContrato, c.Instituicao,
                    e.Moeda, e.ValorContratado, e.SaldoDevedor, e.ValorParcela, ParcelaMensal(e.ValorParcela, e.Periodicidade),
                    e.ParcelasTotal, e.ParcelasPagas, e.ParcelasAVencer, e.ParcelasEmAtraso ?? 0, e.Periodicidade,
                    e.SistemaAmortizacao, e.TaxaJuros, e.PeriodicidadeTaxa, e.Indexador, e.Cet, e.DataContratacao,
                    e.DataVencimentoFinal, ProximoVencimento(e, hoje), c.TotalPago, c.UltimoPagamento, e.SaldoDevedor <= 0,
                    e.AtualizadoEm);
            })
            .OrderBy(e => e.Quitado).ThenByDescending(e => e.SaldoDevedor)
            .ToList();

        ResumoCreditoDto Resumir(NaturezaCredito n)
        {
            var itens = lista.Where(e => e.Natureza == n).ToList();
            var abertos = itens.Where(e => !e.Quitado).ToList();
            return new ResumoCreditoDto(n, abertos.Count, itens.Count - abertos.Count,
                abertos.Sum(e => e.SaldoDevedor), abertos.Sum(e => e.ValorContratado),
                Math.Round(abertos.Sum(e => e.ParcelaMensal), 2), itens.Sum(e => e.TotalPago),
                abertos.Sum(e => e.ParcelasEmAtraso));
        }

        // Pagamentos dos últimos 12 meses (inclui o atual), separados por natureza.
        var inicio = new DateOnly(hoje.Year, hoje.Month, 1).AddMonths(-11);
        var pagamentos = await _db.PagamentosEmprestimo.AsNoTracking()
            .Where(p => p.Data >= inicio && p.Data <= hoje && p.Emprestimo!.ConexaoBancaria!.Ativa)
            .GroupBy(p => new { p.Data.Year, p.Data.Month, p.Emprestimo!.Natureza })
            .Select(g => new { g.Key.Year, g.Key.Month, g.Key.Natureza, Soma = g.Sum(p => p.Valor) })
            .ToListAsync(ct);
        var meses = new List<PagamentosMesCreditoDto>();
        for (var m = inicio; m <= hoje; m = m.AddMonths(1))
        {
            decimal Soma(NaturezaCredito n) => pagamentos
                .Where(p => p.Year == m.Year && p.Month == m.Month && p.Natureza == n).Sum(p => p.Soma);
            meses.Add(new PagamentosMesCreditoDto(m.Year, m.Month, Soma(NaturezaCredito.Financiamento), Soma(NaturezaCredito.Emprestimo)));
        }

        return new PainelEmprestimosDto(Resumir(NaturezaCredito.Financiamento), Resumir(NaturezaCredito.Emprestimo),
            lista, meses);
    }

    private IQueryable<Emprestimo> ContratosAtivos()
        => _db.Emprestimos.AsNoTracking().Where(e => e.ConexaoBancaria!.Ativa && e.SaldoDevedor > 0);

    /// <summary>Converte a parcela para o equivalente mensal (contratos anuais, semestrais...).</summary>
    private static decimal ParcelaMensal(decimal? parcela, string? periodicidade)
    {
        if (parcela is not { } p || p <= 0) return 0m;
        return periodicidade?.ToUpperInvariant() switch
        {
            "YEARLY" or "ANUAL" => p / 12m,
            "SEMIANNUALLY" or "SEMESTRAL" => p / 6m,
            "QUARTERLY" or "TRIMESTRAL" => p / 3m,
            _ => p
        };
    }

    /// <summary>Previsão da próxima parcela para contratos mensais: 1ª parcela + parcelas já vencidas (pagas ou em atraso).</summary>
    private static DateOnly? ProximoVencimento(Emprestimo e, DateOnly hoje)
    {
        if (e.SaldoDevedor <= 0 || e.DataPrimeiraParcela is not { } primeira || e.ParcelasPagas is not { } pagas) return null;
        if (!string.Equals(e.Periodicidade, "MONTHLY", StringComparison.OrdinalIgnoreCase)) return null;
        var proxima = primeira.AddMonths(pagas + (e.ParcelasEmAtraso ?? 0));
        if (e.DataVencimentoFinal is { } fim && proxima > fim) return null;
        return proxima < hoje.AddYears(-1) ? null : proxima;
    }
}
