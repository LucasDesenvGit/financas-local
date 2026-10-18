using System.Text.RegularExpressions;
using Financas.Api.Data;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// Regras de descrição (#123, ADR 0030): "quando a descrição do banco for/contiver X, mostrar Y". Valem nas transações
/// que chegam pela sincronização e nas que já existem, mas só nas de nome automático: com nome do banco
/// (<c>Descricao == DescricaoOriginal</c>) ou dado por uma regra. Nome escolhido à mão (<c>DescricaoManual</c>) e
/// descrições editadas antes desta funcionalidade ficam como estão. A comparação é com a descrição do banco, sem acento
/// e sem diferença de maiúsculas, como nas regras de categoria.
/// </summary>
public partial class RegraDescricaoService
{
    private readonly FinancasDbContext _db;
    private List<(int Id, string Texto, TipoCorrespondencia Tipo, string NovaDescricao)> _regras = new();
    private bool _carregado;

    public RegraDescricaoService(FinancasDbContext db) => _db = db;

    /// <summary>Estado de antes de uma mudança de nome, para o "Desfazer".</summary>
    public sealed record EstadoAnterior(long Id, string Descricao, int? RegraDescricaoId, bool DescricaoManual);

    /// <summary>
    /// Regras ativas, da mais específica para a mais geral: "é igual a", depois "começa com", depois "contém"; no mesmo
    /// tipo, o texto mais longo primeiro e, empatando, a mais antiga.
    /// </summary>
    public async Task CarregarAsync(CancellationToken ct = default)
    {
        var regras = await _db.RegrasDescricao.AsNoTracking().Where(r => r.Ativa)
            .Select(r => new { r.Id, r.Texto, r.TipoCorrespondencia, r.NovaDescricao, r.CriadoEm })
            .ToListAsync(ct);
        _regras = regras
            .Select(r => (r.Id, Texto: CategorizacaoService.Normalizar(r.Texto), r.TipoCorrespondencia, r.NovaDescricao, r.CriadoEm))
            .Where(r => r.Texto.Length > 0)
            .OrderByDescending(r => (int)r.TipoCorrespondencia).ThenByDescending(r => r.Texto.Length).ThenBy(r => r.CriadoEm).ThenBy(r => r.Id)
            .Select(r => (r.Id, r.Texto, r.TipoCorrespondencia, r.NovaDescricao))
            .ToList();
        _carregado = true;
    }

    /// <summary>Nome automático: a transação veio do banco e o nome não foi escolhido à mão.</summary>
    public static bool NomeAutomatico(Transacao t)
        => !t.DescricaoManual && t.DescricaoOriginal is not null
           && (t.RegraDescricaoId is not null || t.Descricao == t.DescricaoOriginal);

    /// <summary>A regra que vale para essa descrição do banco, ou null.</summary>
    public (int Id, string NovaDescricao)? Resolver(string? descricaoOriginal)
    {
        if (!_carregado) throw new InvalidOperationException("Chame CarregarAsync antes de Resolver.");
        if (string.IsNullOrWhiteSpace(descricaoOriginal)) return null;
        var texto = CategorizacaoService.Normalizar(descricaoOriginal);
        foreach (var r in _regras)
            if (Corresponde(texto, r.Texto, r.Tipo)) return (r.Id, r.NovaDescricao);
        return null;
    }

    /// <summary>Dá à transação de nome automático o nome da regra (ou o do banco, sem regra). True se mudou.</summary>
    public bool Aplicar(Transacao t)
    {
        if (!NomeAutomatico(t)) return false;
        var regra = Resolver(t.DescricaoOriginal);
        var descricao = regra?.NovaDescricao ?? t.DescricaoOriginal!;
        if (t.Descricao == descricao && t.RegraDescricaoId == regra?.Id) return false;
        t.Descricao = descricao;
        t.RegraDescricaoId = regra?.Id;
        return true;
    }

    /// <summary>
    /// Reaplica as regras em todas as transações de nome automático (depois de criar, editar, desligar ou apagar uma
    /// regra). Devolve o estado de antes das que mudaram, para o "Desfazer". Não salva: chame SaveChangesAsync.
    /// </summary>
    public async Task<List<EstadoAnterior>> ReaplicarAsync(CancellationToken ct)
    {
        await CarregarAsync(ct);
        var transacoes = await _db.Transacoes
            .Where(t => !t.DescricaoManual && t.DescricaoOriginal != null
                        && (t.RegraDescricaoId != null || t.Descricao == t.DescricaoOriginal))
            .ToListAsync(ct);
        var anteriores = new List<EstadoAnterior>();
        foreach (var t in transacoes)
        {
            var antes = new EstadoAnterior(t.Id, t.Descricao, t.RegraDescricaoId, t.DescricaoManual);
            if (!Aplicar(t)) continue;
            t.AtualizadoEm = DateTime.UtcNow;
            anteriores.Add(antes);
        }
        return anteriores;
    }

    /// <summary>
    /// Quantas transações de nome automático (além de <paramref name="excetoId"/>) têm a descrição do banco que bate com
    /// o texto, e alguns exemplos, para a prévia de "usar também nas outras".
    /// </summary>
    public async Task<(int Quantidade, List<string> Exemplos)> PreviaAsync(string texto, TipoCorrespondencia tipo,
        long excetoId, CancellationToken ct)
    {
        var normalizado = CategorizacaoService.Normalizar(texto);
        if (normalizado.Length == 0) return (0, new());
        var candidatas = await _db.Transacoes.AsNoTracking()
            .Where(t => t.Id != excetoId && !t.DescricaoManual && t.DescricaoOriginal != null
                        && (t.RegraDescricaoId != null || t.Descricao == t.DescricaoOriginal))
            .Select(t => t.DescricaoOriginal!)
            .ToListAsync(ct);
        var batem = candidatas.Where(d => Corresponde(CategorizacaoService.Normalizar(d), normalizado, tipo)).ToList();
        var exemplos = batem.Distinct().Take(3).ToList();
        return (batem.Count, exemplos);
    }

    /// <summary>
    /// Texto sugerido para "contém": a descrição do banco sem os pedaços com número (que mudam a cada mês, como em
    /// "PIX ENVIADO 12345"). Sem sobrar ao menos 3 letras, fica a descrição inteira.
    /// </summary>
    public static string SugerirTexto(string descricaoOriginal)
    {
        var semNumeros = PedacoComNumero().Replace(descricaoOriginal, " ");
        semNumeros = Espacos().Replace(semNumeros, " ").Trim(' ', '-', '*', '/', '.', ',');
        return semNumeros.Count(char.IsLetter) >= 3 ? semNumeros : descricaoOriginal.Trim();
    }

    private static bool Corresponde(string textoNormalizado, string regraNormalizada, TipoCorrespondencia tipo) => tipo switch
    {
        TipoCorrespondencia.Igual => textoNormalizado == regraNormalizada,
        TipoCorrespondencia.ComecaCom => textoNormalizado.StartsWith(regraNormalizada, StringComparison.Ordinal),
        _ => textoNormalizado.Contains(regraNormalizada, StringComparison.Ordinal)
    };

    [GeneratedRegex(@"\S*\d\S*")]
    private static partial Regex PedacoComNumero();

    [GeneratedRegex(@"\s+")]
    private static partial Regex Espacos();
}
