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
    private readonly ReservaService _reservas;
    private readonly ParametroService _parametros;
    private readonly DescricaoPoucoInformativaService _poucoInformativa;
    private readonly ConferenciaInvestimentosService _conferencia;
    private readonly ContaAgendadaService _cobrancas;

    /// <summary>#9: o aviso de descrições pouco informativas olha só as transações recentes.</summary>
    public const int DiasPoucoInformativas = 90;

    public PainelController(FinancasDbContext db, ContaService contas, RelatorioService relatorios, ConfiguracaoService config,
        CarteiraService carteira, EmprestimoService emprestimos, ReservaService reservas, ParametroService parametros,
        DescricaoPoucoInformativaService poucoInformativa, ConferenciaInvestimentosService conferencia,
        ContaAgendadaService cobrancas)
    {
        _cobrancas = cobrancas;
        _poucoInformativa = poucoInformativa;
        _conferencia = conferencia;
        _db = db;
        _contas = contas;
        _relatorios = relatorios;
        _config = config;
        _carteira = carteira;
        _emprestimos = emprestimos;
        _reservas = reservas;
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

        // #206: os orçamentos estourados vão em lista própria (card agrupado no Painel), os piores primeiro.
        var orcamentosEstourados = orcamentos.Where(o => o.Percentual > 100).OrderByDescending(o => o.Percentual).ToList();

        var alertas = await AlertasAsync(proximas, composicao.MoedasSemCotacao, ct);

        var emergenciaMes = Math.Abs(await _db.Transacoes.AsNoTracking()
            .Where(t => t.Data >= inicio && t.Data <= fim && t.Emergencia && t.Valor < 0 && !t.IgnorarRelatorios
                        && t.Tipo != TipoTransacao.Transferencia)
            .SumAsync(t => (decimal?)t.Valor, ct) ?? 0m);

        var ultimaSincronizacao = await _config.ObterDataAsync(ConfiguracaoService.UltimaSincronizacaoGeral, ct);

        // UX-12/#103: o que cada cartão lançou no mês escolhido (a mesma conta de Contas e cartões).
        var faturasDoMes = (await _contas.FaturasDoMesAsync(inicio, fim, hoje, ct))
            .Where(f => f.ValorMes > 0 || f.FaturaId is not null).ToList();

        // #9: aviso discreto (não é alerta) com as descrições pouco informativas dos últimos 90 dias.
        var (quantidadePoucoInformativas, desdePoucoInformativas, exemplo) = await PoucoInformativasAsync(hoje, ct);

        // #170: trocos e vendas da corretora contando como receita (a correção é com prévia, em Conferir investimentos).
        var creditosCorretora = await _conferencia.CreditosComoReceitaAsync(ct);
        // #173 b): os trocos que a sincronização classificou sozinha.
        var trocosAutomaticos = await _conferencia.ResumoTrocosAutomaticosAsync(ct);

        return new PainelDto(saldoContas, dividaCartoes, totalInvestido, patrimonio,
            composicao.RendaVariavel, patrimonio - composicao.RendaVariavel,
            receitas, despesas, gastos, evolucao, orcamentos.Take(6).ToList(), proximas, ultimas, alertas,
            ultimaSincronizacao, emergenciaMes, credito.SaldoFinanciamentos, credito.SaldoEmprestimos, diasVencimentos,
            faturasDoMes, faturasDoMes.Sum(f => f.ValorMes),
            quantidadePoucoInformativas, desdePoucoInformativas, exemplo,
            creditosCorretora.Quantidade, creditosCorretora.Total, trocosAutomaticos, orcamentosEstourados);
    }
    /// <summary>
    /// #327: os avisos e os orçamentos estourados do mês atual, para as notificações da barra do topo (em todas as
    /// telas). O mesmo que o Painel calcula, sem o resto do Painel.
    /// </summary>
    [HttpGet("avisos")]
    public async Task<ActionResult<AvisosDto>> Avisos(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var orcamentos = await _relatorios.OrcamentosDoMesAsync(hoje.Year, hoje.Month, ct);
        var estourados = orcamentos.Where(o => o.Percentual > 100).OrderByDescending(o => o.Percentual).ToList();
        var diasVencimentos = await _parametros.ObterInteiroAsync(ConfiguracaoService.DiasProximosVencimentos, ct);
        var proximas = await _relatorios.ContasAgendadasAsync(diasVencimentos, ct);
        var composicao = await _carteira.ComposicaoAsync(ct);
        var (quantidade, desde, exemplo) = await PoucoInformativasAsync(hoje, ct);
        return new AvisosDto(hoje.Year, hoje.Month, await AlertasAsync(proximas, composicao.MoedasSemCotacao, ct), estourados,
            quantidade, desde, exemplo);
    }

    /// <summary>#9: as transações dos últimos 90 dias com descrição pouco informativa (quantas, desde quando e um exemplo).</summary>
    private async Task<(int Quantidade, DateOnly Desde, string? Exemplo)> PoucoInformativasAsync(DateOnly hoje, CancellationToken ct)
    {
        var desde = hoje.AddDays(-(DiasPoucoInformativas - 1));
        var lista = await _poucoInformativa.ListarAsync(
            _db.Transacoes.AsNoTracking().Where(t => t.Data >= desde && t.Data <= hoje), ct);
        var exemplo = lista.GroupBy(x => x.Descricao.Trim().ToUpperInvariant())
            .OrderByDescending(g => g.Count()).Select(g => g.First().Descricao.Trim()).FirstOrDefault();
        return (lista.Count, desde, exemplo is { Length: > 60 } ? exemplo[..60] : exemplo);
    }

    /// <summary>Os avisos do Painel e das notificações (#327): conta vencida, Open Finance, erros, contratos em atraso…</summary>
    private async Task<List<string>> AlertasAsync(IEnumerable<ContaAgendadaDto> proximas, IReadOnlyCollection<string> moedasSemCotacao,
        CancellationToken ct)
    {
        var alertas = new List<string>();
        foreach (var p in proximas.Where(p => p.DiasParaVencer < 0))
            alertas.Add($"Conta \"{p.Descricao}\" vencida desde {p.ProximoVencimento:dd/MM}.");
        if (moedasSemCotacao.Count > 0)
            alertas.Add($"Investimentos em {string.Join(", ", moedasSemCotacao)} ficaram fora do patrimônio: "
                + "não há cotação de câmbio guardada para essa moeda.");

        // #172 (ADR 0008): contas são somadas como reais; conta em outra moeda entraria sem conversão.
        var contasEmOutraMoeda = await _db.Contas.AsNoTracking()
            .Where(c => !c.Arquivada && c.Moeda != CambioService.MoedaBase)
            .Select(c => new { c.Nome, c.Moeda, c.IncluirNoSaldo }).ToListAsync(ct);
        foreach (var c in contasEmOutraMoeda)
            alertas.Add($"A conta {c.Nome} é em {c.Moeda}, mas o site trata as contas como reais, sem conversão"
                + (c.IncluirNoSaldo ? ": o saldo dela entra no total como se fosse em reais. Desmarque \"Incluir no saldo\" em Contas e cartões." : ".")
                + " As transações dela entram nos relatórios do mesmo jeito.");

        // #158 (decisão do usuário): no Painel, só "não veio" e "cartão arquivado com cobranças"; os outros, na tela.
        foreach (var aviso in (await _cobrancas.AvisosAsync(ct)).Where(a => a.Tipo is "nao-veio" or "cartao-arquivado"))
            alertas.Add(aviso.Texto + " Veja em Contas a pagar.");

        var reservasAConferir = (await _reservas.ListarAsync(ct))
            .Where(r => r.PodeTerSaldoAnterior || r.ConferenciaAntiga).Select(r => r.Nome).ToList();
        if (reservasAConferir.Count > 0)
            alertas.Add($"Confira o saldo da(s) reserva(s) {string.Join(", ", reservasAConferir)} em Investimentos › Reservas: "
                + "o valor estimado pelo extrato pode estar diferente do real.");

        var limiteConsentimento = DateTime.UtcNow.AddDays(
            await _parametros.ObterInteiroAsync(ConfiguracaoService.DiasAvisoConsentimento, ct));
        var conexoes = await _db.ConexoesBancarias.AsNoTracking().Where(c => c.Ativa)
            .Select(c => new { c.Id, c.ConsentimentoExpiraEm, c.UltimoErro, c.Status })
            .ToListAsync(ct);
        var nomesConexoes = await _contas.NomesDasConexoesAsync(ct);
        foreach (var c in conexoes)
        {
            var nome = nomesConexoes.GetValueOrDefault(c.Id, "Instituição");
            if (c.ConsentimentoExpiraEm is { } exp && exp <= limiteConsentimento)
                alertas.Add(exp <= DateTime.UtcNow
                    ? $"O consentimento do Open Finance de {nome} expirou. Renove no Meu Pluggy."
                    : $"O consentimento do Open Finance de {nome} expira em {exp.ToLocalTime():dd/MM/yyyy}. Renove no Meu Pluggy.");
            if (!string.IsNullOrEmpty(c.UltimoErro))
                alertas.Add($"Falha na última sincronização de {nome}. Veja em Configurações › Open Finance.");
            else if (c.Status is "LOGIN_ERROR" or "WAITING_USER_ACTION")
                alertas.Add($"{nome} precisa de uma ação sua no Meu Pluggy para voltar a sincronizar.");
        }

        // #11: erros (não avisos) da última semana ainda não marcados como resolvidos.
        var desdeErros = DateTime.UtcNow.AddDays(-7);
        var errosAbertos = await _db.ErrosSistema.AsNoTracking()
            .CountAsync(e => !e.Resolvido && e.Nivel >= NivelErro.Erro && e.UltimaEm >= desdeErros, ct);
        if (errosAbertos > 0)
            alertas.Add(errosAbertos == 1
                ? "1 erro do sistema nos últimos 7 dias. Veja em Configurações › Erros."
                : $"{errosAbertos} erros do sistema nos últimos 7 dias. Veja em Configurações › Erros.");

        var contratosEmAtraso = await _db.Emprestimos.AsNoTracking()
            .Where(e => e.ConexaoBancaria!.Ativa && e.SaldoDevedor > 0 && e.ParcelasEmAtraso > 0)
            .Select(e => new { e.Produto, e.Natureza, e.ParcelasEmAtraso })
            .ToListAsync(ct);
        foreach (var e in contratosEmAtraso)
            alertas.Add($"{(e.Natureza == NaturezaCredito.Financiamento ? "Financiamento" : "Empréstimo")} \"{e.Produto}\" " +
                        $"com {e.ParcelasEmAtraso} parcela(s) em atraso.");

        return alertas;
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
    private readonly HistoricoInvestimentosService _historico;

    public InvestimentosController(FinancasDbContext db, CambioService cambio, HistoricoInvestimentosService historico)
    {
        _db = db;
        _cambio = cambio;
        _historico = historico;
    }

    /// <summary>#173 e) (ADR 0045): rendimento de cada mês dos investimentos do Open Finance, pelo histórico de saldo.</summary>
    [HttpGet("historico")]
    public async Task<ActionResult<HistoricoInvestimentosDto>> Historico(CancellationToken ct)
        => await _historico.MesAMesAsync(ct);

    [HttpGet]
    public async Task<ActionResult<List<InvestimentoDto>>> Listar(CancellationToken ct)
    {
        var lista = await _db.Investimentos.AsNoTracking()
            .OrderBy(i => i.Tipo).ThenByDescending(i => i.Saldo)
            .Select(i => new InvestimentoDto(i.Id, i.Nome, i.Tipo, i.Subtipo, i.Codigo, i.Emissor, i.Moeda, i.Saldo, null,
                i.ValorAplicado, i.Rendimento, i.TaxaAnual, i.DataVencimento, i.Status,
                i.ConexaoBancaria!.NomeInstituicao, i.AtualizadoEm, i.Nome, i.NomePersonalizado))
            .ToListAsync(ct);
        var conversor = await _cambio.ObterConversorAsync(ct);
        return lista.Select(i => i with
        {
            SaldoEmReais = conversor.ParaReais(i.Saldo, i.Moeda),
            NomeExibicao = NomeInvestimento.Exibicao(i.Nome, i.Tipo, i.Emissor, i.NomePersonalizado)
        }).ToList();
    }

    /// <summary>#170: dá um nome ao investimento ("Caixinha viagem"); vazio volta ao automático. A sincronização não mexe.</summary>
    [HttpPut("{id:int}/nome")]
    public async Task<IActionResult> Renomear(int id, RenomearInvestimentoDto dto, CancellationToken ct)
    {
        var nome = string.IsNullOrWhiteSpace(dto.Nome) ? null : dto.Nome.Trim();
        var alterados = await _db.Investimentos.Where(i => i.Id == id)
            .ExecuteUpdateAsync(s => s.SetProperty(i => i.NomePersonalizado, nome), ct);
        return alterados == 0 ? NaoEncontrado("Investimento não encontrado.") : NoContent();
    }
}
