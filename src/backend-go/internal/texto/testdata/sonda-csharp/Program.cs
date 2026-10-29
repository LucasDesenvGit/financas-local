// Gabarito da normalização de texto das regras (#77): o CategorizacaoService.Normalizar (regras de categoria e de
// descrição) e o Normalizar do TipoLancamentoService (palavras inteiras), chamados no código da própria API.
using System.Reflection;
using System.Text.Json;
using Financas.Api.Servicos;

var entradas = new[]
{
    "", "   ", "\t\n", "abc", "  Supermercado Exemplo  ", "Ação", "AÇÃO", "ação ", "Pão de Açúcar", "Itaú", "itaú",
    "Café", "Café", "naïve", "Ångström", "ÆØÅ æøå", "straße", "STRASSE", "ﬁnal", "Ǆ ǅ ǆ", "ı İ i I",
    "ΣΊΣΥΦΟΣ σίσυφος ς", "Ӂ ӂ", "Ñandú", "ÿ Ÿ", "µ", "ŉ", "ǰ", "ﬀ", "Ⅻ ⅻ", "①", "ａｂｃ ＡＢＣ", "x y",
    "x y", "a​b", "tab\tdentro", "linha\nnova", "PIX - ENVIADO*João", "TRANSF.123/45", "Méli Dólar",
    "MELI DOLAR", "😀 emoji", "𝐀𝐁𝐂", "a😀b", "日本語テキスト", "한국어", "עברית", "العربية", "ไทย", "é́",
    "́inicio", "Ⓐ", "ǈ", "ꞵ", "Ꞵ", "ỳ", "Ḁ", "ß ẞ", "ʼn", "Kelvin K", "Ω Ohm", "Å angstrom", "LOJA DE MOVEIS FICTICIA 9/10", "ſ longo", "ꭰ ᏸ",
    "ͅ iota", "ẛ", "ﬆ", "և", "ᾀ", "ᾳ", "ⱥ", "ɐ", "ɫ", "ᵹ", "ßé",
    "cafȩ́", "İstanbul", "İ",
};

var tipo = typeof(TipoLancamentoService);
var normalizarPalavras = tipo.GetMethod("Normalizar", BindingFlags.NonPublic | BindingFlags.Static)
    ?? throw new InvalidOperationException("TipoLancamentoService.Normalizar não encontrado.");

var casos = entradas.Select(e => new
{
    entrada = e,
    regras = CategorizacaoService.Normalizar(e),
    palavras = (string)normalizarPalavras.Invoke(null, new object[] { e })!,
}).ToList();

var opcoes = new JsonSerializerOptions { WriteIndented = true, Encoder = System.Text.Encodings.Web.JavaScriptEncoder.UnsafeRelaxedJsonEscaping };
var destino = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "normalizar.json"));
File.WriteAllText(destino, JsonSerializer.Serialize(new { dotnet = Environment.Version.ToString(), casos }, opcoes) + "\n");
Console.WriteLine($"{casos.Count} casos em {destino}");
