using System.ComponentModel.DataAnnotations;
using Financas.Api.Entidades;

namespace Financas.Api.Dtos;

/* ---------------------------- Autenticação ---------------------------- */
/// <summary>Versao: a do site, para o gerenciador do Windows mostrar junto com "no ar" (não exige login).</summary>
public record EstadoAutenticacaoDto(bool Configurado, bool Autenticado, string? Login)
{
    public string Versao { get; init; } = Financas.Api.Servicos.VersaoSistema.Atual;
}

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
    int? ConexaoBancariaId, string? NomeInstituicao, DateTime AtualizadoEm,
    // ADR 0024: banco que vale (escolhido, do cartão herdado da conta ou detectado); BancoDefinido = escolhido.
    string? Banco = null, bool BancoDefinido = false, int? ContaVinculadaId = null,
    // Cartão: o que foi lançado no mês atual e a fatura que vence nele (mesma conta do Painel).
    FaturaMesDto? FaturaMes = null,
    // #171: "você reduziu o limite no app?" (null = sem resposta) e a diferença avaliada na resposta.
    bool? LimiteAjustadoNoApp = null, decimal? LimiteDiferencaAvaliada = null);

/// <summary>#171: true = reduzi o limite no app; false = não reduzi (a diferença é compromisso); null = desfazer.</summary>
public record ResponderLimiteAjustadoDto(bool? Ajustado);

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
    bool Arquivada,
    [StringLength(30)] string? Banco = null,
    int? ContaVinculadaId = null);

public record FaturaDto(int Id, DateOnly DataVencimento, DateOnly? DataFechamento, decimal ValorTotal,
    decimal? ValorMinimo, decimal? ValorPago, int QuantidadeTransacoes, string Situacao = "aberta");

/// <summary>Banco que a tela de Contas sabe identificar (ADR 0024).</summary>
public record BancoDto(string Codigo, string Nome);

/* ----------------------------- Transações ----------------------------- */
public record TransacaoDto(
    long Id, int ContaId, string ContaNome, bool OpenFinance, DateOnly Data, string Descricao,
    string? DescricaoOriginal, string? Estabelecimento, decimal Valor, TipoTransacao Tipo, StatusTransacao Status,
    int? CategoriaId, string? CategoriaNome, string? CategoriaCor, short? ParcelaNumero, short? ParcelaTotal,
    bool IgnorarRelatorios, string? Observacao, bool Emergencia,
    // #111: como o dinheiro entrou ou saiu (calculado; ADR 0025).
    TipoLancamento TipoLancamento = TipoLancamento.Outro,
    // #123: o nome mostrado veio de uma regra de descrição (null = do banco ou escolhido à mão).
    int? RegraDescricaoId = null);

/// <summary>
/// #123: prévia de "usar também nas outras": quantas transações de nome automático (fora esta) têm a descrição do
/// banco que bate com o texto, com exemplos. TextoSugerido: a descrição do banco sem os pedaços com número.
/// </summary>
public record PreviaRenomearDto(string? DescricaoOriginal, string TextoSugerido, int Quantidade, List<string> Exemplos);

/// <summary>
/// #123: novo nome de uma transação. VoltarOriginal: volta ao nome do banco (e as regras não mexem mais nela).
/// AplicarNasOutras: cria a regra "quando a descrição do banco for/contiver Texto, mostrar Descricao", que vale nas
/// outras transações de nome automático e nas próximas. Sem isso, o nome vale só nesta (escolhido à mão).
/// </summary>
public record RenomearTransacaoDto(
    [StringLength(300)] string? Descricao,
    bool VoltarOriginal,
    bool AplicarNasOutras,
    TipoCorrespondencia TipoCorrespondencia = TipoCorrespondencia.Igual,
    [StringLength(200)] string? Texto = null);

/// <summary><c>Desfazer</c>: chave para voltar tudo (vale por alguns minutos).</summary>
public record ResultadoRenomearDto(int Alteradas, int? RegraId, string? Desfazer);

