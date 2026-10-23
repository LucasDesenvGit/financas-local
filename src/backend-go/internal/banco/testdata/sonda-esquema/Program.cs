// Sonda do EsquemaBanco (#297): compila o mesmo Data/EsquemaBanco.cs da API, com os mesmos scripts embutidos, roda
// os cenários que o teste do Go repete (internal/banco/esquema_banco_test.go) e escreve em ../:
//   - esquema-dotnet.json: o gabarito (assinaturas, linhas lidas do .sh, e a estrutura e os dados de cada banco);
//   - entradas-esquema/*.gz: os scripts com que o gabarito foi feito, para o Go aplicar exatamente os mesmos (assim o
//     teste não fica velho quando o banco/ muda).
// Rodar de novo quando o EsquemaBanco.cs mudar, ou para conferir o Go com os scripts de agora:
//   ~/.dotnet/dotnet run --project src/backend-go/internal/banco/testdata/sonda-esquema
using System.IO.Compression;
using System.Reflection;
using System.Security.Cryptography;
using System.Text;
using System.Text.Encodings.Web;
using System.Text.Json;
using System.Text.RegularExpressions;
using Financas.Api.Data;
using Microsoft.Data.Sqlite;

const string Prefixo = "FinancasLocal-antes-atualizacao-";
var testdata = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", ".."));
var raiz = Directory.CreateTempSubdirectory("sonda-esquema-").FullName;
var log = new LogNenhum();
var cenarios = new Dictionary<string, object>();

