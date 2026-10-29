// Gabaritos das contas (#77), chamados no código da própria API: o ciclo da fatura (CicloDaFatura), o mês de cobrança
// de cada lançamento (MesesDeCobranca), o pago (PagoNasTransacoes, Pago) e a situação (SituacaoFatura), do ContaService;
// a detecção do banco (BancoService); a "compra" de uma parcela (ParcelasFuturas.Compra); e a ordem de texto da cultura
// pt-BR, a do ThenBy(f => f.Cartao) da fatura do mês.
using System.Collections;
using System.Globalization;
using System.Reflection;
using System.Text.Encodings.Web;
using System.Text.Json;
using Financas.Api.Servicos;

const BindingFlags Todos = BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Static;
var conta = typeof(ContaService);
var tipoFatura = conta.GetNestedType("FaturaDoCiclo", BindingFlags.NonPublic)!;
var tipoLancamento = conta.GetNestedType("LancamentoFatura", BindingFlags.NonPublic)!;
var tipoPagamento = conta.GetNestedType("PagamentoNoCartao", BindingFlags.NonPublic)!;
var cicloDaFatura = conta.GetMethod("CicloDaFatura", Todos)!;
var mesesDeCobranca = conta.GetMethod("MesesDeCobranca", Todos)!;
var pagoNasTransacoes = conta.GetMethod("PagoNasTransacoes", Todos)!;
var pago = conta.GetMethod("Pago", Todos)!;
var normalizarBanco = typeof(BancoService).GetMethod("Normalizar", Todos)!;
var compra = typeof(ParcelasFuturas).GetMethod("Compra", Todos)!;

static DateOnly D(string s) => DateOnly.ParseExact(s, "yyyy-MM-dd", CultureInfo.InvariantCulture);
static string? Txt(DateOnly? d) => d?.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture);
static object? Prop(object o, string nome) => o.GetType().GetProperty(nome)!.GetValue(o);

IList Lista(Type tipo, IEnumerable<object> itens)
{
    var lista = (IList)Activator.CreateInstance(typeof(List<>).MakeGenericType(tipo))!;
    foreach (var i in itens) lista.Add(i);
    return lista;
}

object Fatura(int id, string? fechamento, string vencimento, decimal total = 100m, decimal? pagoInformado = null)
    => Activator.CreateInstance(tipoFatura, id, 1, fechamento is null ? (DateOnly?)null : D(fechamento), D(vencimento), total,
        pagoInformado)!;

// Conjuntos de faturas importadas, como o banco manda.
var conjuntos = new Dictionary<string, (string? fechamento, string vencimento)[]>
{
    ["nenhuma"] = [],
    // Fecha no dia 3 e vence no 10; o banco empurra o fechamento quando cai no fim de semana.
    ["mensais"] = [("2025-12-03", "2025-12-10"), ("2026-01-05", "2026-01-12"), ("2026-02-03", "2026-02-10"),
        ("2026-03-03", "2026-03-10"), ("2026-04-06", "2026-04-13"), ("2026-05-04", "2026-05-11"), ("2026-06-03", "2026-06-10"),
        ("2026-07-03", "2026-07-10"), ("2026-08-03", "2026-08-10"), ("2026-09-03", "2026-09-10"), ("2026-10-05", "2026-10-13"),
        ("2026-11-03", "2026-11-10")],
    // Sem data de fechamento: só o vencimento, no dia 20.
    ["sem fechamento"] = [(null, "2026-01-20"), (null, "2026-02-20"), (null, "2026-03-20"), (null, "2026-04-20"),
        (null, "2026-05-20"), (null, "2026-06-20"), (null, "2026-07-20"), (null, "2026-08-20"), (null, "2026-09-20"),
        (null, "2026-10-20"), (null, "2026-11-20"), (null, "2026-12-20")],
    // Fechamento no fim do mês, duas no mesmo mês (troca de dia) e uma que pula um mês.
    ["irregulares"] = [("2026-01-31", "2026-02-07"), ("2026-03-02", "2026-03-09"), ("2026-03-30", "2026-04-07"),
        ("2026-06-28", "2026-07-05"), ("2026-07-28", "2026-08-05"), ("2026-12-31", "2027-01-08")],
    // Os limites, no dia exato e no seguinte: o fechamento anterior até 45 dias antes (05/04 → 20/05 são 45), a fatura
    // sem fechamento que vence até 25 dias depois do fechamento (10/06 → 05/07 são 25) e, nos lançamentos, a fatura
    // importada até 40 dias depois (10/04 → 20/05 são 40).
    ["limite 45"] = [("2026-04-05", "2026-04-12"), ("2026-05-20", "2026-05-27")],
    ["limite 46"] = [("2026-04-04", "2026-04-11"), ("2026-05-20", "2026-05-27")],
    ["vencimento 25"] = [(null, "2026-07-05")],
    ["vencimento 26"] = [(null, "2026-07-06")],
};
static bool SoNosLimites(string conjunto) => conjunto.StartsWith("limite") || conjunto.StartsWith("vencimento");