/// <summary>
/// #123: o mesmo nome em várias transações. CriarRegras: uma regra "é igual a" por descrição do banco das
/// selecionadas (a que já existe com esse texto passa a usar o nome novo), que vale também nas outras de nome
/// automático e nas próximas. Sem isso (ou para lançamento manual), o nome fica escolhido à mão.
/// </summary>
public record RenomearEmLoteDto(
    [Required, MinLength(1), MaxLength(1000)] List<long> Ids,
    [Required, StringLength(300, MinimumLength = 1)] string Descricao,
    bool CriarRegras);

public record ResultadoRenomearEmLoteDto(int Alteradas, int RegrasCriadas, int RegrasAlteradas, string? Desfazer);

/// <summary>#123: prévia de uma regra de descrição nova (quantas transações de nome automático ela renomeia).</summary>
public record PreviaRegraDescricaoDto(int Quantidade, List<string> Exemplos);

/// <summary>#123: regra de descrição, com quantas transações estão com o nome dado por ela.</summary>
public record RegraDescricaoDto(int Id, string Texto, TipoCorrespondencia TipoCorrespondencia, string NovaDescricao,
    bool Ativa, int Transacoes, decimal? ValorMinimo = null, decimal? ValorMaximo = null);

/// <summary>#241: <c>ValorMinimo</c>/<c>ValorMaximo</c> (opcionais): a regra só vale com o valor, sem o sinal, na faixa.</summary>
public record SalvarRegraDescricaoDto(
    [Required, StringLength(200, MinimumLength = 2)] string Texto,
    TipoCorrespondencia TipoCorrespondencia,
    [Required, StringLength(300, MinimumLength = 1)] string NovaDescricao,
    bool Ativa = true,
    [Range(0.01d, 999999999d)] decimal? ValorMinimo = null,
    [Range(0.01d, 999999999d)] decimal? ValorMaximo = null);

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
    /// <summary>
    /// Natureza do gasto (regra 50/30/20): a da categoria ou, se ela não tem, a da categoria pai (a mesma regra da
    /// Visão anual). Com <see cref="SemNatureza"/>, soma também as sem classificação (inclusive sem categoria).
    /// </summary>
    [MaxLength(2)] public List<NaturezaGasto>? Naturezas { get; set; }
    public bool SemNatureza { get; set; }
    public bool SomenteEmergencias { get; set; }
    /// <summary>#9: só as de descrição pouco informativa ainda não revisadas.</summary>
    public bool PoucoInformativa { get; set; }
    /// <summary>#111: grupo do filtro "Lançamento" (pix, cartao, fatura, entre-contas…).</summary>
    [RegularExpression("^(cartao|debito|pix|transferencia|entre-contas|fatura|boleto|saque-deposito|tarifas|investimentos|salario|credito|dinheiro|outros)$")]
    public string? Lancamento { get; set; }
    public TipoTransacao? Tipo { get; set; }
    [StringLength(100)] public string? Busca { get; set; }
    /// <summary>#173 d): só estes lançamentos (?ids=1&amp;ids=2), vindos do detalhe de outra tela.</summary>
    [MaxLength(200)] public List<long>? Ids { get; set; }
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

/// <summary>#9: marca (ou desmarca) a descrição pouco informativa como revisada.</summary>
public record RevisadaLoteDto([Required, MinLength(1), MaxLength(1000)] List<long> Ids, bool Revisada);

/// <summary>#87: categoria de uma transação no modo edição (Automatica = voltar para as regras).</summary>
public record AlteracaoCategoriaDto(long Id, int? CategoriaId, bool Automatica);

/// <summary>
/// #87: alteração de categoria em lote. Ou <c>Itens</c> (cada uma com a sua categoria, até 1000), ou <c>Filtro</c>
/// (todas as transações da busca, com <c>CategoriaId</c>/<c>Automatica</c>). Só a categoria pode mudar por aqui.
/// </summary>
public class AlterarCategoriasEmLoteDto
{
    [MaxLength(1000)] public List<AlteracaoCategoriaDto>? Itens { get; set; }
    public FiltroTransacoesDto? Filtro { get; set; }
    public int? CategoriaId { get; set; }
    public bool Automatica { get; set; }
}

/// <summary><c>Desfazer</c>: chave para restaurar o que mudou (vale por alguns minutos; null se nada mudou).</summary>
public record ResultadoCategoriasEmLoteDto(int Alteradas, string? Desfazer);

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

