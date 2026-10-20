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
    /// <summary>#166: contas de cartão de crédito (entrada nelas é pagamento de fatura ou estorno, nunca receita).</summary>
    private HashSet<int> _cartoes = new();
    private int? _categoriaPagamentoFatura;
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

        var categorias = await _db.Categorias.AsNoTracking().Select(c => new { c.Id, c.Tipo, c.Nome }).ToListAsync(ct);
        _tiposCategoria = categorias.ToDictionary(c => c.Id, c => c.Tipo);
        _categoriaPagamentoFatura = categorias
            .FirstOrDefault(c => c.Tipo == TipoCategoria.Transferencia && Normalizar(c.Nome) == "PAGAMENTO DE FATURA")?.Id;
        _cartoes = (await _db.Contas.AsNoTracking().Where(c => c.Tipo == TipoConta.CartaoCredito).Select(c => c.Id)
            .ToListAsync(ct)).ToHashSet();

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

    /// <summary>
    /// Tipo de um lançamento que não é transferência (ADR 0036): saída é despesa; entrada é receita, menos no cartão de
    /// crédito, onde a entrada que não é pagamento de fatura é <b>estorno</b>: despesa com valor positivo, que abate o
    /// gasto da categoria em vez de contar como receita.
    /// </summary>
    public static TipoTransacao TipoPeloSinal(decimal valor, bool noCartao)
        => valor < 0 || noCartao ? TipoTransacao.Despesa : TipoTransacao.Receita;

    public bool NoCartao(int contaId) => _cartoes.Contains(contaId);

    /// <summary>
    /// Ajusta Tipo e IgnorarRelatorios conforme o tipo da categoria e o valor. No cartão (#166, ADR 0036), uma entrada
    /// é pagamento de fatura (transferência, pelo texto ou pela categoria, mesmo sem a categoria certa) ou estorno
    /// (despesa positiva). Estorno que chegou na categoria genérica de transferências (ex.: "Estorno de Uber - NuPay")
    /// ganha a categoria do texto sem o "Estorno de" pelas regras, ou fica sem categoria para você escolher.
    /// </summary>
    public void AplicarTipo(Transacao t)
    {
        var ehTransferencia = EhTransferencia(t.CategoriaId);
        if (NoCartao(t.ContaId) && t.Valor > 0 && t.TransferenciaGrupo is null && !(t.CategoriaManual && ehTransferencia))
        {
            if (EhPagamentoNoCartao(t))
            {
                if (!t.CategoriaManual && _categoriaPagamentoFatura is int pagamento) t.CategoriaId = pagamento;
                t.Tipo = TipoTransacao.Transferencia;
                t.IgnorarRelatorios = true;
                return;
            }
            // Na categoria de transferências, só o texto de estorno vira estorno aqui; o resto (Pix seu pagando a fatura,
            // crédito de terceiro) segue a regra das transferências (ADR 0031), que no cartão também dá despesa.
            if (!ehTransferencia || EhTextoDeEstorno(t))
            {
                if (ehTransferencia) t.CategoriaId = CategoriaDoEstorno(t);
                if (t.Tipo == TipoTransacao.Transferencia) t.IgnorarRelatorios = false;
                t.Tipo = TipoTransacao.Despesa;
                return;
            }
        }
        if (ehTransferencia)
        {
            t.Tipo = TipoTransacao.Transferencia;
            t.IgnorarRelatorios = true;
        }
        else
        {
            if (t.Tipo == TipoTransacao.Transferencia && t.TransferenciaGrupo is null) t.IgnorarRelatorios = false;
            if (t.TransferenciaGrupo is null)
                t.Tipo = TipoPeloSinal(t.Valor, NoCartao(t.ContaId));
        }
    }

    private bool EhTransferencia(int? categoriaId)
        => categoriaId is int id && _tiposCategoria.TryGetValue(id, out var tipo) && tipo == TipoCategoria.Transferencia;

    /// <summary>
    /// Entrada no cartão que é o pagamento da fatura: pela categoria "Pagamento de fatura", ou pela operação e o texto
    /// (o do banco ou o nome que aparece).
    /// </summary>
    private bool EhPagamentoNoCartao(Transacao t)
    {
        if (t.CategoriaId is int c && c == _categoriaPagamentoFatura) return true;
        bool Pagamento(string? texto) => TipoLancamentoService.Classificar(t.Valor, texto ?? t.Descricao, null, t.MeioPagamento,
            t.MesmaTitularidade, false, TipoConta.CartaoCredito) == TipoLancamento.PagamentoFatura;
        return Pagamento(t.DescricaoOriginal) || Pagamento(t.Descricao);
    }

    private static bool EhTextoDeEstorno(Transacao t)
    {
        var texto = " " + Normalizar((t.DescricaoOriginal ?? "") + " " + t.Descricao) + " ";
        return new[] { " ESTORNO ", " ESTORNADO ", " CASHBACK ", " DEVOLUCAO ", " REEMBOLSO " }.Any(texto.Contains);
    }

    /// <summary>A categoria do estorno pelo texto sem "Estorno de" e sem a carteira digital ("- NuPay"), se der uma de despesa.</summary>
    private int? CategoriaDoEstorno(Transacao t)
    {
        var texto = Normalizar(t.DescricaoOriginal ?? t.Descricao);
        foreach (var prefixo in new[] { "ESTORNO DE ", "ESTORNO DA ", "ESTORNO DO ", "ESTORNO " })
            if (texto.StartsWith(prefixo, StringComparison.Ordinal)) { texto = texto[prefixo.Length..]; break; }
        foreach (var sufixo in new[] { " NUPAY", " NU PAY" })
            if (texto.EndsWith(sufixo, StringComparison.Ordinal)) texto = texto[..^sufixo.Length];
        return Resolver(texto.Trim(), null) is int c && !EhTransferencia(c) ? c : null;
    }

    /// <summary>Marca que a regra padrão do Méli Dólar já foi criada (#169): apagada por você, ela não volta.</summary>
    public const string ChaveRegraMeliDolar = "Regras.MeliDolarCriada";
    public const string TextoRegraMeliDolar = "MELI DOLAR";

    /// <summary>
    /// #169 (ADR 0038): uma vez por banco, cria a regra padrão "MELI DOLAR" → Investimentos (a mesma do
    /// 02-dados-iniciais.sql, para quem instalou antes) e passa para ela a compra e a venda de Méli Dólar já importadas
    /// cuja categoria você não escolheu. Comprar e vender dólar é mover o próprio dinheiro, não gasto nem receita.
    /// Roda ao iniciar a API. Não salva: chame SaveChangesAsync. Devolve quantas transações mudaram.
    /// </summary>
    public async Task<int> CriarRegraMeliDolarAsync(ConfiguracaoService config, CancellationToken ct)
    {
        if (await config.ExisteAsync(ChaveRegraMeliDolar, ct)) return 0;
        await config.DefinirAsync(ChaveRegraMeliDolar, DateTime.UtcNow.ToString("O"), ct: ct);
        var investimentos = (await _db.Categorias.AsNoTracking().Where(x => x.Tipo == TipoCategoria.Transferencia)
                .Select(x => new { x.Id, x.Nome }).ToListAsync(ct))
            .FirstOrDefault(x => Normalizar(x.Nome) == "INVESTIMENTOS")?.Id;
        if (investimentos is not int categoria) return 0;
        if (!await _db.RegrasCategorizacao.AnyAsync(r => r.Texto == TextoRegraMeliDolar, ct))
            _db.RegrasCategorizacao.Add(new RegraCategorizacao
            {
                Texto = TextoRegraMeliDolar, TipoCorrespondencia = TipoCorrespondencia.Contem, CategoriaId = categoria,
                Prioridade = 50
            });

        await CarregarAsync(ct);
        var lancamentos = (await _db.Transacoes
                .Where(t => !t.CategoriaManual && t.TransferenciaGrupo == null && t.CategoriaId != categoria
                            && EF.Functions.Like(t.DescricaoOriginal ?? t.Descricao, "%meli d_lar%"))
                .ToListAsync(ct))
            .Where(t => Investimentos.ReservaService.EhMeliDolar(t.DescricaoOriginal ?? t.Descricao)).ToList();
        foreach (var t in lancamentos)
        {
            t.CategoriaId = categoria;
            AplicarTipo(t);
            t.AtualizadoEm = DateTime.UtcNow;
        }
        return lancamentos.Count;
    }

    /// <summary>
    /// #166: acerta as entradas nos cartões que já estão no banco (estornos que contavam como receita, pagamentos sem a
    /// categoria certa). Roda ao iniciar a API, antes das transferências. Não salva: chame SaveChangesAsync.
    /// </summary>
    public async Task<int> ReclassificarEntradasNosCartoesAsync(CancellationToken ct)
    {
        await CarregarAsync(ct);
        if (_cartoes.Count == 0) return 0;
        var ids = _cartoes.ToList();
        var entradas = await _db.Transacoes
            .Where(t => ids.Contains(t.ContaId) && t.Valor > 0 && t.TransferenciaGrupo == null
                        && (t.Tipo == TipoTransacao.Receita || (t.Tipo == TipoTransacao.Transferencia && !t.CategoriaManual)))
            .ToListAsync(ct);
        var mudaram = 0;
        foreach (var t in entradas)
        {
            var (tipo, ignorar, categoria) = (t.Tipo, t.IgnorarRelatorios, t.CategoriaId);
            AplicarTipo(t);
            if (t.Tipo == tipo && t.IgnorarRelatorios == ignorar && t.CategoriaId == categoria) continue;
            t.AtualizadoEm = DateTime.UtcNow;
            mudaram++;
        }
        return mudaram;
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
