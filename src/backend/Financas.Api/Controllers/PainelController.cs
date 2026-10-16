using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos;
using Financas.Api.Servicos.Cambio;
using Financas.Api.Servicos.Investimentos;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Controllers;

[Route("api/painel")]
public class PainelController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly ContaService _contas;
    private readonly RelatorioService _relatorios;
    private readonly ConfiguracaoService _config;
    private readonly CarteiraService _carteira;
    private readonly EmprestimoService _emprestimos;
    private readonly ParametroService _parametros;

    public PainelController(FinancasDbContext db, ContaService contas, RelatorioService relatorios, ConfiguracaoService config,
        CarteiraService carteira, EmprestimoService emprestimos, ParametroService parametros)
    {
        _db = db;
        _contas = contas;
        _relatorios = relatorios;
        _config = config;
        _carteira = carteira;
        _emprestimos = emprestimos;
        _parametros = parametros;
    }

    [HttpGet]
    public async Task<ActionResult<PainelDto>> Obter([FromQuery] int? ano, [FromQuery] int? mes, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var a = ano ?? hoje.Year;
        var m = mes ?? hoje.Month;
        if (a < 2000 || a > 2100 || m < 1 || m > 12) return Erro("Período inválido.");
        var inicio = Relogio.PrimeiroDiaMes(a, m);
        var fim = Relogio.UltimoDiaMes(a, m);

        // Saldos
        var saldos = await _contas.CalcularSaldosAsync(ct);
        var contas = await _db.Contas.AsNoTracking()
            .Where(c => !c.Arquivada && c.IncluirNoSaldo)
            .Select(c => new { c.Id, c.Tipo })
            .ToListAsync(ct);
        decimal saldoContas = 0, dividaCartoes = 0, investidoManual = 0;
        foreach (var c in contas)
        {
            var s = saldos.GetValueOrDefault(c.Id);
            switch (c.Tipo)
            {
                case TipoConta.CartaoCredito: dividaCartoes += Math.Max(0, -s); break;
                case TipoConta.Investimento: investidoManual += s; break;
                default: saldoContas += s; break;
            }
        }
        // Open Finance + carteira de renda variável (cotação atual), sem contar duas vezes o mesmo ativo.
        var composicao = await _carteira.ComposicaoAsync(ct);
        var totalInvestido = investidoManual + composicao.Total;
        // Saldo devedor de financiamentos e empréstimos (Open Finance), mostrados separadamente.
        var credito = await _emprestimos.TotaisAsync(ct);
        var patrimonio = saldoContas + totalInvestido - dividaCartoes - credito.SaldoFinanciamentos - credito.SaldoEmprestimos;

        // Movimento do mês
        var (receitas, despesas) = await _relatorios.TotaisAsync(inicio, fim, ct);
        var gastos = await _relatorios.TotaisPorCategoriaAsync(inicio, fim, TipoTransacao.Despesa, agruparNoPai: true, ct);
        var evolucao = await _relatorios.FluxoMensalAsync(inicio.AddMonths(-11), fim, ct);
        var orcamentos = await _relatorios.OrcamentosDoMesAsync(a, m, ct);
        var diasVencimentos = await _parametros.ObterInteiroAsync(ConfiguracaoService.DiasProximosVencimentos, ct);
        var proximas = await _relatorios.ContasAgendadasAsync(diasVencimentos, ct);

        var ultimas = await _db.Transacoes.AsNoTracking()
            .OrderByDescending(t => t.Data).ThenByDescending(t => t.Id)
            .Take(8)
            .Select(ProjecoesTransacao.ParaDto)
            .ToListAsync(ct);

        // Alertas
        var alertas = new List<string>();
        foreach (var o in orcamentos.Where(o => o.Percentual > 100))
            alertas.Add(o.Situacao == "emergencia"
                ? $"Orçamento de {o.CategoriaNome} passou do limite ({o.Percentual:0}%) por gastos de emergência."
                : $"Orçamento de {o.CategoriaNome} estourado ({o.Percentual:0}%).");
        foreach (var p in proximas.Where(p => p.DiasParaVencer < 0))
            alertas.Add($"Conta \"{p.Descricao}\" vencida desde {p.ProximoVencimento:dd/MM}.");
        if (composicao.MoedasSemCotacao.Count > 0)
            alertas.Add($"Investimentos em {string.Join(", ", composicao.MoedasSemCotacao)} ficaram fora do patrimônio: "
                + "não há cotação de câmbio guardada para essa moeda.");

        var limiteConsentimento = DateTime.UtcNow.AddDays(
            await _parametros.ObterInteiroAsync(ConfiguracaoService.DiasAvisoConsentimento, ct));
        var conexoes = await _db.ConexoesBancarias.AsNoTracking().Where(c => c.Ativa)
            .Select(c => new { c.NomeInstituicao, c.ConsentimentoExpiraEm, c.UltimoErro, c.Status })
            .ToListAsync(ct);
        foreach (var c in conexoes)
        {
            var nome = c.NomeInstituicao ?? "Instituição";
            if (c.ConsentimentoExpiraEm is { } exp && exp <= limiteConsentimento)
                alertas.Add(exp <= DateTime.UtcNow
                    ? $"O consentimento do Open Finance de {nome} expirou. Renove no Meu Pluggy."
                    : $"O consentimento do Open Finance de {nome} expira em {exp.ToLocalTime():dd/MM/yyyy}. Renove no Meu Pluggy.");
            if (!string.IsNullOrEmpty(c.UltimoErro))
                alertas.Add($"Falha na última sincronização de {nome}. Veja em Configurações › Open Finance.");
            else if (c.Status is "LOGIN_ERROR" or "WAITING_USER_ACTION")
                alertas.Add($"{nome} precisa de uma ação sua no Meu Pluggy para voltar a sincronizar.");
        }

        var contratosEmAtraso = await _db.Emprestimos.AsNoTracking()
            .Where(e => e.ConexaoBancaria!.Ativa && e.SaldoDevedor > 0 && e.ParcelasEmAtraso > 0)
            .Select(e => new { e.Produto, e.Natureza, e.ParcelasEmAtraso })
            .ToListAsync(ct);
        foreach (var e in contratosEmAtraso)
            alertas.Add($"{(e.Natureza == NaturezaCredito.Financiamento ? "Financiamento" : "Empréstimo")} \"{e.Produto}\" " +
                        $"com {e.ParcelasEmAtraso} parcela(s) em atraso.");

        var emergenciaMes = Math.Abs(await _db.Transacoes.AsNoTracking()
            .Where(t => t.Data >= inicio && t.Data <= fim && t.Emergencia && t.Valor < 0 && !t.IgnorarRelatorios
                        && t.Tipo != TipoTransacao.Transferencia)
            .SumAsync(t => (decimal?)t.Valor, ct) ?? 0m);

        var ultimaSincronizacao = await _config.ObterDataAsync(ConfiguracaoService.UltimaSincronizacaoGeral, ct);

        return new PainelDto(saldoContas, dividaCartoes, totalInvestido, patrimonio,
            composicao.RendaVariavel, patrimonio - composicao.RendaVariavel,
            receitas, despesas, gastos, evolucao, orcamentos.Take(6).ToList(), proximas, ultimas, alertas,
            ultimaSincronizacao, emergenciaMes, credito.SaldoFinanciamentos, credito.SaldoEmprestimos, diasVencimentos);
    }
}

