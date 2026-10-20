import { abrirTransacoes, parametrosDaUrl } from '../apoio/transacoes';

/**
 * Usabilidade do filtro ativo em Transações (UX-2, #32). Padrão do CLAUDE.md: "filtro ativo sempre visível" (chips
 * com ✕ e "Limpar filtros") em tela que pode abrir já filtrada. Nielsen 1 (visibilidade do estado), 3 (controle e
 * liberdade: desfazer um filtro só) e 8 (sem a faixa quando não há filtro).
 */
describe('Usabilidade: filtros ativos em Transações', () => {
  const chips = (raiz: HTMLElement) =>
    Array.from(raiz.querySelectorAll('.chip-filtro')).map((c) => c.textContent!.replace('✕', '').replace(/\s+/g, ' ').trim());

  it('sem filtro, não mostra a faixa "Filtrando:"', async () => {
    const { elemento } = await abrirTransacoes();
    expect(elemento.querySelector('.faixa-filtros')).toBeNull();
  });

  it('aberta por um link, mostra um chip para cada filtro, com o nome legível', async () => {
    const { elemento } = await abrirTransacoes('/transacoes?categoria=21&conta=3&tipo=despesa&inicio=2026-09-01&fim=2026-09-30&emergencias=1&busca=uber');
    const faixa = elemento.querySelector('.faixa-filtros')!;
    expect(faixa.getAttribute('aria-label')).toBe('Filtros ativos');
    expect(chips(elemento)).toEqual(['setembro/2026', 'Lazer › Cinema', 'Nubank', 'Despesas', 'Só emergências', '“uber”']);
  });

  it('cada chip tem um ✕ com nome acessível, que remove só aquele filtro', async () => {
    const t = await abrirTransacoes('/transacoes?tipo=despesa&busca=uber');
    const remover = t.elemento.querySelector<HTMLButtonElement>('[aria-label="Remover filtro: Despesas"]')!;
    expect(remover.tagName).toBe('BUTTON');
    remover.click();
    await t.assentar();
    expect(chips(t.elemento)).toEqual(['“uber”']);
    expect(parametrosDaUrl(t.router)).toEqual({ busca: 'uber' });
  });

  it('#173 d): a lista vinda de outra tela diz de onde veio e quantos são; o ✕ tira só ela', async () => {
    const t = await abrirTransacoes('/transacoes?inicio=2026-04-01&fim=2026-04-30&ids=7,8&de=fii-rendimentos');
    expect(chips(t.elemento)).toEqual(['abril/2026', 'Rendimentos de FIIs: 2 lançamentos']);
    t.elemento.querySelector<HTMLButtonElement>('[aria-label="Remover filtro: Rendimentos de FIIs: 2 lançamentos"]')!.click();
    await t.assentar();
    expect(chips(t.elemento)).toEqual(['abril/2026']);
    expect(parametrosDaUrl(t.router)).toEqual({ inicio: '2026-04-01', fim: '2026-04-30' });
  });

  it('sem origem conhecida, o chip diz só quantos', async () => {
    const t = await abrirTransacoes('/transacoes?ids=7');
    expect(chips(t.elemento)).toEqual(['Só 1 lançamento escolhido']);
  });

  it('"Limpar filtros" tira todos de uma vez e a faixa some', async () => {
    const t = await abrirTransacoes('/transacoes?tipo=despesa&busca=uber&categoria=sem');
    const limpar = Array.from(t.elemento.querySelectorAll('button')).find((b) => b.textContent!.trim() === 'Limpar filtros')!;
    limpar.click();
    await t.assentar();
    expect(t.elemento.querySelector('.faixa-filtros')).toBeNull();
  });
});
