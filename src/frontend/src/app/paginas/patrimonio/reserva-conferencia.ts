import { Reserva } from '../../core/modelos';

/**
 * #168: valor esperado para conferir uma reserva. É o maior entre a estimativa, a última conferência e o que entrou −
 * o que saiu. Quando tudo isso dá zero ou menos (a reserva já tinha dinheiro antes do histórico importado), vale o que
 * passou por ela (o total guardado).
 */
export function valorEsperadoDaReserva(r: Reserva): number {
  // #169: no Méli Dólar a conferência é em dólar; aqui tudo é comparado em reais.
  const conferido = (r.saldoConferido ?? 0) * (r.moeda === 'USD' ? (r.cotacao ?? 0) : 1);
  const esperado = Math.max(r.saldo, conferido, r.guardado - r.retirado);
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

/**
 * #237: "conferir saldo" fica em toda reserva cujo valor não veio, pelo nome, da lista detalhada do banco. A linha com
 * o restante do total informado pelo banco também não se confere: ela é o total menos as outras reservas da conta.
 */
export function podeConferirReserva(r: Reserva): boolean {
  return !r.informadoPeloBanco && !r.restanteDoBanco;
}

/**
 * #691 (ADR 0063): a diferença que a conferência pode registrar como gasto do cofrinho: o que a estimativa passa do
 * saldo informado, numa reserva em reais e conferível, com a data de hoje (a estimativa de agora é a do dia). 0 quando
 * não cabe (aí a opção nem aparece).
 */
export function diferencaComoGasto(r: Reserva, saldo: number | null, data: string, hoje: string): number {
  if (saldo === null || data !== hoje || r.moeda === 'USD' || !podeConferirReserva(r)) return 0;
  const diferenca = Math.round((r.saldo - saldo) * 100) / 100;
  return diferenca > 0 ? diferenca : 0;
}

/** #691: "registrar saída" vale em toda reserva em reais, menos na linha do restante do total do banco. */
export function podeRegistrarSaida(r: Reserva): boolean {
  return r.moeda !== 'USD' && !r.restanteDoBanco;
}
