namespace Financas.Api.Entidades;

public class Ativo
{
    public int Id { get; set; }
    public string Ticker { get; set; } = string.Empty;
    public string? Nome { get; set; }
    public TipoAtivo Tipo { get; set; } = TipoAtivo.Fii;
    public decimal? PrecoAtual { get; set; }
    public decimal? PrecoAnterior { get; set; }
    public DateTime? PrecoAtualizadoEm { get; set; }
    public bool PrecoManual { get; set; }
    public string? FonteCotacao { get; set; }
    public string? Segmento { get; set; }
    public DateTime CriadoEm { get; set; } = DateTime.UtcNow;

    public List<OperacaoAtivo> Operacoes { get; set; } = new();
    public List<ProventoAtivo> Proventos { get; set; } = new();
    public List<PrecoMensalAtivo> PrecosMensais { get; set; } = new();
}

public class OperacaoAtivo
{
    public int Id { get; set; }
    public int AtivoId { get; set; }
    public TipoOperacao Tipo { get; set; }
    public DateOnly Data { get; set; }
    public decimal Quantidade { get; set; }
    public decimal PrecoUnitario { get; set; }
    public decimal Taxas { get; set; }
    public string? Observacao { get; set; }
    public OrigemOperacao Origem { get; set; } = OrigemOperacao.Manual;
    /// <summary>Id da movimentação do investimento na Pluggy (operações importadas do Open Finance).</summary>
    public string? PluggyTransacaoId { get; set; }
    /// <summary>Transação do extrato que originou a operação (importação pelo extrato).</summary>
    public long? TransacaoId { get; set; }
    public DateTime CriadoEm { get; set; } = DateTime.UtcNow;

    public Ativo? Ativo { get; set; }
}

public class ProventoAtivo
{
    public int Id { get; set; }
    public int AtivoId { get; set; }
    public DateOnly DataCom { get; set; }
    public decimal ValorPorCota { get; set; }

    public Ativo? Ativo { get; set; }
}

public class TaxaSelicDiaria
{
    public DateOnly Data { get; set; }
    /// <summary>Taxa em % ao dia (série 11 do SGS).</summary>
    public decimal Taxa { get; set; }
}

public class PrecoMensalAtivo
{
    public int AtivoId { get; set; }
    /// <summary>Primeiro dia do mês.</summary>
    public DateOnly Mes { get; set; }
    public decimal Fechamento { get; set; }

    public Ativo? Ativo { get; set; }
}

public class IndiceMensal
{
    public string Codigo { get; set; } = string.Empty;
    public DateOnly Mes { get; set; }
    public decimal Fechamento { get; set; }
}
