using System.Text.RegularExpressions;

namespace Financas.Api.Servicos.Investimentos;

/// <summary>
/// Reconhece um FII (ou Fiagro) pelo nome do fundo ou do emissor. Pelo Open Finance, a corretora pode mandar o FII
/// com o subtipo "STOCK" (o Nubank manda), e o final 11 sozinho não basta: units de ações (TAEE11) e ETFs (BOVA11)
/// também terminam em 11.
/// </summary>
public static partial class ClassificacaoAtivo
{
    /// <summary>
    /// "FII"/"Fiagro" no texto, ou fundo + imobiliário ("Fundo de Investimento Imobiliário", "FDO INV IMOB").
    /// Fundo de índice (ETF) com "imobiliário" no nome não conta.
    /// </summary>
    public static bool PareceFii(string? nomeOuEmissor)
    {
        var texto = CategorizacaoService.Normalizar(nomeOuEmissor);
        if (texto.Length == 0 || FundoDeIndice().IsMatch(texto)) return false;
        return Sigla().IsMatch(texto) || (Fundo().IsMatch(texto) && texto.Contains("IMOB", StringComparison.Ordinal));
    }

    [GeneratedRegex(@"\b(FII|FIAGRO)\b")]
    private static partial Regex Sigla();

    [GeneratedRegex(@"\b(FUNDOS?|FDO|FUND)\b")]
    private static partial Regex Fundo();

    [GeneratedRegex(@"\b(INDICE|ETF)\b")]
    private static partial Regex FundoDeIndice();
}
