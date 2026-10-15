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

export const CORES_SUGERIDAS = [
  '#a7a3f5', '#92c8f0', '#8fd5b4', '#8fd8d8', '#b5cf9a', '#f2d98b',
  '#f8c09a', '#f4a89a', '#f5a3b0', '#f3b3d3', '#d4a5e8', '#a9b4c6'
];