[Route("api/emprestimos")]
public class EmprestimosController : BaseController
{
    private readonly EmprestimoService _emprestimos;

    public EmprestimosController(EmprestimoService emprestimos) => _emprestimos = emprestimos;

    /// <summary>Contratos de crédito do Open Finance com os totais de financiamentos e de empréstimos separados.</summary>
    [HttpGet]
    public async Task<ActionResult<PainelEmprestimosDto>> Painel(CancellationToken ct)
        => await _emprestimos.PainelAsync(ct);
}

[Route("api/relatorios")]
public class RelatoriosController : BaseController
{
    private readonly RelatorioService _relatorios;
    private readonly FinancasDbContext _db;

    public RelatoriosController(RelatorioService relatorios, FinancasDbContext db)
    {
        _relatorios = relatorios;
        _db = db;
    }

    [HttpGet("categorias")]
    public async Task<ActionResult<List<ValorCategoriaDto>>> PorCategoria([FromQuery] DateOnly inicio, [FromQuery] DateOnly fim,
        [FromQuery] TipoTransacao tipo = TipoTransacao.Despesa, [FromQuery] bool agrupar = true, CancellationToken ct = default)
    {
        if (fim < inicio || fim.DayNumber - inicio.DayNumber > 3660) return Erro("Período inválido.");
        if (tipo == TipoTransacao.Transferencia) return Erro("Tipo inválido.");
        return await _relatorios.TotaisPorCategoriaAsync(inicio, fim, tipo, agrupar, ct);
    }

