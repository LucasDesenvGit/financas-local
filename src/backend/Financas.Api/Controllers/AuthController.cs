using System.Security.Claims;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Seguranca;
using Financas.Api.Servicos;
using Microsoft.AspNetCore.Antiforgery;
using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Authentication.Cookies;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Identity;
using Microsoft.AspNetCore.Mvc;
using Microsoft.AspNetCore.RateLimiting;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Controllers;

[Route("api/auth")]
public class AuthController : BaseController
{
    private static string? _hashFicticio;

    private readonly FinancasDbContext _db;
    private readonly IPasswordHasher<Usuario> _hasher;
    private readonly IAntiforgery _antiforgery;
    private readonly AuditoriaService _auditoria;
    private readonly ParametroService _parametros;

    public AuthController(FinancasDbContext db, IPasswordHasher<Usuario> hasher, IAntiforgery antiforgery,
        AuditoriaService auditoria, ParametroService parametros)
    {
        _db = db;
        _hasher = hasher;
        _antiforgery = antiforgery;
        _auditoria = auditoria;
        _parametros = parametros;
    }

    /// <summary>Informa se o sistema já tem usuário e se a sessão está ativa. Também emite o token CSRF.</summary>
    [HttpGet("estado")]
    [AllowAnonymous]
    public async Task<ActionResult<EstadoAutenticacaoDto>> Estado(CancellationToken ct)
    {
        var configurado = await _db.Usuarios.AnyAsync(ct);
        TokenAntiforgery.Emitir(HttpContext, _antiforgery);
        var autenticado = User.Identity?.IsAuthenticated == true;
        return new EstadoAutenticacaoDto(configurado, autenticado, autenticado ? User.Identity!.Name : null);
    }

    /// <summary>Cria o único usuário do sistema. Só funciona enquanto nenhum usuário existir.</summary>
    [HttpPost("configurar")]
    [AllowAnonymous]
    [EnableRateLimiting(ConstantesSeguranca.PoliticaLogin)]
    public async Task<ActionResult<EstadoAutenticacaoDto>> Configurar(CredenciaisDto dto, CancellationToken ct)
    {
        if (await _db.Usuarios.AnyAsync(ct))
            return Erro("O sistema já foi configurado.", StatusCodes.Status409Conflict);

        var login = dto.Login.Trim();
        var problema = ValidarSenha(dto.Senha, login);
        if (problema is not null) return Erro(problema);

        var usuario = new Usuario { Login = login, CarimboSeguranca = Guid.NewGuid(), UltimoLoginEm = DateTime.UtcNow };
        usuario.SenhaHash = _hasher.HashPassword(usuario, dto.Senha);
        _db.Usuarios.Add(usuario);
        _auditoria.Registrar("Configuracao", login);
        await _db.SaveChangesAsync(ct);

        await EntrarAsync(usuario);
        return new EstadoAutenticacaoDto(true, true, usuario.Login);
    }

    [HttpPost("entrar")]
    [AllowAnonymous]
    [EnableRateLimiting(ConstantesSeguranca.PoliticaLogin)]
    public async Task<ActionResult<EstadoAutenticacaoDto>> Entrar(CredenciaisDto dto, CancellationToken ct)
    {
        const string falha = "Usuário ou senha inválidos.";
        var login = dto.Login.Trim();
        var usuario = await _db.Usuarios.FirstOrDefaultAsync(u => u.Login == login, ct);

        if (usuario is null)
        {
            // Executa a verificação mesmo assim, para o tempo de resposta não revelar se o usuário existe.
            _hashFicticio ??= _hasher.HashPassword(new Usuario(), Guid.NewGuid().ToString());
            _hasher.VerifyHashedPassword(new Usuario(), _hashFicticio, dto.Senha);
            _auditoria.Registrar("LoginFalha", login);
            await _db.SaveChangesAsync(ct);
            return Erro(falha, StatusCodes.Status401Unauthorized);
        }

        if (usuario.BloqueadoAte is { } bloqueio && bloqueio > DateTime.UtcNow)
        {
            _auditoria.Registrar("LoginBloqueado", login);
            await _db.SaveChangesAsync(ct);
            var minutos = Math.Max(1, (int)Math.Ceiling((bloqueio - DateTime.UtcNow).TotalMinutes));
            return Erro($"Acesso bloqueado temporariamente por excesso de tentativas. Tente novamente em {minutos} minuto(s).",
                StatusCodes.Status429TooManyRequests);
        }

        var resultado = _hasher.VerifyHashedPassword(usuario, usuario.SenhaHash, dto.Senha);
        if (resultado == PasswordVerificationResult.Failed)
        {
            usuario.TentativasFalhas++;
            if (usuario.TentativasFalhas >= await _parametros.ObterInteiroAsync(ConfiguracaoService.MaximoTentativasLogin, ct))
            {
                usuario.BloqueadoAte = DateTime.UtcNow.AddMinutes(
                    await _parametros.ObterInteiroAsync(ConfiguracaoService.MinutosBloqueio, ct));
                usuario.TentativasFalhas = 0;
            }
            _auditoria.Registrar("LoginFalha", login);
            await _db.SaveChangesAsync(ct);
            return Erro(falha, StatusCodes.Status401Unauthorized);
        }

        if (resultado == PasswordVerificationResult.SuccessRehashNeeded)
            usuario.SenhaHash = _hasher.HashPassword(usuario, dto.Senha);

        usuario.TentativasFalhas = 0;
        usuario.BloqueadoAte = null;
        usuario.UltimoLoginEm = DateTime.UtcNow;
        _auditoria.Registrar("LoginSucesso", login);
        await _db.SaveChangesAsync(ct);

        await EntrarAsync(usuario);
        return new EstadoAutenticacaoDto(true, true, usuario.Login);
    }