try
{
    string Banco(params string[] partes) => Path.Combine([raiz, .. partes]);

    // 1. Banco novo: estrutura, colunas e dados iniciais.
    var novo = Banco("novo", "dados", "sql", "FinancasLocal.db");
    EsquemaBanco.Aplicar(novo, log);
    cenarios["novo"] = new { banco = Despejar(novo), copias = PastasComCopia(Banco("novo")) };

    // 2. Os mesmos scripts de novo: nada muda no arquivo, sem cópia.
    var antes = Impressao(novo);
    EsquemaBanco.Aplicar(novo, log);
    cenarios["semMudanca"] = new { arquivoIgual = Impressao(novo) == antes, copias = PastasComCopia(Banco("novo")) };

    // 2b. Arquivo vazio (0 byte) conta como banco novo: recebe os dados iniciais e não ganha cópia.
    var vazio = Banco("vazio", "dados", "sql", "FinancasLocal.db");
    Directory.CreateDirectory(Path.GetDirectoryName(vazio)!);
    File.WriteAllBytes(vazio, []);
    EsquemaBanco.Aplicar(vazio, log);
    cenarios["vazio"] = new { banco = Resumir(Despejar(vazio)), copias = PastasComCopia(Banco("vazio")) };

    // 3. Banco de uma versão antiga: faltam duas colunas que só as linhas adicionar_coluna_se_ausente acrescentam.
    var antigo = Banco("antigo", "dados", "sql", "FinancasLocal.db");
    CopiarBanco(novo, antigo);
    Sql(antigo, "ALTER TABLE Meta DROP COLUMN Icone; ALTER TABLE RegraDescricao DROP COLUMN ValorMaximo; PRAGMA user_version = 7;");
    var antigoAntes = Despejar(antigo);
    EsquemaBanco.Aplicar(antigo, log);
    var copiaAntigo = Directory.GetFiles(Banco("antigo", "dados", "backups"), Prefixo + "*.db").Single();
    cenarios["antigo"] = new
    {
        antes = Resumir(antigoAntes),
        depois = Resumir(Despejar(antigo)),
        copia = Resumir(Despejar(copiaAntigo)),
        nomeDaCopiaNoFormato = Regex.IsMatch(Path.GetFileName(copiaAntigo), @"^FinancasLocal-antes-atualizacao-\d{8}_\d{6}\.db$"),
        copias = PastasComCopia(Banco("antigo")),
    };

    // 4. Rodízio: ficam as 5 cópias mais recentes pelo nome; o resto da pasta não é tocado.
    var rodizio = Banco("rodizio", "dados", "sql", "FinancasLocal.db");
    CopiarBanco(novo, rodizio);
    Sql(rodizio, "PRAGMA user_version = 7;");
    var pastaRodizio = Banco("rodizio", "dados", "backups");
    Directory.CreateDirectory(pastaRodizio);
    foreach (var nome in ArquivosDoRodizio) File.WriteAllText(Path.Combine(pastaRodizio, nome), "");
    foreach (var nome in PastasDoRodizio) Directory.CreateDirectory(Path.Combine(pastaRodizio, nome));
    EsquemaBanco.Aplicar(rodizio, log);
    cenarios["rodizio"] = new
    {
        arquivos = ArquivosDoRodizio,
        pastas = PastasDoRodizio,
        restantes = Directory.GetFileSystemEntries(pastaRodizio).Select(Path.GetFileName)
            .Select(n => ArquivosDoRodizio.Contains(n) || PastasDoRodizio.Contains(n) ? n! : "<nova>")
            .Order(StringComparer.Ordinal).ToArray(),
    };

    // 5. Onde fica a cópia: dados/backups quando o banco está numa pasta "sql" (sem diferença de maiúsculas), senão
    //    backups ao lado do banco.
    foreach (var (cenario, partes) in new[]
             {
                 ("pastaSqlMaiuscula", new[] { "dados", "SQL", "FinancasLocal.db" }),
                 ("pastaSemSql", new[] { "outra", "FinancasLocal.db" }),
             })
    {
        var caminho = Banco([cenario, .. partes]);
        CopiarBanco(novo, caminho);
        Sql(caminho, "PRAGMA user_version = 7;");
        EsquemaBanco.Aplicar(caminho, log);
        cenarios[cenario] = new { copias = PastasComCopia(Banco(cenario)) };
    }

    // A conexão do jeito que o EsquemaBanco abre (o Go tem de abrir igual).
    var pragmas = new Dictionary<string, string>();
    using (var conexao = new SqliteConnection(new SqliteConnectionStringBuilder
           { DataSource = novo, Pooling = false, DefaultTimeout = 30 }.ToString()))
    {
        conexao.Open();
        foreach (var nome in new[] { "foreign_keys", "busy_timeout", "journal_mode" })
            pragmas[nome] = Escalar(conexao, $"PRAGMA {nome};")!.ToString()!;
        pragmas["sqlite_version()"] = Escalar(conexao, "SELECT sqlite_version();")!.ToString()!;
    }

    // Assinatura e leitura das linhas, chamando os métodos privados do EsquemaBanco.
    var tipo = typeof(EsquemaBanco);
    var assinar = tipo.GetMethod("Assinatura", BindingFlags.NonPublic | BindingFlags.Static)!;
    var linhaColuna = (Regex)tipo.GetMethod("LinhaColuna", BindingFlags.NonPublic | BindingFlags.Static)!.Invoke(null, null)!;
    var estrutura = Recurso("01-estrutura.sql");
    var colunas = linhaColuna.Matches(Recurso("2-criar-banco.sh"))
        .Select(m => (m.Groups[1].Value, m.Groups[2].Value, m.Groups[3].Value)).ToArray();
    int Assinar(string texto, (string, string, string)[] lista) => (int)assinar.Invoke(null, [texto, lista])!;

    var assinaturas = new Dictionary<string, int>
    {
        ["scripts"] = Assinar(estrutura, colunas),
        ["scriptsComCrlf"] = Assinar(estrutura.Replace("\n", "\r\n"), colunas),
        ["vazio"] = Assinar("", []),
        ["acentos"] = Assinar("CREATE TABLE Ação (Descrição TEXT);\r\n",
            [("A", "B", "TEXT NULL"), ("C", "D", "TEXT NOT NULL DEFAULT 'é'")]),
    };
    var leitura = linhaColuna.Matches(TextoDeLeitura).Select(m => $"{m.Groups[1].Value}|{m.Groups[2].Value}|{m.Groups[3].Value}").ToArray();

    // Os scripts embutidos, como o gabarito os viu.
    var pastaEntradas = Path.Combine(testdata, "entradas-esquema");
    Directory.CreateDirectory(pastaEntradas);
    var entradas = new Dictionary<string, string>();
    foreach (var nome in new[] { "01-estrutura.sql", "02-dados-iniciais.sql", "2-criar-banco.sh" })
    {
        var bytes = RecursoBytes(nome);
        entradas[nome] = Hash(bytes);
        using var arquivo = File.Create(Path.Combine(pastaEntradas, nome + ".gz"));
        using var gz = new GZipStream(arquivo, CompressionLevel.SmallestSize);
        gz.Write(bytes);
    }

    var destino = Path.Combine(testdata, "esquema-dotnet.json");
    File.WriteAllText(destino, JsonSerializer.Serialize(new
    {
        geradoPor = "testdata/sonda-esquema (Data/EsquemaBanco.cs da API)",
        microsoftDataSqlite = typeof(SqliteConnection).Assembly.GetName().Version!.ToString(),
        pragmas,
        entradas,
        assinaturas,
        textoDeLeitura = TextoDeLeitura,
        leitura,
        cenarios,
    }, new JsonSerializerOptions
    {
        WriteIndented = true, Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping, PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
    }) + "\n");
    Console.WriteLine($"Gravado: {destino}");
}
finally
{
    SqliteConnection.ClearAllPools();
    Directory.Delete(raiz, recursive: true);
}

