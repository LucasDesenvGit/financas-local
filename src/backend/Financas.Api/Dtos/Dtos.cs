using System.ComponentModel.DataAnnotations;
using Financas.Api.Entidades;

namespace Financas.Api.Dtos;

/* ---------------------------- Autenticação ---------------------------- */
public record EstadoAutenticacaoDto(bool Configurado, bool Autenticado, string? Login);

public record CredenciaisDto(
    [Required, StringLength(100, MinimumLength = 3)] string Login,
    [Required, StringLength(200)] string Senha);

public record AlterarSenhaDto(
    [Required, StringLength(200)] string SenhaAtual,
    [Required, StringLength(200)] string NovaSenha);

/* ------------------------------- Contas ------------------------------- */
public record ContaDto(
    int Id, string Nome, TipoConta Tipo, OrigemConta Origem, string? Numero, string Moeda,
    decimal Saldo, decimal SaldoInicial, decimal? LimiteCredito, decimal? LimiteDisponivel,
    byte? DiaFechamento, byte? DiaVencimento, string Cor, bool IncluirNoSaldo, bool Arquivada,
    int? ConexaoBancariaId, string? NomeInstituicao, DateTime AtualizadoEm);

public record SalvarContaDto(
    [Required, StringLength(120)] string Nome,
    TipoConta Tipo,
    [StringLength(60)] string? Numero,
    decimal SaldoInicial,
    decimal? LimiteCredito,
    [Range(1, 31)] byte? DiaFechamento,
    [Range(1, 31)] byte? DiaVencimento,
    [Required, RegularExpression("^#[0-9a-fA-F]{6}$")] string Cor,
    bool IncluirNoSaldo,
    bool Arquivada);

public record FaturaDto(int Id, DateOnly DataVencimento, DateOnly? DataFechamento, decimal ValorTotal,
    decimal? ValorMinimo, decimal? ValorPago, int QuantidadeTransacoes);

/* ----------------------------- Transações ----------------------------- */
public record TransacaoDto(
    long Id, int ContaId, string ContaNome, bool OpenFinance, DateOnly Data, string Descricao,
    string? DescricaoOriginal, string? Estabelecimento, decimal Valor, TipoTransacao Tipo, StatusTransacao Status,
    int? CategoriaId, string? CategoriaNome, string? CategoriaCor, short? ParcelaNumero, short? ParcelaTotal,
    bool IgnorarRelatorios, string? Observacao, bool Emergencia);

public class FiltroTransacoesDto
{
    public DateOnly? Inicio { get; set; }
    public DateOnly? Fim { get; set; }
    public int? ContaId { get; set; }
    public int? CategoriaId { get; set; }
    /// <summary>Várias categorias (?categoriaIds=1&amp;categoriaIds=2); cada uma inclui as subcategorias.</summary>
    [MaxLength(200)] public List<int>? CategoriaIds { get; set; }
    /// <summary>Sem categoria; junto com categorias, soma as duas coisas (OU).</summary>
    public bool SemCategoria { get; set; }
    public bool SomenteEmergencias { get; set; }
    public TipoTransacao? Tipo { get; set; }
    [StringLength(100)] public string? Busca { get; set; }
    [Range(1, int.MaxValue)] public int Pagina { get; set; } = 1;
    [Range(1, 500)] public int TamanhoPagina { get; set; } = 50;
}

public record PaginaTransacoesDto(List<TransacaoDto> Itens, int Total, int Pagina, int TamanhoPagina,
    decimal TotalReceitas, decimal TotalDespesas);

public record SalvarTransacaoDto(
    int ContaId,
    DateOnly Data,
    [Required, StringLength(300)] string Descricao,
    [Range(0.01d, 999999999d)] decimal Valor,
    TipoTransacao Tipo,
    int? CategoriaId,
    [StringLength(1000)] string? Observacao,
    bool IgnorarRelatorios,
    StatusTransacao Status = StatusTransacao.Efetivada,
    bool Emergencia = false);

public record AtualizarTransacaoOpenFinanceDto(
    [Required, StringLength(300)] string Descricao,
    int? CategoriaId,
    [StringLength(1000)] string? Observacao,
    bool IgnorarRelatorios,
    bool Emergencia = false);

