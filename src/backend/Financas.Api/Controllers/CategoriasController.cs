using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Controllers;

[Route("api/categorias")]
public class CategoriasController : BaseController
{
    private readonly FinancasDbContext _db;

    public CategoriasController(FinancasDbContext db) => _db = db;

    [HttpGet]
    public async Task<ActionResult<List<CategoriaDto>>> Listar(CancellationToken ct)
        => await _db.Categorias.AsNoTracking()
            .OrderBy(c => c.Tipo).ThenBy(c => c.Nome)
            .Select(c => new CategoriaDto(c.Id, c.Nome, c.Tipo, c.Cor, c.Icone, c.CategoriaPaiId, c.Ativa, c.Natureza))
            .ToListAsync(ct);

    [HttpPost]
    public async Task<ActionResult<CategoriaDto>> Criar(SalvarCategoriaDto dto, CancellationToken ct)
    {
        var problema = await ValidarAsync(null, dto, ct);
        if (problema is not null) return Erro(problema);

        var c = new Categoria();
        Aplicar(c, dto);
        _db.Categorias.Add(c);
        await _db.SaveChangesAsync(ct);
        return new CategoriaDto(c.Id, c.Nome, c.Tipo, c.Cor, c.Icone, c.CategoriaPaiId, c.Ativa, c.Natureza);
    }

    [HttpPut("{id:int}")]
    public async Task<ActionResult<CategoriaDto>> Atualizar(int id, SalvarCategoriaDto dto, CancellationToken ct)
    {
        var c = await _db.Categorias.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (c is null) return NaoEncontrado();
        var problema = await ValidarAsync(id, dto, ct);
        if (problema is not null) return Erro(problema);

        var mudouTipo = c.Tipo != dto.Tipo;
        Aplicar(c, dto);
        await _db.SaveChangesAsync(ct);

        if (mudouTipo)
        {
            // Recalcula o tipo das transações desta categoria.
            if (dto.Tipo == TipoCategoria.Transferencia)
            {
                await _db.Transacoes.Where(t => t.CategoriaId == id)
                    .ExecuteUpdateAsync(s => s
                        .SetProperty(t => t.Tipo, TipoTransacao.Transferencia)
                        .SetProperty(t => t.IgnorarRelatorios, true), ct);
            }
            else
            {
                await _db.Transacoes.Where(t => t.CategoriaId == id && t.TransferenciaGrupo == null)
                    .ExecuteUpdateAsync(s => s
                        .SetProperty(t => t.Tipo, t => t.Valor < 0 ? TipoTransacao.Despesa : TipoTransacao.Receita)
                        .SetProperty(t => t.IgnorarRelatorios, false), ct);
            }
        }
        return new CategoriaDto(c.Id, c.Nome, c.Tipo, c.Cor, c.Icone, c.CategoriaPaiId, c.Ativa, c.Natureza);
    }

    /// <summary>Exclui a categoria. As transações ficam sem categoria; orçamentos e regras dela são removidos.</summary>
    [HttpDelete("{id:int}")]
    public async Task<IActionResult> Excluir(int id, CancellationToken ct)
    {
        if (!await _db.Categorias.AnyAsync(c => c.Id == id, ct)) return NaoEncontrado();
        if (await _db.Categorias.AnyAsync(c => c.CategoriaPaiId == id, ct))
            return Erro("Exclua ou mova as subcategorias antes.");

        await _db.Transacoes.Where(t => t.CategoriaId == id)
            .ExecuteUpdateAsync(s => s
                .SetProperty(t => t.CategoriaId, (int?)null)
                .SetProperty(t => t.CategoriaManual, false), ct);
        await _db.Categorias.Where(c => c.Id == id).ExecuteDeleteAsync(ct);
        return NoContent();
    }

    private async Task<string?> ValidarAsync(int? id, SalvarCategoriaDto dto, CancellationToken ct)
    {
        var nome = dto.Nome.Trim();
        if (await _db.Categorias.AnyAsync(c => c.Nome == nome && c.Id != id, ct))
            return "Já existe uma categoria com esse nome.";
        if (dto.CategoriaPaiId is { } paiId)
        {
            if (paiId == id) return "Uma categoria não pode ser pai dela mesma.";
            var pai = await _db.Categorias.AsNoTracking().FirstOrDefaultAsync(c => c.Id == paiId, ct);
            if (pai is null) return "Categoria pai inválida.";
            if (pai.CategoriaPaiId is not null) return "Só é permitido um nível de subcategoria.";
            if (pai.Tipo != dto.Tipo) return "A subcategoria deve ter o mesmo tipo da categoria pai.";
            if (id is not null && await _db.Categorias.AnyAsync(c => c.CategoriaPaiId == id, ct))
                return "Esta categoria possui subcategorias e não pode virar subcategoria.";
        }
        return null;
    }

