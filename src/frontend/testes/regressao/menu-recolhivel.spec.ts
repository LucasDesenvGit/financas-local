import { TestBed } from '@angular/core/testing';
import { ShellComponent } from '../../src/app/layout/shell.component';
import { abrirShell } from '../apoio/shell';

/**
 * Regressão #131: o ☰ esconde e mostra o menu lateral também no computador, a escolha é lembrada no navegador
 * (financas.menuRecolhido, ADR 0017) e não vale no celular; a marca "Finanças" leva ao Painel.
 */
describe('Regressão: menu lateral recolhível e marca levando ao Painel', () => {
  it('no computador, o ☰ esconde e mostra o menu, com aria-expanded acompanhando', async () => {
    const t = await abrirShell();
    expect(t.app().classList).not.toContain('menu-recolhido');
    expect(t.botao().getAttribute('aria-expanded')).toBe('true');

    t.botao().click();
    await t.atualizar();
    expect(t.app().classList).toContain('menu-recolhido');
    expect(t.botao().getAttribute('aria-expanded')).toBe('false');

    t.botao().click();
    await t.atualizar();
    expect(t.app().classList).not.toContain('menu-recolhido');
    expect(t.botao().getAttribute('aria-expanded')).toBe('true');
  });

  it('a escolha fica salva em financas.menuRecolhido e volta ao recarregar; o padrão é aberto', async () => {
    expect(localStorage.getItem(ShellComponent.CHAVE_RECOLHIDO)).toBeNull();
    const t = await abrirShell();
    t.botao().click();
    await t.atualizar();
    expect(localStorage.getItem(ShellComponent.CHAVE_RECOLHIDO)).toBe('1');

    // "Recarregar": um Shell novo, que só sabe o que ficou no navegador.
    TestBed.resetTestingModule();
    const recarregado = await abrirShell();
    expect(recarregado.app().classList).toContain('menu-recolhido');
    expect(recarregado.botao().getAttribute('aria-expanded')).toBe('false');
  });

  it('sem armazenamento (modo privado), o menu funciona e só não lembra', async () => {
    jest.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('bloqueado'); });
    jest.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('bloqueado'); });
    const t = await abrirShell();
    expect(t.app().classList).not.toContain('menu-recolhido');
    t.botao().click();
    await t.atualizar();
    expect(t.app().classList).toContain('menu-recolhido');
  });

  it('no celular a preferência não vale: a gaveta começa fechada e o ☰ só abre e fecha a gaveta', async () => {
    const t = await abrirShell({ celular: true, recolhidoSalvo: true });
    expect(t.app().classList).not.toContain('menu-recolhido');
    expect(t.app().classList).not.toContain('menu-aberto');
    expect(t.botao().getAttribute('aria-expanded')).toBe('false');

    t.botao().click();
    await t.atualizar();
    expect(t.app().classList).toContain('menu-aberto');
    expect(t.botao().getAttribute('aria-expanded')).toBe('true');
    expect(localStorage.getItem(ShellComponent.CHAVE_RECOLHIDO)).toBe('1');   // a escolha do computador fica intacta
  });

  it('virar computador ↔ celular troca o comportamento sem recarregar', async () => {
    const t = await abrirShell({ recolhidoSalvo: true });
    expect(t.app().classList).toContain('menu-recolhido');
    await t.girar(true);
    expect(t.app().classList).not.toContain('menu-recolhido');
    expect(t.botao().getAttribute('aria-expanded')).toBe('false');
    await t.girar(false);
    expect(t.app().classList).toContain('menu-recolhido');
  });

  it('na volta do menu, o conteúdo desliza da posição antiga até a nova (FLIP), sem pulo', async () => {
    const t = await abrirShell({ recolhidoSalvo: true });
    const pagina = t.raiz.querySelector<HTMLElement>('main.pagina')!;
    // jsdom não calcula layout nem tem animate: a posição vem do estado do menu, e animate é espionado.
    const shell = t.shell;
    jest.spyOn(pagina, 'getBoundingClientRect')
      .mockImplementation(() => ({ left: shell.menuRecolhido() ? 133 : 288 }) as DOMRect);
    const animar = jest.fn();
    (pagina as unknown as { animate: jest.Mock }).animate = animar;
    // O ☰ e o título também deslizam com o menu (antes, a barra pulava 252 px de uma vez).
    const inicio = t.raiz.querySelector<HTMLElement>('.barra-inicio')!;
    jest.spyOn(inicio, 'getBoundingClientRect')
      .mockImplementation(() => ({ left: shell.menuRecolhido() ? 0 : 252 }) as DOMRect);
    const animarBarra = jest.fn();
    (inicio as unknown as { animate: jest.Mock }).animate = animarBarra;

    t.botao().click();
    await t.atualizar();
    expect(animar).toHaveBeenCalledTimes(1);
    expect(animar.mock.calls[0][0]).toEqual([{ transform: 'translateX(-155px)' }, { transform: 'none' }]);
    expect(animarBarra.mock.calls[0][0]).toEqual([{ transform: 'translateX(-252px)' }, { transform: 'none' }]);

    t.botao().click();                       // esconder não usa o FLIP (a coluna já anima no CSS)
    await t.atualizar();
    expect(animar).toHaveBeenCalledTimes(1);
  });

  it('com movimento reduzido no sistema, a volta do menu não anima', async () => {
    const t = await abrirShell({ recolhidoSalvo: true });
    const pagina = t.raiz.querySelector<HTMLElement>('main.pagina')!;
    const shell = t.shell;
    jest.spyOn(pagina, 'getBoundingClientRect')
      .mockImplementation(() => ({ left: shell.menuRecolhido() ? 133 : 288 }) as DOMRect);
    const animar = jest.fn();
    (pagina as unknown as { animate: jest.Mock }).animate = animar;
    const original = window.matchMedia;
    window.matchMedia = ((q: string) => (q.includes('reduced-motion') ? { matches: true } : original(q))) as typeof window.matchMedia;

    t.botao().click();
    await t.atualizar();
    expect(animar).not.toHaveBeenCalled();
  });

  describe('gaveta do celular: dá para sair sem escolher uma tela', () => {
    async function abrirGaveta() {
      const t = await abrirShell({ celular: true });
      t.botao().click();
      await t.atualizar();
      expect(t.app().classList).toContain('menu-aberto');
      return t;
    }

    it('tocar no fundo escuro fecha a gaveta', async () => {
      const t = await abrirGaveta();
      t.fundo().click();
      await t.atualizar();
      expect(t.app().classList).not.toContain('menu-aberto');
    });

    it('o ✕ fecha a gaveta e o foco volta ao ☰', async () => {
      const t = await abrirGaveta();
      t.botaoFechar().click();
      await t.atualizar();
      expect(t.app().classList).not.toContain('menu-aberto');
      expect(document.activeElement).toBe(t.botao());
    });

    it('Esc fecha a gaveta e o foco volta ao ☰', async () => {
      const t = await abrirGaveta();
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      await t.atualizar();
      expect(t.app().classList).not.toContain('menu-aberto');
      expect(document.activeElement).toBe(t.botao());
    });

    it('arrastar para a esquerda fecha; arraste curto ou vertical (rolar a lista) não fecha', async () => {
      const t = await abrirGaveta();
      await t.arrastar(-30);
      expect(t.app().classList).toContain('menu-aberto');
      await t.arrastar(-80, 120);
      expect(t.app().classList).toContain('menu-aberto');
      await t.arrastar(-100, 10);
      expect(t.app().classList).not.toContain('menu-aberto');
    });

    it('no computador, Esc não mexe no menu', async () => {
      const t = await abrirShell();
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      await t.atualizar();
      expect(t.app().classList).not.toContain('menu-recolhido');
    });
  });

  it('a marca do menu e o título da barra são links para /painel', async () => {
    const t = await abrirShell();
    expect(t.marca().tagName).toBe('A');
    expect(t.marca().getAttribute('href')).toBe('/painel');
    expect(t.tituloBarra().tagName).toBe('A');
    expect(t.tituloBarra().getAttribute('href')).toBe('/painel');

    t.marca().click();
    await t.atualizar();
    expect(t.router.url).toBe('/painel');
  });

  it('no celular, clicar na marca dentro da gaveta leva ao Painel e fecha a gaveta', async () => {
    const t = await abrirShell({ celular: true });
    t.botao().click();
    await t.atualizar();
    expect(t.app().classList).toContain('menu-aberto');

    t.marca().click();
    await t.atualizar();
    expect(t.router.url).toBe('/painel');
    expect(t.app().classList).not.toContain('menu-aberto');
  });
});
