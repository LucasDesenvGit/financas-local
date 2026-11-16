import { PeriodoFerias } from '../../core/modelos';

/** #213: as regras da tela Trabalho › Férias, sem Angular (ADR 0059). As regras da CLT e o cálculo são da API. */

/** A situação do período em texto (Nielsen 1: o estado em palavras, não só em cor). */
export function situacaoDoPeriodo(p: PeriodoFerias): { texto: string; tom: 'erro' | 'atencao' | 'neutro' | 'ok' } {
  if (!p.completo) return { texto: 'Em aquisição', tom: 'neutro' };
  if (p.vencida) return { texto: 'Vencida', tom: 'erro' };
  if (p.vencendo) return { texto: 'Vencendo', tom: 'atencao' };
  if (p.saldo === 0) return { texto: 'Agendada', tom: 'ok' };
  return { texto: 'Para tirar', tom: 'neutro' };
}

/** O período em que dá para agendar: o mais antigo com saldo (completo primeiro), ou null. */
export function periodoParaAgendar(lista: PeriodoFerias[]): PeriodoFerias | null {
  return lista.find((p) => p.saldo > 0 && p.completo) ?? lista.find((p) => p.saldo > 0) ?? null;
}

/** O dia seguinte ao fim do período (aaaa-mm-dd): o primeiro dia em que as férias dele podem começar. */
export function primeiroDiaPossivel(p: PeriodoFerias): string {
  const d = new Date(`${p.fim}T12:00:00Z`);
  d.setUTCDate(d.getUTCDate() + 1);
  return d.toISOString().slice(0, 10);
}

/** O "tirar tudo × vender 1/3" pelo direito do período (revisão do #701): 24 de direito dá 24 × 16 e vender 8. */
export function opcoesDaVenda(direito: number): { tirar: number; vendendo: number; vender: number } {
  const vender = Math.floor(direito / 3);
  return { tirar: direito, vendendo: direito - vender, vender };
}

/** As faltas digitadas: um número inteiro de 0 a 366, ou null (campo vazio ou errado não grava zero). */
export function faltasDigitadas(valor: number | string | null): number | null {
  if (valor === null || valor === '') return null;
  const n = Number(valor);
  return Number.isInteger(n) && n >= 0 && n <= 366 ? n : null;
}

/** Quanto a venda muda o que cai na conta: positivo, vender dá mais dinheiro agora. */
export function diferencaDaVenda(trinta: number, vendendo: number): number {
  return Math.round((vendendo - trinta) * 100) / 100;
}
