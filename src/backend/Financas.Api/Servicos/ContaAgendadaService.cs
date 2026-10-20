using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// #158 (ADR 0043): conta a pagar cobrada sozinha (débito automático ou cartão de crédito) com o texto da cobrança se dá
/// como paga quando a transação chega no extrato: o último pagamento vai para a data dela e o próximo vencimento
/// avança, como no "Pagar". Roda no fim da sincronização, ao iniciar a API e ao salvar uma conta a pagar.
/// #205: a cobrança casada também grava a chave do extrato na conta a pagar (a sugestão dela não volta).
/// #241: a transação com o nome de uma regra de descrição com faixa de valor é reconhecida por esse nome (<see cref="TextoDaCobranca"/>).
/// </summary>
public class ContaAgendadaService
{
    /// <summary>Cobrança que chega até tantos dias antes do vencimento esperado já é a dele (o banco adianta às vezes).</summary>
    public const int DiasAntesDoVencimento = 10;

    private readonly FinancasDbContext _db;

    public ContaAgendadaService(FinancasDbContext db) => _db = db;

    /// <summary>
    /// O texto da transação que o texto da cobrança procura: a descrição do banco (o nome que você deu, #123, não
    /// atrapalha), menos quando o nome veio de uma regra com faixa de valor (#241). Aí as duas cobranças têm a mesma
    /// descrição do banco, e é o nome da regra que diz qual é qual.
    /// </summary>
    public static string TextoDaCobranca(string descricao, string? descricaoOriginal, int? regraDescricaoId,
        HashSet<int> regrasPorValor)
        => regraDescricaoId is int id && regrasPorValor.Contains(id) ? descricao : descricaoOriginal ?? descricao;

