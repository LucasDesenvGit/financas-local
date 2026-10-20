import { TipoConta } from '../../src/app/core/modelos';
import { abrirContas, conta } from '../apoio/contas';
import { abrirPainel, painel } from '../apoio/painel';

/**
 * Regressão do #144: a "fatura do mês" somava do dia 1 ao fim do mês e ficava muito abaixo da fatura do banco. Agora
 * é a fatura que fecha no mês (do fechamento anterior a este); a tela diz quando ela fecha e quando vence, para o
 * valor poder ser conferido com o app do banco (Nielsen 1, Visibility of System Status). O cálculo do ciclo fica na
 * API (ContaService.CicloDaFatura); aqui fica o que a tela mostra.
 */
describe('Regressão #144: fatura do mês segue o fechamento do cartão', () => {
  const texto = (el: Element | null) => el!.textContent!.replace(/\s+/g, ' ').trim();
  const aberta = { contaId: 4, cartao: 'Platinum', valorMes: 2948.22, parcelasMes: 2279.61, faturaId: null,
    vencimento: '2026-11-05', situacao: 'aberta' as const, fechamento: '2026-10-26' };

  it('no Painel, cada cartão diz quando a fatura fecha e quando vence', async () => {
    const t = await abrirPainel(painel({ faturasDoMes: [aberta], totalFaturasDoMes: 2948.22 }), '/painel?ano=2026&mes=10');
    expect(texto(t.raiz.querySelector('.kpi-faturas small'))).toBe('a fatura de cada cartão que fecha no mês');
    expect(texto(t.raiz.querySelector('.lista-faturas li'))).toMatch(
      /Platinum\s*R\$\s2\.948,22\s*R\$\s2\.279,61 em parcelas\s*fecha 26\/10, vence 05\/11 em aberto/);
  });

  it('cartão sem fechamento conhecido continua só com o vencimento', async () => {
    const t = await abrirPainel(painel({ faturasDoMes: [{ ...aberta, fechamento: null }], totalFaturasDoMes: 2948.22 }));
    expect(texto(t.raiz.querySelector('.lista-faturas .detalhe-fatura:last-child'))).toBe('fatura vence 05/11 em aberto');
  });

  it('em Contas e cartões, "Fatura de <mês>" mostra fechamento e vencimento', async () => {
    const t = await abrirContas('/contas', [conta({ id: 4, nome: 'Platinum', tipo: TipoConta.CartaoCredito, saldo: -8802.04,
      limiteCredito: 10000, faturaMes: aberta })]);
    expect(texto(t.cartao(4).querySelector('.valor-fatura-mes strong'))).toMatch(/R\$\s2\.948,22/);
    expect(texto(t.cartao(4).querySelector('.valor-fatura-mes small'))).toBe('fecha 26/10, vence 05/11 em aberto');
  });
});
