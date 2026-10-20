using System.Text.RegularExpressions;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// Sugere contas a pagar a partir do extrato: pagamentos que se repetem todo mês (boleto, débito automático, Pix
/// para a mesma pessoa...) e que ainda não estão cadastrados em Contas a pagar. Só sugere — quem cadastra é o usuário.
/// O Open Finance não informa boletos futuros (DDA), então o próximo vencimento é estimado pelo dia habitual.
/// #158 (ADR 0043): os cartões também entram (assinaturas: valor fixo, ±5%), a assinatura anual é reconhecida, e cada
/// sugestão diz como é cobrada (à mão, débito automático ou cartão) e o texto que a liga às cobranças.
/// #241: a transação com o nome de uma regra de descrição com faixa de valor é agrupada por esse nome, e não pela
/// descrição do banco: duas cobranças com a mesma descrição e valores diferentes viram duas sugestões.
/// </summary>
public partial class SugestaoContaAgendadaService
{
    /// <summary>Janela do histórico analisado para as mensais (~7 meses).</summary>
    private const int DiasHistorico = 210;
    /// <summary>#158: janela das anuais (um pouco mais de um ano, para ver duas cobranças).</summary>
    private const int DiasHistoricoAnual = 400;
    /// <summary>#158: entre duas cobranças de uma assinatura anual (um ano, com folga para o dia variar).</summary>
    private const int MinimoDiasAnual = 330, MaximoDiasAnual = 400;
    /// <summary>
    /// #158: no cartão, assinatura tem valor fixo. Valor que varia mais que isso nas últimas cobranças é gasto frequente
    /// (iFood, Uber), não assinatura (decisão do usuário: ±5%).
    /// </summary>
    private const decimal VariacaoCartao = 0.05m;
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
    private const int MaximoSugestoes = 30;
    /// <summary>
    /// #205: conta a pagar com dia de vencimento até tantos dias do dia habitual da sugestão pode ser a mesma (o valor
    /// usa a mesma folga de <see cref="VariacaoValorFixo"/>).
    /// </summary>
    private const int FolgaDiasParecida = 3;

    private readonly FinancasDbContext _db;

    public SugestaoContaAgendadaService(FinancasDbContext db) => _db = db;

    public async Task<List<SugestaoContaAgendadaDto>> SugerirAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var desde = hoje.AddDays(-DiasHistoricoAnual);
        var desdeMensal = hoje.AddDays(-DiasHistorico);

        // Compra parcelada não é conta recorrente. #158: os cartões entram (assinaturas); o pagamento da fatura não
        // aparece aqui porque é transferência.
        var transacoes = await _db.Transacoes.AsNoTracking()
            .Where(t => t.Data >= desde && t.Data <= hoje && t.Valor < 0 && t.Tipo == TipoTransacao.Despesa
                && t.TransferenciaGrupo == null && !t.IgnorarRelatorios && t.ParcelaTotal == null
                && t.Conta!.Tipo != TipoConta.Investimento && !t.Conta.Arquivada)
            .OrderByDescending(t => t.Data)
            .Select(t => new Lancamento(t.Data, t.Descricao, t.DescricaoOriginal, t.Estabelecimento, t.Valor, t.CategoriaId,
                t.Categoria != null ? t.Categoria.Nome : null, t.ContaId, t.Conta!.Nome, t.Conta.Tipo, t.RegraDescricaoId))
            .Take(20000)
            .ToListAsync(ct);
        var regrasPorValor = await RegraDescricaoService.RegrasPorValorAsync(_db, ct);

        var cadastradas = await _db.ContasAgendadas.AsNoTracking()
            .Select(c => new Cadastrada(c.Id, c.Descricao, c.TextoCobranca, c.ChaveExtrato, c.Ativa, c.Valor,
                c.ProximoVencimento, c.Recorrencia, c.ContaId))
            .ToListAsync(ct);
        // #205: a chave gravada ao cadastrar pela sugestão (ou no "é a mesma") vale qualquer que seja o nome escolhido.
        var jaCadastradas = cadastradas.Select(c => ChaveDe(c.Descricao))
            .Concat(cadastradas.Select(c => c.ChaveExtrato ?? ""))
            .Where(k => k.Length > 0).ToHashSet(StringComparer.Ordinal);
        // #158: a que já tem o texto da cobrança também não volta como sugestão.
        var textosCadastrados = cadastradas.Select(c => CategorizacaoService.Normalizar(c.TextoCobranca))
            .Where(t => t.Length > 0).ToList();
        // #241: o nome inteiro, para comparar com o grupo de uma regra com valor ("Condomínio 1" e "Condomínio 2" têm a
        // mesma chave de três palavras).
        var nomesCadastrados = cadastradas.Select(c => ChaveDaRegra(c.Descricao)).ToHashSet(StringComparer.Ordinal);

