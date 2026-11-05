import { ChartConfiguration } from 'chart.js';

/**
 * Padrão do site para gráfico de linha (#292): todos se leem do mesmo jeito (Nielsen 4, Consistency and Standards).
 * - **Linha principal** (a primeira contínua, sem tracejado): preenchida **da linha até a base do gráfico**
 *   (`fill: 'start'`), com a cor dela suave, e com um ponto em cada mês. Só ela é preenchida, para as áreas não se
 *   sobreporem.
 * - **Linha de comparação ou de referência** (tracejada: ano anterior, Selic, média): sem preenchimento e sem pontos.
 *
 * Por que 'start' e não `fill: true`: no Chart.js, `true` preenche até o zero ('origin'). Com valores negativos
 * (patrimônio, poupança no vermelho), a faixa ficava acima da linha e parecia invertida.
 *
 * Uso: **não precisa chamar.** O `<app-grafico>` aplica esta função em todo gráfico `type: 'line'`. Para fugir do
 * padrão num conjunto, declare no próprio conjunto: `fill: false` (não preencher) ou `pointRadius` (outro tamanho de
 * ponto; 0 tira os pontos, como em série diária com muitos pontos). Conjunto só de pontos (`showLine: false`) fica
 * como está. A função não altera a configuração recebida: devolve uma cópia.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function linhasNoPadrao<T extends ChartConfiguration<any>>(cfg: T): T {
  if (cfg.type !== 'line') return cfg;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const conjuntos = (cfg.data?.datasets ?? []) as any[];
  const tracejada = (c: { borderDash?: number[] }) => Array.isArray(c.borderDash) && c.borderDash.length > 0;
  const principal = conjuntos.findIndex((c) => c.showLine !== false && !tracejada(c) && c.fill !== false);
  const ajustados = conjuntos.map((c, i) => {
    if (c.showLine === false) return c;
    if (tracejada(c)) return { pointRadius: 0, ...c, fill: false };
    if (i !== principal) return { pointRadius: 3, ...c, fill: false };
    const cor = typeof c.borderColor === 'string' ? c.borderColor : null;
    // Fundo suave: a cor da linha com transparência, a não ser que o conjunto já traga uma cor translúcida própria.
    const fundoProprio = typeof c.backgroundColor === 'string' && c.backgroundColor !== cor ? c.backgroundColor : null;
    const fundo = fundoProprio ?? (cor && /^#[0-9a-f]{6}$/i.test(cor) ? cor + '29' : c.backgroundColor);
    return { pointRadius: 3, ...c, backgroundColor: fundo, pointBackgroundColor: c.pointBackgroundColor ?? cor ?? undefined, fill: 'start' };
  });
  return { ...cfg, data: { ...cfg.data, datasets: ajustados } };
}
