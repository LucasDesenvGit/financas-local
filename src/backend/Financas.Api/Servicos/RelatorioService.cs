using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos.Cambio;
using Financas.Api.Servicos.Investimentos;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>Consultas agregadas usadas pelo painel, orçamentos e relatórios.</summary>
public class RelatorioService
{
    private readonly FinancasDbContext _db;
    private readonly ContaService _contas;
    private readonly CarteiraService _carteira;
    private readonly EmprestimoService _emprestimos;
    private readonly CambioService _cambio;
    private readonly ReservaService _reservas;

    public RelatorioService(FinancasDbContext db, ContaService contas, CarteiraService carteira, EmprestimoService emprestimos,
        CambioService cambio, ReservaService reservas)
    {
        _db = db;
        _contas = contas;
        _carteira = carteira;
        _emprestimos = emprestimos;
        _cambio = cambio;
        _reservas = reservas;
    }

    /// <summary>Transações que entram nos relatórios (exclui transferências e itens marcados para ignorar).</summary>
    public IQueryable<Transacao> Relevantes(DateOnly inicio, DateOnly fim)
        => _db.Transacoes.AsNoTracking()
            .Where(t => t.Data >= inicio && t.Data <= fim
                        && !t.IgnorarRelatorios
                        && t.Tipo != TipoTransacao.Transferencia);

    /// <summary>
    /// Total por categoria (valores positivos). Subcategorias podem ser agrupadas na categoria pai.
    /// </summary>
    public async Task<List<ValorCategoriaDto>> TotaisPorCategoriaAsync(DateOnly inicio, DateOnly fim, TipoTransacao tipo,
        bool agruparNoPai, CancellationToken ct, bool somenteEmergencia = false)
    {
        var brutos = await Relevantes(inicio, fim)
            .Where(t => t.Tipo == tipo && (!somenteEmergencia || t.Emergencia))
            .GroupBy(t => t.CategoriaId)
            .Select(g => new { CategoriaId = g.Key, Soma = g.Sum(t => t.Valor) })
            .ToListAsync(ct);

        var categorias = await _db.Categorias.AsNoTracking()
            .ToDictionaryAsync(c => c.Id, c => new { c.Nome, c.Cor, c.CategoriaPaiId }, ct);

        var resultado = new Dictionary<int, (string Nome, string Cor, decimal Valor)>();
        decimal semCategoria = 0;
        foreach (var b in brutos)
        {
            // Despesa é negativa e o estorno, positivo: a soma já sai líquida (ADR 0036).
            var valor = tipo == TipoTransacao.Despesa ? -b.Soma : b.Soma;
            if (b.CategoriaId is not int id || !categorias.TryGetValue(id, out var cat))
            {
                semCategoria += valor;
                continue;
            }
            if (agruparNoPai && cat.CategoriaPaiId is int paiId && categorias.TryGetValue(paiId, out var pai))
            {
                id = paiId;
                cat = pai;
            }
            resultado[id] = resultado.TryGetValue(id, out var atual)
                ? (atual.Nome, atual.Cor, atual.Valor + valor)
                : (cat.Nome, cat.Cor, valor);
        }

        // Categoria só com estorno no período (a compra foi antes) fica de fora: não existe gasto negativo no gráfico.
        var lista = resultado.Where(kv => kv.Value.Valor > 0)
            .Select(kv => new ValorCategoriaDto(kv.Key, kv.Value.Nome, kv.Value.Cor, kv.Value.Valor)).ToList();
        if (semCategoria > 0) lista.Add(new ValorCategoriaDto(null, "Sem categoria", "#94a3b8", semCategoria));
        return lista.OrderByDescending(x => x.Valor).ToList();
    }

    public async Task<(decimal Receitas, decimal Despesas)> TotaisAsync(DateOnly inicio, DateOnly fim, CancellationToken ct)
    {
        var q = Relevantes(inicio, fim);
        // Pelo tipo, não pelo sinal (ADR 0036): estorno no cartão é despesa positiva e abate as despesas.
        var receitas = await q.Where(t => t.Tipo == TipoTransacao.Receita).SumAsync(t => (decimal?)t.Valor, ct) ?? 0m;
        var despesas = await q.Where(t => t.Tipo == TipoTransacao.Despesa).SumAsync(t => (decimal?)t.Valor, ct) ?? 0m;
        return (receitas, -despesas);
    }

