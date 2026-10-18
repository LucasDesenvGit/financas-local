using System.Threading.RateLimiting;
using Financas.Api.Data;
using Financas.Api.Entidades;
using Financas.Api.Seguranca;
using Financas.Api.Servicos;
using Financas.Api.Servicos.Cambio;
using Financas.Api.Servicos.Diagnostico;
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
// "Desfazer" de alterações em lote (#87): o estado anterior fica só na memória, por alguns minutos.
builder.Services.AddMemoryCache();

// ---------------------------------------------------------------- Serviços da aplicação
builder.Services.AddSingleton<IPasswordHasher<Usuario>, PasswordHasher<Usuario>>();
builder.Services.AddScoped<ConfiguracaoService>();
builder.Services.AddScoped<ParametroService>();
builder.Services.AddScoped<AuditoriaService>();
builder.Services.AddScoped<CategorizacaoService>();
builder.Services.AddScoped<TransferenciasService>();
builder.Services.AddScoped<SugestaoCategoriaService>();
builder.Services.AddScoped<SugestaoContaAgendadaService>();
builder.Services.AddScoped<SugestaoOrcamentoService>();
builder.Services.AddScoped<AssistenteMetasService>();
builder.Services.AddScoped<DescricaoPoucoInformativaService>();
builder.Services.AddScoped<ContaService>();
builder.Services.AddScoped<CarteiraService>();
builder.Services.AddScoped<ReservaService>();
builder.Services.AddScoped<CotacaoService>();
builder.Services.AddScoped<ExtratoCarteiraService>();
builder.Services.AddScoped<ConferenciaInvestimentosService>();
builder.Services.AddScoped<FiiService>();
builder.Services.AddScoped<EmprestimoService>();
builder.Services.AddScoped<RelatorioService>();
builder.Services.AddScoped<SincronizacaoService>();
builder.Services.AddHostedService<SincronizacaoHostedService>();
builder.Services.AddHostedService<CambioHostedService>();

// Erros e avisos do próprio código vão também para o banco, sem dados sensíveis (#11, ADR 0022).
builder.Services.AddSingleton<FilaErros>();
builder.Services.AddSingleton<ILoggerProvider, RegistroErrosLoggerProvider>();
builder.Services.AddHostedService<GravadorErrosHostedService>();

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

// Transferência só fica fora dos relatórios quando é entre contas suas (ADR 0031): acerta o que já está no banco.
try
{
    using var escopo = app.Services.CreateScope();
    if (await escopo.ServiceProvider.GetRequiredService<TransferenciasService>().ReclassificarAsync(CancellationToken.None) > 0)
        await escopo.ServiceProvider.GetRequiredService<FinancasDbContext>().SaveChangesAsync();
}
catch (Exception ex)
{
    app.Logger.LogWarning(ex, "Não foi possível conferir as transferências ao iniciar.");
}

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
            // Não expõe detalhes internos; o erro completo vai para o log de eventos e para Configurações › Erros.
            var erro = ctx.Features.Get<IExceptionHandlerFeature>()?.Error;
            ctx.RequestServices.GetRequiredService<ILoggerFactory>().CreateLogger("Erro")
                .LogError(erro, "Erro não tratado em {Caminho}: {Mensagem}", ctx.Request.Path, erro?.Message);
            ctx.Response.StatusCode = StatusCodes.Status500InternalServerError;
            await ctx.Response.WriteAsJsonAsync(new { mensagem = "Ocorreu um erro interno. Ele foi registrado em Configurações › Erros." });
        }
    });
    app.UseHsts();
}

app.UseDefaultFiles();
// Nomes gerados pelo build do Angular: main-ABCD1234.js, chunk-ABCD1234.js, styles-ABCD1234.css.
var arquivoComHash = new System.Text.RegularExpressions.Regex(@"-[A-Z0-9]{8}\.(js|css)$",
    System.Text.RegularExpressions.RegexOptions.Compiled);
app.UseStaticFiles(new StaticFileOptions
{
    OnPrepareResponse = ctx =>
    {
        // Só os arquivos do build com hash no nome (ex.: main-ABCD1234.js) são imutáveis; o resto (index.html,
        // favicon, ícones, manifesto) mantém o nome entre versões e precisa ser revalidado.
        ctx.Context.Response.Headers["Cache-Control"] = arquivoComHash.IsMatch(ctx.File.Name)
            ? "public, max-age=31536000, immutable"
            : "no-cache";
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
