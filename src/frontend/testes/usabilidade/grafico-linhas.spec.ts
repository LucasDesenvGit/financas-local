import { ChartConfiguration } from 'chart.js';
import { linhasNoPadrao } from '../../src/app/compartilhado/grafico-linhas';

/**
 * #292: todo gráfico de linha se lê do mesmo jeito (Nielsen 4). A linha principal (contínua) é preenchida da linha até
 * a base ('start', não até o zero: com valor negativo a faixa ficava acima da linha) e tem pontos; a tracejada
 * (comparação, referência) não tem preenchimento nem pontos. O <app-grafico> aplica sozinho.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Conjunto = Record<string, any>;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const config = (type: string, datasets: Conjunto[]) => ({ type, data: { labels: ['jan'], datasets } }) as ChartConfiguration<any>;
const conjuntos = (cfg: ChartConfiguration<'line'>) => cfg.data.datasets as unknown as Conjunto[];

describe('Gráfico de linha no padrão do site (#292)', () => {
  it('a principal preenche até a base, com a cor suave e pontos; a tracejada fica sem nada', () => {
    const [atual, anterior] = conjuntos(linhasNoPadrao(config('line', [
      { label: '2026', data: [-1], borderColor: '#a7a3f5', backgroundColor: '#a7a3f5' },
      { label: '2025', data: [0], borderColor: '#cccccc', borderDash: [6, 5] }
    ])));
    expect(atual['fill']).toBe('start');
    expect(atual['backgroundColor']).toBe('#a7a3f529');
    expect(atual['pointRadius']).toBeGreaterThan(0);
    expect(atual['pointBackgroundColor']).toBe('#a7a3f5');
    expect(anterior['fill']).toBe(false);
    expect(anterior['pointRadius']).toBe(0);
  });

  it('fill: true (até o zero) vira até a base; o fundo translúcido próprio do conjunto continua', () => {
    const [patrimonio] = conjuntos(linhasNoPadrao(config('line', [
      { label: 'Patrimônio', data: [-11000, -20000], borderColor: '#92c8f0', backgroundColor: '#92c8f033', fill: true }
    ])));
    expect(patrimonio['fill']).toBe('start');
    expect(patrimonio['backgroundColor']).toBe('#92c8f033');
  });

  it('só a primeira contínua é preenchida; quem declara fill: false ou pointRadius fica como quis', () => {
    const [a, b, c] = conjuntos(linhasNoPadrao(config('line', [
      { label: 'sem área', data: [1], borderColor: '#111111', fill: false },
      { label: 'diária', data: [1], borderColor: '#222222', pointRadius: 0 },
      { label: 'outra', data: [1], borderColor: '#333333' }
    ])));
    expect(a['fill']).toBe(false);
    expect(b['fill']).toBe('start');
    expect(b['pointRadius']).toBe(0);
    expect(c['fill']).toBe(false);
  });

  it('pontos soltos (showLine: false) e gráficos que não são de linha ficam como vieram', () => {
    const pontos = { label: 'compras', data: [1], showLine: false, pointRadius: 6 };
    expect(conjuntos(linhasNoPadrao(config('line', [pontos])))[0]).toBe(pontos);
    const barras = config('bar', [{ label: 'a', data: [1] }]);
    expect(linhasNoPadrao(barras)).toBe(barras);
  });
});
