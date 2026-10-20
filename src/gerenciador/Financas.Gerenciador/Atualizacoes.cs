using System.Diagnostics;
using System.Net;
using System.Security.Cryptography;
using System.Text.Json;

namespace Financas.Gerenciador;

/// <summary>Versão no GitHub com instalador (.msi) e somas de verificação (SHA256SUMS.txt).</summary>
internal sealed record VersaoDisponivel(VersaoDoSistema Versao, string Tag, string Notas, Uri Msi, string NomeMsi, Uri? Somas,
    Uri Pagina);

/// <summary>
/// Versão nova pelo GitHub Releases (ADR 0028): só leitura de release público, por HTTPS, sem enviar nada do usuário.
/// O .msi baixado só é aberto se o SHA256 bater com o publicado junto. O .msi de desenvolvimento (X.Y.Z-dev.N) procura
/// no pré-lançamento "desenvolvimento", gerado do develop e testado pela CI; o da versão publicada, só no Release mais
/// recente (ADR 0040).
/// </summary>
internal static class Atualizacoes
{
    private static readonly HttpClient Http = CriarHttp();

    private static HttpClient CriarHttp()
    {
        var http = new HttpClient { Timeout = TimeSpan.FromMinutes(15) };
        http.DefaultRequestHeaders.UserAgent.ParseAdd($"FinancasLocal-Gerenciador/{Instalacao.Atual.VersaoTexto}");
        return http;
    }

    /// <summary>
    /// A última versão publicada (ou, com <paramref name="desenvolvimento"/>, a última de desenvolvimento), ou null se
    /// não houver nenhuma com instalador.
    /// </summary>
    public static async Task<VersaoDisponivel?> ProcurarAsync(bool desenvolvimento, CancellationToken ct)
    {
        var endereco = desenvolvimento ? "releases/tags/desenvolvimento" : "releases/latest";
        using var requisicao = new HttpRequestMessage(HttpMethod.Get,
            $"https://api.github.com/repos/{Instalacao.Repositorio}/{endereco}");
        requisicao.Headers.Accept.ParseAdd("application/vnd.github+json");
        using var resposta = await Http.SendAsync(requisicao, ct);
        if (resposta.StatusCode == HttpStatusCode.NotFound) return null;
        resposta.EnsureSuccessStatusCode();

        using var doc = JsonDocument.Parse(await resposta.Content.ReadAsStringAsync(ct));
        var raiz = doc.RootElement;
        var tag = raiz.GetProperty("tag_name").GetString() ?? "";
        // A de desenvolvimento tem tag fixa ("desenvolvimento"): o número vem do título, "Finanças Local 2.2.1-dev.60 (…)".
        var titulo = raiz.TryGetProperty("name", out var nomeRelease) ? nomeRelease.GetString() ?? "" : "";
        var versao = desenvolvimento ? VersaoDoSistema.Ler(titulo.Replace("Finanças Local", "").Trim()) : VersaoDoSistema.Ler(tag);
        if (versao is null || versao.EhDesenvolvimento != desenvolvimento) return null;

        Uri? msi = null, somas = null;
        string? nomeMsi = null;
        foreach (var arquivo in raiz.GetProperty("assets").EnumerateArray())
        {
            var nome = arquivo.GetProperty("name").GetString() ?? "";
            if (arquivo.GetProperty("browser_download_url").GetString() is not string url) continue;
            if (nome.EndsWith(".msi", StringComparison.OrdinalIgnoreCase)) (msi, nomeMsi) = (new Uri(url), nome);
            else if (nome.Equals("SHA256SUMS.txt", StringComparison.OrdinalIgnoreCase)) somas = new Uri(url);
        }
        if (msi is null || nomeMsi is null) return null; // versão só com o código, sem instalador

        var notas = raiz.TryGetProperty("body", out var corpo) ? corpo.GetString() ?? "" : "";
        var pagina = new Uri(raiz.GetProperty("html_url").GetString() ?? $"https://github.com/{Instalacao.Repositorio}/releases");
        return new VersaoDisponivel(versao, tag, notas, msi, nomeMsi, somas, pagina);
    }

    /// <summary>Baixa o .msi para a pasta temporária e confere o SHA256; se não bater, apaga e avisa.</summary>
    public static async Task<string> BaixarAsync(VersaoDisponivel versao, IProgress<double> progresso, CancellationToken ct)
    {
        if (versao.Somas is null)
            throw new InvalidOperationException("A versão publicada não tem o SHA256SUMS.txt. Sem como conferir o arquivo, nada foi baixado.");
        var esperado = Esperado(await Http.GetStringAsync(versao.Somas, ct), versao.NomeMsi)
                       ?? throw new InvalidOperationException($"O SHA256SUMS.txt não tem o {versao.NomeMsi}. Nada foi baixado.");

        var pasta = Path.Combine(Path.GetTempPath(), "FinancasLocal");
        Directory.CreateDirectory(pasta);
        var destino = Path.Combine(pasta, Path.GetFileName(versao.NomeMsi));
        try
        {
            using (var resposta = await Http.GetAsync(versao.Msi, HttpCompletionOption.ResponseHeadersRead, ct))
            {
                resposta.EnsureSuccessStatusCode();
                var total = resposta.Content.Headers.ContentLength;
                await using var origem = await resposta.Content.ReadAsStreamAsync(ct);
                await using var arquivo = File.Create(destino);
                var buffer = new byte[81920];
                long lidos = 0;
                int n;
                while ((n = await origem.ReadAsync(buffer, ct)) > 0)
                {
                    await arquivo.WriteAsync(buffer.AsMemory(0, n), ct);
                    lidos += n;
                    if (total > 0) progresso.Report((double)lidos / total.Value);
                }
            }

            string hash;
            await using (var arquivo = File.OpenRead(destino))
                hash = Convert.ToHexString(await SHA256.HashDataAsync(arquivo, ct));
            if (!hash.Equals(esperado, StringComparison.OrdinalIgnoreCase))
                throw new InvalidOperationException("O arquivo baixado não confere com o publicado (SHA256). Nada foi instalado; tente de novo.");
            return destino;
        }
        catch
        {
            try { File.Delete(destino); } catch (IOException) { }
            throw;
        }
    }

    /// <summary>
    /// Abre o instalador com a janela dele (que pede o UAC e mostra o progresso). O gerenciador fecha logo depois, para
    /// o instalador poder trocá-lo; no fim, o próprio instalador oferece abrir o gerenciador de novo.
    /// </summary>
    public static void Instalar(string msi)
    {
        var log = Path.ChangeExtension(msi, ".log");
        Process.Start(new ProcessStartInfo("msiexec.exe", $"/i {AcoesElevadas.Aspas(msi)} /l*v {AcoesElevadas.Aspas(log)}")
        {
            UseShellExecute = true
        });
    }

    /// <summary>Linha "hash  nome" (formato do sha256sum) do arquivo pedido.</summary>
    private static string? Esperado(string somas, string nome)
    {
        foreach (var linha in somas.Split('\n'))
        {
            var partes = linha.Trim().Split(' ', 2, StringSplitOptions.RemoveEmptyEntries);
            if (partes.Length == 2 && partes[1].Trim().TrimStart('*') == nome) return partes[0];
        }
        return null;
    }
}
