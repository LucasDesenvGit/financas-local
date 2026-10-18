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

  it('numa grade de cartões (Metas sem metas), o aviso ocupa a linha toda, e não só a 1ª coluna', () => {
    const grade = document.createElement('section');
    grade.className = 'grade-cartoes';
    grade.innerHTML = '<p class="vazio">Crie metas como reserva de emergência, viagem ou entrada de um imóvel.</p>';
    document.body.appendChild(grade);
    // O jsdom não calcula grid-column: confere a regra da folha de estilo que casa com o aviso.
    const regra = Array.from(estilo.sheet!.cssRules).find((r): r is CSSStyleRule =>
      r instanceof CSSStyleRule && r.cssText.includes('grid-column: 1 / -1') && grade.firstElementChild!.matches(r.selectorText));
    expect(regra).toBeDefined();
    grade.remove();
  });

  it('o aviso de vazio centraliza a caixa (margens automáticas), não só o texto', () => {
    const p = computado('<p class="vazio">Nenhum empréstimo ou financiamento encontrado.</p>');
    expect(p.textAlign).toBe('center');
    expect(p.marginLeft).toBe('auto');
    expect(p.marginRight).toBe('auto');
  });
});