public record CategorizarLoteDto([Required, MinLength(1), MaxLength(1000)] List<long> Ids, int? CategoriaId);

public record EmergenciaLoteDto([Required, MinLength(1), MaxLength(1000)] List<long> Ids, bool Emergencia);

/// <summary>Confiança: "alta" (histórico consistente), "media" (histórico parcial), "baixa" (palavra-chave) ou "nenhuma".</summary>
public record SugestaoCategoriaDto(string Chave, string Exemplo, int Quantidade, decimal Total, List<long> TransacaoIds,
    int? CategoriaId, string? CategoriaNome, string? CategoriaCor, string Confianca, string? Motivo, string TextoRegra,
    List<OpcaoCategoriaDto> Alternativas);

public record OpcaoCategoriaDto(int CategoriaId, string Nome, string Cor, string Motivo);

public record TransferenciaDto(
    int ContaOrigemId,
    int ContaDestinoId,
    [Range(0.01d, 999999999d)] decimal Valor,
    DateOnly Data,
    [StringLength(300)] string? Descricao);

/* ----------------------------- Categorias ----------------------------- */
public record CategoriaDto(int Id, string Nome, TipoCategoria Tipo, string Cor, string? Icone,
    int? CategoriaPaiId, bool Ativa, NaturezaGasto? Natureza);

public record SalvarCategoriaDto(
    [Required, StringLength(80)] string Nome,
    TipoCategoria Tipo,
    [Required, RegularExpression("^#[0-9a-fA-F]{6}$")] string Cor,
    [StringLength(40)] string? Icone,
    int? CategoriaPaiId,
    bool Ativa = true,
    NaturezaGasto? Natureza = null);

public record RegraDto(int Id, string Texto, TipoCorrespondencia TipoCorrespondencia, int CategoriaId,
    string CategoriaNome, int Prioridade, bool Ativa);

public record SalvarRegraDto(
    [Required, StringLength(200, MinimumLength = 2)] string Texto,
    TipoCorrespondencia TipoCorrespondencia,
    int CategoriaId,
    [Range(0, 10000)] int Prioridade,
    bool Ativa = true);

public record ResultadoAplicacaoDto(int TransacoesAtualizadas);

/* ----------------------------- Planejamento ----------------------------- */
public record OrcamentoDto(int Id, int CategoriaId, string CategoriaNome, string CategoriaCor,
    short Ano, byte Mes, decimal Valor, decimal Gasto, decimal Percentual, decimal GastoEmergencia = 0,
    string Situacao = "ok");

public record SalvarOrcamentoDto(int CategoriaId, [Range(2000, 2100)] short Ano, [Range(1, 12)] byte Mes,
    [Range(0d, 999999999d)] decimal Valor);

public record CopiarOrcamentoDto([Range(2000, 2100)] short Ano, [Range(1, 12)] byte Mes);

/// <summary>Uso do limite de um cartão: média das últimas faturas fechadas ÷ limite.</summary>
public record CartaoUsoLimiteDto(int ContaId, string Nome, decimal Limite, decimal FaturaMedia, decimal Percentual);

/// <summary>
/// Sugestão para uma categoria principal (subcategorias somam nela). <c>Sugerido</c> é null quando não há dados suficientes;
/// <c>Atual</c> é o orçamento que já existe no mês. <c>Tendencia</c>: "alta", "queda", "estavel" ou null (menos de 6 meses).
/// </summary>
public record ItemSugestaoOrcamentoDto(int CategoriaId, string Nome, string Cor, NaturezaGasto? Natureza, decimal Mediana,
    decimal Media, decimal UltimoMes, string? Tendencia, int MesesComGasto, decimal PagoNoCartaoPercentual,
    decimal? Sugerido, decimal? Atual, bool Selecionado, List<string> Motivos);