// ------------------------------------------------------------------------------------------------- apoio

// Estrutura e dados de um banco, pelas mesmas consultas que o teste do Go faz (despejar, no esquema_banco_test.go).
static Despejo Despejar(string caminho)
{
    using var conexao = new SqliteConnection(new SqliteConnectionStringBuilder
        { DataSource = caminho, Pooling = false, Mode = SqliteOpenMode.ReadOnly }.ToString());
    conexao.Open();
    var tabelas = new List<Tabela>();
    foreach (var (nome, sql) in Linhas(conexao, "SELECT name, sql FROM sqlite_master WHERE type = 'table' ORDER BY name", null,
                 l => (l.GetString(0), l.GetString(1))))
    {
        var colunas = Linhas(conexao,
            "SELECT cid || '|' || name || '|' || type || '|' || \"notnull\" || '|' || ifnull(dflt_value, '<null>') || '|' || pk " +
            "FROM pragma_table_info($t) ORDER BY cid", nome, l => l.GetString(0));
        var indices = new List<string>();
        foreach (var (indice, resto) in Linhas(conexao,
                     "SELECT name, \"unique\" || '|' || origin || '|' || partial FROM pragma_index_list($t) ORDER BY name", nome,
                     l => (l.GetString(0), l.GetString(1))))
        {
            var colunasDoIndice = Linhas(conexao, "SELECT ifnull(name, '<expr>') FROM pragma_index_info($t) ORDER BY seqno", indice,
                l => l.GetString(0));
            var sqlIndice = Linhas(conexao, "SELECT sql FROM sqlite_master WHERE type = 'index' AND name = $t", indice,
                l => l.IsDBNull(0) ? null : l.GetString(0)).Single();
            indices.Add($"{indice}|{resto}|{string.Join(",", colunasDoIndice)}|{(sqlIndice is null ? "<sem sql>" : Hash(sqlIndice))}");
        }
        var chaves = Linhas(conexao,
            "SELECT id || '|' || seq || '|' || \"table\" || '|' || \"from\" || '|' || ifnull(\"to\", '<null>') || '|' || " +
            "on_update || '|' || on_delete || '|' || \"match\" FROM pragma_foreign_key_list($t) ORDER BY id, seq", nome,
            l => l.GetString(0));
        var nomesColunas = Linhas(conexao, "SELECT name FROM pragma_table_info($t) ORDER BY cid", nome, l => l.GetString(0));
        var dados = Linhas(conexao, $"SELECT {ExpressaoDaLinha(nomesColunas)} FROM \"{nome}\" ORDER BY rowid", null, l => l.GetString(0));
        tabelas.Add(new Tabela(nome, Hash(sql), colunas, indices, chaves, dados.Count, Hash(string.Join("\n", dados))));
    }
    return new Despejo(
        Convert.ToInt64(Escalar(conexao, "PRAGMA user_version;")),
        tabelas,
        Linhas(conexao,
            "SELECT type || '|' || name || '|' || tbl_name || '|' || ifnull(sql, '<null>') FROM sqlite_master " +
            "WHERE type NOT IN ('table', 'index') ORDER BY type, name", null, l => l.GetString(0)));
}

