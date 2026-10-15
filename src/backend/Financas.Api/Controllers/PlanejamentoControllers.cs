using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Controllers;

[Route("api/orcamentos")]
public class OrcamentosController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly RelatorioService _relatorios;

    public OrcamentosController(FinancasDbContext db, RelatorioService relatorios)
    {
        _db = db;
        _relatorios = relatorios;
    }

    [HttpGet]
    public async Task<ActionResult<List<OrcamentoDto>>> Listar([FromQuery] int? ano, [FromQuery] int? mes, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var a = ano ?? hoje.Year;
        var m = mes ?? hoje.Month;
        if (a < 2000 || a > 2100 || m < 1 || m > 12) return Erro("Período inválido.");
        return (await _relatorios.OrcamentosDoMesAsync(a, m, ct)).OrderBy(o => o.CategoriaNome).ToList();
    }

    /// <summary>Cria ou altera o orçamento de uma categoria no mês.</summary>
    [HttpPut]
    public async Task<IActionResult> Salvar(SalvarOrcamentoDto dto, CancellationToken ct)
    {
        var categoria = await _db.Categorias.AsNoTracking().FirstOrDefaultAsync(c => c.Id == dto.CategoriaId, ct);
        if (categoria is null) return Erro("Categoria inválida.");
        if (categoria.Tipo != TipoCategoria.Despesa) return Erro("Orçamentos são definidos para categorias de despesa.");

        var o = await _db.Orcamentos.FirstOrDefaultAsync(
            x => x.CategoriaId == dto.CategoriaId && x.Ano == dto.Ano && x.Mes == dto.Mes, ct);
        if (o is null)
        {
            o = new Orcamento { CategoriaId = dto.CategoriaId, Ano = dto.Ano, Mes = dto.Mes };
            _db.Orcamentos.Add(o);
        }
        o.Valor = dto.Valor;
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpDelete("{id:int}")]
    public async Task<IActionResult> Excluir(int id, CancellationToken ct)
    {
        var apagados = await _db.Orcamentos.Where(o => o.Id == id).ExecuteDeleteAsync(ct);
        return apagados == 0 ? NaoEncontrado() : NoContent();
    }

    /// <summary>Copia os orçamentos do mês anterior para o mês informado (sem sobrescrever os existentes).</summary>
    [HttpPost("copiar-mes-anterior")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> CopiarMesAnterior(CopiarOrcamentoDto dto, CancellationToken ct)
    {
        var anterior = new DateOnly(dto.Ano, dto.Mes, 1).AddMonths(-1);
        var origem = await _db.Orcamentos.AsNoTracking()
            .Where(o => o.Ano == anterior.Year && o.Mes == anterior.Month).ToListAsync(ct);
        var existentes = await _db.Orcamentos.AsNoTracking()
            .Where(o => o.Ano == dto.Ano && o.Mes == dto.Mes).Select(o => o.CategoriaId).ToListAsync(ct);

        var novos = origem.Where(o => !existentes.Contains(o.CategoriaId))
            .Select(o => new Orcamento { CategoriaId = o.CategoriaId, Ano = dto.Ano, Mes = dto.Mes, Valor = o.Valor })
            .ToList();
        _db.Orcamentos.AddRange(novos);
        await _db.SaveChangesAsync(ct);
        return new ResultadoAplicacaoDto(novos.Count);
    }
}

