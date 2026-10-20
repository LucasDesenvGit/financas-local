namespace Financas.Api.Entidades;

public class Categoria
{
    public int Id { get; set; }
    public string Nome { get; set; } = string.Empty;
    public TipoCategoria Tipo { get; set; }
    public string Cor { get; set; } = "#64748b";
    public string? Icone { get; set; }
    public int? CategoriaPaiId { get; set; }
    public bool Ativa { get; set; } = true;
    /// <summary>Só para despesas: essencial ou estilo de vida (regra 50/30/20).</summary>
    public NaturezaGasto? Natureza { get; set; }

    public Categoria? CategoriaPai { get; set; }
    public List<Categoria> Subcategorias { get; set; } = new();
}

/// <summary>
/// #123 (ADR 0030): "quando a descrição do banco for/contiver <see cref="Texto"/>, mostrar <see cref="NovaDescricao"/>".
/// </summary>
public class RegraDescricao
{
    public int Id { get; set; }
    public string Texto { get; set; } = string.Empty;
    public TipoCorrespondencia TipoCorrespondencia { get; set; } = TipoCorrespondencia.Igual;
    public string NovaDescricao { get; set; } = string.Empty;
    public bool Ativa { get; set; } = true;
    public DateTime CriadoEm { get; set; } = DateTime.UtcNow;
}

public class RegraCategorizacao
{
    public int Id { get; set; }
    public string Texto { get; set; } = string.Empty;
    public TipoCorrespondencia TipoCorrespondencia { get; set; } = TipoCorrespondencia.Contem;
    public int CategoriaId { get; set; }
    public int Prioridade { get; set; } = 100;
    public bool Ativa { get; set; } = true;
    public DateTime CriadoEm { get; set; } = DateTime.UtcNow;

    public Categoria? Categoria { get; set; }
}

public class Conta
{
    public int Id { get; set; }
    public int? ConexaoBancariaId { get; set; }
    public string? PluggyAccountId { get; set; }
    public string Nome { get; set; } = string.Empty;
    public TipoConta Tipo { get; set; }
    public OrigemConta Origem { get; set; }
    public string? Numero { get; set; }
    public string Moeda { get; set; } = "BRL";
    public decimal SaldoInicial { get; set; }
    /// <summary>Saldo informado pela instituição (contas Open Finance). Para cartão é negativo (dívida).</summary>
    public decimal Saldo { get; set; }
    public decimal? LimiteCredito { get; set; }
    public decimal? LimiteDisponivel { get; set; }
    /// <summary>
    /// #171: resposta a "você reduziu o limite no app?" quando o limite usado (aprovado − disponível) é bem maior que a
    /// dívida. true = a diferença é limite tirado por você; false = é compromisso (compras em processamento,
    /// parcelas); null = ainda sem resposta.
    /// </summary>
    public bool? LimiteAjustadoNoApp { get; set; }
    /// <summary>#171: a diferença (limite usado − dívida) quando você respondeu; o "não" volta a perguntar se ela mudar.</summary>
    public decimal? LimiteDiferencaAvaliada { get; set; }
    public byte? DiaFechamento { get; set; }
    public byte? DiaVencimento { get; set; }
    public string Cor { get; set; } = "#0f766e";
    public bool IncluirNoSaldo { get; set; } = true;
    public bool Arquivada { get; set; }
    /// <summary>ADR 0024: banco escolhido (código do <c>BancoService</c>); null = detectar pelo nome.</summary>
    public string? Banco { get; set; }
    /// <summary>ADR 0024: para cartão, a conta a que ele pertence (de onde a fatura é paga).</summary>
    public int? ContaVinculadaId { get; set; }
    /// <summary>
    /// #168 (ADR 0010): dinheiro separado em reservas/caixinhas que a instituição informa pelo Open Finance
    /// (<c>bankData</c>); null = não informa, e as reservas são estimadas pelo extrato.
    /// </summary>
    public decimal? SaldoReservado { get; set; }
    /// <summary>#168: reservas com nome e valor, quando a instituição detalha (JSON: [{"nome":"…","valor":0.0}]).</summary>
    public string? ReservasInformadas { get; set; }
    public DateTime CriadoEm { get; set; } = DateTime.UtcNow;
    public DateTime AtualizadoEm { get; set; } = DateTime.UtcNow;

    public ConexaoBancaria? ConexaoBancaria { get; set; }
    public List<Transacao> Transacoes { get; set; } = new();
    public List<FaturaCartao> Faturas { get; set; } = new();
}

public class FaturaCartao
{
    public int Id { get; set; }
    public int ContaId { get; set; }
    public string? PluggyBillId { get; set; }
    public DateOnly DataVencimento { get; set; }
    public DateOnly? DataFechamento { get; set; }
    public decimal ValorTotal { get; set; }
    public decimal? ValorMinimo { get; set; }
    public decimal? ValorPago { get; set; }
    public DateTime AtualizadoEm { get; set; } = DateTime.UtcNow;

