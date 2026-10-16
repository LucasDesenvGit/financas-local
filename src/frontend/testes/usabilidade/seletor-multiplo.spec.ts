import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { OpcaoMultipla, SeletorMultiploComponent } from '../../src/app/compartilhado/seletor-multiplo.component';
import { abrirTransacoes, parametrosDaUrl } from '../apoio/transacoes';

@Component({
  imports: [SeletorMultiploComponent],
  template: `
    <app-seletor-multiplo rotulo="Categoria" [opcoes]="opcoes" [selecionadas]="marcadas()" (mudou)="marcadas.set($event)" />
    <p class="fora">fora</p>`
})
class Hospedeiro {
  readonly opcoes: OpcaoMultipla[] = [
    { valor: 'sem', rotulo: 'Sem categoria' },
    { valor: '20', rotulo: 'Lazer', cor: '#f3b3d3' },
    { valor: '21', rotulo: 'Cinema', nivel: 1 },
    { valor: '12', rotulo: 'Mercado' }
  ];
  readonly marcadas = signal<string[]>([]);
}

function montar() {
  const fixture = TestBed.createComponent(Hospedeiro);
  fixture.detectChanges();
  const raiz = fixture.nativeElement as HTMLElement;
  const botao = raiz.querySelector<HTMLButtonElement>('.seletor-multiplo-botao')!;
  const painel = () => raiz.querySelector<HTMLElement>('.seletor-multiplo-painel');
  const caixa = (rotulo: string) => Array.from(raiz.querySelectorAll<HTMLLabelElement>('.seletor-multiplo-opcao'))
    .find((l) => l.textContent!.trim() === rotulo)!.querySelector<HTMLInputElement>('input')!;
  const clicar = (el: HTMLElement) => { el.click(); fixture.detectChanges(); };
  return { fixture, raiz, botao, painel, caixa, clicar };
}

/**
 * Usabilidade do filtro de várias categorias. Caixas de marcar no lugar de <select multiple> (que pede Ctrl+clique e
 * esconde o que está marcado). Nielsen 1: o botão resume a escolha sem abrir; Nielsen 3: desmarcar tudo com um clique;
 * WCAG 4.1.2 e 2.1.1: botão nativo com estado, lista com nome, Esc fecha e devolve o foco.
 */
describe('Usabilidade: seletor de várias categorias', () => {
  it('o botão tem nome (rótulo + escolha atual) e estado aberto/fechado', () => {
    const { raiz, botao } = montar();
    expect(botao.tagName).toBe('BUTTON');
    expect(botao.getAttribute('aria-expanded')).toBe('false');
    const nome = botao.getAttribute('aria-labelledby')!.split(' ').map((id) => raiz.querySelector(`#${id}`)!.textContent!.trim());
    expect(nome).toEqual(['Categoria', 'Todas']);
  });

  it('abre uma lista de caixas de marcar com nome, e a subcategoria aparece recuada', () => {
    const { botao, painel, clicar } = montar();
    clicar(botao);
    const p = painel()!;
    expect(botao.getAttribute('aria-expanded')).toBe('true');
    expect(botao.getAttribute('aria-controls')).toBe(p.id);
    expect(p.getAttribute('role')).toBe('group');
    expect(p.querySelectorAll('input[type=checkbox]').length).toBe(4);
    expect(p.querySelector('.seletor-multiplo-opcao.sub')!.textContent!.trim()).toBe('Cinema');
  });

  it('o resumo mostra o que está marcado sem precisar abrir', () => {
    const { fixture, botao, caixa, clicar } = montar();
    clicar(botao);
    clicar(caixa('Mercado'));
    expect(botao.textContent).toContain('Mercado');
    clicar(caixa('Lazer'));
    clicar(caixa('Sem categoria'));
    expect(botao.textContent).toContain('3 categorias');
    expect(fixture.componentInstance.marcadas()).toEqual(['12', '20', 'sem']);
  });

  it('marcar não fecha a lista (dá para marcar várias seguidas); desmarcar tira só aquela', () => {
    const { fixture, painel, botao, caixa, clicar } = montar();
    clicar(botao);
    clicar(caixa('Mercado'));
    clicar(caixa('Lazer'));
    expect(painel()).not.toBeNull();
    clicar(caixa('Mercado'));
    expect(fixture.componentInstance.marcadas()).toEqual(['20']);
  });

  it('"Todas" desmarca tudo de uma vez e fica desabilitado quando não há nada marcado', () => {
    const { fixture, raiz, botao, caixa, clicar } = montar();
    clicar(botao);
    const todas = () => raiz.querySelector<HTMLButtonElement>('.seletor-multiplo-limpar')!;
    expect(todas().disabled).toBe(true);
    clicar(caixa('Mercado'));
    clicar(caixa('Lazer'));
    clicar(todas());
    expect(fixture.componentInstance.marcadas()).toEqual([]);
    expect(botao.textContent).toContain('Todas');
  });

  it('Esc fecha e devolve o foco ao botão; clique fora também fecha', () => {
    const { fixture, raiz, botao, painel, caixa, clicar } = montar();
    clicar(botao);
    caixa('Mercado').focus();
    caixa('Mercado').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    fixture.detectChanges();
    expect(painel()).toBeNull();
    expect(document.activeElement).toBe(botao);

    clicar(botao);
    clicar(raiz.querySelector<HTMLElement>('.fora')!);
    expect(painel()).toBeNull();
  });
});

describe('Usabilidade: chips com várias categorias em Transações', () => {
  it('um chip por categoria marcada, cada um com o seu ✕', async () => {
    const t = await abrirTransacoes('/transacoes?categoria=12,sem,21');
    const rotulos = Array.from(t.elemento.querySelectorAll('.chip-filtro'))
      .map((c) => c.textContent!.replace('✕', '').replace(/\s+/g, ' ').trim());
    expect(rotulos).toEqual(['Sem categoria', 'Mercado', 'Lazer › Cinema']);

    t.elemento.querySelector<HTMLButtonElement>('[aria-label="Remover filtro: Mercado"]')!.click();
    await t.assentar();
    expect(parametrosDaUrl(t.router)).toEqual({ categoria: 'sem,21' });
    expect(t.buscas.at(-1)).toMatchObject({ categoriaIds: [21], semCategoria: true });
  });

  it('o filtro da tela mostra a escolha vinda do link', async () => {
    const t = await abrirTransacoes('/transacoes?categoria=12,20');
    expect(t.elemento.querySelector('.seletor-multiplo-botao')!.textContent).toContain('2 categorias');
  });
});
