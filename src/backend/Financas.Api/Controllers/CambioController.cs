using Financas.Api.Dtos;
using Financas.Api.Servicos.Cambio;
using Microsoft.AspNetCore.Mvc;

namespace Financas.Api.Controllers;

/// <summary>Câmbio: moedas disponíveis e cotação PTAX do Banco Central.</summary>
[Route("api/cambio")]
public class CambioController : BaseController
{
    private readonly CambioService _cambio;

    public CambioController(CambioService cambio) => _cambio = cambio;

    /// <summary>Moedas com cotação publicada pelo Banco Central.</summary>
    [HttpGet("moedas")]
    public async Task<ActionResult<List<MoedaDto>>> Moedas(CancellationToken ct)
        => await _cambio.ListarMoedasAsync(ct);

    /// <summary>Última cotação guardada de cada moeda (não consulta a internet).</summary>
    [HttpGet("cotacoes")]
    public async Task<ActionResult<CotacoesAtuaisDto>> Cotacoes(CancellationToken ct)
        => await _cambio.UltimasAsync(ct);

    /// <summary>Cotação de fechamento em reais na data (padrão: hoje), ou do último dia útil anterior.</summary>
    [HttpGet("cotacao")]
    public async Task<ActionResult<CotacaoMoedaDto>> Cotacao([FromQuery] string? moeda, [FromQuery] DateOnly? data,
        CancellationToken ct)
    {
        var codigo = CambioService.NormalizarMoeda(moeda);
        if (codigo is null) return Erro("Informe a moeda com o código de 3 letras (ex.: USD, EUR).");

        if (codigo != CambioService.MoedaBase)
        {
            var moedas = await _cambio.ListarMoedasAsync(ct);
            if (moedas.Count > 0 && moedas.All(m => m.Codigo != codigo))
                return Erro($"O Banco Central não publica cotação para {codigo}.");
        }

        var cotacao = await _cambio.ObterCotacaoAsync(codigo, data, ct);
        return cotacao is null
            ? Erro($"Cotação de {codigo} indisponível no momento. Tente novamente mais tarde.",
                StatusCodes.Status503ServiceUnavailable)
            : cotacao;
    }
}