    [HttpGet("fluxo")]
    public async Task<ActionResult<List<MesValoresDto>>> Fluxo([FromQuery] DateOnly inicio, [FromQuery] DateOnly fim,
        CancellationToken ct)
    {
        if (fim < inicio || fim.DayNumber - inicio.DayNumber > 3660) return Erro("Período inválido.");
        return await _relatorios.FluxoMensalAsync(inicio, fim, ct);
    }

    /// <summary>Visão anual comparativa e indicadores de saúde financeira.</summary>
    [HttpGet("anual")]
    public async Task<ActionResult<PainelAnualDto>> Anual([FromQuery] int? ano, [FromQuery] int? comparar,
        CancellationToken ct)
    {
        var a = ano ?? Relogio.Hoje.Year;
        var c = comparar ?? a - 1;
        if (a < 2000 || a > 2100 || c < 2000 || c > 2100) return Erro("Ano inválido.");
        return await _relatorios.PainelAnualAsync(a, c, ct);
    }

    /// <summary>Projeção de receitas, despesas e saldo para os próximos meses.</summary>
    [HttpGet("projecao")]
    public async Task<ActionResult<ProjecaoDto>> Projecao([FromQuery] int meses = 6, CancellationToken ct = default)
    {
        if (meses < 1 || meses > 24) return Erro("Informe entre 1 e 24 meses.");
        return await _relatorios.ProjecaoAsync(meses, ct);
    }

    /// <summary>Maiores despesas do período.</summary>
    [HttpGet("maiores-despesas")]
    public async Task<ActionResult<List<TransacaoDto>>> MaioresDespesas([FromQuery] DateOnly inicio, [FromQuery] DateOnly fim,
        CancellationToken ct)
    {
        if (fim < inicio) return Erro("Período inválido.");
        return await _db.Transacoes.AsNoTracking()
            .Where(t => t.Data >= inicio && t.Data <= fim && t.Tipo == TipoTransacao.Despesa && !t.IgnorarRelatorios)
            .OrderBy(t => t.Valor)
            .Take(15)
            .Select(ProjecoesTransacao.ParaDto)
            .ToListAsync(ct);
    }
}

[Route("api/investimentos")]
public class InvestimentosController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly CambioService _cambio;

    public InvestimentosController(FinancasDbContext db, CambioService cambio)
    {
        _db = db;
        _cambio = cambio;
    }

    [HttpGet]
    public async Task<ActionResult<List<InvestimentoDto>>> Listar(CancellationToken ct)
    {
        var lista = await _db.Investimentos.AsNoTracking()
            .OrderBy(i => i.Tipo).ThenByDescending(i => i.Saldo)
            .Select(i => new InvestimentoDto(i.Id, i.Nome, i.Tipo, i.Subtipo, i.Codigo, i.Emissor, i.Moeda, i.Saldo, null,
                i.ValorAplicado, i.Rendimento, i.TaxaAnual, i.DataVencimento, i.Status,
                i.ConexaoBancaria!.NomeInstituicao, i.AtualizadoEm))
            .ToListAsync(ct);
        var conversor = await _cambio.ObterConversorAsync(ct);
        return lista.Select(i => i with { SaldoEmReais = conversor.ParaReais(i.Saldo, i.Moeda) }).ToList();
    }
}
