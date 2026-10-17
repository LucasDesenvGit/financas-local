using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos.Investimentos;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos.Pluggy;

/// <summary>
/// Sincroniza conexões do Open Finance (via Pluggy) com o banco local:
/// conexão (item) → contas → faturas → transações → investimentos.
/// </summary>
public class SincronizacaoService
{
    private static readonly SemaphoreSlim Trava = new(1, 1);

    private readonly FinancasDbContext _db;
    private readonly PluggyClient _pluggy;
    private readonly ConfiguracaoService _config;
    private readonly CategorizacaoService _categorizacao;
    private readonly ContaService _contas;
    private readonly ILogger<SincronizacaoService> _logger;

    public SincronizacaoService(FinancasDbContext db, PluggyClient pluggy, ConfiguracaoService config,
        CategorizacaoService categorizacao, ContaService contas, ILogger<SincronizacaoService> logger)
    {
        _db = db;
        _pluggy = pluggy;
        _config = config;
        _categorizacao = categorizacao;
        _contas = contas;
        _logger = logger;
    }

    public static bool EmExecucao => Trava.CurrentCount == 0;

    /// <summary>Testa as credenciais configuradas.</summary>
    public async Task<(bool Sucesso, string Mensagem)> TestarCredenciaisAsync(CancellationToken ct)
    {
        var (clientId, secret) = await ObterCredenciaisAsync(ct);
        if (clientId is null || secret is null) return (false, "Client ID e Client Secret ainda não configurados.");
        try
        {
            await _pluggy.CriarSessaoAsync(clientId, secret, ct);
            return (true, "Conexão com a Pluggy realizada com sucesso.");
        }
        catch (PluggyException ex)
        {
            return (false, ex.Message);
        }
    }

    /// <summary>Busca os dados de um item para validar antes de cadastrar a conexão.</summary>
    public async Task<PluggyItem> ConsultarItemAsync(string itemId, CancellationToken ct)
    {
        var (clientId, secret) = await ObterCredenciaisAsync(ct);
        if (clientId is null || secret is null)
            throw new PluggyException("Configure o Client ID e o Client Secret da Pluggy antes de adicionar conexões.");
        var sessao = await _pluggy.CriarSessaoAsync(clientId, secret, ct);
        return await _pluggy.ObterItemAsync(sessao, itemId, ct);
    }

    public async Task<ResultadoSincronizacaoDto> SincronizarAsync(int? conexaoId, string origem, CancellationToken ct)
    {
        if (!await Trava.WaitAsync(0, ct))
            return new ResultadoSincronizacaoDto(false, 0, 0, "Já existe uma sincronização em andamento.");

        try
        {
            var (clientId, secret) = await ObterCredenciaisAsync(ct);
            if (clientId is null || secret is null)
                return new ResultadoSincronizacaoDto(false, 0, 0, "Configure o Client ID e o Client Secret da Pluggy.");

            var conexoes = await _db.ConexoesBancarias
                .Where(c => c.Ativa && (conexaoId == null || c.Id == conexaoId))
                .ToListAsync(ct);
            if (conexoes.Count == 0)
                return new ResultadoSincronizacaoDto(false, 0, 0, "Nenhuma conexão ativa para sincronizar.");

            PluggySessao sessao;
            try
            {
                sessao = await _pluggy.CriarSessaoAsync(clientId, secret, ct);
            }
            catch (PluggyException ex)
            {
                _db.SincronizacaoLogs.Add(new SincronizacaoLog
                {
                    Origem = origem, IniciadoEm = DateTime.UtcNow, FinalizadoEm = DateTime.UtcNow,
                    Sucesso = false, Mensagem = Limitar(ex.Message, 2000)
                });
                await _db.SaveChangesAsync(ct);
                return new ResultadoSincronizacaoDto(false, 0, 0, ex.Message);
            }

            try
            {
                await AtualizarCategoriasPluggyAsync(sessao, ct);
            }
            catch (Exception ex) when (!ct.IsCancellationRequested)
            {
                // Não impede a sincronização das transações.
                _logger.LogWarning(ex, "Não foi possível atualizar a lista de categorias da Pluggy.");
                _db.ChangeTracker.Clear();
            }

            await _categorizacao.CarregarAsync(ct);

            int novas = 0, atualizadas = 0;
            var erros = new List<string>();
            foreach (var conexao in conexoes)
            {
                var r = await SincronizarConexaoAsync(sessao, conexao.Id, origem, ct);
                novas += r.TransacoesNovas;
                atualizadas += r.TransacoesAtualizadas;
                if (!r.Sucesso && r.Mensagem is not null) erros.Add(r.Mensagem);
            }

            await _config.DefinirAsync(ConfiguracaoService.UltimaSincronizacaoGeral, DateTime.UtcNow.ToString("O"), ct: ct);
            await _db.SaveChangesAsync(ct);

            return new ResultadoSincronizacaoDto(erros.Count == 0, novas, atualizadas,
                erros.Count == 0 ? "Sincronização concluída." : string.Join(" | ", erros));
        }
        finally
        {
            Trava.Release();
        }
    }