var meses = Enumerable.Range(1, 12).Select(m => (2026, m)).Concat([(2027, 1), (2027, 2), (2028, 2), (2025, 12)]).ToList();
byte?[] fechamentos = [null, 1, 3, 28, 30, 31];
byte?[] fechamentosDosLimites = [null, 10, 20];
byte?[] vencimentos = [null, 5, 10, 31];

var ciclos = new List<object>();
var entradasDosCiclos = new List<object>();
foreach (var (nomeConjunto, itens) in conjuntos)
{
    var faturas = Lista(tipoFatura, itens.Select((f, i) => Fatura(i + 1, f.fechamento, f.vencimento)));
    entradasDosCiclos.Add(new { nome = nomeConjunto, faturas });
    foreach (var (ano, mes) in meses)
    foreach (var diaFechamento in SoNosLimites(nomeConjunto) ? fechamentosDosLimites : fechamentos)
    foreach (var diaVencimento in vencimentos)
    {
        // Os conjuntos dos limites só nos meses e dias que os alcançam, para o gabarito não crescer à toa.
        if (SoNosLimites(nomeConjunto) && (ano != 2026 || mes is not (5 or 6) || diaVencimento is not (null or 5)))
            continue;
        var inicio = new DateOnly(ano, mes, 1);
        var fim = new DateOnly(ano, mes, DateTime.DaysInMonth(ano, mes));
        var c = cicloDaFatura.Invoke(null, [inicio, fim, diaFechamento, diaVencimento, faturas])!;
        var fatura = Prop(c, "Fatura");
        ciclos.Add(new
        {
            faturas = nomeConjunto, ano, mes, diaFechamento, diaVencimento,
            inicio = Txt((DateOnly)Prop(c, "Inicio")!), fim = Txt((DateOnly)Prop(c, "Fim")!),
            fechamento = Txt((DateOnly?)Prop(c, "Fechamento")), vencimento = Txt((DateOnly?)Prop(c, "Vencimento")),
            fatura = fatura is null ? (int?)null : (int)Prop(fatura, "Id")!,
        });
    }
}

// Lançamentos de um cartão: compras dos dois lados do fechamento, ligadas ou não a uma fatura, e parcelas da mesma
// compra (com a âncora ligada pelo banco) com datas que o Open Finance manda do outro lado do fechamento.
object Lanc(long id, string data, decimal valor, int? faturaId, short? numero, short? total, string descricao)
    => Activator.CreateInstance(tipoLancamento, id, 1, D(data), valor, faturaId, numero, total, descricao)!;
