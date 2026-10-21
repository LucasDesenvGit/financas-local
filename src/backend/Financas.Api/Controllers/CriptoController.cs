using Financas.Api.Dtos;
using Financas.Api.Servicos.Cripto;
using Microsoft.AspNetCore.Mvc;

namespace Financas.Api.Controllers;

/// <summary>Criptomoedas: cotação em reais guardada pela API (Mercado Bitcoin, ADR 0048).</summary>
[Route("api/cripto")]
public class CriptoController : BaseController
{
    private readonly CriptoService _cripto;

    public CriptoController(CriptoService cripto) => _cripto = cripto;

    /// <summary>Última cotação guardada de cada criptomoeda, com a variação em 24 h quando houver (não consulta a internet).</summary>
    [HttpGet("cotacoes")]
    public async Task<ActionResult<CotacoesCriptoDto>> Cotacoes(CancellationToken ct)
        => await _cripto.UltimasAsync(ct);
}
