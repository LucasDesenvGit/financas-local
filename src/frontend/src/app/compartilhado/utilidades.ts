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
  /** #141: transação que /transacoes destaca na lista (rola até ela e põe o foco nela). */
  transacaoId?: number;
  /** Natureza do gasto da regra 50/30/20 (Visão anual); 'sem' = sem classificação. */
  naturezas?: NaturezaUrl[];
  /** #173 d): só estes lançamentos (até {@link MAXIMO_IDS_URL}), os que formam um número de outra tela. */
  ids?: number[];
  /** De onde vêm os `ids`, para o chip de /transacoes dizer o que a lista é. */
  origemIds?: OrigemIdsUrl;
}

/** #173 d): valores de ?de= em /transacoes, junto com ?ids=. */
export type OrigemIdsUrl = 'fii-aportes' | 'fii-rendimentos' | 'trocos-automaticos';

/** O que o chip de /transacoes diz de cada origem dos `ids` (Nielsen 2: as palavras da tela de onde se veio). */
export const ORIGENS_IDS: Record<OrigemIdsUrl, string> = {
  'fii-aportes': 'Compras e vendas de FIIs',
  'fii-rendimentos': 'Rendimentos de FIIs',
  'trocos-automaticos': 'Trocos classificados na sincronização'
};

/** Máximo de lançamentos em ?ids= (o mesmo limite da API), para a URL não crescer sem fim. */
export const MAXIMO_IDS_URL = 200;

/** Valores de ?natureza= em /transacoes (separados por vírgula). */
export type NaturezaUrl = 'essencial' | 'estilo-de-vida' | 'sem';

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
  if (c.transacaoId) p['transacao'] = String(c.transacaoId);
  if (c.naturezas?.length) p['natureza'] = c.naturezas.join(',');
  if (c.ids?.length) {
    p['ids'] = c.ids.slice(0, MAXIMO_IDS_URL).join(',');
    if (c.origemIds) p['de'] = c.origemIds;
  }
  return p;
}

/** Os ids de ?ids= (separados por vírgula): só inteiros positivos, sem repetir, até {@link MAXIMO_IDS_URL}. */
export function idsDaUrl(valor: string | null): number[] {
  const ids = (valor ?? '').split(',').map((v) => Number(v.trim())).filter((n) => Number.isSafeInteger(n) && n > 0);
  return [...new Set(ids)].slice(0, MAXIMO_IDS_URL);
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

/** #316: o nome de cada cor sugerida, para o leitor de tela (Nielsen 2: "Cor lilás", e não "Cor #a7a3f5"). */
export const NOMES_CORES: Record<string, string> = {
  '#a7a3f5': 'lilás', '#92c8f0': 'azul', '#8fd5b4': 'verde', '#8fd8d8': 'turquesa', '#b5cf9a': 'verde-oliva',
  '#f2d98b': 'amarelo', '#f8c09a': 'laranja', '#f4a89a': 'salmão', '#f5a3b0': 'rosa', '#f3b3d3': 'rosa-claro',
  '#d4a5e8': 'roxo', '#a9b4c6': 'cinza-azulado'
};

/** Valor em reais para frases-resumo; com "Ocultar valores" ligado devolve null (a frase usa só percentuais). */
export function reaisOuNada(valor: number, oculto: boolean): string | null {
  return oculto ? null : valor.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL', maximumFractionDigits: 0 });
}

/** Percentual curto para frases-resumo: 12,3 → "12%", 0,4 → "0,4%". */
export function pct(valor: number): string {
  const abs = Math.abs(valor);
  return `${abs.toLocaleString('pt-BR', { maximumFractionDigits: abs < 10 ? 1 : 0 })}%`;
}