var lancamentos = Lista(tipoLancamento, new[]
{
    Lanc(1, "2026-03-02", -10m, null, null, null, "PADARIA"),
    Lanc(2, "2026-03-03", -11m, null, null, null, "PADARIA"),
    Lanc(3, "2026-03-04", -12m, null, null, null, "PADARIA"),
    Lanc(4, "2026-03-04", -13m, 90, null, null, "LANCADO NA ANTERIOR"),
    Lanc(5, "2026-03-31", -14m, null, null, null, "FIM DO MES"),
    Lanc(6, "2026-02-28", -15m, null, null, null, "FEVEREIRO"),
    Lanc(10, "2026-01-15", -100.40m, 2, 1, 4, "LOJA EXEMPLO 01/04"),
    Lanc(11, "2026-02-15", -100.40m, null, 2, 4, "LOJA EXEMPLO 02/04"),
    Lanc(12, "2026-03-15", -100.40m, null, 3, 4, "LOJA EXEMPLO 03/04"),
    Lanc(13, "2026-04-01", -100.40m, null, 4, 4, "LOJA EXEMPLO 04/04"),
    Lanc(20, "2026-02-10", -50m, null, 1, 3, "PARC 1 DE 3 OUTRA LOJA"),
    Lanc(21, "2026-03-10", -50m, 3, 2, 3, "PARC 2 DE 3 OUTRA LOJA"),
    Lanc(22, "2026-04-02", -50m, null, 3, 3, "PARC 3 DE 3 OUTRA LOJA"),
    Lanc(23, "2026-04-02", -50.49m, null, 3, 3, "PARCELA 3/3 OUTRA LOJA"),
    Lanc(24, "2026-04-02", -51m, null, 3, 3, "PARCELA 3/3 OUTRA LOJA"),
    Lanc(30, "2026-05-20", 25m, null, null, null, "ESTORNO PADARIA"),
    Lanc(31, "2026-12-31", -16m, null, null, null, "VIRADA DO ANO"),
    Lanc(32, "2027-01-01", -17m, null, null, null, "ANO NOVO"),
    Lanc(40, "2026-06-01", -20m, 99, 1, 2, "LIGADA A FATURA QUE NAO EXISTE 1/2"),
    Lanc(41, "2026-07-01", -20m, null, 2, 2, "LIGADA A FATURA QUE NAO EXISTE 2/2"),
    Lanc(50, "2026-04-10", -30m, null, null, null, "40 DIAS ANTES DO FECHAMENTO"),
    Lanc(51, "2026-04-09", -31m, null, null, null, "41 DIAS ANTES DO FECHAMENTO"),
});
var cobrancas = new List<object>();
foreach (var (nomeConjunto, itens) in conjuntos)
{
    // Ids 2, 3 e 90 são os que os lançamentos citam; a 90 é a fatura de antes (fecha em 03/03).
    var faturas = Lista(tipoFatura, itens.Select((f, i) => Fatura(i + 1, f.fechamento, f.vencimento)).Append(
        Fatura(90, nomeConjunto == "sem fechamento" ? null : "2026-03-03", "2026-03-10")));
    foreach (var diaFechamento in SoNosLimites(nomeConjunto) ? fechamentosDosLimites : fechamentos)
    {
        var mapa = (IDictionary)mesesDeCobranca.Invoke(null, [lancamentos, faturas, diaFechamento])!;
        cobrancas.Add(new
        {
            conjunto = nomeConjunto, faturas, diaFechamento,
            meses = mapa.Keys.Cast<long>().Order().ToDictionary(k => k.ToString(CultureInfo.InvariantCulture), k => Txt((DateOnly)mapa[k]!)),
        });
    }
}

// Pagamentos lançados no cartão: dentro e fora da janela (depois do vencimento anterior e até este).
object Pag(string data, decimal valor) => Activator.CreateInstance(tipoPagamento, 1, D(data), valor)!;
var pagamentos = Lista(tipoPagamento, new[]
{
    Pag("2026-02-10", 50m), Pag("2026-02-11", 60.5m), Pag("2026-03-01", 70m), Pag("2026-03-10", 80m), Pag("2026-03-11", 90m),
    Pag("2026-04-25", 100.10m), Pag("2026-01-31", 5m),
});
var pagos = new List<object>();
var vencimentosImportados = new Dictionary<string, string[]>
{
    ["nenhum"] = [], ["mensais"] = ["2026-01-10", "2026-02-10", "2026-03-10", "2026-04-10"],
    ["longe"] = ["2026-01-10"], ["perto"] = ["2026-02-28", "2026-03-10"],
    // 24/01 → 10/03 são 45 dias: o vencimento anterior importado vale até aí.
    ["limite 45"] = ["2026-01-24"], ["limite 46"] = ["2026-01-23"],
};
foreach (var (nome, lista) in vencimentosImportados)
foreach (var venc in new[] { "2026-02-10", "2026-03-10", "2026-03-31", "2026-04-10", "2026-05-10" })
{
    var v = lista.Select(D).ToList();
    var valor = (decimal)pagoNasTransacoes.Invoke(null, [D(venc), v, pagamentos])!;
    pagos.Add(new { vencimentos = lista, vencimento = venc, pago = valor.ToString(CultureInfo.InvariantCulture) });
}
var pagosQueValem = new List<object>();
foreach (var informado in new decimal?[] { null, 0m, -100m, 50m, 100.00m })
foreach (var lancado in new[] { 0m, -5m, 30m, 100m, 100.0m })
{
    var p = (decimal?)pago.Invoke(null, [informado, lancado]);
    pagosQueValem.Add(new
    {
        informado = informado?.ToString(CultureInfo.InvariantCulture), lancado = lancado.ToString(CultureInfo.InvariantCulture),
        pago = p?.ToString(CultureInfo.InvariantCulture),
    });
}
var situacoes = new List<object>();
var hoje = D("2026-10-14");
foreach (var total in new[] { 0m, -1m, 100m, 100.00m })
foreach (var p in new decimal?[] { null, 99.99m, 99.98m, -100m, 100m, 0m })
foreach (var fechamento in new[] { null, "2026-10-13", "2026-10-14", "2026-10-15" })
foreach (var venc in new[] { "2026-10-13", "2026-10-14", "2026-10-20" })
    situacoes.Add(new
    {
        total = total.ToString(CultureInfo.InvariantCulture), pago = p?.ToString(CultureInfo.InvariantCulture), fechamento,
        vencimento = venc,
        situacao = ContaService.SituacaoFatura(total, p, fechamento is null ? null : D(fechamento), D(venc), hoje),
    });