    private async Task<ResultadoSincronizacaoDto> SincronizarConexaoAsync(PluggySessao sessao, int conexaoId,
        string origem, CancellationToken ct)
    {
        _db.ChangeTracker.Clear();
        var log = new SincronizacaoLog { ConexaoBancariaId = conexaoId, Origem = origem, IniciadoEm = DateTime.UtcNow };
        _db.SincronizacaoLogs.Add(log);
        await _db.SaveChangesAsync(ct);
        var logId = log.Id;

        int novas = 0, atualizadas = 0;
        try
        {
            var conexao = await _db.ConexoesBancarias.FirstAsync(c => c.Id == conexaoId, ct);

            var item = await _pluggy.ObterItemAsync(sessao, conexao.PluggyItemId, ct);
            conexao.NomeInstituicao = Limitar(item.Connector?.Name, 150) ?? conexao.NomeInstituicao;
            conexao.CorPrimaria = NormalizarCor(item.Connector?.PrimaryColor) ?? conexao.CorPrimaria;
            conexao.Status = Limitar(item.Status, 40);
            conexao.StatusExecucao = Limitar(item.ExecutionStatus, 60);
            conexao.UltimaAtualizacaoInstituicao = item.LastUpdatedAt?.UtcDateTime;
            conexao.ConsentimentoExpiraEm = item.ConsentExpiresAt?.UtcDateTime;
            await _db.SaveChangesAsync(ct);

            var diasRetroativos = await _config.ObterInteiroAsync(ConfiguracaoService.DiasRetroativos, 30, ct);
            var diasPrimeiraCarga = await _config.ObterInteiroAsync(ConfiguracaoService.DiasPrimeiraCarga, 365, ct);
            var hoje = Relogio.Hoje;

            var contasPluggy = await _pluggy.ListarContasAsync(sessao, conexao.PluggyItemId, ct);
            foreach (var pc in contasPluggy)
            {
                var conta = await _db.Contas.FirstOrDefaultAsync(c => c.PluggyAccountId == pc.Id, ct);
                var primeiraCarga = conta is null;
                if (conta is null)
                {
                    conta = new Conta
                    {
                        PluggyAccountId = pc.Id,
                        ConexaoBancariaId = conexao.Id,
                        Origem = OrigemConta.OpenFinance,
                        Nome = Limitar(pc.MarketingName ?? pc.Name, 120) ?? "Conta",
                        Cor = conexao.CorPrimaria is { Length: 7 } cor ? cor : "#0f766e",
                        CriadoEm = DateTime.UtcNow
                    };
                    _db.Contas.Add(conta);
                }

                var ehCartao = string.Equals(pc.Type, "CREDIT", StringComparison.OrdinalIgnoreCase);
                conta.Tipo = ehCartao ? TipoConta.CartaoCredito
                    : string.Equals(pc.Subtype, "SAVINGS_ACCOUNT", StringComparison.OrdinalIgnoreCase) ? TipoConta.Poupanca
                    : TipoConta.ContaCorrente;
                conta.Numero = Limitar(pc.Number, 60);
                conta.Moeda = NormalizarMoeda(pc.CurrencyCode);
                var saldo = Arredondar(pc.Balance ?? 0);
                conta.Saldo = ehCartao ? -saldo : saldo;
                if (ehCartao && pc.CreditData is not null)
                {
                    conta.LimiteCredito = ArredondarNulo(pc.CreditData.CreditLimit);
                    conta.LimiteDisponivel = ArredondarNulo(pc.CreditData.AvailableCreditLimit);
                    if (pc.CreditData.BalanceCloseDate is { } fech) conta.DiaFechamento = (byte)Relogio.ParaDataLocal(fech).Day;
                    if (pc.CreditData.BalanceDueDate is { } venc) conta.DiaVencimento = (byte)Relogio.ParaDataLocal(venc).Day;
                }
                conta.AtualizadoEm = DateTime.UtcNow;
                await _db.SaveChangesAsync(ct);

                var mapaFaturas = ehCartao
                    ? await SincronizarFaturasAsync(sessao, conta, ct)
                    : new Dictionary<string, int>();

                if (!primeiraCarga)
                    primeiraCarga = !await _db.Transacoes.AnyAsync(t => t.ContaId == conta.Id, ct);
                var dataInicial = hoje.AddDays(-(primeiraCarga ? diasPrimeiraCarga : diasRetroativos));

                var (n, a) = await SincronizarTransacoesAsync(sessao, conta, ehCartao, dataInicial, mapaFaturas, ct);
                novas += n;
                atualizadas += a;
            }

            await SincronizarInvestimentosAsync(sessao, conexao, ct);
            await SincronizarEmprestimosAsync(sessao, conexao, ct);

            conexao.UltimaSincronizacaoEm = DateTime.UtcNow;
            conexao.UltimoErro = item.Error?.Message is { Length: > 0 } erroItem
                ? Limitar($"Instituição: {erroItem}", 2000)
                : null;

            var logDb = await _db.SincronizacaoLogs.FirstAsync(l => l.Id == logId, ct);
            logDb.Sucesso = true;
            logDb.FinalizadoEm = DateTime.UtcNow;
            logDb.TransacoesNovas = novas;
            logDb.TransacoesAtualizadas = atualizadas;
            logDb.Mensagem = $"{contasPluggy.Count} conta(s) processada(s).";
            await _db.SaveChangesAsync(ct);

            return new ResultadoSincronizacaoDto(true, novas, atualizadas, null);
        }
        catch (Exception ex) when (!ct.IsCancellationRequested)
        {
            _logger.LogError(ex, "Erro ao sincronizar a conexão {ConexaoId}.", conexaoId);
            _db.ChangeTracker.Clear();
            var mensagem = Limitar(ex is PluggyException ? ex.Message : $"Erro interno: {ex.Message}", 2000);

            await _db.ConexoesBancarias.Where(c => c.Id == conexaoId)
                .ExecuteUpdateAsync(s => s.SetProperty(c => c.UltimoErro, mensagem), ct);
            await _db.SincronizacaoLogs.Where(l => l.Id == logId)
                .ExecuteUpdateAsync(s => s
                    .SetProperty(l => l.Sucesso, false)
                    .SetProperty(l => l.FinalizadoEm, (DateTime?)DateTime.UtcNow)
                    .SetProperty(l => l.TransacoesNovas, novas)
                    .SetProperty(l => l.TransacoesAtualizadas, atualizadas)
                    .SetProperty(l => l.Mensagem, mensagem), ct);

            return new ResultadoSincronizacaoDto(false, novas, atualizadas, mensagem);
        }
    }

