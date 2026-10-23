using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos;
using Financas.Api.Servicos.Investimentos;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Controllers;

[Route("api/orcamentos")]
public class OrcamentosController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly RelatorioService _relatorios;
    private readonly SugestaoOrcamentoService _sugestao;

    public OrcamentosController(FinancasDbContext db, RelatorioService relatorios, SugestaoOrcamentoService sugestao)
    {
        _db = db;
        _relatorios = relatorios;
        _sugestao = sugestao;
    }

    [HttpGet]
    public async Task<ActionResult<List<OrcamentoDto>>> Listar([FromQuery] int? ano, [FromQuery] int? mes, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var a = ano ?? hoje.Year;
        var m = mes ?? hoje.Month;
        if (a < 2000 || a > 2100 || m < 1 || m > 12) return Erro("Período inválido.");
        return (await _relatorios.OrcamentosDoMesAsync(a, m, ct)).OrderBy(o => o.CategoriaNome).ToList();
    }

    /// <summary>Cria ou altera o orçamento de uma categoria no mês.</summary>
    [HttpPut]
    public async Task<IActionResult> Salvar(SalvarOrcamentoDto dto, CancellationToken ct)
    {
        var categoria = await _db.Categorias.AsNoTracking().FirstOrDefaultAsync(c => c.Id == dto.CategoriaId, ct);
        if (categoria is null) return Erro("Categoria inválida.");
        if (categoria.Tipo != TipoCategoria.Despesa) return Erro("Orçamentos são definidos para categorias de despesa.");

        var o = await _db.Orcamentos.FirstOrDefaultAsync(
            x => x.CategoriaId == dto.CategoriaId && x.Ano == dto.Ano && x.Mes == dto.Mes, ct);
        if (o is null)
        {
            o = new Orcamento { CategoriaId = dto.CategoriaId, Ano = dto.Ano, Mes = dto.Mes };
            _db.Orcamentos.Add(o);
        }
        o.Valor = dto.Valor;
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpDelete("{id:int}")]
    public async Task<IActionResult> Excluir(int id, CancellationToken ct)
    {
        var apagados = await _db.Orcamentos.Where(o => o.Id == id).ExecuteDeleteAsync(ct);
        return apagados == 0 ? NaoEncontrado() : NoContent();
    }

    /// <summary>Sugestão de orçamentos por categoria para o mês (só leitura: nada é gravado). #39</summary>
    [HttpGet("sugestao")]
    public async Task<ActionResult<SugestaoOrcamentoDto>> Sugestao([FromQuery] int? ano, [FromQuery] int? mes, CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var a = ano ?? hoje.Year;
        var m = mes ?? hoje.Month;
        if (a < 2000 || a > 2100 || m < 1 || m > 12) return Erro("Período inválido.");
        return await _sugestao.SugerirAsync(a, m, ct);
    }

    /// <summary>Grava os valores escolhidos na sugestão (cria ou altera o orçamento de cada categoria no mês).</summary>
    [HttpPost("aplicar-sugestao")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> AplicarSugestao(AplicarSugestaoOrcamentoDto dto, CancellationToken ct)
    {
        var ids = dto.Itens.Select(i => i.CategoriaId).Distinct().ToList();
        if (ids.Count != dto.Itens.Count) return Erro("Categoria repetida na sugestão.");
        var validas = await _db.Categorias.AsNoTracking()
            .Where(c => ids.Contains(c.Id) && c.Tipo == TipoCategoria.Despesa).Select(c => c.Id).ToListAsync(ct);
        if (validas.Count != ids.Count) return Erro("Orçamentos são definidos para categorias de despesa.");

        var existentes = await _db.Orcamentos
            .Where(o => o.Ano == dto.Ano && o.Mes == dto.Mes && ids.Contains(o.CategoriaId)).ToListAsync(ct);
        foreach (var item in dto.Itens)
        {
            var o = existentes.FirstOrDefault(x => x.CategoriaId == item.CategoriaId);
            if (o is null)
            {
                o = new Orcamento { CategoriaId = item.CategoriaId, Ano = dto.Ano, Mes = dto.Mes };
                _db.Orcamentos.Add(o);
            }
            o.Valor = item.Valor;
        }
        await _db.SaveChangesAsync(ct);
        return new ResultadoAplicacaoDto(dto.Itens.Count);
    }

    /// <summary>Copia os orçamentos do mês anterior para o mês informado (sem sobrescrever os existentes).</summary>
    [HttpPost("copiar-mes-anterior")]
    public async Task<ActionResult<ResultadoAplicacaoDto>> CopiarMesAnterior(CopiarOrcamentoDto dto, CancellationToken ct)
    {
        var anterior = new DateOnly(dto.Ano, dto.Mes, 1).AddMonths(-1);
        var origem = await _db.Orcamentos.AsNoTracking()
            .Where(o => o.Ano == anterior.Year && o.Mes == anterior.Month).ToListAsync(ct);
        var existentes = await _db.Orcamentos.AsNoTracking()
            .Where(o => o.Ano == dto.Ano && o.Mes == dto.Mes).Select(o => o.CategoriaId).ToListAsync(ct);

        var novos = origem.Where(o => !existentes.Contains(o.CategoriaId))
            .Select(o => new Orcamento { CategoriaId = o.CategoriaId, Ano = dto.Ano, Mes = dto.Mes, Valor = o.Valor })
            .ToList();
        _db.Orcamentos.AddRange(novos);
        await _db.SaveChangesAsync(ct);
        return new ResultadoAplicacaoDto(novos.Count);
    }
}

