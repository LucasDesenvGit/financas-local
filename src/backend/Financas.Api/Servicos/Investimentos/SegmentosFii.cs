namespace Financas.Api.Servicos.Investimentos;

/// <summary>
/// Segmentos usados para agrupar os FIIs. A sugestão cobre alguns fundos conhecidos;
/// o segmento pode (e deve) ser conferido e alterado em "editar" no ativo.
/// </summary>
public static class SegmentosFii
{
    public static readonly string[] Todos =
    {
        "Logística", "Shoppings", "Lajes corporativas", "Papel (recebíveis)", "Híbrido", "Fundo de fundos",
        "Renda urbana", "Agronegócio", "Residencial", "Hotéis", "Hospitais/Educação", "Desenvolvimento", "Outros"
    };

    private static readonly Dictionary<string, string> Sugestoes = new(StringComparer.OrdinalIgnoreCase)
    {
        ["HGLG11"] = "Logística", ["BTLG11"] = "Logística", ["XPLG11"] = "Logística", ["VILG11"] = "Logística",
        ["BRCO11"] = "Logística", ["LVBI11"] = "Logística", ["GGRC11"] = "Logística", ["RBRL11"] = "Logística",
        ["XPML11"] = "Shoppings", ["VISC11"] = "Shoppings", ["HGBS11"] = "Shoppings", ["HSML11"] = "Shoppings",
        ["MALL11"] = "Shoppings",
        ["HGRE11"] = "Lajes corporativas", ["PVBI11"] = "Lajes corporativas", ["JSRE11"] = "Lajes corporativas",
        ["RCRB11"] = "Lajes corporativas", ["BRCR11"] = "Lajes corporativas",
        ["KNCR11"] = "Papel (recebíveis)", ["KNIP11"] = "Papel (recebíveis)", ["KNSC11"] = "Papel (recebíveis)",
        ["CPTS11"] = "Papel (recebíveis)", ["IRDM11"] = "Papel (recebíveis)", ["RECR11"] = "Papel (recebíveis)",
        ["VGIR11"] = "Papel (recebíveis)", ["MCCI11"] = "Papel (recebíveis)", ["RBRR11"] = "Papel (recebíveis)",
        ["BCRI11"] = "Papel (recebíveis)", ["VRTA11"] = "Papel (recebíveis)", ["HGCR11"] = "Papel (recebíveis)",
        ["KNHY11"] = "Papel (recebíveis)", ["MXRF11"] = "Papel (recebíveis)", ["DEVA11"] = "Papel (recebíveis)",
        ["KNRI11"] = "Híbrido", ["ALZR11"] = "Híbrido", ["HGRU11"] = "Renda urbana", ["TRXF11"] = "Renda urbana",
        ["BCFF11"] = "Fundo de fundos", ["RBRF11"] = "Fundo de fundos", ["HFOF11"] = "Fundo de fundos",
        ["KFOF11"] = "Fundo de fundos", ["BPFF11"] = "Fundo de fundos",
        ["TGAR11"] = "Desenvolvimento", ["RZTR11"] = "Agronegócio", ["SNAG11"] = "Agronegócio",
        ["KNCA11"] = "Agronegócio", ["VGIA11"] = "Agronegócio", ["HCTR11"] = "Papel (recebíveis)"
    };

    public static string? Sugerir(string ticker) => Sugestoes.GetValueOrDefault(ticker);

    public static bool Valido(string? segmento) => segmento is null || Todos.Contains(segmento);
}