    private static void Aplicar(Categoria c, SalvarCategoriaDto dto)
    {
        c.Nome = dto.Nome.Trim();
        c.Tipo = dto.Tipo;
        c.Cor = dto.Cor;
        c.Icone = string.IsNullOrWhiteSpace(dto.Icone) ? null : dto.Icone.Trim();
        c.CategoriaPaiId = dto.CategoriaPaiId;
        c.Ativa = dto.Ativa;
        c.Natureza = dto.Tipo == TipoCategoria.Despesa ? dto.Natureza : null;
    }
}

[Route("api/regras")]
public class RegrasController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly CategorizacaoService _categorizacao;

    public RegrasController(FinancasDbContext db, CategorizacaoService categorizacao)
    {
        _db = db;
        _categorizacao = categorizacao;
    }

    [HttpGet]
    public async Task<ActionResult<List<RegraDto>>> Listar(CancellationToken ct)
        => await _db.RegrasCategorizacao.AsNoTracking()
            .OrderBy(r => r.Prioridade).ThenBy(r => r.Id)
            .Select(r => new RegraDto(r.Id, r.Texto, r.TipoCorrespondencia, r.CategoriaId, r.Categoria!.Nome,
                r.Prioridade, r.Ativa))
            .ToListAsync(ct);

    [HttpPost]
    public async Task<ActionResult<RegraDto>> Criar(SalvarRegraDto dto, CancellationToken ct)
    {
        var categoria = await _db.Categorias.AsNoTracking().FirstOrDefaultAsync(c => c.Id == dto.CategoriaId, ct);
        if (categoria is null) return Erro("Categoria inválida.");
        var r = new RegraCategorizacao { CriadoEm = DateTime.UtcNow };
        Aplicar(r, dto);
        _db.RegrasCategorizacao.Add(r);
        await _db.SaveChangesAsync(ct);
        return new RegraDto(r.Id, r.Texto, r.TipoCorrespondencia, r.CategoriaId, categoria.Nome, r.Prioridade, r.Ativa);
    }

    [HttpPut("{id:int}")]
    public async Task<ActionResult<RegraDto>> Atualizar(int id, SalvarRegraDto dto, CancellationToken ct)
    {
        var r = await _db.RegrasCategorizacao.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (r is null) return NaoEncontrado();
        var categoria = await _db.Categorias.AsNoTracking().FirstOrDefaultAsync(c => c.Id == dto.CategoriaId, ct);
        if (categoria is null) return Erro("Categoria inválida.");
        Aplicar(r, dto);
        await _db.SaveChangesAsync(ct);
        return new RegraDto(r.Id, r.Texto, r.TipoCorrespondencia, r.CategoriaId, categoria.Nome, r.Prioridade, r.Ativa);
    }

    [HttpDelete("{id:int}")]
    public async Task<IActionResult> Excluir(int id, CancellationToken ct)
    {
        var apagadas = await _db.RegrasCategorizacao.Where(r => r.Id == id).ExecuteDeleteAsync(ct);
        return apagadas == 0 ? NaoEncontrado() : NoContent();
    }

    /// <summary>
    /// Recategoriza todas as transações que não tiveram a categoria escolhida manualmente,
    /// aplicando as regras atuais e o mapeamento de categorias do Open Finance.
    /// </summary>
    [HttpPost("reaplicar")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> Reaplicar(CancellationToken ct)
    {
        await _categorizacao.CarregarAsync(ct);
        var alteradas = 0;
        long ultimoId = 0;
        while (true)
        {
            var lote = await _db.Transacoes
                .Where(t => !t.CategoriaManual && t.Id > ultimoId)
                .OrderBy(t => t.Id).Take(2000).ToListAsync(ct);
            if (lote.Count == 0) break;

            foreach (var t in lote)
            {
                var nova = _categorizacao.Resolver(t.DescricaoOriginal ?? t.Descricao, t.PluggyCategoriaId);
                if (nova != t.CategoriaId)
                {
                    t.CategoriaId = nova;
                    _categorizacao.AplicarTipo(t);
                    t.AtualizadoEm = DateTime.UtcNow;
                    alteradas++;
                }
            }
            ultimoId = lote[^1].Id;
            await _db.SaveChangesAsync(ct);
            _db.ChangeTracker.Clear();
        }
        return new ResultadoAplicacaoDto(alteradas);
    }

    private static void Aplicar(RegraCategorizacao r, SalvarRegraDto dto)
    {
        r.Texto = dto.Texto.Trim();
        r.TipoCorrespondencia = dto.TipoCorrespondencia;
        r.CategoriaId = dto.CategoriaId;
        r.Prioridade = dto.Prioridade;
        r.Ativa = dto.Ativa;
    }
}