// O despejo em poucas linhas: um hash por tabela (o mesmo texto que o resumir do Go monta).
static object Resumir(Despejo d) => new
{
    userVersion = d.UserVersion,
    tabelas = d.Tabelas.ToDictionary(t => t.Nome, t => Hash(string.Join("\n", new[] { t.Nome, "sql:" + t.SqlHash }
        .Concat(t.Colunas.Select(c => "coluna:" + c)).Concat(t.Indices.Select(i => "indice:" + i))
        .Concat(t.ChavesEstrangeiras.Select(f => "fk:" + f)).Append("linhas:" + t.Linhas).Append("dados:" + t.DadosHash)))),
    outros = d.Outros,
};

// Uma linha da tabela como texto (quote() do SQLite); a data e hora de agora dos dados iniciais vira <data-hora>.
static string ExpressaoDaLinha(List<string> colunas) => string.Join(" || '|' || ", colunas.Select(c =>
    $"CASE WHEN typeof(\"{c}\") = 'text' AND \"{c}\" GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:*' " +
    $"THEN '''<data-hora>''' ELSE quote(\"{c}\") END"));

static List<T> Linhas<T>(SqliteConnection conexao, string sql, string? parametro, Func<SqliteDataReader, T> ler)
{
    using var comando = conexao.CreateCommand();
    comando.CommandText = sql;
    if (parametro is not null) comando.Parameters.AddWithValue("$t", parametro);
    using var leitor = comando.ExecuteReader();
    var lista = new List<T>();
    while (leitor.Read()) lista.Add(ler(leitor));
    return lista;
}

static object? Escalar(SqliteConnection conexao, string sql)
{
    using var comando = conexao.CreateCommand();
    comando.CommandText = sql;
    return comando.ExecuteScalar();
}

static void Sql(string caminho, string sql)
{
    using var conexao = new SqliteConnection($"Data Source={caminho};Pooling=False");
    conexao.Open();
    using var comando = conexao.CreateCommand();
    comando.CommandText = sql;
    comando.ExecuteNonQuery();
}

static void CopiarBanco(string origem, string destino)
{
    Directory.CreateDirectory(Path.GetDirectoryName(destino)!);
    File.Copy(origem, destino);
}

// Pastas (relativas ao cenário, com "/") que receberam uma cópia "antes da atualização", e quantas.
static string[] PastasComCopia(string pastaCenario) =>
    Directory.GetFiles(pastaCenario, Prefixo + "*.db", SearchOption.AllDirectories)
        .Select(f => Path.GetRelativePath(pastaCenario, Path.GetDirectoryName(f)!).Replace('\\', '/'))
        .GroupBy(p => p).Select(g => $"{g.Key}: {g.Count()}").Order(StringComparer.Ordinal).ToArray();

static string Impressao(string caminho) => Hash(File.ReadAllBytes(caminho)) + "@" + File.GetLastWriteTimeUtc(caminho).Ticks;

static string Hash(object conteudo) => Convert.ToHexStringLower(SHA256.HashData(
    conteudo as byte[] ?? Encoding.UTF8.GetBytes((string)conteudo)));

