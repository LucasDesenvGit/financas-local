import { ChartConfiguration } from 'chart.js';

/**
 * Padrão do site para gráfico misto, de barras com linha (#282): **a linha fica por cima das barras, com um ponto em
 * cada mês**. Sem isso, a linha sumia atrás das barras (Fluxo de caixa em Relatórios, Saldo previsto na Projeção).
 *
 * Por quê: no Chart.js, por padrão o primeiro conjunto de dados é desenhado por cima, e a opção `order` muda isso:
 * **order menor é desenhado por cima** (https://www.chartjs.org/docs/latest/charts/mixed.html). As linhas recebem
 * `order: 0` e as barras `order: 1`. O ponto em cada mês diz de qual mês é cada valor, e a borda na cor do fundo
 * separa o ponto da barra embaixo dele, nos dois temas.
 *
 * Uso: **não precisa chamar.** O `<app-grafico>` aplica esta função em todo gráfico `type: 'bar'` que tem conjunto
 * `type: 'line'`. Para fugir do padrão num conjunto, declare no próprio conjunto:
 * - `order`: a ordem que você quiser (o padrão só entra quando `order` não foi dito);
 * - `pointRadius: 0`: linha sem pontos, como as de referência ("Média", "Referência 20%"), que não são valores de mês.
 *
 * A função não altera a configuração recebida: devolve uma cópia.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function linhaPorCimaDasBarras<T extends ChartConfiguration<any>>(cfg: T, corDoFundo = '#ffffff'): T {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const conjuntos = (cfg.data?.datasets ?? []) as any[];
  const misto = cfg.type === 'bar' && conjuntos.some((c) => c.type === 'line');
  if (!misto) return cfg;
  const ajustados = conjuntos.map((c) => {
    if (c.type !== 'line') return { order: 1, ...c };
    const pontos = c.pointRadius === undefined
      ? { pointRadius: 4, pointHoverRadius: 6, pointBorderColor: corDoFundo, pointBorderWidth: 2 }
      : {};
    return { order: 0, ...pontos, ...c };
  });
  return { ...cfg, data: { ...cfg.data, datasets: ajustados } };
}
