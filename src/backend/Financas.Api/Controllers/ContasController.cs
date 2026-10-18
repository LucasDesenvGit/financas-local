using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Controllers;

[Route("api/contas")]
public class ContasController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly ContaService _contas;

    public ContasController(FinancasDbContext db, ContaService contas)
    {
        _db = db;
        _contas = contas;
    }

    [HttpGet]
    public async Task<ActionResult<List<ContaDto>>> Listar([FromQuery] bool incluirArquivadas, CancellationToken ct)
    {
        var contas = await _db.Contas.AsNoTracking()
            .Where(c => incluirArquivadas || !c.Arquivada)
            .OrderBy(c => c.Arquivada).ThenBy(c => c.Tipo).ThenBy(c => c.Nome)
            .Select(c => new { Conta = c, Instituicao = c.ConexaoBancaria != null ? c.ConexaoBancaria.NomeInstituicao : null })
            .ToListAsync(ct);
        var extras = await ExtrasAsync(ct);
        return contas.Select(x => ParaDto(x.Conta, x.Instituicao, extras)).ToList();
    }

    [HttpGet("{id:int}")]
    public async Task<ActionResult<ContaDto>> Obter(int id, CancellationToken ct)
    {
        var conta = await _db.Contas.AsNoTracking().Include(c => c.ConexaoBancaria).FirstOrDefaultAsync(c => c.Id == id, ct);
        if (conta is null) return NaoEncontrado();
        return ParaDto(conta, conta.ConexaoBancaria?.NomeInstituicao, await ExtrasAsync(ct));
    }

    /// <summary>Bancos que a tela sabe identificar (ADR 0024), para o campo "Banco".</summary>
    [HttpGet("bancos")]
    public ActionResult<List<BancoDto>> Bancos()
        => BancoService.Catalogo.Select(b => new BancoDto(b.Codigo, b.Nome)).ToList();

    [HttpPost]
    public async Task<ActionResult<ContaDto>> Criar(SalvarContaDto dto, CancellationToken ct)
    {
        if (dto.Tipo == TipoConta.Investimento && dto.SaldoInicial < 0) return Erro("Saldo inicial inválido.");
        if (await ValidarBancoAsync(dto, null, ct) is { } erro) return Erro(erro);
        var conta = new Conta { Origem = OrigemConta.Manual, CriadoEm = DateTime.UtcNow };
        Aplicar(conta, dto, manual: true);
        _db.Contas.Add(conta);
        await _db.SaveChangesAsync(ct);
        return CreatedAtAction(nameof(Obter), new { id = conta.Id }, ParaDto(conta, null, await ExtrasAsync(ct)));
    }

    [HttpPut("{id:int}")]
    public async Task<ActionResult<ContaDto>> Atualizar(int id, SalvarContaDto dto, CancellationToken ct)
    {
        var conta = await _db.Contas.Include(c => c.ConexaoBancaria).FirstOrDefaultAsync(c => c.Id == id, ct);
        if (conta is null) return NaoEncontrado();
        if (await ValidarBancoAsync(dto, conta, ct) is { } erro) return Erro(erro);
        Aplicar(conta, dto, manual: conta.Origem == OrigemConta.Manual);
        await _db.SaveChangesAsync(ct);
        return ParaDto(conta, conta.ConexaoBancaria?.NomeInstituicao, await ExtrasAsync(ct));
    }

    /// <summary>
    /// Banco: um código do catálogo ou nenhum (detectar). Conta do cartão: só para cartão, uma conta que existe, não é
    /// cartão e não é a própria.
    /// </summary>
    private async Task<string?> ValidarBancoAsync(SalvarContaDto dto, Conta? atual, CancellationToken ct)
    {
        if (dto.Banco is not null && !BancoService.Existe(dto.Banco)) return "Banco inválido.";
        if (dto.ContaVinculadaId is not int vinculada) return null;
        var tipo = atual?.Origem == OrigemConta.OpenFinance ? atual.Tipo : dto.Tipo;
        if (tipo != TipoConta.CartaoCredito) return "Só cartão de crédito tem conta vinculada.";
        if (vinculada == atual?.Id) return "O cartão não pode ser a própria conta dele.";
        var outra = await _db.Contas.AsNoTracking().Where(c => c.Id == vinculada).Select(c => new { c.Tipo }).FirstOrDefaultAsync(ct);
        if (outra is null) return "Conta do cartão inválida.";
        return outra.Tipo == TipoConta.CartaoCredito ? "A conta do cartão precisa ser uma conta, não outro cartão." : null;
    }

    /// <summary>Saldos, banco que vale e o mês atual dos cartões — o que a lista mostra além da própria conta.</summary>
    private async Task<ExtrasContas> ExtrasAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var faturas = await _contas.FaturasDoMesAsync(Relogio.PrimeiroDiaMes(hoje.Year, hoje.Month),
            Relogio.UltimoDiaMes(hoje.Year, hoje.Month), hoje, ct);
        return new ExtrasContas(await _contas.CalcularSaldosAsync(ct), await _contas.BancosAsync(ct),
            faturas.ToDictionary(f => f.ContaId));
    }

    private sealed record ExtrasContas(Dictionary<int, decimal> Saldos, Dictionary<int, string?> Bancos,
        Dictionary<int, FaturaMesDto> FaturasDoMes);

    /// <summary>Exclui uma conta manual e suas transações. Contas do Open Finance são removidas pela conexão.</summary>
    [HttpDelete("{id:int}")]
    public async Task<IActionResult> Excluir(int id, CancellationToken ct)
    {
        var conta = await _db.Contas.FirstOrDefaultAsync(c => c.Id == id, ct);
        if (conta is null) return NaoEncontrado();
        if (conta.Origem == OrigemConta.OpenFinance)
            return Erro("Contas do Open Finance são gerenciadas pela conexão. Use 'Arquivar' ou remova a conexão.");

        await _contas.DesvincularFaturasAsync(new[] { id }, ct);
        // Cartões que pertenciam a esta conta ficam sem conta vinculada (não há FK no banco; ADR 0024).
        await _db.Contas.Where(c => c.ContaVinculadaId == id)
            .ExecuteUpdateAsync(s => s.SetProperty(c => c.ContaVinculadaId, (int?)null), ct);
        _db.Contas.Remove(conta);
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpGet("{id:int}/faturas")]
    public async Task<ActionResult<List<FaturaDto>>> Faturas(int id, CancellationToken ct)
    {
        if (!await _db.Contas.AnyAsync(c => c.Id == id, ct)) return NaoEncontrado();
        var hoje = Relogio.Hoje;
        var faturas = await _db.FaturasCartao.AsNoTracking()
            .Where(f => f.ContaId == id)
            .OrderByDescending(f => f.DataVencimento)
            .Take(24)
            .Select(f => new FaturaDto(f.Id, f.DataVencimento, f.DataFechamento, f.ValorTotal, f.ValorMinimo, f.ValorPago,
                _db.Transacoes.Count(t => t.FaturaCartaoId == f.Id), "aberta"))
            .ToListAsync(ct);
        return faturas.Select(f => f with
        {
            Situacao = ContaService.SituacaoFatura(Math.Abs(f.ValorTotal), f.ValorPago, f.DataFechamento, f.DataVencimento, hoje)
        }).ToList();
    }

    private static void Aplicar(Conta conta, SalvarContaDto dto, bool manual)
    {
        // Campos de preferência visual valem para qualquer conta.
        conta.Nome = dto.Nome.Trim();
        conta.Cor = dto.Cor;
        conta.IncluirNoSaldo = dto.IncluirNoSaldo;
        conta.Arquivada = dto.Arquivada;
        // ADR 0024: banco e conta do cartão também valem para contas do Open Finance (a instituição nem sempre diz).
        conta.Banco = dto.Banco;
        conta.AtualizadoEm = DateTime.UtcNow;
        var ehCartaoAgora = (manual ? dto.Tipo : conta.Tipo) == TipoConta.CartaoCredito;
        conta.ContaVinculadaId = ehCartaoAgora ? dto.ContaVinculadaId : null;

        if (!manual) return; // dados financeiros de contas Open Finance vêm da instituição

        conta.Tipo = dto.Tipo;
        conta.Numero = dto.Numero?.Trim();
        conta.SaldoInicial = dto.SaldoInicial;
        var ehCartao = dto.Tipo == TipoConta.CartaoCredito;
        conta.LimiteCredito = ehCartao ? dto.LimiteCredito : null;
        conta.DiaFechamento = ehCartao ? dto.DiaFechamento : null;
        conta.DiaVencimento = ehCartao ? dto.DiaVencimento : null;
    }

    private static ContaDto ParaDto(Conta c, string? instituicao, ExtrasContas extras)
    {
        var saldo = extras.Saldos.GetValueOrDefault(c.Id, c.Origem == OrigemConta.Manual ? c.SaldoInicial : c.Saldo);
        // Conta recém-criada ainda não está no cálculo de bancos: detecta na hora.
        var banco = extras.Bancos.TryGetValue(c.Id, out var b) ? b
            : c.Banco == BancoService.Outro ? null : c.Banco ?? BancoService.Detectar(instituicao, c.Nome);
        decimal? limiteDisponivel = c.LimiteDisponivel;
        if (c.Origem == OrigemConta.Manual && c.Tipo == TipoConta.CartaoCredito && c.LimiteCredito is { } limite)
            limiteDisponivel = limite + saldo; // saldo de cartão é negativo (dívida)

        return new ContaDto(c.Id, c.Nome, c.Tipo, c.Origem, c.Numero, c.Moeda, saldo, c.SaldoInicial, c.LimiteCredito,
            limiteDisponivel, c.DiaFechamento, c.DiaVencimento, c.Cor, c.IncluirNoSaldo, c.Arquivada,
            c.ConexaoBancariaId, instituicao, c.AtualizadoEm, banco, c.Banco is not null, c.ContaVinculadaId,
            c.Tipo == TipoConta.CartaoCredito ? extras.FaturasDoMes.GetValueOrDefault(c.Id) : null);
    }
}
