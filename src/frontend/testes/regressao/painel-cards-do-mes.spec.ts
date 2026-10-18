import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Regressão: no Painel, os cards Receitas/Despesas/Resultado esticavam até a altura de "Faturas do mês" (que lista
 * os cartões) e o valor ficava solto no meio de um card alto e vazio. O jsdom não calcula layout, mas aplica a
 * cascata: confere que a linha do mês não estica os cards e que o conteúdo fica no topo.
 */
describe('Regressão: cards do mês no Painel com a altura do conteúdo', () => {
  let estilo: HTMLStyleElement;

  beforeAll(() => {
    estilo = document.createElement('style');
    estilo.textContent = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
    document.head.appendChild(estilo);
  });

  afterAll(() => estilo.remove());

  it('a linha do mês não estica os cards, e o conteúdo de cada um fica no topo', () => {
    const linha = document.createElement('section');
    linha.className = 'kpis kpis-mes';
    linha.innerHTML = '<article class="kpi"><span>Receitas do mês</span><strong>R$ 1,00</strong></article>';
    document.body.appendChild(linha);
    expect(getComputedStyle(linha).alignItems).toBe('start');
    expect(getComputedStyle(linha.querySelector('.kpi')!).alignContent).toBe('start');
    linha.remove();
  });

  it('as outras linhas de cards não mudam', () => {
    const linha = document.createElement('section');
    linha.className = 'kpis';
    document.body.appendChild(linha);
    expect(getComputedStyle(linha).alignItems).not.toBe('start');
    linha.remove();
  });
});
