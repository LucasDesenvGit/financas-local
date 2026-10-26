// Sonda do System.Text.Json (#402): a mensagem e o caminho do erro de JSON cortado ou estragado, com as opções web do
// ASP.NET e um record com construtor, como os DTOs do projeto. Grava o gabarito que o teste do Go confere
// (internal/respostas/testdata/erros-json-dotnet.json).
//
// Rodar (Linux ou Windows, .NET 10): dotnet run --project src/backend-go/internal/respostas/testdata/sonda-json
using System;
using System.Collections.Generic;
using System.Text.Json;

var opcoes = new JsonSerializerOptions(JsonSerializerDefaults.Web);
string[] corpos = [
    // fim inesperado
    "{\"login\": \"ana", "{\"login\":", "{\"login\": ", "{\"login\": \"ana\"", "{\"login\": \"ana\",", "{\"login\": \"ana\", ",
    "{", "{ ", "{\"log", "{\"login\"", "{\"login\" ", "{\"a\":1", "{\"a\":tru", "\"abc", "{\"a\":\"x\\\"",
    "{\n  \"login\": \"ana\"", "{\"a\":1,   ", "{\"a\":{\"b\":1",
    // caractere inválido
    "{\"a\": x}", "{x}", "{\"a\" 1}", "{\"a\":1 \"b\":2}", "{\"a\":1,}", "{\"login\":\"ana\"\n  x}",
    // lixo depois do valor
    "{\"a\":1} x", "{\"a\":1}\n{", "{\"login\":\"ana\"}}", "{\"a\":1}  ,",
];
var gabarito = new List<object>();
foreach (var c in corpos)
{
    try
    {
        JsonSerializer.Deserialize<Credenciais>(c, opcoes);
        gabarito.Add(new { corpo = c, caminho = (string?)null, mensagem = "OK" });
    }
    catch (JsonException e)
    {
        gabarito.Add(new { corpo = c, caminho = e.Path, mensagem = e.Message });
    }
}
Console.WriteLine(JsonSerializer.Serialize(gabarito, new JsonSerializerOptions { WriteIndented = true }));

record Credenciais(string Login, string Senha);