/// <summary>
/// Meta. Com cofrinho (<c>ReservaNome</c>), o progresso é o valor inicial + <c>SaldoReserva</c> (nunca o saldo da
/// conta junto); <c>ReservaConferir</c> = saldo só estimado, vale conferir; <c>ReservaNaoEncontrada</c> = o cofrinho
/// não aparece mais no extrato (nada é somado e o vínculo não é apagado).
/// </summary>
public record MetaDto(int Id, string Nome, decimal ValorAlvo, decimal ValorAtual, DateOnly? DataAlvo,
    int? ContaId, string? ContaNome, string Cor, bool Concluida, decimal Percentual, decimal? AporteMensalSugerido,
    decimal ValorInicial, decimal SaldoConta, string? ReservaNome = null, decimal? SaldoReserva = null,
    bool ReservaConferir = false, bool ReservaNaoEncontrada = false, int Prioridade = 0);

/// <summary>Nova ordem de prioridade das metas (a primeira da lista é a 1ª prioridade).</summary>
public record PrioridadesMetasDto([Required, MinLength(1), MaxLength(200)] List<int> Ids);

/// <summary>
/// Um mês do caixa (#149): receitas e despesas das contas de dinheiro, faturas de cartão que vencem no mês (com a parte
/// que é parcela) e a sobra = receitas − despesas − faturas.
/// </summary>
public record MesCaixaDto(short Ano, byte Mes, decimal Receitas, decimal Despesas, decimal Faturas, decimal Parcelas, decimal Sobra);

/// <summary>
/// O que o plano das metas precisa (#90, #149, ADR 0035); a divisão entre as metas é feita na tela. <c>Disponivel</c> =
/// false com menos de 3 meses completos de extrato (só a divisão igual). <c>Meses</c>: o caixa a partir do mês atual.
/// <c>SaldoFimDoMes</c>: saldo previsto nas contas de dinheiro no fim do mês, já pagas as faturas que vencem nele
/// (<c>FaturaAPagar</c> é a parte delas que ainda não saiu). <c>MelhorMesRecente</c>: a maior sobra de verdade (pelo
/// caixa) entre os meses completos considerados.
/// </summary>
public record PlanoMetasDto(bool Disponivel, int MesesConsiderados, short Ano, byte Mes, int MargemPercentual,
    decimal SaldoContas, decimal FaturaAPagar, decimal SaldoFimDoMes, decimal MelhorMesRecente, List<MesCaixaDto> Meses);

public record SalvarMetaDto(
    [Required, StringLength(120)] string Nome,
    [Range(0.01d, 999999999d)] decimal ValorAlvo,
    [Range(0d, 999999999d)] decimal ValorAtual,
    DateOnly? DataAlvo,
    int? ContaId,
    [Required, RegularExpression("^#[0-9a-fA-F]{6}$")] string Cor,
    bool Concluida,
    [StringLength(100)] string? ReservaNome = null);

public record AporteDto([Range(-999999999d, 999999999d)] decimal Valor);

public record ContaAgendadaDto(int Id, string Descricao, decimal Valor, DateOnly ProximoVencimento,
    Recorrencia Recorrencia, int? CategoriaId, string? CategoriaNome, int? ContaId, string? ContaNome,
    bool Ativa, DateOnly? UltimoPagamentoEm, int DiasParaVencer,
    // #69: até quando se repete; pagamentos que faltam até lá (contando o próximo); encerrada por ter passado dela.
    DateOnly? DataLimite = null, int? PagamentosRestantes = null, bool Encerrada = false,
    // #158: como é cobrada, o texto que a liga às cobranças e a observação.
    FormaCobranca FormaCobranca = FormaCobranca.AMao, string? TextoCobranca = null, string? Observacao = null);

