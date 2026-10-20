using Financas.Api.Data;
using Financas.Api.Entidades;
using Financas.Api.Servicos.Investimentos;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// Transferência só fica fora dos relatórios quando é entre contas suas (ADR 0031). Vale para a categoria genérica
/// "Transferências", que recebe as transferências do Open Finance (Pix, TED, "Transfers"). Cada lançamento dela é:
/// <list type="bullet">
/// <item><b>Entre contas suas</b>, fora dos relatórios, quando:
/// <list type="bullet">
/// <item>foi lançado pelo site como transferência entre contas;</item>
/// <item>o banco informa o mesmo titular;</item>
/// <item>tem o par em outra conta sua (mesmo valor, sinal oposto, até 5 dias);</item>
/// <item>ou é dinheiro guardado numa reserva ("Dinheiro reservado/retirado").</item>
/// </list></item>
/// <item><b>Para ou de outra pessoa</b>, contando como despesa ou receita: o resto, sempre que o banco informa outro
/// titular.</item>
/// </list>
/// As outras categorias do tipo transferência (Investimentos, Pagamento de fatura e as que você criar, como "Entre
/// minhas contas") continuam sempre fora dos relatórios: são movimentos seus por natureza.
/// </summary>
public class TransferenciasService
{
    /// <summary>Até quantos dias de diferença os dois lados de uma transferência entre contas suas aparecem.</summary>
    public const int DiasDoPar = 5;

    private readonly FinancasDbContext _db;

    public TransferenciasService(FinancasDbContext db) => _db = db;

    /// <summary>A categoria genérica de transferências ("Transferências"/"Transferência", do tipo transferência).</summary>
    public async Task<int?> CategoriaGenericaAsync(CancellationToken ct)
    {
        var candidatas = await _db.Categorias.AsNoTracking().Where(c => c.Tipo == TipoCategoria.Transferencia)
            .Select(c => new { c.Id, c.Nome }).ToListAsync(ct);
        return candidatas.FirstOrDefault(c => CategorizacaoService.Normalizar(c.Nome) is "TRANSFERENCIAS" or "TRANSFERENCIA")?.Id;
    }

    /// <summary>
    /// Decide, para cada lançamento da categoria genérica, se é entre contas suas (fora dos relatórios) ou com outra
    /// pessoa (despesa ou receita). Roda no fim da sincronização, ao iniciar a API e depois de mudar categorias.
    /// Não salva: chame SaveChangesAsync. Devolve quantos mudaram.
    /// </summary>
    public async Task<int> ReclassificarAsync(CancellationToken ct)
    {
        if (await CategoriaGenericaAsync(ct) is not int categoria) return 0;
        var lancamentos = await _db.Transacoes.Where(t => t.CategoriaId == categoria).ToListAsync(ct);
        if (lancamentos.Count == 0) return 0;

        var pareados = await PareadosAsync(lancamentos, ct);
        var cartoes = (await _db.Contas.AsNoTracking().Where(c => c.Tipo == TipoConta.CartaoCredito).Select(c => c.Id)
            .ToListAsync(ct)).ToHashSet();
        var mudaram = 0;
        foreach (var t in lancamentos)
        {
            if (EntreContasSuas(t, pareados.Contains(t.Id)))
            {
                if (t.Tipo == TipoTransacao.Transferencia && t.IgnorarRelatorios) continue;
                t.Tipo = TipoTransacao.Transferencia;
                t.IgnorarRelatorios = true;
            }
            else
            {
                // Só muda o que ainda está como transferência (a categorização acabou de marcar): um lançamento que
                // você mandou ignorar nos relatórios depois de reclassificado continua ignorado.
                if (t.Tipo != TipoTransacao.Transferencia) continue;
                t.Tipo = CategorizacaoService.TipoPeloSinal(t.Valor, cartoes.Contains(t.ContaId));
                t.IgnorarRelatorios = false;
            }
            t.AtualizadoEm = DateTime.UtcNow;
            mudaram++;
        }
        return mudaram;
    }

    /// <summary>Entre contas suas: lançada pelo site, mesmo titular, com par, ou reserva. Outro titular nunca é.</summary>
    public static bool EntreContasSuas(Transacao t, bool temPar)
    {
        if (t.TransferenciaGrupo is not null) return true;
        if (t.MesmaTitularidade is bool mesmo) return mesmo;
        return temPar || ReservaService.EhMovimentoDeReserva(t.DescricaoOriginal ?? t.Descricao);
    }

    /// <summary>
    /// Lançamentos que têm o outro lado em outra conta sua: mesmo valor com o sinal oposto, até <see cref="DiasDoPar"/>
    /// dias de diferença, em qualquer categoria. Cada lado forma um par só, o de data mais próxima.
    /// </summary>
    private async Task<HashSet<long>> PareadosAsync(List<Transacao> lancamentos, CancellationToken ct)
    {
        var de = lancamentos.Min(t => t.Data).AddDays(-DiasDoPar);
        var ate = lancamentos.Max(t => t.Data).AddDays(DiasDoPar);
        var todas = await _db.Transacoes.AsNoTracking().Where(t => t.Data >= de && t.Data <= ate)
            .Select(t => new { t.Id, t.ContaId, t.Data, t.Valor }).ToListAsync(ct);
        var porValor = todas.GroupBy(t => t.Valor).ToDictionary(g => g.Key, g => g.ToList());

        var usados = new HashSet<long>();
        foreach (var t in lancamentos.OrderBy(t => t.Data).ThenBy(t => t.Id))
        {
            if (usados.Contains(t.Id) || t.Valor == 0 || !porValor.TryGetValue(-t.Valor, out var opostos)) continue;
            var par = opostos
                .Where(o => o.ContaId != t.ContaId && !usados.Contains(o.Id) && Math.Abs(o.Data.DayNumber - t.Data.DayNumber) <= DiasDoPar)
                .OrderBy(o => Math.Abs(o.Data.DayNumber - t.Data.DayNumber)).ThenBy(o => o.Id)
                .FirstOrDefault();
            if (par is null) continue;
            usados.Add(t.Id);
            usados.Add(par.Id);
        }
        return usados;
    }
}
