import { FaturaMes } from '../core/modelos';

/**
 * #322: os cartões cuja fatura em aberto aparece no Painel, em "Situação hoje". A escolha é feita em Contas e cartões
 * (interruptor em cada cartão) e fica neste navegador, como as outras preferências de exibição (ADR 0017). Sem
 * armazenamento (modo privado, bloqueado), nenhum cartão aparece e a tela funciona como antes.
 */
export const CHAVE_FATURAS_NO_PAINEL = 'financas.painel.faturasAbertas';

export function lerCartoesNoPainel(): number[] {
  try {
    const salvo: unknown = JSON.parse(localStorage.getItem(CHAVE_FATURAS_NO_PAINEL) ?? '[]');
    return Array.isArray(salvo) ? salvo.filter((id): id is number => Number.isInteger(id)) : [];
  } catch {
    return [];
  }
}

export function gravarCartoesNoPainel(ids: readonly number[]): void {
  try {
    localStorage.setItem(CHAVE_FATURAS_NO_PAINEL, JSON.stringify([...new Set(ids)]));
  } catch {
    /* sem armazenamento: só não lembra */
  }
}

/**
 * A fatura que ainda recebe compras hoje: a do mês, enquanto não fecha (ou sem fechamento conhecido, quando o valor é o
 * do mês do calendário); depois que ela fecha, a próxima (#251). Null quando a do mês fechou e a próxima não veio.
 */
export function faturaEmAberto(f: FaturaMes | null | undefined): FaturaMes | null {
  if (!f) return null;
  if (f.situacao === null || f.situacao === 'aberta') return f;
  return f.proxima ?? null;
}
