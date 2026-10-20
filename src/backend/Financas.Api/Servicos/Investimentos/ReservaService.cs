using System.Text.Json;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Investimentos;

/// <summary>
/// Reservas ("cofrinhos", "caixinhas") das contas. Quando a instituição informa o valor reservado pelo Open Finance
/// (<c>bankData.reservedBalances</c>, #168), ele vale. Senão, o saldo é estimado pelos lançamentos de entrada e saída de
/// cada reserva no extrato da conta (caso do Mercado Pago) e, quando o usuário confere o valor real no app da
/// instituição, parte dessa conferência (ADR 0010).
/// </summary>
public class ReservaService
{
    /// <summary>Prefixos (do extrato) que identificam o dinheiro indo para uma reserva ou voltando dela.</summary>
    private static readonly string[] Prefixos = { "Dinheiro reservado ", "Dinheiro retirado " };

    public const int TamanhoMaximoNome = 100;

    /// <summary>Dinheiro indo para uma reserva ou voltando dela ("Dinheiro reservado …", "Dinheiro retirado …").</summary>
    public static bool EhMovimentoDeReserva(string? descricao)
        => descricao is not null && Prefixos.Any(p => descricao.StartsWith(p, StringComparison.OrdinalIgnoreCase));

    /// <summary>Conferência mais velha que isso é sinalizada para ser refeita.</summary>
    public const int DiasConferenciaAntiga = 30;

    /// <summary>
    /// Reserva cujo primeiro lançamento está até este número de dias depois do primeiro lançamento importado
    /// da conta provavelmente já tinha dinheiro antes da janela importada.
    /// </summary>
    private const int ToleranciaInicioJanela = 7;

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

        // Primeiro lançamento importado de cada conta = início da janela que a sincronização trouxe.
        var idsContas = movimentos.Select(m => m.ContaId).Distinct().ToList();
        var inicioJanela = await _db.Transacoes.AsNoTracking()
            .Where(t => idsContas.Contains(t.ContaId))
            .GroupBy(t => t.ContaId)
            .Select(g => new { ContaId = g.Key, Inicio = g.Min(t => t.Data) })
            .ToDictionaryAsync(x => x.ContaId, x => x.Inicio, ct);
        var hoje = Relogio.Hoje;

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

            DateOnly? primeiro = lista.Count > 0 ? lista.Min(m => m.Data) : null;
            var podeTerSaldoAnterior = conferencia is null && primeiro is DateOnly p
                && inicioJanela.TryGetValue(chave.ContaId, out var inicio)
                && p.DayNumber - inicio.DayNumber <= ToleranciaInicioJanela;
            var conferenciaAntiga = conferencia is not null
                && hoje.DayNumber - conferencia.DataConferencia.DayNumber > DiasConferenciaAntiga;