/// <summary>
/// Sugestão de orçamentos de um mês: o resumo (renda, parcelas, poupança, cartões) e uma linha por categoria.
/// <c>TotalSugerido</c> é o total do mês ao aplicar a sugestão (categorias que já têm orçamento contam com o atual).
/// </summary>
public record SugestaoOrcamentoDto(short Ano, byte Mes, int MesesBase, int MesesRenda, decimal RendaMedia,
    decimal ParcelasCartao, decimal ParcelasFinanciamentos, decimal ParcelasEmprestimos, decimal PercentualPoupanca,
    decimal MetaPoupanca, decimal Disponivel, decimal TotalSugerido, decimal? FaltaParaCaber,
    decimal? UsoLimiteCartoes, string? NivelCartao, decimal LimiteAtencao, decimal LimiteAlerta,
    List<CartaoUsoLimiteDto> Cartoes, List<string> Avisos, List<ItemSugestaoOrcamentoDto> Itens);

public record ItemAplicarSugestaoDto(int CategoriaId, [Range(0.01d, 999999999d)] decimal Valor);

public record AplicarSugestaoOrcamentoDto([Range(2000, 2100)] short Ano, [Range(1, 12)] byte Mes,
    [Required, MinLength(1), MaxLength(500)] List<ItemAplicarSugestaoDto> Itens);

public record MetaDto(int Id, string Nome, decimal ValorAlvo, decimal ValorAtual, DateOnly? DataAlvo,
    int? ContaId, string? ContaNome, string Cor, bool Concluida, decimal Percentual, decimal? AporteMensalSugerido,
    decimal ValorInicial, decimal SaldoConta);

public record SalvarMetaDto(
    [Required, StringLength(120)] string Nome,
    [Range(0.01d, 999999999d)] decimal ValorAlvo,
    [Range(0d, 999999999d)] decimal ValorAtual,
    DateOnly? DataAlvo,
    int? ContaId,
    [Required, RegularExpression("^#[0-9a-fA-F]{6}$")] string Cor,
    bool Concluida);

public record AporteDto([Range(-999999999d, 999999999d)] decimal Valor);

public record ContaAgendadaDto(int Id, string Descricao, decimal Valor, DateOnly ProximoVencimento,
    Recorrencia Recorrencia, int? CategoriaId, string? CategoriaNome, int? ContaId, string? ContaNome,
    bool Ativa, DateOnly? UltimoPagamentoEm, int DiasParaVencer);

public record SalvarContaAgendadaDto(
    [Required, StringLength(150)] string Descricao,
    [Range(0.01d, 999999999d)] decimal Valor,
    DateOnly ProximoVencimento,
    Recorrencia Recorrencia,
    int? CategoriaId,
    int? ContaId,
    bool Ativa = true);

public record PagarContaAgendadaDto(bool LancarTransacao, DateOnly? Data);

/// <summary>
/// Pagamento mensal encontrado no extrato que ainda não está em Contas a pagar. <c>ProximoVencimento</c> nunca é
/// passado; <c>VencimentoSemPagamento</c> é o vencimento esperado que já passou sem o pagamento aparecer no extrato.
/// </summary>
public record SugestaoContaAgendadaDto(string Chave, string Descricao, decimal Valor, bool ValorVariavel,
    decimal ValorMinimo, decimal ValorMaximo, DateOnly ProximoVencimento, int DiaHabitual, int Meses, int Pagamentos,
    DateOnly UltimoPagamento, DateOnly PrimeiroPagamento, int? CategoriaId, string? CategoriaNome, int ContaId,
    string ContaNome, string TermoBusca, DateOnly? VencimentoSemPagamento);

/* ----------------------------- Investimentos ----------------------------- */
/// <summary>Valores na moeda do investimento; <c>SaldoEmReais</c> é null quando não há cotação guardada da moeda.</summary>
public record InvestimentoDto(int Id, string Nome, string Tipo, string? Subtipo, string? Codigo, string? Emissor,
    string Moeda, decimal Saldo, decimal? SaldoEmReais, decimal? ValorAplicado, decimal? Rendimento, decimal? TaxaAnual,
    DateOnly? DataVencimento, string? Status, string? NomeInstituicao, DateTime AtualizadoEm);

/* ----------------------------- Câmbio ----------------------------- */
public record MoedaDto(string Codigo, string Nome);

