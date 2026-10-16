using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// Sugere contas a pagar a partir do extrato: pagamentos que se repetem todo mês (boleto, débito automático, Pix
/// para a mesma pessoa...) e que ainda não estão cadastrados em Contas a pagar. Só sugere — quem cadastra é o usuário.
/// O Open Finance não informa boletos futuros (DDA), então o próximo vencimento é estimado pelo dia habitual.
/// </summary>
public class SugestaoContaAgendadaService
{
    /// <summary>Janela do histórico analisado (~7 meses).</summary>
    private const int DiasHistorico = 210;
    /// <summary>Meses distintos com pagamento para considerar recorrente.</summary>
    private const int MinimoMeses = 3;
    /// <summary>Último pagamento há mais que isso: a conta provavelmente acabou.</summary>
    private const int DiasSemPagamento = 45;
    /// <summary>Mais de ~1,5 pagamento por mês é gasto frequente (recarga, mercado), não conta mensal.</summary>
    private const decimal MaximoPorMes = 1.5m;
    /// <summary>Fração mínima dos meses entre o primeiro e o último pagamento que tiveram pagamento.</summary>
    private const decimal CoberturaMinima = 0.6m;
    /// <summary>Variação de valor acima disso é sinalizada como "valor variável" (luz, gás...).</summary>
    private const decimal VariacaoValorFixo = 0.15m;
    private const int MaximoSugestoes = 20;

    private readonly FinancasDbContext _db;

    public SugestaoContaAgendadaService(FinancasDbContext db) => _db = db;

    public async Task<List<SugestaoContaAgendadaDto>> SugerirAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var desde = hoje.AddDays(-DiasHistorico);

        // Fatura de cartão já entra no sistema por outro caminho; compra parcelada não é conta mensal.
        var transacoes = await _db.Transacoes.AsNoTracking()
            .Where(t => t.Data >= desde && t.Data <= hoje && t.Valor < 0 && t.Tipo == TipoTransacao.Despesa
                && t.TransferenciaGrupo == null && !t.IgnorarRelatorios && t.ParcelaTotal == null
                && t.Conta!.Tipo != TipoConta.CartaoCredito && t.Conta.Tipo != TipoConta.Investimento && !t.Conta.Arquivada)
            .OrderByDescending(t => t.Data)
            .Select(t => new
            {
                t.Data, t.Descricao, t.DescricaoOriginal, t.Estabelecimento, t.Valor, t.CategoriaId,
                CategoriaNome = t.Categoria != null ? t.Categoria.Nome : null, t.ContaId, ContaNome = t.Conta!.Nome
            })
            .Take(10000)
            .ToListAsync(ct);

        var descricoesCadastradas = await _db.ContasAgendadas.AsNoTracking().Select(c => c.Descricao).ToListAsync(ct);
        var jaCadastradas = descricoesCadastradas.Select(ChaveDe)
            .Where(k => k.Length > 0).ToHashSet(StringComparer.Ordinal);

        var resultado = new List<SugestaoContaAgendadaDto>();
        var grupos = transacoes
            .Select(t => new { T = t, Chave = ChaveDe(t.Estabelecimento ?? t.DescricaoOriginal ?? t.Descricao) })
            .Where(x => x.Chave.Length > 0)
            .GroupBy(x => x.Chave, x => x.T);

        foreach (var g in grupos)
        {
            var itens = g.ToList(); // mais recente primeiro
            var recente = itens[0];
            if (jaCadastradas.Contains(g.Key) || jaCadastradas.Contains(ChaveDe(recente.Descricao))) continue;
            if (recente.Data < hoje.AddDays(-DiasSemPagamento)) continue;

            var meses = itens.Select(t => t.Data.Year * 12 + t.Data.Month - 1).Distinct().ToList();
            if (meses.Count < MinimoMeses || itens.Count > meses.Count * MaximoPorMes) continue;
            var mesesNoIntervalo = meses.Max() - meses.Min() + 1;
            if ((decimal)meses.Count / mesesNoIntervalo < CoberturaMinima) continue;

            var valores = itens.Select(t => -t.Valor).ToList();
            var valor = Mediana(valores.Take(3).ToList());
            var minimo = valores.Min();
            var maximo = valores.Max();
            var variavel = valor > 0 && (maximo - minimo) / valor > VariacaoValorFixo;

            var dia = (int)Mediana(itens.Select(t => (decimal)t.Data.Day).ToList());
            var proximoMes = new DateOnly(recente.Data.Year, recente.Data.Month, 1).AddMonths(1);
            var proximo = proximoMes.AddDays(Math.Min(dia, DateTime.DaysInMonth(proximoMes.Year, proximoMes.Month)) - 1);

            var categoria = itens.Where(t => t.CategoriaId != null).GroupBy(t => t.CategoriaId)
                .OrderByDescending(c => c.Count()).Select(c => c.First()).FirstOrDefault();
            var conta = itens.GroupBy(t => t.ContaId).OrderByDescending(c => c.Count()).First().First();
            var descricao = recente.Descricao.Trim();
            var termoBusca = TermoBusca(g.Key, recente.Estabelecimento, descricao);

            resultado.Add(new SugestaoContaAgendadaDto(g.Key,
                descricao.Length > 150 ? descricao[..150] : descricao,
                Math.Round(valor, 2), variavel, Math.Round(minimo, 2), Math.Round(maximo, 2),
                proximo, dia, meses.Count, itens.Count, recente.Data, itens[^1].Data,
                categoria?.CategoriaId, categoria?.CategoriaNome, conta.ContaId, conta.ContaNome,
                termoBusca));
        }

        return resultado.OrderBy(s => s.ProximoVencimento).ThenByDescending(s => s.Valor).Take(MaximoSugestoes).ToList();
    }

    /// <summary>
    /// Mesma chave da sugestão de categorias; se a descrição não tiver palavra significativa (ex.: "BOLETO F I A P"),
    /// usa o texto todo sem números, para não perder o pagamento.
    /// </summary>
    private static string ChaveDe(string? descricao)
    {
        var chave = SugestaoCategoriaService.Chave(descricao);
        if (chave.Length > 0) return chave;
        var texto = CategorizacaoService.Normalizar(descricao);
        return string.Join(' ', texto.Split(' ', StringSplitOptions.RemoveEmptyEntries).Where(p => p.All(char.IsLetter)));
    }

    /// <summary>
    /// Palavra para o link "ver pagamentos" em Transações. A busca de lá diferencia maiúsculas e acentos, então a
    /// palavra é tirada do texto como está gravado (a mais longa da chave), não da chave normalizada.
    /// </summary>
    private static string TermoBusca(string chave, string? estabelecimento, string descricao)
    {
        var alvo = SugestaoCategoriaService.Palavras(chave).OrderByDescending(p => p.Length).FirstOrDefault();
        if (alvo is not null)
        {
            foreach (var texto in new[] { estabelecimento, descricao })
            {
                var palavra = texto?.Split(' ', StringSplitOptions.RemoveEmptyEntries)
                    .FirstOrDefault(p => CategorizacaoService.Normalizar(p).Contains(alvo, StringComparison.Ordinal));
                if (palavra is not null) return palavra.Trim(',', '.', '-', '*', '/');
            }
        }
        return descricao[..Math.Min(descricao.Length, 100)];
    }

    private static decimal Mediana(List<decimal> valores)
    {
        var ordenados = valores.Order().ToList();
        var meio = ordenados.Count / 2;
        return ordenados.Count % 2 == 1 ? ordenados[meio] : (ordenados[meio - 1] + ordenados[meio]) / 2;
    }
}
