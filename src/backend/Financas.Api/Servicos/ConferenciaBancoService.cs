using System.Globalization;
using Financas.Api.Data;
using Financas.Api.Dtos;
using Financas.Api.Entidades;
using Financas.Api.Servicos.Cambio;
using Financas.Api.Servicos.Investimentos;
using Microsoft.EntityFrameworkCore;

namespace Financas.Api.Servicos;

/// <summary>
/// #172: "Conferir com o banco". Para cada conta, fatura do mês e investimento, o valor que o site usa e as pistas do
/// que pode fazer o app do banco mostrar outro número: quando o banco atualizou, reservas, pendentes, parcelas, estornos,
/// compras repetidas. Quem compara é a tela: o usuário digita o valor do app, e a pista com o mesmo valor da diferença
/// aparece primeiro. É a verificação feita à mão em 19/10/2026, dentro do produto. Os textos não levam valores em reais:
/// a tela mostra o <c>Valor</c> pelo pipe que respeita "ocultar valores".
/// </summary>
public class ConferenciaBancoService
{
    /// <summary>Até quantos dias para trás as compras repetidas no mesmo dia são procuradas nos cartões.</summary>
    public const int DiasComprasRepetidas = 45;

    private static readonly CultureInfo PtBr = CultureInfo.GetCultureInfo("pt-BR");

    private readonly FinancasDbContext _db;
    private readonly ContaService _contas;
    private readonly ReservaService _reservas;
    private readonly CarteiraService _carteira;
    private readonly CambioService _cambio;

    public ConferenciaBancoService(FinancasDbContext db, ContaService contas, ReservaService reservas,
        CarteiraService carteira, CambioService cambio)
    {
        _db = db;
        _contas = contas;
        _reservas = reservas;
        _carteira = carteira;
        _cambio = cambio;
    }

