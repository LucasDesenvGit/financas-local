using System.Linq.Expressions;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Controllers;

public static class ProjecoesTransacao
{
    public static readonly Expression<Func<Transacao, TransacaoDto>> ParaDto = t => new TransacaoDto(
        t.Id, t.ContaId, t.Conta!.Nome, t.PluggyTransactionId != null, t.Data, t.Descricao, t.DescricaoOriginal,
        t.Estabelecimento, t.Valor, t.Tipo, t.Status, t.CategoriaId,
        t.Categoria != null ? t.Categoria.Nome : null,
        t.Categoria != null ? t.Categoria.Cor : null,
        t.ParcelaNumero, t.ParcelaTotal, t.IgnorarRelatorios, t.Observacao, t.Emergencia);
}

[Route("api/transacoes")]
public class TransacoesController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly CategorizacaoService _categorizacao;

    public TransacoesController(FinancasDbContext db, CategorizacaoService categorizacao)
    {
        _db = db;
        _categorizacao = categorizacao;
    }

    [HttpGet]
    public async Task<ActionResult<PaginaTransacoesDto>> Listar([FromQuery] FiltroTransacoesDto f, CancellationToken ct)
    {
        var q = _db.Transacoes.AsNoTracking().AsQueryable();
        if (f.Inicio is { } inicio) q = q.Where(t => t.Data >= inicio);
        if (f.Fim is { } fim) q = q.Where(t => t.Data <= fim);
        if (f.ContaId is { } contaId) q = q.Where(t => t.ContaId == contaId);
        // Categorias marcadas (cada uma com as subcategorias) e/ou "sem categoria": basta atender a uma delas.
        var categorias = (f.CategoriaIds ?? []).Concat(f.CategoriaId is { } c ? [c] : []).Distinct().ToList();
        var semCategoria = f.SemCategoria;
        if (semCategoria || categorias.Count > 0)
            q = q.Where(t => (semCategoria && t.CategoriaId == null)
                             || (t.CategoriaId != null && (categorias.Contains(t.CategoriaId.Value)
                                 || (t.Categoria!.CategoriaPaiId != null && categorias.Contains(t.Categoria.CategoriaPaiId.Value)))));
        if (f.Tipo is { } tipo) q = q.Where(t => t.Tipo == tipo);
        if (f.SomenteEmergencias) q = q.Where(t => t.Emergencia);
        if (!string.IsNullOrWhiteSpace(f.Busca))
        {
            var busca = f.Busca.Trim();
            q = q.Where(t => t.Descricao.Contains(busca)
                             || (t.Estabelecimento != null && t.Estabelecimento.Contains(busca))
                             || (t.Observacao != null && t.Observacao.Contains(busca)));
        }

        var total = await q.CountAsync(ct);
        var paraTotais = q.Where(t => !t.IgnorarRelatorios && t.Tipo != TipoTransacao.Transferencia);
        var receitas = await paraTotais.Where(t => t.Valor > 0).SumAsync(t => (decimal?)t.Valor, ct) ?? 0m;
        var despesas = await paraTotais.Where(t => t.Valor < 0).SumAsync(t => (decimal?)t.Valor, ct) ?? 0m;

        var itens = await q
            .OrderByDescending(t => t.Data).ThenByDescending(t => t.Id)
            .Skip((f.Pagina - 1) * f.TamanhoPagina)
            .Take(f.TamanhoPagina)
            .Select(ProjecoesTransacao.ParaDto)
            .ToListAsync(ct);

        return new PaginaTransacoesDto(itens, total, f.Pagina, f.TamanhoPagina, receitas, Math.Abs(despesas));
    }

    [HttpGet("{id:long}")]
    public async Task<ActionResult<TransacaoDto>> Obter(long id, CancellationToken ct)
    {
        var dto = await _db.Transacoes.AsNoTracking().Where(t => t.Id == id).Select(ProjecoesTransacao.ParaDto)
            .FirstOrDefaultAsync(ct);
        if (dto is null) return NaoEncontrado();
        return dto;
    }

    /// <summary>Lançamento manual (somente em contas manuais).</summary>
    [HttpPost]
    public async Task<ActionResult<TransacaoDto>> Criar(SalvarTransacaoDto dto, CancellationToken ct)
    {
        var validacao = await ValidarManualAsync(dto, ct);
        if (validacao is not null) return validacao;

        await _categorizacao.CarregarAsync(ct);
        var t = new Transacao { ContaId = dto.ContaId, CriadoEm = DateTime.UtcNow };
        Aplicar(t, dto);
        _db.Transacoes.Add(t);
        await _db.SaveChangesAsync(ct);
        return CreatedAtAction(nameof(Obter), new { id = t.Id }, await ProjetarAsync(t.Id, ct));
    }

    /// <summary>Edição completa (somente transações manuais).</summary>
    [HttpPut("{id:long}")]
    public async Task<ActionResult<TransacaoDto>> Atualizar(long id, SalvarTransacaoDto dto, CancellationToken ct)
    {
        var t = await _db.Transacoes.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (t is null) return NaoEncontrado();
        if (t.PluggyTransactionId is not null)
            return Erro("Transações do Open Finance só permitem alterar descrição, categoria e observação.");
        if (t.TransferenciaGrupo is not null)
            return Erro("Para alterar uma transferência, exclua-a e lance novamente.");

        var validacao = await ValidarManualAsync(dto, ct);
        if (validacao is not null) return validacao;

        await _categorizacao.CarregarAsync(ct);
        t.ContaId = dto.ContaId;
        Aplicar(t, dto);
        await _db.SaveChangesAsync(ct);
        return await ProjetarAsync(id, ct);
    }

    /// <summary>Ajustes permitidos em qualquer transação (inclusive Open Finance).</summary>
    [HttpPatch("{id:long}")]
    public async Task<ActionResult<TransacaoDto>> Ajustar(long id, AtualizarTransacaoOpenFinanceDto dto, CancellationToken ct)
    {
        var t = await _db.Transacoes.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (t is null) return NaoEncontrado();
        if (dto.CategoriaId is { } cid && !await _db.Categorias.AnyAsync(c => c.Id == cid, ct))
            return Erro("Categoria inválida.");

        await _categorizacao.CarregarAsync(ct);
        t.Descricao = dto.Descricao.Trim();
        t.Observacao = string.IsNullOrWhiteSpace(dto.Observacao) ? null : dto.Observacao.Trim();
        if (dto.CategoriaId is null)
        {
            // Sem categoria escolhida: volta para a categorização automática.
            t.CategoriaManual = false;
            t.CategoriaId = _categorizacao.Resolver(t.DescricaoOriginal ?? t.Descricao, t.PluggyCategoriaId);
        }
        else
        {
            t.CategoriaManual = true;
            t.CategoriaId = dto.CategoriaId;
        }
        _categorizacao.AplicarTipo(t);
        if (t.Tipo != TipoTransacao.Transferencia) t.IgnorarRelatorios = dto.IgnorarRelatorios;
        t.Emergencia = dto.Emergencia && t.Valor < 0;
        t.AtualizadoEm = DateTime.UtcNow;
        await _db.SaveChangesAsync(ct);
        return await ProjetarAsync(id, ct);
    }

    /// <summary>Marca ou desmarca transações como gasto de emergência (somente despesas).</summary>
    [HttpPut("emergencia-em-lote")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> EmergenciaEmLote(EmergenciaLoteDto dto, CancellationToken ct)
    {
        var ids = dto.Ids.Distinct().ToList();
        var alteradas = await _db.Transacoes
            .Where(t => ids.Contains(t.Id) && (!dto.Emergencia || t.Valor < 0))
            .ExecuteUpdateAsync(s => s
                .SetProperty(t => t.Emergencia, dto.Emergencia)
                .SetProperty(t => t.AtualizadoEm, DateTime.UtcNow), ct);
        return new ResultadoAplicacaoDto(alteradas);
    }

    /// <summary>Sugestões de categoria agrupadas por descrição parecida (histórico + palavras-chave).</summary>
    [HttpGet("sugestoes-categoria")]
    public async Task<ActionResult<List<SugestaoCategoriaDto>>> SugestoesCategoria(
        [FromServices] SugestaoCategoriaService sugestoes, [FromQuery] bool somenteSemCategoria = true,
        [FromQuery] int dias = 365, CancellationToken ct = default)
    {
        if (dias < 1 || dias > 3660) return Erro("Período inválido.");
        return await sugestoes.SugerirGruposAsync(somenteSemCategoria, dias, ct);
    }

    /// <summary>Até 3 categorias sugeridas para uma transação.</summary>
    [HttpGet("{id:long}/sugestoes-categoria")]
    public async Task<ActionResult<List<OpcaoCategoriaDto>>> SugestoesDaTransacao(long id,
        [FromServices] SugestaoCategoriaService sugestoes, CancellationToken ct)
    {
        var lista = await sugestoes.SugerirParaTransacaoAsync(id, ct);
        if (lista is null) return NaoEncontrado();
        return lista;
    }

    [HttpDelete("{id:long}")]
    public async Task<IActionResult> Excluir(long id, CancellationToken ct)
    {
        var t = await _db.Transacoes.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (t is null) return NaoEncontrado();
        if (t.PluggyTransactionId is not null)
            return Erro("Transações do Open Finance não podem ser excluídas. Marque 'Ignorar nos relatórios' se necessário.");

        if (t.TransferenciaGrupo is { } grupo)
        {
            var par = await _db.Transacoes.Where(x => x.TransferenciaGrupo == grupo).ToListAsync(ct);
            _db.Transacoes.RemoveRange(par);
        }
        else
        {
            _db.Transacoes.Remove(t);
        }
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpPut("categoria-em-lote")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> CategorizarEmLote(CategorizarLoteDto dto, CancellationToken ct)
    {
        if (dto.CategoriaId is { } cid && !await _db.Categorias.AnyAsync(c => c.Id == cid, ct))
            return Erro("Categoria inválida.");

        await _categorizacao.CarregarAsync(ct);
        var ids = dto.Ids.Distinct().ToList();
        var transacoes = await _db.Transacoes.Where(t => ids.Contains(t.Id)).ToListAsync(ct);
        foreach (var t in transacoes)
        {
            if (dto.CategoriaId is null)
            {
                t.CategoriaManual = false;
                t.CategoriaId = _categorizacao.Resolver(t.DescricaoOriginal ?? t.Descricao, t.PluggyCategoriaId);
            }
            else
            {
                t.CategoriaManual = true;
                t.CategoriaId = dto.CategoriaId;
            }
            _categorizacao.AplicarTipo(t);
            t.AtualizadoEm = DateTime.UtcNow;
        }
        await _db.SaveChangesAsync(ct);
        return new ResultadoAplicacaoDto(transacoes.Count);
    }

    /// <summary>
    /// Transferência entre contas. Lança somente nas contas manuais: o lado Open Finance chega pela sincronização.
    /// </summary>
    [HttpPost("transferencia")]
    public async Task<IActionResult> Transferir(TransferenciaDto dto, CancellationToken ct)
    {
        if (dto.ContaOrigemId == dto.ContaDestinoId) return Erro("Escolha contas diferentes.");
        var contas = await _db.Contas.Where(c => c.Id == dto.ContaOrigemId || c.Id == dto.ContaDestinoId)
            .ToDictionaryAsync(c => c.Id, ct);
        if (contas.Count != 2) return Erro("Conta inválida.");
        var origem = contas[dto.ContaOrigemId];
        var destino = contas[dto.ContaDestinoId];
        if (origem.Origem != OrigemConta.Manual && destino.Origem != OrigemConta.Manual)
            return Erro("Entre contas do Open Finance as transferências são importadas automaticamente.");

        var categoriaTransferencia = await _db.Categorias.AsNoTracking()
            .Where(c => c.Tipo == TipoCategoria.Transferencia && c.Nome == "Transferências")
            .Select(c => (int?)c.Id).FirstOrDefaultAsync(ct);
        var grupo = Guid.NewGuid();
        var descricao = string.IsNullOrWhiteSpace(dto.Descricao) ? "Transferência" : dto.Descricao.Trim();

        void Lancar(Conta conta, decimal valor, string complemento) => _db.Transacoes.Add(new Transacao
        {
            ContaId = conta.Id,
            Data = dto.Data,
            Descricao = $"{descricao} {complemento}",
            Valor = valor,
            Tipo = TipoTransacao.Transferencia,
            Status = StatusTransacao.Efetivada,
            CategoriaId = categoriaTransferencia,
            CategoriaManual = true,
            IgnorarRelatorios = true,
            TransferenciaGrupo = grupo,
            CriadoEm = DateTime.UtcNow,
            AtualizadoEm = DateTime.UtcNow
        });

        if (origem.Origem == OrigemConta.Manual) Lancar(origem, -dto.Valor, $"para {destino.Nome}");
        if (destino.Origem == OrigemConta.Manual) Lancar(destino, dto.Valor, $"de {origem.Nome}");
        await _db.SaveChangesAsync(ct); // um único SaveChanges = transação atômica
        return NoContent();
    }

    private async Task<ObjectResult?> ValidarManualAsync(SalvarTransacaoDto dto, CancellationToken ct)
    {
        if (dto.Tipo == TipoTransacao.Transferencia)
            return Erro("Para transferências use a opção 'Transferência'.");
        var conta = await _db.Contas.AsNoTracking().FirstOrDefaultAsync(c => c.Id == dto.ContaId, ct);
        if (conta is null) return Erro("Conta inválida.");
        if (conta.Origem != OrigemConta.Manual)
            return Erro("Não é possível lançar manualmente em contas do Open Finance. Os lançamentos chegam pela sincronização.");
        if (dto.CategoriaId is { } cid && !await _db.Categorias.AnyAsync(c => c.Id == cid, ct))
            return Erro("Categoria inválida.");
        return null;
    }

    private void Aplicar(Transacao t, SalvarTransacaoDto dto)
    {
        t.Data = dto.Data;
        t.Descricao = dto.Descricao.Trim();
        t.DescricaoOriginal = null;
        t.Valor = dto.Tipo == TipoTransacao.Receita ? Math.Abs(dto.Valor) : -Math.Abs(dto.Valor);
        t.Status = dto.Status;
        t.Observacao = string.IsNullOrWhiteSpace(dto.Observacao) ? null : dto.Observacao.Trim();
        t.CategoriaManual = dto.CategoriaId is not null;
        t.CategoriaId = dto.CategoriaId ?? _categorizacao.Resolver(t.Descricao, null);
        _categorizacao.AplicarTipo(t);
        if (t.Tipo != TipoTransacao.Transferencia) t.IgnorarRelatorios = dto.IgnorarRelatorios;
        t.Emergencia = dto.Emergencia && t.Valor < 0;
        t.AtualizadoEm = DateTime.UtcNow;
    }

    private async Task<TransacaoDto> ProjetarAsync(long id, CancellationToken ct)
        => await _db.Transacoes.AsNoTracking().Where(t => t.Id == id).Select(ProjecoesTransacao.ParaDto).FirstAsync(ct);
}
