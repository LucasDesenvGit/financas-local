using System.Diagnostics;
using System.Globalization;
using System.Reflection;
using System.Runtime.InteropServices;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>Um parâmetro editável pela tela de Parametrização (gravado na tabela Configuracao).</summary>
public sealed record DefinicaoParametro(string Chave, string Grupo, string Nome, string Descricao, TipoParametro Tipo,
    int Padrao, int Minimo = 0, int Maximo = 0, string? Unidade = null, string? ChaveAppSettings = null);

public enum TipoParametro : byte
{
    Inteiro = 1,
    Booleano = 2
}

/// <summary>
/// Parametrização do sistema pelo site. O catálogo abaixo é a única fonte de nomes, faixas e valores padrão:
/// a tela é montada a partir dele e a gravação valida contra ele. Segredos (Client Secret, token da brapi)
/// continuam nos fluxos próprios, criptografados, e não passam por aqui.
/// </summary>
public class ParametroService
{
    public static readonly IReadOnlyList<DefinicaoParametro> Catalogo =
    [
        new(ConfiguracaoService.SincronizacaoHabilitada, "Sincronização", "Sincronização automática",
            "Busca os dados dos bancos sozinha, no intervalo abaixo. Desligada, só sincroniza pelo botão.",
            TipoParametro.Booleano, Padrao: 1),
        new(ConfiguracaoService.IntervaloHoras, "Sincronização", "Intervalo entre sincronizações",
            "De quanto em quanto tempo a sincronização automática roda. O Meu Pluggy atualiza os bancos cerca de uma vez por dia.",
            TipoParametro.Inteiro, 6, 1, 48, "horas"),
        new(ConfiguracaoService.DiasRetroativos, "Sincronização", "Dias relidos a cada sincronização",
            "Quantos dias para trás são relidos para pegar transações que mudaram ou entraram atrasadas.",
            TipoParametro.Inteiro, 30, 1, 365, "dias"),
        new(ConfiguracaoService.DiasPrimeiraCarga, "Sincronização", "Histórico na primeira carga",
            "Quantos dias de histórico são buscados quando uma conta é sincronizada pela primeira vez.",
            TipoParametro.Inteiro, 365, 30, 1825, "dias"),

        new(ConfiguracaoService.DiasProximosVencimentos, "Painel", "Próximos vencimentos",
            "Até quantos dias à frente o painel lista as contas a pagar.",
            TipoParametro.Inteiro, 15, 1, 90, "dias"),
        new(ConfiguracaoService.DiasAvisoConsentimento, "Painel", "Aviso de consentimento expirando",
            "Com quantos dias de antecedência o painel avisa que o consentimento do Open Finance vai expirar.",
            TipoParametro.Inteiro, 30, 1, 120, "dias"),

        new(ConfiguracaoService.MaximoTentativasLogin, "Segurança", "Tentativas de login antes do bloqueio",
            "Quantas senhas erradas seguidas bloqueiam o acesso temporariamente.",
            TipoParametro.Inteiro, 5, 3, 20, "tentativas", "Seguranca:MaximoTentativasLogin"),
        new(ConfiguracaoService.MinutosBloqueio, "Segurança", "Tempo de bloqueio",
            "Por quanto tempo o login fica bloqueado depois do limite de tentativas.",
            TipoParametro.Inteiro, 15, 1, 1440, "minutos", "Seguranca:MinutosBloqueio"),
    ];

    private static readonly DateTime IniciadoEm = Process.GetCurrentProcess().StartTime.ToUniversalTime();

    private readonly FinancasDbContext _db;
    private readonly ConfiguracaoService _config;
    private readonly IConfiguration _cfg;

    public ParametroService(FinancasDbContext db, ConfiguracaoService config, IConfiguration cfg)
    {
        _db = db;
        _config = config;
        _cfg = cfg;
    }

    /// <summary>Valor padrão: o do appsettings, quando o parâmetro existia lá antes, ou o do catálogo.</summary>
    public int Padrao(DefinicaoParametro p)
        => p.ChaveAppSettings is { } chave ? _cfg.GetValue(chave, p.Padrao) : p.Padrao;

    /// <summary>Valor atual de um parâmetro inteiro do catálogo, limitado à faixa válida.</summary>
    public async Task<int> ObterInteiroAsync(string chave, CancellationToken ct = default)
    {
        var p = Definicao(chave);
        var valor = await _config.ObterInteiroAsync(chave, Padrao(p), ct);
        return Math.Clamp(valor, p.Minimo, p.Maximo);
    }

