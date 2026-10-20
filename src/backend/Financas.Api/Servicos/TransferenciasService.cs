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
/// <item>ou é dinheiro guardado numa reserva ("Dinheiro reservado/retirado");</item>
/// <item>sem titular informado, cita o seu nome como aparece nos Pix (Configurações › Open Finance, #172).</item>
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

    /// <summary>#172: o seu nome como aparece nos Pix (Configuracao), para quando o banco não informa o titular.</summary>
    public const string ChaveMeuNome = "Transferencias.MeuNome";

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
        var cartoes = (await _db.Contas.AsNoTracking().Where(c => c.Tipo == TipoConta.CartaoCredito).Select(c => c.Id)
            .ToListAsync(ct)).ToHashSet();
        var mudaram = await PagamentosDeFaturaAsync(cartoes, ct);
        if (await CategoriaGenericaAsync(ct) is not int categoria) return mudaram;
        var lancamentos = await _db.Transacoes.Where(t => t.CategoriaId == categoria).ToListAsync(ct);
        if (lancamentos.Count == 0) return mudaram;

        var pareados = await PareadosAsync(lancamentos, ct);
        var meuNome = PalavrasDoNome(await _db.Configuracoes.AsNoTracking().Where(c => c.Chave == ChaveMeuNome)
            .Select(c => c.Valor).FirstOrDefaultAsync(ct));
        foreach (var t in lancamentos)
        {
            if (EntreContasSuas(t, pareados.Contains(t.Id), meuNome))
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

    /// <summary>
    /// Entre contas suas: lançada pelo site, mesmo titular, com par, reserva ou (#172) Pix que cita o seu nome. O titular
    /// informado pelo banco vale sempre: outro titular nunca é.
    /// </summary>
    public static bool EntreContasSuas(Transacao t, bool temPar, string[]? meuNome = null)
    {
        if (t.TransferenciaGrupo is not null) return true;
        if (t.MesmaTitularidade is bool mesmo) return mesmo;
        return temPar || ReservaService.EhMovimentoDeReserva(t.DescricaoOriginal ?? t.Descricao)
            || (meuNome is not null && CitaONome(t.DescricaoOriginal ?? t.Descricao, meuNome));
    }

    private static readonly HashSet<string> Conectivos = new(StringComparer.Ordinal) { "DE", "DA", "DO", "DAS", "DOS", "E" };

    /// <summary>
    /// #172: o primeiro e o último nome (sem acento, maiúsculas), que os bancos mantêm mesmo abreviando o meio
    /// ("LUCAS F SILVA"). Null com menos de dois nomes: um nome só pegaria Pix de qualquer xará.
    /// </summary>
    public static string[]? PalavrasDoNome(string? nome)
    {
        var palavras = CategorizacaoService.Normalizar(nome)
            .Split(new[] { ' ', '.', ',', '-' }, StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)
            .Where(p => p.Length >= 2 && !Conectivos.Contains(p)).ToArray();
        return palavras.Length >= 2 ? new[] { palavras[0], palavras[^1] } : null;
    }

    /// <summary>A descrição tem o primeiro e o último nome como palavras inteiras.</summary>
    public static bool CitaONome(string? descricao, string[] palavras)
    {
        var texto = " " + string.Concat(CategorizacaoService.Normalizar(descricao)
            .Select(ch => char.IsLetterOrDigit(ch) ? ch : ' ')) + " ";
        return palavras.All(p => texto.Contains(" " + p + " ", StringComparison.Ordinal));
    }

    /// <summary>
    /// #172: débito numa conta que é pagamento de fatura (pelo texto ou pela operação, <c>TipoLancamentoService</c>) e tem o
    /// crédito do mesmo valor num cartão seu, em até <see cref="DiasDoPar"/> dias, vira transferência na categoria
    /// "Pagamento de fatura", mesmo que o banco tenha mandado como boleto ou "outros". Sem esse crédito fica como está:
    /// "fatura" também é conta de telefone e de luz. Não mexe em categoria escolhida à mão. Não salva.
    /// </summary>
    private async Task<int> PagamentosDeFaturaAsync(HashSet<int> cartoes, CancellationToken ct)
    {
        if (cartoes.Count == 0) return 0;
        var transferencias = await _db.Categorias.AsNoTracking().Where(c => c.Tipo == TipoCategoria.Transferencia)
            .Select(c => new { c.Id, c.Nome }).ToListAsync(ct);
        var categoria = transferencias.FirstOrDefault(c => CategorizacaoService.Normalizar(c.Nome) == "PAGAMENTO DE FATURA")?.Id
            ?? await CategoriaGenericaAsync(ct);
        if (categoria is not int pagamento) return 0;
        var idsTransferencia = transferencias.Select(c => c.Id).ToList();
        var idsCartoes = cartoes.ToList();

        // Filtro grosso no banco; a regra certa é a do TipoLancamentoService.
        var candidatos = (await _db.Transacoes
                .Where(t => t.Valor < 0 && !t.CategoriaManual && t.TransferenciaGrupo == null
                            && !idsCartoes.Contains(t.ContaId)
                            && (t.CategoriaId == null || !idsTransferencia.Contains(t.CategoriaId.Value))
                            && (t.MeioPagamento == "PAGAMENTO_FATURA"
                                || EF.Functions.Like(t.DescricaoOriginal ?? t.Descricao, "%fatura%")
                                || EF.Functions.Like(t.DescricaoOriginal ?? t.Descricao, "%cart%")))
                .ToListAsync(ct))
            .Where(t => TipoLancamentoService.Classificar(t.Valor, t.Descricao, t.DescricaoOriginal, t.MeioPagamento,
                t.MesmaTitularidade, false, TipoConta.ContaCorrente) == TipoLancamento.PagamentoFatura)
            .ToList();
        if (candidatos.Count == 0) return 0;

        var de = candidatos.Min(t => t.Data).AddDays(-DiasDoPar);
        var ate = candidatos.Max(t => t.Data).AddDays(DiasDoPar);
        var creditos = await _db.Transacoes.AsNoTracking()
            .Where(t => idsCartoes.Contains(t.ContaId) && t.Valor > 0 && t.Data >= de && t.Data <= ate)
            .Select(t => new { t.Id, t.Data, t.Valor }).ToListAsync(ct);
        var usados = new HashSet<long>();
        var mudaram = 0;
        foreach (var t in candidatos.OrderBy(t => t.Data).ThenBy(t => t.Id))
        {
            var par = creditos.Where(c => c.Valor == -t.Valor && !usados.Contains(c.Id)
                    && Math.Abs(c.Data.DayNumber - t.Data.DayNumber) <= DiasDoPar)
                .OrderBy(c => Math.Abs(c.Data.DayNumber - t.Data.DayNumber)).FirstOrDefault();
            if (par is null) continue;
            usados.Add(par.Id);
            t.CategoriaId = pagamento;
            t.Tipo = TipoTransacao.Transferencia;
            t.IgnorarRelatorios = true;
            t.AtualizadoEm = DateTime.UtcNow;
            mudaram++;
        }
        return mudaram;
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
