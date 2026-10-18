import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Regressão: em Contas e cartões, os cards de cartão tinham alturas diferentes (align-items: start na grade) e, nos
 * cards de conta, um nome de banco longo empurrava o saldo para baixo, desalinhado dos vizinhos. O jsdom não calcula
 * grid nem subgrid: confere, com o styles.css do site, as regras que resolvem.
 */
describe('Regressão: cards de Contas e cartões do mesmo tamanho e alinhados', () => {
  let estilo: HTMLStyleElement;
  beforeAll(() => {
    estilo = document.createElement('style');
    estilo.textContent = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
    document.head.appendChild(estilo);
  });
  afterAll(() => estilo.remove());

  /** Regras (fora de @media) que casam com o elemento. */
  const regras = (el: Element) => Array.from(estilo.sheet!.cssRules)
    .filter((r): r is CSSStyleRule => r instanceof CSSStyleRule && el.matches(r.selectorText)).map((r) => r.cssText);

  function montar(html: string) {
    const div = document.createElement('div');
    div.innerHTML = html;
    document.body.appendChild(div);
    return div;
  }

  it('cartões de crédito: a grade não deixa cada um com a própria altura (sem align-items: start)', () => {
    const div = montar('<div class="grade-cartoes grade-cartoes-credito"><article class="cartao-credito-bloco"></article></div>');
    expect(regras(div.firstElementChild!).some((c) => c.includes('align-items: start'))).toBe(false);
    div.remove();
  });

  it('contas: as linhas de cada card seguem as da grade (subgrid) e o rodapé fica sempre na 5ª linha', () => {
    const div = montar('<div class="grade-cartoes"><article class="cartao-conta"><header></header><footer></footer></article></div>');
    const card = div.querySelector('.cartao-conta')!;
    expect(regras(card).some((c) => c.includes('grid-template-rows: subgrid') && c.includes('grid-row: span 5'))).toBe(true);
    expect(regras(card.querySelector('footer')!).some((c) => c.includes('grid-row: 5'))).toBe(true);
    div.remove();
  });
});
