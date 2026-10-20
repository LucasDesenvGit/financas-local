using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos.Cambio;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Investimentos;

/// <summary>
/// #173 e) (ADR 0045): histórico de saldo dos investimentos do Open Finance. O Open Finance só manda o saldo e o valor
/// aplicado de hoje; guardando um registro por dia, dá para saber quanto a caixinha ou o CDB rendeu em cada mês, como no
/// app do banco. Só renda fixa, fundos e outros em reais: a renda variável tem a carteira própria (FIIs e ações).
/// </summary>
public class HistoricoInvestimentosService
{
    private readonly FinancasDbContext _db;

    public HistoricoInvestimentosService(FinancasDbContext db) => _db = db;

    /// <summary>Entra no histórico: o que não é renda variável e está em reais.</summary>
    public static bool EntraNoHistorico(Investimento i)
        => i.Tipo is not ("EQUITY" or "ETF") && i.Moeda == CambioService.MoedaBase;

    /// <summary>
    /// Guarda o saldo de hoje de cada investimento (a última sincronização do dia vale). Não salva: a sincronização
    /// grava junto com os investimentos.
    /// </summary>
    public async Task RegistrarAsync(IReadOnlyCollection<Investimento> investimentos, CancellationToken ct)
    {
        var lista = investimentos.Where(EntraNoHistorico).ToList();
        if (lista.Count == 0) return;
        var hoje = Relogio.Hoje;
        var ids = lista.Select(i => i.PluggyInvestmentId).ToList();
        var doDia = await _db.HistoricosInvestimento.Where(h => h.Data == hoje && ids.Contains(h.PluggyInvestmentId))
            .ToDictionaryAsync(h => h.PluggyInvestmentId, ct);
        foreach (var i in lista)
        {
            if (!doDia.TryGetValue(i.PluggyInvestmentId, out var h))
            {
                h = new HistoricoInvestimento { PluggyInvestmentId = i.PluggyInvestmentId, Data = hoje };
                _db.HistoricosInvestimento.Add(h);
                doDia[i.PluggyInvestmentId] = h;
            }
            h.ConexaoBancariaId = i.ConexaoBancariaId;
            h.Nome = i.Nome;
            h.Tipo = i.Tipo;
            h.Saldo = i.Saldo;
            h.ValorAplicado = i.ValorAplicado;
            h.AtualizadoEm = DateTime.UtcNow;
        }
    }

    /// <summary>
    /// Rendimento de cada mês, por investimento: quanto o saldo cresceu além do valor aplicado ((saldo − aplicado) no fim
    /// do mês menos no fim do mês anterior). Assim, o que entrou e saiu não conta como rendimento. Sem o valor aplicado
    /// (alguns fundos), só o saldo. O mês em que o histórico começa conta a partir do primeiro registro (<c>Desde</c>).
    /// </summary>
    public async Task<HistoricoInvestimentosDto> MesAMesAsync(CancellationToken ct)
    {
        var registros = await _db.HistoricosInvestimento.AsNoTracking().OrderBy(h => h.Data).ToListAsync(ct);
        if (registros.Count == 0) return new HistoricoInvestimentosDto(null, new());
        var atuais = await _db.Investimentos.AsNoTracking()
            .Select(i => new { i.PluggyInvestmentId, i.Nome, i.Tipo, i.Emissor, i.NomePersonalizado })
            .ToDictionaryAsync(i => i.PluggyInvestmentId, ct);
        string Nome(HistoricoInvestimento h) => atuais.TryGetValue(h.PluggyInvestmentId, out var i)
            ? NomeInvestimento.Exibicao(i.Nome, i.Tipo, i.Emissor, i.NomePersonalizado)
            : NomeInvestimento.Exibicao(h.Nome, h.Tipo, null, null);

        var porInvestimento = registros.GroupBy(r => r.PluggyInvestmentId).Select(g => g.ToList()).ToList();
        var primeiro = registros[0].Data;
        var meses = new List<MesInvestimentosOfDto>();
        for (var mes = new DateOnly(primeiro.Year, primeiro.Month, 1); mes <= Relogio.Hoje; mes = mes.AddMonths(1))
        {
            var fimDoMes = Relogio.UltimoDiaMes(mes.Year, mes.Month);
            var doMes = registros.Where(r => r.Data >= mes && r.Data <= fimDoMes).ToList();
            if (doMes.Count == 0) continue;
            // O saldo no fim é o do último dia com registro no mês (o investimento resgatado antes dele não entra).
            var ultimoDia = doMes.Max(r => r.Data);
            var itens = new List<RendimentoInvestimentoMesDto>();
            foreach (var serie in porInvestimento)
            {
                var fim = serie.LastOrDefault(r => r.Data <= fimDoMes);
                if (fim is null || fim.Data < mes) continue;
                var inicio = serie.LastOrDefault(r => r.Data < mes) ?? serie.First(r => r.Data >= mes);
                decimal? rendimento = inicio.ValorAplicado is decimal aplicadoAntes && fim.ValorAplicado is decimal aplicadoDepois
                    ? Math.Round(fim.Saldo - aplicadoDepois - (inicio.Saldo - aplicadoAntes), 2)
                    : null;
                itens.Add(new RendimentoInvestimentoMesDto(Nome(fim), rendimento, fim.Data == ultimoDia ? fim.Saldo : 0m));
            }
            var comRendimento = itens.Where(i => i.Rendimento is not null).ToList();
            meses.Add(new MesInvestimentosOfDto(mes.Year, mes.Month,
                primeiro > mes ? primeiro : null,
                comRendimento.Count > 0 ? comRendimento.Sum(i => i.Rendimento!.Value) : null,
                itens.Sum(i => i.Saldo),
                itens.OrderByDescending(i => i.Saldo).ThenBy(i => i.Nome).ToList()));
        }
        return new HistoricoInvestimentosDto(primeiro, meses);
    }
}
