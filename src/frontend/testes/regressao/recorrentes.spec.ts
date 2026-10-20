import { FormaCobranca, Recorrencia } from '../../src/app/core/modelos';
import {
  FILTRO_RECORRENTES_PADRAO, agruparRecorrentes, filtrarRecorrentes, resumoPorConta, rotuloQuantidade, valorPorMes
} from '../../src/app/paginas/recorrentes';
import { contaAgendada } from '../apoio/contas-agendadas';

/**
 * #158: o de-para "Assinaturas e recorrentes": cada cobrança que se repete no cartão ou na conta em que cai, com o total
 * por mês (a anual dividida por 12), a busca "onde cai…?" sem acento nem maiúsculas e o resumo por conta.
 */
describe('Regressão: Assinaturas e recorrentes (#158)', () => {
  const netflix = contaAgendada({ id: 1, descricao: 'Netflix', valor: 44.9, contaId: 4, contaNome: 'Nubank',
    formaCobranca: FormaCobranca.CartaoCredito, textoCobranca: 'NETFLIX' });
  const prime = contaAgendada({ id: 2, descricao: 'Amazon Prime', valor: 120, recorrencia: Recorrencia.Anual, contaId: 4,
    contaNome: 'Nubank', formaCobranca: FormaCobranca.CartaoCredito });
  const agua = contaAgendada({ id: 3, descricao: 'Água', valor: 90, contaId: 1, contaNome: 'Itaú',
    formaCobranca: FormaCobranca.DebitoAutomatico });
  const escolaUnica = contaAgendada({ id: 4, descricao: 'Matrícula', valor: 500, recorrencia: Recorrencia.Unica });
  const antiga = contaAgendada({ id: 5, descricao: 'Revista', valor: 30, contaId: 4, contaNome: 'Nubank', ativa: false,
    formaCobranca: FormaCobranca.CartaoCredito });
  const todas = [netflix, prime, agua, escolaUnica, antiga];

  it('a anual entra por mês dividida por 12; a única não é recorrente', () => {
    expect(valorPorMes(prime)).toBe(10);
    expect(valorPorMes(escolaUnica)).toBe(0);
    expect(filtrarRecorrentes(todas, FILTRO_RECORRENTES_PADRAO).map((c) => c.id)).toEqual([1, 2, 3]);
  });

  it('agrupa por cartão ou conta, o maior total por mês primeiro', () => {
    const grupos = agruparRecorrentes(filtrarRecorrentes(todas, FILTRO_RECORRENTES_PADRAO));
    expect(grupos.map((g) => [g.nome, g.porMes])).toEqual([['Itaú', 90], ['Nubank', 54.9]]);
    expect(rotuloQuantidade(grupos[1].itens)).toBe('2 assinaturas');
    expect(rotuloQuantidade(grupos[0].itens)).toBe('1 débito automático');
  });

  it('"onde cai…?" acha sem acento e sem diferenciar maiúsculas, também pelo texto da cobrança', () => {
    const busca = (b: string) => filtrarRecorrentes(todas, { ...FILTRO_RECORRENTES_PADRAO, busca: b }).map((c) => c.id);
    expect(busca('netflix')).toEqual([1]);
    expect(busca('agua')).toEqual([3]);
    expect(busca('nubank')).toEqual([1, 2]);
  });

  it('as encerradas só aparecem com "Mostrar as encerradas" e não somam no total', () => {
    const com = filtrarRecorrentes(todas, { ...FILTRO_RECORRENTES_PADRAO, todas: true });
    expect(com.map((c) => c.id)).toContain(5);
    expect(agruparRecorrentes(com).find((g) => g.nome === 'Nubank')!.porMes).toBeCloseTo(54.9);
  });

  it('o resumo por conta (Contas e cartões) conta só as ativas e recorrentes', () => {
    const r = resumoPorConta(todas);
    expect(r.get(4)).toMatchObject({ porMes: 54.9 });
    expect(r.get(4)!.itens.map((c) => c.id)).toEqual([1, 2]);
    expect(r.get(1)!.porMes).toBe(90);
  });
});