    private async Task<Dictionary<string, int>> SincronizarFaturasAsync(PluggySessao sessao, Conta conta, CancellationToken ct)
    {
        var faturasPluggy = await _pluggy.ListarFaturasAsync(sessao, conta.PluggyAccountId!, ct);
        var existentes = await _db.FaturasCartao.Where(f => f.ContaId == conta.Id && f.PluggyBillId != null)
            .ToDictionaryAsync(f => f.PluggyBillId!, ct);

        foreach (var pf in faturasPluggy)
        {
            if (string.IsNullOrWhiteSpace(pf.Id) || pf.DueDate is null) continue;
            if (!existentes.TryGetValue(pf.Id, out var fatura))
            {
                fatura = new FaturaCartao { ContaId = conta.Id, PluggyBillId = pf.Id };
                _db.FaturasCartao.Add(fatura);
                existentes[pf.Id] = fatura;
            }
            fatura.DataVencimento = Relogio.ParaDataLocal(pf.DueDate.Value);
            fatura.DataFechamento = pf.BillClosingDate is { } f ? Relogio.ParaDataLocal(f) : null;
            fatura.ValorTotal = Arredondar(pf.TotalAmount ?? 0);
            fatura.ValorMinimo = ArredondarNulo(pf.MinimumPaymentAmount);
            fatura.ValorPago = pf.Payments is { Count: > 0 } pagamentos
                ? Arredondar(pagamentos.Sum(p => p.Amount ?? 0))
                : null;
            fatura.AtualizadoEm = DateTime.UtcNow;
        }
        await _db.SaveChangesAsync(ct);
        return existentes.ToDictionary(kv => kv.Key, kv => kv.Value.Id);
    }

    private async Task<(int Novas, int Atualizadas)> SincronizarTransacoesAsync(PluggySessao sessao, Conta conta,
        bool ehCartao, DateOnly dataInicial, Dictionary<string, int> mapaFaturas, CancellationToken ct)
    {
        var transacoesPluggy = await _pluggy.ListarTransacoesAsync(sessao, conta.PluggyAccountId!, dataInicial, ct);

        // Carrega as transações locais do período (com margem) para atualizar em memória.
        var limite = dataInicial.AddDays(-60);
        var locais = await _db.Transacoes
            .Where(t => t.ContaId == conta.Id && t.PluggyTransactionId != null && t.Data >= limite)
            .ToDictionaryAsync(t => t.PluggyTransactionId!, ct);

        var idsRecebidos = new HashSet<string>(StringComparer.Ordinal);
        int novas = 0, atualizadas = 0;

        foreach (var pt in transacoesPluggy)
        {
            if (string.IsNullOrWhiteSpace(pt.Id) || pt.Date is null || !idsRecebidos.Add(pt.Id)) continue;

            if (!locais.TryGetValue(pt.Id, out var t))
            {
                // Pode existir fora da janela carregada (data alterada pela instituição).
                t = await _db.Transacoes.FirstOrDefaultAsync(x => x.PluggyTransactionId == pt.Id, ct);
                if (t is not null && t.ContaId != conta.Id) continue;
            }

            var nova = t is null;
            if (t is null)
            {
                t = new Transacao { ContaId = conta.Id, PluggyTransactionId = pt.Id, CriadoEm = DateTime.UtcNow };
                _db.Transacoes.Add(t);
            }

            var descricaoRecebida = Limitar(pt.Description?.Trim(), 300) is { Length: > 0 } d ? d : "(sem descrição)";
            // Mantém a descrição editada pelo usuário; atualiza se ele nunca editou.
            if (nova || t.Descricao == t.DescricaoOriginal) t.Descricao = descricaoRecebida;
            t.DescricaoOriginal = descricaoRecebida;
            t.Estabelecimento = Limitar(pt.Merchant?.BusinessName ?? pt.Merchant?.Name, 200);
            t.Data = Relogio.ParaDataLocal(pt.Date.Value);
            t.Valor = CalcularValor(pt, ehCartao);
            t.Status = string.Equals(pt.Status, "PENDING", StringComparison.OrdinalIgnoreCase)
                ? StatusTransacao.Pendente : StatusTransacao.Efetivada;
            t.PluggyCategoriaId = Limitar(pt.CategoryId, 20);
            t.ParcelaNumero = (short?)pt.CreditCardMetadata?.InstallmentNumber;
            t.ParcelaTotal = (short?)pt.CreditCardMetadata?.TotalInstallments;
            t.FaturaCartaoId = pt.CreditCardMetadata?.BillId is { } billId && mapaFaturas.TryGetValue(billId, out var fid)
                ? fid : t.FaturaCartaoId;
            // #111: como foi (Pix, boleto, pagamento de fatura…) e se foi entre contas do mesmo titular.
            t.MeioPagamento = Limitar((pt.OperationType ?? pt.PaymentData?.PaymentMethod)?.Trim().ToUpperInvariant(), 40);
            t.MesmaTitularidade = MesmoTitular(pt.PaymentData);

            if (!t.CategoriaManual)
                t.CategoriaId = _categorizacao.Resolver(t.DescricaoOriginal ?? t.Descricao, t.PluggyCategoriaId);
            _categorizacao.AplicarTipo(t);

            if (nova)
            {
                novas++;
            }
            else if (_db.Entry(t).State == EntityState.Modified)
            {
                t.AtualizadoEm = DateTime.UtcNow;
                atualizadas++;
            }
        }

        // Transações pendentes que sumiram na instituição (canceladas ou efetivadas com outro ID).
        var pendentesRemovidas = locais.Values
            .Where(t => t.Status == StatusTransacao.Pendente && t.Data >= dataInicial && !idsRecebidos.Contains(t.PluggyTransactionId!))
            .ToList();
        _db.Transacoes.RemoveRange(pendentesRemovidas);

        await _db.SaveChangesAsync(ct);
        return (novas, atualizadas);
    }