// Bancos (ADR 0024): o primeiro do catálogo cujo termo aparece como palavra inteira, sem acento nem maiúscula.
string?[][] paresDeBanco =
[
    ["Nu Pagamentos S.A.", "Conta"], [null, "Nubank Ultravioleta"], ["Itaú Unibanco", "Conta"], ["Banco Inter", "x"],
    ["Interativa", "Conta"], ["BB", null], ["Caixa Econômica Federal", null], ["C6 BANK", null], ["XP Investimentos", null],
    ["Banco do Brasil", null], ["", "  "], [null, null], ["Mercado Pago", null], ["mercado-pago", null], ["PicPay", null],
    ["nubankø", null], ["Next", null], ["nextel", null], ["Banco Pan Americano", null], ["ÍTAU", null], ["Santander (Brasil)", null],
    ["Will Bank", null], ["willbank", null], ["NU FINANCEIRA", null], ["Itau\u0301", null], ["ITAÚ PERSONNALITÉ", null],
    ["bmg123", null], ["Sicoob/Sicredi", null], ["PAGSEGURO INTERNET", null], ["Banco Inter e Nubank", null], ["ourocard", null],
    ["İnter", null], ["ınter", null], ["Banco Fictício", "Conta Corrente Fictícia"], ["Banco Fictício", "Cartão Nubank Roxinho"],
    ["mercado pago", null], ["mercado\tpago", null], ["c6_bank", null], ["C6-Bank", null], ["BTG Pactual", null],
    ["😀nubank😀", null], ["nubank²", null], ["nubank٣", null], ["Ｎｕｂａｎｋ", null], ["ﬁnal nubank", null], ["wise.com", null],
    ["Revolut Ltd", null], ["PayPal *LOJA", null], ["Agibank", null], ["Neon Pagamentos", null], ["Banco Original", null],
    ["Bradescard", null], ["Banrisul", null], ["Safra", null], ["Nomad", null], ["Sicredi", null], ["Itaucard", null],
    ["uniclass", null], ["CEF", null], ["Caixa", null], ["unibanco", null], ["mercado credito", null], ["pagbank", null],
    ["𝐧𝐮𝐛𝐚𝐧𝐤", null], ["nubank\u0301", null], ["nu\u200dbank", null], ["nu\u200bbank", null], ["NUBANK\u0903", null],
    ["inter\u0300 x", null], ["Ǆ bb", null], ["  BB  ", null], ["bb²", null], ["Ⅻ next", null], ["ẞ santander", null],
];
var bancos = paresDeBanco.Select(p => new
{
    instituicao = p[0], nome = p[1], banco = BancoService.Detectar(p[0], p[1]),
    normalizado = p[0] is null ? null : (string)normalizarBanco.Invoke(null, [p[0]])!,
}).ToList();

// A "compra" de uma parcela: as 8 primeiras letras, sem o número da parcela.
string[] descricoes =
[
    "LOJA EXEMPLO 03/10", "PARC 3 DE 10 LOJA", "PARCELA 2/12 ÁGUA MINERAL", "Compra 1 de 3 em loja", "MERCADOLIVRE*VENDEDOR 02/06",
    "٣/١٠ loja", "loja cinco 1 / 2", "1234/5678 LOJA", "PARC. LOJA", "PARCELADO LOJA", "parc loja 01de03", "", "12345",
    "Ação 1/2", "straße 1/2", "ﬁnal 1/2", "ı İ 1/2", "LOJA😀ABC 1/3", "日本語 1/2", "LOJAABCDEFGH", "AB 1 DE 2 CD 3/4",
    "PARC1/2LOJA", "LOJA 01/10 PARC 02", "x  de  y", "1de2", "PARCELA", "PARCELAS LOJA", "Ǆ ǅ ǆ loja", "ŉ ǰ 1/2", "Ⅻ 1/2",
    "𝐀𝐁𝐂 LOJA 1/2", "PARC\u0301 LOJA", "PARC\u0903 LOJA", "PARC_LOJA", "PARC\u200d LOJA", "PARC\u200c LOJA", "X PARC", "ÁPARC",
    "PARCELA.LOJA", "PARC..LOJA", "١٢٣٤/٥ LOJA", "1\u2003/\u20032 LOJA", "1\u200b/2 LOJA", "1\u0085/2 LOJA", "1 DE 2 LOJA",
    "1\u3000/2 LOJA", "1\u000b/2 LOJA", "PARC𝐀 LOJA", "𝐀PARC LOJA", "PARC² LOJA", "PARCⅫ LOJA", "1 De 2 LOJA", "1 de 2 loja",
    "ﬀ ﬁ ﬂ loja", "µ loja", "ÿ loja", "ß ẞ loja", "ǰ loja",
    "1\u200bDE2 LOJA", "1\u0085DE2 LOJA", "1\u000bDE2 LOJA", "1\u2003DE\u20032 LOJA", "1\u3000DE2 LOJA", "1\u001cDE2 LOJA", "1\u180eDE2 LOJA", "1\u2028DE2 LOJA", "1\u000cDE2 LOJA", "1 DE 2345 LOJA", "12DE345 LOJA", "PARCELADE 1 LOJA", "1DE PARC 2 LOJA", "1/2DE3 LOJA", "1DE2345DE6 LOJA",
];
var compras = descricoes.Select(d => new { descricao = d, compra = (string)compra.Invoke(null, [d])! }).ToList();

