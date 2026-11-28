import { anoDaUrl, camposDaPrimeira } from '../../../src/app/paginas/trabalho/decimo-terceiro';

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

  it('o formulário da 1ª parcela mostra só o que a pessoa informou: o 30/11 padrão não vira data informada (#750)', () => {
    const padrao = { valor: 3000, data: '2026-11-30', informada: false, valorInformado: false, dataInformada: false };
    expect(camposDaPrimeira(padrao)).toEqual({ em: null, valor: null });
    // Só o valor informado: a data continua a padrão, e o campo dela fica vazio (antes, ia 30/11 e "Gravar" a fixava).
    expect(camposDaPrimeira({ ...padrao, valor: 2800, informada: true, valorInformado: true })).toEqual({ em: null, valor: 2800 });
    expect(camposDaPrimeira({ ...padrao, data: '2026-07-10', informada: true, dataInformada: true })).toEqual({ em: '2026-07-10', valor: null });
    // A que veio com as férias: a data é delas, não do formulário.
    expect(camposDaPrimeira({ ...padrao, data: '2026-06-28', informada: true, comFerias: true })).toEqual({ em: null, valor: null });
  });
});
