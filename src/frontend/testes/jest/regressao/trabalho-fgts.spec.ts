import { MesFgts, MotivoSaqueFgts as M, TipoLancamentoFgts as T } from '../../../src/app/core/modelos';
import { descreverLancamento, totalDepositado, ultimosMeses } from '../../../src/app/paginas/trabalho/fgts';

/** #216 (ADR 0066): as regras da tela Trabalho › FGTS (a estimativa é da API, testada no Go). */
describe('Trabalho › FGTS (#216)', () => {
  const mes = (m: string, deposito: number): MesFgts =>
    ({ mes: m, correcao: 0, deposito, distribuicao: 0, saques: 0, saldo: 0, conferido: false });

  it('descreve o lançamento, com o motivo do saque', () => {
    expect(descreverLancamento({ id: 1, data: '2026-11-10', tipo: T.Conferencia, valor: 1, motivo: null })).toBe('Saldo que vi no app do FGTS');
    expect(descreverLancamento({ id: 2, data: '2026-11-10', tipo: T.Saque, valor: 1, motivo: M.SaqueAniversario })).toBe('Saque (saque-aniversário)');
    expect(descreverLancamento({ id: 3, data: '2026-11-10', tipo: 9 as T, valor: 1, motivo: null })).toBe('—');
  });

  it('os últimos meses, do mais novo, e o total depositado em centavos', () => {
    const meses = Array.from({ length: 14 }, (_, i) => mes(`2026-${String((i % 12) + 1).padStart(2, '0')}-01`, 0.1));
    const ultimos = ultimosMeses(meses);
    expect(ultimos).toHaveLength(12);
    expect(ultimos[0]).toBe(meses[13]);
    expect(ultimosMeses(meses, 2)).toEqual([meses[13], meses[12]]);
    expect(totalDepositado([mes('a', 0.1), mes('b', 0.2)])).toBe(0.3);
    expect(totalDepositado([])).toBe(0);
  });
});
