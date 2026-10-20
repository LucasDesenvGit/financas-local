import { readFileSync } from 'fs';
import { join } from 'path';
import { Reserva } from '../../src/app/core/modelos';
import { InvestimentosComponent } from '../../src/app/paginas/investimentos.component';
import { podeConferirReserva } from '../../src/app/paginas/reserva-conferencia';

/**
 * Regressão do #237: desde o #168, quando a conta tinha o reservado informado pelo Open Finance, todas as reservas dela
 * vinham como "informado pelo banco" e o "conferir saldo" sumia de todas. Com o banco mandando só o total, as reservas
 * que não eram a de movimento mais recente ficavam com zero e sem como corrigir. Agora o botão só some da reserva cujo
 * valor veio, pelo nome, da lista detalhada do banco (e da linha "Reservado" com o restante do total).
 */
describe('Regressão: "conferir saldo" nas reservas que o banco não detalha (#237)', () => {
  const reserva = (campos: Partial<Reserva> = {}): Reserva => ({
    contaId: 5, nomeConta: 'Mercado Pago', nome: 'Viagem', saldo: 300, guardado: 500, retirado: 200,
    movimentos: 4, primeiroMovimento: '2026-08-01', ultimoMovimento: '2026-10-10', saldoConferido: null, dataConferencia: null,
    podeTerSaldoAnterior: false, conferenciaAntiga: false, ...campos
  });

  it('reserva estimada numa conta em que o banco informa só o total continua com "conferir saldo"', () => {
    // A API não marca mais essas reservas como "informado pelo banco": o total vira a linha "Reservado".
    expect(podeConferirReserva(reserva())).toBe(true);
    expect(podeConferirReserva(reserva({ saldoConferido: 280, dataConferencia: '2026-10-01' }))).toBe(true);
  });

  it('reserva do extrato fora da lista detalhada do banco continua com "conferir saldo"', () => {
    expect(podeConferirReserva(reserva({ foraDaListaDoBanco: true }))).toBe(true);
  });

  it('só a reserva que veio pelo nome da lista do banco, e o restante do total, ficam sem "conferir saldo"', () => {
    expect(podeConferirReserva(reserva({ informadoPeloBanco: true }))).toBe(false);
    expect(podeConferirReserva(reserva({ nome: 'Reservado', restanteDoBanco: true, totalDoBanco: 1500, movimentos: 0 }))).toBe(false);
  });

  it('a tela decide o botão pela mesma regra, não mais só por "informado pelo banco"', () => {
    const fonte = readFileSync(join(__dirname, '../../src/app/paginas/investimentos.component.ts'), 'utf-8');
    expect(fonte).toContain('@if (podeConferir(r))');
    expect(fonte).not.toContain('@if (!r.informadoPeloBanco)');
    const tela = {} as InvestimentosComponent;
    expect(InvestimentosComponent.prototype.podeConferir.call(tela, reserva())).toBe(true);
    expect(InvestimentosComponent.prototype.podeConferir.call(tela, reserva({ informadoPeloBanco: true }))).toBe(false);
  });
});