    public async Task<ConferenciaBancoDto> MontarAsync(CancellationToken ct)
    {
        var hoje = Relogio.Hoje;
        var saldos = await _contas.CalcularSaldosAsync(ct);
        var bancos = await _contas.BancosAsync(ct);
        var reservas = await _reservas.ListarAsync(ct);
        var contas = await _db.Contas.AsNoTracking().Where(c => !c.Arquivada)
            .OrderBy(c => c.Nome)
            .Select(c => new
            {
                c.Id, c.Nome, c.Tipo, c.Origem, c.AtualizadoEm,
                AtualizadoNoBanco = c.ConexaoBancaria != null ? c.ConexaoBancaria.UltimaAtualizacaoInstituicao : null
            })
            .ToListAsync(ct);
        var pendentes = (await _db.Transacoes.AsNoTracking()
                .Where(t => t.Status == StatusTransacao.Pendente)
                .Select(t => new { t.ContaId, t.Valor }).ToListAsync(ct))
            .GroupBy(t => t.ContaId)
            .ToDictionary(g => g.Key, g => (Quantidade: g.Count(), Total: g.Sum(t => t.Valor)));

        // Contas (sem os cartões).
        var listaContas = new List<ContaConferenciaDto>();
        foreach (var c in contas.Where(c => c.Tipo != TipoConta.CartaoCredito))
        {
            var pistas = new List<PistaConferenciaDto>();
            var openFinance = c.Origem == OrigemConta.OpenFinance;
            if (openFinance)
                pistas.Add(new PistaConferenciaDto(
                    $"O saldo vem do banco, atualizado em {Data(c.AtualizadoNoBanco ?? c.AtualizadoEm)}. O que entrou ou saiu depois ainda não chegou.",
                    null));
            else
                pistas.Add(new PistaConferenciaDto(
                    "Conta manual: o saldo é o inicial mais os lançamentos. Falta algum, ou há um a mais?", null,
                    "/transacoes", DaConta(c.Id)));
            var daConta = reservas.Where(r => r.ContaId == c.Id).Sum(r => r.Saldo);
            if (daConta > 0)
                pistas.Add(new PistaConferenciaDto(
                    "Reservas e caixinhas desta conta (o app pode mostrar o saldo com elas)",
                    daConta, "/investimentos", Parametro("aba", "reservas")));
            if (pendentes.TryGetValue(c.Id, out var pendentesDaConta))
                pistas.Add(new PistaConferenciaDto(
                    $"{Quantos(pendentesDaConta.Quantidade, "lançamento pendente", "lançamentos pendentes")} (o app pode já contar ou não)",
                    Math.Abs(pendentesDaConta.Total), "/transacoes", DaConta(c.Id)));
            listaContas.Add(new ContaConferenciaDto(c.Id, c.Nome, bancos.GetValueOrDefault(c.Id), openFinance,
                saldos.GetValueOrDefault(c.Id), pistas));
        }

        // Cartões: a fatura que fecha neste mês (a mesma do Painel e de Contas e cartões).
        var inicioMes = Relogio.PrimeiroDiaMes(hoje.Year, hoje.Month);
        var fimMes = Relogio.UltimoDiaMes(hoje.Year, hoje.Month);
        var faturas = (await _contas.FaturasDoMesAsync(inicioMes, fimMes, hoje, ct)).ToDictionary(f => f.ContaId);
        var desde = hoje.AddDays(-DiasComprasRepetidas);
        var movimentosCartao = await _db.Transacoes.AsNoTracking()
            .Where(t => t.Conta!.Tipo == TipoConta.CartaoCredito && t.Data >= desde && t.TransferenciaGrupo == null)
            .Select(t => new { t.ContaId, t.Data, t.Valor, t.Descricao, t.Tipo, t.ParcelaNumero })
            .ToListAsync(ct);
        var listaCartoes = new List<CartaoConferenciaDto>();
        foreach (var c in contas.Where(c => c.Tipo == TipoConta.CartaoCredito))
        {
            var pistas = new List<PistaConferenciaDto>();
            var fatura = faturas.GetValueOrDefault(c.Id);
            var valorFatura = fatura?.ValorMes ?? 0m;
            var divida = Math.Max(0m, -saldos.GetValueOrDefault(c.Id));
            if (fatura is { Situacao: "fechada" or "paga" })
                pistas.Add(new PistaConferenciaDto($"Fatura {fatura.Situacao}: vale o total que o banco informou.", null));
            else
                pistas.Add(new PistaConferenciaDto(
                    "Fatura aberta: é a soma das compras do ciclo até agora. O app pode já mostrar compras que ainda não chegaram.",
                    null));
            if (divida - valorFatura > 0.01m)
                pistas.Add(new PistaConferenciaDto(
                    "Dívida total do cartão, com as parcelas futuras, menos a fatura do mês (se o app mostra a dívida total)",
                    divida - valorFatura));
            if (pendentes.TryGetValue(c.Id, out var pendentesDoCartao))
                pistas.Add(new PistaConferenciaDto(
                    $"{Quantos(pendentesDoCartao.Quantidade, "compra pendente", "compras pendentes")} (o app pode já contar na fatura)",
                    Math.Abs(pendentesDoCartao.Total), "/transacoes", DaConta(c.Id)));
            if (fatura is { ParcelasMes: > 0 })
                pistas.Add(new PistaConferenciaDto(
                    "Parcelas nesta fatura (o app pode mostrá-las à parte)",
                    fatura.ParcelasMes));
            var doCartao = movimentosCartao.Where(m => m.ContaId == c.Id).ToList();
            var estornos = doCartao.Where(m => m.Valor > 0 && m.Tipo == TipoTransacao.Despesa && m.Data >= inicioMes && m.Data <= fimMes)
                .Sum(m => m.Valor);
            if (estornos > 0)
                pistas.Add(new PistaConferenciaDto(
                    "Estornos no mês (o site já desconta; o app pode mostrar a fatura antes deles)",
                    estornos, "/transacoes", DaConta(c.Id)));
            var repetidas = doCartao.Where(m => m.Valor < 0 && m.ParcelaNumero == null)
                .GroupBy(m => (m.Data, m.Valor, Descricao: m.Descricao.Trim().ToUpperInvariant()))
                .Where(g => g.Count() > 1)
                .ToList();
            if (repetidas.Count > 0)
            {
                var aMais = repetidas.Sum(g => -g.Key.Valor * (g.Count() - 1));
                pistas.Add(new PistaConferenciaDto(
                    $"{Quantos(repetidas.Count, "compra repetida", "compras repetidas")} no mesmo dia e com o mesmo valor: " +
                    "pode ser compra em dobro de verdade ou importada duas vezes (o que veio a mais)",
                    aMais, "/transacoes", DaConta(c.Id)));
            }
            listaCartoes.Add(new CartaoConferenciaDto(c.Id, c.Nome, bancos.GetValueOrDefault(c.Id), valorFatura,
                fatura?.Situacao, fatura?.Fechamento, fatura?.Vencimento, divida, pistas));
        }

        // Investimentos: o que o Open Finance informa, as reservas e a carteira lançada por você.
        var conversor = await _cambio.ObterConversorAsync(ct);
        var listaInvestimentos = new List<InvestimentoConferenciaDto>();
        var doOpenFinance = await _db.Investimentos.AsNoTracking().Where(i => i.ConexaoBancaria!.Ativa)
            .OrderByDescending(i => i.Saldo)
            .Select(i => new
            {
                i.Id, i.Nome, i.Tipo, i.Emissor, i.NomePersonalizado, i.Saldo, i.Moeda, i.AtualizadoEm,
                AtualizadoNoBanco = i.ConexaoBancaria!.UltimaAtualizacaoInstituicao
            })
            .ToListAsync(ct);
        foreach (var i in doOpenFinance)
        {
            var pistas = new List<PistaConferenciaDto>
            {
                new PistaConferenciaDto($"Valor do banco, atualizado em {Data(i.AtualizadoNoBanco ?? i.AtualizadoEm)}. O Meu Pluggy atualiza " +
                    "cerca de uma vez por dia: o rendimento de hoje pode não ter chegado.", null)
            };
            if (i.Moeda != CambioService.MoedaBase)
                pistas.Add(new PistaConferenciaDto($"Em {i.Moeda}, convertido pela PTAX; o app usa a cotação dele.", null));
            listaInvestimentos.Add(new InvestimentoConferenciaDto($"of-{i.Id}",
                NomeInvestimento.Exibicao(i.Nome, i.Tipo, i.Emissor, i.NomePersonalizado), "Open Finance",
                Math.Round(conversor.ParaReais(i.Saldo, i.Moeda) ?? 0m, 2), pistas));
        }
        foreach (var r in reservas)
        {
            var pistas = new List<PistaConferenciaDto>();
            if (r.InformadoPeloBanco)
                pistas.Add(new PistaConferenciaDto("Valor que o banco informa pelo Open Finance.", null));
            else if (r.DataConferencia is DateOnly conferida)
                pistas.Add(new PistaConferenciaDto(
                    $"Conferido em {conferida.ToString("dd/MM/yyyy", PtBr)} e, depois, estimado pelos lançamentos. O rendimento " +
                    "não aparece no extrato: confira de novo.", null, "/investimentos", Parametro("aba", "reservas")));
            else
                pistas.Add(new PistaConferenciaDto(
                    "Estimado pelo extrato, sem conferência: pode faltar o dinheiro de antes do primeiro lançamento e o rendimento.",
                    null, "/investimentos", Parametro("aba", "reservas")));
            if (r.Moeda != CambioService.MoedaBase && r.Cotacao is decimal cotacao)
                pistas.Add(new PistaConferenciaDto(
                    $"Em dólar, pela PTAX de {cotacao.ToString("N4", PtBr)} (a cotação do Banco Central); o app usa a cotação dele.",
                    null));
            listaInvestimentos.Add(new InvestimentoConferenciaDto($"reserva-{r.ContaId}-{r.Nome}", $"{r.Nome} ({r.NomeConta})",
                "Reserva", r.Saldo, pistas));
        }
        var composicao = await _carteira.ComposicaoAsync(ct);
        if (composicao.Carteira > 0)
            listaInvestimentos.Add(new InvestimentoConferenciaDto("carteira", "Carteira de renda variável (lançada por você)",
                "Carteira", composicao.Carteira, new List<PistaConferenciaDto>
                {
                    new PistaConferenciaDto("Pela última cotação guardada; o app da corretora usa a cotação do momento.", null),
                    new PistaConferenciaDto("Cotas diferentes das da corretora aparecem em “Conferir investimentos”.", null, "/investimentos",
                        Parametro("conferir", "1"))
                }));

        return new ConferenciaBancoDto(listaContas, listaCartoes, listaInvestimentos);
    }

    private static Dictionary<string, string> DaConta(int contaId) => Parametro("conta", contaId.ToString(CultureInfo.InvariantCulture));

    private static Dictionary<string, string> Parametro(string nome, string valor) => new() { [nome] = valor };

    private static string Quantos(int quantidade, string um, string varios) => quantidade == 1 ? $"1 {um}" : $"{quantidade} {varios}";

    /// <summary>Data local (fuso de Brasília) de um instante gravado em UTC.</summary>
    private static string Data(DateTime utc)
        => Relogio.ParaDataLocal(new DateTimeOffset(DateTime.SpecifyKind(utc, DateTimeKind.Utc))).ToString("dd/MM/yyyy", PtBr);
}
