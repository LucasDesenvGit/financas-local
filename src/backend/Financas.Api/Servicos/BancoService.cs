using System.Globalization;
using System.Text;
using System.Text.RegularExpressions;

namespace Financas.Api.Servicos;

/// <summary>Um banco ou carteira que a tela de Contas sabe identificar (o selo de cada um fica no front-end).</summary>
public sealed record DefinicaoBanco(string Codigo, string Nome, string[] Termos);

/// <summary>
/// Banco de cada conta e cartão (ADR 0024). O usuário pode escolher; sem escolha, o banco é <b>detectado</b> pelo nome
/// da instituição que a Pluggy informa e pelo nome da conta (ex.: "Nubank Ultravioleta" → nubank). Cartão sem banco
/// escolhido usa o banco da conta a que pertence (<c>Conta.ContaVinculadaId</c>). Os códigos daqui são os mesmos de
/// <c>compartilhado/bancos.ts</c> no front-end, que desenha o selo de cada um: banco novo entra nos dois lugares.
/// </summary>
public static partial class BancoService
{
    /// <summary>"Outro": escolhido pelo usuário para não identificar banco nenhum (sem selo de marca).</summary>
    public const string Outro = "outro";

    public static readonly IReadOnlyList<DefinicaoBanco> Catalogo =
    [
        new("nubank", "Nubank", ["nubank", "nu pagamentos", "nu financeira", "nuconta", "ultravioleta"]),
        new("itau", "Itaú", ["itau", "itaucard", "unibanco", "personnalite", "uniclass"]),
        new("mercadopago", "Mercado Pago", ["mercado pago", "mercadopago", "mercado credito"]),
        new("inter", "Inter", ["banco inter", "inter"]),
        new("bradesco", "Bradesco", ["bradesco", "bradescard"]),
        new("santander", "Santander", ["santander"]),
        new("bb", "Banco do Brasil", ["banco do brasil", "bb", "ourocard"]),
        new("caixa", "Caixa", ["caixa", "caixa economica", "cef"]),
        new("c6", "C6 Bank", ["c6", "c6 bank", "c6bank"]),
        new("picpay", "PicPay", ["picpay"]),
        new("pagbank", "PagBank", ["pagbank", "pagseguro"]),
        new("btg", "BTG Pactual", ["btg"]),
        new("xp", "XP", ["xp", "xp investimentos"]),
        new("neon", "Neon", ["neon"]),
        new("next", "Next", ["next"]),
        new("will", "Will Bank", ["will bank", "willbank"]),
        new("sicoob", "Sicoob", ["sicoob"]),
        new("sicredi", "Sicredi", ["sicredi"]),
        new("banrisul", "Banrisul", ["banrisul"]),
        new("safra", "Safra", ["safra"]),
        new("original", "Banco Original", ["banco original"]),
        new("pan", "Banco Pan", ["banco pan"]),
        new("bmg", "BMG", ["bmg"]),
        new("agibank", "Agibank", ["agibank"]),
        new("nomad", "Nomad", ["nomad"]),
        new("wise", "Wise", ["wise"]),
        new("revolut", "Revolut", ["revolut"]),
        new("paypal", "PayPal", ["paypal"]),
        new(Outro, "Outro (sem banco)", [])
    ];

    private static readonly Dictionary<string, Regex> Padroes = Catalogo.ToDictionary(b => b.Codigo,
        b => new Regex(b.Termos.Length == 0 ? "(?!)" : @"\b(" + string.Join("|", b.Termos.Select(Regex.Escape)) + @")\b",
            RegexOptions.Compiled | RegexOptions.CultureInvariant));

    public static bool Existe(string? codigo) => codigo is not null && Catalogo.Any(b => b.Codigo == codigo);

    public static string? Nome(string? codigo) => Catalogo.FirstOrDefault(b => b.Codigo == codigo)?.Nome;

    /// <summary>
    /// Primeiro banco do catálogo cujo termo aparece como palavra inteira, sem acento nem maiúscula, em algum dos
    /// textos (na ordem: nome da instituição antes do nome da conta). Null quando nenhum bate.
    /// </summary>
    public static string? Detectar(params string?[] textos)
    {
        foreach (var texto in textos)
        {
            if (string.IsNullOrWhiteSpace(texto)) continue;
            var normalizado = Normalizar(texto);
            foreach (var banco in Catalogo)
                if (Padroes[banco.Codigo].IsMatch(normalizado)) return banco.Codigo;
        }
        return null;
    }

    /// <summary>
    /// Banco de cada conta: o escolhido; senão, para cartão, o da conta vinculada; senão, o detectado pelos nomes.
    /// "Outro" vira null (sem banco), mas impede a detecção.
    /// </summary>
    public static Dictionary<int, string?> Efetivos(IReadOnlyCollection<ContaParaBanco> contas)
    {
        var porId = contas.ToDictionary(c => c.Id);
        var resultado = new Dictionary<int, string?>();
        string? Proprio(ContaParaBanco c) => c.Banco ?? Detectar(c.NomeInstituicao, c.Nome);
        foreach (var c in contas)
        {
            string? banco = c.Banco;
            if (banco is null && c.ContaVinculadaId is int v && porId.TryGetValue(v, out var vinculada))
                banco = vinculada.Banco ?? Detectar(vinculada.NomeInstituicao, vinculada.Nome);
            banco ??= Proprio(c);
            resultado[c.Id] = banco == Outro ? null : banco;
        }
        return resultado;
    }

    private static string Normalizar(string texto)
    {
        var sb = new StringBuilder(texto.Length);
        foreach (var c in texto.Normalize(NormalizationForm.FormD))
        {
            if (CharUnicodeInfo.GetUnicodeCategory(c) == UnicodeCategory.NonSpacingMark) continue;
            sb.Append(char.IsLetterOrDigit(c) ? char.ToLowerInvariant(c) : ' ');
        }
        return MultiplosEspacos().Replace(sb.ToString(), " ").Trim();
    }

    [GeneratedRegex(@"\s+")]
    private static partial Regex MultiplosEspacos();
}

/// <summary>O que a identificação do banco precisa de cada conta.</summary>
public sealed record ContaParaBanco(int Id, string Nome, string? NomeInstituicao, string? Banco, int? ContaVinculadaId);
