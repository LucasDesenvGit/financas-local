import { anoDaUrl } from '../../../src/app/paginas/trabalho/decimo-terceiro';

/** #214: o ano do 13º vem da URL (ADR 0018); fora do formato, o ano de agora. */
describe('Trabalho › 13º salário (#214)', () => {
  it('lê o ano da URL', () => {
    expect(anoDaUrl('2025', 2026)).toBe(2025);
    expect(anoDaUrl(null, 2026)).toBe(2026);
    expect(anoDaUrl('', 2026)).toBe(2026);
    expect(anoDaUrl('abc', 2026)).toBe(2026);
    expect(anoDaUrl('2025.5', 2026)).toBe(2026);
    expect(anoDaUrl('1899', 2026)).toBe(2026);
    expect(anoDaUrl('2201', 2026)).toBe(2026);
    expect(anoDaUrl('1900', 2026)).toBe(1900);
  });
});