    public Conta? Conta { get; set; }
}

public class Transacao
{
    public long Id { get; set; }
    public int ContaId { get; set; }
    public string? PluggyTransactionId { get; set; }
    public DateOnly Data { get; set; }
    public string Descricao { get; set; } = string.Empty;
    public string? DescricaoOriginal { get; set; }
    public string? Estabelecimento { get; set; }
    public decimal Valor { get; set; }
    public TipoTransacao Tipo { get; set; }
    public StatusTransacao Status { get; set; } = StatusTransacao.Efetivada;
    public int? CategoriaId { get; set; }
    public bool CategoriaManual { get; set; }
    public string? PluggyCategoriaId { get; set; }
    public int? FaturaCartaoId { get; set; }
    public short? ParcelaNumero { get; set; }
    public short? ParcelaTotal { get; set; }
    public bool IgnorarRelatorios { get; set; }
    /// <summary>Gasto de emergência (destacado nos relatórios e orçamentos).</summary>
    public bool Emergencia { get; set; }
    /// <summary>#9: descrição pouco informativa já revisada pelo usuário (sai do filtro).</summary>
    public bool DescricaoRevisada { get; set; }
    /// <summary>#111: tipo de operação que o banco informou (ex.: PIX, BOLETO, PAGAMENTO_FATURA); null = não informou.</summary>
    public string? MeioPagamento { get; set; }
    /// <summary>#111: pagador e recebedor com o mesmo documento (só o sim/não; o documento não é guardado).</summary>
    public bool? MesmaTitularidade { get; set; }
    /// <summary>#123: regra de descrição que deu o nome mostrado (null = nome do banco ou escolhido à mão).</summary>
    public int? RegraDescricaoId { get; set; }
    /// <summary>#123: nome escolhido à mão (ou "voltar à do banco"): as regras de descrição não mexem mais.</summary>
    public bool DescricaoManual { get; set; }
    public Guid? TransferenciaGrupo { get; set; }
    public string? Observacao { get; set; }
    public DateTime CriadoEm { get; set; } = DateTime.UtcNow;
    public DateTime AtualizadoEm { get; set; } = DateTime.UtcNow;

    public Conta? Conta { get; set; }
    public Categoria? Categoria { get; set; }
    public FaturaCartao? FaturaCartao { get; set; }
}

public class Orcamento
{
    public int Id { get; set; }
    public int CategoriaId { get; set; }
    public short Ano { get; set; }
    public byte Mes { get; set; }
    public decimal Valor { get; set; }

    public Categoria? Categoria { get; set; }
}

public class Meta
{
    public int Id { get; set; }
    public string Nome { get; set; } = string.Empty;
    public decimal ValorAlvo { get; set; }
    public decimal ValorAtual { get; set; }
    public DateOnly? DataAlvo { get; set; }
    public int? ContaId { get; set; }
    /// <summary>Cofrinho da conta que guarda o dinheiro da meta (nome, como no ReservaService). Null = saldo da conta.</summary>
    public string? ReservaNome { get; set; }
    /// <summary>Ordem de prioridade (1 = primeira) no assistente de metas; 0 = ainda sem ordem (vai para o fim).</summary>
    public int Prioridade { get; set; }
    public string Cor { get; set; } = "#0f766e";
    public bool Concluida { get; set; }
    public DateTime CriadoEm { get; set; } = DateTime.UtcNow;

    public Conta? Conta { get; set; }
}

public class ContaAgendada
{
    public int Id { get; set; }
    public string Descricao { get; set; } = string.Empty;
    public decimal Valor { get; set; }
    public DateOnly ProximoVencimento { get; set; }
    public Recorrencia Recorrencia { get; set; } = Recorrencia.Mensal;
    public int? CategoriaId { get; set; }
    public int? ContaId { get; set; }
    public bool Ativa { get; set; } = true;
    public DateOnly? UltimoPagamentoEm { get; set; }
    /// <summary>#69: último vencimento que ainda se repete (null = sem fim). Passou dele ao pagar, a conta é encerrada.</summary>
    public DateOnly? DataLimite { get; set; }
    /// <summary>#158 (ADR 0043): à mão (boleto, Pix), débito automático ou cartão de crédito.</summary>
    public FormaCobranca FormaCobranca { get; set; } = FormaCobranca.AMao;
    /// <summary>#158: "a descrição do banco contém" isto (ex.: NETFLIX), sem acento e em maiúsculas; liga às cobranças.</summary>
    public string? TextoCobranca { get; set; }
    /// <summary>#158: observação livre (ex.: "conta da família", "cancelar em março").</summary>
    public string? Observacao { get; set; }
    public DateTime CriadoEm { get; set; } = DateTime.UtcNow;

    public Categoria? Categoria { get; set; }
    public Conta? Conta { get; set; }
}
