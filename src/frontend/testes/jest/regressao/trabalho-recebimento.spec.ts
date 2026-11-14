import { RecebimentosHolerite } from '../../../src/app/core/modelos';
import { situacaoDoRecebimento } from '../../../src/app/paginas/trabalho/recebimento';

/** #212: a situação do recebimento em palavras (a conta é da API, testada no Go). */
describe('Trabalho › recebimento do holerite (#212)', () => {
  const base: RecebimentosHolerite = { liquido: 4738.02, recebido: 0, diferenca: -4738.02, recebimentos: [], sugestoes: [], outros: [] };
  const ligado = { id: 1, data: '2026-11-05', descricao: 'X', contaId: 1, conta: 'C', valor: 1, livre: 0, valorLigado: 1 };

  it('sem cálculo, sem crédito, igual, faltando e sobrando', () => {
    expect(situacaoDoRecebimento({ ...base, liquido: null, diferenca: null }).tom).toBe('neutro');
    expect(situacaoDoRecebimento(base).texto).toBe('Nenhum crédito ligado ainda.');
    expect(situacaoDoRecebimento({ ...base, recebimentos: [ligado], diferenca: 0 })).toEqual({ texto: 'Recebido igual ao líquido estimado.', tom: 'ok' });
    const falta = situacaoDoRecebimento({ ...base, recebimentos: [ligado], diferenca: -1500 });
    expect(falta.tom).toBe('atencao');
    expect(falta.texto).toMatch(/^Faltam R\$\s1\.500,00/);
    expect(situacaoDoRecebimento({ ...base, recebimentos: [ligado], diferenca: 6000 }).texto).toMatch(/^Veio R\$\s6\.000,00 a mais/);
    expect(situacaoDoRecebimento({ ...base, recebimentos: [ligado], diferenca: null }).tom).toBe('ok');
  });
});