public record SalvarContaAgendadaDto(
    [Required, StringLength(150)] string Descricao,
    [Range(0.01d, 999999999d)] decimal Valor,
    DateOnly ProximoVencimento,
    Recorrencia Recorrencia,
    int? CategoriaId,
    int? ContaId,
    bool Ativa = true,
    DateOnly? DataLimite = null,
    FormaCobranca FormaCobranca = FormaCobranca.AMao,
    [StringLength(100, ErrorMessage = "O texto da cobrança pode ter até 100 caracteres.")] string? TextoCobranca = null,
    [StringLength(300, ErrorMessage = "A observação pode ter até 300 caracteres.")] string? Observacao = null,
    // #205: a chave da sugestão do extrato que originou o cadastro. Vazia na edição: a ligação que já existe fica.
    [StringLength(150)] string? ChaveExtrato = null);

public record PagarContaAgendadaDto(bool LancarTransacao, DateOnly? Data);

/// <summary>#205: "é a mesma": liga a conta a pagar à sugestão do extrato (a <c>ChaveExtrato</c> da sugestão).</summary>
public record LigarExtratoContaAgendadaDto([Required, StringLength(150)] string ChaveExtrato);

/// <summary>
/// Pagamento mensal encontrado no extrato que ainda não está em Contas a pagar. <c>ProximoVencimento</c> nunca é
/// passado; <c>VencimentoSemPagamento</c> é o vencimento esperado que já passou sem o pagamento aparecer no extrato.
/// </summary>
/// <summary>
/// #158: aviso de uma conta a pagar recorrente. <c>Tipo</c>: "nao-veio", "mudou-de-cartao", "aumentou" ou
/// "cartao-arquivado". <c>ValorNovo</c> e <c>ContaNova*</c> vão com a ação "Atualizar" (o valor e a conta da última
/// cobrança). O texto não leva valores em reais (a tela mostra <c>ValorNovo</c> pelo pipe de "ocultar valores").
/// </summary>
public record AvisoContaAgendadaDto(int ContaAgendadaId, string Descricao, string Tipo, string Texto,
    decimal? ValorAnterior = null, decimal? ValorNovo = null, int? ContaNovaId = null, string? ContaNovaNome = null,
    DateOnly? Data = null);

public record SugestaoContaAgendadaDto(string Chave, string Descricao, decimal Valor, bool ValorVariavel,
    decimal ValorMinimo, decimal ValorMaximo, DateOnly ProximoVencimento, int DiaHabitual, int Meses, int Pagamentos,
    DateOnly UltimoPagamento, DateOnly PrimeiroPagamento, int? CategoriaId, string? CategoriaNome, int ContaId,
    string ContaNome, string TermoBusca, DateOnly? VencimentoSemPagamento,
    // #158: como parece ser cobrada, o texto que a ligaria às cobranças e se é mensal ou anual.
    FormaCobranca FormaCobranca = FormaCobranca.AMao, string? TextoCobranca = null, Recorrencia Recorrencia = Recorrencia.Mensal,
    // #205: a chave da descrição do banco (gravada na conta a pagar ao cadastrar ou no "é a mesma") e, quando há uma
    // conta a pagar ativa parecida (valor, dia e conta) mas não dá para ter certeza, qual é ela.
    string? ChaveExtrato = null, int? PareceSerId = null, string? PareceSerNome = null);

/* ----------------------------- Investimentos ----------------------------- */
/// <summary>Valores na moeda do investimento; <c>SaldoEmReais</c> é null quando não há cotação guardada da moeda.</summary>
public record InvestimentoDto(int Id, string Nome, string Tipo, string? Subtipo, string? Codigo, string? Emissor,
    string Moeda, decimal Saldo, decimal? SaldoEmReais, decimal? ValorAplicado, decimal? Rendimento, decimal? TaxaAnual,
    DateOnly? DataVencimento, string? Status, string? NomeInstituicao, DateTime AtualizadoEm, string NomeExibicao,
    string? NomePersonalizado);

/// <summary>#170: nome escolhido para o investimento; vazio volta ao nome automático.</summary>
public record RenomearInvestimentoDto([MaxLength(120, ErrorMessage = "O nome pode ter até 120 caracteres.")] string? Nome);

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