    /// <summary>Receitas e despesas mês a mês (inclui meses sem movimento).</summary>
    public async Task<List<MesValoresDto>> FluxoMensalAsync(DateOnly inicio, DateOnly fim, CancellationToken ct)
    {
        var dados = await Relevantes(inicio, fim)
            .GroupBy(t => new { t.Data.Year, t.Data.Month })
            .Select(g => new
            {
                g.Key.Year,
                g.Key.Month,
                Receitas = g.Sum(t => t.Tipo == TipoTransacao.Receita ? t.Valor : 0m),
                Despesas = g.Sum(t => t.Tipo == TipoTransacao.Despesa ? t.Valor : 0m),
                Emergencia = g.Sum(t => t.Valor < 0 && t.Emergencia ? t.Valor : 0m)
            })
            .ToListAsync(ct);

        var lista = new List<MesValoresDto>();
        for (var m = new DateOnly(inicio.Year, inicio.Month, 1); m <= fim; m = m.AddMonths(1))
        {
            var d = dados.FirstOrDefault(x => x.Year == m.Year && x.Month == m.Month);
            lista.Add(new MesValoresDto(m.Year, m.Month, d?.Receitas ?? 0m, -(d?.Despesas ?? 0m),
                Math.Abs(d?.Emergencia ?? 0m)));
        }
        return lista;
    }

    public async Task<List<OrcamentoDto>> OrcamentosDoMesAsync(int ano, int mes, CancellationToken ct)
    {
        var orcamentos = await _db.Orcamentos.AsNoTracking()
            .Where(o => o.Ano == ano && o.Mes == mes)
            .Select(o => new { o.Id, o.CategoriaId, o.Valor, o.Categoria!.Nome, o.Categoria.Cor })
            .ToListAsync(ct);
        if (orcamentos.Count == 0) return new List<OrcamentoDto>();

        var gastos = await TotaisPorCategoriaAsync(Relogio.PrimeiroDiaMes(ano, mes), Relogio.UltimoDiaMes(ano, mes),
            TipoTransacao.Despesa, agruparNoPai: true, ct);
        // Gasto de subcategorias também precisa contar quando o orçamento é da própria subcategoria.
        var gastosDiretos = await TotaisPorCategoriaAsync(Relogio.PrimeiroDiaMes(ano, mes), Relogio.UltimoDiaMes(ano, mes),
            TipoTransacao.Despesa, agruparNoPai: false, ct);
        var emergencias = await TotaisPorCategoriaAsync(Relogio.PrimeiroDiaMes(ano, mes), Relogio.UltimoDiaMes(ano, mes),
            TipoTransacao.Despesa, agruparNoPai: true, ct, somenteEmergencia: true);
        var emergenciasDiretas = await TotaisPorCategoriaAsync(Relogio.PrimeiroDiaMes(ano, mes), Relogio.UltimoDiaMes(ano, mes),
            TipoTransacao.Despesa, agruparNoPai: false, ct, somenteEmergencia: true);

        return orcamentos.Select(o =>
        {
            var gasto = gastos.FirstOrDefault(g => g.CategoriaId == o.CategoriaId)?.Valor
                        ?? gastosDiretos.FirstOrDefault(g => g.CategoriaId == o.CategoriaId)?.Valor
                        ?? 0m;
            var percentual = o.Valor > 0 ? Math.Round(gasto / o.Valor * 100m, 1) : 0m;
            var emergencia = emergencias.FirstOrDefault(g => g.CategoriaId == o.CategoriaId)?.Valor
                             ?? emergenciasDiretas.FirstOrDefault(g => g.CategoriaId == o.CategoriaId)?.Valor
                             ?? 0m;
            // "emergencia": estourou, mas sem os gastos de emergência ficaria dentro do limite.
            var situacao = gasto <= o.Valor
                ? (percentual >= 80 ? "alerta" : "ok")
                : gasto - emergencia <= o.Valor ? "emergencia" : "estourado";
            return new OrcamentoDto(o.Id, o.CategoriaId, o.Nome, o.Cor, (short)ano, (byte)mes, o.Valor, gasto, percentual,
                emergencia, situacao);
        })
        .OrderByDescending(o => o.Percentual)
        .ToList();
    }

