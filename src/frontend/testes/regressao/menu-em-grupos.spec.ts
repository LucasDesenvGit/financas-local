import { Location } from '@angular/common';
import { provideLocationMocks } from '@angular/common/testing';
import { TestBed } from '@angular/core/testing';
import { Icones } from '../../src/app/compartilhado/icones';
import { MENU, ShellComponent, ehGrupo, filtrarMenu, grupoDaUrl, telasDoMenu } from '../../src/app/layout/shell.component';
import { abrirShell } from '../apoio/shell';

/**
 * Regressão #227 (ADR 0046): o menu lateral em grupos que abrem e fecham (sanfona). Os grupos são os módulos do
 * roteiro (#193); o cabeçalho é um botão com aria-expanded (WAI-ARIA APG, Disclosure); vários grupos abertos ao mesmo
 * tempo (NN/g, Accordions on Desktop); o grupo da tela atual abre sozinho; a escolha fica em financas.menuGrupos
 * (ADR 0017); a gaveta do celular não fecha ao abrir um grupo.
 */
describe('Regressão: menu lateral em grupos (sanfona)', () => {
  const GRUPOS = ['movimentacao', 'planejamento', 'patrimonio', 'analises', 'ajustes'];

  it('Painel solto no topo, os grupos do #193 na ordem, com as telas de cada um', () => {
    expect(ehGrupo(MENU[0])).toBe(false);
    expect((MENU[0] as { rota: string }).rota).toBe('/painel');
    const grupos = MENU.filter(ehGrupo);
    expect(grupos.map((g) => [g.rotulo, g.telas.map((t) => t.rotulo)])).toEqual([
      ['Movimentação', ['Transações', 'Contas e cartões', 'Contas a pagar']],
      ['Planejamento', ['Orçamentos', 'Metas']],
      ['Patrimônio', ['Investimentos', 'Empréstimos e financiamentos']],
      ['Análises', ['Relatórios', 'Visão anual', 'Projeção']],
      ['Ajustes', ['Categorias e regras', 'Configurações']]
    ]);
    expect(grupos.map((g) => g.id)).toEqual(GRUPOS);
  });

  it('todas as telas de antes continuam no menu, como links, e o Manual fica no rodapé', async () => {
    const t = await abrirShell();
    const links = Array.from(t.raiz.querySelectorAll<HTMLAnchorElement>('nav.menu a'));
    const rotas = links.map((a) => a.getAttribute('href'));
    expect(rotas).toEqual([...telasDoMenu().map((i) => i.rota), '/manual']);
    expect(new Set(rotas).size).toBe(14);
    expect(links.at(-1)!.classList).toContain('item-manual');
    // Elementos do menu com data-roteiro estável e único, para o roteiro (#194) apontar sem depender de classe.
    const alvos = Array.from(t.raiz.querySelectorAll('nav.menu [data-roteiro]')).map((e) => e.getAttribute('data-roteiro'));
    expect(new Set(alvos).size).toBe(alvos.length);
    expect(alvos).toEqual(expect.arrayContaining(['menu-grupo-patrimonio', 'menu-tela-emprestimos', 'menu-tela-painel']));
  });

  it('primeira vez: todos os grupos abertos, nada salvo ainda', async () => {
    const t = await abrirShell({ url: '/painel' });
    for (const g of GRUPOS) {
      expect(t.cabecalho(g).getAttribute('aria-expanded')).toBe('true');
      expect(t.telasDoGrupo(g).hasAttribute('inert')).toBe(false);
    }
    expect(localStorage.getItem(ShellComponent.CHAVE_GRUPOS)).toBeNull();
  });

  it('o cabeçalho é um botão com ícone, nome e seta; clique fecha e abre, com aria-expanded acompanhando', async () => {
    const t = await abrirShell({ url: '/painel' });
    const botao = t.cabecalho('patrimonio');
    // <button type="button"> nativo: Enter e Espaço abrem e fecham sem código a mais (teste no navegador: teclado).
    expect(botao.tagName).toBe('BUTTON');
    expect(botao.getAttribute('type')).toBe('button');
    expect(botao.textContent!.trim()).toBe('Patrimônio');
    expect(botao.querySelectorAll('app-icone').length).toBe(2);   // ícone do grupo e a seta
    expect(botao.getAttribute('aria-controls')).toBe('menu-grupo-patrimonio');

    botao.click();
    await t.atualizar();
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(t.telasDoGrupo('patrimonio').classList).toContain('fechado');
    // Tela de grupo fechado fica fora do Tab (WCAG 2.1.1).
    expect(t.telasDoGrupo('patrimonio').hasAttribute('inert')).toBe(true);

    botao.click();
    await t.atualizar();
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    expect(t.telasDoGrupo('patrimonio').hasAttribute('inert')).toBe(false);
  });

  it('vários grupos podem ficar abertos: abrir um não fecha o outro', async () => {
    const t = await abrirShell({ url: '/painel', gruposFechados: ['planejamento', 'analises'] });
    t.cabecalho('planejamento').click();
    await t.atualizar();
    t.cabecalho('analises').click();
    await t.atualizar();
    for (const g of GRUPOS) expect(t.cabecalho(g).getAttribute('aria-expanded')).toBe('true');
  });

  it('o grupo da tela aberta abre sozinho: pela URL, por um link e pelo voltar do navegador', async () => {
    // Histórico simulado (o do jsdom não avisa o roteador a tempo): o voltar dispara o popstate como no navegador.
    TestBed.configureTestingModule({ providers: [provideLocationMocks()] });
    // Pela URL: entra direto numa tela de grupo que estava fechado.
    const t = await abrirShell({ url: '/emprestimos', gruposFechados: ['patrimonio', 'planejamento'] });
    t.router.setUpLocationChangeListener();   // no site, quem liga é o bootstrap do roteador
    expect(t.cabecalho('patrimonio').getAttribute('aria-expanded')).toBe('true');
    expect(t.cabecalho('planejamento').getAttribute('aria-expanded')).toBe('false');

    // Por um link de outra tela (o roteador navega, sem passar pelo menu).
    await t.router.navigateByUrl('/metas?meta=3');
    await t.atualizar();
    expect(t.cabecalho('planejamento').getAttribute('aria-expanded')).toBe('true');

    // Voltar: fecha Patrimônio estando em Metas e volta para Empréstimos.
    t.cabecalho('patrimonio').click();
    await t.atualizar();
    TestBed.inject(Location).back();
    await new Promise((r) => setTimeout(r, 0));   // o roteador trata o popstate na volta seguinte do laço de eventos
    await t.atualizar();
    expect(t.router.url).toBe('/emprestimos');
    expect(t.cabecalho('patrimonio').getAttribute('aria-expanded')).toBe('true');
  });

  it('fechar o grupo da tela atual: o cabeçalho avisa "tela atual está aqui", e trocar só o filtro não reabre', async () => {
    const t = await abrirShell();   // em /transacoes
    const botao = t.cabecalho('movimentacao');
    expect(botao.classList).not.toContain('contem-atual');
    botao.click();
    await t.atualizar();
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    expect(botao.classList).toContain('contem-atual');
    expect(botao.querySelector('.so-leitor')!.textContent).toBe(', tela atual está aqui');
    expect(botao.querySelector('.grupo-marca-atual')).not.toBeNull();
    // Outro grupo fechado não ganha a marca.
    t.cabecalho('ajustes').click();
    await t.atualizar();
    expect(t.cabecalho('ajustes').classList).not.toContain('contem-atual');

    await t.router.navigateByUrl('/transacoes?categoria=sem');   // mesma tela, filtro novo (replaceUrl nas telas)
    await t.atualizar();
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    // Aberto de novo, a marca sai (a tela já aparece destacada dentro do grupo).
    botao.click();
    await t.atualizar();
    expect(botao.classList).not.toContain('contem-atual');
    expect(botao.querySelector('.so-leitor')).toBeNull();
  });

  it('a escolha fica salva em financas.menuGrupos e volta ao recarregar', async () => {
    const t = await abrirShell({ url: '/painel' });
    t.cabecalho('ajustes').click();
    await t.atualizar();
    t.cabecalho('patrimonio').click();
    await t.atualizar();
    expect(JSON.parse(localStorage.getItem(ShellComponent.CHAVE_GRUPOS)!)).toEqual(['ajustes', 'patrimonio']);

    TestBed.resetTestingModule();
    const recarregado = await abrirShell({ url: '/painel' });
    expect(recarregado.cabecalho('ajustes').getAttribute('aria-expanded')).toBe('false');
    expect(recarregado.cabecalho('patrimonio').getAttribute('aria-expanded')).toBe('false');
    expect(recarregado.cabecalho('movimentacao').getAttribute('aria-expanded')).toBe('true');
  });

  it('valor estranho salvo vale o padrão (tudo aberto); id que não existe é ignorado', async () => {
    localStorage.setItem(ShellComponent.CHAVE_GRUPOS, '{quebrado');
    const t = await abrirShell({ url: '/painel' });
    for (const g of GRUPOS) expect(t.cabecalho(g).getAttribute('aria-expanded')).toBe('true');

    TestBed.resetTestingModule();
    localStorage.setItem(ShellComponent.CHAVE_GRUPOS, JSON.stringify(['grupo-que-saiu', 7, 'ajustes']));
    const outro = await abrirShell({ url: '/painel' });
    expect(outro.cabecalho('ajustes').getAttribute('aria-expanded')).toBe('false');
    expect(outro.cabecalho('analises').getAttribute('aria-expanded')).toBe('true');
  });

  it('sem armazenamento (modo privado), os grupos abrem e fecham e só não lembram', async () => {
    jest.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('bloqueado'); });
    jest.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('bloqueado'); });
    const t = await abrirShell({ url: '/painel' });
    expect(t.cabecalho('analises').getAttribute('aria-expanded')).toBe('true');
    t.cabecalho('analises').click();
    await t.atualizar();
    expect(t.cabecalho('analises').getAttribute('aria-expanded')).toBe('false');
  });

  it('no celular, tocar no grupo abre e fecha sem fechar a gaveta; tocar numa tela fecha', async () => {
    const t = await abrirShell({ celular: true, url: '/painel' });
    t.botao().click();
    await t.atualizar();
    expect(t.app().classList).toContain('menu-aberto');

    t.cabecalho('planejamento').click();
    await t.atualizar();
    expect(t.cabecalho('planejamento').getAttribute('aria-expanded')).toBe('false');
    expect(t.app().classList).toContain('menu-aberto');
    t.cabecalho('planejamento').click();
    await t.atualizar();
    expect(t.app().classList).toContain('menu-aberto');

    t.linkDaTela('/metas').click();
    await t.atualizar();
    expect(t.router.url).toBe('/metas');
    expect(t.app().classList).not.toContain('menu-aberto');
  });

  it('o ☰ do computador (#131) não mexe nos grupos', async () => {
    const t = await abrirShell({ url: '/painel', gruposFechados: ['ajustes'] });
    t.botao().click();
    await t.atualizar();
    t.botao().click();
    await t.atualizar();
    expect(t.cabecalho('ajustes').getAttribute('aria-expanded')).toBe('false');
    expect(t.cabecalho('analises').getAttribute('aria-expanded')).toBe('true');
  });

  it('roteiro (#194): mostrar uma tela no menu abre o grupo dela e, no celular, a gaveta', async () => {
    const t = await abrirShell({ celular: true, url: '/painel', gruposFechados: ['analises'] });
    t.shell.mostrarTelaNoMenu('/projecao');
    await t.atualizar();
    expect(t.cabecalho('analises').getAttribute('aria-expanded')).toBe('true');
    expect(t.app().classList).toContain('menu-aberto');
  });

  describe('módulos que se ligam e desligam (#190): o filtro já está pronto', () => {
    it('tela escondida some do grupo; grupo sem nenhuma tela some também', () => {
      const semEmprestimos = filtrarMenu(MENU, (tela) => tela.rota !== '/emprestimos');
      const patrimonio = semEmprestimos.filter(ehGrupo).find((g) => g.id === 'patrimonio')!;
      expect(patrimonio.telas.map((x) => x.rota)).toEqual(['/investimentos']);

      const semPatrimonio = filtrarMenu(MENU, (tela) => !['/investimentos', '/emprestimos'].includes(tela.rota));
      expect(semPatrimonio.filter(ehGrupo).map((g) => g.id)).toEqual(['movimentacao', 'planejamento', 'analises', 'ajustes']);
      expect(MENU.filter(ehGrupo).length).toBe(5);   // o MENU original fica intacto
    });

    it('na tela, trocar o filtro esconde a tela e o cabeçalho do grupo vazio', async () => {
      const t = await abrirShell({ url: '/painel' });
      t.shell.telaVisivel.set((tela) => !['/categorias', '/configuracoes'].includes(tela.rota));
      await t.atualizar();
      expect(t.cabecalho('ajustes')).toBeNull();
      expect(t.raiz.querySelector('a[href="/categorias"]')).toBeNull();
      expect(t.cabecalho('analises')).not.toBeNull();
    });
  });

  it('grupoDaUrl acha o grupo pela tela, com filtros, âncoras e subcaminhos; tela solta não tem grupo', () => {
    expect(grupoDaUrl('/transacoes?categoria=sem')!.id).toBe('movimentacao');
    expect(grupoDaUrl('/configuracoes?aba=erros#x')!.id).toBe('ajustes');
    expect(grupoDaUrl('/contas-a-pagar')!.id).toBe('movimentacao');
    expect(grupoDaUrl('/painel')).toBeNull();
    expect(grupoDaUrl('/manual/contas')).toBeNull();
  });

  it('ícones próprios para cada grupo, diferentes dos das telas', () => {
    const telas = new Set(telasDoMenu().map((t) => t.icone));
    for (const g of MENU.filter(ehGrupo)) expect(telas.has(g.icone)).toBe(false);
    expect(Icones.setaGrupo).toBeDefined();
  });
});
