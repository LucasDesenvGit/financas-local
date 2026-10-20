using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos;
using Financas.Api.Servicos.Cambio;
using Financas.Api.Servicos.Investimentos;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Controllers;

/// <summary>Carteira de investimentos: renda variável lançada manualmente e comparação com a Selic.</summary>
[Route("api/carteira")]
public class CarteiraController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly CarteiraService _carteira;
    private readonly SelicService _selic;
    private readonly CotacaoService _cotacoes;
    private readonly ConfiguracaoService _config;
    private readonly FiiService _fiis;
    private readonly ExtratoCarteiraService _extrato;
    private readonly CambioService _cambio;
    private readonly ConferenciaInvestimentosService _conferencia;

    public CarteiraController(FinancasDbContext db, CarteiraService carteira, SelicService selic,
        CotacaoService cotacoes, ConfiguracaoService config, FiiService fiis, ExtratoCarteiraService extrato,
        CambioService cambio, ConferenciaInvestimentosService conferencia)
    {
        _conferencia = conferencia;
        _cambio = cambio;
        _db = db;
        _carteira = carteira;
        _selic = selic;
        _cotacoes = cotacoes;
        _config = config;
        _fiis = fiis;
        _extrato = extrato;
    }

    /// <summary>Painel dos FIIs: renda, compras x hoje, proventos por mês, evolução e rentabilidade x Selic/IFIX.</summary>
    [HttpGet("fiis")]
    public async Task<ActionResult<PainelFiisDto>> Fiis(CancellationToken ct)
    {
        var inicio = await _db.OperacoesAtivo.Where(o => o.Ativo!.Tipo == TipoAtivo.Fii)
            .MinAsync(o => (DateOnly?)o.Data, ct) ?? Relogio.Hoje;
        var serie = await _selic.ObterSerieAsync(inicio, ct);
        return await _fiis.MontarAsync(serie, ct);
    }

    /// <summary>
    /// Registra como compras os FIIs/ações/ETFs do Open Finance (com o histórico de compras e vendas da corretora,
    /// quando ela informa) e já busca cotações, proventos e histórico de preços.
    /// </summary>
    [HttpPost("importar-open-finance")]
    public async Task<ActionResult<ResultadoImportacaoCarteiraDto>> ImportarOpenFinance(
        [FromServices] Servicos.Pluggy.SincronizacaoService sincronizacao, CancellationToken ct)
    {
        ResultadoImportacaoCarteiraDto resultado;
        try
        {
            resultado = await sincronizacao.ImportarCarteiraAsync(ct);
        }
        catch (Servicos.Pluggy.PluggyException ex)
        {
            return Erro(ex.Message);
        }
        if (resultado.AtivosCriados > 0 || resultado.OperacoesImportadas > 0)
        {
            try { await _cotacoes.AtualizarAsync(null, ct); }
            catch (Exception) when (!ct.IsCancellationRequested) { /* as cotações podem ser atualizadas depois */ }
        }
        return resultado;
    }

    /// <summary>Compras e vendas encontradas nas descrições das transações (para importar na carteira).</summary>
    [HttpGet("extrato/sugestoes")]
    public async Task<ActionResult<List<SugestaoExtratoDto>>> SugestoesExtrato(CancellationToken ct)
        => await _extrato.SugestoesAsync(ct);

    [HttpPost("extrato/importar")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> ImportarExtrato(ImportarExtratoDto dto, CancellationToken ct)
    {
        var criadas = await _extrato.ImportarAsync(dto.Itens, ct);
        return new ResultadoAplicacaoDto(criadas);
    }

    /// <summary>
    /// "Conferir investimentos" (ADR 0027): FII cadastrado como outro tipo, cotas que faltam em relação à corretora
    /// (com o débito de compra do extrato que explica a diferença) e troco de compra contado como rendimento.
    /// </summary>
    [HttpGet("conferencia")]
    public async Task<ActionResult<ConferenciaInvestimentosDto>> Conferencia(CancellationToken ct)
        => await _conferencia.ConferirAsync(ct);

    /// <summary>Aplica as correções escolhidas numa gravação só; a resposta traz a chave do "Desfazer".</summary>
    [HttpPost("conferencia/corrigir")]
    public async Task<ActionResult<ResultadoCorrecaoInvestimentosDto>> CorrigirConferencia(CorrigirInvestimentosDto dto,
        CancellationToken ct)
    {
        var (resultado, erro) = await _conferencia.CorrigirAsync(dto, User.Identity?.Name, ct);
        return erro is not null ? Erro(erro) : resultado!;
    }

    [HttpPost("conferencia/desfazer/{chave}")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> DesfazerConferencia(string chave, CancellationToken ct)
    {
        if (await _conferencia.DesfazerAsync(chave, User.Identity?.Name, ct) is not int desfeitas)
            return NaoEncontrado("Não dá mais para desfazer: o prazo passou ou já foi desfeito.");
        return new ResultadoAplicacaoDto(desfeitas);
    }

    /// <summary>#173 b) (ADR 0044): volta os trocos que a sincronização classificou sozinha.</summary>
    [HttpPost("conferencia/trocos-automaticos/desfazer")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> DesfazerTrocosAutomaticos(CancellationToken ct)
        => new ResultadoAplicacaoDto(await _conferencia.DesfazerTrocosAutomaticosAsync(User.Identity?.Name, ct));

    /// <summary>#173 b): "Ok" no aviso do Painel (os trocos ficam como estão).</summary>
    [HttpPost("conferencia/trocos-automaticos/dispensar")]
    public async Task<IActionResult> DispensarTrocosAutomaticos(CancellationToken ct)
    {
        await _conferencia.DispensarTrocosAutomaticosAsync(ct);
        return NoContent();
    }

    [HttpGet]
    public async Task<ActionResult<CarteiraDto>> Obter(CancellationToken ct)
    {
        var serie = await _selic.ObterSerieAsync(await _carteira.InicioSerieSelicAsync(ct), ct);
        var meta = await _selic.MetaAtualAsync(ct);
        return await _carteira.MontarAsync(serie, meta, ct);
    }

    /// <summary>Atualiza as cotações e proventos de todos os ativos (ou de um só).</summary>
    [HttpPost("atualizar-cotacoes")]
    public async Task<ActionResult<ResultadoCotacoesDto>> AtualizarCotacoes([FromQuery] int? ativoId, CancellationToken ct)
    {
        var r = await _cotacoes.AtualizarAsync(ativoId is int id ? new[] { id } : null, ct);
        return new ResultadoCotacoesDto(r.Atualizados, r.ProventosNovos, r.Falhas);
    }

    [HttpGet("ativos/{id:int}/operacoes")]
    public async Task<ActionResult<List<OperacaoAtivoDto>>> Operacoes(int id, CancellationToken ct)
    {
        var inicio = await _db.OperacoesAtivo.Where(o => o.AtivoId == id).MinAsync(o => (DateOnly?)o.Data, ct)
                     ?? Relogio.Hoje;
        var serie = await _selic.ObterSerieAsync(inicio, ct);
        var lista = await _carteira.OperacoesAsync(id, serie, ct);
        if (lista is null) return NaoEncontrado();
        return lista;
    }

    [HttpPost("operacoes")]
    public async Task<IActionResult> CriarOperacao(SalvarOperacaoDto dto, CancellationToken ct)
    {
        var moeda = string.IsNullOrWhiteSpace(dto.Moeda) ? CambioService.MoedaBase : CambioService.NormalizarMoeda(dto.Moeda);
        if (moeda is null) return Erro("Moeda inválida. Use o código de 3 letras (ex.: USD).");
        var exterior = moeda != CambioService.MoedaBase;
        var ticker = CotacaoService.NormalizarTicker(dto.Ticker, moeda);
        if (ticker is null)
            return Erro(exterior
                ? "Código inválido. Use o código como no Yahoo Finance, por exemplo AAPL, VOO ou SAP.DE (ativos da B3 ficam em reais)."
                : "Código inválido. Use o código da B3, por exemplo MXRF11 ou PETR4.");
        var validacao = Validar(dto);
        if (validacao is not null) return validacao;

        var ativo = await _db.Ativos.FirstOrDefaultAsync(a => a.Ticker == ticker, ct);
        var novo = ativo is null;
        if (ativo is null)
        {
            if (exterior)
            {
                var moedas = await _cambio.ListarMoedasAsync(ct);
                if (moedas.Count > 0 && moedas.All(m => m.Codigo != moeda))
                    return Erro($"Não há cotação de câmbio do Banco Central para {moeda}.");
                if (dto.TipoAtivo is TipoAtivo.Fii or TipoAtivo.Bdr)
                    return Erro("FIIs e BDRs são da B3: lance em reais.");
            }
            ativo = new Ativo
            {
                Ticker = ticker,
                Moeda = moeda,
                Tipo = dto.TipoAtivo ?? (exterior ? TipoAtivo.Acao : SugerirTipo(ticker))
            };
            _db.Ativos.Add(ativo);
        }
        else if (ativo.Moeda != moeda)
        {
            return Erro($"{ticker} já está lançado em {ativo.Moeda}. A moeda de um ativo não muda: use {ativo.Moeda}.");
        }

        var cambio = await CambioDaOperacaoAsync(ativo.Moeda, dto, ct);
        if (cambio is null)
            return Erro($"Informe o câmbio (R$ por 1 {ativo.Moeda}): não há cotação do Banco Central para essa data.");

        var op = new OperacaoAtivo
        {
            Ativo = ativo,
            Tipo = dto.Tipo,
            Data = dto.Data,
            Quantidade = dto.Quantidade,
            PrecoUnitario = dto.PrecoUnitario,
            Taxas = dto.Taxas,
            Cambio = cambio.Value,
            Observacao = string.IsNullOrWhiteSpace(dto.Observacao) ? null : dto.Observacao.Trim()
        };
        _db.OperacoesAtivo.Add(op);

        if (dto.Tipo == TipoOperacao.Venda && !await VendaPossivelAsync(ativo, op, ct))
            return Erro("A quantidade vendida é maior do que a quantidade em carteira nessa data.");

        await _db.SaveChangesAsync(ct);

        // Primeiro lançamento do ativo: busca cotação e proventos na hora (falha não impede o cadastro).
        if (novo)
        {
            try { await _cotacoes.AtualizarAsync(new[] { ativo.Id }, ct); }
            catch (Exception) when (!ct.IsCancellationRequested) { /* segue com o preço médio */ }
        }
        return Ok(new { op.Id, ativo.Ticker, AtivoId = ativo.Id });
    }

    [HttpPut("operacoes/{id:int}")]
    public async Task<IActionResult> AtualizarOperacao(int id, SalvarOperacaoDto dto, CancellationToken ct)
    {
        var op = await _db.OperacoesAtivo.Include(o => o.Ativo).FirstOrDefaultAsync(o => o.Id == id, ct);
        if (op is null) return NaoEncontrado();
        var ticker = CotacaoService.NormalizarTicker(dto.Ticker, op.Ativo!.Moeda);
        if (ticker is null || ticker != op.Ativo.Ticker)
            return Erro("Não é possível trocar o ativo de uma operação. Exclua e lance novamente.");
        var validacao = Validar(dto);
        if (validacao is not null) return validacao;
        var cambio = await CambioDaOperacaoAsync(op.Ativo.Moeda, dto, ct);
        if (cambio is null)
            return Erro($"Informe o câmbio (R$ por 1 {op.Ativo.Moeda}): não há cotação do Banco Central para essa data.");

        op.Cambio = cambio.Value;
        op.Tipo = dto.Tipo;
        op.Data = dto.Data;
        op.Quantidade = dto.Quantidade;
        op.PrecoUnitario = dto.PrecoUnitario;
        op.Taxas = dto.Taxas;
        op.Observacao = string.IsNullOrWhiteSpace(dto.Observacao) ? null : dto.Observacao.Trim();
        if (!await VendaPossivelAsync(op.Ativo!, null, ct))
            return Erro("Com essa alteração, alguma venda ficaria maior do que a quantidade em carteira.");

        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpDelete("operacoes/{id:int}")]
    public async Task<IActionResult> ExcluirOperacao(int id, CancellationToken ct)
    {
        var op = await _db.OperacoesAtivo.Include(o => o.Ativo).FirstOrDefaultAsync(o => o.Id == id, ct);
        if (op is null) return NaoEncontrado();
        _db.OperacoesAtivo.Remove(op);
        if (!await VendaPossivelAsync(op.Ativo!, null, ct, excluir: op.Id))
            return Erro("Sem essa compra, uma venda posterior ficaria maior do que a quantidade em carteira.");
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpPut("ativos/{id:int}")]
    public async Task<IActionResult> AtualizarAtivo(int id, AtualizarAtivoDto dto, CancellationToken ct)
    {
        var ativo = await _db.Ativos.FirstOrDefaultAsync(a => a.Id == id, ct);
        if (ativo is null) return NaoEncontrado();
        if (!Enum.IsDefined(dto.Tipo)) return Erro("Tipo de ativo inválido.");
        if (ativo.Moeda != CambioService.MoedaBase && dto.Tipo is TipoAtivo.Fii or TipoAtivo.Bdr)
            return Erro("FIIs e BDRs são da B3: um ativo no exterior não pode ter esse tipo.");
        if (dto.PrecoManual && (dto.PrecoAtual is null || dto.PrecoAtual <= 0))
            return Erro("Informe o preço atual para usar preço manual.");

        var segmento = string.IsNullOrWhiteSpace(dto.Segmento) ? null : dto.Segmento.Trim();
        if (!SegmentosFii.Valido(segmento)) return Erro("Segmento inválido.");
        ativo.Nome = string.IsNullOrWhiteSpace(dto.Nome) ? null : dto.Nome.Trim();
        ativo.Tipo = dto.Tipo;
        ativo.Segmento = segmento;
        ativo.PrecoManual = dto.PrecoManual;
        if (dto.PrecoManual)
        {
            ativo.PrecoAnterior = ativo.PrecoAtual;
            ativo.PrecoAtual = dto.PrecoAtual;
            ativo.PrecoAtualizadoEm = DateTime.UtcNow;
            ativo.FonteCotacao = "Manual";
        }
        await _db.SaveChangesAsync(ct);

        if (!dto.PrecoManual && ativo.FonteCotacao == "Manual")
        {
            try { await _cotacoes.AtualizarAsync(new[] { ativo.Id }, ct); }
            catch (Exception) when (!ct.IsCancellationRequested) { /* mantém o último preço */ }
        }
        return NoContent();
    }

    /// <summary>Exclui o ativo com todas as operações e proventos.</summary>
    [HttpDelete("ativos/{id:int}")]
    public async Task<IActionResult> ExcluirAtivo(int id, CancellationToken ct)
    {
        var excluidos = await _db.Ativos.Where(a => a.Id == id).ExecuteDeleteAsync(ct);
        if (excluidos == 0) return NaoEncontrado();
        return NoContent();
    }

    [HttpGet("configuracao")]
    public async Task<ActionResult<ConfiguracaoCotacaoDto>> Configuracao(CancellationToken ct)
        => new ConfiguracaoCotacaoDto(await _config.ExisteAsync(CotacaoService.ChaveTokenBrapi, ct));

    [HttpPut("configuracao")]
    public async Task<IActionResult> SalvarConfiguracao(SalvarConfiguracaoCotacaoDto dto, CancellationToken ct)
    {
        if (dto.Remover)
            await _config.DefinirAsync(CotacaoService.ChaveTokenBrapi, null, ct: ct);
        else if (!string.IsNullOrWhiteSpace(dto.TokenBrapi))
        {
            var token = dto.TokenBrapi.Trim();
            if (token.Any(c => char.IsWhiteSpace(c) || char.IsControl(c))) return Erro("Token inválido.");
            await _config.DefinirAsync(CotacaoService.ChaveTokenBrapi, token, protegido: true, ct: ct);
        }
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    /* ------------------------------------------------------------------ */

    private ObjectResult? Validar(SalvarOperacaoDto dto)
    {
        if (!Enum.IsDefined(dto.Tipo)) return Erro("Tipo de operação inválido.");
        if (dto.TipoAtivo is TipoAtivo t && !Enum.IsDefined(t)) return Erro("Tipo de ativo inválido.");
        if (dto.Data > Relogio.Hoje) return Erro("A data não pode estar no futuro.");
        if (dto.Data < new DateOnly(1995, 1, 1)) return Erro("Data inválida.");
        return null;
    }

    /// <summary>
    /// Câmbio da operação: 1 em reais; o informado; ou, se vier vazio, a PTAX da data — de venda na compra (a corretora
    /// vende a moeda a você) e de compra na venda. Null se não houver como obter.
    /// </summary>
    private async Task<decimal?> CambioDaOperacaoAsync(string moeda, SalvarOperacaoDto dto, CancellationToken ct)
    {
        if (moeda == CambioService.MoedaBase) return 1m;
        if (dto.Cambio is decimal informado && informado > 0) return informado;
        var ptax = await _cambio.ObterCotacaoAsync(moeda, dto.Data, ct);
        if (ptax is null) return null;
        return dto.Tipo == TipoOperacao.Compra ? ptax.Venda : ptax.Compra;
    }

    /// <summary>Confere se nenhuma venda (em ordem cronológica) supera a quantidade em carteira.</summary>
    private async Task<bool> VendaPossivelAsync(Ativo ativo, OperacaoAtivo? nova, CancellationToken ct, int? excluir = null)
    {
        var operacoes = ativo.Id == 0
            ? new List<OperacaoAtivo>()
            : await _db.OperacoesAtivo.AsNoTracking().Where(o => o.AtivoId == ativo.Id).ToListAsync(ct);

        // Reflete as alterações ainda não salvas.
        var alteradas = _db.ChangeTracker.Entries<OperacaoAtivo>()
            .Where(e => e.Entity.AtivoId == ativo.Id && e.State == EntityState.Modified)
            .ToDictionary(e => e.Entity.Id, e => e.Entity);
        operacoes = operacoes.Where(o => o.Id != excluir)
            .Select(o => alteradas.GetValueOrDefault(o.Id) ?? o).ToList();
        if (nova is not null) operacoes.Add(nova);

        decimal quantidade = 0;
        foreach (var o in operacoes.OrderBy(o => o.Data).ThenBy(o => o.Tipo).ThenBy(o => o.Id == 0 ? int.MaxValue : o.Id))
        {
            quantidade += o.Tipo == TipoOperacao.Compra ? o.Quantidade : -o.Quantidade;
            if (quantidade < -0.0000005m) return false;
        }
        return true;
    }

    /// <summary>Sugestão pelo padrão da B3: final 11 costuma ser FII/ETF/unit; 34/35 BDR; 3–8 ações.</summary>
    private static TipoAtivo SugerirTipo(string ticker)
    {
        var numero = new string(ticker.Skip(4).TakeWhile(char.IsDigit).ToArray());
        return numero switch
        {
            "11" => TipoAtivo.Fii,
            "31" or "32" or "33" or "34" or "35" or "39" => TipoAtivo.Bdr,
            "3" or "4" or "5" or "6" or "7" or "8" => TipoAtivo.Acao,
            _ => TipoAtivo.Outro
        };
    }
}

/// <summary>
/// Reservas ("cofrinhos") que a instituição não informa pelo Open Finance, estimadas pelos lançamentos do extrato,
/// e a conferência do saldo real feita pelo usuário.
/// </summary>
[Route("api/reservas")]
public class ReservasController : BaseController
{
    private readonly ReservaService _reservas;

    public ReservasController(ReservaService reservas) => _reservas = reservas;

    [HttpGet]
    public Task<List<ReservaDto>> Listar(CancellationToken ct) => _reservas.ListarAsync(ct);

    /// <summary>Grava o saldo real da reserva; os lançamentos depois da data passam a somar a partir dele.</summary>
    [HttpPut("conferencia")]
    public async Task<IActionResult> Conferir(ConferirReservaDto dto, CancellationToken ct)
    {
        var data = dto.Data ?? Relogio.Hoje;
        if (data > Relogio.Hoje) return Erro("A data da conferência não pode estar no futuro.");
        if (string.IsNullOrWhiteSpace(dto.Nome)) return Erro("Informe o nome da reserva.");
        return await _reservas.ConferirAsync(dto.ContaId, dto.Nome, dto.Saldo, data, dto.Moeda, ct) is { } erro
            ? Erro(erro) : NoContent();
    }

    /// <summary>Apaga a conferência: o saldo volta a ser só a soma dos lançamentos do extrato.</summary>
    [HttpDelete("conferencia")]
    public async Task<IActionResult> RemoverConferencia([FromQuery] int contaId, [FromQuery] string nome,
        CancellationToken ct)
        => await _reservas.RemoverConferenciaAsync(contaId, nome ?? string.Empty, ct) ? NoContent() : NaoEncontrado();
}
