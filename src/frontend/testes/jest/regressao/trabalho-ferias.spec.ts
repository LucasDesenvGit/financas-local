import { PeriodoFerias } from '../../../src/app/core/modelos';
import { diferencaDaVenda, periodoParaAgendar, primeiroDiaPossivel, situacaoDoPeriodo } from '../../../src/app/paginas/trabalho/ferias';

/** #213: as regras da tela Trabalho › Férias (as da CLT e o cálculo são da API, testados no Go). */
describe('Trabalho › Férias (#213)', () => {
  const periodo = (p: Partial<PeriodoFerias>): PeriodoFerias => ({
    inicio: '2025-03-11', fim: '2026-03-10', fimConcessivo: '2027-03-10', completo: true, faltas: 0, diasDireito: 30,
    diasAgendados: 0, diasVendidos: 0, saldo: 30, vencida: false, vencendo: false, agendamentos: [], ...p
  });

  it('a situação em palavras', () => {
    expect(situacaoDoPeriodo(periodo({ completo: false }))).toEqual({ texto: 'Em aquisição', tom: 'neutro' });
    expect(situacaoDoPeriodo(periodo({ vencida: true }))).toEqual({ texto: 'Vencida', tom: 'erro' });
    expect(situacaoDoPeriodo(periodo({ vencendo: true }))).toEqual({ texto: 'Vencendo', tom: 'atencao' });
    expect(situacaoDoPeriodo(periodo({ saldo: 0 }))).toEqual({ texto: 'Agendada', tom: 'ok' });
    expect(situacaoDoPeriodo(periodo({}))).toEqual({ texto: 'Para tirar', tom: 'neutro' });
  });

  it('agenda no período completo mais antigo com saldo; sem completo, no que está em aquisição', () => {
    const a = periodo({ inicio: '2024-03-11', saldo: 0 });
    const b = periodo({ inicio: '2025-03-11' });
    const c = periodo({ inicio: '2026-03-11', completo: false });
    expect(periodoParaAgendar([a, b, c])!.inicio).toBe('2025-03-11');
    expect(periodoParaAgendar([a, c])!.inicio).toBe('2026-03-11');
    expect(periodoParaAgendar([a])).toBeNull();
  });

  it('o primeiro dia possível é o seguinte ao fim do período, também na virada do ano', () => {
    expect(primeiroDiaPossivel(periodo({ fim: '2026-03-10' }))).toBe('2026-03-11');
    expect(primeiroDiaPossivel(periodo({ fim: '2026-12-31' }))).toBe('2027-01-01');
  });

  it('a diferença da venda, em centavos', () => {
    expect(diferencaDaVenda(6040.64, 7329.37)).toBe(1288.73);
  });
});
