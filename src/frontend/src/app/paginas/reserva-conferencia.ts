import { Reserva } from '../core/modelos';

/**
 * #168: valor esperado para conferir uma reserva. É o maior entre a estimativa, a última conferência e o que entrou −
 * o que saiu. Quando tudo isso dá zero ou menos (a reserva já tinha dinheiro antes do histórico importado), vale o que
 * passou por ela (o total guardado).
 */
export function valorEsperadoDaReserva(r: Reserva): number {
  const esperado = Math.max(r.saldo, r.saldoConferido ?? 0, r.guardado - r.retirado);
  return esperado > 0 ? esperado : r.guardado;
}

/**
 * Conferência muito fora do esperado: menos de 1/10 ou mais de 10× o valor esperado. É o caso do "10.123,42" lido como
 * 10,12 por um campo que não entendia o ponto de milhar (Nielsen 5: prevenir o erro antes de salvar).
 */
export function conferenciaForaDoEsperado(r: Reserva, saldo: number): boolean {
  const esperado = valorEsperadoDaReserva(r);
  return esperado > 0 && (saldo < esperado / 10 || saldo > esperado * 10);
}