    private async Task SincronizarInvestimentosAsync(PluggySessao sessao, ConexaoBancaria conexao, CancellationToken ct)
    {
        List<PluggyInvestimento> investimentosPluggy;
        try
        {
            investimentosPluggy = await _pluggy.ListarInvestimentosAsync(sessao, conexao.PluggyItemId, ct);
        }
        catch (PluggyException ex) when (ex.StatusCode is 400 or 403 or 404)
        {
            // Instituição sem produto de investimentos ou não autorizado: ignora.
            return;
        }

        var existentes = await _db.Investimentos.Where(i => i.ConexaoBancariaId == conexao.Id)
            .ToDictionaryAsync(i => i.PluggyInvestmentId, ct);
        var recebidos = new HashSet<string>();

        foreach (var pi in investimentosPluggy)
        {
            if (string.IsNullOrWhiteSpace(pi.Id) || !recebidos.Add(pi.Id)) continue;
            if (!existentes.TryGetValue(pi.Id, out var inv))
            {
                inv = new Investimento { ConexaoBancariaId = conexao.Id, PluggyInvestmentId = pi.Id };
                _db.Investimentos.Add(inv);
            }
            inv.Nome = Limitar(pi.Name, 200) ?? "Investimento";
            inv.Tipo = Limitar(pi.Type, 30) ?? "OTHER";
            inv.Subtipo = Limitar(pi.Subtype, 60);
            inv.Codigo = Limitar(pi.Code, 60);
            inv.Emissor = Limitar(pi.Issuer, 200);
            inv.Moeda = NormalizarMoeda(pi.CurrencyCode);
            inv.Saldo = Arredondar(pi.Balance ?? 0);
            inv.ValorAplicado = ArredondarNulo(pi.AmountOriginal);
            inv.Rendimento = ArredondarNulo(pi.AmountProfit);
            inv.Quantidade = pi.Quantity is { } q ? Math.Round((decimal)q, 8) : null;
            var taxa = pi.AnnualRate ?? pi.FixedAnnualRate;
            inv.TaxaAnual = taxa is { } tx && Math.Abs(tx) < 99999 ? Math.Round((decimal)tx, 4) : null;
            inv.DataVencimento = pi.DueDate is { } dv ? Relogio.ParaDataLocal(dv) : null;
            var aplicacao = pi.PurchaseDate ?? pi.IssueDate;
            inv.DataAplicacao = aplicacao is { } da ? Relogio.ParaDataLocal(da) : inv.DataAplicacao;
            inv.Status = Limitar(pi.Status, 30);
            inv.AtualizadoEm = DateTime.UtcNow;
        }

        _db.Investimentos.RemoveRange(existentes.Values.Where(i => !recebidos.Contains(i.PluggyInvestmentId)));
        await _db.SaveChangesAsync(ct);

        try
        {
            await ImportarRendaVariavelAsync(sessao, conexao.Id, ct);
        }
        catch (Exception ex) when (!ct.IsCancellationRequested)
        {
            // A carteira é complementar: uma falha aqui não invalida a sincronização.
            _logger.LogWarning(ex, "Não foi possível importar as operações de renda variável do Open Finance.");
            _db.ChangeTracker.Clear();
        }
    }

