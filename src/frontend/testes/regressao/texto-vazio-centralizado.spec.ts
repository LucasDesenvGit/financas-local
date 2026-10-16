import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Regressão do bug #46 ("tela desalinhada" em Empréstimos): o aviso de lista vazia (<p class="vazio">) tinha o texto
 * centralizado, mas a caixa do parágrafo ficava encostada à esquerda do bloco, porque todo <p> tem max-width: 78ch.
 * Em tela larga, "Nenhum empréstimo ou financiamento encontrado." aparecia fora do meio, desalinhado do texto de
 * baixo. O jsdom não calcula layout, mas aplica a cascata do CSS: aqui se confere a regra que resolve (caixa com
 * margens automáticas), com o styles.css de verdade.
 */
describe('Regressão: aviso de lista vazia centralizado (#46)', () => {
  let estilo: HTMLStyleElement;

  beforeAll(() => {
    estilo = document.createElement('style');
    estilo.textContent = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
    document.head.appendChild(estilo);
  });

  afterAll(() => estilo.remove());

  function computado(html: string): CSSStyleDeclaration {
    const bloco = document.createElement('section');
    bloco.className = 'painel-bloco';
    bloco.innerHTML = html;
    document.body.appendChild(bloco);
    const estiloFinal = getComputedStyle(bloco.firstElementChild!);
    bloco.remove();
    return estiloFinal;
  }

  it('parágrafo continua com largura máxima de leitura (a regra que causava o deslocamento)', () => {
    expect(computado('<p>texto</p>').maxWidth).toBe('78ch');
  });

  it('o aviso de vazio centraliza a caixa (margens automáticas), não só o texto', () => {
    const p = computado('<p class="vazio">Nenhum empréstimo ou financiamento encontrado.</p>');
    expect(p.textAlign).toBe('center');
    expect(p.marginLeft).toBe('auto');
    expect(p.marginRight).toBe('auto');
  });
});
