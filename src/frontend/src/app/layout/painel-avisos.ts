import { Orcamento } from '../core/modelos';

/**
 * #206: quando os avisos do Painel ficam ocultos e quando voltam. Tudo neste navegador (localStorage), como as outras
 * preferências de exibição (ADR 0017): ocultar é "já vi", não muda dado nenhum.
 */

/** Quantos orçamentos estourados aparecem antes do "Ver todos" (os piores; Progressive Disclosure). */
export const ORCAMENTOS_VISIVEIS = 3;
export const CHAVE_ORCAMENTOS_OCULTOS = 'financas.painel.orcamentosOcultos';
export const CHAVE_AVISOS_OCULTOS = 'financas.painel.avisosOcultos';
/** "Minimizar": recolhido até a pessoa clicar em "Mostrar", sem voltar sozinho (um por card). */
export const CHAVE_ORCAMENTOS_MINIMIZADOS = 'financas.painel.orcamentosMinimizados';
export const CHAVE_AVISOS_MINIMIZADOS = 'financas.painel.avisosMinimizados';
/** Meses lembrados (o Painel abre outros meses pelo seletor; o resto é descartado). */
const PERIODOS_LEMBRADOS = 6;

/** Faixas de quanto o gasto passou do limite: acima de 100%, de 120%, de 150%, de 200% e de 300%. */
const FAIXAS = [120, 150, 200, 300];

/**
 * Nível de um orçamento estourado: a faixa pesa mais; na mesma faixa, estourar de verdade é pior do que passar do
 * limite só por emergência. O aviso oculto volta quando algum orçamento passa a um nível acima do que estava.
 */
export function nivelOrcamento(o: Orcamento): number {
  const faixa = 1 + FAIXAS.filter((f) => o.percentual >= f).length;
  return faixa * 2 + (o.situacao === 'emergencia' ? 0 : 1);
}

/** Mês do Painel em que os avisos foram ocultos → nível de cada categoria naquele momento. */
export type OrcamentosOcultos = Record<string, Record<string, number>>;

export function chavePeriodo(p: { ano: number; mes: number }): string {
  return `${p.ano}-${String(p.mes).padStart(2, '0')}`;
}

/** Continua oculto se o mês é o mesmo e nenhum orçamento estourou de novo nem piorou de faixa. */
export function orcamentosContinuamOcultos(salvo: OrcamentosOcultos, periodo: string, estourados: Orcamento[]): boolean {
  const niveis = salvo[periodo];
  if (!niveis || !estourados.length) return false;
  return estourados.every((o) => (niveis[String(o.categoriaId)] ?? -1) >= nivelOrcamento(o));
}

export function ocultarOrcamentos(salvo: OrcamentosOcultos, periodo: string, estourados: Orcamento[]): OrcamentosOcultos {
  const novo: OrcamentosOcultos = {
    ...salvo,
    [periodo]: Object.fromEntries(estourados.map((o) => [String(o.categoriaId), nivelOrcamento(o)]))
  };
  return Object.fromEntries(Object.keys(novo).sort().slice(-PERIODOS_LEMBRADOS).map((k) => [k, novo[k]]));
}

export function mostrarOrcamentos(salvo: OrcamentosOcultos, periodo: string): OrcamentosOcultos {
  const { [periodo]: _, ...resto } = salvo;
  return resto;
}

/**
 * Os outros avisos (conta vencida, erro de sincronização…) nunca somem: ocultos, viram uma linha com a quantidade.
 * Voltam abertos quando aparece um aviso que não estava lá ao ocultar. `ocultos` guarda a impressão de cada aviso,
 * não o texto: o texto traz nomes de contas e contratos, e o localStorage não guarda dado financeiro (ADR 0017).
 */
export function avisosContinuamOcultos(ocultos: string[], alertas: string[]): boolean {
  return alertas.length > 0 && alertas.every((a) => ocultos.includes(impressaoAviso(a)));
}

/** FNV-1a de 32 bits em hexadecimal: só para reconhecer o mesmo aviso, sem guardar o texto. */
export function impressaoAviso(texto: string): string {
  let h = 0x811c9dc5;
  for (let i = 0; i < texto.length; i++) h = Math.imul(h ^ texto.charCodeAt(i), 0x01000193);
  return (h >>> 0).toString(16).padStart(8, '0');
}

/** Leitura tolerante: navegador sem armazenamento ou valor estragado contam como "nada oculto". */
export function lerOrcamentosOcultos(): OrcamentosOcultos {
  const valor = lerJson(CHAVE_ORCAMENTOS_OCULTOS);
  if (!valor || typeof valor !== 'object' || Array.isArray(valor)) return {};
  const limpo: OrcamentosOcultos = {};
  for (const [periodo, niveis] of Object.entries(valor as Record<string, unknown>)) {
    if (!/^\d{4}-\d{2}$/.test(periodo) || !niveis || typeof niveis !== 'object' || Array.isArray(niveis)) continue;
    limpo[periodo] = Object.fromEntries(
      Object.entries(niveis as Record<string, unknown>).filter((e): e is [string, number] => typeof e[1] === 'number'));
  }
  return limpo;
}

export function lerAvisosOcultos(): string[] {
  const valor = lerJson(CHAVE_AVISOS_OCULTOS);
  return Array.isArray(valor) ? valor.filter((v): v is string => typeof v === 'string') : [];
}

export function lerMinimizado(chave: string): boolean {
  return lerJson(chave) === true;
}

export function guardar(chave: string, valor: unknown): void {
  try {
    localStorage.setItem(chave, JSON.stringify(valor));
  } catch { /* navegador sem armazenamento: só não lembra */ }
}

function lerJson(chave: string): unknown {
  try {
    return JSON.parse(localStorage.getItem(chave) ?? 'null');
  } catch {
    return null;
  }
}
