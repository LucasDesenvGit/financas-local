// Sonda do C# (#81). Grava em ../csharp.json o que o .NET da API faz, para os testes do Go compararem:
//   - divisão (a escala do quociente: a "natural" ou a mínima exata; até 96 bits quando não termina);
//   - multiplicação (a escala é a soma das escalas);
//   - Math.Round(x, casas), que arredonda pelo método bancário e só mexe quando há mais casas que o pedido;
//   - DateTime lido como o ConfiguracaoService.ObterDataAsync (RoundtripKind) e escrito no JSON e no ToString("O").
// É o gabarito do aritmetica_test.go e do instante_test.go. Rode de novo quando o .NET mudar de versão:
//   dotnet run --project src/backend-go/internal/tipos/testdata/sonda-csharp
using System.Globalization;
using System.Text.Json;

var inv = CultureInfo.InvariantCulture;
decimal D(string texto) => decimal.Parse(texto, NumberStyles.Number | NumberStyles.AllowExponent, inv);
string T(decimal d) => d.ToString(inv);

var divisoes = new (string A, string B)[]
{
    ("10", "100"), ("1.00", "1"), ("1", "1.00"), ("10", "4"), ("1", "3"), ("2", "3"), ("-1", "3"), ("-2", "3"),
    ("22", "7"), ("79", "7"), ("6554.45", "612345.67"), ("100.00", "4"), ("0.5", "2"), ("5", "0.5"),
    ("123.4500", "1.5"), ("1", "8"), ("1", "1024"), ("1.10", "1.1"), ("0", "5.00"), ("0.00", "5"),
    // Empates na última casa possível (28): o arredondamento interno da divisão.
    ("1", "20000000000000000000000000000"), ("5", "20000000000000000000000000000"),
    ("3", "20000000000000000000000000000"), ("-5", "20000000000000000000000000000"),
    // Variação de preço como a do CriptoService: (novo - antigo) / antigo.
    ("-6554.45", "618900.12"), ("0.01", "612345.67"), ("100", "99.99"),
    // Inexata que arredonda com zeros à direita: o C# tira os zeros (até a escala natural)?
    ("0.369", "3.0000000000000000000000000001"), ("1.00", "0.9999999999999999999999999999"),
    ("12.30", "1.0000000000000000000000000001"),
};
var produtos = new (string A, string B)[]
{
    ("2.5", "1"), ("10.0", "5.4123"), ("0.0107038398752782884869586814", "100"), ("1.5", "1.5"),
    ("-3.333", "2.00"), ("123.45", "0.055131"),
};
var arredondamentos = new (string X, int Casas)[]
{
    ("2.5", 2), ("2.555", 2), ("2.565", 2), ("-2.565", 2), ("0.125", 2), ("0.135", 2), ("250.0", 2), ("8230.000", 2),
    ("0.0976562500", 2), ("33.333333333333333333333333330", 2), ("1.005", 2), ("-0.005", 2), ("0.015", 2),
    ("54.12300", 2), ("7", 2), ("0.00", 2), ("12.3456789", 4), ("2.5", 0), ("3.5", 0),
};

// Só textos com "Z" ou sem fuso: com deslocamento, o .NET converte para o fuso do computador, que muda de um para outro.
var instantes = new[]
{
    "2026-10-15T00:30:00.0000000Z", "2026-10-15T01:02:03.1234567Z", "2026-10-15T01:02:03.1200000Z",
    "2026-10-15T01:02:03.0000001Z", "2026-10-15T01:02:03", "2026-10-15T01:02:03.5",
};
DateTime Ler(string texto) => DateTime.Parse(texto, null, DateTimeStyles.RoundtripKind);

var resultado = new
{
    instantes = instantes.Select(t => new { texto = t, json = JsonSerializer.Serialize(Ler(t)), redondo = Ler(t).ToString("O"),
        tipo = Ler(t).Kind.ToString() }).ToList(),
    agoraEmJson = JsonSerializer.Serialize(DateTime.SpecifyKind(new DateTime(2026, 10, 14, 21, 0, 0), DateTimeKind.Utc)),
    divisoes = divisoes.Select(c => new { a = c.A, b = c.B, resultado = T(D(c.A) / D(c.B)) }).ToList(),
    produtos = produtos.Select(c => new { a = c.A, b = c.B, resultado = T(D(c.A) * D(c.B)) }).ToList(),
    arredondamentos = arredondamentos.Select(c => new { x = c.X, casas = c.Casas, resultado = T(Math.Round(D(c.X), c.Casas)) }).ToList(),
};

var destino = Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "csharp.json");
File.WriteAllText(Path.GetFullPath(destino),
    JsonSerializer.Serialize(resultado, new JsonSerializerOptions { WriteIndented = true }) + "\n");
Console.WriteLine($"gravado: {Path.GetFullPath(destino)}");
