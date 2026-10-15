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
        var saldos = await _contas.CalcularSaldosAsync(ct);
        var contas = await _db.Contas.AsNoTracking()
            .Where(c => incluirArquivadas || !c.Arquivada)
            .OrderBy(c => c.Arquivada).ThenBy(c => c.Tipo).ThenBy(c => c.Nome)
            .Select(c => new { Conta = c, Instituicao = c.ConexaoBancaria != null ? c.ConexaoBancaria.NomeInstituicao : null })
            .ToListAsync(ct);

        return contas.Select(x => ParaDto(x.Conta, x.Instituicao, saldos.GetValueOrDefault(x.Conta.Id))).ToList();
    }

    [HttpGet("{id:int}")]
    public async Task<ActionResult<ContaDto>> Obter(int id, CancellationToken ct)
    {
        var conta = await _db.Contas.AsNoTracking().Include(c => c.ConexaoBancaria).FirstOrDefaultAsync(c => c.Id == id, ct);
        if (conta is null) return NaoEncontrado();
        var saldos = await _contas.CalcularSaldosAsync(ct);
        return ParaDto(conta, conta.ConexaoBancaria?.NomeInstituicao, saldos.GetValueOrDefault(id));
    }

    [HttpPost]
    public async Task<ActionResult<ContaDto>> Criar(SalvarContaDto dto, CancellationToken ct)
    {
        if (dto.Tipo == TipoConta.Investimento && dto.SaldoInicial < 0) return Erro("Saldo inicial inválido.");
        var conta = new Conta { Origem = OrigemConta.Manual, CriadoEm = DateTime.UtcNow };
        Aplicar(conta, dto, manual: true);
        _db.Contas.Add(conta);
        await _db.SaveChangesAsync(ct);
        return CreatedAtAction(nameof(Obter), new { id = conta.Id }, ParaDto(conta, null, conta.SaldoInicial));
    }

    [HttpPut("{id:int}")]
    public async Task<ActionResult<ContaDto>> Atualizar(int id, SalvarContaDto dto, CancellationToken ct)
    {
        var conta = await _db.Contas.FirstOrDefaultAsync(c => c.Id == id, ct);
        if (conta is null) return NaoEncontrado();
        Aplicar(conta, dto, manual: conta.Origem == OrigemConta.Manual);
        await _db.SaveChangesAsync(ct);
        var saldos = await _contas.CalcularSaldosAsync(ct);
        return ParaDto(conta, null, saldos.GetValueOrDefault(id));
    }

    /// <summary>Exclui uma conta manual e suas transações. Contas do Open Finance são removidas pela conexão.</summary>
    [HttpDelete("{id:int}")]
    public async Task<IActionResult> Excluir(int id, CancellationToken ct)
    {
        var conta = await _db.Contas.FirstOrDefaultAsync(c => c.Id == id, ct);
        if (conta is null) return NaoEncontrado();
        if (conta.Origem == OrigemConta.OpenFinance)
            return Erro("Contas do Open Finance são gerenciadas pela conexão. Use 'Arquivar' ou remova a conexão.");

        await _contas.DesvincularFaturasAsync(new[] { id }, ct);
        _db.Contas.Remove(conta);
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpGet("{id:int}/faturas")]
    public async Task<ActionResult<List<FaturaDto>>> Faturas(int id, CancellationToken ct)
    {
        if (!await _db.Contas.AnyAsync(c => c.Id == id, ct)) return NaoEncontrado();
        return await _db.FaturasCartao.AsNoTracking()
            .Where(f => f.ContaId == id)
            .OrderByDescending(f => f.DataVencimento)
            .Take(24)
            .Select(f => new FaturaDto(f.Id, f.DataVencimento, f.DataFechamento, f.ValorTotal, f.ValorMinimo, f.ValorPago,
                _db.Transacoes.Count(t => t.FaturaCartaoId == f.Id)))
            .ToListAsync(ct);
    }

    private static void Aplicar(Conta conta, SalvarContaDto dto, bool manual)
    {
        // Campos de preferência visual valem para qualquer conta.
        conta.Nome = dto.Nome.Trim();
        conta.Cor = dto.Cor;
        conta.IncluirNoSaldo = dto.IncluirNoSaldo;
        conta.Arquivada = dto.Arquivada;
        conta.AtualizadoEm = DateTime.UtcNow;

        if (!manual) return; // dados financeiros de contas Open Finance vêm da instituição

        conta.Tipo = dto.Tipo;
        conta.Numero = dto.Numero?.Trim();
        conta.SaldoInicial = dto.SaldoInicial;
        var ehCartao = dto.Tipo == TipoConta.CartaoCredito;
        conta.LimiteCredito = ehCartao ? dto.LimiteCredito : null;
        conta.DiaFechamento = ehCartao ? dto.DiaFechamento : null;
        conta.DiaVencimento = ehCartao ? dto.DiaVencimento : null;
    }

    private static ContaDto ParaDto(Conta c, string? instituicao, decimal saldo)
    {
        decimal? limiteDisponivel = c.LimiteDisponivel;
        if (c.Origem == OrigemConta.Manual && c.Tipo == TipoConta.CartaoCredito && c.LimiteCredito is { } limite)
            limiteDisponivel = limite + saldo; // saldo de cartão é negativo (dívida)

        return new ContaDto(c.Id, c.Nome, c.Tipo, c.Origem, c.Numero, c.Moeda, saldo, c.SaldoInicial, c.LimiteCredito,
            limiteDisponivel, c.DiaFechamento, c.DiaVencimento, c.Cor, c.IncluirNoSaldo, c.Arquivada,
            c.ConexaoBancariaId, instituicao, c.AtualizadoEm);
    }
}