/// <summary>Reais por 1 unidade da moeda. <c>Data</c> é o dia útil da cotação, que pode ser anterior a <c>DataSolicitada</c>.</summary>
public record CotacaoMoedaDto(string Moeda, DateOnly DataSolicitada, DateOnly Data, decimal Compra, decimal Venda,
    string Fonte);

/// <summary>Última cotação guardada de cada moeda (disponível mesmo sem internet).</summary>
public record CotacoesAtuaisDto(DateTime? AtualizadoEm, List<CotacaoMoedaDto> Cotacoes);

/* ----------------------------- Empréstimos e financiamentos ----------------------------- */
/// <summary>Saldo devedor e parcela mensal dos contratos em aberto, separados por natureza.</summary>
public record TotaisCreditoDto(decimal SaldoFinanciamentos, decimal SaldoEmprestimos,
    decimal ParcelaMensalFinanciamentos, decimal ParcelaMensalEmprestimos);

public record EmprestimoDto(int Id, NaturezaCredito Natureza, string Produto, string? Modalidade, string ModalidadeDescricao,
    string? NumeroContrato, string? NomeInstituicao, string Moeda, decimal ValorContratado, decimal SaldoDevedor,
    decimal? ValorParcela, decimal ParcelaMensal, int? ParcelasTotal, int? ParcelasPagas, int? ParcelasAVencer,
    int ParcelasEmAtraso, string? Periodicidade, string? SistemaAmortizacao, decimal? TaxaJuros, string? PeriodicidadeTaxa,
    string? Indexador, decimal? Cet, DateOnly? DataContratacao, DateOnly? DataVencimentoFinal, DateOnly? ProximoVencimento,
    decimal TotalPago, DateOnly? UltimoPagamento, bool Quitado, DateTime AtualizadoEm);

public record ResumoCreditoDto(NaturezaCredito Natureza, int ContratosAbertos, int ContratosQuitados, decimal SaldoDevedor,
    decimal ValorContratado, decimal ParcelaMensal, decimal TotalPago, int ParcelasEmAtraso);

public record PagamentosMesCreditoDto(int Ano, int Mes, decimal Financiamentos, decimal Emprestimos);

public record PainelEmprestimosDto(ResumoCreditoDto Financiamentos, ResumoCreditoDto Emprestimos,
    List<EmprestimoDto> Contratos, List<PagamentosMesCreditoDto> PagamentosPorMes);

/* ----------------------------- Painel / relatórios ----------------------------- */
public record ValorCategoriaDto(int? CategoriaId, string Nome, string Cor, decimal Valor);

public record MesValoresDto(int Ano, int Mes, decimal Receitas, decimal Despesas, decimal DespesasEmergencia = 0);

/* ----------------------------- Visão anual ----------------------------- */
public record ResumoAnoDto(decimal Receitas, decimal Despesas, decimal Resultado, decimal TaxaPoupanca);

public record MesComparativoDto(int Mes, decimal Receitas, decimal Despesas, decimal ReceitasAnterior,
    decimal DespesasAnterior, decimal? PatrimonioEstimado, bool Futuro);

public record CategoriaComparativaDto(int? CategoriaId, string Nome, string Cor, NaturezaGasto? Natureza,
    decimal Valor, decimal ValorAnterior, List<decimal> Meses);

public record DistribuicaoRendaDto(decimal Receitas, decimal Essenciais, decimal EstiloDeVida, decimal NaoClassificado,
    decimal Poupanca);

public record SaudeFinanceiraDto(decimal PatrimonioAtual, decimal ReservaLiquida, decimal GastoMensalMedio,
    decimal GastoEssencialMensalMedio, decimal? MesesDeReserva, decimal? MesesDeReservaEssencial,
    decimal RendaMensalMedia, decimal ParcelasMensais, decimal? ComprometimentoParcelas, int MesesAnalisados,
    decimal ParcelasFinanciamentos, decimal ParcelasEmprestimos, decimal DividaFinanciamentos, decimal DividaEmprestimos);

public record PainelAnualDto(int Ano, int AnoComparacao, int MesLimite, List<int> AnosDisponiveis,
    ResumoAnoDto Resumo, ResumoAnoDto ResumoAnteriorMesmoPeriodo, ResumoAnoDto ResumoAnteriorCompleto,
    List<MesComparativoDto> Meses, List<CategoriaComparativaDto> Categorias, DistribuicaoRendaDto Distribuicao,
    SaudeFinanceiraDto Saude);

