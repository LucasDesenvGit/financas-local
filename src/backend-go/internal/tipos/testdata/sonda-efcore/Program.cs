// Sonda do formato do EF Core (#74): grava valores escolhidos num SQLite descartável pelo mesmo EF Core e o mesmo
// provedor da API, e escreve ../efcore.json com, para cada caso, o texto cru que ficou no banco e o JSON que o
// ASP.NET devolveria (JsonSerializerDefaults.Web) depois de ler de volta. Os testes do pacote tipos usam esse arquivo
// como gabarito. Rodar de novo só quando o EF Core mudar de versão:
//   ferramentas/dotnet/dotnet run --project src/backend-go/internal/tipos/testdata/sonda-efcore
using System.Text.Json;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;

var caminho = Path.Combine(Path.GetTempPath(), $"sonda-efcore-{Guid.NewGuid():N}.db");
var casos = new List<Amostra>
{
    new() { Nome = "zero", Valor = 0m, Preciso = 0m },
    new() { Nome = "inteiro-negativo", Valor = -5000m, Preciso = -5000m },
    new() { Nome = "uma-casa", Valor = 4864.5m, Preciso = 4864.5m },
    new() { Nome = "zero-a-direita", Valor = 1.50m, Preciso = 1.50m },
    new() { Nome = "meio-centavo", Valor = 0.125m, Preciso = 0.125m },
    new() { Nome = "oito-casas", Valor = 12345678.12345678m, Preciso = 12345678.12345678m },
    new() { Nome = "centavo-negativo", Valor = -0.01m, Preciso = -0.01m },
    new() { Nome = "cem-sem-casas", Valor = 100m, Preciso = 100m },
    new() { Nome = "nulo", Valor = 7m, Opcional = null },
};
var datas = new DateTime[]
{
    new DateTime(2026, 10, 15, 22, 55, 31, DateTimeKind.Unspecified).AddTicks(3481471),
    new DateTime(2026, 1, 1, 0, 0, 0, DateTimeKind.Unspecified),
    new DateTime(2026, 10, 15, 12, 0, 0, 500, DateTimeKind.Utc),
    new DateTime(2026, 2, 28, 23, 59, 59, DateTimeKind.Unspecified).AddTicks(1000000),
};
for (var i = 0; i < casos.Count; i++)
{
    casos[i].Opcional = casos[i].Nome == "nulo" ? null : casos[i].Valor;
    casos[i].Momento = datas[i % datas.Length];
    casos[i].Dia = new DateOnly(2026, 10, 1 + i);
    casos[i].Identificador = i == 0 ? Guid.Parse("2f00defb-672e-45ef-87d0-702995ae8d2e") : Guid.Empty;
    casos[i].Ativo = i % 2 == 0;
    casos[i].Tipo = (TipoAmostra)(i % 3);
}

using (var ctx = new Contexto(caminho))
{
    ctx.Database.EnsureCreated();
    ctx.Amostras.AddRange(casos);
    ctx.SaveChanges();
}

// PRAGMAs da conexão que o EF Core abre (o Go abre com os mesmos, #74).
var pragmas = new Dictionary<string, string>();
using (var ctx = new Contexto(caminho))
{
    var conexaoEf = ctx.Database.GetDbConnection();
    conexaoEf.Open();
    foreach (var nome in new[] { "foreign_keys", "journal_mode", "busy_timeout", "synchronous" })
    {
        using var comando = conexaoEf.CreateCommand();
        comando.CommandText = $"PRAGMA {nome};";
        pragmas[nome] = comando.ExecuteScalar()?.ToString() ?? "";
    }
}

var json = new JsonSerializerOptions(JsonSerializerDefaults.Web);
var saida = new List<object>();
using (var ctx = new Contexto(caminho))
using (var conexao = new SqliteConnection($"Data Source={caminho}"))
{
    conexao.Open();
    foreach (var lida in ctx.Amostras.AsNoTracking().OrderBy(a => a.Id).ToList())
    {
        var cru = new Dictionary<string, object?>();
        using var comando = conexao.CreateCommand();
        comando.CommandText = "SELECT Valor, Preciso, Opcional, Momento, Dia, Identificador, Ativo, Tipo, " +
            "typeof(Valor), typeof(Momento), typeof(Identificador) FROM Amostras WHERE Id = $id";
        comando.Parameters.AddWithValue("$id", lida.Id);
        using var leitor = comando.ExecuteReader();
        leitor.Read();
        string[] nomes = ["Valor", "Preciso", "Opcional", "Momento", "Dia", "Identificador", "Ativo", "Tipo"];
        for (var c = 0; c < nomes.Length; c++)
            cru[nomes[c]] = leitor.IsDBNull(c) ? null : leitor.GetValue(c).ToString();
        cru["typeof(Valor)"] = leitor.GetString(8);
        cru["typeof(Momento)"] = leitor.GetString(9);
        cru["typeof(Identificador)"] = leitor.GetString(10);
        saida.Add(new
        {
            caso = lida.Nome,
            noBanco = cru,
            json = JsonDocument.Parse(JsonSerializer.Serialize(lida, json)).RootElement,
        });
    }
}
SqliteConnection.ClearAllPools();
File.Delete(caminho);

var destino = Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "efcore.json");
File.WriteAllText(destino, JsonSerializer.Serialize(new
{
    efCore = typeof(DbContext).Assembly.GetName().Version!.ToString(),
    pragmas,
    casos = saida,
}, new JsonSerializerOptions { WriteIndented = true, Encoder = System.Text.Encodings.Web.JavaScriptEncoder.UnsafeRelaxedJsonEscaping }) + "\n");
Console.WriteLine($"Gravado: {Path.GetFullPath(destino)}");

enum TipoAmostra { Zero, Um, Dois }

class Amostra
{
    public int Id { get; set; }
    public string Nome { get; set; } = "";
    public decimal Valor { get; set; }
    public decimal Preciso { get; set; }
    public decimal? Opcional { get; set; }
    public DateTime Momento { get; set; }
    public DateOnly Dia { get; set; }
    public Guid Identificador { get; set; }
    public bool Ativo { get; set; }
    public TipoAmostra Tipo { get; set; }
}

class Contexto(string caminho) : DbContext
{
    public DbSet<Amostra> Amostras => Set<Amostra>();
    protected override void OnConfiguring(DbContextOptionsBuilder o) => o.UseSqlite($"Data Source={caminho}");
    protected override void OnModelCreating(ModelBuilder m) =>
        m.Entity<Amostra>().Property(a => a.Preciso).HasPrecision(18, 2);
}