    [HttpPost("sair")]
    [AllowAnonymous]
    public async Task<IActionResult> Sair(CancellationToken ct)
    {
        if (User.Identity?.IsAuthenticated == true)
        {
            _auditoria.Registrar("Logout", User.Identity.Name);
            await _db.SaveChangesAsync(ct);
        }
        await HttpContext.SignOutAsync(CookieAuthenticationDefaults.AuthenticationScheme);
        HttpContext.User = new ClaimsPrincipal(new ClaimsIdentity());
        TokenAntiforgery.Emitir(HttpContext, _antiforgery);
        return NoContent();
    }

    [HttpPost("alterar-senha")]
    public async Task<IActionResult> AlterarSenha(AlterarSenhaDto dto, CancellationToken ct)
    {
        var id = int.Parse(User.FindFirstValue(ClaimTypes.NameIdentifier)!);
        var usuario = await _db.Usuarios.FirstAsync(u => u.Id == id, ct);

        if (_hasher.VerifyHashedPassword(usuario, usuario.SenhaHash, dto.SenhaAtual) == PasswordVerificationResult.Failed)
            return Erro("A senha atual não confere.");

        var problema = ValidarSenha(dto.NovaSenha, usuario.Login);
        if (problema is not null) return Erro(problema);

        usuario.SenhaHash = _hasher.HashPassword(usuario, dto.NovaSenha);
        usuario.CarimboSeguranca = Guid.NewGuid(); // encerra as demais sessões
        _auditoria.Registrar("SenhaAlterada", usuario.Login);
        await _db.SaveChangesAsync(ct);

        await EntrarAsync(usuario);
        return NoContent();
    }

    [HttpGet("auditoria")]
    public async Task<ActionResult<List<AuditoriaAcesso>>> Auditoria(CancellationToken ct)
        => await _db.AuditoriasAcesso.AsNoTracking().OrderByDescending(a => a.Id).Take(100).ToListAsync(ct);

    private async Task EntrarAsync(Usuario usuario)
    {
        var identidade = new ClaimsIdentity(new[]
        {
            new Claim(ClaimTypes.NameIdentifier, usuario.Id.ToString()),
            new Claim(ClaimTypes.Name, usuario.Login),
            new Claim(ConstantesSeguranca.ClaimCarimbo, usuario.CarimboSeguranca.ToString())
        }, CookieAuthenticationDefaults.AuthenticationScheme);
        var principal = new ClaimsPrincipal(identidade);

        await HttpContext.SignInAsync(CookieAuthenticationDefaults.AuthenticationScheme, principal,
            new AuthenticationProperties { IsPersistent = false, AllowRefresh = true });

        // O token CSRF é vinculado ao usuário: reemite para a nova identidade.
        HttpContext.User = principal;
        TokenAntiforgery.Emitir(HttpContext, _antiforgery);
    }

    private static string? ValidarSenha(string senha, string login)
    {
        if (senha.Length < ConstantesSeguranca.TamanhoMinimoSenha)
            return $"A senha deve ter pelo menos {ConstantesSeguranca.TamanhoMinimoSenha} caracteres.";
        if (senha.Contains(login, StringComparison.OrdinalIgnoreCase))
            return "A senha não pode conter o nome de usuário.";
        return null;
    }
}
