import { TipoConta } from '../../src/app/core/modelos';
import { abrirContas, conta } from '../apoio/contas';

/**
 * Regressão do #251: a fatura do mês já paga (o "Pagamento recebido" estava nas transações do cartão, mas o banco não
 * mandou o pagamento na fatura) aparecia como "fechada", e o card não dizia nada da fatura seguinte, a que importa
 * depois do fechamento. Agora a API reconhece o pagamento lançado no cartão (ContaService.PagoNasTransacoes) e devolve
 * a próxima fatura aberta; aqui fica o que o card mostra (Nielsen 1, Visibility of System Status).
 */
describe('Regressão #251: fatura paga e próxima fatura no card do cartão', () => {
  const texto = (el: Element | null) => el!.textContent!.replace(/\s+/g, ' ').trim();
  const paga = { contaId: 7, cartao: 'Nubank', valorMes: 1593.69, parcelasMes: 0, faturaId: 41, vencimento: '2026-10-14',
    situacao: 'paga' as const, fechamento: '2026-10-07' };
  const proxima = { contaId: 7, cartao: 'Nubank', valorMes: 812.4, parcelasMes: 120, faturaId: null, vencimento: '2026-11-14',
    situacao: 'aberta' as const, fechamento: '2026-11-07' };
  const cartao = (faturaMes: object) => conta({ id: 7, nome: 'Nubank', tipo: TipoConta.CartaoCredito, saldo: -2406.09,
    limiteCredito: 5000, faturaMes: faturaMes as never });

  it('a fatura do mês aparece paga e, logo depois, a próxima com fechamento, vencimento e "em aberto"', async () => {
    const t = await abrirContas('/contas', [cartao({ ...paga, proxima })]);
    const bloco = t.cartao(7).querySelector('.valor-fatura-mes');
    expect(texto(bloco!.querySelector('small'))).toBe('fecha 07/10, vence 14/10 paga');
    expect(bloco!.querySelector('.etiqueta-sucesso')).not.toBeNull();
    expect(texto(t.cartao(7).querySelector('.proxima-fatura'))).toMatch(
      /^Próxima fatura: R\$\s812,40 · fecha 07\/11, vence 14\/11 em aberto$/);
  });

  it('sem próxima fatura (a do mês ainda aberta), o card não mostra a linha', async () => {
    const t = await abrirContas('/contas', [cartao({ ...paga, situacao: 'aberta', proxima: null })]);
    expect(t.cartao(7).querySelector('.proxima-fatura')).toBeNull();
  });
});
