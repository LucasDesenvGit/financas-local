import { ChartConfiguration } from 'chart.js';
import { linhaPorCimaDasBarras } from '../../src/app/compartilhado/grafico-misto';

/**
 * Padrão do site para gráfico de barras com linha (#282): a linha por cima das barras (no Chart.js, `order` menor é
 * desenhado por cima) e com um ponto em cada mês. O <app-grafico> aplica sozinho; o conjunto pode fugir do padrão
 * declarando `order` ou `pointRadius: 0` (linhas de referência).
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Conjunto = Record<string, any>;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const config = (type: string, datasets: Conjunto[]) => ({ type, data: { labels: ['a'], datasets } }) as ChartConfiguration<any>;
const conjuntos = (cfg: ChartConfiguration<'bar'>) => cfg.data.datasets as unknown as Conjunto[];

describe('Gráfico de barras com linha: a linha por cima e com pontos', () => {
  it('a linha recebe order menor que as barras e um ponto por mês, com borda na cor do fundo', () => {
    const [barra, linha] = conjuntos(linhaPorCimaDasBarras(
      config('bar', [{ label: 'Receitas', data: [1] }, { label: 'Saldo', data: [1], type: 'line' }]), '#1d1b26'));
    expect(barra['order']).toBeGreaterThan(linha['order']);
    expect(linha['pointRadius']).toBeGreaterThan(0);
    expect(linha['pointBorderColor']).toBe('#1d1b26');
  });

  it('respeita o que o conjunto já declarou (order e pointRadius: 0 das linhas de referência)', () => {
    const [barra, referencia] = conjuntos(linhaPorCimaDasBarras(config('bar', [
      { label: 'Barras', data: [1], order: 5 },
      { label: 'Média', data: [1], type: 'line', pointRadius: 0 }
    ])));
    expect(barra['order']).toBe(5);
    expect(referencia['pointRadius']).toBe(0);
    expect(referencia['pointBorderColor']).toBeUndefined();
    expect(referencia['order']).toBe(0);
  });

  it('gráfico só de barras ou só de linha fica como veio; a configuração recebida não é alterada', () => {
    const soBarras = config('bar', [{ label: 'a', data: [1] }]);
    expect(linhaPorCimaDasBarras(soBarras)).toBe(soBarras);
    const soLinha = config('line', [{ label: 'a', data: [1] }]);
    expect(linhaPorCimaDasBarras(soLinha)).toBe(soLinha);
    const misto = config('bar', [{ label: 'a', data: [1] }, { label: 'b', data: [1], type: 'line' }]);
    linhaPorCimaDasBarras(misto);
    expect(conjuntos(misto)[0]['order']).toBeUndefined();
  });
});