// A ordem de texto da cultura pt-BR (o Comparer<string>.Default do ThenBy, com CurrentCulture pt-BR).
string[] nomes =
[
    "Bolsa", "árvore", "Árvore", "arvore", "Cartão A", "cartão a", "Cartao A", "Cartão-B", "Cartão B", "Cartão  B", "cartão b",
    "Cartão 10", "Cartão 2", "Zeta", "zeta", "Éclair", "eclair", "Ébano", "Nubank", "nubank", "NUBANK", "Nu bank", "Ação", "Acao",
    "ação", "Inter", "inter", "C6", "c6 Bank", "_Teste", "#1", "1 conta", "Ünico", "Uber", "über", "Ångström", "Zulu", "cartao",
    "Cartão Fictício Platinum", "Cartão Nubank Roxinho", "co-op", "coop", "co op", "Œuvre", "oeuvre", "Ærø", "aero", "ß", "ss",
    "Çedilha", "cedilha", "Ñandú", "nandu", "a", "A", "á", "Á", "ab", "aB", "Ab", "AB",
];
var ptBr = CultureInfo.GetCultureInfo("pt-BR");
CultureInfo.CurrentCulture = ptBr;
var ordenados = nomes.OrderBy(n => 0).ThenBy(n => n).ToList();
var invariante = nomes.Order(StringComparer.Create(CultureInfo.InvariantCulture, false)).ToList();
if (!ordenados.SequenceEqual(invariante)) throw new InvalidOperationException("pt-BR e a cultura invariante ordenam diferente.");

// A query como o model binding do ASP.NET lê: o TypeConverter do tipo, na cultura invariante (SimpleTypeModelBinder).
string[] valoresDaQuery =
[
    "2026", " 2026 ", "+12", "-1", "0", "007", "0x7EA", "0X7ea", "#7EA", "&h7EA", "1e3", "1.0", "1,000", "2026x", "", " ",
    "2147483647", "2147483648", "-2147483648", "-2147483649", "١٢", "0xFFFFFFFF", "0x80000000", "0x7FFFFFFF", "0x100000000", "0x-1", "0x", "# 7EA", " 0x7EA ", "true", "True", " TRUE ", "false", "1", "sim", "yes", "x",
];
static object? Converter(Type tipo, string valor)
{
    try { return System.ComponentModel.TypeDescriptor.GetConverter(tipo).ConvertFrom(null, CultureInfo.InvariantCulture, valor); }
    catch (Exception) { return "erro"; }
}
string[] longos = ["9223372036854775807", "9223372036854775808", "-9223372036854775808", "0xFFFFFFFFFFFFFFFF",
    "0x8000000000000000", "0x7FFFFFFFFFFFFFFF", "0x10000000000000000", "2147483648", " 12 ", "+5", "1.0", "", "x"];
var query = valoresDaQuery.Concat(longos).Select(v => new { valor = v, inteiro = Converter(typeof(int), v)?.ToString(),
    longo = Converter(typeof(long), v)?.ToString(), booleano = Converter(typeof(bool), v)?.ToString() }).ToList();

