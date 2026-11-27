/**
 * O que liga cada tela à seção dela no Manual, longe do texto do Manual (#755): o shell importa daqui, e o
 * `manual-conteudo.ts`, com ~120 KB de texto, fica só no bundle da página do Manual (ADR 0026).
 */

/** Capítulo (e seção) do manual da tela de uma URL ("/transacoes?x=1" → transações); null fora das telas do menu. */
export function ajudaDaUrl(url: string): { capitulo: string; secao?: string } | null {
  // #211: a tela de um módulo tem dois trechos ("trabalho/holerites"); vale o mais específico.
  const partes = url.split(/[?#]/)[0].split('/').filter(Boolean);
  return AJUDA_POR_ROTA[partes.slice(0, 2).join('/')] ?? AJUDA_POR_ROTA[partes[0] ?? ''] ?? null;
}

/**
 * "Ajuda" na barra de cima: a seção do manual que explica a tela aberta (pelo primeiro trecho da rota).
 * Toda rota do menu precisa estar aqui (teste de regressão).
 */
export const AJUDA_POR_ROTA: Record<string, { capitulo: string; secao?: string }> = {
  painel: { capitulo: 'relatorios', secao: 'painel' },
  anual: { capitulo: 'relatorios', secao: 'visao-anual' },
  transacoes: { capitulo: 'transacoes' },
  contas: { capitulo: 'contas' },
  orcamentos: { capitulo: 'orcamentos' },
  metas: { capitulo: 'metas' },
  'contas-a-pagar': { capitulo: 'contas-a-pagar' },
  investimentos: { capitulo: 'investimentos' },
  emprestimos: { capitulo: 'investimentos', secao: 'emprestimos' },
  relatorios: { capitulo: 'relatorios', secao: 'relatorios-tela' },
  projecao: { capitulo: 'relatorios', secao: 'projecao' },
  categorias: { capitulo: 'categorias' },
  configuracoes: { capitulo: 'configuracoes' },
  trabalho: { capitulo: 'trabalho', secao: 'salario' },
  'trabalho/holerites': { capitulo: 'trabalho', secao: 'holerites' },
  'trabalho/decimo-terceiro': { capitulo: 'trabalho', secao: 'decimo-terceiro' },
  'trabalho/ferias': { capitulo: 'trabalho', secao: 'ferias' },
  'trabalho/fgts': { capitulo: 'trabalho', secao: 'fgts' },
  'trabalho/rescisao': { capitulo: 'trabalho', secao: 'rescisao' },
  noticias: { capitulo: 'noticias' },
  'conferir-banco': { capitulo: 'contas', secao: 'conferir-banco' }
};
