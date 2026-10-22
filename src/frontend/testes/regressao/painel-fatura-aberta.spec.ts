import { of } from 'rxjs';
import { CHAVE_FATURAS_NO_PAINEL, faturaEmAberto, lerCartoesNoPainel } from '../../src/app/compartilhado/faturas-no-painel';
import { FaturaMes, TipoConta } from '../../src/app/core/modelos';
import { abrirContas, conta } from '../apoio/contas';
import { abrirPainel, painel } from '../apoio/painel';

/**
 * #322: a fatura em aberto dos cartões escolhidos em Contas e cartões aparece no Painel, em "Situação hoje". A escolha
 * é um interruptor em cada cartão (efeito imediato, ADR 0032) e fica neste navegador (ADR 0017).
 */
const fatura = (campos: Partial<FaturaMes>): FaturaMes => ({
  contaId: 2, cartao: 'Ultravioleta', valorMes: 820, parcelasMes: 0, faturaId: null, vencimento: '2026-11-10',
  situacao: 'aberta', fechamento: '2026-11-03', ...campos
});
const ULTRAVIOLETA = conta({ id: 2, nome: 'Ultravioleta', tipo: TipoConta.CartaoCredito, saldo: -2950,
  faturaMes: fatura({ situacao: 'fechada', valorMes: 820, proxima: fatura({ valorMes: 315.4, fechamento: '2026-12-03', vencimento: '2026-12-10' }) }) });
const ROXO = conta({ id: 3, nome: 'Cartão Roxo', tipo: TipoConta.CartaoCredito, saldo: -1530,
  faturaMes: fatura({ contaId: 3, cartao: 'Cartão Roxo', valorMes: 1530, situacao: 'aberta' }) });

describe('Regressão: fatura em aberto no Painel (#322)', () => {
  beforeEach(() => localStorage.clear());

  it('a fatura em aberto é a do mês enquanto não fecha; depois, a próxima', () => {
    expect(faturaEmAberto(ROXO.faturaMes)!.valorMes).toBe(1530);
    expect(faturaEmAberto(ULTRAVIOLETA.faturaMes)!.valorMes).toBe(315.4);
    // Sem fechamento conhecido (situação null), vale o mês do calendário.
    expect(faturaEmAberto(fatura({ situacao: null, valorMes: 99 }))!.valorMes).toBe(99);
    // Fechou e a próxima não veio: nenhuma.
    expect(faturaEmAberto(fatura({ situacao: 'paga', proxima: null }))).toBeNull();
    expect(faturaEmAberto(null)).toBeNull();
  });

  it('a escolha guardada que não é lista de números é ignorada', () => {
    localStorage.setItem(CHAVE_FATURAS_NO_PAINEL, '{"x":1}');
    expect(lerCartoesNoPainel()).toEqual([]);
    localStorage.setItem(CHAVE_FATURAS_NO_PAINEL, '[2,"3",4.5,7]');
    expect(lerCartoesNoPainel()).toEqual([2, 7]);
  });

  it('o interruptor de cada cartão liga e desliga a fatura no Painel, na hora', async () => {
    const { raiz, atualizar } = await abrirContas('/contas', [ULTRAVIOLETA, ROXO]);
    const interruptores = [...raiz.querySelectorAll<HTMLInputElement>('.interruptor-painel input[role="switch"]')];
    expect(interruptores).toHaveLength(2);
    const doCartao = (nome: string) => interruptores.find((i) => i.closest('label')!.textContent!.includes(`(${nome})`))!;
    expect(doCartao('Ultravioleta').closest('label')!.textContent!.replace(/\s+/g, ' ').trim()).toBe('Fatura no Painel (Ultravioleta)');
    doCartao('Cartão Roxo').click();
    await atualizar();
    expect(lerCartoesNoPainel()).toEqual([3]);
    expect(doCartao('Cartão Roxo').checked).toBe(true);
    expect(doCartao('Ultravioleta').checked).toBe(false);
    doCartao('Cartão Roxo').click();
    await atualizar();
    expect(lerCartoesNoPainel()).toEqual([]);
  });

  it('sem cartão escolhido, o Painel não pede as contas nem mostra o card', async () => {
    let pedidas = 0;
    const contas = () => { pedidas++; return of([ULTRAVIOLETA, ROXO]); };
    const sem = await abrirPainel(painel(), '/painel', { contas });
    expect(pedidas).toBe(0);
    expect(sem.raiz.querySelector('.kpi-fatura-aberta')).toBeNull();
  });

  it('no Painel, um card só com o total; "ver detalhes" abre a fatura de cada cartão', async () => {
    localStorage.setItem(CHAVE_FATURAS_NO_PAINEL, '[2,3]');
    const { raiz, atualizar } = await abrirPainel(painel(), '/painel', { contas: () => of([ULTRAVIOLETA, ROXO]) });
    const cards = [...raiz.querySelectorAll('.kpi-fatura-aberta')];
    expect(cards).toHaveLength(1);
    const card = cards[0];
    // 315,40 (a próxima do Ultravioleta, porque a do mês fechou) + 1.530,00 (a do Roxo, aberta).
    expect(card.querySelector('strong')!.textContent!.replace(/\s+/g, ' ').trim()).toBe('R$ 1.845,40');
    expect(card.textContent).toContain('total dos 2 cartões escolhidos');
    const botao = card.querySelector<HTMLButtonElement>('.botao-detalhes')!;
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(card.querySelector('.lista-faturas-abertas')).toBeNull();
    botao.click();
    await atualizar();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    expect(card.querySelector('#' + botao.getAttribute('aria-controls') + ' .lista-faturas-abertas')).not.toBeNull();
    const linhas = [...card.querySelectorAll('.lista-faturas-abertas li')].map((l) => l.textContent!.replace(/\s+/g, ' ').trim());
    for (const pedaco of ['Ultravioleta', 'R$ 315,40', 'fecha 03/12, vence 10/12']) expect(linhas[0]).toContain(pedaco);
    for (const pedaco of ['Cartão Roxo', 'R$ 1.530,00']) expect(linhas[1]).toContain(pedaco);
    expect(card.querySelector('.lista-faturas-abertas a')!.getAttribute('href')).toBe('/contas?faturas=2');
  });

  it('em Contas e cartões, o mesmo card no resumo, que muda na hora com o interruptor', async () => {
    const { raiz, atualizar } = await abrirContas('/contas', [ULTRAVIOLETA, ROXO]);
    expect(raiz.querySelector('.kpis-contas .kpi-fatura-aberta')).toBeNull();
    const roxo = [...raiz.querySelectorAll<HTMLInputElement>('.interruptor-painel input')]
      .find((i) => i.closest('label')!.textContent!.includes('(Cartão Roxo)'))!;
    roxo.click();
    await atualizar();
    const card = raiz.querySelector('.kpis-contas .kpi-fatura-aberta')!;
    expect(card.querySelector('strong')!.textContent!.replace(/\s+/g, ' ').trim()).toBe('R$ 1.530,00');
    expect(card.textContent).toContain('do cartão escolhido');
    card.querySelector<HTMLButtonElement>('.botao-detalhes')!.click();
    await atualizar();
    // Em Contas, o cartão já está na tela: o nome não é link.
    expect(card.querySelector('.lista-faturas-abertas a')).toBeNull();
  });
});