    public async Task<List<ContaAgendadaDto>> ContasAgendadasAsync(int? proximosDias, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var q = _db.ContasAgendadas.AsNoTracking().AsQueryable();
        if (proximosDias is int dias)
        {
            var limite = hoje.AddDays(dias);
            // #158: no Painel, o que cai no cartão não é conta a pagar: entra na fatura, que já está lá.
            q = q.Where(c => c.Ativa && c.ProximoVencimento <= limite && c.FormaCobranca != FormaCobranca.CartaoCredito);
        }

        var lista = await q.OrderBy(c => c.ProximoVencimento)
            .Select(c => new
            {
                c.Id, c.Descricao, c.Valor, c.ProximoVencimento, c.Recorrencia, c.CategoriaId,
                CategoriaNome = c.Categoria != null ? c.Categoria.Nome : null,
                c.ContaId,
                ContaNome = c.Conta != null ? c.Conta.Nome : null,
                c.Ativa, c.UltimoPagamentoEm, c.DataLimite, c.FormaCobranca, c.TextoCobranca, c.Observacao
            })
            .ToListAsync(ct);

        return lista.Select(c =>
            {
                var restantes = c.Ativa ? PagamentosAte(c.ProximoVencimento, c.Recorrencia, c.DataLimite) : null;
                var encerrada = !c.Ativa && c.DataLimite is { } limite && c.Recorrencia != Recorrencia.Unica
                    && c.ProximoVencimento > limite;
                return new ContaAgendadaDto(c.Id, c.Descricao, c.Valor, c.ProximoVencimento, c.Recorrencia,
                    c.CategoriaId, c.CategoriaNome, c.ContaId, c.ContaNome, c.Ativa, c.UltimoPagamentoEm,
                    c.ProximoVencimento.DayNumber - hoje.DayNumber, c.DataLimite, restantes, encerrada,
                    c.FormaCobranca, c.TextoCobranca, c.Observacao);
            })
            .ToList();
    }

    /// <summary>
    /// Próximo vencimento de uma conta recorrente, do mesmo jeito que o pagamento avança (mês a mês ou ano a ano a
    /// partir do anterior). Única não tem próximo.
    /// </summary>
    public static DateOnly? ProximoVencimentoApos(DateOnly vencimento, Recorrencia recorrencia) => recorrencia switch
    {
        Recorrencia.Mensal => vencimento.AddMonths(1),
        Recorrencia.Anual => vencimento.AddYears(1),
        _ => null
    };

    /// <summary>#69: quantos pagamentos faltam até a data limite, contando o próximo (null = sem data limite).</summary>
    public static int? PagamentosAte(DateOnly proximo, Recorrencia recorrencia, DateOnly? limite)
    {
        if (limite is not { } fim || recorrencia == Recorrencia.Unica) return null;
        var n = 0;
        for (DateOnly? v = proximo; v is { } d && d <= fim && n < 1200; v = ProximoVencimentoApos(d, recorrencia)) n++;
        return n;
    }

