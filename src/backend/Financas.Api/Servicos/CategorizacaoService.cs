using System.Globalization;
using System.Text;
using Financas.Api.Data;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// Define a categoria das transações. Ordem de prioridade:
/// 1) categoria escolhida manualmente pelo usuário (nunca sobrescrita);
/// 2) regras de categorização do usuário;
/// 3) mapeamento da categoria sugerida pela Pluggy.
/// </summary>
public class CategorizacaoService
{
    /// <summary>Sugestão inicial de mapeamento (descrições da Pluggy em inglês → categorias padrão).</summary>
    private static readonly Dictionary<string, string> MapaPadraoPluggy = new(StringComparer.OrdinalIgnoreCase)
    {
        ["Salary"] = "Salário",
        ["Income"] = "Outras receitas",
        ["Retirement"] = "Outras receitas",
        ["Entrepreneurial activities"] = "Outras receitas",
        ["Government aid"] = "Outras receitas",
        ["Non-recurring income"] = "Outras receitas",
        ["Proceeds interests and dividends"] = "Rendimentos",
        ["Investment income"] = "Rendimentos",
        ["Investments"] = "Investimentos",
        ["Automatic investment"] = "Investimentos",
        ["Same person transfer"] = "Transferências",
        ["Transfers"] = "Transferências",
        ["Transfer - PIX"] = "Transferências",
        ["Transfer - TED"] = "Transferências",
        ["Credit card payment"] = "Pagamento de fatura",
        ["Loans and financing"] = "Juros e empréstimos",
        ["Interests charged"] = "Juros e empréstimos",
        ["Late payment and overdraft costs"] = "Juros e empréstimos",
        ["Services"] = "Serviços e assinaturas",
        ["Telecommunications"] = "Serviços e assinaturas",
        ["Digital services"] = "Serviços e assinaturas",
        ["Video streaming"] = "Serviços e assinaturas",
        ["Music streaming"] = "Serviços e assinaturas",
        ["Shopping"] = "Compras",
        ["Online shopping"] = "Compras",
        ["Electronics"] = "Compras",
        ["Clothing"] = "Compras",
        ["Groceries"] = "Mercado",
        ["Food and drinks"] = "Alimentação",
        ["Eating out"] = "Alimentação",
        ["Food delivery"] = "Alimentação",
        ["Travel"] = "Viagens",
        ["Accomodation"] = "Viagens",
        ["Airport and airlines"] = "Viagens",
        ["Donations"] = "Doações",
        ["Taxes"] = "Impostos e taxas",
        ["Legal obligations"] = "Impostos e taxas",
        ["Bank fees"] = "Tarifas bancárias",
        ["Housing"] = "Moradia",
        ["Rent"] = "Moradia",
        ["Utilities"] = "Moradia",
        ["Healthcare"] = "Saúde",
        ["Pharmacy"] = "Saúde",
        ["Transportation"] = "Transporte",
        ["Automotive"] = "Transporte",
        ["Gas stations"] = "Transporte",
        ["Taxi and ride-hailing"] = "Transporte",
        ["Insurance"] = "Seguros",
        ["Leisure"] = "Lazer",
        ["Gambling"] = "Lazer",
        ["Education"] = "Educação",
        ["Pet supplies and vet"] = "Pets",
        ["Wellness and fitness"] = "Cuidados pessoais",
        ["Personal care"] = "Cuidados pessoais",
    };

    private readonly FinancasDbContext _db;
    private List<(string TextoNormalizado, TipoCorrespondencia Tipo, int CategoriaId)> _regras = new();
    private Dictionary<int, TipoCategoria> _tiposCategoria = new();
    private Dictionary<string, int?> _mapaPluggy = new();
    private bool _carregado;

    public CategorizacaoService(FinancasDbContext db) => _db = db;

