using System.Linq.Expressions;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Caching.Memory;

namespace Financas.Api.Controllers;

public static class ProjecoesTransacao
{
    public static readonly Expression<Func<Transacao, TransacaoDto>> ParaDto = t => new TransacaoDto(
        t.Id, t.ContaId, t.Conta!.Nome, t.PluggyTransactionId != null, t.Data, t.Descricao, t.DescricaoOriginal,
        t.Estabelecimento, t.Valor, t.Tipo, t.Status, t.CategoriaId,
        t.Categoria != null ? t.Categoria.Nome : null,
        t.Categoria != null ? t.Categoria.Cor : null,
        t.ParcelaNumero, t.ParcelaTotal, t.IgnorarRelatorios, t.Observacao, t.Emergencia,
        // Calculado no cliente (EF traz as colunas e chama o método na projeção final).
        TipoLancamentoService.Classificar(t.Valor, t.Descricao, t.DescricaoOriginal, t.MeioPagamento, t.MesmaTitularidade,
            t.TransferenciaGrupo != null, t.Conta!.Tipo),
        t.RegraDescricaoId);
}

[Route("api/transacoes")]
public class TransacoesController : BaseController
{
    /// <summary>#87: "todas do filtro" de uma vez, no máximo (acima disso, restringir o período).</summary>
    public const int MaximoPorFiltro = 5000;
    private static readonly TimeSpan PrazoDesfazer = TimeSpan.FromMinutes(10);

    private readonly FinancasDbContext _db;
    private readonly CategorizacaoService _categorizacao;
    private readonly AuditoriaService _auditoria;
    private readonly IMemoryCache _cache;
    private readonly DescricaoPoucoInformativaService _poucoInformativa;
    private readonly RegraDescricaoService _descricoes;
    private readonly TransferenciasService _transferencias;

    public TransacoesController(FinancasDbContext db, CategorizacaoService categorizacao, AuditoriaService auditoria,
        IMemoryCache cache, DescricaoPoucoInformativaService poucoInformativa, RegraDescricaoService descricoes,
        TransferenciasService transferencias)
    {
        _descricoes = descricoes;
        _transferencias = transferencias;
        _db = db;
        _categorizacao = categorizacao;
        _auditoria = auditoria;
        _cache = cache;
        _poucoInformativa = poucoInformativa;
    }

    [HttpGet]
    public async Task<ActionResult<PaginaTransacoesDto>> Listar([FromQuery] FiltroTransacoesDto f, CancellationToken ct)
    {
        var q = await FiltrarAsync(f, rastrear: false, ct);

        var total = await q.CountAsync(ct);
        var paraTotais = q.Where(t => !t.IgnorarRelatorios && t.Tipo != TipoTransacao.Transferencia);
        // Pelo tipo, não pelo sinal: estorno no cartão é despesa positiva e abate as despesas (ADR 0036).
        var receitas = await paraTotais.Where(t => t.Tipo == TipoTransacao.Receita).SumAsync(t => (decimal?)t.Valor, ct) ?? 0m;
        var despesas = await paraTotais.Where(t => t.Tipo == TipoTransacao.Despesa).SumAsync(t => (decimal?)t.Valor, ct) ?? 0m;

        var itens = await q
            .OrderByDescending(t => t.Data).ThenByDescending(t => t.Id)
            .Skip((f.Pagina - 1) * f.TamanhoPagina)
            .Take(f.TamanhoPagina)
            .Select(ProjecoesTransacao.ParaDto)
            .ToListAsync(ct);

        return new PaginaTransacoesDto(itens, total, f.Pagina, f.TamanhoPagina, receitas, -despesas);
    }

