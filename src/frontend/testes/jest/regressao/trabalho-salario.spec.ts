import { Emprego, RegimeEmprego } from '../../../src/app/core/modelos';
import {
  aliquota, empregoAtual, explicarDeducao, formatarCnpj, formularioDe, formularioVazio, historicoDeSalarios, mesDaApi,
  nomeDoMes, paraSalvar, rotuloRegime, variacao
} from '../../../src/app/paginas/trabalho/salario';

/** #210 (ADR 0065): as regras da tela Trabalho › Salário (o cálculo do INSS e do IRRF é da API, testado no Go). */
describe('Trabalho › Salário (#210)', () => {
  const emprego = (id: number, saida: string | null, admissao = '2024-03-11'): Emprego => ({
    id, empregador: `E${id}`, cnpj: '12345678000190', admissao, saida, regime: RegimeEmprego.Clt, jornadaMensalHoras: 220,
    dependentes: 1, contaId: 3, contaNome: 'Conta', salarios: [
      { id: 2, inicio: '2026-06-01', salarioBruto: 6000 }, { id: 1, inicio: '2024-03-01', salarioBruto: 5000 }]
  });

  it('o emprego atual é o primeiro sem saída; sem nenhum, o mais recente', () => {
    expect(empregoAtual([emprego(1, '2023-01-01'), emprego(2, null)])!.id).toBe(2);
    expect(empregoAtual([emprego(1, '2023-01-01'), emprego(2, '2024-01-01')])!.id).toBe(1);
    expect(empregoAtual([])).toBeNull();
  });

  it('o histórico mostra a variação de cada aumento sobre o salário de antes', () => {
    const h = historicoDeSalarios(emprego(1, null).salarios);
    expect(h.map((l) => l.variacao)).toEqual([0.19999999999999996, null]);
    expect(variacao(h[0].variacao!)).toBe('+20,0%');
    expect(variacao(-0.05)).toBe('−5,0%');
    expect(historicoDeSalarios([{ id: 1, inicio: '2026-01-01', salarioBruto: 1 }, { id: 0, inicio: '2025-01-01', salarioBruto: 0 }])[0].variacao)
      .toBeNull();
  });

  it('textos: mês, alíquota, regime, CNPJ e a dedução usada', () => {
    expect(nomeDoMes('2026-03-01')).toBe('março de 2026');
    expect(mesDaApi({ ano: 2026, mes: 3 })).toBe('2026-03');
    expect(aliquota(0.075)).toBe('7,5%');
    expect(aliquota(0.275)).toBe('27,5%');
    expect(rotuloRegime(RegimeEmprego.Pj)).toBe('PJ');
    expect(rotuloRegime(9 as RegimeEmprego)).toBe('—');
    expect(formatarCnpj('12345678000190')).toBe('12.345.678/0001-90');
    expect(formatarCnpj('123')).toBe('123');
    expect(formatarCnpj(null)).toBe('');
    const irrf = { deducaoUsada: 'simplificada' } as Parameters<typeof explicarDeducao>[0];
    expect(explicarDeducao(irrf)).toContain('desconto simplificado');
    expect(explicarDeducao({ ...irrf, deducaoUsada: 'legal' })).toContain('Usei o INSS e os dependentes');
  });

  it('o formulário: vazio com a jornada da CLT; ao criar leva o salário; ao editar, não', () => {
    expect(formularioVazio()).toMatchObject({ regime: RegimeEmprego.Clt, jornadaMensalHoras: 220, dependentes: 0 });
    const f = { ...formularioVazio(), empregador: '  Empresa  ', cnpj: ' ', admissao: '2026-01-05', salarioBruto: 3000, contaId: 2 };
    expect(paraSalvar(f, true)).toEqual({ empregador: 'Empresa', cnpj: null, admissao: '2026-01-05', saida: null, regime: 1,
      jornadaMensalHoras: 220, dependentes: 0, contaId: 2, salarioBruto: 3000 });
    expect(paraSalvar(f, false).salarioBruto).toBeUndefined();
    const edicao = formularioDe(emprego(4, null));
    expect(edicao.cnpj).toBe('12.345.678/0001-90');
    expect(edicao.salarioBruto).toBeNull();
    expect(paraSalvar({ ...edicao, jornadaMensalHoras: null, dependentes: null, contaId: null }, false))
      .toMatchObject({ jornadaMensalHoras: 220, dependentes: 0, contaId: null });
  });
});