    public async Task CarregarAsync(CancellationToken ct = default)
    {
        var regras = await _db.RegrasCategorizacao.AsNoTracking()
            .Where(r => r.Ativa)
            .OrderBy(r => r.Prioridade).ThenBy(r => r.Id)
            .Select(r => new { r.Texto, r.TipoCorrespondencia, r.CategoriaId })
            .ToListAsync(ct);
        _regras = regras.Select(r => (Normalizar(r.Texto), r.TipoCorrespondencia, r.CategoriaId)).ToList();

        _tiposCategoria = await _db.Categorias.AsNoTracking().ToDictionaryAsync(c => c.Id, c => c.Tipo, ct);

        var mapas = await _db.CategoriasPluggy.AsNoTracking()
            .Select(c => new { c.PluggyCategoriaId, c.CategoriaId, c.PaiPluggyId })
            .ToListAsync(ct);
        var porId = mapas.ToDictionary(m => m.PluggyCategoriaId, m => m.CategoriaId);
        _mapaPluggy = new Dictionary<string, int?>();
        foreach (var m in mapas)
        {
            // Se a subcategoria não tiver mapeamento, herda o da categoria pai.
            var categoria = m.CategoriaId;
            if (categoria is null && m.PaiPluggyId is not null && porId.TryGetValue(m.PaiPluggyId, out var doPai))
                categoria = doPai;
            _mapaPluggy[m.PluggyCategoriaId] = categoria;
        }
        _carregado = true;
    }

    public int? Resolver(string descricao, string? pluggyCategoriaId)
    {
        if (!_carregado) throw new InvalidOperationException("Chame CarregarAsync antes de Resolver.");

        var texto = Normalizar(descricao);
        foreach (var r in _regras)
        {
            var corresponde = r.Tipo switch
            {
                TipoCorrespondencia.ComecaCom => texto.StartsWith(r.TextoNormalizado, StringComparison.Ordinal),
                TipoCorrespondencia.Igual => texto == r.TextoNormalizado,
                _ => texto.Contains(r.TextoNormalizado, StringComparison.Ordinal)
            };
            if (corresponde && _tiposCategoria.ContainsKey(r.CategoriaId)) return r.CategoriaId;
        }

        if (pluggyCategoriaId is not null && _mapaPluggy.TryGetValue(pluggyCategoriaId, out var mapeada))
            return mapeada;

        return null;
    }

    /// <summary>Ajusta Tipo e IgnorarRelatorios conforme o sinal do valor e o tipo da categoria.</summary>
    public void AplicarTipo(Transacao t)
    {
        var ehTransferencia = t.CategoriaId is int id && _tiposCategoria.TryGetValue(id, out var tipo)
                              && tipo == TipoCategoria.Transferencia;
        if (ehTransferencia)
        {
            t.Tipo = TipoTransacao.Transferencia;
            t.IgnorarRelatorios = true;
        }
        else
        {
            if (t.Tipo == TipoTransacao.Transferencia && t.TransferenciaGrupo is null) t.IgnorarRelatorios = false;
            if (t.TransferenciaGrupo is null)
                t.Tipo = t.Valor < 0 ? TipoTransacao.Despesa : TipoTransacao.Receita;
        }
    }

    public TipoCategoria? TipoDaCategoria(int? categoriaId)
        => categoriaId is int id && _tiposCategoria.TryGetValue(id, out var tipo) ? tipo : null;

    /// <summary>Sugere a categoria local para uma categoria nova da Pluggy (pelo nome dela ou do pai).</summary>
    public static string? SugerirCategoriaPadrao(string? descricao, string? paiDescricao)
    {
        if (descricao is not null && MapaPadraoPluggy.TryGetValue(descricao.Trim(), out var nome)) return nome;
        if (paiDescricao is not null && MapaPadraoPluggy.TryGetValue(paiDescricao.Trim(), out var nomePai)) return nomePai;
        return null;
    }

    /// <summary>Maiúsculas e sem acentos, para comparação de regras.</summary>
    public static string Normalizar(string? valor)
    {
        if (string.IsNullOrWhiteSpace(valor)) return string.Empty;
        var decomposto = valor.Trim().Normalize(NormalizationForm.FormD);
        var sb = new StringBuilder(decomposto.Length);
        foreach (var c in decomposto)
        {
            if (CharUnicodeInfo.GetUnicodeCategory(c) != UnicodeCategory.NonSpacingMark) sb.Append(c);
        }
        return sb.ToString().Normalize(NormalizationForm.FormC).ToUpperInvariant();
    }
}
