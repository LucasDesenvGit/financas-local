// Sonda descartável da #75: hashes gerados pelo PasswordHasher do ASP.NET Identity (o mesmo da API), e a conferência
// de hashes vindos de fora (args: "conferir" <hash> <senha>).
using Microsoft.AspNetCore.Identity;

var hasher = new PasswordHasher<object>();
if (args.Length == 3 && args[0] == "conferir")
{
    Console.WriteLine(hasher.VerifyHashedPassword(new object(), args[1], args[2]));
    return;
}
foreach (var senha in new[] { "Senha-Forte-123", "çãõ €moji 😀" })
    Console.WriteLine($"{senha}\t{hasher.HashPassword(new object(), senha)}");
// Formato v2 (antigo): o .NET ainda aceita, e pede para refazer.
var v2 = new PasswordHasher<object>(Microsoft.Extensions.Options.Options.Create(new PasswordHasherOptions { CompatibilityMode = PasswordHasherCompatibilityMode.IdentityV2 }));
Console.WriteLine($"v2:Senha-Forte-123\t{v2.HashPassword(new object(), "Senha-Forte-123")}");
// v3 com menos iterações que as de hoje (instalação antiga): o .NET aceita e pede para refazer.
var fraco = new PasswordHasher<object>(Microsoft.Extensions.Options.Options.Create(new PasswordHasherOptions { IterationCount = 10_000 }));
Console.WriteLine($"v3-10mil:Senha-Forte-123\t{fraco.HashPassword(new object(), "Senha-Forte-123")}");
