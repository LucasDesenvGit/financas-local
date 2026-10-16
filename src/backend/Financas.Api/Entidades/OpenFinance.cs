namespace Financas.Api.Entidades;

public class ConexaoBancaria
{
    public int Id { get; set; }
    public string PluggyItemId { get; set; } = string.Empty;
    public string? NomeInstituicao { get; set; }
    public string? CorPrimaria { get; set; }
    public string? Status { get; set; }
    public string? StatusExecucao { get; set; }
    public DateTime? UltimaAtualizacaoInstituicao { get; set; }
    public DateTime? ConsentimentoExpiraEm { get; set; }
    public DateTime? UltimaSincronizacaoEm { get; set; }
    public string? UltimoErro { get; set; }
    public bool Ativa { get; set; } = true;
    public DateTime CriadoEm { get; set; } = DateTime.UtcNow;

    public List<Conta> Contas { get; set; } = new();
    public List<Investimento> Investimentos { get; set; } = new();
    public List<Emprestimo> Emprestimos { get; set; } = new();
}

public class SincronizacaoLog
{
    public long Id { get; set; }
    public int? ConexaoBancariaId { get; set; }
    public string Origem { get; set; } = "Automatica";
    public DateTime IniciadoEm { get; set; } = DateTime.UtcNow;
    public DateTime? FinalizadoEm { get; set; }
    public bool Sucesso { get; set; }
    public int TransacoesNovas { get; set; }
    public int TransacoesAtualizadas { get; set; }
    public string? Mensagem { get; set; }

    public ConexaoBancaria? ConexaoBancaria { get; set; }
}

/// <summary>
/// Saldo real de uma reserva ("cofrinho") conferido pelo usuário. A instituição não informa esse saldo pelo
/// Open Finance; ele é estimado pelos lançamentos "Dinheiro reservado/retirado {Nome}" da conta
/// (ver <c>ReservaService</c>), partindo deste valor conferido quando houver.
/// </summary>
public class ReservaConferencia
{
    public int Id { get; set; }
    public int ContaId { get; set; }
    /// <summary>Nome da reserva como aparece no extrato (ex.: "Emergência").</summary>
    public string Nome { get; set; } = string.Empty;
    public decimal SaldoConferido { get; set; }
    /// <summary>Data da conferência: lançamentos até esse dia já estão no saldo conferido.</summary>
    public DateOnly DataConferencia { get; set; }
    public DateTime AtualizadoEm { get; set; } = DateTime.UtcNow;

    public Conta? Conta { get; set; }
}

public class CategoriaPluggy
{
    public string PluggyCategoriaId { get; set; } = string.Empty;
    public string Descricao { get; set; } = string.Empty;
    public string? PaiPluggyId { get; set; }
    public string? PaiDescricao { get; set; }
    public int? CategoriaId { get; set; }

    public Categoria? Categoria { get; set; }
}

public class Investimento
{
    public int Id { get; set; }
    public int ConexaoBancariaId { get; set; }
    public string PluggyInvestmentId { get; set; } = string.Empty;
    public string Nome { get; set; } = string.Empty;
    public string Tipo { get; set; } = string.Empty;
    public string? Subtipo { get; set; }
    public string? Codigo { get; set; }
    public string? Emissor { get; set; }
    public string Moeda { get; set; } = "BRL";
    public decimal Saldo { get; set; }
    public decimal? ValorAplicado { get; set; }
    public decimal? Rendimento { get; set; }
    public decimal? Quantidade { get; set; }
    public decimal? TaxaAnual { get; set; }
    public DateOnly? DataVencimento { get; set; }
    public DateOnly? DataAplicacao { get; set; }
    public string? Status { get; set; }
    public DateTime AtualizadoEm { get; set; } = DateTime.UtcNow;

    public ConexaoBancaria? ConexaoBancaria { get; set; }
}

/// <summary>Contrato de empréstimo ou financiamento (somente leitura, vindo do Open Finance via GET /loans).</summary>
public class Emprestimo
{
    public int Id { get; set; }
    public int ConexaoBancariaId { get; set; }
    public string PluggyLoanId { get; set; } = string.Empty;
    public NaturezaCredito Natureza { get; set; }
    /// <summary>Modalidade do Open Finance (ex.: FINANCIAMENTO_HABITACIONAL_SFH, CREDITO_PESSOAL_COM_CONSIGNACAO).</summary>
    public string? Modalidade { get; set; }
    public string Produto { get; set; } = string.Empty;
    public string? NumeroContrato { get; set; }
    public string Moeda { get; set; } = "BRL";
    public decimal ValorContratado { get; set; }
    public decimal SaldoDevedor { get; set; }
    /// <summary>Valor do último pagamento regular; na falta dele, saldo devedor ÷ parcelas a vencer.</summary>
    public decimal? ValorParcela { get; set; }
    public int? ParcelasTotal { get; set; }
    public int? ParcelasPagas { get; set; }
    public int? ParcelasAVencer { get; set; }
    public int? ParcelasEmAtraso { get; set; }
    /// <summary>MONTHLY, YEARLY, IRREGULAR...</summary>
    public string? Periodicidade { get; set; }
    /// <summary>SAC, PRICE, SAM...</summary>
    public string? SistemaAmortizacao { get; set; }
    /// <summary>Taxa pré-fixada do contrato, em fração (0.0975 = 9,75%).</summary>
    public decimal? TaxaJuros { get; set; }
    /// <summary>AA (ao ano) ou AM (ao mês).</summary>
    public string? PeriodicidadeTaxa { get; set; }
    /// <summary>Indexador da parte pós-fixada (ex.: TR, IPCA, CDI), quando houver.</summary>
    public string? Indexador { get; set; }
    /// <summary>Custo efetivo total, em fração ao ano.</summary>
    public decimal? Cet { get; set; }
    public DateOnly? DataContratacao { get; set; }
    public DateOnly? DataPrimeiraParcela { get; set; }
    public DateOnly? DataVencimentoFinal { get; set; }
    public DateTime AtualizadoEm { get; set; } = DateTime.UtcNow;

    public ConexaoBancaria? ConexaoBancaria { get; set; }
    public List<PagamentoEmprestimo> Pagamentos { get; set; } = new();
}

/// <summary>Pagamento efetuado de um contrato (payments.releases da Pluggy). Recarregado a cada sincronização.</summary>
public class PagamentoEmprestimo
{
    public long Id { get; set; }
    public int EmprestimoId { get; set; }
    public DateOnly Data { get; set; }
    public decimal Valor { get; set; }
    /// <summary>Pagamento fora da parcela (amortização extraordinária, antecipação).</summary>
    public bool Extraordinario { get; set; }

    public Emprestimo? Emprestimo { get; set; }
}