/// <summary>
/// Um cartão no mês do Painel (UX-12, #144): a fatura que fecha no mês. <c>ValorMes</c> é o total informado pelo banco
/// quando a fatura já fechou; aberta, a soma das transações do ciclo (do fechamento anterior a <c>Fechamento</c>:
/// compras, a parcela que cai nele, estornos descontados). <c>ParcelasMes</c> é a parte do ciclo que é parcela.
/// <c>FaturaId</c> só vem quando a fatura já foi importada; o vencimento é estimado pelo dia de vencimento enquanto não.
/// Situação: "paga" (pagamentos cobrem o total), "fechada" (já fechou ou venceu sem pagamento informado) ou "aberta"
/// (ainda recebendo compras). Sem <c>Fechamento</c>, o cartão não tem dia de fechamento conhecido e o valor é o do mês
/// do calendário. #251: o pagamento vale o que o banco informou ou o lançado no cartão; <c>Proxima</c> é a fatura
/// seguinte, ainda aberta, e só vem quando a do mês já fechou.
/// </summary>
public record FaturaMesDto(int ContaId, string Cartao, decimal ValorMes, decimal ParcelasMes, int? FaturaId,
    DateOnly? Vencimento, string? Situacao, string? Banco = null, DateOnly? Fechamento = null, FaturaMesDto? Proxima = null);

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
    int DiasProximosVencimentos = 15,
    List<FaturaMesDto>? FaturasDoMes = null,
    decimal TotalFaturasDoMes = 0,
    // #9: transações recentes com descrição pouco informativa ainda não revisadas (aviso discreto no Painel).
    int PoucoInformativas = 0,
    DateOnly? PoucoInformativasDesde = null,
    string? ExemploPoucoInformativa = null,
    // #170: trocos e vendas de renda variável contando como receita (aviso com link para "Conferir investimentos").
    int CreditosCorretoraComoReceita = 0,
    decimal TotalCreditosCorretoraComoReceita = 0,
    // #173 b) (ADR 0044): trocos que a sincronização classificou sozinha (aviso com "Ver", "Desfazer" e "Ok").
    TrocosAutomaticosDto? TrocosAutomaticos = null);

/// <summary>#173 b): os trocos classificados na sincronização que ainda estão como ficaram (Desde/Ate: as datas deles).</summary>
public record TrocosAutomaticosDto(int Quantidade, decimal Total, List<long> Ids, DateOnly Desde, DateOnly Ate);

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
    int DiasRetroativos, bool Habilitada,
    // #172: o seu nome como aparece nos Pix (para os bancos que não informam o titular).
    string? MeuNome = null);

/// <summary>#172: o seu nome como aparece nos Pix; vazio apaga.</summary>
public record MeuNomeDto([StringLength(120, ErrorMessage = "O nome pode ter até 120 caracteres.")] string? Nome);

public record SalvarConfiguracaoPluggyDto(
    [Required, StringLength(100)] string ClientId,
    [StringLength(200)] string? ClientSecret,
    // Opcionais: a tela de Parametrização cuida deles; quando omitidos, ficam como estão.
    [Range(1, 48)] int? IntervaloHoras = null,
    [Range(1, 365)] int? DiasRetroativos = null,
    bool? Habilitada = null);

public record ConexaoDto(int Id, string PluggyItemId, string? NomeInstituicao, string? CorPrimaria, string? Status,
    string? StatusExecucao, DateTime? UltimaAtualizacaoInstituicao, DateTime? ConsentimentoExpiraEm,
    DateTime? UltimaSincronizacaoEm, string? UltimoErro, bool Ativa, int QuantidadeContas, string? NomeExibicao = null);

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
    decimal? PercentualDaSelic, bool RendaVariavel, bool JaNaCarteira, DateTime AtualizadoEm, string NomeExibicao,
    string? NomePersonalizado);