    /// <summary>
    /// Meses completos com dados que entram nas médias (no máximo 6, terminando no mês passado), para não diluir a
    /// média com meses vazios. Usada pela Projeção e pelo assistente de metas.
    /// </summary>
    public async Task<(DateOnly Inicio, int Meses)> JanelaHistoricoAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var inicioMesAtual = new DateOnly(hoje.Year, hoje.Month, 1);
        var primeiraData = await _db.Transacoes.AsNoTracking().MinAsync(t => (DateOnly?)t.Data, ct);
        var inicio = inicioMesAtual.AddMonths(-6);
        if (primeiraData is { } pd)
        {
            var primeiroMesCompleto = pd.Day == 1 ? new DateOnly(pd.Year, pd.Month, 1) : new DateOnly(pd.Year, pd.Month, 1).AddMonths(1);
            if (primeiroMesCompleto > inicio) inicio = primeiroMesCompleto;
        }
        var meses = inicio >= inicioMesAtual
            ? 0
            : (inicioMesAtual.Year - inicio.Year) * 12 + inicioMesAtual.Month - inicio.Month;
        return (inicio, meses);
    }

    /// <summary>
    /// Projeção mês a mês com base no histórico:
    /// - receitas futuras = média mensal das receitas dos últimos meses completos (até 6);
    /// - despesas futuras = média mensal das despesas à vista (sem parcelados) + parcelas futuras já contratadas;
    /// - mês atual = realizado até hoje + média proporcional aos dias restantes;
    /// - saldo projetado parte do saldo disponível (contas − faturas de cartão).
    /// </summary>
    public async Task<ProjecaoDto> ProjecaoAsync(int mesesFuturos, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var inicioMesAtual = new DateOnly(hoje.Year, hoje.Month, 1);
        var fimHistorico = inicioMesAtual.AddDays(-1);
        var (inicioHistorico, mesesConsiderados) = await JanelaHistoricoAsync(ct);

        var historico = mesesConsiderados > 0
            ? await FluxoMensalAsync(inicioHistorico, fimHistorico, ct)
            : new List<MesValoresDto>();

        decimal mediaReceitas = 0, mediaDespesasAVista = 0;
        var categorias = new List<ProjecaoCategoriaDto>();
        if (mesesConsiderados > 0)
        {
            var periodo = Relevantes(inicioHistorico, fimHistorico);
            var somaReceitas = await periodo.Where(t => t.Tipo == TipoTransacao.Receita).SumAsync(t => (decimal?)t.Valor, ct) ?? 0m;
            var aVista = periodo.Where(t => t.Tipo == TipoTransacao.Despesa && (t.ParcelaTotal == null || t.ParcelaTotal <= 1));
            var somaDespesas = await aVista.SumAsync(t => (decimal?)t.Valor, ct) ?? 0m;
            mediaReceitas = Math.Round(somaReceitas / mesesConsiderados, 2);
            mediaDespesasAVista = Math.Round(Math.Max(0m, -somaDespesas) / mesesConsiderados, 2);

            // Média por categoria (despesas à vista) e valor do último mês completo.
            var porCategoria = await aVista.GroupBy(t => t.CategoriaId)
                .Select(g => new { CategoriaId = g.Key, Soma = g.Sum(t => t.Valor) })
                .ToListAsync(ct);
            var inicioUltimoMes = inicioMesAtual.AddMonths(-1);
            var ultimoMes = await aVista.Where(t => t.Data >= inicioUltimoMes)
                .GroupBy(t => t.CategoriaId)
                .Select(g => new { CategoriaId = g.Key, Soma = g.Sum(t => t.Valor) })
                .ToListAsync(ct);

            var media = await AgruparPorCategoriaAsync(porCategoria.Select(x => (x.CategoriaId, x.Soma)), ct);
            var ultimo = await AgruparPorCategoriaAsync(ultimoMes.Select(x => (x.CategoriaId, x.Soma)), ct);
            categorias = media
                .Select(c => new ProjecaoCategoriaDto(c.CategoriaId, c.Nome, c.Cor,
                    Math.Round(c.Valor / mesesConsiderados, 2),
                    ultimo.FirstOrDefault(u => u.CategoriaId == c.CategoriaId && u.Nome == c.Nome)?.Valor ?? 0m))
                .OrderByDescending(c => c.MediaMensal)
                .ToList();
        }

        // Parcelas futuras (#165): as que o banco já mandou com data futura e, depois da última de cada compra, as
        // projetadas. Uma compra só uma vez, mesmo com várias parcelas recentes.
        var limiteParcelas = hoje.AddDays(-ParcelasFuturas.DiasRecentes);
        var parcelas = await _db.Transacoes.AsNoTracking()
            .Where(t => t.Data >= limiteParcelas && t.Valor < 0 && !t.IgnorarRelatorios
                        && t.ParcelaTotal != null && t.ParcelaTotal > 1 && t.ParcelaNumero != null)
            .Select(t => new ParcelasFuturas.Parcela(t.Id, t.ContaId, t.Data, t.Valor, t.ParcelaNumero!.Value, t.ParcelaTotal!.Value,
                t.DescricaoOriginal ?? t.Descricao))
            .ToListAsync(ct);
        var parcelasPorMes = new Dictionary<(int Ano, int Mes), decimal>();
        var futuras = ParcelasFuturas.Projetar(parcelas, hoje);
        foreach (var p in futuras)
        {
            var chave = (p.Data.Year, p.Data.Month);
            parcelasPorMes[chave] = parcelasPorMes.GetValueOrDefault(chave) + p.Valor;
        }

        // Saldo disponível hoje: contas (exceto investimentos) − dívida dos cartões.
        var saldos = await _contas.CalcularSaldosAsync(ct);
        var contas = await _db.Contas.AsNoTracking().Where(c => !c.Arquivada && c.IncluirNoSaldo)
            .Select(c => new { c.Id, c.Tipo, c.Origem }).ToListAsync(ct);
        var saldoDisponivel = contas
            .Where(c => c.Tipo != TipoConta.Investimento)
            .Sum(c => saldos.GetValueOrDefault(c.Id));
        // #257: o saldo do cartão do Open Finance é a dívida toda, com as parcelas que ainda vão vencer, e os meses previstos
        // abaixo tiram essas parcelas de novo. As dos meses seguintes voltam ao saldo de partida para sair uma vez só, no
        // mês em que vencem (as do mês atual ficam: o mês em andamento não as tira). O cartão manual não precisa: o saldo
        // dele é a soma do que já foi lançado.
        var cartoesOpenFinance = contas.Where(c => c.Tipo == TipoConta.CartaoCredito && c.Origem == OrigemConta.OpenFinance)
            .Select(c => c.Id).ToHashSet();
        var inicioProximoMes = inicioMesAtual.AddMonths(1);
        var parcelasNoSaldoDoCartao = Math.Round(futuras
            .Where(f => cartoesOpenFinance.Contains(f.ContaId) && f.Data >= inicioProximoMes).Sum(f => f.Valor), 2);
        saldoDisponivel += parcelasNoSaldoDoCartao;

        var meses = historico
            .Select(h => new ProjecaoMesDto(h.Ano, h.Mes, "realizado", h.Receitas, h.Despesas, 0m, h.Receitas - h.Despesas, null))
            .ToList();

        // Mês atual
        var (receitasAtual, despesasAtual) = await TotaisAsync(inicioMesAtual, hoje, ct);
        var diasMes = DateTime.DaysInMonth(hoje.Year, hoje.Month);
        var fracaoRestante = (decimal)(diasMes - hoje.Day) / diasMes;
        var receitasMesAtual = Math.Max(receitasAtual, mediaReceitas);
        var despesasMesAtual = Math.Round(despesasAtual + mediaDespesasAVista * fracaoRestante, 2);
        var saldo = saldoDisponivel + (receitasMesAtual - receitasAtual) - (despesasMesAtual - despesasAtual);
        meses.Add(new ProjecaoMesDto(hoje.Year, hoje.Month, "em andamento", receitasMesAtual, despesasMesAtual, 0m,
            receitasMesAtual - despesasMesAtual, Math.Round(saldo, 2)));

        // Meses futuros
        for (var k = 1; k <= mesesFuturos; k++)
        {
            var m = inicioMesAtual.AddMonths(k);
            var parcelasMes = Math.Round(parcelasPorMes.GetValueOrDefault((m.Year, m.Month)), 2);
            var despesas = mediaDespesasAVista + parcelasMes;
            saldo += mediaReceitas - despesas;
            meses.Add(new ProjecaoMesDto(m.Year, m.Month, "projetado", mediaReceitas, despesas, parcelasMes,
                mediaReceitas - despesas, Math.Round(saldo, 2)));
        }

        return new ProjecaoDto(mesesConsiderados, mediaReceitas, mediaDespesasAVista, Math.Round(saldoDisponivel, 2),
            meses, categorias);
    }

    /// <summary>Converte somas por CategoriaId em linhas nomeadas, agrupando subcategorias na categoria pai.</summary>
    private async Task<List<ValorCategoriaDto>> AgruparPorCategoriaAsync(IEnumerable<(int? CategoriaId, decimal Soma)> brutos,
        CancellationToken ct)
    {
        var categorias = await _db.Categorias.AsNoTracking()
            .ToDictionaryAsync(c => c.Id, c => new { c.Nome, c.Cor, c.CategoriaPaiId }, ct);
        var resultado = new Dictionary<int, (string Nome, string Cor, decimal Valor)>();
        decimal semCategoria = 0;
        foreach (var (categoriaId, soma) in brutos)
        {
            var valor = -soma; // despesas líquidas de estornos (ADR 0036)
            if (categoriaId is not int id || !categorias.TryGetValue(id, out var cat))
            {
                semCategoria += valor;
                continue;
            }
            if (cat.CategoriaPaiId is int paiId && categorias.TryGetValue(paiId, out var pai))
            {
                id = paiId;
                cat = pai;
            }
            resultado[id] = resultado.TryGetValue(id, out var atual)
                ? (atual.Nome, atual.Cor, atual.Valor + valor)
                : (cat.Nome, cat.Cor, valor);
        }
        var lista = resultado.Where(kv => kv.Value.Valor > 0)
            .Select(kv => new ValorCategoriaDto(kv.Key, kv.Value.Nome, kv.Value.Cor, kv.Value.Valor)).ToList();
        if (semCategoria > 0) lista.Add(new ValorCategoriaDto(null, "Sem categoria", "#a9b4c6", semCategoria));
        return lista;
    }

    /// <summary>Painel anual: ano escolhido × ano de comparação, distribuição da renda e indicadores de saúde financeira.</summary>
    public async Task<PainelAnualDto> PainelAnualAsync(int ano, int anoComparacao, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var mesLimite = ano < hoje.Year ? 12 : ano == hoje.Year ? hoje.Month : 0;

        var anosDisponiveis = await _db.Transacoes.AsNoTracking()
            .Select(t => t.Data.Year).Distinct().OrderByDescending(a => a).ToListAsync(ct);
        if (!anosDisponiveis.Contains(hoje.Year)) anosDisponiveis.Insert(0, hoje.Year);

        var fluxoAno = await FluxoMensalAsync(new DateOnly(ano, 1, 1), new DateOnly(ano, 12, 31), ct);
        var fluxoAnterior = await FluxoMensalAsync(new DateOnly(anoComparacao, 1, 1), new DateOnly(anoComparacao, 12, 31), ct);

        static ResumoAnoDto Resumir(IEnumerable<MesValoresDto> meses)
        {
            var r = meses.Sum(m => m.Receitas);
            var d = meses.Sum(m => m.Despesas);
            return new ResumoAnoDto(r, d, r - d, r > 0 ? Math.Round((r - d) / r * 100m, 1) : 0m);
        }

        var resumo = Resumir(fluxoAno.Where(m => m.Mes <= mesLimite));
        var resumoAnteriorMesmoPeriodo = Resumir(fluxoAnterior.Where(m => m.Mes <= Math.Max(mesLimite, 1)));
        var resumoAnteriorCompleto = Resumir(fluxoAnterior);

        // Patrimônio estimado ao fim de cada mês: patrimônio de hoje menos o resultado dos meses seguintes.
        var credito = await _emprestimos.TotaisAsync(ct);
        var patrimonioAtual = await PatrimonioAtualAsync(credito, ct);
        var resultadoPorMes = (await FluxoMensalAsync(new DateOnly(ano, 1, 1), hoje, ct))
            .ToDictionary(m => (m.Ano, m.Mes), m => m.Receitas - m.Despesas);
        var meses = new List<MesComparativoDto>();
        for (var mes = 1; mes <= 12; mes++)
        {
            var atual = fluxoAno[mes - 1];
            var anterior = fluxoAnterior[mes - 1];
            var futuro = mes > mesLimite;
            decimal? patrimonio = null;
            if (!futuro)
            {
                var fimMes = Relogio.UltimoDiaMes(ano, mes);
                var resultadoDepois = resultadoPorMes
                    .Where(kv => new DateOnly(kv.Key.Ano, kv.Key.Mes, 1) > fimMes)
                    .Sum(kv => kv.Value);
                patrimonio = Math.Round(patrimonioAtual - resultadoDepois, 2);
            }
            meses.Add(new MesComparativoDto(mes, atual.Receitas, atual.Despesas, anterior.Receitas, anterior.Despesas,
                patrimonio, futuro));
        }

        // Categorias: valor no ano (até o mês limite), mesmo período do ano anterior e valor mês a mês.
        var categoriasInfo = await _db.Categorias.AsNoTracking()
            .ToDictionaryAsync(c => c.Id, c => new { c.Nome, c.Cor, c.CategoriaPaiId, c.Natureza }, ct);
        (int? Id, string Nome, string Cor, NaturezaGasto? Natureza) Raiz(int? categoriaId)
        {
            if (categoriaId is not int id || !categoriasInfo.TryGetValue(id, out var c)) return (null, "Sem categoria", "#a9b4c6", null);
            if (c.CategoriaPaiId is int paiId && categoriasInfo.TryGetValue(paiId, out var pai))
                return (paiId, pai.Nome, pai.Cor, c.Natureza ?? pai.Natureza);
            return (id, c.Nome, c.Cor, c.Natureza);
        }

        var fimAno = mesLimite == 0 ? new DateOnly(ano, 1, 1).AddDays(-1) : Relogio.UltimoDiaMes(ano, Math.Max(mesLimite, 1));
        var despesasAno = mesLimite == 0
            ? new List<(int? CategoriaId, int Mes, decimal Soma)>()
            : (await Relevantes(new DateOnly(ano, 1, 1), fimAno)
                .Where(t => t.Tipo == TipoTransacao.Despesa)
                .GroupBy(t => new { t.CategoriaId, t.Data.Month })
                .Select(g => new { g.Key.CategoriaId, g.Key.Month, Soma = g.Sum(t => t.Valor) })
                .ToListAsync(ct))
                .Select(x => (x.CategoriaId, x.Month, -x.Soma)).ToList();
        var fimAnterior = Relogio.UltimoDiaMes(anoComparacao, Math.Max(mesLimite, 1));
        var despesasAnterior = (await Relevantes(new DateOnly(anoComparacao, 1, 1), fimAnterior)
                .Where(t => t.Tipo == TipoTransacao.Despesa)
                .GroupBy(t => t.CategoriaId)
                .Select(g => new { CategoriaId = g.Key, Soma = g.Sum(t => t.Valor) })
                .ToListAsync(ct))
            .Select(x => (x.CategoriaId, -x.Soma)).ToList();

        var porRaiz = new Dictionary<string, (int? Id, string Nome, string Cor, NaturezaGasto? Natureza, decimal Valor, decimal Anterior, decimal[] Meses)>();
        string Chave((int? Id, string Nome, string Cor, NaturezaGasto? Natureza) r) => r.Id?.ToString() ?? "sem";
        foreach (var (categoriaId, mes, soma) in despesasAno)
        {
            var r = Raiz(categoriaId);
            var k = Chave(r);
            if (!porRaiz.TryGetValue(k, out var item)) item = (r.Id, r.Nome, r.Cor, r.Natureza, 0m, 0m, new decimal[12]);
            item.Valor += soma;
            item.Meses[mes - 1] += soma;
            porRaiz[k] = item;
        }
        foreach (var (categoriaId, soma) in despesasAnterior)
        {
            var r = Raiz(categoriaId);
            var k = Chave(r);
            if (!porRaiz.TryGetValue(k, out var item)) item = (r.Id, r.Nome, r.Cor, r.Natureza, 0m, 0m, new decimal[12]);
            item.Anterior += soma;
            porRaiz[k] = item;
        }
        var categorias = porRaiz.Values
            .Select(c => new CategoriaComparativaDto(c.Id, c.Nome, c.Cor, c.Natureza, c.Valor, c.Anterior, c.Meses.ToList()))
            .OrderByDescending(c => c.Valor).ThenByDescending(c => c.ValorAnterior)
            .ToList();

        // Por categoria de cada despesa (não pela raiz): uma subcategoria com natureza própria conta na dela, e o
        // total bate com Transações filtrada pela natureza.
        decimal SomaNatureza(NaturezaGasto? natureza) => despesasAno.Where(d => Raiz(d.Item1).Natureza == natureza).Sum(d => d.Item3);
        var distribuicao = new DistribuicaoRendaDto(
            resumo.Receitas,
            SomaNatureza(NaturezaGasto.Essencial),
            SomaNatureza(NaturezaGasto.EstiloDeVida),
            SomaNatureza(null),
            resumo.Resultado);

        var saude = await SaudeFinanceiraAsync(patrimonioAtual, credito, categoriasInfo.Where(c => c.Value.Natureza == NaturezaGasto.Essencial
            || (c.Value.Natureza == null && c.Value.CategoriaPaiId is int p && categoriasInfo.TryGetValue(p, out var pai) && pai.Natureza == NaturezaGasto.Essencial))
            .Select(c => c.Key).ToList(), ct);

        return new PainelAnualDto(ano, anoComparacao, mesLimite, anosDisponiveis, resumo, resumoAnteriorMesmoPeriodo,
            resumoAnteriorCompleto, meses, categorias, distribuicao, saude);
    }

    /// <summary>Contas + investimentos − faturas de cartão − financiamentos − empréstimos (mesma regra do painel).</summary>
    private async Task<decimal> PatrimonioAtualAsync(TotaisCreditoDto credito, CancellationToken ct)
    {
        var saldos = await _contas.CalcularSaldosAsync(ct);
        var contas = await _db.Contas.AsNoTracking().Where(c => !c.Arquivada && c.IncluirNoSaldo)
            .Select(c => new { c.Id, c.Tipo }).ToListAsync(ct);
        var total = contas.Sum(c => c.Tipo == TipoConta.CartaoCredito
            ? -Math.Max(0, -saldos.GetValueOrDefault(c.Id))
            : saldos.GetValueOrDefault(c.Id));
        var investimentos = await _carteira.ComposicaoAsync(ct);
        return total + investimentos.Total - credito.SaldoFinanciamentos - credito.SaldoEmprestimos;
    }

    /// <summary>
    /// Os últimos <paramref name="quantidade"/> meses completos antes do mês atual, a partir do primeiro mês completo
    /// com transações (quem começou a usar há pouco tem menos meses). Meses = 0 quando ainda não há mês completo.
    /// </summary>
    public async Task<(DateOnly Inicio, DateOnly Fim, int Meses)> MesesCompletosAsync(int quantidade, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var inicioMesAtual = new DateOnly(hoje.Year, hoje.Month, 1);
        var fim = inicioMesAtual.AddDays(-1);
        var inicio = inicioMesAtual.AddMonths(-quantidade);
        var primeiraData = await _db.Transacoes.AsNoTracking().MinAsync(t => (DateOnly?)t.Data, ct);
        if (primeiraData is { } pd)
        {
            var primeiroMesCompleto = pd.Day == 1 ? new DateOnly(pd.Year, pd.Month, 1) : new DateOnly(pd.Year, pd.Month, 1).AddMonths(1);
            if (primeiroMesCompleto > inicio) inicio = primeiroMesCompleto;
        }
        var meses = inicio >= inicioMesAtual ? 0 : (inicioMesAtual.Year - inicio.Year) * 12 + inicioMesAtual.Month - inicio.Month;
        return (inicio, fim, meses);
    }

    /// <summary>
    /// Renda mensal média dos últimos 12 meses completos e parcelas de cartão do último mês: a mesma regra da Saúde
    /// financeira, usada também pela sugestão de orçamentos.
    /// </summary>
    public async Task<(decimal Renda, decimal ParcelasCartao, int Meses)> RendaEParcelasAsync(CancellationToken ct)
    {
        var (inicio, fim, meses) = await MesesCompletosAsync(12, ct);
        if (meses == 0) return (0m, 0m, 0);
        var renda = (await Relevantes(inicio, fim).Where(t => t.Tipo == TipoTransacao.Receita).SumAsync(t => (decimal?)t.Valor, ct) ?? 0m) / meses;
        var parcelas = Math.Max(0m, -(await Relevantes(new DateOnly(fim.Year, fim.Month, 1), fim)
            .Where(t => t.Tipo == TipoTransacao.Despesa && t.ParcelaTotal != null && t.ParcelaTotal > 1)
            .SumAsync(t => (decimal?)t.Valor, ct) ?? 0m));
        return (renda, parcelas, meses);
    }

    /// <summary>
    /// Indicadores com base nos últimos 12 meses completos. O comprometimento da renda soma as parcelas de cartão
    /// (último mês) com as parcelas mensais dos contratos de financiamento e de empréstimo, informadas separadamente.
    /// </summary>
    private async Task<SaudeFinanceiraDto> SaudeFinanceiraAsync(decimal patrimonioAtual, TotaisCreditoDto credito,
        List<int> categoriasEssenciais, CancellationToken ct)
    {
        var (inicio, fim, meses) = await MesesCompletosAsync(12, ct);
        decimal renda = 0, gasto = 0, gastoEssencial = 0, parcelas = 0;
        if (meses > 0)
        {
            (renda, parcelas, _) = await RendaEParcelasAsync(ct);
            var periodo = Relevantes(inicio, fim);
            gasto = Math.Max(0m, -(await periodo.Where(t => t.Tipo == TipoTransacao.Despesa).SumAsync(t => (decimal?)t.Valor, ct) ?? 0m)) / meses;
            gastoEssencial = Math.Max(0m, -(await periodo
                .Where(t => t.Tipo == TipoTransacao.Despesa && t.CategoriaId != null && categoriasEssenciais.Contains(t.CategoriaId.Value))
                .SumAsync(t => (decimal?)t.Valor, ct) ?? 0m)) / meses;
        }

        // Reserva líquida: contas (exceto cartões e investimentos) + investimentos de renda fixa (em reais; sem cotação
        // fica de fora) + reservas ("cofrinhos"). #172: ações, FIIs e ETFs ficam de fora, também os do Open Finance (como
        // a carteira): o preço oscila e a venda leva dias para virar dinheiro. O ⓘ da reserva de emergência explica.
        var saldos = await _contas.CalcularSaldosAsync(ct);
        var contas = await _db.Contas.AsNoTracking().Where(c => !c.Arquivada && c.IncluirNoSaldo)
            .Select(c => new { c.Id, c.Tipo }).ToListAsync(ct);
        var conversor = await _cambio.ObterConversorAsync(ct);
        var investimentos = await _db.Investimentos.AsNoTracking().Where(i => i.ConexaoBancaria!.Ativa)
            .Select(i => new { i.Saldo, i.Moeda, i.Tipo }).ToListAsync(ct);
        var reserva = contas.Where(c => c.Tipo != TipoConta.CartaoCredito).Sum(c => Math.Max(0, saldos.GetValueOrDefault(c.Id)))
            + investimentos.Where(i => !CarteiraService.EhRendaVariavel(i.Tipo)).Sum(i => conversor.ParaReais(i.Saldo, i.Moeda) ?? 0m)
            + await _reservas.TotalAsync(ct);

        var totalParcelas = parcelas + credito.ParcelaMensalFinanciamentos + credito.ParcelaMensalEmprestimos;
        return new SaudeFinanceiraDto(
            Math.Round(patrimonioAtual, 2), Math.Round(reserva, 2), Math.Round(gasto, 2), Math.Round(gastoEssencial, 2),
            gasto > 0 ? Math.Round(reserva / gasto, 1) : null,
            gastoEssencial > 0 ? Math.Round(reserva / gastoEssencial, 1) : null,
            Math.Round(renda, 2), Math.Round(parcelas, 2),
            renda > 0 ? Math.Round(totalParcelas / renda * 100m, 1) : null,
            meses,
            credito.ParcelaMensalFinanciamentos, credito.ParcelaMensalEmprestimos,
            credito.SaldoFinanciamentos, credito.SaldoEmprestimos);
    }
}