/* ----------------------------- Projeção ----------------------------- */
/// <summary>Situacao: "realizado", "em andamento" ou "projetado".</summary>
public record ProjecaoMesDto(int Ano, int Mes, string Situacao, decimal Receitas, decimal Despesas,
    decimal Parcelas, decimal Resultado, decimal? SaldoProjetado);

public record ProjecaoCategoriaDto(int? CategoriaId, string Nome, string Cor, decimal MediaMensal, decimal UltimoMes);

public record ProjecaoDto(int MesesConsiderados, decimal MediaReceitas, decimal MediaDespesas,
    decimal SaldoDisponivelAtual, List<ProjecaoMesDto> Meses, List<ProjecaoCategoriaDto> Categorias);

public record PainelDto(
    decimal SaldoContas,
    decimal DividaCartoes,
    decimal TotalInvestido,
    decimal PatrimonioLiquido,
    decimal RendaVariavel,
    decimal PatrimonioSemRendaVariavel,
    decimal ReceitasMes,
    decimal DespesasMes,
    List<ValorCategoriaDto> GastosPorCategoria,
    List<MesValoresDto> Evolucao,
    List<OrcamentoDto> Orcamentos,
    List<ContaAgendadaDto> ProximasContas,
    List<TransacaoDto> UltimasTransacoes,
    List<string> Alertas,
    DateTime? UltimaSincronizacao,
    decimal DespesasEmergenciaMes = 0,
    decimal DividaFinanciamentos = 0,
    decimal DividaEmprestimos = 0,
    int DiasProximosVencimentos = 15);

/* ----------------------------- Parametrização ----------------------------- */
/// <summary>Booleanos usam 0/1 em Valor e Padrao.</summary>
public record ParametroDto(string Chave, string Grupo, string Nome, string Descricao, Servicos.TipoParametro Tipo,
    int Valor, int Padrao, int? Minimo, int? Maximo, string? Unidade);

public record SalvarParametrosDto([Required] Dictionary<string, int> Valores);

public record InfoSistemaDto(string Versao, string DotNet, string SistemaOperacional, DateTime IniciadoEm,
    string CaminhoBanco, long? TamanhoBancoBytes, string PastaChaves, int Contas, int Transacoes, int ConexoesAtivas,
    DateTime? UltimaSincronizacao);

/* ----------------------------- Open Finance (Pluggy) ----------------------------- */
public record ConfiguracaoPluggyDto(string? ClientId, bool SecretConfigurado, int IntervaloHoras,
    int DiasRetroativos, bool Habilitada);

public record SalvarConfiguracaoPluggyDto(
    [Required, StringLength(100)] string ClientId,
    [StringLength(200)] string? ClientSecret,
    // Opcionais: a tela de Parametrização cuida deles; quando omitidos, ficam como estão.
    [Range(1, 48)] int? IntervaloHoras = null,
    [Range(1, 365)] int? DiasRetroativos = null,
    bool? Habilitada = null);

public record ConexaoDto(int Id, string PluggyItemId, string? NomeInstituicao, string? CorPrimaria, string? Status,
    string? StatusExecucao, DateTime? UltimaAtualizacaoInstituicao, DateTime? ConsentimentoExpiraEm,
    DateTime? UltimaSincronizacaoEm, string? UltimoErro, bool Ativa, int QuantidadeContas);

public record AdicionarConexaoDto(
    [Required, RegularExpression("^[0-9a-fA-F-]{20,64}$", ErrorMessage = "Item ID inválido.")] string ItemId);

public record SincronizacaoLogDto(long Id, int? ConexaoBancariaId, string? NomeInstituicao, string Origem,
    DateTime IniciadoEm, DateTime? FinalizadoEm, bool Sucesso, int TransacoesNovas, int TransacoesAtualizadas,
    string? Mensagem);

public record ResultadoSincronizacaoDto(bool Sucesso, int TransacoesNovas, int TransacoesAtualizadas, string? Mensagem);

