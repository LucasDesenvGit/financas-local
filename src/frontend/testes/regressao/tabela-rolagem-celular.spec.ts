import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Regressão: em Contas a pagar, no celular, a página ficava mais larga que a tela (607px numa tela de 390px) e o
 * formulário abria cortado à direita. Causa: o texto só para leitor de tela do cabeçalho ("Ações", .so-leitor) é
 * position: absolute; sem ancestral posicionado, ele saía da .tabela-rolagem (overflow-x: auto) e alargava a página.
 * O jsdom não calcula layout, mas aplica a cascata: aqui se confere que a .tabela-rolagem é o bloco de contenção.
 */
describe('Regressão: tabela com rolagem não alarga a página no celular', () => {
  let estilo: HTMLStyleElement;

  beforeAll(() => {
    estilo = document.createElement('style');
    estilo.textContent = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
    document.head.appendChild(estilo);
  });

  afterAll(() => estilo.remove());

  it('a .tabela-rolagem segura o .so-leitor (position: relative) e rola na horizontal', () => {
    const div = document.createElement('div');
    div.className = 'tabela-rolagem';
    div.innerHTML = '<table class="tabela"><thead><tr><th><span class="so-leitor">Ações</span></th></tr></thead></table>';
    document.body.appendChild(div);
    expect(getComputedStyle(div).position).toBe('relative');
    expect(getComputedStyle(div).overflowX).toBe('auto');
    expect(getComputedStyle(div.querySelector('.so-leitor')!).position).toBe('absolute');
    div.remove();
  });
});