/// <summary>
/// #123 (ADR 0030): regras de descrição, "quando a descrição do banco for/contiver X, mostrar Y". Editar, desligar ou
/// apagar uma regra reaplica as regras nas transações de nome automático (nome escolhido à mão não muda).
/// </summary>
[Route("api/regras-descricao")]
public class RegrasDescricaoController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly RegraDescricaoService _descricoes;

    public RegrasDescricaoController(FinancasDbContext db, RegraDescricaoService descricoes)
    {
        _db = db;
        _descricoes = descricoes;
    }

    [HttpGet]
    public async Task<ActionResult<List<RegraDescricaoDto>>> Listar(CancellationToken ct)
    {
        var contagem = await _db.Transacoes.AsNoTracking().Where(t => t.RegraDescricaoId != null)
            .GroupBy(t => t.RegraDescricaoId!.Value).Select(g => new { g.Key, Quantidade = g.Count() })
            .ToDictionaryAsync(x => x.Key, x => x.Quantidade, ct);
        var regras = await _db.RegrasDescricao.AsNoTracking().OrderBy(r => r.NovaDescricao).ThenBy(r => r.Id).ToListAsync(ct);
        return regras.Select(r => ParaDto(r, contagem.GetValueOrDefault(r.Id))).ToList();
    }

    /// <summary>Quantas transações de nome automático uma regra nova renomearia (e alguns exemplos).</summary>
    [HttpGet("previa")]
    public async Task<ActionResult<PreviaRegraDescricaoDto>> Previa([FromQuery] string? texto,
        [FromQuery] TipoCorrespondencia tipo = TipoCorrespondencia.Contem, CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(texto)) return new PreviaRegraDescricaoDto(0, new());
        var (quantidade, exemplos) = await _descricoes.PreviaAsync(texto.Trim(), tipo, excetoId: 0, ct);
        return new PreviaRegraDescricaoDto(quantidade, exemplos);
    }

    /// <summary>Regra nova pela tela de regras: renomeia de uma vez as transações de nome automático que batem.</summary>
    [HttpPost]
    public async Task<ActionResult<RegraDescricaoDto>> Criar(SalvarRegraDescricaoDto dto, CancellationToken ct)
    {
        if (!Enum.IsDefined(dto.TipoCorrespondencia)) return Erro("Tipo de correspondência inválido.");
        var r = new RegraDescricao
        {
            Texto = dto.Texto.Trim(), TipoCorrespondencia = dto.TipoCorrespondencia, NovaDescricao = dto.NovaDescricao.Trim(),
            Ativa = dto.Ativa, CriadoEm = DateTime.UtcNow
        };
        _db.RegrasDescricao.Add(r);
        await _db.SaveChangesAsync(ct);
        await _descricoes.ReaplicarAsync(ct);
        await _db.SaveChangesAsync(ct);
        var quantidade = await _db.Transacoes.CountAsync(t => t.RegraDescricaoId == r.Id, ct);
        return ParaDto(r, quantidade);
    }

    [HttpPut("{id:int}")]
    public async Task<ActionResult<RegraDescricaoDto>> Atualizar(int id, SalvarRegraDescricaoDto dto, CancellationToken ct)
    {
        var r = await _db.RegrasDescricao.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (r is null) return NaoEncontrado();
        if (!Enum.IsDefined(dto.TipoCorrespondencia)) return Erro("Tipo de correspondência inválido.");
        r.Texto = dto.Texto.Trim();
        r.TipoCorrespondencia = dto.TipoCorrespondencia;
        r.NovaDescricao = dto.NovaDescricao.Trim();
        r.Ativa = dto.Ativa;
        await _db.SaveChangesAsync(ct);
        await _descricoes.ReaplicarAsync(ct);
        await _db.SaveChangesAsync(ct);
        var quantidade = await _db.Transacoes.CountAsync(t => t.RegraDescricaoId == id, ct);
        return ParaDto(r, quantidade);
    }

    /// <summary>Apaga a regra; as transações com o nome dado por ela voltam ao nome do banco (ou de outra regra).</summary>
    [HttpDelete("{id:int}")]
    public async Task<IActionResult> Excluir(int id, CancellationToken ct)
    {
        var r = await _db.RegrasDescricao.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (r is null) return NaoEncontrado();
        var transacoes = await _db.Transacoes.Where(t => t.RegraDescricaoId == id).ToListAsync(ct);
        foreach (var t in transacoes)
        {
            t.Descricao = t.DescricaoOriginal ?? t.Descricao;
            t.RegraDescricaoId = null;
            t.AtualizadoEm = DateTime.UtcNow;
        }
        _db.RegrasDescricao.Remove(r);
        await _db.SaveChangesAsync(ct);
        await _descricoes.ReaplicarAsync(ct);
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    private static RegraDescricaoDto ParaDto(RegraDescricao r, int transacoes)
        => new(r.Id, r.Texto, r.TipoCorrespondencia, r.NovaDescricao, r.Ativa, transacoes);
}