public record CarteiraDto(SelicResumoDto Selic, TotaisRendaVariavelDto RendaVariavel, List<PosicaoAtivoDto> Posicoes,
    List<EvolucaoCarteiraDto> Evolucao, List<InvestimentoComparadoDto> OpenFinance, decimal TotalOpenFinance,
    decimal TotalInvestimentosManuais, decimal TotalGeral, List<DistribuicaoInvestimentoDto> Distribuicao,
    DateTime? CotacoesAtualizadasEm, bool CotacoesDesatualizadas, int OpenFinanceNaoRegistrados,
    List<string> MoedasSemCotacao, List<ReservaDto> Reservas, decimal TotalReservas, List<string> CotasDiferentes);

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
    DateOnly? DataConferencia, bool PodeTerSaldoAnterior, bool ConferenciaAntiga, bool InformadoPeloBanco = false,
    // #169: o Méli Dólar é uma reserva em dólar: Quantidade (e SaldoConferido) em US$, Saldo em reais pela Cotacao (PTAX).
    string Moeda = "BRL", decimal? Quantidade = null, decimal? Cotacao = null,
    // #237: a conta tem a lista detalhada do banco e esta reserva do extrato não está nela (continua estimada e conferível).
    bool ForaDaListaDoBanco = false,
    // #237: linha com a parte do total reservado informado pelo banco (TotalDoBanco) que não está nas reservas do extrato.
    bool RestanteDoBanco = false, decimal? TotalDoBanco = null);

/// <summary>Saldo real da reserva conferido no app da instituição. <c>Data</c> vazia = hoje.</summary>
public record ConferirReservaDto(
    int ContaId,
    [Required, StringLength(100)] string Nome,
    [Range(0, 999999999999)] decimal Saldo,
    DateOnly? Data,
    // #169: no Méli Dólar, "BRL" quando o saldo foi informado em reais (convertido pela PTAX da data); senão, em dólar.
    [RegularExpression("^(BRL|USD)$", ErrorMessage = "Moeda inválida: use BRL ou USD.")] string? Moeda = null);

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

/// <summary>
/// #173: o mês da carteira de FIIs como no app da corretora. Aportes = compras (cotas × preço + taxas), sem as
/// subscrições, que vêm à parte. Rendimentos recebidos = créditos de provento no extrato, sem os trocos e vendas.
/// Valorização = saldo no fim − saldo no fim do mês anterior − compras + vendas.
/// </summary>
public record MesAMesFiiDto(int Ano, int Mes, decimal Aportes, decimal Subscricoes, decimal Resgates,
    decimal RendimentosRecebidos, decimal RendimentosEstimados, decimal Valorizacao, decimal Saldo,
    // #173 c): os rendimentos recebidos por fundo (Ticker null = crédito que não deu para ligar a nenhum fundo).
    List<RendimentoPorFiiDto>? PorFii = null,
    // #173 d): os lançamentos que formam os aportes (compras, subscrições e vendas ligadas a um lançamento do banco) e
    // os rendimentos do mês, para o detalhe em Transações.
    List<long>? IdsAportes = null, List<long>? IdsRendimentos = null);

/// <summary>#173 c): quanto um fundo pagou no mês (Ticker null: "sem fundo identificado").</summary>
public record RendimentoPorFiiDto(string? Ticker, decimal Valor);

public record RentabilidadeMesDto(int Ano, int Mes, decimal Carteira, decimal Selic, decimal? Ifix);

public record ResumoFiisDto(decimal ValorMercado, decimal Custo, decimal Valorizacao, decimal? ValorizacaoPercentual,
    decimal Renda12Meses, decimal RendaMediaMensal, decimal RendaMensalEstimada, decimal? Dy12Meses,
    decimal? YieldOnCost12Meses, decimal RecebidoExtrato12Meses, decimal? RentabilidadeTotal,
    decimal? RentabilidadeSelic, decimal? RentabilidadeIfix, int QuantidadeFiis);

public record PontoPrecoDto(int Ano, int Mes, decimal Fechamento);

public record HistoricoPrecoDto(string Ticker, List<PontoPrecoDto> Pontos);

public record PainelFiisDto(ResumoFiisDto Resumo, List<FiiDto> Fiis, List<LoteFiiDto> Lotes,
    List<ProventoMesDto> Proventos, List<EvolucaoFiiMesDto> Evolucao, List<RentabilidadeMesDto> Rentabilidade,
    List<DistribuicaoInvestimentoDto> Segmentos, List<HistoricoPrecoDto> Historicos, List<string> SegmentosDisponiveis,
    List<MesAMesFiiDto> MesAMes);

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

/* ----------------------------- Histórico dos investimentos do Open Finance (#173 e, ADR 0045) ----------------------------- */