        var resultado = new List<SugestaoContaAgendadaDto>();
        var grupos = transacoes
            .Where(t => t.ContaTipo != TipoConta.CartaoCredito || !ParcelaNoTexto(t.DescricaoOriginal ?? t.Descricao))
            .Select(t =>
            {
                var daRegra = t.RegraDescricaoId is int r && regrasPorValor.Contains(r);
                var chave = daRegra ? ChaveDaRegra(t.Descricao) : ChaveDe(t.Estabelecimento ?? t.DescricaoOriginal ?? t.Descricao);
                return new { T = t, DaRegra = daRegra, Chave = chave };
            })
            .Where(x => x.Chave.Length > 0)
            .GroupBy(x => (x.Chave, x.DaRegra), x => x.T);

        foreach (var g in grupos)
        {
            var (chave, daRegra) = g.Key;
            var todos = g.ToList(); // mais recente primeiro
            var recente = todos[0];
            var textoBanco = CategorizacaoService.Normalizar(recente.DescricaoOriginal ?? recente.Descricao);
            if (jaCadastradas.Contains(chave)) continue;
            if (daRegra ? nomesCadastrados.Contains(chave)
                : jaCadastradas.Contains(ChaveDe(recente.Descricao)) || textosCadastrados.Any(t => textoBanco.Contains(t, StringComparison.Ordinal)))
                continue;

            // #158: cada conta ou cartão primeiro (a mesma marca pode aparecer em dois lugares, com valores diferentes);
            // juntas só se nenhuma passar sozinha, o que cobre a assinatura que trocou de cartão no meio do caminho.
            var porConta = todos.GroupBy(t => t.ContaId).ToList();
            var achadas = porConta.Count > 1
                ? porConta.Select(c => Avaliar($"{chave}#{c.Key}", chave, c.ToList(), hoje, desdeMensal, daRegra))
                    .OfType<SugestaoContaAgendadaDto>().ToList()
                : new List<SugestaoContaAgendadaDto>();
            if (achadas.Count == 0 && Avaliar(chave, chave, todos, hoje, desdeMensal, daRegra) is { } juntas) achadas.Add(juntas);
            // #241: uma conta a pagar com o texto da cobrança mais geral (ex.: CONDOMINIO, que está nas duas) só é a
            // cadastrada de uma delas, a de valor parecido.
            if (daRegra)
                achadas.RemoveAll(s => cadastradas.Any(c => CategorizacaoService.Normalizar(c.TextoCobranca) is { Length: > 0 } t
                    && (chave.Contains(t, StringComparison.Ordinal) || textoBanco.Contains(t, StringComparison.Ordinal))
                    && ValorPerto(s.Valor, c.Valor)));
            resultado.AddRange(achadas);
        }