[Route("api/metas")]
public class MetasController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly ContaService _contas;

    public MetasController(FinancasDbContext db, ContaService contas)
    {
        _db = db;
        _contas = contas;
    }

    [HttpGet]
    public async Task<ActionResult<List<MetaDto>>> Listar(CancellationToken ct)
    {
        var metas = await _db.Metas.AsNoTracking().Include(m => m.Conta)
            .OrderBy(m => m.Concluida).ThenBy(m => m.DataAlvo).ThenBy(m => m.Nome)
            .ToListAsync(ct);
        var saldos = metas.Any(m => m.ContaId != null)
            ? await _contas.CalcularSaldosAsync(ct)
            : new Dictionary<int, decimal>();
        return metas.Select(m => ParaDto(m, saldos)).ToList();
    }

    [HttpPost]
    public async Task<ActionResult<MetaDto>> Criar(SalvarMetaDto dto, CancellationToken ct)
    {
        if (dto.ContaId is { } cid && !await _db.Contas.AnyAsync(c => c.Id == cid, ct)) return Erro("Conta inválida.");
        var m = new Meta { CriadoEm = DateTime.UtcNow };
        Aplicar(m, dto);
        _db.Metas.Add(m);
        await _db.SaveChangesAsync(ct);
        return await ObterDtoAsync(m.Id, ct);
    }

    [HttpPut("{id:int}")]
    public async Task<ActionResult<MetaDto>> Atualizar(int id, SalvarMetaDto dto, CancellationToken ct)
    {
        var m = await _db.Metas.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (m is null) return NaoEncontrado();
        if (dto.ContaId is { } cid && !await _db.Contas.AnyAsync(c => c.Id == cid, ct)) return Erro("Conta inválida.");
        Aplicar(m, dto);
        await _db.SaveChangesAsync(ct);
        return await ObterDtoAsync(id, ct);
    }

    /// <summary>Soma (ou subtrai, se negativo) um valor ao progresso da meta.</summary>
    [HttpPost("{id:int}/aporte")]
    public async Task<ActionResult<MetaDto>> Aporte(int id, AporteDto dto, CancellationToken ct)
    {
        var m = await _db.Metas.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (m is null) return NaoEncontrado();
        m.ValorAtual = Math.Max(0, m.ValorAtual + dto.Valor);
        if (m.ValorAtual >= m.ValorAlvo) m.Concluida = true;
        await _db.SaveChangesAsync(ct);
        return await ObterDtoAsync(id, ct);
    }

    [HttpDelete("{id:int}")]
    public async Task<IActionResult> Excluir(int id, CancellationToken ct)
    {
        var apagadas = await _db.Metas.Where(m => m.Id == id).ExecuteDeleteAsync(ct);
        return apagadas == 0 ? NaoEncontrado() : NoContent();
    }

    private async Task<MetaDto> ObterDtoAsync(int id, CancellationToken ct)
    {
        var m = await _db.Metas.AsNoTracking().Include(x => x.Conta).FirstAsync(x => x.Id == id, ct);
        var saldos = m.ContaId is null ? new Dictionary<int, decimal>() : await _contas.CalcularSaldosAsync(ct);
        return ParaDto(m, saldos);
    }

    private static void Aplicar(Meta m, SalvarMetaDto dto)
    {
        m.Nome = dto.Nome.Trim();
        m.ValorAlvo = dto.ValorAlvo;
        m.ValorAtual = dto.ValorAtual;
        m.DataAlvo = dto.DataAlvo;
        m.ContaId = dto.ContaId;
        m.Cor = dto.Cor;
        m.Concluida = dto.Concluida;
    }

    private static MetaDto ParaDto(Meta m, Dictionary<int, decimal> saldos)
    {
        // O valor inicial (ValorAtual gravado) sempre conta; se houver conta vinculada, soma o saldo dela.
        var saldoConta = m.ContaId is int cid && saldos.TryGetValue(cid, out var saldo) ? Math.Max(0, saldo) : 0m;
        var atual = m.ValorAtual + saldoConta;
        var percentual = m.ValorAlvo > 0 ? Math.Min(100m, Math.Round(atual / m.ValorAlvo * 100m, 1)) : 0m;

        decimal? sugestao = null;
        if (m.DataAlvo is { } alvo && !m.Concluida && atual < m.ValorAlvo)
        {
            var hoje = Relogio.Hoje;
            var meses = (alvo.Year - hoje.Year) * 12 + alvo.Month - hoje.Month;
            sugestao = Math.Round((m.ValorAlvo - atual) / Math.Max(1, meses), 2);
        }

        return new MetaDto(m.Id, m.Nome, m.ValorAlvo, atual, m.DataAlvo, m.ContaId, m.Conta?.Nome, m.Cor,
            m.Concluida || atual >= m.ValorAlvo, percentual, sugestao, m.ValorAtual, saldoConta);
    }
}

