namespace Financas.Api.Entidades;

/// <summary>
/// Erro ou aviso do sistema (#11), gravado pela API sem dados sensíveis (ver <c>Servicos/Diagnostico</c>). A mesma
/// ocorrência repetida enquanto está em aberto vira uma linha só: <see cref="Ocorrencias"/> e <see cref="UltimaEm"/>.
/// </summary>
public class ErroSistema
{
    public long Id { get; set; }
    /// <summary>UTC.</summary>
    public DateTime PrimeiraEm { get; set; }
    /// <summary>UTC.</summary>
    public DateTime UltimaEm { get; set; }
    public int Ocorrencias { get; set; } = 1;
    public NivelErro Nivel { get; set; }
    /// <summary>De onde veio, para filtrar: API, Open Finance, Investimentos, Câmbio, Cripto, Front-end ou Sistema.</summary>
    public string Origem { get; set; } = string.Empty;
    /// <summary>Categoria do log (classe que registrou).</summary>
    public string Categoria { get; set; } = string.Empty;
    public string Mensagem { get; set; } = string.Empty;
    /// <summary>Exceção completa (tipo, mensagem e pilha), já limpa.</summary>
    public string? Detalhe { get; set; }
    /// <summary>Método e caminho da requisição, sem query string.</summary>
    public string? Rota { get; set; }
    public string? Usuario { get; set; }
    /// <summary>Hash de categoria + mensagem + tipo da exceção, para agrupar as repetições.</summary>
    public string Assinatura { get; set; } = string.Empty;
    public bool Resolvido { get; set; }
    public DateTime? ResolvidoEm { get; set; }
}