/// <summary>Status leve da sincronização para a barra superior (UX-8): consultado a cada poucos minutos em toda tela.</summary>
public record StatusSincronizacaoDto(DateTime? UltimaSincronizacao, bool EmExecucao, int ConexoesAtivas,
    int ConexoesComErro, int ConsentimentosVencendo);

public record CategoriaPluggyDto(string PluggyCategoriaId, string Descricao, string? PaiDescricao,
    int? CategoriaId, string? CategoriaNome, int QuantidadeTransacoes);

public record MapearCategoriaPluggyDto(int? CategoriaId, bool AplicarExistentes);

public record MensagemDto(string Mensagem);

/* ----------------------------- Carteira de renda variável ----------------------------- */
public record SelicResumoDto(decimal? MetaAnual, decimal? Acumulado12Meses, decimal? AcumuladoAno, DateOnly? AtualizadaAte);

public record UltimoProventoDto(DateOnly DataCom, decimal ValorPorCota);

/// <summary>
/// Posição de um ativo da carteira manual. <c>PrecoMedio</c>, <c>PrecoAtual</c>, <c>PrecoAnterior</c> e o último provento
/// estão na moeda do ativo; custo, valores e resultados estão em reais.
/// </summary>
public record PosicaoAtivoDto(int Id, string Ticker, string? Nome, TipoAtivo Tipo, string Moeda,
    decimal Quantidade, decimal PrecoMedio, decimal Custo,
    decimal? PrecoAtual, decimal? PrecoAtualEmReais, decimal? PrecoAnterior, decimal? VariacaoDia,
    decimal ValorMercado, decimal Valorizacao, decimal? ValorizacaoPercentual,
    decimal LucroRealizado, decimal ProventosRecebidos, decimal Proventos12Meses, decimal? DividendYield12Meses,
    decimal ResultadoTotal, decimal? ResultadoTotalPercentual,
    decimal ValorSeFosseSelic, decimal DiferencaSelic, decimal? ResultadoSelicPercentual,
    DateOnly? PrimeiraCompra, UltimoProventoDto? UltimoProvento,
    bool PrecoManual, DateTime? PrecoAtualizadoEm, string? FonteCotacao, bool SemCotacao, string? Segmento);

public record TotaisRendaVariavelDto(decimal ValorMercado, decimal Custo, decimal Valorizacao,
    decimal? ValorizacaoPercentual, decimal LucroRealizado, decimal ProventosRecebidos, decimal Proventos12Meses,
    decimal ResultadoTotal, decimal ValorSeFosseSelic, decimal DiferencaSelic, decimal? ResultadoTotalPercentual,
    decimal? ResultadoSelicPercentual);

public record EvolucaoCarteiraDto(int Ano, int Mes, decimal Aplicado, decimal AplicadoNaSelic, decimal? ValorMercado);

public record DistribuicaoInvestimentoDto(string Nome, decimal Valor);

/// <summary>
/// Investimento vindo do Open Finance comparado com a Selic do período. Saldo, valor aplicado e rentabilidade
/// estão na moeda do investimento; <c>SaldoEmReais</c> é null quando não há cotação guardada da moeda.
/// </summary>
public record InvestimentoComparadoDto(int Id, string Nome, string Tipo, string? Subtipo, string? Codigo,
    string? Emissor, string? NomeInstituicao, string Moeda, decimal Saldo, decimal? SaldoEmReais,
    decimal? ValorAplicado, DateOnly? DataAplicacao,
    DateOnly? DataVencimento, decimal? TaxaAnual, decimal? Rentabilidade, decimal? SelicNoPeriodo,
    decimal? PercentualDaSelic, bool RendaVariavel, bool JaNaCarteira, DateTime AtualizadoEm);

public record CarteiraDto(SelicResumoDto Selic, TotaisRendaVariavelDto RendaVariavel, List<PosicaoAtivoDto> Posicoes,
    List<EvolucaoCarteiraDto> Evolucao, List<InvestimentoComparadoDto> OpenFinance, decimal TotalOpenFinance,
    decimal TotalInvestimentosManuais, decimal TotalGeral, List<DistribuicaoInvestimentoDto> Distribuicao,
    DateTime? CotacoesAtualizadasEm, bool CotacoesDesatualizadas, int OpenFinanceNaoRegistrados,
    List<string> MoedasSemCotacao, List<ReservaDto> Reservas, decimal TotalReservas);

