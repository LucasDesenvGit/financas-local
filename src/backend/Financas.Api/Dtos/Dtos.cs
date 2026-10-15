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

/* ----------------------------- Investimentos ----------------------------- */
public record InvestimentoDto(int Id, string Nome, string Tipo, string? Subtipo, string? Codigo, string? Emissor,
    string Moeda, decimal Saldo, decimal? ValorAplicado, decimal? Rendimento, decimal? TaxaAnual,
    DateOnly? DataVencimento, string? Status, string? NomeInstituicao, DateTime AtualizadoEm);

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
    decimal RendaMensalMedia, decimal ParcelasMensais, decimal? ComprometimentoParcelas, int MesesAnalisados);

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
    decimal DespesasEmergenciaMes = 0);

/* ----------------------------- Open Finance (Pluggy) ----------------------------- */
public record ConfiguracaoPluggyDto(string? ClientId, bool SecretConfigurado, int IntervaloHoras,
    int DiasRetroativos, bool Habilitada);

public record SalvarConfiguracaoPluggyDto(
    [Required, StringLength(100)] string ClientId,
    [StringLength(200)] string? ClientSecret,
    [Range(1, 48)] int IntervaloHoras,
    [Range(1, 365)] int DiasRetroativos,
    bool Habilitada);

public record ConexaoDto(int Id, string PluggyItemId, string? NomeInstituicao, string? CorPrimaria, string? Status,
    string? StatusExecucao, DateTime? UltimaAtualizacaoInstituicao, DateTime? ConsentimentoExpiraEm,
    DateTime? UltimaSincronizacaoEm, string? UltimoErro, bool Ativa, int QuantidadeContas);

public record AdicionarConexaoDto(
    [Required, RegularExpression("^[0-9a-fA-F-]{20,64}$", ErrorMessage = "Item ID inválido.")] string ItemId);

public record SincronizacaoLogDto(long Id, int? ConexaoBancariaId, string? NomeInstituicao, string Origem,
    DateTime IniciadoEm, DateTime? FinalizadoEm, bool Sucesso, int TransacoesNovas, int TransacoesAtualizadas,
    string? Mensagem);

public record ResultadoSincronizacaoDto(bool Sucesso, int TransacoesNovas, int TransacoesAtualizadas, string? Mensagem);

public record CategoriaPluggyDto(string PluggyCategoriaId, string Descricao, string? PaiDescricao,
    int? CategoriaId, string? CategoriaNome, int QuantidadeTransacoes);

public record MapearCategoriaPluggyDto(int? CategoriaId, bool AplicarExistentes);

public record MensagemDto(string Mensagem);

/* ----------------------------- Carteira de renda variável ----------------------------- */
public record SelicResumoDto(decimal? MetaAnual, decimal? Acumulado12Meses, decimal? AcumuladoAno, DateOnly? AtualizadaAte);

public record UltimoProventoDto(DateOnly DataCom, decimal ValorPorCota);

public record PosicaoAtivoDto(int Id, string Ticker, string? Nome, TipoAtivo Tipo,
    decimal Quantidade, decimal PrecoMedio, decimal Custo,
    decimal? PrecoAtual, decimal? PrecoAnterior, decimal? VariacaoDia,
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

/// <summary>Investimento vindo do Open Finance comparado com a Selic do período.</summary>
public record InvestimentoComparadoDto(int Id, string Nome, string Tipo, string? Subtipo, string? Codigo,
    string? Emissor, string? NomeInstituicao, decimal Saldo, decimal? ValorAplicado, DateOnly? DataAplicacao,
    DateOnly? DataVencimento, decimal? TaxaAnual, decimal? Rentabilidade, decimal? SelicNoPeriodo,
    decimal? PercentualDaSelic, bool RendaVariavel, bool JaNaCarteira, DateTime AtualizadoEm);

public record CarteiraDto(SelicResumoDto Selic, TotaisRendaVariavelDto RendaVariavel, List<PosicaoAtivoDto> Posicoes,
    List<EvolucaoCarteiraDto> Evolucao, List<InvestimentoComparadoDto> OpenFinance, decimal TotalOpenFinance,
    decimal TotalInvestimentosManuais, decimal TotalGeral, List<DistribuicaoInvestimentoDto> Distribuicao,
    DateTime? CotacoesAtualizadasEm, bool CotacoesDesatualizadas, int OpenFinanceNaoRegistrados);

public record OperacaoAtivoDto(int Id, int AtivoId, string Ticker, TipoOperacao Tipo, DateOnly Data,
    decimal Quantidade, decimal PrecoUnitario, decimal Taxas, decimal ValorTotal, string? Observacao,
    decimal? PrecoAtual, decimal? VariacaoPreco, decimal? ValorAtual, decimal ValorSeFosseSelic,
    OrigemOperacao Origem);

public record SalvarOperacaoDto(
    [Required, StringLength(12, MinimumLength = 5)] string Ticker,
    TipoAtivo? TipoAtivo,
    TipoOperacao Tipo,
    DateOnly Data,
    [Range(0.000001, 1000000000.0)] decimal Quantidade,
    [Range(0.0, 100000000.0)] decimal PrecoUnitario,
    [Range(0.0, 10000000.0)] decimal Taxas,
    [StringLength(300)] string? Observacao);

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
    [Required, StringLength(12, MinimumLength = 5)] string Ticker,
    TipoOperacao Tipo,
    [Range(0.000001, 1000000000.0)] decimal Quantidade,
    [Range(0.0, 100000000.0)] decimal PrecoUnitario,
    [Range(0.0, 10000000.0)] decimal Taxas);

public record ImportarExtratoDto([Required, MinLength(1), MaxLength(500)] List<ImportarExtratoItemDto> Itens);

/// <summary>Resultado de "Registrar compras do Open Finance".</summary>
public record ResultadoImportacaoCarteiraDto(int AtivosCriados, int OperacoesImportadas, List<string> SemHistorico,
    List<string> ComLancamentoManual, List<string> Erros, string? Mensagem);
