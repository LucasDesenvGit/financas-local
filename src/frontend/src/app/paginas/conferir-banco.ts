import { PistaConferencia } from '../core/modelos';

/** #172: pista marcada quando tem o mesmo valor da diferença (até 1 centavo): é a explicação mais provável. */
export interface PistaComparada extends PistaConferencia {
  explica: boolean;
}

export interface Comparacao {
  /** Valor do app − valor do site (positivo: o app mostra mais). */
  diferenca: number;
  bate: boolean;
  /** As pistas, com as que explicam a diferença primeiro. */
  pistas: PistaComparada[];
}

/** Compara o valor digitado (do app do banco) com o do site; null enquanto nada foi digitado. */
export function comparar(site: number, app: number | null | undefined, pistas: PistaConferencia[]): Comparacao | null {
  if (app === null || app === undefined || Number.isNaN(app)) return null;
  const diferenca = Math.round((app - site) * 100) / 100;
  const bate = Math.abs(diferenca) < 0.01;
  const marcadas = pistas.map((p) => ({
    ...p,
    explica: !bate && p.valor != null && Math.abs(Math.abs(diferenca) - p.valor) < 0.015
  }));
  return { diferenca, bate, pistas: [...marcadas.filter((p) => p.explica), ...marcadas.filter((p) => !p.explica)] };
}
