using System.Security.Claims;
using Financas.Api.Data;
using Microsoft.AspNetCore.Antiforgery;
using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Authentication.Cookies;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Seguranca;

public static class ConstantesSeguranca
{
    public const string CookieSessao = "__Host-Financas";
    public const string CookieAntiforgery = "__Host-Financas-AF";
    /// <summary>Cookie legível pelo Angular (HttpClient envia o valor no cabeçalho X-XSRF-TOKEN).</summary>
    public const string CookieXsrf = "__Host-XSRF-TOKEN";
    public const string CabecalhoXsrf = "X-XSRF-TOKEN";
    public const string ClaimCarimbo = "financas:carimbo";
    public const string PoliticaLogin = "login";
    public const int TamanhoMinimoSenha = 8;
}

public static class TokenAntiforgery
{
    /// <summary>Gera o par de tokens antifalsificação para o usuário atual de <paramref name="ctx"/>.</summary>
    public static void Emitir(HttpContext ctx, IAntiforgery antiforgery)
    {
        var tokens = antiforgery.GetAndStoreTokens(ctx);
        if (tokens.RequestToken is null) return;
        ctx.Response.Cookies.Append(ConstantesSeguranca.CookieXsrf, tokens.RequestToken, new CookieOptions
        {
            HttpOnly = false,
            Secure = true,
            SameSite = SameSiteMode.Strict,
            Path = "/",
            IsEssential = true
        });
    }
}

/// <summary>
/// A cada requisição confere se o "carimbo de segurança" da sessão ainda é o do usuário.
/// Ao trocar a senha o carimbo muda e todas as sessões abertas são encerradas.
/// </summary>
public static class ValidadorSessao
{
    public static async Task ValidarAsync(CookieValidatePrincipalContext ctx)
    {
        var idTexto = ctx.Principal?.FindFirstValue(ClaimTypes.NameIdentifier);
        var carimbo = ctx.Principal?.FindFirstValue(ConstantesSeguranca.ClaimCarimbo);
        if (!int.TryParse(idTexto, out var id) || !Guid.TryParse(carimbo, out var carimboGuid))
        {
            await RejeitarAsync(ctx);
            return;
        }

        var db = ctx.HttpContext.RequestServices.GetRequiredService<FinancasDbContext>();
        var valido = await db.Usuarios.AsNoTracking()
            .AnyAsync(u => u.Id == id && u.CarimboSeguranca == carimboGuid, ctx.HttpContext.RequestAborted);
        if (!valido) await RejeitarAsync(ctx);
    }

    private static async Task RejeitarAsync(CookieValidatePrincipalContext ctx)
    {
        ctx.RejectPrincipal();
        await ctx.HttpContext.SignOutAsync(CookieAuthenticationDefaults.AuthenticationScheme);
    }
}

/// <summary>Cabeçalhos de segurança aplicados a todas as respostas.</summary>
public class CabecalhosSegurancaMiddleware
{
    private const string Csp =
        "default-src 'self'; " +
        "script-src 'self'; " +
        "style-src 'self' 'unsafe-inline'; " +
        "img-src 'self' data:; " +
        "font-src 'self'; " +
        "connect-src 'self'; " +
        "object-src 'none'; " +
        "base-uri 'self'; " +
        "form-action 'self'; " +
        "frame-ancestors 'none'";

    private readonly RequestDelegate _next;

    public CabecalhosSegurancaMiddleware(RequestDelegate next) => _next = next;

    public Task InvokeAsync(HttpContext ctx)
    {
        ctx.Response.OnStarting(() =>
        {
            var h = ctx.Response.Headers;
            h["Content-Security-Policy"] = Csp;
            h["X-Content-Type-Options"] = "nosniff";
            h["X-Frame-Options"] = "DENY";
            h["Referrer-Policy"] = "no-referrer";
            h["Permissions-Policy"] = "camera=(), microphone=(), geolocation=(), payment=(), usb=()";
            h["Cross-Origin-Opener-Policy"] = "same-origin";
            h["Cross-Origin-Resource-Policy"] = "same-origin";
            h.Remove("Server");
            h.Remove("X-Powered-By");
            if (ctx.Request.Path.StartsWithSegments("/api"))
            {
                // Dados financeiros nunca devem ficar em cache do navegador.
                h["Cache-Control"] = "no-store";
                h["Pragma"] = "no-cache";
            }
            return Task.CompletedTask;
        });
        return _next(ctx);
    }
}
