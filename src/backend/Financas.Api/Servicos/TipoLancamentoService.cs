using System.Globalization;
using System.Text;
using System.Text.RegularExpressions;
using Financas.Api.Entidades;

namespace Financas.Api.Servicos;

/// <summary>
/// #111 (ADR 0025): como o dinheiro entrou ou saiu — compra no cartão, Pix enviado, transferência entre suas contas,
/// pagamento de fatura… Calculado na hora (não gravado), nesta ordem:
/// 1. transferência lançada pelo próprio sistema entre contas suas;
/// 2. o tipo de operação que o banco informa pelo Open Finance (<c>Transacao.MeioPagamento</c>, o <c>operationType</c>
///    ou o <c>paymentData.paymentMethod</c> da Pluggy), com "mesma titularidade" para separar Pix para você de Pix
///    para outra pessoa;
/// 3. sem isso (transação antiga, manual ou banco que não informa), pela descrição, por palavras inteiras.
/// Quando nada indica, "outro": não inventa.
/// </summary>
public static partial class TipoLancamentoService
{
    /// <summary>Grupos do filtro "Lançamento" da tela de Transações (código na URL → tipos).</summary>
    public static readonly IReadOnlyDictionary<string, TipoLancamento[]> Grupos = new Dictionary<string, TipoLancamento[]>
    {
        ["cartao"] = [TipoLancamento.CompraCartao, TipoLancamento.EstornoCartao],
        ["debito"] = [TipoLancamento.CompraDebito],
        ["pix"] = [TipoLancamento.PixEnviado, TipoLancamento.PixRecebido],
        ["transferencia"] = [TipoLancamento.TransferenciaEnviada, TipoLancamento.TransferenciaRecebida],
        ["entre-contas"] = [TipoLancamento.EntreContas],
        ["fatura"] = [TipoLancamento.PagamentoFatura],
        ["boleto"] = [TipoLancamento.Boleto],
        ["saque-deposito"] = [TipoLancamento.Saque, TipoLancamento.Deposito],
        ["tarifas"] = [TipoLancamento.TarifaJuros],
        ["investimentos"] = [TipoLancamento.Rendimento, TipoLancamento.AplicacaoResgate],
        ["salario"] = [TipoLancamento.Salario],
        ["credito"] = [TipoLancamento.Credito],
        ["dinheiro"] = [TipoLancamento.Dinheiro],
        ["outros"] = [TipoLancamento.Outro]
    };

    public static TipoLancamento Classificar(decimal valor, string descricao, string? descricaoOriginal, string? meio,
        bool? mesmaTitularidade, bool transferenciaDoSistema, TipoConta tipoConta)
    {
        if (transferenciaDoSistema) return TipoLancamento.EntreContas;
        var saida = valor < 0;
        var texto = Normalizar(descricaoOriginal ?? descricao);
        var operacao = meio?.Trim().ToUpperInvariant();

        if (tipoConta == TipoConta.CartaoCredito) return NoCartao(saida, texto, operacao);
        if (tipoConta == TipoConta.Dinheiro) return TipoLancamento.Dinheiro;

        // Pagamento de fatura pela conta: o banco costuma mandar como boleto, convênio ou "outros"; a descrição diz.
        if (saida && EhPagamentoFatura(texto)) return TipoLancamento.PagamentoFatura;

        var informado = operacao switch
        {
            "PIX" => saida ? TipoLancamento.PixEnviado : TipoLancamento.PixRecebido,
            // #172: o banco pode mandar a operação "pagamento de fatura" também do lado da conta.
            "PAGAMENTO_FATURA" => saida ? TipoLancamento.PagamentoFatura : TipoLancamento.Outro,
            "TED" or "DOC" or "TEF" or "TRANSFERENCIA_MESMA_INSTITUICAO" =>
                saida ? TipoLancamento.TransferenciaEnviada : TipoLancamento.TransferenciaRecebida,
            "BOLETO" or "CONVENIO_ARRECADACAO" => TipoLancamento.Boleto,
            "CARTAO" => saida ? TipoLancamento.CompraDebito : TipoLancamento.Outro,
            "SAQUE" => TipoLancamento.Saque,
            "DEPOSITO" => TipoLancamento.Deposito,
            "PACOTE_TARIFA_SERVICOS" or "TARIFA_SERVICOS_AVULSOS" or "ENCARGOS_JUROS_CHEQUE_ESPECIAL" => TipoLancamento.TarifaJuros,
            "RENDIMENTO_APLIC_FINANCEIRA" => TipoLancamento.Rendimento,
            "RESGATE_APLIC_FINANCEIRA" => TipoLancamento.AplicacaoResgate,
            "FOLHA_PAGAMENTO" or "PORTABILIDADE_SALARIO" => TipoLancamento.Salario,
            "OPERACAO_CREDITO" => TipoLancamento.Credito,
            _ => (TipoLancamento?)null
        };
        if (informado is TipoLancamento t && t != TipoLancamento.Outro)
            return mesmaTitularidade == true && EhTransferencia(t) ? TipoLancamento.EntreContas : t;

        return PelaDescricao(saida, texto, mesmaTitularidade);
    }

