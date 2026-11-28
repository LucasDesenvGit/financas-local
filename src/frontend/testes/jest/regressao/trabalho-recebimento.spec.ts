import { RecebimentosHolerite } from '../../../src/app/core/modelos';
import { esperadoEmPartes, situacaoDoRecebimento } from '../../../src/app/paginas/trabalho/recebimento';

/** #212: a situação do recebimento em palavras (a conta é da API, testada no Go). */
describe('Trabalho › recebimento do holerite (#212)', () => {
  const base: RecebimentosHolerite = { liquido: 4738.02, adiantamento: null, esperado: 4738.02, recebido: 0, diferenca: -4738.02, recebimentos: [], sugestoes: [], outros: [] };
  const ligado = { id: 1, data: '2026-11-05', descricao: 'X', contaId: 1, conta: 'C', valor: 1, livre: 0, valorLigado: 1, sumiu: false, debito: false, valorMudou: false };

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

  it('#768: com o adiantamento descontado, o esperado vem em partes e o texto diz "esperado"', () => {
    const com = { ...base, liquido: 3723.75, adiantamento: 3200, esperado: 6923.75, recebido: 3723.75, diferenca: -3200, recebimentos: [ligado] };
    expect(esperadoEmPartes(com)).toMatch(/^adiantamento R\$\s3\.200,00 \+ saldo R\$\s3\.723,75 = R\$\s6\.923,75$/);
    expect(situacaoDoRecebimento(com).texto).toMatch(/^Faltam R\$\s3\.200,00 para o esperado/);
    expect(situacaoDoRecebimento({ ...com, diferenca: 0 }).texto).toBe('Recebido igual ao esperado.');
    expect(esperadoEmPartes(base)).toBeNull(); // sem a linha do adiantamento nada muda
    expect(esperadoEmPartes({ ...com, liquido: null })).toBeNull();
  });
});