            reservas.Add(new ReservaDto(chave.ContaId,
                conferencia?.NomeConta ?? lista[0].NomeConta,
                conferencia?.Nome ?? lista.OrderByDescending(m => m.Data).First().Nome!,
                Math.Round(Math.Max(0, saldo), 2), Math.Round(guardado, 2), Math.Round(retirado, 2),
                lista.Count, primeiro,
                lista.Count > 0 ? lista.Max(m => m.Data) : null,
                conferencia?.SaldoConferido, conferencia?.DataConferencia,
                podeTerSaldoAnterior, conferenciaAntiga));
        }

        return SubstituirPeloBanco(reservas, await InformadasPeloBancoAsync(ct))
            .OrderByDescending(r => r.Saldo).ThenBy(r => r.Nome).ToList();
    }

    /// <summary>Reserva que a instituição informa pelo Open Finance (#168), na forma gravada em <c>Conta.ReservasInformadas</c>.</summary>
    public sealed record ReservaInformada(string Nome, decimal Valor);

    private static readonly JsonSerializerOptions Json = new(JsonSerializerDefaults.Web);

    /// <summary>
    /// #168: lê o <c>bankData</c> da Pluggy. Devolve o total reservado (null = a instituição não informa; aí vale a
    /// estimativa pelo extrato), as reservas com nome quando vierem detalhadas e o nome dos campos de cada item (para o
    /// log: a documentação da Pluggy não descreve esses campos). Sem os detalhes, vale o "investido automaticamente".
    /// </summary>
    public static (decimal? Total, string? ReservasJson, string[] Campos) DoOpenFinance(Pluggy.PluggyDadosBancarios? dados)
    {
        if (dados is null) return (null, null, []);
        var reservas = new List<ReservaInformada>();
        var campos = new SortedSet<string>(StringComparer.Ordinal);
        foreach (var item in dados.ReservedBalances ?? [])
        {
            if (item.ValueKind != JsonValueKind.Object) continue;
            decimal? valor = null;
            string? nome = null;
            foreach (var p in item.EnumerateObject())
            {
                campos.Add(p.Name);
                var chave = p.Name.ToLowerInvariant();
                if (valor is null && chave is "amount" or "balance" or "value" or "total" or "reservedamount" or "reservedbalance")
                    valor = Numero(p.Value);
                if (nome is null && p.Value.ValueKind == JsonValueKind.String
                    && chave is "name" or "description" or "label" or "goal" or "type" or "category")
                    nome = p.Value.GetString();
            }
            if (valor is decimal v)
                reservas.Add(new ReservaInformada(string.IsNullOrWhiteSpace(nome) ? "Reservado" : nome.Trim()[..Math.Min(nome.Trim().Length, TamanhoMaximoNome)],
                    Math.Round(v, 2)));
        }
        if (reservas.Count > 0) return (reservas.Sum(r => r.Valor), JsonSerializer.Serialize(reservas, Json), campos.ToArray());
        if (dados.AutomaticallyInvestedBalance is double automatico && automatico > 0)
            return (Math.Round((decimal)automatico, 2), null, campos.ToArray());
        return (null, null, campos.ToArray());

        static decimal? Numero(JsonElement e) => e.ValueKind switch
        {
            JsonValueKind.Number => e.GetDecimal(),
            // { "amount": { "value": 10.0, "currency": "BRL" } }
            JsonValueKind.Object => e.EnumerateObject().Where(q => q.Value.ValueKind == JsonValueKind.Number
                    && q.Name.ToLowerInvariant() is "value" or "amount").Select(q => (decimal?)q.Value.GetDecimal()).FirstOrDefault(),
            _ => null
        };
    }

    /// <summary>Contas com o reservado informado pela instituição: total e, quando houver, as reservas com nome.</summary>
    private async Task<Dictionary<int, (string NomeConta, decimal Total, List<ReservaInformada> Reservas)>> InformadasPeloBancoAsync(
        CancellationToken ct)
    {
        var contas = await _db.Contas.AsNoTracking()
            .Where(c => c.SaldoReservado != null && c.Origem == OrigemConta.OpenFinance && !c.Arquivada && c.IncluirNoSaldo
                        && (c.ConexaoBancaria == null || c.ConexaoBancaria.Ativa))
            .Select(c => new { c.Id, c.Nome, c.SaldoReservado, c.ReservasInformadas })
            .ToListAsync(ct);
        return contas.ToDictionary(c => c.Id, c => (c.Nome, c.SaldoReservado!.Value,
            c.ReservasInformadas is { Length: > 0 } j ? JsonSerializer.Deserialize<List<ReservaInformada>>(j, Json) ?? [] : []));
    }

    /// <summary>
    /// #168 (ADR 0010): nas contas em que a instituição informa o reservado, ele vale no lugar da estimativa pelo extrato.
    /// Com as reservas detalhadas, uma por nome (o nome do extrato é mantido quando bate, para as metas ligadas a ele).
    /// Só com o total, ele vai para a reserva com movimento mais recente e as outras ficam com zero; sem reserva no
    /// extrato, aparece como "Reservado".
    /// </summary>
    private static List<ReservaDto> SubstituirPeloBanco(List<ReservaDto> estimadas,
        Dictionary<int, (string NomeConta, decimal Total, List<ReservaInformada> Reservas)> doBanco)
    {
        if (doBanco.Count == 0) return estimadas;
        var resultado = estimadas.Where(r => !doBanco.ContainsKey(r.ContaId)).ToList();
        foreach (var (contaId, (nomeConta, total, informadas)) in doBanco)
        {
            var daConta = estimadas.Where(r => r.ContaId == contaId).ToList();
            ReservaDto DoBanco(string nome, decimal saldo, ReservaDto? estimada) => new(contaId, nomeConta, nome, Math.Round(saldo, 2),
                estimada?.Guardado ?? 0m, estimada?.Retirado ?? 0m, estimada?.Movimentos ?? 0, estimada?.PrimeiroMovimento,
                estimada?.UltimoMovimento, null, null, false, false, InformadoPeloBanco: true);
            if (informadas.Count > 0)
            {
                foreach (var i in informadas)
                {
                    var estimada = daConta.FirstOrDefault(r => string.Equals(r.Nome, i.Nome, StringComparison.OrdinalIgnoreCase));
                    resultado.Add(DoBanco(estimada?.Nome ?? i.Nome, i.Valor, estimada));
                }
                continue;
            }
            var principal = daConta.OrderByDescending(r => r.UltimoMovimento ?? r.DataConferencia ?? DateOnly.MinValue).FirstOrDefault();
            if (principal is null)
            {
                resultado.Add(DoBanco("Reservado", total, null));
                continue;
            }
            foreach (var r in daConta) resultado.Add(DoBanco(r.Nome, r == principal ? total : 0m, r));
        }
        return resultado;
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