    public async Task<List<ParametroDto>> ListarAsync(CancellationToken ct)
    {
        var chaves = Catalogo.Select(p => p.Chave).ToList();
        var gravados = await _db.Configuracoes.AsNoTracking()
            .Where(c => chaves.Contains(c.Chave) && !c.Protegido)
            .ToDictionaryAsync(c => c.Chave, c => c.Valor, ct);

        return Catalogo.Select(p =>
        {
            var padrao = Padrao(p);
            var valor = gravados.GetValueOrDefault(p.Chave) is { } texto ? Normalizar(p, texto) ?? padrao : padrao;
            return new ParametroDto(p.Chave, p.Grupo, p.Nome, p.Descricao, p.Tipo, valor, padrao,
                p.Tipo == TipoParametro.Inteiro ? p.Minimo : null, p.Tipo == TipoParametro.Inteiro ? p.Maximo : null, p.Unidade);
        }).ToList();
    }

    /// <summary>Valida tudo antes de gravar: ou salva todos os valores enviados, ou nenhum.</summary>
    public async Task<List<string>> SalvarAsync(Dictionary<string, int> valores, CancellationToken ct)
    {
        var erros = new List<string>();
        foreach (var (chave, valor) in valores)
        {
            var p = Catalogo.FirstOrDefault(d => d.Chave == chave);
            if (p is null) erros.Add($"Parâmetro desconhecido: {chave}.");
            else if (p.Tipo == TipoParametro.Booleano && valor is not (0 or 1)) erros.Add($"{p.Nome}: use ligado ou desligado.");
            else if (p.Tipo == TipoParametro.Inteiro && (valor < p.Minimo || valor > p.Maximo))
                erros.Add($"{p.Nome}: informe um valor entre {p.Minimo} e {p.Maximo}{(p.Unidade is null ? "" : " " + p.Unidade)}.");
        }
        if (erros.Count > 0) return erros;

        foreach (var (chave, valor) in valores)
        {
            var p = Definicao(chave);
            var texto = p.Tipo == TipoParametro.Booleano
                ? (valor == 1 ? "true" : "false")
                : valor.ToString(CultureInfo.InvariantCulture);
            await _config.DefinirAsync(chave, texto, ct: ct);
        }
        await _db.SaveChangesAsync(ct);
        return erros;
    }

    /// <summary>Informações de diagnóstico, somente leitura (não expõe segredos).</summary>
    public async Task<InfoSistemaDto> InfoSistemaAsync(CancellationToken ct)
    {
        var caminhoBanco = Path.GetFullPath(_cfg["Banco:Caminho"] ?? Path.Combine(AppContext.BaseDirectory, "dados", "sql", "FinancasLocal.db"));
        var pastaChaves = _cfg["Seguranca:PastaChaves"] is { Length: > 0 } pasta
            ? Path.GetFullPath(pasta)
            : Path.Combine(AppContext.BaseDirectory, "chaves");
        long? tamanho = File.Exists(caminhoBanco) ? new FileInfo(caminhoBanco).Length : null;
        var versao = Assembly.GetEntryAssembly()?.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion
                     ?? Assembly.GetEntryAssembly()?.GetName().Version?.ToString() ?? "—";
        var indiceMais = versao.IndexOf('+');
        if (indiceMais > 0) versao = versao[..indiceMais];

        return new InfoSistemaDto(
            versao,
            RuntimeInformation.FrameworkDescription,
            RuntimeInformation.OSDescription,
            IniciadoEm,
            caminhoBanco,
            tamanho,
            pastaChaves,
            await _db.Contas.CountAsync(ct),
            await _db.Transacoes.CountAsync(ct),
            await _db.ConexoesBancarias.CountAsync(c => c.Ativa, ct),
            await _config.ObterDataAsync(ConfiguracaoService.UltimaSincronizacaoGeral, ct));
    }

    private static DefinicaoParametro Definicao(string chave)
        => Catalogo.FirstOrDefault(p => p.Chave == chave)
           ?? throw new ArgumentException($"Parâmetro fora do catálogo: {chave}", nameof(chave));

    private static int? Normalizar(DefinicaoParametro p, string texto)
        => p.Tipo == TipoParametro.Booleano
            ? bool.TryParse(texto, out var b) ? (b ? 1 : 0) : null
            : int.TryParse(texto, NumberStyles.Integer, CultureInfo.InvariantCulture, out var n) ? Math.Clamp(n, p.Minimo, p.Maximo) : null;
}
