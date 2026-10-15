namespace Financas.Api.Servicos;

/// <summary>Datas sempre no fuso de Brasília, independentemente do fuso do servidor.</summary>
public static class Relogio
{
    private static readonly TimeZoneInfo Fuso = ObterFuso();

    private static TimeZoneInfo ObterFuso()
    {
        foreach (var id in new[] { "America/Sao_Paulo", "E. South America Standard Time" })
        {
            if (TimeZoneInfo.TryFindSystemTimeZoneById(id, out var tz)) return tz;
        }
        return TimeZoneInfo.Local;
    }

    public static DateTime AgoraLocal => TimeZoneInfo.ConvertTime(DateTime.UtcNow, Fuso);

    public static DateOnly Hoje => DateOnly.FromDateTime(AgoraLocal);

    public static DateOnly ParaDataLocal(DateTimeOffset valor)
        => DateOnly.FromDateTime(TimeZoneInfo.ConvertTime(valor, Fuso).DateTime);

    public static DateOnly PrimeiroDiaMes(int ano, int mes) => new(ano, mes, 1);

    public static DateOnly UltimoDiaMes(int ano, int mes) => new(ano, mes, DateTime.DaysInMonth(ano, mes));
}
