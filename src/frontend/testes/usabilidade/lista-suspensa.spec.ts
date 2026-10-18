import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Usabilidade: a lista suspensa (<select>) fechada tem o visual do site (seta própria, sem a caixa do sistema), numa
 * linha só, e continua sendo o <select> nativo (teclado, leitor de tela e a lista do celular não mudam).
 */
describe('Usabilidade: lista suspensa no visual do site', () => {
  let estilo: HTMLStyleElement;
  beforeAll(() => {
    estilo = document.createElement('style');
    estilo.textContent = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
    document.head.appendChild(estilo);
  });
  afterAll(() => estilo.remove());

  const regras = (el: Element) => Array.from(estilo.sheet!.cssRules)
    .filter((r): r is CSSStyleRule => r instanceof CSSStyleRule && el.matches(r.selectorText)).map((r) => r.cssText).join('\n');

  it('select comum: sem a aparência do sistema, com a seta desenhada pelo site e espaço para ela', () => {
    const select = document.createElement('select');
    document.body.appendChild(select);
    const css = regras(select);
    expect(css).toContain('appearance: none');
    expect(css).toMatch(/background-image: linear-gradient\(45deg/);
    expect(parseFloat(getComputedStyle(select).paddingRight)).toBeGreaterThanOrEqual(30);
    expect(getComputedStyle(select).whiteSpace).toBe('nowrap');
    select.remove();
  });

  it('a seta continua no foco (a regra da seta vem depois da regra de foco, que troca o fundo)', () => {
    const css = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
    expect(css.indexOf('select:not([multiple]) {')).toBeGreaterThan(css.indexOf('input:focus, select:focus, textarea:focus'));
  });

  it('select múltiplo fica de fora (não ganha a seta)', () => {
    const multiplo = document.createElement('select');
    multiplo.multiple = true;
    expect(regras(multiplo)).not.toContain('linear-gradient(45deg');
  });
});
