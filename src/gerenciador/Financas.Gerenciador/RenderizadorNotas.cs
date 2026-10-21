using System.Text.RegularExpressions;

namespace Financas.Gerenciador;

/// <summary>Um link escrito na caixa de notas: onde está no texto e para onde vai.</summary>
internal sealed record LinkNasNotas(int Inicio, int Tamanho, Uri Destino);

/// <summary>
/// #294: escreve as notas da versão (Markdown) formatadas numa <see cref="RichTextBox"/>, só com o que o CHANGELOG e as
/// notas usam: títulos (#, ##, ###, ####), listas com "-" (com recuo por nível), **negrito**, `código` e [links](url).
/// Sem dependência nova: monta o texto pela própria caixa (fonte, cor, marcador e recuo de cada trecho). Comentários
/// HTML (os marcadores dos blocos) não aparecem.
/// </summary>
internal static partial class RenderizadorNotas
{
    [GeneratedRegex(@"\*\*(?<negrito>.+?)\*\*|`(?<codigo>[^`]+)`|\[(?<texto>[^\]]+)\]\((?<url>[^)\s]+)\)")]
    private static partial Regex Trechos();

    [GeneratedRegex(@"^(?<recuo>\s*)[-*] (?<texto>.*)$")]
    private static partial Regex ItemDeLista();

    [GeneratedRegex(@"^(?<nivel>#{1,4}) (?<texto>.*)$")]
    private static partial Regex Titulo();

    public static List<LinkNasNotas> Escrever(RichTextBox caixa, string markdown, Tema tema, float escala)
    {
        var links = new List<LinkNasNotas>();
        var tamanhoBase = caixa.Font.Size;
        var familia = caixa.Font.FontFamily;
        var primeira = true;
        var espacoPendente = false;
        var emComentario = false;

        foreach (var bruta in markdown.Replace("\r\n", "\n").Split('\n'))
        {
            var linha = bruta.TrimEnd();
            // Comentários HTML (<!-- ... -->), inteiros numa linha ou em várias.
            if (emComentario) { if (linha.Contains("-->")) emComentario = false; continue; }
            if (linha.TrimStart().StartsWith("<!--", StringComparison.Ordinal))
            {
                if (!linha.Contains("-->")) emComentario = true;
                continue;
            }
            // Linha em branco: só um respiro antes da próxima linha com texto (várias seguidas valem uma).
            if (linha.Length == 0) { espacoPendente = !primeira; continue; }

            // A quebra vai ANTES de cada linha (menos a primeira): no fim do texto não sobra parágrafo vazio, que
            // herdaria o marcador da última lista. O respiro é um parágrafo pequeno, sem marcador nem recuo.
            if (!primeira)
            {
                Escrever(caixa, "\n", caixa.Font, tema.Texto);
                if (espacoPendente)
                {
                    SemMarcador(caixa);
                    using var meia = new Font(familia, tamanhoBase * 0.5f);
                    Escrever(caixa, "\n", meia, tema.Texto);
                }
            }
            primeira = false;
            espacoPendente = false;
            SemMarcador(caixa);

            if (Titulo().Match(linha) is { Success: true } titulo)
            {
                var nivel = titulo.Groups["nivel"].Length;
                var tamanho = nivel switch { 1 => tamanhoBase + 4f, 2 => tamanhoBase + 2.5f, 3 => tamanhoBase + 1f, _ => tamanhoBase };
                using var fonte = new Font(familia, tamanho, FontStyle.Bold);
                EscreverTrechos(caixa, titulo.Groups["texto"].Value, fonte, tema, links, tema.Texto);
            }
            else if (ItemDeLista().Match(linha) is { Success: true } item)
            {
                var nivel = item.Groups["recuo"].Value.Replace("\t", "  ").Length / 2;
                caixa.SelectionBullet = true;
                caixa.SelectionIndent = (int)((6 + nivel * 18) * escala);
                caixa.SelectionHangingIndent = (int)(12 * escala);
                caixa.BulletIndent = (int)(12 * escala);
                using var fonte = new Font(familia, tamanhoBase);
                EscreverTrechos(caixa, item.Groups["texto"].Value, fonte, tema, links, tema.Texto);
            }
            else
            {
                using var fonte = new Font(familia, tamanhoBase);
                EscreverTrechos(caixa, linha.Trim(), fonte, tema, links, tema.Texto);
            }
        }
        caixa.Select(0, 0);
        return links;
    }

    /// <summary>O parágrafo que começa no fim do texto, sem marcador nem recuo.</summary>
    private static void SemMarcador(RichTextBox caixa)
    {
        caixa.Select(caixa.TextLength, 0);
        caixa.SelectionBullet = false;
        caixa.SelectionIndent = 0;
        caixa.SelectionHangingIndent = 0;
    }

    /// <summary>Uma linha com **negrito**, `código` e [links](url) no meio do texto comum.</summary>
    private static void EscreverTrechos(RichTextBox caixa, string texto, Font fonte, Tema tema, List<LinkNasNotas> links, Color cor)
    {
        var posicao = 0;
        foreach (Match m in Trechos().Matches(texto))
        {
            if (m.Index > posicao) Escrever(caixa, texto[posicao..m.Index], fonte, cor);
            if (m.Groups["negrito"].Success)
            {
                // Pode ter link dentro do negrito: escreve o miolo de novo, em negrito.
                using var negrito = new Font(fonte, fonte.Style | FontStyle.Bold);
                EscreverTrechos(caixa, m.Groups["negrito"].Value, negrito, tema, links, cor);
            }
            else if (m.Groups["codigo"].Success)
            {
                using var codigo = new Font("Consolas", fonte.Size - 0.5f);
                Escrever(caixa, m.Groups["codigo"].Value, codigo, tema.TextoSuave);
            }
            else
            {
                var inicio = caixa.TextLength;
                using var sublinhada = new Font(fonte, fonte.Style | FontStyle.Underline);
                Escrever(caixa, m.Groups["texto"].Value, sublinhada, tema.Primaria);
                // Só links da web abrem no navegador (nada de arquivo local ou outro esquema vindo das notas).
                if (Uri.TryCreate(m.Groups["url"].Value, UriKind.Absolute, out var destino)
                    && (destino.Scheme == Uri.UriSchemeHttps || destino.Scheme == Uri.UriSchemeHttp))
                    links.Add(new LinkNasNotas(inicio, caixa.TextLength - inicio, destino));
            }
            posicao = m.Index + m.Length;
        }
        if (posicao < texto.Length) Escrever(caixa, texto[posicao..], fonte, cor);
    }

    private static void Escrever(RichTextBox caixa, string texto, Font fonte, Color cor)
    {
        caixa.Select(caixa.TextLength, 0);
        caixa.SelectionFont = fonte;
        caixa.SelectionColor = cor;
        caixa.SelectedText = texto;
    }
}