    /// <summary>Os filtros da busca (a mesma consulta da lista e do "todas do filtro" do modo edição).</summary>
    private async Task<IQueryable<Transacao>> FiltrarAsync(FiltroTransacoesDto f, bool rastrear, CancellationToken ct)
    {
        var q = rastrear ? _db.Transacoes.AsQueryable() : _db.Transacoes.AsNoTracking();
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
        // Natureza da categoria ou, se ela não tem, a da categoria pai; basta atender a uma das marcadas (OU).
        var naturezas = (f.Naturezas ?? []).Distinct().ToList();
        var semNatureza = f.SemNatureza;
        if (semNatureza || naturezas.Count > 0)
            q = q.Where(t => (semNatureza && (t.Categoria!.Natureza ?? t.Categoria.CategoriaPai!.Natureza) == null)
                             || naturezas.Contains((t.Categoria!.Natureza ?? t.Categoria.CategoriaPai!.Natureza)!.Value));
        if (f.Tipo is { } tipo) q = q.Where(t => t.Tipo == tipo);
        if (f.SomenteEmergencias) q = q.Where(t => t.Emergencia);
        if (!string.IsNullOrWhiteSpace(f.Busca))
        {
            // #123: acha pelo nome mostrado e pelo nome que veio do banco. #158: sem diferenciar maiúsculas ("netflix"
            // acha "NETFLIX.COM"): o LIKE do SQLite ignora a caixa das letras sem acento; % e _ digitados valem como texto.
            var padrao = "%" + f.Busca.Trim().Replace("\\", "\\\\").Replace("%", "\\%").Replace("_", "\\_") + "%";
            q = q.Where(t => EF.Functions.Like(t.Descricao, padrao, "\\")
                             || (t.DescricaoOriginal != null && EF.Functions.Like(t.DescricaoOriginal, padrao, "\\"))
                             || (t.Estabelecimento != null && EF.Functions.Like(t.Estabelecimento, padrao, "\\"))
                             || (t.Observacao != null && EF.Functions.Like(t.Observacao, padrao, "\\")));
        }
        if (f.Lancamento is { } grupo && TipoLancamentoService.Grupos.TryGetValue(grupo, out var tipos))
        {
            // #111: o tipo é calculado (dado do banco ou descrição): a conta é na memória, sobre o que já foi filtrado.
            var candidatas = await q.Select(t => new
            {
                t.Id, t.Valor, t.Descricao, t.DescricaoOriginal, t.MeioPagamento, t.MesmaTitularidade,
                Sistema = t.TransferenciaGrupo != null, ContaTipo = t.Conta!.Tipo
            }).ToListAsync(ct);
            var ids = candidatas.Where(c => tipos.Contains(TipoLancamentoService.Classificar(c.Valor, c.Descricao,
                c.DescricaoOriginal, c.MeioPagamento, c.MesmaTitularidade, c.Sistema, c.ContaTipo))).Select(c => c.Id).ToList();
            q = q.Where(t => ids.Contains(t.Id));
        }
        if (f.PoucoInformativa)
        {
            // #9: a conta é na memória, sobre o que os outros filtros já restringiram.
            var ids = (await _poucoInformativa.ListarAsync(q, ct)).Select(x => x.Id).ToList();
            q = q.Where(t => ids.Contains(t.Id));
        }
        return q;
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
        await AcertarTransferenciasAsync(ct);
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
        await AcertarTransferenciasAsync(ct);
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
        var descricao = dto.Descricao.Trim();
        if (descricao != t.Descricao)
        {
            // #123: nome trocado no formulário é escolhido à mão; as regras de descrição não mexem mais nele.
            t.Descricao = descricao;
            t.DescricaoManual = true;
            t.RegraDescricaoId = null;
        }
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
        await AcertarTransferenciasAsync(ct);
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

    /// <summary>#9: marca as transações de descrição pouco informativa como revisadas (saem do filtro), ou desfaz.</summary>
    [HttpPut("revisada-em-lote")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> RevisadaEmLote(RevisadaLoteDto dto, CancellationToken ct)
    {
        var ids = dto.Ids.Distinct().ToList();
        var alteradas = await _db.Transacoes
            .Where(t => ids.Contains(t.Id))
            .ExecuteUpdateAsync(s => s
                .SetProperty(t => t.DescricaoRevisada, dto.Revisada)
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
        await AcertarTransferenciasAsync(ct);
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
        await AcertarTransferenciasAsync(ct);
        return new ResultadoAplicacaoDto(transacoes.Count);
    }

    /// <summary>Estado de antes de uma alteração em lote, guardado só na memória da API para o "Desfazer".</summary>
    private sealed record EstadoAnterior(long Id, int? CategoriaId, bool CategoriaManual, TipoTransacao Tipo,
        bool IgnorarRelatorios, int? CategoriaAplicada);

    /// <summary>
    /// #87 (modo edição de Transações): troca só a categoria, por transação (<c>Itens</c>) ou de todas as transações
    /// do filtro da busca. Mesmas regras do PATCH de uma transação: categoria escolhida fica manual; "automática"
    /// reaplica as regras; o tipo acompanha a categoria (transferência). O estado anterior fica na memória da API
    /// por alguns minutos, para o "Desfazer" — o navegador recebe só a chave, não tem como mandar outros campos.
    /// </summary>
    [HttpPatch("em-lote")]
    public async Task<ActionResult<ResultadoCategoriasEmLoteDto>> AlterarCategoriasEmLote(AlterarCategoriasEmLoteDto dto,
        CancellationToken ct)
    {
        var porItens = dto.Itens is { Count: > 0 };
        if (porItens == (dto.Filtro is not null)) return Erro("Informe as transações ou o filtro (um dos dois).");

        var escolhas = porItens
            ? dto.Itens!.Select(i => (i.CategoriaId, i.Automatica)).ToList()
            : [(dto.CategoriaId, dto.Automatica)];
        if (escolhas.Any(e => !e.Automatica && e.CategoriaId is null))
            return Erro("Escolha uma categoria ou a categorização automática.");
        var categorias = escolhas.Where(e => !e.Automatica).Select(e => e.CategoriaId!.Value).Distinct().ToList();
        if (categorias.Count > 0 && await _db.Categorias.CountAsync(c => categorias.Contains(c.Id), ct) != categorias.Count)
            return Erro("Categoria inválida.");

        List<Transacao> transacoes;
        Func<Transacao, (int? CategoriaId, bool Automatica)> escolhaDe;
        if (porItens)
        {
            var ids = dto.Itens!.Select(i => i.Id).Distinct().ToList();
            if (ids.Count != dto.Itens!.Count) return Erro("Transação repetida na lista.");
            transacoes = await _db.Transacoes.Where(t => ids.Contains(t.Id)).ToListAsync(ct);
            if (transacoes.Count != ids.Count) return NaoEncontrado("Alguma transação não existe mais. Atualize a lista.");
            var porId = dto.Itens!.ToDictionary(i => i.Id, i => (i.CategoriaId, i.Automatica));
            escolhaDe = t => porId[t.Id];
        }
        else
        {
            var q = await FiltrarAsync(dto.Filtro!, rastrear: true, ct);
            var total = await q.CountAsync(ct);
            if (total > MaximoPorFiltro)
                return Erro($"O filtro tem {total} transações; o limite de uma vez é {MaximoPorFiltro}. Restrinja o período.");
            transacoes = await q.ToListAsync(ct);
            escolhaDe = _ => (dto.CategoriaId, dto.Automatica);
        }

        await _categorizacao.CarregarAsync(ct);
        var anteriores = new List<EstadoAnterior>();
        foreach (var t in transacoes)
        {
            var antes = new EstadoAnterior(t.Id, t.CategoriaId, t.CategoriaManual, t.Tipo, t.IgnorarRelatorios, null);
            var (categoriaId, automatica) = escolhaDe(t);
            if (automatica)
            {
                t.CategoriaManual = false;
                t.CategoriaId = _categorizacao.Resolver(t.DescricaoOriginal ?? t.Descricao, t.PluggyCategoriaId);
            }
            else
            {
                t.CategoriaManual = true;
                t.CategoriaId = categoriaId;
            }
            _categorizacao.AplicarTipo(t);
            if (t.CategoriaId == antes.CategoriaId && t.CategoriaManual == antes.CategoriaManual) continue;
            t.AtualizadoEm = DateTime.UtcNow;
            anteriores.Add(antes with { CategoriaAplicada = t.CategoriaId });
        }
        if (anteriores.Count == 0) return new ResultadoCategoriasEmLoteDto(0, null);

        _auditoria.Registrar($"CategoriaEmLote ({anteriores.Count})", User.Identity?.Name);
        await _db.SaveChangesAsync(ct);
        await AcertarTransferenciasAsync(ct);
        var chave = Guid.NewGuid().ToString("N");
        _cache.Set(ChaveDesfazer(chave), anteriores, PrazoDesfazer);
        return new ResultadoCategoriasEmLoteDto(anteriores.Count, chave);
    }

    /// <summary>
    /// Desfaz uma alteração em lote: volta categoria, "manual", tipo e "ignorar nos relatórios" de antes. Transação
    /// que mudou de novo depois disso fica como está (não desfaz por cima de outra edição).
    /// </summary>
    [HttpPost("em-lote/desfazer/{chave}")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> DesfazerCategoriasEmLote(string chave, CancellationToken ct)
    {
        if (!_cache.TryGetValue(ChaveDesfazer(chave), out List<EstadoAnterior>? anteriores) || anteriores is null)
            return NaoEncontrado("Não dá mais para desfazer: o prazo passou ou já foi desfeito.");
        _cache.Remove(ChaveDesfazer(chave));

        var porId = anteriores.ToDictionary(a => a.Id);
        var ids = porId.Keys.ToList();
        var transacoes = await _db.Transacoes.Where(t => ids.Contains(t.Id)).ToListAsync(ct);
        var categoriasExistentes = (await _db.Categorias.Select(c => c.Id).ToListAsync(ct)).ToHashSet();
        var restauradas = 0;
        foreach (var t in transacoes)
        {
            var a = porId[t.Id];
            if (t.CategoriaId != a.CategoriaAplicada) continue;
            t.CategoriaId = a.CategoriaId is int c && !categoriasExistentes.Contains(c) ? null : a.CategoriaId;
            t.CategoriaManual = a.CategoriaManual;
            t.Tipo = a.Tipo;
            t.IgnorarRelatorios = a.IgnorarRelatorios;
            t.AtualizadoEm = DateTime.UtcNow;
            restauradas++;
        }
        _auditoria.Registrar($"CategoriaEmLoteDesfeita ({restauradas})", User.Identity?.Name);
        await _db.SaveChangesAsync(ct);
        await AcertarTransferenciasAsync(ct);
        return new ResultadoAplicacaoDto(restauradas);
    }

    private static string ChaveDesfazer(string chave) => "transacoes-em-lote:" + chave;

    /* ------------------------------------------------------------ #123: renomear */

    /// <summary>Prévia de "usar também nas outras" (quantas e quais), para o texto e o tipo escolhidos.</summary>
    [HttpGet("{id:long}/renomear/previa")]
    public async Task<ActionResult<PreviaRenomearDto>> PreviaRenomear(long id, [FromQuery] string? texto,
        [FromQuery] TipoCorrespondencia tipo = TipoCorrespondencia.Igual, CancellationToken ct = default)
    {
        var t = await _db.Transacoes.AsNoTracking().Where(x => x.Id == id)
            .Select(x => new { x.DescricaoOriginal }).FirstOrDefaultAsync(ct);
        if (t is null) return NaoEncontrado();
        if (t.DescricaoOriginal is null) return new PreviaRenomearDto(null, "", 0, new());
        var sugerido = RegraDescricaoService.SugerirTexto(t.DescricaoOriginal);
        var alvo = !string.IsNullOrWhiteSpace(texto) ? texto.Trim()
            : tipo == TipoCorrespondencia.Igual ? t.DescricaoOriginal : sugerido;
        var (quantidade, exemplos) = await _descricoes.PreviaAsync(alvo, tipo, id, ct);
        return new PreviaRenomearDto(t.DescricaoOriginal, sugerido, quantidade, exemplos);
    }

    /// <summary>
    /// Novo nome de uma transação (#123, ADR 0030): só nesta (escolhido à mão), de volta ao do banco, ou como regra que
    /// vale nas outras de nome automático e nas próximas. O "Desfazer" guarda o estado de antes de cada transação
    /// que mudou e apaga a regra criada.
    /// </summary>
    [HttpPut("{id:long}/descricao")]
    public async Task<ActionResult<ResultadoRenomearDto>> Renomear(long id, RenomearTransacaoDto dto, CancellationToken ct)
    {
        var t = await _db.Transacoes.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (t is null) return NaoEncontrado();
        var descricao = dto.Descricao?.Trim() ?? "";
        if (!dto.VoltarOriginal && descricao.Length == 0) return Erro("Escreva a nova descrição.");
        if (dto.AplicarNasOutras && t.DescricaoOriginal is null)
            return Erro("Só transações que vieram do banco podem virar regra: esta foi lançada à mão.");
        if (!Enum.IsDefined(dto.TipoCorrespondencia)) return Erro("Tipo de correspondência inválido.");

        await using var transacao = await _db.Database.BeginTransactionAsync(ct);
        var anteriores = new List<RegraDescricaoService.EstadoAnterior>
        {
            new(t.Id, t.Descricao, t.RegraDescricaoId, t.DescricaoManual)
        };
        RegraDescricao? regra = null;
        if (dto.VoltarOriginal)
        {
            t.Descricao = t.DescricaoOriginal ?? t.Descricao;
            t.DescricaoManual = true;
            t.RegraDescricaoId = null;
        }
        else if (dto.AplicarNasOutras)
        {
            var texto = string.IsNullOrWhiteSpace(dto.Texto) ? t.DescricaoOriginal! : dto.Texto.Trim();
            if (CategorizacaoService.Normalizar(texto).Count(char.IsLetterOrDigit) < 2)
                return Erro("O texto da regra precisa de pelo menos 2 letras ou números.");
            regra = new RegraDescricao
            {
                Texto = texto, TipoCorrespondencia = dto.TipoCorrespondencia, NovaDescricao = descricao,
                CriadoEm = DateTime.UtcNow
            };
            _db.RegrasDescricao.Add(regra);
            await _db.SaveChangesAsync(ct);

            // A própria transação passa a ter o nome pela regra, que tem de valer para ela.
            t.DescricaoManual = false;
            t.RegraDescricaoId = null;
            t.Descricao = t.DescricaoOriginal!;
            anteriores.AddRange((await _descricoes.ReaplicarAsync(ct)).Where(a => a.Id != t.Id));
            if (t.RegraDescricaoId != regra.Id)
            {
                var outra = t.RegraDescricaoId is int idOutra
                    ? await _db.RegrasDescricao.AsNoTracking().Where(r => r.Id == idOutra).Select(r => r.Texto).FirstOrDefaultAsync(ct)
                    : null;
                return Erro(outra is null
                    ? $"A descrição do banco desta transação (\"{t.DescricaoOriginal}\") não bate com \"{texto}\"."
                    : $"Já existe uma regra mais específica para esta descrição (\"{outra}\"). Edite-a em Categorias › Regras automáticas.");
            }
        }
        else
        {
            t.Descricao = descricao;
            t.DescricaoManual = true;
            t.RegraDescricaoId = null;
        }

        var mudaram = anteriores.Where(a => a.Id != t.Id).ToList();
        if (anteriores[0].Descricao != t.Descricao || anteriores[0].RegraDescricaoId != t.RegraDescricaoId
            || anteriores[0].DescricaoManual != t.DescricaoManual)
        {
            t.AtualizadoEm = DateTime.UtcNow;
            mudaram.Insert(0, anteriores[0]);
        }
        _auditoria.Registrar(regra is null ? "TransacaoRenomeada" : $"RegraDescricaoCriada ({mudaram.Count})", User.Identity?.Name);
        await _db.SaveChangesAsync(ct);
        await transacao.CommitAsync(ct);

        if (mudaram.Count == 0 && regra is null) return new ResultadoRenomearDto(0, null, null);
        var chave = GuardarDesfazerNome(new DesfazerNome(mudaram, regra is null ? new() : [regra.Id], new()));
        return new ResultadoRenomearDto(mudaram.Count, regra?.Id, chave);
    }

    /// <summary>
    /// O mesmo nome em várias transações de uma vez (#123), só nelas ou como regras "é igual a" por descrição do banco
    /// (que valem nas outras e nas próximas). Com "Desfazer", como o renomear de uma.
    /// </summary>
    [HttpPatch("descricao/em-lote")]
    public async Task<ActionResult<ResultadoRenomearEmLoteDto>> RenomearEmLote(RenomearEmLoteDto dto, CancellationToken ct)
    {
        var ids = dto.Ids.Distinct().ToList();
        var transacoes = await _db.Transacoes.Where(t => ids.Contains(t.Id)).ToListAsync(ct);
        if (transacoes.Count != ids.Count) return NaoEncontrado("Alguma transação não existe mais. Atualize a lista.");
        var nome = dto.Descricao.Trim();
        if (nome.Length == 0) return Erro("Escreva o novo nome.");

        var originais = dto.CriarRegras
            ? transacoes.Where(t => t.DescricaoOriginal is not null)
                .GroupBy(t => CategorizacaoService.Normalizar(t.DescricaoOriginal)).Select(g => g.First().DescricaoOriginal!).ToList()
            : new List<string>();
        if (originais.Count > MaximoRegrasPorLote)
            return Erro($"As selecionadas têm {originais.Count} descrições do banco diferentes; o limite é {MaximoRegrasPorLote} regras de uma vez. "
                        + "Selecione menos, ou crie uma regra \"contém\" em Categorias › Regras automáticas.");

        await using var transacaoBanco = await _db.Database.BeginTransactionAsync(ct);
        var anteriores = transacoes.Select(t => new RegraDescricaoService.EstadoAnterior(t.Id, t.Descricao, t.RegraDescricaoId, t.DescricaoManual))
            .ToDictionary(a => a.Id);
        var criadas = new List<RegraDescricao>();
        var alteradas = new List<RegraAntes>();
        if (originais.Count > 0)
        {
            var existentes = await _db.RegrasDescricao.Where(r => r.TipoCorrespondencia == TipoCorrespondencia.Igual).ToListAsync(ct);
            foreach (var original in originais)
            {
                var normalizado = CategorizacaoService.Normalizar(original);
                var regra = existentes.FirstOrDefault(r => CategorizacaoService.Normalizar(r.Texto) == normalizado);
                if (regra is null)
                {
                    regra = new RegraDescricao
                    {
                        Texto = original, TipoCorrespondencia = TipoCorrespondencia.Igual, NovaDescricao = nome, CriadoEm = DateTime.UtcNow
                    };
                    _db.RegrasDescricao.Add(regra);
                    criadas.Add(regra);
                }
                else if (regra.NovaDescricao != nome || !regra.Ativa)
                {
                    alteradas.Add(new RegraAntes(regra.Id, regra.NovaDescricao, regra.Ativa));
                    regra.NovaDescricao = nome;
                    regra.Ativa = true;
                }
            }
            await _db.SaveChangesAsync(ct);
        }

        foreach (var t in transacoes)
        {
            if (originais.Count > 0 && t.DescricaoOriginal is not null)
            {
                // O nome vem da regra: a transação volta ao nome automático e a reaplicação abaixo dá o nome.
                t.DescricaoManual = false;
                t.RegraDescricaoId = null;
                t.Descricao = t.DescricaoOriginal;
            }
            else
            {
                t.Descricao = nome;
                t.DescricaoManual = true;
                t.RegraDescricaoId = null;
            }
        }
        var outras = originais.Count > 0 ? await _descricoes.ReaplicarAsync(ct) : new();

        var mudaram = transacoes.Where(t => anteriores[t.Id] is var a
                && (a.Descricao != t.Descricao || a.RegraDescricaoId != t.RegraDescricaoId || a.DescricaoManual != t.DescricaoManual))
            .Select(t => anteriores[t.Id]).ToList();
        mudaram.AddRange(outras.Where(o => !anteriores.ContainsKey(o.Id)));
        foreach (var t in transacoes.Where(t => mudaram.Any(m => m.Id == t.Id))) t.AtualizadoEm = DateTime.UtcNow;
        _auditoria.Registrar($"RenomearEmLote ({mudaram.Count}, {criadas.Count} regra)", User.Identity?.Name);
        await _db.SaveChangesAsync(ct);
        await transacaoBanco.CommitAsync(ct);

        if (mudaram.Count == 0 && criadas.Count == 0 && alteradas.Count == 0)
            return new ResultadoRenomearEmLoteDto(0, 0, 0, null);
        var chave = GuardarDesfazerNome(new DesfazerNome(mudaram, criadas.Select(r => r.Id).ToList(), alteradas));
        return new ResultadoRenomearEmLoteDto(mudaram.Count, criadas.Count, alteradas.Count, chave);
    }

    /// <summary>Desfaz um "Renomear": volta o nome de antes de cada transação e apaga a regra criada.</summary>
    [HttpPost("descricao/desfazer/{chave}")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> DesfazerRenomear(string chave, CancellationToken ct)
    {
        if (!_cache.TryGetValue(ChaveDesfazerNome(chave), out DesfazerNome? estado) || estado is null)
            return NaoEncontrado("Não dá mais para desfazer: o prazo passou ou já foi desfeito.");
        _cache.Remove(ChaveDesfazerNome(chave));

        var porId = estado.Anteriores.ToDictionary(a => a.Id);
        var ids = porId.Keys.ToList();
        var transacoes = await _db.Transacoes.Where(t => ids.Contains(t.Id)).ToListAsync(ct);
        foreach (var t in transacoes)
        {
            var a = porId[t.Id];
            t.Descricao = a.Descricao;
            t.RegraDescricaoId = a.RegraDescricaoId;
            t.DescricaoManual = a.DescricaoManual;
            t.AtualizadoEm = DateTime.UtcNow;
        }
        if (estado.RegrasCriadas.Count > 0)
            await _db.RegrasDescricao.Where(r => estado.RegrasCriadas.Contains(r.Id)).ExecuteDeleteAsync(ct);
        foreach (var antes in estado.RegrasAlteradas)
        {
            var regra = await _db.RegrasDescricao.FirstOrDefaultAsync(r => r.Id == antes.Id, ct);
            if (regra is null) continue;
            regra.NovaDescricao = antes.NovaDescricao;
            regra.Ativa = antes.Ativa;
        }
        _auditoria.Registrar($"RenomearDesfeito ({transacoes.Count})", User.Identity?.Name);
        await _db.SaveChangesAsync(ct);
        return new ResultadoAplicacaoDto(transacoes.Count);
    }

    private static string ChaveDesfazerNome(string chave) => "transacoes-renomear:" + chave;

    /// <summary>No máximo esta quantidade de regras criadas de uma vez pelo renomear em lote.</summary>
    private const int MaximoRegrasPorLote = 50;

    /// <summary>Regra de descrição que o lote reaproveitou, com o nome e a situação de antes.</summary>
    private sealed record RegraAntes(int Id, string NovaDescricao, bool Ativa);

    /// <summary>O que o "Desfazer" de um renomear volta: nomes das transações, regras criadas e regras alteradas.</summary>
    private sealed record DesfazerNome(List<RegraDescricaoService.EstadoAnterior> Anteriores, List<int> RegrasCriadas,
        List<RegraAntes> RegrasAlteradas);

    private string GuardarDesfazerNome(DesfazerNome estado)
    {
        var chave = Guid.NewGuid().ToString("N");
        _cache.Set(ChaveDesfazerNome(chave), estado, PrazoDesfazer);
        return chave;
    }

    /// <summary>
    /// Transferência só fica fora dos relatórios quando é entre contas suas (ADR 0031): depois de mudar categoria,
    /// valor ou excluir um lado de um par, a categoria "Transferências" é conferida de novo.
    /// </summary>
    private async Task AcertarTransferenciasAsync(CancellationToken ct)
    {
        if (await _transferencias.ReclassificarAsync(ct) > 0) await _db.SaveChangesAsync(ct);
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