/// <summary>Rendimento de cada mês dos investimentos do Open Finance (renda fixa e fundos); <c>Desde</c>: o primeiro registro.</summary>
public record HistoricoInvestimentosDto(DateOnly? Desde, List<MesInvestimentosOfDto> Meses);

/// <summary>
/// Um mês: o rendimento (soma dos investimentos com valor aplicado; null se nenhum tem) e o saldo no fim. <c>Desde</c>:
/// o mês em que o histórico começou conta a partir desse dia.
/// </summary>
public record MesInvestimentosOfDto(int Ano, int Mes, DateOnly? Desde, decimal? Rendimento, decimal Saldo,
    List<RendimentoInvestimentoMesDto> PorInvestimento);

public record RendimentoInvestimentoMesDto(string Nome, decimal? Rendimento, decimal Saldo);

/* ----------------------------- Conferir investimentos (ADR 0027) ----------------------------- */

/// <summary>O que parece errado na carteira e nas transações de renda variável, com a correção sugerida.</summary>
public record ConferenciaInvestimentosDto(List<TipoParaCorrigirDto> Tipos, List<CotasDiferentesDto> Cotas,
    List<CompraSemOperacaoDto> ComprasSemOperacao, List<CreditoDaCorretoraDto> Trocos,
    List<RendimentoComoInvestimentoDto> Rendimentos, CategoriaDoTrocoDto? CategoriaDoTroco, List<TaxaB3Dto> Taxas);

/// <summary>
/// #173 a): a taxa da B3 de um dia de compras, pelo extrato: o que saiu da conta (<c>Debitado</c>) menos o troco que
/// voltou (<c>Trocos</c>) menos cotas × preço (<c>ValorDasCotas</c>). Dividida entre as compras do dia pelo valor.
/// </summary>
public record TaxaB3Dto(DateOnly Data, string Conta, decimal Debitado, decimal Trocos, decimal ValorDasCotas, decimal Taxa,
    List<TaxaB3OperacaoDto> Operacoes);

public record TaxaB3OperacaoDto(int OperacaoId, string Ticker, decimal Quantidade, decimal Taxa);

/// <summary>Ativo cadastrado como ação ou "outro" que é FII (Motivo: o nome do fundo ou o emissor da corretora).</summary>
public record TipoParaCorrigirDto(int AtivoId, string Ticker, string? Nome, TipoAtivo TipoAtual, string Motivo);

/// <summary>
/// Cotas na corretora (posição da última sincronização, em <c>PosicaoEm</c>) × na carteira. Quando faltam cotas, o
/// débito de compra sem operação sugerido, com o preço (débito − troco ÷ cotas) e o fechamento daquele dia.
/// </summary>
public record CotasDiferentesDto(int AtivoId, string Ticker, decimal NaCorretora, decimal NaCarteira, DateTime PosicaoEm,
    long? DebitoSugeridoId, decimal? PrecoSugerido, decimal? FechamentoNoDia);

/// <summary>Débito de compra de renda variável que nenhuma operação da carteira explica (Troco: o que voltou dele).</summary>
public record CompraSemOperacaoDto(long TransacaoId, DateOnly Data, decimal Valor, decimal Troco, string Conta, string Descricao);

/// <summary>
/// Crédito da corretora que é troco de uma compra (<c>Compra*</c>: o débito) ou o valor de uma venda (<c>Venda*</c>),
/// mas está contando como receita (rendimento).
/// </summary>
public record CreditoDaCorretoraDto(long TransacaoId, DateOnly Data, decimal Valor, string Descricao, string Conta,
    string? Categoria, DateOnly? CompraData, decimal? CompraValor, string? VendaTicker, decimal? VendaQuantidade,
    DateOnly? VendaData);

/// <summary>Provento (crédito da corretora que não é troco nem venda) numa categoria de transferência.</summary>
public record RendimentoComoInvestimentoDto(long TransacaoId, DateOnly Data, decimal Valor, string Descricao, string Conta,
    string? Categoria, string CategoriaAutomatica);

public record CategoriaDoTrocoDto(int Id, string Nome);

