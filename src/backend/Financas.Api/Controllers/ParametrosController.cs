using Financas.Api.Dtos;
using Financas.Api.Servicos;
using Microsoft.AspNetCore.Mvc;

namespace Financas.Api.Controllers;

[Route("api/parametros")]
public class ParametrosController : BaseController
{
    private readonly ParametroService _parametros;
    private readonly AuditoriaService _auditoria;

    public ParametrosController(ParametroService parametros, AuditoriaService auditoria)
    {
        _parametros = parametros;
        _auditoria = auditoria;
    }

    [HttpGet]
    public async Task<ActionResult<List<ParametroDto>>> Listar(CancellationToken ct)
        => await _parametros.ListarAsync(ct);

    [HttpPut]
    public async Task<ActionResult<List<ParametroDto>>> Salvar(SalvarParametrosDto dto, CancellationToken ct)
    {
        if (dto.Valores.Count == 0) return Erro("Nenhum parâmetro informado.");
        // Registrado antes: é gravado junto com os parâmetros, e só se todos forem válidos.
        _auditoria.Registrar("ParametrosAlterados", User.Identity?.Name);
        var erros = await _parametros.SalvarAsync(dto.Valores, ct);
        if (erros.Count > 0) return Erro(string.Join(" ", erros));
        return await _parametros.ListarAsync(ct);
    }

    /// <summary>Diagnóstico somente leitura: versão, caminhos, tamanho do banco e totais.</summary>
    [HttpGet("sistema")]
    public async Task<ActionResult<InfoSistemaDto>> Sistema(CancellationToken ct)
        => await _parametros.InfoSistemaAsync(ct);
}