[Route("api/metas")]
public class MetasController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly ContaService _contas;
    private readonly ReservaService _reservas;
    private readonly AssistenteMetasService _assistente;

    public MetasController(FinancasDbContext db, ContaService contas, ReservaService reservas, AssistenteMetasService assistente)
    {
        _db = db;
        _contas = contas;
        _reservas = reservas;
        _assistente = assistente;
    }

    [HttpGet]
    public async Task<ActionResult<List<MetaDto>>> Listar(CancellationToken ct) => await ListarMetasAsync(ct);

    /// <summary>#90/#149: o caixa mês a mês para o plano das metas (a divisão entre as metas é feita na tela, ADR 0035).</summary>
    [HttpGet("plano")]
    public async Task<ActionResult<PlanoMetasDto>> Plano(CancellationToken ct)
        => await _assistente.PlanejarAsync(ct);

    /// <summary>Nova ordem de prioridade: a primeira da lista é a 1ª. Metas fora da lista vão para o fim.</summary>
    [HttpPut("prioridades")]
    public async Task<IActionResult> Prioridades(PrioridadesMetasDto dto, CancellationToken ct)
    {
        if (dto.Ids.Distinct().Count() != dto.Ids.Count) return Erro("Meta repetida na ordem.");
        var metas = await _db.Metas.ToListAsync(ct);
        if (dto.Ids.Any(id => metas.All(m => m.Id != id))) return Erro("Meta inválida.");
        var proxima = 1;
        foreach (var id in dto.Ids) metas.First(m => m.Id == id).Prioridade = proxima++;
        foreach (var m in metas.Where(m => !dto.Ids.Contains(m.Id)).OrderBy(m => m.Prioridade == 0 ? int.MaxValue : m.Prioridade))
            m.Prioridade = proxima++;
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    /// <summary>Metas com o progresso calculado, em ordem de prioridade (sem ordem definida vão para o fim).</summary>
    private async Task<List<MetaDto>> ListarMetasAsync(CancellationToken ct)
    {
        var metas = await _db.Metas.AsNoTracking().Include(m => m.Conta)
            .OrderBy(m => m.Concluida).ThenBy(m => m.Prioridade == 0 ? int.MaxValue : m.Prioridade)
            .ThenBy(m => m.DataAlvo == null).ThenBy(m => m.DataAlvo).ThenBy(m => m.Nome)
            .ToListAsync(ct);
        var saldos = metas.Any(m => m.ContaId != null)
            ? await _contas.CalcularSaldosAsync(ct)
            : new Dictionary<int, decimal>();
        var reservas = metas.Any(m => m.ReservaNome != null) ? await _reservas.ListarAsync(ct) : [];
        return metas.Select(m => ParaDto(m, saldos, reservas)).ToList();
    }

    [HttpPost]
    public async Task<ActionResult<MetaDto>> Criar(SalvarMetaDto dto, CancellationToken ct)
    {
        if (dto.ContaId is { } cid && !await _db.Contas.AnyAsync(c => c.Id == cid, ct)) return Erro("Conta inválida.");
        var (erro, cofrinho) = await ValidarCofrinhoAsync(dto, null, ct);
        if (erro is not null) return Erro(erro);
        // Meta nova entra no fim da ordem de prioridade.
        var ultima = await _db.Metas.MaxAsync(x => (int?)x.Prioridade, ct) ?? 0;
        var m = new Meta { CriadoEm = DateTime.UtcNow, Prioridade = ultima + 1 };
        Aplicar(m, dto, cofrinho);
        _db.Metas.Add(m);
        await _db.SaveChangesAsync(ct);
        return await ObterDtoAsync(m.Id, ct);
    }

    [HttpPut("{id:int}")]
    public async Task<ActionResult<MetaDto>> Atualizar(int id, SalvarMetaDto dto, CancellationToken ct)
    {
        var m = await _db.Metas.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (m is null) return NaoEncontrado();
        if (dto.ContaId is { } cid && !await _db.Contas.AnyAsync(c => c.Id == cid, ct)) return Erro("Conta inválida.");
        var (erro, cofrinho) = await ValidarCofrinhoAsync(dto, m, ct);
        if (erro is not null) return Erro(erro);
        Aplicar(m, dto, cofrinho);
        await _db.SaveChangesAsync(ct);
        return await ObterDtoAsync(id, ct);
    }

    /// <summary>Soma (ou subtrai, se negativo) um valor ao progresso da meta.</summary>
    [HttpPost("{id:int}/aporte")]
    public async Task<ActionResult<MetaDto>> Aporte(int id, AporteDto dto, CancellationToken ct)
    {
        var m = await _db.Metas.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (m is null) return NaoEncontrado();
        var antes = m.ValorAtual;
        m.ValorAtual = Math.Max(0, m.ValorAtual + dto.Valor);
        if (m.ValorAtual >= m.ValorAlvo) m.Concluida = true;
        // #318: guarda o aporte com a data, para o gráfico da meta (o que de fato mudou: a retirada para no zero).
        if (m.ValorAtual != antes)
            _db.MetaAportes.Add(new MetaAporte { MetaId = id, Data = Relogio.Hoje, Valor = m.ValorAtual - antes });
        await _db.SaveChangesAsync(ct);
        return await ObterDtoAsync(id, ct);
    }

    /// <summary>#318: os aportes da meta, do mais antigo ao mais novo (o gráfico "Como está indo").</summary>
    [HttpGet("{id:int}/aportes")]
    public async Task<ActionResult<List<MetaAporteDto>>> Aportes(int id, CancellationToken ct)
    {
        if (!await _db.Metas.AnyAsync(m => m.Id == id, ct)) return NaoEncontrado();
        var aportes = await _db.MetaAportes.AsNoTracking().Where(a => a.MetaId == id)
            .OrderBy(a => a.Data).ThenBy(a => a.Id).ToListAsync(ct);
        // #359: CriadoEm é gravado em UTC (o EF devolve sem o Kind): a hora vai no fuso de Brasília.
        return aportes.Select(a => new MetaAporteDto(a.Data, a.Valor,
            Relogio.ParaHoraLocal(DateTime.SpecifyKind(a.CriadoEm, DateTimeKind.Utc)).ToString("HH:mm"))).ToList();
    }

    [HttpDelete("{id:int}")]
    public async Task<IActionResult> Excluir(int id, CancellationToken ct)
    {
        var apagadas = await _db.Metas.Where(m => m.Id == id).ExecuteDeleteAsync(ct);
        return apagadas == 0 ? NaoEncontrado() : NoContent();
    }

    private async Task<MetaDto> ObterDtoAsync(int id, CancellationToken ct)
    {
        var m = await _db.Metas.AsNoTracking().Include(x => x.Conta).FirstAsync(x => x.Id == id, ct);
        var saldos = m.ContaId is null ? new Dictionary<int, decimal>() : await _contas.CalcularSaldosAsync(ct);
        var reservas = m.ReservaNome is null ? [] : await _reservas.ListarAsync(ct);
        return ParaDto(m, saldos, reservas);
    }

    /// <summary>
    /// Cofrinho só com conta, e precisa existir nela (mesma identificação do ReservaService: conta + nome, sem
    /// diferenciar maiúsculas). Ao editar, manter o cofrinho que já estava gravado é aceito mesmo que ele tenha sumido
    /// do extrato — o cartão avisa, e a pessoa decide trocar ou não. Devolve o nome como está no extrato.
    /// </summary>
    private async Task<(string? Erro, string? Nome)> ValidarCofrinhoAsync(SalvarMetaDto dto, Meta? atual, CancellationToken ct)
    {
        var nome = string.IsNullOrWhiteSpace(dto.ReservaNome) ? null : dto.ReservaNome.Trim();
        if (nome is null) return (null, null);
        if (dto.ContaId is not int contaId) return ("Escolha a conta do cofrinho.", null);
        var reserva = (await _reservas.ListarAsync(ct))
            .FirstOrDefault(r => r.ContaId == contaId && string.Equals(r.Nome, nome, StringComparison.OrdinalIgnoreCase));
        if (reserva is not null) return (null, reserva.Nome);
        if (atual is not null && atual.ContaId == contaId
            && string.Equals(atual.ReservaNome, nome, StringComparison.OrdinalIgnoreCase)) return (null, atual.ReservaNome);
        return ("Cofrinho inválido para esta conta.", null);
    }

    private static void Aplicar(Meta m, SalvarMetaDto dto, string? cofrinho)
    {
        m.Nome = dto.Nome.Trim();
        m.ValorAlvo = dto.ValorAlvo;
        m.ValorAtual = dto.ValorAtual;
        m.DataAlvo = dto.DataAlvo;
        m.ContaId = dto.ContaId;
        m.ReservaNome = dto.ContaId is null ? null : cofrinho;
        m.Cor = dto.Cor;
        m.Icone = dto.Icone;
        m.GuardarPorMes = dto.GuardarPorMes;
        m.Concluida = dto.Concluida;
    }

    private static MetaDto ParaDto(Meta m, Dictionary<int, decimal> saldos, List<ReservaDto> reservas)
    {
        // O valor inicial (ValorAtual gravado) sempre conta. Com cofrinho, soma o saldo dele (nunca o da conta junto,
        // para não contar duas vezes); sem cofrinho, soma o saldo da conta vinculada, como antes (#89).
        ReservaDto? reserva = null;
        var saldoConta = 0m;
        if (m.ReservaNome is not null)
            reserva = reservas.FirstOrDefault(r => r.ContaId == m.ContaId
                && string.Equals(r.Nome, m.ReservaNome, StringComparison.OrdinalIgnoreCase));
        else if (m.ContaId is int cid && saldos.TryGetValue(cid, out var saldo))
            saldoConta = Math.Max(0, saldo);
        var atual = m.ValorAtual + saldoConta + (reserva?.Saldo ?? 0m);
        var percentual = m.ValorAlvo > 0 ? Math.Min(100m, Math.Round(atual / m.ValorAlvo * 100m, 1)) : 0m;

        decimal? sugestao = null;
        if (m.DataAlvo is { } alvo && !m.Concluida && atual < m.ValorAlvo)
        {
            var hoje = Relogio.Hoje;
            var meses = (alvo.Year - hoje.Year) * 12 + alvo.Month - hoje.Month;
            sugestao = Math.Round((m.ValorAlvo - atual) / Math.Max(1, meses), 2);
        }

        return new MetaDto(m.Id, m.Nome, m.ValorAlvo, atual, m.DataAlvo, m.ContaId, m.Conta?.Nome, m.Cor,
            m.Concluida || atual >= m.ValorAlvo, percentual, sugestao, m.ValorAtual, saldoConta,
            reserva?.Nome ?? m.ReservaNome, reserva?.Saldo,
            reserva is not null && (reserva.PodeTerSaldoAnterior || reserva.ConferenciaAntiga),
            m.ReservaNome is not null && reserva is null, m.Prioridade, m.Icone, m.GuardarPorMes);
    }
}

