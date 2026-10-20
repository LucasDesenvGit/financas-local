namespace Financas.Api.Servicos.Investimentos;

/// <summary>
/// #170: nome do investimento que o usuário reconhece. O banco manda a razão social do emissor ("CDB - NU FINANCEIRA
/// S.A. - SOCIEDADE DE CREDITO, F…"), que esconde a caixinha do Nubank e o CDB do Mercado Pago; aqui vira
/// "CDB · Nubank". O nome escolhido pelo usuário vale sempre; ações e ETFs ficam como vieram (o ticker já é o nome).
/// </summary>
public static class NomeInvestimento
{
    /// <summary>Até este tamanho, o texto antes do " - " é o produto (CDB, LCI, RDB…), e não parte da razão social.</summary>
    private const int TamanhoMaximoProduto = 24;

    public static string Exibicao(string nome, string tipo, string? emissor, string? personalizado)
    {
        if (!string.IsNullOrWhiteSpace(personalizado)) return personalizado.Trim();
        if (tipo is "EQUITY" or "ETF") return nome;
        var banco = BancoService.Nome(BancoService.Detectar(emissor, nome));
        if (banco is null) return nome;

        // "CDB - NU FINANCEIRA S.A. - …": o produto é o que vem antes do primeiro " - ".
        var partes = nome.Split(" - ", 2, StringSplitOptions.TrimEntries);
        if (partes.Length == 2 && partes[0].Length is > 0 and <= TamanhoMaximoProduto
            && BancoService.Detectar(partes[0]) is null && BancoService.Detectar(partes[1]) is not null)
            return $"{partes[0]} · {banco}";
        // "CDB" com o emissor à parte: o nome curto ganha o banco.
        if (partes.Length == 1 && nome.Trim().Length is > 0 and <= TamanhoMaximoProduto && BancoService.Detectar(nome) is null)
            return $"{nome.Trim()} · {banco}";
        return nome;
    }
}