    /// <summary>
    /// Contratos de crédito (empréstimos e financiamentos) e os pagamentos já feitos de cada um.
    /// Complementar: se a instituição não compartilha "operações de crédito", a sincronização segue sem eles.
    /// </summary>
    private async Task SincronizarEmprestimosAsync(PluggySessao sessao, ConexaoBancaria conexao, CancellationToken ct)
    {
        List<PluggyEmprestimo> emprestimosPluggy;
        try
        {
            emprestimosPluggy = await _pluggy.ListarEmprestimosAsync(sessao, conexao.PluggyItemId, ct);
        }
        catch (PluggyException ex) when (ex.StatusCode is 400 or 403 or 404)
        {
            // Instituição sem o produto de crédito ou consentimento sem "operações de crédito": ignora.
            return;
        }
        catch (Exception ex) when (ex is PluggyException or System.Text.Json.JsonException or HttpRequestException)
        {
            _logger.LogWarning(ex, "Não foi possível obter os empréstimos/financiamentos da conexão {ConexaoId}.", conexao.Id);
            return;
        }

        var existentes = await _db.Emprestimos.Include(e => e.Pagamentos)
            .Where(e => e.ConexaoBancariaId == conexao.Id)
            .ToDictionaryAsync(e => e.PluggyLoanId, ct);
        var recebidos = new HashSet<string>();

        foreach (var pe in emprestimosPluggy)
        {
            if (string.IsNullOrWhiteSpace(pe.Id) || !recebidos.Add(pe.Id)) continue;
            if (!existentes.TryGetValue(pe.Id, out var emp))
            {
                emp = new Emprestimo { ConexaoBancariaId = conexao.Id, PluggyLoanId = pe.Id };
                _db.Emprestimos.Add(emp);
            }

            emp.Natureza = ClassificacaoCredito.Natureza(pe.Kind, pe.Type, pe.ProductName);
            emp.Modalidade = Limitar(pe.Type, 80);
            emp.Produto = Limitar(pe.ProductName?.Trim(), 200) is { Length: > 0 } produto
                ? produto
                : ClassificacaoCredito.DescreverModalidade(pe.Type, emp.Natureza);
            emp.NumeroContrato = Limitar(pe.ContractNumber, 100);
            emp.Moeda = NormalizarMoeda(pe.CurrencyCode);
            emp.ValorContratado = Arredondar(pe.ContractAmount ?? 0);
            emp.SaldoDevedor = Arredondar(Math.Max(0, pe.Payments?.ContractOutstandingBalance ?? 0));
            emp.ParcelasTotal = pe.Installments?.TotalNumberOfInstallments;
            emp.ParcelasPagas = pe.Installments?.PaidInstallments;
            emp.ParcelasAVencer = pe.Installments?.DueInstallments;
            emp.ParcelasEmAtraso = pe.Installments?.PastDueInstallments;
            emp.Periodicidade = Limitar(pe.InstallmentPeriodicity, 30);
            emp.SistemaAmortizacao = Limitar(pe.AmortizationScheduled, 30);
            var taxa = pe.InterestRates?.FirstOrDefault();
            emp.TaxaJuros = Fracao(taxa?.PreFixedRate);
            emp.PeriodicidadeTaxa = Limitar(taxa?.TaxPeriodicity, 10);
            emp.Indexador = Limitar(taxa?.ReferentialRateIndexerSubType ?? taxa?.ReferentialRateIndexerType, 60);
            emp.Cet = Fracao(pe.Cet);
            emp.DataContratacao = pe.ContractDate is { } dc ? Relogio.ParaDataLocal(dc) : null;
            emp.DataPrimeiraParcela = pe.FirstInstallmentDueDate is { } dp ? Relogio.ParaDataLocal(dp) : null;
            emp.DataVencimentoFinal = pe.DueDate is { } dv ? Relogio.ParaDataLocal(dv) : null;

            // Pagamentos: a instituição sempre devolve o histórico completo, então recarrega.
            _db.PagamentosEmprestimo.RemoveRange(emp.Pagamentos);
            emp.Pagamentos = (pe.Payments?.Releases ?? new())
                .Where(r => r.PaidDate is not null && r.PaidAmount is > 0)
                .Select(r => new PagamentoEmprestimo
                {
                    Data = Relogio.ParaDataLocal(r.PaidDate!.Value),
                    Valor = Arredondar(r.PaidAmount!.Value),
                    Extraordinario = r.IsOverParcelPayment == true
                })
                .ToList();

            var ultimaParcela = emp.Pagamentos.Where(p => !p.Extraordinario).MaxBy(p => p.Data);
            emp.ValorParcela = ultimaParcela?.Valor
                ?? (emp.ParcelasAVencer is > 0 && emp.SaldoDevedor > 0
                    ? Math.Round(emp.SaldoDevedor / emp.ParcelasAVencer.Value, 2)
                    : null);
            emp.AtualizadoEm = DateTime.UtcNow;
        }

        _db.Emprestimos.RemoveRange(existentes.Values.Where(e => !recebidos.Contains(e.PluggyLoanId)));
        await _db.SaveChangesAsync(ct);
    }

    /// <summary>Taxas vêm em fração (0.0975 = 9,75%); descarta valores absurdos.</summary>
    private static decimal? Fracao(double? valor)
        => valor is { } v && Math.Abs(v) < 1000 ? Math.Round((decimal)v, 8) : null;

    /// <summary>
    /// Registra na carteira, como compras, os FIIs/ações/ETFs que o Open Finance informa (todas as conexões ativas).
    /// Chamado pelo botão da página de investimentos.
    /// </summary>
    public async Task<ResultadoImportacaoCarteiraDto> ImportarCarteiraAsync(CancellationToken ct)
    {
        if (!await Trava.WaitAsync(0, ct))
            return new ResultadoImportacaoCarteiraDto(0, 0, new(), new(), new(), "Há uma sincronização em andamento. Tente em instantes.");
        try
        {
            var (clientId, secret) = await ObterCredenciaisAsync(ct);
            if (clientId is null || secret is null)
                return new ResultadoImportacaoCarteiraDto(0, 0, new(), new(), new(), "Configure o Client ID e o Client Secret da Pluggy.");
            var sessao = await _pluggy.CriarSessaoAsync(clientId, secret, ct);
            var conexoes = await _db.ConexoesBancarias.Where(c => c.Ativa).Select(c => c.Id).ToListAsync(ct);
            var total = new ResultadoImportacaoCarteiraDto(0, 0, new(), new(), new(), null);
            foreach (var id in conexoes)
            {
                var r = await ImportarRendaVariavelAsync(sessao, id, ct);
                total = total with
                {
                    AtivosCriados = total.AtivosCriados + r.AtivosCriados,
                    OperacoesImportadas = total.OperacoesImportadas + r.OperacoesImportadas
                };
                total.SemHistorico.AddRange(r.SemHistorico);
                total.ComLancamentoManual.AddRange(r.ComLancamentoManual);
                total.Erros.AddRange(r.Erros);
            }
            return total;
        }
        finally
        {
            Trava.Release();
        }
    }

