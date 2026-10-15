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