/// <summary>
/// Reserva ("cofrinho") cujo saldo a instituição não informa pelo Open Finance, estimado pelos lançamentos
/// "Dinheiro reservado/retirado {Nome}" da conta. Com conferência, <c>Saldo</c> = saldo conferido + movimentos
/// depois de <c>DataConferencia</c>; sem ela, só a soma dos movimentos importados (fica abaixo do real se a
/// reserva já tinha dinheiro antes do primeiro lançamento importado ou se o rendimento ainda não foi retirado).
/// <c>Guardado</c> e <c>Retirado</c> consideram os mesmos movimentos que entram no saldo.
/// <c>PodeTerSaldoAnterior</c>: sem conferência e o primeiro lançamento da reserva está no início da janela importada
/// da conta (a reserva provavelmente já tinha dinheiro antes). <c>ConferenciaAntiga</c>: conferida há mais de
/// <c>ReservaService.DiasConferenciaAntiga</c> dias.
/// </summary>
public record ReservaDto(int ContaId, string NomeConta, string Nome, decimal Saldo, decimal Guardado, decimal Retirado,
    int Movimentos, DateOnly? PrimeiroMovimento, DateOnly? UltimoMovimento, decimal? SaldoConferido,
    DateOnly? DataConferencia, bool PodeTerSaldoAnterior, bool ConferenciaAntiga);

/// <summary>Saldo real da reserva conferido no app da instituição. <c>Data</c> vazia = hoje.</summary>
public record ConferirReservaDto(
    int ContaId,
    [Required, StringLength(100)] string Nome,
    [Range(0, 999999999999)] decimal Saldo,
    DateOnly? Data);

/// <summary>Preço, taxas e preço atual na moeda do ativo; <c>ValorTotal</c>, <c>ValorAtual</c> e Selic em reais.</summary>
public record OperacaoAtivoDto(int Id, int AtivoId, string Ticker, TipoOperacao Tipo, DateOnly Data,
    decimal Quantidade, decimal PrecoUnitario, decimal Taxas, string Moeda, decimal Cambio, decimal ValorTotal,
    string? Observacao,
    decimal? PrecoAtual, decimal? VariacaoPreco, decimal? ValorAtual, decimal ValorSeFosseSelic,
    OrigemOperacao Origem);

/// <summary>
/// Preço e taxas na moeda do ativo. <c>Moeda</c> só vale no primeiro lançamento do ativo (padrão BRL = B3).
/// <c>Cambio</c> = reais por 1 unidade da moeda na operação; ignorado em BRL e, se vier vazio, usa a PTAX da data.
/// </summary>
public record SalvarOperacaoDto(
    [Required, StringLength(12, MinimumLength = 1)] string Ticker,
    TipoAtivo? TipoAtivo,
    TipoOperacao Tipo,
    DateOnly Data,
    [Range(0.000001, 1000000000.0)] decimal Quantidade,
    [Range(0.0, 100000000.0)] decimal PrecoUnitario,
    [Range(0.0, 10000000.0)] decimal Taxas,
    [StringLength(300)] string? Observacao,
    [StringLength(3)] string? Moeda = null,
    [Range(0.000001, 100000.0)] decimal? Cambio = null);

public record AtualizarAtivoDto(
    [StringLength(150)] string? Nome,
    TipoAtivo Tipo,
    [StringLength(40)] string? Segmento,
    bool PrecoManual,
    [Range(0.0, 100000000.0)] decimal? PrecoAtual);

public record ResultadoCotacoesDto(int Atualizados, int ProventosNovos, List<string> Falhas);

public record ConfiguracaoCotacaoDto(bool TokenBrapiConfigurado);

public record SalvarConfiguracaoCotacaoDto([StringLength(200)] string? TokenBrapi, bool Remover);

