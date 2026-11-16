import { Emprego, LancamentoFgts, MesFgts, MotivoSaqueFgts as M, RegimeEmprego, TipoLancamentoFgts as T } from '../../core/modelos';
import { empregoAtual } from './salario';

/** #216 (ADR 0066): as regras da tela Trabalho › FGTS, sem Angular (ADR 0059). A estimativa é da API. */

/** Os empregos com FGTS (CLT), os atuais primeiro (revisão do #702: o FGTS de um CLT antigo continua à mão). */
export function empregosComFgts(lista: Emprego[]): Emprego[] {
  return lista.filter((e) => e.regime === RegimeEmprego.Clt);
}

/** O emprego que a tela abre: o atual com FGTS; sem nenhum CLT, o atual (a API diz que ele não tem FGTS). */
export function empregoDoFgts(lista: Emprego[]): Emprego | null {
  return empregoAtual(empregosComFgts(lista)) ?? empregoAtual(lista);
}

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
