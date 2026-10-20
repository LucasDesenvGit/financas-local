using System.Text.RegularExpressions;

namespace Financas.Api.Servicos;

/// <summary>
/// Parcelas que ainda vão ser cobradas (#165), a partir das parcelas recentes e das que o banco já mandou com data
/// futura. Usado pela Projeção e pelo assistente de metas.
/// <list type="bullet">
/// <item>De cada compra parcelada vale só a parcela mais recente: projetar a partir de duas parcelas da mesma compra
/// contava o resto em dobro.</item>
/// <item>Parcela com data futura que já está no banco entra como é; a projeção começa depois da última conhecida.</item>
/// <item>A mesma compra é reconhecida pela conta, pelo total de parcelas, pelo valor em reais inteiros (o banco
/// arredonda a 1ª parcela: 291,19 e depois 291,17) e pelas 8 primeiras letras da descrição sem o "3/10" (o banco corta
/// a descrição de jeitos diferentes: "MERCADOLIVRE*MERCADOLIVRE" e "MERCADOLIVRE*MERC").</item>
/// </list>
/// </summary>
public static partial class ParcelasFuturas
{
    /// <summary>Até quantos dias para trás uma parcela conta como "recente" (base da projeção).</summary>
    public const int DiasRecentes = 45;

    public sealed record Parcela(long Id, int ContaId, DateOnly Data, decimal Valor, short Numero, short Total, string Descricao);

    /// <summary>
    /// Uma parcela por vir. <c>JaNoBanco</c>: o banco já mandou (data futura); senão, foi projetada <c>Passos</c> meses
    /// depois da parcela <c>OrigemId</c> (a fatura dela + <c>Passos</c>, #167).
    /// </summary>
    public sealed record Futura(int ContaId, DateOnly Data, decimal Valor, bool JaNoBanco, long OrigemId, int Passos);

    /// <param name="parcelas">Parcelas (de qualquer conta) com data a partir de hoje − <see cref="DiasRecentes"/>, incluindo
    /// as de data futura.</param>
    public static List<Futura> Projetar(IEnumerable<Parcela> parcelas, DateOnly hoje)
    {
        var validas = parcelas.Where(p => p.Total > 1 && p.Numero >= 1 && p.Numero <= p.Total && p.Valor != 0).ToList();
        var futuras = validas.Where(p => p.Data > hoje)
            .Select(p => new Futura(p.ContaId, p.Data, Math.Abs(p.Valor), true, p.Id, 0)).ToList();
        foreach (var compra in validas.GroupBy(Chave))
        {
            var ultima = compra.MaxBy(p => p.Numero)!;
            for (var k = 1; k <= ultima.Total - ultima.Numero; k++)
            {
                var data = ultima.Data.AddMonths(k);
                if (data > hoje) futuras.Add(new Futura(ultima.ContaId, data, Math.Abs(ultima.Valor), false, ultima.Id, k));
            }
        }
        return futuras;
    }

    private static (int, short, decimal, string) Chave(Parcela p)
        => (p.ContaId, p.Total, Math.Round(Math.Abs(p.Valor), 0), Compra(p.Descricao));

    /// <summary>As 8 primeiras letras da descrição, sem o número da parcela ("03/10", "PARC 3 DE 10").</summary>
    internal static string Compra(string descricao)
    {
        var semNumero = NumeroDaParcela().Replace(descricao.ToUpperInvariant(), " ");
        var letras = new string(semNumero.Where(char.IsLetter).ToArray());
        return letras.Length > 8 ? letras[..8] : letras;
    }

    [GeneratedRegex(@"\bPARC(ELA)?\b\.?|\d{1,3}\s*(/|DE)\s*\d{1,3}")]
    private static partial Regex NumeroDaParcela();
}