[Route("api/contas-agendadas")]
public class ContasAgendadasController : BaseController
{
    private readonly FinancasDbContext _db;
    private readonly RelatorioService _relatorios;
    private readonly CategorizacaoService _categorizacao;
    private readonly SugestaoContaAgendadaService _sugestoes;
    private readonly ContaAgendadaService _cobrancas;

    public ContasAgendadasController(FinancasDbContext db, RelatorioService relatorios, CategorizacaoService categorizacao,
        SugestaoContaAgendadaService sugestoes, ContaAgendadaService cobrancas)
    {
        _cobrancas = cobrancas;
        _db = db;
        _relatorios = relatorios;
        _categorizacao = categorizacao;
        _sugestoes = sugestoes;
    }

    [HttpGet]
    public async Task<ActionResult<List<ContaAgendadaDto>>> Listar(CancellationToken ct)
        => await _relatorios.ContasAgendadasAsync(null, ct);

    /// <summary>#158: avisos das recorrentes (não veio, mudou de cartão, aumentou, cartão arquivado). Só lê.</summary>
    [HttpGet("avisos")]
    public async Task<ActionResult<List<AvisoContaAgendadaDto>>> Avisos(CancellationToken ct)
        => await _cobrancas.AvisosAsync(ct);

    /// <summary>Pagamentos mensais do extrato que ainda não estão cadastrados (só sugestão; nada é gravado).</summary>
    [HttpGet("sugestoes")]
    public async Task<ActionResult<List<SugestaoContaAgendadaDto>>> Sugestoes(CancellationToken ct)
        => await _sugestoes.SugerirAsync(ct);

