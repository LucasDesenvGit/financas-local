using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos;
using Financas.Api.Servicos.Diagnostico;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Controllers;

/// <summary>Erros e avisos do sistema (#11, ADR 0022): lista com filtros, detalhe sob demanda e "resolvido".</summary>
[Route("api/erros")]
public class ErrosController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly FilaErros _fila;

    public ErrosController(FinancasDbContext db, FilaErros fila)
    {
        _db = db;
        _fila = fila;
    }

    [HttpGet]
    public async Task<ActionResult<PaginaErrosDto>> Listar([FromQuery] FiltroErrosDto filtro, CancellationToken ct)
    {
        var q = Filtrar(filtro);
        var total = await q.CountAsync(ct);
        var itens = await q.OrderByDescending(e => e.UltimaEm).ThenByDescending(e => e.Id)
            .Skip((filtro.Pagina - 1) * filtro.TamanhoPagina).Take(filtro.TamanhoPagina)
            .Select(e => new ErroSistemaDto(e.Id, e.PrimeiraEm, e.UltimaEm, e.Ocorrencias, e.Nivel, e.Origem,
                e.Mensagem, e.Rota, e.Usuario, e.Resolvido, e.ResolvidoEm))
            .ToListAsync(ct);
        var abertos = await _db.ErrosSistema.CountAsync(e => !e.Resolvido, ct);
        var origens = await _db.ErrosSistema.Select(e => e.Origem).Distinct().OrderBy(o => o).ToListAsync(ct);
        return new PaginaErrosDto(itens.Select(Utc).ToList(), total, filtro.Pagina, filtro.TamanhoPagina, abertos, origens);
    }

    /// <summary>Categoria e pilha: só quando a pessoa abre o detalhe (podem ter alguns KB cada).</summary>
    [HttpGet("{id:long}")]
    public async Task<ActionResult<ErroSistemaDetalheDto>> Detalhe(long id, CancellationToken ct)
    {
        var e = await _db.ErrosSistema.AsNoTracking()
            .Where(x => x.Id == id)
            .Select(x => new ErroSistemaDetalheDto(x.Id, x.Categoria, x.Detalhe))
            .FirstOrDefaultAsync(ct);
        return e is null ? NaoEncontrado() : e;
    }

    [HttpPut("{id:long}/situacao")]
    public async Task<IActionResult> Situacao(long id, SituacaoErroDto dto, CancellationToken ct)
    {
        var alterados = await _db.ErrosSistema.Where(e => e.Id == id)
            .ExecuteUpdateAsync(s => s
                .SetProperty(e => e.Resolvido, dto.Resolvido)
                .SetProperty(e => e.ResolvidoEm, dto.Resolvido ? DateTime.UtcNow : null), ct);
        return alterados == 0 ? NaoEncontrado() : NoContent();
    }

    /// <summary>Marca como resolvidos todos os em aberto que passam no filtro (todas as páginas).</summary>
    [HttpPost("resolver")]
    public async Task<ActionResult<QuantidadeDto>> ResolverTodos([FromQuery] FiltroErrosDto filtro, CancellationToken ct)
    {
        var agora = DateTime.UtcNow;
        var quantidade = await Filtrar(filtro).Where(e => !e.Resolvido)
            .ExecuteUpdateAsync(s => s.SetProperty(e => e.Resolvido, true).SetProperty(e => e.ResolvidoEm, agora), ct);
        return new QuantidadeDto(quantidade);
    }

    /// <summary>Erro não tratado no navegador (ErrorHandler do Angular). Sempre 204: descartar não é erro.</summary>
    [HttpPost("front-end")]
    public IActionResult RegistrarDoFrontEnd(ErroFrontEndDto dto)
    {
        _fila.RegistrarDoFrontEnd(new RegistroErro(DateTime.UtcNow, NivelErro.Erro, FilaErros.OrigemFrontEnd,
            FilaErros.OrigemFrontEnd, dto.Mensagem, dto.Detalhe, dto.Rota is null ? null : $"tela {dto.Rota}",
            User.Identity?.Name));
        return NoContent();
    }

    private IQueryable<ErroSistema> Filtrar(FiltroErrosDto f)
    {
        var q = _db.ErrosSistema.AsNoTracking().AsQueryable();
        q = (f.Situacao ?? "abertos") switch
        {
            "resolvidos" => q.Where(e => e.Resolvido),
            "todos" => q,
            _ => q.Where(e => !e.Resolvido)
        };
        if (!string.IsNullOrWhiteSpace(f.Origem)) q = q.Where(e => e.Origem == f.Origem);
        if (f.Nivel is { } nivel) q = q.Where(e => e.Nivel == nivel);
        // Período pela última ocorrência, em dias de Brasília.
        if (f.Inicio is { } inicio)
        {
            var desde = Relogio.InicioDoDiaUtc(inicio);
            q = q.Where(e => e.UltimaEm >= desde);
        }
        if (f.Fim is { } fim)
        {
            var ate = Relogio.InicioDoDiaUtc(fim.AddDays(1));
            q = q.Where(e => e.UltimaEm < ate);
        }
        return q;
    }

    /// <summary>O SQLite devolve as datas sem fuso; elas foram gravadas em UTC.</summary>
    private static ErroSistemaDto Utc(ErroSistemaDto e) => e with
    {
        PrimeiraEm = DateTime.SpecifyKind(e.PrimeiraEm, DateTimeKind.Utc),
        UltimaEm = DateTime.SpecifyKind(e.UltimaEm, DateTimeKind.Utc),
        ResolvidoEm = e.ResolvidoEm is { } r ? DateTime.SpecifyKind(r, DateTimeKind.Utc) : null
    };
}
