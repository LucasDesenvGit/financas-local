import { Meta, PlanoMetas } from '../../src/app/core/modelos';
import { meta } from './metas';

/** Três metas em aberto, na ordem de prioridade da API: Viagem (prazo), Emergência (sem prazo), Carro (prazo apertado). */
export const METAS: Meta[] = [
  meta({ id: 1, nome: 'Viagem', dataAlvo: '2027-12-01', reservaNome: null, saldoReserva: null, prioridade: 1, aporteMensalSugerido: 785.71 }),
  meta({ id: 2, nome: 'Emergência', dataAlvo: null, reservaNome: null, saldoReserva: null, prioridade: 2 }),
  meta({ id: 3, nome: 'Carro', dataAlvo: '2027-06-01', reservaNome: null, saldoReserva: null, prioridade: 3, aporteMensalSugerido: 7500 })
];

export function plano(campos: Partial<PlanoMetas> = {}): PlanoMetas {
  return {
    disponivel: true, mesesConsiderados: 6, ano: 2026, mes: 10, sobraPrevista: 3527.94, margemPercentual: 10,
    capacidade: 3175.15, totalPedido: 7169.8, totalRecomendado: 3175.14, sobraNegativa: false,
    metas: [
      { metaId: 1, falta: 11000, recomendado: 706.92, pedidoMes: 706.92, divisaoIgual: 785.71, dataProvavel: '2027-12-01',
        noPrazo: true, paraManterPrazo: null,
        plano: [{ ano: 2026, mes: 10, sobraPrevista: 3527.94, parcelas: 0, recomendado: 706.92 },
                { ano: 2026, mes: 11, sobraPrevista: 3590.59, parcelas: 100, recomendado: 719.48 }] },
      { metaId: 2, falta: 8000, recomendado: 1058.38, pedidoMes: null, divisaoIgual: null, dataProvavel: '2027-05-01',
        noPrazo: null, paraManterPrazo: null, plano: [] },
      { metaId: 3, falta: 60000, recomendado: 1409.84, pedidoMes: 6462.88, divisaoIgual: 7500, dataProvavel: '2028-09-01',
        noPrazo: false, paraManterPrazo: 7323.77, plano: [] }
    ],
    ...campos
  };
}
