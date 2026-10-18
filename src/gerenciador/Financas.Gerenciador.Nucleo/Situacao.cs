using System.Net.NetworkInformation;
using System.Net.Security;
using System.Text.Json;

namespace Financas.Gerenciador;

public enum EstadoSite { NaoInstalado, Desligado, Mudando, NoAr, SemResposta }

/// <summary>
/// A situação em palavras do dia a dia (Nielsen 2): o título responde "o site está funcionando?". VersaoSite: a que o
/// site no ar informa (null quando ele não responde).
/// </summary>
public sealed record Situacao(EstadoSite Estado, string Titulo, string Detalhe, string? VersaoSite = null);

/// <summary>Consulta o serviço do sistema e se o site responde (GET /api/auth/estado, que não exige login).</summary>
public static class VerificadorSituacao
{
    private static readonly HttpClient Http = new(new HttpClientHandler
    {
        // Só pergunta se o site responde, neste computador: aceita o certificado local mesmo que o sistema ainda não
        // confie nele (nenhum dado é enviado nessa consulta).
        ServerCertificateCustomValidationCallback = (requisicao, _, _, erros) =>
            erros == SslPolicyErrors.None || requisicao.RequestUri is { IsLoopback: true }
    })
    {
        Timeout = TimeSpan.FromSeconds(4)
    };

    public static async Task<Situacao> ConsultarAsync(Instalacao instalacao, CancellationToken ct)
    {
        switch (ServicoDoSite.Atual.Estado())
        {
            case EstadoServico.NaoInstalado:
                return new Situacao(EstadoSite.NaoInstalado, "O Finanças Local não está instalado neste computador.",
                    ServicoDoSite.Atual.ComoInstalar);
            case EstadoServico.Parado:
                return new Situacao(EstadoSite.Desligado, "O site está desligado.", PortaOcupada(instalacao)
                    ? $"A porta {instalacao.Site.Port} está em uso por outro programa: o site não liga enquanto ela estiver ocupada."
                    : "Clique em “Ligar” para usar.");
            case EstadoServico.Rodando:
                break;
            case EstadoServico.Ligando:
                return new Situacao(EstadoSite.Mudando, "O site está ligando…", "Aguarde alguns segundos.");
            default:
                return new Situacao(EstadoSite.Mudando, "O site está desligando…", "Aguarde alguns segundos.");
        }

        try
        {
            using var resposta = await Http.GetAsync(instalacao.Pagina("api/auth/estado"), ct);
            if (resposta.IsSuccessStatusCode)
            {
                var versao = LerVersao(await resposta.Content.ReadAsStringAsync(ct));
                return new Situacao(EstadoSite.NoAr, "O site está no ar.",
                    versao is null ? instalacao.Site.ToString() : $"Versão {versao} · {instalacao.Site}", versao);
            }
        }
        catch (HttpRequestException) { }
        catch (TaskCanceledException) when (!ct.IsCancellationRequested) { }
        return new Situacao(EstadoSite.SemResposta, "O site está ligado, mas não responde.",
            "Acabou de ligar? Espere alguns segundos. Se continuar assim, clique em “Reiniciar”.");
    }

    /// <summary>"versao" da resposta de /api/auth/estado (versões antigas do site não informam).</summary>
    private static string? LerVersao(string json)
    {
        try
        {
            using var doc = JsonDocument.Parse(json);
            return doc.RootElement.TryGetProperty("versao", out var v) && v.ValueKind == JsonValueKind.String ? v.GetString() : null;
        }
        catch (JsonException)
        {
            return null;
        }
    }

    private static bool PortaOcupada(Instalacao instalacao)
    {
        try
        {
            return IPGlobalProperties.GetIPGlobalProperties().GetActiveTcpListeners()
                .Any(e => e.Port == instalacao.Site.Port);
        }
        catch (NetworkInformationException)
        {
            return false;
        }
    }
}
