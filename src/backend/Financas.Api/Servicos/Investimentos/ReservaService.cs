using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Investimentos;

/// <summary>
/// Reservas ("cofrinhos") que a instituição não informa pelo Open Finance — caso do Mercado Pago, que só
/// compartilha os lançamentos de entrada e saída de cada reserva no extrato da conta. O saldo é estimado
/// por esses lançamentos e, quando o usuário confere o valor real no app da instituição, parte dessa conferência.
/// </summary>
public class ReservaService
{
    /// <summary>Prefixos (do extrato) que identificam o dinheiro indo para uma reserva ou voltando dela.</summary>
    private static readonly string[] Prefixos = { "Dinheiro reservado ", "Dinheiro retirado " };

    public const int TamanhoMaximoNome = 100;

    private readonly FinancasDbContext _db;

    public ReservaService(FinancasDbContext db) => _db = db;

    /// <summary>Nome da reserva a partir da descrição do lançamento, ou null se não for movimento de reserva.</summary>
    public static string? ExtrairNome(string? descricao)
    {
        if (string.IsNullOrWhiteSpace(descricao)) return null;
        foreach (var prefixo in Prefixos)
        {
            if (!descricao.StartsWith(prefixo, StringComparison.OrdinalIgnoreCase)) continue;
            var nome = descricao[prefixo.Length..].Trim();
            if (nome.Length == 0) return null;
            return nome.Length > TamanhoMaximoNome ? nome[..TamanhoMaximoNome] : nome;
        }
        return null;
    }

    /// <summary>Reservas das contas que entram no patrimônio, com o saldo estimado (nunca negativo).</summary>
    public async Task<List<ReservaDto>> ListarAsync(CancellationToken ct)
    {
        // Filtro grosso no banco (LIKE); a regra exata fica em ExtrairNome. Usa a descrição original do
        // extrato, que não muda quando o usuário renomeia a transação.
        var movimentos = await _db.Transacoes.AsNoTracking()
            .Where(t => t.Status == StatusTransacao.Efetivada
                        && t.Conta!.Origem == OrigemConta.OpenFinance
                        && !t.Conta.Arquivada && t.Conta.IncluirNoSaldo
                        && (t.Conta.ConexaoBancaria == null || t.Conta.ConexaoBancaria.Ativa)
                        && (EF.Functions.Like(t.DescricaoOriginal ?? t.Descricao, "Dinheiro reservado %")
                            || EF.Functions.Like(t.DescricaoOriginal ?? t.Descricao, "Dinheiro retirado %")))
            .Select(t => new { t.ContaId, NomeConta = t.Conta!.Nome, Descricao = t.DescricaoOriginal ?? t.Descricao, t.Data, t.Valor })
            .ToListAsync(ct);

        var conferencias = await _db.ReservasConferidas.AsNoTracking()
            .Where(r => !r.Conta!.Arquivada && r.Conta.IncluirNoSaldo)
            .Select(r => new { r.ContaId, NomeConta = r.Conta!.Nome, r.Nome, r.SaldoConferido, r.DataConferencia })
            .ToListAsync(ct);

        var grupos = movimentos
            .Select(m => new { m.ContaId, m.NomeConta, Nome = ExtrairNome(m.Descricao), m.Data, m.Valor })
            .Where(m => m.Nome is not null)
            .GroupBy(m => (m.ContaId, Chave: m.Nome!.ToUpperInvariant()))
            .ToDictionary(g => g.Key, g => g.ToList());

        var chaves = grupos.Keys
            .Concat(conferencias.Select(c => (c.ContaId, Chave: c.Nome.ToUpperInvariant())))
            .Distinct();

        var reservas = new List<ReservaDto>();
        foreach (var chave in chaves)
        {
            var lista = grupos.GetValueOrDefault(chave) ?? new();
            var conferencia = conferencias.FirstOrDefault(c => c.ContaId == chave.ContaId
                && string.Equals(c.Nome, chave.Chave, StringComparison.OrdinalIgnoreCase));

            // Valor negativo na conta = dinheiro indo para a reserva; positivo = voltando.
            var considerados = conferencia is null ? lista : lista.Where(m => m.Data > conferencia.DataConferencia).ToList();
            var guardado = considerados.Where(m => m.Valor < 0).Sum(m => -m.Valor);
            var retirado = considerados.Where(m => m.Valor > 0).Sum(m => m.Valor);
            var saldo = (conferencia?.SaldoConferido ?? 0m) + guardado - retirado;

            reservas.Add(new ReservaDto(chave.ContaId,
                conferencia?.NomeConta ?? lista[0].NomeConta,
                conferencia?.Nome ?? lista.OrderByDescending(m => m.Data).First().Nome!,
                Math.Round(Math.Max(0, saldo), 2), Math.Round(guardado, 2), Math.Round(retirado, 2),
                lista.Count,
                lista.Count > 0 ? lista.Min(m => m.Data) : null,
                lista.Count > 0 ? lista.Max(m => m.Data) : null,
                conferencia?.SaldoConferido, conferencia?.DataConferencia));
        }

        return reservas.OrderByDescending(r => r.Saldo).ThenBy(r => r.Nome).ToList();
    }

    /// <summary>Soma dos saldos estimados das reservas (em reais).</summary>
    public async Task<decimal> TotalAsync(CancellationToken ct)
        => (await ListarAsync(ct)).Sum(r => r.Saldo);

    /// <summary>Grava (ou substitui) o saldo conferido de uma reserva. Retorna false se a conta não existir.</summary>
    public async Task<bool> ConferirAsync(int contaId, string nome, decimal saldo, DateOnly data, CancellationToken ct)
    {
        if (!await _db.Contas.AnyAsync(c => c.Id == contaId, ct)) return false;
        nome = nome.Trim();
        var conferencia = (await _db.ReservasConferidas.Where(r => r.ContaId == contaId).ToListAsync(ct))
            .FirstOrDefault(r => string.Equals(r.Nome, nome, StringComparison.OrdinalIgnoreCase));
        if (conferencia is null)
        {
            conferencia = new ReservaConferencia { ContaId = contaId, Nome = nome };
            _db.ReservasConferidas.Add(conferencia);
        }
        conferencia.SaldoConferido = Math.Round(saldo, 2);
        conferencia.DataConferencia = data;
        conferencia.AtualizadoEm = DateTime.UtcNow;
        await _db.SaveChangesAsync(ct);
        return true;
    }

    /// <summary>Apaga a conferência: o saldo volta a ser só a soma dos movimentos do extrato.</summary>
    public async Task<bool> RemoverConferenciaAsync(int contaId, string nome, CancellationToken ct)
    {
        nome = nome.Trim();
        var ids = (await _db.ReservasConferidas.AsNoTracking().Where(r => r.ContaId == contaId)
                .Select(r => new { r.Id, r.Nome }).ToListAsync(ct))
            .Where(r => string.Equals(r.Nome, nome, StringComparison.OrdinalIgnoreCase))
            .Select(r => r.Id).ToList();
        return ids.Count > 0
            && await _db.ReservasConferidas.Where(r => ids.Contains(r.Id)).ExecuteDeleteAsync(ct) > 0;
    }
}
