/**
 * #336: as fatias da rosca de categorias. As menores que `minimo` (3% do total) viram uma fatia só, "Outras", para o
 * gráfico não ter fatias finas demais para ver e clicar (WCAG 2.5.8); a lista ao lado continua com todas.
 * `indices[k]` é a posição do item da fatia k na lista original (-1 em "Outras"), para o clique na fatia abrir a
 * categoria certa.
 */
export interface FatiasRosca {
  rotulos: string[];
  valores: number[];
  cores: string[];
  indices: number[];
}

export const MINIMO_FATIA = 0.03;
export const COR_OUTRAS = '#a9b4c6';

export function fatiasRosca(itens: readonly { nome: string; valor: number; cor: string }[], minimo = MINIMO_FATIA): FatiasRosca {
  const total = itens.reduce((s, i) => s + Math.max(0, i.valor), 0);
  const fatias: FatiasRosca = { rotulos: [], valores: [], cores: [], indices: [] };
  let outras = 0;
  itens.forEach((item, k) => {
    if (item.valor <= 0) return;
    if (total > 0 && item.valor / total < minimo) {
      outras += item.valor;
      return;
    }
    fatias.rotulos.push(item.nome);
    fatias.valores.push(item.valor);
    fatias.cores.push(item.cor);
    fatias.indices.push(k);
  });
  if (outras > 0) {
    fatias.rotulos.push('Outras');
    fatias.valores.push(Math.round(outras * 100) / 100);
    fatias.cores.push(COR_OUTRAS);
    fatias.indices.push(-1);
  }
  return fatias;
}