    /// <summary>Não salva: chame SaveChangesAsync. Devolve quantas contas a pagar mudaram.</summary>
    public async Task<int> ConciliarAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var itens = await _db.ContasAgendadas
            .Where(c => c.Ativa && c.FormaCobranca != FormaCobranca.AMao && c.TextoCobranca != null)
            .ToListAsync(ct);
        var mudaram = 0;
        var regrasPorValor = itens.Count > 0 ? await RegraDescricaoService.RegrasPorValorAsync(_db, ct) : [];
        foreach (var c in itens)
        {
            var texto = CategorizacaoService.Normalizar(c.TextoCobranca);
            if (texto.Length == 0) continue;
            var desde = c.ProximoVencimento.AddDays(-DiasAntesDoVencimento);
            var contaId = c.ContaId;
            var candidatas = (await _db.Transacoes.AsNoTracking()
                .Where(t => t.Valor < 0 && t.Data >= desde && t.Data <= hoje && (contaId == null || t.ContaId == contaId))
                .Select(t => new { t.Data, t.Descricao, t.DescricaoOriginal, t.RegraDescricaoId, t.Estabelecimento })
                .ToListAsync(ct))
                .Select(t => new
                {
                    t.Data, t.Estabelecimento,
                    Texto = TextoDaCobranca(t.Descricao, t.DescricaoOriginal, t.RegraDescricaoId, regrasPorValor),
                    PorValor = t.RegraDescricaoId is int r && regrasPorValor.Contains(r)
                });
            var casada = candidatas
                .Where(x => CategorizacaoService.Normalizar(x.Texto).Contains(texto, StringComparison.Ordinal))
                .OrderByDescending(x => x.Data).FirstOrDefault();
            if (casada is null) continue;
            var data = casada.Data;

            var antes = (c.UltimoPagamentoEm, c.ProximoVencimento, c.Ativa, c.ChaveExtrato);
            // #205: a cobrança casada liga a conta a pagar à sugestão do extrato (a mesma chave do agrupamento de lá).
            var chave = casada.PorValor
                ? SugestaoContaAgendadaService.ChaveDaRegra(casada.Texto)
                : SugestaoContaAgendadaService.ChaveDe(casada.Estabelecimento ?? casada.Texto);
            if (c.ChaveExtrato is null && chave is { Length: > 0 and <= 150 })
                c.ChaveExtrato = chave;
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
            if ((c.UltimoPagamentoEm, c.ProximoVencimento, c.Ativa, c.ChaveExtrato) != antes) mudaram++;
        }
        return mudaram;
    }

    /// <summary>"Não veio": a cobrança automática passou tantos dias do vencimento sem dar baixa.</summary>
    public const int DiasNaoVeio = 5;
    /// <summary>"Aumentou": a última cobrança passou disto em relação ao valor da conta a pagar.</summary>
    public const decimal VariacaoAumento = 0.05m;
    /// <summary>Valor que varia mais que isso entre as últimas cobranças (luz, água) não gera o aviso de aumento.</summary>
    public const decimal VariacaoValorVariavel = 0.15m;
    /// <summary>Até quantos dias para trás as cobranças são olhadas para os avisos.</summary>
    public const int DiasAvisos = 70;

    /// <summary>
    /// #158 (ADR 0043): avisos das contas a pagar recorrentes que estão ativas.
    /// - "nao-veio": cobrada sozinha, com o texto, e o vencimento passou <see cref="DiasNaoVeio"/> dias sem a cobrança;
    /// - "mudou-de-cartao": a última cobrança com o texto veio em outra conta ou cartão;
    /// - "aumentou": a última cobrança passou <see cref="VariacaoAumento"/> do valor (menos quando o valor varia sempre);
    /// - "cartao-arquivado": a conta ou o cartão em que ela cai está arquivado.
    /// Só lê: as ações ("Atualizar") são o PUT da conta a pagar.
    /// </summary>
    public async Task<List<AvisoContaAgendadaDto>> AvisosAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var itens = await _db.ContasAgendadas.AsNoTracking()
            .Where(c => c.Ativa && c.Recorrencia != Recorrencia.Unica)
            .Select(c => new
            {
                c.Id, c.Descricao, c.Valor, c.ProximoVencimento, c.FormaCobranca, c.TextoCobranca, c.ContaId,
                ContaNome = c.Conta != null ? c.Conta.Nome : null, ContaArquivada = c.Conta != null && c.Conta.Arquivada
            })
            .ToListAsync(ct);
        var desde = hoje.AddDays(-DiasAvisos);
        var comTexto = itens.Any(c => c.TextoCobranca != null);
        var cobrancas = comTexto
            ? await _db.Transacoes.AsNoTracking()
                .Where(t => t.Valor < 0 && t.Data >= desde && t.Data <= hoje)
                .Select(t => new { t.Data, t.Valor, t.ContaId, ContaNome = t.Conta!.Nome, t.Descricao, t.DescricaoOriginal, t.RegraDescricaoId })
                .ToListAsync(ct)
            : [];
        var regrasPorValor = comTexto ? await RegraDescricaoService.RegrasPorValorAsync(_db, ct) : [];
        var normalizadas = cobrancas.Select(t => (t.Data, Valor: -t.Valor, t.ContaId, t.ContaNome,
            Texto: CategorizacaoService.Normalizar(TextoDaCobranca(t.Descricao, t.DescricaoOriginal, t.RegraDescricaoId, regrasPorValor))))
            .ToList();

        var avisos = new List<AvisoContaAgendadaDto>();
        foreach (var c in itens)
        {
            if (c.ContaArquivada)
                avisos.Add(new AvisoContaAgendadaDto(c.Id, c.Descricao, "cartao-arquivado",
                    $"{c.ContaNome} está arquivado, mas {c.Descricao} ainda cobra nele: troque nos serviços."));
            var texto = CategorizacaoService.Normalizar(c.TextoCobranca);
            if (texto.Length == 0) continue;
            var minhas = normalizadas.Where(t => t.Texto.Contains(texto, StringComparison.Ordinal))
                .OrderByDescending(t => t.Data).ToList();

            if (c.FormaCobranca != FormaCobranca.AMao && c.ProximoVencimento.AddDays(DiasNaoVeio) < hoje)
                avisos.Add(new AvisoContaAgendadaDto(c.Id, c.Descricao, "nao-veio",
                    $"A cobrança de {c.Descricao} esperada em {c.ProximoVencimento:dd/MM} não apareceu. Cancelada? Cartão recusado?",
                    Data: c.ProximoVencimento));
            if (minhas.Count == 0) continue;
            var ultima = minhas[0];
            if (c.ContaId is int contaId && ultima.ContaId != contaId)
                avisos.Add(new AvisoContaAgendadaDto(c.Id, c.Descricao, "mudou-de-cartao",
                    $"A última cobrança de {c.Descricao} ({ultima.Data:dd/MM}) veio em {ultima.ContaNome}, e não em {c.ContaNome}.",
                    ContaNovaId: ultima.ContaId, ContaNovaNome: ultima.ContaNome, Data: ultima.Data));
            // As três últimas da mesma conta ou cartão da última: a mesma marca em outro lugar não mistura os valores.
            var ultimas = minhas.Where(t => t.ContaId == ultima.ContaId).Take(3).Select(t => t.Valor).ToList();
            var variaSempre = ultimas.Count >= 3 && ultimas.Max() > 0
                && (ultimas.Max() - ultimas.Min()) / ultimas.Max() > VariacaoValorVariavel;
            if (!variaSempre && c.Valor > 0 && Math.Abs(ultima.Valor - c.Valor) / c.Valor > VariacaoAumento)
                avisos.Add(new AvisoContaAgendadaDto(c.Id, c.Descricao, "aumentou",
                    ultima.Valor > c.Valor
                        ? $"{c.Descricao} aumentou na cobrança de {ultima.Data:dd/MM}."
                        : $"{c.Descricao} baixou na cobrança de {ultima.Data:dd/MM}.",
                    ValorAnterior: c.Valor, ValorNovo: Math.Round(ultima.Valor, 2), Data: ultima.Data));
        }
        return avisos;
    }
}
