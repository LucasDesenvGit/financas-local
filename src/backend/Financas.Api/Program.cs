using System.Threading.RateLimiting;
using Financas.Api.Data;
using Financas.Api.Entidades;
using Financas.Api.Seguranca;
using Financas.Api.Servicos;
using Financas.Api.Servicos.Cambio;
using Financas.Api.Servicos.Investimentos;
using Financas.Api.Servicos.Pluggy;
using Microsoft.AspNetCore.Authentication.Cookies;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.DataProtection;
using Microsoft.AspNetCore.Diagnostics;
using Microsoft.AspNetCore.Identity;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

var builder = WebApplication.CreateBuilder(args);
builder.Host.UseWindowsService();
builder.Host.UseSystemd();
builder.WebHost.ConfigureKestrel(o =>
{
    o.AddServerHeader = false;
    o.Limits.MaxRequestBodySize = 1_048_576;
});
var cfg = builder.Configuration;

// ---------------------------------------------------------------- Banco de dados
var caminhoBanco = cfg["Banco:Caminho"] ?? Path.Combine(AppContext.BaseDirectory, "dados", "sql", "FinancasLocal.db");
builder.Services.AddDbContext<FinancasDbContext>(opcoes =>
    opcoes.UseSqlite($"Data Source={caminhoBanco}"));

// ---------------------------------------------------------------- Proteção de dados (chaves em D:, protegidas por DPAPI)
var pastaChaves = cfg["Seguranca:PastaChaves"];
if (string.IsNullOrWhiteSpace(pastaChaves)) pastaChaves = Path.Combine(AppContext.BaseDirectory, "chaves");
var protecao = builder.Services.AddDataProtection()
    .SetApplicationName("FinancasLocal")
    .PersistKeysToFileSystem(new DirectoryInfo(pastaChaves));
if (OperatingSystem.IsWindows()) protecao.ProtectKeysWithDpapi();

// ---------------------------------------------------------------- Autenticação (cookie HttpOnly + SameSite=Strict)
var minutosSessao = cfg.GetValue("Seguranca:MinutosSessao", 120);
builder.Services.AddAuthentication(CookieAuthenticationDefaults.AuthenticationScheme)
    .AddCookie(o =>
    {
        o.Cookie.Name = ConstantesSeguranca.CookieSessao;
        o.Cookie.HttpOnly = true;
        o.Cookie.SecurePolicy = CookieSecurePolicy.Always;
        o.Cookie.SameSite = SameSiteMode.Strict;
        o.Cookie.Path = "/";
        o.ExpireTimeSpan = TimeSpan.FromMinutes(minutosSessao);
        o.SlidingExpiration = true;
        o.Events = new CookieAuthenticationEvents
        {
            OnRedirectToLogin = ctx => { ctx.Response.StatusCode = StatusCodes.Status401Unauthorized; return Task.CompletedTask; },
            OnRedirectToAccessDenied = ctx => { ctx.Response.StatusCode = StatusCodes.Status403Forbidden; return Task.CompletedTask; },
            OnValidatePrincipal = ValidadorSessao.ValidarAsync
        };
    });

// Tudo exige login, exceto o que for marcado com [AllowAnonymous].
builder.Services.AddAuthorization(o =>
    o.FallbackPolicy = new AuthorizationPolicyBuilder().RequireAuthenticatedUser().Build());

// ---------------------------------------------------------------- Proteção contra CSRF
builder.Services.AddAntiforgery(o =>
{
    o.HeaderName = ConstantesSeguranca.CabecalhoXsrf;
    o.Cookie.Name = ConstantesSeguranca.CookieAntiforgery;
    o.Cookie.HttpOnly = true;
    o.Cookie.SecurePolicy = CookieSecurePolicy.Always;
    o.Cookie.SameSite = SameSiteMode.Strict;
});

// ---------------------------------------------------------------- Limite de tentativas de login
builder.Services.AddRateLimiter(o =>
{
    o.RejectionStatusCode = StatusCodes.Status429TooManyRequests;
    o.AddPolicy(ConstantesSeguranca.PoliticaLogin, ctx =>
        RateLimitPartition.GetFixedWindowLimiter(
            ctx.Connection.RemoteIpAddress?.ToString() ?? "local",
            _ => new FixedWindowRateLimiterOptions
            {
                PermitLimit = 5,
                Window = TimeSpan.FromMinutes(1),
                QueueLimit = 0
            }));
});

// ---------------------------------------------------------------- MVC
// AddControllersWithViews registra o filtro de validacao antifalsificacao (AddControllers sozinho nao registra).
builder.Services.AddControllersWithViews(o => o.Filters.Add(new AutoValidateAntiforgeryTokenAttribute()));
builder.Services.AddHttpContextAccessor();

