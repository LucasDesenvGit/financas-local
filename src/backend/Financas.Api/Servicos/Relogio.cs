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

    /// <summary>Converte um instante (UTC ou com Kind definido) para a hora de Brasília.</summary>
    public static DateTime ParaHoraLocal(DateTime instante) => TimeZoneInfo.ConvertTime(instante.ToUniversalTime(), Fuso);

    public static DateOnly ParaDataLocal(DateTimeOffset valor)
        => DateOnly.FromDateTime(TimeZoneInfo.ConvertTime(valor, Fuso).DateTime);

    /// <summary>
    /// Instante UTC em que começa o dia (meia-noite em Brasília). No dia em que o horário de verão começava (até 2019),
    /// a meia-noite não existia: o relógio ia de 23:59:59 direto para 01:00, e o ConvertTimeToUtc lança exceção para
    /// hora que não existe. O dia então começa na primeira hora que existe, a da troca de fuso, como o
    /// relogio.InicioDoDiaUtc do Go (#393).
    /// </summary>
    public static DateTime InicioDoDiaUtc(DateOnly dia)
    {
        var inicio = dia.ToDateTime(TimeOnly.MinValue, DateTimeKind.Unspecified);
        while (Fuso.IsInvalidTime(inicio)) inicio = inicio.AddMinutes(1);
        return TimeZoneInfo.ConvertTimeToUtc(inicio, Fuso);
    }

    public static DateOnly PrimeiroDiaMes(int ano, int mes) => new(ano, mes, 1);

    public static DateOnly UltimoDiaMes(int ano, int mes) => new(ano, mes, DateTime.DaysInMonth(ano, mes));
}
