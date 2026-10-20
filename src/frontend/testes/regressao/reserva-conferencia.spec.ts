import { Reserva } from '../../src/app/core/modelos';
import { conferenciaForaDoEsperado, valorEsperadoDaReserva } from '../../src/app/paginas/reserva-conferencia';

/**
 * Regressão do #168: a conferência "10.123,42" foi gravada como 10,12 (campo numérico do navegador lendo o ponto de
 * milhar como decimal) e o patrimônio não mudou. Valor muito fora do esperado agora pede confirmação.
 */
describe('Regressão: conferência de reserva fora do esperado', () => {
  const reserva = (campos: Partial<Reserva>): Reserva => ({
    contaId: 5, nomeConta: 'Mercado Pago', nome: 'Reserva de emergencia', saldo: 0, guardado: 0, retirado: 0,
    movimentos: 0, primeiroMovimento: null, ultimoMovimento: null, saldoConferido: null, dataConferencia: null,
    podeTerSaldoAnterior: false, conferenciaAntiga: false, ...campos
  });

  it('reserva com dinheiro de antes do histórico (estimativa zero): o esperado é o que passou por ela', () => {
    // Caso real: retiradas na janela maiores que os depósitos, estimativa 0, mas R$ 10,4 mil guardados no período.
    const r = reserva({ saldo: 0, guardado: 10427.66, retirado: 11951.1 });
    expect(valorEsperadoDaReserva(r)).toBe(10427.66);
    expect(conferenciaForaDoEsperado(r, 10.12)).toBe(true);
    expect(conferenciaForaDoEsperado(r, 10123.42)).toBe(false);
  });

  it('valor 10× maior que o esperado (ex.: digitou sem a vírgula) também pede confirmação', () => {
    const r = reserva({ saldo: 2000, guardado: 2000 });
    expect(conferenciaForaDoEsperado(r, 200000)).toBe(true);
    expect(conferenciaForaDoEsperado(r, 2350.5)).toBe(false);
  });

  it('reserva sem nenhum histórico não tem como comparar: não pede confirmação', () => {
    expect(conferenciaForaDoEsperado(reserva({}), 10.12)).toBe(false);
  });
});
