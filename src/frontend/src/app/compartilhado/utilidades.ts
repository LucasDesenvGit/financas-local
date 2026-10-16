export const NOMES_MESES = [
  'janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho',
  'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro'
];

export const NOMES_MESES_CURTOS = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];

/** Data local no formato yyyy-MM-dd (sem conversão para UTC). */
export function dataIso(d: Date = new Date()): string {
  const ano = d.getFullYear();
  const mes = String(d.getMonth() + 1).padStart(2, '0');
  const dia = String(d.getDate()).padStart(2, '0');
  return `${ano}-${mes}-${dia}`;
}

export function primeiroDiaMes(ano: number, mes: number): string {
  return `${ano}-${String(mes).padStart(2, '0')}-01`;
}

export function ultimoDiaMes(ano: number, mes: number): string {
  const ultimo = new Date(ano, mes, 0).getDate();
  return `${ano}-${String(mes).padStart(2, '0')}-${String(ultimo).padStart(2, '0')}`;
}

/** Converte "yyyy-MM-dd" em Date local (evita o deslocamento de fuso do construtor com string). */
export function paraData(iso: string): Date {
  const [a, m, d] = iso.substring(0, 10).split('-').map(Number);
  return new Date(a, m - 1, d);
}

/** Data "yyyy-MM-dd" válida (formato e dia existente no calendário). */
export function dataIsoValida(valor: string | null | undefined): valor is string {
  if (!valor || !/^\d{4}-\d{2}-\d{2}$/.test(valor)) return false;
  const d = paraData(valor);
  return !isNaN(d.getTime()) && dataIso(d) === valor;
}

/** Contexto que uma tela de resumo leva para /transacoes (categoria + período + tipo). */
export interface ContextoTransacoes {
  /** Id da categoria; null = "Sem categoria". Omitido = todas. */
  categoriaId?: number | null;
  inicio?: string;
  fim?: string;
  tipo?: 'despesa' | 'receita' | 'transferencia';
  contaId?: number;
}

/**
 * Query params de /transacoes (UX-1). Use sempre esta função para montar o link, em vez de string na mão:
 * `<a routerLink="/transacoes" [queryParams]="paramsTransacoes({ categoriaId, inicio, fim, tipo: 'despesa' })">`.
 */
export function paramsTransacoes(c: ContextoTransacoes): Record<string, string> {
  const p: Record<string, string> = {};
  if (c.categoriaId !== undefined) p['categoria'] = c.categoriaId === null ? 'sem' : String(c.categoriaId);
  if (c.inicio) p['inicio'] = c.inicio;
  if (c.fim) p['fim'] = c.fim;
  if (c.tipo) p['tipo'] = c.tipo;
  if (c.contaId) p['conta'] = String(c.contaId);
  return p;
}

/** Mês/ano vindo da URL (?ano=2026&mes=9), para o voltar do navegador reabrir a tela no mesmo período. */
export function mesAnoDaUrl(params: { get(nome: string): string | null }): { ano: number; mes: number } | null {
  const ano = Number(params.get('ano'));
  const mes = Number(params.get('mes'));
  return Number.isInteger(ano) && ano >= 2000 && ano <= 2100 && Number.isInteger(mes) && mes >= 1 && mes <= 12
    ? { ano, mes } : null;
}

/**
 * Tempo relativo curto, em português: "agora há pouco", "há 5 min", "há 2 h" (mesmo dia), "ontem",
 * "há 3 dias" (até uma semana) e "em 02/10" depois disso.
 */
export function tempoRelativo(quando: Date, agora: Date = new Date()): string {
  const minutos = Math.floor((agora.getTime() - quando.getTime()) / 60000);
  if (minutos < 1) return 'agora há pouco';
  const inicioHoje = new Date(agora.getFullYear(), agora.getMonth(), agora.getDate());
  if (quando >= inicioHoje) return minutos < 60 ? `há ${minutos} min` : `há ${Math.floor(minutos / 60)} h`;
  const inicioOntem = new Date(inicioHoje.getFullYear(), inicioHoje.getMonth(), inicioHoje.getDate() - 1);
  if (quando >= inicioOntem) return 'ontem';
  const dias = Math.round((inicioHoje.getTime() - new Date(quando.getFullYear(), quando.getMonth(), quando.getDate()).getTime()) / 86400000);
  if (dias <= 7) return `há ${dias} dias`;
  return `em ${String(quando.getDate()).padStart(2, '0')}/${String(quando.getMonth() + 1).padStart(2, '0')}`;
}

export const CORES_SUGERIDAS = [
  '#a7a3f5', '#92c8f0', '#8fd5b4', '#8fd8d8', '#b5cf9a', '#f2d98b',
  '#f8c09a', '#f4a89a', '#f5a3b0', '#f3b3d3', '#d4a5e8', '#a9b4c6'
];
