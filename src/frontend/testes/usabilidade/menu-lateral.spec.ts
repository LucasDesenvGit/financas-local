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
 * Grupos que abrem e fecham (#227, ADR 0046):
 * - WAI-ARIA APG, Disclosure Pattern: cabeçalho é botão com aria-expanded e aria-controls apontando para as telas.
 * - WCAG 2.1.1 (Keyboard): tela de grupo fechado fica fora do Tab (inert e invisível).
 * - WCAG 2.5.8 (Target Size): o cabeçalho tem pelo menos 24 px de altura e ocupa a largura do menu.
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

  it('gaveta do celular: o foco vai para o ✕ ("Fechar menu") e o conteúdo atrás sai do Tab', async () => {
    const t = await abrirShell({ celular: true });
    expect(t.conteudo().hasAttribute('inert')).toBe(false);
    t.botao().click();
    await t.atualizar();
    expect(t.botaoFechar().getAttribute('aria-label')).toBe('Fechar menu');
    expect(document.activeElement).toBe(t.botaoFechar());
    expect(t.conteudo().hasAttribute('inert')).toBe(true);
    expect(t.fundo().getAttribute('aria-hidden')).toBe('true');   // só para toque; teclado usa o ✕ e o Esc
    t.botaoFechar().click();
    await t.atualizar();
    expect(t.conteudo().hasAttribute('inert')).toBe(false);
  });

  it('o ✕ é só do celular (no computador, o ☰ da barra já esconde o menu)', async () => {
    const t = await abrirShell();
    expect(t.botaoFechar().classList).toContain('so-celular');
  });

  it('cabeçalho do grupo controla a lista de telas, que se apresenta como grupo com o nome dele', async () => {
    const t = await abrirShell();
    const botao = t.cabecalho('analises');
    const lista = document.getElementById(botao.getAttribute('aria-controls')!)!;
    expect(lista).toBe(t.telasDoGrupo('analises'));
    expect(lista.getAttribute('role')).toBe('group');
    expect(document.getElementById(lista.getAttribute('aria-labelledby')!)).toBe(botao);
    // Nome fixo (o estado vai no aria-expanded) e os ícones decorativos.
    botao.click();
    await t.atualizar();
    expect(botao.textContent!.trim()).toBe('Análises');
    botao.querySelectorAll('svg').forEach((svg) => expect(svg.getAttribute('aria-hidden')).toBe('true'));
  });

  it('a marca se chama "Finanças, ir para o Painel" e o ícone é decorativo', async () => {
    const t = await abrirShell();
    expect(t.marca().getAttribute('aria-label')).toBe('Finanças, ir para o Painel');
    expect(t.marca().querySelector('.marca-simbolo')!.getAttribute('aria-hidden')).toBe('true');
    expect(t.tituloBarra().getAttribute('aria-label')).toBe('Finanças, ir para o Painel');
    expect(t.tituloBarra().querySelector('.barra-titulo-simbolo')!.getAttribute('aria-hidden')).toBe('true');
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

    it('a tela aberta pelo roteador entra com animação; o router-outlet em si não anima', async () => {
      const t = await abrirShell();
      const pagina = t.raiz.querySelector('main.pagina')!;
      // O jsdom não interpreta "animation": confere a regra na folha de estilo e quem casa com o seletor dela.
      const regra = Array.from(estilo.sheet!.cssRules).find(
        (r): r is CSSStyleRule => r instanceof CSSStyleRule && r.style.cssText.includes('entrar-tela'))!;
      expect(regra).toBeDefined();
      const tela = pagina.querySelector(':scope > :not(router-outlet)')!;
      expect(tela.matches(regra.selectorText)).toBe(true);
      expect(pagina.querySelector('router-outlet')!.matches(regra.selectorText)).toBe(false);
    });

    it('cabeçalho do grupo: alvo de pelo menos 24 px de altura, na largura toda do menu', async () => {
      const t = await abrirShell();
      const css = getComputedStyle(t.cabecalho('movimentacao'));
      expect(parseFloat(css.minHeight)).toBeGreaterThanOrEqual(24);
      expect(css.width).toBe('100%');
    });

    it('grupo fechado fica invisível (fora do Tab e do leitor de tela); aberto, as telas voltam', async () => {
      const t = await abrirShell({ url: '/painel' });
      expect(getComputedStyle(t.telasDoGrupo('ajustes')).visibility).toBe('visible');
      t.cabecalho('ajustes').click();
      await t.atualizar();
      expect(getComputedStyle(t.telasDoGrupo('ajustes')).visibility).toBe('hidden');
      expect(getComputedStyle(t.linkDaTela('/categorias')).visibility).toBe('hidden');
    });

    it('com movimento reduzido no sistema, abrir e fechar o grupo não anima (regra global sem transição)', () => {
      const regra = Array.from(estilo.sheet!.cssRules).find(
        (r): r is CSSMediaRule => r instanceof CSSMediaRule && r.conditionText.includes('prefers-reduced-motion'))!;
      expect(regra.cssText).toMatch(/\*.*transition: none !important/s);
    });

    it('a marca ocupa a largura toda do topo do menu (área clicável inteira, não só o texto)', async () => {
      const t = await abrirShell();
      expect(getComputedStyle(t.marca()).flexGrow).toBe('1');
    });

    it('a marca e o título da barra não aparecem sublinhados', async () => {
      const t = await abrirShell();
      expect(getComputedStyle(t.marca()).textDecoration).toContain('none');
      expect(getComputedStyle(t.tituloBarra()).textDecoration).toContain('none');
    });
  });
});
