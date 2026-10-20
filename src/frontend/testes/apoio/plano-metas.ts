import { MesCaixa, Meta, PlanoMetas } from '../../src/app/core/modelos';
import { meta } from './metas';

/** Três metas em aberto, na ordem de prioridade da API: Viagem (prazo), Emergência (sem prazo), Carro (prazo apertado). */
export const METAS: Meta[] = [
  meta({ id: 1, nome: 'Viagem', dataAlvo: '2027-12-01', reservaNome: null, saldoReserva: null, prioridade: 1, aporteMensalSugerido: 785.71 }),
  meta({ id: 2, nome: 'Emergência', dataAlvo: null, reservaNome: null, saldoReserva: null, prioridade: 2 }),
  meta({ id: 3, nome: 'Carro', dataAlvo: '2027-06-01', reservaNome: null, saldoReserva: null, prioridade: 3, aporteMensalSugerido: 7500 })
];

export interface OpcoesCaixa {
  receitas?: number;
  despesas?: number;
  /** Fatura de cada mês a partir de out/2026; a última vale para o resto. */
  faturas?: number[];
}

/**
 * O caixa que a API devolve (#149), de out/2026 em diante, 120 meses: receitas e despesas fixas e as faturas que vencem
 * em cada mês. Padrão: R$ 10.000 − R$ 4.000 − fatura de R$ 2.500 = sobra de R$ 3.500 todo mês.
 */
export function plano(campos: Partial<PlanoMetas> = {}, opcoes: OpcoesCaixa = {}): PlanoMetas {
  const receitas = opcoes.receitas ?? 10000;
  const despesas = opcoes.despesas ?? 4000;
  const faturas = opcoes.faturas ?? [2500];
  const meses: MesCaixa[] = Array.from({ length: 120 }, (_, k) => {
    const f = faturas[Math.min(k, faturas.length - 1)];
    const total = 2026 * 12 + 9 + k;
    return { ano: Math.floor(total / 12), mes: (total % 12) + 1, receitas, despesas, faturas: f, parcelas: 0, sobra: receitas - despesas - f };
  });
  return {
    disponivel: true, mesesConsiderados: 6, ano: 2026, mes: 10, margemPercentual: 10, saldoContas: 9000, faturaAPagar: 0,
    saldoFimDoMes: 9000, melhorMesRecente: 5000, meses, ...campos
  };
}