    /// <summary>
    /// FIIs, ações e ETFs da conexão viram ativos da carteira, com as compras e vendas informadas pela corretora
    /// (GET /investments/{id}/transactions). Sem histórico de movimentações, registra a posição atual como uma compra
    /// (custo informado pela corretora ou, na falta dele, o saldo atual) para você ajustar data e preço.
    /// Ativos com lançamentos manuais ou do extrato não recebem importação, para não duplicar.
    /// Cada investimento é salvo separadamente: a falha de um não impede os outros.
    /// </summary>
    private async Task<ResultadoImportacaoCarteiraDto> ImportarRendaVariavelAsync(PluggySessao sessao, int conexaoId,
        CancellationToken ct)
    {
        var resultado = new ResultadoImportacaoCarteiraDto(0, 0, new(), new(), new(), null);
        var investimentos = await _db.Investimentos.AsNoTracking()
            .Where(i => i.ConexaoBancariaId == conexaoId && (i.Tipo == "EQUITY" || i.Tipo == "ETF"))
            .ToListAsync(ct);
        var criados = 0;
        var importadas = 0;

        foreach (var inv in investimentos)
        {
            var ticker = CotacaoService.NormalizarTicker(inv.Codigo) ?? CotacaoService.NormalizarTicker(inv.Nome);
            if (ticker is null) continue;
            try
            {
                var ativo = await _db.Ativos.Include(a => a.Operacoes).FirstOrDefaultAsync(a => a.Ticker == ticker, ct);
                if (ativo is null)
                {
                    ativo = new Ativo
                    {
                        Ticker = ticker,
                        Tipo = TipoPorPluggy(inv.Tipo.ToUpperInvariant(), inv.Subtipo, ticker, inv.Emissor, inv.Nome)
                    };
                    _db.Ativos.Add(ativo);
                    criados++;
                }
                // Ativo só com lançamentos seus: não importa, para não duplicar o que você já lançou. Se a corretora já
                // mandou movimentações dele, um lançamento seu só completa o que ela não mandou (ex.: uma compra que
                // faltou no histórico) e as próximas movimentações continuam chegando.
                var temMovimentoDaCorretora = ativo.Operacoes.Any(EhMovimentoDaCorretora);
                var lancamentosSeus = ativo.Operacoes.Where(o => o.Origem != OrigemOperacao.OpenFinance).ToList();
                if (lancamentosSeus.Count > 0 && !temMovimentoDaCorretora)
                {
                    resultado.ComLancamentoManual.Add(ticker);
                    await _db.SaveChangesAsync(ct);
                    continue;
                }

                // Preço unitário informado pela corretora (saldo ÷ quantidade): usado até a primeira cotação.
                var precoCorretora = inv.Quantidade is decimal qi && qi > 0 && inv.Saldo > 0 ? inv.Saldo / qi : (decimal?)null;
                if (ativo.PrecoAtual is null && !ativo.PrecoManual && precoCorretora is decimal pc)
                {
                    ativo.PrecoAtual = Math.Round(pc, 4);
                    ativo.PrecoAtualizadoEm = DateTime.UtcNow;
                    ativo.FonteCotacao = "Open Finance";
                }

                List<PluggyMovimentoInvestimento> movimentos;
                try
                {
                    movimentos = await _pluggy.ListarMovimentosInvestimentoAsync(sessao, inv.PluggyInvestmentId, ct);
                }
                catch (PluggyException ex)
                {
                    _logger.LogInformation("Movimentações de {Ticker} indisponíveis na Pluggy: {Erro}", ticker, ex.Message);
                    movimentos = new();
                }

                var jaImportados = ativo.Operacoes.Where(o => o.PluggyTransacaoId != null)
                    .Select(o => o.PluggyTransacaoId!).ToHashSet();
                var temMovimento = false;
                foreach (var m in movimentos)
                {
                    var operacao = m.Type?.ToUpperInvariant() switch
                    {
                        "BUY" => TipoOperacao.Compra,
                        "SELL" => TipoOperacao.Venda,
                        _ => (TipoOperacao?)null
                    };
                    if (operacao is null || string.IsNullOrWhiteSpace(m.Id)) continue;
                    var quantidade = m.Quantity is double q ? Math.Abs((decimal)q) : 0m;
                    if (quantidade <= 0) continue;
                    temMovimento = true;
                    var idExterno = Limitar(m.Id, 100)!;
                    if (!jaImportados.Add(idExterno)) continue;
                    var data = m.TradeDate ?? m.Date;
                    var dataOperacao = data is { } d ? Relogio.ParaDataLocal(d) : Relogio.Hoje;

                    // A corretora mandou depois uma operação que você já tinha lançado: liga as duas, sem duplicar.
                    var lancada = lancamentosSeus.FirstOrDefault(o => o.PluggyTransacaoId is null && o.Tipo == operacao
                        && o.Quantidade == Math.Round(quantidade, 6)
                        && Math.Abs(o.Data.DayNumber - dataOperacao.DayNumber) <= DiasParaCasarLancamento);
                    if (lancada is not null)
                    {
                        lancada.PluggyTransacaoId = idExterno;
                        continue;
                    }

                    var preco = m.Value is double pu && pu > 0
                        ? (decimal)pu
                        : m.Amount is double valorTotal && valorTotal != 0 ? Math.Abs((decimal)valorTotal) / quantidade : 0m;
                    var taxas = m.Expenses?.Where(e => e.Key != "incomeTax" && e.Value is > 0d)
                        .Sum(e => (decimal)e.Value!.Value) ?? 0m;
                    ativo.Operacoes.Add(new OperacaoAtivo
                    {
                        Tipo = operacao.Value,
                        Data = dataOperacao,
                        Quantidade = Math.Round(quantidade, 6),
                        PrecoUnitario = Math.Round(preco, 6),
                        Taxas = Math.Round(taxas, 2),
                        Origem = OrigemOperacao.OpenFinance,
                        PluggyTransacaoId = idExterno,
                        Observacao = "Importada do Open Finance"
                    });
                    importadas++;
                }

                // Sem histórico: a posição atual vira uma compra para ajustar (data e preço).
                var idPosicao = "posicao:" + Limitar(inv.PluggyInvestmentId, 90);
                var posicao = ativo.Operacoes.FirstOrDefault(o => o.PluggyTransacaoId == idPosicao);
                if (temMovimento && posicao is not null)
                {
                    // O histórico chegou depois: a compra provisória sai.
                    ativo.Operacoes.Remove(posicao);
                    _db.OperacoesAtivo.Remove(posicao);
                }
                else if (!temMovimento && !temMovimentoDaCorretora && lancamentosSeus.Count == 0 && inv.Saldo > 0)
                {
                    // Só para ativo sem histórico: se a Pluggy falhou ao listar as movimentações de um ativo que já tem
                    // histórico, a posição inteira não pode virar uma compra a mais.
                    var quantidade = inv.Quantidade is decimal qtd && qtd > 0 ? qtd : (decimal?)null;
                    if (quantidade is null && ativo.PrecoAtual is decimal pa && pa > 0)
                        quantidade = Math.Round(inv.Saldo / pa); // cotas = saldo ÷ cotação
                    if (quantidade is decimal qtdFinal && qtdFinal > 0)
                    {
                        if (posicao is null)
                        {
                            posicao = new OperacaoAtivo
                            {
                                Tipo = TipoOperacao.Compra,
                                Origem = OrigemOperacao.OpenFinance,
                                PluggyTransacaoId = idPosicao,
                                Data = inv.DataAplicacao ?? Relogio.Hoje,
                                Observacao = "Posição do Open Finance sem histórico de compras: confira a data e o preço pago."
                            };
                            ativo.Operacoes.Add(posicao);
                            importadas++;
                        }
                        if (inv.DataAplicacao is DateOnly da) posicao.Data = da;
                        posicao.Quantidade = Math.Round(qtdFinal, 6);
                        // Só atualiza o preço se ainda for o provisório (o usuário pode ter corrigido).
                        if (posicao.PrecoUnitario == 0 || posicao.Id == 0)
                        {
                            var custo = inv.ValorAplicado is decimal va && va > 0 ? va : inv.Saldo;
                            posicao.PrecoUnitario = Math.Round(custo / qtdFinal, 6);
                        }
                        resultado.SemHistorico.Add(ticker);
                    }
                }

                await _db.SaveChangesAsync(ct);
            }
            catch (Exception ex) when (!ct.IsCancellationRequested)
            {
                _logger.LogWarning(ex, "Falha ao importar {Ticker} do Open Finance.", ticker);
                resultado.Erros.Add(ticker);
                _db.ChangeTracker.Clear();
            }
        }
        return resultado with { AtivosCriados = criados, OperacoesImportadas = importadas };
    }

