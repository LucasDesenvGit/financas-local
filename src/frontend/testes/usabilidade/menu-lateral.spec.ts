import { readFileSync } from 'fs';
import { join } from 'path';
import { abrirShell } from '../apoio/shell';

/**
 * Usabilidade do menu lateral recolhível (#131).
 * - Nielsen 3 (User Control and Freedom): a pessoa esconde o menu e volta com um clique; a marca é a "saída" para
 *   o começo, de qualquer tela.
 * - Nielsen 4 (Consistency and Standards): o mesmo ☰ no mesmo lugar no computador e no celular; logotipo leva à
 *   tela inicial.
 * - WCAG 4.1.2 (Name, Role, Value): nome fixo e o estado em aria-expanded, com aria-controls para o menu.
 * - WCAG 2.1.1 (Keyboard): o Tab não para em link escondido (inert).
 * - WCAG 2.5.8 (Target Size): o botão tem pelo menos 24×24 px. O jsdom não calcula layout, mas aplica a cascata.
 */
describe('Usabilidade: menu lateral recolhível', () => {
  it('o ☰ tem nome acessível fixo, que não muda com o estado, e controla o menu lateral', async () => {
    const t = await abrirShell();
    const nome = () => t.botao().getAttribute('aria-label');
    expect(nome()).toBe('Menu de navegação');
    expect(document.getElementById(t.botao().getAttribute('aria-controls')!)).toBe(t.lateral());
    t.botao().click();
    await t.atualizar();
    expect(nome()).toBe('Menu de navegação');
    expect(t.botao().getAttribute('aria-expanded')).toBe('false');
  });

  it('o ☰ aparece também no computador (não é mais só do celular)', async () => {
    const t = await abrirShell();
    expect(t.botao().classList).not.toContain('so-celular');
  });

  it('menu escondido sai da ordem do Tab (inert); aberto, os links voltam', async () => {
    const t = await abrirShell();
    expect(t.lateral().hasAttribute('inert')).toBe(false);
    t.botao().click();
    await t.atualizar();
    expect(t.lateral().hasAttribute('inert')).toBe(true);
    t.botao().click();
    await t.atualizar();
    expect(t.lateral().hasAttribute('inert')).toBe(false);
  });

  it('no celular, a gaveta fechada também fica fora do Tab', async () => {
    const t = await abrirShell({ celular: true });
    expect(t.lateral().hasAttribute('inert')).toBe(true);
    t.botao().click();
    await t.atualizar();
    expect(t.lateral().hasAttribute('inert')).toBe(false);
  });

  it('a marca se chama "Finanças, ir para o Painel" e o ícone é decorativo', async () => {
    const t = await abrirShell();
    expect(t.marca().getAttribute('aria-label')).toBe('Finanças, ir para o Painel');
    expect(t.marca().querySelector('.marca-simbolo')!.getAttribute('aria-hidden')).toBe('true');
    expect(t.tituloBarra().getAttribute('aria-label')).toBe('Finanças, ir para o Painel');
  });

  describe('com o styles.css do site', () => {
    let estilo: HTMLStyleElement;
    beforeAll(() => {
      estilo = document.createElement('style');
      estilo.textContent = readFileSync(join(__dirname, '../../src/styles.css'), 'utf8');
      document.head.appendChild(estilo);
    });
    afterAll(() => estilo.remove());

    it('o ☰ tem alvo de pelo menos 24×24 px', async () => {
      const t = await abrirShell();
      const css = getComputedStyle(t.botao());
      expect(parseFloat(css.minWidth)).toBeGreaterThanOrEqual(24);
      expect(parseFloat(css.height)).toBeGreaterThanOrEqual(24);
    });

    it('a marca e o título da barra não aparecem sublinhados', async () => {
      const t = await abrirShell();
      expect(getComputedStyle(t.marca()).textDecoration).toContain('none');
      expect(getComputedStyle(t.tituloBarra()).textDecoration).toContain('none');
    });
  });
});
