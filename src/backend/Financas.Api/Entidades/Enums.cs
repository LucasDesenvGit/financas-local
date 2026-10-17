namespace Financas.Api.Entidades;

public enum TipoCategoria : byte
{
    Despesa = 1,
    Receita = 2,
    Transferencia = 3
}

/// <summary>Natureza do gasto (regra 50/30/20).</summary>
public enum NaturezaGasto : byte
{
    Essencial = 1,
    EstiloDeVida = 2
}

public enum TipoAtivo : byte
{
    Fii = 1,
    Acao = 2,
    Etf = 3,
    Bdr = 4,
    Outro = 5
}

public enum TipoOperacao : byte
{
    Compra = 1,
    Venda = 2
}

public enum OrigemOperacao : byte
{
    Manual = 1,
    OpenFinance = 2,
    Extrato = 3
}

public enum TipoConta : byte
{
    ContaCorrente = 1,
    Poupanca = 2,
    CartaoCredito = 3,
    Investimento = 4,
    Dinheiro = 5,
    Outro = 6
}

/// <summary>Separa financiamentos (crédito vinculado a um bem: imóvel, veículo) dos demais empréstimos.</summary>
public enum NaturezaCredito : byte
{
    Emprestimo = 1,
    Financiamento = 2
}

public enum OrigemConta : byte
{
    Manual = 1,
    OpenFinance = 2
}

public enum TipoTransacao : byte
{
    Despesa = 1,
    Receita = 2,
    Transferencia = 3
}

public enum StatusTransacao : byte
{
    Efetivada = 1,
    Pendente = 2
}

public enum TipoCorrespondencia : byte
{
    Contem = 1,
    ComecaCom = 2,
    Igual = 3
}

public enum Recorrencia : byte
{
    Unica = 0,
    Mensal = 1,
    Anual = 2
}

/// <summary>Nível de um erro do sistema; os mesmos números do <c>LogLevel</c> do .NET.</summary>
public enum NivelErro : byte
{
    Aviso = 3,
    Erro = 4,
    Critico = 5
}
