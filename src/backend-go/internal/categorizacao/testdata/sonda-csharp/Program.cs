// Gabarito dos acertos da partida (#444), chamado no código da própria API: as três rotinas do CategorizacaoService que
// o Program.cs roda ao subir (regras padrão novas, entradas nos cartões, compras parceladas estornadas), uma depois da
// outra e com o SaveChanges entre elas, sobre o banco de ../acertos-cenario.sql. Depois de cada uma, grava o resultado e o
// que ela deixou nas tabelas (sem os carimbos, que são da hora). Roda também a segunda partida, que não muda nada.
using System.Text.Encodings.Web;
using System.Text.Json;
using System.Text.RegularExpressions;
using Financas.Api.Data;
using Financas.Api.Servicos;
using Microsoft.AspNetCore.DataProtection;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;

var raiz = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "..", "..", "..", "..", ".."));
var pasta = Path.Combine(raiz, "src", "backend-go", "internal", "categorizacao", "testdata");
var arquivo = Path.Combine(Path.GetTempPath(), $"sonda-acertos-{Guid.NewGuid():N}.db");
var hoje = Relogio.Hoje;

using (var c = new SqliteConnection($"Data Source={arquivo};Pooling=False"))
{
    c.Open();
    var cenario = Regex.Replace(File.ReadAllText(Path.Combine(pasta, "acertos-cenario.sql")), @"@D(-?\d+)",
        m => hoje.AddDays(int.Parse(m.Groups[1].Value)).ToString("yyyy-MM-dd"));
    foreach (var texto in new[] { File.ReadAllText(Path.Combine(raiz, "banco", "01-estrutura.sql")), cenario })
    {
        using var cmd = c.CreateCommand();
        cmd.CommandText = texto;
        cmd.ExecuteNonQuery();
    }
}

List<Dictionary<string, object?>> LerTabela(SqliteConnection c, string consulta)
{
    using var cmd = c.CreateCommand();
    cmd.CommandText = consulta;
    using var leitor = cmd.ExecuteReader();
    var linhas = new List<Dictionary<string, object?>>();
    while (leitor.Read())
    {
        var linha = new Dictionary<string, object?>();
        for (var i = 0; i < leitor.FieldCount; i++) linha[leitor.GetName(i)] = leitor.IsDBNull(i) ? null : leitor.GetValue(i);
        linhas.Add(linha);
    }
    return linhas;
}

var passos = new List<object>();
async Task Passo(string nome, Func<FinancasDbContext, CategorizacaoService, ConfiguracaoService, Task<int>> acao)
{
    int resultado;
    using (var db = new FinancasDbContext(new DbContextOptionsBuilder<FinancasDbContext>().UseSqlite($"Data Source={arquivo};Pooling=False").Options))
    {
        resultado = await acao(db, new CategorizacaoService(db), new ConfiguracaoService(db, new EphemeralDataProtectionProvider()));
    }
    using var c = new SqliteConnection($"Data Source={arquivo};Pooling=False");
    c.Open();
    passos.Add(new
    {
        nome, mudaram = resultado,
        transacoes = LerTabela(c, "SELECT Id, CategoriaId, CategoriaManual, Tipo, IgnorarRelatorios FROM Transacao ORDER BY Id"),
        regras = LerTabela(c, "SELECT Id, Texto, TipoCorrespondencia, CategoriaId, Prioridade, Ativa FROM RegraCategorizacao ORDER BY Id"),
        marcas = LerTabela(c, "SELECT Chave, Valor IS NOT NULL AS TemValor, Protegido FROM Configuracao WHERE Chave LIKE 'Regras.%' ORDER BY Chave"),
    });
}

// Como o Program.cs: cada rotina, e o SaveChanges se ela mudou alguma coisa (a primeira salva sempre, por causa da marca).
async Task Partida(string prefixo)
{
    await Passo(prefixo + "regras padrão novas", async (db, cat, config) =>
    {
        var n = await cat.CriarRegrasPadraoAsync(config, CancellationToken.None);
        await db.SaveChangesAsync();
        return n;
    });
    await Passo(prefixo + "entradas nos cartões", async (db, cat, _) =>
    {
        var n = await cat.ReclassificarEntradasNosCartoesAsync(CancellationToken.None);
        if (n > 0) await db.SaveChangesAsync();
        return n;
    });
    await Passo(prefixo + "compras parceladas estornadas", async (db, cat, _) =>
    {
        var n = await cat.IgnorarComprasParceladasEstornadasAsync(CancellationToken.None);
        if (n > 0) await db.SaveChangesAsync();
        return n;
    });
}
await Partida("");
await Partida("segunda partida: ");
File.Delete(arquivo);

var opcoes = new JsonSerializerOptions { Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping };
var destino = Path.Combine(pasta, "acertos.json");
// Um passo por linha: o diff do gabarito mostra o passo que mudou.
File.WriteAllText(destino, "{\n  \"dotnet\": [" + JsonSerializer.Serialize(Environment.Version.ToString()) + "],\n  \"passos\": [\n    " +
    string.Join(",\n    ", passos.Select(p => JsonSerializer.Serialize(p, opcoes))) + "\n  ]\n}\n");
Console.WriteLine(destino);