// ---------------------------------------------------------------- Serviços da aplicação
builder.Services.AddSingleton<IPasswordHasher<Usuario>, PasswordHasher<Usuario>>();
builder.Services.AddScoped<ConfiguracaoService>();
builder.Services.AddScoped<AuditoriaService>();
builder.Services.AddScoped<CategorizacaoService>();
builder.Services.AddScoped<SugestaoCategoriaService>();
builder.Services.AddScoped<ContaService>();
builder.Services.AddScoped<CarteiraService>();
builder.Services.AddScoped<CotacaoService>();
builder.Services.AddScoped<ExtratoCarteiraService>();
builder.Services.AddScoped<FiiService>();
builder.Services.AddScoped<EmprestimoService>();
builder.Services.AddScoped<RelatorioService>();
builder.Services.AddScoped<SincronizacaoService>();
builder.Services.AddHostedService<SincronizacaoHostedService>();
builder.Services.AddHostedService<CambioHostedService>();

var urlPluggy = new Uri(cfg["Pluggy:UrlBase"] ?? "https://api.pluggy.ai/");
if (urlPluggy.Scheme != Uri.UriSchemeHttps) throw new InvalidOperationException("A URL da Pluggy deve usar HTTPS.");
builder.Services.AddHttpClient<PluggyClient>(c =>
{
    c.BaseAddress = urlPluggy;
    c.Timeout = TimeSpan.FromSeconds(100);
    c.DefaultRequestHeaders.UserAgent.ParseAdd("FinancasLocal/1.0");
});

// Fontes públicas para investimentos (somente HTTPS; nenhum dado pessoal é enviado).
const string AgenteNavegador = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36";
builder.Services.AddHttpClient<SelicService>(c =>
{
    c.BaseAddress = new Uri("https://api.bcb.gov.br/");
    c.Timeout = TimeSpan.FromSeconds(30);
    c.DefaultRequestHeaders.UserAgent.ParseAdd("FinancasLocal/1.0");
    c.DefaultRequestHeaders.Accept.ParseAdd("application/json");
});
builder.Services.AddHttpClient<CambioService>(c =>
{
    c.BaseAddress = new Uri("https://olinda.bcb.gov.br/olinda/servico/PTAX/versao/v1/odata/");
    c.Timeout = TimeSpan.FromSeconds(30);
    c.DefaultRequestHeaders.UserAgent.ParseAdd("FinancasLocal/1.0");
    c.DefaultRequestHeaders.Accept.ParseAdd("application/json");
});
builder.Services.AddHttpClient(CotacaoService.ClienteYahoo, c =>
{
    c.BaseAddress = new Uri("https://query1.finance.yahoo.com/");
    c.Timeout = TimeSpan.FromSeconds(20);
    c.DefaultRequestHeaders.UserAgent.ParseAdd(AgenteNavegador);
    c.DefaultRequestHeaders.Accept.ParseAdd("application/json");
});
builder.Services.AddHttpClient(CotacaoService.ClienteBrapi, c =>
{
    c.BaseAddress = new Uri("https://brapi.dev/");
    c.Timeout = TimeSpan.FromSeconds(20);
    c.DefaultRequestHeaders.UserAgent.ParseAdd("FinancasLocal/1.0");
    c.DefaultRequestHeaders.Accept.ParseAdd("application/json");
});

var app = builder.Build();

// ---------------------------------------------------------------- Pipeline
app.UseMiddleware<CabecalhosSegurancaMiddleware>();

if (app.Environment.IsDevelopment())
{
    app.UseDeveloperExceptionPage();
}
else
{
    app.UseExceptionHandler(new ExceptionHandlerOptions
    {
        ExceptionHandler = async ctx =>
        {
            // Não expõe detalhes internos; o erro completo vai para o log de eventos do Windows.
            var erro = ctx.Features.Get<IExceptionHandlerFeature>()?.Error;
            ctx.RequestServices.GetRequiredService<ILoggerFactory>().CreateLogger("Erro")
                .LogError(erro, "Erro não tratado em {Caminho}", ctx.Request.Path);
            ctx.Response.StatusCode = StatusCodes.Status500InternalServerError;
            await ctx.Response.WriteAsJsonAsync(new { mensagem = "Ocorreu um erro interno. Consulte o log de eventos." });
        }
    });
    app.UseHsts();
}

app.UseDefaultFiles();
app.UseStaticFiles(new StaticFileOptions
{
    OnPrepareResponse = ctx =>
    {
        var nome = ctx.File.Name;
        ctx.Context.Response.Headers["Cache-Control"] =
            nome.Equals("index.html", StringComparison.OrdinalIgnoreCase)
                ? "no-cache"
                : "public, max-age=31536000, immutable";
    }
});

app.UseRouting();
app.UseRateLimiter();
app.UseAuthentication();
app.UseAuthorization();

app.MapControllers();
app.MapFallback("/api/{**caminho}", () => Results.NotFound()).AllowAnonymous();
app.MapFallbackToFile("index.html").AllowAnonymous();

app.Run();
