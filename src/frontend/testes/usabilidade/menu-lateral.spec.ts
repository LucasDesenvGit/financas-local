import { abrirShell } from '../apoio/shell';

/**
 * Usabilidade do menu lateral recolhível (#131).
 * - Nielsen 3 (User Control and Freedom): a pessoa esconde o menu e volta com um clique; a marca é a "saída" para
 *   o começo, de qualquer tela.
 * - Nielsen 4 (Consistency and Standards): o mesmo ☰ no mesmo lugar no computador e no celular; logotipo leva à
 *   tela inicial.
 * - WCAG 4.1.2 (Name, Role, Value): nome fixo e o estado em aria-expanded, com aria-controls para o menu.
 * - WCAG 2.1.1 (Keyboard): o Tab não para em link escondido (inert).
 * - WCAG 2.5.8 (Target Size), a marca na largura toda e a animação de entrada da tela: no navegador, com o
 *   tamanho e o estilo calculados de verdade (testes-navegador/estilos.spec.ts, #204).
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

  it('a marca se chama "Finanças, ir para o Painel" e o ícone é decorativo', async () => {
    const t = await abrirShell();
    expect(t.marca().getAttribute('aria-label')).toBe('Finanças, ir para o Painel');
    expect(t.marca().querySelector('.marca-simbolo')!.getAttribute('aria-hidden')).toBe('true');
    expect(t.tituloBarra().getAttribute('aria-label')).toBe('Finanças, ir para o Painel');
    expect(t.tituloBarra().querySelector('.barra-titulo-simbolo')!.getAttribute('aria-hidden')).toBe('true');
  });
});