static byte[] RecursoBytes(string nome)
{
    using var fluxo = typeof(EsquemaBanco).Assembly.GetManifestResourceStream("banco." + nome)!;
    using var memoria = new MemoryStream();
    fluxo.CopyTo(memoria);
    return memoria.ToArray();
}

// Como o EsquemaBanco.Ler lê (StreamReader em UTF-8).
static string Recurso(string nome)
{
    using var leitor = new StreamReader(new MemoryStream(RecursoBytes(nome)), Encoding.UTF8);
    return leitor.ReadToEnd();
}

partial class Program
{
    // Arquivos e pastas que já estão em dados/backups antes da atualização, no cenário do rodízio.
    static readonly string[] ArquivosDoRodizio =
    [
        "FinancasLocal-antes-atualizacao-20200101_000000.db",
        "FinancasLocal-antes-atualizacao-20200102_000000.db",
        "FinancasLocal-antes-atualizacao-20200103_000000.db",
        "FinancasLocal-antes-atualizacao-20200104_000000.db",
        "FinancasLocal-antes-atualizacao-20200105_000000.db",
        "FinancasLocal-antes-atualizacao-20200106_000000.db",
        "FinancasLocal-antes-atualizacao-.db",
        "FinancasLocal-antes-atualizacao-20200107_000000.db.txt",
        "FinancasLocal-antes-instalacao-20200101_000000.db",
        "outro.db",
    ];
    static readonly string[] PastasDoRodizio = ["FinancasLocal-antes-atualizacao-29991231_235959.db"];

    // Linhas para conferir a leitura do .sh: espaços do .NET (o \s dele pega \v, \f, \x85, NBSP e os espaços do Unicode),
    // comentário, aspas simples, acento no nome, definição em duas linhas e CRLF.
    const string TextoDeLeitura =
        "adicionar_coluna_se_ausente \"A\" \"B\" \"TEXT NULL\"\n" +
        "   adicionar_coluna_se_ausente  \"C\"\t\"D\" \"INTEGER NOT NULL DEFAULT 0\" # comentário\n" +
        "# adicionar_coluna_se_ausente \"X\" \"Y\" \"TEXT\"\n" +
        "\tadicionar_coluna_se_ausente \"E\" \"F\" \"TEXT NOT NULL DEFAULT 'BRL'\"\n" +
        "echo adicionar_coluna_se_ausente \"G\" \"H\" \"TEXT\"\n" +
        "adicionar_coluna_se_ausente \"Ação\" \"I\" \"TEXT\"\n" +
        "adicionar_coluna_se_ausente 'J' 'K' 'TEXT'\n" +
        "adicionar_coluna_se_ausente \"L\" \"M\" \"duas\nlinhas\"\n" +
        "\u000badicionar_coluna_se_ausente \"N\" \"O\" \"TEXT\"\r\n" +
        "adicionar_coluna_se_ausente \"P\" \"Q\" \"TEXT NULL\"\r\n" +
        "\u0085\u2028adicionar_coluna_se_ausente\u0085\"T\"\u3000\"U\"\f\"TEXT\"\n" +
        "adicionar_coluna_se_ausente \"R\" \"S\"\n";
}

record Despejo(long UserVersion, List<Tabela> Tabelas, List<string> Outros);
record Tabela(string Nome, string SqlHash, List<string> Colunas, List<string> Indices, List<string> ChavesEstrangeiras, int Linhas, string DadosHash);

// O EsquemaBanco só registra mensagens informativas; a sonda não precisa delas.
sealed class LogNenhum : ILogger
{
    public IDisposable? BeginScope<TState>(TState state) where TState : notnull => null;
    public bool IsEnabled(LogLevel logLevel) => false;
    public void Log<TState>(LogLevel logLevel, EventId eventId, TState state, Exception? exception, Func<TState, Exception?, string> formatter) { }
}
