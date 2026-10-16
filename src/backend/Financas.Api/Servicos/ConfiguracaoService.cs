using Financas.Api.Data;
using Financas.Api.Entidades;
using Microsoft.AspNetCore.DataProtection;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// Leitura e gravação de configurações. Valores marcados como protegidos (ex.: Client Secret da Pluggy)
/// são criptografados com ASP.NET Core Data Protection (chaves protegidas por DPAPI no Windows).
/// </summary>
public class ConfiguracaoService
{
    public const string PluggyClientId = "Pluggy.ClientId";
    public const string PluggyClientSecret = "Pluggy.ClientSecret";
    public const string IntervaloHoras = "Sincronizacao.IntervaloHoras";
    public const string DiasRetroativos = "Sincronizacao.DiasRetroativos";
    public const string DiasPrimeiraCarga = "Sincronizacao.DiasPrimeiraCarga";
    public const string SincronizacaoHabilitada = "Sincronizacao.Habilitada";
    public const string UltimaSincronizacaoGeral = "Sincronizacao.UltimaExecucao";
    public const string DiasProximosVencimentos = "Painel.DiasProximosVencimentos";
    public const string DiasAvisoConsentimento = "Painel.DiasAvisoConsentimento";
    public const string MaximoTentativasLogin = "Seguranca.MaximoTentativasLogin";
    public const string MinutosBloqueio = "Seguranca.MinutosBloqueio";
    public const string OrcamentoFolgaEssenciais = "Orcamento.FolgaEssenciais";
    public const string OrcamentoMetaPoupanca = "Orcamento.MetaPoupanca";
    public const string OrcamentoCartaoAtencao = "Orcamento.CartaoAtencao";
    public const string OrcamentoCartaoAlerta = "Orcamento.CartaoAlerta";
    public const string OrcamentoCorteCartao = "Orcamento.CorteCartao";
    public const string MetasMargemSeguranca = "Metas.MargemSeguranca";

    private readonly FinancasDbContext _db;
    private readonly IDataProtector _protetor;

    public ConfiguracaoService(FinancasDbContext db, IDataProtectionProvider provedor)
    {
        _db = db;
        _protetor = provedor.CreateProtector("FinancasLocal.Configuracao.v1");
    }

    public async Task<string?> ObterAsync(string chave, CancellationToken ct = default)
    {
        var item = await _db.Configuracoes.AsNoTracking().FirstOrDefaultAsync(c => c.Chave == chave, ct);
        if (item?.Valor is null) return null;
        return item.Protegido ? _protetor.Unprotect(item.Valor) : item.Valor;
    }

    public async Task<bool> ExisteAsync(string chave, CancellationToken ct = default)
        => await _db.Configuracoes.AnyAsync(c => c.Chave == chave && c.Valor != null, ct);

    public async Task<int> ObterInteiroAsync(string chave, int padrao, CancellationToken ct = default)
        => int.TryParse(await ObterAsync(chave, ct), out var v) ? v : padrao;

    public async Task<bool> ObterBoolAsync(string chave, bool padrao, CancellationToken ct = default)
        => bool.TryParse(await ObterAsync(chave, ct), out var v) ? v : padrao;

    public async Task<DateTime?> ObterDataAsync(string chave, CancellationToken ct = default)
        => DateTime.TryParse(await ObterAsync(chave, ct), null, System.Globalization.DateTimeStyles.RoundtripKind, out var v)
            ? v : null;

    /// <summary>Define o valor (não salva: chame SaveChangesAsync no DbContext).</summary>
    public async Task DefinirAsync(string chave, string? valor, bool protegido = false, CancellationToken ct = default)
    {
        var item = await _db.Configuracoes.FirstOrDefaultAsync(c => c.Chave == chave, ct);
        if (item is null)
        {
            item = new Configuracao { Chave = chave };
            _db.Configuracoes.Add(item);
        }
        item.Protegido = protegido;
        item.Valor = valor is null ? null : protegido ? _protetor.Protect(valor) : valor;
        item.AtualizadoEm = DateTime.UtcNow;
    }
}