    private static TipoLancamento NoCartao(bool saida, string texto, string? operacao)
    {
        switch (operacao)
        {
            case "PAGAMENTO_FATURA": return TipoLancamento.PagamentoFatura;
            case "ESTORNO" or "CASHBACK": return TipoLancamento.EstornoCartao;
            case "TARIFA": return TipoLancamento.TarifaJuros;
            case "OPERACOES_CREDITO_CONTRATADAS_CARTAO": return TipoLancamento.Credito;
            case "PAGAMENTO": return saida ? TipoLancamento.CompraCartao : TipoLancamento.EstornoCartao;
        }
        // No cartão, entrada é o pagamento da fatura ou um estorno.
        if (!saida) return Tem(texto, "PAGAMENTO", "PAGTO", "PGTO", "PAG") ? TipoLancamento.PagamentoFatura : TipoLancamento.EstornoCartao;
        if (Tem(texto, "ANUIDADE", "IOF", "JUROS", "ENCARGOS", "ENCARGO", "TARIFA", "MULTA")) return TipoLancamento.TarifaJuros;
        return TipoLancamento.CompraCartao;
    }

    private static TipoLancamento PelaDescricao(bool saida, string texto, bool? mesmaTitularidade)
    {
        if (Tem(texto, "PIX"))
            return mesmaTitularidade == true ? TipoLancamento.EntreContas : saida ? TipoLancamento.PixEnviado : TipoLancamento.PixRecebido;
        if (Tem(texto, "TED", "DOC", "TEF", "TRANSFERENCIA", "TRANSF"))
            return mesmaTitularidade == true ? TipoLancamento.EntreContas
                : saida ? TipoLancamento.TransferenciaEnviada : TipoLancamento.TransferenciaRecebida;
        if (Tem(texto, "BOLETO", "TITULO", "CONVENIO", "DEBITO AUTOMATICO", "DEB AUTOMATICO", "DEB AUT")) return TipoLancamento.Boleto;
        if (Tem(texto, "SAQUE")) return TipoLancamento.Saque;
        if (Tem(texto, "DEPOSITO")) return TipoLancamento.Deposito;
        if (Tem(texto, "TARIFA", "JUROS", "IOF", "ENCARGOS", "CESTA", "ANUIDADE")) return TipoLancamento.TarifaJuros;
        if (Tem(texto, "RENDIMENTO", "RENDIMENTOS", "REMUNERACAO")) return TipoLancamento.Rendimento;
        if (Tem(texto, "RESGATE", "APLICACAO", "APLIC")) return TipoLancamento.AplicacaoResgate;
        if (Tem(texto, "SALARIO", "FOLHA")) return TipoLancamento.Salario;
        if (saida && Tem(texto, "COMPRA", "DEBITO", "COMPRA DEBITO")) return TipoLancamento.CompraDebito;
        return TipoLancamento.Outro;
    }

    private static bool EhTransferencia(TipoLancamento t) => t is TipoLancamento.PixEnviado or TipoLancamento.PixRecebido
        or TipoLancamento.TransferenciaEnviada or TipoLancamento.TransferenciaRecebida;

    private static bool EhPagamentoFatura(string texto)
        => (Tem(texto, "FATURA") && Tem(texto, "PAGAMENTO", "PAGTO", "PGTO", "PAG", "CARTAO"))
           || Tem(texto, "PAGAMENTO CARTAO", "PAGTO CARTAO", "PGTO CARTAO", "PAG CARTAO");

    /// <summary>Alguma das palavras (ou expressões) aparece inteira no texto já normalizado.</summary>
    private static bool Tem(string texto, params string[] termos)
        => termos.Any(t => (" " + texto + " ").Contains(" " + t + " ", StringComparison.Ordinal));

    /// <summary>Maiúsculas, sem acento, só letras e dígitos separados por um espaço.</summary>
    private static string Normalizar(string texto)
    {
        var sb = new StringBuilder(texto.Length);
        foreach (var c in texto.Normalize(NormalizationForm.FormD))
        {
            if (CharUnicodeInfo.GetUnicodeCategory(c) == UnicodeCategory.NonSpacingMark) continue;
            sb.Append(char.IsLetterOrDigit(c) ? char.ToUpperInvariant(c) : ' ');
        }
        return Espacos().Replace(sb.ToString(), " ").Trim();
    }

    [GeneratedRegex(@"\s+")]
    private static partial Regex Espacos();
}