/// <summary>
/// Correções escolhidas: ativos que viram FII, compras que faltaram (ligadas ao débito), trocos e vendas (para a
/// categoria da compra), rendimentos (de volta para a categoria automática) e, #173 a), as operações que recebem a
/// taxa da B3 (o valor é recalculado na API, não vem da tela).
/// </summary>
public record CorrigirInvestimentosDto(
    [MaxLength(200)] List<int>? AtivosParaFii,
    [MaxLength(200)] List<ImportarExtratoItemDto>? Compras,
    [MaxLength(2000)] List<long>? Trocos,
    [MaxLength(2000)] List<long>? Rendimentos,
    [MaxLength(2000)] List<int>? Taxas = null);

/// <summary><c>Desfazer</c>: chave para voltar tudo (vale por alguns minutos; null se nada mudou).</summary>
public record ResultadoCorrecaoInvestimentosDto(int Tipos, int Compras, int Trocos, int Rendimentos, int Taxas, string? Desfazer);

/* ----------------------------- Erros do sistema (#11) ----------------------------- */
public class FiltroErrosDto
{
    public DateOnly? Inicio { get; set; }
    public DateOnly? Fim { get; set; }
    [StringLength(30)] public string? Origem { get; set; }
    public NivelErro? Nivel { get; set; }
    /// <summary>abertos (padrão), resolvidos ou todos.</summary>
    [RegularExpression("^(abertos|resolvidos|todos)$")] public string? Situacao { get; set; }
    [Range(1, int.MaxValue)] public int Pagina { get; set; } = 1;
    [Range(1, 100)] public int TamanhoPagina { get; set; } = 25;
}

/// <summary>Datas em UTC. Sem a pilha (vem por <see cref="ErroSistemaDetalheDto"/>, só ao abrir).</summary>
public record ErroSistemaDto(long Id, DateTime PrimeiraEm, DateTime UltimaEm, int Ocorrencias, NivelErro Nivel,
    string Origem, string Mensagem, string? Rota, string? Usuario, bool Resolvido, DateTime? ResolvidoEm);

public record ErroSistemaDetalheDto(long Id, string Categoria, string? Detalhe);

/// <summary><c>Abertos</c>: total em aberto (sem filtro), para o contador da aba.</summary>
public record PaginaErrosDto(List<ErroSistemaDto> Itens, int Total, int Pagina, int TamanhoPagina, int Abertos,
    List<string> Origens);

public record SituacaoErroDto(bool Resolvido);

public record QuantidadeDto(int Quantidade);

/// <summary>Erro não tratado no navegador. A API limpa dados sensíveis e limita a quantidade por hora.</summary>
public record ErroFrontEndDto(
    [Required, StringLength(1000)] string Mensagem,
    [StringLength(8000)] string? Detalhe,
    [StringLength(300)] string? Rota);

/* ----------------------------- Conferir com o banco (#172) ----------------------------- */

/// <summary>
/// O que pode explicar a diferença entre o site e o app do banco. <c>Valor</c>, quando há, é comparado com a diferença
/// na tela; <c>Rota</c> e <c>Parametros</c> levam à tela que mostra o detalhe.
/// </summary>
public record PistaConferenciaDto(string Texto, decimal? Valor, string? Rota = null, Dictionary<string, string>? Parametros = null);

public record ContaConferenciaDto(int Id, string Nome, string? Banco, bool OpenFinance, decimal Saldo,
    List<PistaConferenciaDto> Pistas);

/// <summary>A fatura que fecha no mês (a mesma do Painel) e a dívida total do cartão.</summary>
public record CartaoConferenciaDto(int Id, string Nome, string? Banco, decimal FaturaMes, string? Situacao,
    DateOnly? Fechamento, DateOnly? Vencimento, decimal DividaTotal, List<PistaConferenciaDto> Pistas);

/// <summary>Investimento do Open Finance, reserva ou a carteira lançada por você; <c>Saldo</c> em reais.</summary>
public record InvestimentoConferenciaDto(string Chave, string Nome, string Origem, decimal Saldo, List<PistaConferenciaDto> Pistas);

public record ConferenciaBancoDto(List<ContaConferenciaDto> Contas, List<CartaoConferenciaDto> Cartoes,
    List<InvestimentoConferenciaDto> Investimentos);