[Route("api/contas-agendadas")]
public class ContasAgendadasController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly RelatorioService _relatorios;
    private readonly CategorizacaoService _categorizacao;

    public ContasAgendadasController(FinancasDbContext db, RelatorioService relatorios, CategorizacaoService categorizacao)
    {
        _db = db;
        _relatorios = relatorios;
        _categorizacao = categorizacao;
    }

    [HttpGet]
    public async Task<ActionResult<List<ContaAgendadaDto>>> Listar(CancellationToken ct)
        => await _relatorios.ContasAgendadasAsync(null, ct);

    [HttpPost]
    public async Task<IActionResult> Criar(SalvarContaAgendadaDto dto, CancellationToken ct)
    {
        var problema = await ValidarAsync(dto, ct);
        if (problema is not null) return Erro(problema);
        var c = new ContaAgendada { CriadoEm = DateTime.UtcNow };
        Aplicar(c, dto);
        _db.ContasAgendadas.Add(c);
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpPut("{id:int}")]
    public async Task<IActionResult> Atualizar(int id, SalvarContaAgendadaDto dto, CancellationToken ct)
    {
        var c = await _db.ContasAgendadas.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (c is null) return NaoEncontrado();
        var problema = await ValidarAsync(dto, ct);
        if (problema is not null) return Erro(problema);
        Aplicar(c, dto);
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpDelete("{id:int}")]
    public async Task<IActionResult> Excluir(int id, CancellationToken ct)
    {
        var apagadas = await _db.ContasAgendadas.Where(c => c.Id == id).ExecuteDeleteAsync(ct);
        return apagadas == 0 ? NaoEncontrado() : NoContent();
    }

    /// <summary>
    /// Marca o vencimento atual como pago e avança para o próximo (ou desativa, se for única).
    /// Opcionalmente lança a despesa, quando a conta vinculada é manual.
    /// </summary>
    [HttpPost("{id:int}/pagar")]
    public async Task<IActionResult> Pagar(int id, PagarContaAgendadaDto dto, CancellationToken ct)
    {
        var c = await _db.ContasAgendadas.Include(x => x.Conta).FirstOrDefaultAsync(x => x.Id == id, ct);
        if (c is null) return NaoEncontrado();
        var dataPagamento = dto.Data ?? Relogio.Hoje;

        if (dto.LancarTransacao)
        {
            if (c.Conta is null || c.Conta.Origem != OrigemConta.Manual)
                return Erro("Para lançar a despesa, vincule uma conta manual. Em contas do Open Finance o pagamento chega pela sincronização.");
            await _categorizacao.CarregarAsync(ct);
            var t = new Transacao
            {
                ContaId = c.Conta.Id,
                Data = dataPagamento,
                Descricao = c.Descricao,
                Valor = -c.Valor,
                Status = StatusTransacao.Efetivada,
                CategoriaId = c.CategoriaId,
                CategoriaManual = c.CategoriaId is not null,
                CriadoEm = DateTime.UtcNow,
                AtualizadoEm = DateTime.UtcNow
            };
            _categorizacao.AplicarTipo(t);
            _db.Transacoes.Add(t);
        }

        c.UltimoPagamentoEm = dataPagamento;
        switch (c.Recorrencia)
        {
            case Recorrencia.Mensal: c.ProximoVencimento = c.ProximoVencimento.AddMonths(1); break;
            case Recorrencia.Anual: c.ProximoVencimento = c.ProximoVencimento.AddYears(1); break;
            default: c.Ativa = false; break;
        }
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    private async Task<string?> ValidarAsync(SalvarContaAgendadaDto dto, CancellationToken ct)
    {
        if (dto.CategoriaId is { } cat && !await _db.Categorias.AnyAsync(c => c.Id == cat, ct)) return "Categoria inválida.";
        if (dto.ContaId is { } conta && !await _db.Contas.AnyAsync(c => c.Id == conta, ct)) return "Conta inválida.";
        return null;
    }

    private static void Aplicar(ContaAgendada c, SalvarContaAgendadaDto dto)
    {
        c.Descricao = dto.Descricao.Trim();
        c.Valor = dto.Valor;
        c.ProximoVencimento = dto.ProximoVencimento;
        c.Recorrencia = dto.Recorrencia;
        c.CategoriaId = dto.CategoriaId;
        c.ContaId = dto.ContaId;
        c.Ativa = dto.Ativa;
    }
}
