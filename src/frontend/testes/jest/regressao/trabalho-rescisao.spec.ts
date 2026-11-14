import { AvisoRescisao as A, TipoRescisao as T } from '../../../src/app/core/modelos';
import { avisoPadrao, avisosDoTipo, textoDoFgts } from '../../../src/app/paginas/trabalho/rescisao';

/** #215: as regras da tela Trabalho › Rescisão (as da CLT e o cálculo são da API, testados no Go). */
describe('Trabalho › Rescisão (#215)', () => {
  it('os avisos que fazem sentido para cada tipo, e o padrão', () => {
    expect(avisosDoTipo(T.JustaCausa).map((a) => a.valor)).toEqual([A.Sem]);
    expect(avisosDoTipo(T.FimExperiencia).map((a) => a.valor)).toEqual([A.Sem]);
    expect(avisosDoTipo(T.PedidoDemissao).map((a) => a.valor)).toEqual([A.Trabalhado, A.Indenizado]);
    expect(avisosDoTipo(T.SemJustaCausa).map((a) => a.valor)).toEqual([A.Indenizado, A.Trabalhado]);
    expect(avisoPadrao(T.Acordo)).toBe(A.Indenizado);
    expect(avisoPadrao(T.PedidoDemissao)).toBe(A.Trabalhado);
  });

  it('o FGTS em palavras', () => {
    expect(textoDoFgts(0, 0)).toBe('Sem multa e sem saque: o saldo fica na conta do FGTS.');
    expect(textoDoFgts(0, 100)).toBe('Sem multa; saca 100% do saldo.');
    expect(textoDoFgts(20, 80)).toBe('Multa de 20% sobre o saldo; saca 80% do saldo e a multa.');
  });
});