        resultado = SemAsParecidas(resultado, cadastradas);
        return resultado.OrderBy(s => s.ProximoVencimento).ThenByDescending(s => s.Valor).Take(MaximoSugestoes).ToList();
    }

    /// <summary>#205: a conta a pagar como a comparação com as sugestões precisa dela.</summary>
    private sealed record Cadastrada(int Id, string Descricao, string? TextoCobranca, string? ChaveExtrato, bool Ativa,
        decimal Valor, DateOnly ProximoVencimento, Recorrencia Recorrencia, int? ContaId);

    /// <summary>
    /// #205: contas a pagar cadastradas à mão (ou antes da <c>ChaveExtrato</c>) com outro nome. Uma conta a pagar ativa,
    /// ainda sem ligação, com valor parecido (folga de <see cref="VariacaoValorFixo"/>), dia de vencimento a até
    /// <see cref="FolgaDiasParecida"/> dias e a mesma conta é a sugestão:
    /// - certeza (some): ela é a única candidata da sugestão, tem a mesma conta, não tem texto da cobrança (que a ligaria a
    ///   outra cobrança) e não serve para nenhuma outra sugestão;
    /// - dúvida (fica, com "parece ser"): mais de uma candidata, a mesma conta a pagar serve para mais de uma sugestão,
    ///   a conta a pagar não diz a conta ou já tem outro texto da cobrança. "É a mesma" grava a ligação.
    /// </summary>
    private static List<SugestaoContaAgendadaDto> SemAsParecidas(List<SugestaoContaAgendadaDto> sugestoes,
        List<Cadastrada> cadastradas)
    {
        var livres = cadastradas.Where(c => c.Ativa && c.ChaveExtrato == null).ToList();
        if (livres.Count == 0 || sugestoes.Count == 0) return sugestoes;

        var candidatas = sugestoes.Select(s => (Sugestao: s, Lista: livres.Where(c => Parecida(s, c)).ToList())).ToList();
        var usos = candidatas.SelectMany(x => x.Lista).GroupBy(c => c.Id).ToDictionary(g => g.Key, g => g.Count());
        var resultado = new List<SugestaoContaAgendadaDto>();
        foreach (var (s, lista) in candidatas)
        {
            if (lista.Count == 0)
            {
                resultado.Add(s);
                continue;
            }
            if (lista.Count == 1 && Certeza(s, lista[0]) && usos[lista[0].Id] == 1) continue;
            var melhor = lista.OrderByDescending(c => Certeza(s, c))
                .ThenBy(c => Math.Abs(c.Valor - s.Valor))
                .ThenBy(c => DistanciaDias(c, s))
                .First();
            resultado.Add(s with { PareceSerId = melhor.Id, PareceSerNome = melhor.Descricao });
        }
        return resultado;
    }

    /// <summary>#205: valor, dia e conta parecidos (a conta a pagar sem conta serve para qualquer uma, mas só com dúvida).</summary>
    private static bool Parecida(SugestaoContaAgendadaDto s, Cadastrada c)
    {
        if (c.ContaId is int contaId && contaId != s.ContaId) return false;
        // Anual com anual; a mensal pode ter sido cadastrada como mensal ou única.
        if ((s.Recorrencia == Recorrencia.Anual) != (c.Recorrencia == Recorrencia.Anual)) return false;
        var naFaixa = s.ValorVariavel && c.Valor >= s.ValorMinimo && c.Valor <= s.ValorMaximo;
        return (ValorPerto(s.Valor, c.Valor) || naFaixa) && DistanciaDias(c, s) <= FolgaDiasParecida;
    }

    /// <summary>#205: valores a até <see cref="VariacaoValorFixo"/> um do outro.</summary>
    private static bool ValorPerto(decimal a, decimal b)
    {
        var referencia = Math.Max(a, b);
        return referencia > 0 && Math.Abs(a - b) <= referencia * VariacaoValorFixo;
    }

    /// <summary>#205: a mesma conta informada e nenhum texto da cobrança que a ligue a outra coisa.</summary>
    private static bool Certeza(SugestaoContaAgendadaDto s, Cadastrada c)
        => c.ContaId == s.ContaId && string.IsNullOrWhiteSpace(c.TextoCobranca);

    /// <summary>
    /// #205: dias entre o vencimento da conta a pagar e o da sugestão. Mensal: pelo dia do mês, dando a volta (dia 30 e
    /// dia 1 estão a 2 dias). Anual: pelo dia do ano.
    /// </summary>
    private static int DistanciaDias(Cadastrada c, SugestaoContaAgendadaDto s)
    {
        var (a, b, ciclo) = s.Recorrencia == Recorrencia.Anual
            ? (c.ProximoVencimento.DayOfYear, s.ProximoVencimento.DayOfYear, 365)
            : (c.ProximoVencimento.Day, s.DiaHabitual, 31);
        var d = Math.Abs(a - b);
        return Math.Min(d, ciclo - d);
    }

    /// <summary>Um lançamento do extrato, como a sugestão precisa dele.</summary>
    private sealed record Lancamento(DateOnly Data, string Descricao, string? DescricaoOriginal, string? Estabelecimento,
        decimal Valor, int? CategoriaId, string? CategoriaNome, int ContaId, string ContaNome, TipoConta ContaTipo,
        int? RegraDescricaoId);

    /// <summary>
    /// A sugestão de um grupo de lançamentos (mais recente primeiro), ou null se ele não for conta recorrente.
    /// <paramref name="chave"/> identifica a sugestão na tela; <paramref name="chaveTexto"/> dá o texto da cobrança.
    /// <paramref name="daRegra"/> (#241): o grupo é o nome de uma regra com faixa de valor, e o texto da cobrança é ele
    /// inteiro (a palavra mais longa, CONDOMINIO, ligaria as duas).
    /// </summary>
    private static SugestaoContaAgendadaDto? Avaliar(string chave, string chaveTexto, List<Lancamento> todos, DateOnly hoje,
        DateOnly desdeMensal, bool daRegra = false)
    {
        var recente = todos[0];
        var conta = todos.GroupBy(t => t.ContaId).OrderByDescending(c => c.Count()).First().First();
        var noCartao = conta.ContaTipo == TipoConta.CartaoCredito;

        var recorrencia = Recorrencia.Mensal;
        var itens = todos.Where(t => t.Data >= desdeMensal).ToList();
        if (!EhMensal(itens.Select(t => t.Data).ToList(), hoje))
        {
            // #158: assinatura anual, uma cobrança por ano (o histórico importado precisa ter as duas).
            if (!EhAnual(todos.Select(t => t.Data).ToList())) return null;
            recorrencia = Recorrencia.Anual;
            itens = todos;
        }

        var valores = itens.Select(t => -t.Valor).ToList();
        var valor = Mediana(valores.Take(3).ToList());
        var minimo = valores.Min();
        var maximo = valores.Max();
        var variavel = valor > 0 && (maximo - minimo) / valor > VariacaoValorFixo;
        // No cartão, só as últimas cobranças contam para o valor fixo: um aumento de preço antigo não tira a assinatura.
        var ultimos = valores.Take(3).ToList();
        if (noCartao && valor > 0 && (ultimos.Max() - ultimos.Min()) / valor > VariacaoCartao) return null;

        var dia = (int)Mediana(itens.Select(t => (decimal)t.Data.Day).ToList());
        DateOnly esperado, proximo;
        if (recorrencia == Recorrencia.Anual)
        {
            esperado = recente.Data.AddYears(1);
            proximo = esperado;
            while (proximo < hoje) proximo = proximo.AddYears(1);
        }
        else
        {
            // Vencimento esperado no mês seguinte ao último pagamento. Se já passou sem o pagamento aparecer (o último pode
            // ter até DiasSemPagamento dias), a sugestão avisa e propõe o próximo que ainda vai vencer: cadastrar não pode
            // criar uma conta já vencida.
            esperado = NoDia(new DateOnly(recente.Data.Year, recente.Data.Month, 1).AddMonths(1), dia);
            proximo = esperado;
            while (proximo < hoje) proximo = NoDia(new DateOnly(proximo.Year, proximo.Month, 1).AddMonths(1), dia);
        }
        DateOnly? semPagamento = esperado < hoje ? esperado : null;

        var categoria = itens.Where(t => t.CategoriaId != null).GroupBy(t => t.CategoriaId)
            .OrderByDescending(c => c.Count()).Select(c => c.First()).FirstOrDefault();
        var descricao = recente.Descricao.Trim();
        var termoBusca = daRegra ? descricao[..Math.Min(descricao.Length, 100)] : TermoBusca(chaveTexto, recente.Estabelecimento, descricao);
        var forma = noCartao ? FormaCobranca.CartaoCredito
            : itens.Any(t => EhDebitoAutomatico(t.DescricaoOriginal ?? t.Descricao)) ? FormaCobranca.DebitoAutomatico
            : FormaCobranca.AMao;
        // O texto que liga a conta às cobranças: a palavra mais longa da chave (ex.: "NETFLIX").
        var textoCobranca = daRegra ? chaveTexto[..Math.Min(chaveTexto.Length, 100)]
            : SugestaoCategoriaService.Palavras(chaveTexto).OrderByDescending(p => p.Length).FirstOrDefault() ?? chaveTexto;

        return new SugestaoContaAgendadaDto(chave,
            descricao.Length > 150 ? descricao[..150] : descricao,
            Math.Round(valor, 2), variavel, Math.Round(minimo, 2), Math.Round(maximo, 2),
            proximo, dia, itens.Select(t => t.Data.Year * 12 + t.Data.Month - 1).Distinct().Count(), itens.Count,
            recente.Data, itens[^1].Data,
            categoria?.CategoriaId, categoria?.CategoriaNome, conta.ContaId, conta.ContaNome,
            termoBusca, semPagamento, forma, textoCobranca, recorrencia, chaveTexto);
    }

    /// <summary>
    /// Mensal: pago recentemente, em pelo menos <see cref="MinimoMeses"/> meses distintos, sem ser gasto frequente
    /// (mais de ~1,5 por mês) e cobrindo a maior parte dos meses do intervalo. <paramref name="datas"/>: mais recente
    /// primeiro.
    /// </summary>
    private static bool EhMensal(List<DateOnly> datas, DateOnly hoje)
    {
        if (datas.Count == 0 || datas[0] < hoje.AddDays(-DiasSemPagamento)) return false;
        var meses = datas.Select(d => d.Year * 12 + d.Month - 1).Distinct().ToList();
        if (meses.Count < MinimoMeses || datas.Count > meses.Count * MaximoPorMes) return false;
        var mesesNoIntervalo = meses.Max() - meses.Min() + 1;
        return (decimal)meses.Count / mesesNoIntervalo >= CoberturaMinima;
    }

    /// <summary>#158: anual: 2 ou 3 cobranças, cada uma de 330 a 400 dias depois da anterior (mais recente primeiro).</summary>
    private static bool EhAnual(List<DateOnly> datas)
    {
        if (datas.Count is < 2 or > 3) return false;
        for (var i = 0; i < datas.Count - 1; i++)
        {
            var dias = datas[i].DayNumber - datas[i + 1].DayNumber;
            if (dias < MinimoDiasAnual || dias > MaximoDiasAnual) return false;
        }
        return true;
    }

    /// <summary>#158: "DEB AUT", "DEBITO AUTOMATICO", "DEB.AUT." — a conta é debitada sozinha, sem você pagar.</summary>
    private static bool EhDebitoAutomatico(string? descricao)
    {
        var texto = " " + SoLetrasENumeros().Replace(CategorizacaoService.Normalizar(descricao), " ") + " ";
        return texto.Contains(" DEB AUT", StringComparison.Ordinal) || texto.Contains(" DEBITO AUT", StringComparison.Ordinal);
    }

    /// <summary>#158: compra parcelada no cartão que veio sem os campos de parcela ("PARC 3/10", "3/10").</summary>
    private static bool ParcelaNoTexto(string? descricao) => descricao is not null && Parcela().IsMatch(descricao);

    [GeneratedRegex(@"\b\d{1,2}\s*/\s*\d{1,2}\b|\bPARC(ELA)?\b", RegexOptions.IgnoreCase | RegexOptions.CultureInvariant)]
    private static partial Regex Parcela();

    [GeneratedRegex("[^A-Z0-9]+")]
    private static partial Regex SoLetrasENumeros();

    /// <summary>O dia habitual dentro do mês (dia 31 vira o último dia em meses mais curtos).</summary>
    private static DateOnly NoDia(DateOnly inicioDoMes, int dia)
        => inicioDoMes.AddDays(Math.Min(dia, DateTime.DaysInMonth(inicioDoMes.Year, inicioDoMes.Month)) - 1);

    /// <summary>
    /// Mesma chave da sugestão de categorias; se a descrição não tiver palavra significativa (ex.: "BOLETO F I A P"),
    /// usa o texto todo sem números, para não perder o pagamento.
    /// </summary>
    internal static string ChaveDe(string? descricao)
    {
        var chave = SugestaoCategoriaService.Chave(descricao);
        if (chave.Length > 0) return chave;
        var texto = CategorizacaoService.Normalizar(descricao);
        return string.Join(' ', texto.Split(' ', StringSplitOptions.RemoveEmptyEntries).Where(p => p.All(char.IsLetter)));
    }

    /// <summary>
    /// #241: a chave do grupo de uma regra com faixa de valor: o nome inteiro, sem acento e em maiúsculas (com os números:
    /// "Condomínio 1" e "Condomínio 2" são dois).
    /// </summary>
    internal static string ChaveDaRegra(string? nome)
    {
        var chave = CategorizacaoService.Normalizar(nome);
        return chave.Length > 150 ? chave[..150].TrimEnd() : chave;
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
