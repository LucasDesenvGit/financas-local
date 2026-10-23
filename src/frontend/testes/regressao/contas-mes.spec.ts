import { of } from 'rxjs';
import { CHAVE_FATURAS_NO_PAINEL } from '../../src/app/compartilhado/faturas-no-painel';
import { FaturaMes, TipoConta } from '../../src/app/core/modelos';
import { NOMES_MESES } from '../../src/app/compartilhado/utilidades';
import { abrirContas, conta } from '../apoio/contas';
import { abrirPainel, painel } from '../apoio/painel';

/**
 * #323: Contas e cartões mostra as faturas de outros meses, como o Painel. O mês fica na URL; saldo, dívida, limite e a
 * fatura em aberto continuam sendo os de hoje.
 */
const hoje = new Date();
const fatura = (campos: Partial<FaturaMes>): FaturaMes => ({
  contaId: 2, cartao: 'Ultravioleta', valorMes: 820, parcelasMes: 0, faturaId: 30, vencimento: '2026-10-10',
  situacao: 'aberta', fechamento: '2026-10-03', ...campos
});
const ULTRAVIOLETA = conta({ id: 2, nome: 'Ultravioleta', tipo: TipoConta.CartaoCredito, saldo: -2950,
  faturaMes: fatura({ valorMes: 820 }) });
const SETEMBRO_PASSADO = fatura({ valorMes: 1234.56, situacao: 'paga', vencimento: '2025-09-10', fechamento: '2025-09-03' });

describe('Regressão: Contas e cartões em outro mês (#323)', () => {
  beforeEach(() => localStorage.clear());

  it('?ano=&mes= abre no mês, busca as faturas dele e diz o que continua sendo de hoje', async () => {
    const pedidos: [number, number][] = [];
    localStorage.setItem(CHAVE_FATURAS_NO_PAINEL, '[2]');
    const { raiz, tela, cartao } = await abrirContas('/contas?ano=2025&mes=9', [ULTRAVIOLETA], {},
      { faturasDoMes: (ano: number, mes: number) => { pedidos.push([ano, mes]); return of([SETEMBRO_PASSADO]); } });
    expect(pedidos).toEqual([[2025, 9]]);
    expect(tela.nomeMes()).toBe('setembro/2025');
    const valor = cartao(2).querySelector('.valor-fatura-mes')!.textContent!.replace(/\s+/g, ' ');
    expect(valor).toContain('Fatura de setembro/2025');
    expect(valor).toContain('R$ 1.234,56');
    expect(valor).toContain('paga');
    expect(raiz.querySelector('.nota-mes')!.textContent).toContain('Saldo, dívida total e limite continuam sendo os de hoje');
    // A fatura em aberto (card do resumo) é a de hoje, e não a do mês escolhido.
    expect(raiz.querySelector('.kpi-fatura-aberta strong')!.textContent!.replace(/\s+/g, ' ').trim()).toBe('R$ 820,00');
    // "Próxima fatura" é coisa de hoje: some em outro mês.
    expect(cartao(2).querySelector('.proxima-fatura')).toBeNull();
  });

  it('no mês atual, não busca nada a mais; voltar ao mês atual tira ano e mês da URL', async () => {
    const pedidos: [number, number][] = [];
    const { raiz, tela, atualizar, router } = await abrirContas('/contas?ano=2025&mes=9', [ULTRAVIOLETA], {},
      { faturasDoMes: (ano: number, mes: number) => { pedidos.push([ano, mes]); return of([SETEMBRO_PASSADO]); } });
    (raiz.querySelector('.nota-mes button') as HTMLButtonElement).click();
    await atualizar();
    expect(tela.mesAtual()).toBe(true);
    expect(router.url).toBe('/contas');
    expect(pedidos).toHaveLength(1);
    expect(raiz.querySelector('.nota-mes')).toBeNull();
    const texto = raiz.textContent!.replace(/\s+/g, ' ');
    expect(texto).toContain(`Fatura de ${NOMES_MESES[hoje.getMonth()]}`);
    expect(texto).toContain('R$ 820,00');
  });

  it('trocar de mês grava ano e mês na URL e busca as faturas', async () => {
    const pedidos: [number, number][] = [];
    const { tela, atualizar, router } = await abrirContas('/contas', [ULTRAVIOLETA], {},
      { faturasDoMes: (ano: number, mes: number) => { pedidos.push([ano, mes]); return of([]); } });
    expect(pedidos).toHaveLength(0);
    tela.mudarPeriodo({ ano: 2025, mes: 9 });
    await atualizar();
    expect(router.url).toBe('/contas?ano=2025&mes=9');
    expect(pedidos).toEqual([[2025, 9]]);
  });

  it('o link de "Faturas do mês" no Painel leva o mês escolhido lá', async () => {
    const { raiz } = await abrirPainel(painel({ faturasDoMes: [fatura({ valorMes: 820 })], totalFaturasDoMes: 820 }),
      '/painel?ano=2025&mes=9');
    const href = raiz.querySelector('.lista-faturas a')!.getAttribute('href')!;
    expect(href).toBe('/contas?faturas=2&ano=2025&mes=9');
  });
});
