import { LancamentoFgts, MesFgts, MotivoSaqueFgts as M, TipoLancamentoFgts as T } from '../../core/modelos';

/** #216 (ADR 0066): as regras da tela Trabalho › FGTS, sem Angular (ADR 0059). A estimativa é da API. */

export const TIPOS: { valor: T; rotulo: string }[] = [
  { valor: T.Conferencia, rotulo: 'Saldo que vi no app do FGTS' },
  { valor: T.Saque, rotulo: 'Saque' },
  { valor: T.Distribuicao, rotulo: 'Distribuição de resultados' }
];

export const MOTIVOS: { valor: M; rotulo: string }[] = [
  { valor: M.Rescisao, rotulo: 'Rescisão' },
  { valor: M.SaqueAniversario, rotulo: 'Saque-aniversário' },
  { valor: M.Imovel, rotulo: 'Compra de imóvel' },
  { valor: M.Outro, rotulo: 'Outro' }
];

export function descreverLancamento(l: LancamentoFgts): string {
  const tipo = TIPOS.find((t) => t.valor === l.tipo)?.rotulo ?? '—';
  const motivo = l.motivo ? MOTIVOS.find((m) => m.valor === l.motivo)?.rotulo : null;
  return motivo ? `${tipo} (${motivo.toLowerCase()})` : tipo;
}

/** Os últimos n meses, do mais novo para o mais antigo (a API manda do mais antigo). */
export function ultimosMeses(meses: MesFgts[], n = 12): MesFgts[] {
  return meses.slice(-n).reverse();
}

/** O que entrou de depósito no total. */
export function totalDepositado(meses: MesFgts[]): number {
  return Math.round(meses.reduce((s, m) => s + m.deposito, 0) * 100) / 100;
}
