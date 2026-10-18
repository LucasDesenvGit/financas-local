import { readFileSync } from 'fs';
import { join } from 'path';
import { abrirPainel, painel } from '../apoio/painel';

/**
 * Usabilidade do Painel: divisórias entre as seções, abaixo do título de cada bloco e entre os cartões de "Faturas
 * do mês". Nielsen 8 (Aesthetic and Minimalist Design) e a lei de região comum: a linha agrupa o que é do mesmo
 * assunto e separa títulos de conteúdo, sem acrescentar texto. O jsdom não desenha: aqui se confere, com o
 * styles.css do site, que as linhas existem onde devem.
 */
describe('Usabilidade: divisórias do Painel', () => {
  let estilo: HTMLStyleElement;
  beforeAll(() => {
    estilo = document.createElement('style');
    estilo.textContent = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
    document.head.appendChild(estilo);
  });
  afterAll(() => estilo.remove());

  const dados = painel({
    // Sem categorias: o gráfico (Chart.js + tema) não interessa aqui, e o bloco aparece com o título mesmo vazio.
    receitasMes: 1000, despesasMes: 400, totalFaturasDoMes: 620,
    faturasDoMes: [
      { contaId: 1, cartao: 'Santander', valorMes: 500, parcelasMes: 0, faturaId: null, vencimento: null, situacao: null },
      { contaId: 2, cartao: 'gold', valorMes: 120, parcelasMes: 0, faturaId: null, vencimento: null, situacao: null }
    ] as never
  });
  /**
   * O jsdom não calcula "border-top: 1px solid var(--linha)" (atalho com var()), mas guarda a regra: procura uma
   * regra com a linha cujo seletor casa com o elemento.
   */
  const borda = (el: Element, lado: 'Top' | 'Bottom') =>
    Array.from(document.styleSheets).flatMap((s) => Array.from(s.cssRules)).some((r) =>
      r instanceof CSSStyleRule && r.cssText.includes(`border-${lado.toLowerCase()}: 1px solid`) && el.matches(r.selectorText))
      ? 'solid' : 'nenhuma';

  it('uma linha separa "Mês de …" da seção "Situação hoje"', async () => {
    const t = await abrirPainel(dados);
    const [hoje, mes] = Array.from(t.raiz.querySelectorAll('.secao-painel'));
    expect(borda(hoje, 'Top')).not.toBe('solid');
    expect(borda(mes, 'Top')).toBe('solid');
  });

  it('todo bloco do Painel tem o título separado do conteúdo por uma linha', async () => {
    const t = await abrirPainel(dados);
    const blocos = Array.from(t.raiz.querySelectorAll('.painel-bloco'));
    expect(blocos.length).toBeGreaterThan(0);
    for (const bloco of blocos) {
      const titulo = bloco.querySelector(':scope > .bloco-titulo');
      expect(titulo).not.toBeNull();
      expect(borda(titulo!, 'Bottom')).toBe('solid');
    }
  });

  it('em "Faturas do mês", uma linha abaixo do total e entre cada cartão', async () => {
    const t = await abrirPainel(dados);
    const lista = t.raiz.querySelector('.lista-faturas')!;
    expect(borda(lista, 'Top')).toBe('solid');
    const [primeiro, segundo] = Array.from(lista.querySelectorAll('li'));
    expect(borda(primeiro, 'Top')).not.toBe('solid');
    expect(borda(segundo, 'Top')).toBe('solid');
  });
});
