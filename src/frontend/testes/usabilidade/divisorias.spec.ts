import { readFileSync } from 'fs';
import { join } from 'path';
import { abrirContas } from '../apoio/contas';
import { abrirPainel, painel } from '../apoio/painel';

/**
 * Usabilidade: divisórias em todas as telas — abaixo do cabeçalho da página, entre as seções, abaixo do título de
 * cada bloco e entre os cartões de "Faturas do mês" no Painel. Nielsen 8 (Aesthetic and Minimalist Design) e
 * Nielsen 4 (Consistency and Standards): a mesma linha separa título de conteúdo e um assunto do outro em todas as
 * telas, sem acrescentar texto. O jsdom não desenha: aqui se confere, com o styles.css do site, que as linhas
 * existem onde devem.
 */
describe('Usabilidade: divisórias', () => {
  let estilo: HTMLStyleElement;
  beforeAll(() => {
    estilo = document.createElement('style');
    estilo.textContent = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
    document.head.appendChild(estilo);
  });
  afterAll(() => estilo.remove());

  /**
   * O jsdom não calcula "border-top: 1px solid var(--linha)" (atalho com var()), mas guarda a regra: procura uma
   * regra com a linha cujo seletor casa com o elemento.
   */
  const temLinha = (el: Element, lado: 'top' | 'bottom' | 'left') =>
    Array.from(document.styleSheets).flatMap((s) => Array.from(s.cssRules)).some((r) =>
      r instanceof CSSStyleRule && r.cssText.includes(`border-${lado}: 1px solid`) && el.matches(r.selectorText));

  describe('Painel', () => {
    const dados = painel({
      // Sem categorias: o gráfico (Chart.js + tema) não interessa aqui, e o bloco aparece com o título mesmo vazio.
      receitasMes: 1000, despesasMes: 400, totalFaturasDoMes: 620,
      faturasDoMes: [
        { contaId: 1, cartao: 'Santander', valorMes: 500, parcelasMes: 0, faturaId: null, vencimento: null, situacao: null },
        { contaId: 2, cartao: 'gold', valorMes: 120, parcelasMes: 0, faturaId: null, vencimento: null, situacao: null }
      ] as never
    });

    it('linha abaixo do cabeçalho da página', async () => {
      const t = await abrirPainel(dados);
      expect(temLinha(t.raiz.querySelector('.cabecalho-pagina')!, 'bottom')).toBe(true);
    });

    it('uma linha separa "Mês de …" da seção "Situação hoje" (e nada acima da primeira)', async () => {
      const t = await abrirPainel(dados);
      const [hoje, mes] = Array.from(t.raiz.querySelectorAll('.secao-painel'));
      expect(temLinha(hoje, 'top')).toBe(false);
      expect(temLinha(mes, 'top')).toBe(true);
    });

    it('todo bloco do Painel tem o título separado do conteúdo por uma linha', async () => {
      const t = await abrirPainel(dados);
      const blocos = Array.from(t.raiz.querySelectorAll('.painel-bloco'));
      expect(blocos.length).toBeGreaterThan(0);
      for (const bloco of blocos) {
        const titulo = bloco.querySelector(':scope > .bloco-titulo')!;
        expect(titulo).not.toBeNull();
        expect(temLinha(titulo, 'bottom')).toBe(true);
      }
    });

    it('em "Faturas do mês", uma linha abaixo do total e uma linha vertical entre os cartões lado a lado', async () => {
      const t = await abrirPainel(dados);
      const lista = t.raiz.querySelector('.lista-faturas')!;
      expect(temLinha(lista, 'top')).toBe(true);
      // Regras de computador (fora de @media); no celular os cartões empilham com a linha em cima.
      const [primeiro, segundo] = Array.from(lista.querySelectorAll('li'));
      expect(temLinha(primeiro, 'left')).toBe(false);
      expect(temLinha(segundo, 'left')).toBe(true);
    });
  });

  describe('Contas e cartões', () => {
    it('linha abaixo do cabeçalho e entre as seções "Contas" e "Cartões de crédito"', async () => {
      const t = await abrirContas();
      expect(temLinha(t.raiz.querySelector('.cabecalho-pagina')!, 'bottom')).toBe(true);
      const [contas, cartoes] = Array.from(t.raiz.querySelectorAll('.secao-contas'));
      expect(contas.querySelector('#titulo-contas')).not.toBeNull();
      expect(cartoes.querySelector('#titulo-cartoes')).not.toBeNull();
      expect(temLinha(contas, 'top')).toBe(false);
      expect(temLinha(cartoes, 'top')).toBe(true);
    });
  });

  describe('blocos das outras telas, sem mudar o HTML de cada uma', () => {
    function bloco(html: string) {
      const el = document.createElement('section');
      el.className = 'painel-bloco';
      el.innerHTML = html;
      document.body.appendChild(el);
      return el;
    }

    it('o h2 solto que abre o bloco (Visão anual, Relatórios, Projeção, Configurações…) ganha a linha', () => {
      const el = bloco('<h2>Fluxo de caixa mensal</h2><p>conteúdo</p><h2>outro título no meio</h2>');
      const [primeiro, segundo] = Array.from(el.querySelectorAll('h2'));
      expect(temLinha(primeiro, 'bottom')).toBe(true);
      expect(temLinha(segundo, 'bottom')).toBe(false);   // só o título do bloco, não todo h2
      el.remove();
    });

    it('o .cabecalho-bloco (Investimentos e FIIs) também', () => {
      const el = bloco('<div class="cabecalho-bloco"><h2>Posições</h2></div>');
      expect(temLinha(el.querySelector('.cabecalho-bloco')!, 'bottom')).toBe(true);
      el.remove();
    });
  });
});
