import { Transacao } from '../../src/app/core/modelos';
import { dataIso } from '../../src/app/compartilhado/utilidades';
import { abrirPainel, painel } from '../apoio/painel';

/**
 * #334: "Últimas transações" mostrava parcelas que ainda vão cair (15/04 Casasbahiacom 11/11…), porque a lista era a
 * das datas mais novas, e as parcelas futuras já lançadas ficavam no topo. Agora "Últimas" vai até hoje (a API), e as
 * futuras ficam atrás do botão "Futuras (N)", com o ano na data e "ver todas" no mesmo recorte.
 */
const transacao = (id: number, descricao: string, data: string, valor: number) =>
  ({ id, descricao, data, valor, descricaoOriginal: descricao, estabelecimento: null, categoriaNome: null, tipoLancamento: 0 }) as unknown as Transacao;

const hoje = new Date();
const amanha = new Date(hoje.getFullYear(), hoje.getMonth(), hoje.getDate() + 1);

async function abrir(futuras: Transacao[], quantidade: number) {
  const t = await abrirPainel(painel({
    ultimasTransacoes: [transacao(1, 'Mercado do bairro', dataIso(hoje), -80)],
    transacoesFuturas: futuras, quantidadeFuturas: quantidade
  }));
  // As últimas transações ficam em "Mais análises".
  const mais = [...t.raiz.querySelectorAll<HTMLButtonElement>('button')].find((b) => b.textContent!.includes('Mais análises'))!;
  if (mais.getAttribute('aria-expanded') !== 'true') { mais.click(); await t.atualizar(); }
  const bloco = () => [...t.raiz.querySelectorAll('.painel-bloco')].find((s) => /transações$/.test(s.querySelector('h2')?.textContent?.trim() ?? ''))!;
  return { ...t, bloco };
}

describe('Regressão: Painel separa as últimas transações das futuras (#334)', () => {
  beforeEach(() => {
    localStorage.clear();
    // O gráfico de "Mais análises" usa o tema, que lê matchMedia (o jsdom não tem).
    Object.defineProperty(window, 'matchMedia', {
      configurable: true, writable: true,
      value: () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })
    });
  });

  it('"Até hoje" é o padrão; "Futuras (N)" mostra as parcelas já lançadas, com o ano', async () => {
    const { bloco, atualizar } = await abrir([transacao(9, 'Casasbahiacom 11/11', '2027-04-15', -200.73)], 7);
    expect(bloco().querySelector('h2')!.textContent!.trim()).toBe('Últimas transações');
    expect(bloco().textContent).toContain('Mercado do bairro');
    expect(bloco().textContent).not.toContain('Casasbahiacom');
    const botoes = [...bloco().querySelectorAll<HTMLButtonElement>('.segmentado-transacoes button')];
    expect(botoes.map((b) => [b.textContent!.trim(), b.getAttribute('aria-pressed')])).toEqual([['Até hoje', 'true'], ['Futuras (7)', 'false']]);
    // "ver todas" no mesmo recorte: até hoje.
    expect(decodeURIComponent(bloco().querySelector('.bloco-titulo a')!.getAttribute('href')!)).toBe(`/transacoes?fim=${dataIso(hoje)}`);

    botoes[1].click();
    await atualizar();
    expect(bloco().querySelector('h2')!.textContent!.trim()).toBe('Próximas transações');
    expect(bloco().textContent).toContain('Casasbahiacom 11/11');
    expect(bloco().textContent).toContain('15/04/27');
    expect(bloco().textContent).not.toContain('Mercado do bairro');
    expect(decodeURIComponent(bloco().querySelector('.bloco-titulo a')!.getAttribute('href')!)).toBe(`/transacoes?inicio=${dataIso(amanha)}`);
  });

  it('sem transação futura, não há o botão, e a lista é só a de até hoje', async () => {
    const { bloco } = await abrir([], 0);
    expect(bloco().querySelector('.segmentado-transacoes')).toBeNull();
    expect(bloco().textContent).toContain('Mercado do bairro');
  });
});