/* ----------------------------- FIIs ----------------------------- */
public record FiiDto(int Id, string Ticker, string? Nome, string? Segmento, bool SegmentoSugerido,
    decimal Quantidade, decimal PrecoMedio, decimal Custo, decimal? PrecoAtual, decimal? VariacaoDia,
    decimal ValorMercado, decimal Valorizacao, decimal? ValorizacaoPercentual,
    decimal? UltimoRendimento, DateOnly? DataComUltimo, decimal RendimentosPorCota12Meses,
    decimal? DyMensal, decimal? Dy12Meses, decimal? YieldOnCost12Meses, decimal? YieldOnCostMensal,
    decimal RendaMensalEstimada, decimal Proventos12Meses, decimal ProventosTotal, decimal RecebidoExtrato12Meses,
    decimal PesoCarteira, string? FonteCotacao, bool PrecoManual);

/// <summary>Cada compra de FII comparada com a cotação e a renda de hoje.</summary>
public record LoteFiiDto(int OperacaoId, int AtivoId, string Ticker, DateOnly Data, decimal Quantidade,
    decimal PrecoPago, decimal ValorPago, decimal? PrecoAtual, decimal? ValorAtual, decimal? VariacaoPercentual,
    decimal? DyNaCompra, decimal? YieldOnCost12Meses, decimal? DyAtual12Meses, decimal RendaMensalLote,
    decimal ProventosRecebidosLote, OrigemOperacao Origem);

public record ValorAtivoDto(string Ticker, decimal Valor);

public record ProventoMesDto(int Ano, int Mes, decimal Total, decimal RecebidoExtrato, List<ValorAtivoDto> PorAtivo);

public record EvolucaoFiiMesDto(int Ano, int Mes, decimal Aplicado, decimal? ValorMercado);

public record RentabilidadeMesDto(int Ano, int Mes, decimal Carteira, decimal Selic, decimal? Ifix);

public record ResumoFiisDto(decimal ValorMercado, decimal Custo, decimal Valorizacao, decimal? ValorizacaoPercentual,
    decimal Renda12Meses, decimal RendaMediaMensal, decimal RendaMensalEstimada, decimal? Dy12Meses,
    decimal? YieldOnCost12Meses, decimal RecebidoExtrato12Meses, decimal? RentabilidadeTotal,
    decimal? RentabilidadeSelic, decimal? RentabilidadeIfix, int QuantidadeFiis);

public record PontoPrecoDto(int Ano, int Mes, decimal Fechamento);

public record HistoricoPrecoDto(string Ticker, List<PontoPrecoDto> Pontos);

public record PainelFiisDto(ResumoFiisDto Resumo, List<FiiDto> Fiis, List<LoteFiiDto> Lotes,
    List<ProventoMesDto> Proventos, List<EvolucaoFiiMesDto> Evolucao, List<RentabilidadeMesDto> Rentabilidade,
    List<DistribuicaoInvestimentoDto> Segmentos, List<HistoricoPrecoDto> Historicos, List<string> SegmentosDisponiveis);

/* ----------------------------- Importação pelo extrato ----------------------------- */
/// <summary>Origem: "extrato", "quantidade do extrato", "preço do extrato", "cotação do dia" ou null (preencher).</summary>
public record SugestaoExtratoDto(long TransacaoId, DateOnly Data, string Descricao, decimal Valor, string ContaNome,
    string Ticker, TipoOperacao Tipo, decimal? Quantidade, decimal? PrecoUnitario, decimal? FechamentoNoDia,
    string? Origem, bool JaImportada, bool PossivelDuplicata);

public record ImportarExtratoItemDto(
    long TransacaoId,
    [Required, StringLength(12, MinimumLength = 3)] string Ticker,
    TipoOperacao Tipo,
    [Range(0.000001, 1000000000.0)] decimal Quantidade,
    [Range(0.0, 100000000.0)] decimal PrecoUnitario,
    [Range(0.0, 10000000.0)] decimal Taxas);

public record ImportarExtratoDto([Required, MinLength(1), MaxLength(500)] List<ImportarExtratoItemDto> Itens);

/// <summary>Resultado de "Registrar compras do Open Finance".</summary>
public record ResultadoImportacaoCarteiraDto(int AtivosCriados, int OperacoesImportadas, List<string> SemHistorico,
    List<string> ComLancamentoManual, List<string> Erros, string? Mensagem);
