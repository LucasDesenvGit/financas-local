using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos;
using Financas.Api.Servicos.Pluggy;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Controllers;

[Route("api/open-finance")]
public class OpenFinanceController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly ConfiguracaoService _config;
    private readonly SincronizacaoService _sincronizacao;
    private readonly ContaService _contas;
    private readonly CategorizacaoService _categorizacao;
    private readonly AuditoriaService _auditoria;
    private readonly ParametroService _parametros;

    public OpenFinanceController(FinancasDbContext db, ConfiguracaoService config, SincronizacaoService sincronizacao,
        ContaService contas, CategorizacaoService categorizacao, AuditoriaService auditoria, ParametroService parametros)
    {
        _db = db;
        _config = config;
        _sincronizacao = sincronizacao;
        _contas = contas;
        _categorizacao = categorizacao;
        _auditoria = auditoria;
        _parametros = parametros;
    }

    /* ---------------------------- Credenciais ---------------------------- */

    [HttpGet("configuracao")]
    public async Task<ActionResult<ConfiguracaoPluggyDto>> ObterConfiguracao(CancellationToken ct)
        => new ConfiguracaoPluggyDto(
            await _config.ObterAsync(ConfiguracaoService.PluggyClientId, ct),
            await _config.ExisteAsync(ConfiguracaoService.PluggyClientSecret, ct),
            await _config.ObterInteiroAsync(ConfiguracaoService.IntervaloHoras, 6, ct),
            await _config.ObterInteiroAsync(ConfiguracaoService.DiasRetroativos, 30, ct),
            await _config.ObterBoolAsync(ConfiguracaoService.SincronizacaoHabilitada, true, ct),
            await _config.ObterAsync(TransferenciasService.ChaveMeuNome, ct));

    /// <summary>
    /// #172 (ADR 0031): o seu nome como aparece nos Pix. Sem titular informado pelo banco, Pix que cita esse nome é entre
    /// contas suas. Reclassifica as transferências na hora.
    /// </summary>
    [HttpPut("meu-nome")]
    public async Task<IActionResult> SalvarMeuNome(MeuNomeDto dto, [FromServices] TransferenciasService transferencias,
        CancellationToken ct)
    {
        var nome = string.IsNullOrWhiteSpace(dto.Nome) ? null : dto.Nome.Trim();
        if (nome is not null && TransferenciasService.PalavrasDoNome(nome) is null)
            return Erro("Informe pelo menos o primeiro e o último nome, como aparecem nos Pix.");
        await _config.DefinirAsync(TransferenciasService.ChaveMeuNome, nome, ct: ct);
        await _db.SaveChangesAsync(ct);
        if (await transferencias.ReclassificarAsync(ct) > 0) await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    /// <summary>Salva as credenciais. O Client Secret é gravado criptografado e nunca é devolvido pela API.</summary>
    [HttpPut("configuracao")]
    public async Task<IActionResult> SalvarConfiguracao(SalvarConfiguracaoPluggyDto dto, CancellationToken ct)
    {
        await _config.DefinirAsync(ConfiguracaoService.PluggyClientId, dto.ClientId.Trim(), ct: ct);
        if (!string.IsNullOrWhiteSpace(dto.ClientSecret))
        {
            await _config.DefinirAsync(ConfiguracaoService.PluggyClientSecret, dto.ClientSecret.Trim(), protegido: true, ct: ct);
            _auditoria.Registrar("CredencialPluggyAlterada", User.Identity?.Name);
        }
        if (dto.IntervaloHoras is { } intervalo)
            await _config.DefinirAsync(ConfiguracaoService.IntervaloHoras, intervalo.ToString(), ct: ct);
        if (dto.DiasRetroativos is { } dias)
            await _config.DefinirAsync(ConfiguracaoService.DiasRetroativos, dias.ToString(), ct: ct);
        if (dto.Habilitada is { } habilitada)
            await _config.DefinirAsync(ConfiguracaoService.SincronizacaoHabilitada, habilitada ? "true" : "false", ct: ct);
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpDelete("configuracao/secret")]
    public async Task<IActionResult> RemoverSecret(CancellationToken ct)
    {
        await _db.Configuracoes.Where(c => c.Chave == ConfiguracaoService.PluggyClientSecret).ExecuteDeleteAsync(ct);
        _auditoria.Registrar("CredencialPluggyRemovida", User.Identity?.Name);
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpPost("testar")]
    public async Task<ActionResult<MensagemDto>> Testar(CancellationToken ct)
    {
        var (ok, mensagem) = await _sincronizacao.TestarCredenciaisAsync(ct);
        if (!ok) return Erro(mensagem);
        return new MensagemDto(mensagem);
    }

    /* ---------------------------- Conexões ---------------------------- */

    [HttpGet("conexoes")]
    public async Task<ActionResult<List<ConexaoDto>>> ListarConexoes(CancellationToken ct)
    {
        var conexoes = await _db.ConexoesBancarias.AsNoTracking()
            .Select(c => new ConexaoDto(c.Id, c.PluggyItemId, c.NomeInstituicao, c.CorPrimaria, c.Status, c.StatusExecucao,
                c.UltimaAtualizacaoInstituicao, c.ConsentimentoExpiraEm, c.UltimaSincronizacaoEm, c.UltimoErro, c.Ativa,
                c.Contas.Count, null))
            .ToListAsync(ct);
        // #170: o banco (deduzido das contas), e não "MeuPluggy", que é o nome do conector.
        var nomes = await _contas.NomesDasConexoesAsync(ct);
        return conexoes.Select(c => c with { NomeExibicao = nomes.GetValueOrDefault(c.Id) })
            .OrderBy(c => c.NomeExibicao).ToList();
    }

    /// <summary>Cadastra um item do Meu Pluggy (valida na Pluggy antes) e já faz a primeira sincronização.</summary>
    [HttpPost("conexoes")]
    public async Task<ActionResult<ResultadoSincronizacaoDto>> AdicionarConexao(AdicionarConexaoDto dto, CancellationToken ct)
    {
        var itemId = dto.ItemId.Trim().ToLowerInvariant();
        if (await _db.ConexoesBancarias.AnyAsync(c => c.PluggyItemId == itemId, ct))
            return Erro("Esta conexão já está cadastrada.", StatusCodes.Status409Conflict);

        PluggyItem item;
        try
        {
            item = await _sincronizacao.ConsultarItemAsync(itemId, ct);
        }
        catch (PluggyException ex)
        {
            return Erro(ex.Message);
        }

        var conexao = new ConexaoBancaria
        {
            PluggyItemId = item.Id.Length is > 0 and <= 64 ? item.Id.ToLowerInvariant() : itemId,
            NomeInstituicao = item.Connector?.Name is { } nome ? (nome.Length > 150 ? nome[..150] : nome) : null,
            Status = item.Status,
            Ativa = true,
            CriadoEm = DateTime.UtcNow
        };
        _db.ConexoesBancarias.Add(conexao);
        _auditoria.Registrar("ConexaoAdicionada", User.Identity?.Name);
        await _db.SaveChangesAsync(ct);

        return await _sincronizacao.SincronizarAsync(conexao.Id, "Manual", ct);
    }

    [HttpPut("conexoes/{id:int}/ativa")]
    public async Task<IActionResult> AlterarAtiva(int id, [FromQuery] bool ativa, CancellationToken ct)
    {
        var alteradas = await _db.ConexoesBancarias.Where(c => c.Id == id)
            .ExecuteUpdateAsync(s => s.SetProperty(c => c.Ativa, ativa), ct);
        return alteradas == 0 ? NaoEncontrado() : NoContent();
    }

    /// <summary>
    /// Remove a conexão e todos os dados importados por ela (contas, transações, faturas, investimentos, empréstimos,
    /// o histórico dos investimentos e o registro das sincronizações). O banco apaga em cascata o que tem chave estrangeira;
    /// o histórico (sem chave estrangeira, de propósito) e o registro (ON DELETE SET NULL) são apagados aqui (#575).
    /// </summary>
    [HttpDelete("conexoes/{id:int}")]
    public async Task<IActionResult> RemoverConexao(int id, CancellationToken ct)
    {
        if (SincronizacaoService.EmExecucao) return Erro("Aguarde a sincronização em andamento terminar.", StatusCodes.Status409Conflict);
        var conexao = await _db.ConexoesBancarias.FirstOrDefaultAsync(c => c.Id == id, ct);
        if (conexao is null) return NaoEncontrado();

        await using var transacao = await _db.Database.BeginTransactionAsync(ct);
        var contaIds = await _db.Contas.Where(c => c.ConexaoBancariaId == id).Select(c => c.Id).ToListAsync(ct);
        await _contas.DesvincularFaturasAsync(contaIds, ct);
        await _db.HistoricosInvestimento.Where(h => h.ConexaoBancariaId == id).ExecuteDeleteAsync(ct);
        await _db.SincronizacaoLogs.Where(l => l.ConexaoBancariaId == id).ExecuteDeleteAsync(ct);
        await _db.ConexoesBancarias.Where(c => c.Id == id).ExecuteDeleteAsync(ct);
        _auditoria.Registrar("ConexaoRemovida", User.Identity?.Name);
        await _db.SaveChangesAsync(ct);
        await transacao.CommitAsync(ct);
        return NoContent();
    }

    /* ---------------------------- Sincronização ---------------------------- */

    [HttpPost("sincronizar")]
    public async Task<ActionResult<ResultadoSincronizacaoDto>> Sincronizar([FromQuery] int? conexaoId, CancellationToken ct)
        => await _sincronizacao.SincronizarAsync(conexaoId, "Manual", ct);

    [HttpGet("sincronizacao/estado")]
    public IActionResult EstadoSincronizacao() => Ok(new { emExecucao = SincronizacaoService.EmExecucao });

    /// <summary>
    /// UX-8: status leve para a barra superior (última sincronização, se há uma rodando e conexões que pedem atenção).
    /// Lê só a configuração e as conexões ativas, para poder ser chamado de qualquer tela sem pesar.
    /// </summary>
    [HttpGet("status")]
    public async Task<ActionResult<StatusSincronizacaoDto>> Status(CancellationToken ct)
    {
        var ultima = await _config.ObterDataAsync(ConfiguracaoService.UltimaSincronizacaoGeral, ct);
        var limiteConsentimento = DateTime.UtcNow.AddDays(
            await _parametros.ObterInteiroAsync(ConfiguracaoService.DiasAvisoConsentimento, ct));
        var conexoes = await _db.ConexoesBancarias.AsNoTracking()
            .Where(c => c.Ativa)
            .Select(c => new { c.UltimoErro, c.Status, c.ConsentimentoExpiraEm })
            .ToListAsync(ct);
        // Mesma regra dos alertas do Painel: erro na última sincronização ou instituição pedindo ação no Meu Pluggy.
        var comErro = conexoes.Count(c => !string.IsNullOrEmpty(c.UltimoErro) || c.Status is "LOGIN_ERROR" or "WAITING_USER_ACTION");
        var consentimentosVencendo = conexoes.Count(c => c.ConsentimentoExpiraEm is { } exp && exp <= limiteConsentimento);
        return new StatusSincronizacaoDto(
            ultima is { } u ? DateTime.SpecifyKind(u.ToUniversalTime(), DateTimeKind.Utc) : null,
            SincronizacaoService.EmExecucao, conexoes.Count, comErro, consentimentosVencendo);
    }

    [HttpGet("logs")]
    public async Task<ActionResult<List<SincronizacaoLogDto>>> Logs(CancellationToken ct)
    {
        var logs = await _db.SincronizacaoLogs.AsNoTracking()
            .OrderByDescending(l => l.Id)
            .Take(50)
            .Select(l => new SincronizacaoLogDto(l.Id, l.ConexaoBancariaId,
                l.ConexaoBancaria != null ? l.ConexaoBancaria.NomeInstituicao : null,
                l.Origem, l.IniciadoEm, l.FinalizadoEm, l.Sucesso, l.TransacoesNovas, l.TransacoesAtualizadas, l.Mensagem))
            .ToListAsync(ct);
        // #170: o mesmo nome da lista de conexões.
        var nomes = await _contas.NomesDasConexoesAsync(ct);
        return logs.Select(l => l.ConexaoBancariaId is int id && nomes.TryGetValue(id, out var nome)
            ? l with { NomeInstituicao = nome } : l).ToList();
    }

    /* ---------------------------- Categorias da Pluggy ---------------------------- */

    [HttpGet("categorias")]
    public async Task<ActionResult<List<CategoriaPluggyDto>>> Categorias(CancellationToken ct)
    {
        var usadas = await _db.Transacoes.AsNoTracking()
            .Where(t => t.PluggyCategoriaId != null)
            .GroupBy(t => t.PluggyCategoriaId!)
            .Select(g => new { Id = g.Key, Quantidade = g.Count() })
            .ToDictionaryAsync(x => x.Id, x => x.Quantidade, ct);

        var lista = await _db.CategoriasPluggy.AsNoTracking()
            .Select(c => new { c.PluggyCategoriaId, c.Descricao, c.PaiDescricao, c.CategoriaId,
                CategoriaNome = c.Categoria != null ? c.Categoria.Nome : null })
            .ToListAsync(ct);

        return lista
            .Select(c => new CategoriaPluggyDto(c.PluggyCategoriaId, c.Descricao, c.PaiDescricao, c.CategoriaId,
                c.CategoriaNome, usadas.GetValueOrDefault(c.PluggyCategoriaId)))
            .OrderByDescending(c => c.QuantidadeTransacoes).ThenBy(c => c.PaiDescricao).ThenBy(c => c.Descricao)
            .ToList();
    }

    [HttpPut("categorias/{pluggyCategoriaId}")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> MapearCategoria(string pluggyCategoriaId,
        MapearCategoriaPluggyDto dto, [FromServices] TransferenciasService transferencias, CancellationToken ct)
    {
        var registro = await _db.CategoriasPluggy.FirstOrDefaultAsync(c => c.PluggyCategoriaId == pluggyCategoriaId, ct);
        if (registro is null) return NaoEncontrado();
        if (dto.CategoriaId is { } cid && !await _db.Categorias.AnyAsync(c => c.Id == cid, ct))
            return Erro("Categoria inválida.");

        registro.CategoriaId = dto.CategoriaId;
        await _db.SaveChangesAsync(ct);

        var alteradas = 0;
        if (dto.AplicarExistentes)
        {
            await _categorizacao.CarregarAsync(ct);
            var subcategorias = await _db.CategoriasPluggy.AsNoTracking()
                .Where(c => c.PaiPluggyId == pluggyCategoriaId)
                .Select(c => c.PluggyCategoriaId).ToListAsync(ct);
            subcategorias.Add(pluggyCategoriaId);

            var transacoes = await _db.Transacoes
                .Where(t => !t.CategoriaManual && t.PluggyCategoriaId != null && subcategorias.Contains(t.PluggyCategoriaId))
                .ToListAsync(ct);
            foreach (var t in transacoes)
            {
                var nova = _categorizacao.Resolver(t);
                if (nova == t.CategoriaId) continue;
                t.CategoriaId = nova;
                _categorizacao.AplicarTipo(t);
                t.AtualizadoEm = DateTime.UtcNow;
                alteradas++;
            }
            await _db.SaveChangesAsync(ct);
            // Transferência só fica fora dos relatórios quando é entre contas suas (ADR 0031).
            if (await transferencias.ReclassificarAsync(ct) > 0) await _db.SaveChangesAsync(ct);
        }
        return new ResultadoAplicacaoDto(alteradas);
    }
}
