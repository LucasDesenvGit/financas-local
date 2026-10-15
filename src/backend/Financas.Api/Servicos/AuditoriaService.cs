using Financas.Api.Data;
using Financas.Api.Entidades;

namespace Financas.Api.Servicos;

public class AuditoriaService
{
    private readonly FinancasDbContext _db;
    private readonly IHttpContextAccessor _http;

    public AuditoriaService(FinancasDbContext db, IHttpContextAccessor http)
    {
        _db = db;
        _http = http;
    }

    /// <summary>Registra o evento (não salva: chame SaveChangesAsync).</summary>
    public void Registrar(string evento, string? login)
    {
        var ctx = _http.HttpContext;
        var userAgent = ctx?.Request.Headers.UserAgent.ToString();
        if (userAgent is { Length: > 400 }) userAgent = userAgent[..400];

        _db.AuditoriasAcesso.Add(new AuditoriaAcesso
        {
            Evento = evento,
            Login = login is { Length: > 100 } ? login[..100] : login,
            Ip = ctx?.Connection.RemoteIpAddress?.ToString(),
            UserAgent = userAgent,
            DataHora = DateTime.UtcNow
        });
    }
}
