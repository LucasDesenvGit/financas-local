import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Usabilidade: o que aparece depois (um botão, um aviso, um card, um menu que abre…) entra com um fade curto em vez
 * de surgir de uma vez, e nada anima com "reduzir movimento" no sistema. O jsdom não roda animações: confere as
 * regras do styles.css.
 */
describe('Usabilidade: elementos que aparecem entram com animação', () => {
  const css = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
  let estilo: HTMLStyleElement;
  beforeAll(() => {
    estilo = document.createElement('style');
    estilo.textContent = css;
    document.head.appendChild(estilo);
  });
  afterAll(() => estilo.remove());

  const regraCom = (el: Element, animacao: string) => Array.from(estilo.sheet!.cssRules).some((r) =>
    r instanceof CSSStyleRule && r.cssText.includes(animacao) && el.matches(r.selectorText));
  /** Elemento dentro da área da página (.pagina), onde a regra de entrada vale. */
  function naPagina(el: HTMLElement): HTMLElement {
    const pagina = document.createElement('main');
    pagina.className = 'pagina';
    pagina.appendChild(el);
    document.body.appendChild(pagina);
    return el;
  }
  afterEach(() => document.querySelectorAll('main.pagina').forEach((m) => m.remove()));

  it.each(['botao', 'botao-icone', 'etiqueta', 'aviso-inline', 'painel-bloco', 'kpi', 'cartao-conta', 'indicador',
    'vazio', 'chip'])('.%s entra com o fade "surgir"', (classe) => {
    const el = document.createElement('div');
    el.className = classe;
    expect(regraCom(naPagina(el), 'surgir')).toBe(true);
  });

  it.each(['section', 'article', 'li', 'figure'])('<%s> que aparece na página entra com o fade', (tag) => {
    expect(regraCom(naPagina(document.createElement(tag)), 'surgir')).toBe(true);
  });

  it('menus que abrem por cima (menu do usuário, seletor de categorias) também entram com o fade', () => {
    for (const classe of ['menu-flutuante', 'seletor-multiplo-painel']) {
      const el = document.createElement('div');
      el.className = classe;
      expect(regraCom(el, 'surgir')).toBe(true);
    }
  });

  it('texto comum, tabelas e linhas de tabela não animam (re-render de lista não fica piscando)', () => {
    for (const tag of ['p', 'table', 'tr', 'td', 'span']) {
      expect(regraCom(naPagina(document.createElement(tag)), 'surgir')).toBe(false);
    }
  });

  it('regiões que abrem e fecham no meio da página animam a altura (expandir/recolher) e vencem o fade comum', () => {
    const bloco = document.createElement('section');
    bloco.className = 'painel-bloco barra-edicao expandir';
    naPagina(bloco);
    expect(regraCom(bloco, 'expandir')).toBe(true);
    const css = estilo.textContent!;
    expect(css.indexOf(':root .expandir')).toBeGreaterThan(css.indexOf('.pagina :is(section'));
    expect(css).toContain('interpolate-size: allow-keywords');
    const saindo = document.createElement('section');
    saindo.className = 'painel-bloco recolher';
    expect(regraCom(naPagina(saindo), 'recolher')).toBe(true);
  });

  it('a barra do modo edição de Transações usa expandir/recolher (não fecha de uma vez)', () => {
    const fonte = readFileSync(join(__dirname, '../../src/app/paginas/transacoes.component.ts'), 'utf8');
    expect(fonte).toMatch(/class="painel-bloco barra-edicao expandir"[^>]*animate\.leave="recolher"/);
  });

  it('saída: .sair some com fade e vence a regra de entrada (mesma especificidade, vem depois)', () => {
    const el = document.createElement('div');
    el.className = 'botao sair';
    naPagina(el);
    expect(regraCom(el, 'sumir')).toBe(true);
    const css = estilo.textContent!;
    expect(css.indexOf(':root .sair')).toBeGreaterThan(css.indexOf('.pagina :is(section'));
  });

  it('"reduzir movimento" desliga todas as animações e transições', () => {
    expect(css).toMatch(/@media \(prefers-reduced-motion: reduce\)\s*\{\s*\*, \*::before, \*::after \{ transition: none !important; animation: none !important; \}/);
  });
});