    [HttpPost]
    public async Task<IActionResult> Criar(SalvarContaAgendadaDto dto, CancellationToken ct)
    {
        var problema = await ValidarAsync(dto, ct);
        if (problema is not null) return Erro(problema);
        var c = new ContaAgendada { CriadoEm = DateTime.UtcNow };
        Aplicar(c, dto);
        _db.ContasAgendadas.Add(c);
        await _db.SaveChangesAsync(ct);
        if (await _cobrancas.ConciliarAsync(ct) > 0) await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    [HttpPut("{id:int}")]
    public async Task<IActionResult> Atualizar(int id, SalvarContaAgendadaDto dto, CancellationToken ct)
    {
        var c = await _db.ContasAgendadas.FirstOrDefaultAsync(x => x.Id == id, ct);
        if (c is null) return NaoEncontrado();
        var problema = await ValidarAsync(dto, ct);
        if (problema is not null) return Erro(problema);
        Aplicar(c, dto);
        await _db.SaveChangesAsync(ct);
        if (await _cobrancas.ConciliarAsync(ct) > 0) await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    /// <summary>
    /// #205: "é a mesma": a sugestão do extrato é esta conta a pagar (cadastrada à mão ou com outro nome). Grava a chave
    /// da descrição do banco na conta, e a sugestão não volta mais.
    /// </summary>
    [HttpPost("{id:int}/ligar-extrato")]
    public async Task<IActionResult> LigarExtrato(int id, LigarExtratoContaAgendadaDto dto, CancellationToken ct)
    {
        var chave = dto.ChaveExtrato.Trim();
        if (chave.Length == 0) return Erro("Informe a sugestão do extrato.");
        var alteradas = await _db.ContasAgendadas.Where(c => c.Id == id)
            .ExecuteUpdateAsync(s => s.SetProperty(c => c.ChaveExtrato, chave), ct);
        return alteradas == 0 ? NaoEncontrado() : NoContent();
    }

    /// <summary>#205: o "Desfazer" do "é a mesma": a conta a pagar volta a não ter ligação, e a sugestão pode voltar.</summary>
    [HttpDelete("{id:int}/ligar-extrato")]
    public async Task<IActionResult> DesligarExtrato(int id, CancellationToken ct)
    {
        var alteradas = await _db.ContasAgendadas.Where(c => c.Id == id)
            .ExecuteUpdateAsync(s => s.SetProperty(c => c.ChaveExtrato, (string?)null), ct);
        return alteradas == 0 ? NaoEncontrado() : NoContent();
    }

    [HttpDelete("{id:int}")]
    public async Task<IActionResult> Excluir(int id, CancellationToken ct)
    {
        var apagadas = await _db.ContasAgendadas.Where(c => c.Id == id).ExecuteDeleteAsync(ct);
        return apagadas == 0 ? NaoEncontrado() : NoContent();
    }

    /// <summary>
    /// Marca o vencimento atual como pago e avança para o próximo (ou desativa, se for única).
    /// Opcionalmente lança a despesa, quando a conta vinculada é manual.
    /// </summary>
    [HttpPost("{id:int}/pagar")]
    public async Task<IActionResult> Pagar(int id, PagarContaAgendadaDto dto, CancellationToken ct)
    {
        var c = await _db.ContasAgendadas.Include(x => x.Conta).FirstOrDefaultAsync(x => x.Id == id, ct);
        if (c is null) return NaoEncontrado();
        var dataPagamento = dto.Data ?? Relogio.Hoje;

        if (dto.LancarTransacao)
        {
            if (c.Conta is null || c.Conta.Origem != OrigemConta.Manual)
                return Erro("Para lançar a despesa, vincule uma conta manual. Em contas do Open Finance o pagamento chega pela sincronização.");
            await _categorizacao.CarregarAsync(ct);
            var t = new Transacao
            {
                ContaId = c.Conta.Id,
                Data = dataPagamento,
                Descricao = c.Descricao,
                Valor = -c.Valor,
                Status = StatusTransacao.Efetivada,
                CategoriaId = c.CategoriaId,
                CategoriaManual = c.CategoriaId is not null,
                CriadoEm = DateTime.UtcNow,
                AtualizadoEm = DateTime.UtcNow
            };
            _categorizacao.AplicarTipo(t);
            _db.Transacoes.Add(t);
        }

        c.UltimoPagamentoEm = dataPagamento;
        if (RelatorioService.ProximoVencimentoApos(c.ProximoVencimento, c.Recorrencia) is { } proximo)
        {
            c.ProximoVencimento = proximo;
            // #69: passou da data limite, a conta é encerrada (sai do Painel; continua na lista como "encerrada").
            if (c.DataLimite is { } limite && proximo > limite) c.Ativa = false;
        }
        else c.Ativa = false;
        await _db.SaveChangesAsync(ct);
        return NoContent();
    }

    private async Task<string?> ValidarAsync(SalvarContaAgendadaDto dto, CancellationToken ct)
    {
        if (dto.CategoriaId is { } cat && !await _db.Categorias.AnyAsync(c => c.Id == cat, ct)) return "Categoria inválida.";
        if (dto.ContaId is { } conta && !await _db.Contas.AnyAsync(c => c.Id == conta, ct)) return "Conta inválida.";
        // Conta encerrada (inativa) pode ser editada sem mexer nas datas; só a ativa precisa de um vencimento até o limite.
        if (dto.Ativa && dto.Recorrencia != Recorrencia.Unica && dto.DataLimite is { } limite && limite < dto.ProximoVencimento)
            return $"A data limite (até {limite:dd/MM/yyyy}) não pode ser antes do próximo vencimento ({dto.ProximoVencimento:dd/MM/yyyy}).";
        return null;
    }

    private static void Aplicar(ContaAgendada c, SalvarContaAgendadaDto dto)
    {
        c.Descricao = dto.Descricao.Trim();
        c.Valor = dto.Valor;
        c.ProximoVencimento = dto.ProximoVencimento;
        c.Recorrencia = dto.Recorrencia;
        c.CategoriaId = dto.CategoriaId;
        c.ContaId = dto.ContaId;
        c.Ativa = dto.Ativa;
        c.DataLimite = dto.Recorrencia == Recorrencia.Unica ? null : dto.DataLimite;
        // #158: o texto da cobrança fica sem acento e em maiúsculas, como a comparação com o extrato.
        c.FormaCobranca = Enum.IsDefined(dto.FormaCobranca) ? dto.FormaCobranca : FormaCobranca.AMao;
        c.TextoCobranca = CategorizacaoService.Normalizar(dto.TextoCobranca) is { Length: > 0 } texto ? texto : null;
        c.Observacao = string.IsNullOrWhiteSpace(dto.Observacao) ? null : dto.Observacao.Trim();
        // #205: veio da sugestão do extrato. Na edição o campo vem vazio e a ligação que já existe fica.
        if (!string.IsNullOrWhiteSpace(dto.ChaveExtrato)) c.ChaveExtrato = dto.ChaveExtrato.Trim();
    }
}