    /// <summary>Até quantos dias de diferença um lançamento seu é a mesma operação que a corretora mandou depois.</summary>
    private const int DiasParaCasarLancamento = 5;

    /// <summary>Compra ou venda do histórico da corretora (não a posição provisória de um ativo sem histórico).</summary>
    private static bool EhMovimentoDaCorretora(OperacaoAtivo o)
        => o.Origem == OrigemOperacao.OpenFinance && o.PluggyTransacaoId is string id
           && !id.StartsWith("posicao:", StringComparison.Ordinal);

    private static TipoAtivo TipoPorPluggy(string tipo, string? subtipo, string ticker, string? emissor, string? nome)
    {
        if (tipo == "ETF") return TipoAtivo.Etf;
        // O Nubank manda FII com o subtipo "STOCK": o nome do emissor ("... FDO INV IMOB - FII") desempata.
        if (ClassificacaoAtivo.PareceFii(emissor) || ClassificacaoAtivo.PareceFii(nome)) return TipoAtivo.Fii;
        return subtipo?.ToUpperInvariant() switch
        {
            "REAL_ESTATE_FUND" => TipoAtivo.Fii,
            "BDR" => TipoAtivo.Bdr,
            "STOCK" => TipoAtivo.Acao,
            // Final 11 (cota de FII) e 12/13 (direitos e recibos de subscrição de FII).
            _ => ticker.EndsWith("11", StringComparison.Ordinal) || ticker.EndsWith("12", StringComparison.Ordinal)
                 || ticker.EndsWith("13", StringComparison.Ordinal) ? TipoAtivo.Fii : TipoAtivo.Acao
        };
    }