// O saldo das contas pelo próprio EF Core (CalcularSaldosAsync), num SQLite temporário com o esquema de verdade: o Sum
// do decimal volta do banco no formato dele, e a escala do saldo depende disso.
var arquivoBanco = Path.Combine(Path.GetTempPath(), $"sonda-saldos-{Guid.NewGuid():N}.db");
var raiz = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "..", "..", "..", "..", ".."));
List<object> saldos;
using (var conexao = new Microsoft.Data.Sqlite.SqliteConnection($"Data Source={arquivoBanco};Pooling=False"))
{
    conexao.Open();
    using (var comando = conexao.CreateCommand())
    {
        comando.CommandText = File.ReadAllText(Path.Combine(raiz, "banco", "01-estrutura.sql")) + @"
INSERT INTO Conta (Id, Nome, Tipo, Origem, Moeda, SaldoInicial, Saldo, Cor, IncluirNoSaldo, Arquivada, CriadoEm, AtualizadoEm) VALUES
 (1, 'A', 1, 1, 'BRL', '0.0', '0.0', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (2, 'B', 1, 1, 'BRL', '100.0', '0.0', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (3, 'C', 1, 1, 'BRL', '5000.0', '0.0', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (4, 'D', 1, 1, 'BRL', '1.5', '0.0', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (5, 'E', 1, 2, 'BRL', '0.0', '123.40', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (6, 'F', 3, 1, 'BRL', '0', '0.0', '#000000', 1, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00');
INSERT INTO Transacao (ContaId, Data, Descricao, Valor, Tipo, Status, CategoriaManual, IgnorarRelatorios, Emergencia,
 DescricaoRevisada, DescricaoManual, CriadoEm, AtualizadoEm) VALUES
 (1, '2026-01-02', 'x', '-0.25', 1, 1, 0, 0, 0, 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (1, '2026-01-02', 'x', '-0.75', 1, 1, 0, 0, 0, 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (2, '2026-01-02', 'x', '10.50', 2, 1, 0, 0, 0, 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (2, '2026-01-02', 'x', '0.50', 2, 1, 0, 0, 0, 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (2, '2026-01-02', 'x', '-999.999', 1, 2, 0, 0, 0, 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (4, '2026-01-02', 'x', '-1.5', 1, 1, 0, 0, 0, 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (5, '2026-01-02', 'x', '-50.0', 1, 1, 0, 0, 0, 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (6, '2026-01-02', 'x', '-0.1', 1, 1, 0, 0, 0, 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (6, '2026-01-02', 'x', '-0.2', 1, 1, 0, 0, 0, 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
 (6, '2026-01-02', 'x', '0.30', 2, 1, 0, 0, 0, 0, 0, '2026-01-01 00:00:00', '2026-01-01 00:00:00');";
        comando.ExecuteNonQuery();
    }
}
var opcoesDb = new Microsoft.EntityFrameworkCore.DbContextOptionsBuilder<Financas.Api.Data.FinancasDbContext>();
Microsoft.EntityFrameworkCore.SqliteDbContextOptionsBuilderExtensions.UseSqlite(opcoesDb, $"Data Source={arquivoBanco};Pooling=False");
using (var db = new Financas.Api.Data.FinancasDbContext(opcoesDb.Options))
{
    var calculados = await new ContaService(db).CalcularSaldosAsync();
    saldos = calculados.OrderBy(kv => kv.Key).Select(kv => (object)new { conta = kv.Key, saldo = kv.Value.ToString(CultureInfo.InvariantCulture) }).ToList();
}
File.Delete(arquivoBanco);

// #9: as letras úteis da descrição (sem acento, pontuação, números soltos e termos genéricos).
string[] descricoesPoucoInformativas =
[
    "PIX ENVIADO", "PAGAMENTO", "TED 123456", "Banco Inter", "99APP", "UBER *TRIP", "Pix enviado para João", "", "   ", "A",
    "PIX - ENVIADO*12/10", "Lançamento da conta antiga", "SUPERMERCADO EXEMPLO", "PAGAMENTO FATURA CARTAO", "PIX RECEBIDO JOAO FICTICIO",
    "ab", "abc", "abcd", "Ação", "ﬁo", "straße", "ı", "𝐀𝐁𝐂𝐃", "日本語テキスト", "한국어", "Ǆ", "x1y2", "12 34", "COMPRA PENDENTE MERCADINHO",
    "TRANSFERENCIA PARA POUPANCA", "Transferência da conta corrente", "Feira", "nu bank", "NU PAGAMENTOS SA", "c6 bank", "C6BANK",
    "Pix enviado", "PAG*JOSE", "DEB AUT", "e-commerce", "café", "ÉÉÉ", "ĳ", "Ⅻ",
];
var letras = descricoesPoucoInformativas.Select(d => new { descricao = d, letras = DescricaoPoucoInformativaService.LetrasUteis(d) }).ToList();

// As sugestões de categoria (#78): o percentual "P0" na cultura pt-BR, o Math.Round do total de parecidas, as palavras
// e a chave de uma descrição e o texto sugerido para a regra de um grupo.
var ptBrSugestoes = CultureInfo.GetCultureInfo("pt-BR");
double[] razoes = [1, 0.5, 2.0 / 3, 1.0 / 3, 0.125, 0.375, 0.625, 0.875, 0.145, 0.285, 0.005, 0.995, 0.9995, 1.0 / 7, 2.0 / 7,
    0.25, 0.75, 0.0049, 0.015, 0.0, 3.0 / 11, 0.065, 0.135, 0.665, 0.335, 1.0 / 6, 5.0 / 6, 0.045, 0.055];
var percentuais = razoes.Select(r => new { razao = r.ToString("R", CultureInfo.InvariantCulture), texto = r.ToString("P0", ptBrSugestoes) }).ToList();
double[] totaisParecidas = [0.5, 1.5, 2.5, 3.5, 1, 2.4, 2.6, 7.000000001, 0.4, 4.5, 3];
var arredondados = totaisParecidas.Select(t => new { total = t.ToString("R", CultureInfo.InvariantCulture), inteiro = Math.Max(1, (int)Math.Round(t)) }).ToList();
string[] descricoesSugestao =
[
    "PIX ENVIADO JOAO DA SILVA", "UBER *TRIP 12/10", "IFOOD *RESTAURANTE SABOR", "COMPRA CARTAO MERCADO LIVRE 3/10", "",
    "Supermercado Exemplo LTDA", "PAG*JoseDaSilva", "99APP *99POP", "Netflix.com", "Açaí da Praça", "ab cd efg hij klm",
    "DROGA RAIA 1234 SAO PAULO BR", "PAGAMENTO FATURA CARTAO", "SALARIO EMPRESA FICTICIA LTDA", "Restaurante Ação", "straße ßß",
    // U+10000 é letra (Lo), mas fora do plano básico: no C#, as duas metades viram espaço.
    "ABC\U00010000DEF",
];
var palavras = descricoesSugestao.Select(d => new { descricao = d, palavras = SugestaoCategoriaService.Palavras(d), chave = SugestaoCategoriaService.Chave(d) }).ToList();
string[][] gruposRegra =
[
    ["UBER *TRIP 12/10", "UBER *TRIP 13/10", "UBER EATS"],
    ["UBER EATS 1", "UBER EATS 2", "UBER EATS 3"],
    ["PIX ENVIADO JOAO", "PIX ENVIADO MARIA"],
    ["PAGAMENTO CARTAO CREDITO BCE 1", "PAGAMENTO CARTAO CREDITO BCE 2"],
    ["NETFLIX.COM", "NETFLIX.COM", "NETFLIX SERVICOS"],
    ["BANCO DO BRASIL", "BANCO DO BRASIL SA"],
    ["IFD*RESTAURANTE A", "IFD*RESTAURANTE B", "IFD*LANCHONETE C", "IFD*PADARIA D", "IFD*MERCADO E"],
    ["TED 123", "TED 456"],
    ["Açaí da Praça", "ACAI DA PRACA"],
    ["", "  "],
    ["LOJA DE MOVEIS FICTICIA 1/10", "LOJA DE MOVEIS FICTICIA 2/10"],
    ["SUPERMERCADO EXEMPLO", "SUPERMERCADO EXEMPLO", "SUPERMERCADO OUTRO", "SUPERMERCADO EXEMPLO", "SUPERMERCADO EXEMPLO"],
    ["ABC", "ABD"],
    ["PAGAMENTO RECEBIDO", "PAGAMENTO RECEBIDO"],
    // As repetidas contam uma vez só: sem isso, "UBER" estaria em 4 das 5.
    ["UBER TRIP", "UBER TRIP", "UBER TRIP", "UBER TRIP", "CABIFY X"],
    // A palavra curta que é a última não tem com quem fazer dupla.
    ["UBER", "UBER *"],
    // O começo comum: com 10 letras exatas; parado num sinal (não corta); quando um texto acaba ali (não corta); no
    // meio de uma palavra de todos (corta na última palavra inteira).
    ["ABC DEF GH 1", "ABC DEF GH 2"],
    ["ABC DEF GHI*X", "ABC DEF GHI#Y"],
    ["ABC DEF GHIJ", "ABC DEF GHI"],
    ["ABC DEF GHI XYZ1", "ABC DEF GHI XYZ2"],
];
var textosRegra = gruposRegra.Select(g => new { descricoes = g, texto = SugestaoCategoriaService.TextoRegra(g) }).ToList();

// O texto sugerido para a regra de descrição (#123): sem os pedaços com número; com menos de 3 letras, a descrição
// inteira. Os dígitos e os espaços de Unicode contam (o \d e o \s do Regex do .NET).
string[] descricoesParaSugerir =
[
    "LOJA DE MOVEIS FICTICIA 1/10", "UBER *TRIP 12/10", "PIX ENVIADO 123456", "12/10 34", "AB 12 CD", " - IFOOD *RESTAURANTE - ",
    "PAG*JOSE 2026-10-01", "Açaí ٣٤ da Praça", "TED 123 FIM", "COMPRA  CARTAO\t\tMERCADO", "ABC\U0001D400 1",
    "", "   ", "12345", "A1 B2 C3", "XY 1 Z", "...ABC...", "VIVO*FIBRA", "NETFLIX.COM 12", "DROGA RAIA 1234 SAO PAULO BR",
    "AB\U0001D400\U0001D401 9", "  ab  ", "-*/.,", "ÁÉÍ 7", "x1y PAGAMENTO", "MERCADO　LIVRE 3",
];
var sugeridos = descricoesParaSugerir.Select(d => new { descricao = d, texto = RegraDescricaoService.SugerirTexto(d) }).ToList();

var opcoes = new JsonSerializerOptions { Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping };
string Gravar(string relativo, object conteudo)
{
    var destino = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", relativo));
    // Um caso por linha: o diff do gabarito mostra o caso que mudou.
    var linhas = conteudo is IDictionary<string, object> partes
        ? "{\n" + string.Join(",\n", partes.Select(p => "  " + JsonSerializer.Serialize(p.Key) + ": [\n    " +
            string.Join(",\n    ", ((IEnumerable)p.Value).Cast<object>().Select(o => JsonSerializer.Serialize(o, opcoes))) + "\n  ]")) + "\n}\n"
        : throw new ArgumentException("conteúdo");
    File.WriteAllText(destino, linhas);
    return destino;
}
Console.WriteLine(Gravar("faturas.json", new Dictionary<string, object>
{
    ["dotnet"] = new[] { Environment.Version.ToString() }, ["ciclos"] = ciclos, ["cobrancas"] = cobrancas, ["pagos"] = pagos,
    ["pagosQueValem"] = pagosQueValem, ["situacoes"] = situacoes, ["conjuntos"] = entradasDosCiclos,
    ["lancamentos"] = lancamentos, ["pagamentos"] = pagamentos,
}));
Console.WriteLine(Gravar(Path.Combine("..", "..", "bancos", "testdata", "bancos.json"), new Dictionary<string, object>
{
    ["dotnet"] = new[] { Environment.Version.ToString() }, ["bancos"] = bancos,
}));
Console.WriteLine(Gravar(Path.Combine("..", "..", "parcelas", "testdata", "compra.json"), new Dictionary<string, object>
{
    ["dotnet"] = new[] { Environment.Version.ToString() }, ["compras"] = compras,
}));
Console.WriteLine(Gravar(Path.Combine("..", "..", "contas", "testdata", "saldos.json"), new Dictionary<string, object>
{
    ["dotnet"] = new[] { Environment.Version.ToString() }, ["saldos"] = saldos,
}));
Console.WriteLine(Gravar(Path.Combine("..", "..", "sugestoes", "testdata", "sugestoes.json"), new Dictionary<string, object>
{
    ["dotnet"] = new[] { Environment.Version.ToString() }, ["percentuais"] = percentuais, ["arredondados"] = arredondados,
    ["palavras"] = palavras, ["textosRegra"] = textosRegra,
}));
Console.WriteLine(Gravar(Path.Combine("..", "..", "poucoinformativa", "testdata", "letras.json"), new Dictionary<string, object>
{
    ["dotnet"] = new[] { Environment.Version.ToString() }, ["letras"] = letras,
}));
Console.WriteLine(Gravar(Path.Combine("..", "..", "respostas", "testdata", "query.json"), new Dictionary<string, object>
{
    ["dotnet"] = new[] { Environment.Version.ToString() }, ["valores"] = query,
}));
Console.WriteLine(Gravar(Path.Combine("..", "..", "texto", "testdata", "ordem.json"), new Dictionary<string, object>
{
    ["dotnet"] = new[] { Environment.Version.ToString() }, ["nomes"] = nomes, ["ordenados"] = ordenados,
}));
Console.WriteLine(Gravar(Path.Combine("..", "..", "regrasdescricao", "testdata", "sugerir.json"), new Dictionary<string, object>
{
    ["dotnet"] = new[] { Environment.Version.ToString() }, ["sugeridos"] = sugeridos,
}));
