import { ContaAgendada, FormaCobranca, Recorrencia } from '../core/modelos';

/** #158: filtros da aba "Assinaturas e recorrentes", todos na URL (ADR 0018). */
export interface FiltroRecorrentes {
  contaId: number | null;
  forma: FormaCobranca | null;
  /** Mostrar também as encerradas ou canceladas (inativas). */
  todas: boolean;
  busca: string;
}

export const FILTRO_RECORRENTES_PADRAO: FiltroRecorrentes = { contaId: null, forma: null, todas: false, busca: '' };

/** Um grupo do de-para: uma conta ou um cartão e o que cobra nele. */
/** #242: grupos fechados da aba Assinaturas e recorrentes, neste navegador (ADR 0017): o id da conta ou "sem". */
export const CHAVE_GRUPOS_FECHADOS = 'financas.recorrentesFechados';

export function chaveGrupo(contaId: number | null): string {
  return contaId === null ? 'sem' : String(contaId);
}

export function lerGruposFechados(): Set<string> {
  try {
    const valor: unknown = JSON.parse(localStorage.getItem(CHAVE_GRUPOS_FECHADOS) ?? '[]');
    return new Set(Array.isArray(valor) ? valor.filter((v): v is string => typeof v === 'string') : []);
  } catch {
    return new Set();
  }
}

export function guardarGruposFechados(fechados: Set<string>): void {
  try {
    localStorage.setItem(CHAVE_GRUPOS_FECHADOS, JSON.stringify([...fechados]));
  } catch { /* navegador sem armazenamento: só não lembra */ }
}

export interface GrupoRecorrentes {
  contaId: number | null;
  nome: string;
  itens: ContaAgendada[];
  /** Total por mês das ativas (a anual entra dividida por 12). */
  porMes: number;
}

/** Valor por mês de uma conta recorrente: a anual entra dividida por 12; a única não é recorrente. */
export function valorPorMes(c: ContaAgendada): number {
  if (c.recorrencia === Recorrencia.Anual) return c.valor / 12;
  return c.recorrencia === Recorrencia.Mensal ? c.valor : 0;
}

/** Maiúsculas e sem acento, para a busca "onde cai…?" achar "netflix" em "NETFLIX.COM" e "agua" em "Água". */
export function normalizarBusca(texto: string | null | undefined): string {
  return (texto ?? '').normalize('NFD').replace(/[̀-ͯ]/g, '').toUpperCase();
}

/** As recorrentes (mensais e anuais) que passam nos filtros; a busca olha nome, texto da cobrança, observação e conta. */
export function filtrarRecorrentes(itens: ContaAgendada[], f: FiltroRecorrentes): ContaAgendada[] {
  const busca = normalizarBusca(f.busca.trim());
  return itens.filter((c) => c.recorrencia !== Recorrencia.Unica
    && (f.todas || c.ativa)
    && (f.contaId === null || c.contaId === f.contaId)
    && (f.forma === null || c.formaCobranca === f.forma)
    && (!busca || [c.descricao, c.textoCobranca, c.observacao, c.contaNome].some((t) => normalizarBusca(t).includes(busca))));
}

/** O de-para: um grupo por conta ou cartão, o de maior total por mês primeiro. */
export function agruparRecorrentes(itens: ContaAgendada[]): GrupoRecorrentes[] {
  const grupos = new Map<number | null, GrupoRecorrentes>();
  for (const c of itens) {
    const g = grupos.get(c.contaId)
      ?? { contaId: c.contaId, nome: c.contaNome ?? 'Sem conta ou cartão definido', itens: [], porMes: 0 };
    g.itens.push(c);
    if (c.ativa) g.porMes += valorPorMes(c);
    grupos.set(c.contaId, g);
  }
  return [...grupos.values()].sort((a, b) => b.porMes - a.porMes || a.nome.localeCompare(b.nome, 'pt-BR'));
}

/** "6 assinaturas", "3 débitos automáticos" ou "4 cobranças recorrentes", pelo que tem no grupo (Nielsen 2). */
export function rotuloQuantidade(itens: ContaAgendada[]): string {
  const n = itens.length;
  const formas = new Set(itens.map((c) => c.formaCobranca));
  if (formas.size === 1 && formas.has(FormaCobranca.CartaoCredito)) return n === 1 ? '1 assinatura' : `${n} assinaturas`;
  if (formas.size === 1 && formas.has(FormaCobranca.DebitoAutomatico)) {
    return n === 1 ? '1 débito automático' : `${n} débitos automáticos`;
  }
  return n === 1 ? '1 cobrança recorrente' : `${n} cobranças recorrentes`;
}

/** Para Contas e cartões: quantas recorrentes ativas caem em cada conta ou cartão, e quanto por mês. */
export function resumoPorConta(itens: ContaAgendada[]): Map<number, { itens: ContaAgendada[]; porMes: number }> {
  const resumo = new Map<number, { itens: ContaAgendada[]; porMes: number }>();
  for (const c of itens) {
    if (!c.ativa || c.contaId === null || c.recorrencia === Recorrencia.Unica) continue;
    const r = resumo.get(c.contaId) ?? { itens: [], porMes: 0 };
    r.itens.push(c);
    r.porMes += valorPorMes(c);
    resumo.set(c.contaId, r);
  }
  return resumo;
}
