import { signal } from '@angular/core';
import { of } from 'rxjs';
import { Reserva } from '../../src/app/core/modelos';
import { InvestimentosComponent } from '../../src/app/paginas/investimentos.component';
import { conferenciaForaDoEsperado, valorEsperadoDaReserva } from '../../src/app/paginas/reserva-conferencia';

/**
 * #169: o Méli Dólar não aparecia em lugar nenhum. Agora é uma reserva em dólar: a quantidade (e a conferência) em US$,
 * o saldo em reais pela PTAX. A conferência pode ser informada em reais ou em dólares.
 */
describe('Regressão: Méli Dólar como reserva em dólar (#169)', () => {
  const reserva = (campos: Partial<Reserva> = {}): Reserva => ({
    contaId: 1, nomeConta: 'Mercado Pago', nome: 'Méli Dólar', saldo: 209.6, guardado: 761.36, retirado: 562.08,
    movimentos: 6, primeiroMovimento: '2026-04-29', ultimoMovimento: '2026-10-17', saldoConferido: null, dataConferencia: null,
    podeTerSaldoAnterior: false, conferenciaAntiga: false, moeda: 'USD', quantidade: 41.46, cotacao: 5.0558, ...campos
  });

  function salvar(r: Reserva, saldo: number, moeda: 'BRL' | 'USD') {
    const pedidos: unknown[][] = [];
    const tela = {
      formConferencia: { reserva: r, saldo, data: '2026-10-19', moeda },
      salvando: signal(false),
      conferenciaPlausivel: InvestimentosComponent.prototype.conferenciaPlausivel,
      api: { conferirReserva: (...args: unknown[]) => { pedidos.push(args); return of(undefined); } },
      avisos: { sucesso: jest.fn(), erro: jest.fn() },
      recarregarDepoisDeAlterar: jest.fn()
    };
    InvestimentosComponent.prototype.salvarConferencia.call(tela);
    return pedidos;
  }

  afterEach(() => jest.restoreAllMocks());

  it('a conferência do Méli Dólar abre em reais, com a estimativa atual', () => {
    const tela = { hoje: '2026-10-19', formConferencia: null as unknown };
    InvestimentosComponent.prototype.abrirConferencia.call(tela, reserva({ saldoConferido: 10.4 }));
    expect(tela.formConferencia).toMatchObject({ saldo: 209.6, moeda: 'BRL' });
  });

  it('manda a moeda informada só para o Méli Dólar', () => {
    expect(salvar(reserva(), 52.71, 'BRL')).toEqual([[1, 'Méli Dólar', 52.71, '2026-10-19', 'BRL']]);
    expect(salvar(reserva(), 10, 'USD')).toEqual([[1, 'Méli Dólar', 10, '2026-10-19', 'USD']]);
    const emReais = reserva({ nome: 'Viagem', moeda: 'BRL', quantidade: null, cotacao: null, saldo: 1200, guardado: 1300 });
    expect(salvar(emReais, 1250, 'BRL')).toEqual([[1, 'Viagem', 1250, '2026-10-19', undefined]]);
  });

  it('em dólar, a comparação com o esperado é feita em reais (pela PTAX)', () => {
    const confirmar = jest.spyOn(window, 'confirm').mockReturnValue(false);
    // US$ 41 ≈ R$ 207: perto do esperado, salva sem perguntar.
    expect(salvar(reserva(), 41, 'USD')).toHaveLength(1);
    expect(confirmar).not.toHaveBeenCalled();
    // US$ 4.100 ≈ R$ 20,7 mil: mais de 10× o esperado, pergunta antes (e não salva com "Cancelar").
    expect(salvar(reserva(), 4100, 'USD')).toHaveLength(0);
    expect(confirmar).toHaveBeenCalled();
  });

  it('a conferência guardada em dólar entra no esperado convertida para reais', () => {
    const r = reserva({ saldo: 0, guardado: 0, retirado: 0, saldoConferido: 10, cotacao: 5 });
    expect(valorEsperadoDaReserva(r)).toBe(50);
    expect(conferenciaForaDoEsperado(r, 52.71)).toBe(false);
  });
});
