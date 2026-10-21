using System.Text.RegularExpressions;

namespace Financas.Gerenciador;

/// <summary>
/// #294: as notas de uma versão separadas em "o que mudou desde a sua versão" e "o resto" (Nielsen, Progressive
/// Disclosure). As notas (publicada e de desenvolvimento) trazem o CHANGELOG com uma seção por versão, um nível abaixo:
/// "### 2.26.1 — 2026-10-21 · desenvolvimento", com "#### Correções" dentro (ADR 0039). As novidades são as seções de
/// versão mais nova que a instalada; o resto (como instalar, a lista de PRs por versão de desenvolvimento e as versões
/// que você já tem) fica atrás de "Ver tudo".
/// Sem seção de versão (notas antigas): os blocos de PRs por versão de desenvolvimento, entre
/// <c>&lt;!-- por-versao:inicio --&gt;</c> e <c>&lt;!-- por-versao:fim --&gt;</c>, mais novos que a instalada; sem eles
/// também, as notas inteiras.
/// </summary>
public sealed partial record NotasDaVersao(string Novidades, string Resto, int QuantasVersoesNovas)
{
    public const string MarcadorInicio = "<!-- por-versao:inicio -->";
    public const string MarcadorFim = "<!-- por-versao:fim -->";

    /// <summary>Tem o que esconder atrás de "Ver tudo".</summary>
    public bool TemResto => !string.IsNullOrWhiteSpace(Resto);

    /// <summary>"### 2.26.1 — 2026-10-21 · desenvolvimento" (seção do CHANGELOG nas notas).</summary>
    [GeneratedRegex(@"^### (\d+\.\d+\.\d+) — ")]
    private static partial Regex TituloDeVersao();

    /// <summary>"### 2.26.1-dev.180 · 21/10/2026" (bloco de PRs da versão de desenvolvimento).</summary>
    [GeneratedRegex(@"^### (\d+\.\d+\.\d+-dev\.\d+) · ")]
    private static partial Regex TituloDeBloco();

    public static NotasDaVersao Separar(string? notas, VersaoDoSistema? instalada)
    {
        var linhas = (notas ?? string.Empty).Replace("\r\n", "\n").Split('\n');
        var titulo = instalada is null ? "## O que mudou" : $"## O que mudou desde a sua versão ({instalada})";

        // 1) Seções de versão do CHANGELOG mais novas que a instalada (pelo X.Y.Z: a dev.N da 2.25.0 já tem a 2.25.0).
        var secoes = Trechos(linhas, TituloDeVersao(), dentroDosMarcadores: false);
        var novas = secoes.Where(s => instalada is null || s.Versao.Base > instalada.Base).ToList();
        if (instalada is null && novas.Count > 1) novas = [novas[0]];
        if (novas.Count > 0) return Montar(linhas, novas, titulo);

        // 2) Notas antigas, sem seção de versão: os blocos de PRs mais novos que a instalada.
        var blocos = Trechos(linhas, TituloDeBloco(), dentroDosMarcadores: true);
        var blocosNovos = blocos.Where(b => instalada is null || b.Versao.CompareTo(instalada) > 0).ToList();
        if (blocosNovos.Count == 0 && blocos.Count > 0) blocosNovos = [blocos[0]];
        if (blocosNovos.Count > 0) return Montar(linhas, blocosNovos, titulo);

        // 3) Nada para separar: as notas inteiras.
        return new NotasDaVersao(string.Join('\n', linhas).Trim(), string.Empty, 0);
    }

    private sealed record Trecho(VersaoDoSistema Versao, int Inicio, int Fim);

    /// <summary>Cada título que casa, até o próximo título de mesmo nível ou acima ("### " ou "## ").</summary>
    private static List<Trecho> Trechos(string[] linhas, Regex padrao, bool dentroDosMarcadores)
    {
        var trechos = new List<Trecho>();
        var dentro = false;
        for (var i = 0; i < linhas.Length; i++)
        {
            if (linhas[i].Contains(MarcadorInicio, StringComparison.Ordinal)) { dentro = true; continue; }
            if (linhas[i].Contains(MarcadorFim, StringComparison.Ordinal)) { dentro = false; continue; }
            if (dentro != dentroDosMarcadores) continue;
            var m = padrao.Match(linhas[i]);
            if (!m.Success || VersaoDoSistema.Ler(m.Groups[1].Value) is not { } versao) continue;
            var fim = i + 1;
            while (fim < linhas.Length && !linhas[fim].StartsWith("### ", StringComparison.Ordinal)
                   && !linhas[fim].StartsWith("## ", StringComparison.Ordinal)
                   && !linhas[fim].Contains(MarcadorFim, StringComparison.Ordinal))
                fim++;
            trechos.Add(new Trecho(versao, i, fim));
        }
        return trechos;
    }

    /// <summary>As novidades em cima, com o título; o resto, na ordem original, sem elas.</summary>
    private static NotasDaVersao Montar(string[] linhas, List<Trecho> novos, string titulo)
    {
        var usadas = new HashSet<int>(novos.SelectMany(t => Enumerable.Range(t.Inicio, t.Fim - t.Inicio)));
        var novidades = titulo + "\n\n" + string.Join("\n", novos.Select(t => string.Join('\n', linhas[t.Inicio..t.Fim]).Trim() + "\n"));
        var resto = string.Join('\n', linhas.Where((_, i) => !usadas.Contains(i)));
        return new NotasDaVersao(novidades.Trim(), resto.Trim(), novos.Count);
    }
}