    private async Task AtualizarCategoriasPluggyAsync(PluggySessao sessao, CancellationToken ct)
    {
        var categoriasPluggy = await _pluggy.ListarCategoriasAsync(sessao, ct);
        if (categoriasPluggy.Count == 0) return;

        var porId = categoriasPluggy.Where(c => !string.IsNullOrWhiteSpace(c.Id))
            .GroupBy(c => c.Id).ToDictionary(g => g.Key, g => g.First());
        var existentes = await _db.CategoriasPluggy.ToDictionaryAsync(c => c.PluggyCategoriaId, ct);
        var categoriasLocais = await _db.Categorias.AsNoTracking()
            .Select(c => new { c.Id, c.Nome }).ToListAsync(ct);
        var idPorNome = categoriasLocais.ToDictionary(c => c.Nome, c => c.Id, StringComparer.OrdinalIgnoreCase);

        foreach (var pc in porId.Values)
        {
            if (pc.Id.Length > 20) continue;
            var nomeExibicao = pc.DescriptionTranslated ?? pc.Description ?? pc.Id;
            string? paiExibicao = null;
            if (pc.ParentId is not null && porId.TryGetValue(pc.ParentId, out var pai))
                paiExibicao = pai.DescriptionTranslated ?? pai.Description;
            paiExibicao ??= pc.ParentDescription;

            if (!existentes.TryGetValue(pc.Id, out var registro))
            {
                registro = new CategoriaPluggy { PluggyCategoriaId = pc.Id };
                var sugestao = CategorizacaoService.SugerirCategoriaPadrao(pc.Description, pc.ParentDescription);
                if (sugestao is not null && idPorNome.TryGetValue(sugestao, out var idLocal)) registro.CategoriaId = idLocal;
                _db.CategoriasPluggy.Add(registro);
            }
            registro.Descricao = Limitar(nomeExibicao, 150) ?? pc.Id;
            registro.PaiPluggyId = pc.ParentId is { Length: <= 20 } p ? p : null;
            registro.PaiDescricao = Limitar(paiExibicao, 150);
        }
        await _db.SaveChangesAsync(ct);
    }

    private async Task<(string? ClientId, string? Secret)> ObterCredenciaisAsync(CancellationToken ct)
    {
        var clientId = await _config.ObterAsync(ConfiguracaoService.PluggyClientId, ct);
        var secret = await _config.ObterAsync(ConfiguracaoService.PluggyClientSecret, ct);
        return (string.IsNullOrWhiteSpace(clientId) ? null : clientId, string.IsNullOrWhiteSpace(secret) ? null : secret);
    }

    /// <summary>
    /// Normaliza o sinal: no banco local, negativo = saída e positivo = entrada.
    /// Na Pluggy, em cartão de crédito valor positivo é compra (saída); em conta, o tipo DEBIT/CREDIT define o sentido.
    /// </summary>
    /// <summary>
    /// #111: pagador e recebedor com o mesmo CPF/CNPJ (só dígitos, completos). Documento mascarado ou ausente dá
    /// null (não dá para saber). Só o resultado é gravado; os documentos não.
    /// </summary>
    private static bool? MesmoTitular(PluggyDadosPagamento? dados)
    {
        static string? Digitos(PluggyParticipante? p)
        {
            var valor = p?.Documento();
            if (string.IsNullOrWhiteSpace(valor) || valor.Contains('*')) return null;
            var digitos = new string(valor.Where(char.IsDigit).ToArray());
            return digitos.Length is 11 or 14 ? digitos : null;
        }
        var pagador = Digitos(dados?.Payer);
        var recebedor = Digitos(dados?.Receiver);
        return pagador is null || recebedor is null ? null : pagador == recebedor;
    }

    private static decimal CalcularValor(PluggyTransacao pt, bool ehCartao)
    {
        var valor = Arredondar(pt.AmountInAccountCurrency ?? pt.Amount ?? 0);
        if (ehCartao) return -valor;
        var absoluto = Math.Abs(valor);
        return pt.Type?.ToUpperInvariant() switch
        {
            "DEBIT" => -absoluto,
            "CREDIT" => absoluto,
            _ => valor
        };
    }

    private static decimal Arredondar(double valor) => Math.Round((decimal)valor, 2, MidpointRounding.AwayFromZero);

    private static decimal? ArredondarNulo(double? valor) => valor is { } v ? Arredondar(v) : null;

    private static string NormalizarMoeda(string? moeda)
        => moeda is { Length: 3 } m ? m.ToUpperInvariant() : "BRL";

    private static string? NormalizarCor(string? cor)
    {
        if (string.IsNullOrWhiteSpace(cor)) return null;
        var c = cor.Trim().TrimStart('#');
        return c.Length == 6 && c.All(Uri.IsHexDigit) ? "#" + c.ToLowerInvariant() : null;
    }

    private static string? Limitar(string? valor, int max)
        => valor is null ? null : valor.Length <= max ? valor : valor[..max];
}
