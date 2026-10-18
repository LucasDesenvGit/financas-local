using System.Text.RegularExpressions;

namespace Financas.Api.Servicos.Diagnostico;

/// <summary>
/// Tira de mensagens e pilhas de erro o que não pode ir para a tabela de erros (#11, ADR 0022): query strings de
/// URLs (onde ficam tokens como o da brapi), pares "chave=valor" de segredos, tokens Bearer e JWT, CPF e valores em
/// reais. É uma limpeza por padrão de texto, não uma garantia: o código continua não colocando segredos em log.
/// </summary>
public static partial class Sanitizador
{
    public const string Removido = "[removido]";

    [GeneratedRegex(@"(https?://[^\s?#""'<>]+)\?[^\s#""'<>]*", RegexOptions.IgnoreCase)]
    private static partial Regex QueryString();

    [GeneratedRegex(@"\b(access_?token|refresh_?token|token|api_?key|x-api-key|client_?secret|secret|senha|password|passwd|pwd|authorization|cookie)(\s*[=:]\s*""?|""\s*:\s*"")([^\s&,;""']+)", RegexOptions.IgnoreCase)]
    private static partial Regex ChaveValor();

    [GeneratedRegex(@"\bBearer\s+[A-Za-z0-9\-._~+/]+=*", RegexOptions.IgnoreCase)]
    private static partial Regex Bearer();

    [GeneratedRegex(@"\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*")]
    private static partial Regex Jwt();

    [GeneratedRegex(@"\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b")]
    private static partial Regex Cpf();

    [GeneratedRegex(@"R\$\s?-?\d[\d.]*(,\d{1,2})?")]
    private static partial Regex Reais();

    public static string Limpar(string texto)
    {
        if (string.IsNullOrEmpty(texto)) return texto;
        texto = QueryString().Replace(texto, "$1?" + Removido);
        texto = ChaveValor().Replace(texto, "$1$2" + Removido);
        texto = Bearer().Replace(texto, "Bearer " + Removido);
        texto = Jwt().Replace(texto, Removido);
        texto = Cpf().Replace(texto, Removido);
        texto = Reais().Replace(texto, "R$ " + Removido);
        return texto;
    }

    /// <summary>Limpa e corta no tamanho da coluna (com "…" no fim quando corta).</summary>
    public static string? LimparECortar(string? texto, int maximo)
    {
        if (texto is null) return null;
        var limpo = Limpar(texto);
        return limpo.Length <= maximo ? limpo : limpo[..(maximo - 1)] + "…";
    }
}
