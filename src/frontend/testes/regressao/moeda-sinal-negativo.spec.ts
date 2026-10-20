import { TestBed } from '@angular/core/testing';
import { MoedaPipe } from '../../src/app/compartilhado/moeda.pipe';

/**
 * #257: valor negativo com o "−" de verdade (U+2212). Com o hífen do teclado, o navegador quebrava a linha logo depois
 * dele numa coluna estreita (Projeção, "Saldo previsto") e o "-" ficava sozinho em cima do valor.
 */
describe('pipe de moeda: sinal de menos', () => {
  const formatar = (v: number) => TestBed.runInInjectionContext(() => new MoedaPipe()).transform(v);

  it('negativo começa com U+2212, sem hífen', () => {
    const texto = formatar(-14863.64)!;
    expect(texto.charCodeAt(0)).toBe(0x2212);
    expect(texto).not.toContain('-');
    expect(texto.slice(1)).toBe(formatar(14863.64));
  });

  it('positivo e zero não mudam', () => {
    expect(formatar(1234.5)).not.toMatch(/[-−]/);
    expect(formatar(0)).not.toMatch(/[-−]/);
  });
});
