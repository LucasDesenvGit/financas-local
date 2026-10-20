using Financas.Api.Data;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// #158 (ADR 0043): conta a pagar cobrada sozinha (débito automático ou cartão de crédito) com o texto da cobrança se dá
/// como paga quando a transação chega no extrato: o último pagamento vai para a data dela e o próximo vencimento
/// avança, como no "Pagar". Roda no fim da sincronização, ao iniciar a API e ao salvar uma conta a pagar.
/// </summary>
public class ContaAgendadaService
{
    /// <summary>Cobrança que chega até tantos dias antes do vencimento esperado já é a dele (o banco adianta às vezes).</summary>
    public const int DiasAntesDoVencimento = 10;

    private readonly FinancasDbContext _db;

    public ContaAgendadaService(FinancasDbContext db) => _db = db;

    /// <summary>Não salva: chame SaveChangesAsync. Devolve quantas contas a pagar mudaram.</summary>
    public async Task<int> ConciliarAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var itens = await _db.ContasAgendadas
            .Where(c => c.Ativa && c.FormaCobranca != FormaCobranca.AMao && c.TextoCobranca != null)
            .ToListAsync(ct);
        var mudaram = 0;
        foreach (var c in itens)
        {
            var texto = CategorizacaoService.Normalizar(c.TextoCobranca);
            if (texto.Length == 0) continue;
            var desde = c.ProximoVencimento.AddDays(-DiasAntesDoVencimento);
            var contaId = c.ContaId;
            // A descrição original do banco: o nome que você deu à transação (#123) não atrapalha.
            var candidatas = await _db.Transacoes.AsNoTracking()
                .Where(t => t.Valor < 0 && t.Data >= desde && t.Data <= hoje && (contaId == null || t.ContaId == contaId))
                .Select(t => new { t.Data, Texto = t.DescricaoOriginal ?? t.Descricao })
                .ToListAsync(ct);
            var ultima = candidatas
                .Where(x => CategorizacaoService.Normalizar(x.Texto).Contains(texto, StringComparison.Ordinal))
                .Select(x => (DateOnly?)x.Data).Max();
            if (ultima is not DateOnly data) continue;

            var antes = (c.UltimoPagamentoEm, c.ProximoVencimento, c.Ativa);
            c.UltimoPagamentoEm = data;
            while (c.Ativa && c.ProximoVencimento <= data.AddDays(DiasAntesDoVencimento))
            {
                if (RelatorioService.ProximoVencimentoApos(c.ProximoVencimento, c.Recorrencia) is not { } proximo)
                {
                    c.Ativa = false;
                    break;
                }
                c.ProximoVencimento = proximo;
                // #69: passou da data limite, a conta é encerrada.
                if (c.DataLimite is { } limite && proximo > limite) c.Ativa = false;
            }
            if ((c.UltimoPagamentoEm, c.ProximoVencimento, c.Ativa) != antes) mudaram++;
        }
        return mudaram;
    }
}
