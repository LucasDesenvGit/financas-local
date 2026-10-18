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

  it('os três cards têm a mesma altura (esticam juntos) e o valor fica logo abaixo do rótulo, não no meio', () => {
    const linha = document.createElement('section');
    linha.className = 'kpis kpis-mes';
    linha.innerHTML = '<article class="kpi"><span>Receitas do mês</span><strong>R$ 1,00</strong></article>';
    document.body.appendChild(linha);
    expect(getComputedStyle(linha).alignItems).not.toBe('start');
    expect(getComputedStyle(linha.querySelector('.kpi')!).alignContent).toBe('start');
    linha.remove();
  });

  /** O jsdom não calcula propriedades de grid: procura a regra da folha de estilo que casa com o elemento. */
  const regraCom = (el: Element, declaracao: string) => Array.from(estilo.sheet!.cssRules).some((r) =>
    r instanceof CSSStyleRule && r.cssText.includes(declaracao) && el.matches(r.selectorText));

  it('Receitas, Despesas e Resultado em três colunas iguais; "Faturas do mês" numa linha própria', () => {
    const linha = document.createElement('section');
    linha.className = 'kpis kpis-mes';
    linha.innerHTML = '<article class="kpi"></article><article class="kpi"></article><article class="kpi"></article>'
      + '<article class="kpi kpi-faturas"></article>';
    document.body.appendChild(linha);
    expect(regraCom(linha, 'grid-template-columns: repeat(3, minmax(0, 1fr))')).toBe(true);
    const cards = Array.from(linha.children);
    expect(regraCom(cards[3], 'grid-column: 1 / -1')).toBe(true);
    expect(cards.slice(0, 3).some((c) => regraCom(c, 'grid-column: 1 / -1'))).toBe(false);
    linha.remove();
  });

  it('os cartões de "Faturas do mês" dividem a largura toda (auto-fit), sem sobra vazia à direita', () => {
    const painel = document.createElement('app-painel');
    painel.innerHTML = '<ul class="lista-faturas"><li></li><li></li></ul>';
    document.body.appendChild(painel);
    expect(regraCom(painel.querySelector('.lista-faturas')!, 'repeat(auto-fit, minmax(260px, 1fr))')).toBe(true);
    painel.remove();
  });

  it('as outras linhas de cards não mudam (sem as três colunas fixas do mês)', () => {
    const linha = document.createElement('section');
    linha.className = 'kpis';
    document.body.appendChild(linha);
    expect(regraCom(linha, 'grid-template-columns: repeat(3, minmax(0, 1fr))')).toBe(false);
    linha.remove();
  });
});
