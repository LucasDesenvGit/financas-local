namespace Financas.Api.Entidades;

public class Usuario
{
    public int Id { get; set; }
    public string Login { get; set; } = string.Empty;
    public string SenhaHash { get; set; } = string.Empty;
    public Guid CarimboSeguranca { get; set; } = Guid.NewGuid();
    public int TentativasFalhas { get; set; }
    public DateTime? BloqueadoAte { get; set; }
    public DateTime? UltimoLoginEm { get; set; }
    public DateTime CriadoEm { get; set; } = DateTime.UtcNow;
}

public class AuditoriaAcesso
{
    public long Id { get; set; }
    public DateTime DataHora { get; set; } = DateTime.UtcNow;
    public string Evento { get; set; } = string.Empty;
    public string? Login { get; set; }
    public string? Ip { get; set; }
    public string? UserAgent { get; set; }
}

public class Configuracao
{
    public string Chave { get; set; } = string.Empty;
    public string? Valor { get; set; }
    public bool Protegido { get; set; }
    public DateTime AtualizadoEm { get; set; } = DateTime.UtcNow;
}
