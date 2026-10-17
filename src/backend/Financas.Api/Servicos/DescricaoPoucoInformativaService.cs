using System.Globalization;
using System.Text;
using Financas.Api.Entidades;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// #9: transações com descrição que não diz o que foi ("PIX ENVIADO", "PAGAMENTO", "TED 123456", só o nome do
/// banco). Sem palavras úteis, nem as regras nem as sugestões pelo histórico conseguem categorizar, e o gasto fica
/// escondido nos relatórios. A conta: separa as palavras por espaço e pontuação, descarta as que são só números e os
/// <see cref="TermosGenericos"/>, e soma as letras das que sobram ("99APP" é uma palavra com 3 letras, não "APP");
/// menos letras que o mínimo da Parametrização (<c>Transacoes.MinimoLetrasDescricao</c>) é pouco informativa.
/// Entram só as que ainda pedem atenção: sem categoria escolhida à mão, não marcadas como revisadas e fora das
/// transferências entre contas lançadas pelo próprio sistema.
/// </summary>
public class DescricaoPoucoInformativaService
{
    /// <summary>Palavras que, sozinhas, não dizem o que foi: tipo de operação, nome de banco, conectivos.</summary>
    public static readonly IReadOnlySet<string> TermosGenericos = new HashSet<string>(StringComparer.Ordinal)
    {
        // Tipo de operação
        "PIX", "ENVIADO", "ENVIADA", "RECEBIDO", "RECEBIDA", "TRANSFERENCIA", "TRANSFERENCIAS", "TRANSF", "TRANSFER",
        "TED", "DOC", "TEF", "PAGAMENTO", "PAGAMENTOS", "PAGTO", "PGTO", "PAG", "PAGO", "PAGAR", "RECEBER", "COMPRA",
        "COMPRAS", "DEBITO", "DEB", "CREDITO", "CRED", "CARTAO", "SAQUE", "DEPOSITO", "BOLETO", "BOLETOS", "OPERACAO",
        "LANCAMENTO", "MOVIMENTACAO", "TRANSACAO", "TRANSACOES", "ESTORNO", "AGENDADO", "AGENDADA", "AGENDAMENTO",
        "REALIZADO", "REALIZADA", "EFETUADO", "EFETUADA", "ENVIO", "RECEBIMENTO", "AUTORIZADO", "AUTORIZADA", "AUT",
        "AUTOMATICO", "AUTOMATICA", "CONTA", "CONTAS", "CORRENTE", "ENTRE", "MESMA", "TITULARIDADE", "FAVORECIDO",
        "QR", "CODE", "QRCODE", "CHAVE", "APP", "INTERNET", "BANKING", "MOBILE", "TITULO", "VALOR", "ELETRONICA",
        "ELETRONICO", "OUTROS", "OUTRAS", "DIVERSOS", "DIVERSAS",
        // Bancos e carteiras
        "BANCO", "NUBANK", "NU", "ITAU", "UNIBANCO", "BRADESCO", "SANTANDER", "CAIXA", "ECONOMICA", "FEDERAL", "INTER",
        "BRASIL", "BB", "SICOOB", "SICREDI", "ORIGINAL", "NEON", "PAN", "BTG", "PACTUAL", "XP", "PICPAY", "PAGBANK",
        "PAGSEGURO", "MERCADOPAGO", "SAFRA", "BMG", "AGIBANK", "BANRISUL", "NEXT", "WILL", "C6", "BANK", "SA",
        // Conectivos
        "DE", "DA", "DO", "DAS", "DOS", "PARA", "PRA", "EM", "NO", "NA", "NOS", "NAS", "A", "O", "AS", "OS", "E", "AO",
        "VIA", "POR", "COM", "SEM"
    };

    private readonly ParametroService _parametros;

    public DescricaoPoucoInformativaService(ParametroService parametros) => _parametros = parametros;

    /// <summary>Letras das palavras que sobram depois de tirar acentos, pontuação, números soltos e termos genéricos.</summary>
    public static int LetrasUteis(string? descricao)
    {
        if (string.IsNullOrWhiteSpace(descricao)) return 0;
        var semAcento = new StringBuilder(descricao.Length);
        foreach (var c in descricao.Normalize(NormalizationForm.FormD))
        {
            if (CharUnicodeInfo.GetUnicodeCategory(c) == UnicodeCategory.NonSpacingMark) continue;
            semAcento.Append(char.IsLetterOrDigit(c) ? char.ToUpperInvariant(c) : ' ');
        }
        return semAcento.ToString()
            .Split(' ', StringSplitOptions.RemoveEmptyEntries)
            .Where(palavra => !TermosGenericos.Contains(palavra))
            .Sum(palavra => palavra.Count(char.IsLetter));
    }

    public static bool EhPoucoInformativa(string? descricao, int minimoLetras) => LetrasUteis(descricao) < minimoLetras;

    /// <summary>As que ainda pedem atenção (as outras nem são olhadas).</summary>
    public static IQueryable<Transacao> Candidatas(IQueryable<Transacao> q)
        => q.Where(t => !t.DescricaoRevisada && !t.CategoriaManual && t.TransferenciaGrupo == null);

    /// <summary>
    /// Ids e descrições das pouco informativas dentro da consulta. A conta é feita na memória (não dá para traduzir
    /// para SQL): a consulta já deve vir restrita (período, conta…).
    /// </summary>
    public async Task<List<(long Id, string Descricao)>> ListarAsync(IQueryable<Transacao> q, CancellationToken ct)
    {
        var minimo = await _parametros.ObterInteiroAsync(ConfiguracaoService.MinimoLetrasDescricao, ct);
        var candidatas = await Candidatas(q).Select(t => new { t.Id, t.Descricao }).ToListAsync(ct);
        return candidatas.Where(c => EhPoucoInformativa(c.Descricao, minimo)).Select(c => (c.Id, c.Descricao)).ToList();
    }
}
